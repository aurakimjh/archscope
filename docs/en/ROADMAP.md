# Roadmap

## Phase 1: Foundation

- Repository skeleton
- Desktop UI skeleton
- Python engine skeleton
- Access log parser MVP
- Collapsed profiler parser MVP
- Sample charts
- JSON result format
- English/Korean documentation and UI i18n foundation
- Engine-UI bridge — initially Electron IPC + Python CLI; replaced in the 2026-05 web pivot (T-206..T-209) with a FastAPI HTTP boundary (`/api/...`) + in-process analyzer dispatch
- Explicit Python runtime dependencies and CLI entry point
- Parser diagnostics for malformed records
- Encoding fallback correctness
- Type-specific `AnalysisResult` contracts for Access Log and Profiler
- Focused parser, utility, and JSON exporter tests

## Phase 2: Report-ready Charts

- Chart Studio
- Theme editor
- ECharts 6 upgrade evaluation
- Dark mode and dynamic chart themes
- Broken-axis and distribution chart options
- PNG/SVG export
- CSV export
- Chart Studio template preview/edit MVP
- Access log advanced statistics
- Access log diagnostic findings beyond raw charts
- Profiler flamegraph drill-down, Jennifer CSV import, and execution breakdown
- Custom regex parser
- Report label language toggle

## Phase 3: JVM Diagnostics and Distribution

- GC log analyzer MVP
- Java thread dump analyzer MVP
- Java exception analyzer MVP
- JFR recording parser design and feasibility spike
- Timeline correlation
- Electron supported-version upgrade
- Electron + PyInstaller packaging spike

## Phase 4: Multi-runtime and Observability Inputs

- Timeline correlation `AnalysisResult` design
- JFR recording parser design using the JDK `jfr` command spike path
- OpenTelemetry log input design with trace/span context mapping
- Node.js log and stack analyzer
- Python traceback analyzer
- Go panic/goroutine analyzer
- .NET exception/IIS analyzer
- OpenTelemetry JSONL log analyzer and cross-service trace correlation MVP
- OpenTelemetry parent-span service path analysis and failure propagation
- Broader OpenTelemetry envelope ingestion and span timing correlation
- Cross-evidence timeline correlation across access logs, GC, profiler, thread, JFR, and OTel evidence

## Phase 5: Report Automation

- Before/after diff
- HTML report generation
- Portable static HTML report MVP for `AnalysisResult` and parser debug JSON
- Static HTML flamegraph rendering for profiler result JSON
- PowerPoint export
- Minimal PowerPoint `.pptx` report MVP
- Executive summary generator
- AI-assisted interpretation, optional and evidence-bound
- Optional local LLM/Ollama interpretation with validated evidence references
- AI interpretation hardening: canonical `evidence_ref` grammar, `InterpretationResult` contract, runtime validator, prompt-injection defense, local-only runtime policy, provenance UI, and evaluation gates

## Phase 6: Industry-tool parity (post-0.2.0-beta)

Driven by gap analysis against TDA (`C:\workspace\tda-main`) for thread
dumps and async-profiler for profiler workflows. Each milestone bundles
engine + UI work.

### Profiler (M1–M4 — completed 2026-04/05)

- **M1 — JFR first class.** Binary `.jfr` auto-converted via the JDK
  `jfr` CLI (PATH / `JAVA_HOME` / `ARCHSCOPE_JFR_CLI`); multi-event
  mode filter (`cpu` / `wall` / `alloc` / `lock` / `gc` /
  `exception` / `io` / `nativemem`); time-range filter (ISO,
  `HH:MM:SS`, relative `+30s` / `-2m` / `500ms`); thread-state filter;
  min-duration filter.
- **M2 — Differential flame + display options.** Two-side diff with
  divergent red/blue gradient on a normalized total comparison; flame
  display toolbar (highlight regex, simple class names, normalize
  lambdas, dotted package, icicle inverted view, min-width
  simplification).
