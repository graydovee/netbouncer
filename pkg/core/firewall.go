package core

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/store"
)

// NewFirewallFromConfig 根据配置创建相应的防火墙实例
func NewFirewallFromConfig(cfg *config.Config) (*Firewall, error) {
	// 首先创建IP存储
	ipStore, err := store.NewIpStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP store: %w", err)
	}

	var core FirewallCore

	// 根据配置创建相应的防火墙核心
	if cfg.Debug {
		// 调试模式使用Mock防火墙
		slog.Info("使用Mock防火墙")
		core = &MockFirewallCore{}
	} else if !cfg.Firewall.DisableIpSet {
		// 如果配置了ipset，使用ipset防火墙
		if cfg.Firewall.Chain == "" {
			return nil, fmt.Errorf("ipset chain is required")
		}
		if cfg.Firewall.IpSet == "" {
			return nil, fmt.Errorf("ipset name is required")
		}
		slog.Info("使用IpSet防火墙", "ipset", cfg.Firewall.IpSet, "chain", cfg.Firewall.Chain)
		core = &IpSetFirewallCore{
			ipset: cfg.Firewall.IpSet,
			chain: cfg.Firewall.Chain,
		}
	} else {
		// 默认使用iptables防火墙
		if cfg.Firewall.Chain == "" {
			return nil, fmt.Errorf("iptables chain is required")
		}
		slog.Info("使用Iptables防火墙", "chain", cfg.Firewall.Chain)
		core = &IptablesFirewallCore{
			chain: cfg.Firewall.Chain,
		}
	}

	return NewFirewall(ipStore, core), nil
}

// FirewallCore 定义防火墙核心操作接口
type FirewallCore interface {
	// 初始化防火墙规则
	InitRules() error
	// 添加IP到防火墙规则
	AddToRules(ip string) error
	// 从防火墙规则中删除IP
	RemoveFromRules(ip string) error
	// 清理防火墙规则
	CleanupRules() error
}

// Firewall 提供统一的防火墙接口，通过组合不同的FirewallCore实现不同功能
type Firewall struct {
	ipStore store.IpStore
	core    FirewallCore
}

func NewFirewall(ipStore store.IpStore, core FirewallCore) *Firewall {
	firewall := &Firewall{
		ipStore: ipStore,
		core:    core,
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

func (f *Firewall) Init() error {
	// 初始化防火墙规则
	err := f.core.InitRules()
	if err != nil {
		return fmt.Errorf("初始化防火墙规则失败: %w", err)
	}

	// 从存储中加载所有已存在的IP到防火墙规则
	ips, err := f.ipStore.GetBlacklist()
	if err != nil {
		return fmt.Errorf("获取黑名单失败: %w", err)
	}

	for _, ip := range ips {
		err = f.core.AddToRules(ip.Ip)
		if err != nil {
			_ = f.core.CleanupRules()
			return fmt.Errorf("初始化IP规则失败 %s: %w", ip.Ip, err)
		}
	}

	return nil
}

func (f *Firewall) Ban(ip string) error {
	// 检查IP是否已经在黑名单中
	ipRecord := store.Ip{Ip: ip}
	if f.ipStore.IsInBlacklist(ipRecord) {
		return nil
	}

	// 添加到防火墙规则
	err := f.core.AddToRules(ip)
	if err != nil {
		return fmt.Errorf("添加IP到防火墙规则失败: %w", err)
	}

	// 添加到存储
	now := time.Now()
	ipRecord.CreatedAt = now
	ipRecord.UpdatedAt = now
	err = f.ipStore.AddIpBlacklist(ipRecord)
	if err != nil {
		// 如果存储失败，需要从防火墙规则中删除
		_ = f.core.RemoveFromRules(ip)
		return fmt.Errorf("添加IP到存储失败: %w", err)
	}

	return nil
}

func (f *Firewall) Unban(ip string) error {
	// 检查IP是否在黑名单中
	ipRecord := store.Ip{Ip: ip}
	if !f.ipStore.IsInBlacklist(ipRecord) {
		return nil
	}

	// 从防火墙规则中删除
	err := f.core.RemoveFromRules(ip)
	if err != nil {
		return fmt.Errorf("从防火墙规则中删除IP失败: %w", err)
	}

	// 从存储中删除
	err = f.ipStore.RemoveIpBlacklist(ipRecord)
	if err != nil {
		// 如果存储删除失败，需要重新添加到防火墙规则
		_ = f.core.AddToRules(ip)
		return fmt.Errorf("从存储中删除IP失败: %w", err)
	}

	return nil
}

func (f *Firewall) IsBanned(ip string) bool {
	ipRecord := store.Ip{Ip: ip}
	return f.ipStore.IsInBlacklist(ipRecord)
}

func (f *Firewall) GetBannedIPs() ([]string, error) {
	ips, err := f.ipStore.GetBlacklist()
	if err != nil {
		return nil, err
	}

	ipStrings := make([]string, len(ips))
	for i, ip := range ips {
		ipStrings[i] = ip.Ip
	}
	sort.Strings(ipStrings)
	return ipStrings, nil
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

func (m *MockFirewallCore) AddToRules(ip string) error {
	slog.Info("添加到Mock防火墙规则", "ip", ip)
	return nil
}

func (m *MockFirewallCore) RemoveFromRules(ip string) error {
	slog.Info("从Mock防火墙规则中删除", "ip", ip)
	return nil
}

func (m *MockFirewallCore) CleanupRules() error {
	// Mock防火墙不需要清理规则
	return nil
}
