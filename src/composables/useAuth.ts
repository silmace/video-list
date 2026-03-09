import { computed, ref } from 'vue';
import { api, setAuthToken, getAuthToken } from '../services/api';

interface AuthStatusResponse {
  authEnabled: boolean;
  authenticated: boolean;
  taskPollIntervalMs: number;
}

interface LoginResponse {
  success: boolean;
  token?: string;
  passwordNeeded?: boolean;
}

const authEnabled = ref(false);
const authenticated = ref(false);
const authLoading = ref(false);
const taskPollIntervalMs = ref(1500);

const hasToken = computed(() => !!getAuthToken());

export async function checkAuthStatus(): Promise<AuthStatusResponse> {
  authLoading.value = true;
  try {
    const response = await api.get<AuthStatusResponse>('/api/auth/status');
    authEnabled.value = response.data.authEnabled;
    authenticated.value = response.data.authenticated;
    taskPollIntervalMs.value = response.data.taskPollIntervalMs || 1500;

    if (authEnabled.value && !authenticated.value && hasToken.value) {
      setAuthToken('');
    }

    return response.data;
  } finally {
    authLoading.value = false;
  }
}

export async function login(password: string): Promise<void> {
  const response = await api.post<LoginResponse>('/api/auth/login', { password });
  const token = response.data.token || '';
  setAuthToken(token);
  await checkAuthStatus();
}

export async function logout(): Promise<void> {
  try {
    await api.post('/api/auth/logout');
  } catch {
    // ignore network errors on logout and always clear local session token
  }
  setAuthToken('');
  authenticated.value = false;
}

export const authState = {
  authEnabled,
  authenticated,
  authLoading,
  taskPollIntervalMs,
};
