package core

import (
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/coreos/go-iptables/iptables"
)

// ensureChainAndJump 确保自定义链存在，并确保 INPUT 链中存在指向它的跳转规则。
// iptables 模式与 ipset 模式都需要这段初始化逻辑。
func ensureChainAndJump(ipt *iptables.IPTables, chain string) error {
	// 检查链是否存在，存在则清空，不存在则新建
	if err := ensureChain(ipt, chain); err != nil {
		return err
	}

	// 检查 INPUT 链是否已经包含对自定义链的引用，不存在时在第一位插入
	rules, err := ipt.List("filter", "INPUT")
	if err != nil {
		return fmt.Errorf("列出INPUT链规则失败: %w", err)
	}

	if !slices.Contains(rules, "-j "+chain) {
		slog.Info("初始化自定义链", "cmd", "iptables -I INPUT 1 -j "+chain)
		if err := ipt.Insert("filter", "INPUT", 1, "-j", chain); err != nil {
			return fmt.Errorf("插入INPUT跳转规则失败: %w", err)
		}
	}

	return nil
}

// outChainName 出站链名称
func outChainName(chain string) string {
	return chain + "_OUT"
}

// ensureOutChainAndJump 确保出站链存在并挂载到 OUTPUT 链首位。
// 出站链首条规则放行回环流量，避免本机内部调用被限速/封禁波及。
func ensureOutChainAndJump(ipt *iptables.IPTables, chain string) error {
	outChain := outChainName(chain)

	if err := ensureChain(ipt, outChain); err != nil {
		return err
	}

	// 回环流量直接返回
	rules, err := ipt.List("filter", outChain)
	if err != nil {
		return fmt.Errorf("列出%s链规则失败: %w", outChain, err)
	}
	if !slices.Contains(rules, "-o lo -j RETURN") {
		if err := ipt.AppendUnique("filter", outChain, "-o", "lo", "-j", "RETURN"); err != nil {
			return fmt.Errorf("添加回环放行规则失败: %w", err)
		}
	}

	// 检查 OUTPUT 链是否已经包含对出站链的引用，不存在时在第一位插入
	outputRules, err := ipt.List("filter", "OUTPUT")
	if err != nil {
		return fmt.Errorf("列出OUTPUT链规则失败: %w", err)
	}
	if !slices.Contains(outputRules, "-j "+outChain) {
		slog.Info("初始化出站链", "cmd", "iptables -I OUTPUT 1 -j "+outChain)
		if err := ipt.Insert("filter", "OUTPUT", 1, "-j", outChain); err != nil {
			return fmt.Errorf("插入OUTPUT跳转规则失败: %w", err)
		}
	}

	return nil
}

// ensureChain 确保自定义链存在，存在则清空
func ensureChain(ipt *iptables.IPTables, chain string) error {
	chains, err := ipt.ListChains("filter")
	if err != nil {
		return fmt.Errorf("列出iptables链失败: %w", err)
	}

	if slices.Contains(chains, chain) {
		slog.Info("清空链中的所有规则", "cmd", "iptables -F "+chain)
		if err := ipt.ClearChain("filter", chain); err != nil {
			return fmt.Errorf("清空iptables链失败: %w", err)
		}
	} else {
		slog.Info("创建新的自定义链", "cmd", "iptables -N "+chain)
		if err := ipt.NewChain("filter", chain); err != nil {
			return fmt.Errorf("创建iptables链失败: %w", err)
		}
	}
	return nil
}

// removeChainJumpAndChain 从 INPUT 链移除所有指向自定义链的跳转规则，
// 然后清空并删除自定义链。用于 CleanupRules。
func removeChainJumpAndChain(ipt *iptables.IPTables, chain string) error {
	// 循环删除，直到没有更多匹配的规则
	for {
		err := ipt.Delete("filter", "INPUT", "-j", chain)
		if err != nil {
			if !isNotExistErr(err) {
				return fmt.Errorf("移除INPUT跳转规则失败: %w", err)
			}
			break
		}
		slog.Info("清除自定义链的规则", "cmd", "iptables -D INPUT -j "+chain)
	}

	slog.Info("清空自定义链中的所有规则", "cmd", "iptables -F "+chain)
	if err := ipt.ClearChain("filter", chain); err != nil && !isNotExistErr(err) {
		return fmt.Errorf("清空自定义链失败: %w", err)
	}

	slog.Info("删除自定义链", "cmd", "iptables -X "+chain)
	if err := ipt.DeleteChain("filter", chain); err != nil && !isNotExistErr(err) {
		return fmt.Errorf("删除自定义链失败: %w", err)
	}

	return nil
}

