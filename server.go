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

func main() {
	// Define the flag for BaseDir
	flag.StringVar(&BaseDir, "baseDir", "/www", "Base directory to serve files from")
	flag.Parse()
	println("BaseDir:", BaseDir)

	// Initialize logging
	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Could not open log file:", err)
	}
	defer logFile.Close()

	log.SetOutput(logFile)

	http.HandleFunc("/api/files", handleFiles)
	http.HandleFunc("/api/media", handleMediaStream)
	http.HandleFunc("/api/edit-video", handleEditVideo)

	// Serve the Vite front-end static files from embedded files
	subFS, err := fs.Sub(embeddedFiles, "dist")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", http.FileServer(http.FS(subFS)))

	fmt.Println("Server running on http://localhost:3001")
	http.ListenAndServe(":3001", nil)
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
	requestedPath := r.URL.Query().Get("path")
	if requestedPath == "" {
		requestedPath = "/"
	}
	fullPath := filepath.Join(BaseDir, requestedPath)

	if !strings.HasPrefix(fullPath, BaseDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	files, err := ioutil.ReadDir(fullPath)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		log.Println("Error reading directory:", err)
		return
	}

	var fileList []FileInfo
	for _, file := range files {
		fileInfo := FileInfo{
			Name:         file.Name(),
			Path:         filepath.Join(fullPath, file.Name()),
			IsDirectory:  file.IsDir(),
			Size:         file.Size(),
			ModifiedTime: file.ModTime(),
		}
		fileList = append(fileList, fileInfo)
	}

	json.NewEncoder(w).Encode(fileList)
}

// Delete file
func deleteFile(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")

	if !strings.HasPrefix(filePath, BaseDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		log.Println("Access denied to:", filePath)
		return
	}

	var err error
	if info, err := os.Stat(filePath); err == nil && info.IsDir() {
		err = os.RemoveAll(filePath)
	} else {
		err = os.Remove(filePath)
	}

	if err != nil {
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		log.Println("Error deleting file:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success": true}`))
}

// Stream video or image file
func handleMediaStream(w http.ResponseWriter, r *http.Request) {
	mediaPath := r.URL.Query().Get("path")

	if !strings.HasPrefix(mediaPath, BaseDir) || !fileExists(mediaPath) {
		http.Error(w, "Media not found or access denied", http.StatusForbidden)
		log.Println("Media not found or access denied:", mediaPath)
		return
	}

	file, err := os.Open(mediaPath)
	if err != nil {
		http.Error(w, "Failed to open media file", http.StatusInternalServerError)
		log.Println("Error opening media file:", err)
		return
	}
	defer file.Close()

	contentType := "application/octet-stream"
	if strings.HasSuffix(mediaPath, ".mp4") {
		contentType = "video/mp4"
	} else if strings.HasSuffix(mediaPath, ".jpg") || strings.HasSuffix(mediaPath, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(mediaPath, ".png") {
		contentType = "image/png"
	} else {
		contentType = "application/file"
	}

	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, mediaPath, time.Now(), file)
}

// Process video segments
func handleEditVideo(w http.ResponseWriter, r *http.Request) {
	var req VideoEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		log.Println("Invalid request payload:", err)
		return
	}

	inputPath := req.VideoPath
	if !strings.HasPrefix(inputPath, BaseDir) {
		http.Error(w, "Access denied", http.StatusForbidden)
		log.Println("Access denied to:", inputPath)
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

		outputPath := strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + "_merge.mp4"
		err = processSegment(inputPath, outputPath, segment.StartTime, duration)
		if err != nil {
			http.Error(w, "Failed to edit video", http.StatusInternalServerError)
			log.Println("Error processing video segment:", err)
			return
		}

		log.Printf("Video edited successfully, output: %s", outputPath)
		w.Write([]byte(fmt.Sprintf(`{"success": true, "output": "%s"}`, filepath.Base(outputPath))))
	} else {
		// Handle multiple segments
		segmentFiles, err := processSegments(inputPath, req.Segments)
		if err != nil {
			http.Error(w, "Failed to process segments", http.StatusInternalServerError)
			log.Println("Error processing segments:", err)
			return
		}
		defer cleanupFiles(segmentFiles)

		outputPath := strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + "_merged.mp4"
		err = mergeSegments(segmentFiles, outputPath)
		if err != nil {
			http.Error(w, "Failed to merge video segments", http.StatusInternalServerError)
			log.Println("Error merging video segments:", err)
			return
		}

		log.Printf("Video merged successfully, output: %s", outputPath)
		w.Write([]byte(fmt.Sprintf(`{"success": true, "output": "%s"}`, filepath.Base(outputPath))))
	}
}

func getTimeDifference(start, end string) (string, error) {
	startParts := strings.Split(start, ":")
	endParts := strings.Split(end, ":")

	if len(startParts) != 3 || len(endParts) != 3 {
		return "", errors.New("invalid time format")
	}

	// Use strconv.Atoi and handle the error properly
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

// 处理一段视频
func processSegment(inputPath, outputPath, startTime, duration string) error {
	log.Printf("Running ffmpeg command: ffmpeg -i %s -ss %s -t %s -c copy %s", inputPath, startTime, duration, outputPath)
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-ss", startTime, "-t", duration, "-c", "copy", outputPath)
	err := cmd.Run()
	if err != nil {
		log.Println("Error running ffmpeg command:", err)
	}
	return err
}

// 处理多段视频
func processSegments(inputPath string, segments []Segment) ([]string, error) {
	var segmentFiles []string
	for i, segment := range segments {
		outputPath := fmt.Sprintf("./videos/.temp/segment_%d.mp4", i)
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
	concatFile := "./videos/.temp/concat.txt"
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
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			log.Printf("Failed to clean up file %s: %v", file, err)
		} else {
			log.Printf("Successfully cleaned up file %s", file)
		}
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}
