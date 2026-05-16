# ArchScope Work Status

Last updated: 2026-05-16

This file is the current execution status for the active ArchScope product line.
The previous long-form history was archived to
`archive/work_status_legacy_2026-05-09.md`.

## Current Baseline

- Active product: unified Go/Wails desktop implementation under
  `apps/engine-native`.
- Active UI: Wails v3 React frontend under
  `apps/engine-native/cmd/archscope-app/frontend`.
- Active engine: Go parser/analyzer/exporter/AI interpretation modules under
  `apps/engine-native/internal`.
- Release baseline: `v0.3.4` is the latest stable GitHub release. The
  `v0.3.1-rc1` prerelease remains available as the Jennifer MSA network-time
  release candidate.
- Current execution focus: finish the remaining v0.3.6 access/edge-log
  hardening tasks T-483 and T-484 after completing the T-473 through T-482
  parser and projection coverage wave.
- Retired implementation: Python/FastAPI/browser sources are archived under
  `archive/python-engine` and `archive/web-frontend-python`.
- Historical native POC module has been folded into `apps/engine-native`.

## Completed In This Cycle

- Consolidated the Go engine, profiler core, Cobra CLIs, and Wails desktop app
  into `apps/engine-native`.
- Archived the Python engine and browser frontend instead of deleting them.
- Ported the evidence-bound AI interpretation module to Go.
- Updated release packaging around the Go/Wails desktop artifact.
- Rebuilt the `v0.3.0-rc1` release contents and changed the release from
  prerelease to latest.
- Verified representative Go parser/analyzer packages and profiler benchmarks.
- Audited large-file parsing behavior across logs, JSON inputs, profiler files,
  and thread-dump plugins.
- Aligned the active Go baseline with the latest stable Go toolchain
  (`go1.26.3`) and updated CI/release workflows to use `1.26.x`.
- Completed release verification for `v0.3.0-rc1`: local macOS Wails package,
  DMG validation, CLI analysis/export smoke tests, GitHub Release matrix review,
  and release asset checksum validation.
- Changed Jennifer profile analysis tabs so Summary, MSA Timeline, Profile
  Timeline, and Parser Report are visible before a file is analyzed.
- Optimized hot parser/profiler paths for common nginx access logs with
  response time and collapsed-stack flame graph construction.
- Optimized large graph rendering paths by capping displayed MSA timeline bars,
  reducing ECharts animation/resize churn, and using row-bucket hit testing for
  canvas flame graph hover lookup.
- Removed POC-era profiler-suffixed command/app path names from the active
  source and build surface; the desktop app now builds from
  `apps/engine-native/cmd/archscope-app`.
- Added Jennifer MSA service-call network-time summaries and network-aware
  topology placement so internal single-digit millisecond calls and
  gateway/external double-digit millisecond calls separate visually.
- Added `v0.3.1-rc1` and promoted the line to the stable `v0.3.1` desktop
  release.
- Added external trace import MVP support for OTLP JSON-file traces and
  Zipkin v2 JSON traces under `internal/parsers/traceimport` and
  `internal/analyzers/traceimport`.
- Added `archscope-engine trace import --in <file> --format
  auto|otlp-json|zipkin-v2-json` and trace sample fixtures under
  `examples/traces`.
- Researched product expansion and external APM import priorities around
  local-first trace evidence, Evidence Board, Incident Timeline,
  SLO/Golden Signals, service flow, and deferred SaaS connectors.
- Consolidated product expansion and external APM planning into the English and
  Korean roadmap documents, then removed the former Korean-only planning notes
  so `docs/en` and `docs/ko` Markdown files stay paired.
- Restored and extended GC log memory-space analysis so young/old/metaspace
  event series are emitted again, and added OOM plus long-pause alert findings.
- Connected `trace_import` to the Wails desktop UI with summary cards,
  service dependency and service latency charts, trace/span tables, critical
  path rows, parser diagnostics, and findings.
- Added Elastic APM file import for Elasticsearch `_search` response JSON and
  source-only NDJSON exports.
- Added trace critical-path analysis and MVP findings:
  `SLOW_TRACE_P95`, `CLOCK_SKEW_SUSPECTED`,
  `UNBALANCED_SERVICE_LATENCY`, and `HIGH_ERROR_SERVICE_EDGE`.
- Added the first Evidence Board skeleton with local evidence cards for trace
  findings, service edges, traces, and source metadata.
- Added a Windows GUI smoke workflow that builds the Wails Windows executable
  on `windows-latest`, launches `archscope.exe`, verifies it stays alive, and
  shuts it down cleanly.
- Hardened the 0.3.x release workflow with macOS signing/notarization secret
  preflight, signature/stapler verification, route-level frontend lazy loading,
  modular ECharts bundling, and explicit frontend chunk budgets.
- Clarified the JFR analyzer contract: the JDK `jfr` CLI is only the `.jfr` to
  `jfr print --json` conversion boundary, while ArchScope focuses on
  report-oriented summaries, stack evidence, hints, and Evidence Board capture.
- Added async-profiler-oriented JFR stack aggregation from `stackTrace.frames`
  into top methods, packages, threads, sample stacks, and a flamegraph-ready
  call tree.
- Added JFR recording UX hints for empty filters, missing stack samples,
  async-profiler-style sample recordings, and sparse stack samples.
- Expanded Evidence Board capture beyond Trace Import to Access Log findings,
  GC findings/alerts, JFR findings/profile rows, and native-memory call sites;
  added local HTML report and JSON evidence-pack export.
- Restored the selected Wails workspace/export parity surfaces: Analysis
  Workspace, Export Center, general report diff, and Chart Studio MVP.
- Added a session Analysis Workspace that retains successful analyzer outputs
  across page navigation and connects those results to Evidence Board,
  Export Center, report diff, and Chart Studio workflows.
- Prepared the stable `v0.3.2` desktop release with app/package metadata,
  Windows executable version metadata, changelog notes, local Windows build,
  and GUI launch smoke verification.
- Added Jaeger QueryService/local trace JSON import and schema-guarded
  SkyWalking GraphQL `queryTrace.spans` validation to the `trace_import`
  parser, analyzer tests, CLI format list, Wails Trace Import selector, and
  English/Korean data-model and roadmap documents.
- Added the Wails Incident Timeline MVP with a common session event model,
  cross-analyzer event mapping, filtering, and Evidence Board capture.
- Added the Wails SLO / Golden Signals MVP with session-wide signal inventory,
  SLI metric normalization, default SLO target evaluation, violation/error
  budget tables, affected-scope breakdowns, and Evidence Board capture.
- Added the Wails Service Flow MVP with a shared Trace Import/Jennifer MSA
  service-edge model, deterministic service-flow findings, Evidence Board
  capture, Mermaid sequence-like export, and JSON export.
- Prepared the stable `v0.3.3` desktop release with updated app/package
  metadata, changelog notes, and 0.3.x Evidence Studio release scope.
- Published the stable `v0.3.4` desktop release with Incident Timeline mapping
  fixes, AI gate hardening, SLO unit/deduplication fixes, report-pack ZIP path
  sanitization, derived export alignment, and frontend state regression tests.
- Added the Mid-Term Plus shared ingestion foundations for `v0.3.5`: evidence
  family boundary specs, reusable source-format registry, golden fixture
  diagnostic harness, normalized source metadata, and cross-source correlation
  key model.
- Added the first access/edge-log coverage wave for `v0.3.6`: Tomcat/Jetty,
  HAProxy HTTP, Envoy/Istio text and JSON, AWS ELB/ALB/CloudFront, GCP/Azure
  edge JSON, IIS W3C, Caddy/Traefik JSON, and Kong/Tyk/API Gateway JSON parsing
  now feed additive access-log summaries, edge dependency tables, Service Flow,
  and Golden Signals.

## Current Risk

The Electron-to-Wails migration risk is closed. The highest large-file issue
found in the 2026-05-09 audit has been mitigated: GC log analysis no longer
emits chart series for every event, and access-log/OTel analyzer entrypoints no
longer materialize the full parser record slice before aggregation.

