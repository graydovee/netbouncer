package service

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
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
	Ban(ipNet string, direction string) error
	RevertBan(ipNet string, direction string) error
	Allow(ipNet string) error
	RevertAllow(ipNet string) error
	CleanupIpNet(ipNet string) error
	ApplyRateLimit(rule core.RateLimitRule) error
	RemoveRateLimit(rule core.RateLimitRule) error
}

// RiskProvider 提供各 IP 的风险分（由策略引擎实现）
type RiskProvider interface {
	RiskScore(ip string) int
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

	// riskProvider/portProvider 由策略引擎在装配时注入，可为 nil（引擎禁用时）
	riskProvider RiskProvider
	portProvider PortStatsProvider
}

func NewNetService(monitor Monitor, firewall Firewall, store *store.Store) *NetService {
	return &NetService{
		monitor:  monitor,
		firewall: firewall,
		store:    store,
	}
}

// SetPolicyEngine 注入策略引擎（提供风险分与实时端口聚合）
func (s *NetService) SetPolicyEngine(e *PolicyEngine) {
	s.riskProvider = e
	s.portProvider = e
}

// SetRiskProvider 注入风险分提供者
func (s *NetService) SetRiskProvider(p RiskProvider) {
	s.riskProvider = p
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

	// 加载所有未过期的规则（过期临时封禁在引擎的清理循环中自动解除）
	ips, err := s.store.IpNetStore.FindAllActive()
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

	// 精确命中表：规则为单个 IP（/32、/128）时按规范化 IP 串建索引，
	// 供前端区分"精确规则管控"与"被网段规则覆盖"
	rules := append(bannedEntities, allowEntities...)
	exact := make(map[string]ruleRef, len(rules))
	for _, e := range rules {
		if n := parseIpNet(e.IpNet); n != nil {
			if ones, bits := n.Mask.Size(); ones == bits {
				exact[n.IP.String()] = ruleRef{id: e.ID, action: e.Action, expiresAt: e.ExpiresAt}
			}
		}
	}

	trafficData := make([]TrafficData, 0, len(stats))
	for _, stat := range stats {
		ref := exact[stat.RemoteIP]

		item := TrafficData{
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
			RuleAction:      ref.action,
			RuleID:          ref.id,
			Protocols:       convertProtoStats(stat.Protocols),
			Ports:           convertPortStats(stat.Ports, topPortsPerIP),
		}
		if ref.expiresAt != nil {
			item.BannedUntil = ref.expiresAt.Format(time.RFC3339)
		}
		if s.riskProvider != nil {
			item.RiskScore = s.riskProvider.RiskScore(stat.RemoteIP)
			item.RiskLevel = riskLevel(item.RiskScore)
		}
		trafficData = append(trafficData, item)
	}
	return trafficData, nil
}

// topPortsPerIP 实时接口中每个 IP 返回的端口明细条数上限
const topPortsPerIP = 8

// riskLevel 风险分到风险等级的映射
func riskLevel(score int) string {
	switch {
	case score >= 6:
		return "high"
	case score >= 3:
		return "medium"
	case score >= 1:
		return "low"
	}
	return "none"
}

func convertProtoStats(protocols map[string]*core.ProtoPortStats) []ProtoStat {
	if len(protocols) == 0 {
		return nil
	}
	result := make([]ProtoStat, 0, len(protocols))
	for proto, p := range protocols {
		result = append(result, ProtoStat{
			Proto:      proto,
			BytesIn:    p.BytesRecv,
			BytesOut:   p.BytesSent,
			PacketsIn:  p.PacketsRecv,
			PacketsOut: p.PacketsSent,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].BytesIn+result[i].BytesOut > result[j].BytesIn+result[j].BytesOut
	})
	return result
}

func convertPortStats(ports map[string]map[uint16]*core.ProtoPortStats, top int) []PortStat {
	if len(ports) == 0 {
		return nil
	}
	result := make([]PortStat, 0, len(ports)*2)
	for proto, pm := range ports {
		for port, p := range pm {
			result = append(result, PortStat{
				Proto:      proto,
				Port:       int(port),
				BytesIn:    p.BytesRecv,
				BytesOut:   p.BytesSent,
				PacketsIn:  p.PacketsRecv,
				PacketsOut: p.PacketsSent,
				Conns:      int(p.ConnsIn + p.ConnsOut),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].BytesIn+result[i].BytesOut > result[j].BytesIn+result[j].BytesOut
	})
	if len(result) > top {
		result = result[:top]
	}
	return result
}

// ruleRef 精确命中规则的引用
type ruleRef struct {
	id        uint
	action    string
	expiresAt *time.Time
}

// GetStats 获取（排除网段后的）IP流量统计
func (s *NetService) GetStats() ([]TrafficData, error) {
	stats := s.monitor.GetStats()
	return s.buildTrafficData(stats)
}

// CreateOrUpdateIpNet 创建或更新IP网络
// 如果IP网络已存在，则更新action, 忽略组信息
func (s *NetService) CreateOrUpdateIpNet(ipnet string, groupId uint, action string) error {
	return s.CreateOrUpdateIpNetWithOptions(ipnet, groupId, action, IpNetMutateOptions{})
}

// IpNetMutateOptions 创建/更新 IP 规则时的可选项（零值字段取默认值）
type IpNetMutateOptions struct {
	Direction string     // in/out/both，空视为 in
	ExpiresAt *time.Time // 非 nil 表示临时封禁
	Source    string     // 规则来源，空视为 manual
}

// CreateOrUpdateIpNetWithOptions 带选项的创建或更新
func (s *NetService) CreateOrUpdateIpNetWithOptions(ipnet string, groupId uint, action string, opts IpNetMutateOptions) error {
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
		// 如果IP网络已存在，则更新action与可选项, 忽略组信息
		ipNet, err := s.store.IpNetStore.FindByIpNet(ipnet)
		if err != nil {
			return Internalf("查询IP规则失败: %v", err)
		}

		return s.updateIpNetRecord(ipNet, action, opts)
	}

	// 创建IP网络记录
	ipNet, err := s.store.IpNetStore.CreateWithOptions(ipnet, groupId, action, store.IpNetOptions{
		Direction: opts.Direction,
		ExpiresAt: opts.ExpiresAt,
		Source:    opts.Source,
	})
	if err != nil {
		return Internalf("创建IP规则失败: %v", err)
	}

	if err := s.applyAction(ipNet); err != nil {
		return err
	}

	return nil
}

