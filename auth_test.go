package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	db := newTestDB(t)
	handler := AuthMiddleware(db, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareInvalidKey(t *testing.T) {
	db := newTestDB(t)
	handler := AuthMiddleware(db, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddlewareValidKey(t *testing.T) {
	db := newTestDB(t)
	key, err := db.CreateAPIKey("test")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}

	called := false
	handler := AuthMiddleware(db, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+key.Key)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !called {
		t.Error("next handler was not called")
	}
}

func TestAuthMiddlewareBadFormat(t *testing.T) {
	db := newTestDB(t)
	handler := AuthMiddleware(db, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
