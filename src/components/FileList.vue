<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { motion } from 'motion-v';
import { useRoute, useRouter } from 'vue-router';
import {
  ArrowLeft,
  ArrowRight,
  CheckSquare,
  Download,
  File,
  FileArchive,
  FileAudio,
  FileCode,
  FileImage,
  FileText,
  FileVideo,
  Folder,
  FolderPlus,
  MoveRight,
  PencilLine,
  RefreshCw,
  Search,
  Upload,
  X,
  ZoomIn,
  ZoomOut,
} from 'lucide-vue-next';
import type { FileItem, TaskItem } from '../types';
import { api, buildMediaUrl } from '../services/api';
import PathBreadcrumb from './PathBreadcrumb.vue';
import { authState } from '../composables/useAuth';
import { useLocale } from '../composables/useLocale';
import { inferFileType, useFileVisuals } from '../composables/useFileVisuals';

type SnackbarTone = 'success' | 'error' | 'warning' | 'info';

const route = useRoute();
const router = useRouter();
const { t } = useLocale();
const { getFileAccent, getMatchingTag, getDominantAccent } = useFileVisuals();

const files = ref<FileItem[]>([]);
const loading = ref(false);
const currentPath = ref('');
const search = ref('');
const isMobile = ref(window.innerWidth < 860);
const selectedPaths = ref<Set<string>>(new Set());
const fileInputRef = ref<HTMLInputElement | null>(null);
const uploading = ref(false);
const uploadProgress = ref(0);
const dragActive = ref(false);
const dragDepth = ref(0);
const showMoveDialog = ref(false);
const submittingMoveTask = ref(false);
const moveDestination = ref('/');
const showCreateFolderDialog = ref(false);
const newFolderName = ref('');
const showRenameDialog = ref(false);
const renameTargetPath = ref('');
const renameName = ref('');
const submittingRename = ref(false);
const showImageDialog = ref(false);
const selectedImageIndex = ref(0);
const previewScale = ref(1);
const fitToScreen = ref(true);
const taskPollTimerIds = ref<number[]>([]);
const snackbar = ref({ show: false, message: '', tone: 'info' as SnackbarTone });
let snackbarTimer: number | null = null;

const selectedCount = computed(() => selectedPaths.value.size);
const hasSelection = computed(() => selectedCount.value > 0);
const selectedSingleItem = computed(() => {
  if (selectedCount.value !== 1) {
    return null;
  }
  const [singlePath] = Array.from(selectedPaths.value);
  return files.value.find((item) => item.path === singlePath) || null;
});

const displayedFiles = computed(() => {
  const keyword = search.value.trim().toLowerCase();
  const list = keyword
    ? files.value.filter((file) => file.name.toLowerCase().includes(keyword))
    : files.value;

  return [...list].sort((a, b) => {
    if (a.isDirectory !== b.isDirectory) {
      return a.isDirectory ? -1 : 1;
    }
    return a.name.localeCompare(b.name);
  });
});

const selectedItems = computed(() => displayedFiles.value.filter((item) => selectedPaths.value.has(item.path)));
const imageFiles = computed(() => displayedFiles.value.filter((file) => isImage(file.name) && !file.isDirectory));
const selectedImage = computed(() => imageFiles.value[selectedImageIndex.value] || null);
const selectedImageUrl = computed(() => (selectedImage.value ? buildMediaUrl(selectedImage.value.path) : ''));

const accentColor = computed(() => {
  if (selectedItems.value.length > 0) {
    return getFileAccent(selectedItems.value[0]);
  }
  return getDominantAccent(displayedFiles.value);
});

const snackbarClass = computed(() => `snackbar-${snackbar.value.tone}`);

const showSnackbar = (message: string, tone: SnackbarTone) => {
  snackbar.value = { show: true, message, tone };
  if (snackbarTimer !== null) {
    window.clearTimeout(snackbarTimer);
  }
  snackbarTimer = window.setTimeout(() => {
    snackbar.value.show = false;
  }, 2200);
};

const normalizePath = (input: string) => {
  let path = input.trim() || '/';
  if (!path.startsWith('/')) {
    path = `/${path}`;
  }
  path = path.replace(/\/+/g, '/');
  if (path !== '/' && !path.endsWith('/')) {
    path = `${path}/`;
  }
  return path;
};

const getPathFromRoute = () => {
  if (!route.params.pathMatch) {
    return '/';
  }

  const path = Array.isArray(route.params.pathMatch)
    ? route.params.pathMatch.join('/')
    : route.params.pathMatch;
  return normalizePath(`/${path}`);
};

const isVideo = (name: string) => /\.(mp4|webm|mov|mkv|m4v|avi)$/i.test(name);
const isImage = (name: string) => /\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i.test(name);

