<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { VideoCodecOption, VideoExportMode, VideoSegment } from '../types';
import { buildMediaUrl } from '../services/api';
import { createVideoTask as createVideoTaskRequest, fetchVideoOptions } from '../services/video';
import Artplayer from 'artplayer';
import PathBreadcrumb from './PathBreadcrumb.vue';
import { useLocale } from '../composables/useLocale';
import { Alert } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';

const route = useRoute();
const router = useRouter();
const videoPath = ref('');
const segments = ref<VideoSegment[]>([{ startTime: '00:00:00', endTime: '00:00:00' }]);
const exportMode = ref<VideoExportMode>('copy');
const selectedCodec = ref('copy');
const codecOptions = ref<VideoCodecOption[]>([]);
const artRef = ref<HTMLDivElement | null>(null);
const loading = ref(false);
const snackbar = ref({ show: false, message: '', color: '' });
const latestTaskId = ref('');
const currentPlaybackTime = ref('00:00:00');
let art: Artplayer | null = null;
const { t } = useLocale();
const mediaUrl = computed(() => buildMediaUrl(videoPath.value));

const availableCodecOptions = computed(() => codecOptions.value.filter((item) => item.mode === exportMode.value));
const segmentErrors = computed(() => segments.value.map((segment) => validateSegment(segment)));
const hasInvalidSegments = computed(() => segmentErrors.value.some(Boolean));
const hasValidCodecSelection = computed(() => availableCodecOptions.value.some((item) => item.id === selectedCodec.value));
const totalClipDuration = computed(() => {
  return segments.value.reduce((sum, segment) => sum + Math.max(0, parseTime(segment.endTime) - parseTime(segment.startTime)), 0);
});

const getVideoPath = () => {
  if (!route.params.pathMatch) return '';
  const rawPath = Array.isArray(route.params.pathMatch)
    ? route.params.pathMatch.join('/')
    : route.params.pathMatch;
  const decodedPath = rawPath
    .split('/')
    .map((segment) => {
      try {
        return decodeURIComponent(segment);
      } catch {
        return segment;
      }
    })
    .join('/');
  return `/${decodedPath}`;
};

const showSnackbar = (message: string, color: string) => {
  snackbar.value = { show: true, message, color };
};

const resolveApiErrorMessage = (error: unknown, fallback: string) => {
  const maybe = error as { response?: { data?: { error?: string } } };
  return maybe?.response?.data?.error || fallback;
};

onMounted(() => {
  videoPath.value = getVideoPath();

  if (!videoPath.value) {
    router.push('/');
    return;
  }

  if (artRef.value) {
    art = new Artplayer({
      container: artRef.value,
      url: mediaUrl.value,
      volume: 0.5,
      autoplay: false,
      pip: true,
      screenshot: true,
      setting: true,
      flip: true,
      playbackRate: true,
      aspectRatio: true,
      fullscreen: true,
      fullscreenWeb: true,
      subtitleOffset: true,
      miniProgressBar: true,
      mutex: true,
      backdrop: true,
      playsInline: true,
      autoSize: true,
      autoMini: true,
      autoOrientation: true,
      theme: '#0f766e',
    });

    art.on('video:error', () => {
      showSnackbar('Video failed to load, check format/token/path.', 'error');
    });

    art.on('video:timeupdate', () => {
      currentPlaybackTime.value = formatTime(art?.currentTime || 0);
    });
  }
});

onUnmounted(() => {
  if (art) {
    art.destroy();
    art = null;
  }
});

const addSegment = () => {
  segments.value.push({ startTime: '00:00:00', endTime: '00:00:00' });
};

const duplicateSegment = (index: number) => {
  const item = segments.value[index];
  segments.value.splice(index + 1, 0, { ...item });
};

const moveSegment = (index: number, step: number) => {
  const targetIndex = index + step;
  if (targetIndex < 0 || targetIndex >= segments.value.length) {
    return;
  }
  const draft = [...segments.value];
  const [item] = draft.splice(index, 1);
  draft.splice(targetIndex, 0, item);
  segments.value = draft;
};

const removeSegment = (index: number) => {
  if (segments.value.length === 1) return;
  segments.value.splice(index, 1);
};

const formatTime = (time: number): string => {
  const hours = Math.floor(time / 3600);
  const minutes = Math.floor((time % 3600) / 60);
  const seconds = Math.floor(time % 60);
  return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
};

