package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Worker downloads CSV files from S3 and upserts the rows into the records table.
type Worker struct {
	db     *DB
	minio  *minio.Client
	bucket string
}

// NewWorker creates a Worker backed by the given MinIO/S3 configuration.
func NewWorker(db *DB, cfg Config) (*Worker, error) {
	if cfg.S3Endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT is required for worker")
	}
	endpoint := strings.TrimPrefix(cfg.S3Endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &Worker{db: db, minio: client, bucket: cfg.S3Bucket}, nil
}

// ProcessUploadEvent handles an SFTPGo upload event by downloading the CSV
// from S3 and upserting each row into the records table.
//
// Expected CSV columns: key, title, description, category, value
func (w *Worker) ProcessUploadEvent(event map[string]any) {
	username, _ := event["username"].(string)
	virtualPath, _ := event["virtual_path"].(string)

	if username == "" || virtualPath == "" {
		log.Printf("worker: missing username or virtual_path in event")
		return
	}

	if !strings.HasSuffix(strings.ToLower(virtualPath), ".csv") {
		log.Printf("worker: skipping non-CSV file %s", virtualPath)
		return
	}

	tenant, err := w.db.GetTenantByUsername(username)
	if err != nil {
		log.Printf("worker: tenant %s not found: %v", username, err)
		return
	}

	objectKey := tenant.TenantID + "/" + strings.TrimPrefix(virtualPath, "/")

	obj, err := w.minio.GetObject(context.Background(), w.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("worker: failed to get object %s: %v", objectKey, err)
		return
	}
	defer obj.Close()

	reader := csv.NewReader(obj)

	header, err := reader.Read()
	if err != nil {
		log.Printf("worker: failed to read CSV header: %v", err)
		return
	}

	colIndex := make(map[string]int, len(header))
	for i, h := range header {
		colIndex[strings.TrimSpace(strings.ToLower(h))] = i
	}
	for _, required := range []string{"key", "title", "value"} {
		if _, ok := colIndex[required]; !ok {
			log.Printf("worker: CSV missing required column: %s", required)
			return
		}
	}

	count := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("worker: CSV read error: %v", err)
			return
		}

		recordKey := strings.TrimSpace(row[colIndex["key"]])
		title := strings.TrimSpace(row[colIndex["title"]])

		var description string
		if idx, ok := colIndex["description"]; ok {
			description = strings.TrimSpace(row[idx])
		}

		var category string
		if idx, ok := colIndex["category"]; ok {
			category = strings.TrimSpace(row[idx])
		}

		value, err := strconv.ParseFloat(strings.TrimSpace(row[colIndex["value"]]), 64)
		if err != nil {
			log.Printf("worker: invalid value in row: %v", err)
			continue
		}

		if err := w.db.UpsertRecord(tenant.TenantID, recordKey, title, description, category, value); err != nil {
			log.Printf("worker: upsert record error: %v", err)
			continue
		}
		count++
	}

	log.Printf("worker: processed %d records for tenant %s", count, tenant.TenantID)
}
