<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  ArrowUp,
  CheckSquare,
  Copy,
  FolderPlus,
  History,
  MoveRight,
  RefreshCw,
  Search,
  Star,
  Trash2,
  Upload,
} from 'lucide-vue-next';
import { AxiosError } from 'axios';
import type { FileFilterType, FileItem, FileSortBy, FileSortOrder } from '../types';
import { buildMediaUrl } from '../services/api';
import {
  createBatchCopyTask,
  createBatchDeleteTask,
  createBatchMoveTask,
  createFolder,
  listFiles,
  renameFile,
  uploadFile,
} from '../services/files';
import { fetchTask } from '../services/tasks';
import PathBreadcrumb from './PathBreadcrumb.vue';
import FileBrowserPane from './file-manager/FileBrowserPane.vue';
import DestinationPickerDialog from './file-manager/DestinationPickerDialog.vue';
import ImagePreviewDialog from './file-manager/ImagePreviewDialog.vue';
import { authState } from '../composables/useAuth';
import { inferFileType, useFileVisuals } from '../composables/useFileVisuals';
import { useLocale } from '../composables/useLocale';

type SnackbarTone = 'success' | 'error' | 'warning' | 'info';
type TransferMode = 'move' | 'copy';

const FAVORITES_STORAGE_KEY = 'video_list_favorites';

const route = useRoute();
const router = useRouter();
const { t } = useLocale();
const { getFileAccent, getMatchingTag, getDominantAccent } = useFileVisuals();

const files = ref<FileItem[]>([]);
const loading = ref(false);
const currentPath = ref('/');
const search = ref('');
const sortBy = ref<FileSortBy>('name');
const sortOrder = ref<FileSortOrder>('asc');
const typeFilter = ref<FileFilterType>('all');
const includeHidden = ref(false);
const isMobile = ref(window.innerWidth < 960);
const selectedPaths = ref<Set<string>>(new Set());
const favorites = ref<string[]>(loadFavorites());
const fileInputRef = ref<HTMLInputElement | null>(null);
const uploading = ref(false);
const uploadProgress = ref(0);
const dragActive = ref(false);
const dragDepth = ref(0);
const taskPollTimerIds = ref<number[]>([]);
const showCreateFolderDialog = ref(false);
const newFolderName = ref('');
const showRenameDialog = ref(false);
const renameTargetPath = ref('');
const renameName = ref('');
const submittingRename = ref(false);
const showDeleteDialog = ref(false);
const showTransferDialog = ref(false);
const transferMode = ref<TransferMode>('move');
const transferPath = ref('/');
const transferFolders = ref<FileItem[]>([]);
const transferLoading = ref(false);
const submittingTransfer = ref(false);
const showImageDialog = ref(false);
const selectedImageIndex = ref(0);
const previewScale = ref(1);
const fitToScreen = ref(true);
const snackbar = ref({ show: false, message: '', tone: 'info' as SnackbarTone });
let snackbarTimer: number | null = null;
let filterTimer: number | null = null;
let syncingRouteQuery = false;

const selectedCount = computed(() => selectedPaths.value.size);
const hasSelection = computed(() => selectedCount.value > 0);
const selectedItems = computed(() => files.value.filter((item) => selectedPaths.value.has(item.path)));
const selectedSingleItem = computed(() => (selectedCount.value === 1 ? selectedItems.value[0] || null : null));
const folderEntries = computed(() => files.value.filter((file) => file.isDirectory));
const imageFiles = computed(() => files.value.filter((file) => isImage(file.name) && !file.isDirectory));
const selectedImage = computed(() => imageFiles.value[selectedImageIndex.value] || null);
const selectedImageUrl = computed(() => (selectedImage.value ? buildMediaUrl(selectedImage.value.path) : ''));
const activeInspectorItem = computed(() => selectedSingleItem.value || files.value[0] || null);
const accentColor = computed(() => {
  if (selectedItems.value.length > 0) {
    return getFileAccent(selectedItems.value[0]);
  }
  return getDominantAccent(files.value);
});
const transferTitle = computed(() => (transferMode.value === 'copy' ? t('copySelection') : t('moveSelection')));
const parentPath = computed(() => getParentPath(currentPath.value));
const filterTypeOptions = computed(() => [
  { value: 'all', label: t('allTypes') },
  { value: 'folder', label: t('type_folder') },
  { value: 'video', label: t('type_video') },
  { value: 'image', label: t('type_image') },
  { value: 'audio', label: t('type_audio') },
  { value: 'archive', label: t('type_archive') },
  { value: 'document', label: t('type_document') },
  { value: 'code', label: t('type_code') },
  { value: 'other', label: t('type_other') },
]);

