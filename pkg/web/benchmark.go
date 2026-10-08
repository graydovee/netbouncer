package web

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/history"
	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/store"
	"github.com/labstack/echo/v4"
)

type benchmarkMonitor struct{ stats map[string]*core.TrafficStats }

func (m *benchmarkMonitor) GetStats() map[string]*core.TrafficStats { return m.stats }

type benchmarkFirewall struct{}

func (benchmarkFirewall) Init([]store.IpNet) error                 { return nil }
func (benchmarkFirewall) Ban(string, string) error                 { return nil }
func (benchmarkFirewall) Allow(string) error                       { return nil }
func (benchmarkFirewall) RevertBan(string, string) error           { return nil }
func (benchmarkFirewall) RevertAllow(string) error                 { return nil }
func (benchmarkFirewall) CleanupIpNet(string) error                { return nil }
func (benchmarkFirewall) ApplyRateLimit(core.RateLimitRule) error  { return nil }
func (benchmarkFirewall) RemoveRateLimit(core.RateLimitRule) error { return nil }

type BenchmarkMetric struct {
	Path     string  `json:"path"`
	ColdMS   float64 `json:"cold_ms"`
	P95MS    float64 `json:"p95_ms"`
	MaxMS    float64 `json:"max_ms"`
	Errors   int     `json:"errors"`
	BudgetMS float64 `json:"budget_ms"`
	Passed   bool    `json:"passed"`
}
type BenchmarkReport struct {
	Concurrency      int               `json:"concurrency"`
	RequestsPerRoute int               `json:"requests_per_route"`
	IPs              int               `json:"ips"`
	Rules            int               `json:"rules"`
	HistoryRows      int               `json:"history_rows"`
	StorageBytes     int64             `json:"storage_bytes"`
	Metrics          []BenchmarkMetric `json:"metrics"`
	Passed           bool              `json:"passed"`
}

