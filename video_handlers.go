package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

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

func detectAvailableVideoCodecs() []VideoCodecOption {
	encoders := readFFmpegEncoders()
	options := []VideoCodecOption{
		{ID: "copy", Label: "Direct Stream Copy", Description: "Fastest, keeps original codec when possible", Container: "mp4", Mode: "copy", Available: true},
		{ID: "h264", Label: "H.264", Description: "Balanced compatibility and speed", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "h265", Label: "H.265 / HEVC", Description: "Smaller output, slower encode", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "h264_nvenc", Label: "H.264 (NVIDIA NVENC)", Description: "GPU accelerated H.264 encoding on NVIDIA GPUs", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "hevc_nvenc", Label: "H.265 (NVIDIA NVENC)", Description: "GPU accelerated HEVC encoding on NVIDIA GPUs", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "h264_qsv", Label: "H.264 (Intel Quick Sync)", Description: "GPU accelerated H.264 encoding on Intel iGPU", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "hevc_qsv", Label: "H.265 (Intel Quick Sync)", Description: "GPU accelerated HEVC encoding on Intel iGPU", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "h264_amf", Label: "H.264 (AMD AMF)", Description: "GPU accelerated H.264 encoding on AMD GPUs", Container: "mp4", Mode: "transcode", Available: false},
		{ID: "hevc_amf", Label: "H.265 (AMD AMF)", Description: "GPU accelerated HEVC encoding on AMD GPUs", Container: "mp4", Mode: "transcode", Available: false},
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
		case "h264_nvenc":
			options[i].Available = strings.Contains(encoders, "h264_nvenc")
		case "hevc_nvenc":
			options[i].Available = strings.Contains(encoders, "hevc_nvenc")
		case "h264_qsv":
			options[i].Available = strings.Contains(encoders, "h264_qsv")
		case "hevc_qsv":
			options[i].Available = strings.Contains(encoders, "hevc_qsv")
		case "h264_amf":
			options[i].Available = strings.Contains(encoders, "h264_amf")
		case "hevc_amf":
			options[i].Available = strings.Contains(encoders, "hevc_amf")
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
	case "h264_nvenc":
		return []string{"-c:v", "h264_nvenc", "-preset", "p5", "-cq", "23", "-c:a", "aac", "-b:a", "192k"}
	case "hevc_nvenc":
		return []string{"-c:v", "hevc_nvenc", "-preset", "p5", "-cq", "28", "-c:a", "aac", "-b:a", "192k"}
	case "h264_qsv":
		return []string{"-c:v", "h264_qsv", "-global_quality", "23", "-c:a", "aac", "-b:a", "192k"}
	case "hevc_qsv":
		return []string{"-c:v", "hevc_qsv", "-global_quality", "28", "-c:a", "aac", "-b:a", "192k"}
	case "h264_amf":
		return []string{"-c:v", "h264_amf", "-quality", "balanced", "-c:a", "aac", "-b:a", "192k"}
	case "hevc_amf":
		return []string{"-c:v", "hevc_amf", "-quality", "balanced", "-c:a", "aac", "-b:a", "192k"}
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
