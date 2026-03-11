<script setup lang="ts">
import Navbar from './components/Navbar.vue'
import { useThemePreference } from './composables/useThemePreference'

useThemePreference()
</script>

<template>
  <v-app class="app-shell">
    <Navbar />
    <v-main class="app-main">
      <router-view v-slot="{ Component }">
        <v-fade-transition mode="out-in">
          <component :is="Component" />
        </v-fade-transition>
      </router-view>
    </v-main>
  </v-app>
</template>