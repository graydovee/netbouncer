package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

// 策略引擎边界常量
const (
	minEvalInterval   = 5  // 评估间隔下限（秒）
	minPolicyWindow   = 10 // 策略统计窗口下限（秒）
	maxPolicyWindow   = 86400
	minBanSec         = 10
	maxBanSec         = 30 * 24 * 3600
	topClientsPerPort = 5   // 实时端口排行展示的客户端数
	maxPortSummaries  = 200 // 内存中保留的实时端口聚合条数上限
)

// PolicyEngine 策略引擎：
//   - 评估循环（默认 10s）：按策略匹配条件聚合每 IP 的流量/连接/端口指标，超阈值执行
//     mark/ban 动作并记录风险事件；风险分达到阈值的 IP 自动升级临时封禁
//   - 限速动作由内核 hashlimit 常驻规则实现，引擎只负责装卸与实时端口聚合的刷新
//   - 过期解封循环：每 30s 解除到期的临时封禁
type PolicyEngine struct {
	monitor      Monitor
	firewall     Firewall
	net          *NetService
	store        *store.Store
	evalInterval time.Duration
	riskWindow   time.Duration

	mu          sync.Mutex
	states      map[uint]map[string]*ipPolicyState // policyID -> ip -> 窗口状态
	riskScores  map[string]int                     // ip -> 窗口内风险分
	lastRebuild time.Time
	portStats   []PortTraffic            // 实时端口聚合缓存（供 API 读取）
	portPrev    map[string]portPrevEntry // proto:port -> 上次快照
	portPrevTs  time.Time
}

// ipPolicyState 单策略×单 IP 的评估状态
type ipPolicyState struct {
	prev        matchCounters // 上次快照（按策略匹配维度聚合）
	buckets     []windowBucket
	lastTrigger time.Time
}

// matchCounters 按策略匹配维度聚合后的累计计数器
type matchCounters struct {
	bytesIn    uint64
	bytesOut   uint64
	packetsIn  uint64
	packetsOut uint64
	connsIn    uint64
	connsOut   uint64
	portCum    map[uint32]portCum // 各端口键（proto:port 编码）的累计计数，仅扫描检测策略收集
}

// portCum 单端口的累计收发字节数
type portCum struct {
	in  uint64
	out uint64
}

// activePorts 计算 tick 间有新增流量的端口集合（排除端口 0 的"其他"）
func (c *matchCounters) activePorts(prev matchCounters) map[uint32]struct{} {
	if c.portCum == nil {
		return nil
	}
	var active map[uint32]struct{}
	for k, cur := range c.portCum {
		if k&0xFFFF == 0 {
			continue // 端口 0 为"其他/未分类"，不计入扫描检测
		}
		p := prev.portCum[k]
		if cur.in > p.in || cur.out > p.out {
			if active == nil {
				active = make(map[uint32]struct{})
			}
			active[k] = struct{}{}
		}
	}
	return active
}

func (c *matchCounters) clone() matchCounters {
	n := *c
	if c.portCum != nil {
		n.portCum = make(map[uint32]portCum, len(c.portCum))
		for k, v := range c.portCum {
			n.portCum[k] = v
		}
	}
	return n
}

// windowBucket 一个评估间隔内的增量
type windowBucket struct {
	startTs int64
	matchCounters
	ports map[uint32]struct{} // 本 tick 内有新增流量的端口键（扫描检测用）
}

// portPrevEntry 实时端口聚合的上次快照
type portPrevEntry struct {
	counters matchCounters
	ips      int
}

func NewPolicyEngine(monitor Monitor, firewall Firewall, net *NetService, st *store.Store, cfg *config.PolicyConfig) *PolicyEngine {
	interval := time.Duration(cfg.EvalInterval) * time.Second
	if interval < time.Duration(minEvalInterval)*time.Second {
		interval = time.Duration(minEvalInterval) * time.Second
	}
	riskWindow := time.Duration(cfg.RiskWindow) * time.Second
	if riskWindow <= 0 {
		riskWindow = 24 * time.Hour
	}
	return &PolicyEngine{
		monitor:      monitor,
		firewall:     firewall,
		net:          net,
		store:        st,
		evalInterval: interval,
		riskWindow:   riskWindow,
		states:       make(map[uint]map[string]*ipPolicyState),
		riskScores:   make(map[string]int),
		portPrev:     make(map[string]portPrevEntry),
	}
}

