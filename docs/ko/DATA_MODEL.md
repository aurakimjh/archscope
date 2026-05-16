# 데이터 모델

ArchScope는 모든 parser output을 공통 analysis structure로 표준화한다. 이를 통해 UI, chart template, exporter가 안정적인 contract를 기준으로 동작한다.

## 공통 결과 모델

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

### TypeScript Interface

```typescript
interface AnalysisResult {
  type: AnalyzerType;
  source_files: string[];
  created_at: string;  // ISO 8601
  summary: Record<string, number | string | null>;
  series: Record<string, SeriesRow[]>;
  tables: Record<string, TableRow[]>;
  charts?: ChartData;
  metadata: ResultMetadata;
}

type AnalyzerType =
  | "access_log"
  | "profiler_collapsed"
  | "gc_log"
  | "thread_dump"
  | "exception_stack"
  | "nodejs_stack"
  | "python_traceback"
  | "go_panic"
  | "dotnet_exception_iis"
  | "otel_logs"
  | "comparison_report";

interface ResultMetadata {
  format?: string;
  parser: string;
  schema_version: string;
  diagnostics: ParserDiagnostics;
  analysis_options?: Record<string, unknown>;
  findings?: Finding[];
  ai_interpretation?: InterpretationResult;
}

interface ParserDiagnostics {
  total_lines: number;
  parsed_records: number;
  skipped_lines: number;
  skipped_by_reason: Record<string, number>;
  samples: DiagnosticSample[];
}

interface DiagnosticSample {
  line_number: number;
  reason: string;
  message: string;
  raw_preview: string;
}
```

### Python Dataclass

```python
from dataclasses import dataclass, field
from typing import Any

@dataclass
class AnalysisResult:
    type: str
    source_files: list[str]
    created_at: str
    summary: dict[str, Any]
    series: dict[str, list[dict[str, Any]]]
    tables: dict[str, list[dict[str, Any]]]
    metadata: dict[str, Any]
    charts: dict[str, Any] = field(default_factory=dict)
```

### 샘플 출력 (Access Log)

```json
{
  "type": "access_log",
  "source_files": ["/data/logs/nginx-access.log"],
  "created_at": "2026-04-30T10:30:00Z",
  "summary": {
    "total_requests": 15234,
    "avg_response_ms": 42.5,
    "p95_response_ms": 187.3,
    "p99_response_ms": 523.1,
    "error_rate": 2.3
  },
  "series": {
    "requests_per_minute": [
      {"time": "2026-04-30T10:00:00Z", "value": 312},
      {"time": "2026-04-30T10:01:00Z", "value": 298}
    ],
    "status_code_distribution": [
      {"status": "2xx", "count": 14882},
      {"status": "4xx", "count": 287},
      {"status": "5xx", "count": 65}
    ]
  },
  "tables": {
    "sample_records": [
      {
        "timestamp": "2026-04-30T10:00:01Z",
        "method": "GET",
        "uri": "/api/orders/1001",
        "status": 200,
        "response_time_ms": 45.2
      }
    ]
  },
  "charts": {},
  "metadata": {
    "format": "nginx",
    "parser": "archscope_access_log_parser",
    "schema_version": "0.1.0",
    "diagnostics": {
      "total_lines": 15300,
      "parsed_records": 15234,
      "skipped_lines": 66,
      "skipped_by_reason": {"NO_FORMAT_MATCH": 66},
      "samples": [
        {
          "line_number": 42,
          "reason": "NO_FORMAT_MATCH",
          "message": "Line does not match nginx combined format",
          "raw_preview": "# This is a comment line..."
        }
      ]
    },
    "analysis_options": {
      "max_lines": null,
      "start_time": null,
      "end_time": null
    },
    "findings": [
      {
        "severity": "warning",
        "code": "ELEVATED_ERROR_RATE",
        "message": "Error rate is 2.3%, above 5% warning threshold consideration",
        "evidence": {"error_rate": 2.3, "threshold": 5.0}
      }
    ]
  }
}
```

### 필드 목적

