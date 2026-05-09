// ─────────────────────────────────────────────────────────────────────
// [한글] utils/formatters.ts — 메트릭 카드/테이블에서 공용으로 사용하는
//   숫자 포매터. null/undefined 는 "-" 로 표기해 빈 셀을 시각적으로 통일.
//   현재 로케일은 toLocaleString 의 기본값(브라우저 로케일) 을 따르며,
//   필요 시 i18n 의 locale 을 인자로 받도록 확장할 수 있습니다.
// ─────────────────────────────────────────────────────────────────────
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
