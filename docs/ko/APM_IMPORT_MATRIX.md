# APM Import Matrix

Last updated: 2026-05-11

이 문서는 ArchScope가 다른 APM/observability 도구에서 내려받은 데이터를
로컬에서 재분석할 수 있는지 판단하기 위한 조사 메모다. 기준은
`PRODUCT_EXPANSION_NOTES.md`의 "Jennifer처럼 로컬로 내려받아 재분석 가능한가"이다.

## 결론 요약

ArchScope의 1차 import 대상은 SaaS별 API가 아니라 표준/오픈 포맷이어야 한다.

1. `OpenTelemetry OTLP JSON file`
   - OpenTelemetry Collector file exporter가 traces/metrics/logs를 파일로 쓸 수 있다.
   - OTLP JSON은 trace, metric, log 신호를 공통 envelope로 보존하므로 장기적으로
     Evidence Board, Incident Timeline, SLO/Golden Signals의 공통 입력이 된다.
2. `Zipkin v2 JSON`
   - Zipkin 자체와 Jaeger, SkyWalking, 여러 gateway/tracer가 수용하는 단순 span 배열이다.
   - `traceId`, `id`, `parentId`, `timestamp`, `duration`, endpoint, tags 구조가 명확해
     빠르게 importer를 만들 수 있다.
3. `Elastic APM Elasticsearch documents`
   - Elastic APM trace는 Elasticsearch data stream에 저장된다.
   - `_search` 또는 Kibana Discover CSV/JSON export로 로컬 파일화하기 쉽다.

Datadog, Dynatrace, New Relic, AppDynamics는 공식 API/export 경로가 있지만
계정 권한, 보존 기간, 샘플링, rate limit, 라이선스 조건이 강하게 개입한다.
따라서 1차 MVP에서는 "파일 import"를 만들고, SaaS API connector는 별도 단계로
분리하는 편이 안전하다.

## APM별 import 가능성

| 도구 | 공식 export/API 확인 | 로컬 파일 적합도 | ArchScope 우선순위 | 판단 |
|---|---|---:|---:|---|
| OpenTelemetry Collector | file exporter가 telemetry를 디스크 파일로 기록한다. OTLP JSON으로 export하면 OTLP JSON File receiver로 다시 읽을 수 있다고 명시되어 있다. | 높음 | P0 | 표준 입력 포맷 1순위. |
| Zipkin | Zipkin은 v1/v2 JSON codec을 제공하고, query/search endpoint와 trace-id 조회를 제공한다. Elasticsearch storage도 span을 Zipkin v2 JSON 형태로 저장한다. | 높음 | P0 | 단일 trace/span JSON import에 가장 단순하다. |
| Elastic APM | APM trace는 `traces-apm-*` data stream에 저장되고 Elasticsearch `_search` API로 조회할 수 있다. Discover saved search는 CSV report 생성이 가능하다. | 높음 | P0/P1 | Elasticsearch search hits 또는 NDJSON export importer가 적합하다. |
| Jaeger | 안정적 retrieval API는 QueryService gRPC다. HTTP JSON `/api/traces/{trace-id}`는 UI용 내부 API로 문서상 변경 가능성이 있다. Jaeger 2.x는 OTLP 기반 API도 제공한다. | 중간 | P1 | HTTP JSON은 best-effort, 안정 경로는 OTLP/QueryService 쪽으로 둔다. |
| Apache SkyWalking | GraphQL Query Protocol에서 trace list, single trace, topology, metrics, profiling query를 제공한다. | 중간 | P1/P2 | GraphQL 응답 JSON import는 가능하나 스키마/버전 확인이 필요하다. |
| New Relic | NerdGraph로 trace detail을 조회할 수 있고, Historical Data Export는 JSON 파일 다운로드 링크를 제공한다. | 중간 | P2 | JSON export는 강하지만 Data Plus, 12시간 지연, 계정 권한 조건이 있다. |
| Datadog APM | Spans API가 span search/list/aggregate를 제공한다. Continuous Profiler는 기본 profile 보존 8일로 짧다. | 중간/낮음 | P2 | API import는 가능하지만 rate limit, retention filter, indexed span sampling을 고려해야 한다. |
| Dynatrace | Distributed Traces Classic은 overview table CSV export를 제공한다. 최신 Distributed Tracing app은 visible table download와 DQL API call 복사를 제공한다. | 낮음/중간 | P2/P3 | CSV overview는 증거 카드에는 유용하지만 full trace 재구성에는 부족하다. |
| Splunk/Cisco AppDynamics | Metric and Snapshot API에서 `/controller/rest/applications/{application}/request-snapshots`로 transaction snapshot을 조회할 수 있다. | 중간/낮음 | P2/P3 | snapshot은 sampled evidence라 전체 trace corpus로 보기 어렵다. |
| Pinpoint | 이번 조사에서 안정적인 공식 export/API 문서를 확인하지 못했다. | 미정 | P3 | 공식 API 확인 전까지는 직접 지원 대상에서 제외한다. |

