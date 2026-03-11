import { api, setAuthToken } from './api';

export interface AuthStatusResponse {
  authEnabled: boolean;
  authenticated: boolean;
  taskPollIntervalMs: number;
}

interface LoginResponse {
  success: boolean;
  token?: string;
}

export async function fetchAuthStatus(): Promise<AuthStatusResponse> {
  const response = await api.get<AuthStatusResponse>('/api/auth/status');
  return response.data;
}

export async function loginWithPassword(password: string): Promise<void> {
  const response = await api.post<LoginResponse>('/api/auth/login', { password });
  setAuthToken(response.data.token || '');
}

export async function performLogout(): Promise<void> {
  try {
    await api.post('/api/auth/logout');
  } catch {
    // Ignore network errors and always clear local token.
  }
  setAuthToken('');
}
