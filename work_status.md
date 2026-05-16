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
- Release baseline: `v0.3.3` is the latest stable GitHub release. The
  `v0.3.1-rc1` prerelease remains available as the Jennifer MSA network-time
  release candidate.
- Current execution focus: preparing a `v0.3.4` stabilization pass from the
  2026-05-16 mid-term roadmap review, with priority on P0 correctness,
  privacy, SLO accuracy, report-pack safety, and AnalysisResult guardrails.
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

1. Execute the accepted P0 findings from the 2026-05-16 mid-term roadmap
   review before any external demo: Incident Timeline mapping, AI gate
   enforcement, privacy redaction, SLO unit accuracy, signal deduplication,
   and report-pack ZIP safety.
2. Restore the architecture guardrails incrementally by moving the simplest
   derived workflow, Service Flow, toward a Go analyzer/exportable
   `AnalysisResult` pattern first, then use that pattern for Incident Timeline
   and SLO projections.
3. Add deterministic export/test coverage around the 0.3.4 stabilization work
   so timeline, SLO, service-flow, report-pack, and AI-gate regressions are
   caught before the next release.
4. Keep release verification healthy before the next 0.3.x cut by repeating
   Windows GUI smoke, macOS signing/notarization validation, and frontend
   bundle budget checks.
5. Keep Jaeger and SkyWalking compatibility fixtures representative as real
   customer exports become available.

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
| T-457 | P0 | [ ] | Sanitize report-pack ZIP entry paths with a shared path-normalization helper that rejects absolute, parent-directory, and platform-specific traversal forms before writing archive headers. | T-445 | Review accepted 2026-05-16: close report-pack ZIP traversal surface |
| T-458 | P1 | [ ] | Move Service Flow toward the first Go analyzer/exportable `AnalysisResult` pattern for derived Evidence Studio workflows, including parser/findings/diagnostics metadata, CLI/exporter compatibility, service-edge normalization, and fixtures. | T-441, T-442, T-443, T-444 | Review accepted 2026-05-16: establish guardrail-restoration pattern with the simplest derived workflow |
| T-459 | P1 | [ ] | Apply the proven derived-analysis contract pattern to Incident Timeline and SLO/Golden Signals projections so exported payloads satisfy the common `AnalysisResult` envelope and remain readable by Go exporters. | T-458, T-452, T-455 | Review accepted 2026-05-16: reduce Wails-only projection drift |
| T-460 | P1 | [ ] | Stop report-pack export from recomputing analysis at export time; report packs should consume persisted Analysis Workspace results and derived `AnalysisResult` artifacts produced earlier in the workflow. | T-458, T-459, T-445 | Review accepted 2026-05-16: keep exporters from mutating or reanalyzing evidence |
| T-461 | P1 | [ ] | Add service identity normalization for Service Flow and SLO aggregation, including case, hyphen/underscore, display-name, and alias-table matching for Trace Import and Jennifer MSA service names. | T-441, T-458 | Review accepted 2026-05-16: make T-441/T-442 unification effective in real exports |
| T-462 | P1 | [ ] | Strengthen AI provenance in Evidence Board and report packs with prompt hash, generation parameters, token counts when available, source/evidence integrity hashes, and validation status. | T-453, T-446 | Review accepted 2026-05-16: make AI provenance auditable instead of descriptive only |
| T-463 | P1 | [ ] | Preserve AI-assisted versus deterministic finding separation after Evidence Board capture and in report-pack rendering, including visible AI badges, confidence, limitations, and evidence-reference status. | T-449, T-451 | Review accepted 2026-05-16: keep AI/deterministic distinction across all report surfaces |
| T-464 | P2 | [ ] | Make derived workflow exports deterministic by injecting `now` explicitly, supporting fixed ZIP timestamps, parsing Unix-second timestamps, replacing locale-dependent formatting, stabilizing dedupe keys, and optimizing narrative sorting with precomputed event maps. | T-457, T-459 | Review accepted 2026-05-16: enable byte-stable report diffs where inputs match |
| T-465 | P2 | [ ] | Expand SLO and runtime signal robustness with configurable SLO target overrides, safe non-spread max scans for large series, and runtime-stack signal extraction for Node.js, Python, Go panic, and .NET analyzer result types. | T-455 | Review accepted 2026-05-16: complete the missing configuration/runtime-signal paths |
| T-466 | P2 | [ ] | Add frontend state regression tests for Incident Timeline, SLO/Golden Signals, Service Flow, Report Pack, and AI gate behavior, plus Go AI redaction/evidence-reference tests for the new hardening paths. | T-452, T-453, T-454, T-455, T-456, T-457 | Review accepted 2026-05-16: prevent repeat silent-break regressions |
| T-467 | P2 | [ ] | Deduplicate shared frontend state utilities and fix Mermaid sequence export edge cases, including empty-service/source-only findings and quoted labels. | T-458, T-461 | Review accepted 2026-05-16: reduce maintenance drift and malformed diagram output |

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
