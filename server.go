package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

//go:embed dist/*
var embeddedFiles embed.FS

type contextKey string

const (
	requestIDKey       contextKey = "request_id"
	tokenTTL                      = 24 * time.Hour
	maxUploadSizeBytes int64      = 1024 << 20
	maxTaskRuntime                = 4 * time.Hour
	maxConcurrentTasks            = 2
)

const (
	TaskStatusPending  = "pending"
	TaskStatusRunning  = "running"
	TaskStatusSuccess  = "success"
	TaskStatusFailed   = "failed"
	TaskStatusCanceled = "canceled"
)

type AppConfig struct {
	BaseDir            string `json:"baseDir"`
	VideoOutputDir     string `json:"videoOutputDir"`
	PasswordHash       string `json:"passwordHash,omitempty"`
	LogDir             string `json:"logDir"`
	LogLevel           string `json:"logLevel"`
	LogRotationHours   int    `json:"logRotationHours"`
	LogMaxAgeDays      int    `json:"logMaxAgeDays"`
	TaskPollIntervalMs int    `json:"taskPollIntervalMs"`
}

type PublicConfig struct {
	BaseDir            string `json:"baseDir"`
	VideoOutputDir     string `json:"videoOutputDir"`
	AuthEnabled        bool   `json:"authEnabled"`
	LogDir             string `json:"logDir"`
	LogLevel           string `json:"logLevel"`
	LogRotationHours   int    `json:"logRotationHours"`
	LogMaxAgeDays      int    `json:"logMaxAgeDays"`
	TaskPollIntervalMs int    `json:"taskPollIntervalMs"`
}

type SettingsUpdateRequest struct {
	BaseDir            *string `json:"baseDir"`
	VideoOutputDir     *string `json:"videoOutputDir"`
	LogDir             *string `json:"logDir"`
	LogLevel           *string `json:"logLevel"`
	LogRotationHours   *int    `json:"logRotationHours"`
	LogMaxAgeDays      *int    `json:"logMaxAgeDays"`
	TaskPollIntervalMs *int    `json:"taskPollIntervalMs"`
	CurrentPassword    string  `json:"currentPassword"`
	NewPassword        string  `json:"newPassword"`
}

type LoginRequest struct {
	Password string `json:"password"`
}

type AuthStatusResponse struct {
	AuthEnabled        bool `json:"authEnabled"`
	Authenticated      bool `json:"authenticated"`
	TaskPollIntervalMs int  `json:"taskPollIntervalMs"`
}

type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	IsDirectory  bool      `json:"isDirectory"`
	Size         int64     `json:"size"`
	ModifiedTime time.Time `json:"modifiedTime"`
}

type Segment struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type VideoEditRequest struct {
	VideoPath  string    `json:"videoPath"`
	Segments   []Segment `json:"segments"`
	ExportMode string    `json:"exportMode,omitempty"`
	VideoCodec string    `json:"videoCodec,omitempty"`
}

type BatchDeleteRequest struct {
	Paths []string `json:"paths"`
}

type BatchMoveRequest struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination"`
}

type BatchCopyRequest struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination"`
}

type CreateFolderRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type RenameFileRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type VideoCodecOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Container   string `json:"container"`
	Mode        string `json:"mode"`
	Available   bool   `json:"available"`
}

