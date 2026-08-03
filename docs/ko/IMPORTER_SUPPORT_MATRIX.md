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
