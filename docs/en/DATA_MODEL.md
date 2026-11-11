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

The active Go/Wails contract-hardening scope covers the analyzer and derived
result types implemented in `apps/engine-native`:

- `access_log`
- `gc_log`
- `jfr_recording`
- `native_memory`
- `thread_dump`
- `thread_dump_multi`
- `thread_dump_locks`
- `exception_stack`
- `nodejs_stack`
- `python_traceback`
- `go_panic`
- `dotnet_exception_iis`
- `otel_logs`
- `metrics_snapshot`
- `observability_evidence`
- `server_log`
- `database_slow_query`
- `broker_log`
- `kubernetes_evidence`
- `trace_import`
- `profile_evidence`
- `stitched_evidence`
- `api_contract_analysis`
- `architecture_docs`
- `incident_timeline`
- `slo_golden_signals`
- `service_flow`
- `jennifer_profile`
- `profiler_collapsed`
- `profiler_jennifer`
- `comparison_report`

The Go `internal/models.AnalysisResult` struct is the outer transport model for
new analyzer work. The hardening layer adds type-specific contracts for the
contents of `summary`, `series`, `tables`, and `metadata`; frontend TypeScript
interfaces mirror the same JSON envelope.

### Included In This Scope

- Go result contracts under `internal/models` and analyzer-specific packages.
- Matching TypeScript interfaces for renderer, workspace, and chart code.
- Required keys, value types, and units documented in this file.
- `schema_version` retained in `metadata` for future migration.

### Excluded From This Scope

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
| `source_kind` | string | Evidence source family, such as `access_log`, `trace_import`, `server_log`, `database_slow_query`, or `broker_log` |
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

Browser and V8 inputs (`chrome-trace-json`, `v8-cpuprofile`) add the following
to this contract without changing the envelope:

- `Sample.Value` is microseconds (`value_unit: "microseconds"`); `samples[]`
  is authoritative and `hitCount` is a cross-check. `hitCount`-only inputs use
  an aggregate path that reports no durations.
- Samples carry a `ts_us` label (offset from profile start). Before collapsing,
  the analyzer derives `tables.cpu_sample_runs` (top-N contiguous same-stack
  runs with `start_us`/`end_us`/`duration_us`/`sample_count`/`top_frame`) and
  `series.cpu_activity` (bounded CPU-occupancy buckets, at most 1,000). Both
  are omitted for formats without `ts_us`, for downsampled inputs, and for
  `hitCount`-only inputs.
- Runs exceeding the 100 ms threshold produce the `SAMPLED_CPU_HOTSPOT`
  warning finding. The wording deliberately avoids "Long Task": these are
  sampled observation windows, not browser task boundaries.
- Oversized inputs are streamed with a 256 MiB byte guard and a 500,000-sample
  cap; when downsampling occurs `metadata.partial_result`,
  `metadata.downsampled_from_samples`, `metadata.downsampled_to_samples`, and
  the `PROFILE_DOWNSAMPLED` diagnostic record it.

### HTTP Capture Result

`type`: `http_capture`

Phase 1 imports redacted HAR files (`http-capture analyze --in file.har`) into
a bounded envelope. The parser detects the generating dialect from
`log.creator`/`log.browser` (Chrome, Firefox, Safari, Charles, Fiddler,
Proxyman, Insomnia, or `generic`), caps imported entries (default 100,000,
`HAR_ENTRY_CAP` warning on overflow), and skips invalid entries with
`INVALID_HAR` diagnostics.

| Envelope field | Content |
|---|---|
| `summary` | `total_transactions`, `error_transactions`, `unique_hosts`, `unique_endpoints`, `source_format`, `dialect` |
| `series.timeline` | Per-minute request/error/byte rows (top-N bounded) |
| `tables` | `transactions`, `endpoints` (`method path` keys), `hosts` — each top-N bounded (default 50) |
| `metadata.extra.http_capture` | `dialect`, `fidelity: "har_import"`, `redaction`, `detail_storage: "inline_phase1"` |
| `metadata.extra.capture_aggregate_snapshot` | Deterministic aggregation snapshot shared with the future live-capture path, keeping offline start-order and live completion-order projections comparable |

Failed responses raise the `HTTP_CAPTURE_ERRORS` warning finding. Phase 1
stores details inline; the versioned `CaptureSessionStore` with cursor
pagination applies from live capture onward (see
`docs/en/SYSTEM_HTTP_CAPTURE.md`).

### Stitched Evidence

`type`: `stitched_evidence`

Stitched evidence reads existing `AnalysisResult` JSON files and joins rows by
correlation keys: trace ID, span ID, parent span ID, request/correlation ID,
TXID/GUID, tenant/customer ID, pod/container/host, and PID. It can also create
lower-confidence matches from timestamp windows plus normalized service aliases,
and it links `profile_evidence` samples to traces when profile labels carry
trace metadata.

Core tables:

| Field | Row shape |
|---|---|
| `matches` | `{ key_kind, key_value, match_reason, confidence, alias_reason, time_window_seconds, event_count, source_types, evidence_refs, first_seen, last_seen, services }` |
| `gaps` | `{ code, severity, message, source_type, evidence_ref, timestamp, service, correlation }` |
| `evidence_nodes` | Normalized source rows with source type, table, timestamp, service, target, message, evidence ref, correlation keys, and the bounded raw row |
| `match_drilldowns` | Drilldown-ready match rows with confidence, reason, source node IDs, evidence refs, and raw source rows |
| `service_dependencies` | Stitched service edges for Service Flow when database/broker evidence matches request or trace evidence, including match status and match reasons |

Gap finding codes include `MISSING_TRACE_ID`, `DROPPED_PARENT_SPAN`,
`UNMATCHED_REQUEST_LOG`, `UNMATCHED_DATABASE_CALL`, and
`UNMATCHED_BROKER_EVENT`.

### API And Event Contract Analysis

`type`: `api_contract_analysis`

API contract analysis compares OpenAPI operations with access-log
`AnalysisResult` tables and AsyncAPI channels with broker evidence. It is a
second-pass analyzer: the raw logs are parsed by the access/broker importers,
then `api-contract analyze` reads the result JSON files and contract specs.

Core tables:

