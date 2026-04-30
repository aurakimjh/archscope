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

export type BreakdownTopItem = {
  name: string;
  samples: number;
};

export type ExecutionBreakdownRow = {
  category: string;
  executive_label: string;
  primary_category: string;
  wait_reason: string | null;
  samples: number;
  estimated_seconds: number;
  total_ratio: number;
  parent_stage_ratio: number;
  elapsed_ratio: number | null;
  top_methods: BreakdownTopItem[];
  top_stacks: BreakdownTopItem[];
};

export type FlameNode = {
  id: string;
  parentId: string | null;
  name: string;
  samples: number;
  ratio: number;
  category: string | null;
  color: string | null;
  children: FlameNode[];
  path: string[];
};

export type DrilldownStage = {
  index: number;
  label: string;
  breadcrumb: string[];
  filter: Record<string, AnalysisValue> | null;
  metrics: {
    total_samples: number;
    matched_samples: number;
    estimated_seconds: number;
    total_ratio: number;
    parent_stage_ratio: number;
    elapsed_ratio: number | null;
  };
  flamegraph: FlameNode;
  top_stacks: Array<Record<string, AnalysisValue>>;
  top_child_frames: Array<Record<string, AnalysisValue>>;
};

export type ProfilerCollapsedSeries = {
  top_stacks: ProfilerTopStackSeriesRow[];
  component_breakdown: ComponentBreakdownRow[];
  execution_breakdown?: ExecutionBreakdownRow[];
};

export type ProfilerTopStackTableRow = ProfilerTopStackSeriesRow & {
  frames: string[];
};

export type ProfilerCollapsedTables = {
  top_stacks: ProfilerTopStackTableRow[];
  top_child_frames?: Array<{
    frame: string;
    samples: number;
    ratio: number;
  }>;
};

export type ProfilerCharts = {
  flamegraph?: FlameNode;
  drilldown_stages?: DrilldownStage[];
};

export type ProfilerCollapsedMetadata = {
  parser: string;
  schema_version: string;
  diagnostics: ParserDiagnostics;
};

export type AiSeverity = "info" | "warning" | "critical";

export type AiFinding = {
  id: string;
  label: string;
  severity: AiSeverity;
  generated_by: "ai";
  model: string;
  summary: string;
  reasoning: string;
  evidence_refs: string[];
  evidence_quotes?: Record<string, string>;
  confidence: number;
  limitations: string[];
};

export type InterpretationResult = {
  schema_version: "0.1.0";
  provider: string;
  model: string;
  prompt_version: string;
  source_result_type: string;
  source_schema_version: string;
  generated_at: string;
  findings: AiFinding[];
  disabled: boolean;
};

export type AiInterpretationSettings = {
  enabled: boolean;
  provider: "ollama";
  baseUrl: string;
  model: string;
  timeoutSeconds: number;
  maxConcurrency: 1;
  logPrompts: false;
  logResponses: false;
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

export type AnalyzerExecuteRequest =
  | {
      type: "access_log";
      params: AnalyzeAccessLogRequest;
    }
  | {
      type: "profiler_collapsed";
      params: AnalyzeCollapsedProfileRequest;
    };

export type AnalyzerExecutionResult =
  | AccessLogAnalysisResult
  | ProfilerCollapsedAnalysisResult;

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
  engine_messages?: string[];
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
  ProfilerCharts,
  ProfilerCollapsedMetadata
>;

export type ArchScopeAnalyzerBridge = {
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

export type ArchScopeRendererApi = {
  platform: string;
  selectFile?: (request?: SelectFileRequest) => Promise<SelectFileResponse>;
  analyzer?: ArchScopeAnalyzerBridge;
};

export function isInterpretationResult(value: unknown): value is InterpretationResult {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return false;
  }

  const candidate = value as Record<string, unknown>;
  return (
    candidate.schema_version === "0.1.0" &&
    typeof candidate.provider === "string" &&
    typeof candidate.model === "string" &&
    typeof candidate.prompt_version === "string" &&
    typeof candidate.source_result_type === "string" &&
    typeof candidate.source_schema_version === "string" &&
    typeof candidate.generated_at === "string" &&
    typeof candidate.disabled === "boolean" &&
    Array.isArray(candidate.findings) &&
    candidate.findings.every(isAiFinding)
  );
}

export function isAiFinding(value: unknown): value is AiFinding {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return false;
  }

  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.id === "string" &&
    typeof candidate.label === "string" &&
    ["info", "warning", "critical"].includes(String(candidate.severity)) &&
    candidate.generated_by === "ai" &&
    typeof candidate.model === "string" &&
    typeof candidate.summary === "string" &&
    typeof candidate.reasoning === "string" &&
    Array.isArray(candidate.evidence_refs) &&
    candidate.evidence_refs.length > 0 &&
    candidate.evidence_refs.every((ref) => typeof ref === "string" && ref.trim()) &&
    typeof candidate.confidence === "number" &&
    candidate.confidence >= 0 &&
    candidate.confidence <= 1 &&
    Array.isArray(candidate.limitations) &&
    candidate.limitations.every((item) => typeof item === "string") &&
    isOptionalStringRecord(candidate.evidence_quotes)
  );
}

function isOptionalStringRecord(value: unknown): boolean {
  if (value === undefined) {
    return true;
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return false;
  }
  return Object.values(value).every((item) => typeof item === "string");
}