// BanTemporary 策略触发的临时封禁：已存在规则时改写为临时封禁，到期由引擎自动解封
func (s *NetService) BanTemporary(ipnet string, direction string, expires time.Time, source string) error {
	return s.CreateOrUpdateIpNetWithOptions(ipnet, 0, store.ActionBan, IpNetMutateOptions{
		Direction: direction,
		ExpiresAt: &expires,
		Source:    source,
	})
}

// CleanupExpiredTempBans 解除所有已过期的临时封禁，返回解除数量
func (s *NetService) CleanupExpiredTempBans() (int, error) {
	expired, err := s.store.IpNetStore.FindExpired(time.Now())
	if err != nil {
		return 0, Internalf("查询过期临时封禁失败: %v", err)
	}

	removed := 0
	for _, ipNet := range expired {
		if err := s.firewall.RevertBan(ipNet.IpNet, ipNet.Direction); err != nil {
			slog.Error("解除过期临时封禁失败", "ipnet", ipNet.IpNet, "error", err)
			continue
		}
		if err := s.store.IpNetStore.DeleteByID(ipNet.ID); err != nil {
			slog.Error("删除过期临时封禁记录失败", "ipnet", ipNet.IpNet, "error", err)
			continue
		}
		removed++
		slog.Info("临时封禁已到期自动解封", "ipnet", ipNet.IpNet, "expired_at", ipNet.ExpiresAt.Format(time.RFC3339))
	}
	return removed, nil
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

func isValidDirection(direction string) bool {
	return direction == store.DirectionIn || direction == store.DirectionOut || direction == store.DirectionBoth
}

// updateIpNetRecord 更新已存在记录的动作与可选项；动作或方向变化时先撤销旧规则再应用新规则
func (s *NetService) updateIpNetRecord(ipNet *store.IpNet, action string, opts IpNetMutateOptions) error {
	newDirection := opts.Direction
	if newDirection == "" {
		newDirection = store.DirectionIn
	}

	needRevert := action != ipNet.Action || newDirection != ipNet.Direction
	if needRevert {
		if err := s.revertAction(ipNet); err != nil {
			return Internalf("撤销原有行为失败: %v", err)
		}
	}

	if err := s.store.IpNetStore.UpdateWithOptions(ipNet.ID, action, store.IpNetOptions{
		Direction: opts.Direction,
		ExpiresAt: opts.ExpiresAt,
		Source:    opts.Source,
	}); err != nil {
		return Internalf("更新IP规则失败: %v", err)
	}
	ipNet.Action = action
	ipNet.Direction = newDirection
	ipNet.ExpiresAt = opts.ExpiresAt
	ipNet.Source = opts.Source

	if needRevert {
		if err := s.applyAction(ipNet); err != nil {
			return Internalf("应用新行为失败: %v", err)
		}
	}
	return nil
}

func (s *NetService) applyAction(ipNet *store.IpNet) error {
	switch ipNet.Action {
	case store.ActionBan:
		return s.firewall.Ban(ipNet.IpNet, ipNet.Direction)
	case store.ActionAllow:
		return s.firewall.Allow(ipNet.IpNet)
	default:
		return Invalidf("不支持的防火墙动作: %s", ipNet.Action)
	}
}

func (s *NetService) revertAction(ipNet *store.IpNet) error {
	switch ipNet.Action {
	case store.ActionBan:
		return s.firewall.RevertBan(ipNet.IpNet, ipNet.Direction)
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

	if action == ipNet.Action && ipNet.ExpiresAt == nil {
		return nil
	}

	// 手动操作视为永久规则：清除临时封禁的过期时间与策略来源标记
	return s.updateIpNetRecord(ipNet, action, IpNetMutateOptions{
		Direction: ipNet.Direction,
		ExpiresAt: nil,
		Source:    "manual",
	})
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
	item := IpNet{
		ID:        ip.ID,
		IpNet:     ip.IpNet,
		CreatedAt: ip.CreatedAt.Format(time.RFC3339),
		UpdatedAt: ip.UpdatedAt.Format(time.RFC3339),
		Group:     groupMap[ip.GroupID],
		Action:    ip.Action,
		Direction: ip.Direction,
		Source:    ip.Source,
	}
	if ip.ExpiresAt != nil {
		item.ExpiresAt = ip.ExpiresAt.Format(time.RFC3339)
	}
	return item
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
