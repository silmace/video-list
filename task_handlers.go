package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func handleCreateBatchDeleteTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req BatchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths are required")
		return
	}

	task := taskManager.Create("batch-delete", fmt.Sprintf("deleting %d items", len(req.Paths)))
	ctx, cancel := context.WithTimeout(context.Background(), maxTaskRuntime)
	taskManager.SetCancel(task.ID, cancel)

	go runBatchDeleteTask(ctx, task.ID, req.Paths)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "taskId": task.ID})
}

func runBatchDeleteTask(ctx context.Context, taskID string, paths []string) {
	release, err := acquireTaskSlot(ctx)
	if err != nil {
		taskManager.Fail(taskID, err)
		return
	}
	defer release()

	total := len(paths)
	taskManager.Update(taskID, func(t *Task) {
		t.Status = TaskStatusRunning
		t.Stage = "deleting"
		t.Progress = 5
		t.Message = "starting batch delete"
		t.Total = total
		t.Current = 0
		t.CurrentItem = ""
	})

	for i, path := range paths {
		select {
		case <-ctx.Done():
			taskManager.MarkCanceled(taskID)
			return
		default:
		}

		absPath, err := toAbsolutePath(path)
		if err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := ensureSafePath(absPath, false); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		if err := os.RemoveAll(absPath); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		progress := 10 + int((float64(i+1)/float64(total))*90)
		currentItem := filepath.Base(absPath)
		taskManager.Update(taskID, func(t *Task) {
			t.Progress = progress
			t.Message = fmt.Sprintf("deleted %d/%d", i+1, total)
			t.Current = i + 1
			t.CurrentItem = currentItem
		})
	}

	taskManager.Complete(taskID, "batch delete completed", "")
}

func handleCreateBatchMoveTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req BatchMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths are required")
		return
	}
	if strings.TrimSpace(req.Destination) == "" {
		writeError(w, http.StatusBadRequest, "destination is required")
		return
	}

	destination, err := toAbsolutePath(req.Destination)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination path")
		return
	}

	task := taskManager.Create("batch-move", fmt.Sprintf("moving %d items", len(req.Paths)))
	ctx, cancel := context.WithTimeout(context.Background(), maxTaskRuntime)
	taskManager.SetCancel(task.ID, cancel)

	go runBatchMoveTask(ctx, task.ID, req.Paths, destination)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "taskId": task.ID})
}

func runBatchMoveTask(ctx context.Context, taskID string, paths []string, destination string) {
	release, err := acquireTaskSlot(ctx)
	if err != nil {
		taskManager.Fail(taskID, err)
		return
	}
	defer release()

	total := len(paths)
	taskManager.Update(taskID, func(t *Task) {
		t.Status = TaskStatusRunning
		t.Stage = "moving"
		t.Progress = 5
		t.Message = "starting batch move"
		t.Total = total
		t.Current = 0
		t.CurrentItem = ""
	})

	for i, path := range paths {
		select {
		case <-ctx.Done():
			taskManager.MarkCanceled(taskID)
			return
		default:
		}

		absPath, err := toAbsolutePath(path)
		if err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := ensureSafePath(absPath, false); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		targetPath := filepath.Join(destination, filepath.Base(absPath))
		if err := ensureSafePath(targetPath, true); err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := movePath(absPath, targetPath); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		progress := 10 + int((float64(i+1)/float64(total))*90)
		currentItem := filepath.Base(absPath)
		taskManager.Update(taskID, func(t *Task) {
			t.Progress = progress
			t.Message = fmt.Sprintf("moved %d/%d", i+1, total)
			t.Current = i + 1
			t.CurrentItem = currentItem
		})
	}

	taskManager.Complete(taskID, "batch move completed", toRelativePath(destination))
}

func handleCreateBatchCopyTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req BatchCopyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request payload")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths are required")
		return
	}
	if strings.TrimSpace(req.Destination) == "" {
		writeError(w, http.StatusBadRequest, "destination is required")
		return
	}

	destination, err := toAbsolutePath(req.Destination)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid destination path")
		return
	}
	if err := ensureSafePath(destination, false); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task := taskManager.Create("batch-copy", fmt.Sprintf("copying %d items", len(req.Paths)))
	ctx, cancel := context.WithTimeout(context.Background(), maxTaskRuntime)
	taskManager.SetCancel(task.ID, cancel)

	go runBatchCopyTask(ctx, task.ID, req.Paths, destination)
	writeJSON(w, http.StatusAccepted, map[string]any{"success": true, "taskId": task.ID})
}

func runBatchCopyTask(ctx context.Context, taskID string, paths []string, destination string) {
	release, err := acquireTaskSlot(ctx)
	if err != nil {
		taskManager.Fail(taskID, err)
		return
	}
	defer release()

	total := len(paths)
	taskManager.Update(taskID, func(t *Task) {
		t.Status = TaskStatusRunning
		t.Stage = "copying"
		t.Progress = 5
		t.Message = "starting batch copy"
		t.Total = total
		t.Current = 0
		t.CurrentItem = ""
	})

	for i, rawPath := range paths {
		select {
		case <-ctx.Done():
			taskManager.MarkCanceled(taskID)
			return
		default:
		}

		absPath, err := toAbsolutePath(rawPath)
		if err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if err := ensureSafePath(absPath, false); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		targetPath := filepath.Join(destination, filepath.Base(absPath))
		if err := ensureSafePath(targetPath, true); err != nil {
			taskManager.Fail(taskID, err)
			return
		}
		if _, err := os.Stat(targetPath); err == nil {
			taskManager.Fail(taskID, fmt.Errorf("target already exists: %s", filepath.Base(targetPath)))
			return
		}
		if err := copyPath(absPath, targetPath); err != nil {
			taskManager.Fail(taskID, err)
			return
		}

		progress := 10 + int((float64(i+1)/float64(total))*90)
		currentItem := filepath.Base(absPath)
		taskManager.Update(taskID, func(t *Task) {
			t.Progress = progress
			t.Message = fmt.Sprintf("copied %d/%d", i+1, total)
			t.Current = i + 1
			t.CurrentItem = currentItem
		})
	}

	taskManager.Complete(taskID, "batch copy completed", toRelativePath(destination))
}

func handleTaskList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "tasks": taskManager.List()})
}

func handleTaskByID(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, ok := taskManager.Get(taskID)
		if !ok {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "task": task})
	case http.MethodDelete:
		if !taskManager.Cancel(taskID) {
			writeError(w, http.StatusBadRequest, "task cannot be canceled")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func movePath(src string, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := copyPath(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyPath(src string, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlink copy is not allowed")
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			srcChild := filepath.Join(src, entry.Name())
			dstChild := filepath.Join(dst, entry.Name())
			if err := copyPath(srcChild, dstChild); err != nil {
				return err
			}
		}
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

func acquireTaskSlot(ctx context.Context) (func(), error) {
	select {
	case taskSlots <- struct{}{}:
		return func() {
			<-taskSlots
		}, nil
	case <-ctx.Done():
		return nil, context.Canceled
	}
}
