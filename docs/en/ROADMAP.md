# Roadmap

Last updated: 2026-05-14

This document is the consolidated product roadmap for the active Go/Wails
ArchScope line. It pulls together the previous phase roadmap, the product
expansion notes, the APM import matrix, and the AI interpretation design into a
single planning view.

Use this file for product direction. Use the root `work_status.md` for the
current execution queue and verification status. This roadmap supersedes the
former single-language product expansion notes, product expansion TODO, and APM
import matrix documents.

## Roadmap Inputs

- `work_status.md` - active execution status and current TO-DO IDs.
- `docs/en/AI_INTERPRETATION.md` and `docs/ko/AI_INTERPRETATION.md` -
  evidence-bound AI interpretation policy and runtime guardrails.
- `docs/en/REPORT_EXPORT_DESIGN.md` and `docs/ko/REPORT_EXPORT_DESIGN.md` -
  report export and before/after design context.

## Product Direction

ArchScope is moving from a collection of local diagnostic analyzers into an
Evidence Studio for architecture, operations, and customer reporting. The core
product promise is local-first evidence reanalysis: users can import files or
exports from real systems, inspect deterministic findings, collect supporting
evidence, and produce reports without sending customer data to a SaaS service.

Guiding principles:

- Prefer local files and standard/open formats before SaaS-specific API
  connectors.
- Keep parser, analyzer, exporter, and UI responsibilities separate.
- Preserve the shared `AnalysisResult` contract as the deterministic source of
  truth.
- Treat AI interpretation as optional, evidence-bound, and separate from
  deterministic analyzer output.
- Keep product documentation available in both English and Korean when a topic
  becomes part of the canonical roadmap.

## Current Baseline

- Active product: Go/Wails desktop app under `apps/engine-native`.
- Active UI: Wails v3 React frontend under
  `apps/engine-native/cmd/archscope-app/frontend`.
- Active engine: Go parser, analyzer, exporter, profiler, and AI
  interpretation modules under `apps/engine-native/internal`.
- Release baseline: `v0.3.2` stable.
- Archived implementation: Python/FastAPI/browser sources are kept under
  `archive/` for historical reference only.

## Delivered Foundation

### Foundation and Charts

- Repository, desktop UI, parser, exporter, and JSON result foundations.
- Access log and collapsed profiler MVPs.
- Parser diagnostics, encoding fallback, and type-specific `AnalysisResult`
  contracts.
- Chart Studio, chart templates, PNG/SVG/CSV export, report label language
  toggle, and report-ready chart improvements.

### JVM and Runtime Diagnostics

- GC log, Java thread dump, Java exception, and JFR parser/analyzer paths.
- Timeline correlation design across access logs, GC, profiler, thread, JFR,
  OTel, and runtime evidence.
- Multi-language stack and dump parsing for Java, Go, Python, Node.js, and .NET
  sources.
- Thread dump hardening with carrier-pinning, SMR/zombie-thread, lock
  contention, and deadlock findings.
- JFR analyzer contract clarified: the JDK `jfr` CLI is the `.jfr` to
  `jfr print --json` conversion boundary, while ArchScope builds
  report-oriented event summaries, async-profiler stack evidence, and UX hints.

### Go/Wails Consolidation

- Active product line consolidated into the Go/Wails desktop implementation.
- Python and browser implementations archived instead of receiving new product
  features.
- Wails service bindings selected as the active engine/UI boundary.
- Large-file guardrails added for access log, OTel JSONL, GC log, JFR JSON,
  thread dump, Jennifer profile, and profiler HTML/SVG paths.
- Windows GUI smoke workflow added for the Go/Wails desktop app: it builds the
  Windows executable on a GitHub-hosted Windows runner, launches
  `archscope.exe`, verifies that the GUI process stays alive, and shuts it down
  cleanly.
- Release workflow hardening added macOS signing/notarization secret preflight,
  post-build codesign/stapler validation, route-level frontend lazy loading,
  modular ECharts bundling, and explicit startup/chart-runtime chunk budgets.

### Profiler, Access Log, GC, and MSA Improvements

- JFR-first profiler workflows, differential flame views, heatmap selection,
  per-thread isolation, pprof export, tree view, native-memory leak findings,
  and recent-files workflow.
- Access log overhaul with URL statistics, static/API classification,
  percentile timelines, throughput, status distribution, and findings such as
  `SLOW_URL_P95` and `ERROR_BURST_DETECTED`.
- GC log deep-dive with JVM info cards, worker/CPU mismatch warnings,
  toggleable heap series, pause overlay, rectangle zoom, and point
  decimation. Young/old/metaspace series, OOM alerts, and long-pause event
  findings are covered in the current Go analyzer.
