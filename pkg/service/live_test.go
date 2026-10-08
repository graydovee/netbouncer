package service

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

func TestRuleIndexIPv6AllowPrecedenceAndExpiry(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	rules := []store.IpNet{{ID: 1, IpNet: "2001:db8::/32", Action: store.ActionBan}, {ID: 2, IpNet: "2001:db8:1::/48", Action: store.ActionAllow}, {ID: 3, IpNet: "2001:db8:2::1", Action: store.ActionAllow, ExpiresAt: &past}, {ID: 4, IpNet: "1.2.3.0/24", Action: store.ActionBan}, {ID: 5, IpNet: "1.2.3.4", Action: store.ActionAllow}}
	idx := NewRuleIndex(rules, 1)
	for _, ip := range []string{"2001:db8:1::5", "1.2.3.4"} {
		if !idx.Allowed(ip) || idx.Banned(ip) {
			t.Fatal(ip)
		}
	}
	for _, ip := range []string{"2001:db8:2::1", "1.2.3.5"} {
		if idx.Allowed(ip) || !idx.Banned(ip) {
			t.Fatal(ip)
		}
	}
	if idx.Exact("2001:db8:2::1").id != 0 {
		t.Fatal("expired rule visible")
	}
	if p, ok := parsePrefix("2001:db8::1"); !ok || p.Bits() != 128 {
		t.Fatal(p)
	}
	if p := parseIpNet("2001:db8::1"); p == nil || !p.Contains(netip.MustParseAddr("2001:db8::1").AsSlice()) {
		t.Fatal("IPv6 exact parser")
	}
}
func TestRuleCacheUpdatesAfterMutationAndExpiry(t *testing.T) {
	env := newTestEnv(t)
	idx, err := env.svc.rules()
	if err != nil {
		t.Fatal(err)
	}
	if idx.Banned("8.8.8.8") {
		t.Fatal("initial ban")
	}
	if err = env.svc.CreateOrUpdateIpNet("8.8.8.8", 0, store.ActionBan); err != nil {
		t.Fatal(err)
	}
	idx, err = env.svc.rules()
	if err != nil || !idx.Banned("8.8.8.8") {
		t.Fatalf("mutation cache %v", err)
	}
	if err = env.svc.CreateOrUpdateIpNet("8.8.8.8", 0, store.ActionAllow); err != nil {
		t.Fatal(err)
	}
	idx, _ = env.svc.rules()
	if idx.Banned("8.8.8.8") || !idx.Allowed("8.8.8.8") {
		t.Fatal("allow mutation not reflected")
	}
}
func TestPageAndOverviewAreIndependentOfPagination(t *testing.T) {
	env := newTestEnv(t)
	mon := env.svc.monitor.(*fakeMonitor)
	mon.stats = map[string]*core.TrafficStats{}
	for i := 0; i < 101; i++ {
		ip := fmt.Sprintf("1.2.3.%d", i)
		mon.stats[ip] = &core.TrafficStats{RemoteIP: ip, LocalIP: "10.0.0.1", BytesRecvPerSec: 1, BytesSentPerSec: 2, BytesRecv: uint64(i), Ports: map[string]map[uint16]*core.ProtoPortStats{"tcp": {80: {BytesRecv: 5}}}, Protocols: map[string]*core.ProtoPortStats{"tcp": {BytesRecv: 5}}}
	}
	p, err := env.svc.TrafficPage(0, 500, "total_bytes_in", "desc", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 100 || p.Total != 101 || p.Items[0].TotalBytesIn != 100 {
		t.Fatalf("page %+v", p)
	}
	for _, r := range p.Items {
		if len(r.Ports) != 0 || len(r.Protocols) != 0 {
			t.Fatal("details leaked into page")
		}
	}
	v, err := env.svc.TrafficOverview()
	if err != nil {
		t.Fatal(err)
	}
	if v.Total != 101 || v.BytesInPerSec != 101 || v.BytesOutPerSec != 202 {
		t.Fatalf("overview %+v", v)
	}
	filtered, err := env.svc.TrafficPage(0, 25, "remote_ip", "asc", "1.2.3.100", "")
	if err != nil || filtered.Total != 1 {
		t.Fatalf("filter %+v %v", filtered, err)
	}
	ports, err := env.svc.GetPortTraffic()
	if err != nil || len(ports) != 1 || ports[0].IPCount != 101 {
		t.Fatalf("ports without policy engine %+v %v", ports, err)
	}
}
func BenchmarkRuleIndex9000Rules10000IPs(b *testing.B) {
	rules := make([]store.IpNet, 9000)
	for i := range rules {
		rules[i] = store.IpNet{IpNet: fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256), Action: store.ActionAllow}
	}
	idx := NewRuleIndex(rules, 1)
	ips := make([]string, 10000)
	for i := range ips {
		ips[i] = fmt.Sprintf("10.%d.%d.%d", i/65536, (i/256)%256, i%256)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ip := range ips {
			idx.Banned(ip)
		}
	}
}
