import type { EChartsOption } from "echarts";

import type {
  ComponentBreakdownRow,
  GcCauseBreakdownRow,
  GcHeapPoint,
  GcPauseHistogramBucket,
  GcPauseTimelinePoint,
  GcTypeBreakdownRow,
  StatusCodeDistributionRow,
  TimeValuePoint,
} from "../api/analyzerClient";

const LARGE_LINE_THRESHOLD = 2_000;
const LARGE_BAR_THRESHOLD = 1_000;
const PROGRESSIVE_CHUNK_SIZE = 800;
const PROGRESSIVE_THRESHOLD = 2_000;

export type ChartLabels = {
  requestsAxis: string;
  millisecondsAxis: string;
  statusSeries: string;
  samplesAxis: string;
  p95Series: string;
};

export type GcChartLabels = {
  pauseMs: string;
  heapMb: string;
  gcType: string;
  heapBefore: string;
  heapAfter: string;
  youngAfter: string;
  cause: string;
  events: string;
};

const GC_TYPE_COLORS: Record<string, string> = {
  "G1 Young": "#52c41a",
  "G1 Mixed": "#fa8c16",
  "G1 Remark": "#1890ff",
  "G1 Cleanup": "#13c2c2",
  "G1 Young (initial-mark)": "#a0d911",
  "G1 Young (to-space exhausted)": "#ff4d4f",
  "G1 Young (to-space overflow)": "#ff7a45",
  "Pause Young": "#52c41a",
  "Pause Mixed": "#fa8c16",
  "Pause Full": "#f5222d",
  "Pause Initial Mark": "#a0d911",
  "Full GC": "#f5222d",
  "Full GC (Parallel)": "#cf1322",
  "Full GC (CMS)": "#d4380d",
  "Young GC": "#52c41a",
  "Young GC (Serial)": "#73d13d",
  "Young GC (Parallel)": "#95de64",
  "Young GC (CMS)": "#b7eb8f",
  "G1 Partial": "#ffc53d",
  UNKNOWN: "#8c8c8c",
};

function gcTypeColor(gcType: string): string {
  return GC_TYPE_COLORS[gcType] ?? GC_TYPE_COLORS["UNKNOWN"];
}

export type GcPauseTimelineChartData = {
  series: {
    pause_timeline: GcPauseTimelinePoint[];
  };
};

export type GcHeapChartData = {
  series: {
    heap_after_mb: GcHeapPoint[];
  };
};

export type GcTypeBreakdownChartData = {
  series: {
    gc_type_breakdown: GcTypeBreakdownRow[];
    cause_breakdown?: GcCauseBreakdownRow[];
  };
};

export type GcCauseBreakdownChartData = {
  series: {
    cause_breakdown: GcCauseBreakdownRow[];
  };
};

export type GcHeapBeforeAfterChartData = {
  series: {
    heap_before_mb: GcHeapPoint[];
    heap_after_mb: GcHeapPoint[];
    young_after_mb?: GcHeapPoint[];
  };
};

export type GcPauseHistogramChartData = {
  series: {
    pause_histogram: GcPauseHistogramBucket[];
  };
};

export type RequestCountChartData = {
  series: {
    requests_per_minute: TimeValuePoint[];
  };
};

export type P95ChartData = {
  series: {
    p95_response_time_per_minute: TimeValuePoint[];
  };
};

export type StatusDistributionChartData = {
  series: {
    status_code_distribution: StatusCodeDistributionRow[];
  };
};

export type ProfilerBreakdownChartData = {
  series: {
    component_breakdown?: ComponentBreakdownRow[];
    profiler_component_breakdown?: ComponentBreakdownRow[];
  };
};

export function requestCountTrendOption(
  data: RequestCountChartData,
  labels: ChartLabels,
): EChartsOption {
  const rows = data.series.requests_per_minute;
  const largeOptions = lineLargeDataOptions(rows.length);
  return {
    tooltip: { trigger: "axis" },
    xAxis: { type: "category", data: rows.map((row) => row.time) },
    yAxis: { type: "value", name: labels.requestsAxis },
    series: [
      {
        type: "line",
        smooth: true,
        areaStyle: {},
        name: labels.requestsAxis,
        data: rows.map((row) => row.value),
        ...largeOptions,
      },
    ],
  };
}

export function p95TrendOption(
  data: P95ChartData,
  labels: ChartLabels,
): EChartsOption {
  const rows = data.series.p95_response_time_per_minute;
  const largeOptions = lineLargeDataOptions(rows.length);
  return {
    tooltip: { trigger: "axis" },
    xAxis: { type: "category", data: rows.map((row) => row.time) },
    yAxis: { type: "value", name: labels.millisecondsAxis },
    series: [
      {
        type: "line",
        smooth: true,
        name: labels.p95Series,
        data: rows.map((row) => row.value),
        ...largeOptions,
      },
    ],
  };
}

