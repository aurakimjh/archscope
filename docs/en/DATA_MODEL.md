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

## Shared Ingestion Metadata

New Mid-Term Plus evidence families should attach normalized source metadata
under `metadata.source_metadata` when a result is produced from local files.
Existing analyzers may adopt this additively without changing their summary,
series, or table contracts.

`SourceMetadata` shape:

| Field | Type | Meaning |
|---|---|---|
| `source_kind` | string | Evidence source family, such as `access_log`, `trace`, `server_log`, `database_log`, or `broker_log` |
| `source_format` | string | Detected or selected parser format |
| `product` | string | Product or ecosystem name, such as nginx, OpenTelemetry, PostgreSQL, or Kafka |
| `product_version` | string | Optional product version when the source exposes it |
| `host` | string | Optional host identity when available and safe to expose |
| `service` | string | Optional service/workload name |
| `environment` | string | Optional environment label |
| `file` | object | Sanitized file identity containing basename, extension, size, and a non-path `sanitized_id` |

Cross-source stitching should use `metadata.correlation_keys` when an analyzer
can derive them without expanding result size unboundedly. The canonical key
model includes:

```text
trace_id
span_id
parent_span_id
request_id
tenant_id
customer_id
container_id
pod_uid
host_id
pid
timestamp_window
stable_id
```

Tenant and customer identifiers must be sanitized or explicitly allow-listed
before they are stored. `stable_id` is a derived hash over the non-empty
normalized fields and is safe for matching without exposing raw path data.

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

Optional access/edge extensions are additive. Existing consumers can ignore
them and still rely on the required access-log fields above.

Optional `summary` fields:

| Field | Type | Unit / meaning |
|---|---|---|
| `detected_format_count` | integer | Number of source formats observed in parsed records |
| `upstream_service_count` | integer | Number of upstream services or clusters observed |
| `route_count` | integer | Number of gateway or mesh routes observed |
| `service_edge_count` | integer | Number of inferred caller-to-upstream edges |
| `gateway_avg_latency_ms` | number | Average gateway latency in milliseconds |
| `gateway_p95_latency_ms` | number | 95th percentile gateway latency in milliseconds |
| `retry_count` | integer | Sum of retry attempts reported by edge logs |
| `termination_error_count` | integer | HAProxy termination-state rows treated as abnormal |

Optional `series` fields:

| Field | Row shape |
|---|---|
| `source_format_distribution` | `{ source_format: string, count: integer }` |
| `upstream_service_distribution` | `{ upstream_service: string, count: integer }` |

Optional `tables` fields:

| Field | Row shape |
|---|---|
| `service_dependencies` | `{ caller: string, callee: string, call_count: integer, total_duration_ms: number, avg_duration_ms: number, max_duration_ms: number, error_count: integer, error_rate: number, retry_count: integer, routes: string[], source_formats: string[] }` |
| `route_stats` | `{ route: string, count: integer, avg_response_ms: number, max_response_ms: number, error_count: integer, error_rate: number, source_formats: string[] }` |

`tables.service_dependencies.error_rate` is a fraction from `0.0` to `1.0`,
matching Trace Import service-dependency tables. `tables.route_stats.error_rate`
is a percent value like the access-log summary error rate. `sample_records` may
also include optional `source_format`, `route`, `upstream_service`, `trace_id`,
and `request_id` fields.

Optional `metadata.source_format_diagnostics` records access/edge auto-detect
evidence with `selected_format`, `auto_detect_enabled`, `detected_format_count`,
`parsed_by_format`, and `skipped_by_reason`.

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

### Server Log Result

`type`: `server_log`

The server-log contract covers application-server and web-server error evidence
from Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere, GlassFish/Payara, nginx
error logs, and Apache error logs.

Core `summary` fields:

