package core

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/coreos/go-iptables/iptables"
)

// IptablesFirewallCore 实现iptables防火墙的核心操作
type IptablesFirewallCore struct {
	ipt   *iptables.IPTables
	chain string
}

func (i *IptablesFirewallCore) InitRules() error {
	slog.Info("初始化iptables规则")
	ipt, err := iptables.New()
	if err != nil {
		return err
	}
	i.ipt = ipt

	return ensureChainAndJump(ipt, i.chain)
}

func (i *IptablesFirewallCore) Ban(ipNet string) error {
	return i.addToBanRules(ipNet)
}

func (i *IptablesFirewallCore) RevertBan(ipNet string) error {
	return i.removeFromBanRules(ipNet)
}

func (i *IptablesFirewallCore) Allow(ipNet string) error {
	return i.addToAllowRules(ipNet)
}

func (i *IptablesFirewallCore) RevertAllow(ipNet string) error {
	return i.removeFromAllowRules(ipNet)
}

func (i *IptablesFirewallCore) CleanupIpNetRules(ipNet string) error {
	// 先尝试删除禁止规则，再尝试删除允许规则；规则不存在视为成功
	var errs []error
	if err := i.removeFromBanRules(ipNet); err != nil && !isNotExistErr(err) {
		errs = append(errs, err)
	}
	if err := i.removeFromAllowRules(ipNet); err != nil && !isNotExistErr(err) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (i *IptablesFirewallCore) addToBanRules(ipNet string) error {
	slog.Info("添加到iptables规则", "ip", ipNet, "cmd", "iptables -A "+i.chain+" -s "+ipNet+" -j DROP")
	if err := i.ipt.AppendUnique("filter", i.chain, "-s", ipNet, "-j", "DROP"); err != nil {
		// AppendUnique 已保证幂等性，规则已存在视为成功
		if isAlreadyExistsErr(err) {
			return nil
		}
		return fmt.Errorf("添加到iptables规则失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) removeFromBanRules(ipNet string) error {
	slog.Info("从iptables规则中删除", "ip", ipNet, "cmd", "iptables -D "+i.chain+" -s "+ipNet+" -j DROP")
	if err := i.ipt.Delete("filter", i.chain, "-s", ipNet, "-j", "DROP"); err != nil {
		// 规则不存在视为成功（幂等操作）
		if isNotExistErr(err) {
			slog.Info("iptables规则不存在", "ip", ipNet)
			return nil
		}
		return fmt.Errorf("从iptables规则中删除失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) addToAllowRules(ipNet string) error {
	slog.Info("添加到iptables允许规则", "ip", ipNet, "cmd", "iptables -I "+i.chain+" 1 -s "+ipNet+" -j ACCEPT")
	if err := i.ipt.Insert("filter", i.chain, 1, "-s", ipNet, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("添加到iptables允许规则失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) removeFromAllowRules(ipNet string) error {
	slog.Info("从iptables允许规则中删除", "ip", ipNet, "cmd", "iptables -D "+i.chain+" -s "+ipNet+" -j ACCEPT")
	if err := i.ipt.Delete("filter", i.chain, "-s", ipNet, "-j", "ACCEPT"); err != nil {
		// 规则不存在视为成功（幂等操作）
		if isNotExistErr(err) {
			slog.Info("iptables允许规则不存在", "ip", ipNet)
			return nil
		}
		return fmt.Errorf("从iptables允许规则中删除失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) CleanupRules() error {
	slog.Info("清理iptables规则")
	if i.ipt == nil {
		ipt, err := iptables.New()
		if err != nil {
			return fmt.Errorf("创建iptables实例失败: %w", err)
		}
		i.ipt = ipt
	}
	return removeChainJumpAndChain(i.ipt, i.chain)
}
