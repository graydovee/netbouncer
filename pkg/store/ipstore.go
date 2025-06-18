package store

import (
	"sync"
	"time"

	"gorm.io/gorm"
)

// IpModel 数据库模型
type IpModel struct {
	ID        uint   `gorm:"primarykey"`
	IP        string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName 指定表名
func (IpModel) TableName() string {
	return "ip_blacklist"
}

type DbIpStore struct {
	db *gorm.DB
}

// NewDbIpStore 创建数据库存储实例
func NewDbIpStore(db *gorm.DB) (*DbIpStore, error) {
	// 自动迁移数据库表结构
	if err := db.AutoMigrate(&IpModel{}); err != nil {
		return nil, err
	}

	return &DbIpStore{db: db}, nil
}

func (s *DbIpStore) AddIpBlacklist(ip Ip) error {
	model := IpModel{
		IP:        ip.Ip,
		CreatedAt: ip.CreatedAt,
		UpdatedAt: ip.UpdatedAt,
	}

	// 使用 Upsert 操作，如果IP已存在则更新，否则插入
	return s.db.Where(IpModel{IP: ip.Ip}).
		Assign(model).
		FirstOrCreate(&model).Error
}

func (s *DbIpStore) RemoveIpBlacklist(ip Ip) error {
	return s.db.Where("ip = ?", ip.Ip).Delete(&IpModel{}).Error
}

func (s *DbIpStore) IsInBlacklist(ip Ip) bool {
	var count int64
	s.db.Model(&IpModel{}).Where("ip = ?", ip.Ip).Count(&count)
	return count > 0
}

func (s *DbIpStore) GetBlacklist() ([]Ip, error) {
	var models []IpModel
	if err := s.db.Find(&models).Error; err != nil {
		return nil, err
	}

	ips := make([]Ip, len(models))
	for i, model := range models {
		ips[i] = Ip{
			Id:        int64(model.ID),
			Ip:        model.IP,
			CreatedAt: model.CreatedAt,
			UpdatedAt: model.UpdatedAt,
		}
	}

	return ips, nil
}

// ------------------------------------------------------------

type MemoryIpStore struct {
	mu        sync.Mutex
	blacklist map[string]Ip
}

func NewMemoryIpStore() *MemoryIpStore {
	return &MemoryIpStore{
		blacklist: make(map[string]Ip),
	}
}

func (s *MemoryIpStore) AddIpBlacklist(ip Ip) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blacklist[ip.Ip] = ip
	return nil
}

func (s *MemoryIpStore) RemoveIpBlacklist(ip Ip) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.blacklist, ip.Ip)
	return nil
}

func (s *MemoryIpStore) IsInBlacklist(ip Ip) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.blacklist[ip.Ip]
	return ok
}

func (s *MemoryIpStore) GetBlacklist() ([]Ip, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ips := make([]Ip, 0, len(s.blacklist))
	for _, ip := range s.blacklist {
		ips = append(ips, ip)
	}
	return ips, nil
}
