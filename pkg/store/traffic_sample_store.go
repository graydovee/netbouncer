package store

import (
	"gorm.io/gorm"
)

// TrafficSampleStore 流量历史采样点的存储
type TrafficSampleStore struct {
	db *gorm.DB
}

func NewTrafficSampleStore(db *gorm.DB) *TrafficSampleStore {
	return &TrafficSampleStore{db: db}
}

// InsertBatch 批量写入采样点（事务）
func (s *TrafficSampleStore) InsertBatch(samples []TrafficSample) error {
	if len(samples) == 0 {
		return nil
	}
	return s.db.CreateInBatches(samples, 1000).Error
}

// HistoryPoint 按时间桶聚合后的流量点
type HistoryPoint struct {
	BucketTs   int64  `json:"ts"`
	BytesIn    uint64 `json:"bytes_in"`
	BytesOut   uint64 `json:"bytes_out"`
	PacketsIn  uint64 `json:"packets_in"`
	PacketsOut uint64 `json:"packets_out"`
}

// TopEntry 时间范围内按总流量排序的 IP 条目
type TopEntry struct {
	RemoteIP string `json:"ip"`
	BytesIn  uint64 `json:"bytes_in"`
	BytesOut uint64 `json:"bytes_out"`
	BytesSum uint64 `json:"bytes_sum"`
	LastSeen int64  `json:"last_seen"`
}

// QueryHistory 按时间桶聚合查询 [start,end] 区间内的流量增量。
// ip 为空时汇总所有 IP，否则仅统计该 IP。
// 桶编号 = ts / bucket * bucket，保证桶边界对齐到整除点。
func (s *TrafficSampleStore) QueryHistory(start, end, bucket int64, ip string) ([]HistoryPoint, error) {
	if bucket <= 0 {
		bucket = 60
	}

	query := s.db.Model(&TrafficSample{}).
		Select("(ts / ?) * ? AS bucket_ts, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(packets_in) AS packets_in, SUM(packets_out) AS packets_out",
			bucket, bucket).
		Where("ts >= ? AND ts < ?", start, end).
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

// QueryTop 统计 [start,end] 区间内流量最大的前 limit 个 IP
func (s *TrafficSampleStore) QueryTop(start, end int64, limit int) ([]TopEntry, error) {
	if limit <= 0 {
		limit = 10
	}

	var entries []TopEntry
	err := s.db.Model(&TrafficSample{}).
		Select("remote_ip, SUM(bytes_in) AS bytes_in, SUM(bytes_out) AS bytes_out, "+
			"SUM(bytes_in + bytes_out) AS bytes_sum, MAX(ts) AS last_seen").
		Where("ts >= ? AND ts < ?", start, end).
		Group("remote_ip").
		Order("bytes_sum DESC").
		Limit(limit).
		Scan(&entries).Error
	return entries, err
}

// Cleanup 删除保留期之前的采样点，返回删除的行数
func (s *TrafficSampleStore) Cleanup(before int64) (int64, error) {
	result := s.db.Where("ts < ?", before).Delete(&TrafficSample{})
	return result.RowsAffected, result.Error
}
