package core

import (
	"fmt"
	"log"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/graydovee/netbouncer/pkg/config"
)

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
	windowSize time.Duration // 窗口大小（如30秒）
	points     []windowPoint // 窗口数据点
	mutex      sync.Mutex    // 新增互斥锁，保证线程安全
}

// windowPoint 窗口数据点
type windowPoint struct {
	timestamp time.Time
	increment uint64 // 这次增加的字节数
}

// newTrafficWindow 创建新的流量滑动窗口
func newTrafficWindow(windowSize time.Duration) *trafficWindow {
	if windowSize <= 0 {
		windowSize = 30 * time.Second
	}
	return &trafficWindow{
		windowSize: windowSize,
		points:     make([]windowPoint, 0, 30),
	}
}

// addPoint 添加数据点
func (tw *trafficWindow) addPoint(increment uint64) {
	tw.mutex.Lock()
	defer tw.mutex.Unlock()
	now := time.Now()

	// 移除过期的数据点
	tw.cleanup(now)
	// 添加新数据点
	tw.points = append(tw.points, windowPoint{
		timestamp: now,
		increment: increment,
	})
}

// cleanup 清理过期的数据点
func (tw *trafficWindow) cleanup(now time.Time) {
	cutoff := now.Add(-tw.windowSize)
	validStart := len(tw.points)
	for i, point := range tw.points {
		if point.timestamp.After(cutoff) {
			validStart = i
			break
		}
	}
	tw.points = tw.points[validStart:]
}

// getRate 计算当前速率（字节/秒）
func (tw *trafficWindow) getRate() float64 {
	tw.mutex.Lock()
	defer tw.mutex.Unlock()

	now := time.Now()
	tw.cleanup(now)

	if len(tw.points) == 0 {
		return 0
	}

	// 计算窗口内的总增量字节数
	var totalIncrement uint64
	for _, point := range tw.points {
		totalIncrement += point.increment
	}

	// 计算时间窗口大小
	windowDuration := now.Sub(tw.points[0].timestamp).Seconds()
	if windowDuration <= 0 {
		return 0
	}

	return float64(totalIncrement) / windowDuration
}

// internalTrafficStats 内部使用的流量统计信息
type internalTrafficStats struct {
	remoteIP    string
	localIP     string
	bytesSent   uint64
	bytesRecv   uint64
	packetsSent uint64
	packetsRecv uint64
	lastSeen    time.Time
	firstSeen   time.Time
	connections int
	sentWindow  *trafficWindow // 发送流量滑动窗口
	recvWindow  *trafficWindow // 接收流量滑动窗口
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
	stats     map[string]*internalTrafficStats
	mutex     sync.RWMutex
	handle    *pcap.Handle
	localIPs  map[string]bool
	isRunning bool
	stopChan  chan bool
	device    string

	windowSize        time.Duration // 滑动窗口大小（如30秒）
	connectionTimeout time.Duration // 连接超时时间
	excludeSubnets    []*net.IPNet
}

// NewMonitor 创建新的监控器
func NewMonitor(cfg *config.MonitorConfig) (*Monitor, error) {
	device := cfg.Interface
	windowSize := time.Duration(cfg.Window) * time.Second
	connectionTimeout := time.Duration(cfg.Timeout) * time.Second

	var excludedSubnets []*net.IPNet
	if cfg.ExcludeSubnets != "" {
		excludedSubnetStrs := strings.Split(cfg.ExcludeSubnets, ",")
		for _, subnetStr := range excludedSubnetStrs {
			subnetStr = strings.TrimSpace(subnetStr)
			if subnetStr == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(subnetStr)
			if err != nil {
				return nil, fmt.Errorf("解析排除的子网失败 %s: %w", subnetStr, err)
			}
			excludedSubnets = append(excludedSubnets, ipNet)
			slog.Info("排除子网", "subnet", ipNet)
		}
	}

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

	if windowSize <= 0 {
		windowSize = 30 * time.Second // 默认30秒
	}
	if connectionTimeout <= 0 {
		connectionTimeout = 24 * time.Hour // 默认24小时
	}

	monitor := &Monitor{
		stats:             make(map[string]*internalTrafficStats),
		localIPs:          make(map[string]bool),
		stopChan:          make(chan bool),
		device:            device,
		windowSize:        windowSize,
		connectionTimeout: connectionTimeout,
		excludeSubnets:    excludedSubnets,
	}

	// 获取本地IP地址
	if err := monitor.getLocalIPs(); err != nil {
		return nil, fmt.Errorf("failed to get local IPs: %v", err)
	}

	return monitor, nil
}

// SetWindowSize 设置滑动窗口大小
func (m *Monitor) SetWindowSize(size time.Duration) {
	if size <= 0 {
		size = 30 * time.Second // 默认30秒
	}
	m.windowSize = size
}

