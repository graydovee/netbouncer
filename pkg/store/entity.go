package store

import "time"

const (
	ActionAllow = "allow"
	ActionBan   = "ban"
)

// 封禁/加白的生效方向
const (
	DirectionIn   = "in"
	DirectionOut  = "out"
	DirectionBoth = "both"
)

// 策略动作
const (
	PolicyActionRateLimit = "rate_limit" // 内核 hashlimit 限速
	PolicyActionBan       = "ban"        // 触发后临时封禁
	PolicyActionMark      = "mark"       // 仅标记风险
)

// 策略匹配的协议取值
const (
	ProtocolAny = "any"
	ProtocolTCP = "tcp"
	ProtocolUDP = "udp"
)

// IpModel 数据库模型
type IpNet struct {
	ID        uint   `gorm:"primarykey"`
	IpNet     string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	GroupID   uint       `gorm:"index"`
	Action    string     `gorm:"type:varchar(10);not null;index"`
	Direction string     `gorm:"type:varchar(8);not null;default:in"`      // ban 生效方向: in/out/both
	ExpiresAt *time.Time `gorm:"index"`                                    // 非 nil 表示临时封禁，到期自动解封
	Source    string     `gorm:"type:varchar(32);not null;default:manual"` // 规则来源: manual / policy:<id>
}

// TableName 指定表名
func (IpNet) TableName() string {
	return "banned_ip_net"
}

// IsExpired 临时封禁是否已过期（永久规则恒为 false）
func (i *IpNet) IsExpired(now time.Time) bool {
	return i.ExpiresAt != nil && i.ExpiresAt.Before(now)
}

