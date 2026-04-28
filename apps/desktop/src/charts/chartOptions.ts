import type { EChartsOption } from "echarts";

import type { SampleAnalysisResult } from "../api/analyzerClient";

type ChartLabels = {
  requestsAxis: string;
  millisecondsAxis: string;
  statusSeries: string;
  samplesAxis: string;
};

export function requestCountTrendOption(
  data: SampleAnalysisResult,
  labels: ChartLabels,
): EChartsOption {
  const rows = data.series.requests_per_minute;
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
      },
    ],
  };
}

export function p95TrendOption(
  data: SampleAnalysisResult,
  labels: ChartLabels,
): EChartsOption {
  const rows = data.series.p95_response_time_per_minute;
  return {
    tooltip: { trigger: "axis" },
    xAxis: { type: "category", data: rows.map((row) => row.time) },
    yAxis: { type: "value", name: labels.millisecondsAxis },
    series: [
      {
        type: "line",
        smooth: true,
        name: "p95",
        data: rows.map((row) => row.value),
      },
    ],
  };
}

export function statusCodeDistributionOption(
  data: SampleAnalysisResult,
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
  data: SampleAnalysisResult,
  labels: ChartLabels,
): EChartsOption {
  const rows = data.series.profiler_component_breakdown;
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
      },
    ],
  };
}
