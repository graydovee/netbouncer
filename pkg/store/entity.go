package store

import "time"

// IpModel 数据库模型
type BannedIpNet struct {
	ID        uint   `gorm:"primarykey"`
	IpNet     string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	GroupID   uint `gorm:"index"`
}

// TableName 指定表名
func (BannedIpNet) TableName() string {
	return "banned_ip_net"
}

type BannedIpNetGroup struct {
	ID          uint   `gorm:"primarykey"`
	Name        string `gorm:"uniqueIndex;not null"`
	Description string `gorm:"type:text"`
	IsDefault   bool   `gorm:"default:false"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (BannedIpNetGroup) TableName() string {
	return "banned_ip_net_group"
}
