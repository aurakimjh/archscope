// ─────────────────────────────────────────────────────────────────────
// [한글] apiBase.ts — 엔진 REST API 기본 URL 결정 헬퍼.
//
// 책임/목적:
//   - 브라우저(웹) 모드와 Electron 데스크톱 모드의 URL 차이를 흡수.
//   - 모든 fetch/IPC 호출에서 동일한 함수로 base URL 을 얻을 수 있도록.
//
// 동작 규칙:
//   - 웹 모드(Vite/FastAPI same-origin): `/api` 만 반환 → 동일 origin
//     으로 fetch.
//   - Electron 모드: preload 가 `window.archscope.engineUrl` 에
//     `http://127.0.0.1:<port>` 형태로 주입. 페이지는 `file://` 로
//     로드되므로 same-origin 으로는 절대 도달 불가 → 절대 URL 필요.
//   - SSR/노드 환경(typeof window === "undefined") 은 `/api` 로 fallback.
// ─────────────────────────────────────────────────────────────────────
/**
 * Resolve the engine API base URL.
 *
 * - Browser / web mode: `/api` (same-origin; engine serves the static frontend).
 * - Electron mode: the absolute URL injected by the preload script, since
 *   the page is loaded via `file://` and has no relatable origin.
 */
// [한글] getApiBase — 호출 컨텍스트(브라우저/Electron/SSR)에 맞는
//   엔진 API base URL 을 문자열로 반환. trailing slash 는 제거.
export function getApiBase(): string {
  if (typeof window === "undefined") return "/api";
  const electronBase = (window as { archscope?: { engineUrl?: string } }).archscope?.engineUrl;
  if (electronBase) return `${electronBase.replace(/\/$/, "")}/api`;
  return "/api";
}