const iconFor = (file: FileItem) => {
  const type = inferFileType(file);
  if (type === 'folder') return Folder;
  if (type === 'video') return FileVideo;
  if (type === 'image') return FileImage;
  if (type === 'audio') return FileAudio;
  if (type === 'archive') return FileArchive;
  if (type === 'document') return FileText;
  if (type === 'code') return FileCode;
  return File;
};

const formatSize = (size: number): string => {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = size;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : 2)} ${units[unitIndex]}`;
};

const formatDate = (value: string) => new Date(value).toLocaleString();

const fetchFiles = async (path: string = '/', shouldPush = true) => {
  const normalizedPath = normalizePath(path);
  loading.value = true;
  try {
    const response = await api.get<FileItem[]>('/api/files', {
      params: { path: normalizedPath },
    });

    files.value = response.data;
    currentPath.value = normalizedPath;
    selectedPaths.value.clear();

    if (shouldPush && normalizedPath !== getPathFromRoute()) {
      await router.push(normalizedPath);
    }
  } catch {
    showSnackbar(t('fetchFilesError'), 'error');
  } finally {
    loading.value = false;
  }
};

const downloadFile = async (file: FileItem) => {
  try {
    const response = await api.get('/api/media', {
      params: { path: file.path },
      responseType: 'blob',
    });

    const url = window.URL.createObjectURL(new Blob([response.data]));
    const link = document.createElement('a');
    link.href = url;
    link.setAttribute('download', file.name);
    document.body.appendChild(link);
    link.click();
    link.remove();
    window.URL.revokeObjectURL(url);
    showSnackbar(t('downloadSuccess'), 'success');
  } catch {
    showSnackbar(t('downloadError'), 'error');
  }
};

const openImagePreview = (file: FileItem) => {
  const index = imageFiles.value.findIndex((item) => item.path === file.path);
  if (index < 0) {
    return;
  }
  selectedImageIndex.value = index;
  previewScale.value = 1;
  fitToScreen.value = true;
  showImageDialog.value = true;
};

const openFile = async (file: FileItem) => {
  if (file.isDirectory) {
    const nextPath = normalizePath(`${currentPath.value}${file.name}`);
    await fetchFiles(nextPath);
    return;
  }

  if (isVideo(file.name)) {
    const videoPath = file.path.startsWith('/') ? file.path.slice(1) : file.path;
    await router.push(`/edit/${videoPath}`);
    return;
  }

  if (isImage(file.name)) {
    openImagePreview(file);
    return;
  }

  await downloadFile(file);
};

const onFileClick = async (file: FileItem, event: MouseEvent) => {
  const selectIntent = hasSelection.value || event.ctrlKey || event.metaKey || event.shiftKey;
  if (selectIntent) {
    toggleSelection(file.path);
    return;
  }

  await openFile(file);
};

const isSelected = (file: FileItem) => selectedPaths.value.has(file.path);
const toggleSelection = (filePath: string) => {
  if (selectedPaths.value.has(filePath)) {
    selectedPaths.value.delete(filePath);
    return;
  }
  selectedPaths.value.add(filePath);
};
const clearSelection = () => selectedPaths.value.clear();

const selectAllVisible = () => {
  displayedFiles.value.forEach((item) => selectedPaths.value.add(item.path));
};

const scheduleTaskPolling = (callback: () => Promise<void>) => {
  const pollInterval = authState.taskPollIntervalMs.value || 1200;
  const id = window.setTimeout(async () => {
    await callback();
  }, pollInterval);
  taskPollTimerIds.value.push(id);
};

const pollTaskCompletion = (taskId: string) => {
  const poll = async () => {
    try {
      const response = await api.get<{ success: boolean; task: TaskItem }>(`/api/tasks/${taskId}`);
      const task = response.data.task;
      if (task.status === 'success' || task.status === 'failed' || task.status === 'canceled') {
        await fetchFiles(currentPath.value, false);
        return;
      }
    } catch {
      return;
    }
    scheduleTaskPolling(poll);
  };

  scheduleTaskPolling(poll);
};

const createBatchDeleteTask = async () => {
  if (!hasSelection.value) {
    return;
  }

  const deletingPaths = new Set(selectedPaths.value);
  try {
    const response = await api.post<{ success: boolean; taskId: string }>('/api/tasks/batch-delete', {
      paths: Array.from(deletingPaths),
    });

    files.value = files.value.filter((item) => !deletingPaths.has(item.path));
    clearSelection();
    showSnackbar(t('batchDeleteTaskCreated'), 'success');
    pollTaskCompletion(response.data.taskId);
  } catch {
    showSnackbar(t('batchDeleteTaskError'), 'error');
  }
};

const openMoveTaskDialog = () => {
  moveDestination.value = currentPath.value || '/';
  showMoveDialog.value = true;
};

const createBatchMoveTask = async () => {
  submittingMoveTask.value = true;
  try {
    const response = await api.post<{ success: boolean; taskId: string }>('/api/tasks/batch-move', {
      paths: Array.from(selectedPaths.value),
      destination: normalizePath(moveDestination.value),
    });

    showMoveDialog.value = false;
    clearSelection();
    showSnackbar(t('batchMoveTaskCreated'), 'success');
    pollTaskCompletion(response.data.taskId);
  } catch {
    showSnackbar(t('batchMoveTaskError'), 'error');
  } finally {
    submittingMoveTask.value = false;
  }
};

const downloadSelected = async () => {
  const items = selectedItems.value.filter((item) => !item.isDirectory);
  if (items.length === 0) {
    showSnackbar(t('noDownloadableSelected'), 'warning');
    return;
  }

  for (const item of items) {
    if (isImage(item.name) || isVideo(item.name)) {
      await downloadFile(item);
      continue;
    }
    await openFile(item);
  }
};

const uploadFiles = async (uploadList: File[]) => {
  if (uploadList.length === 0) {
    return;
  }

  uploading.value = true;
  uploadProgress.value = 0;

  try {
    for (let i = 0; i < uploadList.length; i += 1) {
      const file = uploadList[i];
      const formData = new FormData();
      formData.append('file', file);
      formData.append('path', currentPath.value || '/');

      await api.post('/api/files/upload', formData, {
        onUploadProgress: (event) => {
          const part = event.total ? event.loaded / event.total : 0;
          const overall = ((i + part) / uploadList.length) * 100;
          uploadProgress.value = Math.min(100, Math.max(0, Math.round(overall)));
        },
      });
    }

    uploadProgress.value = 100;
    showSnackbar(t('uploadSuccess'), 'success');
    await fetchFiles(currentPath.value || '/', false);
  } catch {
    showSnackbar(t('uploadError'), 'error');
  } finally {
    window.setTimeout(() => {
      uploading.value = false;
      uploadProgress.value = 0;
    }, 240);
  }
};

const openUploadDialog = () => {
  fileInputRef.value?.click();
};

const onFileInputChange = async (event: Event) => {
  const target = event.target as HTMLInputElement;
  const selected = target.files;
  if (!selected) {
    return;
  }
  await uploadFiles(Array.from(selected));
  target.value = '';
};

const hasDraggedFiles = (event: DragEvent) => {
  return Array.from(event.dataTransfer?.types || []).includes('Files');
};

const onDragEnterShell = (event: DragEvent) => {
  if (!hasDraggedFiles(event)) {
    return;
  }
  dragDepth.value += 1;
  dragActive.value = true;
};

const onDragOverShell = (event: DragEvent) => {
  if (!hasDraggedFiles(event)) {
    return;
  }
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy';
  }
};

const onDragLeaveShell = (event: DragEvent) => {
  if (!hasDraggedFiles(event)) {
    return;
  }
  dragDepth.value = Math.max(0, dragDepth.value - 1);
  if (dragDepth.value === 0) {
    dragActive.value = false;
  }
};

const onDropShell = async (event: DragEvent) => {
  if (!hasDraggedFiles(event)) {
    return;
  }
  dragDepth.value = 0;
  dragActive.value = false;
  const dropped = event.dataTransfer?.files;
  if (!dropped || dropped.length === 0) {
    return;
  }
  await uploadFiles(Array.from(dropped));
};

const createFolder = async () => {
  const folderName = newFolderName.value.trim();
  if (!folderName) {
    return;
  }

  try {
    await api.post('/api/files/mkdir', {
      path: currentPath.value || '/',
      name: folderName,
    });
    showCreateFolderDialog.value = false;
    newFolderName.value = '';
    showSnackbar(t('folderCreated'), 'success');
    await fetchFiles(currentPath.value || '/', false);
  } catch {
    showSnackbar(t('folderCreateError'), 'error');
  }
};

const openRenameDialog = (file: FileItem) => {
  renameTargetPath.value = file.path;
  renameName.value = file.name;
  showRenameDialog.value = true;
};

const openRenameDialogForSelection = () => {
  const item = selectedSingleItem.value;
  if (!item) {
    showSnackbar(t('renameSelectionHint'), 'warning');
    return;
  }
  openRenameDialog(item);
};

const renameFile = async () => {
  const nextName = renameName.value.trim();
  if (!renameTargetPath.value || !nextName) {
    return;
  }

  submittingRename.value = true;
  try {
    await api.post('/api/files/rename', {
      path: renameTargetPath.value,
      name: nextName,
    });
    showRenameDialog.value = false;
    showSnackbar(t('renameSuccess'), 'success');
    await fetchFiles(currentPath.value || '/', false);
  } catch {
    showSnackbar(t('renameError'), 'error');
  } finally {
    submittingRename.value = false;
  }
};

const zoomIn = () => {
  previewScale.value = Math.min(3, Number((previewScale.value + 0.2).toFixed(1)));
};

const zoomOut = () => {
  previewScale.value = Math.max(0.4, Number((previewScale.value - 0.2).toFixed(1)));
};

const switchImage = (step: number) => {
  if (imageFiles.value.length === 0) {
    return;
  }
  const total = imageFiles.value.length;
  selectedImageIndex.value = (selectedImageIndex.value + step + total) % total;
};

const closeImageDialog = () => {
  showImageDialog.value = false;
};

const onResize = () => {
  isMobile.value = window.innerWidth < 860;
};

watch(
  () => route.params.pathMatch,
  () => {
    const path = getPathFromRoute();
    if (path !== currentPath.value) {
      void fetchFiles(path, false);
    }
  },
  { immediate: true }
);

onMounted(() => {
  window.addEventListener('resize', onResize);
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize);
  taskPollTimerIds.value.forEach((id) => window.clearTimeout(id));
  taskPollTimerIds.value = [];
  if (snackbarTimer !== null) {
    window.clearTimeout(snackbarTimer);
  }
});
</script>

<template>
  <div
    class="file-manager-shell"
    :style="{ '--accent': accentColor }"
    @dragenter.prevent="onDragEnterShell"
    @dragover.prevent="onDragOverShell"
    @dragleave.prevent="onDragLeaveShell"
    @drop.prevent="onDropShell"
  >
    <section class="bento-grid">
      <article class="glass-card hero-card">
        <PathBreadcrumb :path="currentPath || '/'" :on-navigate="fetchFiles" />

        <div class="hero-controls">
          <label class="input-shell search-input">
            <Search :size="16" />
            <input v-model="search" :placeholder="t('search')" type="text">
          </label>
          <button type="button" class="shad-btn" @click="selectAllVisible">
            <CheckSquare :size="16" />
            {{ t('selectAll') }}
          </button>
          <button type="button" class="shad-btn" :disabled="loading" @click="fetchFiles(currentPath || '/')">
            <RefreshCw :size="16" :class="{ spin: loading }" />
            {{ t('refresh') }}
          </button>
          <button type="button" class="shad-btn" @click="showCreateFolderDialog = true">
            <FolderPlus :size="16" />
            {{ t('newFolder') }}
          </button>
          <button type="button" class="shad-btn shad-btn-primary" :disabled="uploading" @click="openUploadDialog">
            <Upload :size="16" />
            {{ uploading ? t('uploadingNow') : t('upload') }}
          </button>
        </div>

        <motion.div
          v-if="uploading"
          class="hero-upload-progress"
          :initial="{ opacity: 0, y: 8 }"
          :animate="{ opacity: 1, y: 0 }"
          :transition="{ type: 'spring', stiffness: 180, damping: 20 }"
        >
          <div class="upload-progress-track">
            <div class="upload-progress-value" :style="{ width: `${uploadProgress}%` }" />
          </div>
          <span>{{ uploadProgress }}%</span>
        </motion.div>
      </article>

      <article class="glass-card file-list-card">
        <div class="list-header">
          <span>{{ t('name') }}</span>
          <span>{{ t('size') }}</span>
          <span>{{ t('modified') }}</span>
        </div>

        <div v-if="loading" class="list-loading">{{ t('loadingFiles') }}</div>
        <div v-else-if="displayedFiles.length === 0" class="list-empty">{{ t('emptyFolderHint') }}</div>

        <div v-else-if="isMobile" class="mobile-card-list">
          <div
            v-for="file in displayedFiles"
            :key="file.path"
            class="file-mobile-card"
            :class="{ selected: isSelected(file) }"
            @click="onFileClick(file, $event)"
          >
            <div class="file-main" :style="{ '--row-accent': getFileAccent(file) }">
              <button type="button" class="checkbox-hit" @click.stop="toggleSelection(file.path)">
                <span class="checkbox-dot" :class="{ checked: isSelected(file) }" />
              </button>
              <component :is="iconFor(file)" :size="18" />
              <span class="file-name">{{ file.name }}</span>
              <button type="button" class="icon-action-btn" @click.stop="openRenameDialog(file)">
                <PencilLine :size="14" />
              </button>
              <span class="file-tag-dot" :style="{ background: getFileAccent(file) }" />
            </div>
            <div class="file-sub">
              <span>{{ file.isDirectory ? '-' : formatSize(file.size) }}</span>
              <span>{{ formatDate(file.modifiedTime) }}</span>
            </div>
          </div>
        </div>

        <div v-else class="table-wrap">
          <table class="file-table">
            <tbody>
              <tr
                v-for="file in displayedFiles"
                :key="file.path"
                class="file-row"
                :class="{ selected: isSelected(file) }"
                :style="{ '--row-accent': getFileAccent(file) }"
                @click="onFileClick(file, $event)"
              >
                <td class="file-name-cell">
                  <button
                    type="button"
                    class="checkbox-hit"
                    @click.stop="toggleSelection(file.path)"
                  >
                    <span class="checkbox-dot" :class="{ checked: isSelected(file) }" />
                  </button>
                  <component :is="iconFor(file)" :size="18" />
                  <span class="file-name">{{ file.name }}</span>
                  <button type="button" class="icon-action-btn" @click.stop="openRenameDialog(file)">
                    <PencilLine :size="14" />
                  </button>
                  <span
                    v-if="getMatchingTag(file)"
                    class="tag-pill"
                    :style="{ borderColor: getFileAccent(file), color: getFileAccent(file) }"
                  >
                    {{ getMatchingTag(file)?.label }}
                  </span>
                </td>
                <td>{{ file.isDirectory ? '-' : formatSize(file.size) }}</td>
                <td>{{ formatDate(file.modifiedTime) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </article>
    </section>

    <input ref="fileInputRef" type="file" multiple class="hidden" @change="onFileInputChange">

    <motion.div
      v-if="hasSelection"
      class="floating-action-bar"
      :initial="isMobile ? { opacity: 0, y: 24 } : { opacity: 0, y: 24, x: '-50%' }"
      :animate="isMobile ? { opacity: 1, y: 0 } : { opacity: 1, y: 0, x: '-50%' }"
      :transition="{ type: 'spring', stiffness: 180, damping: 20 }"
    >
      <span class="selection-count">{{ t('selectedCount', { count: selectedCount }) }}</span>
      <button type="button" class="shad-btn shad-btn-danger" @click="createBatchDeleteTask">
        <X :size="14" />
        {{ t('delete') }}
      </button>
      <button type="button" class="shad-btn" @click="openMoveTaskDialog">
        <MoveRight :size="14" />
        {{ t('move') }}
      </button>
      <button type="button" class="shad-btn" @click="downloadSelected">
        <Download :size="14" />
        {{ t('download') }}
      </button>
      <button type="button" class="shad-btn" :disabled="selectedCount !== 1" @click="openRenameDialogForSelection">
        <PencilLine :size="14" />
        {{ t('rename') }}
      </button>
      <button type="button" class="shad-btn" @click="clearSelection">{{ t('clear') }}</button>
    </motion.div>

    <div v-if="showMoveDialog" class="modal-overlay" @click.self="showMoveDialog = false">
      <div class="modal-card">
        <h3>{{ t('moveSelection') }}</h3>
        <label class="input-shell">
          <input v-model="moveDestination" type="text" :placeholder="t('destinationPath')">
        </label>
        <p class="modal-hint">{{ t('destinationHint') }}</p>
        <div class="modal-actions">
          <button type="button" class="shad-btn" @click="showMoveDialog = false">{{ t('cancel') }}</button>
          <button type="button" class="shad-btn shad-btn-primary" :disabled="submittingMoveTask" @click="createBatchMoveTask">
            {{ t('createTask') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showCreateFolderDialog" class="modal-overlay" @click.self="showCreateFolderDialog = false">
      <div class="modal-card">
        <h3>{{ t('newFolder') }}</h3>
        <label class="input-shell">
          <input v-model="newFolderName" type="text" :placeholder="t('folderName')" @keyup.enter="createFolder">
        </label>
        <p class="modal-hint">{{ t('folderNameHint') }}</p>
        <div class="modal-actions">
          <button type="button" class="shad-btn" @click="showCreateFolderDialog = false">{{ t('cancel') }}</button>
          <button type="button" class="shad-btn shad-btn-primary" @click="createFolder">{{ t('create') }}</button>
        </div>
      </div>
    </div>

    <div v-if="showRenameDialog" class="modal-overlay" @click.self="showRenameDialog = false">
      <div class="modal-card">
        <h3>{{ t('rename') }}</h3>
        <label class="input-shell">
          <input v-model="renameName" type="text" :placeholder="t('newName')" @keyup.enter="renameFile">
        </label>
        <p class="modal-hint">{{ t('renameHint') }}</p>
        <div class="modal-actions">
          <button type="button" class="shad-btn" @click="showRenameDialog = false">{{ t('cancel') }}</button>
          <button type="button" class="shad-btn shad-btn-primary" :disabled="submittingRename" @click="renameFile">
            {{ t('rename') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showImageDialog && selectedImage" class="modal-overlay image-modal" @click.self="closeImageDialog">
      <div class="image-modal-card">
        <header class="image-header">
          <div class="image-title-wrap">
            <div class="image-name">{{ selectedImage.name }}</div>
            <div class="image-inline-meta">
              <span>{{ selectedImageIndex + 1 }} / {{ imageFiles.length }}</span>
              <span>{{ formatSize(selectedImage.size) }}</span>
              <span>{{ formatDate(selectedImage.modifiedTime) }}</span>
            </div>
          </div>
          <div class="image-tools">
            <button type="button" class="shad-btn shad-btn-sm" @click="switchImage(-1)">
              <ArrowLeft :size="14" />
            </button>
            <button type="button" class="shad-btn shad-btn-sm" @click="switchImage(1)">
              <ArrowRight :size="14" />
            </button>
            <button type="button" class="shad-btn shad-btn-sm" @click="zoomOut">
              <ZoomOut :size="14" />
            </button>
            <button type="button" class="shad-btn shad-btn-sm" @click="zoomIn">
              <ZoomIn :size="14" />
            </button>
            <button type="button" class="shad-btn shad-btn-sm" @click="fitToScreen = !fitToScreen">
              {{ fitToScreen ? t('originalSize') : t('fitWindow') }}
            </button>
            <button type="button" class="shad-btn shad-btn-sm" @click="downloadFile(selectedImage)">
              <Download :size="14" />
            </button>
            <button type="button" class="shad-btn shad-btn-sm" @click="closeImageDialog">
              <X :size="14" />
            </button>
          </div>
        </header>
        <div class="image-viewer">
          <img
            :src="selectedImageUrl"
            :alt="selectedImage.name"
            :class="{ fit: fitToScreen }"
            :style="{ transform: fitToScreen ? 'none' : `scale(${previewScale})` }"
          >
        </div>
        <footer class="image-footer">
          <span>{{ selectedImageIndex + 1 }} / {{ imageFiles.length }}</span>
          <span>{{ formatSize(selectedImage.size) }}</span>
          <span>{{ formatDate(selectedImage.modifiedTime) }}</span>
        </footer>
      </div>
    </div>

    <motion.div
      v-if="snackbar.show"
      class="snackbar"
      :class="snackbarClass"
      :initial="{ opacity: 0, y: 8 }"
      :animate="{ opacity: 1, y: 0 }"
      :transition="{ duration: 0.18 }"
    >
      {{ snackbar.message }}
    </motion.div>

    <motion.div
      v-if="dragActive"
      class="drag-upload-overlay"
      :initial="{ opacity: 0 }"
      :animate="{ opacity: 1 }"
      :exit="{ opacity: 0 }"
    >
      <Upload :size="28" />
      <div class="drag-title">{{ t('dragUploadOverlay') }}</div>
      <div class="drag-sub">{{ t('dragUploadSub') }}</div>
    </motion.div>
  </div>
</template>

<style scoped>
.file-manager-shell {
  padding: 20px;
  --surface-card: rgba(255, 255, 255, 0.58);
  --surface-card-border: rgba(226, 232, 240, 0.9);
  --surface-row: rgba(255, 255, 255, 0.58);
  --surface-row-border: rgba(226, 232, 240, 0.9);
  --surface-input: rgba(255, 255, 255, 0.7);
  --surface-button: rgba(248, 250, 252, 0.88);
  --surface-modal: rgba(255, 255, 255, 0.94);
  --text-body: #0f172a;
  --text-muted: #64748b;
  --card-shadow: 0 22px 56px rgba(17, 24, 39, 0.14);
}

:global(body.theme-dark) .file-manager-shell {
  --surface-card: rgba(10, 15, 26, 0.9);
  --surface-card-border: rgba(100, 116, 139, 0.22);
  --surface-row: rgba(10, 15, 26, 0.92);
  --surface-row-border: rgba(100, 116, 139, 0.2);
  --surface-input: rgba(10, 15, 26, 0.92);
  --surface-button: rgba(18, 24, 38, 0.88);
  --surface-modal: rgba(5, 10, 18, 0.96);
  --text-body: #e2e8f0;
  --text-muted: #94a3b8;
  --card-shadow: 0 22px 56px rgba(2, 8, 18, 0.52);
}

.bento-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: 1fr;
}

.glass-card {
  border-radius: 22px;
  border: 1px solid color-mix(in srgb, var(--accent) 24%, var(--surface-card-border));
  background: linear-gradient(135deg, color-mix(in srgb, var(--surface-card) 92%, transparent), color-mix(in srgb, var(--surface-card) 72%, transparent));
  backdrop-filter: blur(16px);
  box-shadow: var(--card-shadow);
}

.hero-card {
  padding: 18px;
}

.hero-controls {
  margin-top: 14px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.hero-upload-progress {
  margin-top: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.file-list-card {
  padding: 14px;
  min-height: 420px;
}

.list-header {
  display: grid;
  grid-template-columns: 1.2fr 130px 180px;
  padding: 8px 10px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-muted);
}

.list-loading,
.list-empty {
  color: var(--text-muted);
  padding: 20px 12px;
}

.table-wrap {
  overflow: auto;
}

.file-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0 8px;
}

.file-row {
  cursor: pointer;
}

.file-row td {
  background: var(--surface-row);
  padding: 11px 12px;
  border-top: 1px solid var(--surface-row-border);
  border-bottom: 1px solid var(--surface-row-border);
}

.file-row td:first-child {
  border-left: 1px solid var(--surface-row-border);
  border-radius: 12px 0 0 12px;
}

.file-row td:last-child {
  border-right: 1px solid var(--surface-row-border);
  border-radius: 0 12px 12px 0;
}

.file-row:hover td {
  border-color: color-mix(in srgb, var(--row-accent) 45%, var(--surface-row-border));
  background: color-mix(in srgb, var(--row-accent) 16%, var(--surface-row));
}

.file-row.selected td {
  background: color-mix(in srgb, var(--row-accent) 20%, var(--surface-row));
}

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.file-name {
  max-width: 460px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.checkbox-hit {
  border: none;
  background: transparent;
  padding: 0;
  cursor: pointer;
}

.checkbox-dot {
  width: 16px;
  height: 16px;
  border-radius: 5px;
  border: 1px solid #94a3b8;
  display: inline-block;
}

.checkbox-dot.checked {
  border-color: var(--row-accent);
  background: var(--row-accent);
}

.tag-pill {
  margin-left: auto;
  border: 1px solid;
  border-radius: 999px;
  font-size: 11px;
  padding: 3px 8px;
}

.file-tag-dot {
  width: 9px;
  height: 9px;
  border-radius: 99px;
  margin-left: auto;
}

.mobile-card-list {
  display: grid;
  gap: 8px;
  margin-top: 8px;
}

.file-mobile-card {
  border: 1px solid var(--surface-row-border);
  background: var(--surface-row);
  border-radius: 14px;
  padding: 11px;
  text-align: left;
  cursor: pointer;
}

.file-mobile-card.selected {
  border-color: color-mix(in srgb, var(--accent) 45%, rgba(226, 232, 240, 0.9));
  background: color-mix(in srgb, var(--accent) 10%, rgba(255, 255, 255, 0.62));
}

.file-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.icon-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--surface-row-border) 80%, #94a3b8);
  background: color-mix(in srgb, var(--surface-row) 90%, transparent);
  color: var(--text-muted);
  cursor: pointer;
}

.icon-action-btn:hover {
  border-color: color-mix(in srgb, var(--row-accent) 45%, var(--surface-row-border));
  color: var(--text-body);
}

.file-sub {
  margin-top: 8px;
  display: flex;
  justify-content: space-between;
  color: var(--text-muted);
  font-size: 12px;
  gap: 12px;
}

.floating-action-bar {
  position: fixed;
  left: 50%;
  bottom: 22px;
  transform: translateX(-50%);
  box-sizing: border-box;
  z-index: 25;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 8px;
  width: auto;
  max-width: calc(100vw - 34px);
  padding: 10px;
  border-radius: 16px;
  border: 1px solid color-mix(in srgb, var(--accent) 32%, var(--floating-border));
  background: var(--floating-bg);
  backdrop-filter: blur(12px);
  box-shadow: var(--floating-shadow);
}

.selection-count {
  font-size: 12px;
  color: var(--text-muted);
  margin-right: 6px;
}

.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 30;
  background: rgba(15, 23, 42, 0.38);
  backdrop-filter: blur(4px);
  display: grid;
  place-items: center;
  padding: 16px;
}

.modal-card {
  width: min(560px, 100%);
  border-radius: 16px;
  border: 1px solid var(--surface-row-border);
  background: var(--surface-modal);
  color: var(--text-body);
  padding: 16px;
}

.modal-card h3 {
  margin-top: 0;
}

.modal-hint {
  font-size: 12px;
  color: var(--text-muted);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.image-modal {
  padding: 10px;
}

.image-modal-card {
  width: min(1240px, 100%);
  height: min(92vh, 920px);
  display: flex;
  flex-direction: column;
  gap: 10px;
  border-radius: 20px;
  border: 1px solid color-mix(in srgb, var(--accent) 28%, rgba(148, 163, 184, 0.45));
  background: linear-gradient(140deg, rgba(7, 10, 22, 0.94), rgba(7, 14, 28, 0.9));
  color: #e2e8f0;
  padding: 14px;
}

.image-header,
.image-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.image-title-wrap {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.image-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-inline-meta {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  font-size: 12px;
  color: #cbd5e1;
}

.image-tools {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.image-viewer {
  flex: 1;
  min-height: 0;
  display: grid;
  place-items: center;
  overflow: auto;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.86);
}

.image-viewer img {
  display: block;
  max-width: none;
  max-height: none;
  transform-origin: center;
  transition: transform 0.2s ease;
}

.image-viewer img.fit {
  width: 100%;
  height: 100%;
  object-fit: contain;
  max-width: 100%;
  max-height: 100%;
}

.image-footer {
  font-size: 12px;
  color: #cbd5e1;
}

.upload-progress-track {
  flex: 1;
  height: 7px;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.25);
  overflow: hidden;
}

.upload-progress-value {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, var(--accent), color-mix(in srgb, var(--accent) 72%, white));
  transition: width 0.3s ease;
}

.input-shell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border: 1px solid color-mix(in srgb, var(--surface-row-border) 82%, #94a3b8);
  border-radius: 11px;
  padding: 8px 10px;
  background: var(--surface-input);
}

.input-shell input {
  border: none;
  background: transparent;
  outline: none;
  color: var(--text-body);
  min-width: 180px;
}

.search-input {
  flex: 1;
  min-width: 220px;
}

.shad-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid color-mix(in srgb, var(--surface-row-border) 78%, #94a3b8);
  background: var(--surface-button);
  color: var(--text-body);
  border-radius: 11px;
  padding: 8px 10px;
  cursor: pointer;
  font-size: 13px;
  transition: transform 0.16s ease, border-color 0.16s ease;
}

.shad-btn:hover {
  transform: translateY(-1px);
  border-color: color-mix(in srgb, var(--accent) 40%, rgba(148, 163, 184, 0.45));
}

.shad-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.shad-btn-primary {
  border-color: color-mix(in srgb, var(--accent) 58%, transparent);
  background: color-mix(in srgb, var(--accent) 20%, #f8fafc);
}

.shad-btn-danger {
  border-color: rgba(239, 68, 68, 0.42);
  background: rgba(254, 242, 242, 0.92);
  color: #991b1b;
}

.shad-btn-sm {
  padding: 6px 8px;
  font-size: 12px;
}

.snackbar {
  position: fixed;
  right: 18px;
  bottom: 22px;
  z-index: 40;
  color: white;
  border-radius: 10px;
  padding: 8px 12px;
}

.snackbar-success {
  background: #16a34a;
}

.snackbar-error {
  background: #dc2626;
}

.snackbar-warning {
  background: #d97706;
}

.snackbar-info {
  background: #2563eb;
}

.drag-upload-overlay {
  position: fixed;
  inset: 0;
  z-index: 60;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: #fff;
  background: linear-gradient(145deg, rgba(30, 64, 175, 0.56), rgba(14, 165, 233, 0.44));
  backdrop-filter: blur(6px);
}

.drag-title {
  font-size: 20px;
  font-weight: 600;
}

.drag-sub {
  font-size: 13px;
  opacity: 0.9;
}

.hidden {
  display: none;
}

.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 860px) {
  .file-manager-shell {
    padding: 12px;
  }

  .hero-card,
  .file-list-card {
    padding: 12px;
  }

  .hero-controls {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .search-input {
    grid-column: 1 / -1;
    min-width: 0;
    width: 100%;
  }

  .input-shell {
    width: 100%;
  }

  .input-shell input {
    min-width: 0;
    width: 100%;
  }

  .hero-controls .shad-btn {
    width: 100%;
    justify-content: center;
  }

  .list-header {
    display: none;
  }

  .file-name {
    max-width: calc(100vw - 172px);
  }

  .file-sub {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .floating-action-bar {
    width: calc(100vw - 20px);
    max-width: calc(100vw - 20px);
    left: 10px;
    right: 10px;
    bottom: max(10px, env(safe-area-inset-bottom));
    transform: none;
    border-radius: 14px;
    padding: 10px;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
  }

  .selection-count {
    grid-column: 1 / -1;
    margin-right: 0;
  }

  .floating-action-bar .shad-btn {
    width: 100%;
    min-width: 0;
    justify-content: center;
  }

  .image-modal-card {
    height: 92vh;
    padding: 10px;
  }

  .image-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .image-title-wrap {
    width: 100%;
  }

  .image-tools {
    width: 100%;
    justify-content: space-between;
  }

  .image-footer {
    display: none;
  }
}
</style>
