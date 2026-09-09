package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

const DefaultGroupName = "default"

// Monitor 流量监控接口，解耦对 *core.Monitor 的直接依赖，便于测试
type Monitor interface {
	GetStats() map[string]*core.TrafficStats
}

// Firewall 防火墙接口，解耦对 *core.Firewall 的直接依赖，便于测试
type Firewall interface {
	Init(ipList []store.IpNet) error
	Ban(ipNet string) error
	RevertBan(ipNet string) error
	Allow(ipNet string) error
	RevertAllow(ipNet string) error
	CleanupIpNet(ipNet string) error
}

// 编译期确认具体实现满足接口
var (
	_ Monitor  = (*core.Monitor)(nil)
	_ Firewall = (*core.Firewall)(nil)
)

type NetService struct {
	monitor  Monitor
	firewall Firewall

	store *store.Store
}

func NewNetService(monitor Monitor, firewall Firewall, store *store.Store) *NetService {
	return &NetService{
		monitor:  monitor,
		firewall: firewall,
		store:    store,
	}
}

// Init 初始化服务
func (s *NetService) Init(items []config.RulesInitConfig) error {
	// 确保存在默认组
	if _, err := s.store.IpNetGroupStore.FindDefault(); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return Internalf("查询默认组失败: %v", err)
		}
		if _, err := s.ensureDefaultGroup(); err != nil {
			return err
		}
	}

	// 从存储中加载所有已存在的IP
	ips, err := s.store.IpNetStore.FindAll()
	if err != nil {
		return Internalf("加载IP规则失败: %v", err)
	}

	// 初始化防火墙
	if err := s.firewall.Init(ips); err != nil {
		return err
	}

	// 初始化默认规则
	groupCache := make(map[string]*store.IpNetGroup)
	for _, item := range items {
		group, ok := groupCache[item.Group]
		if !ok {
			group, err = s.store.IpNetGroupStore.FindByName(item.Group)
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return Internalf("查询组失败: %v", err)
				}
				group, err = s.store.IpNetGroupStore.Create(item.Group, item.GroupDescription)
				if err != nil {
					return Internalf("创建组失败: %v", err)
				}
			}
			groupCache[item.Group] = group
		}
		for _, ipNet := range item.IpNets {
			_, err := s.store.IpNetStore.FindByIpNet(ipNet)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return Internalf("查询IP规则失败: %v", err)
			} else if err == nil && !item.Override {
				slog.Info("初始化规则: 跳过已存在的规则", "ip", ipNet, "group", group.Name, "action", item.Action)
				continue
			}

			if err := s.CreateOrUpdateIpNet(ipNet, group.ID, item.Action); err != nil {
				return fmt.Errorf("初始化规则失败 %s: %w", ipNet, err)
			}
			slog.Info("初始化规则", "ip", ipNet, "group", group.Name, "action", item.Action)
		}
	}

	return nil
}

// ensureDefaultGroup 确保默认组存在，返回默认组
func (s *NetService) ensureDefaultGroup() (*store.IpNetGroup, error) {
	defaultGroup, err := s.store.IpNetGroupStore.Create(DefaultGroupName, "系统默认的IP禁用组")
	if err != nil {
		return nil, Internalf("创建默认组失败: %v", err)
	}
	if err := s.store.IpNetGroupStore.SetDefault(defaultGroup.ID); err != nil {
		return nil, Internalf("设置默认组失败: %v", err)
	}
	return defaultGroup, nil
}

// buildTrafficData 将流量统计与封禁状态组装为对外的 TrafficData
func (s *NetService) buildTrafficData(stats map[string]*core.TrafficStats) ([]TrafficData, error) {
	bannedEntities, err := s.store.IpNetStore.FindByAction(store.ActionBan)
	if err != nil {
		return nil, Internalf("查询封禁列表失败: %v", err)
	}

	allowEntities, err := s.store.IpNetStore.FindByAction(store.ActionAllow)
	if err != nil {
		return nil, Internalf("查询白名单失败: %v", err)
	}

	bannedIpNets := convertToIpNet(bannedEntities...)
	allowIpNets := convertToIpNet(allowEntities...)

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
			IsBanned:        IsBanned(bannedIpNets, allowIpNets, stat.RemoteIP),
		})
	}
	return trafficData, nil
}

// GetStats 获取（排除网段后的）IP流量统计
func (s *NetService) GetStats() ([]TrafficData, error) {
	stats := s.monitor.GetStats()
	return s.buildTrafficData(stats)
}

