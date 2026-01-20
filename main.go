package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "sftpgo-manager/docs"
)

// @title SFTPGo Manager API
// @version 1.0
// @description Multi-tenant SFTP management with S3-backed storage and automatic CSV ingestion into a records table.

// @host localhost:9090
// @BasePath /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter "Bearer <api_key>"

func main() {
	cfg := LoadConfig()

	db, err := NewDB("sftpgo.db")
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	sftpgoClient := NewSFTPGoClient(cfg.SFTPGoURL, cfg.AdminUser, cfg.AdminPass)

	h := &Handlers{db: db, sftpgo: sftpgoClient, cfg: cfg}

	if cfg.S3Endpoint != "" {
		worker, err := NewWorker(db, cfg)
		if err != nil {
			log.Printf("warning: worker init failed (CSV processing disabled): %v", err)
		} else {
			h.worker = worker
			log.Printf("worker initialized, CSV processing enabled")
		}
	}

	mux := http.NewServeMux()

	// Bootstrap endpoint — no auth
	mux.HandleFunc("/api/keys", h.CreateAPIKey)

	// SFTPGo hook endpoints — no API key auth (called by SFTPGo internally)
	mux.HandleFunc("/api/auth/hook", h.ExternalAuthHook)
	mux.HandleFunc("/api/events/upload", h.UploadEventHook)

	// Authenticated endpoints
	mux.HandleFunc("/api/tenants", AuthMiddleware(db, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.CreateTenant(w, r)
		} else {
			h.ListTenants(w, r)
		}
	}))

	mux.HandleFunc("/api/tenants/", AuthMiddleware(db, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/records") {
			h.ListTenantRecords(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/validate") {
			h.ValidateTenant(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/keys") {
			h.UpdateTenantKeys(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.GetTenant(w, r)
		case http.MethodDelete:
			h.DeleteTenant(w, r)
		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}))

	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: mux}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("received %s, shutting down", sig)
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("listening on %s", cfg.ListenAddr)
	log.Printf("swagger UI: http://localhost%s/swagger/index.html", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
