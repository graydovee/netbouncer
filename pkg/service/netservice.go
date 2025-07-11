package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
	"gorm.io/gorm"
)

const DefaultGroupName = "default"

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
func (s *NetService) Init(items []config.RulesInitConfig) error {
	// 确保存在默认组
	_, err := s.store.IpNetGroupStore.FindDefault()
	if err != nil {
		// 如果没有默认组，创建一个
		defaultGroup, err := s.store.IpNetGroupStore.Create(DefaultGroupName, "系统默认的IP禁用组")
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

	// 初始化默认规则
	groupCache := make(map[string]*store.IpNetGroup)
	for _, item := range items {

		group, ok := groupCache[item.Group]
		if !ok {
			var err error
			group, err = s.store.IpNetGroupStore.FindByName(item.Group)
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
				group, err = s.store.IpNetGroupStore.Create(item.Group, item.GroupDescription)
				if err != nil {
					return err
				}
			}
			groupCache[item.Group] = group
		}
		for _, ipNet := range item.IpNets {
			_, err := s.store.IpNetStore.FindByIpNet(ipNet)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			} else if err == nil && !item.Override {
				slog.Info("初始化规则: 跳过已存在的规则", "ip", ipNet, "group", group.Name, "action", item.Action)
				continue
			}

			s.CreateOrUpdateIpNet(ipNet, group.ID, item.Action)
			slog.Info("初始化规则", "ip", ipNet, "group", group.Name, "action", item.Action)
		}
	}

	return nil
}

// GetAllStats 获取所有IP的流量统计
func (s *NetService) GetAllStats() ([]TrafficData, error) {
	stats := s.monitor.GetAllStats()
	trafficData := make([]TrafficData, 0, len(stats))

	bannedipNetEntity, err := s.store.IpNetStore.FindByAction(store.ActionBan)
	if err != nil {
		return nil, err
	}

	allowIpNetEntity, err := s.store.IpNetStore.FindByAction(store.ActionAllow)
	if err != nil {
		return nil, err
	}

	bannedIpNets := convertToIpNet(bannedipNetEntity...)
	allowIpNets := convertToIpNet(allowIpNetEntity...)

	for _, stat := range stats {
		isBanned := IsBanned(bannedIpNets, allowIpNets, stat.RemoteIP)

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

	bannedipNetEntity, err := s.store.IpNetStore.FindByAction(store.ActionBan)
	if err != nil {
		return nil, err
	}

	allowIpNetEntity, err := s.store.IpNetStore.FindByAction(store.ActionAllow)
	if err != nil {
		return nil, err
	}

	bannedIpNets := convertToIpNet(bannedipNetEntity...)
	allowIpNets := convertToIpNet(allowIpNetEntity...)

	for _, stat := range stats {
		isBanned := IsBanned(bannedIpNets, allowIpNets, stat.RemoteIP)

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

// CreateOrUpdateIpNet 创建或更新IP网络
// 如果IP网络已存在，则更新action, 忽略组信息
func (s *NetService) CreateOrUpdateIpNet(ipnet string, groupId uint, action string) error {

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
		// 如果IP网络已存在，则更新action, 忽略组信息
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

	err = s.applyAction(ipNet)
	if err != nil {
		return err
	}

	return nil
}

func (s *NetService) applyAction(ipNet *store.IpNet) error {
	switch ipNet.Action {
	case store.ActionBan:
		return s.firewall.Ban(ipNet.IpNet)
	case store.ActionAllow:
		return s.firewall.Allow(ipNet.IpNet)
	default:
		return fmt.Errorf("不支持的防火墙动作: %s", ipNet.Action)
	}
}

func (s *NetService) revertAction(ipNet *store.IpNet) error {
	switch ipNet.Action {
	case store.ActionBan:
		return s.firewall.RevertBan(ipNet.IpNet)
	case store.ActionAllow:
		return s.firewall.RevertAllow(ipNet.IpNet)
	default:
		return fmt.Errorf("不支持的防火墙动作: %s", ipNet.Action)
	}
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

	err = s.revertAction(ipNet)
	if err != nil {
		return fmt.Errorf("撤销原有行为失败: %w", err)
	}

	ipNet.Action = action

	err = s.applyAction(ipNet)
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

func (s *NetService) ImportIpNet(text string, groupId uint, action string) (int, int, error) {
	ipnets := extractIPsAndCIDRs(text)
	slog.Info("导入地址", "count", len(ipnets))
	if len(ipnets) == 0 {
		return 0, 0, nil
	}

	// 检查指定的组是否存在
	group, err := s.store.IpNetGroupStore.FindByID(groupId)
	if err != nil || group == nil {
		return 0, 0, fmt.Errorf("指定的组不存在: %w", err)
	}

	// 查找已存在的IP网络记录
	slog.Info("查询已存在的IP网络记录")
	existingIpNets, err := s.store.IpNetStore.FindByIpNets(ipnets)
	if err != nil {
		return 0, 0, fmt.Errorf("查询已存在的IP网络失败: %w", err)
	}

	// 构建已存在IP的映射，用于快速查找
	existingMap := make(map[string]*store.IpNet)
	for i := range existingIpNets {
		existingMap[existingIpNets[i].IpNet] = &existingIpNets[i]
	}

	// 分离需要更新action的IP和需要新增的IP
	var toUpdate []*store.IpNet
	var toCreate []string

	for _, ipnet := range ipnets {
		if existing, exists := existingMap[ipnet]; exists {
			// 如果action不一致，需要更新
			if existing.Action != action {
				toUpdate = append(toUpdate, existing)
			}
			// 如果action一致，跳过
		} else {
			// 不存在，需要新增
			toCreate = append(toCreate, ipnet)
		}
	}

	slog.Info("导入地址分类", "to_update", len(toUpdate), "to_create", len(toCreate))

	successCount := 0
	errorCount := 0

	// 1. 批量更新已存在但action不一致的IP
	if len(toUpdate) > 0 {
		slog.Info("开始更新已存在的IP网络action")
		for _, ipNet := range toUpdate {
			err := s.UpdateIpNetAction(ipNet.ID, action)
			if err != nil {
				errorCount++
				slog.Error("更新IP网络action失败", "ipnet", ipNet.IpNet, "error", err)
			} else {
				successCount++
				slog.Info("更新IP网络action成功", "ipnet", ipNet.IpNet, "action", action)
			}
		}
	}

	// 2. 批量插入新的IP网络记录
	if len(toCreate) > 0 {
		slog.Info("开始批量创建新的IP网络记录", "count", len(toCreate))
		newIpNets, err := s.store.IpNetStore.BatchCreate(toCreate, groupId, action)
		if err != nil {
			errorCount += len(toCreate)
			slog.Error("批量创建IP网络失败", "error", err)
		} else {
			successCount += len(newIpNets)
			slog.Info("批量创建IP网络成功", "count", len(newIpNets))

			// 3. 批量应用防火墙规则
			slog.Info("开始应用防火墙规则", "count", len(newIpNets))
			for _, ipNet := range newIpNets {
				err := s.applyAction(&ipNet)
				if err != nil {
					slog.Error("应用防火墙规则失败", "ipnet", ipNet.IpNet, "error", err)
					// 注意：这里不增加errorCount，因为数据库插入已经成功
				} else {
					slog.Info("应用防火墙规则成功", "ipnet", ipNet.IpNet, "action", action)
				}
			}
		}
	}

	slog.Info("导入完成", "success", successCount, "error", errorCount)
	return successCount, errorCount, nil
}
