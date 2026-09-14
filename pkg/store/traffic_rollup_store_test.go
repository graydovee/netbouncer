package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
)

func newRollupTestStore(t *testing.T) *Store {
	t.Helper()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	st, err := NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: dbFile, LogLevel: "silent"})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return st
}

func TestRollupFromRawIdempotent(t *testing.T) {
	st := newRollupTestStore(t)

	bucket := int64(RollupBucket10m)
	b := time.Now().Unix() - time.Now().Unix()%bucket - bucket // 一个已完结的桶起点
	from := b
	to := b + bucket

	// 写入该桶内 2 个 IP 的 raw 采样
	samples := []TrafficSample{
		{RemoteIP: "1.1.1.1", Ts: b + 10, BytesIn: 100, BytesOut: 50, PacketsIn: 10, PacketsOut: 5, Connections: 3},
		{RemoteIP: "1.1.1.1", Ts: b + 300, BytesIn: 200, BytesOut: 60, PacketsIn: 20, PacketsOut: 6, Connections: 7},
		{RemoteIP: "2.2.2.2", Ts: b + 30, BytesIn: 1000, BytesOut: 0, PacketsIn: 100, PacketsOut: 0, Connections: 1},
	}
	if err := st.TrafficSampleStore.InsertBatch(samples); err != nil {
		t.Fatalf("insert raw: %v", err)
	}

	// 聚合一次
	if err := st.TrafficRollupStore.RollupIpFromRaw(bucket, from, to); err != nil {
		t.Fatalf("rollup: %v", err)
	}
	// 重复聚合（幂等性：总量不应翻倍）
	if err := st.TrafficRollupStore.RollupIpFromRaw(bucket, from, to); err != nil {
		t.Fatalf("rollup twice: %v", err)
	}

	points, err := st.TrafficRollupStore.QueryIpHistory(bucket, from, to, bucket, "")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("应聚合出 1 个桶, got %d", len(points))
	}
	p := points[0]
	if p.BytesIn != 1300 || p.BytesOut != 110 || p.PacketsIn != 130 || p.PacketsOut != 11 {
		t.Fatalf("聚合值错误: %+v", p)
	}

	// 单 IP 过滤
	points, err = st.TrafficRollupStore.QueryIpHistory(bucket, from, to, bucket, "2.2.2.2")
	if err != nil {
		t.Fatalf("query ip: %v", err)
	}
	if len(points) != 1 || points[0].BytesIn != 1000 {
		t.Fatalf("单 IP 聚合错误: %+v", points)
	}
}

func TestRollupPortAndProto(t *testing.T) {
	st := newRollupTestStore(t)

	bucket := int64(RollupBucket10m)
	b := time.Now().Unix() - time.Now().Unix()%bucket - bucket
	from, to := b, b+bucket

	portSamples := []TrafficPortSample{
		{RemoteIP: "1.1.1.1", Proto: "tcp", Port: 443, Ts: b + 10, BytesIn: 500, BytesOut: 100, PacketsIn: 5, PacketsOut: 1},
		{RemoteIP: "2.2.2.2", Proto: "tcp", Port: 443, Ts: b + 20, BytesIn: 300, BytesOut: 40, PacketsIn: 3, PacketsOut: 1},
		{RemoteIP: "1.1.1.1", Proto: "icmp", Port: 0, Ts: b + 30, BytesIn: 64, BytesOut: 0, PacketsIn: 1, PacketsOut: 0},
	}
	if err := st.TrafficPortStore.InsertBatch(portSamples); err != nil {
		t.Fatalf("insert raw ports: %v", err)
	}

	if err := st.TrafficRollupStore.RollupPortFromRaw(bucket, from, to); err != nil {
		t.Fatalf("rollup ports: %v", err)
	}

	// 端口维度查询（跨 IP 聚合）
	points, err := st.TrafficRollupStore.QueryPortHistory(bucket, from, to, bucket, PortHistoryFilter{Proto: "tcp", Port: 443})
	if err != nil {
		t.Fatalf("query port history: %v", err)
	}
	if len(points) != 1 || points[0].BytesIn != 800 || points[0].BytesOut != 140 {
		t.Fatalf("端口聚合错误: %+v", points)
	}

	// 协议维度上卷
	protos, err := st.TrafficRollupStore.QueryProtoHistory(bucket, from, to, bucket, "")
	if err != nil {
		t.Fatalf("query proto history: %v", err)
	}
	protoTotal := map[string]uint64{}
	for _, p := range protos {
		protoTotal[p.Proto] += p.BytesIn
	}
	if protoTotal["tcp"] != 800 || protoTotal["icmp"] != 64 {
		t.Fatalf("协议上卷错误: %+v", protoTotal)
	}

	// 端口 Top 榜
	tops, err := st.TrafficRollupStore.QueryPortTop(bucket, from, to, 10, PortHistoryFilter{Port: -1})
	if err != nil {
		t.Fatalf("query port top: %v", err)
	}
	if len(tops) != 2 || tops[0].Proto != "tcp" || tops[0].Port != 443 {
		t.Fatalf("端口 Top 错误: %+v", tops)
	}
}

