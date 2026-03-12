export interface PublicSettings {
  baseDir: string;
  videoOutputDir: string;
  showHiddenItems: boolean;
  authEnabled: boolean;
  logDir: string;
  logLevel: string;
  logRotationHours: number;
  logMaxAgeDays: number;
  taskPollIntervalMs: number;
}

export interface SettingsResponse {
  success: boolean;
  settings: PublicSettings;
}

export interface SettingsUpdatePayload {
  baseDir: string;
  videoOutputDir: string;
  showHiddenItems: boolean;
  logDir: string;
  logLevel: string;
  logRotationHours: number;
  logMaxAgeDays: number;
  taskPollIntervalMs: number;
  currentPassword: string;
  newPassword: string;
}
