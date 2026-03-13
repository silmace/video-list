package backend

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

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
