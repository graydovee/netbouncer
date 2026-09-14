package service

import (
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

// TrafficData 实时流量数据（按远程 IP）
type TrafficData struct {
	RemoteIP        string      `json:"remote_ip"`         // 远程IP
	LocalIP         string      `json:"local_ip"`          // 本地IP
	TotalBytesIn    uint64      `json:"total_bytes_in"`    // 总接收字节数
	TotalBytesOut   uint64      `json:"total_bytes_out"`   // 总发送字节数
	TotalPacketsIn  uint64      `json:"total_packets_in"`  // 总接收包数
	TotalPacketsOut uint64      `json:"total_packets_out"` // 总发送包数
	BytesInPerSec   float64     `json:"bytes_in_per_sec"`  // 每秒接收字节数
	BytesOutPerSec  float64     `json:"bytes_out_per_sec"` // 每秒发送字节数
	Connections     int         `json:"connections"`       // 连接数
	FirstSeen       string      `json:"first_seen"`        // 首次发现时间
	LastSeen        string      `json:"last_seen"`         // 最后活动时间
	IsBanned        bool        `json:"is_banned"`         // 是否被ban（含被网段规则覆盖的情况）
	RuleAction      string      `json:"rule_action"`       // 精确命中该IP的规则动作：""/ban/allow（网段覆盖时为空）
	RuleID          uint        `json:"rule_id"`           // 精确命中规则的ID，0 表示无精确规则
	BannedUntil     string      `json:"banned_until"`      // 临时封禁到期时间（RFC3339），空 = 永久封禁或未封禁
	RiskScore       int         `json:"risk_score"`        // 风险分（统计窗口内策略触发累加）
	RiskLevel       string      `json:"risk_level"`        // 风险等级: none|low|medium|high
	Protocols       []ProtoStat `json:"protocols"`         // 协议维度累计统计
	Ports           []PortStat  `json:"ports"`             // 端口维度累计统计（按流量取前若干条）
}

// ProtoStat 单协议的累计流量
type ProtoStat struct {
	Proto      string `json:"proto"`
	BytesIn    uint64 `json:"bytes_in"`
	BytesOut   uint64 `json:"bytes_out"`
	PacketsIn  uint64 `json:"packets_in"`
	PacketsOut uint64 `json:"packets_out"`
}

// PortStat 单协议+端口的累计流量
type PortStat struct {
	Proto      string `json:"proto"`
	Port       int    `json:"port"` // 0 表示"其他/未分类"
	BytesIn    uint64 `json:"bytes_in"`
	BytesOut   uint64 `json:"bytes_out"`
	PacketsIn  uint64 `json:"packets_in"`
	PacketsOut uint64 `json:"packets_out"`
	Conns      int    `json:"conns"` // 该端口累计新建连接数
}

// PortTraffic 实时端口排行条目（跨 IP 聚合）
type PortTraffic struct {
	Proto          string   `json:"proto"`
	Port           int      `json:"port"` // 0 表示"其他/未分类"
	BytesIn        uint64   `json:"bytes_in"`
	BytesOut       uint64   `json:"bytes_out"`
	BytesInPerSec  float64  `json:"bytes_in_per_sec"`
	BytesOutPerSec float64  `json:"bytes_out_per_sec"`
	IPCount        int      `json:"ip_count"`    // 活跃客户端 IP 数
	NewConns       int      `json:"new_conns"`   // 评估间隔内新建连接数
	TopClients     []string `json:"top_clients"` // 按流量取前若干个客户端 IP
}

// IpNet IP规则
type IpNet struct {
	ID        uint     `json:"id"`
	IpNet     string   `json:"ip_net"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	Group     *IpGroup `json:"group"`
	Action    string   `json:"action"`
	Direction string   `json:"direction"`  // ban 生效方向: in/out/both
	ExpiresAt string   `json:"expires_at"` // 临时封禁到期时间（RFC3339），空 = 永久
	Source    string   `json:"source"`     // 规则来源: manual / policy:<id>
}

// IpGroup IP分组
type IpGroup struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	IsDefault   bool   `json:"is_default"`
	IPCount     int64  `json:"ip_count"` // 组内IP规则数量
}

// Policy 策略
type Policy struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`

	// 匹配条件
	Direction string `json:"direction"` // in|out|both
	Protocol  string `json:"protocol"`  // any|tcp|udp
	Port      int    `json:"port"`      // 0 = 任意端口

	// 触发条件（0 = 不启用）
	RateKBps      float64 `json:"rate_kbps"`
	TotalMB       float64 `json:"total_mb"`
	ConnRate      int     `json:"conn_rate"`
	DistinctPorts int     `json:"distinct_ports"`
	WindowSec     int     `json:"window_sec"`

	// 动作
	Action    string  `json:"action"` // rate_limit|ban|mark
	LimitKBps float64 `json:"limit_kbps"`
	BurstKBps float64 `json:"burst_kbps"`
	BanSec    int     `json:"ban_sec"`
	RiskScore int     `json:"risk_score"`

	// 风险升级
	RiskBanThreshold int `json:"risk_ban_threshold"`
	RiskBanSec       int `json:"risk_ban_sec"`
	CooldownSec      int `json:"cooldown_sec"`
}

func convertToPolicy(p *store.Policy) Policy {
	return Policy{
		ID:               p.ID,
		Name:             p.Name,
		Enabled:          p.Enabled,
		CreatedAt:        p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        p.UpdatedAt.Format(time.RFC3339),
		Direction:        p.Direction,
		Protocol:         p.Protocol,
		Port:             p.Port,
		RateKBps:         p.RateKBps,
		TotalMB:          p.TotalMB,
		ConnRate:         p.ConnRate,
		DistinctPorts:    p.DistinctPorts,
		WindowSec:        p.WindowSec,
		Action:           p.Action,
		LimitKBps:        p.LimitKBps,
		BurstKBps:        p.BurstKBps,
		BanSec:           p.BanSec,
		RiskScore:        p.RiskScore,
		RiskBanThreshold: p.RiskBanThreshold,
		RiskBanSec:       p.RiskBanSec,
		CooldownSec:      p.CooldownSec,
	}
}

// RiskEvent 风险事件
type RiskEvent struct {
	ID           int64  `json:"id"`
	RemoteIP     string `json:"remote_ip"`
	Ts           int64  `json:"ts"`
	PolicyID     uint   `json:"policy_id"`
	PolicyName   string `json:"policy_name"`
	TriggerValue string `json:"trigger_value"`
	Action       string `json:"action"` // mark|ban|auto_ban
	Score        int    `json:"score"`
}

func convertToRiskEvent(e *store.RiskEvent) RiskEvent {
	return RiskEvent{
		ID:           e.ID,
		RemoteIP:     e.RemoteIP,
		Ts:           e.Ts,
		PolicyID:     e.PolicyID,
		PolicyName:   e.PolicyName,
		TriggerValue: e.TriggerValue,
		Action:       e.Action,
		Score:        e.Score,
	}
}

// RiskEventListResult 风险事件分页结果
type RiskEventListResult struct {
	Items []RiskEvent `json:"items"`
	Total int64       `json:"total"`
}
