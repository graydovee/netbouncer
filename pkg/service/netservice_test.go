package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/store"
)

// fakeMonitor 用于测试的监控器
type fakeMonitor struct {
	stats map[string]*core.TrafficStats
}

func (f *fakeMonitor) GetStats() map[string]*core.TrafficStats { return f.stats }

// fakeFirewall 记录调用的防火墙
type fakeFirewall struct {
	banned    map[string]bool
	allowed   map[string]bool
	failOn    map[string]error
	initRules []store.IpNet
}

func newFakeFirewall() *fakeFirewall {
	return &fakeFirewall{
		banned:  map[string]bool{},
		allowed: map[string]bool{},
		failOn:  map[string]error{},
	}
}

func (f *fakeFirewall) Init(ipList []store.IpNet) error {
	f.initRules = ipList
	return nil
}
func (f *fakeFirewall) Ban(ipNet string, direction string) error {
	if err, ok := f.failOn[ipNet]; ok {
		return err
	}
	f.banned[ipNet] = true
	return nil
}
func (f *fakeFirewall) RevertBan(ipNet string, direction string) error {
	delete(f.banned, ipNet)
	return nil
}
func (f *fakeFirewall) Allow(ipNet string) error {
	if err, ok := f.failOn[ipNet]; ok {
		return err
	}
	f.allowed[ipNet] = true
	return nil
}
func (f *fakeFirewall) RevertAllow(ipNet string) error {
	delete(f.allowed, ipNet)
	return nil
}
func (f *fakeFirewall) ApplyRateLimit(rule core.RateLimitRule) error {
	return nil
}

func (f *fakeFirewall) RemoveRateLimit(rule core.RateLimitRule) error {
	return nil
}

func (f *fakeFirewall) CleanupIpNet(ipNet string) error {
	delete(f.banned, ipNet)
	delete(f.allowed, ipNet)
	return nil
}

type testEnv struct {
	svc   *NetService
	fw    *fakeFirewall
	store *store.Store
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dbFile := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: dbFile, LogLevel: "silent"})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	fw := newFakeFirewall()
	mon := &fakeMonitor{stats: map[string]*core.TrafficStats{
		"8.8.8.8": {RemoteIP: "8.8.8.8", LocalIP: "10.0.0.1"},
		"1.1.1.1": {RemoteIP: "1.1.1.1", LocalIP: "10.0.0.1"},
	}}
	svc := NewNetService(mon, fw, st)

	if err := svc.Init(nil); err != nil {
		t.Fatalf("init service: %v", err)
	}

	return &testEnv{svc: svc, fw: fw, store: st}
}

func TestInitCreatesDefaultGroup(t *testing.T) {
	env := newTestEnv(t)

	group, err := env.store.IpNetGroupStore.FindDefault()
	if err != nil {
		t.Fatalf("default group should exist: %v", err)
	}
	if group.Name != DefaultGroupName {
		t.Errorf("default group name = %s", group.Name)
	}
}

func TestCreateOrUpdateIpNet(t *testing.T) {
	env := newTestEnv(t)

	if err := env.svc.CreateOrUpdateIpNet("192.168.1.1", 0, store.ActionBan); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !env.fw.banned["192.168.1.1"] {
		t.Fatal("firewall should have banned the ip")
	}

	// 重复创建仅更新action
	if err := env.svc.CreateOrUpdateIpNet("192.168.1.1", 0, store.ActionAllow); err != nil {
		t.Fatalf("update: %v", err)
	}
	if env.fw.banned["192.168.1.1"] {
		t.Error("ban should have been reverted")
	}
	if !env.fw.allowed["192.168.1.1"] {
		t.Error("allow should have been applied")
	}

	ipNet, err := env.store.IpNetStore.FindByIpNet("192.168.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if ipNet.Action != store.ActionAllow {
		t.Errorf("stored action = %s", ipNet.Action)
	}
}

