import { computed, ref, watch } from 'vue';
import { getStoredString, setStoredString } from '@/lib/safeStorage';

export type AppThemeName = 'light' | 'dark';

const STORAGE_KEY = 'video_list_theme';
const themeName = ref<AppThemeName>((getStoredString(STORAGE_KEY) || getStoredString('theme')) === 'dark' ? 'dark' : 'light');
let initialized = false;

function applyTheme(nextTheme: AppThemeName) {
  document.body.dataset.theme = nextTheme;
  document.documentElement.dataset.theme = nextTheme;
  setStoredString(STORAGE_KEY, nextTheme);
}

export function useThemePreference() {
  if (!initialized) {
    initialized = true;
    watch(
      themeName,
      (nextTheme) => {
        applyTheme(nextTheme);
      },
      { immediate: true }
    );
  } else {
    applyTheme(themeName.value);
  }

  const setTheme = (nextTheme: AppThemeName) => {
    themeName.value = nextTheme;
  };

  const toggleTheme = () => {
    themeName.value = themeName.value === 'dark' ? 'light' : 'dark';
  };

  return {
    themeName,
    isDark: computed(() => themeName.value === 'dark'),
    setTheme,
    toggleTheme,
  };
}