- `type`: `access_log`, `profiler_collapsed` 같은 진단 결과 유형
- `source_files`: 결과 생성에 사용한 source file 목록
- `created_at`: ISO 8601 생성 시각
- `summary`: card와 executive summary에 사용할 핵심 metric
- `series`: chart-ready time series 또는 distribution
- `tables`: 보고서용 table data
- `charts`: chart template reference와 rendering metadata
- `metadata`: parser format, runtime, time zone, interval 등 부가 정보

## 계약 강화 범위

1차 계약 강화는 이미 코드에 존재하는 analyzer result type으로 제한한다.

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

공통 `AnalysisResult` dataclass는 당분간 외부 transport model로 유지한다. 계약 강화 계층은 `summary`, `series`, `tables`, `metadata` 내부에 들어가는 type별 필수 key를 정의하는 방식으로 적용한다.

### 이번 범위에 포함

- Access Log와 Profiler result section에 대한 Python `TypedDict` 정의
- Renderer와 chart code에서 사용할 대응 TypeScript interface
- 필수 key, 값 type, unit 문서화
- 향후 migration을 위한 `metadata.schema_version` 유지

### 이번 범위에서 제외

- Pydantic model 전면 전환
- 모든 nested field에 대한 runtime validation
- Chart Studio template schema
- Dashboard sample data를 canonical contract로 취급하는 것. `dashboard_sample`은 UI fixture data로만 본다.

### Versioning 규칙

- Optional field 추가는 동일 `schema_version`에서 허용한다.
- Required field 제거 또는 rename은 `schema_version` bump가 필요하다.
- Numeric field는 unit이 명확하지 않으면 key에 `_ms`, `_sec`, `_percent` 같은 unit suffix를 둔다.
- Malformed-input 처리를 지원하는 parser의 diagnostics는 `metadata.diagnostics` 아래에 둔다.
- Portable parser debug log는 별도 JSON artifact이며 `AnalysisResult` 내부 field가 아니다. Parser 개발을 위해 redacted raw context, `field_shapes`, partial match data, traceback data를 포함할 수 있다.

## 필수 Result Contract

### Access Log Result

`type`: `access_log`

필수 `summary` fields:

| Field | Type | Unit / 의미 |
|---|---|---|
| `total_requests` | integer | 파싱된 request 수 |
| `avg_response_ms` | number | 평균 응답 시간, milliseconds |
| `p95_response_ms` | number | 95 percentile 응답 시간, milliseconds |
| `p99_response_ms` | number | 99 percentile 응답 시간, milliseconds |
| `error_rate` | number | HTTP status `>= 400` 요청 비율, percent |

필수 `series` fields:

| Field | Row shape |
|---|---|
| `requests_per_minute` | `{ time: string, value: number }` |
| `avg_response_time_per_minute` | `{ time: string, value: number }` |
| `p95_response_time_per_minute` | `{ time: string, value: number }` |
| `status_code_distribution` | `{ status: string, count: integer }` |
| `top_urls_by_count` | `{ uri: string, count: integer }` |
| `top_urls_by_avg_response_time` | `{ uri: string, avg_response_ms: number, count: integer }` |

필수 `tables` fields:

| Field | Row shape |
|---|---|
| `sample_records` | `{ timestamp: string, method: string, uri: string, status: integer, response_time_ms: number }` |

필수 `metadata` fields:

| Field | Type | 의미 |
|---|---|---|
| `format` | string | Access log format selector |
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line count와 skipped record sample |
| `analysis_options` | `AccessLogAnalysisOptions` | 적용된 sampling 및 time-range option |
| `findings` | array of `AccessLogFinding` | 보고서 지향 access-log observation |

`AccessLogAnalysisOptions` 필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `max_lines` | integer or null | source file에서 읽을 최대 physical line 수 |
| `start_time` | string or null | inclusive ISO 8601 lower timestamp bound |
| `end_time` | string or null | inclusive ISO 8601 upper timestamp bound |

`AccessLogFinding` 필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `severity` | string | `warning`, `critical` 같은 finding severity |
| `code` | string | stable finding code |
| `message` | string | 사람이 읽을 수 있는 finding summary |
| `evidence` | object | finding을 뒷받침하는 작은 structured value |

### Profiler Collapsed Result

