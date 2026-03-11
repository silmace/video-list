import { api } from './api';
import type { TaskDetailResponse, TaskListResponse } from '../types';

export async function fetchTasks() {
  const response = await api.get<TaskListResponse>('/api/tasks');
  return response.data.tasks;
}

export async function fetchTask(taskId: string) {
  const response = await api.get<TaskDetailResponse>(`/api/tasks/${taskId}`);
  return response.data.task;
}

export async function cancelTask(taskId: string): Promise<void> {
  await api.delete(`/api/tasks/${taskId}`);
}
