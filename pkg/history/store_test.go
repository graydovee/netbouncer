package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(Options{Dir: t.TempDir(), FreeSpace: func(string) (uint64, error) { return 100 << 30, nil }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func write(t *testing.T, s *Store, ts int64, d ...Delta) {
	t.Helper()
	if err := s.Write(context.Background(), Batch{ID: fmt.Sprintf("batch-%d", ts), TS: ts, Deltas: d}); err != nil {
		t.Fatal(err)
	}
}
func query(t *testing.T, s *Store, q Query) Result {
	t.Helper()
	r, err := s.Query(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func sum(points []Point) (uint64, uint64, uint64, uint64) {
	var a, b, c, d uint64
	for _, p := range points {
		a += p.BytesIn
		b += p.BytesOut
		c += p.PacketsIn
		d += p.PacketsOut
	}
	return a, b, c, d
}
func TestConservationAcrossDimensionsRollupAndRestart(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	start := time.Now().Unix()/3600*3600 - 2*3600
	for i := int64(0); i < 60; i++ {
		write(t, s, start+i*60, Delta{IP: "2001:db8::1", Proto: "tcp", Port: 443, BytesIn: 100, BytesOut: 20, PacketsIn: 2, PacketsOut: 1, LastSeen: start + i*60 + 20}, Delta{IP: "1.1.1.1", Proto: "udp", Port: 53, BytesIn: 10, PacketsIn: 1, LastSeen: start + i*60 + 30})
	}
	for _, kind := range []string{"history", "ip_top", "ports", "port_top", "protocols"} {
		r := query(t, s, Query{Start: start, End: start + 3600, Bucket: 60, Port: -1, Kind: kind, Limit: 100})
		a, b, c, d := sum(r.Items)
		if a != 6600 || b != 1200 || c != 180 || d != 60 {
			t.Fatalf("%s sums %d/%d/%d/%d", kind, a, b, c, d)
		}
	}
	if err := s.Maintain(ctx, start+3600); err != nil {
		t.Fatal(err)
	}
	for _, bucket := range []int64{60, 600, 3600} {
		r := query(t, s, Query{Start: start, End: start + 3600, Bucket: bucket, Port: -1, Kind: "history"})
		a, b, c, d := sum(r.Items)
		if a != 6600 || b != 1200 || c != 180 || d != 60 {
			t.Fatalf("bucket %d sums %d/%d/%d/%d", bucket, a, b, c, d)
		}
		if bucket == 60 && r.Meta.Bucket != 60 {
			t.Fatalf("fine coverage ignored: %+v", r.Meta)
		}
		if len(r.Meta.Gaps) != 0 {
			t.Fatalf("unexpected gaps %+v", r.Meta.Gaps)
		}
	}
	// Repeated maintenance after a restart must not double metrics or erase an archived bucket.
	dir := s.opt.Dir
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(Options{Dir: dir, FreeSpace: s.opt.FreeSpace})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if err = restarted.Maintain(ctx, start+3600); err != nil {
		t.Fatal(err)
	}
	r := query(t, restarted, Query{Start: start, End: start + 3600, Bucket: 3600, IP: "2001:db8::1", Port: -1})
	if a, _, _, _ := sum(r.Items); a != 6000 {
		t.Fatalf("restarted total %d", a)
	}
}
func TestBatchRetryIsAtomicAndIdempotent(t *testing.T) {
	s := testStore(t)
	ts := time.Now().Unix()/60*60 - 60
	b := Batch{ID: "retry", TS: ts, Deltas: []Delta{{IP: "1.1.1.1", Proto: "tcp", Port: 80, BytesIn: 7, PacketsIn: 1, LastSeen: ts + 1}}}
	db, done, err := s.acquire(context.Background(), nameFor(Minute, ts), true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TRIGGER fail BEFORE INSERT ON metrics WHEN NEW.dim=5 BEGIN SELECT RAISE(ABORT,'injected failure');END;`)
	if err != nil {
		t.Fatal(err)
	}
	done()
	if err = s.Write(context.Background(), b); err == nil {
		t.Fatal("expected transaction failure")
	}
	db, done, err = s.acquire(context.Background(), nameFor(Minute, ts), true)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM batches").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("batch ledger committed without metrics")
	}
	if _, err = db.Exec("DROP TRIGGER fail"); err != nil {
		t.Fatal(err)
	}
	done()
	if err = s.Write(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if err = s.Write(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	r := query(t, s, Query{Start: ts, End: ts + 60, Port: -1})
	if a, _, _, _ := sum(r.Items); a != 7 {
		t.Fatalf("retry total %d", a)
	}
}
func TestGlobalTopDoesNotTruncateSegments(t *testing.T) {
	s := testStore(t)
	ts := time.Now().Unix()/3600*3600 - 3600
	write(t, s, ts-60, Delta{IP: "A", Proto: "tcp", Port: 80, BytesIn: 100}, Delta{IP: "C", Proto: "tcp", Port: 80, BytesIn: 90})
	write(t, s, ts, Delta{IP: "B", Proto: "tcp", Port: 80, BytesIn: 100}, Delta{IP: "C", Proto: "tcp", Port: 80, BytesIn: 90})
	r := query(t, s, Query{Start: ts - 60, End: ts + 60, Kind: "ip_top", Port: -1, Limit: 1})
	if len(r.Items) != 1 || r.Items[0].IP != "C" || r.Items[0].BytesSum != 180 {
		t.Fatalf("global top %+v", r.Items)
	}
}
func TestMissingCoverageIsNotZeroOrRawFallback(t *testing.T) {
	s := testStore(t)
	ts := time.Now().Unix()/600*600 - 600
	write(t, s, ts, Delta{IP: "1.1.1.1", Proto: "tcp", Port: 443, BytesIn: 5})
	if err := s.Maintain(context.Background(), ts+600); err != nil {
		t.Fatal(err)
	}
	r := query(t, s, Query{Start: ts, End: ts + 600, Bucket: 600, Port: -1})
	if a, _, _, _ := sum(r.Items); a != 5 {
		t.Fatal(a)
	}
	if len(r.Meta.Gaps) == 0 {
		t.Fatal("missing minutes must be visible")
	}
}
func TestCapacityEvictionAndDiskFullPause(t *testing.T) {
	s := testStore(t)
	old := time.Now().Unix()/3600*3600 - 24*3600
	write(t, s, old, Delta{IP: "1.1.1.1", Proto: "tcp", Port: 80, BytesIn: 1})
	s.opt.BudgetBytes = s.fileBytes(nameFor(Minute, old)) + 256*1024 - 1
	now := time.Now().Unix()/60*60 - 60
	write(t, s, now)
	if _, err := os.Stat(filepath.Join(s.opt.Dir, nameFor(Minute, old))); !os.IsNotExist(err) {
		t.Fatalf("old shard was not physically removed: %v", err)
	}
	s.opt.FreeSpace = func(string) (uint64, error) { return 0, nil }
	err := s.Write(context.Background(), Batch{ID: "full", TS: now + 60})
	if !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity error %v", err)
	}
	st, err := s.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Paused || st.WriteFailures == 0 {
		t.Fatalf("missing pause status %+v", st)
	}
	s.opt.FreeSpace = func(string) (uint64, error) { return 100 << 30, nil }
	s.opt.BudgetBytes = 2 << 30
	if err = s.Write(context.Background(), Batch{ID: "full", TS: now + 60}); err != nil {
		t.Fatal(err)
	}
	st, _ = s.Status(context.Background())
	if st.Paused {
		t.Fatal("did not resume")
	}
}
func TestCancelledQueryAndIndexPlans(t *testing.T) {
	s := testStore(t)
	ts := time.Now().Unix()/60*60 - 60
	write(t, s, ts, Delta{IP: "1.1.1.1", Proto: "tcp", Port: 443, BytesIn: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Query(ctx, Query{Start: ts, End: ts + 60, Port: -1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
	db, done, err := s.acquire(context.Background(), nameFor(Minute, ts), false)
	if err != nil {
		t.Fatal(err)
	}
	defer done()
	for _, q := range []string{"SELECT * FROM metrics WHERE dim=0 AND ts>=0 AND ts<9999999999", "SELECT * FROM metrics WHERE dim=1 AND ip='1.1.1.1' AND ts>0", "SELECT * FROM metrics WHERE dim=3 AND proto='tcp' AND port=443 AND ts>0"} {
		rows, err := db.Query("EXPLAIN QUERY PLAN " + q)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			if len(detail) < 6 || detail[:6] != "SEARCH" {
				t.Fatal(detail)
			}
		}
		rows.Close()
	}
}
func TestZeroTrafficOnlyWritesCoverageAndSchemaRejectsUnknown(t *testing.T) {
	s := testStore(t)
	ts := time.Now().Unix()/60*60 - 60
	write(t, s, ts, Delta{IP: "idle", Proto: "tcp", Port: 80})
	db, done, err := s.acquire(context.Background(), nameFor(Minute, ts), false)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err = db.QueryRow("SELECT count(*) FROM metrics").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal(n)
	}
	done()
	s.Close()
	path := filepath.Join(s.opt.Dir, nameFor(Minute, ts))
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	reopened, err := Open(Options{Dir: s.opt.Dir, FreeSpace: s.opt.FreeSpace})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err = reopened.Query(context.Background(), Query{Start: ts, End: ts + 60, Port: -1}); err == nil {
		t.Fatal("unknown schema accepted")
	}
}
