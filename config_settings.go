package main

import (
	"encoding/json"

	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

func defaultConfig() AppConfig {
	baseDir := defaultBaseDir()
	logDir := filepath.Join(filepath.Dir(resolveConfigPath("")), "logs")
	return AppConfig{
		BaseDir:            baseDir,
		VideoOutputDir:     filepath.Join(baseDir, "output"),
		ShowHiddenItems:    false,
		PasswordHash:       "",
		LogDir:             logDir,
		LogLevel:           "info",
		LogRotationHours:   24,
		LogMaxAgeDays:      7,
		TaskPollIntervalMs: 1500,
	}
}

func defaultBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		if runtime.GOOS == "windows" {
			return "D:\\"
		}
		return "/"
	}
	if runtime.GOOS == "windows" {
		videos := filepath.Join(home, "Videos")
		if stat, err := os.Stat(videos); err == nil && stat.IsDir() {
			return videos
		}
	}
	return home
}

func resolveConfigPath(override string) string {
	if override != "" {
		return override
	}

	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			return filepath.Join(appData, "video-list", "config.json")
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "video-list-config.json")
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Roaming", "video-list", "config.json")
	}
	return filepath.Join(home, ".video-list", "config.json")
}

func loadOrInitConfig(path string, baseDirOverride string) (AppConfig, error) {
	cfg := defaultConfig()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return cfg, err
	}

	if content, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(content, &cfg)
	}

	if baseDirOverride != "" {
		cfg.BaseDir = baseDirOverride
	}

	baseDir, err := normalizeAndValidateDir(cfg.BaseDir)
	if err != nil {
		fallbackBaseDir, fallbackErr := normalizeAndValidateDir(defaultBaseDir())
		if fallbackErr != nil {
			return cfg, err
		}
		cfg.BaseDir = fallbackBaseDir
	} else {
		cfg.BaseDir = baseDir
	}

	if cfg.LogDir == "" {
		cfg.LogDir = filepath.Join(filepath.Dir(path), "logs")
	}
	resolvedOutputDir, err := normalizeOutputDir(cfg.BaseDir, cfg.VideoOutputDir)
	if err != nil {
		cfg.VideoOutputDir = filepath.Join(cfg.BaseDir, "output")
	} else {
		cfg.VideoOutputDir = resolvedOutputDir
	}
	if err := os.MkdirAll(cfg.VideoOutputDir, 0755); err != nil {
		return cfg, err
	}
	if cfg.LogRotationHours <= 0 {
		cfg.LogRotationHours = 24
	}
	if cfg.LogMaxAgeDays <= 0 {
		cfg.LogMaxAgeDays = 7
	}
	if cfg.TaskPollIntervalMs < 500 {
		cfg.TaskPollIntervalMs = 1500
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if err := saveConfig(path, cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func saveConfig(path string, cfg AppConfig) error {
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func getConfig() AppConfig {
	configMu.RLock()
	defer configMu.RUnlock()
	return appConfig
}

func setConfig(cfg AppConfig) {
	configMu.Lock()
	defer configMu.Unlock()
	appConfig = cfg
}

func isAuthEnabled() bool {
	return getConfig().PasswordHash != ""
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"success": false,
		"error":   message,
	})
}

func publicConfig(cfg AppConfig) PublicConfig {
	return PublicConfig{
		BaseDir:            cfg.BaseDir,
		VideoOutputDir:     cfg.VideoOutputDir,
		ShowHiddenItems:    cfg.ShowHiddenItems,
		AuthEnabled:        cfg.PasswordHash != "",
		LogDir:             cfg.LogDir,
		LogLevel:           cfg.LogLevel,
		LogRotationHours:   cfg.LogRotationHours,
		LogMaxAgeDays:      cfg.LogMaxAgeDays,
		TaskPollIntervalMs: cfg.TaskPollIntervalMs,
	}
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := getConfig()
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "settings": publicConfig(cfg)})
	case http.MethodPut:
		var req SettingsUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request payload")
			return
		}

		cfg := getConfig()

		if req.BaseDir != nil {
			normalized, err := normalizeAndValidateDir(*req.BaseDir)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			cfg.BaseDir = normalized
			if req.VideoOutputDir == nil {
				cfg.VideoOutputDir = filepath.Join(cfg.BaseDir, "output")
			}
		}
		if req.VideoOutputDir != nil {
			resolvedOutputDir, err := normalizeOutputDir(cfg.BaseDir, *req.VideoOutputDir)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := os.MkdirAll(resolvedOutputDir, 0755); err != nil {
				writeError(w, http.StatusBadRequest, "failed to prepare video output directory")
				return
			}
			cfg.VideoOutputDir = resolvedOutputDir
		}
		if req.ShowHiddenItems != nil {
			cfg.ShowHiddenItems = *req.ShowHiddenItems
		}
		if req.LogDir != nil {
			normalizedLogDir, err := normalizePath(*req.LogDir)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			cfg.LogDir = normalizedLogDir
		}
		if req.LogLevel != nil && *req.LogLevel != "" {
			cfg.LogLevel = strings.ToLower(strings.TrimSpace(*req.LogLevel))
		}
		if req.LogRotationHours != nil && *req.LogRotationHours > 0 {
			cfg.LogRotationHours = *req.LogRotationHours
		}
		if req.LogMaxAgeDays != nil && *req.LogMaxAgeDays > 0 {
			cfg.LogMaxAgeDays = *req.LogMaxAgeDays
		}
		if req.TaskPollIntervalMs != nil && *req.TaskPollIntervalMs >= 500 {
			cfg.TaskPollIntervalMs = *req.TaskPollIntervalMs
		}

		if strings.TrimSpace(req.NewPassword) != "" {
			if len(req.NewPassword) < 6 {
				writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
				return
			}
			if cfg.PasswordHash != "" {
				if err := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(req.CurrentPassword)); err != nil {
					writeError(w, http.StatusUnauthorized, "current password is incorrect")
					return
				}
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to update password")
				return
			}
			cfg.PasswordHash = string(hash)
		}

		if err := saveConfig(appConfigPath, cfg); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save settings")
			return
		}

		setConfig(cfg)
		if err := SetupLogger(LoggingOptions{
			Dir:           cfg.LogDir,
			Level:         cfg.LogLevel,
			RotationHours: cfg.LogRotationHours,
			MaxAgeDays:    cfg.LogMaxAgeDays,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to reconfigure logger")
			return
		}

		LoggerWith(logrus.Fields{"request_id": requestIDFromContext(r.Context())}).Info("settings updated")
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "settings": publicConfig(cfg)})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
