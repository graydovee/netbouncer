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

	if err := ensureChainAndJump(ipt, i.chain); err != nil {
		return err
	}
	return ensureOutChainAndJump(ipt, i.chain)
}

func (i *IptablesFirewallCore) Ban(ipNet string, direction string) error {
	return i.forEachBanChain(direction, func(chain string, outbound bool) error {
		spec := banSpec(ipNet, outbound)
		slog.Info("添加到iptables规则", "ip", ipNet, "cmd", "iptables -A "+chain, "args", spec)
		if err := i.ipt.AppendUnique("filter", chain, spec...); err != nil {
			// AppendUnique 已保证幂等性，规则已存在视为成功
			if isAlreadyExistsErr(err) {
				return nil
			}
			return fmt.Errorf("添加到iptables规则失败: %w", err)
		}
		return nil
	})
}

func (i *IptablesFirewallCore) RevertBan(ipNet string, direction string) error {
	return i.forEachBanChain(direction, func(chain string, outbound bool) error {
		spec := banSpec(ipNet, outbound)
		slog.Info("从iptables规则中删除", "ip", ipNet, "cmd", "iptables -D "+chain, "args", spec)
		if err := i.ipt.Delete("filter", chain, spec...); err != nil {
			// 规则不存在视为成功（幂等操作）
			if isNotExistErr(err) {
				slog.Info("iptables规则不存在", "ip", ipNet)
				return nil
			}
			return fmt.Errorf("从iptables规则中删除失败: %w", err)
		}
		return nil
	})
}

func (i *IptablesFirewallCore) Allow(ipNet string) error {
	var errs []error
	// 入站白名单插入链首位（优先级最高）
	slog.Info("添加到iptables允许规则", "ip", ipNet, "cmd", "iptables -I "+i.chain+" 1 -s "+ipNet+" -j ACCEPT")
	if err := i.ipt.Insert("filter", i.chain, 1, "-s", ipNet, "-j", "ACCEPT"); err != nil {
		errs = append(errs, fmt.Errorf("添加到iptables允许规则失败: %w", err))
	}
	// 出站白名单按目标地址放行，使白名单 IP 不受出站限速/封禁影响
	outChain := outChainName(i.chain)
	slog.Info("添加到iptables出站允许规则", "ip", ipNet, "cmd", "iptables -I "+outChain+" 1 -d "+ipNet+" -j ACCEPT")
	if err := i.ipt.Insert("filter", outChain, 1, "-d", ipNet, "-j", "ACCEPT"); err != nil {
		errs = append(errs, fmt.Errorf("添加到iptables出站允许规则失败: %w", err))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (i *IptablesFirewallCore) RevertAllow(ipNet string) error {
	var errs []error
	if err := i.ipt.Delete("filter", i.chain, "-s", ipNet, "-j", "ACCEPT"); err != nil && !isNotExistErr(err) {
		errs = append(errs, fmt.Errorf("从iptables允许规则中删除失败: %w", err))
	}
	if err := i.ipt.Delete("filter", outChainName(i.chain), "-d", ipNet, "-j", "ACCEPT"); err != nil && !isNotExistErr(err) {
		errs = append(errs, fmt.Errorf("从iptables出站允许规则中删除失败: %w", err))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (i *IptablesFirewallCore) ApplyRateLimit(rule RateLimitRule) error {
	return applyRateLimitRules(i.ipt, i.chain, rule)
}

func (i *IptablesFirewallCore) RemoveRateLimit(rule RateLimitRule) error {
	return removeRateLimitRules(i.ipt, i.chain, rule)
}

// forEachBanChain 按 direction 逐条处理 ban 规则所在的链
func (i *IptablesFirewallCore) forEachBanChain(direction string, fn func(chain string, outbound bool) error) error {
	applyIn := direction == "" || direction == "in" || direction == "both"
	applyOut := direction == "out" || direction == "both"

	var errs []error
	if applyIn {
		if err := fn(i.chain, false); err != nil {
			errs = append(errs, err)
		}
	}
	if applyOut {
		if err := fn(outChainName(i.chain), true); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// banSpec 封禁规则参数：入站按源地址，出站按目标地址
func banSpec(ipNet string, outbound bool) []string {
	if outbound {
		return []string{"-d", ipNet, "-j", "DROP"}
	}
	return []string{"-s", ipNet, "-j", "DROP"}
}

func (i *IptablesFirewallCore) CleanupIpNetRules(ipNet string) error {
	// 先尝试删除禁止规则（双向），再尝试删除允许规则；规则不存在视为成功
	var errs []error
	if err := i.RevertBan(ipNet, "both"); err != nil && !isNotExistErr(err) {
		errs = append(errs, err)
	}
	if err := i.RevertAllow(ipNet); err != nil && !isNotExistErr(err) {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
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
	if err := removeChainJumpAndChain(i.ipt, i.chain); err != nil {
		return err
	}
	return removeOutChainJumpAndChain(i.ipt, i.chain)
}
