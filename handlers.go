package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

// Handlers groups HTTP handler methods and their dependencies.
type Handlers struct {
	db     *DB
	sftpgo *SFTPGoClient
	cfg    Config
	worker *Worker
}

// CreateAPIKey godoc
// @Summary Bootstrap a new API key
// @Description Creates a new API key for authenticating subsequent requests. No auth required.
// @Tags keys
// @Accept json
// @Produce json
// @Param body body object{label=string} false "Optional label"
// @Success 201 {object} APIKey
// @Failure 500 {object} object{error=string}
// @Router /keys [post]
func (h *Handlers) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	key, err := h.db.CreateAPIKey(req.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

// CreateTenant godoc
// @Summary Create a new tenant
// @Description Creates a new SFTP tenant with an auto-generated tenant_id. The tenant_id becomes the S3 key prefix for file isolation. A password is auto-generated if not provided.
// @Tags tenants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body object{username=string,password=string,public_key=string} true "Tenant details (only username required)"
// @Success 201 {object} object{tenant=Tenant,password=string,tenant_id=string}
// @Failure 400 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Router /tenants [post]
func (h *Handlers) CreateTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		http.Error(w, `{"error":"username is required"}`, http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("generate password: %w", err))
			return
		}
		req.Password = hex.EncodeToString(b)
	}

	tidBytes := make([]byte, 16)
	if _, err := rand.Read(tidBytes); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("generate tenant_id: %w", err))
		return
	}
	tenantID := hex.EncodeToString(tidBytes)

	homeDir := filepath.Join(h.cfg.DataDir, tenantID)

	var pubKeys []string
	if req.PublicKey != "" {
		pubKeys = []string{req.PublicKey}
	}

	var s3Cfg *S3Config
	if h.cfg.S3Endpoint != "" {
		s3Cfg = &S3Config{
			Bucket:    h.cfg.S3Bucket,
			Region:    h.cfg.S3Region,
			Endpoint:  h.cfg.S3Endpoint,
			AccessKey: h.cfg.S3AccessKey,
			SecretKey: h.cfg.S3SecretKey,
		}
	}

	if err := h.sftpgo.CreateUser(req.Username, req.Password, homeDir, pubKeys, s3Cfg, tenantID); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	tenant, err := h.db.CreateTenant(tenantID, req.Username, req.Password, req.PublicKey, homeDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"tenant":    tenant,
		"password":  req.Password,
		"tenant_id": tenantID,
	})
}

// ListTenants godoc
// @Summary List all tenants
// @Description Returns all registered tenants.
// @Tags tenants
// @Produce json
// @Security BearerAuth
// @Success 200 {array} Tenant
// @Router /tenants [get]
func (h *Handlers) ListTenants(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	tenants, err := h.db.ListTenants()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if tenants == nil {
		tenants = []Tenant{}
	}
	writeJSON(w, http.StatusOK, tenants)
}

// GetTenant godoc
// @Summary Get tenant by ID
// @Description Returns a single tenant by database ID.
// @Tags tenants
// @Produce json
// @Security BearerAuth
// @Param id path int true "Tenant ID"
// @Success 200 {object} Tenant
// @Failure 404 {object} object{error=string}
// @Router /tenants/{id} [get]
func (h *Handlers) GetTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	id, err := parseID(r.URL.Path, "/api/tenants/")
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	tenant, err := h.db.GetTenant(id)
	if err != nil {
		http.Error(w, `{"error":"tenant not found"}`, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

// DeleteTenant godoc
// @Summary Delete a tenant
// @Description Removes a tenant from both the local DB and SFTPGo.
// @Tags tenants
// @Produce json
// @Security BearerAuth
// @Param id path int true "Tenant ID"
// @Success 200 {object} object{status=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{id} [delete]
func (h *Handlers) DeleteTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	id, err := parseID(r.URL.Path, "/api/tenants/")
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	username, err := h.db.DeleteTenant(id)
	if err != nil {
		http.Error(w, `{"error":"tenant not found"}`, http.StatusNotFound)
		return
	}
	if err := h.sftpgo.DeleteUser(username); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("deleted from db but sftpgo failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ValidateTenant godoc
// @Summary Validate tenant in SFTPGo
// @Description Checks whether a tenant's SFTP account is active and valid in SFTPGo.
// @Tags tenants
// @Produce json
// @Security BearerAuth
// @Param id path int true "Tenant ID"
// @Success 200 {object} object{valid=bool,username=string}
// @Router /tenants/{id}/validate [post]
func (h *Handlers) ValidateTenant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/validate")
	id, err := parseID(path, "/api/tenants/")
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	tenant, err := h.db.GetTenant(id)
	if err != nil {
		http.Error(w, `{"error":"tenant not found"}`, http.StatusNotFound)
		return
	}
	sftpgoUser, err := h.sftpgo.GetUser(tenant.Username)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"valid": false, "reason": err.Error()})
		return
	}
	status, _ := sftpgoUser["status"].(float64)
	writeJSON(w, http.StatusOK, map[string]any{"valid": status == 1, "username": tenant.Username})
}

