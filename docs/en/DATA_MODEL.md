# Data Model

ArchScope normalizes all parser outputs into shared analysis structures. This lets the UI, chart templates, and exporters work against stable result contracts.

## Common Result Model

```text
AnalysisResult
  type
  source_files
  created_at
  summary
  series
  tables
  charts
  metadata
```

### Field Purpose

- `type`: Diagnostic result type, such as `access_log` or `profiler_collapsed`.
- `source_files`: Source file paths used to produce the result.
- `created_at`: ISO 8601 timestamp.
- `summary`: High-level metrics suitable for cards and executive summaries.
- `series`: Chart-ready time series or distributions.
- `tables`: Report-ready tabular data.
- `charts`: Optional chart template references and rendering metadata.
- `metadata`: Parser format, runtime, time zone, intervals, and other context.

## Contract Hardening Scope

The first contract-hardening pass is limited to the analyzer result types that already exist in code:

- `access_log`
- `profiler_collapsed`
- `gc_log`
- `thread_dump`
- `exception_stack`
- `nodejs_stack`
- `python_traceback`
- `go_panic`
- `dotnet_exception_iis`
- `otel_logs`
- `comparison_report`

The common `AnalysisResult` dataclass remains the outer transport model for now. The hardening layer adds type-specific contracts for the contents of `summary`, `series`, `tables`, and `metadata`.

### Included In This Scope

- Python `TypedDict` definitions for Access Log and Profiler result sections.
- Matching TypeScript interfaces for renderer and chart code.
- Required keys, value types, and units documented in this file.
- `schema_version` retained in `metadata` for future migration.

### Excluded From This Scope

- Full Pydantic model migration.
- Runtime validation for every nested field.
- Chart Studio template schema.
- Dashboard sample data as a canonical contract. `dashboard_sample` is UI fixture data only.

### Versioning Rules

- Additive optional fields may be introduced under the same `schema_version`.
- Removing or renaming required fields requires a `schema_version` bump.
- Numeric fields must use explicit units in the key name when the unit is not obvious, for example `_ms`, `_sec`, or `_percent`.
- Parser diagnostics live under `metadata.diagnostics` for parsers that support malformed-input handling.
- Portable parser debug logs are separate JSON artifacts, not nested `AnalysisResult` fields. They may include redacted raw context, `field_shapes`, partial match data, and traceback data for parser development.

## Required Result Contracts

### Access Log Result

`type`: `access_log`

Required `summary` fields:

| Field | Type | Unit / meaning |
|---|---|---|
| `total_requests` | integer | Parsed request count |
| `avg_response_ms` | number | Average response time in milliseconds |
| `p95_response_ms` | number | 95th percentile response time in milliseconds |
| `p99_response_ms` | number | 99th percentile response time in milliseconds |
| `error_rate` | number | Percent of requests with status `>= 400` |

Required `series` fields:

| Field | Row shape |
|---|---|
| `requests_per_minute` | `{ time: string, value: number }` |
| `avg_response_time_per_minute` | `{ time: string, value: number }` |
| `p95_response_time_per_minute` | `{ time: string, value: number }` |
| `status_code_distribution` | `{ status: string, count: integer }` |
| `top_urls_by_count` | `{ uri: string, count: integer }` |
| `top_urls_by_avg_response_time` | `{ uri: string, avg_response_ms: number, count: integer }` |

Required `tables` fields:

| Field | Row shape |
|---|---|
| `sample_records` | `{ timestamp: string, method: string, uri: string, status: integer, response_time_ms: number }` |

Required `metadata` fields:

| Field | Type | Meaning |
|---|---|---|
| `format` | string | Access log format selector |
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line counts and skipped-record samples |
| `analysis_options` | `AccessLogAnalysisOptions` | Applied sampling and time-range options |
| `findings` | array of `AccessLogFinding` | Report-oriented access-log observations |

`AccessLogAnalysisOptions` required fields:

| Field | Type | Meaning |
|---|---|---|
| `max_lines` | integer or null | Maximum physical lines read from the source file |
| `start_time` | string or null | Inclusive ISO 8601 lower timestamp bound |
| `end_time` | string or null | Inclusive ISO 8601 upper timestamp bound |

`AccessLogFinding` required fields:

| Field | Type | Meaning |
|---|---|---|
| `severity` | string | Finding severity such as `warning` or `critical` |
| `code` | string | Stable finding code |
| `message` | string | Human-readable finding summary |
| `evidence` | object | Small structured values supporting the finding |

### Profiler Collapsed Result

`type`: `profiler_collapsed`

Required `summary` fields:

| Field | Type | Unit / meaning |
|---|---|---|
| `profile_kind` | string | Profile type, initially `wall` |
| `total_samples` | integer | Total collapsed stack sample count |
| `interval_ms` | number | Sampling interval in milliseconds |
| `estimated_seconds` | number | Estimated sampled time in seconds |
| `elapsed_seconds` | number or null | Optional observed elapsed time in seconds |

Required `series` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null }` |
| `component_breakdown` | `{ component: string, samples: integer }` |
| `execution_breakdown` | optional `{ category, executive_label, primary_category, wait_reason, samples, estimated_seconds, total_ratio, parent_stage_ratio, elapsed_ratio, top_methods, top_stacks }` |
| `timeline_analysis` | optional `{ index, segment, label, samples, estimated_seconds, stage_ratio, total_ratio, elapsed_ratio, top_methods, method_chains, top_stacks }` |

Required `tables` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null, frames: string[] }` |
| `top_child_frames` | optional `{ frame: string, samples: integer, ratio: number }` |
| `timeline_analysis` | optional same row shape as `series.timeline_analysis` |

Optional `charts` fields:

| Field | Shape |
|---|---|
| `flamegraph` | `FlameNode` |
| `drilldown_stages` | array of drill-down stage objects |

`FlameNode` shape:

```text
{
  id: string,
  parentId: string | null,
  name: string,
  samples: integer,
  ratio: number,
  category: string | null,
  color: string | null,
  children: FlameNode[],
  path: string[]
}
```

Execution breakdown categories:

```text
SQL_DATABASE
EXTERNAL_API_HTTP
NETWORK_IO_WAIT
APPLICATION_LOGIC
FRAMEWORK_MIDDLEWARE
LOCK_SYNCHRONIZATION_WAIT
CONNECTION_POOL_WAIT
FILE_IO
GC_JVM_RUNTIME
IDLE_BACKGROUND
UNKNOWN
```

Timeline analysis segments:

```text
STARTUP_FRAMEWORK
INTERNAL_METHOD
SQL_EXECUTION
DB_NETWORK_WAIT
EXTERNAL_CALL
EXTERNAL_NETWORK_WAIT
CONNECTION_POOL_WAIT
LOCK_SYNCHRONIZATION_WAIT
NETWORK_IO_WAIT
FILE_IO
JVM_GC_RUNTIME
UNKNOWN
```

`timeline_analysis` is a flamegraph sample composition view. It converts
samples to estimated seconds with the configured sampling interval, but it is
not a timestamp-ordered trace.

Required `metadata` fields:

| Field | Type | Meaning |
|---|---|---|
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line counts and skipped-record samples |

### ParserDiagnostics

Required fields:

| Field | Type | Meaning |
|---|---|---|
| `total_lines` | integer | Physical lines read from the source file |
| `parsed_records` | integer | Valid non-blank records accepted by the parser |
| `skipped_lines` | integer | Malformed non-blank records skipped by the parser |
| `skipped_by_reason` | object mapping string to integer | Skipped record counts by reason code |
| `samples` | array of `DiagnosticSample` | Bounded examples of skipped records |

`DiagnosticSample` required fields:

| Field | Type | Meaning |
|---|---|---|
| `line_number` | integer | 1-based source line number |
| `reason` | string | Stable reason code, such as `NO_FORMAT_MATCH` |
| `message` | string | Human-readable parser message |
| `raw_preview` | string | Truncated raw input preview, currently capped at 200 characters |

### Multi-runtime Stack Results

`nodejs_stack`, `python_traceback`, and `go_panic` use the same stack-oriented result shape.

Required `summary` fields:

| Field | Type | Meaning |
|---|---|---|
| `total_records` | integer | Parsed stack or runtime blocks |
| `unique_record_types` | integer | Distinct error/panic/goroutine types |
| `unique_signatures` | integer | Distinct normalized stack signatures |
| `top_record_type` | string or null | Most frequent record type |