Release verification found no new blocker for the 0.3.x Go/Wails line. Direct
local Windows desktop testing is still environment-dependent, but automated
Windows GUI smoke now runs on a GitHub-hosted Windows Server 2025 runner. The
smoke builds the Wails Windows executable, launches `archscope.exe`, verifies
that the GUI process stays alive for the startup window, and shuts it down.
Windows confidence now includes tests/builds/packages, release artifact
checksum and PE inspection, plus the GUI launch smoke.

Release hardening now fails fast when macOS Developer ID signing or Apple-ID
notarization secrets are only partially configured. The release workflow also
verifies the built app signature and validates the stapled ticket when
notarization credentials are available. The Wails frontend startup shell is
split from the analyzer pages and shared chart runtime, so the initial bundle
is within the documented release budget.

The MSA timeline discoverability issue is closed: the tab list now renders
before analysis and each tab shows a neutral empty-result state until data is
available. Large MSA timelines are display-capped for browser responsiveness.
Jennifer MSA topology now uses network-time groups to estimate service
distance, while the underlying `AnalysisResult` tables retain detailed call
metrics.

Trace import is now a desktop workflow, not only an engine/CLI MVP. OTLP
JSON-file, Zipkin v2 JSON, Elastic APM `_search` JSON, Elastic source NDJSON,
Jaeger QueryService/local trace JSON, and schema-guarded SkyWalking GraphQL
`queryTrace.spans` responses are covered. The UI exposes summary metrics,
service dependencies, traces, spans, critical paths, parser diagnostics,
deterministic findings, and basic Evidence Board capture. Evidence Board
capture also now covers selected non-trace analyzers and can export local
HTML/JSON evidence packs. Broader SaaS connectors remain roadmap candidates
until they are explicitly promoted into the active TO-DO.

JFR analysis now has an explicit product boundary and async-profiler-oriented
stack evidence, but direct JFR JSON loading is still an object-materializing
path. Large recordings should still be constrained with `jfr print` event,
time-window, or stack-depth filters before analysis.

Remaining large-file risk is concentrated in structured formats that naturally
require object materialization, such as JFR JSON, Node diagnostic reports, jcmd
JSON, and self-contained HTML profiler files. These paths now have documented
guardrails or targeted preflight, but multi-GB structured inputs should still be
filtered before analysis.

## Large-File Audit Snapshot

| Analyzer | Synthetic input | Time | Peak RSS | Output | Status |
|---|---:|---:|---:|---:|---|
| Access log | 30 MB / 300k lines | 0.53s | 19 MB | 10 KB | Streaming parser/analyzer path verified |
| OTel JSONL | 31 MB / 200k lines | 0.25s | 18 MB | 6.3 KB | Streaming parser/analyzer path with trace-detail cap verified |
| GC log | 34 MB / 300k lines | 1.55s | 305 MB | 3.5 MB | Series cap/downsampling verified |
| Collapsed profiler | 29 MB / 500k lines | 0.26s | 16 MB | 6.8 KB | Good baseline |

## Next Execution Queue

1. Finish T-483 and T-484 to harden auto-detect diagnostics and formal
   access-log edge metadata contract coverage.
2. Ship `v0.3.6` after the first user-visible access/edge log coverage wave
   lands, because it already has a mature `AnalysisResult`, SLO, Incident
   Timeline, and report path.
3. Cut or verify `v0.3.5` foundation release artifacts if the release line
   needs a separate foundation checkpoint before `v0.3.6`.
4. Continue one release per evidence family through server logs, OpenTelemetry
   Logs, database slow-query evidence, broker logs, Kubernetes/cloud evidence,
   and multi-language profiling.
5. Reserve `v0.4.0` for the completed Mid-Term Plus roll-up after cross-source
   evidence stitching and continuous-profiling imports have stabilized.
6. Keep release verification healthy before each release cut by repeating
   Windows GUI smoke, macOS signing/notarization validation, frontend bundle
   budget checks, and representative real-export fixture updates.

## Review Intake Decisions

| Review | Decision | Follow-up |
|---|---|---|
| 2026-05-16 mid-term roadmap review P0-1 through P0-6 | Accepted | T-452 through T-457 |
| P1-7 UI-heavy derived analysis and P1-8 AnalysisResult drift | Accepted as phased remediation, not a single rewrite | T-458, T-459 |
| P1-9 exporter recomputes analysis | Accepted | T-460 |
| P1-10 service-edge identity matching | Accepted | T-461 |
| P1-11 AI provenance depth | Accepted | T-462 |
| P1-12 AI/deterministic visual separation after Evidence Board capture | Accepted | T-463 |
| P2 determinism, timestamp, runtime-stack, testing, shared utility, Mermaid, narrative sorting, and dedupe issues | Accepted and grouped by risk/dependency | T-464 through T-467 |
| Full immediate migration of all Wails state projections to Go analyzers | Deferred as too broad for one patch; start with Service Flow and migrate the remaining projections after the contract pattern is proven | T-458, T-459 |
| Korean PII allow-list coverage | Accepted as a review point inside the privacy hardening task; exact rule set needs implementation-time examples | T-454 |

## Mid-Term Plus Intake Plan

| Classification | Priority | TO-DO range | Target release | Execution rule |
|---|---|---|---|---|
| Shared ingestion foundations | P1 | T-468 through T-472 | v0.3.5 | Build first; every new source should reuse these contracts, diagnostics, fixtures, and correlation keys. |
| Access and edge logs | P1 | T-473 through T-484 | v0.3.6 | First functional coverage wave because it extends existing access-log, SLO, Incident Timeline, and report-pack paths. |
| Application and web server logs | P1/P2 | T-485 through T-492 | v0.3.7 | Start with Tomcat/Jetty and nginx/Apache error logs, then add enterprise app-server variants. |
| Observability logs and metrics | P1/P2 | T-493 through T-498 | v0.3.8 | Add OpenTelemetry Logs first, then offline metrics and LGTM export formats. |
| Database slow-query and engine logs | P1/P2 | T-499 through T-506 | v0.3.9 | Prioritize PostgreSQL/MySQL slow-query evidence before broader database engines. |
| Broker and streaming middleware | P1/P2 | T-507 through T-513 | v0.3.10 | Prioritize Kafka and RabbitMQ before Pulsar, NATS, and ActiveMQ. |
| Kubernetes, container, and cloud evidence | P2 | T-514 through T-519 | v0.3.11 | Add platform context after app/server/database/broker evidence contracts are available. |
| Multi-language stack and profiler evidence | P1/P2 | T-520 through T-528 | v0.3.12 | Add generic pprof first, then raw language-specific profiler inputs and unified rollups. |
| Correlation and evidence stitching | P1/P2 | T-529 through T-531 | v0.3.13 | Requires enough source coverage to validate stitching and correlation-gap findings. |
| Continuous profiling imports and docs | P2/P3 | T-532 through T-536 | v0.3.14 | Add snapshot imports first; keep OTLP Profiles as a spec-tracking decision item. |

## Version Milestone Plan

| Release | Complete through | Release type | Required contents before cut |
|---|---|---|---|
| v0.3.5 | T-468 through T-472 | Foundation release | Shared ingestion architecture, source-format registry, golden fixture/diagnostic harness, normalized source metadata, and correlation-key model are implemented and documented. |
| v0.3.6 | T-473 through T-484 | User-visible evidence release | Tomcat/Jetty, HAProxy, Envoy/Istio, cloud edge, IIS/Caddy/Traefik, and gateway access formats are imported through auto-detect and remain compatible with the existing access-log contract. |
| v0.3.7 | T-485 through T-492 | Server diagnostics release | Server-log contract, Tomcat/Jetty, nginx/Apache error logs, and at least the planned enterprise app-server parsers feed Incident Timeline, Evidence Board, and report packs. |
| v0.3.8 | T-493 through T-498 | Observability evidence release | OpenTelemetry Logs, offline metrics, Loki/Tempo exports, and Grafana dashboard references are accepted as local evidence and mapped into Golden Signals/report workflows. |
| v0.3.9 | T-499 through T-506 | Database evidence release | PostgreSQL and MySQL/MariaDB slow-query evidence, EXPLAIN plan cards, and the broader database engine parser set integrate with Timeline, SLO, Service Flow, and reports. |
| v0.3.10 | T-507 through T-513 | Broker evidence release | Kafka and RabbitMQ evidence are complete, follow-up broker parsers are in place, and broker findings appear beside service-edge evidence. |
| v0.3.11 | T-514 through T-519 | Platform evidence release | Kubernetes events/pod evidence, kubelet/runtime logs, and cloud audit logs can back Timeline, Evidence Board, SLO saturation, and security report sections. |
| v0.3.12 | T-520 through T-528 | Multi-language profiling release | Unified profile schema, generic pprof, async-profiler parity, and priority language/runtime profiler inputs flow through flamegraph and Evidence Board paths. |
| v0.3.13 | T-529 through T-531 | Cross-source stitching release | Correlation-key stitching joins access, trace, runtime stack, broker, and database evidence and emits deterministic correlation-gap findings. |
| v0.3.14 | T-532 through T-536 | Continuous profiling plus docs release | Pyroscope/Phlare and Parca snapshot imports are routed through profile analysis, OTLP Profiles tracking is documented, and English/Korean docs/support matrices are current. |
| v0.4.0 | T-468 through T-536 plus stabilization | Mid-Term Plus roll-up | Full Mid-Term Plus scope is smoke-tested as one product story with sample packs, report exports, regression tests, and release notes that present the expanded Evidence Studio capability coherently. |

