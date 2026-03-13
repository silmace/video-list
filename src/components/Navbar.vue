<script setup lang="ts">
import { useRouter } from 'vue-router';
import { onMounted } from 'vue';
import { useDisplay } from 'vuetify';
import { authState, checkAuthStatus, logout } from '../composables/useAuth';
import { useLocale } from '../composables/useLocale';
import { useThemePreference } from '../composables/useThemePreference';

const router = useRouter();
const { t } = useLocale();
const { isDark, toggleTheme } = useThemePreference();
const { smAndDown } = useDisplay();

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
    <v-app-bar-title class="navbar-title">
      <v-btn
        variant="text"
        class="navbar-brand pill-button"
        :title="t('appTitle')"
        :aria-label="t('appTitle')"
        @click="navigateHome"
      >
        <v-icon start icon="mdi-folder-multiple-image" color="secondary" class="navbar-brand-icon"></v-icon>
        <span class="navbar-brand-label">{{ t('appTitle') }}</span>
      </v-btn>
    </v-app-bar-title>

    <template v-slot:append>
      <div class="navbar-actions">
        <v-btn
          v-if="(!authState.authEnabled.value || authState.authenticated.value) && !smAndDown"
          variant="text"
          class="navbar-icon-btn"
          icon="mdi-progress-clock"
          :title="t('tasks')"
          :aria-label="t('tasks')"
          @click="navigateTasks"
        />
        <v-btn
          v-if="(!authState.authEnabled.value || authState.authenticated.value) && !smAndDown"
          variant="text"
          class="navbar-icon-btn"
          icon="mdi-cog-outline"
          :title="t('settings')"
          :aria-label="t('settings')"
          @click="navigateSettings"
        />
        <v-menu v-if="(!authState.authEnabled.value || authState.authenticated.value) && smAndDown" location="bottom end">
          <template #activator="{ props }">
            <v-btn
              v-bind="props"
              variant="text"
              class="navbar-icon-btn"
              icon="mdi-dots-vertical"
              :title="t('settings')"
              :aria-label="t('settings')"
            />
          </template>
          <v-list density="compact" min-width="148">
            <v-list-item prepend-icon="mdi-progress-clock" :title="t('tasks')" @click="navigateTasks" />
            <v-list-item prepend-icon="mdi-cog-outline" :title="t('settings')" @click="navigateSettings" />
          </v-list>
        </v-menu>
        <v-btn
          variant="tonal"
          class="pill-button theme-toggle-btn"
          :size="smAndDown ? 'small' : 'default'"
          :icon="smAndDown"
          :title="isDark ? t('darkModeOn') : t('lightModeOn')"
          :aria-label="isDark ? t('darkModeOn') : t('lightModeOn')"
          @click="toggleTheme"
        >
          <v-icon :start="!smAndDown">{{ isDark ? 'mdi-weather-night' : 'mdi-white-balance-sunny' }}</v-icon>
          <span v-if="!smAndDown">{{ isDark ? t('darkModeOn') : t('lightModeOn') }}</span>
        </v-btn>
        <v-btn
          v-if="authState.authEnabled.value && !authState.authenticated.value"
          variant="tonal"
          class="pill-button"
          color="primary"
          :size="smAndDown ? 'small' : 'default'"
          :icon="smAndDown ? 'mdi-login' : undefined"
          @click="navigateLogin"
        >
          <span v-if="!smAndDown">{{ t('login') }}</span>
        </v-btn>
        <v-btn
          v-if="authState.authEnabled.value && authState.authenticated.value"
          variant="tonal"
          class="pill-button"
          color="error"
          :size="smAndDown ? 'small' : 'default'"
          :icon="'mdi-logout'"
          :title="t('logout')"
          :aria-label="t('logout')"
          @click="doLogout"
        />
      </div>
    </template>
  </v-app-bar>
</template>

<style scoped>
.navbar-shell {
  padding-inline: 10px;
}

.navbar-shell :deep(.v-toolbar__content) {
  min-width: 0;
  gap: 8px;
  padding-inline: 8px;
}

.navbar-shell :deep(.v-toolbar-title__placeholder) {
  min-width: 0;
}

.navbar-title {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
}

.navbar-title :deep(.v-btn) {
  justify-content: flex-start;
}

.navbar-brand {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  padding-inline: 10px;
  font-size: 1rem;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.navbar-brand :deep(.v-btn__content) {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
  gap: 8px;
}

.navbar-brand-icon {
  flex: 0 0 auto;
}

.navbar-brand-label {
  flex: 1 1 auto;
  display: inline-block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.navbar-actions {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: nowrap;
  gap: 6px;
  min-width: 0;
}

.navbar-icon-btn {
  flex: 0 0 auto;
}

.navbar-actions :deep(.v-btn) {
  flex: 0 0 auto;
  min-width: 34px;
}

.navbar-actions :deep(.v-btn__content) {
  min-width: 0;
}

.theme-toggle-btn {
  min-width: 42px;
}

.theme-toggle-btn :deep(.v-btn__content) {
  min-width: 18px;
}

@media (max-width: 720px) {
  .navbar-shell {
    padding-inline: 4px;
  }

  .navbar-shell :deep(.v-toolbar__content) {
    gap: 4px;
    padding-inline: 4px;
  }

  .navbar-brand {
    min-width: 0;
    padding-inline: 6px;
    max-width: calc(100vw - 174px);
  }

  .navbar-brand-label {
    font-size: 0.92rem;
  }

  .navbar-actions {
    gap: 4px;
  }

  .navbar-actions :deep(.v-btn) {
    min-width: 32px;
    padding-inline: 6px;
  }

  .theme-toggle-btn {
    min-width: 34px;
  }
}
</style>
