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
// Exception (exception_stack) typed shapes
// ──────────────────────────────────────────────────────────────────
//
// Mirrors apps/engine-native/internal/analyzers/exception.Build:
//   - summary: total_exceptions / unique_exception_types /
//     unique_signatures / top_exception_type
//   - series : exception_type_distribution, root_cause_distribution,
//     top_stack_signatures
//   - tables : exceptions[]  (timestamp, language, exception_type,
//     message, root_cause, signature, top_frame, stack)
//
// Findings emitted by `_build_findings`:
//   REPEATED_EXCEPTION_SIGNATURE — top signature seen > 1 time
//   ROOT_CAUSE_PRESENT          — any record carries a root_cause

export type ExceptionStackSummary = {
  total_exceptions: number;
  unique_exception_types: number;
  unique_signatures: number;
  top_exception_type: string | null;
};

export type ExceptionTypeDistributionRow = {
  exception_type: string;
  count: number;
};

export type ExceptionRootCauseDistributionRow = {
  root_cause: string;
  count: number;
};

export type ExceptionSignatureRow = {
  signature: string;
  count: number;
};

// ──────────────────────────────────────────────────────────────────
// ThreadDump typed shapes (T-351'-Phase-2 — single + multi + locks).
//
// Three engine analyzers feed the ThreadDumpAnalyzerPage:
//   - threaddump.Build       → type "thread_dump"      (sync, JVM jstack)
//   - multithread.Analyze    → type "thread_dump_multi"(async, N dumps)
//   - lockcontention.Analyze → type "lock_contention"  (async, N dumps)
//
// We keep three result types side-by-side; the renderer chooses the
// shape per mode (single / multi / locks). Cross-mode summary fields
// like `dumps`, `findings`, and `diagnostics` are repeated rather than
// hoisted because each analyzer's emit shape is independently testable
// against its Go side.
//
// Mirrors apps/engine-native/internal/analyzers/{threaddump,multithread,
// lockcontention}/analyzer.go.
// ──────────────────────────────────────────────────────────────────

// — Single dump —
export type ThreadDumpStateRow = {
  state: string;
  count: number;
};

export type ThreadDumpCategoryRow = {
  category: string;
  count: number;
};

export type ThreadDumpSignatureRow = {
  signature: string;
  count: number;
};

export type ExceptionStackSeries = {
  exception_type_distribution: ExceptionTypeDistributionRow[];
  root_cause_distribution: ExceptionRootCauseDistributionRow[];
  top_stack_signatures: ExceptionSignatureRow[];
};

export type ExceptionEventRow = {
  timestamp: string | null;
  language: string | null;
  exception_type: string | null;
  message: string | null;
  root_cause: string | null;
  signature: string | null;
  top_frame: string | null;
  stack: string[] | null;
};

export type ExceptionStackTables = {
  exceptions: ExceptionEventRow[];
};

export type ExceptionFinding = {
  severity: string;
  code: string;
  message: string;
  evidence: Record<string, string | number>;
};

export type ExceptionStackMetadata = {
  parser: string;
  schema_version: string;
  diagnostics: ParserDiagnostics;
  findings: ExceptionFinding[];
};

export type ExceptionStackAnalysisResult = AnalysisResult<
  "exception_stack",
  ExceptionStackSummary,
  ExceptionStackSeries,
  ExceptionStackTables,
  AnalysisObject,
  ExceptionStackMetadata
>;

export type ThreadDumpThreadRow = {
  name?: string;
  thread_name?: string;
  state?: string;
  category?: string | null;
  top_frame?: string | null;
  lock_info?: string | null;
  stack_signature?: string;
  frames?: string[];
};

export type ThreadDumpFinding = {
  severity: "info" | "warning" | "critical" | string;
  code: string;
  message: string;
  evidence?: Record<string, AnalysisValue>;
};

export type ThreadDumpSingleSummary = {
  total_threads: number;
  runnable_threads: number;
  blocked_threads: number;
  waiting_threads: number;
  threads_with_locks: number;
};

export type ThreadDumpSingleSeries = {
  state_distribution: ThreadDumpStateRow[];
  category_distribution: ThreadDumpCategoryRow[];
  top_stack_signatures: ThreadDumpSignatureRow[];
};

export type ThreadDumpSingleTables = {
  threads: ThreadDumpThreadRow[];
};

export type ThreadDumpSingleMetadata = {
  schema_version?: string;
  parser?: string;
  diagnostics: ParserDiagnostics;
  findings?: ThreadDumpFinding[];
};

export type ThreadDumpSingleResult = AnalysisResult<
  "thread_dump",
  ThreadDumpSingleSummary,
  ThreadDumpSingleSeries,
  ThreadDumpSingleTables,
  AnalysisObject,
  ThreadDumpSingleMetadata
>;

