package app

import (
	"os"
	"path/filepath"
	"testing"
)

func withTestConfigBaseDir(t *testing.T) string {
	t.Helper()
	old := getConfig()
	base := t.TempDir()
	setConfig(AppConfig{
		BaseDir:        base,
		VideoOutputDir: filepath.Join(base, "output"),
	})
	t.Cleanup(func() {
		setConfig(old)
	})
	return base
}

func TestCleanRelativeInput(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "root", input: "/", want: ""},
		{name: "empty", input: "", want: ""},
		{name: "slashes", input: "//a//b///", want: "a/b"},
		{name: "backslashes", input: `a\\b\\c`, want: "a/b/c"},
		{name: "dotdot cleaned", input: "../folder", want: "folder"},
		{name: "null byte rejected", input: "a\x00b", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanRelativeInput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected result: got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToAbsolutePathAndWithinBase(t *testing.T) {
	base := withTestConfigBaseDir(t)

	gotRoot, err := toAbsolutePath("/")
	if err != nil {
		t.Fatalf("unexpected error for root: %v", err)
	}
	if gotRoot != base {
		t.Fatalf("root path mismatch: got %q, want %q", gotRoot, base)
	}

	gotNested, err := toAbsolutePath("/folder/sub")
	if err != nil {
		t.Fatalf("unexpected error for nested path: %v", err)
	}
	if !isPathWithinBase(gotNested) {
		t.Fatalf("expected path within base, got %q", gotNested)
	}

	if _, err := toAbsolutePath("bad\x00path"); err == nil {
		t.Fatalf("expected invalid path error for null byte input")
	}
}

func TestToAbsolutePathRejectsSymlinkSegments(t *testing.T) {
	base := withTestConfigBaseDir(t)

	targetDir := t.TempDir()
	linkPath := filepath.Join(base, "linked")
	if err := os.Symlink(targetDir, linkPath); err != nil {
		t.Skipf("symlink not supported in this environment: %v", err)
	}

	if _, err := toAbsolutePath("/linked/file.txt"); err == nil {
		t.Fatalf("expected symlink path rejection")
	}
}

func TestSanitizeFileName(t *testing.T) {
	valid, err := sanitizeFileName("video.mp4")
	if err != nil {
		t.Fatalf("unexpected error for valid file name: %v", err)
	}
	if valid != "video.mp4" {
		t.Fatalf("unexpected sanitized file name: %q", valid)
	}

	invalidInputs := []string{"", "..", "bad/name.mp4", "bad\\name.mp4", "NUL.txt"}
	for _, input := range invalidInputs {
		if _, err := sanitizeFileName(input); err == nil {
			t.Fatalf("expected invalid file name error for %q", input)
		}
	}
}

func TestInferFileCategoryAndHidden(t *testing.T) {
	if got := inferFileCategory("clip.mp4", false); got != "video" {
		t.Fatalf("expected video category, got %q", got)
	}
	if got := inferFileCategory("cover.png", false); got != "image" {
		t.Fatalf("expected image category, got %q", got)
	}
	if got := inferFileCategory("src.go", false); got != "code" {
		t.Fatalf("expected code category, got %q", got)
	}
	if got := inferFileCategory("folder", true); got != "folder" {
		t.Fatalf("expected folder category, got %q", got)
	}

	if !isHiddenName(".gitignore") {
		t.Fatalf("expected hidden name detection for .gitignore")
	}
	if isHiddenName("visible.txt") {
		t.Fatalf("did not expect hidden name detection for visible.txt")
	}
}
