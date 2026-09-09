package service

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

// 采样器测试用的可变 fake 监控器
type mutableMonitor struct {
	mu    sync.Mutex
	stats map[string]*core.TrafficStats
}

func (m *mutableMonitor) GetStats() map[string]*core.TrafficStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]*core.TrafficStats, len(m.stats))
	for k, v := range m.stats {
		cp := *v
		out[k] = &cp
	}
	return out
}

func (m *mutableMonitor) set(ip string, in, out uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats[ip] = &core.TrafficStats{RemoteIP: ip, BytesRecv: in, BytesSent: out}
}

func (m *mutableMonitor) remove(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.stats, ip)
}

func newSamplerEnv(t *testing.T) (*Sampler, *mutableMonitor, *store.TrafficSampleStore) {
	t.Helper()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: dbFile, LogLevel: "silent"})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	mon := &mutableMonitor{stats: map[string]*core.TrafficStats{}}
	s := NewSampler(mon, st.TrafficSampleStore, time.Minute, 24*time.Hour)
	return s, mon, st.TrafficSampleStore
}

func TestSamplerIncrement(t *testing.T) {
	s, mon, samples := newSamplerEnv(t)

	// 第一次采样：基线未知，增量记 0
	mon.set("1.1.1.1", 1000, 500)
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}
	// 第二次采样：增量 = 差值
	mon.set("1.1.1.1", 3000, 800)
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}

	points, err := samples.QueryHistory(0, time.Now().Add(time.Hour).Unix(), 60, "1.1.1.1")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	var totalIn, totalOut uint64
	for _, p := range points {
		totalIn += p.BytesIn
		totalOut += p.BytesOut
	}
	if totalIn != 2000 || totalOut != 300 {
		t.Errorf("total in/out = %d/%d, want 2000/300", totalIn, totalOut)
	}
}

func TestSamplerCounterReset(t *testing.T) {
	s, mon, samples := newSamplerEnv(t)

	mon.set("2.2.2.2", 5000, 0)
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}
	// IP 条目被清理后重建，计数器从 0 重新累计
	mon.set("2.2.2.2", 300, 100)
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}

	points, _ := samples.QueryHistory(0, time.Now().Add(time.Hour).Unix(), 60, "2.2.2.2")
	var totalIn uint64
	for _, p := range points {
		totalIn += p.BytesIn
	}
	// 重置后当前值 300 全部计入本区间
	if totalIn != 300 {
		t.Errorf("total in = %d, want 300", totalIn)
	}
}

func TestSamplerIPDisappeared(t *testing.T) {
	s, mon, samples := newSamplerEnv(t)

	mon.set("3.3.3.3", 100, 100)
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}
	// IP 从活跃列表消失（超时清理），快照中不再保留基线
	mon.remove("3.3.3.3")
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}
	// 重新出现：作为首次采样，增量记 0 而不是把 1000 全算进本区间
	mon.set("3.3.3.3", 1000, 1000)
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}

	points, _ := samples.QueryHistory(0, time.Now().Add(time.Hour).Unix(), 60, "3.3.3.3")
	var totalIn uint64
	for _, p := range points {
		totalIn += p.BytesIn
	}
	if totalIn != 0 {
		t.Errorf("total in = %d, want 0", totalIn)
	}
}
