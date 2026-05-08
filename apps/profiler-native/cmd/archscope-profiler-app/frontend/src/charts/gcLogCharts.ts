import type { EChartsOption } from "echarts";

import type { GcLogAnalysisResult } from "@/bridge/types";

// EChartsOption builders for the GC log analyzer page.
//
// The web app (apps/frontend/src/components/charts/D3*.tsx) renders the
// GC log charts on top of D3, with brushing, fullscreen and per-chart
// PNG export. Phase 2 reuses the AccessLog ChartPanel — a thin echarts
// host card — so we trade the brushing for a smaller bundle. The web
// niceties can return alongside the Save-All-Charts batch exporter
// in a later phase.

export type GcChartLabels = {
  pauseAxis: string;
  pauseSeries: string;
  fullGcMarker: string;
  heapAxis: string;
  heapBefore: string;
  heapAfter: string;
  heapCommitted: string;
  heapYoungBefore: string;
  heapYoungAfter: string;
  heapOldBefore: string;
  heapOldAfter: string;
  heapMetaspaceBefore: string;
  heapMetaspaceAfter: string;
  heapPauseOverlay: string;
  rateAxis: string;
  allocSeries: string;
  promSeries: string;
  countAxis: string;
};

// HeapSeriesId mirrors the web GC page's series-toggle vocabulary
// (apps/frontend/src/pages/GcLogAnalyzerPage.tsx). Each ID maps to a
// GcLogAnalysisResult.series field; the page persists the user's
// selection so a re-analysis keeps the same view.
export type HeapSeriesId =
  | "heap_committed"
  | "heap_before"
  | "heap_after"
  | "young_before"
  | "young_after"
  | "old_before"
  | "old_after"
  | "metaspace_before"
  | "metaspace_after";

export type HeapSeriesDef = {
  id: HeapSeriesId;
  field:
    | "heap_committed_mb"
    | "heap_before_mb"
    | "heap_after_mb"
    | "young_before_mb"
    | "young_after_mb"
    | "old_before_mb"
    | "old_after_mb"
    | "metaspace_before_mb"
    | "metaspace_after_mb";
  label: string;
  color: string;
  area?: boolean;
  dashed?: boolean;
};

// HEAP_SERIES_DEFS_BUILDER constructs the series catalogue from the
// page's i18n labels. The colours mirror the web page so cross-window
// users see the same palette.
export function heapSeriesDefs(labels: GcChartLabels): HeapSeriesDef[] {
  return [
    { id: "heap_committed", field: "heap_committed_mb", label: labels.heapCommitted, color: "#64748b", dashed: true },
    { id: "heap_before", field: "heap_before_mb", label: labels.heapBefore, color: "#0ea5e9", area: true },
    { id: "heap_after", field: "heap_after_mb", label: labels.heapAfter, color: "#14b8a6" },
    { id: "young_before", field: "young_before_mb", label: labels.heapYoungBefore, color: "#22c55e" },
    { id: "young_after", field: "young_after_mb", label: labels.heapYoungAfter, color: "#84cc16" },
    { id: "old_before", field: "old_before_mb", label: labels.heapOldBefore, color: "#f59e0b" },
    { id: "old_after", field: "old_after_mb", label: labels.heapOldAfter, color: "#a855f7" },
    { id: "metaspace_before", field: "metaspace_before_mb", label: labels.heapMetaspaceBefore, color: "#ec4899" },
    { id: "metaspace_after", field: "metaspace_after_mb", label: labels.heapMetaspaceAfter, color: "#d946ef" },
  ];
}

const baseGrid = { left: 60, right: 24, top: 32, bottom: 56 };

// Standard zoom configuration: a draggable slider below the x-axis +
// inside-zoom that lets the wheel/trackpad zoom the time axis. The
// slider keeps the zoomed-out preview visible so users don't lose
// context. Both controls share the same data range so they stay in
// sync. When the chart owns a right axis (heap + pause overlay), the
// slider is bound to the time axis only (xAxisIndex defaults to 0).
const baseZoom = [
  {
    type: "slider" as const,
    height: 18,
    bottom: 6,
    borderColor: "rgba(148,163,184,0.3)",
    handleSize: "80%",
  },
  {
    type: "inside" as const,
    zoomOnMouseWheel: true,
    moveOnMouseWheel: true,
    moveOnMouseMove: true,
  },
];

