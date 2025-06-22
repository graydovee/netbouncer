package core

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

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

	// 检查链是否存在，存在则清空，不存在则新建
	// iptables -L <chain> 检查链是否存在
	chains, err := i.ipt.ListChains("filter")
	if err != nil {
		return err
	}
	if slices.Contains(chains, i.chain) {
		// iptables -F <chain> 清空链中的所有规则
		slog.Info("清空链中的所有规则", "cmd", "iptables -F "+i.chain)
		_ = i.ipt.ClearChain("filter", i.chain)
	} else {
		// iptables -N <chain> 创建新的自定义链
		slog.Info("创建新的自定义链", "cmd", "iptables -N "+i.chain)
		_ = i.ipt.NewChain("filter", i.chain)
	}

	// 检查 INPUT 链是否已经包含对自定义链的引用
	// iptables -L INPUT 列出 INPUT 链的所有规则
	rules, err := i.ipt.List("filter", "INPUT")
	if err != nil {
		return err
	}

	// 查找是否已存在指向自定义链的规则
	ruleExists := slices.Contains(rules, "-j "+i.chain)

	// 只有在规则不存在时才插入
	if !ruleExists {
		// iptables -I INPUT 1 -j <chain> 在 INPUT 链的第1位插入规则，跳转到自定义链
		slog.Info("初始化自定义链", "cmd", "iptables -I INPUT 1 -j "+i.chain)
		_ = i.ipt.Insert("filter", "INPUT", 1, "-j", i.chain)
	}

	return nil
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
	// 先尝试删除禁止规则
	var errs []error
	err := i.removeFromBanRules(ipNet)
	if err != nil {
		// 如果IP不存在，则返回成功
		if !strings.Contains(err.Error(), "not found") {
			// 继续尝试删除允许规则
			errs = append(errs, err)
		}
	}

	// 再尝试删除允许规则
	err = i.removeFromAllowRules(ipNet)
	if err != nil {
		// 如果IP不存在，则返回成功
		if !strings.Contains(err.Error(), "not found") {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (i *IptablesFirewallCore) addToBanRules(ipNet string) error {
	slog.Info("添加到iptables规则", "ip", ipNet, "cmd", "iptables -A "+i.chain+" -s "+ipNet+" -j DROP")
	err := i.ipt.AppendUnique("filter", i.chain, "-s", ipNet, "-j", "DROP")
	// AppendUnique 已经保证了幂等性，如果规则已存在则不会重复添加
	if err != nil {
		return fmt.Errorf("添加到iptables规则失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) removeFromBanRules(ipNet string) error {
	slog.Info("从iptables规则中删除", "ip", ipNet, "cmd", "iptables -D "+i.chain+" -s "+ipNet+" -j DROP")
	err := i.ipt.Delete("filter", i.chain, "-s", ipNet, "-j", "DROP")
	// 如果规则不存在，则返回成功（幂等操作）
	errStr := err.Error()
	if err != nil && strings.Contains(errStr, "Bad rule") {
		slog.Info("iptables规则不存在", "ip", ipNet)
		return nil
	}
	if err != nil {
		return fmt.Errorf("从iptables规则中删除失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) addToAllowRules(ipNet string) error {
	slog.Info("添加到iptables允许规则", "ip", ipNet, "cmd", "iptables -I "+i.chain+" 1 -s "+ipNet+" -j ACCEPT")
	err := i.ipt.Insert("filter", i.chain, 1, "-s", ipNet, "-j", "ACCEPT")
	// Insert 已经保证了幂等性，如果规则已存在则不会重复添加
	if err != nil {
		return fmt.Errorf("添加到iptables允许规则失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) removeFromAllowRules(ipNet string) error {
	slog.Info("从iptables允许规则中删除", "ip", ipNet, "cmd", "iptables -D "+i.chain+" -s "+ipNet+" -j ACCEPT")
	err := i.ipt.Delete("filter", i.chain, "-s", ipNet, "-j", "ACCEPT")
	// 如果规则不存在，则返回成功（幂等操作）
	errStr := err.Error()
	if err != nil && strings.Contains(errStr, "Bad rule") {
		slog.Info("iptables允许规则不存在", "ip", ipNet)
		return nil
	}
	if err != nil {
		return fmt.Errorf("从iptables允许规则中删除失败: %w", err)
	}
	return nil
}

func (i *IptablesFirewallCore) CleanupRules() error {
	slog.Info("清理iptables规则")
	// 清空自定义链中的所有规则
	// iptables -F <chain> 清空链中的所有规则
	slog.Info("清空自定义链中的所有规则", "cmd", "iptables -F "+i.chain)
	_ = i.ipt.ClearChain("filter", i.chain)

	// 从INPUT链移除所有指向自定义链的规则
	// 使用循环删除，直到没有更多匹配的规则
	for {
		// iptables -D INPUT -j <chain> 从 INPUT 链中删除跳转到自定义链的规则
		// 尝试删除规则，如果删除失败说明没有更多匹配的规则
		err := i.ipt.Delete("filter", "INPUT", "-j", i.chain)
		if err != nil {
			// 没有更多匹配的规则，退出循环
			break
		}
		slog.Info("清除自定义链的规则", "cmd", "iptables -D INPUT -j "+i.chain)
	}

	// 删除自定义链
	// iptables -X <chain> 删除自定义链（链必须为空）
	slog.Info("删除自定义链", "cmd", "iptables -X "+i.chain)
	_ = i.ipt.DeleteChain("filter", i.chain)
	return nil
}
