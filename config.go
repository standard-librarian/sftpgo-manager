package main

import "os"

// Config holds all application configuration loaded from environment variables.
type Config struct {
	SFTPGoURL  string
	AdminUser  string
	AdminPass  string
	ListenAddr string
	DataDir    string

	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3UseSSL    bool
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() Config {
	return Config{
		SFTPGoURL:   envOr("SFTPGO_URL", "http://localhost:8080"),
		AdminUser:   envOr("SFTPGO_ADMIN_USER", "admin"),
		AdminPass:   envOr("SFTPGO_ADMIN_PASS", "admin"),
		ListenAddr:  envOr("LISTEN_ADDR", ":9090"),
		DataDir:     envOr("DATA_DIR", "/srv/sftpgo/data"),
		S3Bucket:    envOr("S3_BUCKET", "sftpgo"),
		S3Region:    envOr("S3_REGION", "us-east-1"),
		S3Endpoint:  envOr("S3_ENDPOINT", ""),
		S3AccessKey: envOr("S3_ACCESS_KEY", ""),
		S3SecretKey: envOr("S3_SECRET_KEY", ""),
		S3UseSSL:    os.Getenv("S3_USE_SSL") == "true",
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
