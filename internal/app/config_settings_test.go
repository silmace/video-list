package app

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeJSONBodyStrictParsing(t *testing.T) {
	t.Run("accepts valid payload", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"password":"secret"}`))
		rr := httptest.NewRecorder()

		var body LoginRequest
		if err := decodeJSONBody(rr, req, &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if body.Password != "secret" {
			t.Fatalf("decoded password mismatch: got %q", body.Password)
		}
	})

	t.Run("rejects unknown fields", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"password":"secret","extra":1}`))
		rr := httptest.NewRecorder()

		var body LoginRequest
		if err := decodeJSONBody(rr, req, &body); err == nil {
			t.Fatalf("expected unknown field error")
		}
	})

	t.Run("rejects trailing payload", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(`{"password":"secret"}{"extra":1}`))
		rr := httptest.NewRecorder()

		var body LoginRequest
		if err := decodeJSONBody(rr, req, &body); err == nil {
			t.Fatalf("expected trailing payload rejection")
		}
	})
}

func TestSaveAndLoadConfigYaml(t *testing.T) {
	tempRoot := t.TempDir()
	configPath := filepath.Join(tempRoot, "config", "config.yaml")
	baseDir := filepath.Join(tempRoot, "base")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("failed to create base dir: %v", err)
	}

	loaded, err := loadOrInitConfig(configPath, baseDir)
	if err != nil {
		t.Fatalf("failed to init config: %v", err)
	}
	if loaded.BaseDir != baseDir {
		t.Fatalf("base dir mismatch: got %q, want %q", loaded.BaseDir, baseDir)
	}
	if loaded.VideoOutputDir == "" {
		t.Fatalf("expected non-empty video output dir")
	}
	if stat, err := os.Stat(loaded.VideoOutputDir); err != nil || !stat.IsDir() {
		t.Fatalf("expected output directory to exist, err=%v", err)
	}

	loaded.ShowHiddenItems = true
	loaded.LogLevel = "debug"
	loaded.TaskPollIntervalMs = 900
	if err := saveConfig(configPath, loaded); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	reloaded, err := loadOrInitConfig(configPath, "")
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if !reloaded.ShowHiddenItems {
		t.Fatalf("expected ShowHiddenItems=true after reload")
	}
	if reloaded.LogLevel != "debug" {
		t.Fatalf("expected log level debug, got %q", reloaded.LogLevel)
	}
	if reloaded.TaskPollIntervalMs != 900 {
		t.Fatalf("expected poll interval 900, got %d", reloaded.TaskPollIntervalMs)
	}
}

func TestLoadConfigRejectsInvalidYaml(t *testing.T) {
	tempRoot := t.TempDir()
	configPath := filepath.Join(tempRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("baseDir: [invalid"), 0o600); err != nil {
		t.Fatalf("failed to write malformed config: %v", err)
	}

	if _, err := loadOrInitConfig(configPath, ""); err == nil {
		t.Fatalf("expected YAML parse error")
	}
}
