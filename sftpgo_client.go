package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// SFTPGoClient wraps the SFTPGo admin REST API with token caching.
type SFTPGoClient struct {
	baseURL   string
	adminUser string
	adminPass string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// NewSFTPGoClient returns a client configured to talk to the SFTPGo admin API.
func NewSFTPGoClient(baseURL, user, pass string) *SFTPGoClient {
	return &SFTPGoClient{baseURL: baseURL, adminUser: user, adminPass: pass}
}

func (c *SFTPGoClient) getToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	req, err := http.NewRequest("GET", c.baseURL+"/api/v2/token", nil)
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.SetBasicAuth(c.adminUser, c.adminPass)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed (%d): %s", resp.StatusCode, body)
	}

	var result struct {
		AccessToken string    `json:"access_token"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token: %w", err)
	}
	c.token = result.AccessToken
	c.tokenExp = result.ExpiresAt.Add(-30 * time.Second)
	return c.token, nil
}

func (c *SFTPGoClient) doAuth(req *http.Request) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

// S3Config holds the S3/MinIO credentials used when creating SFTPGo users
// with S3-backed storage.
type S3Config struct {
	Bucket    string
	Region    string
	Endpoint  string
	AccessKey string
	SecretKey string
}

// CreateUser creates a new user in SFTPGo with the given credentials and storage config.
// The tenantID is used as the S3 key prefix to isolate the tenant's files.
func (c *SFTPGoClient) CreateUser(username, password, homeDir string, publicKeys []string, s3 *S3Config, tenantID string) error {
	payload := map[string]any{
		"username":    username,
		"password":    password,
		"status":      1,
		"home_dir":    homeDir,
		"permissions": map[string][]string{"/": {"*"}},
	}
	if len(publicKeys) > 0 {
		payload["public_keys"] = publicKeys
	}
	if s3 != nil {
		payload["filesystem"] = map[string]any{
			"provider": 1,
			"s3config": map[string]any{
				"bucket":           s3.Bucket,
				"region":           s3.Region,
				"endpoint":         s3.Endpoint,
				"access_key":       s3.AccessKey,
				"access_secret":    map[string]string{"status": "Plain", "payload": s3.SecretKey},
				"key_prefix":       tenantID + "/",
				"force_path_style": true,
				"skip_tls_verify":  true,
			},
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal create user payload: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/users", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build create user request: %w", err)
	}

	resp, err := c.doAuth(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sftpgo create user (%d): %s", resp.StatusCode, b)
	}
	return nil
}

// GetUser retrieves user details from SFTPGo by username.
func (c *SFTPGoClient) GetUser(username string) (map[string]any, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v2/users/"+username, nil)
	if err != nil {
		return nil, fmt.Errorf("build get user request: %w", err)
	}

	resp, err := c.doAuth(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found in sftpgo")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sftpgo get user (%d): %s", resp.StatusCode, b)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode sftpgo user: %w", err)
	}
	return result, nil
}

// UpdateUserPublicKeys replaces the public keys for a user in SFTPGo.
func (c *SFTPGoClient) UpdateUserPublicKeys(username string, keys []string) error {
	payload := map[string]any{"public_keys": keys}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal update keys payload: %w", err)
	}

	req, err := http.NewRequest("PUT", c.baseURL+"/api/v2/users/"+username, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build update keys request: %w", err)
	}

	resp, err := c.doAuth(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sftpgo update user keys (%d): %s", resp.StatusCode, b)
	}
	return nil
}

// DeleteUser removes a user from SFTPGo by username.
func (c *SFTPGoClient) DeleteUser(username string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/api/v2/users/"+username, nil)
	if err != nil {
		return fmt.Errorf("build delete user request: %w", err)
	}

	resp, err := c.doAuth(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sftpgo delete user (%d): %s", resp.StatusCode, b)
	}
	return nil
}
