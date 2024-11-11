<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import type { FileItem } from '../types';
import axios from 'axios';

const files = ref<FileItem[]>([]);
const router = useRouter();
const currentPath = ref('/');

const fetchFiles = async (path: string = '/') => {
  try {
    const response = await axios.get(`/api/files?path=${encodeURIComponent(path)}`);
    files.value = response.data;
    currentPath.value = path;
  } catch (error) {
    console.error('Error fetching files:', error);
  }
};

const handleFileClick = (file: FileItem) => {
  if (file.isDirectory) {
    const newPath = `${currentPath.value}${file.name}/`;
    fetchFiles(newPath);
  } else if (file.name.match(/\.(mp4|webm|mov)$/i)) {
    router.push(`/edit?path=${encodeURIComponent(file.path)}`);
  }
};

const formatSize = (size: number): string => {
  const units = ['B', 'KB', 'MB', 'GB'];
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index++;
  }
  return `${size.toFixed(2)} ${units[index]}`;
};

onMounted(() => {
  fetchFiles();
});
</script>

<template>
  <div class="container mx-auto p-4">
    <div class="bg-white rounded-lg shadow">
      <div class="p-4 border-b">
        <h2 class="text-xl font-semibold">Files</h2>
        <p class="text-gray-600">Current path: {{ currentPath }}</p>
      </div>
      <div class="overflow-x-auto">
        <table class="min-w-full">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Size</th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Modified</th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr v-if="currentPath !== '/'" @click="fetchFiles(currentPath.split('/').slice(0, -2).join('/') + '/')" 
                class="hover:bg-gray-100 cursor-pointer">
              <td class="px-6 py-4">
                <div class="flex items-center">
                  <span class="ml-2">..</span>
                </div>
              </td>
              <td class="px-6 py-4">-</td>
              <td class="px-6 py-4">-</td>
            </tr>
            <tr v-for="file in files" :key="file.path" @click="handleFileClick(file)" 
                class="hover:bg-gray-100 cursor-pointer">
              <td class="px-6 py-4">
                <div class="flex items-center">
                  <span class="ml-2">{{ file.name }}</span>
                </div>
              </td>
              <td class="px-6 py-4">{{ formatSize(file.size) }}</td>
              <td class="px-6 py-4">{{ new Date(file.modifiedTime).toLocaleString() }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>