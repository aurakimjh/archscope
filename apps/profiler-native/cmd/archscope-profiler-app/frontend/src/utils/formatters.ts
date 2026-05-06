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
