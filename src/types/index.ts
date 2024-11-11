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