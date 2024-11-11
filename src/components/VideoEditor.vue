<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRoute } from 'vue-router';
import type { VideoSegment } from '../types';
import axios from 'axios';

const route = useRoute();
const videoPath = ref(route.query.path as string);
const segments = ref<VideoSegment[]>([{ startTime: '00:00:00', endTime: '00:00:00' }]);
const videoRef = ref<HTMLVideoElement | null>(null);

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
  if (videoRef.value) {
    const currentTime = videoRef.value.currentTime;
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
</script>

<template>
  <div class="container mx-auto p-4">
    <div class="bg-white rounded-lg shadow p-4">
      <h2 class="text-xl font-semibold mb-4">Video Editor</h2>
      <video ref="videoRef" controls class="w-full max-h-[60vh] mb-4">
        <source :src="`/api/video?path=${encodeURIComponent(videoPath)}`" type="video/mp4">
        Your browser does not support the video tag.
      </video>

      <div class="space-y-4">
        <div v-for="(segment, index) in segments" :key="index" class="flex items-center space-x-4">
          <div class="flex-1">
            <label class="block text-sm font-medium text-gray-700">Start Time</label>
            <div class="flex items-center space-x-2">
              <input v-model="segment.startTime" type="text" 
                     class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
              <button @click="setCurrentTime(index, 'start')" 
                      class="px-3 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">
                Set Current
              </button>
            </div>
          </div>

          <div class="flex-1">
            <label class="block text-sm font-medium text-gray-700">End Time</label>
            <div class="flex items-center space-x-2">
              <input v-model="segment.endTime" type="text" 
                     class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500">
              <button @click="setCurrentTime(index, 'end')" 
                      class="px-3 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">
                Set Current
              </button>
            </div>
          </div>

          <button @click="removeSegment(index)" 
                  class="mt-6 px-3 py-2 bg-red-600 text-white rounded hover:bg-red-700">
            Remove
          </button>
        </div>
      </div>

      <div class="mt-4 space-x-4">
        <button @click="addSegment" 
                class="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700">
          Add Segment
        </button>
        <button @click="saveSegments" 
                class="px-4 py-2 bg-indigo-600 text-white rounded hover:bg-indigo-700">
          Save Segments
        </button>
      </div>
    </div>
  </div>
</template>