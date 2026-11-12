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
corrected py-spy/Speedscope/pprof input semantics. The separate T-634 source path
below supplies new semantics; T-631 supplies generic screens. Internal adapter 1.0.1 chunks a bounded <=32 MiB model into
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
policies apply only to T-634 qualified canonical source models below. Old models
and generic scalar comparison remain strict; durable baseline wiring is T-635.

### Declarative snapshot pack (T-632)

`data.synthetic-metrics` / `synthetic-metrics` is installed by the signed bootstrap
pack. With an explicit `evidence --pack-store` or the desktop catalog it emits
metrics_snapshot 0.1.1. Only synthetic.metrics 1.0.0 JSONL fixture conformance is
qualified. Literal guards/aliases/exact scaling are fixed interpreter operations;
this does not add vendor, time-series or arbitrary-script support. See [Offline packs](OFFLINE_PACKS.md).

### Input assistance and JFR conversion

Input inspection enumerates the current generic catalog separately from these
legacy entry points. It adds no producer support claim. Binary JFR uses explicit
absolute installed JDK configuration with bounded execution and provenance; JSON
analysis remains available without a converter. Offline sessions require
preconverted JSON. See [Input assistance](INPUT_ASSISTANCE.md).

### Exact original-source profiles (T-634)

`profile inspect-source`, `normalize-source` and `import-source` preserve exact
weights/units and require explicit profile/metric selection. Desktop Workspace
normalizes originals to `profile.source-set-json` / `profile-source-set`, producing
`profile_evidence` 0.4.0. `profile import` retains legacy semantics. Evented
Speedscope uses exclusive elapsed intervals and explicit unattributed gaps, not
event counts; sampled time weights are duration, not inferred CPU time. pprof
keeps all columns, signed values, labels and period metadata without double scaling.

| Slice | Qualification | Capture policy |
|---|---|---|
| py-spy 0.4.1 raw / sampled Speedscope seconds | Actual synthetic subprocess producer fixtures, digest pinned | Compiled 1.0.0 policy for these metric/format bindings |
| Go runtime/pprof go1.26.10 heap | Actual synthetic four-column producer fixture, digest pinned | Compiled 1.0.0 policy for alloc/inuse objects and space |
| Other structurally supported sampled/evented Speedscope / supported-unit pprof | Synthetic format regressions; producer/platform/option coverage unqualified | Unknown/no-policy options remain strict |

No broad vendor/platform promotion, dump-text support or original-source
identity authentication follows from these fixtures. Native Windows/UI acceptance
and independent review are separate. See [Exact profile sources](PROFILE_SOURCE_IMPORT.md)
for bounds, CLI examples, projection limits, policy keys and evidence scope.

### T-635 canonical queries, verified baseline and R11 revision

[Case evidence queries](CASE_EVIDENCE_QUERIES.md) defines bounded cursor-bound
profile views/evidence, source/summary case search with indexed coverage, original
coordinate links, immutable selected results, and baseline replay/mapping gates.
R11 correction emits source normalizer 1.1.0 / result 0.4.0: original type/unit is
part of metric identity without a producer policy; unitless Speedscope weights
are generic count, not proven samples. Source 0.3.0 snapshots keep old bytes and
hashes/read/save/query/export, but comparisons require new original reimport.
New compiled policy snapshots disclose declared/file-exporter basis and warnings.
Durable verified reuse replays both originals on a new query and rechecks both
original/result byte hashes and full case bindings on every page, preserving
identity_basis=declared. R12 permits only a bounded process-local validated diff
cache; imported receipts never grant verification. Existing generic scalar comparison stays strict; the new
canonical/baseline consumer uses qualified policies. Clock alignment and unknown
producer promotion are not inferred. Native Windows/UI and independent review
remain separate from local implementation verification.

### T-638 metric history

[Exact metric history](METRIC_HISTORY.md) defines MetricHistory model/normalizer
1.0.0, `metrics.history-json` transport and `metrics_timeseries` payload 0.2.0 (saved 0.1.0 reader retained).
Neutral snapshot sequences, explicit-provenance Prometheus range matrices and
OTLP JSON preserve exact values, resource/scope/schema/attribute identity and
temporality. Delta/cumulative/reset/gap/overlap and explicit/exponential buckets
remain distinct. Generic detail/timeline and saved-result query/export share the
common envelope. SDK Python 1.38.0 synthetic fixtures qualify only that slice;
Q04/GenAI aliases and native acceptance stay separate. Legacy metrics_snapshot
semantics are unchanged; there is no automatic cross-series sum or quantile math.

