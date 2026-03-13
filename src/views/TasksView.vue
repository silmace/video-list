<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { authState } from '../composables/useAuth';
import type { TaskItem } from '../types';
import { useLocale } from '../composables/useLocale';
import { cancelTask as cancelTaskRequest, fetchTasks as loadTasks } from '../services/tasks';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';

const router = useRouter();
const tasks = ref<TaskItem[]>([]);
const loading = ref(false);
const snackbar = ref({ show: false, message: '', color: 'success' });
let timer: ReturnType<typeof setInterval> | null = null;
const { t } = useLocale();

const activeCount = computed(() =>
  tasks.value.filter((task) => task.status === 'pending' || task.status === 'running').length
);

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

const statusVariant = (status: TaskItem['status']) => {
  if (status === 'success') {
    return 'success';
  }
  if (status === 'failed') {
    return 'destructive';
  }
  if (status === 'canceled') {
    return 'secondary';
  }
  return 'default';
};

const progressClass = (status: TaskItem['status']) => {
  if (status === 'success') {
    return 'bg-emerald-600';
  }
  if (status === 'failed') {
    return 'bg-rose-600';
  }
  if (status === 'canceled') {
    return 'bg-amber-500';
  }
  return 'bg-[var(--accent)]';
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
  <div class="app-page grid gap-4">
    <Card class="glass-panel p-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-2xl font-extrabold tracking-tight">{{ t('taskCenter') }}</h1>
          <p class="text-sm text-[var(--text-2)]">{{ t('activeTasks', { count: activeCount }) }}</p>
        </div>
        <Button variant="outline" @click="fetchTasks">{{ t('refresh') }}</Button>
      </div>
    </Card>

    <Card class="glass-panel p-3">
      <div v-if="loading && tasks.length === 0" class="p-6 text-sm text-[var(--text-2)]">
        {{ t('loadingFiles') }}
      </div>

      <div v-else-if="tasks.length === 0" class="p-6 text-sm text-[var(--text-2)]">
        {{ t('noTasksYet') }}
      </div>

      <div v-else class="grid gap-3">
        <article v-for="item in tasks" :key="item.id" class="task-card rounded-xl border border-[var(--border-soft)] p-4">
          <div class="flex flex-wrap items-center gap-2">
            <Badge variant="outline">{{ taskTypeLabel(item.type) }}</Badge>
            <Badge :variant="statusVariant(item.status)">{{ statusLabel(item.status) }}</Badge>
            <span class="ml-auto text-xs text-[var(--text-3)]">{{ new Date(item.updatedAt).toLocaleString() }}</span>
          </div>

          <div class="mt-2 text-xs font-semibold uppercase tracking-wide text-[var(--accent)]">
            {{ taskStageLabel(item.stage) }}
          </div>

          <div class="mt-2 h-2 w-full overflow-hidden rounded-full bg-[var(--surface-3)]">
            <div class="h-full transition-all" :class="progressClass(item.status)" :style="{ width: `${item.progress}%` }" />
          </div>

          <div class="mt-3 text-sm leading-relaxed text-[var(--text-1)]">
            {{ taskDetailText(item) }}
          </div>
          <div v-if="item.currentItem" class="mt-1 break-all text-xs text-[var(--text-3)]">
            {{ item.currentItem }}
          </div>

          <div class="mt-3 flex flex-wrap gap-2">
            <Button
              v-if="item.status === 'running' || item.status === 'pending'"
              variant="secondary"
              size="sm"
              @click="cancelTask(item.id)"
            >
              {{ t('cancel') }}
            </Button>
            <Button
              v-if="item.outputPath"
              variant="outline"
              size="sm"
              @click="openOutputFolder(item.outputPath)"
            >
              {{ t('openOutput') }}
            </Button>
          </div>
        </article>
      </div>
    </Card>

    <div v-if="snackbar.show" class="snackbar" :class="`snackbar-${snackbar.color}`">
      {{ snackbar.message }}
    </div>
  </div>
</template>

<style scoped>
.task-card {
  background: color-mix(in srgb, var(--surface-2) 88%, transparent);
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
</style>
