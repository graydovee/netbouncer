package service

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/graydovee/netbouncer/pkg/history"
)

type PortHistoryParams struct {
	Start, End, Bucket int64
	IP, Proto          string
	Port               int
}
type cachedQuery struct {
	ready   chan struct{}
	result  history.Result
	err     error
	expires time.Time
}
type queryCache struct {
	mu      sync.Mutex
	entries map[string]*cachedQuery
}

func (s *NetService) SetHistory(h *history.Store) { s.history = h }
func (s *NetService) HistoryStatus(ctx context.Context) (history.Status, error) {
	if s.history == nil {
		return history.Status{}, Invalidf("历史采样未启用")
	}
	return s.history.Status(ctx)
}
func (s *NetService) QueryHistory(ctx context.Context, q history.Query) (history.Result, error) {
	if s.history == nil {
		return history.Result{Items: []history.Point{}, Meta: history.Meta{Gaps: []history.Gap{{Start: q.Start, End: q.End, Reason: "history_disabled"}}}}, nil
	}
	if q.Start < 0 || q.End < 0 || q.Start > 0 && q.End > 0 && q.Start >= q.End {
		return history.Result{}, Invalidf("无效时间范围")
	}
	if q.IP != "" {
		if addr, err := netip.ParseAddr(q.IP); err != nil {
			return history.Result{}, Invalidf("无效 IP")
		} else {
			q.IP = addr.Unmap().String()
		}
	}
	if q.Proto != "" && q.Proto != "tcp" && q.Proto != "udp" && q.Proto != "icmp" && q.Proto != "other" {
		return history.Result{}, Invalidf("无效协议")
	}
	// Normalize moving 'now' requests to a cache epoch. Responses expose actual freshness/range.
	now := time.Now().Unix() / 30 * 30
	if q.End <= 0 || q.End > now {
		q.End = now
	}
	if q.Start <= 0 {
		q.Start = q.End - 86400
	}
	q.Start = q.Start / 30 * 30
	if q.Start >= q.End {
		return history.Result{}, Invalidf("无效时间范围")
	}
	key := fmt.Sprintf("%+v", q)
	cache := &s.historyCache
	cache.mu.Lock()
	if cache.entries == nil {
		cache.entries = map[string]*cachedQuery{}
	}
	if entry := cache.entries[key]; entry != nil && time.Now().Before(entry.expires) {
		cache.mu.Unlock()
		select {
		case <-ctx.Done():
			return history.Result{}, ctx.Err()
		case <-entry.ready:
			return entry.result, entry.err
		}
	}
	for k, v := range cache.entries {
		if time.Now().After(v.expires) {
			delete(cache.entries, k)
		}
	}
	if len(cache.entries) >= 128 {
		cache.mu.Unlock()
		return history.Result{}, Invalidf("历史查询繁忙，请稍后重试")
	}
	entry := &cachedQuery{ready: make(chan struct{}), expires: time.Now().Add(30 * time.Second)}
	cache.entries[key] = entry
	cache.mu.Unlock()
	// The owner request owns the SQL context; waiting requests can independently cancel.
	result, err := s.history.Query(ctx, q)
	cache.mu.Lock()
	entry.result = result
	entry.err = err
	if err != nil {
		delete(cache.entries, key)
	}
	close(entry.ready)
	cache.mu.Unlock()
	return result, err
}