## Active TO-DO

| ID | Priority | Status | Task | Depends on | Output |
|---|---|---|---|---|---|
| T-414 | P1 | [x] | Connect `trace_import` to the Wails UI with summary cards, service dependency view, trace table, span table, and findings panel. | Trace import MVP | Completed 2026-05-13: Trace Import desktop workflow |
| T-415 | P1 | [x] | Add Elastic APM `_search` response and source-only NDJSON importers. | Trace import MVP | Completed 2026-05-13: Elastic trace evidence import |
| T-416 | P1 | [x] | Add trace critical-path analysis and current MVP findings: `SLOW_TRACE_P95`, `CLOCK_SKEW_SUSPECTED`, `UNBALANCED_SERVICE_LATENCY`, and `HIGH_ERROR_SERVICE_EDGE`. | Trace import MVP | Completed 2026-05-13: Root-cause oriented trace diagnostics |
| T-417 | P1 | [x] | Design and build the Evidence Board skeleton around reusable evidence cards. | Analyzer result contracts | Completed 2026-05-13: Cross-analyzer evidence pack foundation |
| T-418 | P1 | [x] | Run direct Windows GUI launch smoke-test for the 0.3.1 line on a Windows host/VM. | 0.3.1 release assets | Completed 2026-05-13: Windows runner GUI process launch smoke |
| T-419 | P2 | [x] | Continue 0.3.x release hardening for signing/notarization and frontend bundle splitting. | 0.3.1 release baseline | Completed 2026-05-13: signing/notarization preflight plus frontend bundle split |
| T-420 | P1 | [x] | Clarify the JFR analyzer contract: use the JDK `jfr` CLI only for `.jfr` to `jfr print --json` conversion, and avoid reimplementing the full JDK `jfr view` / `jfr summary` feature set in the desktop UI. | Existing JFR parser bridge | Completed 2026-05-13: JFR scope note, metadata contract, and UI copy |
| T-421 | P1 | [x] | Add async-profiler-oriented JFR stack analysis by aggregating `stackTrace.frames` from sample events into top methods, top packages, top threads, and call-tree/flamegraph-ready rows. | JFR JSON parser, profiler flamegraph components | Completed 2026-05-13: async-profiler stack evidence tables and flamegraph-ready tree |
| T-422 | P2 | [x] | Add JFR recording UX hints that detect sparse or mode-specific recordings, especially async-profiler JFR files with mostly `jdk.ExecutionSample`, `jdk.NativeMethodSample`, `jdk.ObjectAllocationSample`, or lock events. | T-421 preferred | Completed 2026-05-13: empty-filter, missing-stack, async-profiler-style, and sparse-stack hints |
| T-423 | P2 | [x] | Expand Evidence Board capture beyond Trace Import and add report export around saved evidence cards. | T-417 | Completed 2026-05-13: non-trace evidence capture plus HTML/JSON evidence export |
| T-424 | P1 | [x] | Restore a Wails Export Center page and menu entry using the existing `ExportJSON`, `ExportHTML`, `ExportPPTX`, `ExportCSV`, and `ExportCSVDir` engine bindings for full `AnalysisResult` exports. | Engine exporter bindings | Completed 2026-05-13: desktop Export Center for JSON/HTML/PPTX/CSV artifacts |
| T-425 | P1 | [x] | Add an Analysis Workspace / Recent Results surface that stores successful analyzer results across page navigation and lets users send them to Evidence Board, Export Center, or comparison workflows. | T-424 preferred, current analyzer pages | Completed 2026-05-13: reusable session result registry and workspace UI |
| T-426 | P2 | [x] | Add a general AnalysisResult comparison workflow around the existing `DiffReports` engine binding so non-profiler JSON results can be compared before/after or release-to-release. | T-425 preferred | Completed 2026-05-13: normalized AnalysisResult JSON comparison page |
| T-427 | P2 | [x] | Reintroduce a focused Chart Studio MVP for Wails with chart template selection, analyzer-result preview, and PNG/SVG/CSV-style chart export before deeper custom chart editing. | T-425 preferred, chart/export foundation | Completed 2026-05-13: report-ready chart preview/export workflow |
| T-428 | P1 | [x] | Decide and implement the Jaeger trace import contract for the file-first compatibility pass. | Trace import MVP | Completed 2026-05-15: Jaeger QueryService/local trace JSON importer, auto-detect, fixtures, CLI/UI format option |
| T-429 | P1 | [x] | Add SkyWalking schema/version handling before widening import support. | Trace import MVP | Completed 2026-05-15: schema-guarded SkyWalking GraphQL `queryTrace.spans` importer diagnostics and fixture coverage |
| T-430 | P2 | [x] | Update trace compatibility fixtures, tests, and English/Korean docs after Jaeger/SkyWalking work. | T-428, T-429 | Completed 2026-05-15: parser/analyzer tests, data-model docs, roadmap, Trace Import UI/CLI format list |
| T-431 | P1 | [x] | Define a common Incident Timeline event model for timestamp/range, source analyzer, severity, label, and evidence reference. | Analysis Workspace, Evidence Board | Completed 2026-05-15: reusable Wails session timeline event model |
| T-432 | P1 | [x] | Map existing analyzer results into Incident Timeline events across findings, access-log bursts/latency, GC alerts, JFR pauses/notable events, exceptions, thread dumps, and trace import. | T-431 | Completed 2026-05-15: cross-analyzer event builder from Analysis Workspace results |
| T-433 | P1 | [x] | Add a Wails Incident Timeline workspace page with event filters and Evidence Board capture. | T-431, T-432 | Completed 2026-05-15: desktop Incident Timeline MVP under Workspace |
| T-434 | P1 | [x] | Promote the Wails session Incident Timeline into an engine-level or exportable `AnalysisResult` when report-pack generation needs persisted timeline data. | T-431, T-432, T-433 | Completed 2026-05-16: exportable `incident_timeline` AnalysisResult projection |
| T-435 | P2 | [x] | Add richer Incident Timeline event ranges, correlation IDs, and timeline grouping for multi-file incidents. | T-434 | Completed 2026-05-16: event ranges, correlation IDs, group metadata, grouped UI filters, and exportable `tables.groups` |
| T-436 | P2 | [x] | Add Incident Timeline narrative support that explains what happened and in what order during an incident. | T-435, T-446 preferred | Completed 2026-05-16: deterministic evidence-linked narrative steps in UI, exportable timeline, and report packs |
| T-437 | P1 | [x] | Build a Golden Signals inventory across access logs, trace import, Jennifer MSA, exceptions, GC, JFR, thread dumps, and JVM signals. | T-425, T-433 | Completed 2026-05-16: session-wide Golden Signals inventory model |
| T-438 | P1 | [x] | Define SLI metrics for latency, traffic, errors, and saturation from the Golden Signals inventory. | T-437 | Completed 2026-05-16: normalized latency, traffic, error, and saturation SLI metric model |
| T-439 | P1 | [x] | Add SLO target configuration, violating-window detection, error-budget burn tables, and affected service/endpoint breakdowns. | T-438 | Completed 2026-05-16: session SLO targets, violations, error-budget, and affected-scope breakdowns |
| T-440 | P1 | [x] | Add a Wails SLO / Golden Signals workspace page with signal inventory, SLO violations, and Evidence Board capture. | T-437, T-438, T-439 | Completed 2026-05-16: desktop SLO / Golden Signals workspace workflow |
| T-441 | P1 | [x] | Unify Jennifer MSA topology and trace-import dependency models. | T-427, T-430 | Completed 2026-05-16: shared service-flow input model for Trace Import and Jennifer MSA |
| T-442 | P1 | [x] | Define a common service-edge schema with caller, callee, call count, average/max/total latency, error count, and network gap. | T-441 | Completed 2026-05-16: normalized service-edge contract with latency, error, and network-gap aggregation |
| T-443 | P1 | [x] | Normalize unmatched calls, missing parents, and network gaps into service-edge findings. | T-442 | Completed 2026-05-16: deterministic service-flow findings for unmatched calls, missing parents, and high network gaps |
| T-444 | P2 | [x] | Add C4 dynamic view or sequence-like export for service-flow evidence. | T-442, T-443 | Completed 2026-05-16: Wails Service Flow page with Mermaid sequence-like and JSON export |
| T-445 | P1 | [x] | Generate report-ready HTML, ZIP, and later PPTX/PDF outputs from Evidence Board content. | T-423, T-434 preferred | Completed 2026-05-16: Evidence Board report pack HTML/ZIP export with session artifacts |
| T-446 | P1 | [x] | Preserve source metadata, analyzer options, captured evidence, deterministic findings, and optional AI interpretation provenance in report packs. | T-445 | Completed 2026-05-16: report pack provenance contract for source results, evidence, findings, derived artifacts, and AI provenance |
| T-447 | P2 | [x] | Support customer-facing summaries without hiding raw evidence behind conclusions. | T-445, T-446 | Completed 2026-05-16: evidence-linked customer summary with raw evidence appendix |
| T-448 | P2 | [x] | Surface AI interpretation provenance in the UI. | AI interpretation module, T-425 | Completed 2026-05-16: Analysis Workspace AI provenance status with provider/model/prompt metadata |
| T-449 | P2 | [x] | Keep AI findings visually separate from deterministic findings. | T-448 | Completed 2026-05-16: separate AI-assisted findings panel with confidence, reasoning, limitations, and evidence refs |
| T-450 | P1 | [x] | Add AI evaluation gates using golden diagnostics, evidence-reference integrity, quote-to-source matching, low-confidence filtering, and hallucination review. | T-448, T-449 | Completed 2026-05-16: Go/UI AI gate for schema, evidence refs, quote matching, confidence, and hallucinated refs |
| T-451 | P1 | [x] | Connect AI interpretation to Evidence Board and report generation only when every generated claim has valid evidence references. | T-446, T-450 | Completed 2026-05-16: gated AI Evidence Board capture and Report Pack AI findings |
| T-452 | P0 | [x] | Fix Incident Timeline cross-analyzer mappings so thread dump, lock contention, multi-thread dump, and exception events read the table keys actually emitted by Go analyzers, or add the missing analyzer tables where that is the better contract. Add regression fixtures for each corrected mapping. | T-431, T-432 | Completed 2026-05-16: Incident Timeline now reads `thread_dump`, `thread_dump_multi`, and `thread_dump_locks` tables emitted by Go analyzers; lock-contention UI accepts the Go result type |
| T-453 | P0 | [x] | Make AI interpretation gating authoritative through the Go `internal/aiinterpretation` evaluator exposed via Wails bindings; keep TypeScript as presentation logic only, require non-empty model/summary/reasoning fields, require evidence quotes when quote matching is enabled, make minimum confidence configurable, and preserve validation issues with provenance. | T-448, T-449, T-450, T-451 | Completed 2026-05-16: exposed `EvaluateAiInterpretation` through EngineService, aligned the TS gate issue codes with Go, required model/summary/reasoning, configurable confidence, and quote-required diagnostics |
| T-454 | P0 | [x] | Harden privacy redaction before any LLM dispatch by adding allow-list or stronger deny-list coverage for stack traces, hostnames, IPv6, bearer tokens, JWTs, SQL fragments, and Korean PII patterns, with Go tests for redaction edge cases. | T-453 preferred | Completed 2026-05-16: redacts bearer/JWT secrets, host/URL targets, IPv6, filesystem paths in stack traces, SQL literals, Korean phone/RRN, and existing email/IP/token patterns before prompt construction |
| T-455 | P0 | [x] | Normalize SLO error-rate units across access-log and trace-import signals, add explicit fraction/percent conversion at the SLI boundary, and fix error-budget burn formulas so objective percent and threshold semantics are applied correctly. | T-437, T-438, T-439, T-440 | Completed 2026-05-16: percent signals now carry explicit `rate_unit`, trace-import fractions are converted to percent at signal normalization, and budget consumption uses target semantics for upper/lower percent bounds |
| T-456 | P0 | [x] | Prevent Trace Import and Jennifer MSA traffic double-counting in Golden Signals by making aggregation source-aware and deduplicating equivalent caller-to-callee traffic where both analyzers describe the same edge. | T-441, T-455 | Completed 2026-05-16: equivalent service-edge traffic signals share a metric family and use source-aware max aggregation across analyzer totals instead of summing trace-import and Jennifer MSA observations |
| T-457 | P0 | [x] | Sanitize report-pack ZIP entry paths with a shared path-normalization helper that rejects absolute, parent-directory, and platform-specific traversal forms before writing archive headers. | T-445 | Completed 2026-05-16: report-pack ZIP entries now pass through `normalizeZipEntryPath`, rejecting absolute paths, drive/UNC forms, parent/current directory segments, empty segments, NUL bytes, encoded dot segments, and duplicate archive names before writing headers |
| T-458 | P1 | [x] | Move Service Flow toward the first Go analyzer/exportable `AnalysisResult` pattern for derived Evidence Studio workflows, including parser/findings/diagnostics metadata, CLI/exporter compatibility, service-edge normalization, and fixtures. | T-441, T-442, T-443, T-444 | Completed 2026-05-16: Service Flow export now uses a `service_flow` AnalysisResult envelope with `summary/series/tables/charts/metadata`, parser/diagnostics/findings metadata, source-result extras, and compatibility top-level edge/finding fields |
| T-459 | P1 | [x] | Apply the proven derived-analysis contract pattern to Incident Timeline and SLO/Golden Signals projections so exported payloads satisfy the common `AnalysisResult` envelope and remain readable by Go exporters. | T-458, T-452, T-455 | Completed 2026-05-16: Incident Timeline metadata now includes parser/diagnostics/findings/extra fields; SLO / Golden Signals now has an exportable `slo_golden_signals` AnalysisResult envelope with tables for signals, metrics, targets, violations, budget rows, and affected breakdown |
| T-460 | P1 | [x] | Stop report-pack export from recomputing analysis at export time; report packs should consume persisted Analysis Workspace results and derived `AnalysisResult` artifacts produced earlier in the workflow. | T-458, T-459, T-445 | Completed 2026-05-16: report-pack artifact resolution now prefers existing `incident_timeline`, `slo_golden_signals`, and `service_flow` workspace AnalysisResults and filters derived artifacts out of source-result provenance, with one compatibility fallback for sessions that have not persisted derived artifacts yet |
| T-461 | P1 | [x] | Add service identity normalization for Service Flow and SLO aggregation, including case, hyphen/underscore, display-name, and alias-table matching for Trace Import and Jennifer MSA service names. | T-441, T-458 | Completed 2026-05-16: Service Flow and SLO edge/service grouping now use a shared canonical service identity that handles case, camel/display names, hyphen/underscore variants, URL hosts, ports, and an alias-table hook |
| T-462 | P1 | [x] | Strengthen AI provenance in Evidence Board and report packs with prompt hash, generation parameters, token counts when available, source/evidence integrity hashes, and validation status. | T-453, T-446 | Completed 2026-05-16: report-pack AI provenance now records prompt hash/params/token counts when present, source integrity hashes, evidence integrity hashes, and validation status alongside gate issues |
| T-463 | P1 | [x] | Preserve AI-assisted versus deterministic finding separation after Evidence Board capture and in report-pack rendering, including visible AI badges, confidence, limitations, and evidence-reference status. | T-449, T-451 | Completed 2026-05-16: report-pack rendering keeps deterministic findings separate from accepted AI findings and shows AI-assisted badges, validation status, confidence, limitations, and evidence quote status |
| T-464 | P2 | [x] | Make derived workflow exports deterministic by injecting `now` explicitly, supporting fixed ZIP timestamps, parsing Unix-second timestamps, replacing locale-dependent formatting, stabilizing dedupe keys, and optimizing narrative sorting with precomputed event maps. | T-457, T-459 | Completed 2026-05-16: Incident Timeline parses Unix-second timestamps and uses stable ISO labels, narrative sorting uses precomputed event maps, report-pack ZIP builders support fixed timestamps, and report formatting now avoids locale-default number formatting in updated derived surfaces |
| T-465 | P2 | [x] | Expand SLO and runtime signal robustness with configurable SLO target overrides, safe non-spread max scans for large series, and runtime-stack signal extraction for Node.js, Python, Go panic, and .NET analyzer result types. | T-455 | Completed 2026-05-16: added SLO target override merging, non-spread max/min scans, and Golden Signal extraction for `nodejs_stack`, `python_traceback`, `go_panic`, and `dotnet_exception_iis` runtime results |
| T-466 | P2 | [x] | Add frontend state regression tests for Incident Timeline, SLO/Golden Signals, Service Flow, Report Pack, and AI gate behavior, plus Go AI redaction/evidence-reference tests for the new hardening paths. | T-452, T-453, T-454, T-455, T-456, T-457 | Completed 2026-05-16: added `npm run test:state` with frontend state regressions for service identity, SLO percent conversion/dedupe, runtime signals, Mermaid source-only findings, Unix timestamps, and AI quote enforcement; Go AI hardening tests were added with T-453/T-454 |
| T-467 | P2 | [x] | Deduplicate shared frontend state utilities and fix Mermaid sequence export edge cases, including empty-service/source-only findings and quoted labels. | T-458, T-461 | Completed 2026-05-16: service identity canonicalization is shared by Service Flow and SLO, Mermaid source-only findings now get a fallback participant, quoted labels continue to use JSON escaping, and large-series max scans avoid spread-based limits |
| T-468 | P1 | [x] | Define the Mid-Term Plus ingestion architecture boundary: parser package layout, analyzer package layout, CLI command naming, Wails binding pattern, and `AnalysisResult` naming for new evidence families. | Roadmap Mid-Term Plus | Completed 2026-05-16: `internal/ingestion` evidence family specs define parser/analyzer packages, CLI leaves, Wails bindings, and result types for Mid-Term Plus source families |
| T-469 | P1 | [x] | Add a reusable source-format registry and auto-detect contract that can be shared by access logs, trace import, server logs, database logs, broker logs, and platform evidence. | T-468 | Completed 2026-05-16: reusable bounded-probe `FormatRegistry`, `SourceFormat`, detector, and unknown-format contract |
| T-470 | P1 | [x] | Create a golden fixture and parser diagnostic harness for new importers, including valid, partial, malformed, unknown-format, and large-file samples. | T-468 | Completed 2026-05-16: parser-agnostic fixture expectation/check harness covers valid, partial, malformed, unknown-format, and large-file diagnostic assertions |
| T-471 | P1 | [x] | Normalize source metadata fields for new evidence families, including source kind, source format, product, version, host, service, environment, and sanitized file identity. | T-468 | Completed 2026-05-16: `SourceMetadata`/`FileIdentity` contract plus additive access-log and trace-import `metadata.source_metadata` |
| T-472 | P1 | [x] | Define the cross-source correlation-key model for trace ID, span ID, request ID, tenant/customer ID, container ID, pod UID, host ID, PID, and source timestamp windows. | T-468, T-471 | Completed 2026-05-16: `CorrelationKeys` with normalized stable IDs and timestamp windows; trace import now emits bounded correlation-key samples |
| T-473 | P1 | [x] | Add Tomcat access valve and Jetty NCSA request-log parsers with format auto-detection fixtures. | T-469, T-470 | Completed 2026-05-16: Tomcat and Jetty now use the combined/common access parser path with source-format tagging and auto-detect coverage |
| T-474 | P1 | [x] | Map Tomcat and Jetty access records into the existing access-log `AnalysisResult` contract without breaking nginx/apache fields. | T-473 | Completed 2026-05-16: Tomcat/Jetty records flow through existing access-log summaries, series, sample records, and diagnostics without changing required nginx/apache fields |
| T-475 | P1 | [x] | Add HAProxy default and HTTP log parsers with backend/server timing fields and diagnostics for partial timing data. | T-469, T-470 | Completed 2026-05-16: HAProxy HTTP parser extracts backend/server, queue/connect/response timings, termination state, retry count, and classified parse diagnostics |
| T-476 | P1 | [x] | Analyze HAProxy access evidence for latency, status, backend saturation, retry, termination-state, and error-burst findings. | T-475 | Completed 2026-05-16: access-log analyzer now summarizes gateway latency/retries/termination errors and emits HAProxy/edge findings plus service-edge tables |
| T-477 | P1 | [x] | Add Envoy and Istio default text plus JSON access-log parsers, including service-mesh trace headers and upstream cluster metadata. | T-469, T-470, T-472 | Completed 2026-05-16: Envoy/Istio text and JSON parsers capture upstream cluster/service, route, response flags, TLS, trace ID, and request ID metadata |
| T-478 | P1 | [x] | Feed Envoy/Istio upstream service, trace ID, TLS, response flag, and route data into access-log summaries, Service Flow, and Golden Signals. | T-477 | Completed 2026-05-16: upstream service distributions, route stats, service dependency rows, access edge Service Flow edges, and access edge Golden Signals are emitted |
| T-479 | P2 | [x] | Add AWS ELB/ALB classic, ALB v2, and CloudFront standard log parsers with cloud source metadata. | T-469, T-470, T-471 | Completed 2026-05-16: AWS ELB, ALB, and CloudFront access rows parse with AWS cloud provider and edge/location or load-balancer metadata |
| T-480 | P2 | [x] | Add GCP HTTP(S) Load Balancer JSON and Azure App Service/Front Door JSON access-log parsers. | T-469, T-470, T-471 | Completed 2026-05-16: GCP HTTP LB JSON and Azure App Service/Front Door JSON parse request, status, latency, host/service, backend, route, and trace fields |
| T-481 | P2 | [x] | Add IIS W3C extended log and Caddy/Traefik JSON access-log parsers. | T-469, T-470 | Completed 2026-05-16: IIS W3C field headers, Caddy JSON, and Traefik JSON parse into normalized access-log records with route/upstream/backend metadata where available |
| T-482 | P2 | [x] | Add Kong, Tyk, and AWS API Gateway access-log parsers with route, consumer, upstream, and gateway-latency fields. | T-469, T-470, T-472 | Completed 2026-05-16: Kong, Tyk, and AWS API Gateway JSON access logs parse gateway latency, route, consumer, upstream service, request ID, and trace ID fields |
| T-483 | P1 | [x] | Wire all new access/edge formats into access-log auto-detect dispatch and per-source parser diagnostics. | T-473, T-475, T-477 | Completed 2026-05-16: `auto` dispatch covers the added access/edge formats and `metadata.source_format_diagnostics` reports selected format, parsed-by-format counts, and skipped reasons |
| T-484 | P1 | [x] | Extend the access-log contract and tests for upstream service, mesh trace ID, gateway latency, TLS, route, and cloud-edge metadata while keeping existing exports compatible. | T-474, T-476, T-478, T-483 | Completed 2026-05-16: optional access/edge summary, series, service dependency, route, and sample-record metadata are documented and covered by parser/analyzer/frontend state tests |
| T-485 | P1 | [x] | Define the server-log `AnalysisResult` contract and finding taxonomy for startup, deployment, datasource pool, stuck thread, hung thread, severe error, and worker error events. | T-468, T-471, T-472 | Completed 2026-05-16: `server_log` contract, tables, summaries, source metadata, correlation keys, and finding taxonomy are documented and implemented |
| T-486 | P1 | [x] | Add Tomcat catalina.out and Jetty server-log parsers/analyzers with startup, deployment, datasource, stuck-thread, and severe-error findings. | T-485, T-470 | Completed 2026-05-16: Tomcat and Jetty server-log selectors parse into normalized server events and analyzer findings |
| T-487 | P2 | [x] | Add JBoss/WildFly server-log parsing and findings for deployment failures, datasource warnings, thread pool pressure, and severe errors. | T-485, T-470 | Completed 2026-05-16: JBoss/WildFly log selectors parse deployment, datasource, thread-pool, and error evidence |
| T-488 | P2 | [x] | Add WebLogic AdminServer/ManagedServer log parsing and findings for deployment, stuck thread, JDBC pool, and managed-server health events. | T-485, T-470 | Completed 2026-05-16: WebLogic angle-bracket server logs map JDBC, deployment, thread, and managed-server health events |
| T-489 | P2 | [x] | Add WebSphere SystemOut/SystemErr parsing and findings for hung threads, application lifecycle events, datasource warnings, and severe errors. | T-485, T-470 | Completed 2026-05-16: WebSphere SystemOut/SystemErr rows parse severity, component, timestamp, hung-thread, lifecycle, datasource, and error events |
| T-490 | P2 | [x] | Add GlassFish/Payara server-log parsing and findings for deployment, JDBC pool, thread pool, and severe application-server errors. | T-485, T-470 | Completed 2026-05-16: GlassFish/Payara bracketed logs parse into server events with deployment/JDBC/thread-pool/error taxonomy |
| T-491 | P1 | [x] | Add nginx and Apache error-log parsers/analyzers so worker errors can be correlated with access-log request evidence. | T-485, T-472 | Completed 2026-05-16: nginx and Apache error logs emit worker/upstream error rows plus request/trace correlation candidates |
| T-492 | P1 | [x] | Map server-log and web-server error findings into Incident Timeline, Evidence Board, and report-pack artifacts with access/error correlation where keys are available. | T-486, T-491 | Completed 2026-05-16: server-log findings flow through generic Evidence Board/report-pack paths, server-log events feed Incident Timeline, and Golden Signals captures server error/thread saturation counts |
| T-493 | P1 | [x] | Define and implement OpenTelemetry Logs OTLP JSON and NDJSON parser contracts with severity, body, attributes, resource metadata, and trace/span references. | T-468, T-470, T-472 | Completed 2026-05-16: OTel parser now accepts JSONL/NDJSON records and OTLP Logs `resourceLogs` with attributes/resource metadata and trace/span references |
| T-494 | P1 | [x] | Add the OpenTelemetry Logs analyzer with severity bursts, error signatures, resource grouping, and trace/span correlation diagnostics. | T-493 | Completed 2026-05-16: OTel analyzer emits resource groups, error signatures, severity bursts, and existing trace/span topology diagnostics |
| T-495 | P2 | [x] | Add Prometheus snapshot and OpenMetrics import for offline metrics evidence and Golden Signals enrichment. | T-468, T-470 | Completed 2026-05-16: `metrics_snapshot` parser/analyzer imports Prometheus/OpenMetrics text and exposes Golden Signal candidate rows |
| T-496 | P2 | [x] | Add Loki query JSON export and Tempo trace JSON export importers so LGTM stack exports become first-class local evidence sources. | T-493, Trace import MVP | Completed 2026-05-16: `observability_evidence` imports Loki query JSON and Tempo trace JSON as local records with trace references |
| T-497 | P2 | [x] | Add Grafana dashboard JSON ingestion that lets Evidence Board cards reference dashboard panels without treating dashboards as raw metrics evidence. | T-468, T-471 | Completed 2026-05-16: Grafana dashboard JSON panels import as reference records and findings rather than metric samples |
| T-498 | P1 | [x] | Map OTel logs, metrics snapshots, Loki, Tempo, and Grafana references into Incident Timeline, SLO/Golden Signals, Evidence Board, and report packs. | T-494, T-495, T-496, T-497 | Completed 2026-05-16: observability findings flow through Evidence Board/report packs, observability records feed Incident Timeline, and metrics/observability summaries feed Golden Signals |
| T-499 | P1 | [ ] | Define the slow-query and database engine-log `AnalysisResult` contract with query fingerprint, latency percentiles, lock wait, error, row count, database, schema, and sanitized SQL fields. | T-468, T-471, T-472 |  |
| T-500 | P1 | [ ] | Add PostgreSQL csvlog and text log parsers/analyzers for slow queries, errors, lock waits, checkpoints, autovacuum signals, and top query fingerprints. | T-499, T-470 |  |
| T-501 | P1 | [ ] | Add MySQL/MariaDB slow query log parsing and analysis for query fingerprints, p95/p99 latency, lock time, rows examined, and top query findings. | T-499, T-470 |  |
| T-502 | P2 | [ ] | Add MongoDB profiler and diagnostic.data export parsing for slow operations, collection/index hints, lock/queue indicators, and error counts. | T-499, T-470 |  |
| T-503 | P2 | [ ] | Add Redis slowlog get output parsing and analysis for command fingerprints, latency, key-pattern sanitization, and top slow commands. | T-499, T-470 |  |
| T-504 | P2 | [ ] | Add SQL Server extended events JSON parsing for slow query, wait, deadlock, and error evidence. | T-499, T-470 |  |
| T-505 | P1 | [ ] | Add PostgreSQL and MySQL EXPLAIN plan importers and Evidence Board plan cards linked to slow-query fingerprints. | T-499, T-500, T-501 |  |
| T-506 | P1 | [ ] | Integrate database slow-query evidence into Incident Timeline, SLO/Golden Signals, Service Flow enrichment, and report packs. | T-500, T-501, T-505 |  |
| T-507 | P1 | [ ] | Define the broker/streaming middleware `AnalysisResult` contract and finding taxonomy for rebalance, replication, queue pressure, dead letter, partition, and broker health events. | T-468, T-471, T-472 |  |
| T-508 | P1 | [ ] | Add Kafka broker, controller, and state-change log parsing with ISR change, rebalance, under-replicated partition, log compaction, and KRaft quorum findings. | T-507, T-470 |  |
| T-509 | P1 | [ ] | Add RabbitMQ server-log parsing plus `rabbitmq-diagnostics` JSON import for connection churn, queue length, dead-letter, partition, and node health findings. | T-507, T-470 |  |
| T-510 | P2 | [ ] | Add Pulsar broker-log parsing for topic, ledger, bookie, broker health, and backlog-related findings. | T-507, T-470 |  |
| T-511 | P2 | [ ] | Add NATS server-log parsing for connection churn, slow consumers, cluster events, JetStream pressure, and authorization failures. | T-507, T-470 |  |
| T-512 | P2 | [ ] | Add ActiveMQ broker-log parsing for queue pressure, consumer churn, dead letter, store usage, and broker health events. | T-507, T-470 |  |
| T-513 | P1 | [ ] | Surface broker findings in Incident Timeline and Service Flow beside trace-import dependencies and service-edge evidence. | T-508, T-509, T-472 |  |
| T-514 | P2 | [ ] | Define the Kubernetes/container evidence contract and identity model for cluster, namespace, workload, pod, container, node, image, restart count, and owner references. | T-468, T-471, T-472 |  |
| T-515 | P2 | [ ] | Add `kubectl get events` and `describe pod` JSON importers with OOMKilled, restart, eviction, scheduling, image pull, and readiness findings. | T-514, T-470 |  |
| T-516 | P2 | [ ] | Add kubelet log parsing for pod lifecycle, eviction, probe, image, runtime, and node-pressure events. | T-514, T-470 |  |
| T-517 | P2 | [ ] | Add container runtime log parsing for containerd, CRI-O, and Docker daemon events that accompany application evidence. | T-514, T-470 |  |
| T-518 | P2 | [ ] | Add cloud audit/log importers for AWS CloudTrail JSON, GCP Cloud Audit Logging JSON, and Azure Activity Logs JSON with security-incident evidence fields. | T-468, T-470, T-471 |  |
| T-519 | P2 | [ ] | Map Kubernetes, container, and cloud audit evidence into Incident Timeline, Evidence Board, SLO saturation signals, and security-oriented report sections. | T-515, T-516, T-517, T-518 |  |
| T-520 | P1 | [ ] | Define a unified profile evidence schema with language-tagged frames, native-versus-managed split, async frame markers, source runtime, and flamegraph rollup metadata. | T-468, existing profiler analyzers |  |
| T-521 | P1 | [ ] | Add a generic pprof binary `.pb.gz` importer shared by Go pprof, Datadog Ruby/PHP profiler exports, py-spy pprof-compatible output, and other pprof runtimes. | T-520, T-470 |  |
| T-522 | P2 | [ ] | Add py-spy raw output and rbspy raw output importers with Python/Ruby frame tagging and thread/process metadata. | T-520, T-470 |  |
| T-523 | P1 | [ ] | Add async-profiler `.html` and collapsed input parity for the unified profile path while preserving existing collapsed/JFR behavior. | T-520, existing profiler analyzers |  |
| T-524 | P2 | [ ] | Add dotnet-trace `.nettrace` and speedscope output importers with .NET frame tagging and managed/native separation. | T-520, T-470 |  |
| T-525 | P2 | [ ] | Add perf script collapsed import for Rust/native profiling evidence with symbol, thread, and process metadata. | T-520, T-470 |  |
| T-526 | P2 | [ ] | Add Ruby stackprof plus PHP Excimer, Tideways CLI export, and Xdebug profile importers where stable file contracts are available. | T-520, T-470 |  |
| T-527 | P3 | [ ] | Add Swift backtrace and generic async stack trace parsing after the primary server-side runtime profiler inputs stabilize. | T-520, T-470 |  |
| T-528 | P1 | [ ] | Promote collapsed and JFR profiler outputs into the unified multi-language profile analyzer with cross-language flamegraph rollups and Evidence Board capture. | T-521, T-523, T-520 |  |
| T-529 | P1 | [ ] | Implement the first evidence-stitching pass that joins access logs, traces, runtime stacks, broker logs, and database slow logs by correlation key. | T-472, T-484, Trace import MVP, T-506, T-513 |  |
| T-530 | P1 | [ ] | Surface correlation-gap findings for missing trace ID, dropped parent span, unmatched request log, unmatched database call, and unmatched broker event. | T-529 |  |
| T-531 | P2 | [ ] | Add stitched evidence views in Incident Timeline, Evidence Board, Service Flow, and report packs with raw source evidence preserved. | T-529, T-530 |  |
| T-532 | P2 | [ ] | Add Grafana Pyroscope/Phlare snapshot export importer and map it into the unified profile evidence schema. | T-520, T-470 |  |
| T-533 | P2 | [ ] | Add Polar Signals Parca snapshot export importer and map it into the unified profile evidence schema. | T-520, T-470 |  |
| T-534 | P2 | [ ] | Route continuous-profiling snapshots through the existing flamegraph analyzer, Evidence Board capture, and report-pack export paths. | T-532, T-533, T-528 |  |
| T-535 | P3 | [ ] | Track the OpenTelemetry Profiles signal lifecycle and add a decision note for when OTLP Profiles should move from long-term radar to active ingestion work. | T-520 |  |
| T-536 | P2 | [ ] | Keep English/Korean roadmap, data-model notes, examples, and importer support matrices current as Mid-Term Plus tasks are implemented. | T-468 through T-535 |  |

