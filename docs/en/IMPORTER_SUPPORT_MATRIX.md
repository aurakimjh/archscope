# Importer Support Matrix

This matrix tracks the currently supported local evidence importers, including
the unreleased browser-profile, Lighthouse, and HAR additions after `v0.3.5`.

| Family | Result type | CLI | Representative inputs |
|---|---|---|---|
| Access and edge logs | `access_log` | `access-log analyze` | nginx/common/combined, Apache/OHS, Tomcat/Jetty, HAProxy, Envoy/Istio, cloud load balancer, API Gateway |
| Server logs | `server_log` | `server-log analyze` | Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere, GlassFish/Payara, nginx/Apache error logs |
| OpenTelemetry logs | `otel_logs` | `otel analyze` | JSONL/NDJSON, OTLP Logs JSON |
| Metrics snapshots | `metrics_snapshot` | `metrics import` | Prometheus/OpenMetrics text |
| Observability exports | `observability_evidence` | `observability import` | Loki query JSON, Tempo trace JSON, Grafana dashboard JSON |
| Trace imports | `trace_import` | `trace import` | OTLP JSON/JSONL traces, Zipkin v2 JSON, Elastic APM `_search` JSON, Elastic source NDJSON, Jaeger QueryService/local trace JSON, guarded SkyWalking GraphQL `queryTrace.spans` JSON |
| Database evidence | `database_slow_query` | `database-log analyze` | PostgreSQL logs/csvlog, MySQL slow log, MongoDB profiler JSON, Redis slowlog, SQL Server xevent JSON, PostgreSQL/MySQL EXPLAIN JSON |
| Broker evidence | `broker_log` | `broker-log analyze` | Kafka, RabbitMQ logs/diagnostics JSON, Pulsar, NATS, ActiveMQ |
| Platform evidence | `kubernetes_evidence` | `platform import` | Kubernetes events/pod JSON, kubelet/runtime logs, CloudTrail, GCP audit, Azure Activity |
| Runtime profiles | `profile_evidence` | `profile import` | pprof `.pb.gz`, async-profiler collapsed/HTML, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, JFR JSON stacks, StackProf, PHP profiler JSON, Xdebug, Swift/async stacks, Pyroscope/Parca snapshots, Chrome Performance trace `.json`/`.json.gz` (`chrome-trace-json`), V8 `.cpuprofile` including Node `--cpu-prof` and CDP `Profiler.stop` (`v8-cpuprofile`) |
| Browser audits | `browser_audit_evidence` | `browser import` | Lighthouse report JSON (`lighthouse-json`), with report-score preservation, Core Web Vitals/audit/resource projections, URL redaction, and bounded tables |
| HTTP capture evidence | `http_capture` | `http-capture analyze` | HAR 1.2 files with dialect detection (Chrome, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic); import-time redaction; bounded entry cap |
| Evidence stitching | `stitched_evidence` | `stitch analyze` | Existing `AnalysisResult` JSON files from access, trace, runtime profile, database, broker, and platform importers; supports exact keys plus timestamp/service-alias stitching |
| API/event contracts | `api_contract_analysis` | `api-contract analyze` | OpenAPI JSON/YAML plus access-log result JSON; AsyncAPI JSON/YAML plus broker result JSON |
| Architecture documentation | `architecture_docs` | `architecture-docs draft` | Existing `AnalysisResult` JSON files with service, contract, runtime, deployment, finding, and risk evidence |

All importers preserve parser diagnostics under `metadata.diagnostics` and emit
bounded tables suitable for Evidence Board and report-pack capture.

Windows live HTTP capture is not an additional file importer. It produces the
same bounded `http_capture` analysis contract after a finalized local session,
but its release scope is narrower: Windows-only, loopback proxy, HTTP/1.x
metadata, no bodies, HTTP/2 and QUIC passthrough/unsupported, and
connection-specific endpoint attribution rather than whole-machine coverage.

### Internal ProfileSet transport (T-629)

`profile.set-json` / `profile-set` is a limited internal ProfileSet 1.0.0 JSON
transport, producing `profile_evidence` 0.2.0 with explicit profile/metric selection.
Its bundled fixture is synthetic and qualifies core behavior, not a producer/version.
The existing `profile import` route retains 0.1.x normalization semantics.
`profile-set normalize-legacy` discloses those limitations; it does not qualify
corrected py-spy/Speedscope/pprof input semantics. Those remain T-634. New generic
screens remain T-631. Internal adapter 1.0.1 chunks a bounded <=32 MiB model into
<=256 KiB raw records, retaining the common 1 MiB serialized-record ceiling.
Normalized/result/temp ceilings apply jointly; see DATA_MODEL for the effective
limits and the verified 3,000-stack/54,001-node case (not a production guarantee).
Automatic verified detection is limited to complete documents within the 64 KiB
probe; larger documents require explicit profile-set selection and full validation.


### Exact baseline/target comparison (R2 N-01)

CompareEvidenceScalar accepts ordered baseline/target handles for complete or
explicitly selected imports with matching versioned comparison policies. Different
source hashes are retained in the response instead of blocking a nonzero delta.
ProfileSet comparison checks source-independent model conditions and returns both
model/source/result references. Unknown/missing policies keep old scalar snapshots
on their original identity rules. Different selection, metric, semantic/capture or
coverage conditions are refused; partial imports do not gain cross-source scalar
comparison. Durable baseline management and entity/time matching remain T-630/T-635.
Legacy profiler diff keeps its separate compatibility behavior.

Comparison identity is explicitly `declared`, not original-source verification.
Stable scalar refusal codes are available as Wails error `cause.code` (DATA_MODEL,
R3-01/R3-02). Whitespace-only max_lines suffixes no longer mark selected coverage;
existing saved coverage is preserved. Reimport is needed to correct old coverage
when comparing different sources. T-635 owns verified durable baseline binding.

R4 profile comparison accepts differing measured quality counters/ratios and
preserves both snapshots plus change warnings. Clock/symbol/conversion categories
still gate comparison; process/thread/worker differences return
entity_mapping_required until T-635 supplies explicit mapping. Existing stored
policy hashes are preserved, not silently promoted to the new scalar policy.

The effective wire limit includes 4 MiB per decoded UTF-8 string/key in addition
to aggregate/document bounds. Oversize strings report json_string_bytes budget
detail. Opaque capture options remain strictly equal; semantic/incidental key
policies require producer-specific T-634 qualification and T-635 baseline wiring
(DATA_MODEL R5). No current option whitelist or new producer support is claimed.
