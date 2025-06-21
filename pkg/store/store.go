package store

import (
	"fmt"

	"github.com/graydovee/netbouncer/pkg/config"
)

type Store struct {
	IpNetStore      *IpNetStore
	IpNetGroupStore *IpNetGroupStore
}

func NewStore(cfg *config.DatabaseConfig) (*Store, error) {

	// 创建数据库连接
	db, err := NewDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}

	// 自动迁移数据库表结构
	if err := db.AutoMigrate(IpNet{}, IpNetGroup{}); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	ipNetStore := NewIpNetStore(db)
	ipNetGroupStore := NewIpNetGroupStore(db)

	return &Store{
		IpNetStore:      ipNetStore,
		IpNetGroupStore: ipNetGroupStore,
	}, nil
}