Required `series` fields:

| Field | Row shape |
|---|---|
| `record_type_distribution` | `{ record_type: string, count: integer }` |
| `top_stack_signatures` | `{ signature: string, count: integer }` |

Required `tables` fields:

| Field | Row shape |
|---|---|
| `records` | `{ runtime, record_type, headline, message, signature, top_frame, stack }` |

`dotnet_exception_iis` extends the same stack series/tables with IIS access summaries:

| Field | Location | Meaning |
|---|---|---|
| `iis_requests` | `summary` | Parsed IIS W3C request count |
| `iis_error_requests` | `summary` | IIS requests with status `>= 500` |
| `max_iis_time_taken_ms` | `summary` | Maximum `time-taken` value |
| `iis_status_distribution` | `series` | Status-class distribution |
| `iis_slowest_urls` | `series` | Slowest IIS URI rows |

### OpenTelemetry Logs Result

`type`: `otel_logs`

Required `summary` fields:

| Field | Type | Meaning |
|---|---|---|
| `total_records` | integer | Parsed OTel JSONL log records |
| `unique_traces` | integer | Distinct non-empty trace IDs |
| `unique_services` | integer | Distinct non-empty service names |
| `cross_service_traces` | integer | Traces seen in more than one service |
| `failed_traces` | integer | Traces that contain an error signal |
| `failure_propagation_traces` | integer | Failed traces with downstream service activity after the first failure |
| `error_records` | integer | Records with error-level severity |

Required `series` fields:

| Field | Row shape |
|---|---|
| `severity_distribution` | `{ severity: string, count: integer }` |
| `service_distribution` | `{ service: string, count: integer }` |
| `top_traces` | `{ trace_id: string, count: integer }` |

Required `tables` fields:

| Field | Row shape |
|---|---|
| `records` | `{ timestamp, trace_id, span_id, service_name, severity, body }` |
| `cross_service_traces` | `{ trace_id: string, services: string[] }` |
| `trace_service_paths` | `{ trace_id, service_path, service_count, record_count, has_error, max_severity }` |
| `trace_failures` | `{ trace_id, services, first_failing_service, error_services, error_count, first_error }` |
| `service_failure_propagation` | `{ trace_id, service_path, first_failing_service, downstream_services, downstream_service_count, first_error }` |
| `trace_span_topology` | `{ trace_id, span_id, parent_span_id, service, child_count, has_error }` |
| `service_trace_matrix` | `{ trace_id, service, record_count }` |

### Comparison Report Result

`type`: `comparison_report`

`comparison_report` compares two existing `AnalysisResult` JSON files without
re-parsing raw evidence.

Required `summary` fields:

| Field | Type | Meaning |
|---|---|---|
| `before_type` | string | Before result type |
| `after_type` | string | After result type |
| `changed_metrics` | integer | Numeric summary fields with non-zero delta |
| `before_findings` | integer | Finding count in before result |
| `after_findings` | integer | Finding count in after result |
| `finding_delta` | integer | After finding count minus before finding count |

Required `series` fields:

| Field | Row shape |
|---|---|
| `summary_metric_deltas` | `{ metric, before, after, delta, change_percent }` |
| `finding_count_comparison` | `{ side, finding_count }` |

## AccessLogRecord

```text
timestamp
method
uri
status
response_time_ms
bytes_sent
client_ip
user_agent
referer
raw_line
```

## GcEvent

```text
timestamp
uptime_sec
gc_type
cause
pause_ms
heap_before_mb
heap_after_mb
heap_committed_mb
young_before_mb
young_after_mb
old_before_mb
old_after_mb
metaspace_before_mb
metaspace_after_mb
raw_line
```

## JVM Analyzer Result Contracts

JVM analyzer MVPs use additive `AnalysisResult` contracts with `metadata.schema_version = "0.1.0"` and parser diagnostics under `metadata.diagnostics`.

### GC Log Result

`type`: `gc_log`

Required `summary` fields:

| Field | Type | Unit / meaning |
|---|---|---|
| `total_events` | number | parsed GC events |
| `total_pause_ms` | number | total GC pause time |
| `avg_pause_ms` | number | average parsed pause |
| `max_pause_ms` | number | maximum parsed pause |
| `young_gc_count` | number | young GC events |
| `full_gc_count` | number | full GC events |

