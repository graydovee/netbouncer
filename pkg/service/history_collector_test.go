package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/history"
)

type fakeDeltaMonitor struct {
	deltas []core.HistoryDelta
	lost   []int64
}

func (m *fakeDeltaMonitor) DrainHistory(int64) ([]core.HistoryDelta, []int64) {
	d, l := m.deltas, m.lost
	m.deltas = nil
	m.lost = nil
	return d, l
}
func TestCollectorKeepsRetryBatchAndDoesNotRecount(t *testing.T) {
	free := uint64(0)
	h, err := history.Open(history.Options{Dir: t.TempDir(), FreeSpace: func(string) (uint64, error) { return free, nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ts := time.Now().Unix()/60*60 - 60
	mon := &fakeDeltaMonitor{deltas: []core.HistoryDelta{{TS: ts, IP: "1.1.1.1", Proto: "tcp", Port: 80, BytesIn: 100, PacketsIn: 1, LastSeen: ts + 1}}}
	collector := NewHistoryCollector(mon, h)
	collector.next = ts
	if err = collector.collect(context.Background(), ts+60); !errors.Is(err, history.ErrCapacity) {
		t.Fatalf("disk full %v", err)
	}
	if len(collector.pending) != 1 {
		t.Fatal("lost retry batch")
	}
	id := collector.pending[0].ID
	free = 100 << 30
	if err = collector.collect(context.Background(), ts+60); err != nil {
		t.Fatal(err)
	}
	if len(collector.pending) != 0 {
		t.Fatal("retry not consumed")
	}
	if err = h.Write(context.Background(), history.Batch{ID: id, TS: ts, Deltas: []history.Delta{{IP: "1.1.1.1", Proto: "tcp", Port: 80, BytesIn: 100}}}); err != nil {
		t.Fatal(err)
	}
	result, err := h.Query(context.Background(), history.Query{Start: ts, End: ts + 60, Port: -1})
	if err != nil {
		t.Fatal(err)
	}
	var total uint64
	for _, p := range result.Items {
		total += p.BytesIn
	}
	if total != 100 {
		t.Fatal(total)
	}
}
func TestCollectorEmptyMinutesAreCoverageAndCaptureLossVisible(t *testing.T) {
	h, err := history.Open(history.Options{Dir: t.TempDir(), FreeSpace: func(string) (uint64, error) { return 100 << 30, nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ts := time.Now().Unix()/600*600 - 600
	mon := &fakeDeltaMonitor{lost: []int64{ts}}
	collector := NewHistoryCollector(mon, h)
	collector.next = ts
	if err = collector.collect(context.Background(), ts+600); err != nil {
		t.Fatal(err)
	}
	result, err := h.Query(context.Background(), history.Query{Start: ts, End: ts + 600, Bucket: 60, Port: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 0 {
		t.Fatal("zero metrics were written")
	}
	if len(result.Meta.Gaps) == 0 {
		t.Fatal("capture loss hidden")
	}
}
