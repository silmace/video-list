<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { VideoCodecOption, VideoExportMode, VideoSegment } from '../types';
import { buildMediaUrl } from '../services/api';
import { createVideoTask as createVideoTaskRequest, fetchVideoOptions } from '../services/video';
import Artplayer from 'artplayer';
import PathBreadcrumb from './PathBreadcrumb.vue';
import { useLocale } from '../composables/useLocale';

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

const availableCodecOptions = computed(() => codecOptions.value.filter((item) => item.mode === exportMode.value));
const segmentErrors = computed(() => segments.value.map((segment) => validateSegment(segment)));
const hasInvalidSegments = computed(() => segmentErrors.value.some(Boolean));
const hasValidCodecSelection = computed(() => availableCodecOptions.value.some((item) => item.id === selectedCodec.value));
const totalClipDuration = computed(() => {
  return segments.value.reduce((sum, segment) => sum + Math.max(0, parseTime(segment.endTime) - parseTime(segment.startTime)), 0);
});

const getVideoPath = () => {
  if (!route.params.pathMatch) return '';
  const path = Array.isArray(route.params.pathMatch)
    ? route.params.pathMatch.join('/')
    : route.params.pathMatch;
  return `/${path}`;
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
      url: buildMediaUrl(videoPath.value),
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

const selectExportMode = (mode: VideoExportMode) => {
  exportMode.value = mode;
  const preferred = availableCodecOptions.value[0];
  selectedCodec.value = preferred?.id || '';
};

onMounted(async () => {
  try {
    codecOptions.value = await fetchVideoOptions();
    const preferred = codecOptions.value.find((item) => item.id === 'copy' && item.available);
    if (preferred) {
      selectedCodec.value = preferred.id;
    }
  } catch {
    codecOptions.value = [];
  }
});
</script>

<template>
  <v-container class="app-page">
    <v-card class="glass-panel mb-4">
      <v-card-text>
        <PathBreadcrumb :path="videoPath" :onNavigate="handlePathNavigation" />
      </v-card-text>
    </v-card>

    <v-card class="glass-panel">
      <v-card-title class="text-h5 px-4 pt-4 d-flex flex-wrap align-center ga-3">
        <span>{{ getFileName() }}</span>
        <v-chip size="small" color="secondary" variant="tonal">{{ t('currentPlaybackTime') }} {{ currentPlaybackTime }}</v-chip>
        <v-chip size="small" color="primary" variant="tonal">{{ t('totalClipDuration') }} {{ formatDuration(totalClipDuration) }}</v-chip>
      </v-card-title>

      <v-card-text>
        <div ref="artRef" class="video-player mb-6" />

        <div class="export-panel mb-6">
          <div class="text-subtitle-1 mb-2">{{ t('exportModeTitle') }}</div>
          <div class="d-flex flex-wrap ga-2 mb-4">
            <v-btn :variant="exportMode === 'copy' ? 'flat' : 'tonal'" class="pill-button" @click="selectExportMode('copy')">
              {{ t('exportMode_copy') }}
            </v-btn>
            <v-btn :variant="exportMode === 'transcode' ? 'flat' : 'tonal'" class="pill-button" @click="selectExportMode('transcode')">
              {{ t('exportMode_transcode') }}
            </v-btn>
          </div>

          <div class="text-subtitle-1 mb-2">{{ t('videoCodecTitle') }}</div>
          <div class="d-flex flex-wrap ga-2">
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

      <v-card-actions class="px-4 pb-4">
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

.export-panel {
  padding: 16px;
  border-radius: 18px;
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-2) 92%, transparent);
}
</style>
