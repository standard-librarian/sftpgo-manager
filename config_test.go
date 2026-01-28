package main

import (
	"os"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear any env vars that might be set.
	for _, k := range []string{"SFTPGO_URL", "SFTPGO_ADMIN_USER", "LISTEN_ADDR", "DATA_DIR", "S3_BUCKET", "S3_REGION", "S3_ENDPOINT"} {
		t.Setenv(k, "")
	}

	cfg := LoadConfig()

	if cfg.SFTPGoURL != "http://localhost:8080" {
		t.Errorf("SFTPGoURL = %q, want default", cfg.SFTPGoURL)
	}
	if cfg.AdminUser != "admin" {
		t.Errorf("AdminUser = %q, want %q", cfg.AdminUser, "admin")
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.S3Endpoint != "" {
		t.Errorf("S3Endpoint = %q, want empty", cfg.S3Endpoint)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("SFTPGO_URL", "http://custom:8080")
	t.Setenv("LISTEN_ADDR", ":3000")
	t.Setenv("S3_ENDPOINT", "http://minio:9000")
	t.Setenv("S3_USE_SSL", "true")

	cfg := LoadConfig()

	if cfg.SFTPGoURL != "http://custom:8080" {
		t.Errorf("SFTPGoURL = %q, want %q", cfg.SFTPGoURL, "http://custom:8080")
	}
	if cfg.ListenAddr != ":3000" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":3000")
	}
	if cfg.S3Endpoint != "http://minio:9000" {
		t.Errorf("S3Endpoint = %q, want %q", cfg.S3Endpoint, "http://minio:9000")
	}
	if !cfg.S3UseSSL {
		t.Error("S3UseSSL should be true")
	}
}

func TestEnvOr(t *testing.T) {
	key := "TEST_ENVOR_KEY_" + t.Name()
	_ = os.Unsetenv(key)

	if got := envOr(key, "default"); got != "default" {
		t.Errorf("envOr = %q, want %q", got, "default")
	}

	t.Setenv(key, "custom")
	if got := envOr(key, "default"); got != "custom" {
		t.Errorf("envOr = %q, want %q", got, "custom")
	}
}