type Task struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Progress    int       `json:"progress"`
	Stage       string    `json:"stage"`
	Message     string    `json:"message"`
	Error       string    `json:"error,omitempty"`
	Total       int       `json:"total,omitempty"`
	Current     int       `json:"current,omitempty"`
	CurrentItem string    `json:"currentItem,omitempty"`
	Detail      string    `json:"detail,omitempty"`
	OutputPath  string    `json:"outputPath,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type TaskRecord struct {
	Task   Task
	Cancel context.CancelFunc
}

type TaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*TaskRecord
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

var (
	configMu      sync.RWMutex
	appConfig     AppConfig
	appConfigPath string

	sessionMu sync.RWMutex
	sessions  = make(map[string]time.Time)

	taskManager = NewTaskManager()
	taskSlots   = make(chan struct{}, maxConcurrentTasks)
)

func NewTaskManager() *TaskManager {
	return &TaskManager{tasks: make(map[string]*TaskRecord)}
}

func (m *TaskManager) Create(taskType string, message string) Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	t := Task{
		ID:        generateID("task"),
		Type:      taskType,
		Status:    TaskStatusPending,
		Progress:  0,
		Stage:     "pending",
		Message:   message,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.tasks[t.ID] = &TaskRecord{Task: t}
	return t
}

func (m *TaskManager) SetCancel(taskID string, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.tasks[taskID]; ok {
		rec.Cancel = cancel
	}
}

func (m *TaskManager) Update(taskID string, updater func(t *Task)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rec, ok := m.tasks[taskID]; ok {
		updater(&rec.Task)
		rec.Task.UpdatedAt = time.Now()
	}
}

func (m *TaskManager) Complete(taskID string, message string, outputPath string) {
	m.Update(taskID, func(t *Task) {
		t.Status = TaskStatusSuccess
		t.Progress = 100
		t.Stage = "completed"
		t.Message = message
		if t.Total > 0 {
			t.Current = t.Total
		}
		t.CurrentItem = ""
		t.Detail = ""
		t.OutputPath = outputPath
		t.Error = ""
	})
}

func (m *TaskManager) Fail(taskID string, err error) {
	m.Update(taskID, func(t *Task) {
		t.Status = TaskStatusFailed
		t.Stage = "failed"
		t.Message = "task failed"
		if err != nil {
			t.Error = err.Error()
			t.Detail = err.Error()
		}
	})
}

func (m *TaskManager) MarkCanceled(taskID string) {
	m.Update(taskID, func(t *Task) {
		t.Status = TaskStatusCanceled
		t.Stage = "canceled"
		t.Message = "task canceled"
		t.CurrentItem = ""
	})
}

func (m *TaskManager) Cancel(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec, ok := m.tasks[taskID]
	if !ok {
		return false
	}
	if rec.Task.Status == TaskStatusSuccess || rec.Task.Status == TaskStatusFailed || rec.Task.Status == TaskStatusCanceled {
		return false
	}
	if rec.Cancel != nil {
		rec.Cancel()
		rec.Cancel = nil
	}
	rec.Task.Status = TaskStatusCanceled
	rec.Task.Stage = "canceled"
	rec.Task.Message = "task canceled"
	rec.Task.CurrentItem = ""
	rec.Task.UpdatedAt = time.Now()
	return true
}

func (m *TaskManager) Get(taskID string) (Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.tasks[taskID]
	if !ok {
		return Task{}, false
	}
	return rec.Task, true
}

func (m *TaskManager) List() []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]Task, 0, len(m.tasks))
	for _, rec := range m.tasks {
		items = append(items, rec.Task)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func main() {
	var baseDirOverride string
	var cfgPathOverride string

	flag.StringVar(&baseDirOverride, "baseDir", "", "Base directory override")
	flag.StringVar(&cfgPathOverride, "config", "", "Config file path")
	flag.Parse()

	appConfigPath = resolveConfigPath(cfgPathOverride)
	cfg, err := loadOrInitConfig(appConfigPath, baseDirOverride)
	if err != nil {
		panic(err)
	}
	setConfig(cfg)

	if err := SetupLogger(LoggingOptions{
		Dir:           cfg.LogDir,
		Level:         cfg.LogLevel,
		RotationHours: cfg.LogRotationHours,
		MaxAgeDays:    cfg.LogMaxAgeDays,
	}); err != nil {
		panic(err)
	}

	AppLogger.WithFields(logrus.Fields{
		"config_path": appConfigPath,
		"base_dir":    cfg.BaseDir,
		"log_dir":     cfg.LogDir,
	}).Info("server startup")

	mux := http.NewServeMux()

	mux.HandleFunc("/api/auth/login", handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", handleAuthLogout)
	mux.HandleFunc("/api/auth/status", handleAuthStatus)

	mux.HandleFunc("/api/settings", withAuth(handleSettings))
	mux.HandleFunc("/api/files", withAuth(handleFiles))
	mux.HandleFunc("/api/files/mkdir", withAuth(handleCreateFolder))
	mux.HandleFunc("/api/files/rename", withAuth(handleRenameFile))
	mux.HandleFunc("/api/files/upload", withAuth(handleUploadFile))
	mux.HandleFunc("/api/media", withAuth(handleMediaStream))
	mux.HandleFunc("/api/edit-video", withAuth(handleEditVideo))
	mux.HandleFunc("/api/video/options", withAuth(handleVideoOptions))

	mux.HandleFunc("/api/tasks", withAuth(handleTaskList))
	mux.HandleFunc("/api/tasks/", withAuth(handleTaskByID))
	mux.HandleFunc("/api/tasks/video", withAuth(handleCreateVideoTask))
	mux.HandleFunc("/api/tasks/batch-delete", withAuth(handleCreateBatchDeleteTask))
	mux.HandleFunc("/api/tasks/batch-move", withAuth(handleCreateBatchMoveTask))
	mux.HandleFunc("/api/tasks/batch-copy", withAuth(handleCreateBatchCopyTask))

	distFS, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		panic(err)
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		path = strings.TrimPrefix(path, "/")

		if content, err := fs.ReadFile(distFS, path); err == nil {
			ext := filepath.Ext(path)
			contentType := mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			w.Header().Set("Content-Type", contentType)
			_, _ = w.Write(content)
			return
		}

		content, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(content)
	})

	handler := requestIDMiddleware(loggingMiddleware(mux))
	fmt.Println("Server running on http://localhost:3001")
	if err := http.ListenAndServe(":3001", handler); err != nil {
		panic(err)
	}
}

func defaultConfig() AppConfig {
	baseDir := defaultBaseDir()
	logDir := filepath.Join(filepath.Dir(resolveConfigPath("")), "logs")
	return AppConfig{
		BaseDir:            baseDir,
		VideoOutputDir:     filepath.Join(baseDir, "output"),
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

func handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg := getConfig()
	if cfg.PasswordHash == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"success":        true,
			"token":          "",
			"passwordNeeded": false,
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cfg.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	token := createSessionToken()
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "token": token, "passwordNeeded": true})
}

func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	token := extractBearerToken(r)
	if token != "" {
		removeSession(token)
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cfg := getConfig()
	authEnabled := cfg.PasswordHash != ""
	authenticated := !authEnabled
	if authEnabled {
		authenticated = validateSessionToken(extractBearerToken(r))
	}

	writeJSON(w, http.StatusOK, AuthStatusResponse{
		AuthEnabled:        authEnabled,
		Authenticated:      authenticated,
		TaskPollIntervalMs: cfg.TaskPollIntervalMs,
	})
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthEnabled() {
			next(w, r)
			return
		}

		token := extractAuthToken(r)
		if token == "" || !validateSessionToken(token) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next(w, r)
	}
}

func handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listFiles(w, r)
	case http.MethodDelete:
		deleteFile(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	folderName := strings.TrimSpace(req.Name)
	if folderName == "" {
		writeError(w, http.StatusBadRequest, "folder name is required")
		return
	}
	folderName, err := sanitizeFileName(folderName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	parentPath := req.Path
	if strings.TrimSpace(parentPath) == "" {
		parentPath = "/"
	}

	parentAbsPath, err := toAbsolutePath(parentPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid parent path")
		return
	}

	targetPath := filepath.Clean(filepath.Join(parentAbsPath, folderName))
	if !isPathWithinBase(targetPath) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if err := ensureSafePath(targetPath, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := os.Mkdir(targetPath, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create folder")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"path":    toRelativePath(targetPath),
	})
}

func handleUploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSizeBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	relPath := strings.TrimSpace(r.FormValue("path"))
	if relPath == "" {
		relPath = "/"
	}

	destinationDir, err := toAbsolutePath(relPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid upload path")
		return
	}

	src, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer src.Close()

	fileName, err := sanitizeFileName(filepath.Base(header.Filename))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if fileName == "" || fileName == "." {
		writeError(w, http.StatusBadRequest, "file name is invalid")
		return
	}

	targetPath := filepath.Clean(filepath.Join(destinationDir, fileName))
	if !isPathWithinBase(targetPath) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if err := ensureSafePath(targetPath, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	overwrite := strings.TrimSpace(r.FormValue("overwrite")) == "1"
	if _, err := os.Stat(targetPath); err == nil && !overwrite {
		writeError(w, http.StatusConflict, "file already exists")
		return
	}

	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create file")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save uploaded file")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"path":    toRelativePath(targetPath),
	})
}

func handleRenameFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RenameFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if strings.TrimSpace(req.Path) == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	newName := strings.TrimSpace(req.Name)
	if newName == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	newName, err := sanitizeFileName(newName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	absPath, err := toAbsolutePath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if err := ensureSafePath(absPath, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to access file")
		return
	}

	targetPath := filepath.Clean(filepath.Join(filepath.Dir(absPath), newName))
	if !isPathWithinBase(targetPath) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	if err := ensureSafePath(targetPath, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if absPath == targetPath {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "path": toRelativePath(absPath)})
		return
	}

	if _, err := os.Stat(targetPath); err == nil {
		writeError(w, http.StatusConflict, "target already exists")
		return
	} else if !os.IsNotExist(err) {
		writeError(w, http.StatusInternalServerError, "failed to validate target")
		return
	}

	if err := os.Rename(absPath, targetPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"path":    toRelativePath(targetPath),
	})
}

func listFiles(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		relPath = "/"
	}

	absPath, err := toAbsolutePath(relPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read directory")
		return
	}

	fileList := make([]FileInfo, 0, len(entries))
	searchTerm := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	sortBy := strings.TrimSpace(r.URL.Query().Get("sortBy"))
	if sortBy == "" {
		sortBy = "name"
	}
	order := strings.TrimSpace(r.URL.Query().Get("order"))
	if order != "desc" {
		order = "asc"
	}
	typeFilter := strings.TrimSpace(r.URL.Query().Get("type"))
	includeHidden := r.URL.Query().Get("includeHidden") == "1"

	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !includeHidden && isHiddenName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		fileType := inferFileCategory(entry.Name(), entry.IsDir())
		if searchTerm != "" && !strings.Contains(strings.ToLower(entry.Name()), searchTerm) {
			continue
		}
		if typeFilter != "" && typeFilter != "all" && typeFilter != fileType {
			continue
		}
		fullPath := filepath.Join(absPath, entry.Name())
		fileList = append(fileList, FileInfo{
			Name:         entry.Name(),
			Path:         toRelativePath(fullPath),
			IsDirectory:  entry.IsDir(),
			Size:         info.Size(),
			ModifiedTime: info.ModTime(),
		})
	}

	sort.Slice(fileList, func(i, j int) bool {
		left := fileList[i]
		right := fileList[j]
		if left.IsDirectory != right.IsDirectory {
			if order == "desc" {
				return !left.IsDirectory
			}
			return left.IsDirectory
		}
		var less bool
		switch sortBy {
		case "size":
			less = left.Size < right.Size
		case "modified":
			less = left.ModifiedTime.Before(right.ModifiedTime)
		default:
			less = strings.ToLower(left.Name) < strings.ToLower(right.Name)
		}
		if order == "desc" {
			return !less
		}
		return less
	})

	writeJSON(w, http.StatusOK, fileList)
}

func deleteFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, err := toAbsolutePath(relPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if err := ensureSafePath(absPath, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if info.IsDir() {
		err = os.RemoveAll(absPath)
	} else {
		err = os.Remove(absPath)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete file")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func handleMediaStream(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, err := toAbsolutePath(relPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "media not found")
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read media metadata")
		return
	}

	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(absPath)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, filepath.Base(absPath), stat.ModTime(), file)
}

func handleEditVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := ensureFFmpegAvailable(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req VideoEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), maxTaskRuntime)
	defer cancel()
	outputPath, err := processVideo(ctx, req, "sync", nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "output": outputPath})
}

func handleVideoOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"codecs":  detectAvailableVideoCodecs(),
	})
}

func handleCreateVideoTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := ensureFFmpegAvailable(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var req VideoEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if len(req.Segments) == 0 {
		writeError(w, http.StatusBadRequest, "segments are required")
		return
	}
	if _, err := resolveVideoCodecOption(req.ExportMode, req.VideoCodec); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task := taskManager.Create("video-edit", "video task created")
	ctx, cancel := context.WithTimeout(context.Background(), maxTaskRuntime)
	taskManager.SetCancel(task.ID, cancel)

	go runVideoTask(ctx, task.ID, req)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "taskId": task.ID})
}

func runVideoTask(ctx context.Context, taskID string, req VideoEditRequest) {
	release, err := acquireTaskSlot(ctx)
	if err != nil {
		taskManager.Fail(taskID, err)
		return
	}
	defer release()

	updateProgress := func(stage string, progress int, message string) {
		taskManager.Update(taskID, func(t *Task) {
			t.Status = TaskStatusRunning
			t.Stage = stage
			t.Progress = progress
			t.Message = message
			t.Total = len(req.Segments)
		})
	}

	outputPath, err := processVideo(ctx, req, taskID, updateProgress)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			taskManager.MarkCanceled(taskID)
			return
		}
		taskManager.Fail(taskID, err)
		return
	}

	taskManager.Complete(taskID, "video task completed", outputPath)
}

func processVideo(ctx context.Context, req VideoEditRequest, taskID string, progress func(stage string, progress int, message string)) (string, error) {
	absPath, err := toAbsolutePath(req.VideoPath)
	if err != nil {
		return "", errors.New("invalid video path")
	}
	if err := ensureSafePath(absPath, false); err != nil {
		return "", err
	}
	outputDir := getConfig().VideoOutputDir
	if outputDir == "" {
		outputDir = filepath.Join(getConfig().BaseDir, "output")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", errors.New("failed to prepare video output directory")
	}

	if len(req.Segments) == 0 {
		return "", errors.New("segments are required")
	}

	codecOption, err := resolveVideoCodecOption(req.ExportMode, req.VideoCodec)
	if err != nil {
		return "", err
	}

	report := func(stage string, percent int, message string) {
		if progress != nil {
			progress(stage, percent, message)
		}
	}

	report("preparing", 5, "preparing ffmpeg task")

	if len(req.Segments) == 1 {
		segment := req.Segments[0]
		duration, err := getTimeDifference(segment.StartTime, segment.EndTime)
		if err != nil {
			return "", err
		}

		baseName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_export.%s", baseName, codecOption.Container))
		report("processing", 30, "processing segment")
		if err := processSegment(ctx, absPath, outputPath, segment.StartTime, duration, codecOption); err != nil {
			return "", err
		}
		report("finalizing", 90, "finalizing output")
		return toRelativePath(outputPath), nil
	}

	tempDir := filepath.Join(outputDir, ".temp", taskID)
	if taskID == "sync" {
		tempDir = filepath.Join(outputDir, ".temp", generateID("temp"))
	}
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", err
	}
	defer cleanupFiles(tempDir)

	segmentFiles := make([]string, 0, len(req.Segments))
	for i, segment := range req.Segments {
		select {
		case <-ctx.Done():
			return "", context.Canceled
		default:
		}

		duration, err := getTimeDifference(segment.StartTime, segment.EndTime)
		if err != nil {
			return "", err
		}

		outputPath := filepath.Join(tempDir, fmt.Sprintf("segment_%d.mp4", i))
		if codecOption.Container == "mkv" {
			outputPath = filepath.Join(tempDir, fmt.Sprintf("segment_%d.mkv", i))
		}
		progressBase := 10 + int((float64(i)/float64(len(req.Segments)))*60)
		report("extracting", progressBase, fmt.Sprintf("extracting segment %d/%d", i+1, len(req.Segments)))

		if err := processSegment(ctx, absPath, outputPath, segment.StartTime, duration, codecOption); err != nil {
			return "", err
		}
		segmentFiles = append(segmentFiles, outputPath)
	}

	baseName := strings.TrimSuffix(filepath.Base(absPath), filepath.Ext(absPath))
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%s_merged.%s", baseName, codecOption.Container))
	report("merging", 80, "merging segments")
	if err := mergeSegments(ctx, segmentFiles, outputPath, codecOption); err != nil {
		return "", err
	}
	report("finalizing", 95, "finalizing output")
	return toRelativePath(outputPath), nil
}

func handleCreateBatchDeleteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req BatchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths are required")
		return
	}

	task := taskManager.Create("batch-delete", fmt.Sprintf("deleting %d items", len(req.Paths)))
	ctx, cancel := context.WithTimeout(context.Background(), maxTaskRuntime)
	taskManager.SetCancel(task.ID, cancel)

	go runBatchDeleteTask(ctx, task.ID, req.Paths)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "taskId": task.ID})
}

func runBatchDeleteTask(ctx context.Context, taskID string, paths []string) {
	release, err := acquireTaskSlot(ctx)
	if err != nil {
		taskManager.Fail(taskID, err)
		return
	}
	defer release()

	total := len(paths)
	taskManager.Update(taskID, func(t *Task) {
		t.Status = TaskStatusRunning
		t.Stage = "deleting"
		t.Progress = 5
		t.Message = "starting batch delete"
		t.Total = total
		t.Current = 0
		t.CurrentItem = ""
	})

	for i, path := range paths {
		select {
		case <-ctx.Done():
			taskManager.MarkCanceled(taskID)
			return
		default:
		}

		absPath, err := toAbsolutePath(path)
		if err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := ensureSafePath(absPath, false); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		if err := os.RemoveAll(absPath); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		progress := 10 + int((float64(i+1)/float64(total))*90)
		currentItem := filepath.Base(absPath)
		taskManager.Update(taskID, func(t *Task) {
			t.Progress = progress
			t.Message = fmt.Sprintf("deleted %d/%d", i+1, total)
			t.Current = i + 1
			t.CurrentItem = currentItem
		})
	}

	taskManager.Complete(taskID, "batch delete completed", "")
}

func handleCreateBatchMoveTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req BatchMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths are required")
		return
	}
	if strings.TrimSpace(req.Destination) == "" {
		writeError(w, http.StatusBadRequest, "destination is required")
		return
	}

	destination, err := toAbsolutePath(req.Destination)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination path")
		return
	}

	task := taskManager.Create("batch-move", fmt.Sprintf("moving %d items", len(req.Paths)))
	ctx, cancel := context.WithTimeout(context.Background(), maxTaskRuntime)
	taskManager.SetCancel(task.ID, cancel)

	go runBatchMoveTask(ctx, task.ID, req.Paths, destination)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "taskId": task.ID})
}

func runBatchMoveTask(ctx context.Context, taskID string, paths []string, destination string) {
	release, err := acquireTaskSlot(ctx)
	if err != nil {
		taskManager.Fail(taskID, err)
		return
	}
	defer release()

	total := len(paths)
	taskManager.Update(taskID, func(t *Task) {
		t.Status = TaskStatusRunning
		t.Stage = "moving"
		t.Progress = 5
		t.Message = "starting batch move"
		t.Total = total
		t.Current = 0
		t.CurrentItem = ""
	})

	for i, path := range paths {
		select {
		case <-ctx.Done():
			taskManager.MarkCanceled(taskID)
			return
		default:
		}

		absPath, err := toAbsolutePath(path)
		if err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := ensureSafePath(absPath, false); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		targetPath := filepath.Join(destination, filepath.Base(absPath))
		if err := ensureSafePath(targetPath, true); err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := movePath(absPath, targetPath); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		progress := 10 + int((float64(i+1)/float64(total))*90)
		currentItem := filepath.Base(absPath)
		taskManager.Update(taskID, func(t *Task) {
			t.Progress = progress
			t.Message = fmt.Sprintf("moved %d/%d", i+1, total)
			t.Current = i + 1
			t.CurrentItem = currentItem
		})
	}

	taskManager.Complete(taskID, "batch move completed", toRelativePath(destination))
}

func handleCreateBatchCopyTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req BatchCopyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths are required")
		return
	}
	if strings.TrimSpace(req.Destination) == "" {
		writeError(w, http.StatusBadRequest, "destination is required")
		return
	}

	destination, err := toAbsolutePath(req.Destination)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination path")
		return
	}
	if err := ensureSafePath(destination, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task := taskManager.Create("batch-copy", fmt.Sprintf("copying %d items", len(req.Paths)))
	ctx, cancel := context.WithTimeout(context.Background(), maxTaskRuntime)
	taskManager.SetCancel(task.ID, cancel)

	go runBatchCopyTask(ctx, task.ID, req.Paths, destination)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "taskId": task.ID})
}

func runBatchCopyTask(ctx context.Context, taskID string, paths []string, destination string) {
	release, err := acquireTaskSlot(ctx)
	if err != nil {
		taskManager.Fail(taskID, err)
		return
	}
	defer release()

	total := len(paths)
	taskManager.Update(taskID, func(t *Task) {
		t.Status = TaskStatusRunning
		t.Stage = "copying"
		t.Progress = 5
		t.Message = "starting batch copy"
		t.Total = total
		t.Current = 0
		t.CurrentItem = ""
	})

	for i, rawPath := range paths {
		select {
		case <-ctx.Done():
			taskManager.MarkCanceled(taskID)
			return
		default:
		}

		absPath, err := toAbsolutePath(rawPath)
		if err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := ensureSafePath(absPath, false); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		targetPath := filepath.Join(destination, filepath.Base(absPath))
		if err := ensureSafePath(targetPath, true); err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if _, err := os.Stat(targetPath); err == nil {
			taskManager.Fail(taskID, fmt.Errorf("target already exists: %s", filepath.Base(targetPath)))
			return
		}
		if err := copyPath(absPath, targetPath); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		progress := 10 + int((float64(i+1)/float64(total))*90)
		currentItem := filepath.Base(absPath)
		taskManager.Update(taskID, func(t *Task) {
			t.Progress = progress
			t.Message = fmt.Sprintf("copied %d/%d", i+1, total)
			t.Current = i + 1
			t.CurrentItem = currentItem
		})
	}

	taskManager.Complete(taskID, "batch copy completed", toRelativePath(destination))
}

func handleTaskList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "tasks": taskManager.List()})
}

func handleTaskByID(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, ok := taskManager.Get(taskID)
		if !ok {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "task": task})
	case http.MethodDelete:
		if !taskManager.Cancel(taskID) {
			writeError(w, http.StatusBadRequest, "task cannot be canceled")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func getTimeDifference(start, end string) (string, error) {
	startParts := strings.Split(start, ":")
	endParts := strings.Split(end, ":")

	if len(startParts) != 3 || len(endParts) != 3 {
		return "", errors.New("invalid time format, expected HH:MM:SS")
	}

	startHours, err := strconv.Atoi(startParts[0])
	if err != nil {
		return "", err
	}
	startMinutes, err := strconv.Atoi(startParts[1])
	if err != nil {
		return "", err
	}
	startSeconds, err := strconv.Atoi(startParts[2])
	if err != nil {
		return "", err
	}

	endHours, err := strconv.Atoi(endParts[0])
	if err != nil {
		return "", err
	}
	endMinutes, err := strconv.Atoi(endParts[1])
	if err != nil {
		return "", err
	}
	endSeconds, err := strconv.Atoi(endParts[2])
	if err != nil {
		return "", err
	}

	startTotalSeconds := startHours*3600 + startMinutes*60 + startSeconds
	endTotalSeconds := endHours*3600 + endMinutes*60 + endSeconds

	if endTotalSeconds <= startTotalSeconds {
		return "", errors.New("end time must be greater than start time")
	}

	return strconv.Itoa(endTotalSeconds - startTotalSeconds), nil
}

func processSegment(ctx context.Context, inputPath string, outputPath string, startTime string, duration string, codecOption VideoCodecOption) error {
	args := []string{"-y", "-i", inputPath, "-ss", startTime, "-t", duration}
	args = append(args, buildCodecArgs(codecOption)...)
	args = append(args, outputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("ffmpeg not found in PATH. Please install ffmpeg and ensure it is available in system PATH")
		}
		return fmt.Errorf("ffmpeg segment failed: %w, %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func mergeSegments(ctx context.Context, segmentFiles []string, outputPath string, codecOption VideoCodecOption) error {
	if len(segmentFiles) == 0 {
		return errors.New("no segment files to merge")
	}

	concatFile := filepath.Join(filepath.Dir(segmentFiles[0]), "concat.txt")
	var builder strings.Builder
	for _, file := range segmentFiles {
		builder.WriteString(fmt.Sprintf("file '%s'\n", strings.ReplaceAll(file, "'", "'\\''")))
	}

	if err := os.WriteFile(concatFile, []byte(builder.String()), 0644); err != nil {
		return err
	}

	args := []string{"-y", "-f", "concat", "-safe", "0", "-i", concatFile}
	args = append(args, buildMergeCodecArgs(codecOption)...)
	args = append(args, outputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return context.Canceled
		}
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("ffmpeg not found in PATH. Please install ffmpeg and ensure it is available in system PATH")
		}
		return fmt.Errorf("ffmpeg merge failed: %w, %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ensureFFmpegAvailable() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errors.New("ffmpeg not found in PATH. Please install ffmpeg and ensure it is available in system PATH")
	}
	return nil
}

func cleanupFiles(path string) {
	if path == "" {
		return
	}
	_ = os.RemoveAll(path)
}

func toRelativePath(path string) string {
	baseDir := getConfig().BaseDir
	rel, err := filepath.Rel(baseDir, path)
	if err != nil || rel == "." {
		return "/"
	}
	return "/" + filepath.ToSlash(rel)
}

func toAbsolutePath(inputPath string) (string, error) {
	baseDir := getConfig().BaseDir
	if inputPath == "" || inputPath == "/" {
		return baseDir, nil
	}

	cleaned, err := cleanRelativeInput(inputPath)
	if err != nil {
		return "", errors.New("invalid path")
	}
	absPath := filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(cleaned)))
	if !isPathWithinBase(absPath) {
		return "", errors.New("invalid path")
	}
	if err := ensureSafePath(absPath, true); err != nil {
		return "", err
	}
	return absPath, nil
}

func cleanRelativeInput(raw string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if strings.Contains(trimmed, "\x00") {
		return "", errors.New("invalid path")
	}
	if trimmed == "" || trimmed == "/" {
		return "", nil
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(trimmed, "/"))
	if cleaned == "/" {
		return "", nil
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func normalizePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("path cannot be empty")
	}
	if !filepath.IsAbs(trimmed) {
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return "", err
		}
		trimmed = abs
	}
	return filepath.Clean(trimmed), nil
}

func normalizeAndValidateDir(path string) (string, error) {
	normalized, err := normalizePath(path)
	if err != nil {
		return "", err
	}
	stat, err := os.Stat(normalized)
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %s", normalized)
	}
	if !stat.IsDir() {
		return "", errors.New("baseDir must be a directory")
	}
	realPath, err := filepath.EvalSymlinks(normalized)
	if err != nil {
		return normalized, nil
	}
	return realPath, nil
}

func normalizeOutputDir(baseDir string, outputDir string) (string, error) {
	trimmed := strings.TrimSpace(outputDir)
	if trimmed == "" {
		trimmed = filepath.Join(baseDir, "output")
	}

	if !filepath.IsAbs(trimmed) {
		trimmed = filepath.Join(baseDir, trimmed)
	}

	normalized, err := normalizePath(trimmed)
	if err != nil {
		return "", err
	}
	if !isPathWithinBase(normalized) {
		return "", errors.New("video output directory must be within base directory")
	}
	return normalized, nil
}

func isPathWithinBase(path string) bool {
	baseDir := getConfig().BaseDir
	cleanPath := filepath.Clean(path)
	rel, err := filepath.Rel(baseDir, cleanPath)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func ensureSafePath(targetPath string, allowMissing bool) error {
	baseDir := getConfig().BaseDir
	rel, err := filepath.Rel(baseDir, filepath.Clean(targetPath))
	if err != nil {
		return errors.New("invalid path")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("invalid path")
	}
	if rel == "." {
		return nil
	}
	current := baseDir
	for _, segment := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) && allowMissing {
				return nil
			}
			if os.IsNotExist(err) {
				return errors.New("path does not exist")
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink paths are not allowed")
		}
	}
	return nil
}

func sanitizeFileName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || name == "." || name == ".." {
		return "", errors.New("file name is invalid")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.ContainsRune(name, '\x00') {
		return "", errors.New("file name is invalid")
	}
	reserved := map[string]struct{}{
		"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
		"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
		"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
	}
	baseName := strings.TrimSuffix(name, filepath.Ext(name))
	if _, exists := reserved[strings.ToUpper(baseName)]; exists {
		return "", errors.New("file name is reserved on Windows")
	}
	return name, nil
}

func inferFileCategory(name string, isDir bool) string {
	if isDir {
		return "folder"
	}
	lowerName := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lowerName, ".mp4") || strings.HasSuffix(lowerName, ".mov") || strings.HasSuffix(lowerName, ".mkv") || strings.HasSuffix(lowerName, ".avi") || strings.HasSuffix(lowerName, ".webm") || strings.HasSuffix(lowerName, ".m4v"):
		return "video"
	case strings.HasSuffix(lowerName, ".png") || strings.HasSuffix(lowerName, ".jpg") || strings.HasSuffix(lowerName, ".jpeg") || strings.HasSuffix(lowerName, ".gif") || strings.HasSuffix(lowerName, ".bmp") || strings.HasSuffix(lowerName, ".webp") || strings.HasSuffix(lowerName, ".svg"):
		return "image"
	case strings.HasSuffix(lowerName, ".mp3") || strings.HasSuffix(lowerName, ".wav") || strings.HasSuffix(lowerName, ".flac") || strings.HasSuffix(lowerName, ".aac") || strings.HasSuffix(lowerName, ".ogg"):
		return "audio"
	case strings.HasSuffix(lowerName, ".zip") || strings.HasSuffix(lowerName, ".rar") || strings.HasSuffix(lowerName, ".7z") || strings.HasSuffix(lowerName, ".tar") || strings.HasSuffix(lowerName, ".gz"):
		return "archive"
	case strings.HasSuffix(lowerName, ".pdf") || strings.HasSuffix(lowerName, ".doc") || strings.HasSuffix(lowerName, ".docx") || strings.HasSuffix(lowerName, ".txt") || strings.HasSuffix(lowerName, ".md"):
		return "document"
	case strings.HasSuffix(lowerName, ".go") || strings.HasSuffix(lowerName, ".ts") || strings.HasSuffix(lowerName, ".tsx") || strings.HasSuffix(lowerName, ".js") || strings.HasSuffix(lowerName, ".jsx") || strings.HasSuffix(lowerName, ".vue") || strings.HasSuffix(lowerName, ".json") || strings.HasSuffix(lowerName, ".yaml") || strings.HasSuffix(lowerName, ".yml"):
		return "code"
	default:
		return "other"
	}
}

func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}

func createSessionToken() string {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)

	sessionMu.Lock()
	sessions[token] = time.Now().Add(tokenTTL)
	sessionMu.Unlock()
	return token
}

func validateSessionToken(token string) bool {
	if token == "" {
		return false
	}

	sessionMu.Lock()
	defer sessionMu.Unlock()

	expiresAt, ok := sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiresAt) {
		delete(sessions, token)
		return false
	}
	return true
}

func removeSession(token string) {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	delete(sessions, token)
}

func extractBearerToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func extractAuthToken(r *http.Request) string {
	if token := extractBearerToken(r); token != "" {
		return token
	}

	// Media requests from <img>/<video> cannot attach Authorization headers reliably.
	if r.Method == http.MethodGet && r.URL.Path == "/api/media" {
		return strings.TrimSpace(r.URL.Query().Get("token"))
	}

	return ""
}

func generateID(prefix string) string {
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UnixNano(), hex.EncodeToString(raw))
}

func requestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = generateID("req")
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		durationMs := time.Since(start).Milliseconds()
		LoggerWith(logrus.Fields{
			"request_id":  requestIDFromContext(r.Context()),
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      recorder.status,
			"duration_ms": durationMs,
			"remote_addr": r.RemoteAddr,
		}).Info("request completed")
	})
}

func movePath(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := copyPath(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyPath(src string, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlink copy is not allowed")
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			srcChild := filepath.Join(src, entry.Name())
			dstChild := filepath.Join(dst, entry.Name())
			if err := copyPath(srcChild, dstChild); err != nil {
				return err
			}
		}
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

func acquireTaskSlot(ctx context.Context) (func(), error) {
	select {
	case taskSlots <- struct{}{}:
		return func() {
			<-taskSlots
		}, nil
	case <-ctx.Done():
		return nil, context.Canceled
	}
}

func detectAvailableVideoCodecs() []VideoCodecOption {
	encoders := readFFmpegEncoders()
	options := []VideoCodecOption{
		{ID: "copy", Label: "Direct Stream Copy", Description: "Fastest, keeps original codec when possible", Container: "mp4", Mode: "copy", Available: true},
		{ID: "h264", Label: "H.264", Description: "Balanced compatibility and speed", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "h265", Label: "H.265 / HEVC", Description: "Smaller output, slower encode", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "av1", Label: "AV1", Description: "Best compression, slowest encode", Container: "mkv", Mode: "transcode", Available: false},
	}
	if encoders == "" {
		return options
	}
	for i := range options {
		switch options[i].ID {
		case "h264":
			options[i].Available = strings.Contains(encoders, "libx264")
		case "h265":
			options[i].Available = strings.Contains(encoders, "libx265")
		case "av1":
			options[i].Available = strings.Contains(encoders, "libsvtav1") || strings.Contains(encoders, "librav1e") || strings.Contains(encoders, "libaom-av1")
		}
	}
	return options
}

func readFFmpegEncoders() string {
	output, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").CombinedOutput()
	if err != nil {
		return ""
	}
	return string(output)
}

func resolveVideoCodecOption(exportMode string, codec string) (VideoCodecOption, error) {
	mode := strings.TrimSpace(exportMode)
	if mode == "" {
		mode = "copy"
	}
	selected := strings.TrimSpace(codec)
	if selected == "" {
		if mode == "transcode" {
			selected = "h264"
		} else {
			selected = "copy"
		}
	}
	for _, option := range detectAvailableVideoCodecs() {
		if option.ID != selected {
			continue
		}
		if option.Mode != mode {
			return VideoCodecOption{}, errors.New("video codec does not match export mode")
		}
		if !option.Available {
			return VideoCodecOption{}, fmt.Errorf("selected codec is not available: %s", option.Label)
		}
		return option, nil
	}
	return VideoCodecOption{}, errors.New("unsupported video codec")
}

func buildCodecArgs(option VideoCodecOption) []string {
	switch option.ID {
	case "h264":
		return []string{"-c:v", "libx264", "-preset", "medium", "-crf", "23", "-c:a", "aac", "-b:a", "192k"}
	case "h265":
		return []string{"-c:v", "libx265", "-preset", "medium", "-crf", "28", "-c:a", "aac", "-b:a", "192k"}
	case "av1":
		codec := "libsvtav1"
		encoders := readFFmpegEncoders()
		switch {
		case strings.Contains(encoders, "libsvtav1"):
			codec = "libsvtav1"
		case strings.Contains(encoders, "librav1e"):
			codec = "librav1e"
		case strings.Contains(encoders, "libaom-av1"):
			codec = "libaom-av1"
		}
		return []string{"-c:v", codec, "-preset", "6", "-crf", "32", "-c:a", "libopus", "-b:a", "128k"}
	default:
		return []string{"-c", "copy"}
	}
}

func buildMergeCodecArgs(option VideoCodecOption) []string {
	if option.Mode == "copy" {
		return []string{"-c", "copy"}
	}
	return buildCodecArgs(option)
}