func TestRollup10mTo1h(t *testing.T) {
	st := newRollupTestStore(t)

	b := time.Now().Unix() - time.Now().Unix()%int64(RollupBucket1h) - int64(RollupBucket1h) // 一个已完结小时的起点
	from, to := b, b+int64(RollupBucket1h)

	// 写入该小时内的 raw 采样（每个 10 分钟桶的连接数递增）
	var samples []TrafficSample
	for i := 0; i < 6; i++ {
		samples = append(samples, TrafficSample{
			RemoteIP: "1.1.1.1", Ts: b + int64(i)*int64(RollupBucket10m) + 10,
			BytesIn: 100, BytesOut: 10, PacketsIn: 10, PacketsOut: 1, Connections: i + 1,
		})
	}
	if err := st.TrafficSampleStore.InsertBatch(samples); err != nil {
		t.Fatalf("seed raw: %v", err)
	}
	// raw → 10 分钟层
	if err := st.TrafficRollupStore.RollupIpFromRaw(int64(RollupBucket10m), from, to); err != nil {
		t.Fatalf("rollup 10m: %v", err)
	}

	// 10 分钟层 → 1 小时层
	if err := st.TrafficRollupStore.RollupIpFrom10m(from, to); err != nil {
		t.Fatalf("rollup 1h: %v", err)
	}
	// 幂等重算
	if err := st.TrafficRollupStore.RollupIpFrom10m(from, to); err != nil {
		t.Fatalf("rollup 1h twice: %v", err)
	}

	points, err := st.TrafficRollupStore.QueryIpHistory(RollupBucket1h, from, to, RollupBucket1h, "1.1.1.1")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("应聚合出 1 个小时桶, got %d", len(points))
	}
	if points[0].BytesIn != 600 || points[0].BytesOut != 60 {
		t.Fatalf("1小时聚合值错误: %+v", points[0])
	}

	var rolled []TrafficIpRollup
	if err := st.TrafficRollupStore.QueryIpRows(RollupBucket1h, from, to, &rolled); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(rolled) != 1 || rolled[0].ConnMax != 6 {
		t.Fatalf("连接峰值应为桶内最大值 6, got %+v", rolled)
	}
}

func TestIpNetExpiredFilter(t *testing.T) {
	st := newRollupTestStore(t)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	if _, err := st.IpNetStore.CreateWithOptions("1.1.1.1", 0, ActionBan, IpNetOptions{ExpiresAt: &past}); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if _, err := st.IpNetStore.CreateWithOptions("2.2.2.2", 0, ActionBan, IpNetOptions{ExpiresAt: &future}); err != nil {
		t.Fatalf("create active: %v", err)
	}
	if _, err := st.IpNetStore.CreateWithOptions("3.3.3.3", 0, ActionBan, IpNetOptions{}); err != nil {
		t.Fatalf("create permanent: %v", err)
	}

	active, err := st.IpNetStore.FindAllActive()
	if err != nil {
		t.Fatalf("FindAllActive: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("活跃规则应为 2 条（过期的不含）, got %d", len(active))
	}

	expired, err := st.IpNetStore.FindExpired(now)
	if err != nil {
		t.Fatalf("FindExpired: %v", err)
	}
	if len(expired) != 1 || expired[0].IpNet != "1.1.1.1" {
		t.Fatalf("应找到 1 条过期规则: %+v", expired)
	}
}
