<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { VideoSegment } from '../types';
import axios from 'axios';
import Artplayer from 'artplayer';
import PathBreadcrumb from './PathBreadcrumb.vue';

const route = useRoute();
const router = useRouter();
const videoPath = ref(route.query.path as string);
const segments = ref<VideoSegment[]>([{ startTime: '00:00:00', endTime: '00:00:00' }]);
const artRef = ref<HTMLDivElement | null>(null);
let art: Artplayer | null = null;

onMounted(() => {
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
  try {
    await axios.post('/api/edit-video', {
      videoPath: videoPath.value,
      segments: segments.value
    });
    alert('Video segments saved successfully!');
  } catch (error) {
    console.error('Error saving video segments:', error);
    alert('Error saving video segments');
  }
};

const getFileName = () => {
  return videoPath.value?.split('/').pop() || 'Video Editor';
};
</script>

<template>
  <v-container class="pa-0">
    <v-card class="mx-auto" elevation="0">
      <v-card-title class="text-h5 px-4 pt-4 pb-2">
        {{ getFileName() }}
      </v-card-title>
      
      <v-card-subtitle class="px-4">
        <PathBreadcrumb :path="videoPath" />
      </v-card-subtitle>

      <v-card-text>
        <div ref="artRef" class="w-100 video-player mb-6"></div>

        <v-row v-for="(segment, index) in segments" :key="index" class="mb-4">
          <v-col cols="12" sm="5">
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

          <v-col cols="12" sm="5">
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

          <v-col cols="12" sm="2" class="d-flex align-center">
            <v-btn
              color="error"
              variant="text"
              icon
              @click="removeSegment(index)"
              :disabled="segments.length === 1"
            >
              <v-icon>mdi-delete</v-icon>
            </v-btn>
          </v-col>
        </v-row>
      </v-card-text>

      <v-card-actions class="px-4 pb-4">
        <v-btn
          color="success"
          prepend-icon="mdi-plus"
          @click="addSegment"
        >
          Add Segment
        </v-btn>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          prepend-icon="mdi-content-save"
          @click="saveSegments"
        >
          Save Segments
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-container>
</template>

<style scoped>
.video-player {
  aspect-ratio: 16/9;
}
</style>