// CreateOrUpdateIpNet 创建或更新IP网络
// 如果IP网络已存在，则更新action, 忽略组信息
func (s *NetService) CreateOrUpdateIpNet(ipnet string, groupId uint, action string) error {
	if !isValidAction(action) {
		return Invalidf("不支持的防火墙动作: %s", action)
	}

	// 如果没有指定组ID，使用默认组
	if groupId == 0 {
		defaultGroup, err := s.store.IpNetGroupStore.FindDefault()
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return Internalf("查询默认组失败: %v", err)
			}
			defaultGroup, err = s.ensureDefaultGroup()
			if err != nil {
				return err
			}
		}
		groupId = defaultGroup.ID
	} else if _, err := s.getGroup(groupId); err != nil {
		return err
	}

	if s.store.IpNetStore.ExistsByIpNet(ipnet) {
		// 如果IP网络已存在，则更新action, 忽略组信息
		ipNet, err := s.store.IpNetStore.FindByIpNet(ipnet)
		if err != nil {
			return Internalf("查询IP规则失败: %v", err)
		}

		return s.UpdateIpNetAction(ipNet.ID, action)
	}

	// 创建IP网络记录
	ipNet, err := s.store.IpNetStore.Create(ipnet, groupId, action)
	if err != nil {
		return Internalf("创建IP规则失败: %v", err)
	}

	if err := s.applyAction(ipNet); err != nil {
		return err
	}

	return nil
}

// getGroup 按ID查询组，不存在时返回 ErrNotFound
func (s *NetService) getGroup(groupId uint) (*store.IpNetGroup, error) {
	group, err := s.store.IpNetGroupStore.FindByID(groupId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NotFoundf("指定的组不存在 (id=%d)", groupId)
		}
		return nil, Internalf("查询组失败: %v", err)
	}
	return group, nil
}

func isValidAction(action string) bool {
	return action == store.ActionBan || action == store.ActionAllow
}

func (s *NetService) applyAction(ipNet *store.IpNet) error {
	switch ipNet.Action {
	case store.ActionBan:
		return s.firewall.Ban(ipNet.IpNet)
	case store.ActionAllow:
		return s.firewall.Allow(ipNet.IpNet)
	default:
		return Invalidf("不支持的防火墙动作: %s", ipNet.Action)
	}
}

func (s *NetService) revertAction(ipNet *store.IpNet) error {
	switch ipNet.Action {
	case store.ActionBan:
		return s.firewall.RevertBan(ipNet.IpNet)
	case store.ActionAllow:
		return s.firewall.RevertAllow(ipNet.IpNet)
	default:
		return Invalidf("不支持的防火墙动作: %s", ipNet.Action)
	}
}

func (s *NetService) DeleteIpNet(id uint) error {
	ipNet, err := s.store.IpNetStore.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFoundf("IP规则不存在 (id=%d)", id)
		}
		return Internalf("查询IP规则失败: %v", err)
	}

	if err := s.firewall.CleanupIpNet(ipNet.IpNet); err != nil {
		return Internalf("撤销原有行为失败: %v", err)
	}

	if err := s.store.IpNetStore.DeleteByID(id); err != nil {
		return Internalf("删除IP网络失败: %v", err)
	}

	return nil
}

// DeleteIpNets 批量删除IP规则，返回成功删除的数量
func (s *NetService) DeleteIpNets(ids []uint) (int, error) {
	ips, err := s.store.IpNetStore.FindByIDs(ids)
	if err != nil {
		return 0, Internalf("查询IP规则失败: %v", err)
	}

	success := 0
	var failedIDs []uint
	for _, ipNet := range ips {
		if err := s.firewall.CleanupIpNet(ipNet.IpNet); err != nil {
			slog.Error("撤销防火墙规则失败", "ipnet", ipNet.IpNet, "error", err)
			failedIDs = append(failedIDs, ipNet.ID)
			continue
		}
		if err := s.store.IpNetStore.DeleteByID(ipNet.ID); err != nil {
			slog.Error("删除IP规则失败", "ipnet", ipNet.IpNet, "error", err)
			failedIDs = append(failedIDs, ipNet.ID)
			continue
		}
		success++
	}

	// 防火墙撤销失败或删除失败的记录保留在库中，保证状态一致
	if len(failedIDs) > 0 {
		slog.Warn("批量删除部分失败", "failed_ids", failedIDs)
	}

	return success, nil
}

func (s *NetService) UpdateIpNetAction(id uint, action string) error {
	if !isValidAction(action) {
		return Invalidf("不支持的防火墙动作: %s", action)
	}

	ipNet, err := s.store.IpNetStore.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFoundf("IP规则不存在 (id=%d)", id)
		}
		return Internalf("查询IP规则失败: %v", err)
	}

	if action == ipNet.Action {
		return nil
	}

	if err := s.revertAction(ipNet); err != nil {
		return Internalf("撤销原有行为失败: %v", err)
	}

	ipNet.Action = action

	if err := s.applyAction(ipNet); err != nil {
		return Internalf("应用新行为失败: %v", err)
	}

	if err := s.store.IpNetStore.UpdateAction(id, action); err != nil {
		return Internalf("更新IP行为失败: %v", err)
	}

	return nil
}

