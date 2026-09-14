package service

import (
	"errors"
	"log/slog"

	"gorm.io/gorm"

	"github.com/graydovee/netbouncer/pkg/store"
)

// validatePolicy 校验策略参数并填充默认值
func validatePolicy(p *store.Policy) error {
	if p.Name == "" {
		return Invalidf("策略名称不能为空")
	}
	if len([]rune(p.Name)) > 64 {
		return Invalidf("策略名称过长")
	}
	if !isValidDirection(p.Direction) {
		return Invalidf("无效的流量方向: %s", p.Direction)
	}
	if p.Protocol != store.ProtocolAny && p.Protocol != store.ProtocolTCP && p.Protocol != store.ProtocolUDP {
		return Invalidf("无效的协议: %s", p.Protocol)
	}
	if p.Port < 0 || p.Port > 65535 {
		return Invalidf("端口范围无效: %d", p.Port)
	}

	switch p.Action {
	case store.PolicyActionRateLimit:
		if p.LimitKBps <= 0 {
			return Invalidf("限速动作需要设置限速值 limit_kbps")
		}
		if p.Port > 0 && p.Protocol == store.ProtocolAny {
			return Invalidf("指定端口时协议必须为 tcp 或 udp")
		}
	case store.PolicyActionBan:
		if p.BanSec < minBanSec || p.BanSec > maxBanSec {
			return Invalidf("禁用时长需在 %d ~ %d 秒之间", minBanSec, maxBanSec)
		}
	case store.PolicyActionMark:
		// 仅标记，无需额外参数
	default:
		return Invalidf("无效的策略动作: %s", p.Action)
	}

	// mark/ban 需要至少一个触发条件
	if p.Action != store.PolicyActionRateLimit {
		if p.RateKBps <= 0 && p.TotalMB <= 0 && p.ConnRate <= 0 && p.DistinctPorts <= 0 {
			return Invalidf("至少设置一个触发条件（速率/累计流量/连接数/端口数）")
		}
		if p.DistinctPorts > 0 && p.Port > 0 {
			return Invalidf("端口数检测仅在 Port=0（任意端口）时有效")
		}
	}

	if p.RateKBps < 0 || p.TotalMB < 0 || p.ConnRate < 0 || p.DistinctPorts < 0 {
		return Invalidf("触发阈值不能为负数")
	}
	if p.WindowSec == 0 {
		p.WindowSec = 300
	}
	if p.WindowSec < minPolicyWindow || p.WindowSec > maxPolicyWindow {
		return Invalidf("统计窗口需在 %d ~ %d 秒之间", minPolicyWindow, maxPolicyWindow)
	}
	if p.RiskScore < 0 {
		return Invalidf("风险分不能为负数")
	}
	if p.RiskScore == 0 && p.RiskBanThreshold > 0 {
		// 触发不加分则永远达不到升级阈值，视为配置错误
		return Invalidf("启用风险升级时 risk_score 必须大于 0")
	}
	if (p.RiskBanThreshold > 0) != (p.RiskBanSec > 0) {
		return Invalidf("风险升级阈值与升级封禁时长必须同时设置")
	}
	if p.RiskBanSec > 0 && (p.RiskBanSec < minBanSec || p.RiskBanSec > maxBanSec) {
		return Invalidf("升级封禁时长需在 %d ~ %d 秒之间", minBanSec, maxBanSec)
	}
	if p.CooldownSec == 0 {
		p.CooldownSec = 300
	}
	return nil
}

// ListPolicies 获取全部策略
func (s *NetService) ListPolicies() ([]Policy, error) {
	policies, err := s.store.PolicyStore.FindAll()
	if err != nil {
		return nil, Internalf("查询策略失败: %v", err)
	}
	result := make([]Policy, 0, len(policies))
	for i := range policies {
		result = append(result, convertToPolicy(&policies[i]))
	}
	return result, nil
}

// CreatePolicy 创建策略，限速类策略立即下发内核规则
func (s *NetService) CreatePolicy(p *store.Policy) error {
	if err := validatePolicy(p); err != nil {
		return err
	}
	if exists, err := s.store.PolicyStore.ExistsByName(p.Name, 0); err != nil {
		return Internalf("查询策略失败: %v", err)
	} else if exists {
		return Conflictf("策略 %q 已存在", p.Name)
	}

	if err := s.store.PolicyStore.Create(p); err != nil {
		return Internalf("创建策略失败: %v", err)
	}

	s.syncRateLimit(p)
	return nil
}

