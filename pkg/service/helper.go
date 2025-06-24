package service

import (
	"net"
	"regexp"
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

func parseIpNet(ipNet string) *net.IPNet {
	// 首先尝试解析为IP地址
	if ip := net.ParseIP(ipNet); ip != nil {
		return &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(32, 32),
		}
	}

	// 如果不是单个IP，尝试解析为CIDR格式
	if _, ipnet, err := net.ParseCIDR(ipNet); err == nil {
		return ipnet
	}

	return nil
}

func convertToIpNet(ipNet ...store.IpNet) []*net.IPNet {
	ipNets := make([]*net.IPNet, 0, len(ipNet))
	for _, ip := range ipNet {
		ipnet := parseIpNet(ip.IpNet)
		if ipnet != nil {
			ipNets = append(ipNets, ipnet)
		}
	}
	return ipNets
}

func convertToIpNetGroup(storeGroup *store.IpNetGroup) IpGroup {
	return IpGroup{
		ID:          storeGroup.ID,
		Name:        storeGroup.Name,
		Description: storeGroup.Description,
		CreatedAt:   storeGroup.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   storeGroup.UpdatedAt.Format(time.RFC3339),
		IsDefault:   storeGroup.IsDefault,
	}
}

func isContainIpNet(ipNet []*net.IPNet, ip string) bool {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return false
	}

	for _, ipnet := range ipNet {
		if ipnet.Contains(ipAddr) {
			return true
		}
	}
	return false
}

func IsBanned(bannedIpNets, allowIpNets []*net.IPNet, ip string) bool {
	if isContainIpNet(allowIpNets, ip) {
		return false
	}

	if isContainIpNet(bannedIpNets, ip) {
		return true
	}

	return false
}

var ipOrCidrRex = regexp.MustCompile(`(?:(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])\.){3}(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])(?:\/(?:[0-9]|[12][0-9]|3[0-2]))?\b`)

func extractIPsAndCIDRs(text string) []string {
	// 正则表达式说明：
	// 1. \b 匹配单词边界，确保匹配独立的IP或CIDR
	// 2. IP部分：(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])
	//    - 匹配 0-255 的数字，禁止前导零
	// 3. CIDR部分：(?:\/(?:[0-9]|[12][0-9]|3[0-2]))?
	//    - 可选的后缀，匹配 /0 到 /32
	return ipOrCidrRex.FindAllString(text, -1)
}
