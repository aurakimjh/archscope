import type { MessageKey } from "../i18n/messages";

export type ChartTemplateId =
  | "AccessLog.RequestCountTrend"
  | "AccessLog.ResponseTimeP95Trend"
  | "AccessLog.StatusCodeDistribution"
  | "Profiler.ComponentBreakdown";

export type ChartTemplate = {
  id: ChartTemplateId;
  resultType: "access_log" | "dashboard_sample" | "profiler_collapsed";
  chartKind: "line" | "bar" | "donut" | "horizontal_bar";
  titleKey: MessageKey;
  axisLabelKeys: MessageKey[];
  legendLabelKeys: MessageKey[];
  supportedRenderers: Array<"canvas" | "svg">;
  supportsDarkMode: boolean;
  exportFormats: Array<"png" | "svg" | "html">;
};

export const chartTemplates: ChartTemplate[] = [
  {
    id: "AccessLog.RequestCountTrend",
    resultType: "access_log",
    chartKind: "line",
    titleKey: "requestCountTrend",
    axisLabelKeys: ["requestsAxis"],
    legendLabelKeys: ["requestsAxis"],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "AccessLog.ResponseTimeP95Trend",
    resultType: "access_log",
    chartKind: "line",
    titleKey: "responseTimeP95Trend",
    axisLabelKeys: ["millisecondsAxis"],
    legendLabelKeys: ["p95Series"],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "AccessLog.StatusCodeDistribution",
    resultType: "access_log",
    chartKind: "donut",
    titleKey: "statusCodeDistribution",
    axisLabelKeys: [],
    legendLabelKeys: ["statusSeries"],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "Profiler.ComponentBreakdown",
    resultType: "profiler_collapsed",
    chartKind: "horizontal_bar",
    titleKey: "profilerComponentBreakdown",
    axisLabelKeys: ["samplesAxis"],
    legendLabelKeys: ["samplesAxis"],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
];

export const dashboardChartTemplateIds: ChartTemplateId[] = [
  "AccessLog.RequestCountTrend",
  "AccessLog.ResponseTimeP95Trend",
  "AccessLog.StatusCodeDistribution",
  "Profiler.ComponentBreakdown",
];

export function getChartTemplate(id: ChartTemplateId): ChartTemplate {
  const template = chartTemplates.find((candidate) => candidate.id === id);
  if (!template) {
    throw new Error(`Unknown chart template: ${id}`);
  }
  return template;
}
