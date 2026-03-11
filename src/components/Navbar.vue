<script setup lang="ts">
import { useRouter } from 'vue-router';
import { onMounted } from 'vue';
import { authState, checkAuthStatus, logout } from '../composables/useAuth';
import { useLocale } from '../composables/useLocale';
import { useThemePreference } from '../composables/useThemePreference';

const router = useRouter();
const { t } = useLocale();
const { isDark, toggleTheme } = useThemePreference();

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
  <v-app-bar class="glass-appbar navbar-shell" elevation="0">

    <v-app-bar-title>
      <v-btn variant="text" @click="navigateHome" class="navbar-brand pill-button">
        <v-icon start icon="mdi-folder-multiple-image" color="secondary"></v-icon>
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
        variant="tonal"
        class="pill-button"
        :prepend-icon="isDark ? 'mdi-weather-night' : 'mdi-white-balance-sunny'"
        @click="toggleTheme"
      >
        {{ isDark ? t('darkModeOn') : t('lightModeOn') }}
      </v-btn>
      <v-btn
        v-if="authState.authEnabled.value && !authState.authenticated.value"
        variant="tonal"
        class="pill-button"
        color="primary"
        @click="navigateLogin"
      >
        {{ t('login') }}
      </v-btn>
      <v-btn
        v-if="authState.authEnabled.value && authState.authenticated.value"
        variant="tonal"
        class="pill-button"
        color="error"
        :icon="'mdi-logout'"
        @click="doLogout"
      >
      </v-btn>
    </template>
  </v-app-bar>
</template>

<style scoped>
.navbar-shell {
  padding-inline: 10px;
}

.navbar-brand {
  font-size: 1rem;
  font-weight: 800;
  letter-spacing: -0.03em;
}
</style>