| Field | Type | Meaning |
|---|---|---|
| `total_events` | integer | Parsed server-log event count |
| `error_count` | integer | Error/severe/fatal record count |
| `warning_count` | integer | Warning record count |
| `startup_count` | integer | Startup/lifecycle event count |
| `deployment_event_count` | integer | Deployment-related event count |
| `datasource_event_count` | integer | JDBC/datasource/pool event count |
| `stuck_thread_count` | integer | Stuck-thread event count |
| `hung_thread_count` | integer | Hung-thread event count |
| `thread_pool_pressure_count` | integer | Executor/thread-pool pressure count |
| `worker_error_count` | integer | nginx/Apache worker/upstream error count |
| `correlated_event_count` | integer | Events with trace or request IDs |

Core `tables` fields:

| Field | Row shape |
|---|---|
| `events` | `{ timestamp, severity, product, component, thread, host, service_name, event_type, message, trace_id, request_id }` |
| `deployment_events` | Same row shape as `events`, filtered to deployment evidence |
| `datasource_events` | Same row shape as `events`, filtered to datasource/pool evidence |
| `thread_events` | Same row shape as `events`, filtered to stuck/hung/thread-pool evidence |
| `worker_errors` | Same row shape as `events`, filtered to nginx/Apache worker evidence |
| `correlation_candidates` | Events carrying trace or request IDs |

Finding codes include `SERVER_SEVERE_ERRORS`, `DEPLOYMENT_FAILURE`,
`DATASOURCE_POOL_WARNING`, `STUCK_THREAD_DETECTED`, `HUNG_THREAD_DETECTED`,
`THREAD_POOL_PRESSURE`, `WORKER_ERROR_PRESENT`, and `MANAGED_SERVER_HEALTH`.

### Observability Logs And Metrics

`otel_logs` accepts JSONL/NDJSON-style OTel log records and OTLP Logs JSON
`resourceLogs`. It preserves severity, body, attributes, resource metadata,
service name, trace ID, span ID, and parent span ID. Analyzer tables include
records, cross-service traces, trace service paths, failure propagation,
resource groups, error signatures, and severity bursts.

`metrics_snapshot` accepts Prometheus/OpenMetrics text snapshots. It emits
metric sample counts, per-metric distribution, bounded raw samples, and
`golden_signal_candidates` for latency, traffic, errors, and saturation.

`observability_evidence` accepts Loki query JSON exports, Tempo trace JSON
exports, and Grafana dashboard JSON exports. Loki and Tempo records can join the
Incident Timeline through trace IDs; Grafana rows are stored as dashboard-panel
references for Evidence Board and report-pack context rather than raw metric
truth.

### Database Slow Query Result

`type`: `database_slow_query`

The database evidence contract covers PostgreSQL text/csvlog, MySQL/MariaDB
slow query logs, MongoDB profiler JSON, Redis slowlog text, SQL Server extended
events JSON, and PostgreSQL/MySQL EXPLAIN JSON. Core rows preserve sanitized SQL
fingerprints, duration, lock wait, error, row count, database/schema, collection
or operation, and plan summaries. `tables.service_dependencies` exposes
application-to-database edges so Service Flow can place database evidence beside
trace and access-log dependencies.

Finding codes include `SLOW_QUERY_PRESENT`, `LOCK_WAIT_PRESENT`,
`DB_ERRORS_PRESENT`, `HIGH_ROWS_EXAMINED`, and `EXPLAIN_PLAN_IMPORTED`.

### Broker Log Result

`type`: `broker_log`

The broker evidence contract covers Kafka, RabbitMQ diagnostics/server logs,
Pulsar, NATS, and ActiveMQ. Rows normalize rebalance, replication/ISR,
KRaft quorum, queue pressure, dead-letter, partition, slow-consumer,
authorization, store usage, and broker-health events. `tables.service_dependencies`
projects application-to-broker edges for Service Flow.

Finding codes include `BROKER_REBALANCE`, `BROKER_REPLICATION_ISSUE`,
`BROKER_QUEUE_PRESSURE`, `BROKER_DEAD_LETTER`, `BROKER_HEALTH_EVENT`,
`BROKER_SLOW_CONSUMER`, and `BROKER_AUTHORIZATION_FAILURE`.

### Kubernetes, Container, And Cloud Audit Evidence

`type`: `kubernetes_evidence`

