package store

import (
	"fmt"

	"github.com/graydovee/netbouncer/pkg/config"
)

type Store struct {
	IpNetStore         *IpNetStore
	IpNetGroupStore    *IpNetGroupStore
	TrafficSampleStore *TrafficSampleStore
	TrafficPortStore   *TrafficPortSampleStore
	TrafficRollupStore *TrafficRollupStore
	PolicyStore        *PolicyStore
	RiskEventStore     *RiskEventStore
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
		TrafficSample{},
		TrafficPortSample{},
		TrafficIpRollup{},
		TrafficPortRollup{},
		Policy{},
		RiskEvent{},
	); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return &Store{
		IpNetStore:         NewIpNetStore(db),
		IpNetGroupStore:    NewIpNetGroupStore(db),
		TrafficSampleStore: NewTrafficSampleStore(db),
		TrafficPortStore:   NewTrafficPortSampleStore(db),
		TrafficRollupStore: NewTrafficRollupStore(db),
		PolicyStore:        NewPolicyStore(db),
		RiskEventStore:     NewRiskEventStore(db),
	}, nil
}
