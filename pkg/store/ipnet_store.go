package store

import (
	"time"

	"gorm.io/gorm"
)

// IpNetStore 处理 IpNet 表的数据库操作
type IpNetStore struct {
	db *gorm.DB
}

// NewIpNetStore 创建新的 IpNetStore 实例
func NewIpNetStore(db *gorm.DB) *IpNetStore {
	return &IpNetStore{db: db}
}

// Create 创建新的 IP 网络记录
func (s *IpNetStore) Create(ipnet string, groupID uint, action string) (*IpNet, error) {
	model := IpNet{
		IpNet:     ipnet,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		GroupID:   groupID,
		Action:    action,
	}

	err := s.db.Create(&model).Error
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// DeleteByID 根据ID删除IP网络记录
func (s *IpNetStore) DeleteByID(id uint) error {
	return s.db.Delete(&IpNet{}, id).Error
}

// ExistsByIpNetAndAction 检查指定IP网络和操作是否存在
func (s *IpNetStore) ExistsByIpNetAndAction(ipnet string, action string) bool {
	var count int64
	s.db.Model(&IpNet{}).Where("ip_net = ? AND action = ?", ipnet, action).Count(&count)
	return count > 0
}

// ExistsByIpNet 检查指定IP网络是否存在
func (s *IpNetStore) ExistsByIpNet(ipnet string) bool {
	var count int64
	s.db.Model(&IpNet{}).Where("ip_net = ?", ipnet).Count(&count)
	return count > 0
}

// ExistsById 检查指定ID是否存在
func (s *IpNetStore) ExistsById(id uint) bool {
	var count int64
	s.db.Model(&IpNet{}).Where("id = ?", id).Count(&count)
	return count > 0
}

// FindByID 根据ID查找IP网络记录
func (s *IpNetStore) FindByID(id uint) (*IpNet, error) {
	var model IpNet
	if err := s.db.First(&model, id).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// FindByIpNet 根据IP网络地址查找IP网络记录
func (s *IpNetStore) FindByIpNet(ipnet string) (*IpNet, error) {
	var model IpNet
	if err := s.db.Where("ip_net = ?", ipnet).First(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// FindAll 获取所有IP网络记录
func (s *IpNetStore) FindAll() ([]IpNet, error) {
	var models []IpNet
	if err := s.db.Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// FindByAction 根据操作类型查找IP网络记录
func (s *IpNetStore) FindByAction(action string) ([]IpNet, error) {
	var models []IpNet
	if err := s.db.Where("action = ?", action).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// FindByGroupID 根据组ID查找IP网络记录
func (s *IpNetStore) FindByGroupID(groupID uint) ([]IpNet, error) {
	var models []IpNet
	if err := s.db.Where("group_id = ?", groupID).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// UpdateAction 更新IP网络记录的操作
func (s *IpNetStore) UpdateAction(ipNetID uint, action string) error {
	return s.db.Model(&IpNet{}).Where("id = ?", ipNetID).Update("action", action).Error
}

// UpdateGroupID 更新IP网络记录的组ID
func (s *IpNetStore) UpdateGroupID(ipNetID uint, groupID uint) error {
	return s.db.Model(&IpNet{}).Where("id = ?", ipNetID).Update("group_id", groupID).Error
}

// UpdateGroupIDByIPNet 根据IP网络地址更新组ID
func (s *IpNetStore) UpdateGroupIDByIPNet(ipnet string, groupID uint) error {
	return s.db.Model(&IpNet{}).Where("ip_net = ?", ipnet).Update("group_id", groupID).Error
}

// RemoveFromGroup 将IP网络记录从组中移除
func (s *IpNetStore) RemoveFromGroup(ipnet string) error {
	return s.db.Model(&IpNet{}).Where("ip_net = ?", ipnet).Update("group_id", nil).Error
}

// BatchCreate 批量创建IP网络记录，使用事务确保整体成功或失败
func (s *IpNetStore) BatchCreate(ipnets []string, groupID uint, action string) ([]IpNet, error) {
	var allModels []IpNet
	now := time.Now()

	// 分批处理，每批最多1000条记录
	batchSize := 1000
	totalBatches := (len(ipnets) + batchSize - 1) / batchSize

	// 使用事务进行批量插入
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < totalBatches; i++ {
			start := i * batchSize
			end := start + batchSize
			if end > len(ipnets) {
				end = len(ipnets)
			}

			batchIpnets := ipnets[start:end]
			var batchModels []IpNet

			for _, ipnet := range batchIpnets {
				batchModels = append(batchModels, IpNet{
					IpNet:     ipnet,
					CreatedAt: now,
					UpdatedAt: now,
					GroupID:   groupID,
					Action:    action,
				})
			}

			if err := tx.Create(&batchModels).Error; err != nil {
				return err
			}

			allModels = append(allModels, batchModels...)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return allModels, nil
}

// FindByIpNets 根据IP网络地址列表查找已存在的记录
func (s *IpNetStore) FindByIpNets(ipnets []string) ([]IpNet, error) {
	var allModels []IpNet

	// 分批查询，每批最多1000条记录
	batchSize := 1000
	totalBatches := (len(ipnets) + batchSize - 1) / batchSize

	for i := 0; i < totalBatches; i++ {
		start := i * batchSize
		end := start + batchSize
		if end > len(ipnets) {
			end = len(ipnets)
		}

		batchIpnets := ipnets[start:end]
		var batchModels []IpNet

		if err := s.db.Where("ip_net IN ?", batchIpnets).Find(&batchModels).Error; err != nil {
			return nil, err
		}

		allModels = append(allModels, batchModels...)
	}

	return allModels, nil
}