// RiskScore 实现 RiskProvider：返回 IP 当前窗口内的风险分
func (e *PolicyEngine) RiskScore(ip string) int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.riskScores[ip]
}

// PortTrafficSnapshot 返回实时端口聚合（评估循环刷新；引擎禁用时返回空）
func (e *PolicyEngine) PortTrafficSnapshot() []PortTraffic {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]PortTraffic, len(e.portStats))
	copy(out, e.portStats)
	return out
}

// Start 启动评估循环与过期解封循环
func (e *PolicyEngine) Start(ctx context.Context) {
	if e == nil || e.evalInterval <= 0 {
		return
	}

	// 装载启用中的限速规则（防火墙链已重建为空）
	if err := e.reloadRateLimits(); err != nil {
		slog.Error("装载限速规则失败", "error", err)
	}
	// 从数据库重建风险分缓存
	e.rebuildRiskScores(true)

	go e.evalLoop(ctx)
	go e.expiryLoop(ctx)
	slog.Info("策略引擎已启动", "eval_interval", e.evalInterval.String(), "risk_window", e.riskWindow.String())
}

// reloadRateLimits 卸载全部已装规则后重装启用中的限速规则（幂等）
func (e *PolicyEngine) reloadRateLimits() error {
	policies, err := e.store.PolicyStore.FindEnabled()
	if err != nil {
		return err
	}
	for _, p := range policies {
		if p.Action != store.PolicyActionRateLimit {
			continue
		}
		rule := rateLimitRuleFromPolicy(&p)
		if err := e.firewall.ApplyRateLimit(rule); err != nil {
			slog.Error("应用限速规则失败", "policy", p.Name, "error", err)
		} else {
			slog.Info("限速规则已应用", "policy", p.Name, "rate_kbps", p.LimitKBps)
		}
	}
	return nil
}

// rebuildRiskScores 从风险事件表重建风险分缓存
func (e *PolicyEngine) rebuildRiskScores(force bool) {
	e.mu.Lock()
	if !force && time.Since(e.lastRebuild) < 10*time.Minute {
		e.mu.Unlock()
		return
	}
	e.lastRebuild = time.Now()
	e.mu.Unlock()

	since := time.Now().Add(-e.riskWindow).Unix()
	scores, err := e.store.RiskEventStore.SumScoresSince(since, "")
	if err != nil {
		slog.Error("重建风险分缓存失败", "error", err)
		return
	}

	e.mu.Lock()
	e.riskScores = scores
	e.mu.Unlock()
}

func (e *PolicyEngine) evalLoop(ctx context.Context) {
	ticker := time.NewTicker(e.evalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.evaluate(); err != nil {
				slog.Error("策略评估失败", "error", err)
			}
		}
	}
}

func (e *PolicyEngine) expiryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			removed, err := e.net.CleanupExpiredTempBans()
			if err != nil {
				slog.Error("清理过期临时封禁失败", "error", err)
			} else if removed > 0 {
				slog.Info("本次共解除过期临时封禁", "count", removed)
			}
		}
	}
}

