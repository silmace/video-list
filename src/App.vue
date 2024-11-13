<script setup lang="ts">
import Navbar from './components/Navbar.vue'
</script>

<template>
  <v-app>
    <Navbar />
    <v-main>
      <router-view></router-view>
    </v-main>
  </v-app>
</template>