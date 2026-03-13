<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { useDisplay } from 'vuetify';
import { authState } from '../composables/useAuth';
import type { TaskItem } from '../types';
import { useLocale } from '../composables/useLocale';
import { cancelTask as cancelTaskRequest, fetchTasks as loadTasks } from '../services/tasks';

const router = useRouter();
const tasks = ref<TaskItem[]>([]);
const loading = ref(false);
const snackbar = ref({ show: false, message: '', color: 'success' });
let timer: ReturnType<typeof setInterval> | null = null;
const { t } = useLocale();
const { smAndDown } = useDisplay();

const activeCount = computed(() =>
  tasks.value.filter((task) => task.status === 'pending' || task.status === 'running').length
);

const headers = computed(() => [
  { title: t('type'), key: 'type' },
  { title: t('status'), key: 'status' },
  { title: t('progress'), key: 'progress' },
  { title: t('stage'), key: 'stage' },
  { title: t('taskDetails'), key: 'message' },
  { title: t('updated'), key: 'updatedAt' },
  { title: t('actions'), key: 'actions', sortable: false },
]);

const statusLabel = (status: TaskItem['status']) => t(`status_${status}`);

const taskTypeLabel = (type: string) => {
  const key = `task_type_${type}`;
  const translated = t(key);
  return translated === key ? type : translated;
};

const taskStageLabel = (stage: string) => {
  const key = `task_stage_${stage}`;
  const translated = t(key);
  return translated === key ? stage : translated;
};

const taskDetailText = (task: TaskItem) => {
  if (task.type === 'batch-delete') {
    if (task.status === 'success') {
      return t('taskDeleteDone', { count: task.total || 0 });
    }
    if (task.status === 'running') {
      return t('taskDeleteProgress', {
        current: task.current || 0,
        total: task.total || 0,
        item: task.currentItem || '-',
      });
    }
  }

  if (task.type === 'batch-move') {
    if (task.status === 'success') {
      return t('taskMoveDone', { count: task.total || 0 });
    }
    if (task.status === 'running') {
      return t('taskMoveProgress', {
        current: task.current || 0,
        total: task.total || 0,
        item: task.currentItem || '-',
      });
    }
  }

  if (task.type === 'batch-copy') {
    if (task.status === 'success') {
      return t('taskCopyDone', { count: task.total || 0 });
    }
    if (task.status === 'running') {
      return t('taskCopyProgress', {
        current: task.current || 0,
        total: task.total || 0,
        item: task.currentItem || '-',
      });
    }
  }

  if (task.type === 'video-edit' && task.status === 'running') {
    return t('taskVideoProgress', {
      progress: task.progress,
      stage: taskStageLabel(task.stage),
    });
  }

  if (task.error) {
    return task.error;
  }

  return task.message;
};

const showSnackbar = (message: string, color: string) => {
  snackbar.value = { show: true, message, color };
};

const fetchTasks = async () => {
  loading.value = true;
  try {
    tasks.value = await loadTasks();
  } catch {
    showSnackbar(t('tasksLoadError'), 'error');
  } finally {
    loading.value = false;
  }
};

const cancelTask = async (taskId: string) => {
  try {
    await cancelTaskRequest(taskId);
    showSnackbar(t('taskCanceled'), 'success');
    await fetchTasks();
  } catch {
    showSnackbar(t('taskCancelError'), 'error');
  }
};

const openOutputFolder = (outputPath: string) => {
  const cleanPath = outputPath.startsWith('/') ? outputPath.slice(1) : outputPath;
  const segments = cleanPath.split('/');
  segments.pop();
  const dir = segments.length ? `/${segments.join('/')}/` : '/';
  router.push(dir);
};

onMounted(async () => {
  await fetchTasks();
  const interval = authState.taskPollIntervalMs.value || 1500;
  timer = setInterval(fetchTasks, interval);
});

onUnmounted(() => {
  if (timer) {
    clearInterval(timer);
    timer = null;
  }
});
</script>

