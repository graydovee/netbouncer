package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Maintain is serialized with writes. cutoff is the first minute still pending in the collector.
// Target coverage is committed together with its metrics: a crash cannot publish a false watermark.
func (s *Store) Maintain(ctx context.Context, cutoff int64) error {
	s.writer.Lock()
	defer s.writer.Unlock()
	for _, pair := range [][2]int64{{Minute, TenMinutes}, {TenMinutes, Hour}} {
		if err := s.roll(ctx, pair[0], pair[1], cutoff); err != nil {
			s.failure(err)
			return err
		}
	}
	now := time.Now().Unix()
	s.catalog.Lock()
	defer s.catalog.Unlock()
	all, err := s.shards()
	if err != nil {
		return err
	}
	for _, v := range all {
		retention := int64(30 * 86400)
		if v.resolution == Minute {
			retention = 6 * 3600
		}
		if v.resolution == TenMinutes {
			retention = 7 * 86400
		}
		if v.end > now-retention {
			continue
		}
		if v.resolution != Hour {
			// Only expire source shards whose every recorded bucket is durably represented upstairs.
			parent := TenMinutes
			if v.resolution == TenMinutes {
				parent = Hour
			}
			safe, e := s.archived(ctx, v, parent)
			if e != nil {
				return e
			}
			if !safe {
				continue
			}
		}
		if err = s.remove(v, ""); err != nil {
			return err
		}
	}
	return nil
}
func (s *Store) archived(ctx context.Context, v shard, parent int64) (bool, error) {
	db, release, err := s.acquire(ctx, v.name, false)
	if err != nil {
		return false, err
	}
	rows, err := db.QueryContext(ctx, "SELECT DISTINCT (ts / ?) * ? FROM coverage ORDER BY ts", parent, parent)
	if err != nil {
		release()
		return false, err
	}
	starts := []int64{}
	for rows.Next() {
		var ts int64
		if err = rows.Scan(&ts); err != nil {
			break
		}
		starts = append(starts, ts)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	release()
	if err != nil {
		return false, err
	}
	for _, ts := range starts {
		ok, e := s.covered(ctx, parent, ts)
		if e != nil || !ok {
			return false, e
		}
	}
	return true, nil
}
func (s *Store) covered(ctx context.Context, res, ts int64) (bool, error) {
	db, release, err := s.acquire(ctx, nameFor(res, ts), false)
	if err != nil {
		if isMissing(err) {
			return false, nil
		}
		return false, err
	}
	defer release()
	var n int
	err = db.QueryRowContext(ctx, "SELECT 1 FROM coverage WHERE ts=?", ts).Scan(&n)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return n == 1, err
}
func (s *Store) roll(ctx context.Context, source, target, cutoff int64) error {
	all, err := s.shards()
	if err != nil {
		return err
	}
	for _, v := range all {
		if v.resolution != source {
			continue
		}
		db, release, e := s.acquire(ctx, v.name, false)
		if e != nil {
			return e
		}
		rows, e := db.QueryContext(ctx, "SELECT DISTINCT (ts / ?) * ? AS target_ts FROM coverage WHERE ts < ? GROUP BY target_ts ORDER BY target_ts", target, target, cutoff-cutoff%target)
		if e != nil {
			release()
			return e
		}
		starts := []int64{}
		for rows.Next() {
			var ts int64
			if e = rows.Scan(&ts); e != nil {
				break
			}
			starts = append(starts, ts)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		release()
		if e != nil {
			return e
		}
		for _, ts := range starts {
			if ok, e := s.covered(ctx, target, ts); e != nil {
				return e
			} else if ok {
				continue
			}
			if err = s.rollBucket(ctx, v, target, ts); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Store) rollBucket(ctx context.Context, v shard, target, ts int64) error {
	s.catalog.RLock()
	db, release, err := s.acquire(ctx, v.name, false)
	if err != nil {
		s.catalog.RUnlock()
		return err
	}
	var count, complete int
	var seen int64
	err = db.QueryRowContext(ctx, "SELECT COUNT(*),COALESCE(MIN(complete),0),COALESCE(MAX(last_seen),0) FROM coverage WHERE ts>=? AND ts<?", ts, ts+target).Scan(&count, &complete, &seen)
	var metrics []metric
	if err == nil {
		var rows *sql.Rows
		rows, err = db.QueryContext(ctx, `SELECT dim,ip,proto,port,SUM(bytes_in),SUM(bytes_out),SUM(packets_in),SUM(packets_out),MAX(last_seen) FROM metrics WHERE ts>=? AND ts<? GROUP BY dim,ip,proto,port`, ts, ts+target)
		if err == nil {
			for rows.Next() {
				m := metric{}
				m.key.ts = ts
				if err = rows.Scan(&m.key.dim, &m.key.ip, &m.key.proto, &m.key.port, &m.in, &m.out, &m.pin, &m.pout, &m.seen); err != nil {
					break
				}
				metrics = append(metrics, m)
				if len(metrics) > 200000 {
					err = fmt.Errorf("rollup bucket exceeds memory budget")
					break
				}
			}
			if err == nil {
				err = rows.Err()
			}
			rows.Close()
		}
	}
	release()
	s.catalog.RUnlock()
	if err != nil {
		return err
	}
	if count != int(target/v.resolution) {
		complete = 0
	}
	if err = s.enforce(ctx, time.Now().Unix(), int64(len(metrics))*384+256*1024); err != nil {
		return err
	}
	s.catalog.RLock()
	defer s.catalog.RUnlock()
	dest, done, err := s.acquire(ctx, nameFor(target, ts), true)
	if err != nil {
		return err
	}
	defer done()
	tx, err := dest.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, insertMetric)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, m := range metrics {
		k := m.key
		if _, err = stmt.ExecContext(ctx, k.dim, ts, k.ip, k.proto, k.port, m.in, m.out, m.pin, m.pout, m.seen); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO coverage(ts,complete,last_seen) VALUES(?,?,?)", ts, complete, seen); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if complete == 0 {
		s.mu.Lock()
		s.recordGapLocked(ts, ts+target, "incomplete_rollup")
		err = s.saveState()
		s.mu.Unlock()
	}
	// Bound WAL size without waiting for active readers. Subsequent writes/checkpoints retry naturally.
	_, _ = dest.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)")
	return err
}
