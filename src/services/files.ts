import type { AxiosProgressEvent } from 'axios';
import { api } from './api';
import type { FileActionResponse, FileItem, FileQueryOptions, FileTaskResponse } from '../types';

function buildFileParams(options: FileQueryOptions) {
  return {
    path: options.path,
    search: options.search || undefined,
    sortBy: options.sortBy || undefined,
    order: options.order || undefined,
    type: options.type && options.type !== 'all' ? options.type : undefined,
  };
}

export async function listFiles(options: FileQueryOptions): Promise<FileItem[]> {
  const response = await api.get<FileItem[]>('/api/files', {
    params: buildFileParams(options),
  });
  return response.data;
}

export async function createFolder(path: string, name: string): Promise<FileActionResponse> {
  const response = await api.post<FileActionResponse>('/api/files/mkdir', { path, name });
  return response.data;
}

export async function renameFile(path: string, name: string): Promise<FileActionResponse> {
  const response = await api.post<FileActionResponse>('/api/files/rename', { path, name });
  return response.data;
}

export async function uploadFile(
  path: string,
  file: File,
  overwrite = false,
  onUploadProgress?: (event: AxiosProgressEvent) => void
): Promise<FileActionResponse> {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('path', path);
  formData.append('overwrite', overwrite ? '1' : '0');
  const response = await api.post<FileActionResponse>('/api/files/upload', formData, {
    onUploadProgress,
  });
  return response.data;
}

export async function createBatchDeleteTask(paths: string[]): Promise<FileTaskResponse> {
  const response = await api.post<FileTaskResponse>('/api/tasks/batch-delete', { paths });
  return response.data;
}

export async function createBatchMoveTask(paths: string[], destination: string): Promise<FileTaskResponse> {
  const response = await api.post<FileTaskResponse>('/api/tasks/batch-move', { paths, destination });
  return response.data;
}

export async function createBatchCopyTask(paths: string[], destination: string): Promise<FileTaskResponse> {
  const response = await api.post<FileTaskResponse>('/api/tasks/batch-copy', { paths, destination });
  return response.data;
}
