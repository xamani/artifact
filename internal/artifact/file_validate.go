package artifact

import (
	"fmt"
	"path/filepath"
	"strings"
)

type FileValidateRules struct {
	MaxFileSizeBytes  int64
	AllowedExtensions []string
}

func CheckFileExtension(filename string, rules FileValidateRules) error {
	ext := FileExtension(filename, rules.AllowedExtensions)
	if ext == "" {
		return NewRichError(ErrFileNotValid, "file has no extension", map[string]string{
			"field":  "filename",
			"reason": "no_extension",
			"file":   filepath.Base(filename),
		})
	}
	for _, a := range rules.AllowedExtensions {
		if ext == strings.ToLower(strings.TrimSpace(a)) {
			return nil
		}
	}
	return NewRichError(ErrFileNotValid, fmt.Sprintf("extension %q not allowed", ext), map[string]string{
		"field":     "filename",
		"reason":    "extension_not_allowed",
		"extension": ext,
		"file":      filepath.Base(filename),
	})
}

func CheckFileSize(size int64, rules FileValidateRules) error {
	if size <= 0 {
		return NewRichError(ErrFileNotValid, "file is empty", map[string]string{
			"field":  "size",
			"reason": "empty_file",
		})
	}
	if size > rules.MaxFileSizeBytes {
		return NewRichError(ErrFileNotValid, "file exceeds size limit", map[string]string{
			"field":    "size",
			"reason":   "file_too_large",
			"size":     fmt.Sprintf("%d", size),
			"max_size": fmt.Sprintf("%d", rules.MaxFileSizeBytes),
		})
	}
	return nil
}

func ValidateUploadFile(filename string, size int64, rules FileValidateRules) error {
	if err := CheckFileExtension(filename, rules); err != nil {
		return err
	}
	return CheckFileSize(size, rules)
}

func fileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filepath.Base(filename)))
}

func FileExtension(filename string, allowed []string) string {
	base := strings.ToLower(filepath.Base(filename))
	longest := ""
	for _, ext := range allowed {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if strings.HasSuffix(base, ext) && len(ext) > len(longest) {
			longest = ext
		}
	}
	if longest != "" {
		return longest
	}
	return fileExtension(filename)
}
