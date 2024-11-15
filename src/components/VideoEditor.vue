<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { VideoSegment } from '../types';
import axios from 'axios';
import Artplayer from 'artplayer';
import PathBreadcrumb from './PathBreadcrumb.vue';

const route = useRoute();
const router = useRouter();
const videoPath = ref('');
const segments = ref<VideoSegment[]>([{ startTime: '00:00:00', endTime: '00:00:00' }]);
const artRef = ref<HTMLDivElement | null>(null);
const loading = ref(false);
const snackbar = ref({ show: false, message: '', color: '' });
let art: Artplayer | null = null;

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
      theme: '#6750A4'
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
  segments.value.splice(index, 1);
};

const formatTime = (time: number): string => {
  const hours = Math.floor(time / 3600);
  const minutes = Math.floor((time % 3600) / 60);
  const seconds = Math.floor(time % 60);
  return `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
};

const setCurrentTime = (index: number, type: 'start' | 'end') => {
  if (art) {
    const currentTime = art.currentTime;
    segments.value[index][type === 'start' ? 'startTime' : 'endTime'] = formatTime(currentTime);
  }
};

const saveSegments = async () => {
  loading.value = true;
  try {
    await axios.post('/api/edit-video', {
      videoPath: videoPath.value,
      segments: segments.value
    });
    showSnackbar('Video segments saved successfully', 'success');
  } catch (error) {
    showSnackbar('Error saving video segments', 'error');
  } finally {
    loading.value = false;
  }
};

const getFileName = () => {
  return videoPath.value?.split('/').pop() || 'Video Editor';
};

const handlePathNavigation = (path: string) => {
  if (path === '/') {
    router.push('/');
  } else {
    const parentDir = videoPath.value.split('/').slice(0, -1).join('/');
    router.push(parentDir || '/');
  }
};
</script>

<template>
  <v-container class="pa-4">
    <v-card class="mb-4">
      <v-card-text>
        <PathBreadcrumb :path="videoPath" :onNavigate="handlePathNavigation" />
      </v-card-text>
    </v-card>

    <v-card>
      <v-card-title class="text-h5 px-4 pt-4">
        {{ getFileName() }}
      </v-card-title>

      <v-card-text>
        <div ref="artRef" class="video-player mb-6"></div>

        <div v-for="(segment, index) in segments" :key="index" class="mb-2">
          <v-card>
            <v-card-title>
              <v-row align="center" justify="space-between" style="width: 100%;">
                <v-col cols="auto">
                  Segment {{ index + 1 }}
                </v-col>
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
                  <v-text-field
                    v-model="segment.startTime"
                    label="Start Time"
                    hide-details
                    density="comfortable"
                  >
                    <template v-slot:append>
                      <v-btn
                        color="primary"
                        @click="setCurrentTime(index, 'start')"
                      >
                        Set Current
                      </v-btn>
                    </template>
                  </v-text-field>
                </v-col>

                <v-col cols="12" sm="6">
                  <v-text-field
                    v-model="segment.endTime"
                    label="End Time"
                    hide-details
                    density="comfortable"
                  >
                    <template v-slot:append>
                      <v-btn
                        color="primary"
                        @click="setCurrentTime(index, 'end')"
                      >
                        Set Current
                      </v-btn>
                    </template>
                  </v-text-field>
                </v-col>
              </v-row>
            </v-card-text>
          </v-card>
        </div>
      </v-card-text>

      <v-card-actions class="px-4 pb-4">
        <v-btn
          color="primary"
          variant="outlined"
          prepend-icon="mdi-plus"
          @click="addSegment"
        >
          Add Segment
        </v-btn>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          prepend-icon="mdi-content-save"
          :loading="loading"
          @click="saveSegments"
        >
          Save Segments
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
  border-radius: 8px;
  overflow: hidden;
}
</style>