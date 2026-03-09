export interface FileItem {
  name: string;
  path: string;
  isDirectory: boolean;
  size: number;
  modifiedTime: string;
}

export interface VideoSegment {
  startTime: string;
  endTime: string;
}

export interface VideoEditPayload {
  videoPath: string;
  segments: VideoSegment[];
}

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

export interface PublicSettings {
  baseDir: string;
  videoOutputDir: string;
  authEnabled: boolean;
  logDir: string;
  logLevel: string;
  logRotationHours: number;
  logMaxAgeDays: number;
  taskPollIntervalMs: number;
}