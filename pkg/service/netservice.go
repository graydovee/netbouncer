package service

import (
	"fmt"
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

type NetService struct {
	monitor  *core.Monitor
	firewall *core.Firewall
	ipStore  *store.IpStore
}

func NewNetService(monitor *core.Monitor, firewall *core.Firewall, ipStore *store.IpStore) *NetService {
	svc := &NetService{
		monitor:  monitor,
		firewall: firewall,
		ipStore:  ipStore,
	}

	return svc
}

// Init 初始化服务
func (s *NetService) Init() error {
	// 确保存在默认组
	_, err := s.ipStore.GetDefaultGroup()
	if err != nil {
		// 如果没有默认组，创建一个
		defaultGroup, err := s.ipStore.CreateGroup("默认组", "系统默认的IP禁用组")
		if err != nil {
			return fmt.Errorf("创建默认组失败: %w", err)
		}
		// 设置为默认组
		err = s.ipStore.SetDefaultGroup(defaultGroup.ID)
		if err != nil {
			return fmt.Errorf("设置默认组失败: %w", err)
		}
	}

	// 从存储中加载所有已存在的IP
	ips, err := s.ipStore.GetBlacklist()
	if err != nil {
		return err
	}

	// 提取IP地址列表
	ipList := make([]string, len(ips))
	for i, ip := range ips {
		ipList[i] = ip.IpNet
	}

	// 初始化防火墙
	err = s.firewall.Init(ipList)
	if err != nil {
		return err
	}

	return nil
}

// GetAllStats 获取所有IP的流量统计
func (s *NetService) GetAllStats() ([]TrafficData, error) {
	stats := s.monitor.GetAllStats()
	banedIpNets, err := s.ipStore.GetBlacklist()
	if err != nil {
		return nil, err
	}

	trafficData := make([]TrafficData, 0, len(stats))

	ipNets := convertToIpNet(banedIpNets...)

	for _, stat := range stats {
		isBanned := isContainIpNet(ipNets, stat.RemoteIP)

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
			IsBanned:        isBanned,
		})
	}
	return trafficData, nil
}

// GetStats 获取过滤后的IP流量统计
func (s *NetService) GetStats() ([]TrafficData, error) {
	stats := s.monitor.GetStats()
	trafficData := make([]TrafficData, 0, len(stats))

	banedIpNets, err := s.ipStore.GetBlacklist()
	if err != nil {
		return nil, err
	}

	ipNets := convertToIpNet(banedIpNets...)

	for _, stat := range stats {
		isBanned := isContainIpNet(ipNets, stat.RemoteIP)

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
			IsBanned:        isBanned,
		})
	}
	return trafficData, nil
}

// BanIpNet 封禁IP，同时更新防火墙规则和存储
func (s *NetService) BanIpNet(ipNet string, groupId uint) error {
	// 检查IP是否已经在黑名单中
	if s.IsInBlacklist(ipNet) {
		return nil
	}

	// 如果没有指定组ID，使用默认组
	if groupId == 0 {
		defaultGroup, err := s.ipStore.GetDefaultGroup()
		if err != nil {
			// 如果没有默认组，创建一个
			defaultGroup, err = s.ipStore.CreateGroup("默认组", "系统默认的IP禁用组")
			if err != nil {
				return fmt.Errorf("创建默认组失败: %w", err)
			}
			// 设置为默认组
			err = s.ipStore.SetDefaultGroup(defaultGroup.ID)
			if err != nil {
				return fmt.Errorf("设置默认组失败: %w", err)
			}
		}
		groupId = defaultGroup.ID
	} else {
		// 检查指定的组是否存在
		group, err := s.ipStore.GetGroup(groupId)
		if err != nil || group == nil {
			return fmt.Errorf("指定的组不存在: %w", err)
		}
	}

	// 添加到防火墙规则
	err := s.firewall.Ban(ipNet)
	if err != nil {
		return err
	}

	// 添加到存储
	err = s.ipStore.AddIpBlacklist(ipNet, groupId)
	if err != nil {
		// 如果存储失败，需要从防火墙规则中删除
		_ = s.firewall.Unban(ipNet)
		return err
	}

	return nil
}

