package artifact

import "errors"

var (
	ErrFileNotValid      = errors.New("file is not valid")
	ErrArtifactNotFound  = errors.New("artifact not found")
	ErrAuthFailed        = errors.New("authentication failed")
	ErrStorageConnect    = errors.New("storage connection failed")
	ErrBatchTooManyFiles = errors.New("too many files in batch")
)

type UploadFileResult struct {
	Size     int64             `json:"size"`
	Metadata map[string]string `json:"metadata"`
	Key      string            `json:"key"`
	Sha256   string            `json:"sha256"`
	Filename string            `json:"filename"`
}

type ObjectInfo struct {
	Size     int64  `json:"size"`
	Key      string `json:"key"`
	UploadAt string `json:"upload_at,omitempty"`
}