<template>
  <v-container class="app-page">
    <v-card class="glass-panel pa-4 mb-4">
      <div class="d-flex align-center">
        <div>
          <div class="text-h5">{{ t('taskCenter') }}</div>
          <div class="text-medium-emphasis">{{ t('activeTasks', { count: activeCount }) }}</div>
        </div>
        <v-spacer />
        <v-btn color="primary" variant="tonal" @click="fetchTasks">{{ t('refresh') }}</v-btn>
      </div>
    </v-card>

    <v-card class="glass-panel pa-2">
      <div v-if="smAndDown" class="task-mobile-list">
        <v-card
          v-for="item in tasks"
          :key="item.id"
          variant="tonal"
          class="task-mobile-card"
        >
          <v-card-text>
            <div class="mobile-row">
              <v-chip size="small" variant="outlined">{{ taskTypeLabel(item.type) }}</v-chip>
              <v-chip
                size="small"
                :color="
                  item.status === 'success'
                    ? 'success'
                    : item.status === 'failed'
                      ? 'error'
                      : item.status === 'canceled'
                        ? 'warning'
                        : 'info'
                "
                variant="tonal"
              >
                {{ statusLabel(item.status) }}
              </v-chip>
            </div>
            <div class="task-mobile-stage">{{ taskStageLabel(item.stage) }}</div>
            <v-progress-linear :model-value="item.progress" height="8" rounded class="my-2" />
            <div class="task-detail-cell">
              <div>{{ taskDetailText(item) }}</div>
              <div v-if="item.currentItem" class="task-detail-sub">{{ item.currentItem }}</div>
            </div>
            <div class="task-mobile-updated">{{ new Date(item.updatedAt).toLocaleString() }}</div>
            <div class="mobile-row mt-2">
              <v-btn
                v-if="item.status === 'running' || item.status === 'pending'"
                color="warning"
                variant="text"
                size="small"
                @click="cancelTask(item.id)"
              >
                {{ t('cancel') }}
              </v-btn>
              <v-btn
                v-if="item.outputPath"
                color="primary"
                variant="text"
                size="small"
                @click="openOutputFolder(item.outputPath)"
              >
                {{ t('openOutput') }}
              </v-btn>
            </div>
          </v-card-text>
        </v-card>

        <div v-if="!loading && tasks.length === 0" class="task-empty">
          {{ t('noTasksYet') }}
        </div>
      </div>

      <v-data-table
        v-else
        :items="tasks"
        :loading="loading"
        :headers="headers"
        item-value="id"
      >
        <template #item.status="{ item }">
          <v-chip
            size="small"
            :color="
              item.status === 'success'
                ? 'success'
                : item.status === 'failed'
                  ? 'error'
                  : item.status === 'canceled'
                    ? 'warning'
                    : 'info'
            "
            variant="tonal"
          >
            {{ statusLabel(item.status) }}
          </v-chip>
        </template>

        <template #item.type="{ item }">
          <v-chip size="small" variant="outlined">
            {{ taskTypeLabel(item.type) }}
          </v-chip>
        </template>

        <template #item.stage="{ item }">
          <span class="stage-label">{{ taskStageLabel(item.stage) }}</span>
        </template>

        <template #item.message="{ item }">
          <div class="task-detail-cell">
            <div>{{ taskDetailText(item) }}</div>
            <div v-if="item.currentItem" class="task-detail-sub">{{ item.currentItem }}</div>
          </div>
        </template>

        <template #item.progress="{ item }">
          <v-progress-linear :model-value="item.progress" height="8" rounded />
        </template>

        <template #item.updatedAt="{ item }">
          {{ new Date(item.updatedAt).toLocaleString() }}
        </template>

        <template #item.actions="{ item }">
          <div class="d-flex ga-2">
            <v-btn
              v-if="item.status === 'running' || item.status === 'pending'"
              color="warning"
              variant="text"
              @click="cancelTask(item.id)"
            >
              {{ t('cancel') }}
            </v-btn>
            <v-btn
              v-if="item.outputPath"
              color="primary"
              variant="text"
              @click="openOutputFolder(item.outputPath)"
            >
              {{ t('openOutput') }}
            </v-btn>
          </div>
        </template>
      </v-data-table>
    </v-card>

    <v-snackbar v-model="snackbar.show" :color="snackbar.color">
      {{ snackbar.message }}
    </v-snackbar>
  </v-container>
</template>

<style scoped>
.stage-label {
  color: rgb(var(--v-theme-primary));
}

.task-detail-cell {
  max-width: 460px;
  white-space: normal;
  line-height: 1.35;
}

.task-detail-sub {
  margin-top: 3px;
  font-size: 12px;
  color: rgba(100, 116, 139, 0.95);
  word-break: break-all;
}

.task-mobile-list {
  display: grid;
  gap: 10px;
  padding: 6px;
}

.task-mobile-card {
  border-radius: 14px;
}

.mobile-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  flex-wrap: wrap;
}

.task-mobile-stage {
  margin-top: 8px;
  color: rgb(var(--v-theme-primary));
  font-size: 12px;
}

.task-mobile-updated {
  margin-top: 8px;
  color: rgba(100, 116, 139, 0.95);
  font-size: 12px;
}

.task-empty {
  padding: 16px 10px;
  color: rgba(100, 116, 139, 0.95);
  text-align: center;
}
</style>
