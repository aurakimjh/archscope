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
  | "gc_log"
  | "jfr_recording"
  | "native_memory"
  | "thread_dump"
  | "thread_dump_multi"
  | "thread_dump_locks"
  | "exception_stack"
  | "nodejs_stack"
  | "python_traceback"
  | "go_panic"
  | "dotnet_exception_iis"
  | "otel_logs"
  | "metrics_snapshot"
  | "observability_evidence"
  | "server_log"
  | "database_slow_query"
  | "broker_log"
  | "kubernetes_evidence"
  | "trace_import"
  | "profile_evidence"
  | "stitched_evidence"
  | "api_contract_analysis"
  | "architecture_docs"
  | "incident_timeline"
  | "slo_golden_signals"
  | "service_flow"
  | "jennifer_profile"
  | "profiler_collapsed"
  | "profiler_jennifer"
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

### Go Struct

```go
type AnalysisResult struct {
    Type        string         `json:"type"`
    SourceFiles []string       `json:"source_files"`
    CreatedAt   string         `json:"created_at"`
    Summary     map[string]any `json:"summary"`
    Series      map[string]any `json:"series"`
    Tables      map[string]any `json:"tables"`
    Charts      map[string]any `json:"charts"`
    Metadata    Metadata       `json:"metadata"`
}
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

현재 Go/Wails 계약 강화 범위는 `apps/engine-native`에 구현된 analyzer 및
derived result type을 포함한다.

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

Go `internal/models.AnalysisResult` struct가 신규 analyzer 작업의 외부
transport model이다. 계약 강화 계층은 `summary`, `series`, `tables`,
`metadata` 내부에 들어가는 type별 필수 key를 정의하고, frontend TypeScript
interface가 같은 JSON envelope을 mirror한다.

### 이번 범위에 포함

- `internal/models` 및 analyzer-specific package의 Go result contract
- Renderer, workspace, chart code에서 사용할 대응 TypeScript interface
- 필수 key, 값 type, unit 문서화
- 향후 migration을 위한 `metadata.schema_version` 유지

### 이번 범위에서 제외

- 모든 nested field에 대한 runtime validation
- Chart Studio template schema
- Dashboard sample data를 canonical contract로 취급하는 것. `dashboard_sample`은 UI fixture data로만 본다.

### Versioning 규칙

- Optional field 추가는 동일 `schema_version`에서 허용한다.
- Required field 제거 또는 rename은 `schema_version` bump가 필요하다.
- Numeric field는 unit이 명확하지 않으면 key에 `_ms`, `_sec`, `_percent` 같은 unit suffix를 둔다.
- Malformed-input 처리를 지원하는 parser의 diagnostics는 `metadata.diagnostics` 아래에 둔다.
- Portable parser debug log는 별도 JSON artifact이며 `AnalysisResult` 내부 field가 아니다. Parser 개발을 위해 redacted raw context, `field_shapes`, partial match data, traceback data를 포함할 수 있다.

## 공통 Ingestion Metadata

새 Mid-Term Plus evidence family는 local file에서 결과를 만들 때
`metadata.source_metadata` 아래에 normalized source metadata를 붙인다. 기존
analyzer도 summary, series, table contract를 바꾸지 않는 additive field로 이
metadata를 채택할 수 있다.

`SourceMetadata` shape:

| Field | Type | 의미 |
|---|---|---|
| `source_kind` | string | `access_log`, `trace_import`, `server_log`, `database_slow_query`, `broker_log` 같은 evidence source family |
| `source_format` | string | 감지되었거나 사용자가 선택한 parser format |
| `product` | string | nginx, OpenTelemetry, PostgreSQL, Kafka 같은 product 또는 ecosystem 이름 |
| `product_version` | string | source가 제공하는 경우의 product version |
| `host` | string | 노출해도 안전한 경우의 host identity |
| `service` | string | service 또는 workload name |
| `environment` | string | environment label |
| `file` | object | basename, extension, size, path가 아닌 `sanitized_id`를 담는 sanitized file identity |

Cross-source stitching은 analyzer가 결과 크기를 무한히 늘리지 않고 key를 만들
수 있을 때 `metadata.correlation_keys`를 사용한다. Canonical key model은
다음을 포함한다.

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

Tenant/customer identifier는 저장 전에 sanitize하거나 명시적으로 allow-list된
값만 사용해야 한다. `stable_id`는 비어 있지 않은 normalized field를 hash한
값이며 raw path data를 노출하지 않고 matching에 사용할 수 있다.

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

Optional access/edge extension은 additive이다. 기존 consumer는 이 field를
무시하고 위의 필수 access-log field만 계속 사용할 수 있다.

Optional `summary` fields:

| Field | Type | Unit / 의미 |
|---|---|---|
| `detected_format_count` | integer | 파싱 record에서 관측된 source format 수 |
| `upstream_service_count` | integer | 관측된 upstream service 또는 cluster 수 |
| `route_count` | integer | 관측된 gateway 또는 mesh route 수 |
| `service_edge_count` | integer | 추론된 caller-to-upstream edge 수 |
| `gateway_avg_latency_ms` | number | 평균 gateway latency, milliseconds |
| `gateway_p95_latency_ms` | number | 95 percentile gateway latency, milliseconds |
| `retry_count` | integer | Edge log가 보고한 retry attempt 합계 |
| `termination_error_count` | integer | 비정상으로 판단한 HAProxy termination-state row 수 |

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

`tables.service_dependencies.error_rate`는 Trace Import service-dependency
table과 맞춘 `0.0`부터 `1.0`까지의 fraction이다.
`tables.route_stats.error_rate`는 access-log summary error rate와 같은 percent
값이다. `sample_records`는 optional `source_format`, `route`,
`upstream_service`, `trace_id`, `request_id` field도 포함할 수 있다.

Optional `metadata.source_format_diagnostics`는 access/edge auto-detect evidence를
`selected_format`, `auto_detect_enabled`, `detected_format_count`,
`parsed_by_format`, `skipped_by_reason`으로 기록한다.

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

### Server Log Result

`type`: `server_log`

Server-log contract는 Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere,
GlassFish/Payara, nginx error log, Apache error log의 application-server 및
web-server error evidence를 다룬다.

Core `summary` fields:

| Field | Type | 의미 |
|---|---|---|
| `total_events` | integer | 파싱된 server-log event 수 |
| `error_count` | integer | Error/severe/fatal record 수 |
| `warning_count` | integer | Warning record 수 |
| `startup_count` | integer | Startup/lifecycle event 수 |
| `deployment_event_count` | integer | Deployment 관련 event 수 |
| `datasource_event_count` | integer | JDBC/datasource/pool event 수 |
| `stuck_thread_count` | integer | Stuck-thread event 수 |
| `hung_thread_count` | integer | Hung-thread event 수 |
| `thread_pool_pressure_count` | integer | Executor/thread-pool pressure 수 |
| `worker_error_count` | integer | nginx/Apache worker/upstream error 수 |
| `correlated_event_count` | integer | Trace 또는 request ID를 가진 event 수 |

Core `tables` fields:

| Field | Row shape |
|---|---|
| `events` | `{ timestamp, severity, product, component, thread, host, service_name, event_type, message, trace_id, request_id }` |
| `deployment_events` | `events`와 같은 row shape. Deployment evidence만 포함 |
| `datasource_events` | `events`와 같은 row shape. Datasource/pool evidence만 포함 |
| `thread_events` | `events`와 같은 row shape. Stuck/hung/thread-pool evidence만 포함 |
| `worker_errors` | `events`와 같은 row shape. nginx/Apache worker evidence만 포함 |
| `correlation_candidates` | Trace 또는 request ID를 가진 event |

Finding code는 `SERVER_SEVERE_ERRORS`, `DEPLOYMENT_FAILURE`,
`DATASOURCE_POOL_WARNING`, `STUCK_THREAD_DETECTED`, `HUNG_THREAD_DETECTED`,
`THREAD_POOL_PRESSURE`, `WORKER_ERROR_PRESENT`, `MANAGED_SERVER_HEALTH`를
포함한다.

### Observability Logs And Metrics

`otel_logs`는 JSONL/NDJSON 형태의 OTel log record와 OTLP Logs JSON
`resourceLogs`를 받는다. Severity, body, attributes, resource metadata,
service name, trace ID, span ID, parent span ID를 보존한다. Analyzer table은
records, cross-service traces, trace service paths, failure propagation,
resource groups, error signatures, severity bursts를 포함한다.

`metrics_snapshot`은 Prometheus/OpenMetrics text snapshot을 받는다. Metric
sample count, per-metric distribution, bounded raw sample, latency/traffic/errors/
saturation용 `golden_signal_candidates`를 emit한다.

`observability_evidence`는 Loki query JSON export, Tempo trace JSON export,
Grafana dashboard JSON export를 받는다. Loki와 Tempo record는 trace ID로
Incident Timeline에 연결할 수 있고, Grafana row는 raw metric truth가 아니라
Evidence Board와 report pack context를 위한 dashboard-panel reference로
저장한다.

### Database Slow Query Result

`type`: `database_slow_query`

Database evidence contract는 PostgreSQL text/csvlog, MySQL/MariaDB slow query
log, MongoDB profiler JSON, Redis slowlog text, SQL Server extended events JSON,
PostgreSQL/MySQL EXPLAIN JSON을 다룬다. Core row는 sanitized SQL fingerprint,
duration, lock wait, error, row count, database/schema, collection 또는
operation, plan summary를 보존한다. `tables.service_dependencies`는
application-to-database edge를 노출해 Service Flow가 database evidence를 trace
및 access-log dependency 옆에 배치할 수 있게 한다.

Finding code는 `SLOW_QUERY_PRESENT`, `LOCK_WAIT_PRESENT`, `DB_ERRORS_PRESENT`,
`HIGH_ROWS_EXAMINED`, `EXPLAIN_PLAN_IMPORTED`를 포함한다.

### Broker Log Result

`type`: `broker_log`

Broker evidence contract는 Kafka, RabbitMQ diagnostics/server log, Pulsar, NATS,
ActiveMQ를 다룬다. Row는 rebalance, replication/ISR, KRaft quorum, queue
pressure, dead-letter, partition, slow-consumer, authorization, store usage,
broker-health event를 normalize한다. `tables.service_dependencies`는 Service
Flow를 위한 application-to-broker edge를 projection한다.

Finding code는 `BROKER_REBALANCE`, `BROKER_REPLICATION_ISSUE`,
`BROKER_QUEUE_PRESSURE`, `BROKER_DEAD_LETTER`, `BROKER_HEALTH_EVENT`,
`BROKER_SLOW_CONSUMER`, `BROKER_AUTHORIZATION_FAILURE`를 포함한다.

### Kubernetes, Container, And Cloud Audit Evidence

`type`: `kubernetes_evidence`

Platform evidence contract는 cluster, namespace, workload, pod, container,
node, image, restart count, owner-style object identity, kubelet 및
container-runtime event, cloud audit actor/operation/resource field를 보존한다.
지원 입력은 `kubectl get events -o json`, pod JSON, kubelet log,
containerd/CRI-O/Docker daemon log, AWS CloudTrail JSON, GCP Cloud Audit Logging
JSON, Azure Activity Logs JSON이다.

Finding code는 `K8S_OOMKILLED`, `K8S_RESTARTS_PRESENT`, `K8S_EVICTION`,
`K8S_SCHEDULING_ISSUE`, `K8S_IMAGE_PULL_ISSUE`, `K8S_READINESS_ISSUE`,
`K8S_NODE_PRESSURE`, `CLOUD_AUDIT_SECURITY_EVENT`를 포함한다.

### 통합 Profile Evidence

`type`: `profile_evidence`

통합 profile evidence contract는 runtime profiler 입력을 language-tagged stack
sample로 정규화한 뒤 기존 flamegraph analyzer 경로를 재사용한다. 지원 selector는
generic pprof `.pb.gz`, async-profiler collapsed/HTML, py-spy raw, rbspy raw,
dotnet-trace speedscope export를 포함한 speedscope JSON, perf collapsed stack,
JFR JSON stack sample, Ruby StackProf JSON, PHP Excimer/Tideways 계열 JSON,
Xdebug cachegrind text, Swift/generic async stack, Pyroscope/Phlare snapshot,
Parca 계열 profile JSON이다.

핵심 `summary` fields:

| Field | Type | 의미 |
|---|---|---|
| `total_samples` | integer | 정규화된 sample value 합계 |
| `unique_stacks` | integer | 정규화 후 distinct collapsed stack key 수 |
| `runtime_count` | integer | distinct runtime label 수 |
| `language_count` | integer | distinct language label 수 |
| `native_samples` | integer | stack에 native frame이 포함된 sample 수 |
| `managed_samples` | integer | stack에 managed frame이 포함된 sample 수 |
| `async_frame_samples` | integer | async/await/continuation frame이 포함된 sample 수 |
| `thread_count` | integer | thread label이 있을 때 distinct thread 수 |
| `process_count` | integer | process/PID label이 있을 때 distinct process 수 |
| `max_stack_depth` | integer | 최대 정규화 stack depth |

`tables.frames` row는 `name`, `function`, `file`, `line`, `language`,
`runtime`, `kind`, `native`, `async`, `samples`를 보존한다.
`charts.flamegraph`와 `charts.drilldown_stages`는 profiler `FlameNode`
contract를 재사용한다.

브라우저/V8 입력(`chrome-trace-json`, `v8-cpuprofile`)은 envelope 변경 없이
다음을 추가한다.

- `Sample.Value`는 마이크로초(`value_unit: "microseconds"`)이며 `samples[]`가
  정본, `hitCount`는 검증용이다. `hitCount`-only 입력은 duration을 보고하지
  않는 집계 경로를 탄다.
- sample은 `ts_us` label(프로파일 시작 기준 오프셋)을 갖는다. collapse 이전에
  `tables.cpu_sample_runs`(연속 동일 스택 run 상위 N —
  `start_us`/`end_us`/`duration_us`/`sample_count`/`top_frame`)와
  `series.cpu_activity`(CPU 점유율 버킷, 최대 1,000개)를 만든다. `ts_us`가
  없는 포맷, 다운샘플된 입력, `hitCount`-only 입력에서는 두 키를 생성하지
  않는다.
- 100ms 임계를 넘는 run은 `SAMPLED_CPU_HOTSPOT` warning finding이 된다.
  문구는 의도적으로 "롱태스크"를 피한다 — 샘플 관측 구간이지 브라우저 task
  경계가 아니다.
- 대형 입력은 256 MiB 바이트 가드와 500,000 sample 상한으로 스트리밍 처리하며,
  다운샘플 시 `metadata.partial_result`, `metadata.downsampled_from_samples`,
  `metadata.downsampled_to_samples`와 `PROFILE_DOWNSAMPLED` diagnostic이
  남는다.

### HTTP Capture Result

`type`: `http_capture`

Phase 1은 리댁션된 HAR 파일을 bounded envelope로 가져온다
(`http-capture analyze --in file.har`). parser는 `log.creator`/`log.browser`로
생성 방언(Chrome, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia,
`generic`)을 판별하고, entry 수를 상한(기본 100,000, 초과 시 `HAR_ENTRY_CAP`
warning)으로 자르며, 잘못된 entry는 `INVALID_HAR` diagnostic으로 건너뛴다.

| Envelope field | 내용 |
|---|---|
| `summary` | `total_transactions`, `error_transactions`, `unique_hosts`, `unique_endpoints`, `source_format`, `dialect` |
| `series.timeline` | 분 단위 요청/오류/전송량 row (top-N 상한) |
| `tables` | `transactions`, `endpoints`(`method path` key), `hosts` — 각 top-N 상한 (기본 50) |
| `metadata.extra.http_capture` | `dialect`, `fidelity: "har_import"`, `redaction`, `detail_storage: "inline_phase1"` |
| `metadata.extra.capture_aggregate_snapshot` | 향후 live capture와 공유하는 결정적 집계 snapshot — 오프라인 start-order와 라이브 completion-order projection의 비교 가능성 유지 |

실패 응답이 있으면 `HTTP_CAPTURE_ERRORS` warning finding이 추가된다. Phase 1은
상세를 inline으로 저장하며, cursor pagination이 있는 버전 붙은
`CaptureSessionStore`는 live capture부터 적용된다
(`docs/ko/SYSTEM_HTTP_CAPTURE.md` 7.6 참조).

### Stitched Evidence

`type`: `stitched_evidence`

Stitched evidence는 기존 `AnalysisResult` JSON 파일들을 읽고 trace ID, span ID,
parent span ID, request/correlation ID, TXID/GUID, tenant/customer ID,
pod/container/host, PID 같은 correlation key로 row를 연결한다. 또한 timestamp
window와 정규화된 service alias로 낮은 confidence의 보조 match를 만들고,
profile label에 trace metadata가 있으면 `profile_evidence` sample을 trace와
연결한다.

핵심 table:

| Field | Row shape |
|---|---|
| `matches` | `{ key_kind, key_value, match_reason, confidence, alias_reason, time_window_seconds, event_count, source_types, evidence_refs, first_seen, last_seen, services }` |
| `gaps` | `{ code, severity, message, source_type, evidence_ref, timestamp, service, correlation }` |
| `evidence_nodes` | source type, table, timestamp, service, target, message, evidence ref, correlation key, bounded raw row를 가진 정규화 source row |
| `match_drilldowns` | confidence, reason, source node ID, evidence ref, raw source row를 가진 drilldown-ready match row |
| `service_dependencies` | database/broker evidence가 request 또는 trace evidence와 match될 때 Service Flow에 투영하는 stitched service edge. match status와 match reason을 포함한다. |

Gap finding code는 `MISSING_TRACE_ID`, `DROPPED_PARENT_SPAN`,
`UNMATCHED_REQUEST_LOG`, `UNMATCHED_DATABASE_CALL`, `UNMATCHED_BROKER_EVENT`를
포함한다.

### API 및 Event Contract Analysis

`type`: `api_contract_analysis`

API contract analysis는 OpenAPI operation을 access-log `AnalysisResult` table과
대조하고, AsyncAPI channel을 broker evidence와 대조한다. 원본 log를 다시
parsing하지 않는 second-pass analyzer이며, `api-contract analyze`가 기존 result
JSON 파일과 contract spec을 읽는다.

핵심 table:

| Field | Row shape |
|---|---|
| `operations` | observed count, matched route ref, max p95 latency, max error rate를 가진 OpenAPI operation row |
| `observed_routes` | path template으로 정규화한 access-log route |
| `undocumented_routes` | OpenAPI와 match되지 않은 observed route |
| `unused_operations` | access log에서 관측되지 않은 OpenAPI operation |
| `slow_operations` | 설정한 latency threshold를 넘은 observed route |
| `high_error_operations` | 설정한 error-rate threshold를 넘은 observed route |
| `event_channels` | observed broker evidence ref를 가진 AsyncAPI channel row |
| `undocumented_event_channels` | AsyncAPI에 문서화되지 않은 broker channel |
| `unused_event_channels` | broker evidence에서 관측되지 않은 AsyncAPI channel |

Finding code는 `UNDOCUMENTED_API_ROUTE`, `UNUSED_API_OPERATION`,
`SLOW_API_OPERATION`, `HIGH_ERROR_API_OPERATION`,
`UNDOCUMENTED_EVENT_CHANNEL`, `UNUSED_EVENT_CHANNEL`을 포함한다.

### Architecture Documentation Package

`type`: `architecture_docs`

Architecture documentation package는 기존 `AnalysisResult` JSON evidence에서
생성된다. 최종 문서를 바로 렌더링하지 않고, source evidence reference를 보존한
상태로 arc42 및 ADR workflow에 넣을 수 있는 review-ready row를 만든다.

핵심 table:

| Field | Row shape |
|---|---|
| `arc42_sections` | `{ section_id, title, draft_markdown, evidence_tables }` |
| `context_services` | Service Flow, stitched evidence, contract evidence에서 추론한 service/component |
| `interfaces` | API/event contract analysis에서 가져온 HTTP API operation 및 event channel |
| `runtime_views` | evidence-backed runtime interaction, match, gap |
| `deployment_views` | Kubernetes/container/cloud deployment row |
| `quality_requirements` | deterministic finding에서 파생한 quality scenario |
| `risks` | severity, impact, mitigation, evidence ref를 가진 architecture risk |
| `adr_drafts` | decision, context, alternatives, tradeoffs, consequences, evidence ref를 가진 ADR draft row |

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

## Browser Audit Evidence Result

`type`: `browser_audit_evidence`

`browser import --format lighthouse-json`은 local Lighthouse report JSON을
입력받는다. Parser는 Lighthouse가 기록한 category/audit score를 그대로
보존하고, ArchScope가 metric 값에서 threshold를 다시 계산하지 않는다.
requested/final/network-request URL은 결과에 들어가기 전에 리댁션한다.

주요 projection:

- `summary` — Lighthouse 버전, 리댁션된 URL, 수집 mode, performance score,
  존재하는 FCP/LCP/INP/CLS/TBT/Speed Index/TTI/server-response metric,
  request/byte count, run warning/runtime error 상태.
- `series.category_scores` — report category score.
- `series.core_metrics` — 존재하는 browser metric 값과 report score.
- `series.resource_type_distribution` — resource type별 request/transfer byte.
- `tables.audits` — 낮은 score 우선, `top_n` 상한.
- `tables.network_requests` — 큰 transfer 우선, `top_n` 상한.
- `tables.resource_summary` — Lighthouse resource-summary row.

Deterministic finding code는 `LIGHTHOUSE_PERFORMANCE_POOR`,
`LIGHTHOUSE_AUDITS_POOR`, `LIGHTHOUSE_RUNTIME_ERROR`,
`LIGHTHOUSE_RUN_WARNINGS`다. 기본 입력 상한은 64 MiB이며 parser diagnostics는
invalid envelope과 bounded extraction 상한을 보고한다.

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

현재 event source는 analyzer finding, access-log error/latency series, GC
alert, JFR pause/notable event, exception row, thread-dump contention/deadlock
table, trace-import error/critical-path row를 포함한다.
Timeline group은 correlation ID(`trace_id`, `request_id`, `correlation_id`,
`transaction_id`, `thread_id`, thread)를 먼저 사용하고, service/endpoint hint,
마지막으로 category/source fallback key를 사용해 만든다. Exportable result는
`tables.groups`, group별 event count, ranged event count, correlated event
count를 포함하므로 multi-file incident를 flat event list가 아니라 incident
slice 단위로 검토할 수 있다.
`tables.narrative`는 이 group에서 파생한 deterministic incident narrative
step을 담는다. 각 step은 order, group key/label, severity, summary, event ID,
source result ID, evidence ref를 포함하며 AI-generated prose가 아니다.

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

현재 Golden Signal source는 access-log latency/throughput/error metric, access
edge service-dependency latency/traffic/error metric, trace-import
service/dependency/critical-path metric, Jennifer MSA external-call 및 network-gap
metric, exception distribution, GC pause, memory-space, OOM 및 long-pause alert,
JFR pause/sample/thread metric, thread-dump lock/contention/deadlock metric, heap
및 worker count 같은 JVM metadata를 포함한다.

## Service Flow Projection

첫 Service Flow 구현은 Analysis Workspace 결과로부터 만든 Wails session
projection이다. Trace Import `service_dependencies`, Access Log
`tables.service_dependencies`, Jennifer `tables.msa_edges`, Jennifer unprofiled
external-call group을 공통 service-edge view로 통합한다.

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
  customer_summary
  provenance
  artifacts
```

`customer_summary`는 고객-facing overview와 key observation 목록이다. 각
observation은 evidence card ID 또는 source evidence reference를 포함하며,
raw evidence appendix는 pack에 그대로 남는다. `provenance`는 source result
metadata, analyzer option, captured evidence card, deterministic finding,
derived artifact reference, 존재하는 경우 optional AI interpretation provenance를
보존한다. `artifacts`는 현재 evidence card, exportable `incident_timeline`
result, SLO analysis, Service Flow export payload를 포함한다.
AI interpretation provenance는 source gate status와 accepted AI finding을
분리해 기록하며, 전체 interpretation이 evidence gate를 통과한 경우에만
accepted AI finding을 포함한다.

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
