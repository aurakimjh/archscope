import type { EChartsOption } from "echarts";

import type { TraceImportAnalysisResult } from "@/bridge/types";

export type TraceImportChartLabels = {
  dependencyTitle: string;
  serviceTitle: string;
  durationAxis: string;
  countAxis: string;
};

const grid = { left: 120, right: 28, top: 24, bottom: 42 };

export function traceDependencyOption(
  result: TraceImportAnalysisResult | null,
  labels: TraceImportChartLabels,
): EChartsOption {
  const rows = [...(result?.tables.service_dependencies ?? [])]
    .sort((a, b) => b.total_duration_ms - a.total_duration_ms)
    .slice(0, 20);
  return {
    grid,
    tooltip: { trigger: "axis" },
    xAxis: {
      type: "value",
      name: labels.durationAxis,
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "category",
      inverse: true,
      data: rows.map((row) => `${row.caller} -> ${row.callee}`),
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "bar",
        name: labels.dependencyTitle,
        data: rows.map((row) => row.total_duration_ms),
        itemStyle: { color: "#2563eb" },
      },
    ],
  };
}

export function traceServiceOption(
  result: TraceImportAnalysisResult | null,
  labels: TraceImportChartLabels,
): EChartsOption {
  const rows = [...(result?.tables.service_summary ?? [])]
    .sort((a, b) => b.total_duration_ms - a.total_duration_ms)
    .slice(0, 20);
  return {
    grid,
    tooltip: { trigger: "axis" },
    xAxis: {
      type: "value",
      name: labels.durationAxis,
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "category",
      inverse: true,
      data: rows.map((row) => row.service),
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "bar",
        name: labels.serviceTitle,
        data: rows.map((row) => row.total_duration_ms),
        itemStyle: { color: "#0f766e" },
      },
    ],
  };
}
