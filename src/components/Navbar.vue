<script setup lang="ts">
import { useRouter } from 'vue-router';

const router = useRouter();

const navigateHome = () => {
  router.push('/');
};
</script>

<template>
  <nav class="bg-gray-800 p-4">
    <div class="container mx-auto flex items-center justify-between">
      <div class="flex items-center">
        <button @click="navigateHome" class="text-white text-xl font-bold">
          Video File Manager
        </button>
      </div>
    </div>
  </nav>
</template>