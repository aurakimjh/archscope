export type AnalysisPrimitive = string | number | boolean | null;
export type AnalysisValue =
  | AnalysisPrimitive
  | AnalysisValue[]
  | { [key: string]: AnalysisValue };

export type AnalysisObject = Record<string, AnalysisValue>;

export type AnalysisResult<
  TType extends string = string,
  TSummary extends object = AnalysisObject,
  TSeries extends object = AnalysisObject,
  TTables extends object = AnalysisObject,
  TCharts extends object = AnalysisObject,
  TMetadata extends object = AnalysisObject,
> = {
  type: TType;
  source_files: string[];
  created_at: string;
  summary: TSummary;
  series: TSeries;
  tables: TTables;
  charts: TCharts;
  metadata: TMetadata;
};

export type DiagnosticSample = {
  line_number: number;
  reason: string;
  message: string;
  raw_preview: string;
};

export type ParserDiagnostics = {
  total_lines: number;
  parsed_records: number;
  skipped_lines: number;
  skipped_by_reason: Record<string, number>;
  samples: DiagnosticSample[];
};

export type AccessLogSummary = {
  total_requests: number;
  avg_response_ms: number;
  p95_response_ms: number;
  p99_response_ms: number;
  error_rate: number;
};

export type TimeValuePoint = {
  time: string;
  value: number;
};

export type StatusCodeDistributionRow = {
  status: string;
  count: number;
};

export type TopUrlCountRow = {
  uri: string;
  count: number;
};

export type TopUrlAvgResponseRow = {
  uri: string;
  avg_response_ms: number;
  count: number;
};

export type AccessLogFinding = {
  severity: string;
  code: string;
  message: string;
  evidence: Record<string, string | number>;
};

export type AccessLogAnalysisOptions = {
  max_lines: number | null;
  start_time: string | null;
  end_time: string | null;
};

export type AccessLogSeries = {
  requests_per_minute: TimeValuePoint[];
  avg_response_time_per_minute: TimeValuePoint[];
  p95_response_time_per_minute: TimeValuePoint[];
  status_code_distribution: StatusCodeDistributionRow[];
  top_urls_by_count: TopUrlCountRow[];
  top_urls_by_avg_response_time: TopUrlAvgResponseRow[];
};

export type AccessLogSampleRecordRow = {
  timestamp: string;
  method: string;
  uri: string;
  status: number;
  response_time_ms: number;
};

export type AccessLogTables = {
  sample_records: AccessLogSampleRecordRow[];
};

export type AccessLogMetadata = {
  format: string;
  parser: string;
  schema_version: string;
  diagnostics: ParserDiagnostics;
  analysis_options: AccessLogAnalysisOptions;
  findings: AccessLogFinding[];
};

export type ProfilerCollapsedSummary = {
  profile_kind: string;
  total_samples: number;
  interval_ms: number;
  estimated_seconds: number;
  elapsed_seconds: number | null;
};

export type ProfilerTopStackSeriesRow = {
  stack: string;
  samples: number;
  estimated_seconds: number;
  sample_ratio: number;
  elapsed_ratio: number | null;
};

export type ComponentBreakdownRow = {
  component: string;
  samples: number;
};

export type ProfilerCollapsedSeries = {
  top_stacks: ProfilerTopStackSeriesRow[];
  component_breakdown: ComponentBreakdownRow[];
};

export type ProfilerTopStackTableRow = ProfilerTopStackSeriesRow & {
  frames: string[];
};

export type ProfilerCollapsedTables = {
  top_stacks: ProfilerTopStackTableRow[];
};

export type ProfilerCollapsedMetadata = {
  parser: string;
  schema_version: string;
  diagnostics: ParserDiagnostics;
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
  maxLines?: number;
  startTime?: string;
  endTime?: string;
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

export type AccessLogAnalysisResult = AnalysisResult<
  "access_log",
  AccessLogSummary,
  AccessLogSeries,
  AccessLogTables,
  AnalysisObject,
  AccessLogMetadata
>;

export type ProfilerCollapsedAnalysisResult = AnalysisResult<
  "profiler_collapsed",
  ProfilerCollapsedSummary,
  ProfilerCollapsedSeries,
  ProfilerCollapsedTables,
  AnalysisObject,
  ProfilerCollapsedMetadata
>;

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
