import axios from 'axios';
import { getStoredString, removeStoredValue, setStoredString } from '@/lib/safeStorage';

export const AUTH_TOKEN_KEY = 'video_list_auth_token';

export const api = axios.create({
  baseURL: '/',
});

export const getAuthToken = (): string => getStoredString(AUTH_TOKEN_KEY, '');

export const setAuthToken = (token: string): void => {
  if (token) {
    setStoredString(AUTH_TOKEN_KEY, token);
  } else {
    removeStoredValue(AUTH_TOKEN_KEY);
  }
};

export const buildMediaUrl = (path: string): string => {
  const params = new URLSearchParams({ path });
  const token = getAuthToken();
  if (token) {
    params.set('token', token);
  }
  return `/api/media?${params.toString()}`;
};

api.interceptors.request.use((config) => {
  const token = getAuthToken();
  if (token) {
    config.headers = config.headers || {};
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});