## Verification Notes

- `go test ./...` passed under `apps/engine-native`. The Wails app test build
  still emits the known macOS linker warning about object files built for newer
  macOS 26.0 than the 11.0 link target.
- `go test -bench BenchmarkBuildLargeSyntheticGCLog -benchtime=1x -benchmem
  ./internal/analyzers/gclog` passed with a 300k-event JSON payload of about
  1.9 MB.
- Synthetic large-file measurements were captured with the current
  `cmd/archscope-engine` binary built from `apps/engine-native`.
- 2026-05-10 release verification ran with local `go1.26.3`. `go vet ./...`,
  `go build ./cmd/archscope-engine ./cmd/archscope-app`, and
  `go test ./... -race -count=1` passed
  under `apps/engine-native` using `/tmp` Go caches. The race test required
  loopback permission for `httptest`.
- `npm ci` and `npm run build` passed for the Wails frontend during the
  2026-05-10 release pass. That pass exposed the former main-chunk warning
  that was later addressed by T-419.
- `task package` and `task darwin:package:dmg ARCH=arm64` produced
  `bin/archscope.app` and `bin/archscope-arm64.dmg`. The app launched without
  immediate crash, ad-hoc `codesign --verify --deep --strict` passed,
  `CFBundleShortVersionString`/`CFBundleVersion` are `0.3.0-rc1`, and
  `hdiutil verify` reported a valid DMG checksum.
