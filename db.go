package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a sql.DB connection and provides domain-specific queries.
type DB struct {
	conn *sql.DB
}

// APIKey represents an authentication token for the management API.
type APIKey struct {
	ID        int64     `json:"id"`
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
}

// Tenant represents an isolated SFTP account with its own S3 prefix and credentials.
type Tenant struct {
	ID        int64     `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	PublicKey string    `json:"public_key,omitempty"`
	HomeDir   string    `json:"home_dir"`
	CreatedAt time.Time `json:"created_at"`
}

// Record represents a data entry parsed from a CSV upload, keyed by (tenant_id, record_key).
type Record struct {
	ID          int64     `json:"id"`
	TenantID    string    `json:"tenant_id"`
	RecordKey   string    `json:"record_key"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Value       float64   `json:"value"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewDB opens a SQLite database at path and runs migrations.
func NewDB(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id INTEGER PRIMARY KEY,
			key TEXT UNIQUE NOT NULL,
			label TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS tenants (
			id INTEGER PRIMARY KEY,
			tenant_id TEXT UNIQUE NOT NULL,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL DEFAULT '',
			public_key TEXT NOT NULL DEFAULT '',
			home_dir TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS records (
			id INTEGER PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			record_key TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			category TEXT NOT NULL DEFAULT '',
			value REAL NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(tenant_id, record_key)
		);
	`); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{conn: conn}, nil
}

// CreateAPIKey generates and stores a new random 64-char hex API key.
func (db *DB) CreateAPIKey(label string) (*APIKey, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	key := hex.EncodeToString(b)
	res, err := db.conn.Exec("INSERT INTO api_keys (key, label) VALUES (?, ?)", key, label)
	if err != nil {
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return &APIKey{ID: id, Key: key, Label: label, CreatedAt: time.Now()}, nil
}

// ValidateAPIKey reports whether the given key exists in the database.
func (db *DB) ValidateAPIKey(key string) error {
	var id int64
	err := db.conn.QueryRow("SELECT id FROM api_keys WHERE key = ?", key).Scan(&id)
	if err != nil {
		return fmt.Errorf("invalid api key: %w", err)
	}
	return nil
}

// CreateTenant inserts a new tenant and returns it.
func (db *DB) CreateTenant(tenantID, username, password, publicKey, homeDir string) (*Tenant, error) {
	res, err := db.conn.Exec(
		"INSERT INTO tenants (tenant_id, username, password, public_key, home_dir) VALUES (?, ?, ?, ?, ?)",
		tenantID, username, password, publicKey, homeDir,
	)
	if err != nil {
		return nil, fmt.Errorf("insert tenant: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return &Tenant{
		ID: id, TenantID: tenantID, Username: username,
		Password: password, PublicKey: publicKey, HomeDir: homeDir,
		CreatedAt: time.Now(),
	}, nil
}

// ListTenants returns all tenants ordered by ID.
func (db *DB) ListTenants() ([]Tenant, error) {
	rows, err := db.conn.Query("SELECT id, tenant_id, username, password, public_key, home_dir, created_at FROM tenants ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tenants []Tenant
	for rows.Next() {
		var t Tenant
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Username, &t.Password, &t.PublicKey, &t.HomeDir, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}
	return tenants, rows.Err()
}

// GetTenant retrieves a single tenant by database ID.
func (db *DB) GetTenant(id int64) (*Tenant, error) {
	var t Tenant
	err := db.conn.QueryRow(
		"SELECT id, tenant_id, username, password, public_key, home_dir, created_at FROM tenants WHERE id = ?", id,
	).Scan(&t.ID, &t.TenantID, &t.Username, &t.Password, &t.PublicKey, &t.HomeDir, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant %d: %w", id, err)
	}
	return &t, nil
}

// GetTenantByUsername retrieves a tenant by their SFTP username.
func (db *DB) GetTenantByUsername(username string) (*Tenant, error) {
	var t Tenant
	err := db.conn.QueryRow(
		"SELECT id, tenant_id, username, password, public_key, home_dir, created_at FROM tenants WHERE username = ?", username,
	).Scan(&t.ID, &t.TenantID, &t.Username, &t.Password, &t.PublicKey, &t.HomeDir, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant by username %q: %w", username, err)
	}
	return &t, nil
}

// UpdateTenantPublicKey sets a new SSH public key for the given tenant.
func (db *DB) UpdateTenantPublicKey(id int64, publicKey string) error {
	_, err := db.conn.Exec("UPDATE tenants SET public_key = ? WHERE id = ?", publicKey, id)
	if err != nil {
		return fmt.Errorf("update public key for tenant %d: %w", id, err)
	}
	return nil
}

// DeleteTenant removes a tenant by ID and returns their SFTP username.
func (db *DB) DeleteTenant(id int64) (string, error) {
	var username string
	if err := db.conn.QueryRow("SELECT username FROM tenants WHERE id = ?", id).Scan(&username); err != nil {
		return "", fmt.Errorf("find tenant %d: %w", id, err)
	}
	if _, err := db.conn.Exec("DELETE FROM tenants WHERE id = ?", id); err != nil {
		return "", fmt.Errorf("delete tenant %d: %w", id, err)
	}
	return username, nil
}

// UpsertRecord inserts or updates a record identified by (tenantID, recordKey).
func (db *DB) UpsertRecord(tenantID, recordKey, title, description, category string, value float64) error {
	_, err := db.conn.Exec(`
		INSERT INTO records (tenant_id, record_key, title, description, category, value, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, record_key) DO UPDATE SET
			title=excluded.title,
			description=excluded.description,
			category=excluded.category,
			value=excluded.value,
			updated_at=CURRENT_TIMESTAMP`,
		tenantID, recordKey, title, description, category, value,
	)
	if err != nil {
		return fmt.Errorf("upsert record: %w", err)
	}
	return nil
}

// ListRecords returns all records for the given tenant_id, ordered by ID.
func (db *DB) ListRecords(tenantID string) ([]Record, error) {
	rows, err := db.conn.Query(
		"SELECT id, tenant_id, record_key, title, description, category, value, updated_at FROM records WHERE tenant_id = ? ORDER BY id", tenantID,
	)
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var records []Record
	for rows.Next() {
		var r Record
		if err := rows.Scan(&r.ID, &r.TenantID, &r.RecordKey, &r.Title, &r.Description, &r.Category, &r.Value, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan record: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}
