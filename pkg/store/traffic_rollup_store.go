package store

import (
	"gorm.io/gorm"
)

// 降采样聚合层的桶宽（秒）
const (
	RollupBucket10m = 600
	RollupBucket1h  = 3600
)

// TrafficRollupStore 降采样聚合层的读写：
// - 滚动任务周期性地把 raw 层/10分钟层中"已完结"的桶重算进聚合层（先删后插，幂等）
// - 查询端按时间范围路由到 raw 层或聚合层，聚合层的行再按请求桶宽重新分桶
type TrafficRollupStore struct {
	db *gorm.DB
}

func NewTrafficRollupStore(db *gorm.DB) *TrafficRollupStore {
	return &TrafficRollupStore{db: db}
}

// RollupIpFromRaw 将 raw 层 [from,to) 内的 IP 维度数据重算进指定桶宽的 IP 聚合层
func (s *TrafficRollupStore) RollupIpFromRaw(bucketSize int64, from, to int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteRollupIp(tx, bucketSize, from, to); err != nil {
			return err
		}
		return tx.Exec(
			`INSERT INTO traffic_ip_rollup (bucket_size, bucket_ts, remote_ip, bytes_in, bytes_out, packets_in, packets_out, conn_max)
			 SELECT ?, (ts / ?) * ?, remote_ip, SUM(bytes_in), SUM(bytes_out), SUM(packets_in), SUM(packets_out), MAX(connections)
			 FROM traffic_samples
			 WHERE ts >= ? AND ts < ?
			 GROUP BY (ts / ?) * ?, remote_ip`,
			bucketSize, bucketSize, bucketSize, from, to, bucketSize, bucketSize,
		).Error
	})
}

// RollupPortFromRaw 将 raw 层 [from,to) 内的端口维度数据重算进指定桶宽的端口聚合层
func (s *TrafficRollupStore) RollupPortFromRaw(bucketSize int64, from, to int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteRollupPort(tx, bucketSize, from, to); err != nil {
			return err
		}
		return tx.Exec(
			`INSERT INTO traffic_port_rollup (bucket_size, bucket_ts, remote_ip, proto, port, bytes_in, bytes_out, packets_in, packets_out)
			 SELECT ?, (ts / ?) * ?, remote_ip, proto, port, SUM(bytes_in), SUM(bytes_out), SUM(packets_in), SUM(packets_out)
			 FROM traffic_port_samples
			 WHERE ts >= ? AND ts < ?
			 GROUP BY (ts / ?) * ?, remote_ip, proto, port`,
			bucketSize, bucketSize, bucketSize, from, to, bucketSize, bucketSize,
		).Error
	})
}

// RollupIpFrom10m 将 10 分钟 IP 聚合层 [from,to) 内的桶重算进 1 小时层
func (s *TrafficRollupStore) RollupIpFrom10m(from, to int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteRollupIp(tx, RollupBucket1h, from, to); err != nil {
			return err
		}
		return tx.Exec(
			`INSERT INTO traffic_ip_rollup (bucket_size, bucket_ts, remote_ip, bytes_in, bytes_out, packets_in, packets_out, conn_max)
			 SELECT ?, (bucket_ts / ?) * ?, remote_ip, SUM(bytes_in), SUM(bytes_out), SUM(packets_in), SUM(packets_out), MAX(conn_max)
			 FROM traffic_ip_rollup
			 WHERE bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?
			 GROUP BY (bucket_ts / ?) * ?, remote_ip`,
			RollupBucket1h, RollupBucket1h, RollupBucket1h, RollupBucket10m, from, to, RollupBucket1h, RollupBucket1h,
		).Error
	})
}

// RollupPortFrom10m 将 10 分钟端口聚合层 [from,to) 内的桶重算进 1 小时层
func (s *TrafficRollupStore) RollupPortFrom10m(from, to int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteRollupPort(tx, RollupBucket1h, from, to); err != nil {
			return err
		}
		return tx.Exec(
			`INSERT INTO traffic_port_rollup (bucket_size, bucket_ts, remote_ip, proto, port, bytes_in, bytes_out, packets_in, packets_out)
			 SELECT ?, (bucket_ts / ?) * ?, remote_ip, proto, port, SUM(bytes_in), SUM(bytes_out), SUM(packets_in), SUM(packets_out)
			 FROM traffic_port_rollup
			 WHERE bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?
			 GROUP BY (bucket_ts / ?) * ?, remote_ip, proto, port`,
			RollupBucket1h, RollupBucket1h, RollupBucket1h, RollupBucket10m, from, to, RollupBucket1h, RollupBucket1h,
		).Error
	})
}

func deleteRollupIp(tx *gorm.DB, bucketSize int64, from, to int64) error {
	return tx.Exec(
		`DELETE FROM traffic_ip_rollup WHERE bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?`,
		bucketSize, from, to,
	).Error
}

func deleteRollupPort(tx *gorm.DB, bucketSize int64, from, to int64) error {
	return tx.Exec(
		`DELETE FROM traffic_port_rollup WHERE bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?`,
		bucketSize, from, to,
	).Error
}

