package store

import "time"

const (
	ActionAllow = "allow"
	ActionBan   = "ban"
)

// IpModel 数据库模型
type IpNet struct {
	ID        uint   `gorm:"primarykey"`
	IpNet     string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	GroupID   uint   `gorm:"index"`
	Action    string `gorm:"type:varchar(10);not null;index"`
}

// TableName 指定表名
func (IpNet) TableName() string {
	return "banned_ip_net"
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

// TrafficSample 流量历史采样点：记录一个采样间隔内某远程IP的流量增量
// （bytes/packets 为该区间内的新增量，非进程累计值），由采样器定时写入
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
