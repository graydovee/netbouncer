package store

import (
	"gorm.io/gorm"
)

// TrafficPortSampleStore 流量历史采样点（IP×协议×端口维度）的存储
type TrafficPortSampleStore struct {
	db *gorm.DB
}

func NewTrafficPortSampleStore(db *gorm.DB) *TrafficPortSampleStore {
	return &TrafficPortSampleStore{db: db}
}

// InsertBatch 批量写入采样点
func (s *TrafficPortSampleStore) InsertBatch(samples []TrafficPortSample) error {
	if len(samples) == 0 {
		return nil
	}
	return s.db.CreateInBatches(samples, 1000).Error
}

// PortHistoryPoint 按时间桶聚合后的端口维度流量点
type PortHistoryPoint struct {
	BucketTs   int64  `json:"ts"`
	Proto      string `json:"proto"`
	Port       int    `json:"port"`
	BytesIn    uint64 `json:"bytes_in"`
	BytesOut   uint64 `json:"bytes_out"`
	PacketsIn  uint64 `json:"packets_in"`
	PacketsOut uint64 `json:"packets_out"`
}

// PortHistoryFilter 端口维度历史查询条件（字段为零值时不过滤）
type PortHistoryFilter struct {
	RemoteIP string
	Proto    string
	Port     int // -1 表示不过滤（0 是合法值，代表"其他"）
}

// QueryHistory 按时间桶聚合查询端口维度的流量增量
func (s *TrafficPortSampleStore) QueryHistory(start, end, bucket int64, filter PortHistoryFilter) ([]PortHistoryPoint, error) {
	if bucket <= 0 {
		bucket = 60
	}

	query := s.db.Model(&TrafficPortSample{}).
		Select("(ts / ?) * ? AS bucket_ts, proto, port, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(packets_in) AS packets_in, SUM(packets_out) AS packets_out",
			bucket, bucket).
		Where("ts >= ? AND ts < ?", start, end).
		Group("bucket_ts, proto, port").
		Order("bucket_ts ASC")
	query = applyPortFilter(query, filter)

	var points []PortHistoryPoint
	if err := query.Scan(&points).Error; err != nil {
		return nil, err
	}
	return points, nil
}

// PortTopEntry 端口维度流量排行条目
type PortTopEntry struct {
	Proto    string `json:"proto"`
	Port     int    `json:"port"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	BytesSum uint64 `json:"bytes_sum"`
}

// QueryTopPorts 统计区间内流量最大的前 limit 个端口
func (s *TrafficPortSampleStore) QueryTopPorts(start, end int64, limit int, filter PortHistoryFilter) ([]PortTopEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	query := s.db.Model(&TrafficPortSample{}).
		Select("proto, port, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, SUM(bytes_in + bytes_out) AS bytes_sum").
		Where("ts >= ? AND ts < ?", start, end).
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

// ProtoHistoryPoint 按时间桶聚合后的协议维度流量点
type ProtoHistoryPoint struct {
	BucketTs   int64  `json:"ts"`
	Proto      string `json:"proto"`
	BytesIn    uint64 `json:"bytes_in"`
	BytesOut   uint64 `json:"bytes_out"`
	PacketsIn  uint64 `json:"packets_in"`
	PacketsOut uint64 `json:"packets_out"`
}

// QueryProtoHistory 按时间桶聚合查询协议维度的流量增量（所有端口行上卷到协议）
func (s *TrafficPortSampleStore) QueryProtoHistory(start, end, bucket int64, ip string) ([]ProtoHistoryPoint, error) {
	if bucket <= 0 {
		bucket = 60
	}

	query := s.db.Model(&TrafficPortSample{}).
		Select("(ts / ?) * ? AS bucket_ts, proto, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(packets_in) AS packets_in, SUM(packets_out) AS packets_out",
			bucket, bucket).
		Where("ts >= ? AND ts < ?", start, end).
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

// Cleanup 删除保留期之前的采样点，返回删除的行数
func (s *TrafficPortSampleStore) Cleanup(before int64) (int64, error) {
	result := s.db.Where("ts < ?", before).Delete(&TrafficPortSample{})
	return result.RowsAffected, result.Error
}

func applyPortFilter(query *gorm.DB, filter PortHistoryFilter) *gorm.DB {
	if filter.RemoteIP != "" {
		query = query.Where("remote_ip = ?", filter.RemoteIP)
	}
	if filter.Proto != "" {
		query = query.Where("proto = ?", filter.Proto)
	}
	if filter.Port >= 0 {
		query = query.Where("port = ?", filter.Port)
	}
	return query
}
