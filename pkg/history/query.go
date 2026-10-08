package history

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type coverage struct {
	ts, res  int64
	complete bool
	name     string
}
type segment struct {
	start, end, res int64
	name            string
}

func isMissing(err error) bool    { return os.IsNotExist(err) }
func roundUp(v, unit int64) int64 { return (v + unit - 1) / unit * unit }
func dimension(q Query) int {
	switch q.Kind {
	case "ip_top":
		return DimIP
	case "ports", "port_top":
		if q.IP != "" {
			return DimIPPort
		}
		return DimPort
	case "protocols":
		if q.IP != "" {
			return DimIPProto
		}
		return DimProto
	default:
		if q.IP != "" {
			return DimIP
		}
		return DimTotal
	}
}
func (s *Store) Query(ctx context.Context, q Query) (Result, error) {
	result := Result{Items: []Point{}, Meta: Meta{Gaps: []Gap{}}}
	now := time.Now().Unix()
	if q.End <= 0 || q.End > now {
		q.End = now
	}
	if q.Start <= 0 {
		q.Start = q.End - 86400
	}
	if q.Start >= q.End {
		return result, errors.New("invalid history range")
	}
	if q.End-q.Start > 30*86400 {
		q.Start = q.End - 30*86400
	}
	if q.Bucket < Minute {
		q.Bucket = Minute
	}
	if q.Bucket > 86400 {
		q.Bucket = 86400
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.enter(ctx); err != nil {
		return result, err
	}
	defer func() { <-s.slots }()
	s.catalog.RLock()
	defer s.catalog.RUnlock()
	all, err := s.shards()
	if err != nil {
		return result, err
	}
	cover := []coverage{}
	bounds := map[string][2]int64{}
	minAvailable := int64(0)
	fresh := int64(0)
	for _, v := range all {
		if v.end <= q.Start-3600 || v.start >= q.End+3600 {
			continue
		}
		db, release, e := s.acquire(ctx, v.name, false)
		if e != nil {
			return result, e
		}
		var first, last int64
		if e = db.QueryRowContext(ctx, "SELECT COALESCE(MIN(ts),0),COALESCE(MAX(ts),0) FROM coverage").Scan(&first, &last); e != nil {
			release()
			return result, e
		}
		bounds[v.name] = [2]int64{first, last + v.resolution}
		rows, e := db.QueryContext(ctx, "SELECT ts,complete FROM coverage WHERE ts>=? AND ts<? ORDER BY ts", q.Start-q.Start%Hour, roundUp(q.End, Hour))
		if e != nil {
			release()
			return result, e
		}
		for rows.Next() {
			var ts int64
			var complete int
			if e = rows.Scan(&ts, &complete); e != nil {
				break
			}
			cover = append(cover, coverage{ts, v.resolution, complete == 1, v.name})
			if minAvailable == 0 || ts < minAvailable {
				minAvailable = ts
			}
			if ts+v.resolution > fresh {
				fresh = ts + v.resolution
			}
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		release()
		if e != nil {
			return result, e
		}
	}
	// Prefer the coarsest bucket compatible with the requested resolution. Where coarse
	// coverage has not committed yet, consume the actual finer coverage instead.
	res := Minute
	if q.End-q.Start > 6*3600 || q.Bucket >= TenMinutes {
		res = TenMinutes
	}
	if q.End-q.Start > 7*86400 || q.Bucket >= Hour {
		res = Hour
	}
	// Promote only when finer coverage at the requested start is actually unavailable.
	finest := int64(0)
	for _, c := range cover {
		if c.ts <= q.Start && c.ts+c.res > q.Start && (finest == 0 || c.res < finest) {
			finest = c.res
		}
	}
	if finest > res {
		res = finest
	}

	bucket := roundUp(q.Bucket, res)
	bucket = roundUp(max(bucket, (q.End-q.Start+718)/719), res)
	start := q.Start / res * res
	end := min(roundUp(q.End, res), roundUp(now, Minute))
	result.Meta = Meta{Start: start, End: end, Bucket: bucket, Freshness: min(fresh, now), AvailableStart: minAvailable, Gaps: []Gap{}}
	sort.Slice(cover, func(i, j int) bool {
		if cover[i].ts != cover[j].ts {
			return cover[i].ts < cover[j].ts
		}
		return cover[i].res > cover[j].res
	})
	// Represent coverage on the minute grid; select each interval once. No Top-N truncation here.
	slots := map[int64]coverage{}
	for _, c := range cover {
		if c.res > res || c.ts < start || c.ts+c.res > end {
			continue
		}
		free := true
		for t := c.ts; t < c.ts+c.res; t += Minute {
			if _, ok := slots[t]; ok {
				free = false
				break
			}
		}
		if free {
			for t := c.ts; t < c.ts+c.res; t += Minute {
				slots[t] = c
			}
		}
	}
	segments := []segment{}
	for t := start; t < end; {
		c, ok := slots[t]
		if !ok {
			from := t
			for t < end {
				if _, ok := slots[t]; ok {
					break
				}
				t += Minute
			}
			result.Meta.Gaps = append(result.Meta.Gaps, Gap{from, t, "unavailable"})
			continue
		}
		if !c.complete {
			result.Meta.Gaps = append(result.Meta.Gaps, Gap{c.ts, c.ts + c.res, "incomplete_rollup"})
		}
		n := len(segments)
		if n > 0 && segments[n-1].name == c.name && segments[n-1].end == c.ts {
			segments[n-1].end = c.ts + c.res
		} else {
			segments = append(segments, segment{c.ts, c.ts + c.res, c.res, c.name})
		}
		t = c.ts + c.res
	}
	s.mu.Lock()
	for _, g := range s.state.Gaps {
		if g.Reason == "capacity_eviction" || g.Reason == "incomplete_rollup" {
			continue
		}
		if g.Start < end && g.End > start {
			result.Meta.Gaps = append(result.Meta.Gaps, Gap{max(g.Start, start), min(g.End, end), g.Reason})
		}
	}
	s.mu.Unlock()
	type key struct {
		ts        int64
		ip, proto string
		port      int
	}
	merged := map[key]*Point{}
	dim := dimension(q)
	top := q.Kind == "ip_top" || q.Kind == "port_top"
	for _, seg := range segments {
		db, release, e := s.acquire(ctx, seg.name, false)
		if e != nil {
			return result, e
		}
		where := "dim=? AND ts>=? AND ts<?"
		args := []any{dim, seg.start, seg.end}
		if q.IP != "" {
			where += " AND ip=?"
			args = append(args, q.IP)
		}
		if q.Proto != "" {
			where += " AND proto=?"
			args = append(args, q.Proto)
		}
		if q.Port >= 0 {
			where += " AND port=?"
			args = append(args, q.Port)
		}
		projection := "(ts / ?) * ? AS output_ts"
		group := "output_ts,ip,proto,port"
		if top {
			projection = "0 AS output_ts"
			group = "ip,proto,port"
		} else {
			args = append([]any{bucket, bucket}, args...)
		}
		query := fmt.Sprintf(`SELECT %s,ip,proto,port,SUM(bytes_in),SUM(bytes_out),SUM(packets_in),SUM(packets_out),MAX(last_seen) FROM metrics WHERE %s GROUP BY %s`, projection, where, group)
		if b := bounds[seg.name]; top && seg.start <= b[0] && seg.end >= b[1] {
			query = fmt.Sprintf(`SELECT 0,ip,proto,port,bytes_in,bytes_out,packets_in,packets_out,last_seen FROM totals WHERE %s`, strings.ReplaceAll(where, " AND ts>=? AND ts<?", ""))
			args = append([]any{dim}, args[3:]...)
		}
		rows, e := db.QueryContext(ctx, query, args...)
		if e != nil {
			release()
			return result, e
		}
		for rows.Next() {
			p := Point{}
			if e = rows.Scan(&p.TS, &p.IP, &p.Proto, &p.Port, &p.BytesIn, &p.BytesOut, &p.PacketsIn, &p.PacketsOut, &p.LastSeen); e != nil {
				break
			}
			k := key{p.TS, p.IP, p.Proto, p.Port}
			m := merged[k]
			if m == nil {
				cp := p
				merged[k] = &cp
			} else {
				m.BytesIn += p.BytesIn
				m.BytesOut += p.BytesOut
				m.PacketsIn += p.PacketsIn
				m.PacketsOut += p.PacketsOut
				m.LastSeen = max(m.LastSeen, p.LastSeen)
			}
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		release()
		if e != nil {
			return result, e
		}
	}
	for _, p := range merged {
		p.BytesSum = p.BytesIn + p.BytesOut
		result.Items = append(result.Items, *p)
	}
	if q.Kind == "ports" && q.Port < 0 {
		result.Items = limitPortSeries(result.Items, 10)
	}
	sort.Slice(result.Items, func(i, j int) bool {
		a, b := result.Items[i], result.Items[j]
		if top {
			if a.BytesSum != b.BytesSum {
				return a.BytesSum > b.BytesSum
			}
		} else if a.TS != b.TS {
			return a.TS < b.TS
		}
		if a.IP != b.IP {
			return a.IP < b.IP
		}
		if a.Proto != b.Proto {
			return a.Proto < b.Proto
		}
		return a.Port < b.Port
	})
	if top && len(result.Items) > q.Limit {
		result.Items = result.Items[:q.Limit]
	}
	return result, nil
}
func limitPortSeries(points []Point, n int) []Point {
	type key struct {
		proto string
		port  int
	}
	totals := map[key]uint64{}
	for _, p := range points {
		totals[key{p.Proto, p.Port}] += p.BytesSum
	}
	keys := make([]key, 0, len(totals))
	for k := range totals {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if totals[a] != totals[b] {
			return totals[a] > totals[b]
		}
		if a.proto != b.proto {
			return a.proto < b.proto
		}
		return a.port < b.port
	})
	keep := map[key]bool{}
	for i, k := range keys {
		if i < n {
			keep[k] = true
		}
	}
	out := []Point{}
	other := map[int64]*Point{}
	for _, p := range points {
		if keep[key{p.Proto, p.Port}] {
			out = append(out, p)
			continue
		}
		m := other[p.TS]
		if m == nil {
			m = &Point{TS: p.TS, Proto: "other", Port: -1}
			other[p.TS] = m
		}
		m.BytesIn += p.BytesIn
		m.BytesOut += p.BytesOut
		m.PacketsIn += p.PacketsIn
		m.PacketsOut += p.PacketsOut
		m.BytesSum += p.BytesSum
		m.LastSeen = max(m.LastSeen, p.LastSeen)
	}
	for _, p := range other {
		out = append(out, *p)
	}
	return out
}
