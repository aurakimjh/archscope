// ─────────────────────────────────────────────────────────────────────
// [한글] analyzerClient.ts — 분석 엔진 호출의 추상화 계층.
//
// 책임/목적:
//   - 페이지/컴포넌트가 직접 `window.archscope` 를 만지지 않고, 동일
//     인터페이스(AnalyzerClient)로 분석/취소/샘플 데이터를 부르도록.
//   - 실제 엔진이 연결되지 않은 상태(브리지 부재 / IPC 미설치) 에서도
//     UI 가 깨지지 않도록 mockAnalyzerClient 를 fallback 으로 제공.
//   - 분석 결과 타입(AnalysisResult 의 도메인별 specialization)들을
//     re-export 해 페이지에서 한 군데에서 import 할 수 있게 합니다.
//
// 데이터 흐름:
//   page → getAnalyzerClient() → window.archscope.analyzer 가 있으면
//   IPC bridge(데스크톱) 또는 httpBridge(웹) 로, 없으면 mock 응답.
//
// 타입 시스템 메모:
//   AnalysisResult<TType, TSummary, TSeries, TTables, TCharts, TMetadata>
//   는 모든 분석기가 공통으로 따르는 컨트랙트로, summary/series/tables
//   /charts/metadata 다섯 개의 키 형태가 분석기마다 specialization 됩니다.
// ─────────────────────────────────────────────────────────────────────
import { sampleAnalysisResult } from "../charts/sampleCharts";
import type {
  AccessLogAnalysisResult,
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzerExecuteRequest,
  AnalyzerExecutionResult,
  AnalyzerCancelResponse,
  AnalyzerResponse,
  ArchScopeAnalyzerBridge as AnalyzerBridge,
  ProfilerCollapsedAnalysisResult,
  AnalysisResult,
} from "./analyzerContract";

export type {
  AccessLogAnalysisResult,
  AccessLogAnalysisOptions,
  AccessLogFinding,
  AccessLogFormat,
  AccessLogMetadata,
  AccessLogSampleRecordRow,
  AccessLogSeries,
  AccessLogStatusCodeRow,
  AccessLogSummary,
  AccessLogTables,
  AccessLogUrlStatRow,
  AiFinding,
  AiInterpretationSettings,
  AiSeverity,
  AnalysisObject,
  AnalysisPrimitive,
  AnalysisResult,
  AnalysisValue,
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzeFileRequest,
  AnalyzerCancelResponse,
  AnalyzerExecuteRequest,
  AnalyzerExecutionResult,
  AnalyzerFailure,
  AnalyzerResponse,
  AnalyzerSuccess,
  ArchScopeAnalyzerBridge,
  ArchScopeRendererApi,
  BreakdownTopItem,
  BridgeError,
  ComponentBreakdownRow,
  DiagnosticSample,
  DemoListResponse,
  DemoOutputArtifact,
  DemoReferenceFile,
  DemoRunFailure,
  DemoRunRequest,
  DemoRunResponse,
  DemoRunScenarioResult,
  DemoRunSuccess,
  DemoScenarioManifest,
  DrilldownStage,
  ExceptionStackAnalysisResult,
  ExceptionStackSummary,
  ExportExecuteRequest,
  ExportFailure,
  ExportFormat,
  ExportResponse,
  ExportSuccess,
  ExecutionBreakdownRow,
  FlameNode,
  GcCauseBreakdownRow,
  GcEventRow,
  GcHeapPoint,
  GcLogAnalysisResult,
  GcLogMetadata,
  GcLogSeries,
  GcLogSummary,
  GcLogTables,
  GcPauseHistogramBucket,
  GcPauseTimelinePoint,
  GcRatePoint,
  GcTypeBreakdownRow,
  InterpretationResult,
  JfrAnalysisMode,
  JfrAnalysisResult,
  JvmAnalyzerMetadata,
  JvmFinding,
  ParserDiagnostics,
  NativeMemoryAnalysisResult,
  ProfilerCollapsedAnalysisResult,
  ProfilerCollapsedMetadata,
  ProfilerDiffAnalysisResult,
  ProfilerDiffInputFormat,
  ProfilerCollapsedSeries,
  ProfilerCollapsedSummary,
  ProfilerCollapsedTables,
  ProfilerCharts,
  ProfilerTimelineScope,
  ProfilerTopStackSeriesRow,
  ProfilerTopStackTableRow,
  SelectFileRequest,
  SelectFileResponse,
  SelectDirectoryResponse,
  StatusCodeDistributionRow,
  ThreadDumpAnalysisResult,
  ThreadDumpSummary,
  TimelineAnalysisRow,
  TimelineChainRow,
  TimeValuePoint,
  TopUrlAvgResponseRow,
  TopUrlCountRow,
} from "./analyzerContract";

