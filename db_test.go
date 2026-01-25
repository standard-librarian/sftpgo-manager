package main

import (
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := NewDB(":memory:")
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateAndValidateAPIKey(t *testing.T) {
	db := newTestDB(t)

	key, err := db.CreateAPIKey("test-label")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if key.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if len(key.Key) != 64 {
		t.Errorf("expected 64-char key, got %d", len(key.Key))
	}
	if key.Label != "test-label" {
		t.Errorf("label = %q, want %q", key.Label, "test-label")
	}

	if err := db.ValidateAPIKey(key.Key); err != nil {
		t.Errorf("ValidateAPIKey should succeed for valid key: %v", err)
	}
	if err := db.ValidateAPIKey("nonexistent"); err == nil {
		t.Error("ValidateAPIKey should fail for invalid key")
	}
}

func TestCreateAPIKeyEmptyLabel(t *testing.T) {
	db := newTestDB(t)

	key, err := db.CreateAPIKey("")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	if key.Label != "" {
		t.Errorf("label = %q, want empty", key.Label)
	}
}

func TestCreateTenant(t *testing.T) {
	db := newTestDB(t)

	tenant, err := db.CreateTenant("tid123", "testuser", "pass123", "ssh-ed25519 AAAA", "/data/tid123")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if tenant.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if tenant.TenantID != "tid123" {
		t.Errorf("TenantID = %q, want %q", tenant.TenantID, "tid123")
	}
	if tenant.Username != "testuser" {
		t.Errorf("Username = %q, want %q", tenant.Username, "testuser")
	}
}

func TestCreateTenantDuplicateTenantID(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.CreateTenant("tid123", "user1", "p1", "", "/d/1"); err != nil {
		t.Fatalf("first CreateTenant: %v", err)
	}
	if _, err := db.CreateTenant("tid123", "user2", "p2", "", "/d/2"); err == nil {
		t.Error("expected error for duplicate tenant_id")
	}
}

func TestCreateTenantDuplicateUsername(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.CreateTenant("tid1", "sameuser", "p1", "", "/d/1"); err != nil {
		t.Fatalf("first CreateTenant: %v", err)
	}
	if _, err := db.CreateTenant("tid2", "sameuser", "p2", "", "/d/2"); err == nil {
		t.Error("expected error for duplicate username")
	}
}

func TestGetTenant(t *testing.T) {
	db := newTestDB(t)

	created, err := db.CreateTenant("tid123", "testuser", "pass", "", "/data/tid123")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	got, err := db.GetTenant(created.ID)
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if got.TenantID != "tid123" {
		t.Errorf("TenantID = %q, want %q", got.TenantID, "tid123")
	}
}

func TestGetTenantNotFound(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.GetTenant(999); err == nil {
		t.Error("expected error for non-existent tenant")
	}
}

func TestGetTenantByUsername(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.CreateTenant("tid123", "testuser", "pass", "", "/data/tid123"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	got, err := db.GetTenantByUsername("testuser")
	if err != nil {
		t.Fatalf("GetTenantByUsername: %v", err)
	}
	if got.TenantID != "tid123" {
		t.Errorf("TenantID = %q, want %q", got.TenantID, "tid123")
	}
}

func TestGetTenantByUsernameNotFound(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.GetTenantByUsername("nonexistent"); err == nil {
		t.Error("expected error for non-existent username")
	}
}

func TestListTenants(t *testing.T) {
	db := newTestDB(t)

	tenants, err := db.ListTenants()
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if len(tenants) != 0 {
		t.Errorf("expected 0 tenants, got %d", len(tenants))
	}

	if _, err := db.CreateTenant("tid1", "user1", "p1", "", "/d/1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := db.CreateTenant("tid2", "user2", "p2", "", "/d/2"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	tenants, err = db.ListTenants()
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if len(tenants) != 2 {
		t.Errorf("expected 2 tenants, got %d", len(tenants))
	}
}

func TestUpdateTenantPublicKey(t *testing.T) {
	db := newTestDB(t)

	tenant, err := db.CreateTenant("tid123", "testuser", "pass", "", "/data/tid123")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	if err := db.UpdateTenantPublicKey(tenant.ID, "ssh-ed25519 NEWKEY"); err != nil {
		t.Fatalf("UpdateTenantPublicKey: %v", err)
	}

	got, err := db.GetTenant(tenant.ID)
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if got.PublicKey != "ssh-ed25519 NEWKEY" {
		t.Errorf("PublicKey = %q, want %q", got.PublicKey, "ssh-ed25519 NEWKEY")
	}
}

func TestDeleteTenant(t *testing.T) {
	db := newTestDB(t)

	tenant, err := db.CreateTenant("tid123", "testuser", "pass", "", "/data/tid123")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	username, err := db.DeleteTenant(tenant.ID)
	if err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}
	if username != "testuser" {
		t.Errorf("username = %q, want %q", username, "testuser")
	}

	if _, err := db.GetTenant(tenant.ID); err == nil {
		t.Error("expected error after deleting tenant")
	}
}

func TestDeleteTenantNotFound(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.DeleteTenant(999); err == nil {
		t.Error("expected error for non-existent tenant")
	}
}

func TestUpsertAndListRecords(t *testing.T) {
	db := newTestDB(t)

	if err := db.UpsertRecord("tid1", "REC-001", "First Record", "A description", "cat-a", 10.5); err != nil {
		t.Fatalf("UpsertRecord: %v", err)
	}
	if err := db.UpsertRecord("tid1", "REC-002", "Second Record", "Another desc", "cat-b", 20.0); err != nil {
		t.Fatalf("UpsertRecord: %v", err)
	}

	records, err := db.ListRecords("tid1")
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].RecordKey != "REC-001" {
		t.Errorf("first record key = %q, want %q", records[0].RecordKey, "REC-001")
	}
	if records[0].Title != "First Record" {
		t.Errorf("first record title = %q, want %q", records[0].Title, "First Record")
	}
	if records[0].Value != 10.5 {
		t.Errorf("first record value = %f, want %f", records[0].Value, 10.5)
	}
}

func TestUpsertRecordUpdate(t *testing.T) {
	db := newTestDB(t)

	if err := db.UpsertRecord("tid1", "REC-001", "Original", "desc", "cat", 10.0); err != nil {
		t.Fatalf("UpsertRecord: %v", err)
	}
	if err := db.UpsertRecord("tid1", "REC-001", "Updated", "new desc", "new-cat", 99.9); err != nil {
		t.Fatalf("UpsertRecord (update): %v", err)
	}

	records, err := db.ListRecords("tid1")
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after upsert, got %d", len(records))
	}
	if records[0].Title != "Updated" {
		t.Errorf("title = %q, want %q", records[0].Title, "Updated")
	}
	if records[0].Description != "new desc" {
		t.Errorf("description = %q, want %q", records[0].Description, "new desc")
	}
	if records[0].Value != 99.9 {
		t.Errorf("value = %f, want %f", records[0].Value, 99.9)
	}
}

func TestListRecordsIsolation(t *testing.T) {
	db := newTestDB(t)

	if err := db.UpsertRecord("tid1", "R1", "Title1", "", "", 1.0); err != nil {
		t.Fatalf("UpsertRecord: %v", err)
	}
	if err := db.UpsertRecord("tid2", "R2", "Title2", "", "", 2.0); err != nil {
		t.Fatalf("UpsertRecord: %v", err)
	}

	records, err := db.ListRecords("tid1")
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record for tid1, got %d", len(records))
	}
}

func TestListRecordsEmpty(t *testing.T) {
	db := newTestDB(t)

	records, err := db.ListRecords("nonexistent")
	if err != nil {
		t.Fatalf("ListRecords: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil slice for empty results, got %v", records)
	}
}
