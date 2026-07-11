package artifact

import (
	"fmt"
	"path"
	"strings"
	"time"
)

type ObjectKeyInput struct {
	Prefix     string
	Filename   string
	Os         string
	Arch       string
	Sha256     string
	UploadTime time.Time
}

// layout: {prefix}/YYYY/MM/os/arch/{sha16}/filename
func GenerateObjectKey(in ObjectKeyInput) string {
	t := in.UploadTime.UTC()
	if t.IsZero() {
		t = time.Now().UTC()
	}
	hashPrefix := in.Sha256
	if len(hashPrefix) > 16 {
		hashPrefix = hashPrefix[:16]
	}
	year := fmt.Sprintf("%04d", t.Year())
	month := fmt.Sprintf("%02d", int(t.Month()))
	parts := []string{
		strings.Trim(in.Prefix, "/"),
		year,
		month,
		in.Os,
		in.Arch,
		hashPrefix,
		path.Base(in.Filename),
	}
	key := path.Join(parts...)
	return strings.TrimPrefix(key, "/")
}

func NormalizeObjectKey(key string) string {
	return strings.TrimPrefix(strings.TrimSpace(key), "/")
}