| Field | Row shape |
|---|---|
| `operations` | OpenAPI operation rows with observed count, matched route refs, max p95 latency, and max error rate |
| `observed_routes` | Access-log routes normalized to path templates |
| `undocumented_routes` | Observed routes that did not match OpenAPI |
| `unused_operations` | OpenAPI operations not observed in access logs |
| `slow_operations` | Observed routes above the configured latency threshold |
| `high_error_operations` | Observed routes above the configured error-rate threshold |
| `event_channels` | AsyncAPI channel rows with observed broker evidence refs |
| `undocumented_event_channels` | Broker channels not documented in AsyncAPI |
| `unused_event_channels` | AsyncAPI channels not observed in broker evidence |

Finding codes include `UNDOCUMENTED_API_ROUTE`, `UNUSED_API_OPERATION`,
`SLOW_API_OPERATION`, `HIGH_ERROR_API_OPERATION`,
`UNDOCUMENTED_EVENT_CHANNEL`, and `UNUSED_EVENT_CHANNEL`.

### Architecture Documentation Package

`type`: `architecture_docs`

Architecture documentation packages are generated from existing
`AnalysisResult` JSON evidence. The result does not try to render final prose;
it emits review-ready rows for arc42 and ADR workflows while preserving source
evidence references.

Core tables:

| Field | Row shape |
|---|---|
| `arc42_sections` | `{ section_id, title, draft_markdown, evidence_tables }` |
| `context_services` | Services/components inferred from Service Flow, stitched evidence, and contract evidence |
| `interfaces` | HTTP API operations and event channels imported from API/event contract analysis |
| `runtime_views` | Evidence-backed runtime interactions, matches, and gaps |
| `deployment_views` | Kubernetes/container/cloud deployment rows |
| `quality_requirements` | Quality scenarios derived from deterministic findings |
| `risks` | Evidence-backed architecture risks with severity, impact, mitigation, and evidence ref |
| `adr_drafts` | Draft ADR rows with decision, context, alternatives, tradeoffs, consequences, and evidence refs |

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

## Browser Audit Evidence Result

`type`: `browser_audit_evidence`

`browser import --format lighthouse-json` accepts a local Lighthouse report
JSON file. The parser preserves Lighthouse's own category and audit scores;
ArchScope does not recalculate thresholds from metric values. Requested,
final, and network-request URLs are redacted before they enter the result.

Primary projections:

- `summary` — Lighthouse version, redacted URLs, collection mode, performance
  score, available FCP/LCP/INP/CLS/TBT/Speed Index/TTI/server-response metrics,
  request/byte counts, run-warning/runtime-error state, and the explicit
  `imported_lighthouse_report` score provenance/non-recalculation disclosure.
- `series.category_scores` — report category scores.
- `series.core_metrics` — available browser metric values and report scores.
- `series.resource_type_distribution` — request and transfer-byte totals by
  resource type.
- `tables.audits` — worst scored audits first, bounded by `top_n`.
- `tables.network_requests` — largest transfers first, bounded by `top_n`.
- `tables.resource_summary` — Lighthouse resource-summary rows.

The versioned `metadata.browser_audit_contract` freezes the series/table keys,
Evidence Board source paths, bounded table sizes, score provenance, and report
export formats consumed by the desktop page. Every category, metric, audit,
network-request, and resource-summary row carries a stable `source_ref` for
Workspace and Evidence Board capture. Source-file provenance remains in
`metadata.source_metadata`; the UI does not need a second backend method.

Deterministic finding codes are `LIGHTHOUSE_PERFORMANCE_POOR`,
`LIGHTHOUSE_AUDITS_POOR`, `LIGHTHOUSE_RUNTIME_ERROR`, and
`LIGHTHOUSE_RUN_WARNINGS`. The default input limit is 64 MiB, and parser
diagnostics report invalid envelopes and bounded-extraction limits.

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

## Local diagnostics extensions (2026-11-11, T-628 foundation implemented)