const parseTime = (value: string): number => {
  const match = value.match(/^(\d{2}):(\d{2}):(\d{2})$/);
  if (!match) {
    return -1;
  }
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  const seconds = Number(match[3]);
  return hours * 3600 + minutes * 60 + seconds;
};

const validateSegment = (segment: VideoSegment): string => {
  const start = parseTime(segment.startTime);
  const end = parseTime(segment.endTime);
  if (start < 0 || end < 0) {
    return t('invalidTimeFormat');
  }
  if (end <= start) {
    return t('invalidSegmentRange');
  }
  return '';
};

const formatDuration = (seconds: number) => {
  return formatTime(seconds);
};

const setCurrentTime = (index: number, type: 'start' | 'end') => {
  if (!art) return;
  const currentTime = art.currentTime;
  segments.value[index][type === 'start' ? 'startTime' : 'endTime'] = formatTime(currentTime);
};

const createVideoTask = async () => {
  if (hasInvalidSegments.value) {
    showSnackbar(t('invalidSegmentRange'), 'warning');
    return;
  }
  if (!hasValidCodecSelection.value) {
    showSnackbar(t('noCodecAvailable'), 'warning');
    return;
  }
  loading.value = true;
  try {
    const response = await createVideoTaskRequest({
      videoPath: videoPath.value,
      segments: segments.value,
      exportMode: exportMode.value,
      videoCodec: selectedCodec.value,
    });
    latestTaskId.value = response.taskId;
    showSnackbar(t('videoTaskCreated'), 'success');
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('videoTaskCreateError')), 'error');
  } finally {
    loading.value = false;
  }
};

const getFileName = () => videoPath.value?.split('/').pop() || t('videoEditorFallback');

const handlePathNavigation = (path: string) => {
  router.push(path);
};

const openTaskCenter = () => {
  router.push('/tasks');
};

const pickPreferredCodec = (mode: VideoExportMode): string => {
  const candidates = codecOptions.value.filter((item) => item.mode === mode && item.available);
  if (candidates.length === 0) {
    return '';
  }
  if (mode === 'copy') {
    return candidates[0].id;
  }
  const priority = ['h264_nvenc', 'hevc_nvenc', 'h264_qsv', 'hevc_qsv', 'h264_amf', 'hevc_amf', 'h264', 'h265', 'av1'];
  for (const id of priority) {
    const matched = candidates.find((item) => item.id === id);
    if (matched) {
      return matched.id;
    }
  }
  return candidates[0].id;
};

const selectExportMode = (mode: VideoExportMode) => {
  exportMode.value = mode;
  selectedCodec.value = pickPreferredCodec(mode);
};

onMounted(async () => {
  try {
    codecOptions.value = await fetchVideoOptions();
    selectedCodec.value = pickPreferredCodec(exportMode.value);
  } catch {
    codecOptions.value = [];
  }
});
</script>

