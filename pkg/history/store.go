package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schemaVersion = 1
const maxConnections = 8

var ErrCapacity = errors.New("history storage capacity exhausted")

type shard struct {
	name                   string
	resolution, start, end int64
}
type connection struct {
	db    *sql.DB
	read  *sql.DB
	users int
	used  time.Time
}
type diskState struct {
	Gaps       []Gap  `json:"gaps"`
	LastError  string `json:"last_error"`
	Failures   uint64 `json:"failures"`
	Paused     bool   `json:"paused"`
	LastCommit int64  `json:"last_commit"`
}
type Store struct {
	opt     Options
	writer  sync.Mutex
	catalog sync.RWMutex // held for query lifetime; protects files from eviction
	mu      sync.Mutex
	conns   map[string]*connection
	state   diskState
	slots   chan struct{}
	closed  bool
}

func Open(opt Options) (*Store, error) {
	if opt.Dir == "" {
		return nil, errors.New("history directory required")
	}
	if opt.BudgetBytes == 0 {
		opt.BudgetBytes = 2 << 30
	}
	if opt.ReserveBytes == 0 {
		opt.ReserveBytes = 2 << 30
	}
	if opt.FreeSpace == nil {
		opt.FreeSpace = freeSpace
	}
	if err := os.MkdirAll(opt.Dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{opt: opt, conns: map[string]*connection{}, slots: make(chan struct{}, 4)}
	b, err := os.ReadFile(filepath.Join(opt.Dir, "state.json"))
	if err == nil {
		if err = json.Unmarshal(b, &s.state); err != nil {
			return nil, fmt.Errorf("history state: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	// SQLite coverage is authoritative if persisting process metadata failed after commit.
	all, err := s.shards()
	if err != nil {
		return nil, err
	}
	for i := len(all) - 1; i >= 0; i-- {
		v := all[i]
		if v.resolution != Minute {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		db, release, e := s.acquire(ctx, v.name, false)
		if e != nil {
			cancel()
			_ = s.Close()
			return nil, e
		}
		var latest sql.NullInt64
		e = db.QueryRowContext(ctx, "SELECT MAX(ts) FROM coverage").Scan(&latest)
		release()
		cancel()
		if e != nil {
			_ = s.Close()
			return nil, e
		}
		if latest.Valid {
			s.state.LastCommit = max(s.state.LastCommit, latest.Int64+Minute)
		}
		break
	}
	now := time.Now().Unix()
	if s.state.LastCommit == 0 {
		s.recordGapLocked(now/60*60, now, "capture_started")
	}
	if s.state.LastCommit > 0 && now > s.state.LastCommit {
		s.recordGapLocked(s.state.LastCommit, now, "process_unavailable")
	}
	if err = s.saveState(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error {
	s.writer.Lock()
	defer s.writer.Unlock()
	s.catalog.Lock()
	defer s.catalog.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var errs []error
	for _, c := range s.conns {
		errs = append(errs, closeConnection(c))
	}
	s.conns = map[string]*connection{}
	return errors.Join(errs...)
}
func span(res int64) int64 {
	if res == Minute {
		return Hour
	}
	return 86400
}
func nameFor(res, ts int64) string {
	return fmt.Sprintf("v1-%d-%d.sqlite", res, ts/span(res)*span(res))
}
func parseShard(name string) (shard, bool) {
	var res, start int64
	if _, err := fmt.Sscanf(name, "v1-%d-%d.sqlite", &res, &start); err != nil {
		return shard{}, false
	}
	if (res != Minute && res != TenMinutes && res != Hour) || name != nameFor(res, start) {
		return shard{}, false
	}
	return shard{name, res, start, start + span(res)}, true
}
func (s *Store) shards() ([]shard, error) {
	entries, err := os.ReadDir(s.opt.Dir)
	if err != nil {
		return nil, err
	}
	out := []shard{}
	for _, e := range entries {
		if v, ok := parseShard(e.Name()); ok && !e.IsDir() {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].start != out[j].start {
			return out[i].start < out[j].start
		}
		return out[i].resolution < out[j].resolution
	})
	return out, nil
}

// acquire reserves a bounded connection entry; never closes a file used by readers.
func closeConnection(c *connection) error {
	var errs []error
	if c.db != nil {
		errs = append(errs, c.db.Close())
	}
	if c.read != nil {
		errs = append(errs, c.read.Close())
	}
	return errors.Join(errs...)
}
func (s *Store) openConnection(ctx context.Context, name string, write bool) (*sql.DB, error) {
	path := filepath.Join(s.opt.Dir, name)
	if !write {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?_busy_timeout=1000&_cache_size=-2048&_foreign_keys=on"
	if write {
		dsn += "&_journal_mode=WAL&_synchronous=NORMAL"
	} else {
		dsn += "&mode=ro&_query_only=1"
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = schema(ctx, db, write); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Each cached shard has a dedicated writer and a read-only reader (two connections).
func (s *Store) acquire(ctx context.Context, name string, write bool) (*sql.DB, func(), error) {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, nil, errors.New("history closed")
		}
		c := s.conns[name]
		if c == nil && len(s.conns) >= maxConnections {
			oldest := ""
			var used time.Time
			for n, v := range s.conns {
				if v.users == 0 && (oldest == "" || v.used.Before(used)) {
					oldest = n
					used = v.used
				}
			}
			if oldest == "" {
				s.mu.Unlock()
				select {
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				case <-time.After(5 * time.Millisecond):
					continue
				}
			}
			_ = closeConnection(s.conns[oldest])
			delete(s.conns, oldest)
		}
		if c == nil {
			c = &connection{}
		}
		db := c.read
		if write {
			db = c.db
		}
		if db == nil {
			var err error
			db, err = s.openConnection(ctx, name, write)
			if err != nil {
				s.mu.Unlock()
				return nil, nil, err
			}
			if write {
				c.db = db
			} else {
				c.read = db
			}
		}
		c.users++
		c.used = time.Now()
		s.conns[name] = c
		s.mu.Unlock()
		return db, func() { s.release(name) }, nil
	}
}
func (s *Store) release(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c := s.conns[name]; c != nil {
		c.users--
	}
}
func schema(ctx context.Context, db *sql.DB, create bool) error {
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	if v == schemaVersion {
		return nil
	}
	if v != 0 || !create {
		return fmt.Errorf("unsupported history schema %d", v)
	}
	_, err := db.ExecContext(ctx, `BEGIN;
 CREATE TABLE metrics(dim INTEGER NOT NULL,ts INTEGER NOT NULL,ip TEXT NOT NULL,proto TEXT NOT NULL,port INTEGER NOT NULL,bytes_in INTEGER NOT NULL,bytes_out INTEGER NOT NULL,packets_in INTEGER NOT NULL,packets_out INTEGER NOT NULL,last_seen INTEGER NOT NULL,PRIMARY KEY(dim,ts,ip,proto,port)) WITHOUT ROWID;
 CREATE INDEX metrics_time ON metrics(ts,dim,ip,proto,port);
 CREATE INDEX metrics_ip ON metrics(dim,ip,ts,proto,port);
 CREATE INDEX metrics_port ON metrics(dim,proto,port,ts,ip);
 CREATE TABLE totals(dim INTEGER NOT NULL,ip TEXT NOT NULL,proto TEXT NOT NULL,port INTEGER NOT NULL,bytes_in INTEGER NOT NULL,bytes_out INTEGER NOT NULL,packets_in INTEGER NOT NULL,packets_out INTEGER NOT NULL,last_seen INTEGER NOT NULL,PRIMARY KEY(dim,ip,proto,port)) WITHOUT ROWID;
 CREATE TRIGGER metrics_insert_total AFTER INSERT ON metrics BEGIN
 INSERT INTO totals VALUES(NEW.dim,NEW.ip,NEW.proto,NEW.port,NEW.bytes_in,NEW.bytes_out,NEW.packets_in,NEW.packets_out,NEW.last_seen) ON CONFLICT(dim,ip,proto,port) DO UPDATE SET bytes_in=bytes_in+excluded.bytes_in,bytes_out=bytes_out+excluded.bytes_out,packets_in=packets_in+excluded.packets_in,packets_out=packets_out+excluded.packets_out,last_seen=MAX(last_seen,excluded.last_seen);
 END;
 CREATE TRIGGER metrics_update_total AFTER UPDATE ON metrics BEGIN
 UPDATE totals SET bytes_in=bytes_in+NEW.bytes_in-OLD.bytes_in,bytes_out=bytes_out+NEW.bytes_out-OLD.bytes_out,packets_in=packets_in+NEW.packets_in-OLD.packets_in,packets_out=packets_out+NEW.packets_out-OLD.packets_out,last_seen=MAX(last_seen,NEW.last_seen) WHERE dim=NEW.dim AND ip=NEW.ip AND proto=NEW.proto AND port=NEW.port;
 END;
 CREATE TABLE coverage(ts INTEGER PRIMARY KEY,complete INTEGER NOT NULL,last_seen INTEGER NOT NULL) WITHOUT ROWID;
 CREATE TABLE batches(id TEXT PRIMARY KEY,ts INTEGER NOT NULL) WITHOUT ROWID;
 PRAGMA user_version=1; COMMIT;`)
	return err
}
func (s *Store) saveState() error {
	b, err := json.Marshal(s.state)
	if err != nil {
		return err
	}
	path := filepath.Join(s.opt.Dir, "state.json")
	if err = os.WriteFile(path+".tmp", b, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
func (s *Store) recordGapLocked(start, end int64, reason string) {
	if end <= start {
		return
	}
	n := len(s.state.Gaps)
	if n > 0 {
		last := &s.state.Gaps[n-1]
		if last.Reason == reason && start <= last.End {
			if end > last.End {
				last.End = end
			}
			return
		}
	}
	s.state.Gaps = append(s.state.Gaps, Gap{start, end, reason})
	if len(s.state.Gaps) > 256 {
		s.state.Gaps = s.state.Gaps[len(s.state.Gaps)-256:]
	}
}
func (s *Store) RecordGap(start, end int64, reason string) error {
	s.writer.Lock()
	defer s.writer.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordGapLocked(start, end, reason)
	return s.saveState()
}
func (s *Store) failure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Failures++
	s.state.LastError = err.Error()
	if len(s.state.LastError) > 512 {
		s.state.LastError = s.state.LastError[:512]
	}
	s.state.Paused = errors.Is(err, ErrCapacity)
	_ = s.saveState()
}
func (s *Store) fileBytes(name string) int64 {
	var total int64
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if st, e := os.Stat(filepath.Join(s.opt.Dir, name+suffix)); e == nil {
			total += st.Size()
		}
	}
	return total
}
func (s *Store) remove(v shard, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c := s.conns[v.name]; c != nil {
		if c.users > 0 {
			return errors.New("shard still in use")
		}
		if err := closeConnection(c); err != nil {
			return err
		}
		delete(s.conns, v.name)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(filepath.Join(s.opt.Dir, v.name+suffix)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if reason != "" {
		s.recordGapLocked(v.start, v.end, reason)
	}
	return s.saveState()
}

// enforce removes old shards as physical files, including WAL. reserve includes the next transaction.
func (s *Store) enforce(ctx context.Context, now int64, reserve int64) error {
	s.catalog.Lock()
	defer s.catalog.Unlock()
	all, err := s.shards()
	if err != nil {
		return err
	}
	total := int64(0)
	for _, v := range all {
		total += s.fileBytes(v.name)
	}
	available, err := s.opt.FreeSpace(s.opt.Dir)
	if err != nil {
		return err
	}
	for _, v := range all {
		if total+reserve <= s.opt.BudgetBytes && available >= s.opt.ReserveBytes+uint64(reserve) {
			return nil
		}
		// Never unlink the current minute or an actively used shard. Readers finish before catalog lock.
		if v.end > now {
			continue
		}
		n := s.fileBytes(v.name)
		if err = s.remove(v, "capacity_eviction"); err != nil {
			return err
		}
		total -= n
		available, err = s.opt.FreeSpace(s.opt.Dir)
		if err != nil {
			return err
		}
	}
	if total+reserve > s.opt.BudgetBytes || available < s.opt.ReserveBytes+uint64(reserve) {
		return ErrCapacity
	}
	return nil
}

type metricKey struct {
	dim       int
	ts        int64
	ip, proto string
	port      int
}
type metric struct {
	key                metricKey
	in, out, pin, pout uint64
	seen               int64
}

func aggregate(batch Batch) map[metricKey]*metric {
	out := map[metricKey]*metric{}
	for _, d := range batch.Deltas {
		if d.BytesIn|d.BytesOut|d.PacketsIn|d.PacketsOut == 0 {
			continue
		}
		keys := []metricKey{{DimTotal, batch.TS, "", "", -1}, {DimIP, batch.TS, d.IP, "", -1}, {DimProto, batch.TS, "", d.Proto, -1}, {DimPort, batch.TS, "", d.Proto, d.Port}, {DimIPProto, batch.TS, d.IP, d.Proto, -1}, {DimIPPort, batch.TS, d.IP, d.Proto, d.Port}}
		for _, k := range keys {
			m := out[k]
			if m == nil {
				m = &metric{key: k}
				out[k] = m
			}
			m.in += d.BytesIn
			m.out += d.BytesOut
			m.pin += d.PacketsIn
			m.pout += d.PacketsOut
			if d.LastSeen > m.seen {
				m.seen = d.LastSeen
			}
		}
	}
	return out
}

const insertMetric = `INSERT INTO metrics(dim,ts,ip,proto,port,bytes_in,bytes_out,packets_in,packets_out,last_seen) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(dim,ts,ip,proto,port) DO UPDATE SET bytes_in=bytes_in+excluded.bytes_in,bytes_out=bytes_out+excluded.bytes_out,packets_in=packets_in+excluded.packets_in,packets_out=packets_out+excluded.packets_out,last_seen=MAX(last_seen,excluded.last_seen)`

func (s *Store) Write(ctx context.Context, batch Batch) error {
	if batch.ID == "" || batch.TS < 0 || batch.TS%Minute != 0 {
		return errors.New("invalid history batch")
	}
	s.writer.Lock()
	defer s.writer.Unlock()
	// Conservative room for rows, secondary indexes, and WAL; don't allocate an aggregate before capacity check.
	reserve := int64(len(batch.Deltas))*2048 + 256*1024
	if err := s.enforce(ctx, time.Now().Unix(), reserve); err != nil {
		s.failure(err)
		return err
	}
	s.catalog.RLock()
	defer s.catalog.RUnlock()
	db, release, err := s.acquire(ctx, nameFor(Minute, batch.TS), true)
	if err != nil {
		s.failure(err)
		return err
	}
	defer release()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		s.failure(err)
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO batches(id,ts) VALUES(?,?)", batch.ID, batch.TS)
	if err != nil {
		s.failure(err)
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, insertMetric)
	if err != nil {
		s.failure(err)
		return err
	}
	defer stmt.Close()
	var seen int64
	for _, m := range aggregate(batch) {
		k := m.key
		if _, err = stmt.ExecContext(ctx, k.dim, k.ts, k.ip, k.proto, k.port, m.in, m.out, m.pin, m.pout, m.seen); err != nil {
			s.failure(err)
			return err
		}
		if m.seen > seen {
			seen = m.seen
		}
	}
	complete := 1
	s.mu.Lock()
	for _, g := range s.state.Gaps {
		if g.Start < batch.TS+60 && g.End > batch.TS && g.Reason != "capacity_eviction" {
			complete = 0
			break
		}
	}
	s.mu.Unlock()
	if _, err = tx.ExecContext(ctx, "INSERT INTO coverage(ts,complete,last_seen) VALUES(?,?,?) ON CONFLICT(ts) DO UPDATE SET complete=MIN(complete,excluded.complete),last_seen=MAX(last_seen,excluded.last_seen)", batch.TS, complete, seen); err != nil {
		s.failure(err)
		return err
	}
	if err = tx.Commit(); err != nil {
		s.failure(err)
		return err
	}
	s.mu.Lock()
	s.state.Paused = false
	s.state.LastError = ""
	if batch.TS+Minute > s.state.LastCommit {
		s.state.LastCommit = batch.TS + Minute
	}
	err = s.saveState()
	s.mu.Unlock()
	// The batch ledger makes a retry safe even if recording process metadata failed.
	return err
}
func (s *Store) Status(ctx context.Context) (Status, error) {
	s.catalog.RLock()
	defer s.catalog.RUnlock()
	st := Status{Enabled: true, SchemaVersion: schemaVersion, BudgetBytes: s.opt.BudgetBytes, ReserveBytes: s.opt.ReserveBytes, Shards: []ShardStatus{}, Gaps: []Gap{}}
	var err error
	st.FreeBytes, err = s.opt.FreeSpace(s.opt.Dir)
	if err != nil {
		return st, err
	}
	all, err := s.shards()
	if err != nil {
		return st, err
	}
	for _, v := range all {
		n := s.fileBytes(v.name)
		st.Bytes += n
		db, release, e := s.acquire(ctx, v.name, false)
		if e != nil {
			return st, e
		}
		var min, max sql.NullInt64
		e = db.QueryRowContext(ctx, "SELECT MIN(ts),MAX(ts) FROM coverage").Scan(&min, &max)
		release()
		if e != nil {
			return st, e
		}
		watermark := int64(0)
		if max.Valid {
			watermark = max.Int64 + v.resolution
			if watermark > st.Freshness {
				st.Freshness = watermark
			}
		}
		if min.Valid && (st.AvailableStart == 0 || min.Int64 < st.AvailableStart) {
			st.AvailableStart = min.Int64
		}
		st.Shards = append(st.Shards, ShardStatus{v.name, v.resolution, v.start, v.end, n, watermark})
	}
	s.mu.Lock()
	st.Gaps = append(st.Gaps, s.state.Gaps...)
	st.Paused = s.state.Paused
	st.LastError = s.state.LastError
	st.WriteFailures = s.state.Failures
	s.mu.Unlock()
	return st, nil
}
func (s *Store) enter(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