- CLI smoke tests produced access-log and profiler `AnalysisResult` JSON plus
  HTML/CSV report outputs in `/tmp`.
- GitHub Actions run `25602445878` completed the `v0.3.0-rc1` Release workflow
  successfully across darwin-arm64, windows-amd64, and linux-amd64 packaging
  plus GitHub Release creation. Latest main CI runs `25603594053` and
  `25603594056` were also successful, including Windows `go vet`, `go build`,
  and `go test`.
- Downloaded release assets matched `SHA256SUMS`; Windows zip contains an
  x86-64 `archscope.exe`, while the NSIS installer wrapper is a 32-bit PE
  self-extractor as expected.
- 2026-05-10 UI/performance pass: `npm run build` passed for the Wails
  frontend and reported the former >500 KB main chunk warning that was later
  addressed by T-419.
- `go test ./internal/parsers/accesslog ./internal/profiler` and
  `go test ./...` passed under `apps/engine-native` using `/tmp` Go caches.
- Parser/profiler benchmarks after the hot-path changes:
  `BenchmarkParseLineNginxWithResponseTime` 165.4 ns/op, 144 B/op, 1 alloc/op;
  `BenchmarkAnalyzeCollapsedSampleWall` 86.7 ms/op, 53.6 KB/op, 549 allocs/op;
  `BenchmarkAnalyzeJenniferSample` 41.5 us/op, 41.3 KB/op, 422 allocs/op.
