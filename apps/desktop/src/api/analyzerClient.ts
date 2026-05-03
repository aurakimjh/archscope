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
  AccessLogSummary,
  AccessLogTables,
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
  GcLogAnalysisResult,
  GcLogSummary,
  InterpretationResult,
  JvmAnalyzerMetadata,
  JvmFinding,
  ParserDiagnostics,
  ProfilerCollapsedAnalysisResult,
  ProfilerCollapsedMetadata,
  ProfilerCollapsedSeries,
  ProfilerCollapsedSummary,
  ProfilerCollapsedTables,
  ProfilerCharts,
  ProfilerTopStackSeriesRow,
  ProfilerTopStackTableRow,
  SelectFileRequest,
  SelectFileResponse,
  SelectDirectoryResponse,
  StatusCodeDistributionRow,
  ThreadDumpAnalysisResult,
  ThreadDumpSummary,
  TimeValuePoint,
  TopUrlAvgResponseRow,
  TopUrlCountRow,
} from "./analyzerContract";

export {
  isAiFinding,
  isInterpretationResult,
} from "./analyzerContract";

export type DashboardSampleResult = {
  type: string;
  summary: Record<string, number>;
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

export function createIpcAnalyzerClient(bridge: AnalyzerBridge): AnalyzerClient {
  return {
    loadDashboardSample: mockAnalyzerClient.loadDashboardSample,
    execute: bridge.execute,
    cancel: bridge.cancel,
    analyzeAccessLog: bridge.analyzeAccessLog,
    analyzeCollapsedProfile: bridge.analyzeCollapsedProfile,
  };
}

export function createAnalyzerRequestId(): string {
  return (
    globalThis.crypto?.randomUUID?.() ??
    `analysis-${Date.now()}-${Math.random().toString(16).slice(2)}`
  );
}

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