The platform evidence contract preserves cluster, namespace, workload, pod,
container, node, image, restart count, owner-style object identity, kubelet and
container-runtime events, and cloud audit actor/operation/resource fields.
Supported inputs include `kubectl get events -o json`, pod JSON, kubelet logs,
containerd/CRI-O/Docker daemon logs, AWS CloudTrail JSON, GCP Cloud Audit Logging
JSON, and Azure Activity Logs JSON.

Finding codes include `K8S_OOMKILLED`, `K8S_RESTARTS_PRESENT`, `K8S_EVICTION`,
`K8S_SCHEDULING_ISSUE`, `K8S_IMAGE_PULL_ISSUE`, `K8S_READINESS_ISSUE`,
`K8S_NODE_PRESSURE`, and `CLOUD_AUDIT_SECURITY_EVENT`.

### Unified Profile Evidence

`type`: `profile_evidence`

The unified profile evidence contract normalizes runtime profiler inputs into
language-tagged stack samples before reusing the existing flamegraph analyzer.
Supported selectors include generic pprof `.pb.gz`, async-profiler collapsed and
HTML, py-spy raw, rbspy raw, speedscope JSON including dotnet-trace speedscope
exports, perf collapsed stacks, JFR JSON stack samples, Ruby StackProf JSON,
PHP Excimer/Tideways-style JSON, Xdebug cachegrind text, Swift/generic async
stacks, Pyroscope/Phlare snapshots, and Parca-style profile JSON.

Core `summary` fields include:

| Field | Type | Meaning |
|---|---|---|
| `total_samples` | integer | Sum of normalized sample values |
| `unique_stacks` | integer | Distinct collapsed stack keys after normalization |
| `runtime_count` | integer | Distinct runtime labels |
| `language_count` | integer | Distinct language labels |
| `native_samples` | integer | Samples whose stack includes native frames |
| `managed_samples` | integer | Samples whose stack includes managed frames |
| `async_frame_samples` | integer | Samples whose stack includes async/await/continuation frames |
| `thread_count` | integer | Distinct thread labels when present |
| `process_count` | integer | Distinct process/PID labels when present |
| `max_stack_depth` | integer | Maximum normalized stack depth |

`tables.frames` rows preserve `name`, `function`, `file`, `line`, `language`,
`runtime`, `kind`, `native`, `async`, and `samples`. `charts.flamegraph` and
`charts.drilldown_stages` reuse the profiler `FlameNode` contract.

`metadata.unified_profile_schema` records the canonical frame and sample fields,
while `metadata.flamegraph_rollup` records the collapsed-stack rollup source.

### Stitched Evidence

`type`: `stitched_evidence`

Stitched evidence reads existing `AnalysisResult` JSON files and joins rows by
correlation keys: trace ID, span ID, parent span ID, request/correlation ID,
TXID/GUID, tenant/customer ID, pod/container/host, and PID.

Core tables:

| Field | Row shape |
|---|---|
| `matches` | `{ key_kind, key_value, event_count, source_types, evidence_refs, first_seen, last_seen, services }` |
| `gaps` | `{ code, severity, message, source_type, evidence_ref, timestamp, service, correlation }` |
| `evidence_nodes` | Normalized source rows with source type, table, timestamp, service, target, message, evidence ref, and correlation keys |
| `service_dependencies` | Stitched service edges for Service Flow when database/broker evidence matches request or trace evidence |

Gap finding codes include `MISSING_TRACE_ID`, `DROPPED_PARENT_SPAN`,
`UNMATCHED_REQUEST_LOG`, `UNMATCHED_DATABASE_CALL`, and
`UNMATCHED_BROKER_EVENT`.

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

## Trace Import Result

`type`: `trace_import`

Supported local file formats:

- `otlp-json`
- `zipkin-v2-json`
- `elastic-apm-search-json`
- `elastic-apm-source-ndjson`
- `jaeger-query-json`
- `skywalking-graphql-json`

