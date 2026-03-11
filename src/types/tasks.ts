export interface TaskItem {
  id: string;
  type: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'canceled';
  progress: number;
  stage: string;
  message: string;
  error?: string;
  outputPath?: string;
  total?: number;
  current?: number;
  currentItem?: string;
  detail?: string;
  createdAt: string;
  updatedAt: string;
}

export interface TaskListResponse {
  success: boolean;
  tasks: TaskItem[];
}

export interface TaskDetailResponse {
  success: boolean;
  task: TaskItem;
}
