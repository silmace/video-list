<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { VideoSegment } from '../types';
import { api } from '../services/api';
import Artplayer from 'artplayer';
import PathBreadcrumb from './PathBreadcrumb.vue';
import { useLocale } from '../composables/useLocale';

const route = useRoute();
const router = useRouter();
const videoPath = ref('');
const segments = ref<VideoSegment[]>([{ startTime: '00:00:00', endTime: '00:00:00' }]);
const artRef = ref<HTMLDivElement | null>(null);
const loading = ref(false);
const snackbar = ref({ show: false, message: '', color: '' });
const latestTaskId = ref('');
let art: Artplayer | null = null;
const { t } = useLocale();

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
      url: `/api/media?path=${encodeURIComponent(videoPath.value)}`,
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
      theme: '#1a73e8',
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

const setCurrentTime = (index: number, type: 'start' | 'end') => {
  if (!art) return;
  const currentTime = art.currentTime;
  segments.value[index][type === 'start' ? 'startTime' : 'endTime'] = formatTime(currentTime);
};

const createVideoTask = async () => {
  loading.value = true;
  try {
    const response = await api.post<{ success: boolean; taskId: string }>('/api/tasks/video', {
      videoPath: videoPath.value,
      segments: segments.value,
    });
    latestTaskId.value = response.data.taskId;
    showSnackbar(t('videoTaskCreated'), 'success');
  } catch (error) {
    showSnackbar(resolveApiErrorMessage(error, t('videoTaskCreateError')), 'error');
  } finally {
    loading.value = false;
  }
};

const getFileName = () => videoPath.value?.split('/').pop() || t('videoEditorFallback');

const handlePathNavigation = (path: string) => {
  if (path === '/') {
    router.push('/');
    return;
  }
  const parentDir = videoPath.value.split('/').slice(0, -1).join('/');
  router.push(parentDir || '/');
};

const openTaskCenter = () => {
  router.push('/tasks');
};
</script>

<template>
  <v-container class="pa-4">
    <v-card class="glass-panel mb-4">
      <v-card-text>
        <PathBreadcrumb :path="videoPath" :onNavigate="handlePathNavigation" />
      </v-card-text>
    </v-card>

    <v-card class="glass-panel">
      <v-card-title class="text-h5 px-4 pt-4">
        {{ getFileName() }}
      </v-card-title>

      <v-card-text>
        <div ref="artRef" class="video-player mb-6" />

        <div v-for="(segment, index) in segments" :key="index" class="mb-2">
          <v-card variant="tonal">
            <v-card-title>
              <v-row align="center" justify="space-between" style="width: 100%">
                <v-col cols="auto">{{ t('segment') }} {{ index + 1 }}</v-col>
                <v-col cols="auto">
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
        <v-btn color="primary" prepend-icon="mdi-rocket-launch" :loading="loading" @click="createVideoTask">
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
</style>
