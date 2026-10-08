package service

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/history"
)

type DeltaMonitor interface {
	DrainHistory(int64) ([]core.HistoryDelta, []int64)
}
type HistoryCollector struct {
	monitor        DeltaMonitor
	history        *history.Store
	pending        []history.Batch
	next           int64
	lastMaintained int64
	mu             sync.Mutex
	done           chan struct{}
}

func NewHistoryCollector(mon DeltaMonitor, h *history.Store) *HistoryCollector {
	return &HistoryCollector{monitor: mon, history: h, next: time.Now().Unix() / 60 * 60, done: make(chan struct{})}
}
func (c *HistoryCollector) Start(ctx context.Context) {
	go func() {
		defer close(c.done)
		lastWarning := time.Time{}
		timer := time.NewTicker(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = c.collect(flushCtx, time.Now().Unix()/60*60)
				cancel()
				return
			case <-timer.C:
				runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				if err := c.collect(runCtx, time.Now().Unix()/60*60); err != nil && time.Since(lastWarning) >= 30*time.Second {
					lastWarning = time.Now()
					slog.Warn("历史采样暂不可用", "error", err)
				}
				cancel()
			}
		}
	}()
}
func (c *HistoryCollector) Wait(ctx context.Context) error {
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (c *HistoryCollector) collect(ctx context.Context, before int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	deltas, lost := c.monitor.DrainHistory(before)
	byTS := map[int64][]history.Delta{}
	for _, d := range deltas {
		byTS[d.TS] = append(byTS[d.TS], history.Delta{IP: d.IP, Proto: d.Proto, Port: d.Port, BytesIn: d.BytesIn, BytesOut: d.BytesOut, PacketsIn: d.PacketsIn, PacketsOut: d.PacketsOut, LastSeen: d.LastSeen})
	}

	for ts := c.next; ts < before; ts += 60 {
		c.pending = append(c.pending, history.Batch{ID: uuid.NewString(), TS: ts, Deltas: byTS[ts]})
		delete(byTS, ts)
	}
	// Handles a backwards wall clock without silently throwing away transferred increments.
	for ts, d := range byTS {
		c.pending = append(c.pending, history.Batch{ID: uuid.NewString(), TS: ts, Deltas: d})
	}
	if before > c.next {
		c.next = before
	}
	sort.SliceStable(c.pending, func(i, j int) bool { return c.pending[i].TS < c.pending[j].TS })
	size := 0
	for _, b := range c.pending {
		size += len(b.Deltas)*256 + 128
	}
	for size > 32<<20 && len(c.pending) > 0 {
		b := c.pending[0]
		_ = c.history.RecordGap(b.TS, b.TS+60, "retry_buffer_overflow")
		size -= len(b.Deltas)*256 + 128
		c.pending = c.pending[1:]
	}
	for _, ts := range lost {
		_ = c.history.RecordGap(ts, ts+60, "capture_buffer_overflow")
	}

	for len(c.pending) > 0 {
		b := c.pending[0]
		if err := c.history.Write(ctx, b); err != nil {
			return err
		}
		c.pending[0] = history.Batch{}
		c.pending = c.pending[1:]
	}
	if c.lastMaintained == before {
		return nil
	}
	if err := c.history.Maintain(ctx, before); err != nil {
		return err
	}
	c.lastMaintained = before
	return nil
}
