package core

import (
	"sort"
	"sync"

	"github.com/coreos/go-iptables/iptables"
)

type Firewall interface {
	Ban(ip string) error
	Unban(ip string) error
	IsBanned(ip string) bool
	GetBannedIPs() ([]string, error)
	Cleanup() error
}

type IptablesFirewall struct {
	ipt    *iptables.IPTables
	banned map[string]struct{}
	mu     sync.Mutex
	chain  string
}

func NewIptablesFirewall(chain string) (*IptablesFirewall, error) {
	ipt, err := iptables.New()
	if err != nil {
		return nil, err
	}
	// 检查链是否存在，存在则清空，不存在则新建
	// iptables -L <chain> 检查链是否存在
	chains, err := ipt.ListChains("filter")
	if err != nil {
		return nil, err
	}
	exists := false
	for _, c := range chains {
		if c == chain {
			exists = true
			break
		}
	}
	if exists {
		// iptables -F <chain>
		_ = ipt.ClearChain("filter", chain)
	} else {
		// iptables -N <chain>
		_ = ipt.NewChain("filter", chain)
	}
	// 确保链已连接到INPUT
	// iptables -I INPUT 1 -j <chain>
	_ = ipt.Insert("filter", "INPUT", 1, "-j", chain)
	return &IptablesFirewall{
		ipt:    ipt,
		banned: make(map[string]struct{}),
		chain:  chain,
	}, nil
}

func (f *IptablesFirewall) Ban(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.banned[ip]; ok {
		return nil
	}
	// iptables -A <chain> -s <ip> -j DROP
	err := f.ipt.AppendUnique("filter", f.chain, "-s", ip, "-j", "DROP")
	if err == nil {
		f.banned[ip] = struct{}{}
	}
	return err
}

func (f *IptablesFirewall) Unban(ip string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.banned[ip]; !ok {
		return nil
	}
	// iptables -D <chain> -s <ip> -j DROP
	err := f.ipt.Delete("filter", f.chain, "-s", ip, "-j", "DROP")
	if err == nil {
		delete(f.banned, ip)
	}
	return err
}

func (f *IptablesFirewall) IsBanned(ip string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.banned[ip]
	return ok
}

func (f *IptablesFirewall) GetBannedIPs() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ips := make([]string, 0, len(f.banned))
	for ip := range f.banned {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips, nil
}

func (f *IptablesFirewall) Cleanup() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// 清空自定义链
	// iptables -F <chain>
	_ = f.ipt.ClearChain("filter", f.chain)
	// 从INPUT链移除自定义链
	// iptables -D INPUT -j <chain>
	_ = f.ipt.Delete("filter", "INPUT", "-j", f.chain)
	// 删除自定义链
	// iptables -X <chain>
	_ = f.ipt.DeleteChain("filter", f.chain)
	f.banned = make(map[string]struct{})
	return nil
}

type MockFirewall struct {
	mu     sync.Mutex
	banned map[string]struct{}
}

func NewMockFirewall() *MockFirewall {
	return &MockFirewall{
		banned: make(map[string]struct{}),
	}
}

func (m *MockFirewall) Ban(ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.banned[ip] = struct{}{}
	return nil
}

func (m *MockFirewall) Unban(ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.banned[ip]; !ok {
		return nil
	}
	delete(m.banned, ip)
	return nil
}

func (m *MockFirewall) IsBanned(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.banned[ip]
	return ok
}

func (m *MockFirewall) GetBannedIPs() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ips := make([]string, 0, len(m.banned))
	for ip := range m.banned {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips, nil
}

func (m *MockFirewall) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.banned = make(map[string]struct{})
	return nil
}