type IpNetGroup struct {
	ID          uint   `gorm:"primarykey"`
	Name        string `gorm:"uniqueIndex;not null"`
	Description string `gorm:"type:text"`
	IsDefault   bool   `gorm:"default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (IpNetGroup) TableName() string {
	return "banned_ip_net_group"
}

// TrafficSample 流量历史采样点（raw 层，IP 维度）：记录一个采样间隔内某远程IP的流量增量
// （bytes/packets 为该区间内的新增量，非累计值），由采样器定时写入，保留 24h 后滚动进聚合层
type TrafficSample struct {
	ID          int64  `gorm:"primarykey;autoIncrement"`
	RemoteIP    string `gorm:"size:64;not null;index:idx_traffic_ip_ts,priority:1"`
	Ts          int64  `gorm:"not null;index:idx_traffic_ip_ts,priority:2;index:idx_traffic_ts"`
	BytesIn     uint64 `gorm:"not null;default:0"`
	BytesOut    uint64 `gorm:"not null;default:0"`
	PacketsIn   uint64 `gorm:"not null;default:0"`
	PacketsOut  uint64 `gorm:"not null;default:0"`
	Connections int    `gorm:"not null;default:0"`
}

func (TrafficSample) TableName() string {
	return "traffic_samples"
}

// TrafficPortSample 流量历史采样点（raw 层，IP×协议×端口维度）：区间增量，保留 6h 后滚动进聚合层。
// 端口为目标端口（收包=本机服务端口，发包=对端服务端口），端口 0 表示"其他/未分类"
type TrafficPortSample struct {
	ID         int64  `gorm:"primarykey;autoIncrement"`
	RemoteIP   string `gorm:"size:64;not null;index:idx_port_sample_dim,priority:1"`
	Proto      string `gorm:"size:16;not null;index:idx_port_sample_dim,priority:2"`
	Port       int    `gorm:"not null;index:idx_port_sample_dim,priority:3"`
	Ts         int64  `gorm:"not null;index:idx_port_sample_dim,priority:4;index:idx_port_sample_ts"`
	BytesIn    uint64 `gorm:"not null;default:0"`
	BytesOut   uint64 `gorm:"not null;default:0"`
	PacketsIn  uint64 `gorm:"not null;default:0"`
	PacketsOut uint64 `gorm:"not null;default:0"`
}

func (TrafficPortSample) TableName() string {
	return "traffic_port_samples"
}

// TrafficIpRollup IP 维度的降采样聚合行（bucket_size: 600=10分钟层, 3600=1小时层）
type TrafficIpRollup struct {
	ID         int64  `gorm:"primarykey;autoIncrement"`
	BucketSize int    `gorm:"not null;index:idx_ip_rollup_dim,priority:1"`
	BucketTs   int64  `gorm:"not null;index:idx_ip_rollup_dim,priority:2"`
	RemoteIP   string `gorm:"size:64;not null;index:idx_ip_rollup_dim,priority:3"`
	BytesIn    uint64 `gorm:"not null;default:0"`
	BytesOut   uint64 `gorm:"not null;default:0"`
	PacketsIn  uint64 `gorm:"not null;default:0"`
	PacketsOut uint64 `gorm:"not null;default:0"`
	ConnMax    int    `gorm:"not null;default:0"` // 桶内连接数峰值
}

func (TrafficIpRollup) TableName() string {
	return "traffic_ip_rollup"
}

// TrafficPortRollup 端口维度的降采样聚合行（bucket_size: 600=10分钟层, 3600=1小时层）
type TrafficPortRollup struct {
	ID         int64  `gorm:"primarykey;autoIncrement"`
	BucketSize int    `gorm:"not null;index:idx_port_rollup_dim,priority:1"`
	BucketTs   int64  `gorm:"not null;index:idx_port_rollup_dim,priority:2"`
	RemoteIP   string `gorm:"size:64;not null;index:idx_port_rollup_dim,priority:3"`
	Proto      string `gorm:"size:16;not null;index:idx_port_rollup_dim,priority:4"`
	Port       int    `gorm:"not null;index:idx_port_rollup_dim,priority:5"`
	BytesIn    uint64 `gorm:"not null;default:0"`
	BytesOut   uint64 `gorm:"not null;default:0"`
	PacketsIn  uint64 `gorm:"not null;default:0"`
	PacketsOut uint64 `gorm:"not null;default:0"`
}

func (TrafficPortRollup) TableName() string {
	return "traffic_port_rollup"
}

// Policy 流量策略：按匹配条件评估流量指标，超阈值执行限速/临时封禁/风险标记
type Policy struct {
	ID        uint   `gorm:"primarykey"`
	Name      string `gorm:"uniqueIndex;size:64;not null"`
	Enabled   bool   `gorm:"not null;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time

	// 匹配条件
	Direction string `gorm:"size:8;not null;default:both"` // in|out|both: 统计并管控的流量方向
	Protocol  string `gorm:"size:8;not null;default:any"`  // any|tcp|udp
	Port      int    `gorm:"not null;default:0"`           // 0 = 任意端口

	// 触发条件（任一超限即触发，0 = 不启用）
	RateKBps      float64 `gorm:"not null;default:0"` // 速率阈值 KB/s（窗口均速）
	TotalMB       float64 `gorm:"not null;default:0"` // 窗口累计流量阈值 MB
	ConnRate      int     `gorm:"not null;default:0"` // 窗口内新建连接数阈值
	DistinctPorts int     `gorm:"not null;default:0"` // 窗口内触碰的不同端口数阈值（仅 Port=0 时有效，扫描检测）
	WindowSec     int     `gorm:"not null;default:300"`

	// 动作
	Action    string  `gorm:"size:16;not null"`   // rate_limit|ban|mark
	LimitKBps float64 `gorm:"not null;default:0"` // rate_limit 的限速值 KB/s
	BurstKBps float64 `gorm:"not null;default:0"` // rate_limit 的突发容量 KB/s，0 = 2×LimitKBps
	BanSec    int     `gorm:"not null;default:0"` // ban 的禁用时长（秒）
	RiskScore int     `gorm:"not null;default:1"` // 每次触发累加的风险分

	// 风险升级：风险分达到阈值后自动封禁
	RiskBanThreshold int `gorm:"not null;default:0"` // 0 = 不启用
	RiskBanSec       int `gorm:"not null;default:0"`
	CooldownSec      int `gorm:"not null;default:300"` // 同一 IP 两次触发的最小间隔（秒）
}

func (Policy) TableName() string {
	return "policies"
}

// RiskEvent 风险事件：策略触发/自动升级封禁的记录
type RiskEvent struct {
	ID           int64  `gorm:"primarykey;autoIncrement"`
	RemoteIP     string `gorm:"size:64;not null;index:idx_risk_ip_ts,priority:1"`
	Ts           int64  `gorm:"not null;index:idx_risk_ip_ts,priority:2;index:idx_risk_ts"`
	PolicyID     uint   `gorm:"not null;default:0;index"`
	PolicyName   string `gorm:"size:64;not null"`
	TriggerValue string `gorm:"size:128;not null"` // 触发时的观测值描述，如 "12.5MB/s > 10MB/s"
	Action       string `gorm:"size:16;not null"`  // mark|ban|auto_ban
	Score        int    `gorm:"not null;default:0"`
}

func (RiskEvent) TableName() string {
	return "risk_events"
}