export function statusCodeDistributionOption(
  data: StatusDistributionChartData,
  labels: ChartLabels,
): EChartsOption {
  const rows = data.series.status_code_distribution;
  return {
    tooltip: { trigger: "item" },
    legend: { bottom: 0 },
    series: [
      {
        type: "pie",
        radius: ["48%", "72%"],
        name: labels.statusSeries,
        data: rows.map((row) => ({ name: row.status, value: row.count })),
      },
    ],
  };
}

export function profilerBreakdownOption(
  data: ProfilerBreakdownChartData,
  labels: ChartLabels,
): EChartsOption {
  const rows =
    data.series.component_breakdown ?? data.series.profiler_component_breakdown ?? [];
  const largeOptions = barLargeDataOptions(rows.length);
  return {
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    grid: { left: 126, right: 24, top: 28, bottom: 36 },
    xAxis: { type: "value", name: labels.samplesAxis },
    yAxis: {
      type: "category",
      data: rows.map((row) => row.component),
    },
    series: [
      {
        type: "bar",
        name: labels.samplesAxis,
        data: rows.map((row) => row.samples),
        ...largeOptions,
      },
    ],
  };
}

export function gcPauseTimelineOption(
  data: GcPauseTimelineChartData,
  labels: GcChartLabels,
): EChartsOption {
  const rows = data.series?.pause_timeline ?? [];
  if (!rows.length) return {};

  const gcTypes = [...new Set(rows.map((r) => r.gc_type))].sort();
  const isLarge = rows.length > 600;
  const barMaxWidth = isLarge ? 2 : 6;
  const labelInterval = Math.max(0, Math.floor(rows.length / 8) - 1);

  return {
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
      formatter: (params: unknown) => {
        const items = params as { axisValue: string; seriesName: string; value: number | null }[];
        const active = items.filter((p) => p.value != null && p.value > 0);
        if (!active.length) return "";
        const idx = Number(active[0].axisValue);
        const time = rows[idx]?.time ?? "";
        const header = `<b>Event ${idx}</b><br/>${time}<br/>`;
        return header + active.map((p) => `${p.seriesName}: <b>${p.value?.toFixed(1)} ms</b>`).join("<br/>");
      },
    },
    legend: { bottom: 0, type: "scroll", data: gcTypes },
    grid: { top: 28, right: 16, bottom: 80, left: 60 },
    dataZoom: [
      { type: "inside", xAxisIndex: 0 },
      { type: "slider", xAxisIndex: 0, height: 18, bottom: 28 },
    ],
    xAxis: {
      type: "category",
      data: rows.map((_, i) => i),
      axisLabel: {
        interval: labelInterval,
        formatter: (val: string) => {
          const idx = Number(val);
          const t = rows[idx]?.time ?? "";
          const m = t.match(/T(\d{2}:\d{2})/);
          return m ? m[1] : val;
        },
      },
    },
    yAxis: { type: "value", name: labels.pauseMs, minInterval: 1 },
    series: gcTypes.map((gcType) => ({
      type: "bar",
      name: gcType,
      barMaxWidth,
      color: gcTypeColor(gcType),
      emphasis: { focus: "series" },
      data: rows.map((r) => (r.gc_type === gcType ? r.value : null)),
    })),
  };
}

export function gcHeapTrendOption(
  data: GcHeapChartData,
  labels: GcChartLabels,
): EChartsOption {
  const rows = data.series?.heap_after_mb ?? [];
  if (!rows.length) return {};

  return {
    tooltip: {
      trigger: "axis",
      valueFormatter: (val: unknown) =>
        typeof val === "number" ? `${val.toFixed(1)} MB` : "-",
    },
    grid: { top: 28, right: 16, bottom: 56, left: 68 },
    dataZoom: [
      { type: "inside", xAxisIndex: 0 },
      { type: "slider", xAxisIndex: 0, height: 18, bottom: 8 },
    ],
    xAxis: { type: "category", data: rows.map((_, i) => i), axisLabel: { show: false } },
    yAxis: {
      type: "value",
      name: labels.heapMb,
      axisLabel: { formatter: (v: number) => `${v} MB` },
    },
    series: [
      {
        type: "line",
        smooth: false,
        areaStyle: { opacity: 0.2 },
        symbol: "none",
        color: "#1890ff",
        name: labels.heapMb,
        data: rows.map((r) => r.value),
        ...lineLargeDataOptions(rows.length),
      },
    ],
  };
}

