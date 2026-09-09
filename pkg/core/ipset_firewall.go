package core

import (
	"errors"
	"fmt"
	"log/slog"
	"net"

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
	if err := i.createIpSets(); err != nil {
		return fmt.Errorf("创建ipset失败: %w", err)
	}

	// 设置iptables规则
	if err := i.setupIptables(); err != nil {
		return fmt.Errorf("设置iptables规则失败: %w", err)
	}

	return nil
}

// ensureIpSet 确保指定名称的 hash:net ipset 存在，已存在则清空
func ensureIpSet(name string) error {
	if _, err := netlink.IpsetList(name); err == nil {
		slog.Info("清空已存在的ipset", "ipset", name, "cmd", "ipset flush "+name)
		if err := netlink.IpsetFlush(name); err != nil {
			return fmt.Errorf("清空ipset失败: %w", err)
		}
		return nil
	}

	slog.Info("创建新的ipset", "ipset", name, "cmd", "ipset create "+name+" hash:net family inet hashsize 1024 maxelem 65536")
	if err := netlink.IpsetCreate(name, "hash:net", netlink.IpsetCreateOptions{Replace: true}); err != nil {
		return fmt.Errorf("创建ipset失败: %w", err)
	}
	return nil
}

func (i *IpSetFirewallCore) createIpSets() error {
	i.banIpSet = i.ipset + "_ban"
	i.allowIpSet = i.ipset + "_allow"

	if err := ensureIpSet(i.banIpSet); err != nil {
		return err
	}
	return ensureIpSet(i.allowIpSet)
}

