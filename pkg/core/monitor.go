package core

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/graydovee/netbouncer/pkg/config"
)

// 端口/连接维度统计的内存保护上限
const (
	// maxPortsPerProto 每个协议下每 IP 保留的最大端口条目数，超出并入端口 0（"其他"）
	maxPortsPerProto = 96
	// maxUDPFlowsPerIP 每 IP 追踪的最大 UDP 流数，超出时随机淘汰
	maxUDPFlowsPerIP = 256
	// udpFlowTimeout UDP 流的空闲超时，超时后视为流结束（再次出现会计为新流）
	udpFlowTimeout = 5 * time.Minute
	// portOther 端口 0 作为"其他/未分类"的保留端口值
	portOther = 0
)

// TrafficStats 流量统计信息（对外暴露）
type TrafficStats struct {
	RemoteIP        string                                `json:"remote_ip"`
	LocalIP         string                                `json:"local_ip"`
	BytesSent       uint64                                `json:"bytes_sent"`          // 总发送字节数
	BytesRecv       uint64                                `json:"bytes_recv"`          // 总接收字节数
	PacketsSent     uint64                                `json:"packets_sent"`        // 总发送包数
	PacketsRecv     uint64                                `json:"packets_recv"`        // 总接收包数
	BytesSentPerSec float64                               `json:"bytes_sent_per_sec"`  // 每秒发送字节数
	BytesRecvPerSec float64                               `json:"bytes_recv_per_sec"`  // 每秒接收字节数
	LastSeen        time.Time                             `json:"last_seen"`           // 最后活动时间
	FirstSeen       time.Time                             `json:"first_seen"`          // 首次发现时间
	Connections     int                                   `json:"connections"`         // 当前连接数（估算值）
	ConnsIn         uint64                                `json:"conns_in"`            // 累计新建入站连接数（TCP SYN / 新 UDP 流）
	ConnsOut        uint64                                `json:"conns_out"`           // 累计新建出站连接数
	Protocols       map[string]*ProtoPortStats            `json:"protocols,omitempty"` // 协议维度累计统计
	Ports           map[string]map[uint16]*ProtoPortStats `json:"ports,omitempty"`     // 协议→端口维度累计统计
}

// ProtoPortStats 协议/端口维度的流量统计（累计值）
type ProtoPortStats struct {
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	ConnsIn     uint64 `json:"conns_in"`  // 该维度上新建入站连接数
	ConnsOut    uint64 `json:"conns_out"` // 该维度上新建出站连接数
}

// TotalBytes 总字节数
func (s *ProtoPortStats) TotalBytes() uint64 {
	return s.BytesSent + s.BytesRecv
}

// GetTotalBytes 获取总字节数
func (ts *TrafficStats) GetTotalBytes() uint64 {
	return ts.BytesSent + ts.BytesRecv
}

// GetTotalPackets 获取总包数
func (ts *TrafficStats) GetTotalPackets() uint64 {
	return ts.PacketsSent + ts.PacketsRecv
}

// Monitor 网络流量监控器
type Monitor struct {
	stats     map[string]*internalTrafficStats
	mutex     sync.RWMutex
	handle    *pcap.Handle
	localIPs  map[string]bool
	isRunning atomic.Bool
	stopOnce  sync.Once
	stopChan  chan bool
	device    string

	windowSize        time.Duration // 滑动窗口大小（如30秒）
	connectionTimeout time.Duration // 连接超时时间
	excludeSubnets    []*net.IPNet
	captureFilter     string // BPF 捕获过滤器，空则使用默认值
}