function loadFavorites(): string[] {
  try {
    const parsed = JSON.parse(localStorage.getItem(FAVORITES_STORAGE_KEY) || '[]') as string[];
    return Array.isArray(parsed) ? parsed.filter((value) => typeof value === 'string') : [];
  } catch {
    return [];
  }
}

function persistFavorites() {
  localStorage.setItem(FAVORITES_STORAGE_KEY, JSON.stringify(favorites.value));
}

function showSnackbar(message: string, tone: SnackbarTone) {
  snackbar.value = { show: true, message, tone };
  if (snackbarTimer !== null) {
    window.clearTimeout(snackbarTimer);
  }
  snackbarTimer = window.setTimeout(() => {
    snackbar.value.show = false;
  }, 2600);
}

function resolveApiErrorMessage(error: unknown, fallback: string): string {
  const maybeAxios = error as AxiosError<{ error?: string }>;
  return maybeAxios.response?.data?.error || fallback;
}

function normalizePath(input: string) {
  let normalized = input.trim() || '/';
  if (!normalized.startsWith('/')) {
    normalized = `/${normalized}`;
  }
  normalized = normalized.replace(/\/+/g, '/');
  if (normalized !== '/' && !normalized.endsWith('/')) {
    normalized = `${normalized}/`;
  }
  return normalized;
}

function getPathFromRoute() {
  if (!route.params.pathMatch) {
    return '/';
  }
  const path = Array.isArray(route.params.pathMatch)
    ? route.params.pathMatch.join('/')
    : route.params.pathMatch;
  return normalizePath(`/${path}`);
}

function getParentPath(value: string) {
  if (!value || value === '/') {
    return '/';
  }
  const segments = value.split('/').filter(Boolean);
  if (segments.length <= 1) {
    return '/';
  }
  return `/${segments.slice(0, -1).join('/')}/`;
}

function readQueryState() {
  syncingRouteQuery = true;
  search.value = typeof route.query.search === 'string' ? route.query.search : '';
  sortBy.value = route.query.sortBy === 'size' || route.query.sortBy === 'modified' ? route.query.sortBy : 'name';
  sortOrder.value = route.query.order === 'desc' ? 'desc' : 'asc';
  typeFilter.value = typeof route.query.type === 'string' ? (route.query.type as FileFilterType) : 'all';
  includeHidden.value = route.query.hidden === '1';
  syncingRouteQuery = false;
}

function buildRouteQuery() {
  return {
    ...(search.value.trim() ? { search: search.value.trim() } : {}),
    ...(sortBy.value !== 'name' ? { sortBy: sortBy.value } : {}),
    ...(sortOrder.value !== 'asc' ? { order: sortOrder.value } : {}),
    ...(typeFilter.value !== 'all' ? { type: typeFilter.value } : {}),
    ...(includeHidden.value ? { hidden: '1' } : {}),
  };
}

function scheduleFilterRefresh() {
  if (syncingRouteQuery) {
    return;
  }
  if (filterTimer !== null) {
    window.clearTimeout(filterTimer);
  }
  filterTimer = window.setTimeout(() => {
    void fetchFiles(currentPath.value, true);
  }, 220);
}