`jaeger-query-json` accepts Jaeger QueryService-style JSON envelopes with
`data[].spans` plus `processes`, or a local trace object with the same shape.
`skywalking-graphql-json` is schema-guarded: the current importer accepts
SkyWalking GraphQL responses with `data.queryTrace.spans` or
`data.trace.spans`; other SkyWalking response shapes produce parser diagnostics
instead of silently guessing.

Required `summary` fields:

| Field | Type | Meaning |
|---|---|---|
| `source_format` | string | Detected or requested import format |
| `total_spans` | integer | Parsed canonical spans |
| `unique_traces` | integer | Unique trace ids |
| `unique_services` | integer | Non-unknown services |
| `unique_dependencies` | integer | Caller-to-callee service edges |
| `error_spans` | integer | Spans marked as errors |
| `missing_parent_spans` | integer | Spans whose parent was absent from the import |
| `clock_skew_suspected_spans` | integer | Child spans outside the parent time window |
| `p95_trace_duration_ms` | number | Trace-duration p95 |
| `max_trace_duration_ms` | number | Longest trace duration |
| `unbalanced_service_count` | integer | Services dominating total imported span latency |
| `high_error_service_edges` | integer | Service edges above the error-rate threshold |

Primary `tables` rows:

- `spans` — `{ trace_id, span_id, parent_span_id, name, service_name, remote_service, kind, start_unix_nanos, duration_ms, status_code, error, source_format }`
- `traces` — `{ trace_id, span_count, service_count, services, duration_ms, total_span_ms, error_count, slowest_span, slowest_span_ms }`
- `service_dependencies` — `{ caller, callee, call_count, total_duration_ms, avg_duration_ms, error_count, error_rate }`
- `service_summary` — `{ service, span_count, total_duration_ms, avg_duration_ms, error_count }`
- `critical_paths` — `{ trace_id, critical_path_ms, span_count, services, span_names }`

Current deterministic finding codes include `SLOW_TRACE_P95`,
`TRACE_IMPORT_ERROR_SPANS`, `TRACE_IMPORT_MISSING_PARENT`,
`CLOCK_SKEW_SUSPECTED`, `UNBALANCED_SERVICE_LATENCY`,
`HIGH_ERROR_SERVICE_EDGE`, `TRACE_IMPORT_CROSS_SERVICE_TRACE`, and
`TRACE_IMPORT_SLOW_SPAN_DOMINATES_TRACE`.

## Evidence Board Card

The first Evidence Board implementation is local UI state, not a persisted
engine result. Card fields are intentionally generic so analyzer findings,
chart selections, table rows, parser diagnostics, and source metadata can share
one report pipeline later.

```text
id
created_at
source_kind
analyzer
title
severity
source_file
source_ref
payload
comment
hypothesis
impact
recommendation
```

## Incident Timeline Event

The first Incident Timeline implementation is a Wails session projection built
from Analysis Workspace results. It does not replace analyzer-owned
`AnalysisResult` output; it normalizes existing evidence into a timeline view.

```text
id
timestamp
start_time
end_time?
time_label
range_label
duration_ms?
source_analyzer
source_result_id
source_title
source_file
severity
category
group_key
group_label
group_category
correlation_ids
label
description
evidence_ref
payload
```

Current event sources include analyzer findings, access-log error/latency
series, GC alerts, JFR pause/notable events, exception rows, thread-dump
contention/deadlock tables, and trace-import error/critical-path rows.
Timeline groups are derived from correlation IDs first (`trace_id`,
`request_id`, `correlation_id`, `transaction_id`, `thread_id`, thread), then
service/endpoint hints, and finally category/source fallback keys. The
exportable result includes `tables.groups`, grouped event counts, ranged event
counts, and correlated event counts so multi-file incidents can be reviewed by
incident slice rather than only as a flat event list.
`tables.narrative` contains deterministic incident narrative steps derived from
those groups. Each step includes order, group key/label, severity, summary,
event IDs, source result IDs, and evidence refs; it is not AI-generated prose.

For report packs, the same projection can be emitted as an exportable
`AnalysisResult` with `type = "incident_timeline"`. It preserves source files,
summary counts, severity/category/source distributions, `tables.events`, and
`metadata.source_results` so the report artifact can persist the session
timeline without replacing analyzer-owned results.