- 2026-05-10 naming cleanup: `go test ./...`, `go build
  ./cmd/archscope-engine ./cmd/archscope-app`, `npm run build`, `task
  package`, `task darwin:package:dmg ARCH=arm64`, `codesign --verify --deep
  --strict bin/archscope.app`, and `hdiutil verify bin/archscope-arm64.dmg`
  passed from the renamed `cmd/archscope-app` tree.
- 2026-05-10 `v0.3.1-rc1` prerelease was created from the Jennifer MSA
  network-time grouping work. GitHub Release workflow completed successfully
  across darwin-arm64, windows-amd64, linux-amd64, and release creation.
- 2026-05-11 `v0.3.1` is listed as the latest GitHub Release. It promotes the
  0.3.1 line to stable and includes the trace import MVP plus the Jennifer MSA
  network-time improvements carried forward from `v0.3.1-rc1`.
- 2026-05-11 local trace import verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test
  ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine`.
- 2026-05-11 documentation consolidation check passed: `git diff --check`
  succeeded, and `docs/en` and `docs/ko` Markdown file lists match 1:1 after
  removing the former single-language planning notes.
- 2026-05-13 GC alert verification passed in
  `apps/engine-native/internal/analyzers/gclog`: young/old/metaspace series,
  long-pause flags, and `OUT_OF_MEMORY_ERROR` findings are covered by tests.
- 2026-05-13 trace import verification passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine ./api ./cmd/archscope-app`. The Wails app test build
  still emits the known macOS linker warning.