- **M3 — Heatmap + per-thread isolation.** 1D wall-clock density
  strip with drag-to-select that auto-fills the next analyze run's
  time-range filter; per-thread filter dropdown for `-t` collapsed
  output that re-roots the flame on a single thread without a server
  round-trip; min-width simplification finalized.
- **M4 — pprof export + tree view + native-mem leak + JFR
  enhancements.** Hand-rolled minimal protobuf encoder for pprof
  (gzipped, no protobuf runtime dependency, ready for
  Pyroscope / Speedscope / `go tool pprof`); hierarchical expandable
  tree-view table; native-memory leak detection (alloc/free pairing
  with tail-ratio cutoff, byte-weighted flame); recent-files panel
  (localStorage) for continuous diff sessions.

### Thread dumps — TDA hardening (in progress)

Many of the items below were delivered by Codex in a single batch
(commit `e6e6f48`) and have since been extended:

- Virtual-thread carrier-pinning detector
  (`VIRTUAL_THREAD_CARRIER_PINNING`) — flags Loom carrier threads
  pinned because the virtual thread holds a monitor.
- SafeMemoryReclamation / zombie thread detector
  (`SMR_UNRESOLVED_THREAD`).
- Lock-contention owner/waiter graph + DFS deadlock cycle detection.
- Heuristic findings: `THREAD_CONGESTION_DETECTED`,
  `EXTERNAL_RESOURCE_WAIT_HIGH`, `LIKELY_GC_PAUSE_DETECTED`,
  `GROWING_LOCK_CONTENTION`.
- 9-variant parser registry: `java_jstack` (incl. JDK 21+
  no-`nid` form), `java_jcmd_json`, `go_goroutine`, `python_pyspy`,
  `python_faulthandler`, `python_traceback`, `nodejs_diagnostic_report`,
  `nodejs_sample_trace`, `dotnet_clrstack`,
  `dotnet_environment_stacktrace`. UTF-16 / BOM auto-detection.
- Multi-file picker (drag a folder of dumps in one shot).
- JVM signals tab — Carrier-pinning / SMR / Native methods / Class
  histogram sub-tabs, plus a Dump overview card.

### Access log overhaul (completed 2026-05)

- Per-URL stats sortable by count / avg / p95 / total bytes / errors.
- Static / API request classification by file extension and
  well-known asset paths, with mix percentages on every URL row.
- p50 / p90 / p95 / p99 percentiles in the summary and a per-minute
  percentile timeline.
- Throughput (req/s, bytes/s) summary + per-minute series.
- HTTP status family + top status code breakdown, per-minute
  status-class timeline that highlights any minute ≥ 50 % error rate.
- New findings: `SLOW_URL_P95`, `ERROR_BURST_DETECTED`.

### GC log deep-dive (completed 2026-05)

- JVM Info card extracted from the recording header (Version, CPUs,
  Memory, Heap min/initial/max/region, Parallel & Concurrent workers,
  Compressed Oops, NUMA, Pre-touch, Periodic GC, full CommandLine
  flags) with a worker-vs-CPU mismatch warning banner.
- 9 toggleable heap series (Heap before/after/committed, Young
  before/after, Old before/after, Metaspace before/after) with
  optional Pause overlay on a right axis.
- Drag-rectangle zoom (visible blue selection rect) and series-level
  point decimation (max 2 000 / series) for huge logs.

### Windows desktop hardening

- Windows installer (NSIS) + portable zip via Electron, with the
  Python engine PyInstaller-bundled inside.
- ESM `__dirname` fix + static imports in the Electron main process.
- `apiBase` helper resolved from `window.archscope.engineUrl` (set in
  the preload) so the renderer running from `file://` reaches the
  bundled engine at `127.0.0.1:8765`.
- Pretendard Variable bundled inside the renderer so Korean text no
  longer falls back to Malgun Gothic.
- Page zoom locked at 1.0 to keep chart axes stable.

### Future candidates (not yet committed)

- Continuous-session timeline that joins access-log, GC, thread-dump,
  and JFR evidence on a shared time axis.
- async-profiler 3.x packed-binary HTML support.
- Heap dump / `.hprof` analysis remains explicitly out of scope.
