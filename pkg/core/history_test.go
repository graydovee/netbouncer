package core

import (
	"testing"
	"time"
)

func TestHistoryCountsFirstPacketAndSurvivesEviction(t *testing.T) {
	m := newTestMonitor()
	m.historyEnabled = true
	m.updateStats("8.8.8.8", "10.0.0.1", "tcp", 1234, 443, 100, false, true, false)
	m.mutex.Lock()
	delete(m.stats, "8.8.8.8")
	m.mutex.Unlock()
	m.updateStats("8.8.8.8", "10.0.0.1", "tcp", 1234, 443, 200, true, false, false)
	delta, lost := m.DrainHistory(time.Now().Unix()/60*60 + 60)
	if len(lost) != 0 || len(delta) != 1 || delta[0].BytesIn != 100 || delta[0].BytesOut != 200 || delta[0].PacketsIn != 1 || delta[0].PacketsOut != 1 {
		t.Fatalf("increments %+v lost %+v", delta, lost)
	}
	second, _ := m.DrainHistory(time.Now().Unix()/60*60 + 60)
	if len(second) != 0 {
		t.Fatal("drain repeated increments")
	}
}
func TestHistoryClosedMinuteAndExcludedIP(t *testing.T) {
	m := newTestMonitor()
	m.historyEnabled = true
	m.updateStats("8.8.8.8", "10.0.0.1", "udp", 1234, 53, 10, false, false, false)
	before := time.Now().Unix() / 60 * 60
	delta, _ := m.DrainHistory(before)
	if len(delta) != 0 {
		t.Fatal("drained open minute")
	}
	delta, _ = m.DrainHistory(before + 60)
	if len(delta) != 1 {
		t.Fatal(delta)
	}
}
func TestRateWindowMemoryBoundedBySeconds(t *testing.T) {
	w := newTrafficWindow(time.Minute)
	for i := 0; i < 10000; i++ {
		w.addPoint(100)
	}
	if len(w.points) > 2 {
		t.Fatalf("packet rate expanded window to %d", len(w.points))
	}
	var total uint64
	for _, p := range w.points {
		total += p.increment
	}
	if total != 1000000 {
		t.Fatal(total)
	}
}