// UpdateIpNetActions 批量更新IP规则的动作，返回成功数量
func (s *NetService) UpdateIpNetActions(ids []uint, action string) (int, error) {
	if !isValidAction(action) {
		return 0, Invalidf("不支持的防火墙动作: %s", action)
	}

	ips, err := s.store.IpNetStore.FindByIDs(ids)
	if err != nil {
		return 0, Internalf("查询IP规则失败: %v", err)
	}

	success := 0
	for _, ipNet := range ips {
		if err := s.UpdateIpNetAction(ipNet.ID, action); err != nil {
			slog.Error("批量更新IP规则动作失败", "ipnet", ipNet.IpNet, "error", err)
			continue
		}
		success++
	}
	return success, nil
}

// IpNetListParams IP规则列表的查询参数
type IpNetListParams struct {
	Page     int    // 从 1 开始，为 0 时视为 1
	PageSize int    // 为 0 时表示不分页，返回全部
	GroupID  uint   // 为 0 时不过滤
	Action   string // 为空时不过滤
	Search   string // 为空时不过滤
}

// IpNetListResult IP规则列表的查询结果
type IpNetListResult struct {
	Items []IpNet `json:"items"`
	Total int64   `json:"total"`
}

// ListIpNets 按条件分页获取IP规则列表
func (s *NetService) ListIpNets(params IpNetListParams) (*IpNetListResult, error) {
	if params.Action != "" && !isValidAction(params.Action) {
		return nil, Invalidf("不支持的防火墙动作: %s", params.Action)
	}
	if params.Page <= 0 {
		params.Page = 1
	}

	filter := store.IpNetFilter{
		GroupID: params.GroupID,
		Action:  params.Action,
		Search:  params.Search,
		Limit:   params.PageSize,
		Offset:  (params.Page - 1) * params.PageSize,
	}

	ips, total, err := s.store.IpNetStore.FindByFilter(filter)
	if err != nil {
		return nil, Internalf("查询IP规则失败: %v", err)
	}

	groupMap, err := s.getGroupMap()
	if err != nil {
		return nil, err
	}

	items := make([]IpNet, 0, len(ips))
	for _, ip := range ips {
		items = append(items, convertToIpNetItem(ip, groupMap))
	}

	return &IpNetListResult{Items: items, Total: total}, nil
}

// ListAllIpNets 获取所有IP列表（不分页）
func (s *NetService) ListAllIpNets() ([]IpNet, error) {
	result, err := s.ListIpNets(IpNetListParams{})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (s *NetService) ListIpNetsByGroup(groupId uint) ([]IpNet, error) {
	if _, err := s.getGroup(groupId); err != nil {
		return nil, err
	}

	result, err := s.ListIpNets(IpNetListParams{GroupID: groupId})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

// getGroupMap 加载全部组并转为 id -> IpGroup 映射
func (s *NetService) getGroupMap() (map[uint]*IpGroup, error) {
	groups, err := s.store.IpNetGroupStore.FindAll()
	if err != nil {
		return nil, Internalf("查询组列表失败: %v", err)
	}

	groupMap := make(map[uint]*IpGroup, len(groups))
	for _, group := range groups {
		g := convertToIpNetGroup(&group)
		groupMap[group.ID] = &g
	}
	return groupMap, nil
}

func convertToIpNetItem(ip store.IpNet, groupMap map[uint]*IpGroup) IpNet {
	return IpNet{
		ID:        ip.ID,
		IpNet:     ip.IpNet,
		CreatedAt: ip.CreatedAt.Format(time.RFC3339),
		UpdatedAt: ip.UpdatedAt.Format(time.RFC3339),
		Group:     groupMap[ip.GroupID],
		Action:    ip.Action,
	}
}

func (s *NetService) ListAllGroups() ([]IpGroup, error) {
	groups, err := s.store.IpNetGroupStore.FindAll()
	if err != nil {
		return nil, Internalf("查询组列表失败: %v", err)
	}

	counts, err := s.store.IpNetGroupStore.CountByGroupID()
	if err != nil {
		return nil, Internalf("统计组内IP数量失败: %v", err)
	}

	groupList := make([]IpGroup, 0, len(groups))
	for _, group := range groups {
		g := convertToIpNetGroup(&group)
		g.IPCount = counts[group.ID]
		groupList = append(groupList, g)
	}
	return groupList, nil
}

func (s *NetService) CreateGroup(name string, description string) (IpGroup, error) {
	if name == "" {
		return IpGroup{}, Invalidf("组名称不能为空")
	}

	if s.store.IpNetGroupStore.FindByNameExists(name) {
		return IpGroup{}, Conflictf("组 %q 已存在", name)
	}

	group, err := s.store.IpNetGroupStore.Create(name, description)
	if err != nil {
		return IpGroup{}, Internalf("创建组失败: %v", err)
	}
	return convertToIpNetGroup(group), nil
}

func (s *NetService) UpdateGroup(id uint, name string, description string) (IpGroup, error) {
	if name == "" {
		return IpGroup{}, Invalidf("组名称不能为空")
	}

	group, err := s.store.IpNetGroupStore.Update(id, name, description)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return IpGroup{}, NotFoundf("指定的组不存在 (id=%d)", id)
		}
		return IpGroup{}, Internalf("更新组失败: %v", err)
	}
	return convertToIpNetGroup(group), nil
}

