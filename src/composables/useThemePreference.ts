import { computed, ref, watch } from 'vue';
import { useTheme } from 'vuetify';

export type AppThemeName = 'light' | 'dark';

const STORAGE_KEY = 'video_list_theme';
const themeName = ref<AppThemeName>((localStorage.getItem(STORAGE_KEY) || localStorage.getItem('theme')) === 'dark' ? 'dark' : 'light');
let initialized = false;

function applyTheme(nextTheme: AppThemeName, setVuetifyTheme: (name: AppThemeName) => void) {
  setVuetifyTheme(nextTheme);
  document.body.dataset.theme = nextTheme;
  localStorage.setItem(STORAGE_KEY, nextTheme);
}

export function useThemePreference() {
  const theme = useTheme();

  if (!initialized) {
    initialized = true;
    watch(
      themeName,
      (nextTheme) => {
        applyTheme(nextTheme, (name) => {
          theme.global.name.value = name;
        });
      },
      { immediate: true }
    );
  } else {
    applyTheme(themeName.value, (name) => {
      theme.global.name.value = name;
    });
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