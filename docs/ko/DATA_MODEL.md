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
  request/byte count, run warning/runtime error 상태, 명시적인
  `imported_lighthouse_report` score provenance/non-recalculation 고지.
- `series.category_scores` — report category score.
- `series.core_metrics` — 존재하는 browser metric 값과 report score.
- `series.resource_type_distribution` — resource type별 request/transfer byte.
- `tables.audits` — 낮은 score 우선, `top_n` 상한.
- `tables.network_requests` — 큰 transfer 우선, `top_n` 상한.
- `tables.resource_summary` — Lighthouse resource-summary row.

버전이 지정된 `metadata.browser_audit_contract`는 desktop page가 소비할
series/table key, Evidence Board source path, bounded table 크기, score provenance,
report export format을 고정한다. category, metric, audit, network-request,
resource-summary 각 row는 Workspace/Evidence Board capture를 위한 안정적인
`source_ref`를 가진다. Source-file provenance는 `metadata.source_metadata`에
유지되며 UI는 별도 backend method를 호출할 필요가 없다.

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

## 로컬 진단 확장 계약 (2026-11-11, T-628 공통 기반 구현)

[통합 설계 §16](./LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md#16-analysisresult와-확장-계약)의
새 계약은 기존 `AnalysisResult` envelope를 유지한다. 공통 기반은 T-628에서
구현했으며 전체 profile/metric 이행과 정정된 emitter는 후속 제안으로 남는다.

| 항목 | 제안과 호환 규칙 |
|---|---|
| 기준 프로파일 | `ProfileSet → profile analyzer → profile_evidence`; legacy endpoint는 facade로 유지 |
| 측정 metadata | JSON Pointer/collection column descriptor; scalar를 객체로 일괄 교체하지 않음 |
| 수치 정정 | 소수/0 weight·evented duration·복수 profile·pprof 의미 수정은 제안 `profile_evidence` 0.2.0으로 분리하고 원본 재계산 |
| unavailable | schema가 선언한 sample count·시간·interval만 nullable; 0과 구별, UI/export/diff에 전파 |
| 미지 metadata | common extension 보존과 legacy typed-key 우선 `json.RawMessage` 왕복 검증; 구버전 binary의 보존은 가정하지 않음 |
| 버전 | result, envelope, extension, adapter, pack, index, case, bundle, capture, visual spec, renderer/template/font 축 구분 |
| 신구 결과 비교 | reader range·metric·unit·모집단·semantic revision 확인 후 허용; 미지원 의미로 계산 금지 |
| GPU와 AI | GPU telemetry는 `metrics_timeseries` semantic view; LLM 앱 trace는 `trace_import` 확장, serving은 원본 참조 |

버전별 owner, 선언 위치, 읽기 전용·거부·재계산 정책은 통합 설계 §16.9를
따른다. 상세 index는 재생성 가능한 cache이며 schema 불일치 시 원본에서 다시
만든다. 원본이 없으면 snapshot은 보존하고 detail capability를 제한한다.

### T-628 측정과 호환 상세 계약

설계 기준은 `83b8ff9`와 상세 계약 커밋 `eb3e782`이다. T-628의 공통 decoder·
metadata·handle 기반 bridge를 구현했으며, 실제 검증 범위는 문서 끝에 기록한다.
새 profile_evidence 0.2.0 emitter는 구현하지 않았다. 실행 흐름은
[파서 상세 설계](./PARSER_DESIGN.md#실행-인터페이스와-소유권)를 따른다.

#### 유지할 외부 구조와 읽기 경계

최상위 `type`, `source_files`, `created_at`, `summary`, `series`, `tables`,
`charts`, `metadata`를 유지한다. 정상 결과의 빈 section은 `{}`, source 목록과
findings는 `[]`를 사용한다. 실패·취소 상태는 별도 job 응답이며 정상 결과의
section을 전부 비우는 방식으로 표현하지 않는다. 신규 필드는 `metadata`의
versioned extension 또는 type별 payload schema 안에서만 추가한다.

새 공통 read boundary는 파일 크기·depth·문자열/배열 예산과 중복 JSON key를 검사하고,
`json.Decoder.UseNumber`에 해당하는 정밀도 보존 decode 후 type별 reader를 선택한다.
중복 key의 last-wins 동작이나 NaN/Infinity를 허용하지 않는다. unknown optional
extension은 bounded opaque data로 보존하며 문자열·수치에서 실행 코드나 경로를 추출하지 않는다.

설계 시점의 common metadata는 기본 숫자 decode를 사용했고 legacy profiler에는
unknown-key 저장이 없었다. T-628은 bounded UseNumber decoder와 정확히 표현 가능한
안전한 legacy projection, profiler RawMessage 저장을 추가했다. 기존 배포 바이너리의
보존 능력까지 소급하지 않으며, JS object를 거치는 경로의 정밀도는 별도 계약이다.

#### 측정 descriptor와 수치 표현

`metadata.measurement`의 key는 결과 내부 JSON Pointer다. 첫 버전은 `/summary/<key>`의
scalar와 `/series/<key>`, `/tables/<key>`의 collection descriptor만 허용한다.
외부 URI, wildcard, 임의 식은 받지 않는다. `~0`/`~1` escape를 처리하고 잘못된 escape,
없는 target, 정의하지 않은 collection column은 validation 오류다. 임의 중첩 chart
payload는 해당 ResultDescriptor가 등록한 별도 schema로 검증한다.

| descriptor 필드 | 의미와 validation |
|---|---|
| `measurement_kind`, `unit` | CPU/wall/count/bytes 등 실제 의미와 원래 단위. 이름 문자열만으로 unit 변환을 추론하지 않음 |
| `scope`, `population_ref` | 선택 profile/기간/entity와 유효 관측 모집단의 참조. 다른 분모를 같은 비교로 처리하지 않음 |
| `aggregation` | sum/count/observed 등 등록된 연산 의미. histogram percentile과 요청 원본 percentile을 구분 |
| `numeric_encoding` | `json_number`, `exact_integer_string`, `decimal_string`. schema가 허용한 target type과 일치해야 함 |
| `availability`, `reason` | observed/derived/estimated/unavailable. unavailable이면 명시적 null과 이유가 필요; derived/estimated는 산출 근거·방법 참조 필요 |
| `evidence_ref` | 같은 결과/사건에서 해석되는 근거 ID. exporter가 임의 URL/파일을 열게 하지 않음 |
| `columns` | collection의 column별 위 의미. row별 availability/reason을 담는 column 이름도 schema에 선언 |

결과 전체의 합계와 선택 view의 합계를 구분한다. 저장된 결과의 total이 전체 입력 기준이면
필터 적용 화면은 원본 total을 고치는 대신 selection revision과 분모가 고정된 view를 사용한다.
`total_samples`를 duration으로 변경하지 않으며 evented 프로파일의 weight에는
`total_weight`와 시간 descriptor를 사용한다. 실제 sampling count를 모르면 count는 null이다.

정수는 내부에서 정확한 정수/scale로, 소수는 정밀 표현으로 계산한다. JavaScript 안전
정수 범위를 넘는 **새 계약의 알려진 값**은 exact_integer_string으로 전달하고, 정확한
소수 보존이 필요한 값은 decimal_string을 쓴다. 기존 number 필드의 type을 schema 변경 없이
string으로 바꾸지 않는다. 표시용 부동소수 변환은 renderer 경계에서만 수행하며 그것으로
다시 합계·diff·export 수치를 계산하지 않는다. 문자열 수치도 유효 문법·범위·scale을 검증한다.

다음은 완결된 **설명용** `profile_evidence` 예다. 현재 구현의 출력이나 배포된 schema가
아니며, evented duration과 sample count 부재의 분리를 보여 준다.

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

실제 자료에는 해당 결과 schema가 요구하는 evidence/population reference를 추가한다.

#### 미측정·0·잘못된 값의 구분

| 입력 상태 | 저장과 표시 | 비교·export |
|---|---|---|
| 관측된 0 | number/string encoding에 따른 유효값 | 차이에 사용. baseline=0이면 변화율은 계산하지 않음 |
| schema가 허용하는 미측정 | null + unavailable reason | UI는 `—`, chart는 gap, CSV는 빈 cell과 이유, diff는 unavailable |
| optional 필드 자체가 없음 | key 생략, capability에 누락 이유 | null이나 0을 임의로 추가하지 않음 |
| 잘못된 수치·단위·encoding | result validation 실패 | 정상 export·비교에 사용하지 않음 |

#### Unknown metadata와 bridge 왕복

common metadata는 Extra API와의 호환을 유지하면서 unknown number를 정밀도 보존 방식으로
읽는다. legacy profiler에는 `map[string]json.RawMessage`의 unknown-key 저장과 custom
marshal/unmarshal을 추가한다. 공통 typed key가 항상 우선하며, omitempty로 생략할 key도
Extra에서 되살리지 않는다. decode 실패 시 수신 객체를 부분 수정하지 않고, 성공 시에는
이전 객체의 unknown key가 남지 않게 한다.

새 decoder의 `json.Number`를 기존 consumer의 `float64` type assertion에 그대로 전달하지
않는다. number accessor에서 정수·scale·range를 검증하고, 기존 consumer에는 검증된 호환
projection을 전달한다. 원래 값을 보존할 수 없으면 계산을 거부한다. wire 호환과 Go 내부
자료형 호환을 별도로 검증한다.

unknown extension의 큰 정수를 JavaScript object로 펼쳤다가 저장하면 정밀도가 손실될 수
있다. 새 generic/job/detail 경로는 backend에 immutable 결과를 유지하고 UI에는 result
handle과 bounded view를 전달한다. opaque metadata를 전달해야 하면 JSON text로 취급하며
UI에서 parse/stringify해 다시 저장하지 않는다. JSON export는 backend의 원래 revision에서
생성한다. 기존 Wails full-result 경로는 별도 왕복 검증 후 연결한다. Go serializer 수정만으로
end-to-end 정밀도가 보장됐다고 판단하지 않는다.

#### 호환 판정과 migration

reader는 envelope → result payload → extension → 대상 operation 순서로 판정한다.
`metadata.envelope_schema_version`이 없으면 legacy envelope로 읽지만 payload version까지
0.1.0이라고 추정하지 않는다. type/version이 불명확하면 bounded 표시와 원본 보존으로
제한한다. ResultDescriptor는 operation별 required extension과 reader range를 정의한다.
새로운 필수 extension에 의존하면 payload semantic version도 갱신하며 optional block 추가로
취급하지 않는다.

| 조건 | 허용 동작 | 금지 동작 |
|---|---|---|
| 알려진 legacy payload | version 전용 reader·기존 snapshot 표시, 명확히 정의된 기존 계산 | 불명확한 unit/interval을 새 기본값으로 보충 |
| 지원하는 새 payload·required extension | 검증된 capability의 계산·저장·비교 | imported capability 문자열만으로 기능 활성화 |
| 미지 optional block만 있음 | opaque 보존, 알려진 독립 operation | unknown block 삭제·의미 추정·미지 script 실행 |
| 미지원 payload/required block | 원본 보존·가능한 bounded snapshot 표시·업데이트 안내 | 수치 계산, 손실 있는 재저장, schema 번호만 변경 |
| stale/rebuild 대상 index | hash가 일치하는 원본에서 재생성 | 다른 source를 같은 revision의 detail로 연결 |

이 판정은 새 reader의 계약이며 미갱신 구형 binary에 강제할 수 없다. 새 payload를 구형 API로
자동 역변환하지 않는다. migration은 원본 hash·capture/options를 검증해 새 revision으로
재분석하거나 명시적인 lossless projection을 사용한다. 과거 weight에 duration label만 붙이지
않는다. 비교는 kind/unit/population/semantic revision을 확인하고 정밀도·관측 조건이 부족하면
이유와 함께 거부한다. 읽기 전용 표시와 계산 호환을 구분한다.

#### T-628 wire 적합성 사례

다음 사례는 구현 회귀 테스트에 연결했다. Go·bridge·프런트엔드 수준과 실제 UI 실행을
구분한 검증 기록은 문서 끝과 T-628 evidence에 있다.

| ID | 사례 | 필수 기대 결과 |
|---|---|---|
| W01 | metadata에 unknown object/array/null/9007199254740993 포함 | Go load-save의 의미·정수 일치, UI 경유 export에서도 정밀도 유지 |
| W02 | 고정밀 decimal string과 알려진 0 | type/scale 보존, 표시 반올림으로 계산값 덮어쓰기 금지 |
| W03 | typed key와 Extra 충돌, nil diagnostics | typed/생략 규칙 우선, Extra에서 되살리지 않음 |
| W04 | 같은 객체에 두 번 decode, 두 번째 입력은 타입 오류 | 성공 시 stale extension 제거, 실패 시 이전 값 유지 |
| W05 | duplicate key, 깊은 JSON, NaN/Infinity | bounded read에서 거부, 정상 결과 commit 없음 |
| W06 | 잘못된 Pointer·없는 target·미정의 column·encoding 불일치 | result validation 실패 |
| W07 | null/0/absent를 UI·CSV·HTML·diff로 전달 | availability·분모 유지, 미측정을 0으로 처리하지 않음 |
| W08 | old/new payload, unknown optional/required block | 호환표에 따라 계산과 재저장 제어 |
| W09 | result version만 바꾼 과거 profile | 원본 재분석 요구, 기존 count를 duration으로 재해석하지 않음 |
| W10 | unknown JSON의 Wails/Report Pack 왕복 | backend 원본 또는 정밀도 보장 경로 사용, script/URL 실행 금지 |

## Schema 0.2.0 — additive 변경

이 절은 `0.2.0-beta` 이후 일부 분석기에서 도입한 0.2.0 계열 변경을
기록한다. 모든 결과가 같은 버전으로 전환된 것은 아니다. `schema_version`은
result type과 생성 경로별로 확인해야 하며, 아래 additive 항목과 후속 nullable
정정의 호환 조건을 구별한다.

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

#### MSA 드릴다운 결과 계약 (T-557, 버전 1.0.0)

`ProjectJenniferDrilldown({result, selection})`은 원본
`jennifer_profile` AnalysisResult를 사용하며 입력 파일을 다시 열지 않는다.
`selection.mode`는 `single`(필수 `guid`, 선택 `txid`) 또는
`average`(필수 `signature_hash`, 선택 `application` URL)다. 선택 필드를
생략하면 그룹 전체를 사용한다. 잘못되거나 혼합된 선택, 없는 식별자,
선택 그룹 안의 중복 TXID, 이미 드릴다운한 결과의 재투영은 오류로 처리한다.

단일 TXID는 순환 방문을 막으며 matched 하위 호출을 탐색한다. 평균은
기존 signature 통계에서 제외된 그룹을 먼저 빼고, GUID마다 해당 URL의
가장 느린 프로파일 하나를 고른다. 응답시간 동률이면 입력 순서를 유지한다.
SQL·메소드 조인은 GUID와 TXID를 함께 사용한다. GUID가 없는 행만 그룹의
TXID 목록으로 연결한다.

공통 봉투와 type은 유지한다. `series.drilldown_scopes`에 `group`,
`profile_rows`, `timeline_edges`, `topology_edges`, `service_network_edges`,
`slow_sql_rows`, `network_time_by_txid`, `selected_value`,
`selected_application`, `selected_txid`, `is_whole_transaction`을 낸다.
`series.guid_groups`와 `tables.profiles`, `msa_edges`, `slow_sql_events`,
`network_prep_methods`, `method_hotspots`에는 선택 범위의 증거만 담는다.
`series.drilldown_average_breakdown`은 선택 GUID당 평균 시간 구성이다.
`drilldown_average_timeline_edges`는 `call_count`로 가중한 호출 쌍별 평균
도식이며 실제 관측 시각이 아니다.

`metadata.drilldown`에 버전·선택·대표 프로파일 규칙과 후보/표본/제외 수를
기록한다. 수치는 `summary`에도 명시한다. 비용은 임계 경로가 아닌 누적값이다.
병렬 작업으로 응답시간을 넘으면 coverage를 1보다 크게 표시하고 잔여 메소드
시간을 0으로 둔다. 하위 범위에 상위 그룹의 병렬도·사용자 정의 시간 조각을
복사하지 않는다. 전체 파일/signature·사용자 규칙·미수집 호출 집계는 선택
범위로 이름만 바꿔 내보내지 않으며 화면의 전체 보기에서 확인할 수 있다.

입력 제한은 직렬화 크기 64 MiB, 프로파일 100,000개, 그룹 10,000개, 각
edge 컬렉션 500,000개다. 초과 시 잘라내지 않고 오류를 반환한다. 확장
메타데이터는 JSON/Wails 저장·재로드에서도 보존한다.


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

### GC 관측 시간 기준 (스키마 0.2.0)

이벤트 날짜와 JVM 가동 시간은 독립적으로 추출합니다.
`metadata.time_axis`는 `absolute`, `uptime_seconds`, `event_index` 중 하나이며,
절대 시각만 날짜 축으로 표시합니다. 변화율은 공통된 단조 증가 시계가 필요합니다.
`metadata.measurement_clock`는 계산 시계를 명시합니다. 일부 이벤트에만 가동 시간이
있으면 모든 변화율에 날짜를 일관되게 사용하여 서로 다른 시계 기준을 섞지 않습니다.
`wall_time_sec`는 첫 pause 종료부터 마지막 pause 종료까지의 구간입니다.
처리율의 `throughput_pause_ms`는 시작이 관측 구간 밖에 있는 첫 pause를 제외합니다.
pause 합계와 백분위는 모든 이벤트를 사용합니다. 시간 누락·역행·동일 시각·모순된
증거는 지표를 null로 반환하고 `metadata.measurement_unavailable`에 이유를 기록합니다.
할당률과 승격률은 인접한 힙 관측값으로 계산하며, 증거 부족과 실제 측정된 0을 구분합니다.

### 플레임그래프의 누적 가중치

각 노드 값은 자손의 가중치를 포함합니다. Canvas는 d3 집계 전에
`node.value - sum(child.value)`로 자체 가중치를 계산합니다. 중간 노드의 자체
가중치는 자식이 차지하지 않는 공간으로 남으며, 폭·호버 합계·검색 비율은 같은
누적 분모를 사용합니다. 재귀 호출의 검색 결과는 중복 합산하지 않습니다.

### 출력 크기를 제한하는 프로파일러 드릴다운

데스크톱 서비스는 필터를 적용하는 동안에만 Go 내부에 원본 트리를 유지합니다.
각 단계의 지표를 계산한 뒤 IPC 전송 전에 표시 노드를 축약하며, 크기 제한은 유지합니다.
표시용 축약은 다음 필터가 사용할 입력을 변경하지 않습니다.
`FlameNode.hidden_samples`는 생략된 자손의 누적 가중치이며 관측된 자체 샘플이 아닙니다.
노드의 `samples` 안에 포함되며, 추가 노드를 만들 공간이 없어도 Canvas 안내와 호버에
생략 가중치를 표시합니다. 파서에서 버린 입력은 복구 대상으로 주장하지 않습니다.

### 기존 프로파일러의 카운터 단위 (스키마 0.2.0)

CLI `--value-unit`와 Wails `valueUnit`은 `samples`, `nanoseconds`,
`microseconds`, `milliseconds`, `seconds`, `bytes`, `unknown`을 받습니다.
CPU/wall의 기존 기본값은 주기적 샘플 건수이며, 잠금 프로파일의 기본값은 알 수 없음입니다.
잠금 샘플 건수는 이벤트 횟수이므로 주기적 시간으로 환산하지 않습니다.
누적 시간은 원본 단위를 직접 초로 변환하며, bytes·알 수 없는 단위·잠금 건수의 시간은
null과 `duration_unavailable_reason`으로 반환합니다. 요약과 드릴다운은 `value_unit`,
`aggregation`, nullable `seconds_per_weight`를 제공하며 시간 행과 경과 비율도 같은 계약을
따릅니다. pprof는 가중치의 원본 단위를 유지하며 잠금 카운터에 샘플 간격 기반 시간이나
주기를 추가하지 않습니다. V8/Chrome의 기존 마이크로초 계약은 유지합니다.

Trace 경로 시간(T-615): `observed_path_elapsed_ms`는 관측된 부모·자식 호출
경로의 경과 구간이며 정확한 인과 임계 경로의 증명이 아닌 추정값이다.
포함 관계인 상위/하위 span 시간을 다시 합하지 않는다. `critical_path_ms`는
호환용 별칭이고 `selected_chain_cumulative_ms`는 누적 작업량을 별도 보존한다.
시간 누락·시계 불일치·중복 span ID·순환은 null과
`duration_unavailable_reason`으로 반환한다. 화면·SLO·장애 타임라인은 새 의미가
명시된 값만 사용하며 이전의 미검증 누적값은 사용하지 않는다. 전체 계산
가능/불가 Trace 수는 top-N 경로 표와 별도로 유지한다.

Trace 의존 호출(T-616)은 명시적인 CLIENT/SERVER 직접 부모 관계를 Trace/span ID로 연결한다. 일대일 쌍은 CLIENT 경과시간과 양쪽 중 하나라도 실패한 오류 상태를 한 번 집계한다. 한쪽만 있는 export도 유지한다. 집계 CLIENT 하나에 여러 SERVER 시도가 있으면 각 서버 시간·오류로 별도 집계하고 CLIENT를 다시 더하지 않는다. SERVER의 remote peer는 수신 관점으로 처리한다. kind가 불명확하면 서비스 이름만으로 중복을 제거하지 않는다. 정책은 `metadata.dependency_call_policy`에 버전으로 기록한다.

액세스 로그 시간 가용성(schema 0.3.0, T-617): 응답시간 누락·센티널·잘못된
형식은 미측정으로 유지하고 실측 0은 0으로 보존한다. 전체 지연 통계에
`response_time_sample_count` / `response_time_unknown_count`를 붙인다.
표본이 없는 지연 통계·분당 값·샘플 값은 0이 아닌 null이다. URL/route도 같은
분모를 가지며 의존성 행은 `duration_sample_count` /
`duration_unknown_count`를 노출한다. 요청·오류·바이트 수는 정상 수집한 모든
요청을 포함한다. Service Flow는 지연만 실측 수로 가중하며 전체 호출 수는
유지한다. SLO 내보내기의 `target_evaluations`와
`unevaluated_target_count`는 증거 없는 목표를 통과가 아닌 미평가로 표시한다.
기존 프로그램 생성 Record는 기본적으로 실측으로 취급하며 파서가 미측정을
명시한다.

장애 타임라인 완전성(T-618): 원본당 120개를 보존한 뒤 세션에서 가장 이른
1000개 후보 행을 결정론적으로 표시한다. `summary.event_count`는 출력 행 수,
`total_event_count`, `omitted_event_count`, `partial_result`는 전체 후보와
누락 상태다. 심각도·시간·구간·상관관계 요약과 심각도/분류/분석기별 분포는
전체 후보 기준이며 그룹·내러티브와 그 건수는 보존된 행 기준이다.
`metadata.completeness`에 선택 정책, 원본별 전체/출력/누락 수, 누락 심각도
수와 두 요약 범위를 기록한다. 화면과 Report Pack HTML에서 누락된 critical
건수까지 알리며 JSON도 같은 정보를 보존한다.


## T-628 wire 구현과 검증 연결 (2026-11-11)

wirejson은 중복 key·비정상 JSON·깊이/문자열/배열/객체/byte 예산을 검사한 뒤
UseNumber로 임시 객체에 decode하며 성공할 때만 receiver를 교체한다.
opaque metadata/options/record JSON 기본 상한은 64 MiB, 깊이 64, 문자열 4 MiB, 배열 항목 1,000,000개,
객체 key 100,000개다. common Extra의 기존 consumer에는 JS 안전 범위에서
정확히 표현 가능한 숫자만 float64로 projection한다. 나머지는 json.Number이며
Float64Exact는 손실을 거부한다. 산술 accessor는 숫자 문자열 1,024바이트와 exponent 절댓값 4,096을 넘으면 계산을 거부하며, unknown JSON 데이터의 원문 보존과 구분한다. profiler Extra는 RawMessage로 유지하고 typed key와
omitempty의 생략이 항상 우선한다. 알려진 큰 정수는 exact_integer_string,
고정밀 소수는 decimal_string으로 전달하며 문자열은 최대 1,024바이트/소수 scale 128이다.

resultcontract는 type별 payload/envelope range, operation, required extension과
측정 schema를 검증한다. target·column·kind·unit·aggregation·encoding·nullable·참조를
대조한다. population/evidence ID는 같은 결과에서 해석하며 URL/파일을 열지 않는다.
서로 다른 리비전의 비교에는 population 출처와 선택/측정 호환 조건을 검증한다
(아래 R-04 / R2 N-01 참조).
정확한 scalar diff는 rational string으로 반환하고 baseline=0의 변화율은 null,
unavailable/absent의 delta는 null이다. 기존 float 기반 report-diff는 신규 measurement
결과를 거부하고 backend exact comparison을 안내한다. 기존 결과 흐름은 유지한다.

새 server_log payload는 기존 0.1.0 수치를 유지하고 envelope/measurement 1.0.0 metadata를
추가한다. profile_evidence 0.1.x 및 기존 profiler snapshot은 저장·표시·export용
reader만 연결하며 새 계산·profile migration은 활성화하지 않는다. profile_evidence
0.2.0은 아직 지원하지 않으므로 번호만 바꾸면 계산·일반 재저장을 거부한다.
미지 optional metadata는 보존한다. 미지원 payload/필수 block은 bounded read-only
view와 원본 raw-json 복사로 제한하며 정상 새 결과처럼 저장하지 않는다.

| 사례 | 실행 테스트 |
|---|---|
| W01·W03·W04 | resultcontract의 common/profiler 정밀도·typed/Extra·receiver 원자성, 기존 metadata roundtrip |
| W02 | decimal string의 type/scale와 0, exact accessor의 손실 거부 |
| W05 | duplicate/depth/NaN/Infinity 거부와 transactional decoder; invalid record/result의 job commit 거부 |
| W06 | 잘못된 Pointer·target·column·encoding·unit·population reference 거부, 정상 escape |
| W07 | ScalarView와 TS 표시 helper, bridge CSV/HTML·exact diff의 null/0/absent·availability·분모 보존 |
| W08·W09 | operation별 required block, optional unknown, reader range와 과거 profile 재해석 거부 |
| W10 | 실제 CLI export, common·legacy profiler의 opaque Wails 문자열 전송 모사, backend Report Pack source appendix 및 TS appendix의 원문 보존·URL handle 거부 |

GetEvidenceResultJSON의 문자열과 backend ExportEvidenceResult는 JS 숫자 object로
펼치지 않는다. Workspace의 선택적 backend_result_handle을 사용하는 Report Pack은
source JSON 원문을 appendix에 넣는다. backend pack은 source appendix이며 기존
Evidence Board narrative 전체를 재구현한 exporter가 아니다. JS object로 이미 펼친
기존 legacy full-result 경로 전체의 정밀도를 보장했다고 주장하지 않는다.

custom Metadata marshaler는 Wails 3.1 generator에서 opaque type이 된다. 진단 패널은
기존 수동 bridge의 동등한 diagnostic wire 타입을 사용하며 실제 wire key는 유지한다.
Go 메서드 통합과 JSON 문자열 transport 모사·프런트엔드 실행은 실제 네이티브 IPC/UI
실행 증거와 다르다. 실행 결과와 미실행 범위는
[로컬 검증 기록](../review/evidence/2026-11-11-t628/local.json)에 구분한다.

### 리뷰 후 측정·wire 계약 보완 (2026-11-11)

R-01~R-14 후속 수정에서는 전체 결과와 opaque JSON 예산을 분리한다. 구체적인
상한은 PARSER_DESIGN을 따른다. W05의 깊이 64 거절은 metadata/options/record에
적용하며 일반적인 중첩 flame tree에 일괄 적용하지 않는다.

**R-04 / R2 N-01:** population_ref는 결과 내부 label이다. 기존 `source_sha256`,
`selection_hash`, `semantic_revision`, 레코드 수를 포함한 `coverage`는 관측 출처로
유지한다. 같은 identity의 재현 비교는 아래 선택 정책의 유무가 다른 과거/신규
스냅샷 사이에서도 유지한다.

새 import는 `metadata.populations[ref].comparison`에 `version: "1.0.0"`,
원본에 독립적인 `selection_hash`, `coverage_kind`를 추가한다. pilot은 options-v1
정규화 hash를 사용하므로 기본값 생략과 명시는 다를 수 있다. coverage 종류는
`complete_records`, `selected_records:max_lines`, `complete_selected_model`이다.
레코드 수는 호환 조건이 아닌 관측 정보다. partial import는 이 정책을 부여하지
않으므로 다른 원본의 비교를 허용하지 않는다. 정책 누락·미지 버전 및 의미 버전·
선택·coverage 종류·측정 종류/단위/scope/aggregation 차이는 거절한다.

CompareEvidenceScalar의 두 인자는 명시적으로 선택한 baseline과 target backend
handle이다. 호환되는 다른 원본이면 응답에 `comparison.mode=baseline_target`,
적용 정책과 각각의 handle·저장 결과 원문 SHA-256·원본 hash·결과 내부 population
ref를 남긴다. 출처 identity가 같으면 `same_input`이며 스냅샷 바이트가 같다는
주장은 아니다. 캡처·entity·시계 identity는 추정하거나 병합하지 않는다. registry만
호출하면 정규화된 결과 digest를, service/CLI/Wails에서는 실제 저장 바이트 digest를
사용한다.

선택 정책이 없는 과거 결과의 수치와 identity 규칙은 유지한다. 재읽기·내보내기는
가능하지만 서로 다른 원본의 scalar 비교에는 재분석이 필요하다. payload/measurement
버전 번호를 바꾸어 의미를 재해석하지 않는다. ProfileSet comparator는 아래와 같이
보존된 모델에서 조건을 별도로 계산할 수 있다. 영속 baseline 선택·저장 및 확장된
capture/entity/time matching은 T-630/T-635에 남긴다.

**R3-01 / R3-02:** scalar 비교 거절은 안정적인 `code`와 정제된 `message`를 가진
`ComparisonError`다. Go 소비자는 `errors.As`, Wails 소비자는
`CallError.cause.code`를 사용하며 표시 문장을 파싱하지 않는다. 동일 입력 호환
확인 후 의미 버전 → partial 범위 → 정책 존재/지원 → 선택 → coverage 종류 순으로
검사한다. 그 전에 측정/identity 유효성 검사도 수행한다.

| 코드 | 의미 |
|---|---|
| `selection_mismatch` | 선택 hash 차이. 기본값 생략과 명시도 포함 |
| `coverage_mismatch` | 지원되는 coverage 종류가 다름 |
| `partial_not_comparable` | 다른 모집단 비교에 partial로 선언된 import가 포함됨 |
| `semantic_revision_mismatch` | 결과 type/payload 또는 선언된 의미 버전 차이 |
| `policy_missing` | 다른 모집단 비교의 버전 정책 누락 |
| `policy_unsupported` | 정책 버전·hash·coverage 종류 미지원 |
| `identity_missing` / `identity_invalid` | population 참조/identity 누락 또는 형식 오류 |
| `measurement_mismatch` | 측정 종류·단위·scope·aggregation·population 존재 여부 차이 |

성공한 scalar의 `comparison.identity_basis`와 ProfileSet difference의
`identity_basis`는 현재 신규 import를 포함해 항상 `declared`다. backend가 설정하며
저장 파일에 적힌 검증 주장은 신뢰하지 않는다. 모델/schema 검사는 내부 정합성,
저장 결과/모델 SHA-256은 바이트 식별만 입증한다. 원본의 진위나 원본/옵션 결속을
재계산한 증거가 아니다. 구조적으로 유효하게 편집된 identity/정책은 비교될 수 있지만
선언값이라는 표시는 유지한다. `verified_by_reimport` 경로는 아직 없다. 그 주장을
하려면 T-635에서 원본 재분석·옵션·결과 digest·영속 baseline 참조를 결속해야 한다.
응답 필드는 추가 호환이며 기존 저장 수치와 버전은 변경하지 않는다.

**R-06/R-11:** measurement 버전 정책은 result descriptor의 허용 범위를 단일
기준으로 삼는다(pilot: [1.0.0, 1.1.0)). 1.0.1은 허용하고 1.1.0/2.0.0은 거절한다.
미지의 선택 필드는 원 metadata에 보존하고 typed projection은 해석하지 않으며
필수 기능·operation을 부여하지 않는다. 공유·번들 descriptor schema 1.0.1은
scalar/column/collection 및 availability_column/reason_column을 포함한다.
collection에서 비어 있는 scalar 전용 Go 필드는 생략하며 유효한 scalar의 값이
있는 wire 필드는 유지한다. JSON Schema는 구조를, 등록된 result validator는
경로·단위·종류·참조·encoding·nullability·행 의미를 검증한다. schema 문서 버전과
결과의 measurement extension 버전은 별개다. fixture manifest 1.0.1에 수정된
schema digest를 고정한다.

**R-12:** CSV/HTML projection은 scalar와 컬렉션 행·컬럼의 availability를
전달한다(path, row, column, availability, reason, unit, population, numeric encoding).
HTML은 unavailable null을 ‘—’, CSV는 빈 셀로 표현하며 관측된 0은 유지한다.
기존 series.measurement_availability가 있으면 명시적인 export 충돌 오류를
반환하고 원 series를 덮어쓰지 않는다. 저장된 원본 JSON도 변경하지 않는다.

R2 N-04의 출력 예산은 availability 100,000항목 및 직렬화된 appendix JSON 16 MiB다.
결과 사본 생성 전에 행×컬럼 수를 확인하고 각 항목을 추가하기 전에 바이트를 계산한다.
초과하면 typed `ProjectionBudgetError` / `EXPORT_PROJECTION_BUDGET_EXCEEDED`로
거절하며 부분 내보내기나 저장 결과 변경을 하지 않는다. CSV/HTML 표시 투영의
예산이므로 JSON/Report Pack 원문 내보내기는 유지한다.

**R-14:** Go의 custom marshal 보존을 위해 생성된 profiler Metadata는 opaque로
유지한다. 수동 ProfilerMetadata/ProfilerAnalysisResult/ProfilerDrilldownStage facade와
실행 시 검증으로 profiler/diff/drilldown 화면이 읽는 필드를 확인한다. 미지 필드는
any 대신 unknown이며 알려진 필드의 타입·구조 오류는 렌더링 전에 거절한다.
Go의 null 진단 배열은 null로 유지한다. binding을 재생성하고 frontend의 타입 오류
검증 및 실행 시 구조 검증을 W01~W10에 보탠다. 실제 WebView IPC/UI 실행과
독립 재리뷰는 별도 검증이다.

## T-629 ProfileSet 공통 코어 계약 (2026-11-11)

`internal/profilemodel`을 공통 내부 프로파일 모델로 사용한다. 내부 JSON 교환 버전은
1.0.0이며 새로운 AnalysisResult envelope가 아니다. 새 결과는 기존 type인
`profile_evidence`의 payload 0.2.0을 사용한다. 기존 `profile_evidence` 0.1.x 및
profiler typed 결과의 reader·수치·엔드포인트는 유지한다. 한 type에 서로 겹치지
않는 payload 범위별 reader를 등록하며 겹치면 조립 시 거절한다. 구형 결과의
schema 번호만 바꿔서는 새 계산을 활성화할 수 없다. 새 reader는 보존된 model과
알려진 summary/tree/table/measurement/population projection을 재계산·대조한다.

| 계약 | 의미 |
|---|---|
| ProfileSet | 선언된 producer/버전/형식, 원본 hash 집합, opaque 수집 옵션·확장, frame/stack/metric/profile 및 품질 |
| Frame | 원본 symbol/module/build/file/line/runtime/inline/native/kernel/async 속성의 SHA-256 식별자. 표시 이름은 identity에서 제외 |
| Stack | 순서가 있는 frame-ID 경로. 재귀 frame의 반복 출현은 별도 경로 위치로 유지 |
| Metric | 종류·단위·양의 소수 scale·부호 계약. signed difference에는 base_kind 추가 |
| Sample | 정밀 소수 문자열 weight와 metric/stack ID, label/원본 참조 및 선택적 timestamp/interval |
| Profile | process/thread/worker와 aggregate/relative/absolute 시계. 측정된 시간에는 단위·scale 명시 |
| Selection | 명시적 profile ID 집합과 metric 하나, frame-ID prefix/contains 경로 및 교집합 드릴다운 필터 |
| Quality | nullable lost/unknown/truncated·심볼 해석률, 시계 신뢰도와 변환 손실. 미측정을 0으로 채우지 않음 |

관측된 0, 명시적 null 시간, 시간 필드 부재를 왕복 보존한다. line 0과 line 미상도
구분한다. 정의되지 않은 모델 필드는 거절하고 미래 선택 정보는 opaque `extensions`
map에 보관한다. 공통 결과 metadata의 unknown-key 정밀도·보존 계약은 유지한다.
weight·scale은 정밀 소수 문자열(최대 1,024자·소수 128자리)이다. 트리 inclusive/self는
scale을 정확히 곱한 선언 단위의 값이며 비중·변화율은 유리수 문자열이다. float 변환과
최소 1 보정을 하지 않는다. 관측이 없는 선택의 total_weight는 사유를 갖는 unavailable,
0 weight 레코드가 실제 존재하는 선택은 observed 0이다. 빈 대수 트리의 root 0은
query availability로 한정하며 측정 summary 0을 뜻하지 않는다. frame 표에서 표시
이름을 해석하므로 깊은 경로마다 이름을 반복 직렬화하지 않는다.

비교에는 metric ID·종류·단위·scale·부호 및 선택한 profile/entity 차원·producer/
normalizer 버전·품질·수집 옵션·순서 있는 frame 선택 규칙이 일치해야 한다. 원본 hash는
호환 identity에서만 제외하고 각 population identity와 baseline/target 참조에 보존한다.
참조에는 모델 digest와 service의 handle/저장 결과 digest를 남긴다. 기존
`Difference.population`은 baseline의 별칭이며 소비자는 두 참조로 출처를 구분해야 한다.
서로 다른 원본은 `comparison_mode=baseline_target`, 같은 출처 모집단은 `same_input`이다.
과거 보존 모델도 저장 metadata를 고치지 않고 검증할 수 있다. 서로 다른 수집의 entity
재매핑·시계 정렬·영속 baseline 관리는 T-630/T-635에 남긴다. 0 기준 변화율이나
unavailable 비교값을 만들지 않는다. BuildSet은 모델/선택을 복사해 보존하며 질의는
증거를 변경하거나 가지치기한 표시 트리를 다음 계산에 재사용하지 않는다.

`LegacyStackProjection`에는 정수 tick scale을 명시한다. 결과에는 scale·선택한 metric/
profile·집계된 차원을 포함한다. 이름 충돌·구분자 손실이 없을 때만 표시 이름을 key로
사용하고 stable frame-ID key도 지원한다. 정수화할 수 없는 소수, signed 자료, 대상
int 및 합계 범위 초과는 거절한다. pprof도 명시적 tick과 정확한 int64만 허용하며 0
레코드를 유지한다. scale을 적용한 export는 단위를 `ticks`로 표시하고 원래 metric/
unit/scale·선택·frame identity·원본 hash·품질을 Comments에 남긴다. process/thread/
worker·정확한 시간 문맥은 예약 label로 보존하며 충돌은 오류다. 선택한 metric의
projection이므로 전체 다중 metric 원본 백업에는 JSON/Report Pack을 사용한다.

운영 상한은 입력 64 MiB, 참조 문자열 합 32 MiB, frame/stack/sample 각 100,000개,
profile·stack 깊이 1,024, metric·filter 64개, tree node 200,000개, 선택 frame 참조
1,000,000개, 트리 직렬화 8 MiB, 드릴다운 응답 합 32 MiB다. pprof identity Comments는
8 MiB, 출력은 64 MiB로 제한한다. 자원 초과 시 반올림·절단 대신 거절한다. 현재
catalog 교환 경로는 normalized 문서 하나를 내보내므로 job record 상한(기본 1 MiB)도
따른다. 더 큰 producer streaming adapter는 별도 작업이다.
