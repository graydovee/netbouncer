package service

import (
	"fmt"
	"net"
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

type NetService struct {
	monitor  *core.Monitor
	firewall *core.Firewall

	store *store.Store
}

func NewNetService(monitor *core.Monitor, firewall *core.Firewall, store *store.Store) *NetService {
	svc := &NetService{
		monitor:  monitor,
		firewall: firewall,
		store:    store,
	}

	return svc
}

// Init 初始化服务
func (s *NetService) Init() error {
	// 确保存在默认组
	_, err := s.store.IpNetGroupStore.FindDefault()
	if err != nil {
		// 如果没有默认组，创建一个
		defaultGroup, err := s.store.IpNetGroupStore.Create("默认组", "系统默认的IP禁用组")
		if err != nil {
			return fmt.Errorf("创建默认组失败: %w", err)
		}
		// 设置为默认组
		err = s.store.IpNetGroupStore.SetDefault(defaultGroup.ID)
		if err != nil {
			return fmt.Errorf("设置默认组失败: %w", err)
		}
	}

	// 从存储中加载所有已存在的IP
	ips, err := s.store.IpNetStore.FindAll()
	if err != nil {
		return err
	}

	// 初始化防火墙
	err = s.firewall.Init(ips)
	if err != nil {
		return err
	}

	return nil
}

// GetAllStats 获取所有IP的流量统计
func (s *NetService) GetAllStats() ([]TrafficData, error) {
	stats := s.monitor.GetAllStats()
	trafficData := make([]TrafficData, 0, len(stats))

	ipNets, err := s.store.IpNetStore.FindByAction(store.ActionBan)
	if err != nil {
		return nil, err
	}

	banedIpNets := make([]*net.IPNet, 0)
	for _, ip := range ipNets {
		if ip.Action == store.ActionBan {
			banedIpNets = append(banedIpNets, convertToIpNet(ip)...)
		}
	}

	for _, stat := range stats {
		isBanned := isContainIpNet(banedIpNets, stat.RemoteIP)

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

	ipNets, err := s.store.IpNetStore.FindByAction(store.ActionBan)
	if err != nil {
		return nil, err
	}

	banedIpNets := make([]*net.IPNet, 0)
	for _, ip := range ipNets {
		if ip.Action == store.ActionBan {
			banedIpNets = append(banedIpNets, convertToIpNet(ip)...)
		}
	}

	for _, stat := range stats {
		isBanned := isContainIpNet(banedIpNets, stat.RemoteIP)

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

func (s *NetService) CreateIpNet(ipnet string, groupId uint, action string) error {

	// 如果没有指定组ID，使用默认组
	if groupId == 0 {
		defaultGroup, err := s.store.IpNetGroupStore.FindDefault()
		if err != nil {
			// 如果没有默认组，创建一个
			defaultGroup, err = s.store.IpNetGroupStore.Create("默认组", "系统默认的IP禁用组")
			if err != nil {
				return fmt.Errorf("创建默认组失败: %w", err)
			}
			// 设置为默认组
			err = s.store.IpNetGroupStore.SetDefault(defaultGroup.ID)
			if err != nil {
				return fmt.Errorf("设置默认组失败: %w", err)
			}
		}
		groupId = defaultGroup.ID
	} else {
		// 检查指定的组是否存在
		group, err := s.store.IpNetGroupStore.FindByID(groupId)
		if err != nil || group == nil {
			return fmt.Errorf("指定的组不存在: %w", err)
		}
	}

	if s.store.IpNetStore.ExistsByIpNet(ipnet) {
		// 如果IP网络已存在，则更新action
		ipNet, err := s.store.IpNetStore.FindByIpNet(ipnet)
		if err != nil {
			return err
		}

		err = s.UpdateIpNetAction(ipNet.ID, action)
		if err != nil {
			return err
		}

		return nil
	}

	// 创建IP网络记录
	ipNet, err := s.store.IpNetStore.Create(ipnet, groupId, action)
	if err != nil {
		return err
	}

	err = s.firewall.SetAction(ipNet.IpNet, action)
	if err != nil {
		return err
	}

	return nil
}

func (s *NetService) DeleteIpNet(id uint) error {
	ipNet, err := s.store.IpNetStore.FindByID(id)
	if err != nil {
		return err
	}

	err = s.firewall.CleanupIpNet(ipNet.IpNet)
	if err != nil {
		return fmt.Errorf("撤销原有行为失败: %w", err)
	}

	err = s.store.IpNetStore.DeleteByID(id)
	if err != nil {
		return fmt.Errorf("删除IP网络失败: %w", err)
	}

	return nil
}

func (s *NetService) UpdateIpNetAction(id uint, action string) error {
	ipNet, err := s.store.IpNetStore.FindByID(id)
	if err != nil {
		return err
	}

	if action == ipNet.Action {
		return nil
	}

	err = s.firewall.SetAction(ipNet.IpNet, action)
	if err != nil {
		return fmt.Errorf("应用新行为失败: %w", err)
	}

	err = s.store.IpNetStore.UpdateAction(id, action)
	if err != nil {
		return fmt.Errorf("更新IP行为失败: %w", err)
	}

	return nil
}

// ListAllIpNets 获取所有IP列表
func (s *NetService) ListAllIpNets() ([]IpNet, error) {
	groups, err := s.store.IpNetGroupStore.FindAll()
	if err != nil {
		return nil, err
	}

	groupMap := make(map[uint]*IpGroup)
	for _, group := range groups {
		g := convertToIpNetGroup(&group)
		groupMap[group.ID] = &g
	}

	ips, err := s.store.IpNetStore.FindAll()
	if err != nil {
		return nil, err
	}

	ipNets := make([]IpNet, 0, len(ips))
	for _, ip := range ips {
		ipNets = append(ipNets, IpNet{
			ID:        ip.ID,
			IpNet:     ip.IpNet,
			CreatedAt: ip.CreatedAt.Format(time.RFC3339),
			UpdatedAt: ip.UpdatedAt.Format(time.RFC3339),
			Group:     groupMap[ip.GroupID],
			Action:    ip.Action,
		})
	}
	return ipNets, nil
}

func (s *NetService) ListIpNetsByGroup(groupId uint) ([]IpNet, error) {
	group, err := s.store.IpNetGroupStore.FindByID(groupId)
	if err != nil {
		return nil, err
	}

	ips, err := s.store.IpNetStore.FindByGroupID(groupId)
	if err != nil {
		return nil, err
	}

	g := convertToIpNetGroup(group)

	ipNets := make([]IpNet, 0, len(ips))
	for _, ip := range ips {
		ipNets = append(ipNets, IpNet{
			ID:        ip.ID,
			IpNet:     ip.IpNet,
			CreatedAt: ip.CreatedAt.Format(time.RFC3339),
			UpdatedAt: ip.UpdatedAt.Format(time.RFC3339),
			Group:     &g,
			Action:    ip.Action,
		})
	}
	return ipNets, nil
}

func (s *NetService) ListAllGroups() ([]IpGroup, error) {
	groups, err := s.store.IpNetGroupStore.FindAll()
	if err != nil {
		return nil, err
	}

	groupList := make([]IpGroup, 0, len(groups))
	for _, group := range groups {
		groupList = append(groupList, convertToIpNetGroup(&group))
	}
	return groupList, nil
}

func (s *NetService) CreateGroup(name string, description string) (IpGroup, error) {
	group, err := s.store.IpNetGroupStore.Create(name, description)
	if err != nil {
		return IpGroup{}, err
	}
	return convertToIpNetGroup(group), nil
}

func (s *NetService) UpdateGroup(id uint, name string, description string) (IpGroup, error) {
	group, err := s.store.IpNetGroupStore.Update(id, name, description)
	if err != nil {
		return IpGroup{}, err
	}
	return convertToIpNetGroup(group), nil
}

func (s *NetService) DeleteGroup(id uint) error {
	//删除组后，所属组的ip会自动归到default group
	defaultGroup, err := s.store.IpNetGroupStore.FindDefault()
	if err != nil {
		return err
	}

	ips, err := s.store.IpNetStore.FindByGroupID(id)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		err = s.store.IpNetStore.UpdateGroupID(ip.ID, defaultGroup.ID)
		if err != nil {
			return err
		}
	}

	return s.store.IpNetGroupStore.DeleteByID(id)
}

// UpdateIPGroup 修改IP所属组
func (s *NetService) UpdateIPGroup(id uint, groupId uint) error {
	// 检查指定的组是否存在
	group, err := s.store.IpNetGroupStore.FindByID(groupId)
	if err != nil || group == nil {
		return fmt.Errorf("指定的组不存在: %w", err)
	}

	// 更新IP所属组
	err = s.store.IpNetStore.UpdateGroupID(id, groupId)
	if err != nil {
		return fmt.Errorf("更新IP所属组失败: %w", err)
	}

	return nil
}
