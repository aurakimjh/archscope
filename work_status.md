# ArchScope Work Status

Last updated: 2026-07-21

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
- Release baseline: `v0.3.5` is the latest stable GitHub release. The
  `v0.3.1-rc1` prerelease remains available as the Jennifer MSA network-time
  release candidate.
- Current execution focus: resolve the `C-RG1` CONDITIONAL findings for the
  Chrome DevTools/V8 CPU profile slice and obtain an independent re-review
  `PASS`. Only after that group passes, complete the `H-RG1` offline HAR
  analysis slice, then close the T-571 Windows real-NIC coverage proof before
  live HTTP capture. Codex owns engine work and Claude owns UI work; each
  review group must pass before the next group starts.
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
- Published the stable `v0.3.5` desktop release with Mid-Term Plus file-first
  evidence importers, API/event contract analysis, architecture documentation
  drafts, advanced stitching, app/package metadata updates, and changelog
  notes.
- Added the Mid-Term Plus shared ingestion foundations for `v0.3.5`: evidence
  family boundary specs, reusable source-format registry, golden fixture
  diagnostic harness, normalized source metadata, and cross-source correlation
  key model.
- Added the access/edge-log coverage wave as part of `v0.3.5`: Tomcat/Jetty,
  HAProxy HTTP, Envoy/Istio text and JSON, AWS ELB/ALB/CloudFront, GCP/Azure
  edge JSON, IIS W3C, Caddy/Traefik JSON, and Kong/Tyk/API Gateway JSON parsing
  now feed additive access-log summaries, edge dependency tables, Service Flow,
  and Golden Signals.
- Built the T-571 Windows proof-of-capability harness under
  `spikes/t571-windows-coverage` (standalone Go module: loadgen + ETW/WFP/
  TCP-owner probes + a loopback-aware judge that emits the appendix-A ledger
  rows) and ran the first single-machine loopback measurement. Confirmed the
  ETW Kernel-Network payload (PID + `sport`/`dport`, 0 event loss) and that it
  does not observe loopback, and reconfirmed `GetExtendedTcpTable` owner-PID
  attribution at 100% (5/5). `Q-WIN-ETW-PAYLOAD`/`Q-WIN-WFP-ATTR` moved
  open→partial; ETW/WFP real-NIC attribution re-run remains.
- Completed the T-574 MITM proxy build-versus-integrate spike (gate 8). Surveyed
  Go/Python proxy candidates by license/maintenance/H2, produced a 4-option
  comparison matrix and a risk register, and **chose option 3 — an own H1
  semantic MVP with H2 passthrough behind a Proxy/Interceptor interface** (option
  4 native-H2 as a later graft, mitmproxy subprocess as a documented fallback).
  De-risked it with `spikes/t574-mitm-proxy-poc`, a pure-stdlib PoC whose offline
  MITM self-test passes and which cross-compiles for Windows.
- Closed the Chrome/V8 release implementation follow-up by wiring the shared
  15-file fixture manifest into Go golden tests, fixing V8 `children` graph and
  Chrome `ProfileChunk` delta handling, supporting hitCount-only and clamped
  negative-delta inputs, suppressing invalid downsampled timelines, refreshing
  Wails bindings, and completing the local release smoke.
- Upgraded the desktop baseline to Wails `v3.0.0-alpha2.117` with the matching
  `@wailsio/runtime` alpha.97 package, refreshed all outdated direct frontend
  dependencies (including React 19 and TypeScript 7), moved Linux CI and
  packaging to GTK4/WebKitGTK 6.0, upgraded official GitHub Actions to their
  Node 24-based majors, and revalidated the signed macOS app and DMG.
- Converted the Chrome/V8 and HTTP-capture designs into a paired English/Korean
  implementation plan with Codex engine ownership, Claude UI ownership,
  sequential group reviews, and mandatory individual gates only for security,
  coverage, CA/privilege, and Long Task semantics.

## Current Risk

The 2026-07-21 C-RG1 implementation review returned `CONDITIONAL`, so H-RG1
remains blocked. The primary engine risk is that the locked time-attribution
contract disagrees with the implementation while the existing golden tolerance
does not distinguish the two conventions. The primary UI risks are that parser
and timeline diagnostics are not rendered and that the available flamegraph and
drilldown data have no Browser CPU surface. T-578 now owns the complete A-E and
U1-U5 remediation set plus independent re-review; no Chrome release candidate
or HTTP implementation starts before `PASS`.

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

1. Resolve the `C-RG1` (T-578) CONDITIONAL findings. Codex handles A-E: reconcile
   and exactly pin time attribution, emit recording/active/idle/sampled duration
   fields with head/tail accounting, align Chrome-trace negative-delta and
   hitCount diagnostics, and fix the `startTime == 0` first-sample guard. Claude
   handles U1-U5: surface diagnostics and suppressed-timeline reasons, add the
   flamegraph/drilldown, advertise gzip inputs, complete empty-state/i18n/a11y,
   and add frontend regression coverage. Re-run the original independent review
   and require `PASS` before HTTP implementation starts.
2. Complete `H-RG1` (T-579): Codex finishes the manifest-driven bounded HAR
   engine and H-SEC1 security gate; Claude completes timeline/brush,
   list/detail/filter, diagnostics, and Workspace UI; review the vertical slice
   as one group.
3. Complete `H-RG2` through T-571 with the ETW/WFP real-NIC run and direct
   `GetExtendedTcpTable` CAP-5 CPU-overhead rerun. Treat the evidence disposition
   as the mandatory H-COV1 review before Windows live capture.
4. Continue in order through `H-RG3` live engine (T-580), `H-RG4` live UI and
   Windows E2E (T-581), `H-RG5` HTTP Diff (T-582), `X-RG1` cross-analysis
   (T-583), and `R-RG1` release acceptance (T-584). Do not skip a failed or
   conditional gate.
5. Review the long-term external APM import roadmap before assigning any new
   external-connector implementation range. Local file imports already cover
   OTLP, Zipkin, Elastic APM, Jaeger, and SkyWalking-style trace evidence;
   direct SaaS and product-specific connectors remain deferred until token
   storage, redaction, compliance, connector testability, and source-evidence
   contracts are clear.
6. Keep security/compliance evidence as a roadmap candidate, not an active
   TO-DO range, until it is selected against the external APM roadmap and the
   next release objective.
7. Continue pairing English/Korean docs whenever importer, analyzer, UI, or
   report-pack surfaces change.
8. Reserve `v0.4.0` for a broader Evidence Studio roll-up after the remaining
   roadmap candidates are prioritized and the chosen source families stabilize
   as one coherent workflow.
9. Keep release verification healthy before each release cut by repeating
   Windows GUI smoke, macOS signing/notarization validation, frontend bundle
   budget checks, and representative real-export fixture updates.

## Browser Profile / HTTP Capture Review Groups

The authoritative paired plan is
`docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` and
`docs/ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`.

