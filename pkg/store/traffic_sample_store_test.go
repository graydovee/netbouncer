package store

import (
	"path/filepath"
	"testing"

	"github.com/graydovee/netbouncer/pkg/config"
)

func newSampleStore(t *testing.T) *TrafficSampleStore {
	t.Helper()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	st, err := NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: dbFile, LogLevel: "silent"})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return st.TrafficSampleStore
}

func TestInsertAndQueryHistory(t *testing.T) {
	s := newSampleStore(t)

	// 桶宽 60：ts=60,61,119 属于桶 [60,120)；ts=150 属于桶 [120,180)
	samples := []TrafficSample{
		{RemoteIP: "1.1.1.1", Ts: 60, BytesIn: 100, BytesOut: 10, PacketsIn: 2, PacketsOut: 1},
		{RemoteIP: "1.1.1.1", Ts: 61, BytesIn: 200, BytesOut: 20, PacketsIn: 4, PacketsOut: 2},
		{RemoteIP: "1.1.1.1", Ts: 119, BytesIn: 300, BytesOut: 30, PacketsIn: 6, PacketsOut: 3},
		{RemoteIP: "1.1.1.1", Ts: 150, BytesIn: 400, BytesOut: 40, PacketsIn: 8, PacketsOut: 4},
		{RemoteIP: "2.2.2.2", Ts: 60, BytesIn: 1000, BytesOut: 100, PacketsIn: 20, PacketsOut: 10},
	}
	if err := s.InsertBatch(samples); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// 全服汇总
	points, err := s.QueryHistory(0, 1000, 60, "")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("buckets = %d, want 2", len(points))
	}
	// 桶60: in=100+200+300+1000=1600, out=10+20+30+100=160
	if points[0].BucketTs != 60 || points[0].BytesIn != 1600 || points[0].BytesOut != 160 {
		t.Errorf("bucket0 = %+v", points[0])
	}
	// 桶120: in=400
	if points[1].BucketTs != 120 || points[1].BytesIn != 400 {
		t.Errorf("bucket1 = %+v", points[1])
	}

	// 单 IP 过滤
	points, err = s.QueryHistory(0, 1000, 60, "2.2.2.2")
	if err != nil {
		t.Fatalf("query ip: %v", err)
	}
	if len(points) != 1 || points[0].BytesIn != 1000 {
		t.Errorf("single ip points = %+v", points)
	}
}

func TestQueryTop(t *testing.T) {
	s := newSampleStore(t)

	// 3.3.3.3 总量最大，2.2.2.2 次之，1.1.1.1 最小
	samples := []TrafficSample{
		{RemoteIP: "1.1.1.1", Ts: 60, BytesIn: 10, BytesOut: 5},
		{RemoteIP: "2.2.2.2", Ts: 60, BytesIn: 100, BytesOut: 50},
		{RemoteIP: "3.3.3.3", Ts: 60, BytesIn: 1000, BytesOut: 500},
		{RemoteIP: "3.3.3.3", Ts: 120, BytesIn: 2000, BytesOut: 100},
		// 范围之外，不应计入
		{RemoteIP: "9.9.9.9", Ts: 99999, BytesIn: 99999, BytesOut: 99999},
	}
	if err := s.InsertBatch(samples); err != nil {
		t.Fatalf("insert: %v", err)
	}

	entries, err := s.QueryTop(0, 1000, 10)
	if err != nil {
		t.Fatalf("query top: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	if entries[0].RemoteIP != "3.3.3.3" || entries[0].BytesSum != 3600 {
		t.Errorf("top1 = %+v", entries[0])
	}
	if entries[2].RemoteIP != "1.1.1.1" {
		t.Errorf("top3 = %+v", entries[2])
	}

	// limit 截断
	entries, _ = s.QueryTop(0, 1000, 2)
	if len(entries) != 2 {
		t.Errorf("limit entries = %d, want 2", len(entries))
	}
}

func TestCleanup(t *testing.T) {
	s := newSampleStore(t)

	samples := []TrafficSample{
		{RemoteIP: "1.1.1.1", Ts: 100},
		{RemoteIP: "1.1.1.1", Ts: 200},
		{RemoteIP: "2.2.2.2", Ts: 300},
	}
	if err := s.InsertBatch(samples); err != nil {
		t.Fatalf("insert: %v", err)
	}

	deleted, err := s.Cleanup(250)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}

	points, _ := s.QueryHistory(0, 10000, 60, "")
	if len(points) != 1 || points[0].BytesIn != 0 {
		t.Errorf("remaining = %+v", points)
	}
}