async function fetchFiles(path = '/', pushQuery = false) {
  const normalizedPath = normalizePath(path);
  loading.value = true;
  try {
    files.value = await listFiles({
      path: normalizedPath,
      search: search.value.trim(),
      sortBy: sortBy.value,
      order: sortOrder.value,
      type: typeFilter.value,
      includeHidden: includeHidden.value,
    });
    currentPath.value = normalizedPath;
    selectedPaths.value.clear();

    if (pushQuery || normalizedPath !== getPathFromRoute()) {
      await router.replace({
        path: normalizedPath,
        query: buildRouteQuery(),
      });
    }
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('fetchFilesError')), 'error');
  } finally {
    loading.value = false;
  }
}

function triggerBrowserDownload(file: FileItem) {
  const link = document.createElement('a');
  link.href = buildMediaUrl(file.path);
  link.download = file.name;
  document.body.appendChild(link);
  link.click();
  link.remove();
}

function isVideo(name: string) {
  return inferFileType({ name, isDirectory: false } as FileItem) === 'video';
}

function isImage(name: string) {
  return inferFileType({ name, isDirectory: false } as FileItem) === 'image';
}

function formatSize(size: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = size;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  return `${value.toFixed(value >= 100 ? 0 : 2)} ${units[unitIndex]}`;
}

function formatDate(value: string) {
  return new Date(value).toLocaleString();
}

function isSelected(filePath: string) {
  return selectedPaths.value.has(filePath);
}

function toggleSelection(filePath: string) {
  if (selectedPaths.value.has(filePath)) {
    selectedPaths.value.delete(filePath);
    return;
  }
  selectedPaths.value.add(filePath);
}

function clearSelection() {
  selectedPaths.value.clear();
}

function selectAllVisible() {
  files.value.forEach((item) => selectedPaths.value.add(item.path));
}

async function openFile(file: FileItem) {
  if (file.isDirectory) {
    await fetchFiles(file.path, true);
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
  triggerBrowserDownload(file);
}

async function onFileClick(file: FileItem, event: MouseEvent) {
  const selectIntent = hasSelection.value || event.ctrlKey || event.metaKey || event.shiftKey;
  if (selectIntent) {
    toggleSelection(file.path);
    return;
  }
  await openFile(file);
}

function openImagePreview(file: FileItem) {
  const index = imageFiles.value.findIndex((item) => item.path === file.path);
  if (index < 0) {
    return;
  }
  selectedImageIndex.value = index;
  previewScale.value = 1;
  fitToScreen.value = true;
  showImageDialog.value = true;
}

function switchImage(step: number) {
  if (imageFiles.value.length === 0) {
    return;
  }
  const total = imageFiles.value.length;
  selectedImageIndex.value = (selectedImageIndex.value + step + total) % total;
}

function zoomIn() {
  previewScale.value = Math.min(3, Number((previewScale.value + 0.2).toFixed(1)));
}

function zoomOut() {
  previewScale.value = Math.max(0.4, Number((previewScale.value - 0.2).toFixed(1)));
}

function closeImageDialog() {
  showImageDialog.value = false;
}

function openUploadDialog() {
  fileInputRef.value?.click();
}

async function uploadFiles(fileList: File[]) {
  if (fileList.length === 0) {
    return;
  }
  uploading.value = true;
  uploadProgress.value = 0;

  try {
    for (let index = 0; index < fileList.length; index += 1) {
      const file = fileList[index];
      try {
        await uploadFile(currentPath.value, file, false, (event) => {
          const part = event.total ? event.loaded / event.total : 0;
          const overall = ((index + part) / fileList.length) * 100;
          uploadProgress.value = Math.min(100, Math.max(0, Math.round(overall)));
        });
      } catch (error) {
        const maybeAxios = error as AxiosError;
        if (maybeAxios.response?.status === 409) {
          const shouldOverwrite = window.confirm(t('overwriteConflict', { name: file.name }));
          if (!shouldOverwrite) {
            continue;
          }
          await uploadFile(currentPath.value, file, true, (event) => {
            const part = event.total ? event.loaded / event.total : 0;
            const overall = ((index + part) / fileList.length) * 100;
            uploadProgress.value = Math.min(100, Math.max(0, Math.round(overall)));
          });
          continue;
        }
        throw error;
      }
    }
    uploadProgress.value = 100;
    showSnackbar(t('uploadSuccess'), 'success');
    await fetchFiles(currentPath.value, false);
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('uploadError')), 'error');
  } finally {
    window.setTimeout(() => {
      uploading.value = false;
      uploadProgress.value = 0;
    }, 260);
  }
}