func (s *NetService) DeleteGroup(id uint) error {
	// 删除组后，所属组的ip会自动归到default group
	defaultGroup, err := s.store.IpNetGroupStore.FindDefault()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFoundf("默认组不存在")
		}
		return Internalf("查询默认组失败: %v", err)
	}

	if id == defaultGroup.ID {
		return Invalidf("不能删除默认组")
	}

	if _, err := s.getGroup(id); err != nil {
		return err
	}

	ips, err := s.store.IpNetStore.FindByGroupID(id)
	if err != nil {
		return Internalf("查询组内IP失败: %v", err)
	}

	for _, ip := range ips {
		if err := s.store.IpNetStore.UpdateGroupID(ip.ID, defaultGroup.ID); err != nil {
			return Internalf("迁移组内IP失败: %v", err)
		}
	}

	if err := s.store.IpNetGroupStore.DeleteByID(id); err != nil {
		return Internalf("删除组失败: %v", err)
	}
	return nil
}

// UpdateIPGroup 修改IP所属组
func (s *NetService) UpdateIPGroup(id uint, groupId uint) error {
	if _, err := s.getGroup(groupId); err != nil {
		return err
	}

	// 更新IP所属组
	if err := s.store.IpNetStore.UpdateGroupID(id, groupId); err != nil {
		return Internalf("更新IP所属组失败: %v", err)
	}

	return nil
}

// UpdateIPGroups 批量修改IP所属组，返回成功数量
func (s *NetService) UpdateIPGroups(ids []uint, groupId uint) (int, error) {
	if _, err := s.getGroup(groupId); err != nil {
		return 0, err
	}

	success := 0
	for _, id := range ids {
		if err := s.store.IpNetStore.UpdateGroupID(id, groupId); err != nil {
			slog.Error("批量更新IP所属组失败", "id", id, "error", err)
			continue
		}
		success++
	}
	return success, nil
}

// ImportIpNet 从文本导入IP/CIDR列表，返回 (成功数, 失败数, 错误)
func (s *NetService) ImportIpNet(text string, groupId uint, action string) (int, int, error) {
	if !isValidAction(action) {
		return 0, 0, Invalidf("不支持的防火墙动作: %s", action)
	}

	ipnets := extractIPsAndCIDRs(text)
	slog.Info("导入地址", "count", len(ipnets))
	if len(ipnets) == 0 {
		return 0, 0, nil
	}

	if _, err := s.getGroup(groupId); err != nil {
		return 0, 0, err
	}

	// 查找已存在的IP网络记录
	existingIpNets, err := s.store.IpNetStore.FindByIpNets(ipnets)
	if err != nil {
		return 0, 0, Internalf("查询已存在的IP网络失败: %v", err)
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
	for _, ipNet := range toUpdate {
		if err := s.UpdateIpNetAction(ipNet.ID, action); err != nil {
			errorCount++
			slog.Error("更新IP网络action失败", "ipnet", ipNet.IpNet, "error", err)
		} else {
			successCount++
		}
	}

	// 2. 批量插入新的IP网络记录
	if len(toCreate) > 0 {
		newIpNets, err := s.store.IpNetStore.BatchCreate(toCreate, groupId, action)
		if err != nil {
			errorCount += len(toCreate)
			slog.Error("批量创建IP网络失败", "error", err)
		} else {
			// 3. 批量应用防火墙规则；应用失败的记录计入失败数
			for _, ipNet := range newIpNets {
				if err := s.applyAction(&ipNet); err != nil {
					errorCount++
					slog.Error("应用防火墙规则失败", "ipnet", ipNet.IpNet, "error", err)
				} else {
					successCount++
				}
			}
		}
	}

	slog.Info("导入完成", "success", successCount, "error", errorCount)
	return successCount, errorCount, nil
}
