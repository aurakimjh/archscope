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
  youngAfter: string;
  rateAxis: string;
  allocSeries: string;
  promSeries: string;
  countAxis: string;
};

const baseGrid = { left: 60, right: 24, top: 32, bottom: 36 };

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

export function heapUsageOption(
  result: GcLogAnalysisResult | null,
  labels: GcChartLabels,
): EChartsOption {
  const before = result?.series.heap_before_mb ?? [];
  const after = result?.series.heap_after_mb ?? [];
  const committed = result?.series.heap_committed_mb ?? [];
  const youngAfter = result?.series.young_after_mb ?? [];

  const series: NonNullable<EChartsOption["series"]> = [];
  if (committed.length > 0) {
    series.push({
      type: "line",
      name: labels.heapCommitted,
      data: committed.map((p) => [p.time, p.value] as [string, number]),
      smooth: false,
      showSymbol: showSymbols(committed),
      lineStyle: { color: "#64748b", width: 1.5, type: "dashed" },
      itemStyle: { color: "#64748b" },
    });
  }
  if (before.length > 0) {
    series.push({
      type: "line",
      name: labels.heapBefore,
      data: before.map((p) => [p.time, p.value] as [string, number]),
      smooth: false,
      showSymbol: showSymbols(before),
      lineStyle: { color: "#0ea5e9", width: 1.5 },
      itemStyle: { color: "#0ea5e9" },
      areaStyle: { color: "rgba(14,165,233,0.10)" },
    });
  }
  if (after.length > 0) {
    series.push({
      type: "line",
      name: labels.heapAfter,
      data: after.map((p) => [p.time, p.value] as [string, number]),
      smooth: false,
      showSymbol: showSymbols(after),
      lineStyle: { color: "#14b8a6", width: 1.5 },
      itemStyle: { color: "#14b8a6" },
    });
  }
  if (youngAfter.length > 0) {
    series.push({
      type: "line",
      name: labels.youngAfter,
      data: youngAfter.map((p) => [p.time, p.value] as [string, number]),
      smooth: false,
      showSymbol: showSymbols(youngAfter),
      lineStyle: { color: "#84cc16", width: 1.5 },
      itemStyle: { color: "#84cc16" },
    });
  }

  return {
    grid: baseGrid,
    tooltip: { trigger: "axis" },
    legend: { top: 0, textStyle: { fontSize: 10 } },
    xAxis: {
      type: "time",
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "value",
      name: labels.heapAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
    series,
  };
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