// evaluate 单轮评估：快照 → 逐策略逐 IP 聚合 → 触发判断 → 动作执行 → 风险升级
func (e *PolicyEngine) evaluate() error {
	// 窗口外的事件过期后风险分需要回落（内部 10 分钟节流）
	e.rebuildRiskScores(false)

	policies, err := e.store.PolicyStore.FindEnabled()
	if err != nil {
		return fmt.Errorf("查询启用策略失败: %w", err)
	}

	// mark/ban 动作需要评估；rate_limit 由内核常驻规则实现，无需评估
	var evalPolicies []store.Policy
	for _, p := range policies {
		if p.Action == store.PolicyActionBan || p.Action == store.PolicyActionMark {
			evalPolicies = append(evalPolicies, p)
		}
	}

	stats := e.monitor.GetStats()

	// 白名单 IP 跳过所有动作；已封禁 IP 不重复触发封禁
	allowEntities, err := e.store.IpNetStore.FindByAction(store.ActionAllow)
	if err != nil {
		return fmt.Errorf("查询白名单失败: %w", err)
	}
	allowNets := convertToIpNet(allowEntities...)
	bannedEntities, err := e.store.IpNetStore.FindByAction(store.ActionBan)
	if err != nil {
		return fmt.Errorf("查询封禁列表失败: %w", err)
	}
	bannedNets := convertToIpNet(bannedEntities...)

	now := time.Now()
	nextStates := make(map[uint]map[string]*ipPolicyState, len(evalPolicies))
	var events []store.RiskEvent
	var escalations []*store.Policy // 有触发事件发生时需要检查风险升级的策略

	for i := range evalPolicies {
		policy := &evalPolicies[i]
		window := time.Duration(policy.WindowSec) * time.Second
		// 长窗口用更宽的桶（每策略最多约 241 个桶），限制每 IP 状态的内存占用
		bucketSecs := int64(e.evalInterval.Seconds())
		if w := int64(policy.WindowSec) / 240; w > bucketSecs {
			bucketSecs = w
		}

		policyStates := make(map[string]*ipPolicyState, len(stats))
		for ip, st := range stats {
			if isContainIpNet(allowNets, ip) {
				continue
			}

			// 按策略维度聚合当前快照
			cur := matchStats(st, policy)
			state, ok := policyStates[ip]
			if !ok {
				if old := e.takeState(policy.ID, ip); old != nil {
					state = old
				} else {
					state = &ipPolicyState{}
				}
				policyStates[ip] = state
			}

			// 本 tick 内有新增流量的端口集合（扫描检测，需在覆盖快照前计算）
			var activePorts map[uint32]struct{}
			if policy.Port == 0 && policy.DistinctPorts > 0 {
				activePorts = cur.activePorts(state.prev)
			}

			// 计算本 tick 增量，写入当前时间桶（同桶累加，跨桶新建）
			bucketStart := now.Unix() - now.Unix()%bucketSecs
			delta := matchCounters{
				bytesIn:    deltaSub(cur.bytesIn, state.prev.bytesIn),
				bytesOut:   deltaSub(cur.bytesOut, state.prev.bytesOut),
				packetsIn:  deltaSub(cur.packetsIn, state.prev.packetsIn),
				packetsOut: deltaSub(cur.packetsOut, state.prev.packetsOut),
				connsIn:    deltaSub(cur.connsIn, state.prev.connsIn),
				connsOut:   deltaSub(cur.connsOut, state.prev.connsOut),
			}
			state.prev = cur.clone()

			if n := len(state.buckets); n > 0 && state.buckets[n-1].startTs == bucketStart {
				b := &state.buckets[n-1]
				b.bytesIn += delta.bytesIn
				b.bytesOut += delta.bytesOut
				b.packetsIn += delta.packetsIn
				b.packetsOut += delta.packetsOut
				b.connsIn += delta.connsIn
				b.connsOut += delta.connsOut
				for k := range activePorts {
					if b.ports == nil {
						b.ports = make(map[uint32]struct{})
					}
					b.ports[k] = struct{}{}
				}
			} else {
				bucket := windowBucket{startTs: bucketStart, ports: activePorts}
				bucket.matchCounters = delta
				state.buckets = append(state.buckets, bucket)
			}

			// 仅保留窗口内的桶（桶 T 覆盖 (T-bucketSecs, T]，窗口左边界所在的旧桶应剔除）
			cutoff := now.Unix() - int64(window.Seconds())
			trim := 0
			for trim < len(state.buckets) && state.buckets[trim].startTs <= cutoff {
				trim++
			}
			state.buckets = state.buckets[trim:]

			// 窗口内聚合
			agg := aggregateWindow(state.buckets)

			// 冷却期内不重复触发
			if policy.CooldownSec > 0 && now.Sub(state.lastTrigger) < time.Duration(policy.CooldownSec)*time.Second {
				continue
			}

			windowSeconds := float64(len(state.buckets)) * float64(bucketSecs)
			trigger, value := checkTrigger(policy, agg, windowSeconds)
			if !trigger {
				continue
			}

			state.lastTrigger = now
			action := policy.Action
			if action == store.PolicyActionBan {
				if IsBanned(bannedNets, allowNets, ip) {
					// 已封禁则只记事件不再重复下发
					action = store.PolicyActionMark
				} else if err := e.net.BanTemporary(ip, policy.Direction, now.Add(time.Duration(policy.BanSec)*time.Second), policySource(policy.ID)); err != nil {
					slog.Error("策略临时封禁失败", "policy", policy.Name, "ip", ip, "error", err)
					continue
				} else {
					// 封禁列表变更，后续 IP 判断使用最新状态
					bannedNets = appendIfMissing(bannedNets, ip)
				}
			}

			events = append(events, store.RiskEvent{
				RemoteIP:     ip,
				Ts:           now.Unix(),
				PolicyID:     policy.ID,
				PolicyName:   policy.Name,
				TriggerValue: value,
				Action:       action,
				Score:        policy.RiskScore,
			})
			slog.Warn("策略触发", "policy", policy.Name, "ip", ip, "action", action, "value", value)

			if policy.RiskBanThreshold > 0 {
				escalations = append(escalations, policy)
			}
		}
		nextStates[policy.ID] = policyStates
	}

	// 替换状态缓存（消失的 IP/策略状态自动清理）
	e.mu.Lock()
	e.states = nextStates
	for _, ev := range events {
		e.riskScores[ev.RemoteIP] += ev.Score
	}
	e.mu.Unlock()

	// 风险事件落库
	if len(events) > 0 {
		if err := e.store.RiskEventStore.InsertBatch(events); err != nil {
			slog.Error("写入风险事件失败", "error", err)
		}
	}

	// 风险升级检查
	if len(escalations) > 0 {
		e.checkEscalation(escalations, bannedNets, allowNets, now)
	}

	// 刷新实时端口聚合
	e.refreshPortStats(stats, now)
	return nil
}

