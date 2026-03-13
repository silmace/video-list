package routes

import "net/http"

// APIHandlers collects backend handler functions from the backend package.
type APIHandlers struct {
	AuthLogin       http.HandlerFunc
	AuthLogout      http.HandlerFunc
	AuthStatus      http.HandlerFunc
	Settings        http.HandlerFunc
	Files           http.HandlerFunc
	FilesMkdir      http.HandlerFunc
	FilesRename     http.HandlerFunc
	FilesUpload     http.HandlerFunc
	Media           http.HandlerFunc
	EditVideo       http.HandlerFunc
	VideoOptions    http.HandlerFunc
	TasksList       http.HandlerFunc
	TaskByID        http.HandlerFunc
	TaskVideo       http.HandlerFunc
	TaskBatchDelete http.HandlerFunc
	TaskBatchMove   http.HandlerFunc
	TaskBatchCopy   http.HandlerFunc
}

type authWrapper func(http.HandlerFunc) http.HandlerFunc

// RegisterAPI binds grouped API routes onto the given mux.
func RegisterAPI(mux *http.ServeMux, h APIHandlers, withAuth authWrapper) {
	if withAuth == nil {
		withAuth = func(next http.HandlerFunc) http.HandlerFunc { return next }
	}

	registerAuthRoutes(mux, h)
	registerFileRoutes(mux, h, withAuth)
	registerVideoRoutes(mux, h, withAuth)
	registerTaskRoutes(mux, h, withAuth)
}

func registerAuthRoutes(mux *http.ServeMux, h APIHandlers) {
	mux.HandleFunc("/api/auth/login", h.AuthLogin)
	mux.HandleFunc("/api/auth/logout", h.AuthLogout)
	mux.HandleFunc("/api/auth/status", h.AuthStatus)
}

func registerFileRoutes(mux *http.ServeMux, h APIHandlers, withAuth authWrapper) {
	mux.HandleFunc("/api/settings", withAuth(h.Settings))
	mux.HandleFunc("/api/files", withAuth(h.Files))
	mux.HandleFunc("/api/files/mkdir", withAuth(h.FilesMkdir))
	mux.HandleFunc("/api/files/rename", withAuth(h.FilesRename))
	mux.HandleFunc("/api/files/upload", withAuth(h.FilesUpload))
	mux.HandleFunc("/api/media", withAuth(h.Media))
}

func registerVideoRoutes(mux *http.ServeMux, h APIHandlers, withAuth authWrapper) {
	mux.HandleFunc("/api/edit-video", withAuth(h.EditVideo))
	mux.HandleFunc("/api/video/options", withAuth(h.VideoOptions))
}

func registerTaskRoutes(mux *http.ServeMux, h APIHandlers, withAuth authWrapper) {
	mux.HandleFunc("/api/tasks", withAuth(h.TasksList))
	mux.HandleFunc("/api/tasks/", withAuth(h.TaskByID))
	mux.HandleFunc("/api/tasks/video", withAuth(h.TaskVideo))
	mux.HandleFunc("/api/tasks/batch-delete", withAuth(h.TaskBatchDelete))
	mux.HandleFunc("/api/tasks/batch-move", withAuth(h.TaskBatchMove))
	mux.HandleFunc("/api/tasks/batch-copy", withAuth(h.TaskBatchCopy))
}
