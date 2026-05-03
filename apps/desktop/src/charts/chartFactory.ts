import type { EChartsOption } from "echarts";

import type {
  AccessLogAnalysisResult,
  DashboardSampleResult,
  GcLogAnalysisResult,
  ProfilerCollapsedAnalysisResult,
} from "../api/analyzerClient";
import type { ChartTemplateId } from "./chartTemplates";
import {
  gcHeapTrendOption,
  gcPauseTimelineOption,
  gcTypeDistributionOption,
  p95TrendOption,
  profilerBreakdownOption,
  requestCountTrendOption,
  statusCodeDistributionOption,
  type ChartLabels,
  type GcChartLabels,
  type GcHeapChartData,
  type GcPauseTimelineChartData,
  type GcTypeBreakdownChartData,
  type P95ChartData,
  type ProfilerBreakdownChartData,
  type RequestCountChartData,
  type StatusDistributionChartData,
} from "./chartOptions";

export type ChartData =
  | DashboardSampleResult
  | AccessLogAnalysisResult
  | GcLogAnalysisResult
  | ProfilerCollapsedAnalysisResult;

export function createChartOption(
  templateId: ChartTemplateId,
  data: ChartData,
  labels: ChartLabels | GcChartLabels,
): EChartsOption {
  switch (templateId) {
    case "AccessLog.RequestCountTrend":
      return requestCountTrendOption(data as RequestCountChartData, labels as ChartLabels);
    case "AccessLog.ResponseTimeP95Trend":
      return p95TrendOption(data as P95ChartData, labels as ChartLabels);
    case "AccessLog.StatusCodeDistribution":
      return statusCodeDistributionOption(data as StatusDistributionChartData, labels as ChartLabels);
    case "GcLog.PauseTimeline":
      return gcPauseTimelineOption(data as GcPauseTimelineChartData, labels as GcChartLabels);
    case "GcLog.HeapTrend":
      return gcHeapTrendOption(data as GcHeapChartData, labels as GcChartLabels);
    case "GcLog.TypeDistribution":
      return gcTypeDistributionOption(data as GcTypeBreakdownChartData, labels as GcChartLabels);
    case "Profiler.ComponentBreakdown":
      return profilerBreakdownOption(data as ProfilerBreakdownChartData, labels as ChartLabels);
  }
}
