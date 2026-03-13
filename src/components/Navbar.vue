<script setup lang="ts">
import { useRouter } from 'vue-router';
import { onMounted } from 'vue';
import { FolderKanban, ListTodo, LogIn, LogOut, MoonStar, Settings, SunMedium } from 'lucide-vue-next';
import { authState, checkAuthStatus, logout } from '../composables/useAuth';
import { useLocale } from '../composables/useLocale';
import { useThemePreference } from '../composables/useThemePreference';
import { Button } from '@/components/ui/button';

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
  <header class="glass-appbar navbar-shell">
    <div class="navbar-row">
      <Button variant="ghost" class="navbar-brand" @click="navigateHome">
        <FolderKanban :size="18" />
        <span class="brand-title">{{ t('appTitle') }}</span>
      </Button>

      <div class="navbar-actions">
        <Button
          v-if="!authState.authEnabled.value || authState.authenticated.value"
          variant="ghost"
          size="icon"
          :title="t('tasks')"
          :aria-label="t('tasks')"
          @click="navigateTasks"
        >
          <ListTodo :size="18" />
        </Button>
        <Button
          v-if="!authState.authEnabled.value || authState.authenticated.value"
          variant="ghost"
          size="icon"
          :title="t('settings')"
          :aria-label="t('settings')"
          @click="navigateSettings"
        >
          <Settings :size="18" />
        </Button>
        <Button
          variant="outline"
          class="theme-toggle"
          :title="isDark ? t('darkModeOn') : t('lightModeOn')"
          :aria-label="isDark ? t('darkModeOn') : t('lightModeOn')"
          @click="toggleTheme"
        >
          <MoonStar v-if="isDark" :size="16" />
          <SunMedium v-else :size="16" />
          <span class="theme-label">{{ isDark ? t('darkModeOn') : t('lightModeOn') }}</span>
        </Button>
        <Button
          v-if="authState.authEnabled.value && !authState.authenticated.value"
          variant="default"
          @click="navigateLogin"
        >
          <LogIn :size="16" />
          {{ t('login') }}
        </Button>
        <Button
          v-if="authState.authEnabled.value && authState.authenticated.value"
          variant="destructive"
          size="icon"
          title="Logout"
          aria-label="Logout"
          @click="doLogout"
        >
          <LogOut :size="16" />
        </Button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.navbar-shell {
  position: sticky;
  top: 0;
  z-index: 20;
  padding: 10px 12px;
}

.navbar-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.navbar-brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border-radius: 999px;
  font-weight: 800;
}

.brand-title {
  letter-spacing: -0.02em;
}

.navbar-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.theme-toggle {
  border-radius: 999px;
}

.theme-label {
  display: inline-flex;
  align-items: center;
}

@media (max-width: 720px) {
  .navbar-shell {
    padding: 8px;
  }

  .brand-title,
  .theme-label {
    display: none;
  }

  .navbar-actions {
    gap: 6px;
  }
}
</style>
