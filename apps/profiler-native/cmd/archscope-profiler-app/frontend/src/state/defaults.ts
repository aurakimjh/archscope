// Persistent profiler defaults — used by both Profiler page and Settings page.

const STORAGE_KEY = "archscope.profiler.defaults";

export type ProfilerDefaults = {
  intervalMs: number;
  topN: number;
  profileKind: "wall" | "cpu" | "lock";
};

export const FALLBACK_DEFAULTS: ProfilerDefaults = {
  intervalMs: 100,
  topN: 20,
  profileKind: "wall",
};

export function loadDefaults(): ProfilerDefaults {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return FALLBACK_DEFAULTS;
    const parsed = JSON.parse(raw) as Partial<ProfilerDefaults>;
    return {
      intervalMs:
        typeof parsed.intervalMs === "number" && parsed.intervalMs > 0
          ? parsed.intervalMs
          : FALLBACK_DEFAULTS.intervalMs,
      topN:
        typeof parsed.topN === "number" && parsed.topN > 0
          ? parsed.topN
          : FALLBACK_DEFAULTS.topN,
      profileKind:
        parsed.profileKind === "wall" ||
        parsed.profileKind === "cpu" ||
        parsed.profileKind === "lock"
          ? parsed.profileKind
          : FALLBACK_DEFAULTS.profileKind,
    };
  } catch {
    return FALLBACK_DEFAULTS;
  }
}

export function saveDefaults(defaults: ProfilerDefaults) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(defaults));
  } catch {
    // ignore
  }
}