// NewMonitor 创建新的监控器
func NewMonitor(cfg *config.MonitorConfig) (*Monitor, error) {
	device := cfg.Interface
	windowSize := time.Duration(cfg.Window) * time.Second
	connectionTimeout := time.Duration(cfg.Timeout) * time.Second

	var excludedSubnets []*net.IPNet
	if cfg.ExcludeSubnets != "" {
		excludedSubnetStrs := strings.SplitSeq(cfg.ExcludeSubnets, ",")
		for subnetStr := range excludedSubnetStrs {
			subnetStr = strings.TrimSpace(subnetStr)
			if subnetStr == "" {
				continue
			}
			_, ipNet, err := net.ParseCIDR(subnetStr)
			if err != nil {
				return nil, fmt.Errorf("解析排除的子网失败 %s: %w", subnetStr, err)
			}
			excludedSubnets = append(excludedSubnets, ipNet)
			slog.Info("排除网段", "subnet", ipNet)
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
		captureFilter:     strings.TrimSpace(cfg.CaptureFilter),
	}

	// 获取本地IP地址
	if err := monitor.getLocalIPs(); err != nil {
		return nil, fmt.Errorf("failed to get local IPs: %v", err)
	}

	return monitor, nil
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
	if !m.isRunning.CompareAndSwap(false, true) {
		return fmt.Errorf("monitor is already running")
	}

	// 打开网络接口进行捕获
	// snaplen 取最大值，确保统计字节数时能读到完整包
	handle, err := pcap.OpenLive(m.device, 65535, true, pcap.BlockForever)
	if err != nil {
		m.isRunning.Store(false)
		return fmt.Errorf("failed to open device %s: %v", m.device, err)
	}

	m.handle = handle

	// 默认捕获 TCP/UDP/ICMP，可通过 monitor.capture_filter 覆盖
	filter := m.captureFilter
	if filter == "" {
		filter = "tcp or udp or icmp or icmp6"
	}
	if err := m.handle.SetBPFFilter(filter); err != nil {
		m.isRunning.Store(false)
		handle.Close()
		return fmt.Errorf("failed to set BPF filter %q: %v", filter, err)
	}

	// 启动包捕获协程
	go m.capturePackets()

	// 启动清理协程
	m.StartCleanupRoutine()

	slog.Info("Network monitor started on device", "device", m.device, "filter", filter)
	return nil
}

// Stop 停止监控，可安全地重复调用
func (m *Monitor) Stop() {
	m.stopOnce.Do(func() {
		m.isRunning.Store(false)
		close(m.stopChan)

		if m.handle != nil {
			m.handle.Close()
		}

		slog.Info("Network monitor stopped")
	})
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
	var ipProto int // IANA 协议号，用于传输层头缺失时（分片后继包）分类协议

	// 处理IPv4
	if ipv4, ok := ipLayer.(*layers.IPv4); ok {
		srcIP = ipv4.SrcIP.String()
		dstIP = ipv4.DstIP.String()
		ipProto = int(ipv4.Protocol)
		// 优先使用 IP 头中的总长度字段：snaplen 截断或 TCP 分段卸载时
		// packet.Data() 会比真实包小，导致流量少算
		length = uint64(ipv4.Length)
		if length == 0 {
			length = uint64(len(packet.Data()))
		}
	} else if ipv6, ok := ipLayer.(*layers.IPv6); ok {
		// 处理IPv6，Length 字段不含 40 字节固定头部
		srcIP = ipv6.SrcIP.String()
		dstIP = ipv6.DstIP.String()
		ipProto = int(ipv6.NextHeader)
		length = uint64(ipv6.Length) + 40
		if length <= 40 {
			length = uint64(len(packet.Data()))
		}
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

	proto, srcPort, dstPort, isNewConn, isConnEnd := classifyPacket(packet, ipProto)

	m.updateStats(remoteIP, localIP, proto, srcPort, dstPort, length, isSent, isNewConn, isConnEnd)
}

// classifyPacket 识别协议、端口与连接状态变化
// 端口取目标端口：收包为本机服务端口，发包为对端服务端口
func classifyPacket(packet gopacket.Packet, ipProto int) (proto string, srcPort, dstPort uint16, isNewConn, isConnEnd bool) {
	if l := packet.Layer(layers.LayerTypeTCP); l != nil {
		if tcp, ok := l.(*layers.TCP); ok {
			// 只统计SYN包（新连接开始）
			isNewConn = tcp.SYN && !tcp.ACK
			// 统计FIN或RST包（连接结束）
			isConnEnd = tcp.FIN || tcp.RST
			return "tcp", uint16(tcp.SrcPort), uint16(tcp.DstPort), isNewConn, isConnEnd
		}
	}
	if l := packet.Layer(layers.LayerTypeUDP); l != nil {
		if udp, ok := l.(*layers.UDP); ok {
			return "udp", uint16(udp.SrcPort), uint16(udp.DstPort), false, false
		}
	}
	if packet.Layer(layers.LayerTypeICMPv4) != nil || packet.Layer(layers.LayerTypeICMPv6) != nil {
		return "icmp", 0, 0, false, false
	}

	// 传输层头缺失（如分片的后继包），按 IP 协议号分类，端口记 0
	switch ipProto {
	case 6:
		return "tcp", 0, 0, false, false
	case 17:
		return "udp", 0, 0, false, false
	case 1, 58:
		return "icmp", 0, 0, false, false
	}
	return "other", 0, 0, false, false
}

// updateStats 更新流量统计（单一锁内完成总量与协议/端口/连接维度）
func (m *Monitor) updateStats(remoteIP string, localIP string, proto string, srcPort, dstPort uint16, bytes uint64, isSent bool, isNewConn, isConnEnd bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	stats, exists := m.stats[remoteIP]
	if !exists {
		stats = newInternalTrafficStats(remoteIP, localIP, now, m.windowSize)
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

	// 更新连接数：TCP 新连接（SYN）与 UDP 新流都视为一次新建连接
	isNewFlow := false
	if proto == "udp" {
		isNewFlow = stats.trackUDPFlow(srcPort, dstPort, now)
	}
	if isNewConn || isNewFlow {
		stats.connections++
		if isSent {
			stats.connsOut++
		} else {
			stats.connsIn++
		}
	}
	if isConnEnd && stats.connections > 0 {
		stats.connections--
	}

	// 协议维度
	ps, ok := stats.protocols[proto]
	if !ok {
		ps = &protoPortStats{}
		stats.protocols[proto] = ps
	}
	ps.add(bytes, isSent, isNewConn || isNewFlow)

	// 端口维度：以目标端口为服务端口，超出上限并入端口 0（"其他"）
	pm, ok := stats.ports[proto]
	if !ok {
		pm = make(map[uint16]*protoPortStats)
		stats.ports[proto] = pm
	}
	entry, ok := pm[dstPort]
	if !ok {
		if len(pm) >= maxPortsPerProto {
			dstPort = portOther
			entry = pm[portOther]
			if entry == nil {
				entry = &protoPortStats{}
				pm[portOther] = entry
			}
		} else {
			entry = &protoPortStats{}
			pm[dstPort] = entry
		}
	}
	entry.add(bytes, isSent, isNewConn || isNewFlow)

	stats.lastSeen = now
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

// newTrafficWindow 创建新的流量窗口
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
	connsIn     uint64         // 累计新建入站连接
	connsOut    uint64         // 累计新建出站连接
	sentWindow  *trafficWindow // 发送流量滑动窗口
	recvWindow  *trafficWindow // 接收流量滑动窗口
	protocols   map[string]*protoPortStats
	ports       map[string]map[uint16]*protoPortStats
	udpFlows    map[udpFlowKey]time.Time
}

// protoPortStats 协议/端口维度的累计计数器
type protoPortStats struct {
	bytesSent   uint64
	bytesRecv   uint64
	packetsSent uint64
	packetsRecv uint64
	connsIn     uint64
	connsOut    uint64
}

func (p *protoPortStats) add(bytes uint64, isSent, isNewConn bool) {
	if isSent {
		p.bytesSent += bytes
		p.packetsSent++
		if isNewConn {
			p.connsOut++
		}
	} else {
		p.bytesRecv += bytes
		p.packetsRecv++
		if isNewConn {
			p.connsIn++
		}
	}
}

// udpFlowKey UDP 流的四元组键（不含 IP，IP 即条目本身）
type udpFlowKey struct {
	localIP    string
	localPort  uint16
	remotePort uint16
}

// newInternalTrafficStats 创建新的内部统计条目
func newInternalTrafficStats(remoteIP, localIP string, now time.Time, windowSize time.Duration) *internalTrafficStats {
	return &internalTrafficStats{
		remoteIP:   remoteIP,
		localIP:    localIP,
		firstSeen:  now,
		lastSeen:   now,
		sentWindow: newTrafficWindow(windowSize),
		recvWindow: newTrafficWindow(windowSize),
		protocols:  make(map[string]*protoPortStats),
		ports:      make(map[string]map[uint16]*protoPortStats),
		udpFlows:   make(map[udpFlowKey]time.Time),
	}
}

// trackUDPFlow 记录 UDP 包，返回是否为新流；流数达到上限时随机淘汰旧流防止内存膨胀
func (its *internalTrafficStats) trackUDPFlow(srcPort, dstPort uint16, now time.Time) bool {
	key := udpFlowKey{localIP: its.localIP, localPort: dstPort, remotePort: srcPort}
	if _, ok := its.udpFlows[key]; ok {
		its.udpFlows[key] = now
		return false
	}
	if len(its.udpFlows) >= maxUDPFlowsPerIP {
		for k := range its.udpFlows {
			delete(its.udpFlows, k)
			break
		}
	}
	its.udpFlows[key] = now
	return true
}

// expireUDPFlows 清理超时的空闲 UDP 流
func (its *internalTrafficStats) expireUDPFlows(now time.Time) {
	cutoff := now.Add(-udpFlowTimeout)
	for k, t := range its.udpFlows {
		if t.Before(cutoff) {
			delete(its.udpFlows, k)
		}
	}
}

// cleanupInactiveConnections 清理长时间未活动的连接
func (m *Monitor) cleanupInactiveConnections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	for ip, stats := range m.stats {
		if now.Sub(stats.lastSeen) > m.connectionTimeout {
			delete(m.stats, ip)
			continue
		}
		stats.expireUDPFlows(now)
	}
}

// GetStats 获取过滤后的IP流量统计（含协议/端口维度累计值）
func (m *Monitor) GetStats() map[string]*TrafficStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*TrafficStats)

	for ip, stats := range m.stats {
		// 检查IP是否在排除的子网中
		if isIPExcluded(ip, m.excludeSubnets) {
			continue
		}
		result[ip] = stats.toTrafficStats()
	}

	return result
}

// toTrafficStats 将内部统计转换为对外暴露的统计（深拷贝协议/端口维度）
func (its *internalTrafficStats) toTrafficStats() *TrafficStats {
	ts := &TrafficStats{
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
		ConnsIn:         its.connsIn,
		ConnsOut:        its.connsOut,
		Protocols:       make(map[string]*ProtoPortStats, len(its.protocols)),
		Ports:           make(map[string]map[uint16]*ProtoPortStats, len(its.ports)),
	}

	for proto, p := range its.protocols {
		ts.Protocols[proto] = p.toProtoPortStats()
	}
	for proto, pm := range its.ports {
		inner := make(map[uint16]*ProtoPortStats, len(pm))
		for port, p := range pm {
			inner[port] = p.toProtoPortStats()
		}
		ts.Ports[proto] = inner
	}

	return ts
}

func (p *protoPortStats) toProtoPortStats() *ProtoPortStats {
	return &ProtoPortStats{
		BytesSent:   p.bytesSent,
		BytesRecv:   p.bytesRecv,
		PacketsSent: p.packetsSent,
		PacketsRecv: p.packetsRecv,
		ConnsIn:     p.connsIn,
		ConnsOut:    p.connsOut,
	}
}