// takeState 从缓存中取出并移除指定策略的 IP 状态
func (e *PolicyEngine) takeState(policyID uint, ip string) *ipPolicyState {
	e.mu.Lock()
	defer e.mu.Unlock()
	ips, ok := e.states[policyID]
	if !ok {
		return nil
	}
	state := ips[ip]
	delete(ips, ip)
	return state
}

// matchStats 按策略的协议/端口匹配聚合统计
func matchStats(st *core.TrafficStats, policy *store.Policy) matchCounters {
	var out matchCounters

	// 需要端口级扫描检测时收集各端口的累计计数（活跃性由相邻快照差值判定）
	collectPorts := policy.Port == 0 && policy.DistinctPorts > 0
	if collectPorts {
		out.portCum = make(map[uint32]portCum)
	}

	add := func(p *core.ProtoPortStats, proto string, port uint16) {
		out.bytesIn += p.BytesRecv
		out.bytesOut += p.BytesSent
		out.packetsIn += p.PacketsRecv
		out.packetsOut += p.PacketsSent
		out.connsIn += p.ConnsIn
		out.connsOut += p.ConnsOut
		if collectPorts {
			key := portKey(proto, port)
			c := out.portCum[key]
			c.in += p.BytesRecv
			c.out += p.BytesSent
			out.portCum[key] = c
		}
	}

	if policy.Port > 0 {
		// 指定端口：只聚合该端口的数据
		for proto, pm := range st.Ports {
			if !protocolMatch(policy.Protocol, proto) {
				continue
			}
			if p, ok := pm[uint16(policy.Port)]; ok {
				add(p, proto, uint16(policy.Port))
			}
		}
		return out
	}

	if policy.Protocol != store.ProtocolAny {
		// 指定协议 + 任意端口：聚合该协议的全部端口数据；
		// 端口明细缺失时回退到协议汇总
		for port, p := range st.Ports[policy.Protocol] {
			add(p, policy.Protocol, port)
		}
		if out.bytesIn == 0 && out.bytesOut == 0 {
			if p := st.Protocols[policy.Protocol]; p != nil {
				add(p, policy.Protocol, 0)
			}
		}
		return out
	}

	// 任意协议 + 任意端口：总量与全部端口之和恒等，直接取总量
	out.bytesIn = st.BytesRecv
	out.bytesOut = st.BytesSent
	out.packetsIn = st.PacketsRecv
	out.packetsOut = st.PacketsSent
	out.connsIn = st.ConnsIn
	out.connsOut = st.ConnsOut
	if collectPorts {
		for proto, pm := range st.Ports {
			for port, p := range pm {
				key := portKey(proto, port)
				c := out.portCum[key]
				c.in += p.BytesRecv
				c.out += p.BytesSent
				out.portCum[key] = c
			}
		}
	}
	return out
}

// portKey 编码协议与端口为端口集键
func portKey(proto string, port uint16) uint32 {
	code := uint32(0) // other
	switch proto {
	case "tcp":
		code = 1
	case "udp":
		code = 2
	case "icmp":
		code = 3
	}
	return code<<16 | uint32(port)
}

func protocolMatch(policyProto, proto string) bool {
	return policyProto == store.ProtocolAny || policyProto == proto
}

// windowAgg 窗口内聚合结果
type windowAgg struct {
	bytesIn    uint64
	bytesOut   uint64
	packetsIn  uint64
	packetsOut uint64
	connsIn    uint64
	connsOut   uint64
	ports      map[uint32]struct{}
}