Required series keys: `pause_timeline`, `heap_after_mb`, `gc_type_breakdown`, `cause_breakdown`.

### Thread Dump Result

`type`: `thread_dump`

Required `summary` fields: `total_threads`, `runnable_threads`, `blocked_threads`, `waiting_threads`, `threads_with_locks`.

Required series keys: `state_distribution`, `category_distribution`, `top_stack_signatures`.

### Exception Stack Result

`type`: `exception_stack`

Required `summary` fields: `total_exceptions`, `unique_exception_types`, `unique_signatures`, `top_exception_type`.

Required series keys: `exception_type_distribution`, `root_cause_distribution`, `top_stack_signatures`.

## ProfileStack

```text
stack
frames
samples
estimated_seconds
sample_ratio
elapsed_ratio
category
```

## ThreadDumpRecord

```text
thread_name
thread_id
state
stack
lock_info
category
raw_block
```

## ExceptionRecord

```text
timestamp
language
exception_type
message
root_cause
stack
signature
raw_block
```

## AI Interpretation Contract

AI interpretation does not replace `AnalysisResult`. It is a separate `InterpretationResult` linked back to a source result.

```text
InterpretationResult
  schema_version
  provider
  model
  prompt_version
  source_result_type
  source_schema_version
  generated_at
  findings
  disabled
```

`AiFinding` required fields:

| Field | Type | Meaning |
|---|---|---|
| `id` | string | Stable finding id |
| `label` | string | Short title |
| `severity` | string | `info`, `warning`, or `critical` |
| `generated_by` | string | Must be `ai` |
| `model` | string | Local model name |
| `summary` | string | User-facing interpretation |
| `reasoning` | string | Short reasoning bound to evidence |
| `evidence_refs` | array of string | Non-empty canonical evidence references |
| `evidence_quotes` | object | Optional exact evidence substrings keyed by `evidence_ref` |
| `confidence` | number | Model confidence from `0` to `1`; initial display threshold is `0.3` |
| `limitations` | array of string | Missing evidence or uncertainty |

Canonical `evidence_ref` grammar:

```text
{source_type}:{entity_type}:{identifier}
```

Registered namespaces:

| Source | Entities |
|---|---|
| `access_log` | `record`, `finding` |
| `profiler` | `stack`, `frame`, `finding` |
| `jfr` | `event` |
| `otel` | `log`, `span`, `event` |
| `timeline` | `event`, `correlation` |

AI output must pass runtime validation before it is shown. Validation includes non-empty references, grammar and namespace checks, reference presence in the source evidence registry, confidence threshold, and quote-to-source matching when quotes are provided.

## Design Rules

- Parsers preserve raw evidence where practical through `raw_line` or `raw_block`.
- Analyzers produce numeric fields with explicit units.
- Chart inputs come from `series` and `tables`, not parser-specific objects.
- Runtime-specific fields should live under `metadata` unless they are broadly reusable.
- Analyzer sampling and filter settings should be echoed under `metadata.analysis_options`.
- Report-grade interpretations should be expressed as bounded structured findings, not prose-only blobs.
- AI-assisted interpretations must include non-empty `evidence_refs` that point to existing raw evidence such as `raw_line`, `raw_block`, `raw_preview`, or `evidence_ref` rows.

## Schema 0.2.0 — additive changes

`schema_version` was bumped from `0.1.0` to `0.2.0` after the
post-`0.2.0-beta` analyzer overhauls. All existing 0.1.0 fields remain
present and required; the changes below are additive.

### Access log (`type: "access_log"`)

New `summary` fields (all optional for forward compatibility but
emitted by the current analyzer):

| Field | Type | Meaning |
| --- | --- | --- |
| `p50_response_ms` / `p90_response_ms` | number | Additional response-time percentiles |
| `total_bytes` | integer | Sum of response sizes |
| `avg_requests_per_second` | number | Throughput (req/s) over the analyzed window |
| `avg_throughput_bps` | number | Throughput in bytes/s |
| `static_request_count` / `api_request_count` | integer | Static-vs-API split by extension and well-known asset paths |

New `series` rows:

