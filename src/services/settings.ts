import { api } from './api';
import type { SettingsResponse, SettingsUpdatePayload } from '../types';

export async function fetchSettings() {
  const response = await api.get<SettingsResponse>('/api/settings');
  return response.data.settings;
}

export async function updateSettings(payload: SettingsUpdatePayload) {
  const response = await api.put<SettingsResponse>('/api/settings', payload);
  return response.data.settings;
}
