package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/sirupsen/logrus"
)

var AppLogger = logrus.New()

type LoggingOptions struct {
	Dir           string
	Level         string
	RotationHours int
	MaxAgeDays    int
}

func SetupLogger(opts LoggingOptions) error {
	if opts.Dir == "" {
		opts.Dir = "."
	}
	if opts.RotationHours <= 0 {
		opts.RotationHours = 24
	}
	if opts.MaxAgeDays <= 0 {
		opts.MaxAgeDays = 7
	}

	if err := os.MkdirAll(opts.Dir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(opts.Dir, "video-list.log")
	rotateOptions := []rotatelogs.Option{
		rotatelogs.WithRotationTime(time.Duration(opts.RotationHours) * time.Hour),
		rotatelogs.WithMaxAge(time.Duration(opts.MaxAgeDays) * 24 * time.Hour),
	}
	if runtime.GOOS != "windows" {
		rotateOptions = append(rotateOptions, rotatelogs.WithLinkName(logPath))
	}

	rotator, err := rotatelogs.New(logPath+".%Y%m%d%H", rotateOptions...)
	if err != nil {
		return err
	}

	level, err := logrus.ParseLevel(opts.Level)
	if err != nil {
		level = logrus.InfoLevel
	}

	AppLogger.SetLevel(level)
	AppLogger.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
	AppLogger.SetOutput(io.MultiWriter(os.Stdout, rotator))
	return nil
}

func LoggerWith(fields logrus.Fields) *logrus.Entry {
	return AppLogger.WithFields(fields)
}
