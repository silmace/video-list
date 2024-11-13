<script setup lang="ts">
import { useRouter } from 'vue-router';

const router = useRouter();

const navigateHome = () => {
  router.push('/');
};
</script>

<template>
  <v-app-bar color="primary">
    <v-app-bar-title>
      <v-btn variant="text" @click="navigateHome" class="text-h6">
        <v-icon start icon="mdi-video"></v-icon>
        Video File Manager
      </v-btn>
    </v-app-bar-title>
  </v-app-bar>
</template>