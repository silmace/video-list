package app

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
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

	loaded, err := loadOrInitConfig(configPath)
	if err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	// 默认配置应被初始化并落盘
	if loaded.BaseDir == "" {
		t.Fatalf("expected non-empty base dir")
	}
	if loaded.VideoOutputDir == "" {
		t.Fatalf("expected non-empty video output dir")
	}
	if stat, err := os.Stat(loaded.VideoOutputDir); err != nil || !stat.IsDir() {
		t.Fatalf("expected output directory to exist, err=%v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected config file to be created, err=%v", err)
	}

	loaded.ShowHiddenItems = true
	loaded.LogLevel = "debug"
	loaded.TaskPollIntervalMs = 900
	if err := saveConfig(configPath, loaded); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	reloaded, err := loadOrInitConfig(configPath)
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

	// saveConfig 使用 0600（Windows 文件权限语义不同，跳过）
	if runtime.GOOS != "windows" {
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatalf("failed to stat config file: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("config file permission mismatch: got %o, want 600", got)
		}
	}
}

func TestLoadConfigRejectsInvalidYaml(t *testing.T) {
	tempRoot := t.TempDir()
	configPath := filepath.Join(tempRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("baseDir: [invalid"), 0o600); err != nil {
		t.Fatalf("failed to write malformed config: %v", err)
	}

	if _, err := loadOrInitConfig(configPath); err == nil {
		t.Fatalf("expected YAML parse error")
	}
}
