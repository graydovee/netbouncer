package core

import (
	"strings"
)

// iptables / ipset 的底层工具不提供类型化错误，只能通过可执行文件输出或
// netlink 的 ACK 消息判断结果。这里集中维护对"规则不存在"与"规则已存在"
// 两类错误词的匹配，避免散落在各实现中。
// 注意：内核/netlink 可能截断错误消息（例如 "IP set ... doesn't exist" 被
// 截为 "... exis"），因此匹配子串而非完整短语。

// isNotExistErr 判断错误是否表示规则/元素不存在
func isNotExistErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, kw := range []string{
		"Bad rule",        // iptables -D 规则不存在
		"not found",       // ipset/iptables 通用
		"No such file",    // netlink
		"element missing", // ipset 元素不存在
		"exis",            // 内核截断后的 "doesn't exist" / "is not exist"
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

// isAlreadyExistsErr 判断错误是否表示规则/元素已存在
func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, kw := range []string{
		"already exists", // iptables AppendUnique 之外的重复
		"already in set", // ipset 元素已存在
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}
