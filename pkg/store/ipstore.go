package store

import (
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"gorm.io/gorm"
)

// NewIpStore 根据配置创建IP存储实例
func NewIpStore(cfg *config.DatabaseConfig) (*IpStore, error) {
	db, err := NewDatabase(cfg)
	if err != nil {
		return nil, err
	}

	// 自动迁移数据库表结构
	if err := db.AutoMigrate(BannedIpNet{}, BannedIpNetGroup{}); err != nil {
		return nil, err
	}

	return &IpStore{db: db}, nil
}

type IpStore struct {
	db *gorm.DB
}

func (s *IpStore) AddIpBlacklist(ipnet string, groupId uint) error {
	model := BannedIpNet{
		IpNet:     ipnet,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		GroupID:   groupId,
	}

	// 使用 Upsert 操作，如果IP已存在则更新，否则插入
	return s.db.Where(BannedIpNet{IpNet: ipnet}).
		Assign(model).
		FirstOrCreate(&model).Error
}

func (s *IpStore) RemoveIpBlacklist(ipnet string) error {
	return s.db.Where("ip_net = ?", ipnet).Delete(&BannedIpNet{}).Error
}

func (s *IpStore) IsInBlacklist(ipnet string) bool {
	var count int64
	s.db.Model(&BannedIpNet{}).Where("ip_net = ?", ipnet).Count(&count)
	return count > 0
}

func (s *IpStore) GetBlacklist() ([]BannedIpNet, error) {
	var models []BannedIpNet
	if err := s.db.Find(&models).Error; err != nil {
		return nil, err
	}

	return models, nil
}

func (s *IpStore) UpdateIpGroup(ipnet string, groupId uint) error {
	return s.db.Model(&BannedIpNet{}).Where("ip_net = ?", ipnet).Update("group_id", groupId).Error
}

func (s *IpStore) GetDefaultGroup() (*BannedIpNetGroup, error) {
	var group BannedIpNetGroup
	if err := s.db.Where("is_default = ?", true).First(&group).Error; err != nil {
		return nil, err
	}

	return &group, nil
}

func (s *IpStore) CreateGroup(name string, description string) (*BannedIpNetGroup, error) {
	group := BannedIpNetGroup{
		Name:        name,
		Description: description,
		IsDefault:   false,
	}

	if err := s.db.Create(&group).Error; err != nil {
		return nil, err
	}

	return &group, nil
}

func (s *IpStore) GetGroup(id uint) (*BannedIpNetGroup, error) {
	var group BannedIpNetGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return nil, err
	}

	return &group, nil
}

func (s *IpStore) GetGroupByIds(ids ...uint) ([]BannedIpNetGroup, error) {
	var groups []BannedIpNetGroup
	if err := s.db.Where("id IN (?)", ids).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *IpStore) GetBlacklistByGroup(groupId uint) ([]BannedIpNet, error) {
	var ips []BannedIpNet
	if err := s.db.Where("group_id = ?", groupId).Find(&ips).Error; err != nil {
		return nil, err
	}
	return ips, nil
}

func (s *IpStore) GetGroupByName(name string) (*BannedIpNetGroup, error) {
	var group BannedIpNetGroup
	if err := s.db.Where("name = ?", name).First(&group).Error; err != nil {
		return nil, err
	}

	return &group, nil
}

func (s *IpStore) GetAllGroups() ([]BannedIpNetGroup, error) {
	var groups []BannedIpNetGroup
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, err
	}

	return groups, nil
}

func (s *IpStore) UpdateGroup(id uint, name string, description string) (*BannedIpNetGroup, error) {
	group := BannedIpNetGroup{
		ID:          id,
		Name:        name,
		Description: description,
	}

	if err := s.db.Save(&group).Error; err != nil {
		return nil, err
	}

	return &group, nil
}

func (s *IpStore) DeleteGroup(id uint) error {
	return s.db.Delete(&BannedIpNetGroup{}, id).Error
}

func (s *IpStore) AddIpToGroup(ipnet string, groupId uint) error {
	return s.db.Model(&BannedIpNet{}).Where("ip_net = ?", ipnet).Update("group_id", groupId).Error
}

func (s *IpStore) RemoveIpFromGroup(ipnet string, groupId uint) error {
	return s.db.Model(&BannedIpNet{}).Where("ip_net = ?", ipnet).Update("group_id", nil).Error
}

func (s *IpStore) SetDefaultGroup(groupId uint) error {
	// 事务： 1. 如果默认组存在，设置默认组为非默认组 2. 设置新的默认组
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 先将所有组设置为非默认
		if err := tx.Model(&BannedIpNetGroup{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		// 设置指定的组为默认组
		return tx.Model(&BannedIpNetGroup{}).Where("id = ?", groupId).Update("is_default", true).Error
	})
}
