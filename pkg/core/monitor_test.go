package core

import (
	"testing"
	"time"
)

func newTestMonitor() *Monitor {
	return &Monitor{
		stats:             make(map[string]*internalTrafficStats),
		localIPs:          map[string]bool{"10.0.0.1": true},
		stopChan:          make(chan bool),
		windowSize:        30 * time.Second,
		connectionTimeout: 24 * time.Hour,
	}
}

func TestUpdateStatsProtoAndPort(t *testing.T) {
	m := newTestMonitor()

	// 入站 TCP 到本地 443 端口
	m.updateStats("1.1.1.1", "10.0.0.1", "tcp", 51000, 443, 1000, false, true, false)
	m.updateStats("1.1.1.1", "10.0.0.1", "tcp", 51000, 443, 2000, false, false, false)
	// 出站 TCP 到远程 80 端口
	m.updateStats("1.1.1.1", "10.0.0.1", "tcp", 52000, 80, 500, true, true, false)
	// ICMP（无端口，计入端口 0）
	m.updateStats("1.1.1.1", "10.0.0.1", "icmp", 0, 0, 64, false, false, false)

	stats := m.GetStats()["1.1.1.1"]
	if stats == nil {
		t.Fatal("缺少统计条目")
	}

	if stats.BytesRecv != 3064 || stats.BytesSent != 500 {
		t.Fatalf("总量错误: recv=%d sent=%d", stats.BytesRecv, stats.BytesSent)
	}
	if stats.ConnsIn != 1 || stats.ConnsOut != 1 {
		t.Fatalf("连接计数错误: in=%d out=%d", stats.ConnsIn, stats.ConnsOut)
	}

	// 协议维度
	tcp := stats.Protocols["tcp"]
	if tcp == nil || tcp.BytesRecv != 3000 || tcp.BytesSent != 500 {
		t.Fatalf("tcp 协议统计错误: %+v", tcp)
	}
	if tcp.ConnsIn != 1 || tcp.ConnsOut != 1 {
		t.Fatalf("tcp 连接统计错误: %+v", tcp)
	}

	// 端口维度：收包记本机端口，发包记对端端口
	port443 := stats.Ports["tcp"][443]
	if port443 == nil || port443.BytesRecv != 3000 {
		t.Fatalf("tcp/443 统计错误: %+v", port443)
	}
	port80 := stats.Ports["tcp"][80]
	if port80 == nil || port80.BytesSent != 500 || port80.BytesRecv != 0 {
		t.Fatalf("tcp/80 统计错误: %+v", port80)
	}
	if port80.ConnsOut != 1 {
		t.Fatalf("tcp/80 出站连接错误: %+v", port80)
	}
	// ICMP 归入端口 0
	other := stats.Ports["icmp"][0]
	if other == nil || other.BytesRecv != 64 {
		t.Fatalf("icmp 端口 0 统计错误: %+v", other)
	}

	// GetStats 返回深拷贝，修改返回值不影响内部状态
	port443.BytesRecv = 99999
	stats2 := m.GetStats()["1.1.1.1"]
	if stats2.Ports["tcp"][443].BytesRecv != 3000 {
		t.Fatal("GetStats 应返回深拷贝")
	}
}

func TestUpdateStatsUDPFlows(t *testing.T) {
	m := newTestMonitor()

	// 同一条 UDP 流的多个包只计一次新流
	m.updateStats("2.2.2.2", "10.0.0.1", "udp", 53, 53000, 100, false, false, false)
	m.updateStats("2.2.2.2", "10.0.0.1", "udp", 53, 53000, 100, false, false, false)
	m.updateStats("2.2.2.2", "10.0.0.1", "udp", 53, 53000, 100, false, false, false)

	stats := m.GetStats()["2.2.2.2"]
	if stats.ConnsIn != 1 {
		t.Fatalf("UDP 新流应计 1 次连接, got %d", stats.ConnsIn)
	}

	// 不同端口组合视为新流
	m.updateStats("2.2.2.2", "10.0.0.1", "udp", 54, 53000, 100, false, false, false)
	stats = m.GetStats()["2.2.2.2"]
	if stats.ConnsIn != 2 {
		t.Fatalf("新端口组合应计为新流, got %d", stats.ConnsIn)
	}
	if stats.BytesRecv != 400 {
		t.Fatalf("UDP 字节统计错误: %d", stats.BytesRecv)
	}
}

func TestUpdateStatsConnEnd(t *testing.T) {
	m := newTestMonitor()

	m.updateStats("3.3.3.3", "10.0.0.1", "tcp", 51000, 22, 100, false, true, false)
	if got := m.GetStats()["3.3.3.3"].Connections; got != 1 {
		t.Fatalf("新连接后连接数应为 1, got %d", got)
	}
	m.updateStats("3.3.3.3", "10.0.0.1", "tcp", 51000, 22, 100, false, false, true)
	if got := m.GetStats()["3.3.3.3"].Connections; got != 0 {
		t.Fatalf("FIN 后连接数应为 0, got %d", got)
	}
	// 多余的 FIN 不应使连接数为负
	m.updateStats("3.3.3.3", "10.0.0.1", "tcp", 51000, 22, 100, false, false, true)
	if got := m.GetStats()["3.3.3.3"].Connections; got != 0 {
		t.Fatalf("连接数不应为负, got %d", got)
	}
}

func TestUpdateStatsPortCapMerge(t *testing.T) {
	m := newTestMonitor()

	// 超出每协议端口上限后，新端口并入端口 0
	for port := 1; port <= maxPortsPerProto+10; port++ {
		m.updateStats("4.4.4.4", "10.0.0.1", "tcp", 51000, uint16(port), 10, false, false, false)
	}

	stats := m.GetStats()["4.4.4.4"]
	pm := stats.Ports["tcp"]
	// 真实端口填满上限，溢出流量并入端口 0（"其他"），共 maxPortsPerProto+1 个条目
	if len(pm) != maxPortsPerProto+1 {
		t.Fatalf("端口条目数应为 %d, got %d", maxPortsPerProto+1, len(pm))
	}
	other, ok := pm[0]
	if !ok || other.BytesRecv != 100 {
		t.Fatalf("溢出端口应并入端口 0（10 个端口 × 10 字节）: %+v", other)
	}
}
