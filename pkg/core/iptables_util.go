package core

import (
	"fmt"
	"log/slog"
	"slices"

	"github.com/coreos/go-iptables/iptables"
)

// ensureChainAndJump 确保自定义链存在，并确保 INPUT 链中存在指向它的跳转规则。
// iptables 模式与 ipset 模式都需要这段初始化逻辑。
func ensureChainAndJump(ipt *iptables.IPTables, chain string) error {
	// 检查链是否存在，存在则清空，不存在则新建
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