function showSymbols(rows: { length: number }): boolean {
  return rows.length <= 200;
}

export function pauseTimelineOption(
  result: GcLogAnalysisResult | null,
  labels: GcChartLabels,
): EChartsOption {
  const rows = result?.series.pause_timeline ?? [];
  const events = result?.tables.events ?? [];
  // Find Full GC events so they can be highlighted as markPoints.
  const fullGcMarks = events
    .filter(
      (row) =>
        row.gc_type != null && /full/i.test(row.gc_type) && row.timestamp,
    )
    .map((row) => ({
      name: row.gc_type ?? "Full GC",
      coord: [row.timestamp as string, row.pause_ms ?? 0] as [string, number],
      value: row.gc_type ?? "Full GC",
      itemStyle: { color: "#f97316" },
      symbol: "pin",
      symbolSize: 30,
    }));

  return {
    grid: baseGrid,
    tooltip: { trigger: "axis" },
    dataZoom: baseZoom,
    xAxis: {
      type: "time",
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "value",
      name: labels.pauseAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "line",
        smooth: false,
        name: labels.pauseSeries,
        data: rows.map((row) => [row.time, row.value] as [string, number]),
        showSymbol: showSymbols(rows),
        symbolSize: 4,
        lineStyle: { color: "#ef4444", width: 1.5 },
        itemStyle: { color: "#ef4444" },
        areaStyle: { color: "rgba(239,68,68,0.10)" },
        markPoint:
          fullGcMarks.length > 0
            ? {
                data: fullGcMarks,
                label: {
                  fontSize: 9,
                  formatter: () => labels.fullGcMarker,
                },
              }
            : undefined,
      },
    ],
  };
}

// heapSeriesAvailability returns an availability map that the renderer
// uses to disable checkboxes for series that have no data. Keeping
// this in the chart module means the page doesn't have to thread the
// list of fields through.
export function heapSeriesAvailability(
  result: GcLogAnalysisResult | null,
  labels: GcChartLabels,
): Record<HeapSeriesId, boolean> {
  const out = {} as Record<HeapSeriesId, boolean>;
  const series = (result?.series ?? {}) as Record<string, Array<{ time: string; value: number }> | undefined>;
  for (const def of heapSeriesDefs(labels)) {
    out[def.id] = (series[def.field]?.length ?? 0) > 0;
  }
  return out;
}

