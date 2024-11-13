<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import type { FileItem } from '../types';
import axios from 'axios';
import PathBreadcrumb from './PathBreadcrumb.vue';

const files = ref<FileItem[]>([]);
const router = useRouter();
const currentPath = ref('/');
const showImageDialog = ref(false);
const selectedImage = ref<FileItem | null>(null);

const fetchFiles = async (path: string = '/') => {
  try {
    const response = await axios.get(`/api/files?path=${encodeURIComponent(path)}`);
    files.value = response.data;
    currentPath.value = path;
  } catch (error) {
    console.error('Error fetching files:', error);
  }
};

const handleFileClick = async (file: FileItem) => {
  if (file.isDirectory) {
    const newPath = `${currentPath.value}${file.name}/`;
    fetchFiles(newPath);
  } else if (file.name.match(/\.(mp4|webm|mov)$/i)) {
    router.push(`/edit?path=${encodeURIComponent(file.path)}`);
  } else if (file.name.match(/\.(jpg|jpeg|png|gif|webp|svg)$/i)) {
    selectedImage.value = file;
    showImageDialog.value = true;
  } else {
    // Download other file types
    try {
      const response = await axios.get(`/api/media?path=${encodeURIComponent(file.path)}`, {
        responseType: 'blob'
      });
      const url = window.URL.createObjectURL(new Blob([response.data]));
      const link = document.createElement('a');
      link.href = url;
      link.setAttribute('download', file.name);
      document.body.appendChild(link);
      link.click();
      link.remove();
      window.URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Error downloading file:', error);
      alert('Failed to download file');
    }
  }
};

const deleteFile = async (file: FileItem, event: Event) => {
  event.stopPropagation();
  if (confirm(`Are you sure you want to delete ${file.name}?`)) {
    try {
      await axios.delete(`/api/files?path=${encodeURIComponent(file.path)}`);
      fetchFiles(currentPath.value);
    } catch (error) {
      console.error('Error deleting file:', error);
      alert('Failed to delete file');
    }
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

const getFileIcon = (file: FileItem): string => {
  if (file.isDirectory) {
    return 'mdi-folder';
  }
  if (file.name.match(/\.(mp4|webm|mov)$/i)) {
    return 'mdi-video';
  }
  if (file.name.match(/\.(jpg|jpeg|png|gif|webp|svg)$/i)) {
    return 'mdi-image';
  }
  return 'mdi-file';
};

onMounted(() => {
  fetchFiles();
});
</script>

<template>
  <v-container>
    <v-card>
      <v-card-subtitle class="px-4">
        <PathBreadcrumb :path="currentPath" :onNavigate="fetchFiles" />
      </v-card-subtitle>

      <v-table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Size</th>
            <th>Modified</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="currentPath !== '/'" 
              @click="fetchFiles(currentPath.split('/').slice(0, -2).join('/') + '/')"
              style="cursor: pointer">
            <td>
              <div class="d-flex align-center">
                <v-icon class="mr-2">mdi-folder</v-icon>
                <span>..</span>
              </div>
            </td>
            <td>-</td>
            <td>-</td>
            <td></td>
          </tr>
          <tr v-for="file in files" 
              :key="file.path" 
              @click="handleFileClick(file)"
              style="cursor: pointer">
            <td>
              <div class="d-flex align-center">
                <v-icon class="mr-2">{{ getFileIcon(file) }}</v-icon>
                <span>{{ file.name }}</span>
              </div>
            </td>
            <td>{{ formatSize(file.size) }}</td>
            <td>{{ new Date(file.modifiedTime).toLocaleString() }}</td>
            <td>
              <v-btn
                color="error"
                variant="text"
                icon
                @click.stop="deleteFile(file, $event)"
              >
                <v-icon>mdi-delete</v-icon>
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
    </v-card>

    <v-dialog v-model="showImageDialog" max-width="90vw">
      <v-card v-if="selectedImage">
        <v-card-title class="text-h6">
          {{ selectedImage.name }}
          <v-spacer></v-spacer>
          <v-btn icon="mdi-close" variant="text" @click="showImageDialog = false"></v-btn>
        </v-card-title>
        <v-card-text class="pa-0">
          <v-img
            :src="`/api/media?path=${encodeURIComponent(selectedImage.path)}`"
            :alt="selectedImage.name"
            cover
            max-height="80vh"
            class="mx-auto"
          ></v-img>
        </v-card-text>
      </v-card>
    </v-dialog>
  </v-container>
</template>