## SLO and Golden Signals Projection

The first SLO / Golden Signals implementation is a Wails session projection
built from Analysis Workspace results. It does not replace analyzer-owned
`AnalysisResult` output; it normalizes existing metrics into signal, SLI, and
SLO views.

`GoldenSignal` fields:

```text
id
kind                  latency | traffic | errors | saturation
name
value
unit
aggregation
scope_type            global | service | endpoint | edge | runtime | thread | trace | jvm
scope
source_analyzer
source_result_id
source_title
source_file
evidence_ref
payload
tags
```

`SliMetric` groups compatible Golden Signals by kind, name, unit, aggregation,
scope type, and scope:

```text
id
metric_key
kind
name
value
unit
aggregation
scope_type
scope
source_analyzers
source_result_ids
evidence_refs
signal_ids
contributor_count
tags
```

`SloViolation` fields:

```text
id
target_id
target_name
severity
metric_id
metric_key
metric_name
kind
actual
threshold
comparator
unit
window_label
delta
ratio_to_threshold
burn_rate
error_budget_consumed_percent
error_budget_remaining_percent
affected_scope_type
affected_scope
source_analyzers
source_result_ids
evidence_refs
tags
```

Current Golden Signal sources include access-log latency/throughput/error
metrics, access edge service-dependency latency/traffic/error metrics,
trace-import service/dependency/critical-path metrics, Jennifer MSA external-call
and network-gap metrics, exception distributions, GC pause, memory-space, OOM
and long-pause alerts, JFR pause/sample/thread metrics, thread-dump
lock/contention/deadlock metrics, and JVM metadata such as heap and worker
counts.

## Service Flow Projection

The first Service Flow implementation is a Wails session projection built from
Analysis Workspace results. It unifies Trace Import `service_dependencies`,
Access Log `tables.service_dependencies`, Jennifer `tables.msa_edges`, and
Jennifer unprofiled external-call groups into a common service-edge view.

`ServiceFlowInputEdge` fields:

```text
id
source_type            trace_import_dependency | access_edge_dependency | jennifer_msa_edge | jennifer_unprofiled_external_call_group
source_analyzer
source_result_id
source_title
source_file
caller
callee
call_count
total_latency_ms
avg_latency_ms
max_latency_ms
error_count
error_rate
raw_network_gap_ms
adjusted_network_gap_ms
network_gap_ms
match_status
guid
trace_id
external_call_url
evidence_ref
payload
```

`ServiceEdge` groups compatible input edges by caller/callee:

```text
id
caller
callee
call_count
total_latency_ms
avg_latency_ms
max_latency_ms
error_count
error_rate
network_gap_sample_count
total_network_gap_ms
avg_network_gap_ms
max_network_gap_ms
matched_call_count
unmatched_call_count
source_types
source_analyzers
source_result_ids
source_files
evidence_refs
input_edge_ids
```

Current deterministic service-flow finding codes are
`SERVICE_FLOW_UNMATCHED_CALLS`, `SERVICE_FLOW_HIGH_NETWORK_GAP`, and
`SERVICE_FLOW_MISSING_PARENT`. The Wails Service Flow page can export a
Mermaid sequence-like view (`.mmd`) and a JSON `archscope_service_flow` payload.

## Report Pack

The Wails Evidence Board can emit a UI-level report pack before the full engine
report-pack exporter exists.

```text
ReportPack
  type = archscope_report_pack
  schema_version
  created_at
  card_count
  source_result_count
  customer_summary
  provenance
  artifacts
```

`customer_summary` is a customer-facing overview and key-observation list. Each
observation carries evidence card IDs or source evidence references, and the raw
evidence appendix remains in the pack. `provenance` preserves source result
metadata, analyzer options, captured evidence cards, deterministic findings,
derived artifact references, and optional AI interpretation provenance when it
exists. `artifacts` currently include evidence cards, the exportable
`incident_timeline` result, SLO analysis, and the Service Flow export payload.
AI interpretation provenance records source gate status and accepted AI
findings separately; accepted AI findings are included only when the full
interpretation passes the evidence gate.

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
| `trace_import` | `span`, `trace`, `service_edge`, `finding` |
| `evidence_board` | `card` |
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