async function onFileInputChange(event: Event) {
  const target = event.target as HTMLInputElement;
  const selected = target.files;
  if (!selected) {
    return;
  }
  await uploadFiles(Array.from(selected));
  target.value = '';
}

function hasDraggedFiles(event: DragEvent) {
  return Array.from(event.dataTransfer?.types || []).includes('Files');
}

function onDragEnterShell(event: DragEvent) {
  if (!hasDraggedFiles(event)) {
    return;
  }
  dragDepth.value += 1;
  dragActive.value = true;
}

function onDragOverShell(event: DragEvent) {
  if (!hasDraggedFiles(event)) {
    return;
  }
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'copy';
  }
}

function onDragLeaveShell(event: DragEvent) {
  if (!hasDraggedFiles(event)) {
    return;
  }
  dragDepth.value = Math.max(0, dragDepth.value - 1);
  if (dragDepth.value === 0) {
    dragActive.value = false;
  }
}

async function onDropShell(event: DragEvent) {
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
}

async function createFolderAction() {
  const folderName = newFolderName.value.trim();
  if (!folderName) {
    return;
  }
  try {
    await createFolder(currentPath.value, folderName);
    showCreateFolderDialog.value = false;
    newFolderName.value = '';
    showSnackbar(t('folderCreated'), 'success');
    await fetchFiles(currentPath.value, false);
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('folderCreateError')), 'error');
  }
}

function openRenameDialog(file: FileItem) {
  renameTargetPath.value = file.path;
  renameName.value = file.name;
  showRenameDialog.value = true;
}

async function renameFileAction() {
  const nextName = renameName.value.trim();
  if (!renameTargetPath.value || !nextName) {
    return;
  }
  submittingRename.value = true;
  try {
    await renameFile(renameTargetPath.value, nextName);
    showRenameDialog.value = false;
    showSnackbar(t('renameSuccess'), 'success');
    await fetchFiles(currentPath.value, false);
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('renameError')), 'error');
  } finally {
    submittingRename.value = false;
  }
}

function scheduleTaskPolling(callback: () => Promise<void>) {
  const pollInterval = authState.taskPollIntervalMs.value || 1200;
  const id = window.setTimeout(async () => {
    await callback();
  }, pollInterval);
  taskPollTimerIds.value.push(id);
}

function pollTaskCompletion(taskId: string) {
  const poll = async () => {
    try {
      const task = await fetchTask(taskId);
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
}

async function createDeleteTask() {
  if (!hasSelection.value) {
    return;
  }
  try {
    const response = await createBatchDeleteTask(Array.from(selectedPaths.value));
    showDeleteDialog.value = false;
    clearSelection();
    showSnackbar(t('batchDeleteTaskCreated'), 'success');
    pollTaskCompletion(response.taskId);
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('batchDeleteTaskError')), 'error');
  }
}

async function loadTransferFolders(path: string) {
  transferLoading.value = true;
  try {
    transferFolders.value = (await listFiles({
      path,
      sortBy: 'name',
      order: 'asc',
      type: 'folder',
    })).filter((file) => file.isDirectory);
    transferPath.value = normalizePath(path);
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('fetchFilesError')), 'error');
  } finally {
    transferLoading.value = false;
  }
}

async function openTransfer(mode: TransferMode) {
  transferMode.value = mode;
  showTransferDialog.value = true;
  await loadTransferFolders(currentPath.value);
}