- `percentile_response_time_per_minute` — `{ time, p50, p90, p95, p99 }`
- `throughput_per_minute` — `{ time, requests_per_second, bytes_per_second }`
- `status_class_per_minute` — `{ time, c2xx, c3xx, c4xx, c5xx, error_rate }`

New `tables` rows:

- `url_stats` — `{ method, uri, classification, count, avg_ms, p95_ms, total_bytes, error_count, status_mix: {c2xx, c3xx, c4xx, c5xx} }`
- `top_status_codes` — `{ status: integer, count: integer }`

New findings: `SLOW_URL_P95` (warning, p95 ≥ 1 s with ≥ 5 samples) and
`ERROR_BURST_DETECTED` (warning, single-minute error rate ≥ 50 % with
≥ 5 requests).

### GC log (`type: "gc_log"`)

New `metadata.jvm_info` object (extracted by `gc_log_header.py`):

```text
metadata.jvm_info = {
  version: string | null,
  cpus_total: integer | null,
  cpus_available: integer | null,
  memory_mb: integer | null,
  heap_min_mb / heap_initial_mb / heap_max_mb / heap_region_mb: number | null,
  parallel_workers / concurrent_workers / refinement_workers: integer | null,
  compressed_oops: boolean | null,
  numa: boolean | null,
  pre_touch: boolean | null,
  periodic_gc: boolean | null,
  command_line_flags: string[],     # one flag per element
  raw_header_lines: string[],       # bounded copy
  worker_cpu_mismatch: boolean
}
```

New optional `series` rows: `young_heap_before_per_event`,
`young_heap_after_per_event`, `old_heap_before_per_event`,
`old_heap_after_per_event`, `metaspace_before_per_event`,
`metaspace_after_per_event`. The existing `heap_before_per_event` /
`heap_after_per_event` / `heap_committed_per_event` rows are unchanged.

### Profiler (`type: "profiler_collapsed"` and friends)

- `charts.flamegraph_root` — `FlameNode` may carry an optional
  `metadata: { a, b, delta }` populated by `profiler_diff` (samples in
  baseline, target, and signed delta). UIs without diff awareness can
  ignore the field.
- `metadata.threads` — array of `{ name, samples, ratio }` populated
  when a `[Thread]` prefix carries ≥ 80 % of all samples (async-profiler
  `-t` collapsed). Used by the **Filter by thread** dropdown.
- New analyzer types: `profiler_diff` (returns a flame tree with the
  `{a, b, delta}` metadata plus `tables.gainers` / `tables.losers`)
  and `profiler_export_pprof` (returns `{ outputPath, sizeBytes }`
  pointing at the gzipped pprof file under `~/.archscope/uploads/`).

### JFR (`type: "jfr_recording"`)

- `metadata.event_modes`, `metadata.time_from`, `metadata.time_to`,
  `metadata.thread_state`, `metadata.min_duration_ms` — analyzer
  options echoed in metadata so the UI can render the active filters.
- `series.thread_density_per_bucket` — wall-clock heatmap data
  `{ time, count }`.
- New optional sub-result `tables.native_memory_leaks` —
  `{ stack_signature, alloc_count, alloc_bytes, free_bytes, tail_ratio }`
  produced by `native_memory_analyzer.py` when nativemem events are
  present.

### Thread dump multi (`type: "thread_dump_multi"`)

- `metadata.findings` may now include any of the codes listed under
  the Phase 6 roadmap section: `LONG_RUNNING_THREAD`,
  `PERSISTENT_BLOCKED_THREAD`, `LATENCY_SECTION_DETECTED`,
  `GROWING_LOCK_CONTENTION`, `THREAD_CONGESTION_DETECTED`,
  `EXTERNAL_RESOURCE_WAIT_HIGH`, `LIKELY_GC_PAUSE_DETECTED`,
  `VIRTUAL_THREAD_CARRIER_PINNING`, `SMR_UNRESOLVED_THREAD`.
- New `tables.lock_graph` — `{ lock_addr, owner, waiters: string[] }`.
- New `tables.deadlock_cycles` — `{ cycle: string[] }` (DFS over the
  lock graph).
- New `tables.dump_overview` — one row per dump with thread counts,
  state distribution, and the resolved parser plugin id.
