package main

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
	includeHidden := getConfig().ShowHiddenItems

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
