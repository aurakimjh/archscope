// ─────────────────────────────────────────────────────────────────────
// [한글] utils/formatters.ts — 숫자/시간/퍼센트 포매터.
//
// 책임/목적: number | null | undefined 값을 안전하게 사람이 읽는
// 문자열로 변환. null/undefined 는 "-" 로 표시해 비어있는 메트릭
// 카드도 깨지지 않도록 함. web 셸과 동일 어휘 사용으로 화면 일관성.
// ─────────────────────────────────────────────────────────────────────
// Lifted from apps/frontend/src/utils/formatters.ts so the analyzer pages
// share their numeric-formatting vocabulary across the web and Wails apps.

export function formatNumber(value: number | null | undefined): string {
  return typeof value === "number" ? value.toLocaleString() : "-";
}

export function formatMilliseconds(value: number | null | undefined): string {
  return typeof value === "number" ? `${value.toLocaleString()} ms` : "-";
}

export function formatSeconds(value: number | null | undefined): string {
  return typeof value === "number" ? `${value.toLocaleString()} s` : "-";
}

export function formatPercent(value: number | null | undefined): string {
  return typeof value === "number" ? `${value.toLocaleString()}%` : "-";
}