export function gcHeapBeforeAfterOption(
  data: GcHeapBeforeAfterChartData,
  labels: GcChartLabels,
): EChartsOption {
  const beforeRows = data.series?.heap_before_mb ?? [];
  const afterRows = data.series?.heap_after_mb ?? [];
  const youngRows = data.series?.young_after_mb ?? [];
  const baseRows = beforeRows.length >= afterRows.length ? beforeRows : afterRows;
  if (!baseRows.length) return {};
  const len = baseRows.length;

  const series: NonNullable<EChartsOption["series"]> = [];
  if (beforeRows.length) {
    series.push({
      type: "line",
      smooth: false,
      symbol: "none",
      color: "#fa8c16",
      name: labels.heapBefore,
      data: beforeRows.map((r) => r.value),
      ...lineLargeDataOptions(beforeRows.length),
    });
  }
  if (afterRows.length) {
    series.push({
      type: "line",
      smooth: false,
      symbol: "none",
      color: "#1890ff",
      name: labels.heapAfter,
      data: afterRows.map((r) => r.value),
      ...lineLargeDataOptions(afterRows.length),
    });
  }
  if (youngRows.length) {
    series.push({
      type: "line",
      smooth: false,
      symbol: "none",
      color: "#52c41a",
      name: labels.youngAfter,
      data: youngRows.map((r) => r.value),
      ...lineLargeDataOptions(youngRows.length),
    });
  }

  return {
    tooltip: {
      trigger: "axis",
      valueFormatter: (val: unknown) =>
        typeof val === "number" ? `${val.toFixed(1)} MB` : "-",
    },
    legend: { bottom: 0, type: "scroll" },
    grid: { top: 28, right: 16, bottom: 80, left: 68 },
    dataZoom: [
      { type: "inside", xAxisIndex: 0 },
      { type: "slider", xAxisIndex: 0, height: 18, bottom: 32 },
    ],
    xAxis: {
      type: "category",
      data: Array.from({ length: len }, (_, i) => String(i)),
      axisLabel: { show: false },
    },
    yAxis: {
      type: "value",
      name: labels.heapMb,
      axisLabel: { formatter: (v: number) => `${v} MB` },
    },
    series,
  };
}

export function gcCauseDistributionOption(
  data: GcCauseBreakdownChartData,
  labels: GcChartLabels,
): EChartsOption {
  const rows = data.series?.cause_breakdown ?? [];
  if (!rows.length) return {};

  return {
    tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
    legend: { bottom: 0, type: "scroll" },
    series: [
      {
        type: "pie",
        radius: ["45%", "70%"],
        name: labels.cause,
        label: { show: false },
        emphasis: { label: { show: true, fontWeight: "bold" } },
        data: rows.map((r) => ({ name: r.cause, value: r.count })),
      },
    ],
  };
}

export function gcPauseHistogramOption(
  data: GcPauseHistogramChartData,
  labels: GcChartLabels,
): EChartsOption {
  const rows = data.series?.pause_histogram ?? [];
  if (!rows.length) return {};

  return {
    tooltip: {
      trigger: "axis",
      axisPointer: { type: "shadow" },
      formatter: (params: unknown) => {
        const items = params as { axisValue: string; value: number }[];
        if (!items.length) return "";
        return `<b>${items[0].axisValue}</b><br/>${labels.events}: <b>${items[0].value}</b>`;
      },
    },
    grid: { top: 28, right: 16, bottom: 36, left: 60 },
    xAxis: {
      type: "category",
      data: rows.map((r) => r.bucket),
      name: labels.pauseMs,
    },
    yAxis: { type: "value", name: labels.events, minInterval: 1 },
    series: [
      {
        type: "bar",
        name: labels.events,
        data: rows.map((r) => ({
          value: r.count,
          itemStyle: { color: histogramBucketColor(r.min_ms) },
        })),
      },
    ],
  };
}

function histogramBucketColor(minMs: number): string {
  if (minMs < 10) return "#52c41a";
  if (minMs < 100) return "#faad14";
  if (minMs < 1_000) return "#fa8c16";
  return "#f5222d";
}

export function gcTypeDistributionOption(
  data: GcTypeBreakdownChartData,
  labels: GcChartLabels,
): EChartsOption {
  const rows = data.series?.gc_type_breakdown ?? [];
  if (!rows.length) return {};

  return {
    tooltip: { trigger: "item", formatter: "{b}: {c} ({d}%)" },
    legend: { bottom: 0, type: "scroll" },
    series: [
      {
        type: "pie",
        radius: ["45%", "70%"],
        name: labels.gcType,
        label: { show: false },
        emphasis: { label: { show: true, fontWeight: "bold" } },
        data: rows.map((r) => ({
          name: r.gc_type,
          value: r.count,
          itemStyle: { color: gcTypeColor(r.gc_type) },
        })),
      },
    ],
  };
}

function lineLargeDataOptions(rowCount: number): Record<string, unknown> {
  if (rowCount < LARGE_LINE_THRESHOLD) {
    return {};
  }
  return {
    large: true,
    sampling: "lttb",
    progressive: PROGRESSIVE_CHUNK_SIZE,
    progressiveThreshold: PROGRESSIVE_THRESHOLD,
  };
}

function barLargeDataOptions(rowCount: number): Record<string, unknown> {
  if (rowCount < LARGE_BAR_THRESHOLD) {
    return {};
  }
  return {
    large: true,
    progressive: PROGRESSIVE_CHUNK_SIZE,
    progressiveThreshold: PROGRESSIVE_THRESHOLD,
  };
}