// SetConnectionTimeout 设置连接超时时间
func (m *Monitor) SetConnectionTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = 24 * time.Hour
	}
	m.connectionTimeout = timeout
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
	var isTCP bool
	var tcpLayer *layers.TCP

	// 处理IPv4
	if ipv4, ok := ipLayer.(*layers.IPv4); ok {
		srcIP = ipv4.SrcIP.String()
		dstIP = ipv4.DstIP.String()
		// 使用整个数据包的长度，而不是IP层的长度
		length = uint64(len(packet.Data()))
	} else if ipv6, ok := ipLayer.(*layers.IPv6); ok {
		// 处理IPv6
		srcIP = ipv6.SrcIP.String()
		dstIP = ipv6.DstIP.String()
		// 使用整个数据包的长度
		length = uint64(len(packet.Data()))
	} else {
		return
	}

	// 验证包长度合理性
	if length == 0 || length > 65535 {
		// 跳过无效长度的包
		return
	}

	// 检查是否为TCP包
	tcp := packet.Layer(layers.LayerTypeTCP)
	if tcp != nil {
		isTCP = true
		tcpLayer, _ = tcp.(*layers.TCP)
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

	// 统计TCP连接数
	if isTCP && tcpLayer != nil {
		m.mutex.Lock()
		stats, exists := m.stats[remoteIP]
		if !exists {
			stats = &internalTrafficStats{
				remoteIP:   remoteIP,
				localIP:    localIP,
				firstSeen:  time.Now(),
				lastSeen:   time.Now(),
				sentWindow: newTrafficWindow(m.windowSize),
				recvWindow: newTrafficWindow(m.windowSize),
			}
			m.stats[remoteIP] = stats
		}

		// 改进的TCP连接数统计逻辑
		// 只统计SYN包（新连接开始）
		if tcpLayer.SYN && !tcpLayer.ACK {
			stats.connections++
		}
		// 统计FIN或RST包（连接结束）
		if (tcpLayer.FIN || tcpLayer.RST) && stats.connections > 0 {
			stats.connections--
		}
		m.mutex.Unlock()
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
		stats.sentWindow.addPoint(bytes)
	} else {
		stats.bytesRecv += bytes
		stats.packetsRecv++
		stats.recvWindow.addPoint(bytes)
	}

	stats.lastSeen = now
}

// cleanupInactiveConnections 清理长时间未活动的连接
func (m *Monitor) cleanupInactiveConnections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for ip, stats := range m.stats {
		if now.Sub(stats.lastSeen) > m.connectionTimeout {
			delete(m.stats, ip)
		}
	}
}

// GetAllStats 获取所有IP的流量统计
func (m *Monitor) GetAllStats() map[string]*TrafficStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 创建副本以避免并发访问问题
	result := make(map[string]*TrafficStats)
	for ip, stats := range m.stats {
		result[ip] = stats.toTrafficStats()
	}

	return result
}

// GetStats 获取过滤后的IP流量统计
func (m *Monitor) GetStats(excludedSubnets []*net.IPNet) map[string]*TrafficStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*TrafficStats)

	for ip, stats := range m.stats {
		// 检查IP是否在排除的子网中
		if isIPExcluded(ip, excludedSubnets) {
			continue
		}
		result[ip] = stats.toTrafficStats()
	}

	return result
}

// isIPExcluded 检查IP是否在排除的子网中
func isIPExcluded(ipStr string, excludedSubnets []*net.IPNet) bool {
	if len(excludedSubnets) == 0 {
		return false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	for _, subnet := range excludedSubnets {
		if subnet.Contains(ip) {
			return true
		}
	}
	return false
}

// ClearStats 清空统计信息
func (m *Monitor) ClearStats() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.stats = make(map[string]*internalTrafficStats)
}

// GetDebugInfo 获取调试信息
func (m *Monitor) GetDebugInfo() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	debugInfo := make(map[string]interface{})
	debugInfo["total_connections"] = len(m.stats)
	debugInfo["local_ips"] = m.localIPs
	debugInfo["device"] = m.device
	debugInfo["is_running"] = m.isRunning
	debugInfo["window_size"] = m.windowSize.String()
	debugInfo["connection_timeout"] = m.connectionTimeout.String()

	// 统计总流量
	var totalBytesSent, totalBytesRecv uint64
	var totalPacketsSent, totalPacketsRecv uint64
	for _, stats := range m.stats {
		totalBytesSent += stats.bytesSent
		totalBytesRecv += stats.bytesRecv
		totalPacketsSent += stats.packetsSent
		totalPacketsRecv += stats.packetsRecv
	}

	debugInfo["total_bytes_sent"] = totalBytesSent
	debugInfo["total_bytes_recv"] = totalBytesRecv
	debugInfo["total_packets_sent"] = totalPacketsSent
	debugInfo["total_packets_recv"] = totalPacketsRecv
	debugInfo["total_bytes"] = totalBytesSent + totalBytesRecv

	return debugInfo
}