async function submitTransfer() {
  if (!hasSelection.value) {
    return;
  }
  submittingTransfer.value = true;
  try {
    const paths = Array.from(selectedPaths.value);
    const response = transferMode.value === 'copy'
      ? await createBatchCopyTask(paths, normalizePath(transferPath.value))
      : await createBatchMoveTask(paths, normalizePath(transferPath.value));
    showTransferDialog.value = false;
    clearSelection();
    showSnackbar(transferMode.value === 'copy' ? t('copyTaskCreated') : t('batchMoveTaskCreated'), 'success');
    pollTaskCompletion(response.taskId);
  } catch (error) {
    showSnackbar(
      resolveApiErrorMessage(error, transferMode.value === 'copy' ? t('copyTaskError') : t('batchMoveTaskError')),
      'error'
    );
  } finally {
    submittingTransfer.value = false;
  }
}

function downloadSelected() {
  const items = selectedItems.value.filter((item) => !item.isDirectory);
  if (items.length === 0) {
    showSnackbar(t('noDownloadableSelected'), 'warning');
    return;
  }
  items.forEach(triggerBrowserDownload);
}

function toggleFavorite(value: string) {
  const normalized = normalizePath(value);
  if (favorites.value.includes(normalized)) {
    favorites.value = favorites.value.filter((item) => item !== normalized);
    persistFavorites();
    return;
  }
  favorites.value = [normalized, ...favorites.value.filter((item) => item !== normalized)].slice(0, 12);
  persistFavorites();
  showSnackbar(t('favoriteAdded'), 'success');
}

function onResize() {
  isMobile.value = window.innerWidth < 960;
}

watch([search, sortBy, sortOrder, typeFilter, includeHidden], () => {
  scheduleFilterRefresh();
});

