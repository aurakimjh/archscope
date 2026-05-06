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
// GcLog typed shapes (T-351'-Phase-2)
//
// Lifted from `apps/frontend/src/api/analyzerContract.ts` and aligned
// with what the engine emits in
// `apps/engine-native/internal/analyzers/gclog/analyzer.go`:
//   - summary keys come from the analyzer's Summary builder.
//   - series keys hang off Result.Series (pause_timeline,
//     heap_*_mb, allocation/promotion rate, gc_type/cause breakdown).
//   - tables.events carries one row per parsed GC event.
//   - metadata extends the JVM analyzer envelope
//     (parser/schema_version/diagnostics/findings) with the optional
//     gc_format and jvm_info blocks.
// ──────────────────────────────────────────────────────────────────

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

export type GcRatePoint = {
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
  old_before_mb?: number | null;
  old_after_mb?: number | null;
  metaspace_before_mb?: number | null;
  metaspace_after_mb?: number | null;
};

export type GcLogSeries = {
  pause_timeline: GcPauseTimelinePoint[];
  heap_after_mb: GcHeapPoint[];
  heap_before_mb: GcHeapPoint[];
  heap_committed_mb?: GcHeapPoint[];
  young_before_mb?: GcHeapPoint[];
  young_after_mb: GcHeapPoint[];
  old_before_mb?: GcHeapPoint[];
  old_after_mb?: GcHeapPoint[];
  metaspace_before_mb?: GcHeapPoint[];
  metaspace_after_mb?: GcHeapPoint[];
  pause_histogram: GcPauseHistogramBucket[];
  allocation_rate_mb_per_sec: GcRatePoint[];
  promotion_rate_mb_per_sec: GcRatePoint[];
  gc_type_breakdown: GcTypeBreakdownRow[];
  cause_breakdown: GcCauseBreakdownRow[];
};

export type GcLogTables = {
  events: GcEventRow[];
};

export type GcJvmInfo = {
  vm_banner?: string;
  vm_version?: string;
  vm_build?: string;
  platform?: string;
  collector?: string;
  cpus_total?: number;
  cpus_available?: number;
  memory_mb?: number;
  page_size_kb?: number;
  heap_min_mb?: number;
  heap_initial_mb?: number;
  heap_max_mb?: number;
  heap_region_size_mb?: number;
  parallel_workers?: number;
  concurrent_workers?: number;
  concurrent_refinement_workers?: number;
  large_pages?: string;
  numa?: string;
  compressed_oops?: string;
  pre_touch?: string;
  periodic_gc?: string;
  command_line?: string;
  raw_lines?: string[];
};

export type GcLogMetadata = JvmAnalyzerMetadata & {
  gc_format?: string;
  jvm_info?: GcJvmInfo;
};

export type GcLogAnalysisResult = AnalysisResult<
  "gc_log",
  GcLogSummary,
  GcLogSeries,
  GcLogTables,
  AnalysisObject,
  GcLogMetadata
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
