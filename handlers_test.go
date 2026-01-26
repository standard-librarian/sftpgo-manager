package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestHandlers creates a Handlers with an in-memory DB and a mock SFTPGo client.
func newTestHandlers(t *testing.T, sftpgoServer *httptest.Server) *Handlers {
	t.Helper()
	db := newTestDB(t)
	sftpgoURL := "http://localhost:8080"
	if sftpgoServer != nil {
		sftpgoURL = sftpgoServer.URL
	}
	return &Handlers{
		db:     db,
		sftpgo: NewSFTPGoClient(sftpgoURL, "admin", "admin"),
		cfg:    Config{DataDir: "/tmp/test"},
	}
}

func TestCreateAPIKeyHandler(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/keys", strings.NewReader(`{"label":"test"}`))
	rec := httptest.NewRecorder()
	h.CreateAPIKey(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var key APIKey
	if err := json.NewDecoder(rec.Body).Decode(&key); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(key.Key) != 64 {
		t.Errorf("key length = %d, want 64", len(key.Key))
	}
	if key.Label != "test" {
		t.Errorf("label = %q, want %q", key.Label, "test")
	}
}

func TestCreateAPIKeyHandlerWrongMethod(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	rec := httptest.NewRecorder()
	h.CreateAPIKey(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestCreateAPIKeyHandlerNoBody(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/keys", nil)
	rec := httptest.NewRecorder()
	h.CreateAPIKey(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestListTenantsHandlerEmpty(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/tenants", nil)
	rec := httptest.NewRecorder()
	h.ListTenants(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var tenants []Tenant
	if err := json.NewDecoder(rec.Body).Decode(&tenants); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tenants) != 0 {
		t.Errorf("expected empty array, got %d tenants", len(tenants))
	}
}

func TestGetTenantHandlerNotFound(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/999", nil)
	rec := httptest.NewRecorder()
	h.GetTenant(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetTenantHandlerInvalidID(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/abc", nil)
	rec := httptest.NewRecorder()
	h.GetTenant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetTenantHandlerSuccess(t *testing.T) {
	h := newTestHandlers(t, nil)

	if _, err := h.db.CreateTenant("tid1", "testuser", "pass", "", "/data/tid1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/1", nil)
	rec := httptest.NewRecorder()
	h.GetTenant(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var tenant Tenant
	if err := json.NewDecoder(rec.Body).Decode(&tenant); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tenant.Username != "testuser" {
		t.Errorf("username = %q, want %q", tenant.Username, "testuser")
	}
}

func TestCreateTenantHandlerMissingUsername(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/tenants", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.CreateTenant(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestListTenantRecordsHandlerEmpty(t *testing.T) {
	h := newTestHandlers(t, nil)

	if _, err := h.db.CreateTenant("tid1", "testuser", "pass", "", "/data/tid1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/1/records", nil)
	rec := httptest.NewRecorder()
	h.ListTenantRecords(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var records []Record
	if err := json.NewDecoder(rec.Body).Decode(&records); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty array, got %d records", len(records))
	}
}

func TestListTenantRecordsHandlerWithData(t *testing.T) {
	h := newTestHandlers(t, nil)

	if _, err := h.db.CreateTenant("tid1", "testuser", "pass", "", "/data/tid1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if err := h.db.UpsertRecord("tid1", "R1", "Title1", "Desc", "Cat", 42.0); err != nil {
		t.Fatalf("UpsertRecord: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/1/records", nil)
	rec := httptest.NewRecorder()
	h.ListTenantRecords(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var records []Record
	if err := json.NewDecoder(rec.Body).Decode(&records); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Title != "Title1" {
		t.Errorf("title = %q, want %q", records[0].Title, "Title1")
	}
}

func TestListTenantRecordsHandlerTenantNotFound(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/tenants/999/records", nil)
	rec := httptest.NewRecorder()
	h.ListTenantRecords(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestUploadEventHookHandler(t *testing.T) {
	h := newTestHandlers(t, nil)

	body := `{"action":"upload","username":"test","virtual_path":"/data.txt"}`
	req := httptest.NewRequest(http.MethodPost, "/api/events/upload", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.UploadEventHook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestUploadEventHookHandlerInvalidJSON(t *testing.T) {
	h := newTestHandlers(t, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/events/upload", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	h.UploadEventHook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestExternalAuthHookHandlerTenantNotFound(t *testing.T) {
	h := newTestHandlers(t, nil)

	body := `{"username":"nobody","password":"pass","protocol":"SSH","ip":"127.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/hook", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ExternalAuthHook(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestExternalAuthHookHandlerPasswordAuth(t *testing.T) {
	h := newTestHandlers(t, nil)

	if _, err := h.db.CreateTenant("tid1", "testuser", "secret123", "", "/data/tid1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	body := `{"username":"testuser","password":"secret123","protocol":"SSH","ip":"127.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/hook", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ExternalAuthHook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["username"] != "testuser" {
		t.Errorf("username = %v, want %q", resp["username"], "testuser")
	}
	if resp["status"] != float64(1) {
		t.Errorf("status = %v, want 1", resp["status"])
	}
}

func TestExternalAuthHookHandlerWrongPassword(t *testing.T) {
	h := newTestHandlers(t, nil)

	if _, err := h.db.CreateTenant("tid1", "testuser", "secret123", "", "/data/tid1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	body := `{"username":"testuser","password":"wrong","protocol":"SSH","ip":"127.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/hook", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ExternalAuthHook(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestExternalAuthHookHandlerPublicKeyAuth(t *testing.T) {
	h := newTestHandlers(t, nil)

	pubKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey user@host"
	if _, err := h.db.CreateTenant("tid1", "testuser", "", pubKey, "/data/tid1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	body := `{"username":"testuser","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKey","protocol":"SSH","ip":"127.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/hook", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ExternalAuthHook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		path    string
		prefix  string
		want    int64
		wantErr bool
	}{
		{"/api/tenants/1", "/api/tenants/", 1, false},
		{"/api/tenants/42", "/api/tenants/", 42, false},
		{"/api/tenants/abc", "/api/tenants/", 0, true},
		{"/api/tenants/1/records", "/api/tenants/", 1, false},
	}
	for _, tt := range tests {
		got, err := parseID(tt.path, tt.prefix)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseID(%q, %q) error = %v, wantErr %v", tt.path, tt.prefix, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseID(%q, %q) = %d, want %d", tt.path, tt.prefix, got, tt.want)
		}
	}
}
