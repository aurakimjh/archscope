// Recent-files hook — keeps a per-category list of files the user
// has analyzed, with light metadata for the quick re-analyze panel.
//
// Storage layout: one localStorage key per category, scoped under
// `archscope.profiler.recentFiles.<category>`. The default bucket
// (no category arg) preserves the legacy single list so existing
// Settings/Profiler consumers keep working without explicit migration.
//
// Why store metadata: the user explicitly wants "history with quick
// re-analysis" — that means clicking a recent entry should jump
// straight to the analyzer with the same parameters. We persist
// `analyzer` (so the recent panel knows which page handles it),
// `analyzedAt` (for sort + display), and an opaque `meta` map so
// pages can stash analyzer-specific options (intervalMs, format,
// fallbackToTxid, etc.) without changing this hook every time.

import { useCallback, useEffect, useState } from "react";

const STORAGE_PREFIX = "archscope.profiler.recentFiles";
const MAX_ENTRIES = 8;

export type RecentFileEntry = {
  path: string;
  /** Display name (basename). Stored so the panel can render
   *  without re-splitting the path on every render. */
  name?: string;
  /** Analyzer this entry came from, e.g. "profiler", "gc",
   *  "jennifer". Surfaced in the panel as a chip. */
  analyzer?: string;
  /** ISO timestamp of when the entry was last pushed (analyzed). */
  analyzedAt?: string;
  /** Opaque per-analyzer state — pages can stash format / options
   *  here so a one-click re-analyze can replay the same parameters. */
  meta?: Record<string, unknown>;
};

function storageKey(category: string): string {
  return `${STORAGE_PREFIX}.${category}`;
}

function basename(path: string): string {
  const segments = path.split(/[\\/]/);
  return segments[segments.length - 1] || path;
}

function read(category: string): RecentFileEntry[] {
  try {
    const raw = window.localStorage.getItem(storageKey(category));
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    // Backward compat: the old format stored bare strings. We keep
    // reading them so users don't lose their list when they upgrade.
    return parsed
      .map((value): RecentFileEntry | null => {
        if (typeof value === "string") {
          return { path: value, name: basename(value) };
        }
        if (value && typeof value === "object" && typeof (value as any).path === "string") {
          const e = value as RecentFileEntry;
          return { name: basename(e.path), ...e };
        }
        return null;
      })
      .filter((entry): entry is RecentFileEntry => entry !== null)
      .slice(0, MAX_ENTRIES);
  } catch {
    return [];
  }
}

function write(category: string, entries: RecentFileEntry[]) {
  try {
    window.localStorage.setItem(
      storageKey(category),
      JSON.stringify(entries.slice(0, MAX_ENTRIES)),
    );
  } catch {
    // ignore — webview without localStorage just degrades to no-op.
  }
}

export type UseRecentFilesOptions = {
  category?: string;
};

export function useRecentFiles(options: UseRecentFilesOptions = {}) {
  const category = options.category ?? "default";
  const [entries, setEntries] = useState<RecentFileEntry[]>(() => read(category));

  // Multiple windows or the Settings page can mutate the list — pick
  // up changes through the storage event so we never show stale data.
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === storageKey(category)) setEntries(read(category));
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, [category]);

  const push = useCallback(
    (input: string | (Omit<RecentFileEntry, "name"> & { name?: string })) => {
      const entry: RecentFileEntry =
        typeof input === "string"
          ? { path: input, name: basename(input), analyzedAt: new Date().toISOString() }
          : {
              ...input,
              name: input.name ?? basename(input.path),
              analyzedAt: input.analyzedAt ?? new Date().toISOString(),
            };
      if (!entry.path) return;
      setEntries((prev) => {
        const filtered = prev.filter((e) => e.path !== entry.path);
        const next = [entry, ...filtered].slice(0, MAX_ENTRIES);
        write(category, next);
        return next;
      });
    },
    [category],
  );

  const remove = useCallback(
    (path: string) => {
      setEntries((prev) => {
        const next = prev.filter((e) => e.path !== path);
        write(category, next);
        return next;
      });
    },
    [category],
  );

  const clear = useCallback(() => {
    write(category, []);
    setEntries([]);
  }, [category]);

  return { entries, push, remove, clear };
}
