import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { getStoredJSON, getStoredString, removeStoredValue, setStoredString } from './safeStorage';

function createMemoryStorage(): Storage {
  const store = new Map<string, string>();
  return {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key: string) {
      return store.has(key) ? store.get(key)! : null;
    },
    key(index: number) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key: string) {
      store.delete(key);
    },
    setItem(key: string, value: string) {
      store.set(key, value);
    },
  };
}

function installWindowWithStorage(storage: Storage) {
  Object.defineProperty(globalThis, 'window', {
    value: { localStorage: storage },
    configurable: true,
    writable: true,
  });
}

describe('safeStorage', () => {
  const originalWindow = (globalThis as { window?: unknown }).window;

  beforeEach(() => {
    installWindowWithStorage(createMemoryStorage());
  });

  afterEach(() => {
    Object.defineProperty(globalThis, 'window', {
      value: originalWindow,
      configurable: true,
      writable: true,
    });
  });

  it('reads and writes string values with working localStorage', () => {
    expect(getStoredString('token', 'fallback')).toBe('fallback');

    setStoredString('token', 'abc');
    expect(getStoredString('token', '')).toBe('abc');

    removeStoredValue('token');
    expect(getStoredString('token', 'missing')).toBe('missing');
  });

  it('returns fallback when window is unavailable', () => {
    Object.defineProperty(globalThis, 'window', {
      value: undefined,
      configurable: true,
      writable: true,
    });

    expect(getStoredString('k', 'fallback')).toBe('fallback');
    expect(getStoredJSON('k', { ok: true })).toEqual({ ok: true });
    expect(() => setStoredString('k', 'v')).not.toThrow();
    expect(() => removeStoredValue('k')).not.toThrow();
  });

  it('swallows storage access errors and uses fallback values', () => {
    const throwingStorage: Storage = {
      get length() {
        return 0;
      },
      clear() {
        throw new Error('blocked');
      },
      getItem() {
        throw new Error('blocked');
      },
      key() {
        throw new Error('blocked');
      },
      removeItem() {
        throw new Error('blocked');
      },
      setItem() {
        throw new Error('blocked');
      },
    };

    installWindowWithStorage(throwingStorage);

    expect(getStoredString('k', 'fallback')).toBe('fallback');
    expect(getStoredJSON('k', ['fallback'])).toEqual(['fallback']);
    expect(() => setStoredString('k', 'v')).not.toThrow();
    expect(() => removeStoredValue('k')).not.toThrow();
  });

  it('parses json payloads and falls back on invalid json', () => {
    setStoredString('json', JSON.stringify({ a: 1 }));
    expect(getStoredJSON('json', { a: 0 })).toEqual({ a: 1 });

    setStoredString('json', '{not-valid');
    expect(getStoredJSON('json', { a: 2 })).toEqual({ a: 2 });
  });
});
