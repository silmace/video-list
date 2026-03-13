<script setup lang="ts">
import Navbar from './components/Navbar.vue'
import { useThemePreference } from './composables/useThemePreference'

useThemePreference()
</script>

<template>
  <div class="app-shell">
    <Navbar />
    <main class="app-main">
      <router-view v-slot="{ Component }">
        <Transition name="page-fade" mode="out-in">
          <component :is="Component" />
        </Transition>
      </router-view>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
}

.app-main {
  background: transparent;
}

.page-fade-enter-active,
.page-fade-leave-active {
  transition: opacity 0.2s ease;
}

.page-fade-enter-from,
.page-fade-leave-to {
  opacity: 0;
}
</style>