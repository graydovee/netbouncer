package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
)

type TrafficPage struct {
	Items      []TrafficData `json:"items"`
	Total      int           `json:"total"`
	SnapshotID int64         `json:"snapshot_id"`
}
type LiveOverview struct {
	BytesInPerSec  float64     `json:"down"`
	BytesOutPerSec float64     `json:"up"`
	Connections    int         `json:"connections"`
	Banned         int         `json:"banned"`
	Risky          int         `json:"risky"`
	Total          int         `json:"total"`
	Protocols      []ProtoStat `json:"protocols"`
	SnapshotID     int64       `json:"snapshot_id"`
}
type liveSnapshot struct {
	at       time.Time
	rows     []TrafficData
	overview LiveOverview
	ports    []PortTraffic
	previous map[string]portCounter
}
type portCounter struct{ in, out, conns uint64 }

func (s *NetService) StartSnapshots(ctx context.Context) {
	s.snapshotDone = make(chan struct{})
	if _, err := s.refreshLive(); err != nil {
		slog.Warn("实时快照初始化失败", "error", err)
	}
	go func() {
		defer close(s.snapshotDone)
		timer := time.NewTicker(5 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				if _, err := s.refreshLive(); err != nil {
					slog.Warn("实时快照刷新失败", "error", err)
				}
			}
		}
	}()
}
func (s *NetService) currentLive() (*liveSnapshot, error) {
	if snap := s.live.Load(); snap != nil && time.Since(snap.at) < 10*time.Second {
		return snap, nil
	}
	return s.refreshLive()
}
func (s *NetService) refreshLive() (*liveSnapshot, error) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	if snap := s.live.Load(); snap != nil && time.Since(snap.at) < time.Second {
		return snap, nil
	}
	var stats map[string]*core.TrafficStats
	if mon, ok := s.monitor.(interface {
		GetSummaryStats() map[string]*core.TrafficStats
	}); ok {
		stats = mon.GetSummaryStats()
	} else {
		stats = s.monitor.GetStats()
	}
	rows, err := s.buildTrafficData(stats)
	if err != nil {
		return nil, err
	}
	snap := &liveSnapshot{at: time.Now(), rows: rows, previous: map[string]portCounter{}}
	overview := LiveOverview{Total: len(rows), SnapshotID: snap.at.UnixMilli(), Protocols: []ProtoStat{}}
	protos := map[string]*ProtoStat{}
	for i := range snap.rows {
		r := &snap.rows[i]
		overview.BytesInPerSec += r.BytesInPerSec
		overview.BytesOutPerSec += r.BytesOutPerSec
		overview.Connections += r.Connections
		if r.IsBanned {
			overview.Banned++
		}
		if r.RiskScore > 0 {
			overview.Risky++
		}
		for _, p := range r.Protocols {
			m := protos[p.Proto]
			if m == nil {
				m = &ProtoStat{Proto: p.Proto}
				protos[p.Proto] = m
			}
			m.BytesIn += p.BytesIn
			m.BytesOut += p.BytesOut
			m.PacketsIn += p.PacketsIn
			m.PacketsOut += p.PacketsOut
		}
		r.Protocols = nil
		r.Ports = nil
	}
	for _, p := range protos {
		overview.Protocols = append(overview.Protocols, *p)
	}
	sort.Slice(overview.Protocols, func(i, j int) bool { return overview.Protocols[i].Proto < overview.Protocols[j].Proto })
	snap.overview = overview
	var portStats map[string]*core.TrafficStats
	if mon, ok := s.monitor.(interface {
		PortCounters() map[string]*core.TrafficStats
	}); ok {
		portStats = mon.PortCounters()
	} else {
		portStats = stats
	}
	elapsed := 5.0
	previous := s.live.Load()
	if previous != nil {
		elapsed = snap.at.Sub(previous.at).Seconds()
	}
	type agg struct {
		item    PortTraffic
		clients map[string]uint64
	}
	ports := map[string]*agg{}
	for ip, st := range portStats {
		for proto, pm := range st.Ports {
			for port, p := range pm {
				key := fmt.Sprintf("%s/%d", proto, port)
				a := ports[key]
				if a == nil {
					a = &agg{item: PortTraffic{Proto: proto, Port: int(port)}, clients: map[string]uint64{}}
					ports[key] = a
				}
				id := ip + "/" + key
				cur := portCounter{p.BytesRecv, p.BytesSent, p.ConnsIn + p.ConnsOut}
				snap.previous[id] = cur
				prev := portCounter{}
				if previous != nil {
					prev = previous.previous[id]
				}
				a.item.BytesIn += cur.in
				a.item.BytesOut += cur.out
				a.item.BytesInPerSec += float64(deltaSub(cur.in, prev.in)) / elapsed
				a.item.BytesOutPerSec += float64(deltaSub(cur.out, prev.out)) / elapsed
				a.item.NewConns += int(deltaSub(cur.conns, prev.conns))
				a.clients[ip] = cur.in + cur.out
			}
		}
	}
	for _, a := range ports {
		clients := []string{}
		for ip := range a.clients {
			clients = append(clients, ip)
		}
		sort.Slice(clients, func(i, j int) bool {
			if a.clients[clients[i]] != a.clients[clients[j]] {
				return a.clients[clients[i]] > a.clients[clients[j]]
			}
			return clients[i] < clients[j]
		})
		a.item.IPCount = len(clients)
		if len(clients) > 5 {
			clients = clients[:5]
		}
		a.item.TopClients = clients
		snap.ports = append(snap.ports, a.item)
	}
	sort.Slice(snap.ports, func(i, j int) bool {
		a, b := snap.ports[i], snap.ports[j]
		if a.BytesInPerSec+a.BytesOutPerSec != b.BytesInPerSec+b.BytesOutPerSec {
			return a.BytesInPerSec+a.BytesOutPerSec > b.BytesInPerSec+b.BytesOutPerSec
		}
		if a.Proto != b.Proto {
			return a.Proto < b.Proto
		}
		return a.Port < b.Port
	})
	if len(snap.ports) > 200 {
		snap.ports = snap.ports[:200]
	}
	if snap.ports == nil {
		snap.ports = []PortTraffic{}
	}
	s.live.Store(snap)
	return snap, nil
}
func (s *NetService) TrafficOverview() (LiveOverview, error) {
	snap, err := s.currentLive()
	if err != nil {
		return LiveOverview{}, err
	}
	return snap.overview, nil
}
func (s *NetService) TrafficDetail(ip string) (TrafficData, error) {
	var st *core.TrafficStats
	if mon, ok := s.monitor.(interface {
		GetIPStats(string) *core.TrafficStats
	}); ok {
		st = mon.GetIPStats(ip)
	} else {
		st = s.monitor.GetStats()[ip]
	}
	if st == nil {
		return TrafficData{}, NotFoundf("IP 不在实时监控中")
	}
	rows, err := s.buildTrafficData(map[string]*core.TrafficStats{ip: st})
	if err != nil {
		return TrafficData{}, err
	}
	return rows[0], nil
}
func (s *NetService) TrafficPage(page, size int, sortKey, order, remote, local string) (TrafficPage, error) {
	if page < 0 {
		page = 0
	}
	if size <= 0 {
		size = 25
	}
	if size > 100 {
		size = 100
	}
	if sortKey == "" {
		sortKey = "bytes_out_per_sec"
	}
	value := func(r TrafficData) float64 {
		switch sortKey {
		case "total":
			return float64(r.TotalBytesIn + r.TotalBytesOut)
		case "total_bytes_in":
			return float64(r.TotalBytesIn)
		case "total_bytes_out":
			return float64(r.TotalBytesOut)
		case "total_packets_in":
			return float64(r.TotalPacketsIn)
		case "total_packets_out":
			return float64(r.TotalPacketsOut)
		case "bytes_in_per_sec":
			return r.BytesInPerSec
		case "bytes_out_per_sec":
			return r.BytesOutPerSec
		case "connections":
			return float64(r.Connections)
		case "risk_score":
			return float64(r.RiskScore)
		}
		return 0
	}
	switch sortKey {
	case "remote_ip", "local_ip", "first_seen", "last_seen", "total", "total_bytes_in", "total_bytes_out", "total_packets_in", "total_packets_out", "bytes_in_per_sec", "bytes_out_per_sec", "connections", "risk_score":
	default:
		return TrafficPage{}, Invalidf("无效排序字段")
	}
	if order != "" && order != "asc" && order != "desc" {
		return TrafficPage{}, Invalidf("无效排序方向")
	}
	snap, err := s.currentLive()
	if err != nil {
		return TrafficPage{}, err
	}
	rows := make([]TrafficData, 0, len(snap.rows))
	remote = strings.ToLower(remote)
	local = strings.ToLower(local)
	for _, r := range snap.rows {
		if strings.Contains(strings.ToLower(r.RemoteIP), remote) && strings.Contains(strings.ToLower(r.LocalIP), local) {
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		cmp := 0
		switch sortKey {
		case "remote_ip":
			cmp = strings.Compare(a.RemoteIP, b.RemoteIP)
		case "local_ip":
			cmp = strings.Compare(a.LocalIP, b.LocalIP)
		case "first_seen":
			cmp = strings.Compare(a.FirstSeen, b.FirstSeen)
		case "last_seen":
			cmp = strings.Compare(a.LastSeen, b.LastSeen)
		default:
			av, bv := value(a), value(b)
			if av < bv {
				cmp = -1
			} else if av > bv {
				cmp = 1
			}
		}
		if cmp == 0 {
			return a.RemoteIP < b.RemoteIP
		}
		if order == "asc" {
			return cmp < 0
		}
		return cmp > 0
	})
	total := len(rows)
	offset := total
	if page <= total/size {
		offset = min(page*size, total)
	}
	rows = rows[offset:min(offset+size, total)]
	if rows == nil {
		rows = []TrafficData{}
	}
	return TrafficPage{rows, total, snap.overview.SnapshotID}, nil
}
func deltaSub(current, prev uint64) uint64 {
	if current < prev {
		return current
	}
	return current - prev
}

func (s *NetService) WaitSnapshots(ctx context.Context) error {
	if s.snapshotDone == nil {
		return nil
	}
	select {
	case <-s.snapshotDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
