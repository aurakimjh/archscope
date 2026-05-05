/**
 * Resolve the engine API base URL.
 *
 * - Browser / web mode: `/api` (same-origin; engine serves the static frontend).
 * - Electron mode: the absolute URL injected by the preload script, since
 *   the page is loaded via `file://` and has no relatable origin.
 */
export function getApiBase(): string {
  if (typeof window === "undefined") return "/api";
  const electronBase = (window as { archscope?: { engineUrl?: string } }).archscope?.engineUrl;
  if (electronBase) return `${electronBase.replace(/\/$/, "")}/api`;
  return "/api";
}