func aggregateWindow(buckets []windowBucket) windowAgg {
	var agg windowAgg
	for _, b := range buckets {
		agg.bytesIn += b.bytesIn
		agg.bytesOut += b.bytesOut
		agg.packetsIn += b.packetsIn
		agg.packetsOut += b.packetsOut
		agg.connsIn += b.connsIn
		agg.connsOut += b.connsOut
		if b.ports != nil {
			if agg.ports == nil {
				agg.ports = make(map[uint32]struct{}, len(b.ports))
			}
			for k := range b.ports {
				agg.ports[k] = struct{}{}
			}
		}
	}
	return agg
}

// checkTrigger 检查窗口聚合值是否命中任一触发条件，返回 (是否触发, 观测值描述)
func checkTrigger(policy *store.Policy, agg windowAgg, windowSeconds float64) (bool, string) {
	if windowSeconds <= 0 {
		return false, ""
	}

	// 按策略方向取相关侧的指标
	var bytes uint64
	var conns uint64
	switch policy.Direction {
	case store.DirectionIn:
		bytes = agg.bytesIn
		conns = agg.connsIn
	case store.DirectionOut:
		bytes = agg.bytesOut
		conns = agg.connsOut
	default:
		bytes = agg.bytesIn + agg.bytesOut
		conns = agg.connsIn + agg.connsOut
	}

	if policy.RateKBps > 0 {
		kbps := float64(bytes) / windowSeconds / 1024
		if kbps >= policy.RateKBps {
			return true, fmt.Sprintf("%.1fKB/s ≥ %.1fKB/s", kbps, policy.RateKBps)
		}
	}
	if policy.TotalMB > 0 {
		totalMB := float64(bytes) / 1024 / 1024
		if totalMB >= policy.TotalMB {
			return true, fmt.Sprintf("%.1fMB ≥ %.1fMB/%ds", totalMB, policy.TotalMB, policy.WindowSec)
		}
	}
	if policy.ConnRate > 0 && conns >= uint64(policy.ConnRate) {
		return true, fmt.Sprintf("%d次连接 ≥ %d次/%ds", conns, policy.ConnRate, policy.WindowSec)
	}
	if policy.DistinctPorts > 0 && len(agg.ports) >= policy.DistinctPorts {
		return true, fmt.Sprintf("触碰%d个端口 ≥ %d个/%ds", len(agg.ports), policy.DistinctPorts, policy.WindowSec)
	}
	return false, ""
}

// checkEscalation 风险升级：风险分达到任一策略阈值时自动临时封禁并清零重计
func (e *PolicyEngine) checkEscalation(policies []*store.Policy, bannedNets, allowNets []*net.IPNet, now time.Time) {
	type escalation struct {
		policy *store.Policy
		ip     string
		score  int
	}

	// 锁内只做快照，封禁的 DB/iptables 操作放锁外执行
	var pending []escalation
	e.mu.Lock()
	for _, policy := range policies {
		if policy.RiskBanThreshold <= 0 || policy.RiskBanSec <= 0 {
			continue
		}
		for ip, score := range e.riskScores {
			if score >= policy.RiskBanThreshold && !IsBanned(bannedNets, allowNets, ip) {
				pending = append(pending, escalation{policy: policy, ip: ip, score: score})
			}
		}
	}
	e.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	var events []store.RiskEvent
	for _, esc := range pending {
		if err := e.net.BanTemporary(esc.ip, esc.policy.Direction, now.Add(time.Duration(esc.policy.RiskBanSec)*time.Second), policySource(esc.policy.ID)); err != nil {
			slog.Error("风险升级自动封禁失败", "policy", esc.policy.Name, "ip", esc.ip, "error", err)
			continue
		}
		bannedNets = appendIfMissing(bannedNets, esc.ip)
		events = append(events, store.RiskEvent{
			RemoteIP:     esc.ip,
			Ts:           now.Unix(),
			PolicyID:     esc.policy.ID,
			PolicyName:   esc.policy.Name,
			TriggerValue: fmt.Sprintf("风险分 %d ≥ %d", esc.score, esc.policy.RiskBanThreshold),
			Action:       "auto_ban",
			Score:        0,
		})
		// 清零重计，避免封禁到期后立刻再次升级
		e.mu.Lock()
		e.riskScores[esc.ip] = 0
		e.mu.Unlock()
		slog.Warn("风险升级自动封禁", "policy", esc.policy.Name, "ip", esc.ip, "score", esc.score, "duration_sec", esc.policy.RiskBanSec)
	}

	if len(events) > 0 {
		if err := e.store.RiskEventStore.InsertBatch(events); err != nil {
			slog.Error("写入风险升级事件失败", "error", err)
		}
	}
}

