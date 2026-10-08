import { useSyncExternalStore } from "react";

/**
 * A tiny localStorage-backed store. Reads fall back to `initial` on the server,
 * in private windows, or when stored data is unreadable.
 */
export function createLocalStore<T>(key: string, initial: T, merge: (stored: unknown) => T = (s) => s as T) {
  let value: T | undefined;
  const listeners = new Set<() => void>();

  const read = (): T => {
    if (value !== undefined) return value;
    value = initial;
    try {
      const raw = window.localStorage.getItem(key);
      if (raw) value = merge(JSON.parse(raw));
    } catch {
      // ignore: storage blocked or corrupt
    }
    return value;
  };

  const set = (next: T | ((prev: T) => T)) => {
    value = typeof next === "function" ? (next as (prev: T) => T)(read()) : next;
    try {
      window.localStorage.setItem(key, JSON.stringify(value));
    } catch {
      // ignore: storage full or blocked
    }
    listeners.forEach((l) => l());
  };

  const subscribe = (listener: () => void) => {
    listeners.add(listener);
    return () => listeners.delete(listener);
  };

  const useValue = () => useSyncExternalStore(subscribe, read, () => initial);

  return { get: read, set, subscribe, useValue };
}
