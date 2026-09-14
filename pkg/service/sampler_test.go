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
	s := NewSampler(mon, st.TrafficSampleStore, st.TrafficPortStore, time.Minute)
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

// 设置某 IP 的端口维度统计
func (m *mutableMonitor) setPorts(ip string, ports map[string]map[uint16]*core.ProtoPortStats) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stats[ip] = &core.TrafficStats{RemoteIP: ip, Ports: ports}
}

func TestSamplerPortIncrement(t *testing.T) {
	s, mon, _ := newSamplerEnv(t)
	portStore := s.ports

	// 第一次采样：基线未知，端口增量记 0
	mon.setPorts("2.2.2.2", map[string]map[uint16]*core.ProtoPortStats{
		"tcp": {
			443: {BytesRecv: 1000, BytesSent: 500, PacketsRecv: 10, PacketsSent: 5},
			80:  {BytesRecv: 100, BytesSent: 50},
		},
	})
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}

	// 第二次采样：各端口增量独立计算
	mon.setPorts("2.2.2.2", map[string]map[uint16]*core.ProtoPortStats{
		"tcp": {
			443: {BytesRecv: 3000, BytesSent: 1500, PacketsRecv: 30, PacketsSent: 15},
			80:  {BytesRecv: 100, BytesSent: 50}, // 无增量
		},
	})
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}

	points, err := portStore.QueryHistory(0, time.Now().Unix()+60, 60, store.PortHistoryFilter{Proto: "tcp", Port: 443})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	var delta int64
	for _, p := range points {
		delta += int64(p.BytesIn)
	}
	if delta != 2000 {
		t.Fatalf("tcp/443 入站增量 = %d, want 2000", delta)
	}

	// 端口 80 无增量，总量应为 0
	points, err = portStore.QueryHistory(0, time.Now().Unix()+60, 60, store.PortHistoryFilter{Proto: "tcp", Port: 80})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	var total uint64
	for _, p := range points {
		total += p.BytesIn + p.BytesOut
	}
	if total != 0 {
		t.Fatalf("tcp/80 应无增量: got %d", total)
	}
}

func TestSamplerPortCounterReset(t *testing.T) {
	s, mon, _ := newSamplerEnv(t)

	mon.setPorts("3.3.3.3", map[string]map[uint16]*core.ProtoPortStats{
		"udp": {53: {BytesRecv: 5000, BytesSent: 1000}},
	})
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}
	// 计数器回绕（IP 条目被淘汰重建）
	mon.setPorts("3.3.3.3", map[string]map[uint16]*core.ProtoPortStats{
		"udp": {53: {BytesRecv: 700, BytesSent: 100}},
	})
	if err := s.sampleOnce(); err != nil {
		t.Fatalf("sample: %v", err)
	}

	points, err := s.ports.QueryHistory(0, time.Now().Unix()+60, 60, store.PortHistoryFilter{Proto: "udp", Port: 53})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	var delta int64
	for _, p := range points {
		delta += int64(p.BytesIn)
	}
	if delta != 700 {
		t.Fatalf("计数器重置后增量 = %d, want 700", delta)
	}
}
