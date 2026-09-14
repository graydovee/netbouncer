package store

import (
	"gorm.io/gorm"
)

// RiskEventStore 风险事件的存储
type RiskEventStore struct {
	db *gorm.DB
}

func NewRiskEventStore(db *gorm.DB) *RiskEventStore {
	return &RiskEventStore{db: db}
}

// InsertBatch 批量写入风险事件
func (s *RiskEventStore) InsertBatch(events []RiskEvent) error {
	if len(events) == 0 {
		return nil
	}
	return s.db.CreateInBatches(events, 1000).Error
}

// RiskEventFilter 风险事件查询条件
type RiskEventFilter struct {
	RemoteIP string // 为空时不过滤
	Offset   int
	Limit    int // 为 0 时表示不分页
}

// FindByFilter 分页查询风险事件，按时间倒序
func (s *RiskEventStore) FindByFilter(filter RiskEventFilter) ([]RiskEvent, int64, error) {
	query := s.db.Model(&RiskEvent{})
	if filter.RemoteIP != "" {
		query = query.Where("remote_ip = ?", filter.RemoteIP)
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

	var models []RiskEvent
	if err := query.Order("ts DESC, id DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
}

// SumScoresSince 统计时间窗口内各 IP 的风险分总和（ip 为空时统计全部 IP）
func (s *RiskEventStore) SumScoresSince(since int64, ip string) (map[string]int, error) {
	query := s.db.Model(&RiskEvent{}).
		Select("remote_ip, SUM(score) AS total_score").
		Where("ts >= ?", since).
		Group("remote_ip")
	if ip != "" {
		query = query.Where("remote_ip = ?", ip)
	}

	var rows []struct {
		RemoteIP   string `gorm:"column:remote_ip"`
		TotalScore int64  `gorm:"column:total_score"`
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	scores := make(map[string]int, len(rows))
	for _, row := range rows {
		scores[row.RemoteIP] = int(row.TotalScore)
	}
	return scores, nil
}

// Cleanup 删除保留期之前的事件，返回删除的行数
func (s *RiskEventStore) Cleanup(before int64) (int64, error) {
	result := s.db.Where("ts < ?", before).Delete(&RiskEvent{})
	return result.RowsAffected, result.Error
}