- 2026-05-13 full Go verification also passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./...`.
- 2026-05-13 Wails frontend release-hardening build passed: `npm run build`.
  Startup shell chunk is 156.77 KB raw / 50.90 KB gzip; the lazy shared
  ECharts runtime is 668.49 KB raw / 221.26 KB gzip and stays under the
  documented 700 KB chart-runtime budget. The previous Vite large-chunk warning
  is no longer emitted.
- 2026-05-13 release workflow hardening updated macOS signing secret preflight
  and codesign/stapler validation. The workflow change was not exercised by a
  local release-tag run. Local YAML parsing and `git diff --check` passed.
- 2026-05-13 CLI smoke passed for Elastic APM auto-detect:
  `go run ./cmd/archscope-engine trace import --in
  ../../examples/traces/sample-elastic-apm-search.json --format auto --top-n
  10` emitted `source_format = elastic-apm-search-json` and the expected
  trace findings.
- 2026-05-13 Windows GUI smoke run `25800619784` passed on Microsoft Windows
  Server 2025. It built `apps/engine-native/cmd/archscope-app/bin/archscope.exe`
  with `task windows:build ARCH=amd64`, confirmed a 14,060,544-byte executable,
  launched it, observed the GUI process alive for 15 seconds, and requested
  graceful shutdown.
- 2026-05-13 JFR async-profiler verification passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./internal/analyzers/jfr ./internal/parsers/jfr ./cmd/archscope-app`.
  The Wails app test build still emits the known macOS linker warning.