`type`: `profiler_collapsed`

필수 `summary` fields:

| Field | Type | Unit / 의미 |
|---|---|---|
| `profile_kind` | string | Profile type, 초기값은 `wall` |
| `total_samples` | integer | Collapsed stack sample 총수 |
| `interval_ms` | number | Sampling interval, milliseconds |
| `estimated_seconds` | number | 추정 sampled time, seconds |
| `elapsed_seconds` | number or null | Optional observed elapsed time, seconds |

필수 `series` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null }` |
| `component_breakdown` | `{ component: string, samples: integer }` |
| `execution_breakdown` | optional `{ category, executive_label, primary_category, wait_reason, samples, estimated_seconds, total_ratio, parent_stage_ratio, elapsed_ratio, top_methods, top_stacks }` |
| `timeline_analysis` | optional `{ index, segment, label, samples, estimated_seconds, stage_ratio, total_ratio, elapsed_ratio, top_methods, method_chains, top_stacks }` |

필수 `tables` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null, frames: string[] }` |
| `top_child_frames` | optional `{ frame: string, samples: integer, ratio: number }` |
| `timeline_analysis` | optional. `series.timeline_analysis`와 같은 row shape |

Optional `charts` fields:

| Field | Shape |
|---|---|
| `flamegraph` | `FlameNode` |
| `drilldown_stages` | drill-down stage object array |

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

`timeline_analysis`는 플레임그래프 샘플 기반 수행시간 구성 뷰다. 설정된
sampling interval로 sample을 estimated seconds로 변환하지만, 실제 timestamp
순서가 있는 trace는 아니다.

필수 `metadata` fields:

| Field | Type | 의미 |
|---|---|---|
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line count와 skipped record sample |

### ParserDiagnostics

필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `total_lines` | integer | source file에서 읽은 physical line 수 |
| `parsed_records` | integer | parser가 수용한 valid non-blank record 수 |
| `skipped_lines` | integer | parser가 skip한 malformed non-blank record 수 |
| `skipped_by_reason` | object mapping string to integer | reason code별 skipped record count |
| `samples` | array of `DiagnosticSample` | skipped record의 bounded example |

`DiagnosticSample` 필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `line_number` | integer | 1-based source line number |
| `reason` | string | `NO_FORMAT_MATCH` 같은 stable reason code |
| `message` | string | 사람이 읽을 수 있는 parser message |
| `raw_preview` | string | truncated raw input preview, 현재 200 characters로 제한 |

### Multi-runtime Stack Results

`nodejs_stack`, `python_traceback`, `go_panic`은 같은 stack-oriented result shape를 사용한다.

필수 `summary` fields:

| Field | Type | 의미 |
|---|---|---|
| `total_records` | integer | Parsed stack 또는 runtime block 수 |
| `unique_record_types` | integer | Distinct error/panic/goroutine type 수 |
| `unique_signatures` | integer | Distinct normalized stack signature 수 |
| `top_record_type` | string or null | 가장 많이 나온 record type |

필수 `series` fields:

| Field | Row shape |
|---|---|
| `record_type_distribution` | `{ record_type: string, count: integer }` |
| `top_stack_signatures` | `{ signature: string, count: integer }` |

필수 `tables` fields:

| Field | Row shape |
|---|---|
| `records` | `{ runtime, record_type, headline, message, signature, top_frame, stack }` |

`dotnet_exception_iis`는 위 stack series/table에 IIS access summary를 추가한다.

| Field | Location | 의미 |
|---|---|---|
| `iis_requests` | `summary` | Parsed IIS W3C request count |
| `iis_error_requests` | `summary` | status `>= 500`인 IIS request 수 |
| `max_iis_time_taken_ms` | `summary` | 최대 `time-taken` 값 |
| `iis_status_distribution` | `series` | Status-class distribution |
| `iis_slowest_urls` | `series` | 가장 느린 IIS URI rows |

### OpenTelemetry Logs Result

`type`: `otel_logs`

필수 `summary` fields:

| Field | Type | 의미 |
|---|---|---|
| `total_records` | integer | Parsed OTel JSONL log record 수 |
| `unique_traces` | integer | Distinct non-empty trace ID 수 |
| `unique_services` | integer | Distinct non-empty service name 수 |
| `cross_service_traces` | integer | 둘 이상의 service에서 관찰된 trace 수 |
| `failed_traces` | integer | Error signal이 포함된 trace 수 |
| `failure_propagation_traces` | integer | 첫 실패 이후 downstream service 활동이 이어진 failed trace 수 |
| `error_records` | integer | Error-level severity record 수 |

필수 `series` fields:

| Field | Row shape |
|---|---|
| `severity_distribution` | `{ severity: string, count: integer }` |
| `service_distribution` | `{ service: string, count: integer }` |
| `top_traces` | `{ trace_id: string, count: integer }` |

필수 `tables` fields:

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

`comparison_report`는 raw evidence를 다시 parse하지 않고 기존 `AnalysisResult` JSON 2개를 비교한다.

필수 `summary` fields:

| Field | Type | 의미 |
|---|---|---|
| `before_type` | string | Before result type |
| `after_type` | string | After result type |
| `changed_metrics` | integer | Delta가 0이 아닌 numeric summary field 수 |
| `before_findings` | integer | Before result finding count |
| `after_findings` | integer | After result finding count |
| `finding_delta` | integer | After finding count minus before finding count |

필수 `series` fields:

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

## JVM Analyzer Result Contract

JVM analyzer MVP는 additive `AnalysisResult` contract를 사용하며 `metadata.schema_version = "0.1.0"`과 `metadata.diagnostics` parser diagnostics를 유지한다.

### GC Log Result

`type`: `gc_log`

필수 `summary` fields:

| Field | Type | Unit / 의미 |
|---|---|---|
| `total_events` | number | parsed GC event 수 |
| `total_pause_ms` | number | 전체 GC pause 시간 |
| `avg_pause_ms` | number | 평균 pause |
| `max_pause_ms` | number | 최대 pause |
| `young_gc_count` | number | young GC event 수 |
| `full_gc_count` | number | full GC event 수 |

필수 series keys: `pause_timeline`, `heap_after_mb`, `gc_type_breakdown`, `cause_breakdown`.

### Thread Dump Result

`type`: `thread_dump`

필수 `summary` fields: `total_threads`, `runnable_threads`, `blocked_threads`, `waiting_threads`, `threads_with_locks`.

필수 series keys: `state_distribution`, `category_distribution`, `top_stack_signatures`.

### Exception Stack Result

`type`: `exception_stack`

필수 `summary` fields: `total_exceptions`, `unique_exception_types`, `unique_signatures`, `top_exception_type`.

필수 series keys: `exception_type_distribution`, `root_cause_distribution`, `top_stack_signatures`.

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

지원하는 local file format:

- `otlp-json`
- `zipkin-v2-json`
- `elastic-apm-search-json`
- `elastic-apm-source-ndjson`
- `jaeger-query-json`
- `skywalking-graphql-json`

`jaeger-query-json`은 `data[].spans`와 `processes`를 포함하는 Jaeger
QueryService 스타일 JSON envelope 또는 같은 형태의 local trace object를
받는다. `skywalking-graphql-json`은 schema guard를 적용한다. 현재 importer는
`data.queryTrace.spans` 또는 `data.trace.spans`가 있는 SkyWalking GraphQL
응답만 canonical span으로 변환하고, 다른 SkyWalking 응답 형태는 추측하지
않고 parser diagnostics로 보고한다.

필수 `summary` fields:

| Field | Type | 의미 |
|---|---|---|
| `source_format` | string | 감지되었거나 요청된 import format |
| `total_spans` | integer | 파싱된 canonical span 수 |
| `unique_traces` | integer | 고유 trace id 수 |
| `unique_services` | integer | unknown을 제외한 service 수 |
| `unique_dependencies` | integer | caller-to-callee service edge 수 |
| `error_spans` | integer | error로 표시된 span 수 |
| `missing_parent_spans` | integer | import에 parent가 없는 span 수 |
| `clock_skew_suspected_spans` | integer | parent 시간 범위를 벗어난 child span 수 |
| `p95_trace_duration_ms` | number | trace duration p95 |
| `max_trace_duration_ms` | number | 가장 긴 trace duration |
| `unbalanced_service_count` | integer | 전체 span latency를 지배하는 service 수 |
| `high_error_service_edges` | integer | 오류율 threshold를 넘은 service edge 수 |