- Async-profiler-oriented JFR stack aggregation from `stackTrace.frames` into
  top methods, packages, threads, sample stacks, and a flamegraph-ready call
  tree.
- Jennifer MSA service-call network-time summaries and topology placement so
  internal single-digit millisecond calls and gateway/external double-digit
  millisecond calls separate visually.

### Trace Import MVP

- Canonical trace/span model for external trace imports.
- OTLP JSON-file parser, Zipkin v2 JSON parser, Elastic APM Elasticsearch
  `_search` response parser, Elastic APM source-only NDJSON parser, Jaeger
  QueryService/local trace JSON parser, and schema-guarded SkyWalking GraphQL
  `queryTrace.spans` parser diagnostics.
- `trace_import` analyzer result with summary, services, traces, spans,
  dependencies, service summaries, critical paths, and deterministic findings.
- Wails Trace Import page with summary cards, service dependency and service
  latency charts, trace/span tables, critical path rows, parser diagnostics,
  and finding capture into the Evidence Board.
- CLI command:
  `archscope-engine trace import --in <file> --format auto|otlp-json|zipkin-v2-json|elastic-apm-search-json|elastic-apm-source-ndjson|jaeger-query-json|skywalking-graphql-json`.
- Sample trace fixtures under `examples/traces`.

### Evidence Board Skeleton

- Reusable local evidence-card model for analyzer findings, chart selections,
  table rows, parser diagnostics, source metadata, comments, hypotheses,
  impact, and recommendations.
- Initial desktop Evidence Board page backed by local browser storage.
- Trace Import can add findings, service edges, traces, and source metadata to
  the Evidence Board.
- Access Log, GC Log, JFR, and Native Memory views can add selected findings
  and table rows to the Evidence Board.
- Evidence Board can export local static HTML evidence reports and JSON
  evidence packs from saved cards.

### Incident Timeline MVP

- Common session timeline event model with timestamp, source analyzer,
  severity, category, label, evidence reference, and source metadata.
- Wails Incident Timeline page under Workspace using Analysis Workspace results
  as input and Evidence Board cards as output.
- Event mapping covers deterministic findings plus access-log error/latency
  series, GC alerts, JFR pause/notable events, exception rows, thread-dump
  contention/deadlock tables, and trace-import errors/critical paths.

### Evidence-Bound AI Interpretation

- Go implementation under `apps/engine-native/internal/aiinterpretation`.
- Evidence registry, evidence selector, prompt builder, Ollama client, and AI
  finding validator.
- Contract that keeps `InterpretationResult` separate from `AnalysisResult`.
- Local-only default provider policy, prompt-injection defense, validation,
  low-confidence filtering, and evaluation requirements.

## Near-Term Roadmap: 0.3.x Stabilization

These items should stay aligned with `work_status.md`.

1. Keep release verification healthy.
   - Repeat the Windows GUI smoke before release cuts and add manual
     customer-environment smoke where available.
   - Keep macOS signing/notarization credentials complete and validated.
   - Track frontend startup and lazy chart-runtime chunks against the documented
     budgets.
   - Revisit deeper GC event streaming if real-world inputs exceed the current
     bounded-memory envelope.

2. Stabilize JFR and Evidence Board expansion.
   - Validate async-profiler JFR stack aggregation against real CPU, wall,
     allocation, and lock recordings.
   - Extend "Add to Evidence" coverage to remaining analyzers where it is useful.
   - Move Evidence Board export from UI-level HTML/JSON toward the planned
     engine-level report pack workflow.
   - Connect AI interpretation to the board only after evidence-reference
     integrity checks pass.

3. Promote the next Evidence Studio batch after the Incident Timeline MVP.
   - Candidate tracks are SLO/Golden Signals and unified Service Flow.
   - Keep Jaeger and SkyWalking compatibility fixtures representative as real
     customer exports become available.

## Mid-Term Roadmap: Evidence Studio

### Incident Timeline

- Promote the Wails session timeline into an engine-level or exportable
  `AnalysisResult` when report-pack generation needs persisted timeline data.
- Add richer event range handling, correlation IDs, and timeline grouping for
  multi-file incidents.
- Use the timeline to explain what happened and in what order during an
  incident.

### SLO and Golden Signals

- Build a golden-signals inventory across access logs, trace import, Jennifer
  MSA, exceptions, GC, JFR, thread dumps, and JVM signals.
- Define SLI metrics for latency, traffic, errors, and saturation.
- Add SLO target configuration, violating-window detection, error-budget burn
  tables, and affected service/endpoint breakdowns.

### Service Flow and MSA Topology

