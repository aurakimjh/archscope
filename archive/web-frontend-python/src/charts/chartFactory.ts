// ─────────────────────────────────────────────────────────────────────
// [한글] chartFactory.ts — ChartTemplateId 와 분석 결과(data) 를 받아
//   ECharts 옵션 객체를 만들어 주는 팩토리 함수.
//
// 책임/목적:
//   - chartTemplates 에 등록된 차트마다 어떤 chartOptions 빌더(예:
//     gcPauseTimelineOption, statusCodeDistributionOption) 를 호출할지
//     단일 switch 로 매핑.
//   - 컴포넌트는 createChartOption(templateId, data, labels) 한 줄만 호출
//     하면 되므로 페이지/대시보드 코드에서 ECharts 의존을 격리.
//
// 데이터 흐름:
//   page → templateId + AnalysisResult specialization → createChartOption
//   → EChartsOption → ChartPanel 에 전달.
//
// 주의:
//   - 새 차트 템플릿을 추가할 때:
//     1) chartTemplates.ts 에 ChartTemplateId 추가.
//     2) chartOptions.ts 에 빌더 함수 추가.
//     3) 본 파일 switch 에 case 추가.
//   - data 의 타입 단언(as) 은 runtime guarantee 가 아니므로,
//     호출 측에서 templateId 와 data 의 결과 타입이 일치하도록 보장해야
//     합니다(보통 페이지가 자기 도메인 결과만 넘기므로 자연히 충족).
// ─────────────────────────────────────────────────────────────────────
import type { EChartsOption } from "echarts";

import type {
  AccessLogAnalysisResult,
  DashboardSampleResult,
  GcLogAnalysisResult,
  ProfilerCollapsedAnalysisResult,
} from "../api/analyzerClient";
import type { ChartTemplateId } from "./chartTemplates";
import {
  gcAllocationRateOption,
  gcCauseDistributionOption,
  gcHeapBeforeAfterOption,
  gcHeapTrendOption,
  gcPauseHistogramOption,
  gcPauseTimelineOption,
  gcTypeDistributionOption,
  p95TrendOption,
  profilerBreakdownOption,
  requestCountTrendOption,
  statusCodeDistributionOption,
  type ChartLabels,
  type GcAllocationRateChartData,
  type GcCauseBreakdownChartData,
  type GcChartLabels,
  type GcHeapBeforeAfterChartData,
  type GcHeapChartData,
  type GcPauseHistogramChartData,
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
    case "GcLog.HeapBeforeAfter":
      return gcHeapBeforeAfterOption(data as GcHeapBeforeAfterChartData, labels as GcChartLabels);
    case "GcLog.TypeDistribution":
      return gcTypeDistributionOption(data as GcTypeBreakdownChartData, labels as GcChartLabels);
    case "GcLog.CauseDistribution":
      return gcCauseDistributionOption(data as GcCauseBreakdownChartData, labels as GcChartLabels);
    case "GcLog.PauseHistogram":
      return gcPauseHistogramOption(data as GcPauseHistogramChartData, labels as GcChartLabels);
    case "GcLog.AllocationRate":
      return gcAllocationRateOption(data as GcAllocationRateChartData, labels as GcChartLabels);
    case "Profiler.ComponentBreakdown":
      return profilerBreakdownOption(data as ProfilerBreakdownChartData, labels as ChartLabels);
  }
}