// ExternalAuthHook godoc
// @Summary SFTPGo external auth hook
// @Description Called by SFTPGo to authenticate SFTP users. Not for direct use.
// @Tags hooks
// @Accept json
// @Produce json
// @Param body body object{username=string,password=string,public_key=string,protocol=string,ip=string} true "Auth request from SFTPGo"
// @Success 200 {object} object "SFTPGo user JSON"
// @Failure 403 {string} string "Authentication failed"
// @Router /auth/hook [post]
func (h *Handlers) ExternalAuthHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username  string `json:"username"`
		Password  string `json:"password"`
		PublicKey string `json:"public_key"`
		Protocol  string `json:"protocol"`
		IP        string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"username":""}`, http.StatusOK)
		return
	}
	log.Printf("auth hook: user=%s proto=%s ip=%s has_password=%v has_pubkey=%v",
		req.Username, req.Protocol, req.IP, req.Password != "", req.PublicKey != "")

	tenant, err := h.db.GetTenantByUsername(req.Username)
	if err != nil {
		log.Printf("auth hook: tenant %s not found in db", req.Username)
		http.Error(w, "", http.StatusForbidden)
		return
	}

	authenticated := false

	if req.PublicKey != "" && tenant.PublicKey != "" {
		reqParts := strings.Fields(strings.TrimSpace(req.PublicKey))
		storedParts := strings.Fields(strings.TrimSpace(tenant.PublicKey))
		if len(reqParts) >= 2 && len(storedParts) >= 2 && reqParts[1] == storedParts[1] {
			authenticated = true
		}
	}

	if !authenticated && req.Password != "" && tenant.Password != "" {
		if req.Password == tenant.Password {
			authenticated = true
		}
	}

	if !authenticated {
		log.Printf("auth hook: authentication failed for %s", req.Username)
		http.Error(w, "", http.StatusForbidden)
		return
	}

	sftpgoUser := map[string]any{
		"status":      1,
		"username":    tenant.Username,
		"home_dir":    tenant.HomeDir,
		"permissions": map[string][]string{"/": {"*"}},
	}
	if tenant.Password != "" {
		sftpgoUser["password"] = tenant.Password
	}
	if tenant.PublicKey != "" {
		sftpgoUser["public_keys"] = []string{tenant.PublicKey}
	}
	if h.cfg.S3Endpoint != "" {
		sftpgoUser["filesystem"] = map[string]any{
			"provider": 1,
			"s3config": map[string]any{
				"bucket":           h.cfg.S3Bucket,
				"region":           h.cfg.S3Region,
				"endpoint":         h.cfg.S3Endpoint,
				"access_key":       h.cfg.S3AccessKey,
				"access_secret":    map[string]string{"status": "Plain", "payload": h.cfg.S3SecretKey},
				"key_prefix":       tenant.TenantID + "/",
				"force_path_style": true,
				"skip_tls_verify":  true,
			},
		}
	}

	log.Printf("auth hook: tenant %s authenticated via %s", req.Username, req.Protocol)
	writeJSON(w, http.StatusOK, sftpgoUser)
}

// UpdateTenantKeys godoc
// @Summary Update tenant's SSH public key
// @Description Replaces the SSH public key for a tenant in both the local DB and SFTPGo.
// @Tags tenants
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Tenant ID"
// @Param body body object{public_key=string} true "New public key"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{id}/keys [put]
func (h *Handlers) UpdateTenantKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/keys")
	id, err := parseID(path, "/api/tenants/")
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PublicKey == "" {
		http.Error(w, `{"error":"public_key is required"}`, http.StatusBadRequest)
		return
	}
	tenant, err := h.db.GetTenant(id)
	if err != nil {
		http.Error(w, `{"error":"tenant not found"}`, http.StatusNotFound)
		return
	}
	if err := h.db.UpdateTenantPublicKey(id, req.PublicKey); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := h.sftpgo.UpdateUserPublicKeys(tenant.Username, []string{req.PublicKey}); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Errorf("db updated but sftpgo failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// UploadEventHook godoc
// @Summary SFTPGo upload event hook
// @Description Called by SFTPGo after a file upload. If the file is a .csv, it is asynchronously downloaded from S3 and parsed into the records table.
// @Tags hooks
// @Accept json
// @Produce json
// @Param body body object true "SFTPGo event payload"
// @Success 200 {object} object{status=string}
// @Router /events/upload [post]
func (h *Handlers) UploadEventHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var event map[string]any
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	log.Printf("upload event: %+v", event)
	if h.worker != nil {
		go h.worker.ProcessUploadEvent(event)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListTenantRecords godoc
// @Summary List records for a tenant
// @Description Returns all records ingested from CSV uploads for a given tenant.
// @Tags records
// @Produce json
// @Security BearerAuth
// @Param id path int true "Tenant ID"
// @Success 200 {array} Record
// @Failure 404 {object} object{error=string}
// @Router /tenants/{id}/records [get]
func (h *Handlers) ListTenantRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/records")
	id, err := parseID(path, "/api/tenants/")
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	tenant, err := h.db.GetTenant(id)
	if err != nil {
		http.Error(w, `{"error":"tenant not found"}`, http.StatusNotFound)
		return
	}
	records, err := h.db.ListRecords(tenant.TenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if records == nil {
		records = []Record{}
	}
	writeJSON(w, http.StatusOK, records)
}

func parseID(path, prefix string) (int64, error) {
	s := strings.TrimPrefix(path, prefix)
	s = strings.Split(s, "/")[0]
	return strconv.ParseInt(s, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	msg := map[string]string{"error": err.Error()}
	if encErr := json.NewEncoder(w).Encode(msg); encErr != nil {
		log.Printf("write error: %v", encErr)
	}
}