### Jennifer profile (`type: "jennifer_profile"`)

MSA timeline matching still emits matched caller-to-callee edges in
`tables.msa_edges`. External calls without a callee profile are now
kept as first-class unprofiled external calls instead of being folded
into residual method time.

New `summary` field:

| Field | Type | Meaning |
| --- | --- | --- |
| `total_unprofiled_external_call_ms` | integer | Sum of `EXTERNAL_CALL` elapsed time for unmatched / unprofiled downstream calls |

New `tables` row:

- `unprofiled_external_call_groups` — grouped by
  `{ guid, caller_application, target, protocol, client, match_status }` and
  emitted as `{ count, total_elapsed_ms, avg_elapsed_ms, max_elapsed_ms,
  caller_txids, external_call_urls }`. `target` is normalized by dropping query
  strings/fragments and folding obvious numeric/UUID path IDs to `{id}`.

New `series` row:

- `service_call_network_summary` — matched MSA calls grouped by
  `{ caller_application, callee_application }`. Each row includes
  `call_count`, `guid_count`, `external_call_elapsed_ms`,
  `callee_response_time_ms`, `network_gap_ms`, `avg_network_gap_ms`,
  `p95_network_gap_ms`, `max_network_gap_ms`, `total_network_gap_ms`,
  and a stable `network_time_group` / `network_time_group_label`.
  Network groups use average adjusted network gap bands:
  `0-4 ms`, `5-9 ms`, `10-19 ms`, `20-49 ms`, `50-99 ms`, `>=100 ms`.
  The UI uses these groups to separate topology nodes by estimated
  service/network distance.

Updated `series.guid_groups[].metrics.response_time_breakdown`:

- `unprofiled_external_call_ms` is subtracted before `method_time_ms`.
- `network_call_ms` remains the matched MSA network gap
  (`EXTERNAL_CALL elapsed - callee response time`).
- `method_time_ms` is now only the remaining uncategorized application time,
  not a bucket for known external calls whose callee profile was missing.

Event-category option timing:

- METHOD rows promoted by `EventCategoryPatterns` into SQL, Check Query, 2PC,
  Fetch, or Connection Acquire use exclusive elapsed within the same ledger
  bucket. If a promoted 2PC wrapper contains other promoted 2PC methods, the
  child intervals are subtracted from the parent before summing so nested
  methods are not double-counted.
- `EXTERNAL_CALL` remains raw cumulative elapsed because MSA matching,
  unprofiled-call accounting, and parallelism analysis depend on the
  caller-reported wait time.

### JFR (`type: "jfr_recording"`)

- `summary.sample_event_count`, `summary.stack_sample_count`, and
  `summary.unique_sample_stacks` describe how many filtered JFR events can
  participate in async-profiler-style stack aggregation.
- `series.events_over_time`, `series.pause_events`, `series.events_by_type`,
  `series.heatmap_strip`, and `series.sample_events_by_type` remain chart-ready
  event summaries.
- `tables.notable_events` keeps long-duration or otherwise notable event rows.
- `tables.top_methods`, `tables.top_packages`, `tables.top_threads`, and
  `tables.sample_stacks` aggregate `stackTrace.frames` from JFR sample events.
  These tables are report evidence, not a replacement for the full JDK
  Mission Control or `jfr view` feature set.
- `charts.async_profile_flamegraph` is a FlameNode-compatible tree derived from
  sample stacks.
- `metadata.jfr_contract` states that the JDK `jfr` CLI is used only as the
  binary `.jfr` to `jfr print --json` conversion boundary.
- `metadata.findings` may include UX hints such as `JFR_FILTER_EMPTY`,
  `JFR_NO_STACK_SAMPLES`, `JFR_ASYNC_PROFILER_STYLE_RECORDING`, and
  `JFR_SPARSE_STACK_SAMPLES`.

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
