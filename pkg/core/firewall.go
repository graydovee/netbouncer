package core

import (
	"fmt"
	"log/slog"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/store"
)

// NewFirewallFromConfig 根据配置创建相应的防火墙实例
func NewFirewallFromConfig(cfg *config.FirewallConfig) (*Firewall, error) {
	var core FirewallCore

	switch config.FirewallType(cfg.Type) {
	case config.FirewallTypeMock:
		core = &MockFirewallCore{}
	case config.FirewallTypeIpSet:
		// 如果配置了ipset，使用ipset防火墙
		if cfg.Chain == "" {
			return nil, fmt.Errorf("ipset chain is required")
		}
		if cfg.IpSet == "" {
			return nil, fmt.Errorf("ipset name is required")
		}
		slog.Info("使用IpSet防火墙", "ipset", cfg.IpSet, "chain", cfg.Chain)
		core = &IpSetFirewallCore{
			ipset: cfg.IpSet,
			chain: cfg.Chain,
		}
	case config.FirewallTypeIptables:
		if cfg.Chain == "" {
			return nil, fmt.Errorf("iptables chain is required")
		}
		slog.Info("使用Iptables防火墙", "chain", cfg.Chain)
		core = &IptablesFirewallCore{
			chain: cfg.Chain,
		}
	default:
		return nil, fmt.Errorf("invalid firewall type: %s", cfg.Type)
	}

	return NewFirewall(core), nil
}

// RateLimitRule 内核级每 IP 限速规则（hashlimit 令牌桶，超限丢包）
type RateLimitRule struct {
	ID        uint    // 规则标识（策略ID），用于 hashlimit-name
	Protocol  string  // tcp|udp；Port>0 时必须指定
	Port      uint16  // 0 = 任意端口
	RateKBps  float64 // 限速值 KB/s
	BurstKBps float64 // 突发容量 KB/s，0 = 2×RateKBps
	Direction string  // in|out|both
}

// FirewallCore 定义防火墙核心操作接口
type FirewallCore interface {
	// 初始化防火墙规则
	InitRules() error

	// direction: in（入站）/out（出站）/both
	Ban(ipNet string, direction string) error
	RevertBan(ipNet string, direction string) error
	Allow(ipNet string) error
	RevertAllow(ipNet string) error

	// 内核级限速规则的装卸（策略动作 rate_limit）
	ApplyRateLimit(rule RateLimitRule) error
	RemoveRateLimit(rule RateLimitRule) error

	// 清理Ip的防火墙规则
	CleanupIpNetRules(ipNet string) error
	// 清理防火墙规则
	CleanupRules() error
}

// Firewall 提供统一的防火墙接口，通过组合不同的FirewallCore实现不同功能
// 退出时的规则清理由 cmd 层统一在优雅退出流程中调用 Cleanup 完成
type Firewall struct {
	core FirewallCore
}

func NewFirewall(core FirewallCore) *Firewall {
	return &Firewall{core: core}
}

func (f *Firewall) Init(ipList []store.IpNet) error {
	// 初始化防火墙规则
	if err := f.core.InitRules(); err != nil {
		return fmt.Errorf("初始化防火墙规则失败: %w", err)
	}

	// 从传入的IP列表中加载所有IP到防火墙规则
	for _, ipnet := range ipList {
		var err error
		switch ipnet.Action {
		case store.ActionBan:
			err = f.core.Ban(ipnet.IpNet, ipnet.Direction)
		case store.ActionAllow:
			err = f.core.Allow(ipnet.IpNet)
		default:
			return fmt.Errorf("不支持的防火墙动作: %s", ipnet.Action)
		}
		if err != nil {
			_ = f.core.CleanupRules()
			return fmt.Errorf("初始化IP规则失败 %s, action: %s: %w", ipnet.IpNet, ipnet.Action, err)
		}
	}

	return nil
}

func (f *Firewall) Ban(ipNet string, direction string) error {
	return f.core.Ban(ipNet, direction)
}

func (f *Firewall) RevertBan(ipNet string, direction string) error {
	return f.core.RevertBan(ipNet, direction)
}

func (f *Firewall) Allow(ipNet string) error {
	return f.core.Allow(ipNet)
}

func (f *Firewall) RevertAllow(ipNet string) error {
	return f.core.RevertAllow(ipNet)
}

func (f *Firewall) ApplyRateLimit(rule RateLimitRule) error {
	return f.core.ApplyRateLimit(rule)
}

func (f *Firewall) RemoveRateLimit(rule RateLimitRule) error {
	return f.core.RemoveRateLimit(rule)
}

func (f *Firewall) CleanupIpNet(ipNet string) error {
	return f.core.CleanupIpNetRules(ipNet)
}

func (f *Firewall) Cleanup() error {
	return f.core.CleanupRules()
}

// MockFirewallCore 实现Mock防火墙的核心操作
type MockFirewallCore struct{}

func (m *MockFirewallCore) InitRules() error {
	// Mock防火墙不需要复杂的初始化
	return nil
}

func (m *MockFirewallCore) Ban(ipNet string, direction string) error {
	return nil
}

func (m *MockFirewallCore) RevertBan(ipNet string, direction string) error {
	return nil
}

func (m *MockFirewallCore) Allow(ipNet string) error {
	return nil
}

func (m *MockFirewallCore) RevertAllow(ipNet string) error {
	return nil
}

func (m *MockFirewallCore) ApplyRateLimit(rule RateLimitRule) error {
	return nil
}

func (m *MockFirewallCore) RemoveRateLimit(rule RateLimitRule) error {
	return nil
}

func (m *MockFirewallCore) CleanupIpNetRules(ipNet string) error {
	// Mock防火墙不需要清理IP规则
	return nil
}

func (m *MockFirewallCore) CleanupRules() error {
	// Mock防火墙不需要清理规则
	return nil
}
