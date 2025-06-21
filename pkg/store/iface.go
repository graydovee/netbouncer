package store

import (
	"fmt"

	"github.com/graydovee/netbouncer/pkg/config"
)

type IpStore interface {
	AddIpBlacklist(ipnet string) error
	RemoveIpBlacklist(ipnet string) error
	IsInBlacklist(ipnet string) bool
	GetBlacklist() ([]BannedIpNet, error)
}

var (
	_ IpStore = (*MemoryIpStore)(nil)
	_ IpStore = (*DbIpStore)(nil)
)

// NewIpStore 根据配置创建IP存储实例
func NewIpStore(cfg *config.StorageConfig) (IpStore, error) {
	switch cfg.Type {
	case "memory":
		return NewMemoryIpStore(), nil
	case "database":
		db, err := NewDatabase(&cfg.Database)
		if err != nil {
			return nil, err
		}
		return NewDbIpStore(db)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}
