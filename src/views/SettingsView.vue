<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { checkAuthStatus } from '../composables/useAuth';
import { useLocale, type AppLocale } from '../composables/useLocale';
import { useFileVisuals } from '../composables/useFileVisuals';
import type { VideoCodecOption } from '../types';
import { fetchSettings, updateSettings } from '../services/settings';
import { fetchVideoOptions } from '../services/video';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';

const loading = ref(false);
const saving = ref(false);
const snackbar = ref({ show: false, message: '', color: 'success' });

const baseDir = ref('');
const videoOutputDir = ref('');
const showHiddenItems = ref(false);
const logDir = ref('');
const logLevel = ref('info');
const logRotationHours = ref(24);
const logMaxAgeDays = ref(7);
const taskPollIntervalMs = ref(1500);
const currentPassword = ref('');
const newPassword = ref('');
const authEnabled = ref(false);
const codecOptions = ref<VideoCodecOption[]>([]);
const { t, locale, localeOptions, setLocale } = useLocale();
const { tagList, addTag, removeTag, resetTags } = useFileVisuals();
const newTagLabel = ref('');
const newTagPattern = ref('');
const newTagColor = ref('#3B82F6');

const showSnackbar = (message: string, color: string) => {
  snackbar.value = { show: true, message, color };
};

const onLocaleChange = (value: AppLocale | null) => {
  if (value) {
    setLocale(value);
  }
};

const onLocaleChangeFromEvent = (event: Event) => {
  const target = event.target as HTMLSelectElement;
  const value = target.value as AppLocale;
  onLocaleChange(value);
};

const createCustomTag = () => {
  const label = newTagLabel.value.trim();
  const pattern = newTagPattern.value.trim();
  if (!label || !pattern) {
    showSnackbar(t('tagRequiredHint'), 'warning');
    return;
  }

  addTag({
    label,
    pattern,
    color: newTagColor.value,
  });

  newTagLabel.value = '';
  newTagPattern.value = '';
  newTagColor.value = '#3B82F6';
  showSnackbar(t('tagSaved'), 'success');
};

const loadSettings = async () => {
  loading.value = true;
  try {
    const settings = await fetchSettings();
    baseDir.value = settings.baseDir;
    videoOutputDir.value = settings.videoOutputDir;
    showHiddenItems.value = settings.showHiddenItems;
    logDir.value = settings.logDir;
    logLevel.value = settings.logLevel;
    logRotationHours.value = settings.logRotationHours;
    logMaxAgeDays.value = settings.logMaxAgeDays;
    taskPollIntervalMs.value = settings.taskPollIntervalMs;
    authEnabled.value = settings.authEnabled;
  } catch {
    showSnackbar(t('settingsLoadError'), 'error');
  } finally {
    loading.value = false;
  }
};

const saveSettings = async () => {
  saving.value = true;
  try {
    await updateSettings({
      baseDir: baseDir.value,
      videoOutputDir: videoOutputDir.value,
      showHiddenItems: showHiddenItems.value,
      logDir: logDir.value,
      logLevel: logLevel.value,
      logRotationHours: logRotationHours.value,
      logMaxAgeDays: logMaxAgeDays.value,
      taskPollIntervalMs: taskPollIntervalMs.value,
      currentPassword: currentPassword.value,
      newPassword: newPassword.value,
    });

    currentPassword.value = '';
    newPassword.value = '';
    await checkAuthStatus();
    await loadSettings();
    showSnackbar(t('settingsSaved'), 'success');
  } catch {
    showSnackbar(t('settingsSaveError'), 'error');
  } finally {
    saving.value = false;
  }
};

const loadCodecOptions = async () => {
  try {
    codecOptions.value = await fetchVideoOptions();
  } catch {
    codecOptions.value = [];
  }
};

onMounted(async () => {
  await Promise.all([loadSettings(), loadCodecOptions()]);
});
</script>

