package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"artifact/internal/artifact"
	"artifact/internal/config"
	"artifact/internal/storage"
)

func sha256sum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("calculate checksum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type ArtifactService struct {
	cfg   *config.Config
	store storage.ObjectStore
}

func NewArtifactService(store storage.ObjectStore, cfg *config.Config) *ArtifactService {
	return &ArtifactService{store: store, cfg: cfg}
}

type UploadInput struct {
	Size       int64
	Reader     io.Reader
	Filename   string
	UploadUser string
}

func (s *ArtifactService) UploadArtifactFile(ctx context.Context, in UploadInput) (*artifact.UploadFileResult, error) {
	rules := artifact.FileValidateRules{
		AllowedExtensions: s.cfg.Upload.AllowedExtensions,
		MaxFileSizeBytes:  s.cfg.Upload.MaxFileSizeBytes,
	}
	if err := artifact.ValidateUploadFile(in.Filename, in.Size, rules); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(in.Reader)
	if err != nil {
		return nil, fmt.Errorf("read upload file: %w", err)
	}
	checksum, err := sha256sum(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	key := artifact.GenerateObjectKey(artifact.ObjectKeyInput{
		Prefix: s.cfg.Naming.Prefix, Filename: in.Filename, Os: runtime.GOOS, Arch: runtime.GOARCH,
		Sha256: checksum, UploadTime: now,
	})
	contentType := mime.TypeByExtension(filepath.Ext(in.Filename))
	meta := artifact.BuildUploadMetadata(artifact.MetadataInput{
		Filename: in.Filename, Size: in.Size, Sha256: checksum, UploadUser: in.UploadUser, ContentType: contentType,
	})
	if err := s.store.PutObject(ctx, key, bytes.NewReader(data), int64(len(data)), contentType, meta); err != nil {
		return nil, fmt.Errorf("upload %q: %w", key, err)
	}
	return &artifact.UploadFileResult{Key: key, Size: int64(len(data)), Sha256: checksum, Metadata: meta, Filename: in.Filename}, nil
}

func (s *ArtifactService) DownloadArtifactFile(ctx context.Context, key string) (io.ReadCloser, map[string]string, error) {
	return s.store.GetObject(ctx, key)
}

func (s *ArtifactService) GetArtifactMetadata(ctx context.Context, key string) (map[string]string, error) {
	return s.store.StatObject(ctx, key)
}

func (s *ArtifactService) GetArtifactList(ctx context.Context, prefix string, limit int) ([]artifact.ObjectInfo, error) {
	return s.store.ListObjects(ctx, prefix, limit)
}

func (s *ArtifactService) DeleteArtifactFile(ctx context.Context, key string) error {
	return s.store.DeleteObject(ctx, key)
}

func (s *ArtifactService) CheckStorageHealth(ctx context.Context) error {
	return s.store.HealthCheck(ctx)
}

func (s *ArtifactService) UploadMultipleFiles(ctx context.Context, files []UploadInput) ([]*artifact.UploadFileResult, error) {
	max := s.cfg.Upload.MaxBatchFiles
	if len(files) > max {
		return nil, artifact.NewRichError(artifact.ErrBatchTooManyFiles,
			fmt.Sprintf("max %d files per batch", max),
			map[string]string{
				"reason":          "batch_too_many_files",
				"max_batch_files": strconv.Itoa(max),
				"received":        strconv.Itoa(len(files)),
			})
	}
	results := make([]*artifact.UploadFileResult, 0, len(files))
	for _, f := range files {
		res, err := s.UploadArtifactFile(ctx, f)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}