- 2026-05-13 Wails frontend verification passed after JFR/Evidence Board
  expansion: `npm run build`. Startup shell chunk is 156.78 KB raw / 50.91 KB
  gzip; the lazy shared ECharts runtime remains 668.49 KB raw / 221.26 KB gzip.
- 2026-05-13 `git diff --check` passed after the T-420 through T-423 changes.
- 2026-05-13 Wails frontend verification passed after T-424 through T-427:
  `npm run build`. Startup shell chunk is 159.76 KB raw / 51.58 KB gzip; the
  lazy shared ECharts runtime is 689.89 KB raw / 229.49 KB gzip and remains
  under the documented 700 KB chart-runtime budget.
- 2026-05-15 trace compatibility verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test
  ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine`.
- 2026-05-15 full Go verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...`. The Wails app test
  build still emits the known macOS linker warning.
- 2026-05-15 Wails frontend verification passed after adding Jaeger and
  SkyWalking format options: `npm run build`. Startup shell chunk is
  159.75 KB raw / 51.58 KB gzip; the lazy shared ECharts runtime is
  689.89 KB raw / 229.49 KB gzip and remains under the documented 700 KB
  chart-runtime budget.
- 2026-05-15 CLI smoke passed for Jaeger and SkyWalking auto-detect:
  `go run ./cmd/archscope-engine trace import --format auto` emitted
  `source_format = jaeger-query-json` with three spans and
  `source_format = skywalking-graphql-json` with two spans.
- 2026-05-15 Wails frontend verification passed after T-431 through T-433:
  `npm run build`. Startup shell chunk is 160.48 KB raw / 51.79 KB gzip; the
  lazy shared ECharts runtime is 689.89 KB raw / 229.50 KB gzip and remains
  under the documented 700 KB chart-runtime budget.
- 2026-05-16 Wails frontend verification passed after T-437 through T-440:
  `npm run build`. Startup shell chunk is 161.04 KB raw / 51.95 KB gzip; the
  SLO / Golden Signals page is a lazy 41.99 KB raw / 9.17 KB gzip route chunk,
  and the lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
  `git diff --check` passed.
- 2026-05-16 Wails frontend verification passed after T-441 through T-444:
  `npm run build`. Startup shell chunk is 161.40 KB raw / 52.02 KB gzip; the
  Service Flow page is a lazy 22.30 KB raw / 5.84 KB gzip route chunk, and the
  lazy shared ECharts runtime remains 689.89 KB raw / 229.49 KB gzip.
- 2026-05-16 Wails frontend verification passed after T-447:
  `npm run build`. Startup shell chunk is 161.45 KB raw / 52.07 KB gzip; the
  Evidence Board page is a lazy 17.92 KB raw / 5.63 KB gzip route chunk, and
  the lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
- 2026-05-16 Wails frontend verification passed after T-448:
  `npm run build`. Startup shell chunk is 161.45 KB raw / 52.07 KB gzip; the
  Analysis Workspace page is a lazy 6.14 KB raw / 2.21 KB gzip route chunk,
  and the lazy shared ECharts runtime remains 689.89 KB raw / 229.49 KB gzip.
- 2026-05-16 Wails frontend verification passed after T-449:
  `npm run build`. Startup shell chunk is 161.45 KB raw / 52.06 KB gzip; the
  Analysis Workspace page is a lazy 10.15 KB raw / 3.15 KB gzip route chunk,
  and the lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
- 2026-05-16 AI interpretation gate verification passed after T-450:
  `env GOCACHE=/tmp/archscope-go-cache go test ./internal/aiinterpretation`
  passed under `apps/engine-native`; loopback permission was required for the
  existing Ollama `httptest` client case. Wails frontend `npm run build` also
  passed. Startup shell chunk is 161.45 KB raw / 52.05 KB gzip; the Analysis
  Workspace page is a lazy 13.42 KB raw / 4.20 KB gzip route chunk, and the
  lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
- 2026-05-16 AI report integration verification passed after T-451:
  `env GOCACHE=/tmp/archscope-go-cache go test ./internal/aiinterpretation`
  passed under `apps/engine-native`, and Wails frontend `npm run build` passed.
  Startup shell chunk is 161.50 KB raw / 52.08 KB gzip; the Analysis Workspace
  page is a lazy 9.22 KB raw / 2.78 KB gzip route chunk, Evidence Board is a
  lazy 19.97 KB raw / 6.11 KB gzip route chunk, and the shared
  `aiInterpretation` helper is a lazy 5.15 KB raw / 1.88 KB gzip chunk.
- 2026-05-16 Wails frontend verification passed after T-435:
  `npm run build`. Startup shell chunk is 161.50 KB raw / 52.08 KB gzip; the
  Incident Timeline page is a lazy 9.97 KB raw / 2.62 KB gzip route chunk, the
  shared `incidentTimeline` helper is 11.38 KB raw / 3.79 KB gzip, and
  `git diff --check` passed.
- 2026-05-16 Wails frontend verification passed after T-436:
  `npm run build`. Startup shell chunk is 161.50 KB raw / 52.08 KB gzip; the
  Incident Timeline page is a lazy 11.18 KB raw / 2.80 KB gzip route chunk,
  the shared `incidentTimeline` helper is 12.70 KB raw / 4.20 KB gzip, and
  `git diff --check` passed.
- 2026-05-16 `v0.3.3` release-prep verification passed: Wails build assets
  were regenerated from `build/config.yml`, Windows `syso` generation passed
  for the 0.3.3 metadata, `env GOCACHE=/tmp/archscope-go-cache go test ./...`
  passed under `apps/engine-native`, `npm run build` passed for the Wails
  frontend, and `git diff --check` passed. The Wails app test build still
  emits the known macOS linker warning. Startup shell chunk is 161.50 KB raw /
  52.08 KB gzip, and the lazy shared ECharts runtime is 689.89 KB raw /
  229.49 KB gzip.
- 2026-05-16 `v0.3.4` release-prep verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...` passed under
  `apps/engine-native` with the known macOS linker warning,
  `npm run test:state` passed for derived frontend state regressions,
  `npm run build` passed for the Wails frontend, and `git diff --check`
  passed. Frontend package/build metadata is set to 0.3.4.
- 2026-05-16 T-468 through T-472 verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...` passed under
  `apps/engine-native` with the known macOS linker warning, and
  `git diff --check` passed.

## Decisions

- Go/Wails is the primary active runtime and distribution path.
- Wails service bindings are the active engine/UI boundary.
- Local HTTP/FastAPI/browser serving is retired and retained only in `archive/`.
- Python sources are retained for behavior reference and audits only.
- Large-file safety work should happen in Go first; archived Python code should
  not receive new product features.
- Roadmap candidates are not automatically promoted into `Active TO-DO`; they
  should be added only after explicit review and request.

## Archive

- Old work status: `archive/work_status_legacy_2026-05-09.md`
- Archived Python engine: `archive/python-engine`
- Archived browser frontend: `archive/web-frontend-python`
