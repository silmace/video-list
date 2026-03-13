package app

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"
)

type contextKey string

const (
	requestIDKey       contextKey = "request_id"
	tokenTTL                      = 24 * time.Hour
	maxJSONBodyBytes   int64      = 1 << 20
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
	BaseDir            string `json:"baseDir" yaml:"baseDir"`
	VideoOutputDir     string `json:"videoOutputDir" yaml:"videoOutputDir"`
	ShowHiddenItems    bool   `json:"showHiddenItems" yaml:"showHiddenItems"`
	PasswordHash       string `json:"passwordHash,omitempty" yaml:"passwordHash,omitempty"`
	LogDir             string `json:"logDir" yaml:"logDir"`
	LogLevel           string `json:"logLevel" yaml:"logLevel"`
	LogRotationHours   int    `json:"logRotationHours" yaml:"logRotationHours"`
	LogMaxAgeDays      int    `json:"logMaxAgeDays" yaml:"logMaxAgeDays"`
	TaskPollIntervalMs int    `json:"taskPollIntervalMs" yaml:"taskPollIntervalMs"`
}

type PublicConfig struct {
	BaseDir            string `json:"baseDir"`
	VideoOutputDir     string `json:"videoOutputDir"`
	ShowHiddenItems    bool   `json:"showHiddenItems"`
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
	ShowHiddenItems    *bool   `json:"showHiddenItems"`
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
