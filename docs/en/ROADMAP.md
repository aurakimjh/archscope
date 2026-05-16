# Roadmap

Last updated: 2026-05-16

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

### SLO and Golden Signals MVP

- Golden Signals inventory across access logs, trace import, Jennifer MSA,
  exceptions, GC, JFR, thread dumps, and JVM metadata/runtime signals.
- Normalized SLI metric model for latency, traffic, errors, and saturation.
- Session-window SLO target evaluation with default latency, error-rate, GC
  pause/throughput, OOM, deadlock, trace-integrity, and MSA network-gap targets.
- Wails SLO / Golden Signals page under Workspace with signal inventory, SLI
  metrics, SLO violations, affected-scope breakdowns, error-budget burn rows,
  and Evidence Board capture for SLO violations.

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

3. Promote the next Evidence Studio batch after the Incident Timeline and
   SLO/Golden Signals MVPs.
   - The next candidate track is unified Service Flow, followed by report packs
     and evidence-gated AI interpretation productization.
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

- Promote the Wails session SLO / Golden Signals projection into an
  engine-level or exportable `AnalysisResult` when report packs need persisted
  SLO data.
- Add user-editable SLO target presets and per-customer threshold profiles.
- Add true time-window SLO evaluation from source series where analyzers expose
  enough timestamped data, while keeping the current session aggregate view.
- Feed SLO violations and budget rows into report packs without hiding raw
  signal evidence.

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

## Mid-Term Roadmap Plus: Multi-Language and Middleware Evidence

These items extend the active mid-term Evidence Studio cycle. They were
identified by auditing the current parser/analyzer coverage against the product
promise of broad programming-language and middleware support, and they are
sized to land alongside Incident Timeline, SLO/Golden Signals, Service Flow,
report packs, and AI productization.

### Broader Access and Edge Log Coverage

- Access-log parser currently supports nginx (combined, combined with response
  time) and apache combined formats. Extend file-first coverage to:
  - Tomcat access valve and Jetty NCSA request log.
  - HAProxy default and HTTP log formats.
  - Envoy/Istio default and JSON access logs (with service-mesh trace headers).
  - AWS ELB/ALB classic and v2 access logs, AWS CloudFront standard logs.
  - GCP HTTP(S) Load Balancer JSON, Azure App Service/Front Door JSON.
  - IIS W3C extended log format and Caddy/Traefik JSON access logs.
  - Kong/Tyk/AWS API Gateway access logs.
- Add a format auto-detect pass and per-source diagnostics, mirroring the
  trace-import importer dispatch.
- Normalize new fields (upstream service, mesh trace-id, TLS info) into the
  shared access-log `AnalysisResult` without breaking the current contract.

### Application Server and Web Server Diagnostics

- Add a server-log analyzer family for Tomcat catalina.out, Jetty server log,
  JBoss/WildFly server log, WebLogic AdminServer/ManagedServer log, WebSphere
  SystemOut/SystemErr, and GlassFish/Payara server log.
- Parse startup banners, deployment events, datasource pool warnings, stuck
  threads, hung-thread detections, and known severe error signatures.
- Parse nginx and Apache error logs alongside their access logs so request
  evidence and worker errors share an Incident Timeline.

### OpenTelemetry Logs and Wider Observability Signals

- Add an OpenTelemetry Logs (OTLP JSON / NDJSON) parser/analyzer track,
  complementing the existing OTel trace JSONL path. Surface severity, body,
  attributes, resource metadata, trace/span correlation, and severity bursts.
- Add Prometheus snapshot/OpenMetrics import for offline metrics evidence.
- Add Loki query JSON export and Tempo trace JSON export importers so LGTM
  stack exports become first-class evidence sources.
- Add Grafana dashboard JSON ingestion so dashboard panels can be referenced
  from Evidence Board cards.

### Database Slow Query and Engine Log Evidence

- Add slow-query and engine-log parsers/analyzers for PostgreSQL (csvlog and
  text), MySQL/MariaDB slow query log, MongoDB profiler/diagnostic.data
  exports, Redis slowlog get output, and SQL Server extended events JSON.
- Aggregate query fingerprints, p95/p99 latency, top queries, error counts,
  lock waits, and missing-index hints into a slow-query `AnalysisResult`.
- Add an EXPLAIN plan importer for PostgreSQL/MySQL so plan evidence can join
  the Evidence Board.

### Message Broker and Streaming Middleware

- Add Kafka broker/controller/state-change log parsing with ISR change,
  rebalance, under-replicated, log compaction, and KRaft quorum events.
- Add RabbitMQ server log parsing with connection churn, queue length, dead
  letter, and partition events; ingest `rabbitmq-diagnostics` JSON exports.
- Add Pulsar broker log, NATS server log, and ActiveMQ broker log parsers as
  follow-ups.
