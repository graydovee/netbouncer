package monitor

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// 默认的滑动窗口大小（秒）
const defaultWindowSize = 10

// TrafficStats 流量统计信息（对外暴露）
type TrafficStats struct {
	RemoteIP        string    `json:"remote_ip"`
	LocalIP         string    `json:"local_ip"`
	BytesSent       uint64    `json:"bytes_sent"`         // 总发送字节数
	BytesRecv       uint64    `json:"bytes_recv"`         // 总接收字节数
	PacketsSent     uint64    `json:"packets_sent"`       // 总发送包数
	PacketsRecv     uint64    `json:"packets_recv"`       // 总接收包数
	BytesSentPerSec float64   `json:"bytes_sent_per_sec"` // 每秒发送字节数
	BytesRecvPerSec float64   `json:"bytes_recv_per_sec"` // 每秒接收字节数
	LastSeen        time.Time `json:"last_seen"`          // 最后活动时间
	FirstSeen       time.Time `json:"first_seen"`         // 首次发现时间
	Connections     int       `json:"connections"`        // 连接数
}

// GetTotalBytes 获取总字节数
func (ts *TrafficStats) GetTotalBytes() uint64 {
	return ts.BytesSent + ts.BytesRecv
}

// GetTotalPackets 获取总包数
func (ts *TrafficStats) GetTotalPackets() uint64 {
	return ts.PacketsSent + ts.PacketsRecv
}

// trafficWindow 流量滑动窗口
type trafficWindow struct {
	windowSize int           // 窗口大小（秒）
	points     []windowPoint // 窗口数据点
}

// windowPoint 窗口数据点
type windowPoint struct {
	timestamp time.Time
	bytes     uint64
}

// newTrafficWindow 创建新的流量滑动窗口
func newTrafficWindow(windowSize int) *trafficWindow {
	if windowSize <= 0 {
		windowSize = defaultWindowSize
	}
	return &trafficWindow{
		windowSize: windowSize,
		points:     make([]windowPoint, 0, windowSize),
	}
}

// addPoint 添加数据点
func (tw *trafficWindow) addPoint(bytes uint64) {
	now := time.Now()

	// 移除过期的数据点
	tw.cleanup(now)

	// 添加新数据点
	tw.points = append(tw.points, windowPoint{
		timestamp: now,
		bytes:     bytes,
	})
}

// cleanup 清理过期的数据点
func (tw *trafficWindow) cleanup(now time.Time) {
	cutoff := now.Add(-time.Duration(tw.windowSize) * time.Second)

	// 找到第一个未过期的数据点
	validStart := 0
	for i, point := range tw.points {
		if point.timestamp.After(cutoff) {
			validStart = i
			break
		}
	}

	// 保留未过期的数据点
	if validStart > 0 {
		tw.points = tw.points[validStart:]
	}
}

// getRate 计算当前速率（字节/秒）
func (tw *trafficWindow) getRate() float64 {
	if len(tw.points) < 2 {
		return 0
	}

	now := time.Now()
	tw.cleanup(now)

	if len(tw.points) < 2 {
		return 0
	}

	// 计算时间窗口内的总字节数
	totalBytes := tw.points[len(tw.points)-1].bytes - tw.points[0].bytes
	timeDiff := tw.points[len(tw.points)-1].timestamp.Sub(tw.points[0].timestamp).Seconds()

	if timeDiff <= 0 {
		return 0
	}

	return float64(totalBytes) / timeDiff
}

// internalTrafficStats 内部使用的流量统计信息
type internalTrafficStats struct {
	remoteIP        string
	localIP         string
	bytesSent       uint64
	bytesRecv       uint64
	packetsSent     uint64
	packetsRecv     uint64
	bytesSentPerSec float64
	bytesRecvPerSec float64
	lastSeen        time.Time
	firstSeen       time.Time
	connections     int
	sentWindow      *trafficWindow // 发送流量滑动窗口
	recvWindow      *trafficWindow // 接收流量滑动窗口
}

