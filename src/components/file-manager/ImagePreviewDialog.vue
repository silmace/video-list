<script setup lang="ts">
import { ArrowLeft, ArrowRight, Download, X, ZoomIn, ZoomOut } from 'lucide-vue-next';
import type { FileItem } from '../../types';
import { useLocale } from '../../composables/useLocale';
import { Button } from '@/components/ui/button';

defineProps<{
  open: boolean;
  file: FileItem | null;
  index: number;
  total: number;
  imageUrl: string;
  previewScale: number;
  fitToScreen: boolean;
  formatSize: (size: number) => string;
  formatDate: (value: string) => string;
}>();

const emit = defineEmits<{
  close: [];
  previous: [];
  next: [];
  zoomIn: [];
  zoomOut: [];
  toggleFit: [];
  download: [];
}>();

const { t } = useLocale();
</script>

<template>
  <div v-if="open && file" class="modal-overlay image-modal" @click.self="emit('close')">
    <div class="image-modal-card">
      <header class="image-header">
        <div>
          <div class="image-name">{{ file.name }}</div>
          <div class="image-meta">
            <span>{{ index + 1 }} / {{ total }}</span>
            <span>{{ formatSize(file.size) }}</span>
            <span>{{ formatDate(file.modifiedTime) }}</span>
          </div>
        </div>
        <div class="image-tools">
          <Button variant="outline" size="sm" class="tool-btn" @click="emit('previous')"><ArrowLeft :size="14" /></Button>
          <Button variant="outline" size="sm" class="tool-btn" @click="emit('next')"><ArrowRight :size="14" /></Button>
          <Button variant="outline" size="sm" class="tool-btn" @click="emit('zoomOut')"><ZoomOut :size="14" /></Button>
          <Button variant="outline" size="sm" class="tool-btn" @click="emit('zoomIn')"><ZoomIn :size="14" /></Button>
          <Button variant="outline" size="sm" class="tool-btn" @click="emit('toggleFit')">{{ fitToScreen ? t('originalSize') : t('fitWindow') }}</Button>
          <Button variant="outline" size="sm" class="tool-btn" @click="emit('download')"><Download :size="14" /></Button>
          <Button variant="outline" size="sm" class="tool-btn" @click="emit('close')"><X :size="14" /></Button>
        </div>
      </header>

      <div class="image-viewer">
        <img
          :src="imageUrl"
          :alt="file.name"
          :class="{ fit: fitToScreen }"
          :style="{ transform: fitToScreen ? 'none' : `scale(${previewScale})` }"
        >
      </div>
    </div>
  </div>
</template>

<style scoped>
.image-modal-card {
  width: min(1200px, 96vw);
  max-height: 92vh;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  border-radius: 28px;
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-1) 92%, transparent);
  box-shadow: var(--shadow-xl);
}

.image-header,
.image-tools,
.image-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.image-header {
  justify-content: space-between;
}

.image-name {
  font-size: 1.05rem;
  font-weight: 800;
}

.image-meta {
  color: var(--text-2);
  font-size: 12px;
}

.tool-btn {
  border-radius: 999px;
}

.image-viewer {
  flex: 1;
  overflow: auto;
  display: grid;
  place-items: center;
  min-height: 320px;
  border-radius: 20px;
  background: color-mix(in srgb, var(--bg-contrast) 88%, transparent);
}

.image-viewer img {
  max-width: none;
  transform-origin: center center;
}

.image-viewer img.fit {
  max-width: 100%;
  max-height: 72vh;
}
</style>