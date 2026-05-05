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
  source_file?: string | null;
  format?: string | null;
  total_lines: number;
  parsed_records: number;
  skipped_lines: number;
  skipped_by_reason: Record<string, number>;
  samples: DiagnosticSample[];
  warning_count?: number;
  error_count?: number;
  warnings?: DiagnosticSample[];
  errors?: DiagnosticSample[];
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

export type TimelineChainRow = {
  chain: string;
  samples: number;
  frames: string[];
};

export type TimelineAnalysisRow = {
  index: number;
  segment: string;
  label: string;
  samples: number;
  estimated_seconds: number;
  stage_ratio: number;
  total_ratio: number;
  elapsed_ratio: number | null;
  top_methods: BreakdownTopItem[];
  method_chains: TimelineChainRow[];
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
  diagnostics?: Record<string, AnalysisValue> | null;
};

export type ProfilerCollapsedSeries = {
  top_stacks: ProfilerTopStackSeriesRow[];
  component_breakdown: ComponentBreakdownRow[];
  execution_breakdown?: ExecutionBreakdownRow[];
  timeline_analysis?: TimelineAnalysisRow[];
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
  timeline_analysis?: TimelineAnalysisRow[];
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

export type JvmFinding = {
  severity: string;
  code: string;
  message: string;
  evidence: Record<string, string | number | boolean | null>;
};

export type JvmAnalyzerMetadata = {
  parser: string;
  schema_version: string;
  diagnostics: ParserDiagnostics;
  findings: JvmFinding[];
};

export type GcLogSummary = {
  total_events: number;
  total_pause_ms: number;
  avg_pause_ms: number;
  max_pause_ms: number;
  p50_pause_ms: number;
  p95_pause_ms: number;
  p99_pause_ms: number;
  throughput_percent: number;
  wall_time_sec: number;
  young_gc_count: number;
  full_gc_count: number;
  avg_allocation_rate_mb_per_sec: number;
  avg_promotion_rate_mb_per_sec: number;
  humongous_allocation_count: number;
  concurrent_mode_failure_count: number;
  promotion_failure_count: number;
};

export type GcPauseHistogramBucket = {
  bucket: string;
  min_ms: number;
  max_ms: number | null;
  count: number;
};

export type GcPauseTimelinePoint = {
  time: string;
  value: number;
  gc_type: string;
};

export type GcHeapPoint = {
  time: string;
  value: number;
};

export type GcTypeBreakdownRow = {
  gc_type: string;
  count: number;
};

export type GcCauseBreakdownRow = {
  cause: string;
  count: number;
};

export type GcEventRow = {
  timestamp: string | null;
  uptime_sec: number | null;
  gc_type: string | null;
  cause: string | null;
  pause_ms: number | null;
  heap_before_mb: number | null;
  heap_after_mb: number | null;
  heap_committed_mb: number | null;
  young_before_mb: number | null;
  young_after_mb: number | null;
};

export type GcRatePoint = {
  time: string;
  value: number;
};

export type GcLogSeries = {
  pause_timeline: GcPauseTimelinePoint[];
  heap_after_mb: GcHeapPoint[];
  heap_before_mb: GcHeapPoint[];
  young_after_mb: GcHeapPoint[];
  pause_histogram: GcPauseHistogramBucket[];
  allocation_rate_mb_per_sec: GcRatePoint[];
  promotion_rate_mb_per_sec: GcRatePoint[];
  gc_type_breakdown: GcTypeBreakdownRow[];
  cause_breakdown: GcCauseBreakdownRow[];
};

export type GcLogTables = {
  events: GcEventRow[];
};

export type GcLogMetadata = JvmAnalyzerMetadata & {
  gc_format?: string;
};

export type GcLogAnalysisResult = AnalysisResult<
  "gc_log",
  GcLogSummary,
  GcLogSeries,
  GcLogTables,
  AnalysisObject,
  GcLogMetadata
>;

export type ThreadDumpSummary = {
  total_threads: number;
  runnable_threads: number;
  blocked_threads: number;
  waiting_threads: number;
  threads_with_locks: number;
};

export type ThreadDumpAnalysisResult = AnalysisResult<
  "thread_dump",
  ThreadDumpSummary,
  AnalysisObject,
  AnalysisObject,
  AnalysisObject,
  JvmAnalyzerMetadata
>;

export type ExceptionStackSummary = {
  total_exceptions: number;
  unique_exception_types: number;
  unique_signatures: number;
  top_exception_type: string | null;
};

export type ExceptionStackAnalysisResult = AnalysisResult<
  "exception_stack",
  ExceptionStackSummary,
  AnalysisObject,
  AnalysisObject,
  AnalysisObject,
  JvmAnalyzerMetadata
>;

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
  requestId?: string;
  filePath: string;
  format: AccessLogFormat;
  maxLines?: number;
  startTime?: string;
  endTime?: string;
};

export type ProfileKind = "wall" | "cpu" | "lock";
export type ProfileFormat =
  | "collapsed"
  | "jennifer_csv"
  | "flamegraph_svg"
  | "flamegraph_html";

export type AnalyzeCollapsedProfileRequest = {
  requestId?: string;
  wallPath: string;
  wallIntervalMs: number;
  elapsedSec?: number;
  topN?: number;
  profileKind?: ProfileKind;
  profileFormat?: ProfileFormat;
};

export type AnalyzeFileRequest = {
  requestId?: string;
  filePath: string;
  topN?: number;
};