func (i *IpSetFirewallCore) setupIptables() error {
	// 使用go-iptables库设置iptables规则
	ipt, err := iptables.New()
	if err != nil {
		return err
	}
	i.ipt = ipt

	if err := ensureChainAndJump(ipt, i.chain); err != nil {
		return err
	}

	// 添加允许IP的规则到自定义链（优先级最高）
	// iptables -A <chain> -m set --match-set <allow_ipset> src -j ACCEPT
	slog.Info("添加允许ipset规则到iptables", "cmd", "iptables -A "+i.chain+" -m set --match-set "+i.allowIpSet+" src -j ACCEPT")
	if err := ipt.AppendUnique("filter", i.chain, "-m", "set", "--match-set", i.allowIpSet, "src", "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("添加允许ipset规则到iptables失败: %w", err)
	}

	// 添加禁止IP的规则到自定义链（优先级较低）
	// iptables -A <chain> -m set --match-set <ban_ipset> src -j DROP
	slog.Info("添加禁止ipset规则到iptables", "cmd", "iptables -A "+i.chain+" -m set --match-set "+i.banIpSet+" src -j DROP")
	if err := ipt.AppendUnique("filter", i.chain, "-m", "set", "--match-set", i.banIpSet, "src", "-j", "DROP"); err != nil {
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
	// 先尝试从禁止ipset中删除，再尝试从允许ipset中删除；元素不存在视为成功
	var errs []error
	if err := i.removeFromBanRules(ipOrCidr); err != nil && !isNotExistErr(err) {
		errs = append(errs, err)
	}
	if err := i.removeFromAllowRules(ipOrCidr); err != nil && !isNotExistErr(err) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// addSpecialRule 处理特殊地址 0.0.0.0/0：ipset 无法表达全零网络，改走 iptables 规则
func (i *IpSetFirewallCore) addSpecialRule(ipOrCidr string, action string) error {
	if action == "ACCEPT" {
		slog.Info("检测到特殊地址0.0.0.0/0，使用iptables规则", "ip", ipOrCidr, "cmd", "iptables -I "+i.chain+" 1 -s "+ipOrCidr+" -j ACCEPT")
		if err := i.ipt.Insert("filter", i.chain, 1, "-s", ipOrCidr, "-j", "ACCEPT"); err != nil {
			return fmt.Errorf("添加特殊地址iptables规则失败: %w", err)
		}
		return nil
	}

	slog.Info("检测到特殊地址0.0.0.0/0，使用iptables规则", "ip", ipOrCidr, "cmd", "iptables -A "+i.chain+" -s "+ipOrCidr+" -j DROP")
	if err := i.ipt.AppendUnique("filter", i.chain, "-s", ipOrCidr, "-j", "DROP"); err != nil {
		return fmt.Errorf("添加特殊地址iptables规则失败: %w", err)
	}
	return nil
}

// removeSpecialRule 删除特殊地址 0.0.0.0/0 对应的 iptables 规则
func (i *IpSetFirewallCore) removeSpecialRule(ipOrCidr string, action string) error {
	slog.Info("检测到特殊地址0.0.0.0/0，删除iptables规则", "ip", ipOrCidr, "cmd", "iptables -D "+i.chain+" -s "+ipOrCidr+" -j "+action)
	if err := i.ipt.Delete("filter", i.chain, "-s", ipOrCidr, "-j", action); err != nil {
		// 规则不存在视为成功（幂等操作）
		if isNotExistErr(err) {
			slog.Info("特殊地址iptables规则不存在", "ip", ipOrCidr)
			return nil
		}
		return fmt.Errorf("删除特殊地址iptables规则失败: %w", err)
	}
	return nil
}

func (i *IpSetFirewallCore) addToBanRules(ipOrCidr string) error {
	if ipOrCidr == "0.0.0.0/0" {
		return i.addSpecialRule(ipOrCidr, "DROP")
	}

	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("添加到禁止ipset", "ip", entry.IP.String(), "cidr", entry.CIDR, "cmd", "ipset add "+i.banIpSet+" "+ipOrCidr)
	if err := netlink.IpsetAdd(i.banIpSet, entry); err != nil {
		// 已存在视为成功
		if isAlreadyExistsErr(err) {
			return nil
		}
		return fmt.Errorf("添加到禁止ipset失败: %w", err)
	}
	return nil
}

func (i *IpSetFirewallCore) removeFromBanRules(ipOrCidr string) error {
	if ipOrCidr == "0.0.0.0/0" {
		return i.removeSpecialRule(ipOrCidr, "DROP")
	}

	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("从禁止ipset中删除", "ip", entry.IP.String(), "cidr", entry.CIDR, "cmd", "ipset del "+i.banIpSet+" "+ipOrCidr)
	if err := netlink.IpsetDel(i.banIpSet, entry); err != nil {
		// 不存在视为成功（幂等操作）
		if isNotExistErr(err) {
			slog.Info("IP不存在于禁止ipset中", "ip", ipOrCidr)
			return nil
		}
		return fmt.Errorf("从禁止ipset中删除失败: %w", err)
	}
	return nil
}

func (i *IpSetFirewallCore) addToAllowRules(ipOrCidr string) error {
	if ipOrCidr == "0.0.0.0/0" {
		return i.addSpecialRule(ipOrCidr, "ACCEPT")
	}

	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("添加到允许ipset", "ip", entry.IP.String(), "cidr", entry.CIDR, "cmd", "ipset add "+i.allowIpSet+" "+ipOrCidr)
	if err := netlink.IpsetAdd(i.allowIpSet, entry); err != nil {
		// 已存在视为成功
		if isAlreadyExistsErr(err) {
			return nil
		}
		return fmt.Errorf("添加到允许ipset失败: %w", err)
	}
	return nil
}

func (i *IpSetFirewallCore) removeFromAllowRules(ipOrCidr string) error {
	if ipOrCidr == "0.0.0.0/0" {
		return i.removeSpecialRule(ipOrCidr, "ACCEPT")
	}

	entry, err := buildIpSetEntry(ipOrCidr)
	if err != nil {
		return err
	}

	slog.Info("从允许ipset中删除", "ip", entry.IP.String(), "cidr", entry.CIDR, "cmd", "ipset del "+i.allowIpSet+" "+ipOrCidr)
	if err := netlink.IpsetDel(i.allowIpSet, entry); err != nil {
		// 不存在视为成功（幂等操作）
		if isNotExistErr(err) {
			slog.Info("IP不存在于允许ipset中", "ip", ipOrCidr)
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
			return fmt.Errorf("创建iptables实例失败: %w", err)
		}
		i.ipt = ipt
	}

	if err := removeChainJumpAndChain(i.ipt, i.chain); err != nil {
		slog.Error("清理iptables链失败", "error", err)
	}

	// 清空并删除两个ipset
	for _, name := range []string{i.banIpSet, i.allowIpSet} {
		slog.Info("清空ipset", "ipset", name, "cmd", "ipset flush "+name)
		if err := netlink.IpsetFlush(name); err != nil {
			slog.Error("清空ipset失败", "ipset", name, "error", err)
		}

		slog.Info("删除ipset", "ipset", name, "cmd", "ipset destroy "+name)
		if err := netlink.IpsetDestroy(name); err != nil {
			slog.Error("删除ipset失败", "ipset", name, "error", err)
		}
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
		ipNet.IP = net.IPv4zero
	}

	entry := &netlink.IPSetEntry{
		IP:   ipNet.IP,
		CIDR: cidr,
	}
	return entry, nil
}