<template>
  <div class="app-page grid gap-4">
    <Card class="glass-panel p-4">
      <PathBreadcrumb :path="videoPath" :onNavigate="handlePathNavigation" />
    </Card>

    <Card class="glass-panel p-4 md:p-5">
      <div class="mb-4 flex flex-wrap items-center gap-2">
        <h1 class="text-2xl font-extrabold tracking-tight">{{ getFileName() }}</h1>
        <Badge variant="secondary">{{ t('currentPlaybackTime') }} {{ currentPlaybackTime }}</Badge>
        <Badge>{{ t('totalClipDuration') }} {{ formatDuration(totalClipDuration) }}</Badge>
      </div>

      <div ref="artRef" class="video-player mb-6" />

      <section class="export-panel mb-6">
        <div class="mb-2 text-sm font-semibold text-[var(--text-2)]">{{ t('exportModeTitle') }}</div>
        <div class="mode-btn-row mb-4">
          <Button :variant="exportMode === 'copy' ? 'default' : 'outline'" class="mode-btn" @click="selectExportMode('copy')">
            {{ t('exportMode_copy') }}
          </Button>
          <Button :variant="exportMode === 'transcode' ? 'default' : 'outline'" class="mode-btn" @click="selectExportMode('transcode')">
            {{ t('exportMode_transcode') }}
          </Button>
        </div>

        <div class="mb-2 text-sm font-semibold text-[var(--text-2)]">{{ t('videoCodecTitle') }}</div>
        <div class="codec-chip-row">
          <Button
            v-for="codec in availableCodecOptions"
            :key="codec.id"
            :variant="codec.id === selectedCodec ? 'default' : 'outline'"
            size="sm"
            @click="selectedCodec = codec.id"
          >
            {{ codec.label }}
          </Button>
        </div>
      </section>

      <section class="grid gap-3">
        <article v-for="(segment, index) in segments" :key="index" class="segment-card rounded-xl border p-4">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div class="font-semibold">
              {{ t('segment') }} {{ index + 1 }} · {{ formatDuration(Math.max(0, parseTime(segment.endTime) - parseTime(segment.startTime))) }}
            </div>
            <div class="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" :disabled="index === 0" @click.stop="moveSegment(index, -1)">Up</Button>
              <Button variant="outline" size="sm" :disabled="index === segments.length - 1" @click.stop="moveSegment(index, 1)">Down</Button>
              <Button variant="secondary" size="sm" @click.stop="duplicateSegment(index)">Copy</Button>
              <Button variant="destructive" size="sm" :disabled="segments.length === 1" @click.stop="removeSegment(index)">Delete</Button>
            </div>
          </div>

          <div class="segment-grid">
            <label class="field-shell">
              <span>{{ t('startTime') }}</span>
              <input v-model="segment.startTime" type="text" placeholder="00:00:00">
              <Button variant="outline" size="sm" class="w-fit" @click="setCurrentTime(index, 'start')">{{ t('setCurrent') }}</Button>
            </label>

            <label class="field-shell">
              <span>{{ t('endTime') }}</span>
              <input v-model="segment.endTime" type="text" placeholder="00:00:00">
              <Button variant="outline" size="sm" class="w-fit" @click="setCurrentTime(index, 'end')">{{ t('setCurrent') }}</Button>
            </label>
          </div>

          <Alert v-if="segmentErrors[index]" variant="warning" class="mt-3">
            {{ segmentErrors[index] }}
          </Alert>
        </article>
      </section>

      <Alert v-if="latestTaskId" class="mt-4">
        {{ t('latestTaskId') }}: <code>{{ latestTaskId }}</code>
      </Alert>

      <div class="editor-actions mt-5">
        <Button variant="outline" @click="addSegment">{{ t('addSegment') }}</Button>
        <div class="action-right">
          <Button variant="secondary" @click="openTaskCenter">{{ t('taskCenterBtn') }}</Button>
          <Button :disabled="loading || hasInvalidSegments || !hasValidCodecSelection" @click="createVideoTask">
            {{ loading ? '...' : t('createVideoTask') }}
          </Button>
        </div>
      </div>
    </Card>

    <div v-if="snackbar.show" class="snackbar" :class="`snackbar-${snackbar.color}`">
      {{ snackbar.message }}
    </div>
  </div>
</template>

<style scoped>
.video-player {
  aspect-ratio: 16/9;
  background-color: black;
  border-radius: 12px;
  overflow: hidden;
}

.export-panel {
  padding: 16px;
  border-radius: 18px;
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-2) 92%, transparent);
}

.segment-card {
  border-color: var(--border-soft);
  background: color-mix(in srgb, var(--surface-2) 88%, transparent);
}

.segment-grid {
  display: grid;
  gap: 10px;
  grid-template-columns: 1fr;
}

.field-shell {
  display: grid;
  gap: 6px;
  font-size: 13px;
  font-weight: 700;
  color: var(--text-2);
}

.field-shell input {
  width: 100%;
  min-height: 38px;
  border-radius: 10px;
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-3) 90%, transparent);
  padding: 8px 10px;
  color: var(--text-1);
}

.mode-btn-row,
.codec-chip-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.editor-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
}

.action-right {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.snackbar {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 40;
  border-radius: 10px;
  padding: 10px 12px;
  color: #fff;
  box-shadow: var(--shadow-lg);
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

@media (max-width: 960px) {
  .segment-grid {
    grid-template-columns: 1fr;
  }

  .mode-btn-row,
  .codec-chip-row {
    display: grid;
    grid-template-columns: 1fr;
  }

  .mode-btn {
    width: 100%;
  }

  .editor-actions {
    display: grid;
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .editor-actions .action-right,
  .editor-actions .mode-btn {
    width: 100%;
  }

  .editor-actions .action-right :deep(button),
  .editor-actions :deep(button) {
    width: 100%;
  }
}

@media (min-width: 900px) {
  .segment-grid {
    grid-template-columns: 1fr 1fr;
  }
}
</style>
