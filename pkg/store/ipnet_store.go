package store

import (
	"time"

	"gorm.io/gorm"
)

// IpNetFilter IP规则列表的筛选与分页参数
type IpNetFilter struct {
	GroupID uint   // 为 0 时不过滤
	Action  string // 为空时不过滤
	Search  string // 按 ip_net 模糊匹配，为空时不过滤
	Offset  int
	Limit   int // 为 0 时表示不分页
}

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

// DeleteByIDs 根据ID列表批量删除，返回实际删除的条数
func (s *IpNetStore) DeleteByIDs(ids []uint) (int64, error) {
	result := s.db.Where("id IN ?", ids).Delete(&IpNet{})
	return result.RowsAffected, result.Error
}

// ExistsByIpNet 检查指定IP网络是否存在
func (s *IpNetStore) ExistsByIpNet(ipnet string) bool {
	var count int64
	s.db.Model(&IpNet{}).Where("ip_net = ?", ipnet).Count(&count)
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

// FindByIDs 根据ID列表查找IP网络记录
func (s *IpNetStore) FindByIDs(ids []uint) ([]IpNet, error) {
	var models []IpNet
	if err := s.db.Where("id IN ?", ids).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
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

// FindByFilter 按条件分页查询IP网络记录，返回记录与总数
func (s *IpNetStore) FindByFilter(filter IpNetFilter) ([]IpNet, int64, error) {
	query := s.db.Model(&IpNet{})
	if filter.GroupID != 0 {
		query = query.Where("group_id = ?", filter.GroupID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Search != "" {
		query = query.Where("ip_net LIKE ?", "%"+filter.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var models []IpNet
	if err := query.Order("id ASC").Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
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