watch(
  () => [route.params.pathMatch, route.query.search, route.query.sortBy, route.query.order, route.query.type, route.query.hidden],
  () => {
    readQueryState();
    const path = getPathFromRoute();
    if (path !== currentPath.value || files.value.length === 0) {
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
  if (snackbarTimer !== null) {
    window.clearTimeout(snackbarTimer);
  }
  if (filterTimer !== null) {
    window.clearTimeout(filterTimer);
  }
});
</script>

<template>
  <div
    class="file-page"
    :style="{ '--accent': accentColor }"
    @dragenter.prevent="onDragEnterShell"
    @dragover.prevent="onDragOverShell"
    @dragleave.prevent="onDragLeaveShell"
    @drop.prevent="onDropShell"
  >
    <section class="hero-panel glass-panel">
      <div class="hero-head">
        <div>
          <div class="section-title">{{ t('filesHubTitle') }}</div>
          <div class="section-subtitle">{{ t('filesHubSubtitle') }}</div>
        </div>
        <div class="hero-actions">
          <button type="button" class="shad-btn" @click="router.back()">
            <History :size="16" />
            {{ t('historyBack') }}
          </button>
          <button type="button" class="shad-btn" @click="fetchFiles(parentPath, true)">
            <ArrowUp :size="16" />
            {{ t('goUp') }}
          </button>
          <button type="button" class="shad-btn" @click="toggleFavorite(currentPath)">
            <Star :size="16" />
            {{ favorites.includes(currentPath) ? t('favoriteSaved') : t('addFavorite') }}
          </button>
        </div>
      </div>

      <PathBreadcrumb :path="currentPath" :on-navigate="(path) => fetchFiles(path, true)" />

      <div class="controls-grid">
        <label class="input-shell search-shell">
          <Search :size="16" />
          <input v-model="search" :placeholder="t('searchPlaceholder')" type="text">
        </label>

        <label class="input-shell compact-field">
          <span>{{ t('sortBy') }}</span>
          <select v-model="sortBy">
            <option value="name">{{ t('sort_name') }}</option>
            <option value="size">{{ t('sort_size') }}</option>
            <option value="modified">{{ t('sort_modified') }}</option>
          </select>
        </label>

        <label class="input-shell compact-field">
          <span>{{ t('sortOrder') }}</span>
          <select v-model="sortOrder">
            <option value="asc">{{ t('order_asc') }}</option>
            <option value="desc">{{ t('order_desc') }}</option>
          </select>
        </label>

        <label class="input-shell compact-field">
          <span>{{ t('type') }}</span>
          <select v-model="typeFilter">
            <option v-for="option in filterTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>
        </label>

        <label class="toggle-row">
          <input v-model="includeHidden" type="checkbox">
          <span>{{ t('includeHidden') }}</span>
        </label>
      </div>

      <div class="toolbar-row">
        <button type="button" class="shad-btn" @click="selectAllVisible">
          <CheckSquare :size="16" />
          {{ t('selectAll') }}
        </button>
        <button type="button" class="shad-btn" :disabled="loading" @click="fetchFiles(currentPath, false)">
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

      <div v-if="uploading" class="upload-progress-shell">
        <div class="upload-track">
          <div class="upload-fill" :style="{ width: `${uploadProgress}%` }" />
        </div>
        <span>{{ uploadProgress }}%</span>
      </div>
    </section>

    <section class="workspace-grid">
      <article class="glass-panel workspace-main">
        <FileBrowserPane
          :files="files"
          :loading="loading"
          :is-mobile="isMobile"
          :is-selected="isSelected"
          :get-file-accent="getFileAccent"
          :get-matching-tag="getMatchingTag"
          :format-size="formatSize"
          :format-date="formatDate"
          @open="onFileClick"
          @toggle-selection="toggleSelection"
          @rename="openRenameDialog"
        />
      </article>

      <aside class="glass-panel workspace-side">
        <section class="side-section">
          <div class="side-title">{{ t('favorites') }}</div>
          <div class="favorite-list">
            <button
              v-for="favorite in favorites"
              :key="favorite"
              type="button"
              class="favorite-chip"
              @click="fetchFiles(favorite, true)"
            >
              {{ favorite }}
            </button>
            <div v-if="favorites.length === 0" class="side-note">{{ t('favoritesEmpty') }}</div>
          </div>
        </section>

        <section class="side-section">
          <div class="side-title">{{ t('inspectorTitle') }}</div>
          <div v-if="activeInspectorItem" class="inspector-card">
            <div class="inspector-name">{{ activeInspectorItem.name }}</div>
            <div class="inspector-meta">
              <span>{{ activeInspectorItem.isDirectory ? t('type_folder') : t(`type_${inferFileType(activeInspectorItem)}`) }}</span>
              <span>{{ activeInspectorItem.isDirectory ? '-' : formatSize(activeInspectorItem.size) }}</span>
            </div>
            <div class="inspector-meta">{{ formatDate(activeInspectorItem.modifiedTime) }}</div>
            <div class="inspector-actions">
              <button type="button" class="shad-btn" @click="openFile(activeInspectorItem)">{{ t('openItem') }}</button>
              <button type="button" class="shad-btn" @click="openRenameDialog(activeInspectorItem)">{{ t('rename') }}</button>
            </div>
          </div>
          <div v-else class="side-note">{{ t('noSelectionSummary') }}</div>
        </section>

        <section class="side-section">
          <div class="side-title">{{ t('foldersQuickView') }}</div>
          <div class="folder-grid-side">
            <button
              v-for="folder in folderEntries.slice(0, 8)"
              :key="folder.path"
              type="button"
              class="folder-card"
              @click="fetchFiles(folder.path, true)"
            >
              <span>{{ folder.name }}</span>
            </button>
            <div v-if="folderEntries.length === 0" class="side-note">{{ t('noFolderAtLevel') }}</div>
          </div>
        </section>
      </aside>
    </section>

    <input ref="fileInputRef" type="file" multiple class="hidden" @change="onFileInputChange">

    <div v-if="hasSelection" class="floating-action-bar">
      <span class="selection-count">{{ t('selectedCount', { count: selectedCount }) }}</span>
      <button type="button" class="shad-btn shad-btn-danger" @click="showDeleteDialog = true">
        <Trash2 :size="14" />
        {{ t('delete') }}
      </button>
      <button type="button" class="shad-btn" @click="openTransfer('move')">
        <MoveRight :size="14" />
        {{ t('move') }}
      </button>
      <button type="button" class="shad-btn" @click="openTransfer('copy')">
        <Copy :size="14" />
        {{ t('copy') }}
      </button>
      <button type="button" class="shad-btn" @click="downloadSelected">{{ t('download') }}</button>
      <button type="button" class="shad-btn" :disabled="selectedCount !== 1" @click="selectedSingleItem && openRenameDialog(selectedSingleItem)">
        {{ t('rename') }}
      </button>
      <button type="button" class="shad-btn" @click="clearSelection">{{ t('clear') }}</button>
    </div>

    <DestinationPickerDialog
      :open="showTransferDialog"
      :title="transferTitle"
      :path="transferPath"
      :folders="transferFolders"
      :busy="transferLoading"
      :submitting="submittingTransfer"
      @close="showTransferDialog = false"
      @update-path="(path) => { transferPath = normalizePath(path) }"
      @browse="loadTransferFolders"
      @confirm="submitTransfer"
    />

    <div v-if="showCreateFolderDialog" class="modal-overlay" @click.self="showCreateFolderDialog = false">
      <div class="modal-card">
        <h3>{{ t('newFolder') }}</h3>
        <label class="input-shell">
          <input v-model="newFolderName" type="text" :placeholder="t('folderName')" @keyup.enter="createFolderAction">
        </label>
        <p class="modal-hint">{{ t('folderNameHint') }}</p>
        <div class="modal-actions">
          <button type="button" class="shad-btn" @click="showCreateFolderDialog = false">{{ t('cancel') }}</button>
          <button type="button" class="shad-btn shad-btn-primary" @click="createFolderAction">{{ t('create') }}</button>
        </div>
      </div>
    </div>

    <div v-if="showRenameDialog" class="modal-overlay" @click.self="showRenameDialog = false">
      <div class="modal-card">
        <h3>{{ t('rename') }}</h3>
        <label class="input-shell">
          <input v-model="renameName" type="text" :placeholder="t('newName')" @keyup.enter="renameFileAction">
        </label>
        <p class="modal-hint">{{ t('renameHint') }}</p>
        <div class="modal-actions">
          <button type="button" class="shad-btn" @click="showRenameDialog = false">{{ t('cancel') }}</button>
          <button type="button" class="shad-btn shad-btn-primary" :disabled="submittingRename" @click="renameFileAction">
            {{ t('rename') }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="showDeleteDialog" class="modal-overlay" @click.self="showDeleteDialog = false">
      <div class="modal-card">
        <h3>{{ t('deleteSelectionTitle') }}</h3>
        <p class="modal-hint">{{ t('deleteSelectionHint', { count: selectedCount }) }}</p>
        <div class="modal-actions">
          <button type="button" class="shad-btn" @click="showDeleteDialog = false">{{ t('cancel') }}</button>
          <button type="button" class="shad-btn shad-btn-danger" @click="createDeleteTask">{{ t('confirmDelete') }}</button>
        </div>
      </div>
    </div>

    <ImagePreviewDialog
      :open="showImageDialog"
      :file="selectedImage"
      :index="selectedImageIndex"
      :total="imageFiles.length"
      :image-url="selectedImageUrl"
      :preview-scale="previewScale"
      :fit-to-screen="fitToScreen"
      :format-size="formatSize"
      :format-date="formatDate"
      @close="closeImageDialog"
      @previous="switchImage(-1)"
      @next="switchImage(1)"
      @zoom-in="zoomIn"
      @zoom-out="zoomOut"
      @toggle-fit="fitToScreen = !fitToScreen"
      @download="selectedImage && triggerBrowserDownload(selectedImage)"
    />

    <div v-if="snackbar.show" class="snackbar" :class="`snackbar-${snackbar.tone}`">
      {{ snackbar.message }}
    </div>

    <div v-if="dragActive" class="drag-upload-overlay">
      <Upload :size="28" />
      <div class="drag-title">{{ t('dragUploadOverlay') }}</div>
      <div class="drag-sub">{{ t('dragUploadSub') }}</div>
    </div>
  </div>
</template>

<style scoped>
.file-page {
  padding: 24px;
  display: grid;
  gap: 16px;
}

.hero-panel,
.workspace-main,
.workspace-side {
  padding: 18px;
}

.hero-head,
.hero-actions,
.toolbar-row,
.inspector-actions,
.upload-progress-shell {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.hero-head {
  justify-content: space-between;
  margin-bottom: 12px;
}

.controls-grid {
  margin-top: 14px;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(220px, 2fr) repeat(3, minmax(140px, 1fr)) auto;
}

.search-shell {
  min-width: 0;
}

.compact-field {
  gap: 8px;
}

.compact-field span,
.toggle-row,
.side-note,
.modal-hint {
  color: var(--text-2);
}

.compact-field select,
.search-shell input {
  width: 100%;
  border: none;
  background: transparent;
  color: inherit;
  outline: none;
}

.toggle-row {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 4px;
}

.toolbar-row {
  margin-top: 14px;
}

.upload-track {
  flex: 1;
  min-width: 180px;
  height: 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--surface-3) 84%, transparent);
  overflow: hidden;
}

.upload-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent), var(--accent-warm));
}

