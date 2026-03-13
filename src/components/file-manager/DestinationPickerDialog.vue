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
  <v-dialog
    :model-value="open"
    max-width="720"
    @update:model-value="(value) => { if (!value) emit('close') }"
  >
    <v-card class="browse-dialog">
      <v-card-title class="pb-1">{{ title }}</v-card-title>
      <v-card-text>
        <p class="dialog-note mb-3">{{ t('destinationHint') }}</p>

        <PathBreadcrumb :path="path" :on-navigate="(nextPath) => emit('browse', nextPath)" />

        <v-text-field
          class="mt-3"
          :model-value="path"
          :label="t('destinationPath')"
          variant="outlined"
          density="comfortable"
          hide-details="auto"
          @update:model-value="(value) => emit('updatePath', String(value ?? ''))"
        />

        <div class="folder-grid">
          <v-btn size="small" variant="outlined" class="folder-chip" @click="emit('browse', '/')">{{ t('home') }}</v-btn>
          <v-btn
            v-for="folder in folders"
            :key="folder.path"
            size="small"
            variant="outlined"
            class="folder-chip"
            @click="emit('browse', folder.path)"
          >
            {{ folder.name }}
          </v-btn>
        </div>

        <div v-if="busy" class="dialog-note mt-2">{{ t('loadingFiles') }}</div>
        <div v-else-if="folders.length === 0" class="dialog-note mt-2">{{ t('noFoldersHere') }}</div>
      </v-card-text>
      <v-card-actions class="justify-end">
        <v-btn variant="text" @click="emit('close')">{{ t('cancel') }}</v-btn>
        <v-btn color="primary" variant="tonal" :loading="submitting" :disabled="submitting" @click="emit('confirm')">
          {{ t('createTask') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.browse-dialog {
  width: min(720px, 92vw);
}

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
  text-transform: none;
  letter-spacing: 0;
}
</style>
