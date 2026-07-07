package artifact

import (
	"fmt"
	"os"
	"os/user"
	"runtime"
	"time"
)

type MetadataInput struct {
	Size        int64
	Filename    string
	Sha256      string
	UploadUser  string
	ContentType string
}

func BuildUploadMetadata(in MetadataInput) map[string]string {
	hostname, _ := os.Hostname() // ok if empty
	username := in.UploadUser
	if username == "" {
		if u, err := user.Current(); err == nil {
			username = u.Username
		}
	}

	return map[string]string{
		"upload-timestamp":  time.Now().UTC().Format(time.RFC3339),
		"os":                runtime.GOOS,
		"architecture":      runtime.GOARCH,
		"hostname":          hostname,
		"username":          username,
		"sha256":            in.Sha256,
		"original-filename": in.Filename,
		"mime-type":         in.ContentType,
		"size":              fmt.Sprintf("%d", in.Size),
	}
}
