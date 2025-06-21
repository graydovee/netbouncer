package store

import "time"

// IpModel 数据库模型
type BannedIpNet struct {
	ID        uint   `gorm:"primarykey"`
	IpNet     string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName 指定表名
func (BannedIpNet) TableName() string {
	return "banned_ip_net"
}