export function heapUsageOption(
  result: GcLogAnalysisResult | null,
  labels: GcChartLabels,
  selected: ReadonlySet<HeapSeriesId>,
  pauseOverlay: boolean = false,
): EChartsOption {
  const allSeries = (result?.series ?? {}) as Record<
    string,
    Array<{ time: string; value: number }> | undefined
  >;
  const series: NonNullable<EChartsOption["series"]> = [];
  for (const def of heapSeriesDefs(labels)) {
    if (!selected.has(def.id)) continue;
    const points = allSeries[def.field] ?? [];
    if (points.length === 0) continue;
    const item: any = {
      type: "line",
      name: def.label,
      data: points.map((p) => [p.time, p.value] as [string, number]),
      smooth: false,
      showSymbol: showSymbols(points),
      lineStyle: { color: def.color, width: 1.5, type: def.dashed ? "dashed" : undefined },
      itemStyle: { color: def.color },
    };
    if (def.area) {
      item.areaStyle = { color: hexToRgba(def.color, 0.1) };
    }
    series.push(item);
  }

  // Pause overlay rides on a second y-axis (right side) so users can
  // line up GC pauses with heap occupancy at-a-glance — the same
  // affordance the web page exposes.
  const pausePoints = result?.series.pause_timeline ?? [];
  if (pauseOverlay && pausePoints.length > 0) {
    series.push({
      type: "line",
      name: labels.heapPauseOverlay,
      data: pausePoints.map((p) => [p.time, p.value] as [string, number]),
      smooth: false,
      showSymbol: showSymbols(pausePoints),
      yAxisIndex: 1,
      lineStyle: { color: "#ef4444", width: 1, type: "dashed" },
      itemStyle: { color: "#ef4444" },
    });
  }

  const yAxes: NonNullable<EChartsOption["yAxis"]> = [
    {
      type: "value",
      name: labels.heapAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
  ];
  if (pauseOverlay) {
    (yAxes as any[]).push({
      type: "value",
      name: labels.pauseAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
      splitLine: { show: false },
    });
  }

  return {
    grid: baseGrid,
    tooltip: { trigger: "axis" },
    legend: { top: 0, textStyle: { fontSize: 10 } },
    dataZoom: baseZoom,
    xAxis: {
      type: "time",
      axisLabel: { fontSize: 10 },
    },
    yAxis: yAxes,
    series,
  };
}

function hexToRgba(hex: string, alpha: number): string {
  const parsed = hex.replace("#", "");
  const expanded = parsed.length === 3
    ? parsed.split("").map((c) => c + c).join("")
    : parsed;
  const num = parseInt(expanded, 16);
  const r = (num >> 16) & 0xff;
  const g = (num >> 8) & 0xff;
  const b = num & 0xff;
  return `rgba(${r},${g},${b},${alpha})`;
}

export function allocationRateOption(
  result: GcLogAnalysisResult | null,
  labels: GcChartLabels,
): EChartsOption {
  const alloc = result?.series.allocation_rate_mb_per_sec ?? [];
  const prom = result?.series.promotion_rate_mb_per_sec ?? [];

  return {
    grid: baseGrid,
    tooltip: { trigger: "axis" },
    legend: { top: 0, textStyle: { fontSize: 10 } },
    dataZoom: baseZoom,
    xAxis: {
      type: "time",
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "value",
      name: labels.rateAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "line",
        name: labels.allocSeries,
        data: alloc.map((p) => [p.time, p.value] as [string, number]),
        smooth: true,
        showSymbol: showSymbols(alloc),
        lineStyle: { color: "#6366f1", width: 1.5 },
        itemStyle: { color: "#6366f1" },
        areaStyle: { color: "rgba(99,102,241,0.18)" },
      },
      {
        type: "line",
        name: labels.promSeries,
        data: prom.map((p) => [p.time, p.value] as [string, number]),
        smooth: true,
        showSymbol: showSymbols(prom),
        lineStyle: { color: "#a855f7", width: 1.5 },
        itemStyle: { color: "#a855f7" },
      },
    ],
  };
}

export function gcTypeBreakdownOption(
  result: GcLogAnalysisResult | null,
  labels: GcChartLabels,
): EChartsOption {
  const rows = (result?.series.gc_type_breakdown ?? []).slice().sort(
    (a, b) => b.count - a.count,
  );
  return {
    grid: { ...baseGrid, left: 120 },
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    xAxis: {
      type: "value",
      name: labels.countAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "category",
      data: rows.map((r) => r.gc_type),
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "bar",
        name: labels.countAxis,
        data: rows.map((r) => r.count),
        itemStyle: { color: "#0ea5e9" },
      },
    ],
  };
}

export function causeBreakdownOption(
  result: GcLogAnalysisResult | null,
  labels: GcChartLabels,
): EChartsOption {
  const rows = (result?.series.cause_breakdown ?? []).slice().sort(
    (a, b) => b.count - a.count,
  );
  return {
    grid: { ...baseGrid, left: 160 },
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    xAxis: {
      type: "value",
      name: labels.countAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "category",
      data: rows.map((r) => r.cause),
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "bar",
        name: labels.countAxis,
        data: rows.map((r) => r.count),
        itemStyle: { color: "#a855f7" },
      },
    ],
  };
}
