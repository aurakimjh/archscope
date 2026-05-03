import type { EChartsOption } from "echarts";

import type {
  AccessLogAnalysisResult,
  DashboardSampleResult,
  ProfilerCollapsedAnalysisResult,
} from "../api/analyzerClient";
import type { ChartTemplateId } from "./chartTemplates";
import {
  p95TrendOption,
  profilerBreakdownOption,
  requestCountTrendOption,
  statusCodeDistributionOption,
  type ChartLabels,
  type P95ChartData,
  type ProfilerBreakdownChartData,
  type RequestCountChartData,
  type StatusDistributionChartData,
} from "./chartOptions";

export type ChartData =
  | DashboardSampleResult
  | AccessLogAnalysisResult
  | ProfilerCollapsedAnalysisResult;

export function createChartOption(
  templateId: ChartTemplateId,
  data: ChartData,
  labels: ChartLabels,
): EChartsOption {
  switch (templateId) {
    case "AccessLog.RequestCountTrend":
      return requestCountTrendOption(data as RequestCountChartData, labels);
    case "AccessLog.ResponseTimeP95Trend":
      return p95TrendOption(data as P95ChartData, labels);
    case "AccessLog.StatusCodeDistribution":
      return statusCodeDistributionOption(data as StatusDistributionChartData, labels);
    case "Profiler.ComponentBreakdown":
      return profilerBreakdownOption(data as ProfilerBreakdownChartData, labels);
  }
}
