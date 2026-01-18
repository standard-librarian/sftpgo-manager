package main

import (
	"net/http"
	"strings"
)

// AuthMiddleware returns a handler that validates the Bearer token against
// stored API keys before calling next.
func AuthMiddleware(db *DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"missing api key"}`, http.StatusUnauthorized)
			return
		}
		key := strings.TrimPrefix(auth, "Bearer ")
		if err := db.ValidateAPIKey(key); err != nil {
			http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}
