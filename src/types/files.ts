export interface FileItem {
  name: string;
  path: string;
  isDirectory: boolean;
  size: number;
  modifiedTime: string;
}

export type FileSortBy = 'name' | 'size' | 'modified';
export type FileSortOrder = 'asc' | 'desc';
export type FileFilterType = 'all' | 'folder' | 'video' | 'image' | 'audio' | 'archive' | 'document' | 'code' | 'other';

export interface FileQueryOptions {
  path: string;
  search?: string;
  sortBy?: FileSortBy;
  order?: FileSortOrder;
  type?: FileFilterType;
  includeHidden?: boolean;
}

export interface FileTaskResponse {
  success: boolean;
  taskId: string;
}

export interface FileActionResponse {
  success: boolean;
  path?: string;
}
