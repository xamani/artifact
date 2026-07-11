package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"path/filepath"

	"artifact/internal/artifact"
	"artifact/internal/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const listDefaultLimit = 100

type MinioStorage struct {
	client *minio.Client
	bucket string
}

func NewMinioStorage(cfg config.StorageConfig) (*MinioStorage, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &MinioStorage{client: client, bucket: cfg.Bucket}, nil
}

func (m *MinioStorage) HealthCheck(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return fmt.Errorf("%w: %v", artifact.ErrStorageConnect, err)
	}
	if !exists {
		return fmt.Errorf("%w: bucket %q not exist", artifact.ErrStorageConnect, m.bucket)
	}
	return nil
}

// called on /health
func (m *MinioStorage) EnsureBucketExists(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}
	if err := m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return nil
}

func (m *MinioStorage) PutObject(ctx context.Context, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) error {
	key = artifact.NormalizeObjectKey(key)
	if contentType == "" {
		contentType = contentTypeFromFilename(key)
	}
	_, err := m.client.PutObject(ctx, m.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: metadata,
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}
	return nil
}

func (m *MinioStorage) GetObject(ctx context.Context, key string) (io.ReadCloser, map[string]string, error) {
	key = artifact.NormalizeObjectKey(key)
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, mapMinioError(err, key)
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, nil, mapMinioError(err, key)
	}
	return obj, info.UserMetadata, nil
}

func (m *MinioStorage) StatObject(ctx context.Context, key string) (map[string]string, error) {
	key = artifact.NormalizeObjectKey(key)
	info, err := m.client.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return nil, mapMinioError(err, key)
	}
	return info.UserMetadata, nil
}

func (m *MinioStorage) ListObjects(ctx context.Context, prefix string, limit int) ([]artifact.ObjectInfo, error) {
	if limit <= 0 {
		limit = listDefaultLimit
	}
	prefix = artifact.NormalizeObjectKey(prefix)
	ch := m.client.ListObjects(ctx, m.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	out := make([]artifact.ObjectInfo, 0, limit)
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		out = append(out, artifact.ObjectInfo{Key: obj.Key, Size: obj.Size, UploadAt: obj.LastModified.UTC().Format("2006-01-02T15:04:05Z")})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MinioStorage) DeleteObject(ctx context.Context, key string) error {
	key = artifact.NormalizeObjectKey(key)
	if err := m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return mapMinioError(err, key)
	}
	return nil
}

func mapMinioError(err error, key string) error {
	resp := minio.ToErrorResponse(err)
	if resp.Code == "NoSuchKey" || resp.Code == "NotFound" {
		return artifact.NewRichError(artifact.ErrArtifactNotFound,
			fmt.Sprintf("artifact %q not found", key),
			map[string]string{"key": key, "reason": "not_found"})
	}
	return fmt.Errorf("minio operation on %q: %w", key, err)
}

func contentTypeFromFilename(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "application/octet-stream"
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	return "application/octet-stream"
}
