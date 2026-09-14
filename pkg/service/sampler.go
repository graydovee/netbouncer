package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/graydovee/netbouncer/pkg/store"
)

// Sampler 流量历史采样器：定时把各远程IP在采样间隔内的流量增量写入存储。
// 同时写 IP 维度（traffic_samples）与 IP×协议×端口维度（traffic_port_samples），
// 存增量而非累计值，查询端可直接分桶聚合，进程重启/条目淘汰也不会产生错误数据。
type Sampler struct {
	monitor  Monitor
	samples  *store.TrafficSampleStore
	ports    *store.TrafficPortSampleStore
	interval time.Duration

	mu   sync.Mutex
	last map[string]*ipSnapshot
}

type lastCounters struct {
	bytesIn    uint64
	bytesOut   uint64
	packetsIn  uint64
	packetsOut uint64
}

// ipSnapshot 单个 IP 的上次采样快照（总量 + 协议/端口维度）
type ipSnapshot struct {
	totals lastCounters
	ports  map[string]map[uint16]lastCounters
}

func NewSampler(monitor Monitor, samples *store.TrafficSampleStore, ports *store.TrafficPortSampleStore, interval time.Duration) *Sampler {
	return &Sampler{
		monitor:  monitor,
		samples:  samples,
		ports:    ports,
		interval: interval,
		last:     make(map[string]*ipSnapshot),
	}
}

// Start 启动采样循环，随 ctx 取消而退出（过期清理由 Rollup 服务负责）
func (s *Sampler) Start(ctx context.Context) {
	if s == nil || s.interval <= 0 {
		return
	}

	go s.sampleLoop(ctx)
	slog.Info("流量历史采样已启动", "interval", s.interval.String())
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

// sampleOnce 采样一次：与上次快照做差值计算区间增量并批量落库。
// 快照整体重建——从当前统计中消失的 IP（或端口条目）自然被丢弃，重新出现的条目首个区间记 0，
// 避免"上次采样的未知时长流量"被错误归入单个区间。
func (s *Sampler) sampleOnce() error {
	now := time.Now()
	ts := now.Unix() - now.Unix()%int64(s.interval.Seconds())

	stats := s.monitor.GetStats()

	next := make(map[string]*ipSnapshot, len(stats))
	samples := make([]store.TrafficSample, 0, len(stats))
	portSamples := make([]store.TrafficPortSample, 0, len(stats)*2)

	for ip, st := range stats {
		current := &ipSnapshot{
			totals: lastCounters{
				bytesIn:    st.BytesRecv,
				bytesOut:   st.BytesSent,
				packetsIn:  st.PacketsRecv,
				packetsOut: st.PacketsSent,
			},
			ports: make(map[string]map[uint16]lastCounters, len(st.Ports)),
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
		prev := s.last[ip]
		s.mu.Unlock()

		if prev == nil {
			// 首次见到该 IP：基线未知，本区间记 0
			sample.BytesIn = 0
			sample.BytesOut = 0
			sample.PacketsIn = 0
			sample.PacketsOut = 0
		} else {
			// 计数器被重置（IP 条目超时删除后重建）时，把重建以来的流量全记入本区间
			sample.BytesIn = deltaSub(st.BytesRecv, prev.totals.bytesIn)
			sample.BytesOut = deltaSub(st.BytesSent, prev.totals.bytesOut)
			sample.PacketsIn = deltaSub(st.PacketsRecv, prev.totals.packetsIn)
			sample.PacketsOut = deltaSub(st.PacketsSent, prev.totals.packetsOut)
		}

		// 协议/端口维度差值
		for proto, ports := range st.Ports {
			prevProto := map[uint16]lastCounters(nil)
			if prev != nil {
				prevProto = prev.ports[proto]
			}
			curProto := make(map[uint16]lastCounters, len(ports))
			for port, p := range ports {
				cur := lastCounters{
					bytesIn:    p.BytesRecv,
					bytesOut:   p.BytesSent,
					packetsIn:  p.PacketsRecv,
					packetsOut: p.PacketsSent,
				}
				curProto[port] = cur

				ps := store.TrafficPortSample{
					RemoteIP:  ip,
					Proto:     proto,
					Port:      int(port),
					Ts:        ts,
					BytesIn:   p.BytesRecv,
					BytesOut:  p.BytesSent,
					PacketsIn: p.PacketsRecv,
				}
				ps.PacketsOut = p.PacketsSent

				var prevEntry lastCounters
				if prevProto != nil {
					prevEntry = prevProto[port]
				}
				if prevProto == nil {
					// 该协议维度首次出现，本区间记 0
					ps.BytesIn, ps.BytesOut, ps.PacketsIn, ps.PacketsOut = 0, 0, 0, 0
				} else {
					ps.BytesIn = deltaSub(cur.bytesIn, prevEntry.bytesIn)
					ps.BytesOut = deltaSub(cur.bytesOut, prevEntry.bytesOut)
					ps.PacketsIn = deltaSub(cur.packetsIn, prevEntry.packetsIn)
					ps.PacketsOut = deltaSub(cur.packetsOut, prevEntry.packetsOut)
				}

				portSamples = append(portSamples, ps)
			}
			current.ports[proto] = curProto
		}

		next[ip] = current
		samples = append(samples, sample)
	}

	if err := s.samples.InsertBatch(samples); err != nil {
		return err
	}
	if err := s.ports.InsertBatch(portSamples); err != nil {
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
