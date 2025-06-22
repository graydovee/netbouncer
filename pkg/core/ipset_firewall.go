package core

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

// IpSetFirewallCore 实现ipset防火墙的核心操作
type IpSetFirewallCore struct {
	ipset      string
	chain      string
	banIpSet   string
	allowIpSet string
	ipt        *iptables.IPTables
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
	// 初始化ipset名称
	i.banIpSet = i.ipset + "_ban"
	i.allowIpSet = i.ipset + "_allow"

	// 创建禁止IP的ipset
	_, err := netlink.IpsetList(i.banIpSet)
	if err == nil {
		// ipset已存在，清空它
		slog.Info("清空已存在的禁止ipset", "ipset", i.banIpSet, "cmd", "ipset flush "+i.banIpSet)
		err = netlink.IpsetFlush(i.banIpSet)
		if err != nil {
			return fmt.Errorf("清空禁止ipset失败: %w", err)
		}
	} else {
		// 创建新的ipset
		slog.Info("创建新的禁止ipset", "ipset", i.banIpSet, "cmd", "ipset create "+i.banIpSet+" hash:net family inet hashsize 1024 maxelem 65536")
		options := netlink.IpsetCreateOptions{
			Replace: true,
		}
		err = netlink.IpsetCreate(i.banIpSet, "hash:net", options)
		if err != nil {
			return fmt.Errorf("创建禁止ipset失败: %w", err)
		}
	}

	// 创建允许IP的ipset
	_, err = netlink.IpsetList(i.allowIpSet)
	if err == nil {
		// ipset已存在，清空它
		slog.Info("清空已存在的允许ipset", "ipset", i.allowIpSet, "cmd", "ipset flush "+i.allowIpSet)
		err = netlink.IpsetFlush(i.allowIpSet)
		if err != nil {
			return fmt.Errorf("清空允许ipset失败: %w", err)
		}
	} else {
		// 创建新的ipset
		slog.Info("创建新的允许ipset", "ipset", i.allowIpSet, "cmd", "ipset create "+i.allowIpSet+" hash:net family inet hashsize 1024 maxelem 65536")
		options := netlink.IpsetCreateOptions{
			Replace: true,
		}
		err = netlink.IpsetCreate(i.allowIpSet, "hash:net", options)
		if err != nil {
			return fmt.Errorf("创建允许ipset失败: %w", err)
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
	i.ipt = ipt

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

	// 添加允许IP的规则到自定义链（优先级最高）
	// iptables -A <chain> -m set --match-set <allow_ipset> src -j ACCEPT
	slog.Info("添加允许ipset规则到iptables", "cmd", "iptables -A "+i.chain+" -m set --match-set "+i.allowIpSet+" src -j ACCEPT")
	err = ipt.AppendUnique("filter", i.chain, "-m", "set", "--match-set", i.allowIpSet, "src", "-j", "ACCEPT")
	if err != nil {
		return fmt.Errorf("添加允许ipset规则到iptables失败: %w", err)
	}

	// 添加禁止IP的规则到自定义链（优先级较低）
	// iptables -A <chain> -m set --match-set <ban_ipset> src -j DROP
	slog.Info("添加禁止ipset规则到iptables", "cmd", "iptables -A "+i.chain+" -m set --match-set "+i.banIpSet+" src -j DROP")
	err = ipt.AppendUnique("filter", i.chain, "-m", "set", "--match-set", i.banIpSet, "src", "-j", "DROP")
	if err != nil {
		return fmt.Errorf("添加禁止ipset规则到iptables失败: %w", err)
	}

	return nil
}

func (i *IpSetFirewallCore) Ban(ipOrCidr string) error {
	return i.addToBanRules(ipOrCidr)
}

func (i *IpSetFirewallCore) RevertBan(ipOrCidr string) error {
	return i.removeFromBanRules(ipOrCidr)
}

func (i *IpSetFirewallCore) Allow(ipOrCidr string) error {
	return i.addToAllowRules(ipOrCidr)
}

func (i *IpSetFirewallCore) RevertAllow(ipOrCidr string) error {
	return i.removeFromAllowRules(ipOrCidr)
}

func (i *IpSetFirewallCore) CleanupIpNetRules(ipOrCidr string) error {
	// 先尝试从禁止ipset中删除

	var errs []error
	err := i.removeFromBanRules(ipOrCidr)
	if err != nil {
		// 如果IP不存在，则认为成功
		if !strings.Contains(err.Error(), "not found") {
			errs = append(errs, err)
		}
	}

	// 再尝试从允许ipset中删除
	err = i.removeFromAllowRules(ipOrCidr)
	if err != nil {
		// 如果IP不存在，则认为成功
		if !strings.Contains(err.Error(), "not found") {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (i *IpSetFirewallCore) addToBanRules(ipOrCidr string) error {
	// 检查是否为特殊地址 0.0.0.0/0
	if ipOrCidr == "0.0.0.0/0" {
		slog.Info("检测到特殊地址0.0.0.0/0，使用iptables规则", "ip", ipOrCidr, "cmd", "iptables -A "+i.chain+" -s "+ipOrCidr+" -j DROP")
		err := i.ipt.AppendUnique("filter", i.chain, "-s", ipOrCidr, "-j", "DROP")
		if err != nil {
			return fmt.Errorf("添加特殊地址iptables规则失败: %w", err)
		}
		return nil
	}

	// 解析IP或CIDR
	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("添加到禁止ipset", "ip", entry.IP, "cidr", entry.CIDR, "cmd", "ipset add "+i.banIpSet+" "+ipOrCidr)
	err = netlink.IpsetAdd(i.banIpSet, entry)
	// 如果ipset中已存在，则返回成功
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	if err != nil {
		return fmt.Errorf("添加到禁止ipset失败: %w", err)
	}

	return nil
}

func (i *IpSetFirewallCore) removeFromBanRules(ipOrCidr string) error {
	// 检查是否为特殊地址 0.0.0.0/0
	if ipOrCidr == "0.0.0.0/0" {
		slog.Info("检测到特殊地址0.0.0.0/0，删除iptables规则", "ip", ipOrCidr, "cmd", "iptables -D "+i.chain+" -s "+ipOrCidr+" -j DROP")
		err := i.ipt.Delete("filter", i.chain, "-s", ipOrCidr, "-j", "DROP")
		if err != nil {
			// 如果规则不存在，则返回成功（幂等操作）
			if strings.Contains(err.Error(), "Bad rule") {
				slog.Info("特殊地址iptables规则不存在", "ip", ipOrCidr)
				return nil
			}
			return fmt.Errorf("删除特殊地址iptables规则失败: %w", err)
		}
		return nil
	}

	// 解析IP或CIDR
	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("从禁止ipset中删除", "ip", entry.IP, "cidr", entry.CIDR, "cmd", "ipset del "+i.banIpSet+" "+ipOrCidr)
	err = netlink.IpsetDel(i.banIpSet, entry)
	// 如果ipset中不存在，则返回成功（幂等操作）
	if err != nil {
		errStr := err.Error()
		// 检查各种可能的"不存在"错误
		if strings.Contains(errStr, "exis") { // 处理截断的错误信息
			slog.Info("IP不存在于禁止ipset中", "ip", ipOrCidr, "error", errStr)
			return nil
		}
		return fmt.Errorf("从禁止ipset中删除失败: %w", err)
	}

	return nil
}

func (i *IpSetFirewallCore) addToAllowRules(ipOrCidr string) error {
	// 检查是否为特殊地址 0.0.0.0/0
	if ipOrCidr == "0.0.0.0/0" {
		slog.Info("检测到特殊地址0.0.0.0/0，使用iptables规则", "ip", ipOrCidr, "cmd", "iptables -I "+i.chain+" 1 -s "+ipOrCidr+" -j ACCEPT")
		err := i.ipt.Insert("filter", i.chain, 1, "-s", ipOrCidr, "-j", "ACCEPT")
		if err != nil {
			return fmt.Errorf("添加特殊地址iptables规则失败: %w", err)
		}
		return nil
	}

	// 解析IP或CIDR
	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("添加到允许ipset", "ip", entry.IP, "cidr", entry.CIDR, "cmd", "ipset add "+i.allowIpSet+" "+ipOrCidr)
	err = netlink.IpsetAdd(i.allowIpSet, entry)
	// 如果ipset中已存在，则返回成功
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	if err != nil {
		return fmt.Errorf("添加到允许ipset失败: %w", err)
	}

	return nil
}

func (i *IpSetFirewallCore) removeFromAllowRules(ipOrCidr string) error {
	// 检查是否为特殊地址 0.0.0.0/0
	if ipOrCidr == "0.0.0.0/0" {
		slog.Info("检测到特殊地址0.0.0.0/0，删除iptables规则", "ip", ipOrCidr, "cmd", "iptables -D "+i.chain+" -s "+ipOrCidr+" -j ACCEPT")
		err := i.ipt.Delete("filter", i.chain, "-s", ipOrCidr, "-j", "ACCEPT")
		if err != nil {
			// 如果规则不存在，则返回成功（幂等操作）
			if strings.Contains(err.Error(), "Bad rule") {
				slog.Info("特殊地址iptables规则不存在", "ip", ipOrCidr)
				return nil
			}
			return fmt.Errorf("删除特殊地址iptables规则失败: %w", err)
		}
		return nil
	}

	// 解析IP或CIDR
	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("从允许ipset中删除", "ip", entry.IP, "cidr", entry.CIDR, "cmd", "ipset del "+i.allowIpSet+" "+ipOrCidr)
	err = netlink.IpsetDel(i.allowIpSet, entry)
	// 如果ipset中不存在，则返回成功（幂等操作）
	if err != nil {
		errStr := err.Error()
		// 检查各种可能的"不存在"错误
		if strings.Contains(errStr, "exis") { // 处理截断的错误信息
			slog.Info("IP不存在于允许ipset中", "ip", ipOrCidr, "error", errStr)
			return nil
		}
		return fmt.Errorf("从允许ipset中删除失败: %w", err)
	}

	return nil
}

func (i *IpSetFirewallCore) CleanupRules() error {
	slog.Info("清理ipset防火墙规则")

	// 先清理iptables规则，避免ipset被引用
	if i.ipt == nil {
		ipt, err := iptables.New()
		if err != nil {
			slog.Error("创建iptables实例失败", "error", err)
			return err
		}
		i.ipt = ipt
	}

	// 从INPUT链移除所有指向自定义链的规则
	for {
		err := i.ipt.Delete("filter", "INPUT", "-j", i.chain)
		if err != nil {
			break
		}
		slog.Info("清除自定义链的规则", "cmd", "iptables -D INPUT -j "+i.chain)
	}

	// 清空自定义链中的所有规则
	slog.Info("清空自定义链中的所有规则", "cmd", "iptables -F "+i.chain)
	_ = i.ipt.ClearChain("filter", i.chain)

	// 删除自定义链
	slog.Info("删除自定义链", "cmd", "iptables -X "+i.chain)
	_ = i.ipt.DeleteChain("filter", i.chain)

	// 清空禁止ipset
	slog.Info("清空禁止ipset", "ipset", i.banIpSet, "cmd", "ipset flush "+i.banIpSet)
	err := netlink.IpsetFlush(i.banIpSet)
	if err != nil {
		slog.Error("清空禁止ipset失败", "error", err)
	}

	// 删除禁止ipset
	slog.Info("删除禁止ipset", "ipset", i.banIpSet, "cmd", "ipset destroy "+i.banIpSet)
	err = netlink.IpsetDestroy(i.banIpSet)
	if err != nil {
		slog.Error("删除禁止ipset失败", "error", err)
	}

	// 清空允许ipset
	slog.Info("清空允许ipset", "ipset", i.allowIpSet, "cmd", "ipset flush "+i.allowIpSet)
	err = netlink.IpsetFlush(i.allowIpSet)
	if err != nil {
		slog.Error("清空允许ipset失败", "error", err)
	}

	// 删除允许ipset
	slog.Info("删除允许ipset", "ipset", i.allowIpSet, "cmd", "ipset destroy "+i.allowIpSet)
	err = netlink.IpsetDestroy(i.allowIpSet)
	if err != nil {
		slog.Error("删除允许ipset失败", "error", err)
	}

	return nil
}

func buildIpSetEntry(ipOrCidr string) (*netlink.IPSetEntry, error) {
	// 解析IP或CIDR
	_, ipNet, err := net.ParseCIDR(ipOrCidr)
	if err != nil {
		// 如果不是CIDR格式，尝试解析为单个IP
		parsedIP := net.ParseIP(ipOrCidr)
		if parsedIP == nil {
			return nil, fmt.Errorf("无效的IP或CIDR格式: %s", ipOrCidr)
		}
		// 单个IP转换为/32或/128 CIDR
		if parsedIP.To4() != nil {
			ipNet = &net.IPNet{IP: parsedIP, Mask: net.CIDRMask(32, 32)}
		} else {
			ipNet = &net.IPNet{IP: parsedIP, Mask: net.CIDRMask(128, 128)}
		}
	}

	// 计算CIDR前缀长度
	ones, _ := ipNet.Mask.Size()
	cidr := uint8(ones)
	if cidr == 0 {
		cidr = 0
		ipNet.IP = net.IPv4zero
	}

	entry := &netlink.IPSetEntry{
		IP:   ipNet.IP,
		CIDR: cidr,
	}
	return entry, nil
}
