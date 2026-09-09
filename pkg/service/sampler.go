package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

// Sampler 流量历史采样器：定时把各远程IP在采样间隔内的流量增量写入存储。
// 存增量而非累计值，查询端可直接分桶聚合，进程重启/条目淘汰也不会产生错误数据。
type Sampler struct {
	monitor   Monitor
	samples   *store.TrafficSampleStore
	interval  time.Duration
	retention time.Duration

	mu   sync.Mutex
	last map[string]lastCounters
}

type lastCounters struct {
	bytesIn    uint64
	bytesOut   uint64
	packetsIn  uint64
	packetsOut uint64
}

func NewSampler(monitor Monitor, samples *store.TrafficSampleStore, interval time.Duration, retention time.Duration) *Sampler {
	return &Sampler{
		monitor:   monitor,
		samples:   samples,
		interval:  interval,
		retention: retention,
		last:      make(map[string]lastCounters),
	}
}

// Start 启动采样循环与过期清理循环，随 ctx 取消而退出
func (s *Sampler) Start(ctx context.Context) {
	if s == nil || s.interval <= 0 {
		return
	}

	go s.sampleLoop(ctx)
	go s.cleanupLoop(ctx)
	slog.Info("流量历史采样已启动", "interval", s.interval.String(), "retention", s.retention.String())
}

func (s *Sampler) sampleLoop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.sampleOnce(); err != nil {
				slog.Error("流量采样失败", "error", err)
			}
		}
	}
}

func (s *Sampler) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			before := time.Now().Add(-s.retention).Unix()
			deleted, err := s.samples.Cleanup(before)
			if err != nil {
				slog.Error("清理过期流量历史失败", "error", err)
			} else if deleted > 0 {
				slog.Info("已清理过期流量历史", "deleted", deleted)
			}
		}
	}
}

// sampleOnce 采样一次：与上次快照做差值计算区间增量并批量落库。
// 快照整体重建——从当前统计中消失的 IP 自然被丢弃，重新出现的 IP 首个区间记 0，
// 避免"上次采样的未知时长流量"被错误归入单个区间。
func (s *Sampler) sampleOnce() error {
	now := time.Now()
	ts := now.Unix() - now.Unix()%int64(s.interval.Seconds())

	stats := s.monitor.GetStats()

	next := make(map[string]lastCounters, len(stats))
	samples := make([]store.TrafficSample, 0, len(stats))

	for ip, st := range stats {
		current := lastCounters{
			bytesIn:    st.BytesRecv,
			bytesOut:   st.BytesSent,
			packetsIn:  st.PacketsRecv,
			packetsOut: st.PacketsSent,
		}

		sample := store.TrafficSample{
			RemoteIP:    ip,
			Ts:          ts,
			BytesIn:     st.BytesRecv,
			BytesOut:    st.BytesSent,
			PacketsIn:   st.PacketsRecv,
			PacketsOut:  st.PacketsSent,
			Connections: st.Connections,
		}

		s.mu.Lock()
		prev, ok := s.last[ip]
		s.mu.Unlock()
		if ok {
			// 计数器被重置（IP 条目超时删除后重建）时，把重建以来的流量全记入本区间
			sample.BytesIn = deltaSub(st.BytesRecv, prev.bytesIn)
			sample.BytesOut = deltaSub(st.BytesSent, prev.bytesOut)
			sample.PacketsIn = deltaSub(st.PacketsRecv, prev.packetsIn)
			sample.PacketsOut = deltaSub(st.PacketsSent, prev.packetsOut)
		} else {
			// 首次见到该 IP：基线未知，本区间记 0
			sample.BytesIn = 0
			sample.BytesOut = 0
			sample.PacketsIn = 0
			sample.PacketsOut = 0
		}

		next[ip] = current
		samples = append(samples, sample)
	}

	if err := s.samples.InsertBatch(samples); err != nil {
		return err
	}

	s.mu.Lock()
	s.last = next
	s.mu.Unlock()
	return nil
}

// deltaSub 计算计数器增量；当前值小于上次值说明计数器被重置，返回当前值
func deltaSub(current, prev uint64) uint64 {
	if current < prev {
		return current
	}
	return current - prev
}
