<script setup lang="ts">
import {
  File,
  FileArchive,
  FileAudio,
  FileCode,
  FileImage,
  FileText,
  FileVideo,
  Folder,
  PencilLine,
} from 'lucide-vue-next';
import { inferFileType } from '../../composables/useFileVisuals';
import type { CustomColorTag } from '../../composables/useFileVisuals';
import type { FileItem } from '../../types';
import type { FileSortBy } from '../../types';
import { useLocale } from '../../composables/useLocale';
import { Button } from '@/components/ui/button';

const props = defineProps<{
  files: FileItem[];
  loading: boolean;
  isMobile: boolean;
  hasFiles: boolean;
  allSelected: boolean;
  isSelected: (filePath: string) => boolean;
  getFileAccent: (file: FileItem) => string;
  getMatchingTag: (file: FileItem) => CustomColorTag | null;
  formatSize: (size: number) => string;
  formatDate: (value: string) => string;
  sortBy: FileSortBy;
  sortOrder: 'asc' | 'desc';
}>();

const emit = defineEmits<{
  open: [file: FileItem, event: MouseEvent];
  toggleSelection: [filePath: string];
  toggleSelectAll: [];
  rename: [file: FileItem];
  sort: [field: FileSortBy];
}>();

const { t } = useLocale();

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
</script>

<template>
  <section class="browser-pane">
    <header class="browser-head">
      <div class="name-head">
        <input
          type="checkbox"
          class="header-checkbox"
          :checked="allSelected"
          :disabled="!hasFiles"
          @change="emit('toggleSelectAll')"
        >
        <Button
          variant="ghost"
          size="sm"
          :class="sortBy === 'name' ? 'sort-btn active' : 'sort-btn'"
          @click="emit('sort', 'name')"
        >
          {{ t('name') }}
          <span class="sort-arrow">{{ sortBy === 'name' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span>
        </Button>
      </div>
      <Button
        variant="ghost"
        size="sm"
        :class="sortBy === 'size' ? 'sort-btn active' : 'sort-btn'"
        @click="emit('sort', 'size')"
      >
        {{ t('size') }}
        <span class="sort-arrow">{{ sortBy === 'size' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span>
      </Button>
      <Button
        variant="ghost"
        size="sm"
        :class="sortBy === 'modified' ? 'sort-btn active' : 'sort-btn'"
        @click="emit('sort', 'modified')"
      >
        {{ t('modified') }}
        <span class="sort-arrow">{{ sortBy === 'modified' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span>
      </Button>
    </header>

    <div v-if="loading" class="browser-state">{{ t('loadingFiles') }}</div>
    <div v-else-if="files.length === 0" class="browser-state">{{ t('emptyFolderHint') }}</div>

    <div v-else-if="isMobile" class="mobile-cards">
      <article
        v-for="file in files"
        :key="file.path"
        class="mobile-card"
        :class="{ selected: isSelected(file.path) }"
        :style="{ '--row-accent': getFileAccent(file) }"
        @click="emit('open', file, $event)"
      >
        <div class="row-main">
          <button type="button" class="checkbox-hit" @click.stop="emit('toggleSelection', file.path)">
            <span class="checkbox-dot" :class="{ checked: isSelected(file.path) }" />
          </button>
          <component :is="iconFor(file)" :size="18" />
          <span class="file-name">{{ file.name }}</span>
          <Button variant="ghost" size="icon" class="icon-action-btn" @click.stop="emit('rename', file)">
            <PencilLine :size="14" />
          </Button>
        </div>
        <div class="row-meta">
          <span>{{ file.isDirectory ? '-' : formatSize(file.size) }}</span>
          <span>{{ formatDate(file.modifiedTime) }}</span>
        </div>
      </article>
    </div>

    <div v-else class="table-wrap">
      <table class="file-table">
        <tbody>
          <tr
            v-for="file in files"
            :key="file.path"
            class="file-row"
            :class="{ selected: isSelected(file.path) }"
            :style="{ '--row-accent': getFileAccent(file) }"
            @click="emit('open', file, $event)"
          >
            <td class="file-name-cell">
              <button type="button" class="checkbox-hit" @click.stop="emit('toggleSelection', file.path)">
                <span class="checkbox-dot" :class="{ checked: isSelected(file.path) }" />
              </button>
              <component :is="iconFor(file)" :size="18" />
              <span class="file-name">{{ file.name }}</span>
              <Button variant="ghost" size="icon" class="icon-action-btn" @click.stop="emit('rename', file)">
                <PencilLine :size="14" />
              </Button>
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
  </section>
</template>

<style scoped>
.browser-pane {
  min-height: 420px;
}

.browser-head {
  display: grid;
  grid-template-columns: 1.2fr 130px 180px;
  padding: 8px 10px;
}

.name-head {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.header-checkbox {
  width: 16px;
  height: 16px;
  border-radius: 5px;
  border: 1px solid var(--border-strong);
  display: inline-block;
  accent-color: var(--accent);
  margin: 0;
}

.sort-btn {
  justify-content: flex-start;
  gap: 4px;
  color: var(--text-3);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  font-weight: 700;
  min-height: 28px;
  padding: 0 4px;
  text-align: left;
  transition: color 0.15s ease;
}

.sort-btn:hover {
  color: var(--text-1);
}

.sort-btn.active {
  color: var(--accent);
}

.sort-arrow {
  font-size: 11px;
  opacity: 0.6;
}

.sort-btn.active .sort-arrow {
  opacity: 1;
}

.browser-state {
  padding: 24px 12px;
  color: var(--text-2);
}

.table-wrap {
  overflow: auto;
}

.file-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0 10px;
}

.file-row {
  cursor: pointer;
}

.file-row td {
  padding: 12px;
  background: color-mix(in srgb, var(--surface-2) 94%, transparent);
  border-top: 1px solid var(--border-soft);
  border-bottom: 1px solid var(--border-soft);
}

.file-row td:first-child {
  border-left: 1px solid var(--border-soft);
  border-radius: var(--radius-sm) 0 0 var(--radius-sm);
}

.file-row td:last-child {
  border-right: 1px solid var(--border-soft);
  border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
}

.file-row:hover td,
.file-row.selected td {
  background: color-mix(in srgb, var(--row-accent) 14%, var(--surface-2));
  border-color: color-mix(in srgb, var(--row-accent) 38%, var(--border-soft));
}

.file-name-cell,
.row-main {
  display: flex;
  align-items: center;
  gap: 10px;
}

.file-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-cards {
  display: grid;
  gap: 10px;
}

.mobile-card {
  padding: 14px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-2) 96%, transparent);
}

.mobile-card.selected {
  background: color-mix(in srgb, var(--row-accent) 14%, var(--surface-2));
  border-color: color-mix(in srgb, var(--row-accent) 38%, var(--border-soft));
}

.row-meta {
  margin-top: 10px;
  display: flex;
  justify-content: space-between;
  gap: 10px;
  font-size: 12px;
  color: var(--text-2);
}

.checkbox-hit,
.icon-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: inherit;
}

.icon-action-btn {
  width: 28px;
  height: 28px;
}

.checkbox-dot {
  width: 16px;
  height: 16px;
  border-radius: 5px;
  border: 1px solid var(--border-strong);
  display: inline-block;
}

.checkbox-dot.checked {
  background: var(--accent);
  border-color: var(--accent);
}

.tag-pill {
  border: 1px solid currentColor;
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 11px;
  font-weight: 700;
}

@media (max-width: 860px) {
  .browser-head {
    grid-template-columns: 1fr 90px 140px;
  }
}
</style>
