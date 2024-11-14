<script setup lang="ts">
import Navbar from './components/Navbar.vue'
import { useTheme } from 'vuetify'
import { onMounted } from 'vue'

const theme = useTheme()

onMounted(() => {
  const isDark = localStorage.getItem('theme') === 'dark'
  theme.global.name.value = isDark ? 'dark' : 'light'
})
</script>

<template>
  <v-app>
    <Navbar />
    <v-main class="bg-background">
      <router-view v-slot="{ Component }">
        <v-fade-transition mode="out-in">
          <component :is="Component" />
        </v-fade-transition>
      </router-view>
    </v-main>
  </v-app>
</template>