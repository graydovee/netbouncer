package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

// 降采样阶梯的固定常量：raw 层短保留，聚合层长保留
const (
	rawIPRetention     = 24 * time.Hour     // IP 维度 raw 保留 24h
	rawPortRetention   = 6 * time.Hour      // 端口维度 raw 保留 6h
	rollup10mRetention = 7 * 24 * time.Hour // 10 分钟聚合层保留 7 天
)

// Rollup 降采样滚动任务：
// - 每 10 分钟：raw 层已完结的 10 分钟桶重算进 10 分钟聚合层（先删后插，幂等），清理过期的 raw 行
// - 每小时：10 分钟层最近完结的小时重算进 1 小时层，清理过期聚合行与风险事件，执行 WAL checkpoint
type Rollup struct {
	store         *store.Store
	retention     time.Duration // 1 小时层保留时长
	riskRetention time.Duration // 风险事件保留时长

	mu                sync.Mutex
	ipWatermark       int64 // 已重算到的 10 分钟桶起点（秒）
	rollup1hWatermark int64 // 已重算到的 1 小时桶起点（秒）
}

func NewRollup(st *store.Store, retention time.Duration, riskRetention time.Duration) *Rollup {
	// retention/riskRetention 为 0 表示永久保留（不做清理）
	return &Rollup{
		store:         st,
		retention:     retention,
		riskRetention: riskRetention,
	}
}

// Start 启动滚动任务，随 ctx 取消而退出
func (r *Rollup) Start(ctx context.Context) {
	if r == nil {
		return
	}

	go r.loop(ctx, 10*time.Minute, r.run10m)
	go r.loop(ctx, time.Hour, r.runHourly)
	slog.Info("流量降采样任务已启动",
		"raw_ip_retention", rawIPRetention.String(),
		"raw_port_retention", rawPortRetention.String(),
		"rollup10m_retention", rollup10mRetention.String(),
		"rollup1h_retention", r.retention.String())
}

func (r *Rollup) loop(ctx context.Context, every time.Duration, job func()) {
	// 立即执行一次，保证重启后尽快补齐缺口
	job()
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job()
		}
	}
}

// run10m 把 raw 层自上次水位以来的完整 10 分钟桶重算进聚合层，并清理过期 raw 行
func (r *Rollup) run10m() {
	now := time.Now().Unix()
	b := now - now%store.RollupBucket10m // 当前未完结桶的起点

	r.mu.Lock()
	from := r.ipWatermark
	r.mu.Unlock()

	// 首次运行回溯整个 raw 保留窗口（升级/重启后补齐缺口）；
	// 此后只处理新完结的桶（多回看一个桶防时钟偏差）
	minFrom := b - 2*store.RollupBucket10m
	if from == 0 {
		from = b - int64(rawIPRetention.Seconds())
		if from < 0 {
			from = 0
		}
	} else if from > minFrom {
		from = minFrom
	}
	if from >= b {
		return
	}

	if err := r.store.TrafficRollupStore.RollupIpFromRaw(store.RollupBucket10m, from, b); err != nil {
		slog.Error("IP维度10分钟聚合失败", "error", err)
		return
	}
	if err := r.store.TrafficRollupStore.RollupPortFromRaw(store.RollupBucket10m, from, b); err != nil {
		slog.Error("端口维度10分钟聚合失败", "error", err)
		return
	}

	r.mu.Lock()
	r.ipWatermark = b
	r.mu.Unlock()

	// 清理过期的 raw 行
	if n, err := r.store.TrafficSampleStore.Cleanup(now - int64(rawIPRetention.Seconds())); err != nil {
		slog.Error("清理过期IP采样失败", "error", err)
	} else if n > 0 {
		slog.Info("已清理过期IP采样", "deleted", n)
	}
	if n, err := r.store.TrafficPortStore.Cleanup(now - int64(rawPortRetention.Seconds())); err != nil {
		slog.Error("清理过期端口采样失败", "error", err)
	} else if n > 0 {
		slog.Info("已清理过期端口采样", "deleted", n)
	}
}

// runHourly 把 10 分钟层自上次水位以来的完整小时重算进 1 小时层，并做保留期清理
func (r *Rollup) runHourly() {
	now := time.Now().Unix()
	h := now - now%store.RollupBucket1h // 当前未完结小时的起点

	r.mu.Lock()
	from := r.rollup1hWatermark
	r.mu.Unlock()

	// 首次运行回溯整个 10 分钟层保留窗口，保证 1 小时层无缺口
	minFrom := h - 2*store.RollupBucket1h
	if from == 0 {
		from = h - int64(rollup10mRetention.Seconds())
		if from < 0 {
			from = 0
		}
	} else if from > minFrom {
		from = minFrom
	}
	if from >= h {
		// 无新桶也要做保留期清理
		r.cleanupHourly(now)
		return
	}

	if err := r.store.TrafficRollupStore.RollupIpFrom10m(from, h); err != nil {
		slog.Error("IP维度1小时聚合失败", "error", err)
		return
	}
	if err := r.store.TrafficRollupStore.RollupPortFrom10m(from, h); err != nil {
		slog.Error("端口维度1小时聚合失败", "error", err)
		return
	}

	r.mu.Lock()
	r.rollup1hWatermark = h
	r.mu.Unlock()

	r.cleanupHourly(now)
}

func (r *Rollup) cleanupHourly(now int64) {
	if n, err := r.store.TrafficRollupStore.CleanupIp(store.RollupBucket10m, now-int64(rollup10mRetention.Seconds())); err != nil {
		slog.Error("清理10分钟聚合层失败", "error", err)
	} else if n > 0 {
		slog.Info("已清理10分钟聚合层", "deleted", n)
	}
	if r.retention > 0 {
		if n, err := r.store.TrafficRollupStore.CleanupIp(store.RollupBucket1h, now-int64(r.retention.Seconds())); err != nil {
			slog.Error("清理1小时聚合层失败", "error", err)
		} else if n > 0 {
			slog.Info("已清理1小时聚合层", "deleted", n)
		}
		if n, err := r.store.TrafficRollupStore.CleanupPort(store.RollupBucket1h, now-int64(r.retention.Seconds())); err != nil {
			slog.Error("清理端口1小时聚合层失败", "error", err)
		} else if n > 0 {
			slog.Info("已清理端口1小时聚合层", "deleted", n)
		}
	}
	if n, err := r.store.TrafficRollupStore.CleanupPort(store.RollupBucket10m, now-int64(rollup10mRetention.Seconds())); err != nil {
		slog.Error("清理端口10分钟聚合层失败", "error", err)
	} else if n > 0 {
		slog.Info("已清理端口10分钟聚合层", "deleted", n)
	}
	if n, err := r.store.RiskEventStore.Cleanup(now - int64(r.riskRetention.Seconds())); err != nil {
		slog.Error("清理风险事件失败", "error", err)
	} else if n > 0 {
		slog.Info("已清理过期风险事件", "deleted", n)
	}
	if err := r.store.TrafficRollupStore.Checkpoint(); err != nil {
		slog.Warn("WAL checkpoint 失败", "error", err)
	}
}
