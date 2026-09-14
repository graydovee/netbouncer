package web

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/store"
)

// CreatePolicyRequest 创建/更新策略请求
type CreatePolicyRequest struct {
	Name      string `json:"name"`
	Enabled   *bool  `json:"enabled"`
	Direction string `json:"direction"`
	Protocol  string `json:"protocol"`
	Port      int    `json:"port"`

	RateKBps      float64 `json:"rate_kbps"`
	TotalMB       float64 `json:"total_mb"`
	ConnRate      int     `json:"conn_rate"`
	DistinctPorts int     `json:"distinct_ports"`
	WindowSec     int     `json:"window_sec"`

	Action    string  `json:"action"`
	LimitKBps float64 `json:"limit_kbps"`
	BurstKBps float64 `json:"burst_kbps"`
	BanSec    int     `json:"ban_sec"`
	RiskScore *int    `json:"risk_score"`

	RiskBanThreshold int `json:"risk_ban_threshold"`
	RiskBanSec       int `json:"risk_ban_sec"`
	CooldownSec      int `json:"cooldown_sec"`
}

// toPolicy 请求转换为策略实体
func (r *CreatePolicyRequest) toPolicy() *store.Policy {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	riskScore := 1
	if r.RiskScore != nil {
		riskScore = *r.RiskScore
	}

	direction := r.Direction
	if direction == "" {
		direction = store.DirectionBoth
	}
	protocol := r.Protocol
	if protocol == "" {
		protocol = store.ProtocolAny
	}

	return &store.Policy{
		Name:             r.Name,
		Enabled:          enabled,
		Direction:        direction,
		Protocol:         protocol,
		Port:             r.Port,
		RateKBps:         r.RateKBps,
		TotalMB:          r.TotalMB,
		ConnRate:         r.ConnRate,
		DistinctPorts:    r.DistinctPorts,
		WindowSec:        r.WindowSec,
		Action:           r.Action,
		LimitKBps:        r.LimitKBps,
		BurstKBps:        r.BurstKBps,
		BanSec:           r.BanSec,
		RiskScore:        riskScore,
		RiskBanThreshold: r.RiskBanThreshold,
		RiskBanSec:       r.RiskBanSec,
		CooldownSec:      r.CooldownSec,
	}
}

func (s *Server) handleListPolicies(c echo.Context) error {
	policies, err := s.netService.ListPolicies()
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, policies)
}

func (s *Server) handleCreatePolicy(c echo.Context) error {
	var r CreatePolicyRequest
	ok, err := bindAndValidate(c, &r, func() error {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("name 不能为空")
		}
		if r.Action == "" {
			return fmt.Errorf("action 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	if err := s.netService.CreatePolicy(r.toPolicy()); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

func (s *Server) handleUpdatePolicy(c echo.Context) error {
	id, ok, err := pathUint(c, "id")
	if !ok {
		return err
	}

	var r CreatePolicyRequest
	ok, err = bindAndValidate(c, &r, func() error {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("name 不能为空")
		}
		if r.Action == "" {
			return fmt.Errorf("action 不能为空")
		}
		return nil
	})
	if !ok {
		return err
	}

	if err := s.netService.UpdatePolicy(id, r.toPolicy()); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

func (s *Server) handleDeletePolicy(c echo.Context) error {
	id, ok, err := pathUint(c, "id")
	if !ok {
		return err
	}

	if err := s.netService.DeletePolicy(id); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

// SetPolicyEnabledRequest 启停策略请求
type SetPolicyEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleSetPolicyEnabled(c echo.Context) error {
	id, ok, err := pathUint(c, "id")
	if !ok {
		return err
	}

	var r SetPolicyEnabledRequest
	ok, err = bindAndValidate(c, &r, nil)
	if !ok {
		return err
	}

	if err := s.netService.SetPolicyEnabled(id, r.Enabled); err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, nil)
}

// handleListRiskEvents 分页查询风险事件
// 查询参数: ip(可选), page, page_size
func (s *Server) handleListRiskEvents(c echo.Context) error {
	page, ok, err := queryInt(c, "page", 1)
	if !ok {
		return err
	}
	pageSize, ok, err := queryInt(c, "page_size", 20)
	if !ok {
		return err
	}

	result, err := s.netService.ListRiskEvents(c.QueryParam("ip"), page, pageSize)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, result)
}

// handleGetPortTraffic 实时端口排行（跨 IP 聚合）
func (s *Server) handleGetPortTraffic(c echo.Context) error {
	ports, err := s.netService.GetPortTraffic()
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, ports)
}

// parsePortHistoryParams 解析端口/协议维度历史查询的公共参数
func parsePortHistoryParams(c echo.Context) (service.PortHistoryParams, bool, error) {
	start, ok, err := queryInt(c, "start", 0)
	if !ok {
		return service.PortHistoryParams{}, false, err
	}
	end, ok, err := queryInt(c, "end", 0)
	if !ok {
		return service.PortHistoryParams{}, false, err
	}
	bucket, ok, err := queryInt(c, "bucket", 300)
	if !ok {
		return service.PortHistoryParams{}, false, err
	}
	port, ok, err := queryInt(c, "port", -1)
	if !ok {
		return service.PortHistoryParams{}, false, err
	}
	if port < -1 || port > 65535 {
		return service.PortHistoryParams{}, false, respondError(c, http.StatusBadRequest, "无效的port参数")
	}

	return service.PortHistoryParams{
		Start:  int64(start),
		End:    int64(end),
		Bucket: int64(bucket),
		IP:     c.QueryParam("ip"),
		Proto:  c.QueryParam("proto"),
		Port:   port,
	}, true, nil
}

// handleTrafficPortHistory 端口维度历史趋势
func (s *Server) handleTrafficPortHistory(c echo.Context) error {
	params, ok, err := parsePortHistoryParams(c)
	if !ok {
		return err
	}

	points, err := s.netService.TrafficPortHistory(params)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, points)
}

// handleTrafficPortHistoryTop 端口维度流量排行
func (s *Server) handleTrafficPortHistoryTop(c echo.Context) error {
	params, ok, err := parsePortHistoryParams(c)
	if !ok {
		return err
	}
	limit, ok, err := queryInt(c, "limit", 10)
	if !ok {
		return err
	}

	entries, err := s.netService.TrafficPortHistoryTop(params, limit)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, entries)
}

// handleTrafficProtoHistory 协议维度历史趋势
func (s *Server) handleTrafficProtoHistory(c echo.Context) error {
	params, ok, err := parsePortHistoryParams(c)
	if !ok {
		return err
	}

	points, err := s.netService.TrafficProtoHistory(params)
	if err != nil {
		return respondServiceError(c, err)
	}
	return respondSuccess(c, points)
}
