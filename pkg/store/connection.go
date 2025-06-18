package store

import (
	"fmt"
	"log"
	"log/slog"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/graydovee/netbouncer/pkg/config"
)

// NewDatabase 根据配置创建数据库连接
func NewDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	var dsn string

	// 如果提供了DSN，直接使用
	if cfg.DSN != "" {
		dsn = cfg.DSN
	} else {
		// 根据驱动类型构建DSN
		switch cfg.Driver {
		case "sqlite":
			if cfg.Database == "" {
				cfg.Database = "netbouncer.db"
			}
			dsn = cfg.Database
		case "mysql":
			dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
				cfg.Username, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
		case "postgres":
			dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
				cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database)
		default:
			return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
		}
	}

	slog.Info("使用数据库", "dsn", dsn)

	var dialector gorm.Dialector
	switch cfg.Driver {
	case "sqlite":
		dialector = sqlite.Open(dsn)
	case "mysql":
		// 这里需要添加mysql驱动，暂时只支持sqlite
		return nil, fmt.Errorf("mysql driver not implemented yet")
	case "postgres":
		// 这里需要添加postgres驱动，暂时只支持sqlite
		return nil, fmt.Errorf("postgres driver not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	// 配置GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 测试连接
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Successfully connected to %s database", cfg.Driver)
	return db, nil
}
