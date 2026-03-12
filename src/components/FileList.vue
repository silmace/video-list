<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
  ChevronLeft,
  ChevronRight,
  Copy,
  FolderPlus,
  MoveRight,
  RefreshCw,
  Settings2,
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
const sortBy = ref<FileSortBy>('name');
const sortOrder = ref<FileSortOrder>('asc');
const typeFilter = ref<FileFilterType>('all');
const isMobile = ref(window.innerWidth < 960);
const toolbarExpanded = ref(false);
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
const imageFiles = computed(() => files.value.filter((file) => isImage(file.name) && !file.isDirectory));
const selectedImage = computed(() => imageFiles.value[selectedImageIndex.value] || null);
const selectedImageUrl = computed(() => (selectedImage.value ? buildMediaUrl(selectedImage.value.path) : ''));
const allVisibleSelected = computed(() => files.value.length > 0 && selectedPaths.value.size === files.value.length);
const accentColor = computed(() => {
  if (selectedItems.value.length > 0) {
    return getFileAccent(selectedItems.value[0]);
  }
  return getDominantAccent(files.value);
});
const transferTitle = computed(() => (transferMode.value === 'copy' ? t('copySelection') : t('moveSelection')));

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

function handleSort(field: FileSortBy) {
  if (sortBy.value === field) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc';
    return;
  }
  sortBy.value = field;
  sortOrder.value = field === 'name' ? 'asc' : 'desc';
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

function readQueryState() {
  syncingRouteQuery = true;
  sortBy.value = route.query.sortBy === 'size' || route.query.sortBy === 'modified' ? route.query.sortBy : 'name';
  sortOrder.value = route.query.order === 'desc' ? 'desc' : 'asc';
  typeFilter.value = typeof route.query.type === 'string' ? (route.query.type as FileFilterType) : 'all';
  syncingRouteQuery = false;
}

function buildRouteQuery() {
  return {
    ...(sortBy.value !== 'name' ? { sortBy: sortBy.value } : {}),
    ...(sortOrder.value !== 'asc' ? { order: sortOrder.value } : {}),
    ...(typeFilter.value !== 'all' ? { type: typeFilter.value } : {}),
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
      sortBy: sortBy.value,
      order: sortOrder.value,
      type: typeFilter.value,
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

function toggleSelectAllVisible() {
  if (allVisibleSelected.value) {
    clearSelection();
    return;
  }
  selectAllVisible();
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
  fitToScreen.value = false;
  previewScale.value = Math.min(3, Number((previewScale.value + 0.2).toFixed(1)));
}

function zoomOut() {
  fitToScreen.value = false;
  previewScale.value = Math.max(0.4, Number((previewScale.value - 0.2).toFixed(1)));
}

function toggleFitMode() {
  fitToScreen.value = !fitToScreen.value;
  if (!fitToScreen.value && previewScale.value === 1) {
    previewScale.value = 1.1;
  }
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
  const nextMobile = window.innerWidth < 960;
  if (nextMobile && !isMobile.value) {
    toolbarExpanded.value = false;
  }
  isMobile.value = nextMobile;
}

watch([sortBy, sortOrder, typeFilter], () => {
  scheduleFilterRefresh();
});

watch(
  () => [route.params.pathMatch, route.query.sortBy, route.query.order, route.query.type],
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
    <aside class="left-toolbar-box" :class="{ open: toolbarExpanded }">
      <button
        type="button"
        class="toolbar-toggle-btn"
        :aria-label="toolbarExpanded ? t('collapseToolbar') : t('expandToolbar')"
        @click="toolbarExpanded = !toolbarExpanded"
      >
        <Settings2 :size="16" />
        <span v-if="toolbarExpanded">{{ t('toolbar') }}</span>
        <ChevronLeft v-if="toolbarExpanded" :size="14" />
        <ChevronRight v-else :size="14" />
      </button>

      <div v-if="toolbarExpanded" class="toolbar-list">
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
        <button type="button" class="shad-btn" @click="toggleFavorite(currentPath)">
          <Star :size="16" />
          {{ favorites.includes(currentPath) ? t('favoriteSaved') : t('addFavorite') }}
        </button>

        <div v-if="uploading" class="upload-progress-shell">
          <div class="upload-track">
            <div class="upload-fill" :style="{ width: `${uploadProgress}%` }" />
          </div>
          <span>{{ uploadProgress }}%</span>
        </div>
      </div>
    </aside>

    <section class="hero-panel glass-panel">
      <PathBreadcrumb :path="currentPath" :on-navigate="(path) => fetchFiles(path, true)" />
      <div class="hero-summary">
        <span>{{ t('itemsCount', { count: files.length }) }}</span>
        <span>{{ t('selectedCount', { count: selectedCount }) }}</span>
      </div>
    </section>

    <section class="workspace-grid">
      <article class="glass-panel workspace-main">
        <FileBrowserPane
          :files="files"
          :loading="loading"
          :is-mobile="isMobile"
          :has-files="files.length > 0"
          :all-selected="allVisibleSelected"
          :is-selected="isSelected"
          :get-file-accent="getFileAccent"
          :get-matching-tag="getMatchingTag"
          :format-size="formatSize"
          :format-date="formatDate"
          :sort-by="sortBy"
          :sort-order="sortOrder"
          @sort="handleSort"
          @open="onFileClick"
          @toggle-selection="toggleSelection"
          @toggle-select-all="toggleSelectAllVisible"
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
      @toggle-fit="toggleFitMode"
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
  position: relative;
  padding: 20px;
  padding-left: 86px;
  display: grid;
  gap: 14px;
}

.upload-track {
  flex: 1;
  min-width: 120px;
  height: 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--surface-3) 84%, transparent);
  overflow: hidden;
}