주요 `tables` row:

- `spans` — `{ trace_id, span_id, parent_span_id, name, service_name, remote_service, kind, start_unix_nanos, duration_ms, status_code, error, source_format }`
- `traces` — `{ trace_id, span_count, service_count, services, duration_ms, total_span_ms, error_count, slowest_span, slowest_span_ms }`
- `service_dependencies` — `{ caller, callee, call_count, total_duration_ms, avg_duration_ms, error_count, error_rate }`
- `service_summary` — `{ service, span_count, total_duration_ms, avg_duration_ms, error_count }`
- `critical_paths` — `{ trace_id, critical_path_ms, span_count, services, span_names }`

현재 deterministic finding code는 `SLOW_TRACE_P95`,
`TRACE_IMPORT_ERROR_SPANS`, `TRACE_IMPORT_MISSING_PARENT`,
`CLOCK_SKEW_SUSPECTED`, `UNBALANCED_SERVICE_LATENCY`,
`HIGH_ERROR_SERVICE_EDGE`, `TRACE_IMPORT_CROSS_SERVICE_TRACE`,
`TRACE_IMPORT_SLOW_SPAN_DOMINATES_TRACE`를 포함한다.

## Evidence Board Card

첫 Evidence Board 구현은 persisted engine result가 아니라 local UI state다.
Analyzer finding, chart selection, table row, parser diagnostic, source
metadata를 이후 하나의 report pipeline에 묶기 위해 card field를 범용적으로
둔다.

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

첫 Incident Timeline 구현은 Analysis Workspace 결과로부터 만든 Wails session
projection이다. Analyzer가 소유한 `AnalysisResult` 출력을 대체하지 않고,
기존 evidence를 timeline view로 정규화한다.

```text
id
timestamp
time_label
source_analyzer
source_result_id
source_title
source_file
severity
category
label
description
evidence_ref
payload
```

현재 event source는 analyzer finding, access-log error/latency series, GC
alert, JFR pause/notable event, exception row, thread-dump contention/deadlock
table, trace-import error/critical-path row를 포함한다.

Report pack에서는 같은 projection을 `type = "incident_timeline"`인 exportable
`AnalysisResult`로 낼 수 있다. Source file, summary count,
severity/category/source distribution, `tables.events`, `metadata.source_results`
를 보존하므로 analyzer가 소유한 result를 대체하지 않고 session timeline을
report artifact 안에 지속할 수 있다.

## SLO 및 Golden Signals Projection

첫 SLO / Golden Signals 구현은 Analysis Workspace 결과로부터 만든 Wails
session projection이다. Analyzer가 소유한 `AnalysisResult` 출력을 대체하지
않고, 기존 metric을 signal, SLI, SLO view로 정규화한다.

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

`SliMetric`은 kind, name, unit, aggregation, scope type, scope가 호환되는
Golden Signal을 묶는다.

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

현재 Golden Signal source는 access-log latency/throughput/error metric,
trace-import service/dependency/critical-path metric, Jennifer MSA external-call
및 network-gap metric, exception distribution, GC pause, memory-space, OOM 및
long-pause alert, JFR pause/sample/thread metric, thread-dump
lock/contention/deadlock metric, heap 및 worker count 같은 JVM metadata를
포함한다.

## Service Flow Projection

첫 Service Flow 구현은 Analysis Workspace 결과로부터 만든 Wails session
projection이다. Trace Import `service_dependencies`, Jennifer `tables.msa_edges`,
Jennifer unprofiled external-call group을 공통 service-edge view로 통합한다.

`ServiceFlowInputEdge` fields:

```text
id
source_type            trace_import_dependency | jennifer_msa_edge | jennifer_unprofiled_external_call_group
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

`ServiceEdge`는 caller/callee가 호환되는 input edge를 묶는다.

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

현재 deterministic service-flow finding code는
`SERVICE_FLOW_UNMATCHED_CALLS`, `SERVICE_FLOW_HIGH_NETWORK_GAP`,
`SERVICE_FLOW_MISSING_PARENT`다. Wails Service Flow page는 Mermaid
sequence-like view(`.mmd`)와 JSON `archscope_service_flow` payload를 export할
수 있다.

## Report Pack

Wails Evidence Board는 전체 engine report-pack exporter가 완성되기 전에도
UI-level report pack을 만들 수 있다.

```text
ReportPack
  type = archscope_report_pack
  schema_version
  created_at
  card_count
  source_result_count
  provenance
  artifacts
```

`provenance`는 source result metadata, analyzer option, captured evidence card,
deterministic finding, derived artifact reference, 존재하는 경우 optional AI
interpretation provenance를 보존한다. `artifacts`는 현재 evidence card,
exportable `incident_timeline` result, SLO analysis, Service Flow export
payload를 포함한다.

## AI Interpretation Contract

AI interpretation은 `AnalysisResult`를 대체하지 않는다. Source result에 연결된 별도 `InterpretationResult`로 취급한다.

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

`AiFinding` 필수 fields:

| Field | Type | 의미 |
|---|---|---|
| `id` | string | stable finding id |
| `label` | string | 짧은 제목 |
| `severity` | string | `info`, `warning`, `critical` 중 하나 |
| `generated_by` | string | 반드시 `ai` |
| `model` | string | local model name |
| `summary` | string | 사용자에게 표시할 interpretation |
| `reasoning` | string | evidence에 묶인 짧은 reasoning |
| `evidence_refs` | array of string | 비어 있지 않은 canonical evidence reference |
| `evidence_quotes` | object | 선택 사항. `evidence_ref`별 exact evidence substring |
| `confidence` | number | `0`부터 `1`까지의 model confidence. 초기 표시 threshold는 `0.3` |
| `limitations` | array of string | missing evidence 또는 uncertainty |

Canonical `evidence_ref` 문법:

```text
{source_type}:{entity_type}:{identifier}
```

등록된 namespace:

| Source | Entities |
|---|---|
| `access_log` | `record`, `finding` |
| `profiler` | `stack`, `frame`, `finding` |
| `jfr` | `event` |
| `otel` | `log`, `span`, `event` |
| `trace_import` | `span`, `trace`, `service_edge`, `finding` |
| `evidence_board` | `card` |
| `timeline` | `event`, `correlation` |

AI output은 표시 전에 runtime validation을 통과해야 한다. Validation은 non-empty reference, grammar 및 namespace 검사, source evidence registry 내 reference 존재 여부, confidence threshold, quote-to-source matching을 포함한다.

## 설계 원칙

- Parser는 가능한 경우 `raw_line` 또는 `raw_block`으로 원본 근거를 보존한다.
- Analyzer는 숫자 필드에 명확한 unit을 사용한다.
- Chart input은 parser-specific object가 아니라 `series`와 `tables`를 사용한다.
- Runtime-specific field는 범용성이 낮으면 `metadata`에 둔다.
- Analyzer sampling 및 filter setting은 `metadata.analysis_options` 아래에 echo한다.
- 보고서용 interpretation은 prose-only blob이 아니라 bounded structured finding으로 표현한다.
- AI-assisted interpretation은 기존 raw evidence를 가리키는 non-empty `evidence_refs`를 포함해야 한다. Evidence는 `raw_line`, `raw_block`, `raw_preview`, `evidence_ref` row 중 하나로 추적 가능해야 한다.

## Schema 0.2.0 — additive 변경

`0.2.0-beta` 이후의 분석기 개편으로 `schema_version`은 `0.1.0`에서
`0.2.0`으로 올랐다. 기존 0.1.0 필드는 모두 그대로 존재하고 required
유지; 아래 변경은 모두 additive다.

### Access log (`type: "access_log"`)

새 `summary` 필드(forward compatibility를 위해 모두 optional이지만
현재 분석기가 emit한다):

| Field | Type | 의미 |
| --- | --- | --- |
| `p50_response_ms` / `p90_response_ms` | number | 응답시간 percentile 추가 |
| `total_bytes` | integer | 응답 크기 합 |
| `avg_requests_per_second` | number | 분석 윈도우 throughput (req/s) |
| `avg_throughput_bps` | number | bytes/s throughput |
| `static_request_count` / `api_request_count` | integer | 확장자/asset 경로 기준 정적/API 분류 |

새 `series` row:

- `percentile_response_time_per_minute` — `{ time, p50, p90, p95, p99 }`
- `throughput_per_minute` — `{ time, requests_per_second, bytes_per_second }`
- `status_class_per_minute` — `{ time, c2xx, c3xx, c4xx, c5xx, error_rate }`

새 `tables` row:

- `url_stats` — `{ method, uri, classification, count, avg_ms, p95_ms, total_bytes, error_count, status_mix: {c2xx, c3xx, c4xx, c5xx} }`
- `top_status_codes` — `{ status: integer, count: integer }`

새 finding: `SLOW_URL_P95`(warning, p95 ≥ 1 s, 샘플 ≥ 5),
`ERROR_BURST_DETECTED`(warning, 분당 오류율 ≥ 50%, 요청 ≥ 5).

### GC log (`type: "gc_log"`)

새 `metadata.jvm_info` 객체(`gc_log_header.py`가 추출):

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
  command_line_flags: string[],     # 플래그당 한 요소
  raw_header_lines: string[],       # bounded copy
  worker_cpu_mismatch: boolean
}
```