| Order | Group | Owners | Status | Gate |
|---:|---|---|---|---|
| 1 | C-RG1 Chrome/V8 release implementation acceptance | Codex engine, Claude UI fixes | CONDITIONAL; A-E and U1-U5 remediation required | Independent re-review PASS |
| 2 | H-RG1 offline HAR analysis completion | Codex engine, Claude UI | Waiting on C-RG1 | H-SEC1 and group PASS |
| 3 | H-RG2 Windows coverage proof | Codex | Waiting on H-RG1 and Windows real-NIC run | H-COV1 PASS |
| 4 | H-RG3 live-capture engine foundation | Codex | Planned | H-SEC2 and group PASS |
| 5 | H-RG4 live UI and Windows E2E | Claude UI, Codex integration | Planned | Group PASS |
| 6 | H-RG5 HTTP session Diff | Codex engine, Claude UI | Planned | Group PASS |
| 7 | X-RG1 HTTP/profile/server-evidence correlation | Codex engine, Claude UI | Planned | Group PASS |
| 8 | R-RG1 integrated release acceptance | Codex + Claude + independent reviewer | Planned | Release PASS |

Group verdicts are PASS, CONDITIONAL, or FAIL. CONDITIONAL does not unblock the
next group. Individual reviews are limited to H-SEC1 (HAR security), H-COV1
(coverage evidence), H-SEC2 (CA/TLS/privilege), and optional C-SEM1 (`ph:"X"`
Long Task semantics).

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
| 2026-07-18 Chrome DevTools CPU profile design review P0-1 through P0-4 | Accepted as implementation gates; G1/G2 closed on 2026-07-20 with Chrome trace chosen as the first input and a microsecond `int64` measurement contract | T-558 through T-561 |
| Chrome/V8 classification, parser validation and frame identity, large-file/trace streaming findings | Accepted as dependent design hardening | T-562 through T-564 |
| Chrome profile end-to-end fixtures and English/Korean documentation parity | Accepted after the P0 contracts stabilize | T-565 |
| 2026-07-19 system HTTP capture design review P0-1 through P0-5 | Accepted; design note revised on 2026-07-19 to withdraw the per-process `bytesSystem` coverage premise, the raw-wire header guarantee, the single timing model, the size-threshold `AnalysisResult` contract, and automatic TLS passthrough | T-566 through T-571 |
| Proxy build-versus-integrate decision, capture lifecycle/recovery, HAR dialect identity and fixture corpus, capability tiers | Accepted as design completeness work reflected in the revised note | T-572 through T-575 |
| Repository-relative paths, cross-reference numbering, and scoped competitive claims | Accepted and applied directly to `docs/ko/SYSTEM_HTTP_CAPTURE.md`; the source ledger was filled on 2026-07-20 and the gate now reads "no design decision rests on an unverified row" | T-566 |
| 2026-07-20 Phase 0 design-gate authoring | Gates 1 through 5 written into `docs/ko/SYSTEM_HTTP_CAPTURE.md` and renumbered 1-11 with design gates separated from measurement gates; Phase 1 unblocked | T-566 through T-570 |
| English documentation parity for the HTTP capture note | Deferred until the Phase 1 contracts are approved, to avoid translating a design that is still moving | T-567 through T-571 |
| 2026-07-21 C-RG1 Chrome/V8 release implementation review | Accepted `CONDITIONAL`; A-E engine and U1-U5 UI findings must all be fixed inside C-RG1, then independently re-reviewed. The P1 time-attribution contract/golden mismatch and missing diagnostics/flamegraph UI block `PASS`; P2/P3 duration decomposition, trace diagnostic parity, first-sample guard, gzip discoverability, empty-state/i18n/a11y, and frontend regression coverage remain required in the same group | T-578; H-RG1/T-579 remains blocked |

## Mid-Term Plus Intake Plan

| Classification | Priority | TO-DO range | Target release | Execution rule |
|---|---|---|---|---|
| Completed Mid-Term Plus roll-up | P1/P2 | T-468 through T-555 | v0.3.5 | Shipped as one stable release covering ingestion foundations, access/edge, server, observability, database, broker, platform, profiling, stitching, API/event contracts, architecture docs, and documentation alignment. |
| Long-term external APM import | P2/P3 | Not assigned | Roadmap candidate | Keep direct SaaS/product connectors deferred until local token policy, redaction, compliance, connector fixtures, and evidence contracts are ready. |
| Security/compliance evidence | P1/P2 | Not assigned | Roadmap candidate | Promote only evidence-backed checks after the external APM roadmap and next release objective are reviewed. |
| Evidence Studio stabilization | P1/P2 | Not assigned | v0.4.0 candidate | Treat the full importer, contract, stitching, report-pack, and AI-gated interpretation workflow as one user story before a broader roll-up release. |

## Version Milestone Plan

| Release | Complete through | Release type | Required contents before cut |
|---|---|---|---|
| v0.3.5 | T-468 through T-555 | Stable Evidence Studio expansion | Released 2026-05-17 with Mid-Term Plus importers, API/event contract analysis, architecture docs drafts, advanced stitching, version metadata, changelog, release tag, and release workflow verification. |
| Next candidate | T-560 through T-565 implemented; T-578 remediation and re-review gated | Chrome browser/V8 profile slice | The 2026-07-21 C-RG1 review returned `CONDITIONAL`. Complete A-E engine and U1-U5 UI remediation, pass targeted goldens/frontend regressions, and obtain independent `PASS` before selecting the candidate version/tag or cutting the release. |
| Future candidate | T-571 and T-579 through T-584 review-gated | Windows-first HTTP capture and cross-OS evidence analysis slice | Start only after C-RG1/T-578 passes. Finish H-RG1 offline HAR analysis, then T-571/H-RG2 coverage proof, live engine/UI, HTTP Diff, cross-analysis, and integrated release acceptance in review-group order. Analyze Linux/macOS-generated supported evidence in the Windows UI. |
| v0.4.0 candidate | Not assigned | Evidence Studio roll-up | Full local evidence workflow is smoke-tested as one product story with sample packs, report exports, regression tests, AI gate checks, and release notes that present the expanded capability coherently. |

## Active TO-DO