export type AnalyzerExecuteRequest = {
  requestId?: string;
} & (
  | {
      type: "access_log";
      params: AnalyzeAccessLogRequest;
    }
  | {
      type: "profiler_collapsed";
      params: AnalyzeCollapsedProfileRequest;
    }
  | {
      type: "gc_log";
      params: AnalyzeFileRequest;
    }
  | {
      type: "thread_dump";
      params: AnalyzeFileRequest;
    }
  | {
      type: "exception_stack";
      params: AnalyzeFileRequest;
    }
  | {
      type: "jfr_recording";
      params: AnalyzeFileRequest;
    }
  | {
      type: "thread_dump_multi";
      params: AnalyzeMultiThreadDumpRequest;
    }
  | {
      type: "thread_dump_locks";
      params: AnalyzeLockContentionRequest;
    }
);

export type AnalyzeMultiThreadDumpRequest = {
  requestId?: string;
  filePaths: string[];
  consecutiveThreshold?: number;
  format?: string;
  topN?: number;
};

export type AnalyzeLockContentionRequest = {
  requestId?: string;
  filePaths: string[];
  format?: string;
  topN?: number;
};

export type JfrAnalysisResult = AnalysisResult<"jfr_recording">;

export type AnalyzerExecutionResult =
  | AccessLogAnalysisResult
  | ProfilerCollapsedAnalysisResult
  | GcLogAnalysisResult
  | ThreadDumpAnalysisResult
  | ExceptionStackAnalysisResult
  | JfrAnalysisResult;

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

export type SelectDirectoryResponse = {
  canceled: boolean;
  directoryPath?: string;
};

export type ExportFormat = "html" | "diff" | "pptx";

export type ExportExecuteRequest =
  | {
      format: "html";
      inputPath: string;
      title?: string;
    }
  | {
      format: "pptx";
      inputPath: string;
      title?: string;
    }
  | {
      format: "diff";
      beforePath: string;
      afterPath: string;
      label?: string;
    };

export type ExportSuccess = {
  ok: true;
  outputPaths: string[];
  engine_messages?: string[];
};

export type ExportFailure = {
  ok: false;
  error: BridgeError;
};

export type ExportResponse = ExportSuccess | ExportFailure;

export type DemoScenarioManifest = {
  scenario: string;
  dataSource: "real" | "synthetic" | "unknown";
  manifestPath: string;
  description: string;
  analyzers: string[];
};

export type DemoListResponse =
  | {
      ok: true;
      manifestRoot: string;
      scenarios: DemoScenarioManifest[];
    }
  | {
      ok: false;
      error: BridgeError;
    };

export type DemoRunRequest = {
  manifestRoot: string;
  outputRoot?: string;
  scenario?: string;
  dataSource?: "real" | "synthetic";
};

export type DemoOutputArtifact = {
  kind: "json" | "html" | "pptx" | "index" | "summary" | "comparison";
  label: string;
  path: string;
  exportable: boolean;
};

export type DemoReferenceFile = {
  file: string;
  description?: string;
  path: string;
};

export type DemoRunScenarioResult = {
  scenario: string;
  dataSource: "real" | "synthetic" | "unknown";
  bundleIndexPath: string;
  summaryPath: string;
  summary: {
    analyzerOutputs: number;
    failedAnalyzers: number;
    skippedLines: number;
    referenceFiles: number;
    findingCount: number;
    comparisonReports: number;
  };
  artifacts: DemoOutputArtifact[];
  referenceFiles: DemoReferenceFile[];
  failedAnalyzers: Array<{
    file: string;
    analyzerType: string;
    error?: string;
  }>;
  skippedLineReport: Array<{
    file: string;
    analyzerType: string;
    skippedLines: number;
  }>;
};

export type DemoRunSuccess = {
  ok: true;
  outputPaths: string[];
  exportInputPaths: string[];
  scenarios: DemoRunScenarioResult[];
  engine_messages?: string[];
};

export type DemoRunFailure = {
  ok: false;
  error: BridgeError;
};

export type DemoRunResponse = DemoRunSuccess | DemoRunFailure;

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
  cancel(requestId: string): Promise<AnalyzerCancelResponse>;
  analyzeAccessLog(
    request: AnalyzeAccessLogRequest,
  ): Promise<AnalyzerResponse<AccessLogAnalysisResult>>;
  analyzeCollapsedProfile(
    request: AnalyzeCollapsedProfileRequest,
  ): Promise<AnalyzerResponse<ProfilerCollapsedAnalysisResult>>;
};

export type AnalyzerCancelResponse =
  | { ok: true; canceled: boolean }
  | { ok: false; error: BridgeError };

export type ArchScopeExportBridge = {
  execute(request: ExportExecuteRequest): Promise<ExportResponse>;
};

export type ArchScopeDemoBridge = {
  list(manifestRoot?: string): Promise<DemoListResponse>;
  run(request: DemoRunRequest): Promise<DemoRunResponse>;
  openPath(path: string): Promise<{ ok: true } | { ok: false; error: BridgeError }>;
};

export type AppSettings = {
  enginePath: string;
  chartTheme: "light" | "dark";
  locale: "en" | "ko";
};

export type ArchScopeSettingsBridge = {
  get: () => Promise<AppSettings>;
  set: (settings: AppSettings) => Promise<{ ok: boolean }>;
};

export type ArchScopeRendererApi = {
  platform: string;
  selectFile?: (request?: SelectFileRequest) => Promise<SelectFileResponse>;
  selectDirectory?: (request?: { title?: string }) => Promise<SelectDirectoryResponse>;
  analyzer?: ArchScopeAnalyzerBridge;
  exporter?: ArchScopeExportBridge;
  demo?: ArchScopeDemoBridge;
  settings?: ArchScopeSettingsBridge;
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
