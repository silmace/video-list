export interface VideoSegment {
  startTime: string;
  endTime: string;
}

export type VideoExportMode = 'copy' | 'transcode';

export interface VideoCodecOption {
  id: string;
  label: string;
  description: string;
  container: 'mp4' | 'mkv';
  mode: VideoExportMode;
  available: boolean;
}

export interface VideoEditPayload {
  videoPath: string;
  segments: VideoSegment[];
  exportMode: VideoExportMode;
  videoCodec: string;
}

export interface VideoOptionsResponse {
  success: boolean;
  codecs: VideoCodecOption[];
}
