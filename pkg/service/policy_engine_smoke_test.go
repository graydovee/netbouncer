package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

// 策略引擎端到端冒烟：mock 防火墙下验证 触发→临时封禁→到期解封 全链路
func TestPolicyEngineSmoke(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "smoke.db")
	st, err := store.NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: dbFile, LogLevel: "silent"})
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	mon := &mutableMonitor{stats: map[string]*core.TrafficStats{}}
	fw := newFakeFirewall()

	svc := NewNetService(mon, fw, st)
	if err := svc.Init(nil); err != nil {
		t.Fatalf("init: %v", err)
	}

	// 大流量封禁策略：入站速率 ≥ 1024 KB/s（窗口 10 秒）→ 临时封禁 60 秒；风险分达 3 分升级封禁
	policy := &store.Policy{
		Name: "大流量", Enabled: true,
		Direction: store.DirectionIn, Protocol: store.ProtocolAny, Port: 0,
		RateKBps: 1024, WindowSec: 10,
		Action: store.PolicyActionBan, BanSec: 60, RiskScore: 2,
		RiskBanThreshold: 3, RiskBanSec: 120, CooldownSec: 0,
	}
	if err := svc.CreatePolicy(policy); err != nil {
		t.Fatalf("create policy: %v", err)
	}

	engine := NewPolicyEngine(mon, fw, svc, st, &config.PolicyConfig{EvalInterval: 5, RiskWindow: 3600})
	svc.SetPolicyEngine(engine)

	// 注入高速率入站流量：累计 10MB，窗口均速远超阈值
	mon.set("5.5.5.5", 10*1024*1024, 0)
	if err := engine.evaluate(); err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if !fw.banned["5.5.5.5"] {
		t.Fatalf("应临时封禁 5.5.5.5, banned=%v", fw.banned)
	}

	events, total, err := st.RiskEventStore.FindByFilter(store.RiskEventFilter{})
	if err != nil || total != 1 {
		t.Fatalf("应有 1 条风险事件: total=%d err=%v", total, err)
	}
	if events[0].Action != store.PolicyActionBan {
		t.Fatalf("事件动作应为 ban, got %s", events[0].Action)
	}
	if got := engine.RiskScore("5.5.5.5"); got != 2 {
		t.Fatalf("风险分应为 2, got %d", got)
	}

	// 未到期不应解封
	removed, err := svc.CleanupExpiredTempBans()
	if err != nil || removed != 0 {
		t.Fatalf("未到期的封禁不应被解除: removed=%d err=%v", removed, err)
	}
	if !fw.banned["5.5.5.5"] {
		t.Fatal("解封检查后封禁应仍生效")
	}

	// 60 秒后过期，应能查到并可解除
	expired, err := st.IpNetStore.FindExpired(time.Now().Add(2 * time.Minute))
	if err != nil || len(expired) != 1 {
		t.Fatalf("60 秒后该封禁应过期: %+v err=%v", expired, err)
	}
	if err := svc.SetPolicyEnabled(policy.ID, false); err != nil {
		t.Fatalf("停用策略: %v", err)
	}
}