.upload-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent), var(--accent-warm));
}

.left-toolbar-box {
  position: fixed;
  left: 16px;
  top: 88px;
  z-index: 15;
  width: 58px;
  border: 1px solid var(--border-soft);
  border-radius: 18px;
  background: color-mix(in srgb, var(--surface-1) 93%, transparent);
  backdrop-filter: blur(10px);
  box-shadow: var(--shadow-lg);
  transition: width 0.2s ease;
}

.left-toolbar-box.open {
  width: 230px;
}

.toolbar-toggle-btn {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: transparent;
  border: none;
  color: var(--text-1);
  padding: 11px 10px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-soft);
}

.toolbar-list {
  display: grid;
  gap: 8px;
  padding: 10px;
}

.upload-progress-shell {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-2);
  font-size: 12px;
}

.hero-panel,
.workspace-main,
.workspace-side {
  padding: 14px;
}

.hero-summary {
  margin-top: 10px;
  display: flex;
  gap: 18px;
  flex-wrap: wrap;
  color: var(--text-2);
  font-size: 13px;
}

.workspace-grid {
  display: grid;
  gap: 14px;
  grid-template-columns: minmax(0, 1.9fr) minmax(220px, 0.6fr);
}

.workspace-side {
  height: fit-content;
  display: flex;
  flex-direction: column;
  gap: 12px;
  align-content: start;
}

.side-section {
  display: grid;
  gap: 8px;
}

.side-title {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-3);
}

.favorite-list {
  display: grid;
  gap: 8px;
}

.favorite-chip {
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-sm);
  background: color-mix(in srgb, var(--surface-2) 94%, transparent);
  color: var(--text-1);
  padding: 9px 10px;
  text-align: left;
  cursor: pointer;
}

.favorite-chip:hover {
  border-color: color-mix(in srgb, var(--accent) 34%, var(--border-soft));
}

.side-note,
.modal-hint {
  color: var(--text-2);
}

.floating-action-bar {
  position: fixed;
  left: 50%;
  transform: translateX(-50%);
  bottom: 18px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  max-width: calc(100vw - 40px);
  padding: 10px;
  border: 1px solid var(--border-soft);
  border-radius: 16px;
  background: color-mix(in srgb, var(--surface-1) 92%, transparent);
  box-shadow: var(--shadow-lg);
  z-index: 25;
}

.selection-count {
  color: var(--text-2);
  font-size: 12px;
  font-weight: 700;
}

.snackbar {
  position: fixed;
  right: 16px;
  bottom: 16px;
  padding: 10px 12px;
  border-radius: 12px;
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
  font-size: 1.05rem;
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
  .workspace-grid {
    grid-template-columns: 1fr;
  }

  .workspace-side {
    order: -1;
  }
}

@media (max-width: 720px) {
  .file-page {
    padding: 12px;
    padding-bottom: 136px;
  }

  .left-toolbar-box {
    top: auto;
    left: 10px;
    right: auto;
    bottom: 78px;
    width: 56px;
    border-radius: 14px;
  }

  .left-toolbar-box.open {
    width: min(220px, 70vw);
  }

  .toolbar-toggle-btn {
    width: 44px;
    min-height: 36px;
    padding: 8px;
    margin: 6px;
    border-bottom: none;
    border-radius: 10px;
  }

  .left-toolbar-box.open .toolbar-toggle-btn {
    width: calc(100% - 12px);
    border-bottom: 1px solid var(--border-soft);
  }

  .toolbar-list {
    grid-template-columns: 1fr 1fr;
  }

  .toolbar-list .shad-btn {
    width: 100%;
  }

  .floating-action-bar {
    width: calc(100vw - 22px);
    border-radius: 14px;
    gap: 6px;
    padding: 8px;
  }

  .floating-action-bar .shad-btn {
    font-size: 12px;
    padding: 7px 9px;
  }

  .selection-count {
    width: 100%;
  }
}
</style>