새 optional `series` row: `young_heap_before_per_event`,
`young_heap_after_per_event`, `old_heap_before_per_event`,
`old_heap_after_per_event`, `metaspace_before_per_event`,
`metaspace_after_per_event`. 기존 `heap_before_per_event` /
`heap_after_per_event` / `heap_committed_per_event`는 그대로.

### Profiler (`type: "profiler_collapsed"` 등)

- `charts.flamegraph_root` — `FlameNode`는 `profiler_diff`가 채우는
  optional `metadata: { a, b, delta }`(baseline 샘플, target 샘플,
  부호 있는 delta)를 가질 수 있다. diff 인지 없는 UI는 무시 가능.
- `metadata.threads` — `[Thread]` prefix가 전체 샘플의 ≥ 80%를
  포함할 때(async-profiler `-t` collapsed) 채워지는
  `{ name, samples, ratio }` 배열. **Filter by thread** 드롭다운이 사용.
- 새 분석기 type: `profiler_diff`(`{a, b, delta}` metadata가 들어 있는
  flame tree와 `tables.gainers` / `tables.losers` 반환),
  `profiler_export_pprof`(`~/.archscope/uploads/`의 gzipped pprof
  파일을 가리키는 `{ outputPath, sizeBytes }` 반환).

### Jennifer profile (`type: "jennifer_profile"`)

MSA 타임라인 매칭은 기존처럼 matched caller-to-callee edge를
`tables.msa_edges`에 낸다. callee profile이 없는 외부호출은 이제
잔여 `method_time_ms`에 섞지 않고 “프로파일 미수집 외부호출”로 별도
분리한다.

새 `summary` 필드:

| Field | Type | 의미 |
| --- | --- | --- |
| `total_unprofiled_external_call_ms` | integer | unmatched / unprofiled downstream call의 `EXTERNAL_CALL` elapsed 합계 |

새 `tables` row:

- `unprofiled_external_call_groups` — `{ guid, caller_application, target,
  protocol, client, match_status }` 기준으로 묶고 `{ count,
  total_elapsed_ms, avg_elapsed_ms, max_elapsed_ms, caller_txids,
  external_call_urls }`를 낸다. `target`은 query string/fragment를 제거하고
  숫자/UUID형 path ID를 `{id}`로 접어 같은 호출군이 묶이도록 한다.

새 `series` row:

- `service_call_network_summary` — matched MSA 호출을
  `{ caller_application, callee_application }` 기준으로 묶는다. 각 row는
  `call_count`, `guid_count`, `external_call_elapsed_ms`,
  `callee_response_time_ms`, `network_gap_ms`, `avg_network_gap_ms`,
  `p95_network_gap_ms`, `max_network_gap_ms`, `total_network_gap_ms`,
  stable `network_time_group` / `network_time_group_label`을 포함한다.
  네트워크 그룹은 평균 adjusted network gap 기준으로 `0-4 ms`,
  `5-9 ms`, `10-19 ms`, `20-49 ms`, `50-99 ms`, `>=100 ms` band를
  사용한다. UI 토폴로지는 이 그룹으로 node 위치를 나누어 서비스의
  네트워크 거리/위치를 추정한다.

