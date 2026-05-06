import type { EChartsOption } from "echarts";

import type { AccessLogAnalysisResult } from "@/bridge/types";

// Slim subset of apps/frontend/src/charts/chartOptions.ts. Phase 1 only
// needs the request-count timeline chart shown on the AccessLog page;
// later phases can extend this with the p95 timeline, status-mix
// stack chart, etc. The web chartFactory's full template registry is
// deferred until more pages land.

export type ChartLabels = {
  requestsAxis: string;
  millisecondsAxis: string;
  p95Series: string;
};

const baseGrid = { left: 56, right: 24, top: 32, bottom: 36 };

export function requestCountTrendOption(
  result: AccessLogAnalysisResult | null,
  labels: ChartLabels,
): EChartsOption {
  const rows = result?.series.requests_per_minute ?? [];
  return {
    grid: baseGrid,
    tooltip: { trigger: "axis" },
    xAxis: {
      type: "category",
      data: rows.map((row) => row.time),
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "value",
      name: labels.requestsAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "line",
        smooth: true,
        areaStyle: { opacity: 0.25 },
        name: labels.requestsAxis,
        data: rows.map((row) => row.value),
        showSymbol: rows.length < 60,
      },
    ],
  };
}

export function p95TrendOption(
  result: AccessLogAnalysisResult | null,
  labels: ChartLabels,
): EChartsOption {
  const rows = result?.series.p95_response_time_per_minute ?? [];
  return {
    grid: baseGrid,
    tooltip: { trigger: "axis" },
    xAxis: {
      type: "category",
      data: rows.map((row) => row.time),
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: "value",
      name: labels.millisecondsAxis,
      nameTextStyle: { fontSize: 10 },
      axisLabel: { fontSize: 10 },
    },
    series: [
      {
        type: "line",
        smooth: true,
        name: labels.p95Series,
        data: rows.map((row) => row.value),
        showSymbol: rows.length < 60,
      },
    ],
  };
}