// BanIpNets 批量ban多个IP地址或网段
func (s *NetService) BanIpNets(ipNets []string, groupId uint) error {
	for _, ipNet := range ipNets {
		if err := s.BanIpNet(ipNet, groupId); err != nil {
			return err
		}
	}
	return nil
}

// UnbanIpNet 解封IP，同时从防火墙规则和存储中删除
func (s *NetService) UnbanIpNet(ipNet string) error {
	// 检查IP是否在黑名单中
	if !s.IsInBlacklist(ipNet) {
		return nil
	}

	// 从防火墙规则中删除
	err := s.firewall.Unban(ipNet)
	if err != nil {
		return err
	}

	// 从存储中删除
	err = s.ipStore.RemoveIpBlacklist(ipNet)
	if err != nil {
		// 如果存储删除失败，需要重新添加到防火墙规则
		_ = s.firewall.Ban(ipNet)
		return err
	}

	return nil
}

// IsInBlacklist 检查IP是否被封禁
func (s *NetService) IsInBlacklist(ip string) bool {
	return s.ipStore.IsInBlacklist(ip)
}

// GetBannedIPs 获取所有被封禁的IP列表
func (s *NetService) GetBannedIPs() ([]BannedIpNet, error) {
	groups, err := s.ipStore.GetAllGroups()
	if err != nil {
		return nil, err
	}

	groupMap := make(map[uint]*BannedIpGroup)
	for _, group := range groups {
		g := convertToIpNetGroup(&group)
		groupMap[group.ID] = &g
	}

	ips, err := s.ipStore.GetBlacklist()
	if err != nil {
		return nil, err
	}

	ipNets := make([]BannedIpNet, 0, len(ips))
	for _, ip := range ips {
		ipNets = append(ipNets, BannedIpNet{
			IpNet:     ip.IpNet,
			CreatedAt: ip.CreatedAt.Format(time.RFC3339),
			UpdatedAt: ip.UpdatedAt.Format(time.RFC3339),
			Group:     groupMap[ip.GroupID],
		})
	}
	return ipNets, nil
}

func (s *NetService) GetBannedIPsByGroup(groupId uint) ([]BannedIpNet, error) {
	group, err := s.ipStore.GetGroup(groupId)
	if err != nil {
		return nil, err
	}

	ips, err := s.ipStore.GetBlacklistByGroup(groupId)
	if err != nil {
		return nil, err
	}

	g := convertToIpNetGroup(group)

	ipNets := make([]BannedIpNet, 0, len(ips))
	for _, ip := range ips {
		ipNets = append(ipNets, BannedIpNet{
			IpNet:     ip.IpNet,
			CreatedAt: ip.CreatedAt.Format(time.RFC3339),
			UpdatedAt: ip.UpdatedAt.Format(time.RFC3339),
			Group:     &g,
		})
	}
	return ipNets, nil
}

func (s *NetService) GetGroups() ([]BannedIpGroup, error) {
	groups, err := s.ipStore.GetAllGroups()
	if err != nil {
		return nil, err
	}

	groupList := make([]BannedIpGroup, 0, len(groups))
	for _, group := range groups {
		groupList = append(groupList, convertToIpNetGroup(&group))
	}
	return groupList, nil
}

func (s *NetService) CreateGroup(name string, description string) (BannedIpGroup, error) {
	group, err := s.ipStore.CreateGroup(name, description)
	if err != nil {
		return BannedIpGroup{}, err
	}
	return convertToIpNetGroup(group), nil
}

func (s *NetService) UpdateGroup(id uint, name string, description string) (BannedIpGroup, error) {
	group, err := s.ipStore.UpdateGroup(id, name, description)
	if err != nil {
		return BannedIpGroup{}, err
	}
	return convertToIpNetGroup(group), nil
}

func (s *NetService) DeleteGroup(id uint) error {
	//删除组后，所属组的ip会自动归到default group
	defaultGroup, err := s.ipStore.GetDefaultGroup()
	if err != nil {
		return err
	}

	ips, err := s.ipStore.GetBlacklistByGroup(id)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		err = s.ipStore.UpdateIpGroup(ip.IpNet, defaultGroup.ID)
		if err != nil {
			return err
		}
	}

	return s.ipStore.DeleteGroup(id)
}