// — Multi-dump correlation —
export type ThreadDumpMultiSummary = {
  total_dumps: number;
  unique_threads: number;
  total_thread_observations?: number;
  long_running_threads: number;
  persistent_blocked_threads: number;
  latency_sections?: number;
  growing_lock_contention?: number;
  virtual_thread_carrier_pinning?: number;
  smr_unresolved_threads?: number;
  native_method_threads?: number;
  class_histogram_classes?: number;
  class_histogram_incomplete?: number;
  languages_detected: string[];
  source_formats: string[];
  consecutive_dump_threshold: number;
};

export type LongRunningStackRow = {
  thread_name: string;
  stack_signature: string;
  dumps: number;
  first_dump_index: number;
};

export type PersistentBlockedRow = {
  thread_name: string;
  dumps: number;
  first_dump_index: number;
  stack_signatures?: string[];
};

export type StatePerDumpRow = {
  dump_index: number;
  dump_label: string | null;
  counts: Record<string, number>;
};

export type ThreadPersistenceRow = {
  thread_name: string;
  observed_in_dumps: number;
};

export type DumpRow = {
  dump_index: number;
  dump_label: string | null;
  source_file: string;
  source_format: string;
  language: string;
  thread_count: number;
};

export type CarrierPinningRow = {
  dump_index: number;
  thread_name: string;
  source_file?: string;
  candidate_method?: string | null;
  top_frame?: string | null;
  reason?: string | null;
};

export type SmrUnresolvedRow = {
  dump_index: number;
  source_file?: string;
  section_line?: number;
  line?: string;
};

export type NativeMethodRow = {
  dump_index: number;
  thread_name: string;
  source_file?: string;
  native_method?: string;
  top_frame?: string | null;
};

export type ClassHistogramRow = {
  rank?: number;
  class_name: string;
  instances?: number;
  bytes?: number;
  source_file?: string;
};

export type ThreadDumpMultiSeries = {
  thread_persistence?: ThreadPersistenceRow[];
  state_distribution_per_dump?: StatePerDumpRow[];
  state_transition_timeline?: AnalysisObject[];
};

export type ThreadDumpMultiTables = {
  long_running_stacks?: LongRunningStackRow[];
  persistent_blocked_threads?: PersistentBlockedRow[];
  latency_sections?: AnalysisObject[];
  growing_lock_contention?: AnalysisObject[];
  dumps?: DumpRow[];
  virtual_thread_carrier_pinning?: CarrierPinningRow[];
  smr_unresolved_threads?: SmrUnresolvedRow[];
  native_method_threads?: NativeMethodRow[];
  class_histogram_top_classes?: ClassHistogramRow[];
  class_histogram_incomplete?: AnalysisObject[];
};

export type ThreadDumpMultiMetadata = {
  schema_version?: string;
  parser?: string;
  diagnostics: ParserDiagnostics;
  findings?: ThreadDumpFinding[];
};

export type ThreadDumpMultiResult = AnalysisResult<
  "thread_dump_multi",
  ThreadDumpMultiSummary,
  ThreadDumpMultiSeries,
  ThreadDumpMultiTables,
  AnalysisObject,
  ThreadDumpMultiMetadata
>;

// — Lock contention —
export type LockContentionRow = {
  lock_id: string;
  lock_class: string | null;
  owner_thread: string | null;
  owner_stack_signature?: string | null;
  owner_count: number;
  waiter_count: number;
  top_waiters: string[];
  all_waiters?: string[];
  contention_candidate?: boolean;
};

export type DeadlockEdge = {
  from_thread: string;
  to_thread: string;
  lock_id: string | null;
  lock_class: string | null;
};

export type DeadlockChain = {
  threads: string[];
  edges: DeadlockEdge[];
};

export type LockContentionSummary = {
  total_dumps: number;
  total_thread_snapshots?: number;
  threads_with_locks: number;
  threads_waiting_on_lock: number;
  unique_locks: number;
  contended_locks: number;
  deadlocks_detected: number;
  languages_detected?: string[];
  source_formats?: string[];
};

export type LockContentionRankingRow = {
  lock_id: string;
  lock_class: string | null;
  waiter_count: number;
};

export type LockContentionSeries = {
  contention_ranking?: LockContentionRankingRow[];
};

export type LockContentionTables = {
  locks?: LockContentionRow[];
  deadlock_chains?: DeadlockChain[];
  dumps?: DumpRow[];
};

export type LockContentionMetadata = {
  schema_version?: string;
  parser?: string;
  diagnostics: ParserDiagnostics;
  findings?: ThreadDumpFinding[];
};

export type LockContentionResult = AnalysisResult<
  "lock_contention",
  LockContentionSummary,
  LockContentionSeries,
  LockContentionTables,
  AnalysisObject,
  LockContentionMetadata
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
