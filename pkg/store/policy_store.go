package store

import (
	"gorm.io/gorm"
)

// PolicyStore 策略的存储
type PolicyStore struct {
	db *gorm.DB
}

func NewPolicyStore(db *gorm.DB) *PolicyStore {
	return &PolicyStore{db: db}
}

// FindAll 获取所有策略
func (s *PolicyStore) FindAll() ([]Policy, error) {
	var models []Policy
	if err := s.db.Order("id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// FindEnabled 获取所有启用中的策略
func (s *PolicyStore) FindEnabled() ([]Policy, error) {
	var models []Policy
	if err := s.db.Where("enabled = ?", true).Order("id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// FindByID 按ID查找策略
func (s *PolicyStore) FindByID(id uint) (*Policy, error) {
	var model Policy
	if err := s.db.First(&model, id).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// ExistsByName 检查同名策略是否已存在（可排除指定ID）
func (s *PolicyStore) ExistsByName(name string, excludeID uint) (bool, error) {
	var count int64
	query := s.db.Model(&Policy{}).Where("name = ?", name)
	if excludeID != 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create 创建策略
func (s *PolicyStore) Create(model *Policy) error {
	return s.db.Create(model).Error
}

// Update 更新策略（全字段，不含主键）
func (s *PolicyStore) Update(model *Policy) error {
	result := s.db.Model(model).Select(
		"name", "enabled",
		"direction", "protocol", "port",
		"rate_kbps", "total_mb", "conn_rate", "distinct_ports", "window_sec",
		"action", "limit_kbps", "burst_kbps", "ban_sec", "risk_score",
		"risk_ban_threshold", "risk_ban_sec", "cooldown_sec",
	).Updates(model)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetEnabled 启用/停用策略
func (s *PolicyStore) SetEnabled(id uint, enabled bool) error {
	result := s.db.Model(&Policy{}).Where("id = ?", id).Update("enabled", enabled)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 删除策略
func (s *PolicyStore) Delete(id uint) error {
	result := s.db.Delete(&Policy{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
