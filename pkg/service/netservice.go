package service

import (
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

type NetService struct {
	monitor  *core.Monitor
	firewall *core.Firewall
	ipStore  store.IpStore
}

func NewNetService(monitor *core.Monitor, firewall *core.Firewall, ipStore store.IpStore) *NetService {
	return &NetService{
		monitor:  monitor,
		firewall: firewall,
		ipStore:  ipStore,
	}
}

// GetAllStats 获取所有IP的流量统计
func (s *NetService) GetAllStats() []TrafficData {
	stats := s.monitor.GetAllStats()
	trafficData := make([]TrafficData, 0, len(stats))

	for _, stat := range stats {
		trafficData = append(trafficData, TrafficData{
			RemoteIP:        stat.RemoteIP,
			LocalIP:         stat.LocalIP,
			TotalBytesIn:    stat.BytesRecv,
			TotalBytesOut:   stat.BytesSent,
			TotalPacketsIn:  stat.PacketsRecv,
			TotalPacketsOut: stat.PacketsSent,
			BytesInPerSec:   stat.BytesRecvPerSec,
			BytesOutPerSec:  stat.BytesSentPerSec,
			Connections:     stat.Connections,
			FirstSeen:       stat.FirstSeen.Format(time.RFC3339),
			LastSeen:        stat.LastSeen.Format(time.RFC3339),
		})
	}
	return trafficData
}

// GetStats 获取过滤后的IP流量统计
func (s *NetService) GetStats() []TrafficData {
	stats := s.monitor.GetStats()
	trafficData := make([]TrafficData, 0, len(stats))

	for _, stat := range stats {
		trafficData = append(trafficData, TrafficData{
			RemoteIP:        stat.RemoteIP,
			LocalIP:         stat.LocalIP,
			TotalBytesIn:    stat.BytesRecv,
			TotalBytesOut:   stat.BytesSent,
			TotalPacketsIn:  stat.PacketsRecv,
			TotalPacketsOut: stat.PacketsSent,
			BytesInPerSec:   stat.BytesRecvPerSec,
			BytesOutPerSec:  stat.BytesSentPerSec,
			Connections:     stat.Connections,
			FirstSeen:       stat.FirstSeen.Format(time.RFC3339),
			LastSeen:        stat.LastSeen.Format(time.RFC3339),
		})
	}
	return trafficData
}

func (s *NetService) BanIpNet(ipNet string) error {
	return s.firewall.Ban(ipNet)
}

// BanIpNets 批量ban多个IP地址或网段
func (s *NetService) BanIpNets(ipNets []string) error {
	for _, ipNet := range ipNets {
		if err := s.firewall.Ban(ipNet); err != nil {
			return err
		}
	}
	return nil
}

func (s *NetService) UnbanIpNet(ipNet string) error {
	return s.firewall.Unban(ipNet)
}

func (s *NetService) GetBannedIPs() ([]BannedIpNet, error) {
	ips, err := s.ipStore.GetBlacklist()
	if err != nil {
		return nil, err
	}

	ipNets := make([]BannedIpNet, 0, len(ips))
	for _, ip := range ips {
		ipNets = append(ipNets, BannedIpNet{IpNet: ip.IpNet, CreatedAt: ip.CreatedAt.Format(time.RFC3339), UpdatedAt: ip.UpdatedAt.Format(time.RFC3339)})
	}
	return ipNets, nil
}
