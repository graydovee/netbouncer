package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/graydovee/netbouncer/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SlogLogger 实现GORM的logger接口，使用Go的slog
type SlogLogger struct {
	level slog.Level
}

var _ logger.Interface = (*SlogLogger)(nil)

// NewSlogLogger 创建新的slog logger
func NewSlogLogger(level slog.Level) *SlogLogger {
	return &SlogLogger{level: level}
}

// LogMode 设置日志级别
func (l *SlogLogger) LogMode(level logger.LogLevel) logger.Interface {
	switch level {
	case logger.Silent:
		l.level = slog.LevelError
	case logger.Error:
		l.level = slog.LevelError
	case logger.Warn:
		l.level = slog.LevelWarn
	case logger.Info:
		l.level = slog.LevelInfo
	}
	return l
}

// Info 记录信息日志
func (l *SlogLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.level <= slog.LevelInfo {
		slog.Info(msg, data...)
	}
}

// Warn 记录警告日志
func (l *SlogLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.level <= slog.LevelWarn {
		slog.Warn(msg, data...)
	}
}

// Error 记录错误日志
func (l *SlogLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.level <= slog.LevelError {
		slog.Error(msg, data...)
	}
}

// Trace 记录SQL跟踪日志
func (l *SlogLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	attrs := []any{
		"elapsed", elapsed,
		"rows", rows,
	}

	if err != nil {
		if l.level <= slog.LevelError {
			slog.Error("SQL执行错误", append(attrs, "error", err, "sql", sql)...)
		}
	} else {
		if l.level <= slog.LevelInfo {
			slog.Info("SQL执行", append(attrs, "sql", sql)...)
		}
	}
}

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

	// 根据配置设置SQL日志级别
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "silent":
		logLevel = slog.LevelError
	case "error":
		logLevel = slog.LevelError
	case "warn":
		logLevel = slog.LevelWarn
	case "info":
		logLevel = slog.LevelInfo
	default:
		logLevel = slog.LevelInfo // 默认级别
	}

	// 配置GORM，使用自定义的slog logger
	gormConfig := &gorm.Config{
		Logger: NewSlogLogger(logLevel),
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

	slog.Info("数据库连接成功", "driver", cfg.Driver, "log_level", cfg.LogLevel)
	return db, nil
}