## 권장 importer 순서

### 1. OTLP JSON file importer

목표 입력:

- OpenTelemetry Collector file exporter 기본 JSON line output
- OTLP/HTTP JSON protobuf envelope
- 이후 Jaeger 2.x OTLP export와도 연결 가능한 구조

필수 매핑:

| Canonical field | OTLP source |
|---|---|
| `trace_id` | `traceId` |
| `span_id` | `spanId` |
| `parent_span_id` | `parentSpanId` |
| `name` | `name` |
| `service_name` | resource attribute `service.name` |
| `start_time_unix_nano` | `startTimeUnixNano` |
| `end_time_unix_nano` | `endTimeUnixNano` |
| `duration_nanos` | `end - start` |
| `kind` | `kind` |
| `status_code` | `status.code` |
| `attributes` | span/resource/scope attributes |

MVP 산출물:

- `examples/otel/sample-otlp-traces.jsonl`
- Go parser package: `internal/parsers/traceimport`
- Analyzer package: `internal/analyzers/traceimport`
- UI page or Jennifer MSA extension은 2차. 먼저 CLI/engine result contract부터 만든다.

### 2. Zipkin v2 JSON importer

목표 입력:

- Zipkin `/trace/{traceId}` 또는 `/traces` 응답
- Zipkin v2 JSON 파일
- Jaeger/SkyWalking/게이트웨이에서 Zipkin-compatible export가 가능한 경우

필수 매핑:

| Canonical field | Zipkin v2 source |
|---|---|
| `trace_id` | `traceId` |
| `span_id` | `id` |
| `parent_span_id` | `parentId` |
| `name` | `name` |
| `service_name` | `localEndpoint.serviceName` |
| `start_time_unix_micro` | `timestamp` |
| `duration_micros` | `duration` |
| `kind` | `kind` |
| `remote_service` | `remoteEndpoint.serviceName` |
| `attributes` | `tags` |

MVP 산출물:

- `examples/traces/sample-zipkin-v2.json`
- 동일 canonical trace model로 변환
- trace tree, critical path, service dependency summary 생성

### 3. Elastic APM search export importer

목표 입력:

- Elasticsearch `_search` JSON response
- `_source`만 NDJSON로 export한 파일
- Kibana CSV는 보조 입력으로 처리

필수 매핑 후보:

| Canonical field | Elastic APM source 후보 |
|---|---|
| `trace_id` | `trace.id` |
| `span_id` | `span.id` 또는 `transaction.id` |
| `parent_span_id` | `parent.id` |
| `name` | `span.name` 또는 `transaction.name` |
| `service_name` | `service.name` |
| `start_time` | `@timestamp` |
| `duration` | `event.duration` |
| `kind/type` | `span.type`, `span.subtype`, `transaction.type` |
| `result/status` | `event.outcome`, `http.response.status_code` |

주의:

- Elastic은 transactions와 spans를 모두 trace data stream에 저장하므로,
  importer는 `transaction.id`도 span-like root node로 다뤄야 한다.
- `_search` 응답의 `hits.hits[*]._source`와 NDJSON source-only 파일을 모두
  허용하면 사용자가 Elasticsearch 접근 방식에 덜 묶인다.

## 공통 canonical model 초안

APM별 스키마를 직접 UI에 노출하지 말고, 먼저 공통 trace evidence 모델로
정규화한다.

