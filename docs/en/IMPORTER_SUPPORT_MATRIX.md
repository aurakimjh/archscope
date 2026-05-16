# Importer Support Matrix

This matrix tracks the local evidence importers implemented for the Mid-Term
Plus ingestion line.

| Family | Result type | CLI | Representative inputs |
|---|---|---|---|
| Access and edge logs | `access_log` | `access-log analyze` | nginx/common/combined, Apache/OHS, Tomcat/Jetty, HAProxy, Envoy/Istio, cloud load balancer, API Gateway |
| Server logs | `server_log` | `server-log analyze` | Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere, GlassFish/Payara, nginx/Apache error logs |
| OpenTelemetry logs | `otel_logs` | `otel analyze` | JSONL/NDJSON, OTLP Logs JSON |
| Metrics snapshots | `metrics_snapshot` | `metrics import` | Prometheus/OpenMetrics text |
| Observability exports | `observability_evidence` | `observability import` | Loki query JSON, Tempo trace JSON, Grafana dashboard JSON |
| Database evidence | `database_slow_query` | `database-log analyze` | PostgreSQL logs/csvlog, MySQL slow log, MongoDB profiler JSON, Redis slowlog, SQL Server xevent JSON, PostgreSQL/MySQL EXPLAIN JSON |
| Broker evidence | `broker_log` | `broker-log analyze` | Kafka, RabbitMQ logs/diagnostics JSON, Pulsar, NATS, ActiveMQ |
| Platform evidence | `kubernetes_evidence` | `platform import` | Kubernetes events/pod JSON, kubelet/runtime logs, CloudTrail, GCP audit, Azure Activity |
| Runtime profiles | `profile_evidence` | `profile import` | pprof `.pb.gz`, async-profiler collapsed/HTML, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, JFR JSON stacks, StackProf, PHP profiler JSON, Xdebug, Swift/async stacks, Pyroscope/Parca snapshots |
| Evidence stitching | `stitched_evidence` | `stitch analyze` | Existing `AnalysisResult` JSON files from access, trace, runtime, database, broker, and platform importers |
| API/event contracts | `api_contract_analysis` | `api-contract analyze` | OpenAPI JSON/YAML plus access-log result JSON; AsyncAPI JSON/YAML plus broker result JSON |
| Architecture documentation | `architecture_docs` | `architecture-docs draft` | Existing `AnalysisResult` JSON files with service, contract, runtime, deployment, finding, and risk evidence |

All importers preserve parser diagnostics under `metadata.diagnostics` and emit
bounded tables suitable for Evidence Board and report-pack capture.