Post-T-555 work includes the Jennifer MSA drilldown follow-up, the Chrome
DevTools/V8 CPU profile design gate reviewed on 2026-07-18, and the system HTTP
capture design gate reviewed on 2026-07-19. The Chrome profile acquisition-format
and measurement-unit contracts were resolved on 2026-07-20 (T-558, T-559), so
Chrome parsing, bounded streaming, desktop integration, and sampled CPU run
projections are now implemented, and the T-565 fixture/documentation parity
set was completed on 2026-07-21 (browser-profile fixture corpus plus paired
English documentation). Go golden-test wiring to the fixture manifest and the
local release smoke were completed on 2026-07-21.
The 2026-07-21 C-RG1 review then returned `CONDITIONAL`: the accepted parser,
bounded-streaming, graph-validation, sampled-CPU semantics, shared analysis
path, and CLI/Wails parity evidence remain valid, but T-578 stays open for A-E
engine and U1-U5 UI remediation and independent re-review. The review's P2/P3
items are required within the same group rather than deferred, and H-RG1 remains
blocked until the verdict is re-issued as `PASS`.
HTTP work must begin with bounded, sanitized HAR import. Phase 1 HAR import is
design-unblocked but is now process-gated on the C-RG1 Chrome implementation
review. Live MITM capture is Windows-first and remains blocked on the T-571
real-NIC coverage measurement and H-RG1 approval. Linux/macOS-generated supported
HAR, profiles, and logs remain valid offline inputs to the Windows UI; Linux/macOS
live-capture parity is not a first-release gate. The Korean design note was
revised on 2026-07-19 to absorb the review: it now states capability tiers
instead of an "all processes" requirement, splits fidelity into
semantic/decoded_wire/raw_wire, separates timing perspectives with explicit
known/not-applicable/unknown states, keeps `AnalysisResult` size-stable behind a
versioned session store, defines backpressure and loss accounting, and hardens
the security defaults. A further 2026-07-20 revision wrote the Phase 0 design
gates themselves: the source ledger, the Windows capability/fidelity matrix, the
transaction/time contract, the bounded session store, and the security model
(T-566 through T-570). With those five closed, **no design gate blocks Phase 1
HAR import**; T-571 keeps its contract half only, because proving Windows
coverage signals needs a real-NIC measurement run on Windows. T-572 through
T-575 are complete: fixture corpus, aggregator/recovery protocol, proxy spike,
and HTTP-specific Diff contract. Other roadmap candidates,
including long-term external APM import and security/compliance evidence, remain
deferred until explicitly promoted.

