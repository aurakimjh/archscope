// ─────────────────────────────────────────────────────────────────────
// [한글] chartTemplates.ts — 모든 차트 템플릿의 메타데이터 카탈로그.
//
// 책임/목적:
//   - ChartTemplateId 별로 어떤 분석 결과 타입(resultType), 어떤 차트
//     종류(chartKind: line/bar/donut/...), 어떤 i18n 라벨 키, 어떤
//     렌더러/export 포맷을 지원하는지 한 곳에 선언.
//   - DashboardPage 가 보여줄 템플릿 목록(dashboardChartTemplateIds),
//     ChartStudioPage 가 셀렉트로 노출할 전체 템플릿 목록,
//     chartFactory 의 createChartOption switch 와 항상 동기화 유지.
//
// 데이터 흐름:
//   chartTemplates → getChartTemplate(id) → 페이지가 렌더 결정에 사용
//   → createChartOption(id, data, labels) 로 EChartsOption 생성.
//
// 주의:
//   - resultType 키는 분석 결과의 type 필드 값과 byte 단위 일치해야 합니다
//     (DashboardPage 의 dashboardSupportedTypes 체크가 이 값을 사용).
//   - 새 템플릿 추가 시 chartFactory.ts 의 switch 도 함께 업데이트.
// ─────────────────────────────────────────────────────────────────────
import type { MessageKey } from "../i18n/messages";

export type ChartTemplateId =
  | "AccessLog.RequestCountTrend"
  | "AccessLog.ResponseTimeP95Trend"
  | "AccessLog.StatusCodeDistribution"
  | "GcLog.PauseTimeline"
  | "GcLog.HeapTrend"
  | "GcLog.HeapBeforeAfter"
  | "GcLog.TypeDistribution"
  | "GcLog.CauseDistribution"
  | "GcLog.PauseHistogram"
  | "GcLog.AllocationRate"
  | "Profiler.ComponentBreakdown";

export type ChartTemplate = {
  id: ChartTemplateId;
  resultType: "access_log" | "gc_log" | "dashboard_sample" | "profiler_collapsed";
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
    id: "GcLog.PauseTimeline",
    resultType: "gc_log",
    chartKind: "bar",
    titleKey: "gcPauseTimeline",
    axisLabelKeys: ["pauseMsAxis"],
    legendLabelKeys: [],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "GcLog.HeapTrend",
    resultType: "gc_log",
    chartKind: "line",
    titleKey: "heapUsageTrend",
    axisLabelKeys: ["heapMbAxis"],
    legendLabelKeys: [],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "GcLog.HeapBeforeAfter",
    resultType: "gc_log",
    chartKind: "line",
    titleKey: "heapBeforeAfter",
    axisLabelKeys: ["heapMbAxis"],
    legendLabelKeys: [],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "GcLog.TypeDistribution",
    resultType: "gc_log",
    chartKind: "donut",
    titleKey: "gcTypeDistribution",
    axisLabelKeys: [],
    legendLabelKeys: [],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "GcLog.CauseDistribution",
    resultType: "gc_log",
    chartKind: "donut",
    titleKey: "gcCauseDistribution",
    axisLabelKeys: [],
    legendLabelKeys: [],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "GcLog.PauseHistogram",
    resultType: "gc_log",
    chartKind: "bar",
    titleKey: "gcPauseHistogram",
    axisLabelKeys: ["pauseMsAxis"],
    legendLabelKeys: [],
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
  },
  {
    id: "GcLog.AllocationRate",
    resultType: "gc_log",
    chartKind: "line",
    titleKey: "gcAllocationRate",
    axisLabelKeys: ["mbPerSecAxis"],
    legendLabelKeys: [],
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