// removeOutChainJumpAndChain 从 OUTPUT 链移除跳转规则并删除出站链
func removeOutChainJumpAndChain(ipt *iptables.IPTables, chain string) error {
	outChain := outChainName(chain)
	for {
		err := ipt.Delete("filter", "OUTPUT", "-j", outChain)
		if err != nil {
			if !isNotExistErr(err) {
				return fmt.Errorf("移除OUTPUT跳转规则失败: %w", err)
			}
			break
		}
		slog.Info("清除出站链的跳转规则", "cmd", "iptables -D OUTPUT -j "+outChain)
	}

	if err := ipt.ClearChain("filter", outChain); err != nil && !isNotExistErr(err) {
		return fmt.Errorf("清空出站链失败: %w", err)
	}
	if err := ipt.DeleteChain("filter", outChain); err != nil && !isNotExistErr(err) {
		return fmt.Errorf("删除出站链失败: %w", err)
	}
	return nil
}

// hashlimitArgs 构建 hashlimit 匹配参数。mode 为 srcip（入站每源IP限速）或 dstip（出站每目标IP限速）
func hashlimitArgs(name string, rule RateLimitRule, mode string) []string {
	rateKB := int64(rule.RateKBps)
	if rateKB < 1 {
		rateKB = 1
	}
	burstKB := int64(rule.BurstKBps)
	if burstKB <= 0 {
		burstKB = rateKB * 2
	}
	return []string{
		"-m", "hashlimit",
		"--hashlimit-name", name,
		"--hashlimit-above", strconv.FormatInt(rateKB, 10) + "kb/second",
		"--hashlimit-burst", strconv.FormatInt(burstKB, 10) + "kb",
		"--hashlimit-mode", mode,
		"--hashlimit-htable-expire", "10000",
	}
}

// rateLimitArgs 构建完整的限速规则参数（含协议/端口匹配与 DROP 目标）
func rateLimitArgs(rule RateLimitRule, mode string) []string {
	args := make([]string, 0, 16)
	if rule.Protocol != "" {
		args = append(args, "-p", rule.Protocol)
	}
	if rule.Port > 0 {
		args = append(args, "--dport", strconv.FormatUint(uint64(rule.Port), 10))
	}
	args = append(args, hashlimitArgs("nb_rl"+strconv.FormatUint(uint64(rule.ID), 10), rule, mode)...)
	args = append(args, "-j", "DROP")
	return args
}

// applyRateLimitRules 按规则的方向把限速规则写入对应链（in=入站链 srcip，out=出站链 dstip）
func applyRateLimitRules(ipt *iptables.IPTables, chain string, rule RateLimitRule) error {
	if rule.Direction == "in" || rule.Direction == "both" {
		args := rateLimitArgs(rule, "srcip")
		slog.Info("添加入站限速规则", "cmd", "iptables -A "+chain, "args", args)
		if err := ipt.AppendUnique("filter", chain, args...); err != nil {
			return fmt.Errorf("添加入站限速规则失败: %w", err)
		}
	}
	if rule.Direction == "out" || rule.Direction == "both" {
		args := rateLimitArgs(rule, "dstip")
		slog.Info("添加出站限速规则", "cmd", "iptables -A "+outChainName(chain), "args", args)
		if err := ipt.AppendUnique("filter", outChainName(chain), args...); err != nil {
			return fmt.Errorf("添加出站限速规则失败: %w", err)
		}
	}
	return nil
}

// removeRateLimitRules 从对应链中按完整参数精确删除限速规则
func removeRateLimitRules(ipt *iptables.IPTables, chain string, rule RateLimitRule) error {
	var errs []error
	if rule.Direction == "in" || rule.Direction == "both" {
		args := rateLimitArgs(rule, "srcip")
		if err := ipt.Delete("filter", chain, args...); err != nil && !isNotExistErr(err) {
			errs = append(errs, fmt.Errorf("删除入站限速规则失败: %w", err))
		}
	}
	if rule.Direction == "out" || rule.Direction == "both" {
		args := rateLimitArgs(rule, "dstip")
		if err := ipt.Delete("filter", outChainName(chain), args...); err != nil && !isNotExistErr(err) {
			errs = append(errs, fmt.Errorf("删除出站限速规则失败: %w", err))
		}
	}
	for _, err := range errs {
		slog.Error("移除限速规则失败", "error", err)
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