func TestCreateIpNetInvalidAction(t *testing.T) {
	env := newTestEnv(t)
	if err := env.svc.CreateOrUpdateIpNet("192.168.1.1", 0, "deny"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestCreateIpNetGroupNotFound(t *testing.T) {
	env := newTestEnv(t)
	err := env.svc.CreateOrUpdateIpNet("192.168.1.1", 999, store.ActionBan)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteIpNetNotFound(t *testing.T) {
	env := newTestEnv(t)
	if err := env.svc.DeleteIpNet(12345); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteIpNet(t *testing.T) {
	env := newTestEnv(t)

	if err := env.svc.CreateOrUpdateIpNet("10.1.1.1", 0, store.ActionBan); err != nil {
		t.Fatal(err)
	}
	ipNet, _ := env.store.IpNetStore.FindByIpNet("10.1.1.1")

	if err := env.svc.DeleteIpNet(ipNet.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if env.fw.banned["10.1.1.1"] {
		t.Error("firewall rule should be cleaned up")
	}
	if env.store.IpNetStore.ExistsByIpNet("10.1.1.1") {
		t.Error("record should be deleted")
	}
}

func TestUpdateIpNetAction(t *testing.T) {
	env := newTestEnv(t)

	if err := env.svc.CreateOrUpdateIpNet("10.1.1.2", 0, store.ActionBan); err != nil {
		t.Fatal(err)
	}
	ipNet, _ := env.store.IpNetStore.FindByIpNet("10.1.1.2")

	if err := env.svc.UpdateIpNetAction(ipNet.ID, store.ActionAllow); err != nil {
		t.Fatalf("update action: %v", err)
	}
	if env.fw.banned["10.1.1.2"] || !env.fw.allowed["10.1.1.2"] {
		t.Error("firewall actions not switched")
	}

	if err := env.svc.UpdateIpNetAction(ipNet.ID, "invalid"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestBatchOperations(t *testing.T) {
	env := newTestEnv(t)

	var ids []uint
	for _, ip := range []string{"10.2.0.1", "10.2.0.2", "10.2.0.3"} {
		if err := env.svc.CreateOrUpdateIpNet(ip, 0, store.ActionBan); err != nil {
			t.Fatal(err)
		}
		item, _ := env.store.IpNetStore.FindByIpNet(ip)
		ids = append(ids, item.ID)
	}

	success, err := env.svc.UpdateIpNetActions(ids[:2], store.ActionAllow)
	if err != nil || success != 2 {
		t.Fatalf("batch update: success=%d err=%v", success, err)
	}

	success, err = env.svc.DeleteIpNets(ids)
	if err != nil || success != 3 {
		t.Fatalf("batch delete: success=%d err=%v", success, err)
	}
	if total := env.store.IpNetStore.ExistsByIpNet("10.2.0.1"); total {
		t.Error("records should be deleted")
	}
}

func TestListIpNetsPaginationAndFilter(t *testing.T) {
	env := newTestEnv(t)

	group, _ := env.store.IpNetGroupStore.Create("page-group", "")
	for i := 1; i <= 5; i++ {
		ip := "10.3.0." + string(rune('0'+i))
		if i <= 3 {
			if err := env.svc.CreateOrUpdateIpNet(ip, group.ID, store.ActionBan); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := env.svc.CreateOrUpdateIpNet(ip, 0, store.ActionBan); err != nil {
				t.Fatal(err)
			}
		}
	}

	result, err := env.svc.ListIpNets(IpNetListParams{Page: 1, PageSize: 2, GroupID: group.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("total = %d, want 3", result.Total)
	}
	if len(result.Items) != 2 {
		t.Errorf("items = %d, want 2", len(result.Items))
	}
	if result.Items[0].Group == nil || result.Items[0].Group.Name != "page-group" {
		t.Error("group info should be attached")
	}

	// 搜索
	result, _ = env.svc.ListIpNets(IpNetListParams{Search: "10.3.0.1"})
	if result.Total != 1 {
		t.Errorf("search total = %d, want 1", result.Total)
	}
}

func TestListAllGroupsIncludesCount(t *testing.T) {
	env := newTestEnv(t)

	if err := env.svc.CreateOrUpdateIpNet("10.4.0.1", 0, store.ActionBan); err != nil {
		t.Fatal(err)
	}

	groups, err := env.svc.ListAllGroups()
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, g := range groups {
		if g.IsDefault && g.IPCount == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("default group should have ip_count=1, got %+v", groups)
	}
}

func TestImportIpNetCountsFirewallFailures(t *testing.T) {
	env := newTestEnv(t)

	group, _ := env.store.IpNetGroupStore.FindDefault()
	env.fw.failOn["9.9.9.9"] = errors.New("boom")

	success, failed, err := env.svc.ImportIpNet("8.8.4.4 9.9.9.9", group.ID, store.ActionBan)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if success != 1 || failed != 1 {
		t.Errorf("success=%d failed=%d, want 1/1", success, failed)
	}
}

func TestImportIpNetUpdatesExisting(t *testing.T) {
	env := newTestEnv(t)

	group, _ := env.store.IpNetGroupStore.FindDefault()
	if err := env.svc.CreateOrUpdateIpNet("7.7.7.7", 0, store.ActionBan); err != nil {
		t.Fatal(err)
	}

	success, failed, err := env.svc.ImportIpNet("7.7.7.7 6.6.6.6", group.ID, store.ActionAllow)
	if err != nil {
		t.Fatal(err)
	}
	if success != 2 || failed != 0 {
		t.Errorf("success=%d failed=%d, want 2/0", success, failed)
	}
}

func TestImportIpNetInvalidAction(t *testing.T) {
	env := newTestEnv(t)
	_, _, err := env.svc.ImportIpNet("8.8.8.8", 0, "deny")
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestDeleteGroupGuards(t *testing.T) {
	env := newTestEnv(t)

	def, _ := env.store.IpNetGroupStore.FindDefault()
	// 不允许删除默认组
	if err := env.svc.DeleteGroup(def.ID); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest for default group, got %v", err)
	}

	// 删除普通组后，组内IP迁移到默认组
	group, _ := env.store.IpNetGroupStore.Create("todelete", "")
	if err := env.svc.CreateOrUpdateIpNet("10.5.0.1", group.ID, store.ActionBan); err != nil {
		t.Fatal(err)
	}
	if err := env.svc.DeleteGroup(group.ID); err != nil {
		t.Fatalf("delete group: %v", err)
	}
	ipNet, err := env.store.IpNetStore.FindByIpNet("10.5.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if ipNet.GroupID != def.ID {
		t.Errorf("ip should be moved to default group, got group %d", ipNet.GroupID)
	}
}

func TestGetStatsIncludesBanStatus(t *testing.T) {
	env := newTestEnv(t)

	if err := env.svc.CreateOrUpdateIpNet("8.8.8.8", 0, store.ActionBan); err != nil {
		t.Fatal(err)
	}

	stats, err := env.svc.GetStats()
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("stats = %d, want 2", len(stats))
	}
	for _, s := range stats {
		if s.RemoteIP == "8.8.8.8" && !s.IsBanned {
			t.Error("8.8.8.8 should be marked banned")
		}
	}
}

func TestCreateGroupDuplicate(t *testing.T) {
	env := newTestEnv(t)

	if _, err := env.svc.CreateGroup("dup", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := env.svc.CreateGroup("dup", ""); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}
