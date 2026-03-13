import { api } from './api';
import type { FileTaskResponse, VideoEditPayload, VideoOptionsResponse } from '../types';

export async function fetchVideoOptions() {
  const response = await api.get<VideoOptionsResponse>('/api/video/options');
  return response.data.codecs;
}

export async function createVideoTask(payload: VideoEditPayload): Promise<FileTaskResponse> {
  const response = await api.post<FileTaskResponse>('/api/tasks/video', payload);
  return response.data;
}
