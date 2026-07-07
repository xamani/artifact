//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"artifact/internal/config"
	"artifact/internal/service"
	"artifact/internal/storage"
)

func TestUploadDownload_ok(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}
	cfg := &config.Config{
		Server: config.ServerConfig{ApiKey: "api-key-test"},
		Storage: config.StorageConfig{
			Endpoint: endpoint, AccessKey: "minioadmin", SecretKey: "minioadmin",
			Bucket: "bucket-1", Region: "us-east-1",
		},
		Upload: config.UploadConfig{
			AllowedExtensions: []string{".deb"}, MaxFileSizeBytes: 1024 * 1024, MaxBatchFiles: 5,
		},
		Naming: config.NamingConfig{Prefix: "artifacts"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := storage.NewMinioStorage(cfg.Storage)
	if err != nil {
		t.Skipf("minio not available: %v", err)
	}
	if err := store.EnsureBucketExists(ctx); err != nil {
		t.Skipf("minio bucket: %v", err)
	}

	svc := service.NewArtifactService(store, cfg)
	body := []byte("fake deb")
	res, err := svc.UploadArtifactFile(ctx, service.UploadInput{
		Filename: "itest.deb", Size: int64(len(body)), Reader: bytes.NewReader(body),
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	reader, _, err := svc.DownloadArtifactFile(ctx, res.Key)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatal("content mismatch")
	}

	if err := svc.DeleteArtifactFile(ctx, res.Key); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
