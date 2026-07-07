package storage

import (
	"context"
	"io"

	"artifact/internal/artifact"
)

type ObjectStore interface {
	HealthCheck(ctx context.Context) error
	EnsureBucketExists(ctx context.Context) error
	PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) error
	GetObject(ctx context.Context, key string) (io.ReadCloser, map[string]string, error)
	StatObject(ctx context.Context, key string) (map[string]string, error)
	ListObjects(ctx context.Context, prefix string, limit int) ([]artifact.ObjectInfo, error)
	DeleteObject(ctx context.Context, key string) error
}
