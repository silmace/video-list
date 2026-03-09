<script setup lang="ts">
import { useRouter } from 'vue-router';
import { useTheme } from 'vuetify';
import { ref, watch, onMounted } from 'vue';
import { authState, checkAuthStatus, logout } from '../composables/useAuth';
import { useLocale } from '../composables/useLocale';

const router = useRouter();
const theme = useTheme();
const { t } = useLocale();

const savedTheme = localStorage.getItem('theme') || 'light';
const isDark = ref(savedTheme === 'dark');

onMounted(() => {
  theme.global.name.value = isDark.value ? 'dark' : 'light';
  document.body.classList.toggle('theme-dark', isDark.value);
});

watch(isDark, (newValue) => {
  theme.global.name.value = newValue ? 'dark' : 'light';
  localStorage.setItem('theme', newValue ? 'dark' : 'light');
  document.body.classList.toggle('theme-dark', newValue);
});

const navigateHome = () => {
  router.push('/');
};

const navigateTasks = () => {
  router.push('/tasks');
};

const navigateSettings = () => {
  router.push('/settings');
};

const navigateLogin = () => {
  router.push('/login');
};

const doLogout = async () => {
  await logout();
  await checkAuthStatus();
  router.push('/login');
};

onMounted(async () => {
  await checkAuthStatus();
});
</script>

<template>
  <v-app-bar class="glass-appbar" elevation="0">

    <v-app-bar-title>
      <v-btn variant="text" @click="navigateHome" class="text-h6">
        <v-icon start icon="mdi-list-box" color="secondary"></v-icon>
        {{ t('appTitle') }}
      </v-btn>
    </v-app-bar-title>

    <template v-slot:append>
      <v-btn
        v-if="!authState.authEnabled.value || authState.authenticated.value"
        variant="text"
        icon="mdi-progress-clock"
        :title="t('tasks')"
        :aria-label="t('tasks')"
        @click="navigateTasks"
      />
      <v-btn
        v-if="!authState.authEnabled.value || authState.authenticated.value"
        variant="text"
        icon="mdi-cog-outline"
        :title="t('settings')"
        :aria-label="t('settings')"
        @click="navigateSettings"
      />
      <v-btn
        :icon="isDark ? 'mdi-weather-night' : 'mdi-weather-sunny'"
        @click="isDark = !isDark"
      ></v-btn>
      <v-btn
        v-if="authState.authEnabled.value && !authState.authenticated.value"
        variant="tonal"
        color="primary"
        @click="navigateLogin"
      >
        {{ t('login') }}
      </v-btn>
      <v-btn
        v-if="authState.authEnabled.value && authState.authenticated.value"
        variant="tonal"
        color="error"
        @click="doLogout"
      >
        {{ t('logout') }}
      </v-btn>
    </template>
  </v-app-bar>
</template>
