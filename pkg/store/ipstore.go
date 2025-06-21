package store

import (
	"sync"
	"time"

	"gorm.io/gorm"
)

type DbIpStore struct {
	db *gorm.DB
}

// NewDbIpStore 创建数据库存储实例
func NewDbIpStore(db *gorm.DB) (*DbIpStore, error) {
	// 自动迁移数据库表结构
	if err := db.AutoMigrate(BannedIpNet{}); err != nil {
		return nil, err
	}

	return &DbIpStore{db: db}, nil
}

func (s *DbIpStore) AddIpBlacklist(ipnet string) error {
	model := BannedIpNet{
		IpNet:     ipnet,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 使用 Upsert 操作，如果IP已存在则更新，否则插入
	return s.db.Where(BannedIpNet{IpNet: ipnet}).
		Assign(model).
		FirstOrCreate(&model).Error
}

func (s *DbIpStore) RemoveIpBlacklist(ipnet string) error {
	return s.db.Where("ip_net = ?", ipnet).Delete(&BannedIpNet{}).Error
}

func (s *DbIpStore) IsInBlacklist(ipnet string) bool {
	var count int64
	s.db.Model(&BannedIpNet{}).Where("ip_net = ?", ipnet).Count(&count)
	return count > 0
}

func (s *DbIpStore) GetBlacklist() ([]BannedIpNet, error) {
	var models []BannedIpNet
	if err := s.db.Find(&models).Error; err != nil {
		return nil, err
	}

	return models, nil
}

// ------------------------------------------------------------

type MemoryIpStore struct {
	mu        sync.Mutex
	blacklist map[string]BannedIpNet
}

func NewMemoryIpStore() *MemoryIpStore {
	return &MemoryIpStore{
		blacklist: make(map[string]BannedIpNet),
	}
}

func (s *MemoryIpStore) AddIpBlacklist(ipnet string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blacklist[ipnet] = BannedIpNet{IpNet: ipnet}
	return nil
}

func (s *MemoryIpStore) RemoveIpBlacklist(ipnet string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.blacklist, ipnet)
	return nil
}

func (s *MemoryIpStore) IsInBlacklist(ipnet string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.blacklist[ipnet]
	return ok
}

func (s *MemoryIpStore) GetBlacklist() ([]BannedIpNet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ips := make([]BannedIpNet, 0, len(s.blacklist))
	for _, ip := range s.blacklist {
		ips = append(ips, ip)
	}
	return ips, nil
}