| ID | Priority | Status | Task | Depends on | Output |
|---|---|---|---|---|---|
| T-577 | P0 | [x] | Read the Chrome/V8 and system HTTP-capture designs and completed reviews, reconcile them with current code, and publish a paired English/Korean implementation plan with Codex engine ownership, Claude UI ownership, grouped review gates, and narrowly scoped individual security/semantic reviews. | T-558 through T-576 | Completed 2026-07-21: paired `BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`; stale T-565 documentation status corrected; work status and execution queue aligned |
| T-578 | P0 | [ ] | Resolve the C-RG1 `CONDITIONAL` verdict and obtain independent re-review `PASS`. Codex A-E: reconcile the time-attribution contract with real Chrome semantics and add an exact convention-pinning golden plus tail handling; emit separate recording/active/idle/sampled durations without conflating `total_duration_us`; add Chrome-trace negative-delta diagnostic fixture/parity and hitCount cross-check; make first-sample handling independent of `startTime`. Claude U1-U5: render engine diagnostics and explicit suppressed/aggregated timeline states; add flamegraph/drilldown; expose `.json.gz`; add empty-state/i18n/table a11y; add Browser CPU regression tests for diagnostics, suppression, and the non-Long-Task wording. | T-565, T-577 | Independent review on 2026-07-21 confirmed normalization, units, graph validation, bounded streaming/downsampling, sampled CPU semantics, shared analysis path, CLI/Wails parity, and 15/15 fixture goldens, but returned `CONDITIONAL`. H-RG1/T-579 and the Chrome release candidate remain blocked until all A-E/U1-U5 findings are fixed and the reviewer re-issues `PASS` |
| T-579 | P0 | [ ] | Complete and review H-RG1 offline HAR analysis. Codex: canonical transaction/timing/fidelity model, staged dialect normalizer, resource limits, shared manifest goldens, dedicated redaction, bounded result and CLI/Wails parity. Claude: pseudo-process tree, timeline/brush, list/detail/filter, fidelity/diagnostics/redaction UX and Workspace regression. | T-578 PASS; T-568 through T-573 | Must pass individual H-SEC1 before UI detail/export handoff, then one full vertical-slice group review |
| T-580 | P0 | [ ] | Implement and review H-RG3 live-capture engine foundation: session lifecycle/recovery, versioned NDJSON/blob store and cursor API, bounded streaming/backpressure/loss counters, H1 semantic MITM with H2 passthrough, Windows process attribution, Wails snapshot/event recovery, and CA/TLS policy. | T-571/H-RG2 PASS, T-579 PASS | Codex-owned engine group; mandatory H-SEC2 CA/TLS/privilege review before live UI handoff |
| T-581 | P1 | [ ] | Implement and review H-RG4 Windows live-capture UI and E2E. Claude: capture controls, CA lifecycle UX, process tree, stable live rows, recovery/backpressure/coverage/fidelity states. Codex: frozen bindings, acceptance fixtures, Windows E2E and packaging support. | T-580 PASS | Group PASS requires browser/curl/JVM/Electron supported-tier scenarios, long sessions, page re-entry, failure recovery, and honest unsupported-state UX |
| T-582 | P1 | [ ] | Implement and review H-RG5 HTTP-specific session Diff with versioned URL templates, bounded dimensions, explicit rate denominators, time-alignment grades, `http_capture_diff` findings, Workspace routing, and grade-aware comparison UI. | T-581 PASS, T-575 | Codex analyzer plus Claude comparison UI; reordered equivalent sessions must compare equal |
| T-583 | P1 | [ ] | Implement and review X-RG1 HTTP correlation with Chrome/V8 CPU runs, Jennifer network-gap evidence, and access logs, including bounded alignment/confidence diagnostics and provenance-aware UI drilldown/overlay. | T-582 PASS | Codex engine plus Claude UI; incompatible clocks must never be presented as causal proof |
| T-584 | P0 | [ ] | Run R-RG1 integrated release acceptance across Go test/vet/build, frontend test/build, Windows live E2E, macOS offline import/package smoke, paired documentation, support/security/performance matrices, and honest release notes. | T-583 PASS | No tag or GitHub release before independent release PASS |
| T-566 | P0 | [x] | Add a reproducible source ledger for every HTTP-capture research claim marked `[V]`, including official URL or source commit, retrieval date, verified fact, and design impact; scope absolute competitive-gap claims to what was actually inspected. | None | Completed 2026-07-20: appendix A rewritten as a filled ledger — claim-ID scheme (`V`/`I`/`N`/`Q` × area × topic), `fixed`/`partial`/`open` status values, four ledger tables (platform/spec, Windows capture, HTTPAnalyzer, competitors), and a refresh procedure. Gate condition restated as "no design decision rests on an `open` row" rather than "all rows verified"; `Q-COMP-MITM-PID` and the 1.5.4 absolutes are recorded as `open` and withdrawn from use |
| T-567 | P0 | [x] | Replace the global “all processes” requirement with a Windows-first platform/mode capability and fidelity matrix covering process attribution, HTTP versions, header/body fidelity, timing perspective, privileges, and explicit unsupported cases; separate Windows runtime support from cross-OS offline evidence compatibility. | T-566 | Completed 2026-07-20: §9.3 adds six per-mode matrices (attribution, HTTP versions, fidelity, timing perspective, privileges, explicit unsupported) across proxy/HAR/ETW/WFP/Npcap/keylog with `미검증` cells reserved for the T-571 spike; §9.4 splits runtime support (Windows only) from evidence compatibility (all OSes) with separate acceptance criteria, and records that HAR dialect differences are a function of the generating tool, not the generating OS |
| T-568 | P0 | [x] | Redesign the HTTP capture model so unknown/not-applicable/zero timings, client-proxy versus proxy-upstream perspectives, transaction state, connection reuse, header semantics, decoded versus wire bytes, and fidelity are represented without overclaiming Go `net/http` guarantees. | T-567 | Completed 2026-07-20: §6.3.1 fixes transaction boundaries (start = first request byte, end = last response byte/terminal state) with eight ambiguous cases resolved; §6.3.2 separates monotonic durations from wall-clock instants and locks ms/`float64`; §6.3.3 adds the state machine with transitions and aggregation rules; §6.3.4 adds versioned invariants `INV-1`~`INV-7` plus opposing H1/H2 golden sets (`INV-H1-1` non-overlap vs `INV-H2-1` overlap-permitted) |
| T-569 | P0 | [x] | Define one bounded session-store and `AnalysisResult` boundary: keep the envelope size-stable, page details through a versioned cursor API, specify capture lifecycle/crash recovery, and define disk-slow/full backpressure and loss accounting. | T-567 | Completed 2026-07-20: §7.6 defines `CaptureSessionStore` — append-only NDJSON layout with rebuildable offset index, three independent version fields, per-tier memory ceilings including truncated live-window records and non-resident offset index, eviction policy with chunked lazy loading and store-side filtering, disk spill limits with `stop`/`body_only`/`rotate` policies defaulting to `stop`, and the interface contract with snapshot-versioned cursors and single-writer semantics |
| T-570 | P0 | [x] | Complete the HTTP capture threat model and safe defaults for imported HAR, CA keys, TLS passthrough, JWT/cookies, blob identifiers, process metadata, user redaction rules, manifests, exports, and resource limits. | T-567 | Completed 2026-07-20: §11.2.1 adds the CA lifecycle state machine (`none`/`generated`/`trusted`/`expired`) with per-store partial-failure rules, rollback on partial install, and explicit "app removal does not remove the CA"; §11.6 consolidates the privilege contract around what elevated code may do (no resident elevated process, helper captures only, peer-credential IPC); §11.7 adds the `SEC-1`~`SEC-15` adversarial matrix, with `SEC-4`~`SEC-7` applying from Phase 1 |
| T-571 | P0 | [ ] | Prove or remove Windows coverage signals first, distinguishing TCP endpoint ownership, ETW event attribution, WFP/Npcap observations, and adapter counters by real scope; do not expose `bytesObserved/bytesSystem` as a process ratio without same-scope evidence. Defer Linux/macOS live-capture proofs without blocking Windows. | T-567 | Contract side completed 2026-07-20 (§10.4): candidate scopes separated, `CAP-1`~`CAP-6` acceptance criteria defined with false attribution set to zero tolerance, and a disposition table whose all-fail branch removes absolute coverage ratios and keeps only the five self-observed counters. **First measurement done 2026-07-20** via the `spikes/t571-windows-coverage` harness (single-machine loopback smoke, build 26200.8737, 5 control processes at ~498 tps). Results (§10.4.5): TCP endpoint ownership passed CAP-1 100% (5/5), CAP-2 0, CAP-4 bypass detected → CAP-1~4 pass, reconfirming the `V-WIN-TCPTABLE` owner-PID lookup; ETW payload carries `Execution` PID + `sport`/`dport` with 0 event loss over 100,541 events, but **Kernel-Network does not observe loopback**, so ETW CAP-1/CAP-4 are N/A in loopback mode; WFP `netsh wfp show netevents` logged no ALLOW connections in default config (audit needed). `Q-WIN-ETW-PAYLOAD` and `Q-WIN-WFP-ATTR` moved open→partial; §9.3.1 cells updated. **Still open: ETW/WFP real-NIC re-run (`-Target host:port`) to settle ETW CAP-1/CAP-4, plus a `GetExtendedTcpTable`-direct rerun for TCP-owner CAP-5 (polling put it at the 9~13%p CPU boundary).** |
| T-558 | P0 | [x] | Decide whether the first supported browser input is the current Chrome Performance trace (`.json`/`.json.gz`) or direct V8 `.cpuprofile`, then align the design title, collection guide, phase order, and menu copy with that decision. | None | Completed 2026-07-20: **Chrome Performance trace is the first input**, with `.cpuprofile` supported in the same Phase 1 because the V8 normalization core is shared. Narrowing the Phase 1 trace adapter to a `ph:"P"` filter removes most of the cost objection — duration-event modelling stays in Phase 4 under G4. Scoping `.cpuprofile` first was rejected as building a different feature, not a cheaper one: it fails both stated motivations (frontend bottleneck evidence, zero collection cost). Title kept, menu copy set to "브라우저 성능 분석", collection guide leads with Save trace, matrix entries registered as `chrome-trace-json`/`v8-cpuprofile`, Phase 1/4 items revised |
| T-559 | P0 | [x] | Define one value/time-unit contract for V8 samples, including delta attribution, `IntervalMS`, idle accounting, summary fields, Diff normalization, and CLI/Wails defaults; lock it with duration parity tests. | T-558 | Completed 2026-07-20 (§4.3.1): `Sample.Value` is **microseconds `int64`** — nanoseconds rejected as precision the source lacks, float ms rejected because accumulation error makes the tolerance undefinable; ms conversion happens once at display. `samples[]` is authoritative and `hitCount` becomes a cross-check (`INV-C3`), with `hitCount`-only inputs served by an aggregate path that reports no duration rather than estimating one. The `IntervalMS` back-calculation is replaced by a `Parsed.ValueUnit` branch, leaving the 22 existing formats unchanged. Cost attribution assigns `timeDeltas[i]` to sample `i-1`, and recording/active/idle durations are recorded separately. Invariants `INV-C1`~`INV-C9` cover `total = self + Σ children`, the two cross-checks that catch dropped subtrees, no double-counting under recursion, and CLI/Wails parity |
| T-560 | P0 | [x] | Define and implement one desktop integration path for profile evidence so Browser CPU analysis, the existing Profiler policy, Analysis Workspace, Diff, and Report Export all consume the same normalized parser output. | T-558, T-559 | Completed 2026-07-20: Browser CPU uses `AnalyzeProfileEvidence`, adds its result to Analysis Workspace/Export flow, and Diff loads V8/Chrome through `parsers/profile.Parsed` rather than adding formats to the legacy switch. |
| T-561 | P0 | [x] | Redesign Phase 3 temporal analysis: use Chrome trace task boundaries for true Long Task findings, or rename cpuprofile-only output to sampled CPU runs; define a pre-collapse ordered-series contract. | T-558, T-559 | Completed 2026-07-20: pre-collapse samples produce bounded `cpu_sample_runs` and `cpu_activity`; runs split on stack/idle/gap, emit only `SAMPLED_CPU_HOTSPOT` at 100ms+, and UI explicitly states that they are not browser Long Tasks. |
| T-556 | P0 | [x] | Add Jennifer MSA application drilldown so a user can choose a middle-tier application/TXID or average-transaction application URL and recalculate response-time composition from that application and its downstream calls instead of the whole GUID root. | Existing Jennifer MSA `guid_groups`, `profiles`, `msa_edges`, `slow_sql_events` contracts | Completed 2026-05-28: single TXID selector plus average-mode unique application URL selector, scoped response breakdown, scoped topology/timeline/treemap, scoped transaction and slow SQL tables |
| T-572 | P1 | [x] | Build the Phase 1 HAR normalization and security fixture corpus for Chrome, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic, malformed, oversized, and secret-bearing inputs in the shared asset repository, including Linux/macOS-generated fixtures consumed by the Windows UI, with provenance and expected diagnostics. | T-568, T-570 | Completed 2026-07-20: synthetic-first corpus at `../projects-assets/test-data/har-fixtures/` — 10 dialect fixtures (Chrome H1/H2/sensitive, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic), 8 malformed (BOM, schema violation, absent bodies, base64-no-encoding, sizes -1, degenerate timestamps, truncated, NetLog rejection), 2 adversarial (secrets-everywhere redaction golden, deep nesting) plus generators for oversized/gzip/extreme-nesting inputs that are never committed. Per-directory manifests fix provenance, basis claims, and expected dialect/diagnostic/redaction assertions; fixtures are deterministic script outputs. Honest limitation recorded: all synthetic, so CreatorMatch strings stay provisional until the real-export collection plan in the corpus README is executed (Chrome/Firefox first; Fiddler alongside the T-574 Windows spike) |
| T-573 | P1 | [x] | Define a deterministic bounded aggregator and recoverable Wails snapshot/event protocol, then prove live completion-order and offline start-order parity with property and long-session tests. | T-568, T-569 | Completed 2026-07-20: HAR import records the shared aggregate snapshot; sequence/snapshot recovery plus permutation and 50k-entry long-session live/offline parity tests are fixed. |
| T-574 | P1 | [x] | Run a Windows-first build-versus-integrate spike for the MITM proxy, comparing vetted libraries, a subprocess adapter, an H1-limited MVP, and a native H1/H2 implementation on licensing, Windows packaging/signing, updates, PID hooks, fidelity, and crash isolation. | T-568, T-569, T-570 | Completed 2026-07-20: library survey (goproxy BSD-3 maintained but H2-limited; martian archived; gomitmproxy GPL-3.0/no-H2/stale; mitmproxy MIT but Python-bundle burden), comparison matrix across the 4 options, and a measured risk register (R-574-1..7) written into `docs/ko/SYSTEM_HTTP_CAPTURE.md` build-vs-integrate 결정. **Decision: option 3 (own H1 semantic MVP + H2 passthrough)** behind a Proxy/Interceptor interface, with option 4 (native H2) as a later graft and option 2 (mitmproxy subprocess) as a documented fallback. Proven by `spikes/t574-mitm-proxy-poc` — pure-stdlib PoC (CA + on-the-fly leaf + CONNECT + ALPN-peek passthrough + H1 terminate/forward/capture) whose offline self-test passes and which cross-compiles for windows/amd64; PID attribution reuses the T-571-validated `GetExtendedTcpTable` path. Gate 8 closed |
| T-575 | P1 | [x] | Define HTTP-specific report and session Diff projections plus the independent current-code navigation integration; do not claim generic Diff or a not-yet-implemented browser menu provides these paths automatically. | T-568, T-569 | Completed 2026-07-20 (§12.4.1): versioned deterministic URL templating (`{id}`/`{uuid}`/`{hash}`/`{token}`/`{email}`, query keys only, top-K template cap with `{other}` bucket); comparison dimensions endpoint/host/process with process disabled for HAR pseudo-process sessions and cross-dimension sum checks; explicit numerator/denominator on every rate, per-minute normalization skipped for degenerate-timestamp sessions, percentiles computed per session and never averaged; time-alignment grades `aligned`/`duration_only`/`none` with overlay views suppressed at `none`; bounded `http_capture_diff` AnalysisResult (top-K 50 tables, HTTP_DIFF_* findings) whose drilldown uses per-session store cursors and whose export never rescans stores (T-460 principle); Wails entry via HttpCapturePage compare action and Workspace comparison routing — no new NavKey, no browser-menu assumption, legacy DiffPage untouched. Implementation is Phase 4 item 31 |
| T-562 | P1 | [x] | Unify the JSON and hard-coded profiler classifiers behind a source-format-aware interface, then define where FlameNode category/color enrichment occurs without reclassifying Node V8 frames as browser frames. | T-559 | Completed 2026-07-20: analyzer-stage flamegraph enrichment assigns browser application/runtime/dependency versus Node runtime categories and colors from source format. |
| T-563 | P1 | [x] | Harden V8 parsing with graph validation, optional-field policy, timestamp reconciliation, canonical per-frame script identity, redaction, and explicit parser/analyzer/Wails options. | T-558, T-559 | Completed 2026-07-20: node/parent graph validation, timestamped microsecond samples, canonical frame identity with URL redaction, and max-bytes/max-samples options flow through parser, analyzer, and Wails. |
| T-564 | P1 | [x] | Replace whole-file/double-unmarshal assumptions with a size/gzip/streaming strategy for direct profiles and Chrome traces; define weighted downsampling and partial-result semantics. | T-558, T-563 | Completed 2026-07-21: V8/Chrome path selects a capped gzip reader before whole-file loading; Chrome trace walks `traceEvents` token-by-token and only decodes `ph:"P"` chunks (with a count-only first pass when bounded output is needed). Default 500k deterministic time-weighted buckets preserve total microsecond duration, emit `partial_result`/source-and-output counts plus `PROFILE_DOWNSAMPLED`, and Browser CPU visibly labels aggregated output. Gzip-stream, weighted-duration, and decompressed-limit regressions pass. |
| T-557 | P1 | [ ] | Promote the Jennifer MSA drilldown calculation from UI projection to a Go analyzer contract if repeated use shows it should be exported/reported outside the desktop page. | T-556 | Planned follow-up: stable result schema for drilldown scopes |
| T-576 | P2 | [x] | Pair the revised Korean HTTP capture note with English documentation, an importer/dialect support matrix, a data-model reference, and a security guide once the Phase 1 contracts are approved; keep the source ledger appendix filled as claims are re-verified. | T-566 through T-571 | Completed 2026-07-21: `docs/en/SYSTEM_HTTP_CAPTURE.md` added as the condensed English pair with dedicated sections for the data-model reference (bounded `http_capture` envelope, `CaptureSessionRef`, versioned `CaptureSessionStore`), the importer/dialect support matrix (7-step pipeline, verified dialect quirks, `HAR_*` diagnostic codes, NetLog rejection, T-572 corpus pointer), and the security guide (import-time redaction rationale with the Okta incident, value-preserving techniques, CA lifecycle, privilege contract, SEC-1~15); also covers the transaction/time model, capability tiers, T-575 Diff contract, phase status, and the T-571 measurement state. The Korean note stays normative and Appendix A remains the auditable claim ledger — English §8 must be refreshed when the T-571 real-NIC rerun lands |
| T-565 | P2 | [x] | Add sanitized Chrome/Node/CDP malformed and end-to-end fixtures, then pair the Korean design with English documentation and update importer matrix, data model, user guide, and performance docs after the P0 contracts stabilize. | T-558 through T-564 | Completed 2026-07-21: synthetic browser-profile corpus at `../projects-assets/test-data/browser-profile-fixtures/` (6 valid — V8/Node/CDP/trace object/array/gzip; 7 malformed — broken children reference, hitCount-only, negative delta, unknown node, no-profile-chunk trace, truncated, empty; 2 e2e reproducing the Phase 3 acceptance scenario of a 210ms renderList run at 3.1s in both cpuprofile and trace form, doubling as the AC-1 parity input and reserving the trace `RunTask` as the Phase 4 `BROWSER_LONG_TASK` golden) with manifest-fixed expected formats/diagnostics/findings; `docs/en/CHROME_DEVTOOLS_CPUPROFILE.md` added as the condensed English pair (Korean note stays normative); importer matrix, data model, user guide, and performance docs updated in both languages with the implemented formats, 256 MiB/500k-sample guards, downsample suppression, and `http_capture` MVP envelope; CLI `profile import --format` help lists `v8-cpuprofile`/`chrome-trace-json`; Go golden tests now consume all 15 manifest entries and enforce Chrome trace/cpuprofile duration parity, diagnostics, findings, sampled-run location/duration, and hitCount-only temporal suppression. |
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
| T-499 | P1 | [x] | Define the slow-query and database engine-log `AnalysisResult` contract with query fingerprint, latency percentiles, lock wait, error, row count, database, schema, and sanitized SQL fields. | T-468, T-471, T-472 | Completed 2026-05-16: `database_slow_query` contract emits sanitized fingerprints, latency percentiles, lock waits, errors, row counts, database/schema, source metadata, and dependency rows |
| T-500 | P1 | [x] | Add PostgreSQL csvlog and text log parsers/analyzers for slow queries, errors, lock waits, checkpoints, autovacuum signals, and top query fingerprints. | T-499, T-470 | Completed 2026-05-16: PostgreSQL text and csvlog duration/error/lock evidence parse into query and fingerprint tables |
| T-501 | P1 | [x] | Add MySQL/MariaDB slow query log parsing and analysis for query fingerprints, p95/p99 latency, lock time, rows examined, and top query findings. | T-499, T-470 | Completed 2026-05-16: MySQL/MariaDB slow-query blocks parse Query_time, Lock_time, Rows_examined, query text, and fingerprints |
| T-502 | P2 | [x] | Add MongoDB profiler and diagnostic.data export parsing for slow operations, collection/index hints, lock/queue indicators, and error counts. | T-499, T-470 | Completed 2026-05-16: MongoDB profiler-style JSON imports operation, namespace, command/query text, millis, and row counts |
| T-503 | P2 | [x] | Add Redis slowlog get output parsing and analysis for command fingerprints, latency, key-pattern sanitization, and top slow commands. | T-499, T-470 | Completed 2026-05-16: Redis slowlog text imports command fingerprints and microsecond latency normalized to milliseconds |
| T-504 | P2 | [x] | Add SQL Server extended events JSON parsing for slow query, wait, deadlock, and error evidence. | T-499, T-470 | Completed 2026-05-16: SQL Server extended-event JSON imports statement/sql_text, duration, waits, database name, and error fields |
| T-505 | P1 | [x] | Add PostgreSQL and MySQL EXPLAIN plan importers and Evidence Board plan cards linked to slow-query fingerprints. | T-499, T-500, T-501 | Completed 2026-05-16: PostgreSQL/MySQL EXPLAIN JSON rows are flagged as plan evidence and linked through query fingerprints and plan summaries |
| T-506 | P1 | [x] | Integrate database slow-query evidence into Incident Timeline, SLO/Golden Signals, Service Flow enrichment, and report packs. | T-500, T-501, T-505 | Completed 2026-05-16: database findings flow through Evidence Board/report packs, query rows feed Incident Timeline, latency/error/lock summaries feed Golden Signals, and dependency rows feed Service Flow |
| T-507 | P1 | [x] | Define the broker/streaming middleware `AnalysisResult` contract and finding taxonomy for rebalance, replication, queue pressure, dead letter, partition, and broker health events. | T-468, T-471, T-472 | Completed 2026-05-16: `broker_log` contract normalizes broker events, dependency rows, source metadata, and broker finding taxonomy |
| T-508 | P1 | [x] | Add Kafka broker, controller, and state-change log parsing with ISR change, rebalance, under-replicated partition, log compaction, and KRaft quorum findings. | T-507, T-470 | Completed 2026-05-16: Kafka-style log lines classify ISR/replication, rebalance, partition, compaction, and KRaft quorum events |
| T-509 | P1 | [x] | Add RabbitMQ server-log parsing plus `rabbitmq-diagnostics` JSON import for connection churn, queue length, dead-letter, partition, and node health findings. | T-507, T-470 | Completed 2026-05-16: RabbitMQ text and diagnostics JSON import queue pressure, connection churn, dead-letter, and broker-health evidence |
| T-510 | P2 | [x] | Add Pulsar broker-log parsing for topic, ledger, bookie, broker health, and backlog-related findings. | T-507, T-470 | Completed 2026-05-16: Pulsar-style log lines classify topic/ledger/bookie/backlog and health events under the broker taxonomy |
| T-511 | P2 | [x] | Add NATS server-log parsing for connection churn, slow consumers, cluster events, JetStream pressure, and authorization failures. | T-507, T-470 | Completed 2026-05-16: NATS/JetStream log lines classify connection churn, slow consumers, pressure, cluster/partition, and auth failures |
| T-512 | P2 | [x] | Add ActiveMQ broker-log parsing for queue pressure, consumer churn, dead letter, store usage, and broker health events. | T-507, T-470 | Completed 2026-05-16: ActiveMQ-style log lines classify queue pressure, dead letters, consumer churn, store usage, and health events |
| T-513 | P1 | [x] | Surface broker findings in Incident Timeline and Service Flow beside trace-import dependencies and service-edge evidence. | T-508, T-509, T-472 | Completed 2026-05-16: broker findings flow to report/Evidence Board paths, broker events feed Incident Timeline, broker summaries feed Golden Signals, and dependency rows feed Service Flow |
| T-514 | P2 | [x] | Define the Kubernetes/container evidence contract and identity model for cluster, namespace, workload, pod, container, node, image, restart count, and owner references. | T-468, T-471, T-472 | Completed 2026-05-16: `kubernetes_evidence` preserves platform identity fields, restart counts, image/node/pod/container context, and source metadata |
| T-515 | P2 | [x] | Add `kubectl get events` and `describe pod` JSON importers with OOMKilled, restart, eviction, scheduling, image pull, and readiness findings. | T-514, T-470 | Completed 2026-05-16: Kubernetes event/pod JSON import OOMKilled, restart, eviction, scheduling, image pull, and readiness/probe evidence |
| T-516 | P2 | [x] | Add kubelet log parsing for pod lifecycle, eviction, probe, image, runtime, and node-pressure events. | T-514, T-470 | Completed 2026-05-16: kubelet text logs classify lifecycle, eviction, probe/readiness, image, runtime, and node-pressure events |
| T-517 | P2 | [x] | Add container runtime log parsing for containerd, CRI-O, and Docker daemon events that accompany application evidence. | T-514, T-470 | Completed 2026-05-16: container-runtime text logs classify containerd/CRI-O/Docker events with pod/container/node identity hints |
| T-518 | P2 | [x] | Add cloud audit/log importers for AWS CloudTrail JSON, GCP Cloud Audit Logging JSON, and Azure Activity Logs JSON with security-incident evidence fields. | T-468, T-470, T-471 | Completed 2026-05-16: AWS CloudTrail, GCP audit, and Azure Activity JSON map actor, operation, resource, provider, and security category fields |
| T-519 | P2 | [x] | Map Kubernetes, container, and cloud audit evidence into Incident Timeline, Evidence Board, SLO saturation signals, and security-oriented report sections. | T-515, T-516, T-517, T-518 | Completed 2026-05-16: platform findings flow through Evidence Board/report packs, platform rows feed Incident Timeline, and restart/OOM/node-pressure/security counts feed Golden Signals |
| T-520 | P1 | [x] | Define a unified profile evidence schema with language-tagged frames, native-versus-managed split, async frame markers, source runtime, and flamegraph rollup metadata. | T-468, existing profiler analyzers | Completed 2026-05-16: `profile_evidence` normalizes language/runtime-tagged frames, native/managed/async markers, and flamegraph rollup metadata |
| T-521 | P1 | [x] | Add a generic pprof binary `.pb.gz` importer shared by Go pprof, Datadog Ruby/PHP profiler exports, py-spy pprof-compatible output, and other pprof runtimes. | T-520, T-470 | Completed 2026-05-16: pprof `.pb.gz` imports use `github.com/google/pprof/profile` and map samples into the unified profile schema |
| T-522 | P2 | [x] | Add py-spy raw output and rbspy raw output importers with Python/Ruby frame tagging and thread/process metadata. | T-520, T-470 | Completed 2026-05-16: py-spy and rbspy raw text stacks import with runtime/language labels and thread block metadata |
| T-523 | P1 | [x] | Add async-profiler `.html` and collapsed input parity for the unified profile path while preserving existing collapsed/JFR behavior. | T-520, existing profiler analyzers | Completed 2026-05-16: unified profile import reuses existing async-profiler collapsed and HTML parser paths |
| T-524 | P2 | [x] | Add dotnet-trace `.nettrace` and speedscope output importers with .NET frame tagging and managed/native separation. | T-520, T-470 | Completed 2026-05-16: speedscope JSON, including dotnet-trace speedscope exports, maps into unified profile frames with .NET/runtime tagging |
| T-525 | P2 | [x] | Add perf script collapsed import for Rust/native profiling evidence with symbol, thread, and process metadata. | T-520, T-470 | Completed 2026-05-16: perf collapsed stacks import through the profile path with native/Rust-oriented frame classification |
| T-526 | P2 | [x] | Add Ruby stackprof plus PHP Excimer, Tideways CLI export, and Xdebug profile importers where stable file contracts are available. | T-520, T-470 | Completed 2026-05-16: StackProf JSON, generic PHP profiler JSON, and Xdebug cachegrind text map into profile samples |
| T-527 | P3 | [x] | Add Swift backtrace and generic async stack trace parsing after the primary server-side runtime profiler inputs stabilize. | T-520, T-470 | Completed 2026-05-16: Swift/generic async text stacks import with async frame markers |
| T-528 | P1 | [x] | Promote collapsed and JFR profiler outputs into the unified multi-language profile analyzer with cross-language flamegraph rollups and Evidence Board capture. | T-521, T-523, T-520 | Completed 2026-05-16: unified analyzer rolls imported samples into existing flamegraph/drilldown paths and Golden Signal capture |
| T-529 | P1 | [x] | Implement the first evidence-stitching pass that joins access logs, traces, runtime stacks, broker logs, and database slow logs by correlation key. | T-472, T-484, Trace import MVP, T-506, T-513 | Completed 2026-05-16: `stitched_evidence` reads AnalysisResult JSON files and joins table rows by trace/span/request/tenant/container/host/PID keys |
| T-530 | P1 | [x] | Surface correlation-gap findings for missing trace ID, dropped parent span, unmatched request log, unmatched database call, and unmatched broker event. | T-529 | Completed 2026-05-16: stitching emits `MISSING_TRACE_ID`, `DROPPED_PARENT_SPAN`, `UNMATCHED_REQUEST_LOG`, `UNMATCHED_DATABASE_CALL`, and `UNMATCHED_BROKER_EVENT` findings |
| T-531 | P2 | [x] | Add stitched evidence views in Incident Timeline, Evidence Board, Service Flow, and report packs with raw source evidence preserved. | T-529, T-530 | Completed 2026-05-16: stitched gaps/matches feed Incident Timeline, findings remain Evidence Board/report-pack compatible, and stitched dependencies feed Service Flow |
| T-532 | P2 | [x] | Add Grafana Pyroscope/Phlare snapshot export importer and map it into the unified profile evidence schema. | T-520, T-470 | Completed 2026-05-16: profile importer accepts Pyroscope/Phlare-style JSON snapshots and maps names/stacks into profile samples |
| T-533 | P2 | [x] | Add Polar Signals Parca snapshot export importer and map it into the unified profile evidence schema. | T-520, T-470 | Completed 2026-05-16: profile importer accepts Parca-style profile JSON snapshots through the generic continuous-profile path |
| T-534 | P2 | [x] | Route continuous-profiling snapshots through the existing flamegraph analyzer, Evidence Board capture, and report-pack export paths. | T-532, T-533, T-528 | Completed 2026-05-16: continuous profile snapshots roll up through `profile_evidence` flamegraph charts, findings, Golden Signals, and generic report-pack capture |
| T-535 | P3 | [x] | Track the OpenTelemetry Profiles signal lifecycle and add a decision note for when OTLP Profiles should move from long-term radar to active ingestion work. | T-520 | Completed 2026-05-16: added English/Korean OTLP Profiles decision notes with activation criteria |
| T-536 | P2 | [x] | Keep English/Korean roadmap, data-model notes, examples, and importer support matrices current as Mid-Term Plus tasks are implemented. | T-468 through T-535 | Completed 2026-05-16: updated data model/parser docs, examples, and English/Korean importer support matrices |
| T-537 | P1 | [x] | Add OpenAPI import for endpoint/method/operation metadata and expose it through the common parser/analyzer boundary. | T-536, T-470 | Completed 2026-05-17: OpenAPI JSON and conservative YAML subset import into `api_contract_analysis` operations |
| T-538 | P1 | [x] | Compare OpenAPI operations with access-log evidence to identify observed, undocumented, and unused API surfaces. | T-537, access-log evidence | Completed 2026-05-17: access-log result tables are normalized to route templates and compared with OpenAPI operations |
| T-539 | P1 | [x] | Detect slow and high-error APIs by combining API contract matches with access-log latency/error evidence. | T-538 | Completed 2026-05-17: slow/high-error operation tables and findings use configurable latency/error thresholds |
| T-540 | P2 | [x] | Add AsyncAPI import for channel, direction, operation, and message metadata. | T-536, broker evidence | Completed 2026-05-17: AsyncAPI JSON and conservative YAML subset import channel and message metadata |
| T-541 | P2 | [x] | Compare AsyncAPI channels with broker/event evidence to find undocumented and unused event channels. | T-540, T-513 | Completed 2026-05-17: broker result tables are matched to AsyncAPI channels with undocumented/unused channel findings |
| T-542 | P2 | [x] | Surface API/event contract findings in Incident Timeline, SLO/Golden Signals, Evidence Board, examples, and support docs. | T-537 through T-541 | Completed 2026-05-17: API contract summaries feed Golden Signals, findings/tables feed generic Evidence Board/report paths, and key findings feed Incident Timeline |
| T-543 | P1 | [x] | Define the architecture-documentation `AnalysisResult` contract for arc42 Context, Runtime View, Deployment View, Quality Requirements, Risks, and ADR draft sections. | T-536 | Completed 2026-05-17: `architecture_docs` contract emits arc42 sections, context/runtime/deployment rows, quality requirements, risks, and ADR drafts |
| T-544 | P1 | [x] | Generate arc42 Context and System Scope draft inputs from service-flow, API contract, and stitched evidence. | T-543, T-542, T-531 | Completed 2026-05-17: context services and interface rows are inferred from service dependencies, API contracts, and stitched evidence |
| T-545 | P1 | [x] | Generate Runtime View draft inputs from Incident Timeline, trace, broker, database, and stitched evidence. | T-543, T-531 | Completed 2026-05-17: runtime view rows capture service dependencies, matches, critical paths, and correlation gaps |
| T-546 | P2 | [x] | Generate Deployment View draft inputs from Kubernetes/platform, cloud audit, host/container, and service metadata evidence. | T-543, T-519 | Completed 2026-05-17: deployment rows preserve namespace, workload/pod, container, node, image, and platform risk hints |
| T-547 | P1 | [x] | Generate Quality Requirement and Risk sections from deterministic findings across latency, errors, saturation, security, and correlation gaps. | T-543, T-542, T-531 | Completed 2026-05-17: quality requirements and risks are derived from deterministic findings with severity, impact, mitigation, and evidence refs |
| T-548 | P2 | [x] | Add ADR draft rows with decision, context, alternatives, tradeoffs, consequences, and evidence references. | T-543, architecture evidence | Completed 2026-05-17: ADR draft rows are generated from highest-priority risk and quality evidence |
| T-549 | P1 | [x] | Enhance evidence stitching with timestamp-window matching so nearby records can correlate even when only request/service identity is shared. | T-529 | Completed 2026-05-17: `stitch analyze` adds configurable timestamp-window matching with `time_window_service_alias` match rows |
| T-550 | P1 | [x] | Add service alias normalization for gateway, database, broker, Kubernetes workload, and application naming variants in stitched evidence. | T-549, T-531 | Completed 2026-05-17: service aliases normalize service/svc/deployment suffixes, database/broker prefixes, and Kubernetes pod replica suffixes |
| T-551 | P2 | [x] | Add trace-profile linkage by joining profile evidence with trace/service/runtime labels when correlation metadata is present. | T-549, T-528 | Completed 2026-05-17: `profile_evidence.profile_samples.labels.trace_id` participates in trace/profile stitching |
| T-552 | P2 | [x] | Add stitched drilldown tables for match confidence, time-window reason, alias reason, and raw evidence references. | T-549, T-550, T-551 | Completed 2026-05-17: `match_drilldowns` preserves confidence, reason, alias, source node IDs, evidence refs, and bounded raw source rows |
| T-553 | P2 | [x] | Surface advanced stitched-evidence signals in Incident Timeline, Service Flow, and Golden Signals. | T-552 | Completed 2026-05-17: time-window and trace-profile matches feed Incident Timeline labels, Service Flow match status, and Golden Signal summaries |
| T-554 | P2 | [x] | Add a stitched-evidence detail view or drilldown-ready state projection for the desktop UI. | T-552 | Completed 2026-05-17: desktop state can drill from match events into `match_drilldowns` and raw source node rows |
| T-555 | P2 | [x] | Update English/Korean docs, examples, and support matrix for architecture docs and advanced stitching work. | T-543 through T-554 | Completed 2026-05-17: updated data model, parser design, roadmap, importer matrix, and stitching examples |

