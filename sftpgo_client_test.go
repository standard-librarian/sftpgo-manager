package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newMockSFTPGo(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v2/token", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "admin" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "mock-token-123",
			"expires_at":   "2099-01-01T00:00:00Z",
		})
	})

	mux.HandleFunc("/api/v2/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mock-token-123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"username": "testuser",
				"status":   1,
			})
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		}
	})

	mux.HandleFunc("/api/v2/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer mock-token-123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSFTPGoClientGetToken(t *testing.T) {
	srv := newMockSFTPGo(t)
	client := NewSFTPGoClient(srv.URL, "admin", "admin")

	token, err := client.getToken()
	if err != nil {
		t.Fatalf("getToken: %v", err)
	}
	if token != "mock-token-123" {
		t.Errorf("token = %q, want %q", token, "mock-token-123")
	}
}

func TestSFTPGoClientGetTokenCaching(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/token" {
			callCount++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "cached-token",
				"expires_at":   "2099-01-01T00:00:00Z",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := NewSFTPGoClient(srv.URL, "admin", "admin")

	if _, err := client.getToken(); err != nil {
		t.Fatalf("first getToken: %v", err)
	}
	if _, err := client.getToken(); err != nil {
		t.Fatalf("second getToken: %v", err)
	}
	if callCount != 1 {
		t.Errorf("token endpoint called %d times, want 1 (should be cached)", callCount)
	}
}

func TestSFTPGoClientGetTokenBadCredentials(t *testing.T) {
	srv := newMockSFTPGo(t)
	client := NewSFTPGoClient(srv.URL, "wrong", "creds")

	if _, err := client.getToken(); err == nil {
		t.Error("expected error for bad credentials")
	}
}

func TestSFTPGoClientCreateUser(t *testing.T) {
	srv := newMockSFTPGo(t)
	client := NewSFTPGoClient(srv.URL, "admin", "admin")

	err := client.CreateUser("testuser", "pass", "/data/test", nil, nil, "tenant1")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
}

func TestSFTPGoClientCreateUserWithS3(t *testing.T) {
	srv := newMockSFTPGo(t)
	client := NewSFTPGoClient(srv.URL, "admin", "admin")

	s3 := &S3Config{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		Endpoint:  "http://minio:9000",
		AccessKey: "access",
		SecretKey: "secret",
	}
	err := client.CreateUser("testuser", "pass", "/data/test", []string{"ssh-ed25519 AAAA"}, s3, "tenant1")
	if err != nil {
		t.Fatalf("CreateUser with S3: %v", err)
	}
}

func TestSFTPGoClientGetUser(t *testing.T) {
	srv := newMockSFTPGo(t)
	client := NewSFTPGoClient(srv.URL, "admin", "admin")

	user, err := client.GetUser("testuser")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if user["username"] != "testuser" {
		t.Errorf("username = %v, want %q", user["username"], "testuser")
	}
}

func TestSFTPGoClientUpdateUserPublicKeys(t *testing.T) {
	srv := newMockSFTPGo(t)
	client := NewSFTPGoClient(srv.URL, "admin", "admin")

	err := client.UpdateUserPublicKeys("testuser", []string{"ssh-ed25519 NEWKEY"})
	if err != nil {
		t.Fatalf("UpdateUserPublicKeys: %v", err)
	}
}

func TestSFTPGoClientDeleteUser(t *testing.T) {
	srv := newMockSFTPGo(t)
	client := NewSFTPGoClient(srv.URL, "admin", "admin")

	err := client.DeleteUser("testuser")
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
}