.workspace-grid {
  display: grid;
  gap: 16px;
  grid-template-columns: minmax(0, 1.6fr) minmax(280px, 0.8fr);
}

.workspace-side {
  display: grid;
  gap: 16px;
  align-content: start;
}

.side-section {
  display: grid;
  gap: 10px;
}

.side-title {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-3);
}

.favorite-list,
.folder-grid-side {
  display: grid;
  gap: 8px;
}

.favorite-chip,
.folder-card {
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--surface-2) 94%, transparent);
  color: var(--text-1);
  padding: 10px 12px;
  text-align: left;
  cursor: pointer;
}

.favorite-chip:hover,
.folder-card:hover {
  border-color: color-mix(in srgb, var(--accent) 34%, var(--border-soft));
}

.inspector-card {
  padding: 14px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-2) 96%, transparent);
}

.inspector-name {
  font-weight: 800;
  margin-bottom: 8px;
}

.inspector-meta {
  color: var(--text-2);
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.floating-action-bar {
  position: sticky;
  bottom: 18px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 12px;
  margin-inline: auto;
  width: fit-content;
  max-width: calc(100vw - 48px);
  border: 1px solid var(--border-soft);
  border-radius: 999px;
  background: color-mix(in srgb, var(--surface-1) 92%, transparent);
  box-shadow: var(--shadow-lg);
  z-index: 12;
}

.selection-count {
  color: var(--text-2);
  font-weight: 700;
  padding-inline: 8px;
}

.snackbar {
  position: fixed;
  right: 24px;
  bottom: 24px;
  padding: 12px 16px;
  border-radius: 16px;
  color: #fff;
  box-shadow: var(--shadow-lg);
  z-index: 30;
}

.snackbar-success {
  background: #166534;
}

.snackbar-error,
.snackbar-warning {
  background: #b45309;
}

.snackbar-info {
  background: #0f766e;
}

.drag-upload-overlay {
  position: fixed;
  inset: 16px;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 8px;
  border-radius: 32px;
  border: 1px dashed color-mix(in srgb, var(--accent) 44%, var(--border-soft));
  background: color-mix(in srgb, var(--surface-1) 78%, transparent);
  box-shadow: var(--shadow-xl);
  backdrop-filter: blur(20px);
  z-index: 24;
}

.drag-title {
  font-size: 1.2rem;
  font-weight: 800;
}

.drag-sub {
  color: var(--text-2);
}

.spin {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 1120px) {
  .controls-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .workspace-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .file-page {
    padding: 16px;
  }

  .controls-grid {
    grid-template-columns: 1fr;
  }

  .floating-action-bar {
    border-radius: 22px;
    width: calc(100vw - 32px);
  }
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
