package service

import (
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

// 历史查询参数边界
const (
	minHistoryBucket = 10        // 最小聚合桶宽（秒）
	maxHistoryBucket = 86400     // 最大聚合桶宽（秒）
	maxHistoryLimit  = 100       // Top 榜最大条数
	maxHistoryRange  = 86400 * 5 // 最大查询跨度（秒）
)

// clampHistoryBounds 校验并修正历史查询的时间区间与桶宽
func clampHistoryBounds(start, end, bucket int64) (int64, int64, int64, bool) {
	now := time.Now().Unix()
	if start <= 0 {
		start = now - 86400
	}
	if end <= 0 {
		end = now
	}
	if end > now {
		end = now
	}
	if start >= end {
		return 0, 0, 0, false
	}
	if end-start > maxHistoryRange {
		start = end - maxHistoryRange
	}
	if bucket < minHistoryBucket {
		bucket = minHistoryBucket
	}
	if bucket > maxHistoryBucket {
		bucket = maxHistoryBucket
	}
	return start, end, bucket, true
}

// TrafficHistory 查询流量历史趋势：ip 为空时为全服汇总，否则为单 IP 曲线
func (s *NetService) TrafficHistory(start, end, bucket int64, ip string) ([]store.HistoryPoint, error) {
	start, end, bucket, ok := clampHistoryBounds(start, end, bucket)
	if !ok {
		return nil, Invalidf("时间区间无效: start=%d end=%d", start, end)
	}

	points, err := s.store.TrafficSampleStore.QueryHistory(start, end, bucket, ip)
	if err != nil {
		return nil, Internalf("查询流量历史失败: %v", err)
	}
	return points, nil
}

// TrafficHistoryTop 查询时间范围内流量最大的前 limit 个 IP
func (s *NetService) TrafficHistoryTop(start, end int64, limit int) ([]store.TopEntry, error) {
	start, end, _, ok := clampHistoryBounds(start, end, minHistoryBucket)
	if !ok {
		return nil, Invalidf("时间区间无效: start=%d end=%d", start, end)
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}

	entries, err := s.store.TrafficSampleStore.QueryTop(start, end, limit)
	if err != nil {
		return nil, Internalf("查询流量排行失败: %v", err)
	}
	return entries, nil
}