// toTrafficStats 将内部统计转换为对外暴露的统计
func (its *internalTrafficStats) toTrafficStats() *TrafficStats {
	return &TrafficStats{
		RemoteIP:        its.remoteIP,
		LocalIP:         its.localIP,
		BytesSent:       its.bytesSent,
		BytesRecv:       its.bytesRecv,
		PacketsSent:     its.packetsSent,
		PacketsRecv:     its.packetsRecv,
		BytesSentPerSec: its.sentWindow.getRate(),
		BytesRecvPerSec: its.recvWindow.getRate(),
		LastSeen:        its.lastSeen,
		FirstSeen:       its.firstSeen,
		Connections:     its.connections,
	}
}

// Monitor 网络流量监控器
type Monitor struct {
	stats      map[string]*internalTrafficStats
	mutex      sync.RWMutex
	handle     *pcap.Handle
	localIPs   map[string]bool
	isRunning  bool
	stopChan   chan bool
	device     string
	windowSize int // 滑动窗口大小（秒）
}

// NewMonitor 创建新的监控器
func NewMonitor(device string) (*Monitor, error) {
	if device == "" {
		// 自动选择默认网络接口
		devices, err := pcap.FindAllDevs()
		if err != nil {
			return nil, fmt.Errorf("failed to find devices: %v", err)
		}

		for _, dev := range devices {
			if len(dev.Addresses) > 0 && dev.Name != "lo" {
				device = dev.Name
				break
			}
		}

		if device == "" {
			return nil, fmt.Errorf("no suitable network device found")
		}
	}

	monitor := &Monitor{
		stats:      make(map[string]*internalTrafficStats),
		localIPs:   make(map[string]bool),
		stopChan:   make(chan bool),
		device:     device,
		windowSize: defaultWindowSize,
	}

	// 获取本地IP地址
	if err := monitor.getLocalIPs(); err != nil {
		return nil, fmt.Errorf("failed to get local IPs: %v", err)
	}

	return monitor, nil
}

// SetWindowSize 设置滑动窗口大小
func (m *Monitor) SetWindowSize(size int) {
	if size <= 0 {
		size = defaultWindowSize
	}
	m.windowSize = size
}

// getLocalIPs 获取本地IP地址列表
func (m *Monitor) getLocalIPs() error {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil || ipnet.IP.To16() != nil {
				m.localIPs[ipnet.IP.String()] = true
			}
		}
	}

	return nil
}

// StartCleanupRoutine 启动定期清理协程
func (m *Monitor) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.cleanupInactiveConnections()
			case <-m.stopChan:
				return
			}
		}
	}()
}

// Start 开始监控
func (m *Monitor) Start() error {
	if m.isRunning {
		return fmt.Errorf("monitor is already running")
	}

	// 打开网络接口进行捕获
	handle, err := pcap.OpenLive(m.device, 1600, true, pcap.BlockForever)
	if err != nil {
		return fmt.Errorf("failed to open device %s: %v", m.device, err)
	}

	m.handle = handle
	m.isRunning = true

	// 设置过滤器，只捕获TCP和UDP包
	err = m.handle.SetBPFFilter("tcp or udp")
	if err != nil {
		return fmt.Errorf("failed to set BPF filter: %v", err)
	}

	// 启动包捕获协程
	go m.capturePackets()

	// 启动清理协程
	m.StartCleanupRoutine()

	log.Printf("Network monitor started on device: %s", m.device)
	return nil
}

// Stop 停止监控
func (m *Monitor) Stop() {
	if !m.isRunning {
		return
	}

	m.isRunning = false
	close(m.stopChan)

	if m.handle != nil {
		m.handle.Close()
	}

	log.Println("Network monitor stopped")
}

// capturePackets 捕获网络包
func (m *Monitor) capturePackets() {
	packetSource := gopacket.NewPacketSource(m.handle, m.handle.LinkType())

	for {
		select {
		case <-m.stopChan:
			return
		case packet := <-packetSource.Packets():
			if packet == nil {
				continue
			}
			m.processPacket(packet)
		}
	}
}

