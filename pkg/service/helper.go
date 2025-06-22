package service

import (
	"net"
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