// refreshPortStats 基于相邻两次快照的差值刷新实时端口聚合缓存
func (e *PolicyEngine) refreshPortStats(stats map[string]*core.TrafficStats, now time.Time) {
	type acc struct {
		proto string
		port  int
		cur   matchCounters
		ips   map[string]uint64 // ip -> 该端口总字节数
	}
	aggs := make(map[string]*acc)

	for ip, st := range stats {
		for proto, pm := range st.Ports {
			for port, p := range pm {
				key := fmt.Sprintf("%s/%d", proto, port)
				a := aggs[key]
				if a == nil {
					a = &acc{proto: proto, port: int(port), ips: make(map[string]uint64)}
					aggs[key] = a
				}
				a.cur.bytesIn += p.BytesRecv
				a.cur.bytesOut += p.BytesSent
				a.cur.connsIn += p.ConnsIn
				a.cur.connsOut += p.ConnsOut
				a.ips[ip] = p.BytesRecv + p.BytesSent
			}
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	elapsed := e.evalInterval.Seconds()
	if !e.portPrevTs.IsZero() {
		if d := now.Sub(e.portPrevTs).Seconds(); d > 0 {
			elapsed = d
		}
	}

	result := make([]PortTraffic, 0, len(aggs))
	for key, a := range aggs {
		prev := e.portPrev[key]
		var newConns int
		if prev.counters.connsIn+prev.counters.connsOut <= a.cur.connsIn+a.cur.connsOut {
			newConns = int(a.cur.connsIn + a.cur.connsOut - prev.counters.connsIn - prev.counters.connsOut)
		}
		item := PortTraffic{
			Proto:          a.proto,
			Port:           a.port,
			BytesIn:        a.cur.bytesIn,
			BytesOut:       a.cur.bytesOut,
			BytesInPerSec:  float64(deltaSub(a.cur.bytesIn, prev.counters.bytesIn)) / elapsed,
			BytesOutPerSec: float64(deltaSub(a.cur.bytesOut, prev.counters.bytesOut)) / elapsed,
			IPCount:        len(a.ips),
			NewConns:       newConns,
		}

		// 按流量取前若干个客户端
		clients := make([]string, 0, len(a.ips))
		for ip := range a.ips {
			clients = append(clients, ip)
		}
		sort.Slice(clients, func(i, j int) bool { return a.ips[clients[i]] > a.ips[clients[j]] })
		if len(clients) > topClientsPerPort {
			clients = clients[:topClientsPerPort]
		}
		item.TopClients = clients

		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].BytesInPerSec+result[i].BytesOutPerSec > result[j].BytesInPerSec+result[j].BytesOutPerSec
	})
	if len(result) > maxPortSummaries {
		result = result[:maxPortSummaries]
	}
	e.portStats = result

	// 更新端口快照基线
	prev := make(map[string]portPrevEntry, len(aggs))
	for key, a := range aggs {
		prev[key] = portPrevEntry{counters: matchCounters{
			bytesIn:  a.cur.bytesIn,
			bytesOut: a.cur.bytesOut,
			connsIn:  a.cur.connsIn,
			connsOut: a.cur.connsOut,
		}}
	}
	e.portPrev = prev
	e.portPrevTs = now
}

// policySource 策略触发的规则的来源标记
func policySource(policyID uint) string {
	return fmt.Sprintf("policy:%d", policyID)
}

// rateLimitRuleFromPolicy 由策略生成内核限速规则
func rateLimitRuleFromPolicy(p *store.Policy) core.RateLimitRule {
	protocol := p.Protocol
	if protocol == store.ProtocolAny {
		protocol = ""
	}
	return core.RateLimitRule{
		ID:        p.ID,
		Protocol:  protocol,
		Port:      uint16(p.Port),
		RateKBps:  p.LimitKBps,
		BurstKBps: p.BurstKBps,
		Direction: p.Direction,
	}
}

// appendIfMissing 向网段列表追加单个 IP（已包含则原样返回）
func appendIfMissing(nets []*net.IPNet, ip string) []*net.IPNet {
	if isContainIpNet(nets, ip) {
		return nets
	}
	if n := parseIpNet(ip); n != nil {
		return append(nets, n)
	}
	return nets
}
