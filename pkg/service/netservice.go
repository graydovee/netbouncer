package service

import (
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
)

type NetService struct {
	monitor  *core.Monitor
	firewall core.Firewall
}

func NewNetService(monitor *core.Monitor, firewall core.Firewall) *NetService {
	return &NetService{
		monitor:  monitor,
		firewall: firewall,
	}
}

func (s *NetService) GetAllRemoteIPStats() []TrafficData {
	stats := s.monitor.GetAllRemoteIPStats()
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

func (s *NetService) BanIP(ip string) error {
	return s.firewall.Ban(ip)
}

func (s *NetService) UnbanIP(ip string) error {
	return s.firewall.Unban(ip)
}

func (s *NetService) GetBannedIPs() ([]string, error) {
	return s.firewall.GetBannedIPs()
}