// UpdatePolicy 更新策略，重新装卸限速规则
func (s *NetService) UpdatePolicy(id uint, p *store.Policy) error {
	if err := validatePolicy(p); err != nil {
		return err
	}
	if exists, err := s.store.PolicyStore.ExistsByName(p.Name, id); err != nil {
		return Internalf("查询策略失败: %v", err)
	} else if exists {
		return Conflictf("策略 %q 已存在", p.Name)
	}

	old, err := s.store.PolicyStore.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFoundf("策略不存在 (id=%d)", id)
		}
		return Internalf("查询策略失败: %v", err)
	}

	p.ID = old.ID
	p.CreatedAt = old.CreatedAt
	if err := s.store.PolicyStore.Update(p); err != nil {
		return Internalf("更新策略失败: %v", err)
	}

	// 旧参数的规则先卸载，再按新参数装载
	s.uninstallRateLimit(old)
	s.syncRateLimit(p)
	return nil
}

// DeletePolicy 删除策略并卸载其限速规则
func (s *NetService) DeletePolicy(id uint) error {
	old, err := s.store.PolicyStore.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFoundf("策略不存在 (id=%d)", id)
		}
		return Internalf("查询策略失败: %v", err)
	}

	s.uninstallRateLimit(old)

	if err := s.store.PolicyStore.Delete(id); err != nil {
		return Internalf("删除策略失败: %v", err)
	}
	return nil
}

// SetPolicyEnabled 启用/停用策略，同步装卸限速规则
func (s *NetService) SetPolicyEnabled(id uint, enabled bool) error {
	p, err := s.store.PolicyStore.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFoundf("策略不存在 (id=%d)", id)
		}
		return Internalf("查询策略失败: %v", err)
	}

	if err := s.store.PolicyStore.SetEnabled(id, enabled); err != nil {
		return Internalf("更新策略状态失败: %v", err)
	}
	p.Enabled = enabled

	s.uninstallRateLimit(p)
	s.syncRateLimit(p)
	return nil
}

// syncRateLimit 若策略为启用中的限速策略则下发内核规则
func (s *NetService) syncRateLimit(p *store.Policy) {
	if !p.Enabled || p.Action != store.PolicyActionRateLimit {
		return
	}
	if err := s.firewall.ApplyRateLimit(rateLimitRuleFromPolicy(p)); err != nil {
		slog.Error("应用限速规则失败", "policy", p.Name, "error", err)
		return
	}
	slog.Info("限速规则已应用", "policy", p.Name, "rate_kbps", p.LimitKBps)
}

// uninstallRateLimit 若策略为限速策略则卸载其内核规则（规则不存在视为成功）
func (s *NetService) uninstallRateLimit(p *store.Policy) {
	if p.Action != store.PolicyActionRateLimit {
		return
	}
	if err := s.firewall.RemoveRateLimit(rateLimitRuleFromPolicy(p)); err != nil {
		slog.Error("卸载限速规则失败", "policy", p.Name, "error", err)
	}
}

// ListRiskEvents 分页查询风险事件
func (s *NetService) ListRiskEvents(ip string, page, pageSize int) (*RiskEventListResult, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	events, total, err := s.store.RiskEventStore.FindByFilter(store.RiskEventFilter{
		RemoteIP: ip,
		Offset:   (page - 1) * pageSize,
		Limit:    pageSize,
	})
	if err != nil {
		return nil, Internalf("查询风险事件失败: %v", err)
	}

	items := make([]RiskEvent, 0, len(events))
	for i := range events {
		items = append(items, convertToRiskEvent(&events[i]))
	}
	return &RiskEventListResult{Items: items, Total: total}, nil
}

// PortStatsProvider 提供实时端口聚合数据（由策略引擎实现，引擎禁用时无数据）
type PortStatsProvider interface {
	PortTrafficSnapshot() []PortTraffic
}

// GetPortTraffic 获取实时端口排行（跨 IP 聚合）
func (s *NetService) GetPortTraffic() ([]PortTraffic, error) {
	if s.portProvider == nil {
		return []PortTraffic{}, nil
	}
	return s.portProvider.PortTrafficSnapshot(), nil
}