## T-637 file-only system evidence

Limited canonical importer `system.evidence-json` / payload `system_evidence`
0.2.0: actual perf 6.1.190 synthetic-data sample export, constrained scheduler
text grammar, explicit event JSONL, and pinned bpftrace 0.22 histogram JSON shape.
Native scheduler/BPF collection, other layouts/versions and arbitrary scripts
remain unqualified. Sample count, period, scheduler durations and aggregate
buckets are separate. No binary perf decoder or live attach.
[Workflow, conditions and bounds](SYSTEM_EVIDENCE.md).


## T-639 device and serving evidence

Limited `inference.evidence-json` carries InferenceEvidence 1.0.0 into common
`inference_serving` 0.1.0. Original MetricHistory is stored once, signals bind
explicit series/entity/context/unit/form/population, and missing values remain
unavailable. Native NVIDIA CSV uses an explicit fixed query contract; fixtures
are synthetic and Q03 version-specific vLLM aliases remain gated. Parser, semantic
analyzer and generic/export consumers remain separate. See
[offline device/serving contract](INFERENCE_EVIDENCE.md) for bounds, precise
request formulas, original metric drilldown and unexecuted native acceptance.

SystemEvidence 1.0.0 / system_evidence 0.1.0 saved readers remain supported; new 1.1.0 models deduplicate frames/stacks and 0.2.0 results retain old snapshots unchanged.

## T-640 implementation boundary (2026-11-12)

The limited `trace.llm-json` importer normalizes LLMTrace 1.0.0 into existing trace_import 0.3.0 / llm_core 1.0.0. Legacy trace 0.2.0 remains unchanged. Parent/link, observed child-union durations, explicit logical/attempt accounting, token boundaries and lifecycle populations stay separate. Body-free normalization precedes case copies/indexing. Exact bounded query/UI and existing providers/exports are reused. Q01 vendor JSON adapters stay gated. See [LLM trace evidence](LLM_TRACE_EVIDENCE.md). T-639 R16 adds explicitly declared untyped range metadata and user event-ID normalization, with 1.0/1.1 model and 0.1/0.2 result readers preserved; see [inference evidence](INFERENCE_EVIDENCE.md).


## T-641 runtime evidence

Runtime evidence uses the separate bounded canonical model within AnalysisResult. See [RUNTIME_EVIDENCE.md](RUNTIME_EVIDENCE.md) for pinned producer formats, identity/window qualification, live allocation profile reuse, GC union, plan per-loop semantics and saved-reader boundaries.


## T-642 execution evidence

Bounded benchmark/crash context uses execution_core 1.0.0 inside common AnalysisResult, execution_evidence 0.1.0. Pinned k6/LLMPerf and structural JTL/actual sanitized HotSpot inputs normalize before generic providers/cases/exports. Comparison is explicitly declared-condition snapshot verification, separate from profile original-reimport baselines. See [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md).

T-643: `apple-ollama-capture-json` normalizes manual Apple Silicon/Ollama 0.40.16 snapshots to the existing metrics.history-json path; shared system bytes, per-model residency estimates and explicit unavailable telemetry. No NVIDIA identity, dedicated VRAM, automatic collector or GPU-utilization inference. Actual fixture scope and bounds: [APPLE_SILICON_EVIDENCE.md](APPLE_SILICON_EVIDENCE.md).

R19-01 adds execution_core 1.1.0 / execution_evidence 0.2.0 for exact large-JTL distributions; saved 1.0.0/0.1.0 stays unchanged. [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md).

T-644: trace.gpu-execution-json / trace_import0.4.0 / gpu_execution_core1.0.0 preserves exact scoped CPU/GPU/rank intervals, explicit correlations, separate kernel sum/union and summary-only evidence. CLI/Wails/Workspace and bounded selected SVG reuse common consumers; existing saved trace/LLM readers remain. [GPU_EXECUTION_EVIDENCE.md](GPU_EXECUTION_EVIDENCE.md).


T-645 bounded actual DB/OS snapshot batch: [Manual database and socket snapshots](SYSTEM_DB_SNAPSHOTS.md). Existing MetricHistory transport, exact epochs/gauges and common consumers; other adapters remain fixture-gated.
