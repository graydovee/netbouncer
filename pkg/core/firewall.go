package core

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"slices"

	"github.com/coreos/go-iptables/iptables"
	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/store"
)

type Firewall interface {
	Init() error
	Ban(ip string) error
	Unban(ip string) error
	IsBanned(ip string) bool
	GetBannedIPs() ([]string, error)
	Cleanup() error
}

var (
	_ Firewall = (*IptablesFirewall)(nil)
	_ Firewall = (*MockFirewall)(nil)
)

// NewFirewall 根据配置创建防火墙实例
func NewFirewall(cfg *config.Config) (Firewall, error) {
	// 首先创建IP存储
	ipStore, err := store.NewIpStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP store: %w", err)
	}

	// 根据配置创建防火墙
	// 这里可以根据需要添加更多防火墙类型
	// 目前只支持iptables和mock
	if cfg.Debug {
		// 调试模式使用Mock防火墙
		return NewMockFirewall(ipStore), nil
	}

	// 生产环境使用iptables防火墙
	if cfg.Firewall.Chain == "" {
		cfg.Firewall.Chain = "NETBOUNCER"
	}

	return NewIptablesFirewall(ipStore, &cfg.Firewall), nil
}

type IptablesFirewall struct {
	ipt     *iptables.IPTables
	ipStore store.IpStore

	chain string
}

func NewIptablesFirewall(ipStore store.IpStore, cfg *config.FirewallConfig) *IptablesFirewall {
	firewall := &IptablesFirewall{
		ipStore: ipStore,
		chain:   cfg.Chain,
	}

	return firewall
}

func (f *IptablesFirewall) Init() error {
	slog.Info("初始化iptables规则")
	ipt, err := iptables.New()
	if err != nil {
		return err
	}
	f.ipt = ipt

	// 注册程序退出信号监听，自动清理 iptables 规则
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

		<-sigChan

		if err := f.Cleanup(); err != nil {
			_ = err
		}

		os.Exit(0)
	}()

	// 检查链是否存在，存在则清空，不存在则新建
	// iptables -L <chain> 检查链是否存在
	chains, err := f.ipt.ListChains("filter")
	if err != nil {
		return err
	}
	if slices.Contains(chains, f.chain) {
		// iptables -F <chain> 清空链中的所有规则
		_ = f.ipt.ClearChain("filter", f.chain)
	} else {
		// iptables -N <chain> 创建新的自定义链
		_ = f.ipt.NewChain("filter", f.chain)
	}

	// 检查 INPUT 链是否已经包含对自定义链的引用
	// iptables -L INPUT 列出 INPUT 链的所有规则
	rules, err := f.ipt.List("filter", "INPUT")
	if err != nil {
		return err
	}

	// 查找是否已存在指向自定义链的规则
	ruleExists := slices.Contains(rules, "-j "+f.chain)

	// 只有在规则不存在时才插入
	if !ruleExists {
		// iptables -I INPUT 1 -j <chain> 在 INPUT 链的第1位插入规则，跳转到自定义链
		_ = f.ipt.Insert("filter", "INPUT", 1, "-j", f.chain)
	}

	// 初始化iptables规则
	ips, err := f.ipStore.GetBlacklist()
	if err != nil {
		return err
	}
	for _, ip := range ips {
		err = f.Ban(ip.Ip)
		if err != nil {
			_ = f.Cleanup()
			return fmt.Errorf("初始化iptables规则失败: %w", err)
		}
		slog.Info("初始化iptables规则", "ip", ip.Ip)
	}

	return nil
}

func (f *IptablesFirewall) Ban(ip string) error {
	// 检查IP是否已经在黑名单中
	ipRecord := store.Ip{Ip: ip}
	if f.ipStore.IsInBlacklist(ipRecord) {
		return nil
	}

	// 添加到iptables规则
	slog.Info("添加到iptables规则", "ip", ip)
	err := f.ipt.AppendUnique("filter", f.chain, "-s", ip, "-j", "DROP")
	if err != nil {
		return err
	}

	// 添加到存储
	now := time.Now()
	ipRecord.CreatedAt = now
	ipRecord.UpdatedAt = now
	return f.ipStore.AddIpBlacklist(ipRecord)
}

func (f *IptablesFirewall) Unban(ip string) error {
	// 检查IP是否在黑名单中
	ipRecord := store.Ip{Ip: ip}
	if !f.ipStore.IsInBlacklist(ipRecord) {
		return nil
	}

	// 从iptables规则中删除
	slog.Info("从iptables规则中删除", "ip", ip)
	err := f.ipt.Delete("filter", f.chain, "-s", ip, "-j", "DROP")
	if err != nil {
		return err
	}

	// 从存储中删除
	return f.ipStore.RemoveIpBlacklist(ipRecord)
}

func (f *IptablesFirewall) IsBanned(ip string) bool {
	ipRecord := store.Ip{Ip: ip}
	return f.ipStore.IsInBlacklist(ipRecord)
}

func (f *IptablesFirewall) GetBannedIPs() ([]string, error) {
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

func (f *IptablesFirewall) Cleanup() error {
	slog.Info("清理iptables规则")
	// 清空自定义链中的所有规则
	// iptables -F <chain> 清空链中的所有规则
	_ = f.ipt.ClearChain("filter", f.chain)

	// 从INPUT链移除所有指向自定义链的规则
	// 使用循环删除，直到没有更多匹配的规则
	for {
		// iptables -D INPUT -j <chain> 从 INPUT 链中删除跳转到自定义链的规则
		// 尝试删除规则，如果删除失败说明没有更多匹配的规则
		err := f.ipt.Delete("filter", "INPUT", "-j", f.chain)
		if err != nil {
			// 没有更多匹配的规则，退出循环
			break
		}
	}

	// 删除自定义链
	// iptables -X <chain> 删除自定义链（链必须为空）
	_ = f.ipt.DeleteChain("filter", f.chain)
	return nil
}

type MockFirewall struct {
	ipStore store.IpStore
}

func NewMockFirewall(ipStore store.IpStore) *MockFirewall {
	return &MockFirewall{
		ipStore: ipStore,
	}
}

func (m *MockFirewall) Init() error {
	// Mock防火墙不需要复杂的初始化
	return nil
}

func (m *MockFirewall) Ban(ip string) error {
	ipRecord := store.Ip{Ip: ip}
	if m.ipStore.IsInBlacklist(ipRecord) {
		return nil
	}

	slog.Info("添加到Mock防火墙规则", "ip", ip)
	now := time.Now()
	ipRecord.CreatedAt = now
	ipRecord.UpdatedAt = now
	return m.ipStore.AddIpBlacklist(ipRecord)
}

func (m *MockFirewall) Unban(ip string) error {
	ipRecord := store.Ip{Ip: ip}
	if !m.ipStore.IsInBlacklist(ipRecord) {
		return nil
	}

	slog.Info("从Mock防火墙规则中删除", "ip", ip)
	return m.ipStore.RemoveIpBlacklist(ipRecord)
}

func (m *MockFirewall) IsBanned(ip string) bool {
	ipRecord := store.Ip{Ip: ip}
	return m.ipStore.IsInBlacklist(ipRecord)
}

func (m *MockFirewall) GetBannedIPs() ([]string, error) {
	ips, err := m.ipStore.GetBlacklist()
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

func (m *MockFirewall) Cleanup() error {
	// Mock防火墙不需要清理iptables规则
	return nil
}
