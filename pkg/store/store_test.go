package store

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"

	"github.com/graydovee/netbouncer/pkg/config"
)

// newTestStore 创建基于临时sqlite文件的存储
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbFile := filepath.Join(t.TempDir(), "test.db")
	st, err := NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: dbFile, LogLevel: "silent"})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	return st
}

func TestIpNetCRUD(t *testing.T) {
	st := newTestStore(t)

	group, err := st.IpNetGroupStore.Create("test-group", "desc")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	created, err := st.IpNetStore.Create("192.168.1.1", group.ID, ActionBan)
	if err != nil {
		t.Fatalf("create ipnet: %v", err)
	}

	found, err := st.IpNetStore.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.IpNet != "192.168.1.1" || found.Action != ActionBan {
		t.Errorf("unexpected record: %+v", found)
	}

	if !st.IpNetStore.ExistsByIpNet("192.168.1.1") {
		t.Error("ExistsByIpNet should be true")
	}

	if err := st.IpNetStore.UpdateAction(created.ID, ActionAllow); err != nil {
		t.Fatalf("update action: %v", err)
	}
	found, _ = st.IpNetStore.FindByID(created.ID)
	if found.Action != ActionAllow {
		t.Errorf("action not updated: %s", found.Action)
	}

	if err := st.IpNetStore.DeleteByID(created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.IpNetStore.FindByID(created.ID); err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestIpNetFindByFilter(t *testing.T) {
	st := newTestStore(t)

	g1, _ := st.IpNetGroupStore.Create("g1", "")
	g2, _ := st.IpNetGroupStore.Create("g2", "")

	for i, ip := range []string{"10.0.0.1", "10.0.0.2", "172.16.0.1", "192.168.1.1"} {
		groupID := g1.ID
		if i%2 == 1 {
			groupID = g2.ID
		}
		action := ActionBan
		if i == 3 {
			action = ActionAllow
		}
		if _, err := st.IpNetStore.Create(ip, groupID, action); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	// 分页
	items, total, err := st.IpNetStore.FindByFilter(IpNetFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if len(items) != 2 {
		t.Errorf("page size = %d, want 2", len(items))
	}

	// 组过滤
	items, total, _ = st.IpNetStore.FindByFilter(IpNetFilter{GroupID: g1.ID})
	if total != 2 {
		t.Errorf("group filter total = %d, want 2", total)
	}
	_ = items

	// 动作过滤
	_, total, _ = st.IpNetStore.FindByFilter(IpNetFilter{Action: ActionAllow})
	if total != 1 {
		t.Errorf("action filter total = %d, want 1", total)
	}

	// 搜索
	_, total, _ = st.IpNetStore.FindByFilter(IpNetFilter{Search: "10.0.0"})
	if total != 2 {
		t.Errorf("search total = %d, want 2", total)
	}
}

func TestIpNetBatchCreateAndFindByIpNets(t *testing.T) {
	st := newTestStore(t)

	group, _ := st.IpNetGroupStore.Create("g", "")
	ips := []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}

	created, err := st.IpNetStore.BatchCreate(ips, group.ID, ActionBan)
	if err != nil {
		t.Fatalf("batch create: %v", err)
	}
	if len(created) != 3 {
		t.Fatalf("created = %d, want 3", len(created))
	}

	found, err := st.IpNetStore.FindByIpNets([]string{"10.0.0.1", "10.0.0.3", "10.0.0.99"})
	if err != nil {
		t.Fatalf("find by ipnets: %v", err)
	}
	if len(found) != 2 {
		t.Errorf("found = %d, want 2", len(found))
	}
}

func TestDeleteByIDs(t *testing.T) {
	st := newTestStore(t)
	group, _ := st.IpNetGroupStore.Create("g", "")

	var ids []uint
	for _, ip := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		item, err := st.IpNetStore.Create(ip, group.ID, ActionBan)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, item.ID)
	}

	deleted, err := st.IpNetStore.DeleteByIDs(ids[:2])
	if err != nil {
		t.Fatalf("delete by ids: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted = %d, want 2", deleted)
	}
}

func TestGroupStore(t *testing.T) {
	st := newTestStore(t)

	def, err := st.IpNetGroupStore.Create(DefaultGroupName(), "系统默认组")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.IpNetGroupStore.SetDefault(def.ID); err != nil {
		t.Fatalf("set default: %v", err)
	}

	// 重复创建同名校验由数据库唯一索引保证
	other, err := st.IpNetGroupStore.Create("other", "")
	if err != nil {
		t.Fatal(err)
	}

	// Update 不应覆盖 CreatedAt
	original, _ := st.IpNetGroupStore.FindByID(other.ID)
	updated, err := st.IpNetGroupStore.Update(other.ID, "other-renamed", "new desc")
	if err != nil {
		t.Fatalf("update group: %v", err)
	}
	if !updated.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("created_at changed on update: %v -> %v", original.CreatedAt, updated.CreatedAt)
	}
	if updated.Name != "other-renamed" {
		t.Errorf("name = %s", updated.Name)
	}

	// 组内计数
	for _, ip := range []string{"10.0.0.1", "10.0.0.2"} {
		if _, err := st.IpNetStore.Create(ip, def.ID, ActionBan); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := st.IpNetGroupStore.CountByGroupID()
	if err != nil {
		t.Fatalf("count by group: %v", err)
	}
	if counts[def.ID] != 2 {
		t.Errorf("count[default] = %d, want 2", counts[def.ID])
	}
}

func TestFindDefaultNotFound(t *testing.T) {
	st := newTestStore(t)
	if _, err := st.IpNetGroupStore.FindDefault(); err != gorm.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v", err)
	}
}

// DefaultGroupName 测试用默认组名（与 service.DefaultGroupName 保持一致）
func DefaultGroupName() string {
	return "default"
}
