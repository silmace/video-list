<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import type { FileItem } from '../types';
import axios from 'axios';
import PathBreadcrumb from './PathBreadcrumb.vue';

const files = ref<FileItem[]>([]);
const router = useRouter();
const route = useRoute();
const currentPath = ref('/');
const showImageDialog = ref(false);
const selectedImage = ref<FileItem | null>(null);
const loading = ref(false);
const search = ref('');
const snackbar = ref({ show: false, message: '', color: '' });

const getPathFromRoute = () => {
  if (!route.params.pathMatch) return '/';
  const path = Array.isArray(route.params.pathMatch)
    ? route.params.pathMatch.join('/')
    : route.params.pathMatch;
  return `/${path}`;
};

const fetchFiles = async (path: string = '/') => {
  loading.value = true;
  try {
    const response = await axios.get(`/api/files?path=${encodeURIComponent(path)}`);
    files.value = response.data;
    currentPath.value = path;
    if (path === '/') {
      router.push('/');
    } else {
      router.push(path);
    }
  } catch (error) {
    showSnackbar('Error fetching files', 'error');
  } finally {
    loading.value = false;
  }
};

const handleFileClick = async (file: FileItem) => {
  if (file.isDirectory) {
    const newPath = `${currentPath.value}${currentPath.value.endsWith('/') ? '' : '/'}${file.name}/`;
    fetchFiles(newPath);
  } else if (file.name.match(/\.(mp4|webm|mov)$/i)) {
    const videoPath = file.path.startsWith('/') ? file.path.slice(1) : file.path;
    router.push(`/edit/${videoPath}`);
  } else if (file.name.match(/\.(jpg|jpeg|png|gif|webp|svg)$/i)) {
    selectedImage.value = file;
    showImageDialog.value = true;
  } else {
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
      showSnackbar('File downloaded successfully', 'success');
    } catch (error) {
      showSnackbar('Error downloading file', 'error');
    }
  }
};

const deleteFile = async (file: FileItem, event: Event) => {
  event.stopPropagation();
  if (confirm(`Are you sure you want to delete ${file.name}?`)) {
    try {
      await axios.delete(`/api/files?path=${encodeURIComponent(file.path)}`);
      fetchFiles(currentPath.value);
      showSnackbar('File deleted successfully', 'success');
    } catch (error) {
      showSnackbar('Error deleting file', 'error');
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

const showSnackbar = (message: string, color: string) => {
  snackbar.value = { show: true, message, color };
};

watch(() => route.params.pathMatch, () => {
  const path = getPathFromRoute();
  if (path !== currentPath.value) {
    fetchFiles(path);
  }
}, { immediate: true });

onMounted(() => {
  const path = getPathFromRoute();
  fetchFiles(path);
});
</script>

<template>
  <v-container class="pa-4">
    <v-card class="mb-4">
      <v-card-text>
        <PathBreadcrumb :path="currentPath" :onNavigate="fetchFiles" />
      </v-card-text>
    </v-card>

    <v-card>
      <v-data-table
        :headers="[
          { title: 'Name', key: 'name', sortable: true },
          { title: 'Size', key: 'size', sortable: true },
          { title: 'Modified', key: 'modifiedTime', sortable: true },
          { title: 'Actions', key: 'actions', sortable: false },
        ]"
        :items="files"
        :loading="loading"
        :search="search"
        :items-per-page="50"
        hover
      >
        <template v-slot:item="{ item }">
          <tr @click="handleFileClick(item)" style="cursor: pointer">
            <td>
              <div class="d-flex align-center">
                <v-icon :color="item.isDirectory ? 'primary' : ''" class="mr-2">
                  {{ getFileIcon(item) }}
                </v-icon>
                <span>{{ item.name }}</span>
              </div>
            </td>
            <td>{{ formatSize(item.size) }}</td>
            <td>{{ new Date(item.modifiedTime).toLocaleString() }}</td>
            <td>
              <v-btn
                color="error"
                variant="text"
                icon
                @click.stop="deleteFile(item, $event)"
              >
                <v-icon>mdi-delete</v-icon>
              </v-btn>
            </td>
          </tr>
        </template>

        <template v-slot:top>
          <v-toolbar flat color="transparent">
            <v-toolbar-title>Files</v-toolbar-title>
            <v-spacer></v-spacer>
            <v-text-field
              v-model="search"
              prepend-icon="mdi-magnify"
              label="Search"
              single-line
              hide-details
              density="comfortable"
              class="mr-4"
            ></v-text-field>
          </v-toolbar>
        </template>
      </v-data-table>
    </v-card>

    <v-dialog v-model="showImageDialog" max-width="90vw">
      <v-card v-if="selectedImage">
        <v-toolbar flat>
          <v-toolbar-title>{{ selectedImage.name }}</v-toolbar-title>
          <v-spacer></v-spacer>
          <v-btn icon @click="showImageDialog = false">
            <v-icon>mdi-close</v-icon>
          </v-btn>
        </v-toolbar>
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

    <v-snackbar v-model="snackbar.show" :color="snackbar.color" top right>
      {{ snackbar.message }}
    </v-snackbar>
  </v-container>
</template>