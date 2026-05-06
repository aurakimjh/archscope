// TypeScript types that mirror the engine-side request structs and the
// `engineapi.AnalysisResult` envelope. Lifted from
// `apps/frontend/src/api/analyzerContract.ts` and pruned to the shapes
// the AccessLog port needs in Phase 1; later phases (T-351'-Phase-2)
// extend this file to cover GcLog, JFR, ThreadDump, etc.
//
// Wails generates an `engineservice.ts` binding once `task generate:bindings`
// runs. The `bridge/engine.ts` wrapper falls back to `Call.ByName` so the
// renderer can call EngineService methods without waiting for that step.
// When the generator is available, callers can switch to the generated
// module — this file's types stay valid because the wire shapes match.

// ──────────────────────────────────────────────────────────────────
// Shared analysis-result envelope (matches engineapi.AnalysisResult)
// ──────────────────────────────────────────────────────────────────

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

// ──────────────────────────────────────────────────────────────────
// AccessLog typed shapes (canonical Phase-1 surface)
// ──────────────────────────────────────────────────────────────────

export type AccessLogFormat =
  | "nginx"
  | "apache"
  | "ohs"
  | "weblogic"
  | "tomcat"
  | "custom-regex";

export type AccessLogSummary = {
  total_requests: number;
  avg_response_ms: number;
  p50_response_ms?: number;
  p90_response_ms?: number;
  p95_response_ms: number;
  p99_response_ms: number;
  error_rate: number;
  error_count?: number;
  total_bytes?: number;
  wall_time_sec?: number;
  avg_requests_per_sec?: number;
  avg_bytes_per_sec?: number;
  static_count?: number;
  api_count?: number;
  static_bytes?: number;
  api_bytes?: number;
  static_avg_response_ms?: number;
  api_avg_response_ms?: number;
  api_p95_response_ms?: number;
  earliest_timestamp?: string | null;
  latest_timestamp?: string | null;
  unique_uris?: number;
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

export type StatusClassPerMinuteRow = {
  time: string;
  "2xx": number;
  "3xx": number;
  "4xx": number;
  "5xx": number;
  other: number;
};

export type ErrorRatePerMinuteRow = {
  time: string;
  value: number;
  errors: number;
  total: number;
};

export type ThroughputPerMinuteRow = {
  time: string;
  requests_per_sec: number;
  bytes_per_sec: number;
};

export type MethodDistributionRow = {
  method: string;
  count: number;
};

export type RequestClassificationRow = {
  classification: string;
  count: number;
};

export type AccessLogSeries = {
  requests_per_minute: TimeValuePoint[];
  avg_response_time_per_minute: TimeValuePoint[];
  p50_response_time_per_minute?: TimeValuePoint[];
  p90_response_time_per_minute?: TimeValuePoint[];
  p95_response_time_per_minute: TimeValuePoint[];
  p99_response_time_per_minute?: TimeValuePoint[];
  status_class_per_minute?: StatusClassPerMinuteRow[];
  error_rate_per_minute?: ErrorRatePerMinuteRow[];
  bytes_per_minute?: TimeValuePoint[];
  throughput_per_minute?: ThroughputPerMinuteRow[];
  status_code_distribution: StatusCodeDistributionRow[];
  method_distribution?: MethodDistributionRow[];
  request_classification?: RequestClassificationRow[];
  top_urls_by_count: TopUrlCountRow[];
  top_urls_by_avg_response_time: TopUrlAvgResponseRow[];
};

export type AccessLogUrlStatRow = {
  uri: string;
  method?: string;
  classification?: "static" | "api" | string;
  count: number;
  avg_response_ms: number;
  p50_response_ms?: number;
  p90_response_ms?: number;
  p95_response_ms: number;
  p99_response_ms?: number;
  total_bytes?: number;
  error_count?: number;
  status_2xx?: number;
  status_3xx?: number;
  status_4xx?: number;
  status_5xx?: number;
};

export type AccessLogStatusCodeRow = {
  status: number;
  count: number;
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
  url_stats?: AccessLogUrlStatRow[];
  top_urls_by_count?: AccessLogUrlStatRow[];
  top_urls_by_avg_response_time?: AccessLogUrlStatRow[];
  top_urls_by_p95_response_time?: AccessLogUrlStatRow[];
  top_urls_by_bytes?: AccessLogUrlStatRow[];
  top_urls_by_errors?: AccessLogUrlStatRow[];
  top_status_codes?: AccessLogStatusCodeRow[];
};

export type AccessLogMetadata = {
  format: string;
  parser: string;
  schema_version: string;
  diagnostics: ParserDiagnostics;
  analysis_options: AccessLogAnalysisOptions;
  findings: AccessLogFinding[];
};

export type AccessLogAnalysisResult = AnalysisResult<
  "access_log",
  AccessLogSummary,
  AccessLogSeries,
  AccessLogTables,
  AnalysisObject,
  AccessLogMetadata
>;

// ──────────────────────────────────────────────────────────────────
// JFR typed shapes (T-351'-Phase-2 — JFR analyzer + native memory).
//
// The Go engine emits two AnalysisResults for the same .jfr/.json
// file depending on which entry point the renderer calls:
//   - AnalyzeJfr           → type "jfr_recording"
//   - AnalyzeNativeMemory  → type "native_memory"
//
// Both modes live on a single page (JfrAnalyzerPage); the types below
// sit side-by-side so the renderer can switch on `result.type`.
//
// Mirrors apps/engine-native/internal/analyzers/jfr/{analyzer,native_memory}.go.
// ──────────────────────────────────────────────────────────────────

export type JfrAnalysisMode =
  | "all"
  | "cpu"
  | "wall"
  | "alloc"
  | "lock"
  | "gc"
  | "exception"
  | "io"
  | "nativemem";

export type JfrSummary = {
  event_count?: number;
  event_count_total?: number;
  duration_ms?: number;
  gc_pause_total_ms?: number;
  blocked_thread_events?: number;
  selected_mode?: string;
};

export type JfrFilterWindow = {
  from?: string | null;
  to?: string | null;
  effective_start?: string | null;
  effective_end?: string | null;
};

export type JfrMetadata = {
  schema_version?: string;
  parser?: string;
  diagnostics?: ParserDiagnostics;
  selected_mode?: string;
  available_modes?: string[];
  supported_modes?: string[];
  available_states?: string[];
  selected_state?: string | null;
  min_duration_ms?: number | null;
  filter_window?: JfrFilterWindow;
  source_format?: string;
  jfr_cli?: string | null;
  jfr_command_version?: string;
};

export type JfrEventTypeRow = {
  event_type: string;
  count: number;
};

export type JfrEventOverTimeRow = {
  time: string;
  event_type: string;
  count: number;
};

export type JfrPauseEventRow = {
  time: string;
  duration_ms: number | null;
  event_type: string;
  thread: string | null;
  sampling_type: string;
};

export type JfrHeatmapBucket = {
  index: number;
  time: string;
  count: number;
};

export type JfrHeatmapStrip = {
  bucket_seconds: number;
  start_time: string | null;
  end_time: string | null;
  max_count: number;
  buckets: JfrHeatmapBucket[];
};

export type JfrNotableEventRow = {
  time?: string;
  event_type?: string;
  duration_ms?: number | null;
  thread?: string | null;
  message?: string;
  frames?: string[];
  sampling_type?: string;
  evidence_ref?: string;
  raw_preview?: string;
};

export type JfrSeries = {
  events_over_time?: JfrEventOverTimeRow[];
  pause_events?: JfrPauseEventRow[];
  events_by_type?: JfrEventTypeRow[];
  heatmap_strip?: JfrHeatmapStrip;
};

export type JfrTables = {
  notable_events?: JfrNotableEventRow[];
};

export type JfrAnalysisResult = AnalysisResult<
  "jfr_recording",
  JfrSummary,
  JfrSeries,
  JfrTables,
  AnalysisObject,
  JfrMetadata
>;

// ── Native memory ────────────────────────────────────────────────

export type NativeMemorySummary = {
  alloc_event_count?: number;
  free_event_count?: number;
  alloc_bytes_total?: number;
  free_bytes_total?: number;
  unfreed_event_count?: number;
  unfreed_bytes_total?: number;
  tail_ratio?: number;
  tail_cutoff?: string | null;
  leak_only?: boolean;
};

export type NativeMemoryCallSiteRow = {
  stack: string;
  bytes: number;
};

export type NativeMemoryTables = {
  top_call_sites?: NativeMemoryCallSiteRow[];
};

// FlameNode mirrors the engine's flamegraph_builder shape — see
// apps/engine-native/internal/analyzers/jfr/native_memory.go::freezeNode.
export type NativeMemoryFlameNode = {
  id: string;
  parentId: string | null;
  name: string;
  samples: number;
  ratio: number;
  category: string | null;
  color: string | null;
  path: string[];
  children: NativeMemoryFlameNode[];
};

export type NativeMemoryCharts = {
  flamegraph?: NativeMemoryFlameNode;
};

export type NativeMemoryMetadata = {
  schema_version?: string;
  parser?: string;
  diagnostics?: ParserDiagnostics;
  unit?: string;
  source_format?: string;
  jfr_cli?: string | null;
};

export type NativeMemoryAnalysisResult = AnalysisResult<
  "native_memory",
  NativeMemorySummary,
  AnalysisObject,
  NativeMemoryTables,
  NativeMemoryCharts,
  NativeMemoryMetadata
>;

// ──────────────────────────────────────────────────────────────────
// Engine request shapes — must mirror engineservice.go field tags.
// ──────────────────────────────────────────────────────────────────

export type AccessLogRequest = {
  path: string;
  format?: string;
  maxLines?: number;
  startTime?: string;
  endTime?: string;
};

export type GcLogRequest = {
  path: string;
  topN?: number;
  maxLines?: number;
  strict?: boolean;
};

export type JfrRequest = {
  path: string;
  topN?: number;
  mode?: string;
  fromTime?: string;
  toTime?: string;
  state?: string;
  leakOnly?: boolean;
  tailRatio?: number;
};

export type ExceptionRequest = {
  path: string;
  topN?: number;
  maxLines?: number;
  strict?: boolean;
};

export type RuntimeRequest = {
  path: string;
  variant: "nodejs" | "python" | "go" | "dotnet";
  topN?: number;
  maxLines?: number;
  strict?: boolean;
};

export type OtelRequest = {
  path: string;
  topN?: number;
};

export type ThreadDumpRequest = {
  path: string;
  topN?: number;
};

export type MultiThreadRequest = {
  paths: string[];
  formatOverride?: string;
  threshold?: number;
  topN?: number;
  includeRawSnapshots?: boolean;
};

export type LockContentionRequest = {
  paths: string[];
  formatOverride?: string;
  topN?: number;
};

export type CollapsedRequest = {
  paths: string[];
  formatOverride?: string;
  includeThreadName?: boolean;
};

export type CollapsedResult = {
  counts: Record<string, number>;
  lines: string[];
};

export type ClassifyRequest = {
  stack: string;
};

export type ClassifyResult = {
  label: string;
};

export type ExportJSONRequest = {
  path: string;
  result: unknown;
};

export type ExportHTMLRequest = ExportJSONRequest;
export type ExportPPTXRequest = ExportJSONRequest;
export type ExportCSVRequest = ExportJSONRequest;

export type EngineDiffRequest = {
  beforePath: string;
  afterPath: string;
  label?: string;
};

export type EngineAsyncResponse = {
  taskId: string;
};

export type EngineDoneEvent = {
  taskId: string;
  result: AnalysisResult;
};

export type EngineErrorEvent = {
  taskId: string;
  message: string;
};

export type EngineCancelledEvent = {
  taskId: string;
};

// ──────────────────────────────────────────────────────────────────
// Renderer-side error shape — keeps the AccessLog UI code identical
// to its web origin. The bridge promotes Wails RuntimeError into this
// envelope so callers don't need to discriminate transport types.
// ──────────────────────────────────────────────────────────────────

export type BridgeError = {
  code: string;
  message: string;
  detail?: string;
};
