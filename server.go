package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed dist/*
var embeddedFiles embed.FS

var BaseDir string

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
	VideoPath string    `json:"videoPath"`
	Segments  []Segment `json:"segments"`
}

// Convert absolute path to relative path
func toRelativePath(path string) string {
	path = filepath.Clean(filepath.FromSlash(path))
	baseDir := filepath.Clean(filepath.FromSlash(BaseDir))

	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		log.Println("Error converting to relative path:", err)
		return ""
	}
	return filepath.ToSlash(rel)
}

// Convert relative path to absolute path and validate
func toAbsolutePath(relPath string) (string, error) {
	relPath = filepath.Clean(filepath.FromSlash(relPath))
	absPath := filepath.Join(BaseDir, relPath)

	if !strings.HasPrefix(absPath, BaseDir) {
		return "", errors.New("invalid path: outside base directory")
	}

	return absPath, nil
}

func main() {
	flag.StringVar(&BaseDir, "baseDir", "/www", "Base directory to serve files from")
	flag.Parse()

	if !filepath.IsAbs(BaseDir) {
		log.Fatal("BaseDir must be an absolute path")
	}

	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Could not open log file:", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)

	// Create a new mux for better route handling
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/files", handleFiles)
	mux.HandleFunc("/api/media", handleMediaStream)
	mux.HandleFunc("/api/edit-video", handleEditVideo)

	// Strip the "dist" prefix from embedded files
	distFS, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		log.Fatal("Failed to create sub filesystem:", err)
	}

	// Handle all other routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If it's an API request, return 404
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve static file from embedded filesystem
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Remove leading slash for fs.Sub
		path = strings.TrimPrefix(path, "/")

		// Try to serve static file
		if content, err := fs.ReadFile(distFS, path); err == nil {
			// Set content type
			ext := filepath.Ext(path)
			contentType := mime.TypeByExtension(ext)
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			w.Header().Set("Content-Type", contentType)
			w.Write(content)
			return
		}

		// If file not found, serve index.html for SPA routing
		content, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(content)
	})

	fmt.Println("Server running on http://localhost:3001")
	if err := http.ListenAndServe(":3001", mux); err != nil {
		log.Fatal(err)
	}
}

// Handle listing and deleting files
func handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		listFiles(w, r)
	} else if r.Method == http.MethodDelete {
		deleteFile(w, r)
	} else {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// List files in directory
func listFiles(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		relPath = "/"
	}

	absPath, err := toAbsolutePath(relPath)
	if err != nil {
		log.Println("Invalid path:", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid path"})
		return
	}

	files, err := ioutil.ReadDir(absPath)
	if err != nil {
		log.Println("Error reading directory:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read directory"})
		return
	}

	if len(files) == 0 {
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(map[string]string{"error": "Directory is empty"})
		return
	}

	var fileList []FileInfo
	for _, file := range files {
		fullPath := filepath.Join(absPath, file.Name())
		fileInfo := FileInfo{
			Name:         file.Name(),
			Path:         toRelativePath(fullPath),
			IsDirectory:  file.IsDir(),
			Size:         file.Size(),
			ModifiedTime: file.ModTime(),
		}
		fileList = append(fileList, fileInfo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileList)
}

// Delete file
func deleteFile(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, err := toAbsolutePath(relPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if info, err := os.Stat(absPath); err == nil && info.IsDir() {
		err = os.RemoveAll(absPath)
	} else {
		err = os.Remove(absPath)
	}

	if err != nil {
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		log.Println("Error deleting file:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success": true}`))
}

// Stream video or image file
func handleMediaStream(w http.ResponseWriter, r *http.Request) {
	relPath := r.URL.Query().Get("path")
	absPath, err := toAbsolutePath(relPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if !fileExists(absPath) {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	file, err := os.Open(absPath)
	if err != nil {
		http.Error(w, "Failed to open media file", http.StatusInternalServerError)
		log.Println("Error opening media file:", err)
		return
	}
	defer file.Close()

	contentType := "application/octet-stream"
	if strings.HasSuffix(absPath, ".mp4") {
		contentType = "video/mp4"
	} else if strings.HasSuffix(absPath, ".jpg") || strings.HasSuffix(absPath, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(absPath, ".png") {
		contentType = "image/png"
	} else {
		contentType = "application/file"
	}

	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, filepath.Base(absPath), time.Now(), file)
}

// Process video segments
func handleEditVideo(w http.ResponseWriter, r *http.Request) {
	var req VideoEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println("Invalid request payload:", err)
		return
	}

	absPath, err := toAbsolutePath(req.VideoPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	log.Printf("Processing video: %s", req.VideoPath)

	if len(req.Segments) == 1 {
		// Handle single segment
		segment := req.Segments[0]
		duration, err := getTimeDifference(segment.StartTime, segment.EndTime)
		if err != nil {
			http.Error(w, "Invalid time format", http.StatusBadRequest)
			log.Println("Invalid time format:", err)
			return
		}

		outputPath := strings.TrimSuffix(absPath, filepath.Ext(absPath)) + "_merge.mp4"
		err = processSegment(absPath, outputPath, segment.StartTime, duration)
		if err != nil {
			http.Error(w, "Failed to edit video", http.StatusInternalServerError)
			log.Println("Error processing video segment:", err)
			return
		}

		log.Printf("Video edited successfully, output: %s", outputPath)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"success": true, "output": "%s"}`, toRelativePath(outputPath))))
	} else {
		// Handle multiple segments
		segmentFiles, err := processSegments(absPath, req.Segments)
		if err != nil {
			http.Error(w, "Failed to process segments", http.StatusInternalServerError)
			log.Println("Error processing segments:", err)
			return
		}
		defer cleanupFiles(segmentFiles)

		outputPath := strings.TrimSuffix(absPath, filepath.Ext(absPath)) + "_merged.mp4"
		err = mergeSegments(segmentFiles, outputPath)
		if err != nil {
			http.Error(w, "Failed to merge video segments", http.StatusInternalServerError)
			log.Println("Error merging video segments:", err)
			return
		}

		log.Printf("Video merged successfully, output: %s", outputPath)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(fmt.Sprintf(`{"success": true, "output": "%s"}`, toRelativePath(outputPath))))
	}
}

func getTimeDifference(start, end string) (string, error) {
	startParts := strings.Split(start, ":")
	endParts := strings.Split(end, ":")

	if len(startParts) != 3 || len(endParts) != 3 {
		return "", errors.New("invalid time format")
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

	return strconv.Itoa(endTotalSeconds - startTotalSeconds), nil
}

func processSegment(inputPath, outputPath, startTime, duration string) error {
	log.Printf("Running ffmpeg command: ffmpeg -i %s -ss %s -t %s -c copy %s", inputPath, startTime, duration, outputPath)
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-ss", startTime, "-t", duration, "-c", "copy", outputPath)
	err := cmd.Run()
	if err != nil {
		log.Println("Error running ffmpeg command:", err)
	}
	return err
}

func processSegments(inputPath string, segments []Segment) ([]string, error) {
	tempDir := filepath.Join(filepath.Dir(inputPath), ".temp")
	os.MkdirAll(tempDir, 0755)

	var segmentFiles []string
	for i, segment := range segments {
		outputPath := filepath.Join(tempDir, fmt.Sprintf("segment_%d.mp4", i))
		duration, err := getTimeDifference(segment.StartTime, segment.EndTime)
		if err != nil {
			log.Println("Invalid time format:", err)
			return nil, err
		}
		err = processSegment(inputPath, outputPath, segment.StartTime, duration)
		if err != nil {
			log.Println("Error processing segment:", err)
			return nil, err
		}
		segmentFiles = append(segmentFiles, outputPath)
	}
	return segmentFiles, nil
}

func mergeSegments(segmentFiles []string, outputPath string) error {
	tempDir := filepath.Dir(segmentFiles[0])
	concatFile := filepath.Join(tempDir, "concat.txt")
	concatContent := ""
	for _, file := range segmentFiles {
		concatContent += fmt.Sprintf("file '%s'\n", file)
	}
	err := ioutil.WriteFile(concatFile, []byte(concatContent), 0644)
	if err != nil {
		log.Println("Error writing concat file:", err)
		return err
	}

	log.Printf("Running ffmpeg command: ffmpeg -f concat -safe 0 -i %s -c copy %s", concatFile, outputPath)
	cmd := exec.Command("ffmpeg", "-f", "concat", "-safe", "0", "-i", concatFile, "-c", "copy", outputPath)
	err = cmd.Run()
	if err != nil {
		log.Println("Error merging segments:", err)
	}
	return err
}

func cleanupFiles(files []string) {
	tempDir := filepath.Dir(files[0])
	err := os.RemoveAll(tempDir)
	if err != nil {
		log.Printf("Failed to clean up temp directory %s: %v", tempDir, err)
	} else {
		log.Printf("Successfully cleaned up temp directory %s", tempDir)
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
