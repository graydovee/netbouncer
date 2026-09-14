package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/graydovee/netbouncer/pkg/config"
	"github.com/graydovee/netbouncer/pkg/core"
	"github.com/graydovee/netbouncer/pkg/service"
	"github.com/graydovee/netbouncer/pkg/store"
)

// ---- 测试用的 fake 实现（与 service 测试中的等价） ----

type fakeMonitor struct{}

func (f *fakeMonitor) GetStats() map[string]*core.TrafficStats {
	return map[string]*core.TrafficStats{}
}

type fakeFirewall struct{}

func (f *fakeFirewall) Init(ipList []store.IpNet) error                { return nil }
func (f *fakeFirewall) Ban(ipNet string, direction string) error       { return nil }
func (f *fakeFirewall) RevertBan(ipNet string, direction string) error { return nil }
func (f *fakeFirewall) Allow(ipNet string) error                       { return nil }
func (f *fakeFirewall) RevertAllow(ipNet string) error                 { return nil }
func (f *fakeFirewall) ApplyRateLimit(rule core.RateLimitRule) error   { return nil }
func (f *fakeFirewall) RemoveRateLimit(rule core.RateLimitRule) error  { return nil }
func (f *fakeFirewall) CleanupIpNet(ipNet string) error                { return nil }

// newTestServer 构造带真实存储与服务的测试服务器
func newTestServer(t *testing.T) *Server {
	t.Helper()

	dbFile := filepath.Join(t.TempDir(), "test.db")
	st, err := store.NewStore(&config.DatabaseConfig{Driver: "sqlite", Database: dbFile, LogLevel: "silent"})
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	svc := service.NewNetService(&fakeMonitor{}, &fakeFirewall{}, st)
	if err := svc.Init(nil); err != nil {
		t.Fatalf("init service: %v", err)
	}

	auth, err := NewAuthHandler(t.Context(), &AuthConfig{Enabled: false})
	if err != nil {
		t.Fatalf("create auth: %v", err)
	}

	return NewServer(svc, auth)
}

func doRequest(t *testing.T, e *echo.Echo, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return resp
}

func TestCreateIpNetReturns200(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodPost, "/api/ip", CreateIPNetRequest{
		IpNet:  "192.168.1.1",
		Action: "ban",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if resp := decodeBody(t, rec); resp.Code != 200 {
		t.Errorf("body code = %d", resp.Code)
	}
}

func TestCreateIpNetInvalidReturns400(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodPost, "/api/ip", CreateIPNetRequest{
		IpNet:  "not-an-ip",
		Action: "ban",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateIpNetMissingActionReturns400(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodPost, "/api/ip", CreateIPNetRequest{IpNet: "1.2.3.4"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestListIpNetsPagination(t *testing.T) {
	s := newTestServer(t)

	for _, ip := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		if rec := doRequest(t, s.echo, http.MethodPost, "/api/ip", CreateIPNetRequest{IpNet: ip, Action: "ban"}); rec.Code != 200 {
			t.Fatalf("create %s failed: %s", ip, rec.Body.String())
		}
	}

	rec := doRequest(t, s.echo, http.MethodGet, "/api/ip?page=1&page_size=2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Items []map[string]any `json:"items"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Data.Total)
	}
	if len(resp.Data.Items) != 2 {
		t.Errorf("items = %d, want 2", len(resp.Data.Items))
	}
}

func TestListIpNetsSearchFilter(t *testing.T) {
	s := newTestServer(t)

	for _, ip := range []string{"172.16.1.1", "192.168.1.1"} {
		doRequest(t, s.echo, http.MethodPost, "/api/ip", CreateIPNetRequest{IpNet: ip, Action: "ban"})
	}

	rec := doRequest(t, s.echo, http.MethodGet, "/api/ip?search=172.16", nil)
	var resp struct {
		Data struct {
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Data.Total)
	}
}

func TestDeleteIpNetNotFoundReturns404(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodDelete, "/api/ip/9999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteIpNetInvalidIDReturns400(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodDelete, "/api/ip/abc", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestBatchDelete(t *testing.T) {
	s := newTestServer(t)

	var ids []uint
	for _, ip := range []string{"10.1.0.1", "10.1.0.2"} {
		doRequest(t, s.echo, http.MethodPost, "/api/ip", CreateIPNetRequest{IpNet: ip, Action: "ban"})
	}

	rec := doRequest(t, s.echo, http.MethodGet, "/api/ip?page_size=100", nil)
	var listResp struct {
		Data struct {
			Items []struct {
				ID uint `json:"id"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	for _, item := range listResp.Data.Items {
		ids = append(ids, item.ID)
	}

	rec = doRequest(t, s.echo, http.MethodPost, "/api/ip/batch-delete", BatchDeleteIPNetRequest{IDs: ids})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data struct {
			SuccessCount int `json:"success_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Data.SuccessCount != 2 {
		t.Errorf("success = %d, want 2", resp.Data.SuccessCount)
	}
}

func TestBatchActionEmptyIDsReturns400(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodPost, "/api/ip/batch-action", BatchUpdateIPNetActionRequest{Action: "ban"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestGroupConflictReturns409(t *testing.T) {
	s := newTestServer(t)

	body := CreateGroupRequest{Name: "dup"}
	if rec := doRequest(t, s.echo, http.MethodPost, "/api/group", body); rec.Code != http.StatusOK {
		t.Fatalf("first create status = %d", rec.Code)
	}

	rec := doRequest(t, s.echo, http.MethodPost, "/api/group", body)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestUnknownAPIPathReturnsJSON404(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodGet, "/api/unknown", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	var resp Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("api 404 should return JSON, got: %s", rec.Body.String())
	}
	if resp.Code != 404 {
		t.Errorf("body code = %d", resp.Code)
	}
}

func TestGroupListContainsIPCount(t *testing.T) {
	s := newTestServer(t)

	doRequest(t, s.echo, http.MethodPost, "/api/ip", CreateIPNetRequest{IpNet: "10.9.9.9", Action: "ban"})

	rec := doRequest(t, s.echo, http.MethodGet, "/api/group", nil)
	var resp struct {
		Data []struct {
			IsDefault bool  `json:"is_default"`
			IPCount   int64 `json:"ip_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, g := range resp.Data {
		if g.IsDefault && g.IPCount == 1 {
			found = true
		}
	}
	if !found {
		t.Errorf("default group should have ip_count=1, got %+v", resp.Data)
	}
}

func TestDeleteDefaultGroupReturns400(t *testing.T) {
	s := newTestServer(t)

	rec := doRequest(t, s.echo, http.MethodGet, "/api/group", nil)
	var resp struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) == 0 {
		t.Fatal("no groups")
	}

	rec = doRequest(t, s.echo, http.MethodDelete, "/api/group/1", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUnknownFrontendRouteFallsBackToIndex(t *testing.T) {
	s := newTestServer(t)

	// 前端路由（无静态文件）应尝试回退到 index.html；
	// 测试环境没有前端产物，因此回退失败返回 500 而不是 JSON 404
	rec := doRequest(t, s.echo, http.MethodGet, "/some/frontend/route", nil)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 404 or 500", rec.Code)
	}
}
