package service

import (
	"github.com/graydovee/netbouncer/pkg/core"
	"sync"
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
