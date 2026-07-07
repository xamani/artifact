package artifact

import "testing"

func TestCheckFileExtension_ok(t *testing.T) {
	rules := FileValidateRules{
		AllowedExtensions: []string{".deb"},
		MaxFileSizeBytes:  1000,
	}
	if err := CheckFileExtension("app.deb", rules); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckFileSize_tooLarge(t *testing.T) {
	rules := FileValidateRules{MaxFileSizeBytes: 10}
	if err := CheckFileSize(100, rules); err == nil {
		t.Fatal("expected size error")
	}
}

func TestCheckFileSize_empty(t *testing.T) {
	rules := FileValidateRules{MaxFileSizeBytes: 10}
	if err := CheckFileSize(0, rules); err == nil {
		t.Fatal("expected empty file error")
	}
}