- Unify Jennifer MSA topology and trace-import dependency models.
- Define a common service-edge schema with caller, callee, call count,
  average/max/total latency, error count, and network gap.
- Normalize unmatched calls, missing parents, and network gaps into service-edge
  findings.
- Add C4 dynamic view or sequence-like export for service-flow evidence.

### Reports and Evidence Packs

- Generate report-ready HTML, ZIP, and eventually PPTX/PDF outputs from
  Evidence Board content.
- Preserve source metadata, analyzer options, captured evidence, deterministic
  findings, and optional AI interpretation provenance.
- Support customer-facing summaries without hiding the raw evidence behind
  conclusions.

### AI Interpretation Productization

- Surface AI interpretation provenance in the UI.
- Keep AI findings visually separate from deterministic findings.
- Add evaluation gates using golden diagnostics, evidence-reference integrity,
  quote-to-source matching, low-confidence filtering, and hallucination review.
- Connect AI interpretation to Evidence Board and report generation only when
  every generated claim has valid evidence references.

## Later Roadmap: Architecture and Operations Expansion

### API and Event Contract Analysis

- Import OpenAPI specifications and compare them with access-log evidence.
- Detect undocumented APIs, unused APIs, slow APIs, high-error APIs, and
  contract drift.
- Import AsyncAPI definitions and analyze Kafka, RabbitMQ, WebSocket, or other
  producer/consumer flows where evidence is available.

### Architecture Documentation Pack

- Generate draft inputs for arc42 sections such as Context, Runtime View,
  Deployment View, Quality Requirements, and Risks.
- Generate ADR drafts with decision, context, alternatives, tradeoffs,
  consequences, and evidence references.
- Connect C4 diagrams, service-flow evidence, incident timelines, and Evidence
  Board cards into architecture-review deliverables.

### Security and Compliance Evidence

- Detect potential sensitive-data exposure in logs.
- Inventory access/error/log patterns from an OWASP Top 10 perspective.
- Investigate SBOM/CycloneDX import feasibility.
- Design vulnerability, license, and affected-service impact maps.
- Add threat-model, security logging, and redaction evidence only when the
  related source evidence is available.

### Before/After Multi-Signal Compare

- Extend comparison beyond profiler outputs.
- Compare access-log latency/error/traffic, GC pause and heap behavior, JFR
  signals, MSA external call latency, thread blocking, and profiler hotspots
  before and after tuning or deployment.

### Product Navigation

- Move toward a workflow-oriented navigation model:
  Workspace, Diagnostics, Service Flow, Architecture, Operations, Security,
  and Settings.
- Consider renaming `MSA Timeline` to `MSA / Service Flow` and `Compare` to
  `Before / After`.
- Keep parser reports near their analyzers while allowing important parser
  evidence to be promoted into the Evidence Board.

## External APM Import Roadmap

### File-First Priority

1. OpenTelemetry OTLP JSON file - completed for trace import MVP.
2. Zipkin v2 JSON - completed for trace import MVP.
3. Elastic APM Elasticsearch `_search` response and source-only NDJSON -
   completed for trace import MVP.
4. Jaeger compatibility import - completed with the stable QueryService/local
   trace JSON contract.
5. Apache SkyWalking GraphQL response import - schema-guarded
   `queryTrace.spans` support is in place; continue validating additional
   SkyWalking response versions before widening the importer.

### SaaS and Product-Specific Connectors

These remain later-stage items until file import contracts and token/security
policies are stable:

- New Relic NerdGraph trace detail and Historical Data Export JSON.
- Datadog Spans API and Continuous Profiler/profile export feasibility.
- Dynatrace distributed trace CSV/table download and DQL/API export.
- Splunk/Cisco AppDynamics transaction snapshot import.
- Pinpoint import only after a stable official export/API path is confirmed.

Deferral rationale:

- Authentication and authorization models differ by product.
- Retention windows, indexed-span sampling, and rate limits can reduce evidence
  completeness.
- SaaS API tokens require explicit local storage, redaction, and compliance
  policies.
- The local file-import UX must stabilize before connector inputs become part
  of the public contract.

## Documentation Roadmap

- Keep this roadmap mirrored in English and Korean.
- Keep product expansion, external APM import, and Evidence Studio planning in
  this roadmap instead of separate single-language notes.
- Keep detailed design documents under `docs/en` and `docs/ko`.
- Keep `work_status.md` focused on active execution status rather than the full
  product roadmap.

## Deferred or Out of Scope

- Heap dump / `.hprof` analysis remains explicitly out of scope.
- Direct SaaS connectors are deferred until local file imports, evidence
  contracts, and token policies are stable.
- Archived Python/FastAPI/browser sources should not receive new product
  features.
