<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useDisplay } from 'vuetify';
import type { VideoCodecOption, VideoExportMode, VideoSegment } from '../types';
import { buildMediaUrl } from '../services/api';
import { createVideoTask as createVideoTaskRequest, fetchVideoOptions } from '../services/video';
import Artplayer from 'artplayer';
import PathBreadcrumb from './PathBreadcrumb.vue';
import { useLocale } from '../composables/useLocale';

type ArtplayerInstance = {
  currentTime: number;
  url: string;
  destroy: (removeHtml?: boolean) => void;
  on: (event: string, handler: (...args: unknown[]) => void) => unknown;
};

type ArtplayerConstructor = new (option: Record<string, unknown>) => ArtplayerInstance;

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
const artFailed = ref(false);
const fallbackVideoRef = ref<HTMLVideoElement | null>(null);
let art: ArtplayerInstance | null = null;
const { t } = useLocale();
const { smAndDown } = useDisplay();

const availableCodecOptions = computed(() => codecOptions.value.filter((item) => item.mode === exportMode.value));
const segmentErrors = computed(() => segments.value.map((segment) => validateSegment(segment)));
const hasInvalidSegments = computed(() => segmentErrors.value.some(Boolean));
const hasValidCodecSelection = computed(() => availableCodecOptions.value.some((item) => item.id === selectedCodec.value));
const breadcrumbPath = computed(() => {
  if (!videoPath.value) {
    return '/';
  }
  const segments = videoPath.value.split('/').filter(Boolean);
  if (segments.length <= 1) {
    return '/';
  }
  return `/${segments.slice(0, -1).join('/')}`;
});
const totalClipDuration = computed(() => {
  return segments.value.reduce((sum, segment) => sum + Math.max(0, parseTime(segment.endTime) - parseTime(segment.startTime)), 0);
});

const getVideoPath = () => {
  if (!route.params.pathMatch) return '';
  const rawSegments = Array.isArray(route.params.pathMatch)
    ? route.params.pathMatch
    : [route.params.pathMatch];

  const decodedSegments = rawSegments
    .map((segment) => {
      const value = String(segment || '').trim();
      if (!value) {
        return '';
      }
      try {
        return decodeURIComponent(value);
      } catch {
        return value;
      }
    })
    .filter(Boolean);

  const normalizedSegments = decodedSegments
    .map((segment) => segment.replace(/^\/+|\/+$/g, ''))
    .filter(Boolean);

  if (normalizedSegments.length === 0) {
    return '';
  }

  return `/${normalizedSegments.join('/')}`;
};

const resolveArtplayerConstructor = (): ArtplayerConstructor | null => {
  const moduleValue = Artplayer as unknown as { default?: unknown };
  const candidate = typeof moduleValue === 'function' ? moduleValue : moduleValue.default;
  return typeof candidate === 'function' ? (candidate as ArtplayerConstructor) : null;
};

const mountNativeFallbackPlayer = (url: string) => {
  if (!artRef.value) {
    return;
  }
  artRef.value.innerHTML = '';
  const video = document.createElement('video');
  video.className = 'native-video-fallback';
  video.controls = true;
  video.preload = 'metadata';
  video.src = url;
  video.playsInline = true;
  fallbackVideoRef.value = video;
  artRef.value.appendChild(video);
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

  if (!artRef.value) {
    return;
  }

  artFailed.value = false;
  const mediaUrl = buildMediaUrl(videoPath.value);
  const ArtplayerCtor = resolveArtplayerConstructor();
  if (!ArtplayerCtor) {
    artFailed.value = true;
    showSnackbar(t('videoPlaybackError'), 'error');
    mountNativeFallbackPlayer(mediaUrl);
    return;
  }

  try {
    art = new ArtplayerCtor({
      container: artRef.value,
      url: mediaUrl,
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
      playsInline: true,
      autoSize: true,
      theme: '#0f766e',
    });
  } catch {
    artFailed.value = true;
    showSnackbar(t('videoPlaybackError'), 'error');
    mountNativeFallbackPlayer(mediaUrl);
    return;
  }

  fallbackVideoRef.value = null;
  art.on('video:timeupdate', () => {
    currentPlaybackTime.value = formatTime(art?.currentTime || 0);
  });
  art.on('video:error', () => {
    artFailed.value = true;
    showSnackbar(t('videoPlaybackError'), 'error');
  });
  art.on('error', () => {
    artFailed.value = true;
    showSnackbar(t('videoPlaybackError'), 'error');
  });
});

watch(
  () => route.params.pathMatch,
  () => {
    const nextVideoPath = getVideoPath();
    if (!nextVideoPath) {
      return;
    }
    videoPath.value = nextVideoPath;
    currentPlaybackTime.value = '00:00:00';
    artFailed.value = false;
    if (art) {
      art.url = buildMediaUrl(nextVideoPath);
    } else if (fallbackVideoRef.value) {
      fallbackVideoRef.value.src = buildMediaUrl(nextVideoPath);
    }
  }
);