<template>
  <div class="app-page grid gap-4">
    <Card class="glass-panel p-4 md:p-6">
      <h1 class="text-2xl font-extrabold tracking-tight">{{ t('settingsTitle') }}</h1>
      <p class="mt-1 text-sm text-[var(--text-2)]">{{ t('settingsSubtitle') }}</p>

      <div v-if="loading" class="mt-4 h-2 w-full overflow-hidden rounded-full bg-[var(--surface-3)]">
        <div class="h-full w-1/3 animate-pulse bg-[var(--accent)]" />
      </div>

      <section class="settings-grid mt-5">
        <label class="field-shell md:col-span-2">
          <span>{{ t('baseDir') }}</span>
          <input v-model="baseDir" type="text">
        </label>
        <label class="field-shell md:col-span-1">
          <span>{{ t('videoOutputDir') }}</span>
          <input v-model="videoOutputDir" type="text">
          <small>{{ t('videoOutputDirHint') }}</small>
        </label>
        <label class="field-shell">
          <span>{{ t('taskPollMs') }}</span>
          <input v-model.number="taskPollIntervalMs" type="number" min="500" step="100">
        </label>
        <label class="field-shell">
          <span>{{ t('language') }}</span>
          <select :value="locale" @change="onLocaleChangeFromEvent">
            <option v-for="option in localeOptions" :key="option.value" :value="option.value">{{ option.title }}</option>
          </select>
        </label>
        <label class="checkbox-shell md:col-span-2">
          <input v-model="showHiddenItems" type="checkbox">
          <span>{{ t('includeHiddenInList') }}</span>
        </label>
      </section>

      <section class="settings-grid mt-4">
        <label class="field-shell md:col-span-2">
          <span>{{ t('logDir') }}</span>
          <input v-model="logDir" type="text">
        </label>
        <label class="field-shell">
          <span>{{ t('logLevel') }}</span>
          <select v-model="logLevel">
            <option value="debug">debug</option>
            <option value="info">info</option>
            <option value="warn">warn</option>
            <option value="error">error</option>
          </select>
        </label>
        <label class="field-shell">
          <span>{{ t('rotateHours') }}</span>
          <input v-model.number="logRotationHours" type="number" min="1">
        </label>
        <label class="field-shell">
          <span>{{ t('keepDays') }}</span>
          <input v-model.number="logMaxAgeDays" type="number" min="1">
        </label>
      </section>

      <section class="mt-6 grid gap-3">
        <h2 class="text-lg font-bold">{{ t('passwordProtection') }}</h2>
        <div class="settings-grid">
          <label v-if="authEnabled" class="field-shell md:col-span-2">
            <span>{{ t('currentPassword') }}</span>
            <input v-model="currentPassword" type="password">
          </label>
          <label class="field-shell md:col-span-2">
            <span>{{ t('newPasswordLabel') }}</span>
            <input v-model="newPassword" type="password">
            <small>{{ t('passwordHint') }}</small>
          </label>
        </div>
      </section>

      <div class="mt-5 flex justify-end">
        <Button :disabled="saving" @click="saveSettings">{{ t('saveSettings') }}</Button>
      </div>

      <hr class="my-6 border-[var(--border-soft)]">

      <section class="grid gap-3">
        <h2 class="text-lg font-bold">{{ t('colorTagsTitle') }}</h2>
        <p class="text-sm text-[var(--text-2)]">{{ t('colorTagsSubtitle') }}</p>

        <div class="settings-grid">
          <label class="field-shell">
            <span>{{ t('tagLabel') }}</span>
            <input v-model="newTagLabel" type="text">
          </label>
          <label class="field-shell md:col-span-2">
            <span>{{ t('tagPattern') }}</span>
            <input v-model="newTagPattern" type="text">
            <small>{{ t('tagPatternHint') }}</small>
          </label>
          <label class="field-shell">
            <span>{{ t('tagColor') }}</span>
            <input v-model="newTagColor" type="text">
          </label>
          <div class="color-row">
            <input v-model="newTagColor" type="color" class="color-input">
            <Button variant="outline" @click="createCustomTag">{{ t('create') }}</Button>
          </div>
        </div>

        <div class="flex flex-wrap gap-2">
          <button
            v-for="tag in tagList"
            :key="tag.id"
            type="button"
            class="tag-chip"
            :style="{ borderColor: tag.color, color: tag.color }"
            @click="removeTag(tag.id)"
          >
            {{ tag.label }} · {{ tag.pattern }}
          </button>
        </div>

        <Button variant="secondary" class="w-fit" @click="resetTags">{{ t('resetDefaultTags') }}</Button>
      </section>

      <hr class="my-6 border-[var(--border-soft)]">

      <section class="grid gap-3">
        <h2 class="text-lg font-bold">{{ t('encoderCapabilitiesTitle') }}</h2>
        <p class="text-sm text-[var(--text-2)]">{{ t('encoderCapabilitiesSubtitle') }}</p>
        <div class="flex flex-wrap gap-2">
          <Badge
            v-for="codec in codecOptions"
            :key="codec.id"
            :variant="codec.available ? 'success' : 'secondary'"
          >
            {{ codec.label }} · {{ codec.available ? t('available') : t('unavailable') }}
          </Badge>
        </div>
      </section>
    </Card>

    <div v-if="snackbar.show" class="snackbar" :class="`snackbar-${snackbar.color}`">
      {{ snackbar.message }}
    </div>
  </div>
</template>

<style scoped>
.settings-grid {
  display: grid;
  gap: 12px;
  grid-template-columns: repeat(1, minmax(0, 1fr));
}

.field-shell {
  display: grid;
  gap: 6px;
  font-size: 13px;
  font-weight: 700;
  color: var(--text-2);
}

.field-shell input,
.field-shell select {
  width: 100%;
  min-height: 40px;
  border-radius: 10px;
  border: 1px solid var(--border-soft);
  background: color-mix(in srgb, var(--surface-3) 90%, transparent);
  padding: 8px 10px;
  color: var(--text-1);
}

.field-shell small {
  font-weight: 500;
  color: var(--text-3);
}

.checkbox-shell {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: var(--text-2);
  font-weight: 700;
}

.color-row {
  display: flex;
  align-items: end;
  gap: 10px;
}

.color-input {
  width: 38px;
  height: 38px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.45);
  padding: 2px;
  background: transparent;
}

.tag-chip {
  border: 1px solid var(--border-soft);
  border-radius: 999px;
  padding: 6px 10px;
  background: transparent;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.snackbar {
  position: fixed;
  right: 16px;
  bottom: 16px;
  z-index: 40;
  border-radius: 10px;
  padding: 10px 12px;
  color: #fff;
  box-shadow: var(--shadow-lg);
}

.snackbar-success {
  background: #166534;
}

.snackbar-error,
.snackbar-warning {
  background: #b45309;
}

.snackbar-info {
  background: #0f766e;
}

@media (min-width: 880px) {
  .settings-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}
</style>
