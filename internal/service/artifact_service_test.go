package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"testing"

	"artifact/internal/artifact"
	"artifact/internal/config"
)

type fakeStore struct {
	mu          sync.Mutex
	body        []byte
	contentType string
	key         string
	metadata    map[string]string
	size        int64
	putCalls    int
}

func (f *fakeStore) HealthCheck(ctx context.Context) error { return nil }
func (f *fakeStore) EnsureBucketExists(ctx context.Context) error {
	return nil
}
func (f *fakeStore) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) error {
	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.body = body
	f.contentType = contentType
	f.key = key
	f.metadata = metadata
	f.size = size
	f.putCalls++
	return nil
}
func (f *fakeStore) GetObject(ctx context.Context, key string) (io.ReadCloser, map[string]string, error) {
	return io.NopCloser(bytes.NewReader(f.body)), f.metadata, nil
}
func (f *fakeStore) StatObject(ctx context.Context, key string) (map[string]string, error) {
	return f.metadata, nil
}
func (f *fakeStore) ListObjects(ctx context.Context, prefix string, limit int) ([]artifact.ObjectInfo, error) {
	return nil, nil
}
func (f *fakeStore) DeleteObject(ctx context.Context, key string) error { return nil }

func testConfig(maxSize int64) *config.Config {
	return &config.Config{
		Upload: config.UploadConfig{
			AllowedExtensions: []string{".tar.gz", ".deb"},
			MaxFileSizeBytes:  maxSize,
			MaxBatchFiles:     10,
		},
		Naming: config.NamingConfig{Prefix: "artifacts"},
	}
}

func TestUploadArtifactFile_streamsThroughStoreWithMetadata(t *testing.T) {
	store := &fakeStore{}
	svc := NewArtifactService(store, testConfig(1024))
	body := []byte("package-content")

	res, err := svc.UploadArtifactFile(context.Background(), UploadInput{
		Filename:   "agent.tar.gz",
		Size:       int64(len(body)),
		Reader:     bytes.NewReader(body),
		UploadUser: "ci",
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	wantHashBytes := sha256.Sum256(body)
	wantHash := hex.EncodeToString(wantHashBytes[:])
	if res.Sha256 != wantHash {
		t.Fatalf("sha256 = %q want %q", res.Sha256, wantHash)
	}
	if !bytes.Equal(store.body, body) {
		t.Fatalf("stored body = %q want %q", store.body, body)
	}
	if store.size != int64(len(body)) {
		t.Fatalf("size = %d want %d", store.size, len(body))
	}
	if store.metadata["sha256"] != wantHash {
		t.Fatalf("metadata sha256 = %q want %q", store.metadata["sha256"], wantHash)
	}
	if store.metadata["username"] != "ci" {
		t.Fatalf("metadata username = %q", store.metadata["username"])
	}
	if store.putCalls != 1 {
		t.Fatalf("put calls = %d", store.putCalls)
	}
}

func TestUploadArtifactFile_rejectsActualOversizeBeforeStore(t *testing.T) {
	store := &fakeStore{}
	svc := NewArtifactService(store, testConfig(3))

	_, err := svc.UploadArtifactFile(context.Background(), UploadInput{
		Filename: "agent.deb",
		Size:     3,
		Reader:   bytes.NewReader([]byte("toolarge")),
	})
	if !errors.Is(err, artifact.ErrFileNotValid) {
		t.Fatalf("err = %v want ErrFileNotValid", err)
	}
	if store.putCalls != 0 {
		t.Fatalf("put calls = %d want 0", store.putCalls)
	}
}

func TestUploadMultipleFiles_preservesResultOrder(t *testing.T) {
	store := &fakeStore{}
	svc := NewArtifactService(store, testConfig(1024))

	results, err := svc.UploadMultipleFiles(context.Background(), []UploadInput{
		{Filename: "first.deb", Size: 5, Reader: bytes.NewReader([]byte("first"))},
		{Filename: "second.deb", Size: 6, Reader: bytes.NewReader([]byte("second"))},
	})
	if err != nil {
		t.Fatalf("batch upload: %v", err)
	}
	if len(results.Artifacts) != 2 {
		t.Fatalf("artifacts len = %d", len(results.Artifacts))
	}
	if results.Artifacts[0].Filename != "first.deb" || results.Artifacts[1].Filename != "second.deb" {
		t.Fatalf("result order = %q, %q", results.Artifacts[0].Filename, results.Artifacts[1].Filename)
	}
	if len(results.Errors) != 0 {
		t.Fatalf("errors = %#v", results.Errors)
	}
	if store.putCalls != 2 {
		t.Fatalf("put calls = %d want 2", store.putCalls)
	}
}

func TestUploadMultipleFiles_returnsPartialSuccess(t *testing.T) {
	store := &fakeStore{}
	svc := NewArtifactService(store, testConfig(1024))

	results, err := svc.UploadMultipleFiles(context.Background(), []UploadInput{
		{Filename: "first.deb", Size: 5, Reader: bytes.NewReader([]byte("first"))},
		{Filename: "bad.exe", Size: 3, Reader: bytes.NewReader([]byte("bad"))},
		{Filename: "second.deb", Size: 6, Reader: bytes.NewReader([]byte("second"))},
	})
	if err != nil {
		t.Fatalf("batch upload: %v", err)
	}
	if len(results.Artifacts) != 2 {
		t.Fatalf("artifacts len = %d", len(results.Artifacts))
	}
	if len(results.Errors) != 1 {
		t.Fatalf("errors len = %d", len(results.Errors))
	}
	if results.Errors[0].Index != 1 || results.Errors[0].Filename != "bad.exe" {
		t.Fatalf("error result = %#v", results.Errors[0])
	}
	if results.Errors[0].Error != "validation_failed" {
		t.Fatalf("error code = %q", results.Errors[0].Error)
	}
	if store.putCalls != 2 {
		t.Fatalf("put calls = %d want 2", store.putCalls)
	}
}
