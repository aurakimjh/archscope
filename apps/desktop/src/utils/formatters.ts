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