## Verification Notes

- 2026-07-21 Chrome/V8 release finishing passed the shared manifest golden test
  across 15 valid, malformed, and E2E fixtures, including 3.1s/210ms
  `renderList` parity between `.cpuprofile` and Chrome trace inputs.
- 2026-07-21 release smoke passed `go test ./...`, `go vet ./...`, and
  `go build ./cmd/archscope-engine ./cmd/archscope-app`; the known macOS
  26.0-to-11.0 linker warning remains non-blocking. Frontend
  `npm run test:state` and `npm run build` passed after resolving the direct
  dependency advisories with ECharts 6.1.0 and Vite 8.1.5; `npm audit` reports
  zero vulnerabilities. The shared ECharts chunk is 698.80 KB raw / 235.56 KB
  gzip, within the 700 KB raw budget.
- 2026-07-21 local Wails packaging produced the ad-hoc-signed
  `bin/archscope.app` and `bin/archscope-arm64.dmg`; strict deep codesign
  verification and `hdiutil verify` both passed.
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
- 2026-05-17 `v0.3.5` release-prep verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...` passed under
  `apps/engine-native` with the known macOS linker warning,
  `npm run test:state` passed for derived frontend state regressions,
  `npm run build` passed for the Wails frontend, and `git diff --check`
  passed. Frontend package/build metadata is set to 0.3.5.
- 2026-05-17 `v0.3.5` GitHub release workflow completed successfully after the
  `Prepare v0.3.5 release` commit and `v0.3.5` tag were pushed. The release is
  the latest stable release; GitHub workflow annotations were limited to
  non-blocking Node.js 20 deprecation notices.
- 2026-05-16 T-468 through T-472 verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...` passed under
  `apps/engine-native` with the known macOS linker warning, and
  `git diff --check` passed.
- 2026-05-17 documentation current-state verification passed after source/doc
  alignment: `git diff --check`, `bash -n scripts/run-demo-site-data.sh`,
  `env GOCACHE=/tmp/archscope-go-cache go test ./cmd/archscope-engine`, CLI
  help checks for report/demo-site/thread-dump/api-contract/stitch/
  architecture-docs/database-log/broker-log, and sample executions for API
  contract, stitching, thread-dump, trace import, database-log, broker-log, and
  architecture-docs workflows.
- 2026-05-28 Wails frontend verification passed after Jennifer MSA application
  drilldown: `npm run build`.

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
