package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateConfig_missingApiKey(t *testing.T) {
	cfg := &Config{
		Server:  ServerConfig{Port: 8080},
		Storage: StorageConfig{Endpoint: "localhost:9000", AccessKey: "a", SecretKey: "b", Bucket: "bkt"},
		Upload:  UploadConfig{AllowedExtensions: []string{".deb"}, MaxFileSizeBytes: 100},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected error for empty api key")
	}
}

func TestValidateConfig_missingBucket(t *testing.T) {
	cfg := &Config{
		Server:  ServerConfig{ApiKey: "key", Port: 8080},
		Storage: StorageConfig{Endpoint: "localhost:9000", AccessKey: "a", SecretKey: "b"},
		Upload:  UploadConfig{AllowedExtensions: []string{".deb"}, MaxFileSizeBytes: 100},
	}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected error for empty bucket")
	}
}

func TestLoadConfigFromFile_ok(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
server:
  host: "0.0.0.0"
  port: 8080
  api_key: "test-key"
storage:
  endpoint: "localhost:9000"
  access_key: "minioadmin"
  secret_key: "minioadmin"
  bucket: "bucket-1"
  region: "us-east-1"
upload:
  allowed_extensions: [".deb"]
  max_file_size_bytes: 1024
  max_batch_files: 10
naming:
  prefix: "artifacts"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigFromFile(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.ApiKey != "test-key" {
		t.Fatalf("api key = %q", cfg.Server.ApiKey)
	}
}
