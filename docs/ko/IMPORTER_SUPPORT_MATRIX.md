# Importer Support Matrix

이 문서는 `v0.3.5` 이후 아직 릴리스되지 않은 browser profile, Lighthouse,
HAR 추가분을 포함해 현재 지원하는 local evidence importer를 정리한다.

| Family | Result type | CLI | 대표 입력 |
|---|---|---|---|
| Access 및 edge log | `access_log` | `access-log analyze` | nginx/common/combined, Apache/OHS, Tomcat/Jetty, HAProxy, Envoy/Istio, cloud load balancer, API Gateway |
| Server log | `server_log` | `server-log analyze` | Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere, GlassFish/Payara, nginx/Apache error log |
| OpenTelemetry log | `otel_logs` | `otel analyze` | JSONL/NDJSON, OTLP Logs JSON |
| Metrics snapshot | `metrics_snapshot` | `metrics import` | Prometheus/OpenMetrics text |
| Observability export | `observability_evidence` | `observability import` | Loki query JSON, Tempo trace JSON, Grafana dashboard JSON |
| Trace import | `trace_import` | `trace import` | OTLP JSON/JSONL trace, Zipkin v2 JSON, Elastic APM `_search` JSON, Elastic source NDJSON, Jaeger QueryService/local trace JSON, guarded SkyWalking GraphQL `queryTrace.spans` JSON |
| Database evidence | `database_slow_query` | `database-log analyze` | PostgreSQL log/csvlog, MySQL slow log, MongoDB profiler JSON, Redis slowlog, SQL Server xevent JSON, PostgreSQL/MySQL EXPLAIN JSON |
| Broker evidence | `broker_log` | `broker-log analyze` | Kafka, RabbitMQ log/diagnostics JSON, Pulsar, NATS, ActiveMQ |
| Platform evidence | `kubernetes_evidence` | `platform import` | Kubernetes event/pod JSON, kubelet/runtime log, CloudTrail, GCP audit, Azure Activity |
| Runtime profile | `profile_evidence` | `profile import` | pprof `.pb.gz`, async-profiler collapsed/HTML, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, JFR JSON stack, StackProf, PHP profiler JSON, Xdebug, Swift/async stack, Pyroscope/Parca snapshot, Chrome Performance trace `.json`/`.json.gz`(`chrome-trace-json`), V8 `.cpuprofile` — Node `--cpu-prof`·CDP `Profiler.stop` 포함(`v8-cpuprofile`) |
| Browser audit | `browser_audit_evidence` | `browser import` | Lighthouse report JSON(`lighthouse-json`). 원본 report score 보존, Core Web Vitals/audit/resource projection, URL 리댁션, bounded table 적용 |
| HTTP capture evidence | `http_capture` | `http-capture analyze` | 방언 판별이 있는 HAR 1.2 (Chrome, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic); 가져오기 시점 리댁션; entry 상한 |
| Evidence stitching | `stitched_evidence` | `stitch analyze` | access, trace, runtime profile, database, broker, platform importer가 만든 기존 `AnalysisResult` JSON. exact key와 timestamp/service-alias stitching 지원 |
| API/event contract | `api_contract_analysis` | `api-contract analyze` | OpenAPI JSON/YAML + access-log result JSON, AsyncAPI JSON/YAML + broker result JSON |
| Architecture documentation | `architecture_docs` | `architecture-docs draft` | service, contract, runtime, deployment, finding, risk evidence를 담은 기존 `AnalysisResult` JSON |

모든 importer는 parser diagnostics를 `metadata.diagnostics` 아래 보존하고,
Evidence Board와 report-pack capture에 적합한 bounded table을 생성한다.

Windows 실시간 HTTP 캡처는 별도의 file importer가 아니다. 종료된 로컬 세션을
같은 bounded `http_capture` 분석 계약으로 전달하지만 릴리스 범위는 더 좁다.
Windows 전용 loopback proxy의 HTTP/1.x metadata만 다루고 body를 저장하지 않으며,
HTTP/2와 QUIC는 passthrough/unsupported다. Endpoint 귀속은 개별 connection
증거이며 시스템 전체 트래픽 coverage를 의미하지 않는다.

### 내부 ProfileSet 교환 경로 (T-629)

`profile.set-json` / `profile-set`은 제한된 내부 ProfileSet 1.0.0 JSON 교환 경로다.
profile/metric을 명시하여 `profile_evidence` 0.2.0을 만든다. 번들 fixture는 합성 자료이며
공통 코어를 검증할 뿐 실제 producer/버전의 지원 근거가 아니다. 기존 `profile import`는
0.1.x 정규화 의미를 유지한다. `profile-set normalize-legacy`도 그 제한을 명시하며
py-spy/Speedscope/pprof 입력의 수치 정정을 입증하지 않는다(T-634). generic 화면은
T-631이다. normalized 문서 하나가 job의 record 바이트 상한도 따르므로 무제한 대용량
producer adapter라고 표시하지 않는다.


### 정밀 baseline/target 비교 (R2 N-01)

CompareEvidenceScalar는 버전이 있는 비교 정책이 일치하는 완료/명시 선택 import의
baseline/target handle을 순서대로 받는다. 서로 다른 원본 hash를 응답에 보존하면서
0이 아닌 차이를 계산한다. ProfileSet 비교는 원본에 독립적인 모델 조건을 확인하고
양쪽 모델/원본/결과 참조를 반환한다. 정책 누락·미지 버전인 과거 scalar 결과는 기존
identity 규칙을 유지한다. 선택·측정·의미/수집·coverage 조건 차이는 거절하고 partial
import에는 다른 원본의 scalar 비교 권한을 부여하지 않는다. 영속 baseline 관리와
entity/time matching은 T-630/T-635에 남기며 legacy profiler diff는 기존 호환 동작을
유지한다.
