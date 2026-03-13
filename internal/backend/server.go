package backend

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"video-list/internal/routes"

	"github.com/sirupsen/logrus"
)

// Run configures and starts the HTTP server.
func Run(embeddedFiles embed.FS, args []string) error {
	var baseDirOverride string
	var cfgPathOverride string
	var portOverride int

	fsFlags := flag.NewFlagSet("video-list", flag.ContinueOnError)
	fsFlags.StringVar(&baseDirOverride, "baseDir", "", "Base directory override")
	fsFlags.StringVar(&cfgPathOverride, "config", "", "Config file path")
	fsFlags.IntVar(&portOverride, "port", 0, "HTTP server port")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}

	port := 3001
	if envPort := strings.TrimSpace(os.Getenv("VIDEO_LIST_PORT")); envPort != "" {
		parsedPort, err := strconv.Atoi(envPort)
		if err != nil {
			return fmt.Errorf("invalid VIDEO_LIST_PORT value %q", envPort)
		}
		port = parsedPort
	}
	if portOverride != 0 {
		port = portOverride
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d, expected 1-65535", port)
	}

	appConfigPath = resolveConfigPath(cfgPathOverride)
	cfg, err := loadOrInitConfig(appConfigPath, baseDirOverride)
	if err != nil {
		return err
	}
	setConfig(cfg)

	if err := SetupLogger(LoggingOptions{
		Dir:           cfg.LogDir,
		Level:         cfg.LogLevel,
		RotationHours: cfg.LogRotationHours,
		MaxAgeDays:    cfg.LogMaxAgeDays,
	}); err != nil {
		return err
	}

	AppLogger.WithFields(logrus.Fields{
		"config_path": appConfigPath,
		"base_dir":    cfg.BaseDir,
		"log_dir":     cfg.LogDir,
	}).Info("server startup")

	mux := http.NewServeMux()
	routes.RegisterAPI(mux, routes.APIHandlers{
		AuthLogin:       handleAuthLogin,
		AuthLogout:      handleAuthLogout,
		AuthStatus:      handleAuthStatus,
		Settings:        handleSettings,
		Files:           handleFiles,
		FilesMkdir:      handleCreateFolder,
		FilesRename:     handleRenameFile,
		FilesUpload:     handleUploadFile,
		Media:           handleMediaStream,
		EditVideo:       handleEditVideo,
		VideoOptions:    handleVideoOptions,
		TasksList:       handleTaskList,
		TaskByID:        handleTaskByID,
		TaskVideo:       handleCreateVideoTask,
		TaskBatchDelete: handleCreateBatchDeleteTask,
		TaskBatchMove:   handleCreateBatchMoveTask,
		TaskBatchCopy:   handleCreateBatchCopyTask,
	}, withAuth)

	distFS, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		return err
	}
	routes.RegisterStaticSPA(mux, distFS)

	handler := requestIDMiddleware(loggingMiddleware(mux))
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
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

	fmt.Printf("Server running on http://localhost:%d\n", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		if isPortInUseError(err) {
			return fmt.Errorf("port %d is already in use, stop the existing process or start with -port=<port> (or VIDEO_LIST_PORT)", port)
		}
		return err
	}
	<-shutdownDone
	AppLogger.Info("server stopped")
	return nil
}

func isPortInUseError(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "address already in use") || strings.Contains(msg, "only one usage of each socket address")
}