The [evolution design §16](./LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md#16-extension-and-compatibility-contracts)
preserves `AnalysisResult`. T-628 implements the foundation subset documented below;
full profile/metric migrations and corrected emitters remain proposals.

| Area | Proposed contract |
|---|---|
| Profile core | ProfileSet → profile analyzer → profile_evidence; legacy endpoints remain facades |
| Measurements | JSON Pointer and collection-column descriptors; scalars stay scalar |
| Corrected semantics | Proposed profile_evidence 0.2.0 separates fractional/zero weights, evented duration, multiple profiles and pprof semantics; reanalyze originals |
| Unavailable | Only declared count/time/interval fields become nullable; preserve known zero through UI/export/diff |
| Unknown metadata | Preserve common extensions and legacy typed-key precedence through json.RawMessage roundtrips; do not assume old binaries preserve new fields |
| Versions | Separate result, envelope, extension, adapter, pack, index, case, bundle, capture, visual spec, and renderer/template/font axes |
| Comparisons | Validate reader ranges, metric/unit/population and semantic revision before computation |
| GPU and AI | GPU telemetry projects metrics_timeseries; LLM app semantics extend trace_import; serving references original evidence |

Section 16.3 specifies owners, declarations, and read-only/reject/recompute behavior.
Detail indexes are rebuildable caches; without originals, preserve snapshots and
disable unavailable detail operations.

### T-628 detailed measurement and compatibility contract

The design baseline is `83b8ff9`, fixed at `eb3e782`. T-628 now implements the
shared decoder, metadata and handle-based bridge; scoped evidence is recorded
below. No corrected profile_evidence 0.2.0 emitter is introduced. Execution follows
the [parser contract](./PARSER_DESIGN.md#execution-interfaces-and-ownership).

#### Envelope and decoding boundary

Retain type, source_files, created_at, summary, series, tables, charts and metadata.
Empty successful sections remain objects; sources/findings remain arrays. Failure
and cancellation use job responses, not empty successful results. Add fields only
through versioned metadata extensions or the relevant result payload schema.

The new shared decoder bounds bytes, depth, strings and arrays, rejects duplicate
keys and nonfinite values, preserves number precision using json.Decoder.UseNumber
or equivalent, then dispatches a type-specific reader. Unknown optional extensions
remain bounded opaque data; they cannot supply executable code or file paths.

At design time, common metadata used default number decoding and legacy profiler
metadata had no unknown-key storage. T-628 adds bounded UseNumber decoding, exact
safe legacy projections and profiler RawMessage retention. Do not attribute this
behavior to older binaries or equate Go serialization with JavaScript fidelity.

#### Measurement descriptors and numeric representation

metadata.measurement keys are internal JSON Pointers. The first contract supports
summary scalar targets and series/tables collection descriptors. No external URIs,
wildcards or expressions. Decode ~0/~1; reject invalid escapes, absent targets and
undeclared columns. Nested chart payloads use separately registered schemas.

| Field | Meaning and validation |
|---|---|
| measurement_kind, unit | Actual CPU/wall/count/bytes semantics; do not infer conversion from a name |
| scope, population_ref | Selected profile/time/entity and valid observation population |
| aggregation | Registered sum/count/observed semantics; distinguish histogram estimates from event-derived percentiles |
| numeric_encoding | json_number, exact_integer_string or decimal_string; must match the declared target type |
| availability, reason | observed/derived/estimated/unavailable; unavailable needs null and reason; derived/estimated need method/evidence references |
| evidence_ref | Locally resolvable result/case evidence ID, not permission to fetch arbitrary URLs/files |
| columns | Per-column semantics and schema-declared row availability/reason columns |

Keep full-result totals separate from immutable selected views. Do not redefine
total_samples as duration. Evented profiles use total_weight with a duration
descriptor; unobservable sampling counts are null.

Compute with exact integers/scales or precise decimals internally. Known values in
the new contract exceeding JavaScript's safe integer range use exact_integer_string;
precision-sensitive decimals use decimal_string. Existing number fields cannot
become strings without a schema change. Floating-point rendering projections must
not feed totals, diff or export calculations. Validate numeric-string syntax,
range and scale as well as ordinary JSON numbers.

This complete illustrative result separates observed duration from absent sample
count. It is not current output or a shipped schema. Real results add the evidence
and population references required by their payload schema.

```json
{
  "type": "profile_evidence",
  "source_files": [],
  "created_at": "2026-11-11T00:00:00Z",
  "summary": {"total_weight": 1.25, "total_samples": null},
  "series": {}, "tables": {}, "charts": {},
  "metadata": {
    "parser": "example-evented", "schema_version": "0.2.0",
    "findings": [], "envelope_schema_version": "1.0.0",
    "extensions": {"measurement": "1.0.0"},
    "measurement": {
      "/summary/total_weight": {
        "measurement_kind": "wall_time", "unit": "seconds",
        "scope": "profile", "aggregation": "sum",
        "numeric_encoding": "json_number", "availability": "observed"
      },
      "/summary/total_samples": {
        "measurement_kind": "sample_count", "unit": "count",
        "scope": "profile", "aggregation": "count",
        "numeric_encoding": "json_number", "availability": "unavailable",
        "reason": "evented_profile_has_no_sample_count"
      }
    }
  }
}
```

| Input state | Storage/display | Comparison/export |
|---|---|---|
| Observed zero | Valid declared numeric encoding | Compute differences; no percent change with zero baseline |
| Schema-permitted unavailable | Null plus reason | UI dash, chart gap, CSV blank/reason, unavailable diff |
| Absent optional field | Omitted key and missing capability reason | Do not synthesize null or zero |
| Invalid value/unit/encoding | Result validation failure | No successful numeric export or comparison |

#### Unknown metadata and bridge roundtrips

Keep the common Extra API compatible while decoding unknown numbers precisely.
Add map[string]json.RawMessage storage and custom marshal/unmarshal to legacy
profiler metadata. Typed keys always win, including omitted optional keys: Extra
must not resurrect them. Failed decode leaves the receiver unchanged; successful
reuse removes stale unknown keys.

Do not pass json.Number straight to existing float64 assertions. Numeric accessors
validate integer/scale/range, and legacy consumers receive verified projections.
Reject conversions that cannot preserve the value. Validate Go internal type
compatibility separately from wire compatibility.

JavaScript object parse/resave can lose large unknown numbers. New generic/job/detail
paths keep immutable results in the backend and give UI result handles plus bounded
views. Where opaque metadata must cross the bridge, use JSON text without UI
parse/stringify persistence. JSON export reads the backend revision. Existing
full-result Wails paths require separate roundtrip evidence; a serializer fix alone
does not establish end-to-end precision.

#### Compatibility and migration

Evaluate envelope, payload, extensions, then the requested operation. Missing
envelope_schema_version means legacy envelope, not an inferred payload 0.1.0.
Unknown type/version permits only bounded viewing and original preservation.
ResultDescriptor defines operation-specific required extensions and reader ranges.
A new mandatory extension also changes payload semantic version; it is not merely
an optional block addition.

| Condition | Allowed | Prohibited |
|---|---|---|
| Known legacy payload | Version-specific reader, snapshots, well-defined existing calculations | Fill unknown unit/interval with new defaults |
| Supported payload and required blocks | Validated capability operations | Trust imported capability strings alone |
| Unknown optional block only | Opaque preservation and independent known operations | Delete/infer/execute the block |
| Unsupported payload/required block | Original preservation, possible bounded snapshot, update guidance | Numeric computation, lossy resave, version-only relabeling |
| Stale index | Rebuild from hash-matched originals | Attach different sources to the same revision |

These rules apply to new readers and cannot be imposed on unupdated binaries.
Do not automatically reverse-convert new payloads into old APIs. Migration verifies
source hash and capture/options and reanalyzes into a new revision, or uses an
explicit lossless projection. Do not label old counts as durations. Comparison
requires compatible kind/unit/population/semantic revision and sufficient precision
and observation context. Read-only display does not establish compute compatibility.

#### T-628 wire conformance cases

These cases now have implementation regressions. The implementation record and
T-628 evidence distinguish Go, bridge and frontend checks from actual UI execution.

| ID | Case | Required result |
|---|---|---|
| W01 | Unknown object/array/null/9007199254740993 | Exact semantic/integer Go and UI-mediated export roundtrips |
| W02 | High-precision decimal string and observed zero | Preserve type/scale; display rounding never replaces calculation values |
| W03 | Typed/Extra collision and nil diagnostics | Typed and omission rules win |
| W04 | Reused receiver and failing second decode | Clear stale keys on success; retain previous value on failure |
| W05 | Duplicate keys, excessive depth, NaN/Infinity | Bounded rejection without successful commit |
| W06 | Invalid pointer/target/column/encoding | Result validation failure |
| W07 | Null/zero/absent through UI/CSV/HTML/diff | Preserve availability and denominators |
| W08 | Old/new payload, unknown optional/required blocks | Follow compatibility matrix for computation/resave |
| W09 | Old profile with only version changed | Require source reanalysis; never reinterpret counts as duration |
| W10 | Unknown JSON across Wails/report packs | Backend originals or proven precise path; no script/URL execution |

## Schema 0.2.0 — additive changes

This section records 0.2.0-family changes introduced by some analyzers after
`0.2.0-beta`. It is not a global version migration. Check `schema_version`
per result type and producing path; distinguish the additive entries below
from subsequent nullable-field corrections and their compatibility rules.

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

#### MSA drilldown projection (T-557, contract 1.0.0)

`ProjectJenniferDrilldown({result, selection})` consumes the original
`jennifer_profile` AnalysisResult without reopening inputs. `selection.mode`
is `single` (required `guid`, optional `txid`) or `average` (required
`signature_hash`, optional exact `application` URL). Empty optional selectors
mean the whole group. Invalid/mixed selectors, unknown identities, duplicate
TXIDs within a selected group and nested projections fail explicitly.

Single TXID selection follows matched downstream edges with a cycle guard.
Average selection excludes groups already excluded from signature statistics,
then selects the slowest matching profile once per GUID; response-time ties
preserve input order. SQL/method joins use GUID plus TXID. Missing-GUID rows
use the group's TXID membership only.

The result keeps the common envelope and type. `series.drilldown_scopes`
contains `group`, `profile_rows`, `timeline_edges`, `topology_edges`,
`service_network_edges`, `slow_sql_rows`, `network_time_by_txid`,
`selected_value`, `selected_application`, `selected_txid` and
`is_whole_transaction`. Canonical `series.guid_groups` and `tables.profiles`,
`msa_edges`, `slow_sql_events`, `network_prep_methods`, `method_hotspots`
contain only selected evidence. `series.drilldown_average_breakdown` holds
mean duration components per selected GUID. `drilldown_average_timeline_edges`
is a synthetic call-pair average weighted by `call_count`, not observed time.

`metadata.drilldown` records the version, selection, representative policy and
explicit candidate/sample/excluded counts (also present in `summary`). Costs
are cumulative, not a critical path; overlapping work may exceed root response
time, reported as coverage above 1 and zero residual method time. Parent
parallelism/custom slices are not copied into a subtree. Global file/signature,
custom-rule and unprofiled-call rollups are omitted rather than relabeled.
Those rollups remain available in the whole-result desktop view.

Limits: 64 MiB serialized input, 100,000 profiles, 10,000 groups and 500,000
edges per input edge collection. Exceeding a limit fails without truncation.
Flattened extension metadata survives JSON/Wails save/reload.


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

### GC observation clock (schema 0.2.0)

GC event dates and JVM uptime are parsed independently. `metadata.time_axis`
is `absolute`, `uptime_seconds`, or `event_index`; only absolute coordinates
use a calendar axis. Rate calculations require a common monotonic clock.
`metadata.measurement_clock` identifies that clock; partial uptime evidence
uses dates consistently for every rate rather than mixing clock bases.
`wall_time_sec` spans the first pause end to the last pause end; throughput's
`throughput_pause_ms` excludes the first pause, whose start is outside that
window. Pause totals/percentiles still include every parsed pause. Missing,
reset, zero-span, or inconsistent evidence produces nullable measurements and
`metadata.measurement_unavailable` reason codes, never a fabricated zero.
Allocation/promotion rates use valid adjacent heap observations; absent heap
measurements are unavailable, whereas a measured zero remains zero.

### Flamegraph inclusive weights

Every node value includes its descendants. Canvas derives self weight as
`node.value - sum(child.value)` before d3 aggregation. Internal self weight
remains visible as unoccupied child space; widths, hover totals, and search
percentages share the same inclusive denominator. Recursive matches are
unioned rather than added repeatedly.

### Bounded profiler drilldown

The desktop service retains the canonical tree only inside Go while applying
filters. It prunes each emitted stage after calculating metrics and before IPC;
limits remain enabled. Display pruning never mutates input for later filters.
`FlameNode.hidden_samples` is the inclusive weight of omitted descendants,
not observed self samples. It stays within the node's inclusive `samples` and
is disclosed in the Canvas notice/tooltip even when no extra node fits.
Parser-discarded input remains unavailable and retains its own diagnostics.

### Legacy profile counter units (schema 0.2.0)

CLI `--value-unit` and Wails `valueUnit` accept `samples`, `nanoseconds`,
`microseconds`, `milliseconds`, `seconds`, `bytes`, or `unknown`. CPU/wall
compatibility defaults to periodic sample counts; lock defaults to unknown.
Lock sample counts are event counts, not periodic durations. Known total-time
weights convert directly to seconds; bytes and unknown/lock-count durations
remain null with `duration_unavailable_reason`. Summary and drilldown expose
`value_unit`, `aggregation`, and nullable `seconds_per_weight`. All duration
rows and elapsed ratios follow this contract. pprof export preserves original
weighted units and adds no interval-derived duration/period for lock counters.
The typed V8/Chrome microsecond contract is unchanged.

Trace path latency (T-615): `observed_path_elapsed_ms` is the elapsed interval
of an observed ancestry chain, explicitly an estimate rather than proof of a
causal critical path. Contained ancestor/descendant intervals are not summed.
`critical_path_ms` is a compatibility alias; `selected_chain_cumulative_ms`
separately preserves inclusive work. Missing timestamps, clock skew, duplicate
span IDs or cycles yield null with `duration_unavailable_reason`. UI, SLO and
incident projections consume only the qualified new semantics, never an
unverified legacy inclusive sum. Overall available/unavailable trace counts
remain separate from the top-N path table.

Trace dependency calls (T-616) pair explicit direct CLIENT/SERVER spans by trace and span identity. A one-to-one pair uses CLIENT elapsed time and either-side error status once. Single-sided exports remain usable. Several SERVER children of one aggregate CLIENT remain separate attempts using their own server timings/errors; the aggregate client is not added again. A SERVER remote peer is incoming, not an outgoing dependency. Unspecified kinds are not deduplicated by service names alone. `metadata.dependency_call_policy` versions this policy.

Access-log latency availability (schema 0.3.0, T-617): missing, sentinel or
malformed response durations remain unavailable; measured zero remains zero.
`response_time_sample_count` / `response_time_unknown_count` accompany total
latency statistics. Empty latency statistics and minute/sample values are null,
not zero. URL/route rows carry the same denominators; dependency rows expose
`duration_sample_count` / `duration_unknown_count`. Request/error/byte counts
include all accepted requests. Service Flow weights latency by measured samples
while retaining full traffic counts. SLO exports include per-target
`target_evaluations` and `unevaluated_target_count`: no matched latency evidence
means not assessed, rather than satisfied. Existing programmatic Record inputs
remain measured by default; parsers explicitly mark unavailable durations.

Incident Timeline completeness (T-618): event rows retain at most 120 per
source, then the earliest 1000 session candidates (deterministic tie breaks).
`summary.event_count` is emitted rows; `total_event_count`,
`omitted_event_count` and `partial_result` disclose the full candidate set.
Severity/time/ranged/correlation summaries and severity/category/analyzer
distributions cover all candidates; groups/narrative and their counts cover
retained rows only. `metadata.completeness` records selection policy, per-source
total/emitted/omitted counts, omitted severity counts and both summary scopes.
The page and Report Pack HTML disclose omissions, including omitted critical
events; JSON retains the same metadata.


## T-628 wire implementation and conformance (2026-11-11)

wirejson rejects duplicate keys, malformed/nonfinite JSON and byte/depth/string/
array/object budgets, decodes with UseNumber into a temporary value, then replaces
receivers only on success. Opaque metadata/options/record defaults are 64 MiB,
depth 64, string 4 MiB, 1,000,000
array items and 100,000 object keys. Common Extra projects only exactly representable
JS-safe numbers for legacy consumers; others stay json.Number. Float64Exact rejects
loss. Arithmetic accessors bound numeric text to 1,024 bytes and absolute exponent to 4,096; opaque JSON preservation is separate. Profiler Extra retains RawMessage, with typed/omitempty precedence. Known
large integers use exact_integer_string; precise decimals use decimal_string,
limited to 1,024 bytes and decimal scale 128.

resultcontract validates type-specific payload/envelope ranges, operations,
required extensions and measurement schemas: target/column/kind/unit/aggregation/
encoding/nullability/references. Evidence/population IDs resolve within the result;
no URL/file is opened. Cross-revision comparison requires validated population provenance and compatible
selection/measurement conditions (see R-04 / R2 N-01 below). Exact scalar deltas are rational strings; zero-baseline
percentages and unavailable/absent deltas are null. Legacy float report-diff refuses
new measurement results and directs callers to backend exact comparison. Existing
legacy flows remain intact.

server_log keeps its 0.1.0 values and adds envelope/measurement 1.0.0 metadata.
profile_evidence 0.1.x and existing profiler snapshots have preservation/display/
export readers only; new computations/profile migration are not activated.
profile_evidence 0.2.0 remains unsupported: relabeling an old result cannot permit
computation or normal resave. Unknown optional metadata survives. Unsupported
payloads/required blocks get bounded read-only views and explicit original raw-json
copy, not a successful new analysis revision.

| Cases | Executed regressions |
|---|---|
| W01/W03/W04 | Common/profiler precision, typed precedence, stale-key replacement and receiver atomicity, existing metadata roundtrip |
| W02 | Exact decimal type/scale and zero; rejection of lossy accessors |
| W05 | Duplicate/depth/NaN/Infinity and transactional decode; invalid record/result commit refusal |
| W06 | Invalid pointers/targets/columns/encoding/unit/population references and valid escaping |
| W07 | ScalarView/TS presentation, bridge CSV/HTML/exact delta preserving null/zero/absence, availability and population |
| W08/W09 | Operation-required versus optional blocks, reader ranges and relabel-only profile refusal |
| W10 | Actual CLI export, opaque common/legacy-profiler bridge string simulation, backend pack and TS source appendix fidelity, URL-as-handle refusal |

GetEvidenceResultJSON and backend ExportEvidenceResult never expand opaque JSON
through JavaScript numbers. Report Pack preserves original source text for optional
Workspace backend_result_handle references. The backend pack is a source appendix,
not a replacement for the entire Evidence Board narrative exporter. Full fidelity
is not claimed for legacy routes that already expanded results into JS objects.

Custom Metadata marshalers become opaque to the Wails 3.1 generator. The diagnostic
panel uses the existing equivalent manual bridge diagnostic type; wire keys remain
unchanged. Direct Go-method integration, string-transport simulation and frontend
execution are distinct from actual native IPC/UI execution. See the scoped
[local verification record](../review/evidence/2026-11-11-t628/local.json).

### Review corrections to measurement and wire contracts (2026-11-11)

The R-01–R-14 remediation uses separate complete-result and opaque JSON budgets;
see PARSER_DESIGN for exact ceilings. W05's depth-64 refusal applies to metadata,
options and records, not to ordinary nested flame trees.

**R-04 / R2 N-01:** A population_ref is a result-local label. The original
`source_sha256`, `selection_hash`, `semantic_revision` and record-count `coverage`
remain observation provenance. Equal identities retain same-input comparison,
including old/new snapshots with or without the additive policy below.

New imports add `metadata.populations[ref].comparison` with `version: "1.0.0"`,
a source-independent `selection_hash` and `coverage_kind`. The pilot selection
hash is the options-v1 canonical hash; omitted and explicit defaults may differ.
Coverage kinds are `complete_records`, `selected_records:max_lines` and
`complete_selected_model`. Record counts remain provenance, not compatibility
requirements. Partial imports omit this policy and cannot grant cross-source
comparison. Missing/unknown policies and differing semantic revision, selection,
coverage kind, measurement kind/unit/scope/aggregation are refused.

The ordered arguments to CompareEvidenceScalar are explicit baseline and target
backend handles. For compatible different sources the response carries
`comparison.mode=baseline_target`, the policy and separate baseline/target handle,
exact saved-result SHA-256, source hashes and local population ref. Equal source
identities use `same_input`; this does not assert equal snapshot bytes. No capture,
entity or clock identity is merged/inferred. Registry-only callers get canonical
result digests; service/CLI/Wails callers get exact stored-byte digests.

Pre-policy snapshots keep their original numbers and identity rules: they remain
readable/exportable, but cross-source scalar comparison requires reanalysis. No
payload/measurement version is relabeled. ProfileSet's model-derived comparator
can independently check the retained model as described below. Durable baseline
selection/persistence and broader capture/entity/time matching remain T-630/T-635.

**R3-01 / R3-02:** Scalar comparison refusals use typed `ComparisonError`
with stable `code` and sanitized `message`. Consumers use `errors.As` in Go or
Wails `CallError.cause.code`; they do not parse the displayed sentence. After
same-input compatibility, checks run in this order: semantic revision, partial
coverage, policy presence/support, selection and coverage kind. Earlier
measurement/identity validation still applies.

| Code | Meaning |
|---|---|
| `selection_mismatch` | Comparison selection hashes differ, including omitted vs explicit defaults |
| `coverage_mismatch` | Supported coverage kinds differ |
| `partial_not_comparable` | Cross-population comparison includes declared partial import coverage |
| `semantic_revision_mismatch` | Result type/payload or declared semantic revision differs |
| `policy_missing` | Cross-population comparison lacks a versioned policy |
| `policy_unsupported` | Policy version, hash or coverage kind is unsupported |
| `identity_missing` / `identity_invalid` | Population reference/identity is missing or malformed |
| `measurement_mismatch` | Measurement kind, unit, scope, aggregation or population presence differs |

Successful scalar `comparison.identity_basis` and ProfileSet difference
`identity_basis` are always `declared` in this implementation, including fresh
imports. The backend sets the field and never trusts a saved file's claimed
verification flag. Model/schema validation proves internal consistency; saved
result/model SHA-256 identifies bytes. Neither proves source authenticity or
recomputes original-source/options binding. Edited but structurally valid saved
identity/policy declarations may remain comparable; their basis stays declared.
No `verified_by_reimport` path exists yet. T-635 must bind original reimport,
options, result digests and durable baseline references before making that claim.
These response fields are additive; stored numbers and versions are unchanged.

**R-06/R-11:** The result descriptor's measurement range is the single version
policy (the pilot supports [1.0.0, 1.1.0)). Patch 1.0.1 is accepted; 1.1.0/2.0.0
are refused. Unknown optional fields remain in the original metadata block and
are ignored by typed projections; they grant no required feature or operation.
The shared/bundled descriptor schema 1.0.1 covers scalar, column and collection
variants, including availability_column/reason_column. Empty scalar-only Go fields
are omitted for a collection; valid nonempty scalar wire fields are unchanged.
Schema execution checks structural variants; registered result validators continue
to enforce paths, units, kinds, references, encoding, nullability and row semantics.
The JSON Schema document version and a result's measurement extension version are
separate contracts. Fixture manifest 1.0.1 pins the corrected schema digest.

**R-12:** CSV/HTML presentation emits availability for scalar and per-row collection
columns (path, row, column, availability, reason, unit, population, numeric encoding).
HTML shows an unavailable null as an em dash, CSV keeps an empty cell, and observed
zero stays zero. Existing series.measurement_availability causes an explicit export
collision error; no source series is overwritten. Stored source JSON is unchanged.

The R2 N-04 output budget is 100,000 availability entries and 16 MiB of serialized
appendix JSON. Entry multiplication is checked before cloning the result; each
entry is byte-accounted before append. Either overflow returns typed
`ProjectionBudgetError` / `EXPORT_PROJECTION_BUDGET_EXCEEDED` with no truncated
export or mutation of the stored result. JSON/Report Pack source exports are
unaffected by this CSV/HTML presentation-only budget.

**R-14:** Generated profiler Metadata remains opaque as required by custom Go
marshaling. Manual ProfilerMetadata/ProfilerAnalysisResult/ProfilerDrilldownStage
facades and runtime checks cover the fields consumed by the profiler, diff and
drilldown screens. Unknown keys are unknown rather than any; known type/shape
errors are refused before rendering. Null Go diagnostic arrays remain null.
Bindings are regenerated, and frontend compile-time negative and runtime shape
regressions supplement the original W01–W10 checks. These checks are distinct from
actual WebView IPC/UI execution and independent re-review.

## T-629 ProfileSet core contract (2026-11-11)

`internal/profilemodel` is the canonical internal profile representation. Its
versioned JSON transport is 1.0.0; it is not another AnalysisResult envelope.
New canonical results use the existing `profile_evidence` type with payload 0.2.0.
Legacy `profile_evidence` 0.1.x and typed profiler results retain their original
readers, numbers and endpoints. Result readers now register disjoint payload
ranges for one type; overlapping ranges fail composition. Editing a legacy schema
number is insufficient: the new reader validates the retained model and recomputes
its known summary/tree/table/measurement/population projections.

| Contract | Meaning |
|---|---|
| ProfileSet | Declared producer/version/format, source hash set, opaque capture options/extensions, frames/stacks/metrics/profiles and quality |
| Frame | Stable SHA-256 identity over original symbol/module/build/file/line/runtime/inline/native/kernel/async attributes; display text is excluded from identity |
| Stack | Stable ordered frame-ID path; repeated recursive frames remain separate path occurrences |
| Metric | Explicit kind, unit, positive decimal scale and sign contract; signed differences also declare base_kind |
| Sample | Exact decimal-string weight and metric/stack IDs, labels/source reference and optional exact timestamp/interval |
| Profile | Independent process/thread/worker and aggregate/relative/absolute clock context; measured times require explicit unit/scale |
| Selection | Explicit profile-ID set and one metric; prefix/contains frame-ID paths and conjunctive drilldown filters |
| Quality | Nullable lost/unknown/truncated counts and symbol resolution, clock confidence, and conversion-loss disclosure; unknown is not zero |

Known zero, explicit null timing, and absent timing survive model round trips.
A zero line number differs from an unknown line. Unknown model fields are refused;
optional future content belongs in the opaque `extensions` map. Common result
metadata retains its existing unknown-key precision/preservation behavior.
Weights and scale remain exact decimal strings (1,024 characters, scale 128).
Tree inclusive/self values are in the metric's declared physical unit, after exact
scale multiplication; shares and percentages are rational strings. No float or
minimum-one correction is used. Empty selections have unavailable total weight
with a reason, while observed zero-weight records produce observed zero. The chart
root's zero in an empty algebraic tree is qualified by the query availability;
it is not a measured summary zero. Frame labels are resolved through the frame
table rather than repeated along every tree path.

Comparisons require identical metric ID/kind/unit/scale/sign, producer/normalizer
revision, capture options and ordered frame-selection rules. R4-02 conditions
exclude measured lost/unknown/truncated counters and symbol-resolution ratios.
Quality categories still match: clock confidence, symbol availability
(unknown/unresolved/resolved: null/zero/positive) and conversion-loss categories.
Both reference objects preserve nullable quality measurements; `quality_differences`
lists changed fields and `quality_measurements_differ` warns of any such change.
This is disclosure, not an invented diagnostic threshold or normalization policy.

Profile/process/thread/worker identities still require exact matching; differences
return typed `ComparisonError` code `entity_mapping_required`. Other model compare
refusals distinguish `semantic_revision_mismatch`, `selection_mismatch`,
`clock_class_mismatch`, `quality_class_mismatch`, `capture_options_mismatch` and
`measurement_mismatch`. No role/entity mapping or clock alignment is inferred.
R4's `profile-conditions-v2` hash is separate from original population provenance.
The validator accepts the exactly recomputed older policy hash on saved results
without rewriting it. New generic scalar policies use the v2 hash; old-vs-new
policy mismatch can require reimport, while model-derived comparison can validate
old retained models directly. No payload/schema-number relabeling changes numbers.

Source/population identities and both model/service result digests remain in
baseline/target references; `Difference.population` aliases the baseline. Different
population identities set `comparison_mode=baseline_target`, equal populations use
`same_input`. Identity basis remains declared. Entity remapping, broader capture
policy and durable baselines remain T-630/T-635. Zero-baseline/unavailable values
are not fabricated. BuildSet freezes models/selections; queries never reuse pruned
view trees or mutate evidence.

`LegacyStackProjection` requires an explicit integer-tick scale. It returns the
scale, selected metric/profiles and aggregated dimensions. Display-name keys are
allowed only without identity collisions or delimiter loss; stable frame-ID keys
are available. Fractional integer projections, signed data and native-int/total
range overflow are refused. pprof export similarly requires an explicit tick and
exact int64 values, retaining zero records. A scaled export labels its unit `ticks`
and embeds original metric/unit/scale, selection, frame identities, source hashes
and quality in pprof Comments. Process/thread/worker and exact time context travel
as reserved labels; collisions are errors. This is a selected-metric projection,
not a full multimetric source backup; use JSON/Report Pack for that backup.

Operational bounds are independent ceilings, not a promise that every maximum
can be reached simultaneously. The model library accepts at most 64 MiB wire
input, 32 MiB referenced strings, 100,000 frames/stacks/samples, 1,024 profiles/
stack depth, 64 metrics/filters, 200,000 tree nodes and 1,000,000 selected frame
references. Tree JSON is accounted exactly (own node bytes + separators, no
quadratic subtree serialization) up to 64 MiB. Combined drilldown output remains
32 MiB; pprof comments/output remain 8/64 MiB. A large tree may therefore be valid
for one query yet exceed a combined drilldown or saved-result limit.

The effective catalog transport has these additional limits:

| Resource | Ceiling / behavior |
|---|---|
| Original normalized JSON document | 32 MiB; internal adapter 1.0.1 |
| Normalized record | Raw chunks <=256 KiB, base64 encoded with byte offsets; each still obeys the common serialized-record ceiling (default 1 MiB) |
| Normalized spool / result JSON | 64 MiB each by service policy; no request raises policy |
| Temporary disk | 160 MiB combined source + spool + result; base64 spool expansion is included |
| Selected CSV/HTML appendix | Existing independent 100,000-entry / 16 MiB projection ceiling |

Chunk order/size, cancellation and total reassembly bytes are checked. Incomplete
or out-of-order documents never publish a partial model. `import_provenance.records`
counts transport chunks, not samples; analytic `sample_records` remains unchanged.
Structured `JobProblem.detail` under IMPORT_BUDGET_EXCEEDED exposes reasonCode,
resource, actual and limit (exact decimal strings, never JS numbers) for
transport/record/spool/result/temp and model cardinality/tree/string refusals. Messages contain resource names/numbers, not paths.
No limit silently truncates or rounds data.

The shared synthetic 3,000-stack/depth-18 core test (FromLegacyCollapsed) imports a
4,961,619-byte model and
retains 54,001 tree nodes / exact total 3,000. Its stored result is approximately
28.9 MB (5.82x model size); these model/tree/frame-table/weight-row copies remain
intentional compatibility projections, not production throughput or memory claims.
The actual legacy-normalization CLI produces a 5,480,531-byte model and a
29,561,628-byte result for the same geometry (5.39x), including normalization
metadata. JSON/Report Pack export passes; full CSV/HTML projection at this size
can exceed the existing 100,000-entry appendix ceiling and refuses explicitly.
Select a smaller population or retain JSON/pack; no large HTML success is claimed.

R4-03 caches successful semantic validations by SHA-256 of the actual serialized
result value within one immutable registry (at most 64 digest entries, no models).
Every call hashes current content; changed quality/metadata/payload cannot reuse
an old verdict. Failed validations are not cached. Operation/version/required-block
checks still run for every Check. Eviction/new registry drops only validation
knowledge; saved results and policy are unchanged. Concurrent cold misses may
validate independently. R4-04 also recomputes and compares metadata.quality against
the retained model before caching; forged copies fail compute/compare/export.

### T-629 R2: wire budgets and capture-policy handoff (R5)

The JSON wire boundary also caps each decoded UTF-8 string **or object key** at
4 MiB (4,194,304 bytes), independently of the 32 MiB aggregate referenced-string
budget and 32 MiB internal transport cap. Exactly 4 MiB is accepted; the next
byte is refused. The count is decoded bytes, not Unicode characters or JSON
escape-text length. Input bytes/depth/array items/object keys retain their
existing limits. `wirejson.LimitError` identifies resource/actual/limit, and
ProfileSet decoding translates it to a model budget refusal. Import returns
IMPORT_BUDGET_EXCEEDED with detail.resource=json_string_bytes,
reasonCode=profile_json_string_bytes and exact decimal-string actual/limit.
Malformed JSON/duplicate keys/invalid UTF-8 remain format errors. A refused
model publishes no result and staging is cleaned. No budget is raised.

**R5-02 disposition: producer-specific implementation belongs to T-634, with
baseline consumers in T-635.** The current v2 comparator still compares the whole
opaque capture_options object. An input-provided comparison_keys/ignore list
has no authority. PID/duration/output-path keys are not globally ignored.
This documented limitation does not block T-629 closure under the independent R2.

T-634 acceptance must include a trusted compiled adapter (or approved signed
schema/pack) policy bound to producer, format, supported version, normalizer
revision and selected metric semantics (kind/unit/scale/sign). The policy has a
stable ID/revision/hash and classifies **known** keys as semantic or incidental.
Unknown keys remain strict or cause explicit refusal; missing policies keep full
opaque equality. Unsupported policies cannot relax comparison. Capture metadata
itself cannot declare the policy or supply a whitelist.

Semantic values, including presence/null/observed zero, exact integers/decimals,
interval/event/weight interpretation and metric-dependent duration/denominators,
are compared under that versioned contract. Duration may be incidental for one
metric and semantic for another; real producer fixtures must decide it. Entity
values such as PID must also bind to the capture/entity model and cannot bypass
entity_mapping_required or clock/quality checks merely by being incidental here.

Both comparison references must retain sanitized complete original options,
policy identity, semantic projection and incidental differences with a disclosure
warning. This grants neither original-source verification nor automatic entity
mapping. Preserve saved bytes and old v2 hashes; apply a new policy revision or
explicit reimport instead of silently reinterpreting old snapshots. Validate
same-policy incidental-only changes, semantic mismatch, no/unknown policy and
keys, policy/version/metric mismatch, nil/zero/absence, forged input whitelists,
round trips and CLI/Wails parity before promoting each actual producer. Q01–Q04
and producer versions/real keys remain undecided by this foundation change.

## T-630 case and capture storage contract (2026-11-11)

The `internal/cases` implementation stores case/capture manifests at version
`1.0.0`. This is a separate version namespace from AnalysisResult payloads,
ProfileSet and importer schemas. Future case/sidecar versions are refused without
rewriting their source. AnalysisResult objects are saved as exact original bytes;
unknown metadata, large integer and decimal tokens, null, zero and absent fields
survive save, restart, Wails string transport and portable restore. No case operation
changes diagnostic measurements or qualifies a new producer/version.

A case contains immutable source/result digests, captures, selections, immutable
baseline pins and artifact specifications. Result references pin source IDs and
hashes, options digest and optional component versions. Source replacement,
omission and encoding edits mark dependent results/artifacts stale; old result
bytes and baseline conditions remain unchanged. `snapshot-only` means originals
are absent/unusable or context changed. `originals-present-unverified` means their
bytes match the declared hashes, **not** that an importer re-executed them or that
comparison compatibility was established. T-635 owns that verification, matching,
case-wide search and reuse gate. A source-less saved snapshot cannot claim original
availability. A baseline never changes to a newer result implicitly.

Capture claims distinguish `observed`, `user`, `estimated` and `unknown` basis;
unknown values are null or absent, not zero. Header conflicts produce warnings
without replacing sidecar claims. Clock declarations are provenance, not automatic
alignment. No file mtime becomes a capture time or anchor. Device vendor, kind,
`unified`/`dedicated`/`unknown` memory model, hierarchy and mapping basis are neutral;
Apple devices require no NVIDIA UUID. Hierarchy cycles and unresolved identities
are rejected. Capture files match complete relative names, hashes and sizes;
identical bytes in different host paths keep distinct source/capture provenance.
Options use bounded semantic allowlisted scalars and include/exclude string lists;
command/environment objects and unrecognized option keys are refused. Descriptive
option strings use the existing text mask. Masking does not establish sharing
safety: explicit review is still required. The bundled capture JSON Schema and
synthetic encoding fixtures are byte-identical offline copies of the shared assets;
`internal/cases/contracts/manifest.json` pins their digests. Reference/cycle/path and
cross-field checks supplement the structural schema.

The authoritative store is `UserConfigDir()/ArchScope/diagnostics/case-store`
(macOS Application Support, Windows Roaming AppData). Kernel locking permits one
writer. Objects and immutable revision JSON are written/synced before an atomic
`HEAD.json` replacement containing revision and digest. Every edit requires the
expected decimal-string revision. Startup discards only owned staging; committed
HEAD ignores interrupted next revisions. Hash mismatch is an error, not a fallback
to different evidence. Immutable history/objects are retained until explicit purge;
cache cleanup touches only rebuildable indexes and owned staging. Rebuild scans
committed manifests and reports unreadable cases instead of silently omitting them.

Lifecycle is active → archived → restored; deleted is a reversible logical state.
Deleted snapshots cannot be read until restored. Retention days are nullable,
user-selected reminders measured since last committed update, never automatic
expiry. Purge requires logical deletion plus a token bound to the current case and
revision. Its plan enumerates managed sources, results, history, workspace/AI and
visual specifications, and disposable case indexes/staging. External originals,
separately exported files, clipboard, OS backups, legacy cache copies and shared
analysis-cache snapshots/caller leases are explicitly outside this case-scoped
operation. Physical SSD erasure is not promised. Deleted cases can be purged even
when an evidence object is corrupt; restoration still requires valid objects.

Portable ZIP export includes only explicitly selected sharing-reviewed sources and
result snapshots. Case context/specifications require a separate explicit manifest
review. External paths are removed, sources default to omitted, Workspace defaults
to omitted, and unselected result/baseline/artifact references are filtered. Restore
checks versions, entry/content hashes, source sizes and result envelopes before a
single new-case commit. It rejects external paths/unreferenced objects, assigns a
new local case ID and clears sharing approvals. This is local content validation,
not a signed package/trusted publisher claim (T-632).

Operational allocation bounds: 128 cases; 128 references per collection; 2 MiB
manifest; 64 MiB object/source/decoded UTF-8; 512 MiB declared sources and referenced
objects per case; 4 GiB normal store budget; 1,024 ordinary revisions. Up to two
extra archive/delete revisions and bounded manifest reserve permit explicit purge
at those logical limits. History is never silently evicted. Bounded full-store
filesystem traversal also caps entries. A fresh portable restore starts a new
history. OS disk-full/I/O failures can still prevent a commit; callers reload HEAD
on any uncertain write outcome. These are implementation resource ceilings, not
diagnostic thresholds or automatic retention policy.