onUnmounted(() => {
  if (art) {
    art.destroy();
    art = null;
  }
  fallbackVideoRef.value = null;
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
  if (!path || path === route.path) {
    return;
  }
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
  <v-container class="app-page">
    <v-card class="glass-panel mb-4">
      <v-card-text>
        <PathBreadcrumb :path="breadcrumbPath" @navigate="handlePathNavigation" />
      </v-card-text>
    </v-card>

    <v-card class="glass-panel">
      <v-card-title class="px-4 pt-4 d-flex flex-wrap align-center ga-3" :class="smAndDown ? 'text-h6' : 'text-h5'">
        <span>{{ getFileName() }}</span>
        <v-chip size="small" color="secondary" variant="tonal">{{ t('currentPlaybackTime') }} {{ currentPlaybackTime }}</v-chip>
        <v-chip size="small" color="primary" variant="tonal">{{ t('totalClipDuration') }} {{ formatDuration(totalClipDuration) }}</v-chip>
      </v-card-title>

      <v-card-text>
        <v-alert v-if="artFailed" type="warning" variant="tonal" class="mb-4">
          {{ t('videoPlaybackError') }}
        </v-alert>
        <div ref="artRef" class="video-player mb-6" />

        <div class="export-panel mb-6">
          <div class="text-subtitle-1 mb-2">{{ t('exportModeTitle') }}</div>
          <div class="d-flex flex-wrap ga-2 mb-4 mode-btn-row">
            <v-btn :variant="exportMode === 'copy' ? 'flat' : 'tonal'" class="pill-button mode-btn" @click="selectExportMode('copy')">
              {{ t('exportMode_copy') }}
            </v-btn>
            <v-btn :variant="exportMode === 'transcode' ? 'flat' : 'tonal'" class="pill-button mode-btn" @click="selectExportMode('transcode')">
              {{ t('exportMode_transcode') }}
            </v-btn>
          </div>

          <div class="text-subtitle-1 mb-2">{{ t('videoCodecTitle') }}</div>
          <div class="d-flex flex-wrap ga-2 codec-chip-row">
            <v-chip
              v-for="codec in availableCodecOptions"
              :key="codec.id"
              :color="codec.id === selectedCodec ? 'primary' : 'default'"
              :variant="codec.id === selectedCodec ? 'flat' : 'outlined'"
              @click="selectedCodec = codec.id"
            >
              {{ codec.label }}
            </v-chip>
          </div>
        </div>

        <div v-for="(segment, index) in segments" :key="index" class="mb-2">
          <v-card variant="tonal">
            <v-card-title>
              <v-row align="center" justify="space-between" style="width: 100%">
                <v-col cols="auto">{{ t('segment') }} {{ index + 1 }} · {{ formatDuration(Math.max(0, parseTime(segment.endTime) - parseTime(segment.startTime))) }}</v-col>
                <v-col cols="auto">
                  <v-btn color="primary" variant="text" icon @click.stop="moveSegment(index, -1)" :disabled="index === 0">
                    <v-icon>mdi-arrow-up</v-icon>
                  </v-btn>
                  <v-btn color="primary" variant="text" icon @click.stop="moveSegment(index, 1)" :disabled="index === segments.length - 1">
                    <v-icon>mdi-arrow-down</v-icon>
                  </v-btn>
                  <v-btn color="secondary" variant="text" icon @click.stop="duplicateSegment(index)">
                    <v-icon>mdi-content-copy</v-icon>
                  </v-btn>
                  <v-btn
                    color="error"
                    variant="text"
                    icon
                    @click.stop="removeSegment(index)"
                    :disabled="segments.length === 1"
                  >
                    <v-icon>mdi-delete</v-icon>
                  </v-btn>
                </v-col>
              </v-row>
            </v-card-title>
            <v-card-text>
              <v-row>
                <v-col cols="12" sm="6">
                  <v-text-field v-model="segment.startTime" :label="t('startTime')" hide-details>
                    <template #append>
                      <v-btn color="primary" @click="setCurrentTime(index, 'start')">{{ t('setCurrent') }}</v-btn>
                    </template>
                  </v-text-field>
                </v-col>

                <v-col cols="12" sm="6">
                  <v-text-field v-model="segment.endTime" :label="t('endTime')" hide-details>
                    <template #append>
                      <v-btn color="primary" @click="setCurrentTime(index, 'end')">{{ t('setCurrent') }}</v-btn>
                    </template>
                  </v-text-field>
                </v-col>
              </v-row>
              <v-alert v-if="segmentErrors[index]" type="warning" variant="tonal" class="mt-4">
                {{ segmentErrors[index] }}
              </v-alert>
            </v-card-text>
          </v-card>
        </div>

        <v-alert v-if="latestTaskId" type="info" variant="tonal" class="mt-4">
          {{ t('latestTaskId') }}: <code>{{ latestTaskId }}</code>
        </v-alert>
      </v-card-text>

      <v-card-actions class="px-4 pb-4 editor-actions">
        <v-btn color="primary" variant="outlined" prepend-icon="mdi-plus" @click="addSegment">{{ t('addSegment') }}</v-btn>
        <v-spacer />
        <v-btn color="secondary" variant="tonal" prepend-icon="mdi-progress-clock" @click="openTaskCenter">
          {{ t('taskCenterBtn') }}
        </v-btn>
        <v-btn color="primary" prepend-icon="mdi-rocket-launch" :loading="loading" :disabled="hasInvalidSegments || !hasValidCodecSelection" @click="createVideoTask">
          {{ t('createVideoTask') }}
        </v-btn>
      </v-card-actions>
    </v-card>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color">
      {{ snackbar.message }}
    </v-snackbar>
  </v-container>
</template>

<style scoped>
.video-player {
  aspect-ratio: 16/9;
  background-color: black;
  border-radius: 12px;
  overflow: hidden;
}

.video-player :deep(.art-video-player) {
  z-index: 1;
}

.video-player .native-video-fallback {
  width: 100%;
  height: 100%;
  display: block;
  background-color: black;
}

.export-panel {
  padding: 16px;
  border-radius: 18px;
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-2) 92%, transparent);
}

@media (max-width: 960px) {
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

  .editor-actions :deep(.v-spacer) {
    display: none;
  }

  .editor-actions .v-btn {
    width: 100%;
  }
}
</style>