```go
type TraceImportResult struct {
    Summary      TraceSummary
    Services     []TraceService
    Traces       []TraceRecord
    Spans        []TraceSpan
    Dependencies []TraceDependency
    Findings     []TraceFinding
    Source       TraceSourceMetadata
}

type TraceSpan struct {
    TraceID        string
    SpanID         string
    ParentSpanID   string
    Name           string
    ServiceName    string
    RemoteService  string
    Kind           string
    StartUnixNanos int64
    DurationNanos  int64
    StatusCode     string
    Error          bool
    Attributes     map[string]string
    SourceFormat   string
}
```

초기 finding 후보:

- `SLOW_TRACE_P95`: trace duration p95가 기준을 초과.
- `SLOW_SPAN_DOMINATES_TRACE`: 단일 span이 trace duration의 대부분을 차지.
- `ERROR_SPAN_IN_TRACE`: error/status failure span 포함.
- `MISSING_PARENT_SPAN`: parent reference가 있으나 parent span이 export에 없음.
- `CLOCK_SKEW_SUSPECTED`: child span 시간이 parent 범위를 크게 벗어남.
- `UNBALANCED_SERVICE_LATENCY`: 특정 service/endpoint의 cumulative duration 비중이 높음.

## SaaS connector를 뒤로 미루는 이유

- 권한과 인증 모델이 제품별로 다르다.
- 무료/기본 플랜에서 API export가 제한될 수 있다.
- span/trace 보존 기간이 짧거나, indexed span sampling 때문에 전체 evidence가
  누락될 수 있다.
- 고객 환경에서 SaaS API token을 ArchScope에 넣는 것은 보안/컴플라이언스
  검토가 필요하다.
- ArchScope의 강점은 로컬 evidence 재분석이므로, 우선 파일 import UX가
  제품 정체성과 맞다.

## 다음 작업

1. `internal/parsers/traceimport`에 OTLP JSON line parser를 추가한다.
2. 동일 package 안에 Zipkin v2 JSON parser를 추가한다.
3. `TraceImportResult`를 기존 `AnalysisResult` contract에 맞춰 내보낸다.
4. `examples/traces/`에 OTLP/Zipkin 샘플을 추가한다.
5. CLI에 `archscope-engine trace import <file>` 또는 기존 command 체계에 맞는
   `trace` analyzer를 추가한다.

## 참고한 공식 문서

- Dynatrace Distributed Traces CSV/table export:
  https://docs.dynatrace.com/docs/how-to-use-dynatrace/diagnostics/diagnostic-distributed-traces/
- Dynatrace Distributed Tracing app/DQL API/table download:
  https://docs.dynatrace.com/docs/observe/application-observability/distributed-tracing/distributed-tracing-app
- Datadog Spans API:
  https://docs.datadoghq.com/api/latest/spans/
- Datadog Continuous Profiler:
  https://docs.datadoghq.com/profiler/
- New Relic NerdGraph distributed trace query:
  https://docs.newrelic.com/docs/apis/nerdgraph/examples/nerdgraph-distributed-trace-data-tutorial/
- New Relic Historical Data Export:
  https://docs.newrelic.com/docs/apis/nerdgraph/examples/nerdgraph-historical-data-export/
- Elastic APM data streams:
  https://www.elastic.co/guide/en/apm/server/current/apm-integration-data-streams.html
- Elastic Elasticsearch `_search` API:
  https://www.elastic.co/docs/solutions/search/the-search-api
- Elastic Kibana reports/CSV:
  https://www.elastic.co/docs/explore-analyze/find-and-organize/reports
- Jaeger APIs:
  https://www.jaegertracing.io/docs/1.76/apis/
- Zipkin project/API notes:
  https://github.com/openzipkin/zipkin
- Apache SkyWalking Query Protocol:
  https://skywalking.incubator.apache.org/docs/main/next/en/api/query-protocol/
- OpenTelemetry Collector exporters:
  https://opentelemetry.io/docs/collector/components/exporter/
- OpenTelemetry Collector file exporter:
  https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/fileexporter/README.md
- OpenTelemetry OTLP specification:
  https://opentelemetry.io/docs/specs/otlp/
- Cisco AppDynamics Metric and Snapshot API:
  https://docs.appdynamics.com/appd/onprem/23.x/23.7/ja/extend-appdynamics/appdynamics-apis/metric-and-snapshot-api
