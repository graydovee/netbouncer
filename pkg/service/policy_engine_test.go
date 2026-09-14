package service

import (
	"testing"
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

// makeStats 构造带协议/端口维度的测试统计（顶层总量与明细一致，monitor 保证同步）
func makeStats() *core.TrafficStats {
	return &core.TrafficStats{
		RemoteIP:    "1.2.3.4",
		BytesRecv:   6264,
		BytesSent:   8800,
		PacketsRecv: 6,
		PacketsSent: 4,
		ConnsIn:     12,
		ConnsOut:    3,
		Ports: map[string]map[uint16]*core.ProtoPortStats{
			"tcp": {
				22:  {BytesRecv: 1000, BytesSent: 500, ConnsIn: 10},
				443: {BytesRecv: 5000, BytesSent: 8000, ConnsIn: 2, ConnsOut: 3},
			},
			"udp": {
				53: {BytesRecv: 200, BytesSent: 300},
			},
			"icmp": {
				0: {BytesRecv: 64},
			},
		},
		Protocols: map[string]*core.ProtoPortStats{
			"tcp":  {BytesRecv: 6000, BytesSent: 8500},
			"udp":  {BytesRecv: 200, BytesSent: 300},
			"icmp": {BytesRecv: 64},
		},
	}
}

func TestMatchStats(t *testing.T) {
	st := makeStats()

	// 任意协议 + 任意端口：聚合全部
	got := matchStats(st, &store.Policy{Protocol: store.ProtocolAny, Port: 0})
	if got.bytesIn != 6264 || got.bytesOut != 8800 {
		t.Fatalf("全量聚合错误: got in=%d out=%d, want in=6264 out=8800", got.bytesIn, got.bytesOut)
	}
	if got.connsIn != 12 || got.connsOut != 3 {
		t.Fatalf("连接聚合错误: got in=%d out=%d, want in=12 out=3", got.connsIn, got.connsOut)
	}

	// 指定协议 + 指定端口：只聚合 tcp/22
	got = matchStats(st, &store.Policy{Protocol: store.ProtocolTCP, Port: 22})
	if got.bytesIn != 1000 || got.bytesOut != 500 {
		t.Fatalf("tcp/22 聚合错误: got in=%d out=%d, want in=1000 out=500", got.bytesIn, got.bytesOut)
	}
	if got.connsIn != 10 {
		t.Fatalf("tcp/22 连接聚合错误: got in=%d, want 10", got.connsIn)
	}

	// 指定协议 + 任意端口：聚合 tcp 全部端口
	got = matchStats(st, &store.Policy{Protocol: store.ProtocolTCP, Port: 0})
	if got.bytesIn != 6000 || got.bytesOut != 8500 {
		t.Fatalf("tcp 全端口聚合错误: got in=%d out=%d, want in=6000 out=8500", got.bytesIn, got.bytesOut)
	}

	// udp 协议不包含 tcp 端口
	got = matchStats(st, &store.Policy{Protocol: store.ProtocolUDP, Port: 22})
	if got.bytesIn != 0 || got.bytesOut != 0 {
		t.Fatalf("udp/22 应为空: got in=%d out=%d", got.bytesIn, got.bytesOut)
	}
}

func TestCheckTrigger(t *testing.T) {
	// 窗口 2 个桶 × 10 秒 = 20 秒，入站总量 20480 KB
	agg := windowAgg{bytesIn: 20480 * 1024, connsIn: 25, ports: map[uint32]struct{}{portKey("tcp", 22): {}, portKey("tcp", 80): {}}}
	windowSeconds := 20.0

	cases := []struct {
		name    string
		policy  store.Policy
		want    bool
		wantVal string
	}{
		{
			name:   "速率未达阈值",
			policy: store.Policy{Direction: store.DirectionIn, RateKBps: 2048},
			want:   false,
		},
		{
			name:   "速率达到阈值(入站)",
			policy: store.Policy{Direction: store.DirectionIn, RateKBps: 1000},
			want:   true,
		},
		{
			name:   "速率阈值按出站侧统计不触发",
			policy: store.Policy{Direction: store.DirectionOut, RateKBps: 1},
			want:   false,
		},
		{
			name:   "双向统计包含入站",
			policy: store.Policy{Direction: store.DirectionBoth, RateKBps: 1000},
			want:   true,
		},
		{
			name:   "累计流量达到阈值",
			policy: store.Policy{Direction: store.DirectionIn, TotalMB: 20},
			want:   true,
		},
		{
			name:   "连接数达到阈值",
			policy: store.Policy{Direction: store.DirectionIn, ConnRate: 25},
			want:   true,
		},
		{
			name:   "连接数未达阈值",
			policy: store.Policy{Direction: store.DirectionIn, ConnRate: 26},
			want:   false,
		},
		{
			name:   "触碰端口数达到阈值",
			policy: store.Policy{Direction: store.DirectionIn, DistinctPorts: 2},
			want:   true,
		},
		{
			name:   "触碰端口数未达阈值",
			policy: store.Policy{Direction: store.DirectionIn, DistinctPorts: 3},
			want:   false,
		},
		{
			name:   "无任何触发条件",
			policy: store.Policy{Direction: store.DirectionIn},
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := checkTrigger(&tc.policy, agg, windowSeconds)
			if got != tc.want {
				t.Fatalf("checkTrigger = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlanRange(t *testing.T) {
	const bucket600 = store.RollupBucket10m

	now := time.Now().Unix()
	rawCutoff := now - int64(rawIPRetention.Seconds())

	t.Run("完全在 raw 窗口内", func(t *testing.T) {
		start := now - 3600
		plan := planRange(start, now, 60, rawCutoff)
		if !plan.useRaw || plan.useRollup {
			t.Fatalf("应只查 raw: %+v", plan)
		}
		if plan.effBucket != 60 {
			t.Fatalf("桶宽不应调整: %d", plan.effBucket)
		}
	})

	t.Run("完全早于 raw 窗口", func(t *testing.T) {
		start := rawCutoff - 3*86400
		end := rawCutoff - 2*86400
		plan := planRange(start, end, 300, rawCutoff)
		if plan.useRaw || !plan.useRollup {
			t.Fatalf("应只查聚合层: %+v", plan)
		}
		if plan.rollupLayer != bucket600 {
			t.Fatalf("5天范围应用10分钟层: %d", plan.rollupLayer)
		}
		if plan.effBucket < bucket600 {
			t.Fatalf("聚合层桶宽不应小于层粒度: %d", plan.effBucket)
		}
	})

	t.Run("跨层查询无缝无重叠", func(t *testing.T) {
		start := rawCutoff - 3*86400
		end := now
		bucket := int64(3600)
		plan := planRange(start, end, bucket, rawCutoff)
		if !plan.useRaw || !plan.useRollup {
			t.Fatalf("跨层查询应同时使用两层: %+v", plan)
		}
		if plan.rollupTo != plan.rawFrom {
			t.Fatalf("聚合层终点(%d)应等于 raw 起点(%d)", plan.rollupTo, plan.rawFrom)
		}
		if plan.rollupFrom != start || plan.rawTo != end {
			t.Fatalf("区间端点错误: %+v", plan)
		}
		// rollupTo 必须对齐到有效桶宽，保证跨越桶完整归聚合层
		if plan.rollupTo%plan.effBucket != 0 {
			t.Fatalf("分界点未对齐桶宽: rollupTo=%d effBucket=%d", plan.rollupTo, plan.effBucket)
		}
	})

	t.Run("大跨度使用1小时层", func(t *testing.T) {
		start := rawCutoff - 20*86400
		end := rawCutoff - 86400
		plan := planRange(start, end, 300, rawCutoff)
		if plan.rollupLayer != store.RollupBucket1h {
			t.Fatalf("20天范围应用1小时层: %d", plan.rollupLayer)
		}
	})
}

func TestValidatePolicy(t *testing.T) {
	valid := store.Policy{
		Name: "测试策略", Enabled: true,
		Direction: store.DirectionIn, Protocol: store.ProtocolTCP, Port: 22,
		ConnRate: 30, WindowSec: 60,
		Action: store.PolicyActionBan, BanSec: 600, RiskScore: 1, CooldownSec: 300,
	}
	if err := validatePolicy(&valid); err != nil {
		t.Fatalf("合法策略不应报错: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*store.Policy)
	}{
		{"名称为空", func(p *store.Policy) { p.Name = "" }},
		{"非法方向", func(p *store.Policy) { p.Direction = "sideways" }},
		{"非法协议", func(p *store.Policy) { p.Protocol = "sctp" }},
		{"端口越界", func(p *store.Policy) { p.Port = 70000 }},
		{"ban 缺少时长", func(p *store.Policy) { p.BanSec = 0 }},
		{"mark 缺少触发条件", func(p *store.Policy) {
			p.Action = store.PolicyActionMark
			p.ConnRate = 0
		}},
		{"限速缺少限速值", func(p *store.Policy) {
			p.Action = store.PolicyActionRateLimit
			p.ConnRate = 0
			p.LimitKBps = 0
		}},
		{"指定端口时协议为 any", func(p *store.Policy) {
			p.Action = store.PolicyActionRateLimit
			p.ConnRate = 0
			p.LimitKBps = 1024
			p.Port = 443
			p.Protocol = store.ProtocolAny
		}},
		{"升级阈值与时长不同时设置", func(p *store.Policy) { p.RiskBanThreshold = 3 }},
		{"distinct_ports 与指定端口冲突", func(p *store.Policy) { p.DistinctPorts = 10 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := valid
			tc.mutate(&p)
			if err := validatePolicy(&p); err == nil {
				t.Fatal("期望校验失败，实际通过")
			}
		})
	}

	// 默认值填充
	p := valid
	p.WindowSec = 0
	p.CooldownSec = 0
	if err := validatePolicy(&p); err != nil {
		t.Fatalf("默认值填充不应报错: %v", err)
	}
	if p.WindowSec != 300 || p.CooldownSec != 300 {
		t.Fatalf("默认值错误: window=%d cooldown=%d", p.WindowSec, p.CooldownSec)
	}
}
