package service

import (
	"sort"
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

// 历史查询参数边界
const (
	minHistoryBucket = 10         // 最小聚合桶宽（秒）
	maxHistoryBucket = 86400      // 最大聚合桶宽（秒）
	maxHistoryLimit  = 100        // Top 榜最大条数
	maxHistoryRange  = 86400 * 30 // 最大查询跨度（秒），由 1 小时聚合层支撑
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

// splitPlan 分层查询计划：把一个时间区间按 raw 保留边界切分为聚合层部分与 raw 部分
type splitPlan struct {
	effBucket int64 // 实际使用的桶宽（涉及聚合层时不小于聚合层粒度）

	useRollup   bool
	rollupLayer int64 // store.RollupBucket10m / RollupBucket1h
	rollupFrom  int64
	rollupTo    int64

	useRaw  bool
	rawFrom int64
	rawTo   int64
}

// planRange 生成查询计划。
// rawCutoff 为 raw 层最早保留时间；聚合层负责 [start, boundary+effBucket)，
// raw 负责 [boundary+effBucket, end)，其中 boundary 为跨越 rawCutoff 的聚合桶起点，
// 保证两段无缝且不重叠（跨越桶由聚合层完整提供，避免 raw 已被清理造成缺口或重复计数）
func planRange(start, end, bucket, rawCutoff int64) splitPlan {
	if start >= rawCutoff {
		return splitPlan{effBucket: bucket, useRaw: true, rawFrom: start, rawTo: end}
	}

	layer := pickRollupLayer(start, end, bucket)
	effBucket := bucket
	if effBucket < layer {
		effBucket = layer
	}

	if end <= rawCutoff {
		return splitPlan{
			effBucket: effBucket, useRollup: true,
			rollupLayer: layer, rollupFrom: start, rollupTo: end,
		}
	}

	boundary := rawCutoff - rawCutoff%effBucket
	splitAt := boundary + effBucket
	plan := splitPlan{effBucket: effBucket, rollupLayer: layer}
	if start < splitAt {
		plan.useRollup = true
		plan.rollupFrom = start
		plan.rollupTo = splitAt
	}
	if end > splitAt {
		plan.useRaw = true
		plan.rawFrom = splitAt
		plan.rawTo = end
	}
	return plan
}

// pickRollupLayer 选择聚合层：桶宽不小于 1 小时或跨度超出 10 分钟层保留窗口时用 1 小时层
func pickRollupLayer(start, end, bucket int64) int64 {
	if bucket >= store.RollupBucket1h {
		return store.RollupBucket1h
	}
	if end-start > int64(rollup10mRetention/time.Second)-store.RollupBucket10m {
		return store.RollupBucket1h
	}
	return store.RollupBucket10m
}

// mergeHistoryPoints 合并多段查询结果，同一桶的增量求和（增量数据可加和）
func mergeHistoryPoints(parts ...[]store.HistoryPoint) []store.HistoryPoint {
	if len(parts) == 1 {
		return parts[0]
	}
	index := make(map[int64]int)
	merged := make([]store.HistoryPoint, 0)
	for _, part := range parts {
		for _, p := range part {
			if i, ok := index[p.BucketTs]; ok {
				merged[i].BytesIn += p.BytesIn
				merged[i].BytesOut += p.BytesOut
				merged[i].PacketsIn += p.PacketsIn
				merged[i].PacketsOut += p.PacketsOut
				continue
			}
			index[p.BucketTs] = len(merged)
			merged = append(merged, p)
		}
	}
	sortHistoryPoints(merged)
	return merged
}

func sortHistoryPoints(points []store.HistoryPoint) {
	sort.Slice(points, func(i, j int) bool { return points[i].BucketTs < points[j].BucketTs })
}

// TrafficHistory 查询流量历史趋势：ip 为空时为全服汇总，否则为单 IP 曲线
func (s *NetService) TrafficHistory(start, end, bucket int64, ip string) ([]store.HistoryPoint, error) {
	start, end, bucket, ok := clampHistoryBounds(start, end, bucket)
	if !ok {
		return nil, Invalidf("时间区间无效: start=%d end=%d", start, end)
	}

	plan := planRange(start, end, bucket, rawIPCutoff())

	var parts [][]store.HistoryPoint
	if plan.useRollup {
		points, err := s.store.TrafficRollupStore.QueryIpHistory(plan.rollupLayer, plan.rollupFrom, plan.rollupTo, plan.effBucket, ip)
		if err != nil {
			return nil, Internalf("查询流量历史(聚合层)失败: %v", err)
		}
		parts = append(parts, points)
	}
	if plan.useRaw {
		points, err := s.store.TrafficSampleStore.QueryHistory(plan.rawFrom, plan.rawTo, plan.effBucket, ip)
		if err != nil {
			return nil, Internalf("查询流量历史失败: %v", err)
		}
		parts = append(parts, points)
	}
	return mergeHistoryPoints(parts...), nil
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

	plan := planRange(start, end, minHistoryBucket, rawIPCutoff())

	var parts [][]store.TopEntry
	if plan.useRollup {
		entries, err := s.store.TrafficRollupStore.QueryIpTop(plan.rollupLayer, plan.rollupFrom, plan.rollupTo, limit)
		if err != nil {
			return nil, Internalf("查询流量排行(聚合层)失败: %v", err)
		}
		parts = append(parts, entries)
	}
	if plan.useRaw {
		entries, err := s.store.TrafficSampleStore.QueryTop(plan.rawFrom, plan.rawTo, limit)
		if err != nil {
			return nil, Internalf("查询流量排行失败: %v", err)
		}
		parts = append(parts, entries)
	}
	return mergeTopEntries(parts, limit), nil
}

// mergeTopEntries 合并多段 Top 结果：同 IP 求和、取最近活动时间，重排序后截取前 limit
func mergeTopEntries(parts [][]store.TopEntry, limit int) []store.TopEntry {
	if len(parts) == 1 {
		return parts[0]
	}
	index := make(map[string]int)
	merged := make([]store.TopEntry, 0)
	for _, part := range parts {
		for _, e := range part {
			if i, ok := index[e.RemoteIP]; ok {
				merged[i].BytesIn += e.BytesIn
				merged[i].BytesOut += e.BytesOut
				merged[i].BytesSum += e.BytesSum
				if e.LastSeen > merged[i].LastSeen {
					merged[i].LastSeen = e.LastSeen
				}
				continue
			}
			index[e.RemoteIP] = len(merged)
			merged = append(merged, e)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].BytesSum > merged[j].BytesSum })
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

// rawIPCutoff raw 层（IP 维度）的最早保留时间
func rawIPCutoff() int64 {
	return time.Now().Add(-rawIPRetention).Unix()
}

// rawPortCutoff raw 层（端口维度）的最早保留时间
func rawPortCutoff() int64 {
	return time.Now().Add(-rawPortRetention).Unix()
}

// --- 端口/协议维度历史 ---

// PortHistoryParams 端口维度历史查询参数
type PortHistoryParams struct {
	Start, End, Bucket int64
	IP                 string // 过滤单个远程 IP
	Proto              string // 过滤协议（tcp/udp/icmp），空为全部
	Port               int    // 过滤端口，-1 表示不过滤（0 是合法值，代表"其他"）
}

// TrafficPortHistory 查询端口维度的历史趋势（跨 IP 聚合，返回按 (proto, port) 分组的分桶序列）
func (s *NetService) TrafficPortHistory(p PortHistoryParams) ([]store.PortHistoryPoint, error) {
	start, end, bucket, ok := clampHistoryBounds(p.Start, p.End, p.Bucket)
	if !ok {
		return nil, Invalidf("时间区间无效: start=%d end=%d", p.Start, p.End)
	}
	filter := store.PortHistoryFilter{RemoteIP: p.IP, Proto: p.Proto, Port: p.Port}

	plan := planRange(start, end, bucket, rawPortCutoff())

	var parts [][]store.PortHistoryPoint
	if plan.useRollup {
		points, err := s.store.TrafficRollupStore.QueryPortHistory(plan.rollupLayer, plan.rollupFrom, plan.rollupTo, plan.effBucket, filter)
		if err != nil {
			return nil, Internalf("查询端口历史(聚合层)失败: %v", err)
		}
		parts = append(parts, points)
	}
	if plan.useRaw {
		points, err := s.store.TrafficPortStore.QueryHistory(plan.rawFrom, plan.rawTo, plan.effBucket, filter)
		if err != nil {
			return nil, Internalf("查询端口历史失败: %v", err)
		}
		parts = append(parts, points)
	}
	return mergePortHistoryPoints(parts...), nil
}

func mergePortHistoryPoints(parts ...[]store.PortHistoryPoint) []store.PortHistoryPoint {
	if len(parts) == 1 {
		return parts[0]
	}
	type key struct {
		ts    int64
		proto string
		port  int
	}
	index := make(map[key]int)
	merged := make([]store.PortHistoryPoint, 0)
	for _, part := range parts {
		for _, p := range part {
			k := key{p.BucketTs, p.Proto, p.Port}
			if i, ok := index[k]; ok {
				merged[i].BytesIn += p.BytesIn
				merged[i].BytesOut += p.BytesOut
				merged[i].PacketsIn += p.PacketsIn
				merged[i].PacketsOut += p.PacketsOut
				continue
			}
			index[k] = len(merged)
			merged = append(merged, p)
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].BucketTs != merged[j].BucketTs {
			return merged[i].BucketTs < merged[j].BucketTs
		}
		if merged[i].Proto != merged[j].Proto {
			return merged[i].Proto < merged[j].Proto
		}
		return merged[i].Port < merged[j].Port
	})
	return merged
}

// TrafficPortHistoryTop 查询时间范围内流量最大的前 limit 个端口
func (s *NetService) TrafficPortHistoryTop(p PortHistoryParams, limit int) ([]store.PortTopEntry, error) {
	start, end, _, ok := clampHistoryBounds(p.Start, p.End, minHistoryBucket)
	if !ok {
		return nil, Invalidf("时间区间无效: start=%d end=%d", p.Start, p.End)
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > maxHistoryLimit {
		limit = maxHistoryLimit
	}
	filter := store.PortHistoryFilter{RemoteIP: p.IP, Proto: p.Proto, Port: p.Port}

	plan := planRange(start, end, minHistoryBucket, rawPortCutoff())

	var parts [][]store.PortTopEntry
	if plan.useRollup {
		entries, err := s.store.TrafficRollupStore.QueryPortTop(plan.rollupLayer, plan.rollupFrom, plan.rollupTo, limit, filter)
		if err != nil {
			return nil, Internalf("查询端口排行(聚合层)失败: %v", err)
		}
		parts = append(parts, entries)
	}
	if plan.useRaw {
		entries, err := s.store.TrafficPortStore.QueryTopPorts(plan.rawFrom, plan.rawTo, limit, filter)
		if err != nil {
			return nil, Internalf("查询端口排行失败: %v", err)
		}
		parts = append(parts, entries)
	}
	return mergePortTopEntries(parts, limit), nil
}

func mergePortTopEntries(parts [][]store.PortTopEntry, limit int) []store.PortTopEntry {
	if len(parts) == 1 {
		return parts[0]
	}
	type key struct {
		proto string
		port  int
	}
	index := make(map[key]int)
	merged := make([]store.PortTopEntry, 0)
	for _, part := range parts {
		for _, e := range part {
			k := key{e.Proto, e.Port}
			if i, ok := index[k]; ok {
				merged[i].BytesIn += e.BytesIn
				merged[i].BytesOut += e.BytesOut
				merged[i].BytesSum += e.BytesSum
				continue
			}
			index[k] = len(merged)
			merged = append(merged, e)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].BytesSum > merged[j].BytesSum })
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged
}

// TrafficProtoHistory 查询协议维度的历史趋势（返回按协议分组的分桶序列）
func (s *NetService) TrafficProtoHistory(p PortHistoryParams) ([]store.ProtoHistoryPoint, error) {
	start, end, bucket, ok := clampHistoryBounds(p.Start, p.End, p.Bucket)
	if !ok {
		return nil, Invalidf("时间区间无效: start=%d end=%d", p.Start, p.End)
	}

	plan := planRange(start, end, bucket, rawPortCutoff())

	var merged []store.ProtoHistoryPoint
	if plan.useRollup {
		points, err := s.store.TrafficRollupStore.QueryProtoHistory(plan.rollupLayer, plan.rollupFrom, plan.rollupTo, plan.effBucket, p.IP)
		if err != nil {
			return nil, Internalf("查询协议历史(聚合层)失败: %v", err)
		}
		merged = append(merged, points...)
	}
	if plan.useRaw {
		points, err := s.store.TrafficPortStore.QueryProtoHistory(plan.rawFrom, plan.rawTo, plan.effBucket, p.IP)
		if err != nil {
			return nil, Internalf("查询协议历史失败: %v", err)
		}
		merged = append(merged, points...)
	}

	type protoKey struct {
		ts    int64
		proto string
	}
	index := make(map[protoKey]int)
	var result []store.ProtoHistoryPoint
	for _, p := range merged {
		k := protoKey{p.BucketTs, p.Proto}
		if i, ok := index[k]; ok {
			result[i].BytesIn += p.BytesIn
			result[i].BytesOut += p.BytesOut
			result[i].PacketsIn += p.PacketsIn
			result[i].PacketsOut += p.PacketsOut
			continue
		}
		index[k] = len(result)
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BucketTs != result[j].BucketTs {
			return result[i].BucketTs < result[j].BucketTs
		}
		return result[i].Proto < result[j].Proto
	})
	return result, nil
}
