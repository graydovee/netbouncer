package store

import (
	"fmt"
	"gorm.io/gorm"

	"github.com/graydovee/netbouncer/pkg/config"
)

type Store struct {
	db              *gorm.DB
	IpNetStore      *IpNetStore
	IpNetGroupStore *IpNetGroupStore
	PolicyStore     *PolicyStore
	RiskEventStore  *RiskEventStore
}

func NewStore(cfg *config.DatabaseConfig) (*Store, error) {

	// 创建数据库连接
	db, err := NewDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接失败: %w", err)
	}

	// 自动迁移数据库表结构
	if err := db.AutoMigrate(
		IpNet{},
		IpNetGroup{},
		Policy{},
		RiskEvent{},
	); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return &Store{
		db:              db,
		IpNetStore:      NewIpNetStore(db),
		IpNetGroupStore: NewIpNetGroupStore(db),
		PolicyStore:     NewPolicyStore(db),
		RiskEventStore:  NewRiskEventStore(db),
	}, nil
}

func (s *Store) Close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
