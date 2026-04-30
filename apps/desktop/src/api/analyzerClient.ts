import { sampleAnalysisResult } from "../charts/sampleCharts";
import type {
  AccessLogAnalysisResult,
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzerExecuteRequest,
  AnalyzerExecutionResult,
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
  AnalyzerExecuteRequest,
  AnalyzerExecutionResult,
  AnalyzerFailure,
  AnalyzerResponse,
  AnalyzerSuccess,
  ArchScopeAnalyzerBridge,
  ArchScopeRendererApi,
  BridgeError,
  ComponentBreakdownRow,
  DiagnosticSample,
  InterpretationResult,
  ParserDiagnostics,
  ProfilerCollapsedAnalysisResult,
  ProfilerCollapsedMetadata,
  ProfilerCollapsedSeries,
  ProfilerCollapsedSummary,
  ProfilerCollapsedTables,
  ProfilerTopStackSeriesRow,
  ProfilerTopStackTableRow,
  SelectFileRequest,
  SelectFileResponse,
  StatusCodeDistributionRow,
  TimeValuePoint,
  TopUrlAvgResponseRow,
  TopUrlCountRow,
} from "./analyzerContract";

export {
  isAiFinding,
  isInterpretationResult,
} from "./analyzerContract";

export type DashboardSampleResult = typeof sampleAnalysisResult;
export type SampleAnalysisResult = DashboardSampleResult;

export type AnalyzerClient = {
  loadDashboardSample(): Promise<DashboardSampleResult>;
  execute(
    request: AnalyzerExecuteRequest,
  ): Promise<AnalyzerResponse<AnalyzerExecutionResult>>;
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
};

export function createIpcAnalyzerClient(bridge: AnalyzerBridge): AnalyzerClient {
  return {
    loadDashboardSample: mockAnalyzerClient.loadDashboardSample,
    execute: bridge.execute,
    analyzeAccessLog: bridge.analyzeAccessLog,
    analyzeCollapsedProfile: bridge.analyzeCollapsedProfile,
  };
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