// RunBenchmark uses synthetic data and real authenticated HTTP routes, without capture
// or firewall privileges. The temporary fixture is removed on completion.
func RunBenchmark(ctx context.Context, parent string) (BenchmarkReport, error) {
	report := BenchmarkReport{Concurrency: 20, RequestsPerRoute: 100, IPs: 10000, Rules: 9000, HistoryRows: 7347672, Passed: true}
	dir, err := os.MkdirTemp(parent, "netbouncer-benchmark-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(dir)
	st, err := store.NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: filepath.Join(dir, "control.sqlite"), LogLevel: "error"})
	if err != nil {
		return report, err
	}
	defer st.Close()
	group, err := st.IpNetGroupStore.Create("benchmark", "synthetic")
	if err != nil {
		return report, err
	}
	ips := make([]string, 9000)
	for i := range ips {
		ips[i] = fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256)
	}
	if _, err = st.IpNetStore.BatchCreate(ips, group.ID, store.ActionAllow); err != nil {
		return report, err
	}
	mon := &benchmarkMonitor{stats: map[string]*core.TrafficStats{}}
	now := time.Now().Unix() / 3600 * 3600
	for i := 0; i < 10000; i++ {
		ip := fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256)
		mon.stats[ip] = &core.TrafficStats{RemoteIP: ip, LocalIP: "192.0.2.1", BytesRecv: uint64(i) * 100, BytesSent: 100, BytesRecvPerSec: 100, BytesSentPerSec: 10, FirstSeen: time.Unix(now, 0), LastSeen: time.Now(), Ports: map[string]map[uint16]*core.ProtoPortStats{"tcp": {443: {BytesRecv: 100}}}, Protocols: map[string]*core.ProtoPortStats{"tcp": {BytesRecv: 100}}}
	}
	h, err := history.Open(history.Options{Dir: filepath.Join(dir, "history")})
	if err != nil {
		return report, err
	}
	defer h.Close()
	if err = h.SeedBenchmark(ctx, now); err != nil {
		return report, err
	}
	svc := service.NewNetService(mon, benchmarkFirewall{}, st)
	svc.SetHistory(h)
	if err = svc.Init(nil); err != nil {
		return report, err
	}
	auth, err := NewAuthHandler(ctx, &AuthConfig{Enabled: true, Type: "basic", BasicUsername: "benchmark", BasicPassword: uuid.NewString()})
	if err != nil {
		return report, err
	}
	// Basic middleware authentication is measured; no benchmark listener is publicly exposed.
	password := auth.(*BasicAuthHandler).password
	server := NewServer(svc, auth)
	server.echo.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("benchmark", true); return next(c) }
	})
	httpServer := httptest.NewServer(server.echo)
	defer httpServer.Close()
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()
	svc.StartSnapshots(ctx)
	// Concurrent current-minute samples while the historical pages are being read.
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ts := time.Now().Unix() / 60 * 60
				_ = h.Write(ctx, history.Batch{ID: uuid.NewString(), TS: ts, Deltas: []history.Delta{{IP: "192.0.2.5", Proto: "tcp", Port: 443, BytesIn: 100, PacketsIn: 1, LastSeen: time.Now().Unix()}}})
			}
		}
	}()
	defer func() {
		cancel()
		<-writerDone
		wait, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = svc.WaitSnapshots(wait)
	}()
	routes := []struct {
		path   string
		budget float64
	}{{"/api/traffic?page_size=25", 300}, {"/api/traffic/overview", 300}, {fmt.Sprintf("/api/traffic/history?start=%d&end=%d&bucket=900", now-86400, now), 1000}, {fmt.Sprintf("/api/traffic/history/top?start=%d&end=%d&limit=10", now-86400, now), 1000}, {fmt.Sprintf("/api/traffic/history?start=%d&end=%d&bucket=14400", now-30*86400, now), 2000}, {fmt.Sprintf("/api/traffic/history/top?start=%d&end=%d&limit=10", now-30*86400, now), 2000}}
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(path string) (float64, error) {
		t := time.Now()
		req, e := http.NewRequestWithContext(ctx, "GET", httpServer.URL+path, nil)
		if e != nil {
			return 0, e
		}
		req.SetBasicAuth("benchmark", password)
		r, e := client.Do(req)
		if e != nil {
			return float64(time.Since(t).Microseconds()) / 1000, e
		}
		defer r.Body.Close()
		var body struct {
			Code int `json:"code"`
		}
		e = json.NewDecoder(r.Body).Decode(&body)
		if e == nil && (r.StatusCode != 200 || body.Code != 200) {
			e = fmt.Errorf("HTTP %d code %d", r.StatusCode, body.Code)
		}
		return float64(time.Since(t).Microseconds()) / 1000, e
	}
	for _, route := range routes {
		metric := BenchmarkMetric{Path: route.path, BudgetMS: route.budget}
		metric.ColdMS, err = request(route.path)
		if err != nil {
			metric.Errors++
		}
		values := make([]float64, 100)
		var mu sync.Mutex
		next := 0
		var wg sync.WaitGroup
		for worker := 0; worker < 20; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					mu.Lock()
					i := next
					next++
					mu.Unlock()
					if i >= 100 {
						return
					}
					ms, e := request(route.path)
					values[i] = ms
					if e != nil {
						mu.Lock()
						metric.Errors++
						mu.Unlock()
					}
				}
			}()
		}
		wg.Wait()
		sort.Float64s(values)
		metric.P95MS = values[int(math.Ceil(float64(len(values))*.95))-1]
		metric.MaxMS = values[len(values)-1]
		metric.Passed = metric.Errors == 0 && metric.P95MS <= route.budget && metric.ColdMS <= route.budget
		report.Passed = report.Passed && metric.Passed
		report.Metrics = append(report.Metrics, metric)
	}
	status, err := h.Status(ctx)
	if err != nil {
		return report, err
	}
	report.StorageBytes = status.Bytes
	return report, nil
}
