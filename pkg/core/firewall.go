package core

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
		core = &IptablesFirewallCore{
			chain: cfg.Chain,
		}
	default:
		return nil, fmt.Errorf("invalid firewall type: %s", cfg.Type)
	}

	return NewFirewall(core), nil
}

// FirewallCore 定义防火墙核心操作接口
type FirewallCore interface {
	// 初始化防火墙规则
	InitRules() error
	// 设置IP的防火墙动作
	SetAction(ipNet string, action string) error
	// 清理Ip的防火墙规则
	CleanupIpNetRules(ipNet string) error
	// 清理防火墙规则
	CleanupRules() error
}

// Firewall 提供统一的防火墙接口，通过组合不同的FirewallCore实现不同功能
type Firewall struct {
	core FirewallCore
}

func NewFirewall(core FirewallCore) *Firewall {
	firewall := &Firewall{
		core: core,
	}

	// 注册程序退出信号监听，自动清理防火墙规则
	go firewall.setupSignalHandler()

	return firewall
}

func (f *Firewall) setupSignalHandler() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-sigChan

	slog.Info("收到退出信号，开始清理防火墙规则")
	if err := f.Cleanup(); err != nil {
		slog.Error("清理防火墙规则失败", "error", err)
	}

	os.Exit(0)
}

func (f *Firewall) Init(ipList []store.IpNet) error {
	// 初始化防火墙规则
	err := f.core.InitRules()
	if err != nil {
		return fmt.Errorf("初始化防火墙规则失败: %w", err)
	}

	// 从传入的IP列表中加载所有IP到防火墙规则
	for _, ipnet := range ipList {
		err = f.core.SetAction(ipnet.IpNet, ipnet.Action)
		if err != nil {
			_ = f.core.CleanupRules()
			return fmt.Errorf("初始化IP规则失败 %s, action: %s: %w", ipnet.IpNet, ipnet.Action, err)
		}
	}

	return nil
}

func (f *Firewall) SetAction(ipNet string, action string) error {
	// 设置IP的防火墙动作
	err := f.core.SetAction(ipNet, action)
	if err != nil {
		return fmt.Errorf("设置IP防火墙动作失败: %w", err)
	}

	return nil
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

func (m *MockFirewallCore) SetAction(ipNet string, action string) error {
	switch action {
	case store.ActionBan:
		slog.Info("设置IP防火墙动作", "ip", ipNet, "action", action)
	case store.ActionAllow:
		slog.Info("设置IP防火墙动作", "ip", ipNet, "action", action)
	default:
		slog.Warn("未知的防火墙动作", "ip", ipNet, "action", action)
	}
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
