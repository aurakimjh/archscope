# Importer Support Matrix

이 문서는 Mid-Term Plus ingestion 라인에서 구현된 local evidence importer를
정리한다.

| Family | Result type | CLI | 대표 입력 |
|---|---|---|---|
| Access 및 edge log | `access_log` | `access-log analyze` | nginx/common/combined, Apache/OHS, Tomcat/Jetty, HAProxy, Envoy/Istio, cloud load balancer, API Gateway |
| Server log | `server_log` | `server-log analyze` | Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere, GlassFish/Payara, nginx/Apache error log |
| OpenTelemetry log | `otel_logs` | `otel analyze` | JSONL/NDJSON, OTLP Logs JSON |
| Metrics snapshot | `metrics_snapshot` | `metrics import` | Prometheus/OpenMetrics text |
| Observability export | `observability_evidence` | `observability import` | Loki query JSON, Tempo trace JSON, Grafana dashboard JSON |
| Database evidence | `database_slow_query` | `database-log analyze` | PostgreSQL log/csvlog, MySQL slow log, MongoDB profiler JSON, Redis slowlog, SQL Server xevent JSON, PostgreSQL/MySQL EXPLAIN JSON |
| Broker evidence | `broker_log` | `broker-log analyze` | Kafka, RabbitMQ log/diagnostics JSON, Pulsar, NATS, ActiveMQ |
| Platform evidence | `kubernetes_evidence` | `platform import` | Kubernetes event/pod JSON, kubelet/runtime log, CloudTrail, GCP audit, Azure Activity |
| Runtime profile | `profile_evidence` | `profile import` | pprof `.pb.gz`, async-profiler collapsed/HTML, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, JFR JSON stack, StackProf, PHP profiler JSON, Xdebug, Swift/async stack, Pyroscope/Parca snapshot |
| Evidence stitching | `stitched_evidence` | `stitch analyze` | access, trace, runtime, database, broker, platform importer가 만든 기존 `AnalysisResult` JSON |

모든 importer는 parser diagnostics를 `metadata.diagnostics` 아래 보존하고,
Evidence Board와 report-pack capture에 적합한 bounded table을 생성한다.
