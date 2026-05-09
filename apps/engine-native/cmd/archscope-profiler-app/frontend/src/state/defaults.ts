// ─────────────────────────────────────────────────────────────────────
// [한글] state/defaults.ts — 프로파일러 기본값 영속화.
//
// 책임/목적: intervalMs (샘플 간격), topN (상위 N행), profileKind
// (wall/cpu/lock) 3 개 기본값을 localStorage 에 저장/로드. 잘못된
// 타입이나 비정상 값은 FALLBACK_DEFAULTS 로 교체해 페이지가 항상
// 안정된 초기 상태로 시작하도록 합니다.
// ProfilerAnalyzerPage 와 SettingsPage 가 공유.
// ─────────────────────────────────────────────────────────────────────
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
