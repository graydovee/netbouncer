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