// processPacket 处理单个网络包
func (m *Monitor) processPacket(packet gopacket.Packet) {
	// 解析IP层
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		// 尝试IPv6
		ipLayer = packet.Layer(layers.LayerTypeIPv6)
		if ipLayer == nil {
			return
		}
	}

	var srcIP, dstIP string
	var length uint64

	// 处理IPv4
	if ipv4, ok := ipLayer.(*layers.IPv4); ok {
		srcIP = ipv4.SrcIP.String()
		dstIP = ipv4.DstIP.String()
		length = uint64(ipv4.Length)
	} else if ipv6, ok := ipLayer.(*layers.IPv6); ok {
		// 处理IPv6
		srcIP = ipv6.SrcIP.String()
		dstIP = ipv6.DstIP.String()
		length = uint64(ipv6.Length)
	} else {
		return
	}

	// 确定远程IP和流量方向
	var remoteIP, localIP string
	var isSent bool

	if m.localIPs[srcIP] && !m.localIPs[dstIP] {
		// 本地发送到远程
		remoteIP = dstIP
		localIP = srcIP
		isSent = true
	} else if !m.localIPs[srcIP] && m.localIPs[dstIP] {
		// 远程发送到本地
		remoteIP = srcIP
		localIP = dstIP
		isSent = false
	} else {
		// 跳过本地到本地或远程到远程的包
		return
	}

	// 更新统计信息
	m.updateStats(remoteIP, localIP, length, isSent)
}

// updateStats 更新流量统计
func (m *Monitor) updateStats(remoteIP string, localIP string, bytes uint64, isSent bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	stats, exists := m.stats[remoteIP]
	if !exists {
		stats = &internalTrafficStats{
			remoteIP:   remoteIP,
			localIP:    localIP,
			firstSeen:  now,
			lastSeen:   now,
			sentWindow: newTrafficWindow(m.windowSize),
			recvWindow: newTrafficWindow(m.windowSize),
		}
		m.stats[remoteIP] = stats
	}

	// 更新总流量
	if isSent {
		stats.bytesSent += bytes
		stats.packetsSent++
		stats.sentWindow.addPoint(stats.bytesSent)
	} else {
		stats.bytesRecv += bytes
		stats.packetsRecv++
		stats.recvWindow.addPoint(stats.bytesRecv)
	}

	stats.lastSeen = now
}

// cleanupInactiveConnections 清理长时间未活动的连接
func (m *Monitor) cleanupInactiveConnections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for ip, stats := range m.stats {
		if now.Sub(stats.lastSeen) > 5*time.Minute {
			delete(m.stats, ip)
		}
	}
}

// GetAllRemoteIPStats 获取所有远程IP的流量统计
func (m *Monitor) GetAllRemoteIPStats() map[string]*TrafficStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建副本以避免并发访问问题
	result := make(map[string]*TrafficStats)
	for ip, stats := range m.stats {
		result[ip] = stats.toTrafficStats()
	}

	return result
}

// GetTopRemoteIPs 获取流量最大的前N个远程IP
func (m *Monitor) GetTopRemoteIPs(n int) []*TrafficStats {
	allStats := m.GetAllRemoteIPStats()

	// 转换为切片并排序
	var statsList []*TrafficStats
	for _, stats := range allStats {
		statsList = append(statsList, stats)
	}

	// 按总流量排序
	for i := 0; i < len(statsList)-1; i++ {
		for j := i + 1; j < len(statsList); j++ {
			if statsList[i].GetTotalBytes() < statsList[j].GetTotalBytes() {
				statsList[i], statsList[j] = statsList[j], statsList[i]
			}
		}
	}

	// 返回前N个
	if n > len(statsList) {
		n = len(statsList)
	}

	return statsList[:n]
}

// ClearStats 清空统计信息
func (m *Monitor) ClearStats() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.stats = make(map[string]*internalTrafficStats)
}

// RemoveOldEntries 移除长时间未活动的条目
func (m *Monitor) RemoveOldEntries(maxAge time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for ip, stats := range m.stats {
		if now.Sub(stats.lastSeen) > maxAge {
			delete(m.stats, ip)
		}
	}
}

// FormatBytes 格式化字节数显示
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
