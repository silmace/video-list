package app

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func Start(embeddedFiles embed.FS) {
	var cfgPathOverride string

	flag.StringVar(&cfgPathOverride, "config", "", "Config file path")
	flag.Parse()

	appConfigPath = resolveConfigPath(cfgPathOverride)
	cfg, err := loadOrInitConfig(appConfigPath)
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
	registerRoutes(mux, embeddedFiles)

	handler := requestIDMiddleware(securityHeadersMiddleware(loggingMiddleware(mux)))
	server := &http.Server{
		Addr:              ":3001",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		sig := <-sigCh
		LoggerWith(logrus.Fields{"signal": sig.String()}).Info("shutdown signal received")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			LoggerWith(logrus.Fields{"error": err.Error()}).Warn("graceful shutdown failed")
		}
	}()

	fmt.Println("Server running on http://localhost:3001")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
	<-shutdownDone
	AppLogger.Info("server stopped")
}

func registerRoutes(mux *http.ServeMux, embeddedFiles embed.FS) {

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
}
