package store

import (
	"fmt"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
)

type Ip struct {
	Id        int64     `json:"id"`
	Ip        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type IpStore interface {
	AddIpBlacklist(ip Ip) error
	RemoveIpBlacklist(ip Ip) error
	IsInBlacklist(ip Ip) bool
	GetBlacklist() ([]Ip, error)
}

var (
	_ IpStore = (*MemoryIpStore)(nil)
	_ IpStore = (*DbIpStore)(nil)
)

// NewIpStore 根据配置创建IP存储实例
func NewIpStore(cfg *config.Config) (IpStore, error) {
	switch cfg.GetStorageType() {
	case "memory":
		return NewMemoryIpStore(), nil
	case "database":
		db, err := NewDatabase(cfg.GetDatabaseConfig())
		if err != nil {
			return nil, err
		}
		return NewDbIpStore(db)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.GetStorageType())
	}
}
