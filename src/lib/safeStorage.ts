function getStorage(): Storage | null {
  try {
    if (typeof window === 'undefined' || !window.localStorage) {
      return null;
    }
    return window.localStorage;
  } catch {
    return null;
  }
}

export function getStoredString(key: string, fallback = ''): string {
  const storage = getStorage();
  if (!storage) {
    return fallback;
  }
  try {
    return storage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

export function setStoredString(key: string, value: string): void {
  const storage = getStorage();
  if (!storage) {
    return;
  }
  try {
    storage.setItem(key, value);
  } catch {
    // Ignore quota/security/storage availability failures.
  }
}

export function removeStoredValue(key: string): void {
  const storage = getStorage();
  if (!storage) {
    return;
  }
  try {
    storage.removeItem(key);
  } catch {
    // Ignore security/storage availability failures.
  }
}

export function getStoredJSON<T>(key: string, fallback: T): T {
  const raw = getStoredString(key, '');
  if (!raw) {
    return fallback;
  }
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}