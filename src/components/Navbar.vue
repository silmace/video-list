<script setup lang="ts">
import { useRouter } from 'vue-router';
import { useTheme } from 'vuetify';
import { ref, watch } from 'vue';

const router = useRouter();
const theme = useTheme();

const isDark = ref(theme.global.name.value === 'dark');

watch(isDark, (newValue) => {
  theme.global.name.value = newValue ? 'dark' : 'light'
  localStorage.setItem('theme', newValue ? 'dark' : 'light')
});

const navigateHome = () => {
  router.push('/');
};
</script>

<template>
  <v-app-bar color="surface" elevation="2">
    <!-- <template v-slot:prepend>
      <v-app-bar-nav-icon></v-app-bar-nav-icon>
    </template> -->

    <v-app-bar-title>
      <v-btn variant="text" @click="navigateHome" class="text-h6">
        <v-icon start icon="mdi-video" color="primary"></v-icon>
        Video List
      </v-btn>
    </v-app-bar-title>

    <template v-slot:append>
      <v-btn
        :icon="isDark ? 'mdi-weather-night' : 'mdi-weather-sunny'"
        @click="isDark = !isDark"
      ></v-btn>
    </template>
  </v-app-bar>
</template>