// QueryIpRows 查询指定桶宽与区间的 IP 聚合原始行（测试与调试用）
func (s *TrafficRollupStore) QueryIpRows(bucketSize int64, from, to int64, out any) error {
	return s.db.Where("bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?", bucketSize, from, to).Find(out).Error
}

// CleanupIp 删除 IP 聚合层保留期之前的行
func (s *TrafficRollupStore) CleanupIp(bucketSize int64, before int64) (int64, error) {
	result := s.db.Where("bucket_size = ? AND bucket_ts < ?", bucketSize, before).Delete(&TrafficIpRollup{})
	return result.RowsAffected, result.Error
}

// CleanupPort 删除端口聚合层保留期之前的行
func (s *TrafficRollupStore) CleanupPort(bucketSize int64, before int64) (int64, error) {
	result := s.db.Where("bucket_size = ? AND bucket_ts < ?", bucketSize, before).Delete(&TrafficPortRollup{})
	return result.RowsAffected, result.Error
}

// --- 查询端 ---

// QueryIpHistory 从指定桶宽的 IP 聚合层按请求桶宽重新分桶查询
func (s *TrafficRollupStore) QueryIpHistory(bucketSize int64, start, end, bucket int64, ip string) ([]HistoryPoint, error) {
	if bucket < bucketSize {
		bucket = bucketSize
	}
	query := s.db.Model(&TrafficIpRollup{}).
		Select("(bucket_ts / ?) * ? AS bucket_ts, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(packets_in) AS packets_in, SUM(packets_out) AS packets_out",
			bucket, bucket).
		Where("bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?", bucketSize, start, end).
		Group("bucket_ts").
		Order("bucket_ts ASC")
	if ip != "" {
		query = query.Where("remote_ip = ?", ip)
	}

	var points []HistoryPoint
	if err := query.Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

// QueryIpTop 从指定桶宽的 IP 聚合层统计流量最大的前 limit 个 IP
func (s *TrafficRollupStore) QueryIpTop(bucketSize int64, start, end int64, limit int) ([]TopEntry, error) {
	var entries []TopEntry
	err := s.db.Model(&TrafficIpRollup{}).
		Select("remote_ip, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(bytes_in + bytes_out) AS bytes_sum, MAX(bucket_ts) AS last_seen").
		Where("bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?", bucketSize, start, end).
		Group("remote_ip").
		Order("bytes_sum DESC").
		Limit(limit).
		Scan(&entries).Error
	return entries, err
}

// QueryPortHistory 从指定桶宽的端口聚合层查询端口维度历史
func (s *TrafficRollupStore) QueryPortHistory(bucketSize int64, start, end, bucket int64, filter PortHistoryFilter) ([]PortHistoryPoint, error) {
	if bucket < bucketSize {
		bucket = bucketSize
	}
	query := s.db.Model(&TrafficPortRollup{}).
		Select("(bucket_ts / ?) * ? AS bucket_ts, proto, port, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(packets_in) AS packets_in, SUM(packets_out) AS packets_out",
			bucket, bucket).
		Where("bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?", bucketSize, start, end).
		Group("bucket_ts, proto, port").
		Order("bucket_ts ASC")
	query = applyPortFilter(query, filter)

	var points []PortHistoryPoint
	if err := query.Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

// QueryProtoHistory 从指定桶宽的端口聚合层按协议聚合查询历史（协议维度可由端口行上卷得到）
func (s *TrafficRollupStore) QueryProtoHistory(bucketSize int64, start, end, bucket int64, ip string) ([]ProtoHistoryPoint, error) {
	if bucket < bucketSize {
		bucket = bucketSize
	}
	query := s.db.Model(&TrafficPortRollup{}).
		Select("(bucket_ts / ?) * ? AS bucket_ts, proto, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(packets_in) AS packets_in, SUM(packets_out) AS packets_out",
			bucket, bucket).
		Where("bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?", bucketSize, start, end).
		Group("bucket_ts, proto").
		Order("bucket_ts ASC")
	if ip != "" {
		query = query.Where("remote_ip = ?", ip)
	}

	var points []ProtoHistoryPoint
	if err := query.Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

// QueryPortTop 从指定桶宽的端口聚合层统计流量最大的前 limit 个端口
func (s *TrafficRollupStore) QueryPortTop(bucketSize int64, start, end int64, limit int, filter PortHistoryFilter) ([]PortTopEntry, error) {
	query := s.db.Model(&TrafficPortRollup{}).
		Select("proto, port, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, SUM(bytes_in + bytes_out) AS bytes_sum").
		Where("bucket_size = ? AND bucket_ts >= ? AND bucket_ts < ?", bucketSize, start, end).
		Group("proto, port").
		Order("bytes_sum DESC").
		Limit(limit)
	query = applyPortFilter(query, filter)

	var entries []PortTopEntry
	if err := query.Scan(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

// Checkpoint 对 SQLite 执行 WAL checkpoint，控制 WAL 文件膨胀（仅 sqlite 有效）
func (s *TrafficRollupStore) Checkpoint() error {
	return s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
}
