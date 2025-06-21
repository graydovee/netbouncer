package store

import (
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

// FindByIDs 根据ID列表查找IP网络组
func (s *IpNetGroupStore) FindByIDs(ids ...uint) ([]IpNetGroup, error) {
	var groups []IpNetGroup
	if err := s.db.Where("id IN (?)", ids).Find(&groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

// FindByName 根据名称查找IP网络组
func (s *IpNetGroupStore) FindByName(name string) (*IpNetGroup, error) {
	var group IpNetGroup
	if err := s.db.Where("name = ?", name).First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
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

// Update 更新IP网络组信息
func (s *IpNetGroupStore) Update(id uint, name string, description string) (*IpNetGroup, error) {
	group := IpNetGroup{
		ID:          id,
		Name:        name,
		Description: description,
	}

	if err := s.db.Save(&group).Error; err != nil {
		return nil, err
	}

	return &group, nil
}

// DeleteByID 根据ID删除IP网络组
func (s *IpNetGroupStore) DeleteByID(id uint) error {
	return s.db.Delete(&IpNetGroup{}, id).Error
}

// SetDefault 设置指定组为默认组
func (s *IpNetGroupStore) SetDefault(groupID uint) error {
	// 事务：1. 将所有组设置为非默认 2. 设置新的默认组
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 先将所有组设置为非默认
		if err := tx.Model(&IpNetGroup{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		// 设置指定的组为默认组
		return tx.Model(&IpNetGroup{}).Where("id = ?", groupID).Update("is_default", true).Error
	})
}
