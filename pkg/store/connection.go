package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/graydovee/netbouncer/pkg/config"
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

// appendSQLitePragmas 为 DSN 追加 WAL/busy_timeout/synchronous 参数（幂等）
func appendSQLitePragmas(dsn string) string {
	pragmas := "_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"
	if strings.Contains(dsn, "?") {
		return dsn + "&" + pragmas
	}
	return dsn + "?" + pragmas
}

// NewDatabase 根据配置创建数据库连接（当前仅支持 sqlite）
func NewDatabase(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Driver != "sqlite" {
		return nil, fmt.Errorf("数据库驱动 %q 暂未实现，当前仅支持 sqlite", cfg.Driver)
	}

	dsn := cfg.DSN
	if dsn == "" {
		if cfg.Database == "" {
			cfg.Database = "netbouncer.db"
		}
		dsn = cfg.Database
	}

	// 历史采样为高频小事务写入，启用 WAL + NORMAL 同步提升吞吐，
	// 并设置 busy_timeout 避免读写并发时立即报 SQLITE_BUSY
	dsn = appendSQLitePragmas(dsn)

	slog.Info("使用数据库", "driver", cfg.Driver, "file", dsn)

	// 根据配置设置SQL日志级别
	var logLevel slog.Level
	switch cfg.LogLevel {
	case "silent", "error":
		logLevel = slog.LevelError
	case "warn":
		logLevel = slog.LevelWarn
	default:
		logLevel = slog.LevelInfo
	}

	// 配置GORM，使用自定义的slog logger
	gormConfig := &gorm.Config{
		Logger: NewSlogLogger(logLevel),
	}

	db, err := gorm.Open(sqlite.Open(dsn), gormConfig)
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