- Surface broker findings into Incident Timeline and Service Flow alongside
  trace-import dependencies.

### Container, Kubernetes, and Cloud Platform Evidence

- Add a Kubernetes evidence importer for `kubectl get events`/`describe pod`
  JSON, audit log NDJSON, kubelet log lines, and OOMKilled/restart/eviction
  signals.
- Parse container runtime logs (containerd, CRI-O, Docker daemon) when they
  accompany application-side evidence.
- Add cloud audit/log importers for AWS CloudTrail JSON, GCP Cloud Audit
  Logging JSON, and Azure Activity Logs JSON to back security-incident
  Evidence Board cards.

### Multi-Language Stack and Profiler Coverage

- Current runtime-stack parsers cover Java, Go, Python, Node.js, and .NET.
  Extend coverage to Ruby (rbspy text/JSON, stackprof), PHP (Excimer, Tideways
  CLI export, Xdebug profile), Rust (perf script, tokio-console export),
  Kotlin/Scala JVM (via JFR/thread dumps), Swift backtrace, and async stack
  traces.
- Add a generic pprof binary (`.pb.gz`) importer so Go pprof, Datadog Ruby/PHP
  profilers, py-spy speedscope-via-pprof, and other pprof-compatible runtimes
  share one analyzer path.
- Add py-spy raw output, rbspy raw output, async-profiler `.html`/collapsed
  formats, dotnet-trace `.nettrace`/speedscope output, and perf script
  collapsed importers.
- Promote the existing collapsed/JFR profiler analyzers into a unified
  multi-language profile analyzer with language-tagged frames, native vs
  managed split, and cross-language flamegraph rollups.

### Correlation and Evidence Stitching

- Define a cross-source correlation key model covering trace-id, span-id,
  request-id, customer/tenant-id, container-id, pod-uid, host-id, and PID.
- Add an evidence stitching pass that joins access logs, traces, runtime
  stacks, broker logs, and database slow logs by correlation key for the
  Incident Timeline and Evidence Board.
- Surface correlation gaps (missing trace-id, dropped parent span, log without
  matching request) as deterministic findings.

### Local Continuous Profiling Imports

- Add file-first importers for Grafana Pyroscope/Phlare snapshot export and
  Polar Signals Parca snapshot export so continuous-profiling evidence flows
  through the same flamegraph analyzers.
- Keep the OpenTelemetry Profiles signal (currently public alpha as of 2026
  Q1) on the active radar — see the long-term roadmap entry for full OTLP
  Profiles ingestion.

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

### OpenTelemetry Profiles and eBPF Continuous Profiling

- Track the OpenTelemetry Profiles signal lifecycle (public alpha in 2026 Q1
  with RC/GA targeted later in the year) and add an OTLP Profiles file
  importer when the spec is stable.
- Plan an eBPF profile evidence importer compatible with the open-source
  OpenTelemetry eBPF profiler and Parca/Pyroscope agents, covering C/C++, Go,
  Rust, Python, Java, Node.js, .NET, PHP, Ruby, and Perl frames.
- Define a unified profile evidence schema that joins JFR, pprof, OTLP
  Profiles, and eBPF samples so flamegraph and Evidence Board capture remain
  language-agnostic.

### Browser, Mobile, and Client Evidence

- Add a Real User Monitoring (RUM) import path for Core Web Vitals
  (LCP, INP, CLS) and resource-timing exports from open RUM beacons.
- Add browser performance trace import (Chrome DevTools `.json`, Lighthouse
  report JSON) and synthetic-check exports.
- Investigate mobile-side performance imports (Firebase Performance export,
  Sentry performance, App Center diagnostic exports) once a stable file
  contract is available.

### Anomaly Detection and Causal Analysis

- Add statistical baselining for golden signals (rolling median/p95,
  seasonality-aware bands) and surface deviations as deterministic findings.
- Add change-point detection for access-log latency, GC pause clusters, JFR
  CPU/lock signals, and trace error rate.
- Add a causal-chain explorer that links incident-timeline events by
  correlation key and proposes ordered root-cause hypotheses.

### Live and Streaming Evidence (Read-Only)

- Investigate read-only tailing of local files (access logs, GC logs, OTel
  exporters, broker logs) for live Evidence Studio sessions, while keeping
  local-first guarantees.
- Investigate streaming-mode Incident Timeline that promotes ongoing events
  into evidence cards as they occur.

### Report Distribution and Workflow Integrations

- Add Markdown/Mermaid/PlantUML report-pack exports so evidence can be pasted
  into issue trackers and ADRs.
- Add optional one-click templates for Jira/GitHub Issues, Slack/Teams summary
  posts, and email-friendly evidence pack zips — strictly opt-in and
  evidence-bound.

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
