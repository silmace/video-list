<script setup lang="ts">
import type { FileItem } from '../../types';
import PathBreadcrumb from '../PathBreadcrumb.vue';
import { useLocale } from '../../composables/useLocale';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';

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

      <Input
        :model-value="path"
        :placeholder="t('destinationPath')"
        @update:modelValue="emit('updatePath', $event)"
      />

      <div class="folder-grid">
        <Button variant="outline" class="folder-chip" @click="emit('browse', '/')">{{ t('home') }}</Button>
        <Button
          v-for="folder in folders"
          :key="folder.path"
          variant="outline"
          class="folder-chip"
          @click="emit('browse', folder.path)"
        >
          {{ folder.name }}
        </Button>
      </div>

      <div v-if="busy" class="dialog-note">{{ t('loadingFiles') }}</div>
      <div v-else-if="folders.length === 0" class="dialog-note">{{ t('noFoldersHere') }}</div>

      <div class="modal-actions">
        <Button variant="outline" @click="emit('close')">{{ t('cancel') }}</Button>
        <Button :disabled="submitting" @click="emit('confirm')">
          {{ t('createTask') }}
        </Button>
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
  border-radius: 999px;
  justify-content: flex-start;
}
</style>
