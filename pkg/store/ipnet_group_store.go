package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// IpNetGroupStore 处理 IpNetGroup 表的数据库操作
type IpNetGroupStore struct {
	db *gorm.DB
}

// NewIpNetGroupStore 创建新的 IpNetGroupStore 实例
func NewIpNetGroupStore(db *gorm.DB) *IpNetGroupStore {
	return &IpNetGroupStore{db: db}
}

// Create 创建新的IP网络组
func (s *IpNetGroupStore) Create(name string, description string) (*IpNetGroup, error) {
	group := IpNetGroup{
		Name:        name,
		Description: description,
		IsDefault:   false,
	}

	if err := s.db.Create(&group).Error; err != nil {
		return nil, err
	}

	return &group, nil
}

// FindByID 根据ID查找IP网络组
func (s *IpNetGroupStore) FindByID(id uint) (*IpNetGroup, error) {
	var group IpNetGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// FindByName 根据名称查找IP网络组
func (s *IpNetGroupStore) FindByName(name string) (*IpNetGroup, error) {
	var group IpNetGroup
	if err := s.db.Where("name = ?", name).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// FindByNameExists 检查指定名称的组是否存在
func (s *IpNetGroupStore) FindByNameExists(name string) bool {
	var count int64
	s.db.Model(&IpNetGroup{}).Where("name = ?", name).Count(&count)
	return count > 0
}

// FindDefault 查找默认组
func (s *IpNetGroupStore) FindDefault() (*IpNetGroup, error) {
	var group IpNetGroup
	if err := s.db.Where("is_default = ?", true).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// FindAll 获取所有IP网络组
func (s *IpNetGroupStore) FindAll() ([]IpNetGroup, error) {
	var groups []IpNetGroup
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// Update 更新IP网络组信息（只更新名称与描述，不覆盖其他字段）
func (s *IpNetGroupStore) Update(id uint, name string, description string) (*IpNetGroup, error) {
	updates := map[string]any{
		"name":        name,
		"description": description,
		"updated_at":  time.Now(),
	}
	if err := s.db.Model(&IpNetGroup{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.FindByID(id)
}

// DeleteByID 根据ID删除IP网络组
func (s *IpNetGroupStore) DeleteByID(id uint) error {
	return s.db.Delete(&IpNetGroup{}, id).Error
}

// CountByGroupID 统计每个组下的IP数量，返回 groupID -> count
func (s *IpNetGroupStore) CountByGroupID() (map[uint]int64, error) {
	var rows []struct {
		GroupID uint  `gorm:"column:group_id"`
		Count   int64 `gorm:"column:cnt"`
	}
	if err := s.db.Model(&IpNet{}).
		Select("group_id, COUNT(*) AS cnt").
		Group("group_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[uint]int64, len(rows))
	for _, row := range rows {
		counts[row.GroupID] = row.Count
	}
	return counts, nil
}

// SetDefault 设置指定组为默认组
func (s *IpNetGroupStore) SetDefault(groupID uint) error {
	// 事务：1. 将所有组设置为非默认 2. 设置新的默认组
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 先将所有组设置为非默认
		if err := tx.Model(&IpNetGroup{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 设置指定的组为默认组
		return tx.Model(&IpNetGroup{}).Where("id = ?", groupID).Update("is_default", true).Error
	})
}