export {
  isAiFinding,
  isInterpretationResult,
} from "./analyzerContract";

// [한글] DashboardSampleResult — 엔진 미연결 상태에서 대시보드가
//   "엔진이 막 돌아간 듯" 보이도록 사용하는 샘플 데이터 타입.
//   AnalysisResult 의 느슨한(unknown) 버전이라 schema validation 없이
//   쉽게 흉내낼 수 있게 했습니다.
export type DashboardSampleResult = {
  type: string;
  summary: Record<string, unknown>;
  series: Record<string, readonly unknown[]>;
  tables?: Record<string, unknown[]>;
  metadata?: Record<string, unknown>;
};
export type SampleAnalysisResult = DashboardSampleResult;

export type AnalyzerClient = {
  loadDashboardSample(): Promise<DashboardSampleResult>;
  execute(
    request: AnalyzerExecuteRequest,
  ): Promise<AnalyzerResponse<AnalyzerExecutionResult>>;
  cancel(requestId: string): Promise<AnalyzerCancelResponse>;
  analyzeAccessLog(
    request: AnalyzeAccessLogRequest,
  ): Promise<AnalyzerResponse<AccessLogAnalysisResult>>;
  analyzeCollapsedProfile(
    request: AnalyzeCollapsedProfileRequest,
  ): Promise<AnalyzerResponse<ProfilerCollapsedAnalysisResult>>;
};

// [한글] mockAnalyzerClient — 엔진 브리지가 없을 때 사용하는 fallback.
//   샘플 차트 데이터(sampleAnalysisResult)는 그대로 반환하지만,
//   실제 분석/취소 호출은 ANALYZER_NOT_CONNECTED 에러로 응답해
//   페이지가 "엔진을 연결하라" 메시지를 표시할 수 있게 합니다.
export const mockAnalyzerClient: AnalyzerClient = {
  async loadDashboardSample() {
    return sampleAnalysisResult;
  },
  async analyzeAccessLog() {
    return notImplemented<AccessLogAnalysisResult>(
      "Access log analysis is not connected to the engine yet.",
    );
  },
  async analyzeCollapsedProfile() {
    return notImplemented<ProfilerCollapsedAnalysisResult>(
      "Profiler analysis is not connected to the engine yet.",
    );
  },
  async execute() {
    return notImplemented<AnalyzerExecutionResult>(
      "Analyzer execution is not connected to the engine yet.",
    );
  },
  async cancel() {
    return {
      ok: false,
      error: {
        code: "ANALYZER_NOT_CONNECTED",
        message: "Analyzer cancellation is not connected to the engine yet.",
      },
    };
  },
};

// [한글] createIpcAnalyzerClient — preload/HTTP 브리지가 노출한
//   AnalyzerBridge 를 받아, 동일한 AnalyzerClient 표면으로 감싸는
//   어댑터. loadDashboardSample 만 mock 의 것을 그대로 사용 — 샘플은
//   순전히 클라이언트 측에서 조립되기 때문입니다.
export function createIpcAnalyzerClient(bridge: AnalyzerBridge): AnalyzerClient {
  return {
    loadDashboardSample: mockAnalyzerClient.loadDashboardSample,
    execute: bridge.execute,
    cancel: bridge.cancel,
    analyzeAccessLog: bridge.analyzeAccessLog,
    analyzeCollapsedProfile: bridge.analyzeCollapsedProfile,
  };
}

// [한글] createAnalyzerRequestId — 분석 요청을 cancel 호출과 매칭하기
//   위해 발급하는 unique ID. crypto.randomUUID 가 있으면 그걸 쓰고,
//   없으면 epoch ms + 16자리 랜덤 헥스로 fallback.
export function createAnalyzerRequestId(): string {
  return (
    globalThis.crypto?.randomUUID?.() ??
    `analysis-${Date.now()}-${Math.random().toString(16).slice(2)}`
  );
}

// [한글] getAnalyzerClient — 페이지 컴포넌트가 호출하는 메인 진입점.
//   `window.archscope.analyzer` 가 있으면 IPC 어댑터, 없으면 mock 반환.
//   매 호출마다 새로 만들지만 stateless 이므로 비용은 무시 가능.
export function getAnalyzerClient(): AnalyzerClient {
  const bridge = window.archscope?.analyzer;

  if (!bridge) {
    return mockAnalyzerClient;
  }

  return createIpcAnalyzerClient(bridge);
}

export async function loadSampleAnalysisResult(): Promise<DashboardSampleResult> {
  return getAnalyzerClient().loadDashboardSample();
}

function notImplemented<T extends AnalysisResult>(message: string): AnalyzerResponse<T> {
  return {
    ok: false,
    error: {
      code: "ANALYZER_NOT_CONNECTED",
      message,
    },
  };
}