변경된 `series.guid_groups[].metrics.response_time_breakdown`:

- `unprofiled_external_call_ms`는 `method_time_ms` 계산 전에 차감한다.
- `network_call_ms`는 기존처럼 matched MSA network gap
  (`EXTERNAL_CALL elapsed - callee response time`)만 의미한다.
- `method_time_ms`는 callee profile이 없지만 caller elapsed는 확인된
  외부호출을 포함하지 않는 순수 잔여 애플리케이션 시간이다.

이벤트 카테고리 옵션의 시간 계산:

- `EventCategoryPatterns`로 METHOD row를 SQL, Check Query, 2PC, Fetch,
  Connection Acquire에 추가한 경우 같은 ledger bucket 안에서는 exclusive
  elapsed를 사용한다. 예를 들어 2PC wrapper 메소드 안에 다른 2PC 메소드가
  포함되어 있으면 부모 메소드에서 자식 interval을 빼고 합산해 중첩 메소드가
  중복 계산되지 않게 한다.
- `EXTERNAL_CALL`은 MSA 매칭, 프로파일 미수집 외부호출 집계, 병렬도 분석이
  caller가 보고한 wait time에 의존하므로 raw cumulative elapsed를 유지한다.

### JFR (`type: "jfr_recording"`)

- `summary.sample_event_count`, `summary.stack_sample_count`,
  `summary.unique_sample_stacks`는 필터된 JFR 이벤트 중 async-profiler 스타일
  스택 집계에 사용할 수 있는 이벤트 규모를 나타낸다.
- `series.events_over_time`, `series.pause_events`, `series.events_by_type`,
  `series.heatmap_strip`, `series.sample_events_by_type`는 chart-ready 이벤트
  요약이다.
- `tables.notable_events`는 긴 duration 또는 주요 이벤트 row를 유지한다.
- `tables.top_methods`, `tables.top_packages`, `tables.top_threads`,
  `tables.sample_stacks`는 JFR sample event의 `stackTrace.frames`를 집계한
  보고서용 evidence다. 이는 JDK Mission Control 또는 `jfr view` 전체
  기능을 대체하는 UI가 아니다.
- `charts.async_profile_flamegraph`는 sample stack에서 만든 FlameNode 호환
  tree다.
- `metadata.jfr_contract`는 JDK `jfr` CLI가 바이너리 `.jfr`를
  `jfr print --json`으로 변환하는 boundary까지만 담당한다는 점을 명시한다.
- `metadata.findings`에는 `JFR_FILTER_EMPTY`, `JFR_NO_STACK_SAMPLES`,
  `JFR_ASYNC_PROFILER_STYLE_RECORDING`, `JFR_SPARSE_STACK_SAMPLES` 같은 UX hint가
  포함될 수 있다.

### Thread dump multi (`type: "thread_dump_multi"`)

- `metadata.findings`는 Phase 6 로드맵에 명시된 코드들을 모두 포함
  가능: `LONG_RUNNING_THREAD`, `PERSISTENT_BLOCKED_THREAD`,
  `LATENCY_SECTION_DETECTED`, `GROWING_LOCK_CONTENTION`,
  `THREAD_CONGESTION_DETECTED`, `EXTERNAL_RESOURCE_WAIT_HIGH`,
  `LIKELY_GC_PAUSE_DETECTED`, `VIRTUAL_THREAD_CARRIER_PINNING`,
  `SMR_UNRESOLVED_THREAD`.
- 새 `tables.lock_graph` — `{ lock_addr, owner, waiters: string[] }`.
- 새 `tables.deadlock_cycles` — `{ cycle: string[] }` (lock graph DFS).
- 새 `tables.dump_overview` — 덤프당 한 행: 쓰레드 수, 상태 분포,
  resolved parser plugin id.
