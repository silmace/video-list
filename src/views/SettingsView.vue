<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { api } from '../services/api';
import { checkAuthStatus } from '../composables/useAuth';
import { useLocale, type AppLocale } from '../composables/useLocale';
import { useFileVisuals } from '../composables/useFileVisuals';

interface SettingsResponse {
  success: boolean;
  settings: {
    baseDir: string;
    videoOutputDir: string;
    authEnabled: boolean;
    logDir: string;
    logLevel: string;
    logRotationHours: number;
    logMaxAgeDays: number;
    taskPollIntervalMs: number;
  };
}

const loading = ref(false);
const saving = ref(false);
const snackbar = ref({ show: false, message: '', color: 'success' });

const baseDir = ref('');
const videoOutputDir = ref('');
const logDir = ref('');
const logLevel = ref('info');
const logRotationHours = ref(24);
const logMaxAgeDays = ref(7);
const taskPollIntervalMs = ref(1500);
const currentPassword = ref('');
const newPassword = ref('');
const authEnabled = ref(false);
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
    const response = await api.get<SettingsResponse>('/api/settings');
    baseDir.value = response.data.settings.baseDir;
    videoOutputDir.value = response.data.settings.videoOutputDir;
    logDir.value = response.data.settings.logDir;
    logLevel.value = response.data.settings.logLevel;
    logRotationHours.value = response.data.settings.logRotationHours;
    logMaxAgeDays.value = response.data.settings.logMaxAgeDays;
    taskPollIntervalMs.value = response.data.settings.taskPollIntervalMs;
    authEnabled.value = response.data.settings.authEnabled;
  } catch {
    showSnackbar(t('settingsLoadError'), 'error');
  } finally {
    loading.value = false;
  }
};

const saveSettings = async () => {
  saving.value = true;
  try {
    await api.put('/api/settings', {
      baseDir: baseDir.value,
      videoOutputDir: videoOutputDir.value,
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

onMounted(loadSettings);
</script>

<template>
  <v-container class="pa-4">
    <v-card class="glass-panel pa-4">
      <v-card-title class="text-h5">{{ t('settingsTitle') }}</v-card-title>
      <v-card-subtitle class="mb-4">{{ t('settingsSubtitle') }}</v-card-subtitle>

      <v-progress-linear v-if="loading" indeterminate color="primary" class="mb-4" />

      <v-row>
        <v-col cols="12" md="5">
          <v-text-field v-model="baseDir" :label="t('baseDir')" variant="outlined" />
        </v-col>
        <v-col cols="12" md="3">
          <v-text-field
            v-model="videoOutputDir"
            :label="t('videoOutputDir')"
            :hint="t('videoOutputDirHint')"
            persistent-hint
            variant="outlined"
          />
        </v-col>
        <v-col cols="12" md="2">
          <v-text-field v-model="taskPollIntervalMs" type="number" :label="t('taskPollMs')" variant="outlined" />
        </v-col>
        <v-col cols="12" md="2">
          <v-select
            :model-value="locale"
            :items="localeOptions"
            item-title="title"
            item-value="value"
            :label="t('language')"
            variant="outlined"
            @update:model-value="onLocaleChange"
          />
        </v-col>
      </v-row>

      <v-row>
        <v-col cols="12" md="5">
          <v-text-field v-model="logDir" :label="t('logDir')" variant="outlined" />
        </v-col>
        <v-col cols="12" md="3">
          <v-select
            v-model="logLevel"
            :items="['debug', 'info', 'warn', 'error']"
            :label="t('logLevel')"
            variant="outlined"
          />
        </v-col>
        <v-col cols="12" md="2">
          <v-text-field v-model="logRotationHours" type="number" :label="t('rotateHours')" variant="outlined" />
        </v-col>
        <v-col cols="12" md="2">
          <v-text-field v-model="logMaxAgeDays" type="number" :label="t('keepDays')" variant="outlined" />
        </v-col>
      </v-row>

      <v-divider class="my-4" />
      <div class="text-subtitle-1 mb-2">{{ t('passwordProtection') }}</div>
      <v-row>
        <v-col cols="12" md="6" v-if="authEnabled">
          <v-text-field
            v-model="currentPassword"
            type="password"
            :label="t('currentPassword')"
            variant="outlined"
          />
        </v-col>
        <v-col cols="12" md="6">
          <v-text-field
            v-model="newPassword"
            type="password"
            :label="t('newPasswordLabel')"
            variant="outlined"
            :hint="t('passwordHint')"
            persistent-hint
          />
        </v-col>
      </v-row>

      <v-card-actions class="px-0 pt-2">
        <v-spacer />
        <v-btn color="primary" :loading="saving" @click="saveSettings">{{ t('saveSettings') }}</v-btn>
      </v-card-actions>

      <v-divider class="my-6" />
      <div class="text-subtitle-1 mb-2">{{ t('colorTagsTitle') }}</div>
      <div class="text-medium-emphasis mb-4">{{ t('colorTagsSubtitle') }}</div>

      <v-row>
        <v-col cols="12" md="3">
          <v-text-field v-model="newTagLabel" :label="t('tagLabel')" variant="outlined" />
        </v-col>
        <v-col cols="12" md="5">
          <v-text-field
            v-model="newTagPattern"
            :label="t('tagPattern')"
            :hint="t('tagPatternHint')"
            persistent-hint
            variant="outlined"
          />
        </v-col>
        <v-col cols="8" md="2">
          <v-text-field v-model="newTagColor" :label="t('tagColor')" variant="outlined" />
        </v-col>
        <v-col cols="4" md="2" class="d-flex align-center">
          <input v-model="newTagColor" type="color" class="color-input" />
          <v-btn color="primary" variant="tonal" class="ml-3" @click="createCustomTag">{{ t('create') }}</v-btn>
        </v-col>
      </v-row>

      <div class="d-flex flex-wrap ga-2 mb-3">
        <v-chip
          v-for="tag in tagList"
          :key="tag.id"
          closable
          :style="{ borderColor: tag.color, color: tag.color }"
          variant="outlined"
          @click:close="removeTag(tag.id)"
        >
          {{ tag.label }} · {{ tag.pattern }}
        </v-chip>
      </div>

      <v-btn variant="text" color="warning" @click="resetTags">{{ t('resetDefaultTags') }}</v-btn>
    </v-card>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color">
      {{ snackbar.message }}
    </v-snackbar>
  </v-container>
</template>

<style scoped>
.color-input {
  width: 38px;
  height: 38px;
  border-radius: 8px;
  border: 1px solid rgba(148, 163, 184, 0.45);
  padding: 2px;
  background: transparent;
}
</style>
