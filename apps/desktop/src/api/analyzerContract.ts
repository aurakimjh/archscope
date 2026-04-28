export type AnalysisPrimitive = string | number | boolean | null;
export type AnalysisValue =
  | AnalysisPrimitive
  | AnalysisValue[]
  | { [key: string]: AnalysisValue };

export type AnalysisObject = Record<string, AnalysisValue>;

export type AnalysisResult<
  TSummary extends AnalysisObject = AnalysisObject,
  TSeries extends AnalysisObject = AnalysisObject,
  TTables extends AnalysisObject = AnalysisObject,
  TCharts extends AnalysisObject = AnalysisObject,
  TMetadata extends AnalysisObject = AnalysisObject,
> = {
  type: string;
  source_files?: string[];
  created_at?: string;
  summary: TSummary;
  series: TSeries;
  tables?: TTables;
  charts?: TCharts;
  metadata?: TMetadata;
};

export type AccessLogFormat =
  | "nginx"
  | "apache"
  | "ohs"
  | "weblogic"
  | "tomcat"
  | "custom-regex";

export type AnalyzeAccessLogRequest = {
  filePath: string;
  format: AccessLogFormat;
};

export type AnalyzeCollapsedProfileRequest = {
  wallPath: string;
  wallIntervalMs: number;
  elapsedSec?: number;
  topN?: number;
};

export type SelectFileRequest = {
  title?: string;
  filters?: Array<{
    name: string;
    extensions: string[];
  }>;
};

export type SelectFileResponse = {
  canceled: boolean;
  filePath?: string;
};

export type BridgeError = {
  code: string;
  message: string;
  detail?: string;
};

export type AnalyzerSuccess<T extends AnalysisResult = AnalysisResult> = {
  ok: true;
  result: T;
};

export type AnalyzerFailure = {
  ok: false;
  error: BridgeError;
};

export type AnalyzerResponse<T extends AnalysisResult = AnalysisResult> =
  | AnalyzerSuccess<T>
  | AnalyzerFailure;

export type AccessLogAnalysisResult = AnalysisResult & {
  type: "access_log";
};

export type ProfilerCollapsedAnalysisResult = AnalysisResult & {
  type: "profiler_collapsed";
};

export type ArchScopeAnalyzerBridge = {
  analyzeAccessLog(
    request: AnalyzeAccessLogRequest,
  ): Promise<AnalyzerResponse<AccessLogAnalysisResult>>;
  analyzeCollapsedProfile(
    request: AnalyzeCollapsedProfileRequest,
  ): Promise<AnalyzerResponse<ProfilerCollapsedAnalysisResult>>;
};

export type ArchScopeRendererApi = {
  platform: string;
  selectFile?: (request?: SelectFileRequest) => Promise<SelectFileResponse>;
  analyzer?: ArchScopeAnalyzerBridge;
};
