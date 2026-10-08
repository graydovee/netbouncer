package history

import (
	"context"
	"errors"
)

// SeedBenchmark creates a reproducible 4.3M-row fixture in an EMPTY store only.
// It is used by the isolated CLI performance check; it cannot change live history.
func (s *Store) SeedBenchmark(ctx context.Context, now int64) error {
	s.writer.Lock()
	defer s.writer.Unlock()
	all, err := s.shards()
	if err != nil {
		return err
	}
	if len(all) != 0 {
		return errors.New("benchmark requires empty history directory")
	}
	end := now / Hour * Hour
	start := end - 30*86400
	for day := start / 86400 * 86400; day < end; day += 86400 {
		db, done, e := s.acquire(ctx, nameFor(Hour, day), true)
		if e != nil {
			return e
		}
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			done()
			return e
		}
		for ts := max(day, start); ts < min(day+86400, end); ts += Hour {
			_, e = tx.ExecContext(ctx, `WITH RECURSIVE ips(i) AS (SELECT 0 UNION ALL SELECT i+1 FROM ips WHERE i<1999), dims(d) AS (VALUES(1),(4),(5)) INSERT INTO metrics SELECT d,?,printf('10.%d.%d.%d',i/65536,(i/256)%256,i%256),CASE WHEN d=1 THEN '' ELSE 'tcp' END,CASE WHEN d=5 THEN 443 ELSE -1 END,6000,1200,60,60,? FROM ips CROSS JOIN dims`, ts, ts+30)
			if e != nil {
				break
			}
			_, e = tx.ExecContext(ctx, `INSERT INTO metrics VALUES(0,?,'','',-1,12000000,2400000,120000,120000,?),(2,?,'','tcp',-1,12000000,2400000,120000,120000,?),(3,?,'','tcp',443,12000000,2400000,120000,120000,?)`, ts, ts+30, ts, ts+30, ts, ts+30)
			if e != nil {
				break
			}
			_, e = tx.ExecContext(ctx, "INSERT INTO coverage VALUES(?,1,?)", ts, ts+30)
			if e != nil {
				break
			}
		}
		if e != nil {
			tx.Rollback()
			done()
			return e
		}
		if e = tx.Commit(); e != nil {
			done()
			return e
		}
		if _, e = db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); e != nil {
			done()
			return e
		}
		done()
	}
	// Include fine recent data so the planner also exercises coarse/fine joins.
	for _, res := range []int64{Minute, TenMinutes} {
		from := end - 6*3600
		if res == TenMinutes {
			from = end - 86400
		}
		for ts := from; ts < end; ts += res {
			db, done, e := s.acquire(ctx, nameFor(res, ts), true)
			if e != nil {
				return e
			}
			tx, e := db.BeginTx(ctx, nil)
			if e != nil {
				done()
				return e
			}
			_, e = tx.ExecContext(ctx, `WITH RECURSIVE ips(i) AS (SELECT 0 UNION ALL SELECT i+1 FROM ips WHERE i<1999), dims(d) AS (VALUES(1),(4),(5)) INSERT INTO metrics SELECT d,?,printf('10.%d.%d.%d',i/65536,(i/256)%256,i%256),CASE WHEN d=1 THEN '' ELSE 'tcp' END,CASE WHEN d=5 THEN 443 ELSE -1 END,?,?,?, ?,? FROM ips CROSS JOIN dims`, ts, res/60*100, res/60*20, res/60, res/60, ts+res-1)
			if e == nil {
				_, e = tx.ExecContext(ctx, `INSERT INTO metrics VALUES(0,?,'','',-1,?,?,?,?,?),(2,?,'','tcp',-1,?,?,?,?,?),(3,?,'','tcp',443,?,?,?,?,?)`, ts, res/60*200000, res/60*40000, res/60*2000, res/60*2000, ts+res-1, ts, res/60*200000, res/60*40000, res/60*2000, res/60*2000, ts+res-1, ts, res/60*200000, res/60*40000, res/60*2000, res/60*2000, ts+res-1)
			}
			if e == nil {
				_, e = tx.ExecContext(ctx, "INSERT INTO coverage VALUES(?,1,?)", ts, ts+res-1)
			}
			if e != nil {
				tx.Rollback()
				done()
				return e
			}
			e = tx.Commit()
			if e == nil {
				_, e = db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
			}
			done()
			if e != nil {
				return e
			}

		}
	}
	return nil
}
