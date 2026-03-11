import { computed, ref } from 'vue';
import { getAuthToken, setAuthToken } from '../services/api';
import { fetchAuthStatus, loginWithPassword, performLogout, type AuthStatusResponse } from '../services/auth';

const authEnabled = ref(false);
const authenticated = ref(false);
const authLoading = ref(false);
const taskPollIntervalMs = ref(1500);

const hasToken = computed(() => !!getAuthToken());

export async function checkAuthStatus(): Promise<AuthStatusResponse> {
  authLoading.value = true;
  try {
    const response = await fetchAuthStatus();
    authEnabled.value = response.authEnabled;
    authenticated.value = response.authenticated;
    taskPollIntervalMs.value = response.taskPollIntervalMs || 1500;

    if (authEnabled.value && !authenticated.value && hasToken.value) {
      setAuthToken('');
    }

    return response;
  } finally {
    authLoading.value = false;
  }
}

export async function login(password: string): Promise<void> {
  await loginWithPassword(password);
  await checkAuthStatus();
}

export async function logout(): Promise<void> {
  await performLogout();
  authenticated.value = false;
}

export const authState = {
  authEnabled,
  authenticated,
  authLoading,
  taskPollIntervalMs,
};
