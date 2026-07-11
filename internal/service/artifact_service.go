package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"time"

	"artifact/internal/artifact"
	"artifact/internal/config"
	"artifact/internal/storage"
)

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

type BatchUploadResult struct {
	Artifacts []*artifact.UploadFileResult `json:"artifacts"`
	Errors    []BatchUploadError           `json:"errors,omitempty"`
}

type BatchUploadError struct {
	Details  map[string]string `json:"details,omitempty"`
	Error    string            `json:"error"`
	Message  string            `json:"message"`
	Filename string            `json:"filename"`
	Index    int               `json:"index"`
}

func (s *ArtifactService) UploadArtifactFile(ctx context.Context, in UploadInput) (*artifact.UploadFileResult, error) {
	rules := artifact.FileValidateRules{
		AllowedExtensions: s.cfg.Upload.AllowedExtensions,
		MaxFileSizeBytes:  s.cfg.Upload.MaxFileSizeBytes,
	}
	if err := artifact.ValidateUploadFile(in.Filename, in.Size, rules); err != nil {
		return nil, err
	}
	tmp, checksum, actualSize, err := spoolAndHash(in.Reader, rules.MaxFileSizeBytes)
	if err != nil {
		return nil, err
	}
	defer func() {
		name := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(name)
	}()
	if err := artifact.CheckFileSize(actualSize, rules); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	key := artifact.GenerateObjectKey(artifact.ObjectKeyInput{
		Prefix: s.cfg.Naming.Prefix, Filename: in.Filename, Os: runtime.GOOS, Arch: runtime.GOARCH,
		Sha256: checksum, UploadTime: now,
	})
	contentType := mime.TypeByExtension(artifact.FileExtension(in.Filename, s.cfg.Upload.AllowedExtensions))
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(in.Filename))
	}
	meta := artifact.BuildUploadMetadata(artifact.MetadataInput{
		Filename: in.Filename, Size: actualSize, Sha256: checksum, UploadUser: in.UploadUser, ContentType: contentType,
	})
	if err := s.store.PutObject(ctx, key, tmp, actualSize, contentType, meta); err != nil {
		return nil, fmt.Errorf("upload %q: %w", key, err)
	}
	return &artifact.UploadFileResult{Key: key, Size: actualSize, Sha256: checksum, Metadata: meta, Filename: in.Filename}, nil
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

func (s *ArtifactService) UploadMultipleFiles(ctx context.Context, files []UploadInput) (*BatchUploadResult, error) {
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
	if len(files) == 0 {
		return &BatchUploadResult{Artifacts: []*artifact.UploadFileResult{}}, nil
	}

	results := make([]*artifact.UploadFileResult, len(files))
	errorsByIndex := make([]*BatchUploadError, len(files))
	limit := minInt(len(files), runtime.NumCPU())
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, f := range files {
		wg.Add(1)
		go func(i int, f UploadInput) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errResult := batchUploadError(i, f.Filename, ctx.Err())
				errorsByIndex[i] = &errResult
				return
			}
			res, err := s.UploadArtifactFile(ctx, f)
			if err != nil {
				errResult := batchUploadError(i, f.Filename, err)
				errorsByIndex[i] = &errResult
				return
			}
			results[i] = res
		}(i, f)
	}
	wg.Wait()

	out := &BatchUploadResult{
		Artifacts: make([]*artifact.UploadFileResult, 0, len(files)),
	}
	for _, res := range results {
		if res != nil {
			out.Artifacts = append(out.Artifacts, res)
		}
	}
	for _, errResult := range errorsByIndex {
		if errResult != nil {
			out.Errors = append(out.Errors, *errResult)
		}
	}
	return out, nil
}

func spoolAndHash(r io.Reader, maxSize int64) (*os.File, string, int64, error) {
	tmp, err := os.CreateTemp("", "artifact-upload-*")
	if err != nil {
		return nil, "", 0, fmt.Errorf("create upload temp file: %w", err)
	}
	cleanup := func() {
		name := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(name)
	}

	h := sha256.New()
	limit := maxSize + 1
	written, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, limit))
	if err != nil {
		cleanup()
		return nil, "", 0, fmt.Errorf("read upload file: %w", err)
	}
	if written > maxSize {
		cleanup()
		return nil, "", written, artifact.NewRichError(artifact.ErrFileNotValid, "file exceeds size limit", map[string]string{
			"field":    "size",
			"reason":   "file_too_large",
			"size":     strconv.FormatInt(written, 10),
			"max_size": strconv.FormatInt(maxSize, 10),
		})
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, "", 0, fmt.Errorf("rewind upload temp file: %w", err)
	}
	return tmp, hex.EncodeToString(h.Sum(nil)), written, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func batchUploadError(index int, filename string, err error) BatchUploadError {
	out := BatchUploadError{
		Index:    index,
		Filename: filename,
		Error:    "upload_failed",
		Message:  "upload failed",
	}
	if err != nil {
		out.Message = err.Error()
	}
	if rich, ok := artifact.AsRichError(err); ok {
		if rich.Sentinel != nil {
			out.Error = errorCode(rich.Sentinel)
		}
		if rich.Message != "" {
			out.Message = rich.Message
		}
		out.Details = rich.Details
	}
	return out
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, artifact.ErrFileNotValid):
		return "validation_failed"
	case errors.Is(err, artifact.ErrArtifactNotFound):
		return "not_found"
	case errors.Is(err, artifact.ErrStorageConnect):
		return "storage_unavailable"
	case errors.Is(err, artifact.ErrAuthFailed):
		return "authentication_failed"
	case errors.Is(err, artifact.ErrBatchTooManyFiles):
		return "batch_too_many_files"
	default:
		return "upload_failed"
	}
}
