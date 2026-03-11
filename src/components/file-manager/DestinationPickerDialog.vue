<script setup lang="ts">
import type { FileItem } from '../../types';
import PathBreadcrumb from '../PathBreadcrumb.vue';
import { useLocale } from '../../composables/useLocale';

defineProps<{
  open: boolean;
  title: string;
  path: string;
  folders: FileItem[];
  busy: boolean;
  submitting: boolean;
}>();

const emit = defineEmits<{
  close: [];
  updatePath: [path: string];
  browse: [path: string];
  confirm: [];
}>();

const { t } = useLocale();
</script>

<template>
  <div v-if="open" class="modal-overlay" @click.self="emit('close')">
    <div class="modal-card browse-dialog">
      <div class="dialog-head">
        <div>
          <h3>{{ title }}</h3>
          <p>{{ t('destinationHint') }}</p>
        </div>
      </div>

      <PathBreadcrumb :path="path" :on-navigate="(nextPath) => emit('browse', nextPath)" />

      <label class="input-shell">
        <input :value="path" type="text" :placeholder="t('destinationPath')" @input="emit('updatePath', ($event.target as HTMLInputElement).value)">
      </label>

      <div class="folder-grid">
        <button type="button" class="folder-chip" @click="emit('browse', '/')">{{ t('home') }}</button>
        <button
          v-for="folder in folders"
          :key="folder.path"
          type="button"
          class="folder-chip"
          @click="emit('browse', folder.path)"
        >
          {{ folder.name }}
        </button>
      </div>

      <div v-if="busy" class="dialog-note">{{ t('loadingFiles') }}</div>
      <div v-else-if="folders.length === 0" class="dialog-note">{{ t('noFoldersHere') }}</div>

      <div class="modal-actions">
        <button type="button" class="shad-btn" @click="emit('close')">{{ t('cancel') }}</button>
        <button type="button" class="shad-btn shad-btn-primary" :disabled="submitting" @click="emit('confirm')">
          {{ t('createTask') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.browse-dialog {
  width: min(720px, 92vw);
}

.dialog-head p,
.dialog-note {
  color: var(--text-2);
}

.folder-grid {
  margin-top: 14px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.folder-chip {
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-2) 94%, transparent);
  color: var(--text-1);
  border-radius: 999px;
  padding: 8px 12px;
  cursor: pointer;
}
</style>
