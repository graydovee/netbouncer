package core

import (
	"fmt"
	"log/slog"
	"net"
	"slices"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

// IpSetFirewallCore 实现ipset防火墙的核心操作
type IpSetFirewallCore struct {
	ipset string
	chain string
}

func (i *IpSetFirewallCore) InitRules() error {
	slog.Info("初始化ipset防火墙", "ipset", i.ipset, "chain", i.chain)

	// 创建ipset
	err := i.createIpSet()
	if err != nil {
		return fmt.Errorf("创建ipset失败: %w", err)
	}

	// 设置iptables规则
	err = i.setupIptables()
	if err != nil {
		return fmt.Errorf("设置iptables规则失败: %w", err)
	}

	return nil
}

func (i *IpSetFirewallCore) createIpSet() error {
	// 检查ipset是否已存在
	_, err := netlink.IpsetList(i.ipset)
	if err == nil {
		// ipset已存在，清空它
		slog.Info("清空已存在的ipset", "ipset", i.ipset, "cmd", "ipset flush "+i.ipset)
		err = netlink.IpsetFlush(i.ipset)
		if err != nil {
			return fmt.Errorf("清空ipset失败: %w", err)
		}
	} else {
		// 创建新的ipset
		slog.Info("创建新的ipset", "ipset", i.ipset, "cmd", "ipset create "+i.ipset+" hash:net family inet hashsize 1024 maxelem 65536")
		options := netlink.IpsetCreateOptions{
			Replace: true,
		}
		err = netlink.IpsetCreate(i.ipset, "hash:net", options)
		if err != nil {
			return fmt.Errorf("创建ipset失败: %w", err)
		}
	}
	return nil
}

func (i *IpSetFirewallCore) setupIptables() error {
	// 使用go-iptables库设置iptables规则
	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	// 检查链是否存在，存在则清空，不存在则新建
	chains, err := ipt.ListChains("filter")
	if err != nil {
		return err
	}
	if slices.Contains(chains, i.chain) {
		slog.Info("清空链中的所有规则", "cmd", "iptables -F "+i.chain)
		_ = ipt.ClearChain("filter", i.chain)
	} else {
		slog.Info("创建新的自定义链", "cmd", "iptables -N "+i.chain)
		_ = ipt.NewChain("filter", i.chain)
	}

	// 检查 INPUT 链是否已经包含对自定义链的引用
	rules, err := ipt.List("filter", "INPUT")
	if err != nil {
		return err
	}

	// 查找是否已存在指向自定义链的规则
	ruleExists := slices.Contains(rules, "-j "+i.chain)

	// 只有在规则不存在时才插入
	if !ruleExists {
		slog.Info("初始化自定义链", "cmd", "iptables -I INPUT 1 -j "+i.chain)
		_ = ipt.Insert("filter", "INPUT", 1, "-j", i.chain)
	}

	// 添加ipset规则到自定义链
	// iptables -A <chain> -m set --match-set <ipset> src -j DROP
	slog.Info("添加ipset规则到iptables", "cmd", "iptables -A "+i.chain+" -m set --match-set "+i.ipset+" src -j DROP")
	err = ipt.AppendUnique("filter", i.chain, "-m", "set", "--match-set", i.ipset, "src", "-j", "DROP")
	if err != nil {
		return fmt.Errorf("添加ipset规则到iptables失败: %w", err)
	}

	return nil
}

func (i *IpSetFirewallCore) AddToRules(ipOrCidr string) error {
	// 解析IP或CIDR
	ip, ipNet, err := net.ParseCIDR(ipOrCidr)
	if err != nil {
		// 如果不是CIDR格式，尝试解析为单个IP
		ip = net.ParseIP(ipOrCidr)
		if ip == nil {
			return fmt.Errorf("无效的IP或CIDR格式: %s", ipOrCidr)
		}
		// 单个IP转换为/32或/128 CIDR
		if ip.To4() != nil {
			ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
		} else {
			ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
		}
	}

	// 计算CIDR前缀长度
	ones, _ := ipNet.Mask.Size()
	cidr := uint8(ones)

	entry := &netlink.IPSetEntry{
		IP:   ipNet.IP,
		CIDR: cidr,
	}

	slog.Info("添加到ipset", "ip", ipOrCidr, "cidr", cidr, "cmd", "ipset add "+i.ipset+" "+ipOrCidr)
	return netlink.IpsetAdd(i.ipset, entry)
}

func (i *IpSetFirewallCore) RemoveFromRules(ipOrCidr string) error {
	// 解析IP或CIDR
	ip, ipNet, err := net.ParseCIDR(ipOrCidr)
	if err != nil {
		// 如果不是CIDR格式，尝试解析为单个IP
		ip = net.ParseIP(ipOrCidr)
		if ip == nil {
			return fmt.Errorf("无效的IP或CIDR格式: %s", ipOrCidr)
		}
		// 单个IP转换为/32或/128 CIDR
		if ip.To4() != nil {
			ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
		} else {
			ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
		}
	}

	// 计算CIDR前缀长度
	ones, _ := ipNet.Mask.Size()
	cidr := uint8(ones)

	entry := &netlink.IPSetEntry{
		IP:   ipNet.IP,
		CIDR: cidr,
	}

	slog.Info("从ipset中删除", "ip", ipOrCidr, "cidr", cidr, "cmd", "ipset del "+i.ipset+" "+ipOrCidr)
	return netlink.IpsetDel(i.ipset, entry)
}

func (i *IpSetFirewallCore) CleanupRules() error {
	slog.Info("清理ipset防火墙规则")

	// 先清理iptables规则，避免ipset被引用
	ipt, err := iptables.New()
	if err != nil {
		slog.Error("创建iptables实例失败", "error", err)
		return err
	}

	// 从INPUT链移除所有指向自定义链的规则
	for {
		err := ipt.Delete("filter", "INPUT", "-j", i.chain)
		if err != nil {
			break
		}
		slog.Info("清除自定义链的规则", "cmd", "iptables -D INPUT -j "+i.chain)
	}

	// 清空自定义链中的所有规则
	slog.Info("清空自定义链中的所有规则", "cmd", "iptables -F "+i.chain)
	_ = ipt.ClearChain("filter", i.chain)

	// 删除自定义链
	slog.Info("删除自定义链", "cmd", "iptables -X "+i.chain)
	_ = ipt.DeleteChain("filter", i.chain)

	// 清空ipset
	slog.Info("清空ipset", "ipset", i.ipset, "cmd", "ipset flush "+i.ipset)
	err = netlink.IpsetFlush(i.ipset)
	if err != nil {
		slog.Error("清空ipset失败", "error", err)
	}

	// 删除ipset
	slog.Info("删除ipset", "ipset", i.ipset, "cmd", "ipset destroy "+i.ipset)
	err = netlink.IpsetDestroy(i.ipset)
	if err != nil {
		slog.Error("删除ipset失败", "error", err)
	}

	return nil
}
