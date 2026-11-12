export type MetricSeriesChoice = { id: string; name: string; unit: string; kind: string; temporality: string; identity_json: string };
export const metricSeriesPageSize = 50;
// Leave room for windows, cursor and query envelope within the 64 KiB IPC query.
export const metricSelectionByteLimit = 60 * 1024;
export function metricSelectionBytes(selection: { series_ids: string[]; from_ns?: string; to_ns?: string }): number {
  return new TextEncoder().encode(JSON.stringify(selection)).byteLength;
}
export function indexMetricSeries(series: MetricSeriesChoice[]): { series: MetricSeriesChoice; text: string }[] {
  return series.map(value => ({ series: value, text: `${value.id} ${value.name} ${value.unit} ${value.kind} ${value.temporality} ${value.identity_json}`.toLowerCase() }));
}
export function filterMetricSeries(index: ReturnType<typeof indexMetricSeries>, search: string): MetricSeriesChoice[] {
  const terms = search.trim().toLowerCase().split(/\s+/).filter(Boolean);
  return index.filter(value => terms.every(term => value.text.includes(term))).map(value => value.series);
}
export function pageMetricSeries(series: MetricSeriesChoice[], requested: number): { rows: MetricSeriesChoice[]; page: number; pages: number } {
  const pages = Math.max(1, Math.ceil(series.length / metricSeriesPageSize));
  const page = Math.min(Math.max(0, Number.isFinite(requested) ? Math.floor(requested) : 0), pages - 1);
  return { rows: series.slice(page * metricSeriesPageSize, (page + 1) * metricSeriesPageSize), page, pages };
}
// Filter actions affect all matching pages and preserve selections outside it.
export function updateMetricSelection(selected: string[], matching: MetricSeriesChoice[], checked: boolean): string[] {
  const matches = new Set(matching.map(value => value.id));
  return checked ? [...new Set([...selected, ...matches])] : selected.filter(id => !matches.has(id));
}
