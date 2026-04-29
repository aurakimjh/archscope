import type { EChartsOption } from "echarts";

import type { DashboardSampleResult } from "../api/analyzerClient";
import type { ChartTemplateId } from "./chartTemplates";
import {
  p95TrendOption,
  profilerBreakdownOption,
  requestCountTrendOption,
  statusCodeDistributionOption,
  type ChartLabels,
} from "./chartOptions";

type ChartFactory = (
  data: DashboardSampleResult,
  labels: ChartLabels,
) => EChartsOption;

const chartFactories: Record<ChartTemplateId, ChartFactory> = {
  "AccessLog.RequestCountTrend": requestCountTrendOption,
  "AccessLog.ResponseTimeP95Trend": p95TrendOption,
  "AccessLog.StatusCodeDistribution": statusCodeDistributionOption,
  "Profiler.ComponentBreakdown": profilerBreakdownOption,
};

export function createChartOption(
  templateId: ChartTemplateId,
  data: DashboardSampleResult,
  labels: ChartLabels,
): EChartsOption {
  return chartFactories[templateId](data, labels);
}
