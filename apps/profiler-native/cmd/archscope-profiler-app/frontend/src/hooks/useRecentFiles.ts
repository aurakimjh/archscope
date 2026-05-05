import { useCallback, useEffect, useState } from "react";

const STORAGE_KEY = "archscope.profiler.recentFiles";
const MAX_ENTRIES = 5;

function read(): string[] {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((value): value is string => typeof value === "string").slice(0, MAX_ENTRIES);
  } catch {
    return [];
  }
}

function write(entries: string[]) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(entries.slice(0, MAX_ENTRIES)));
  } catch {
    // ignore
  }
}

export function useRecentFiles() {
  const [entries, setEntries] = useState<string[]>(() => read());

  // Keep state aligned across multiple windows / Settings page mutations.
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === STORAGE_KEY) setEntries(read());
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const push = useCallback((path: string) => {
    if (!path) return;
    setEntries((prev) => {
      const filtered = prev.filter((entry) => entry !== path);
      const next = [path, ...filtered].slice(0, MAX_ENTRIES);
      write(next);
      return next;
    });
  }, []);

  const clear = useCallback(() => {
    write([]);
    setEntries([]);
  }, []);

  return { entries, push, clear };
}
