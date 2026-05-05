# Changelog

All notable changes to ArchScope are tracked here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.0-rc1] — 2026-05-05

The post-0.2.0-beta cycle landed a major analyzer overhaul: profiler
gained JFR-first-class ingest, differential analysis, wall-clock heatmap,
pprof export, native-memory leak detection, tree view, and per-thread
isolation. Thread-dump analysis adopted the TDA-inspired feature set
(virtual-thread carrier-pinning, SMR/zombie threads, class histogram,
JCmd JSON, native-method threads, heuristic findings, embedded G1 heap
block) and accepts more dump variants. Access-log analysis was
overhauled with per-minute percentile timeline, throughput, static-vs-API
classification, and a sortable per-URL stats table. Windows desktop
build now ships a working binary with Pretendard typography, locked
page zoom, and a host of UX fixes.

### Windows desktop hardening

- Fixed the v0.2.0-beta Electron build that wouldn't open: `__dirname`
  / `require()` ESM polyfills in `electron/main.ts`, preload now
  exposes `archscope.engineUrl` so renderer requests resolve under
  `file://`, and FastAPI CORS is permissive enough for the Electron
  origin.
- Locked page zoom factor to `1.0` and intercepts Ctrl+(+/-/0) so OS
  zoom no longer scales the chart axes.
- Bundled **Pretendard Variable** font (~2 MB woff2) so Korean +
  English text renders identically on every machine instead of falling
  back to Malgun Gothic.
- Multi-file selection added to `FileDock` (opt-in via `multiple`);
  Thread Dump page accepts an entire folder of dumps in a single drop.

### Profiler — JFR first-class (M1)

- New `parsers/jfr_recording.py` auto-detects binary `.jfr` (FLR\0
  magic) and shells out to the JDK `jfr` CLI (PATH, `JAVA_HOME`, or
  `ARCHSCOPE_JFR_CLI`); legacy `jfr print --json` path unchanged.
- Multi-event mode filter (`cpu` / `wall` / `alloc` / `lock` / `gc` /
  `exception` / `io` / `nativemem` / `all`) chooses which JFR event
  types the analyzer keeps. Available modes are auto-detected from
  the recording so the UI dims modes with no data.
- Time-range filter (ISO 8601, `HH:MM:SS`, or relative `+30s` / `-2m`
  / `500ms`) anchored to the recording start/end.
- Thread-state filter (`RUNNABLE`, `BLOCKED`, etc.) and minimum
  duration filter — useful for `MethodTrace ≥ N ms` style queries.
- Wall-clock heatmap strip: adaptive bucket size (0.5 s … 30 min)
  based on recording duration. Frontend `D3HeatmapStrip` renders the
  grid and lets the user drag to set From/To inputs.

### Profiler — differential (M2)

- New `analyzers/profiler_diff.py`. Walks two collapsed / SVG / HTML /
  Jennifer-CSV inputs into a unified flame tree where each node carries
  `metadata = {a, b, delta, delta_ratio}`. Optional total-normalization
  so different sample counts don't dominate. Result includes
  `biggest_increases` / `biggest_decreases` tables.
- `D3FlameGraph` learned `colorMode="diff"` (red/blue divergent
  gradient on `metadata.delta`), `highlightPattern` (regex outline +
  dim non-matches), and display options (`simplifyNames`,
  `normalizeLambdas`, `dottedNames`).
- ProfilerAnalyzerPage gained a **Diff** tab with two FileDocks +
  format selectors, normalize toggle, the diff flame, and colored
  gain/loss tables.

### Profiler — wall-clock anomaly explorer (M3)

- `D3FlameGraph` `inverted` (icicle layout, leaves at top) and
  `minWidthPercent` (skip frames below X% of width) for visual
  simplification of huge profiles.
- Engine: per-thread detection on collapsed `-t` output (≥ 80% of
  stacks carrying a `[Thread]` prefix triggers
  `series.threads = [{name, samples, ratio}]`).
- ProfilerAnalyzerPage `ThreadFilter` dropdown re-roots the displayed
  flame on a single thread's bracket subtree without a server round-trip.

### Profiler — export, tree view, native-memory (M4)

- `exporters/pprof_exporter.py` — hand-rolled minimal protobuf encoder.
  `POST /api/profiler/export-pprof` streams a gzipped pprof; the UI's
  **Export pprof** button downloads `*.pb.gz` ready for Pyroscope /
  Speedscope / `go tool pprof`.
- `FlameTreeTable` component: hierarchical expandable view of the
  flame tree sorted by samples desc with self / inclusive / percent
  columns. Honors the same display + highlight options. Surfaced as
  a new **Tree** tab on the profiler page.
- `analyzers/native_memory_analyzer.py` pairs JFR
  `NativeMemoryAllocation` / `NativeMemoryFree` events by address and
  reports unfreed allocations as a byte-weighted flame; `tail_ratio`
  option (default 10%) ignores allocations made in the last N% of the
  recording. JfrAnalyzerPage gains a **Native memory** tab.
- ProfilerAnalyzerPage diff tab now persists the last 10 analyzed
  files to `localStorage` and shows a **Recent profile files** panel
  (click = baseline, Shift+click = target) for continuous-session
  workflows.

### Thread dump — TDA-inspired hardening

- Java jstack parser: virtual-thread carrier-pinning detection
  (`Carrying virtual thread` + non-`VirtualThread.run`/`ForkJoinPool`
  application frames) and SMR (Safe Memory Reclamation) diagnostics —
  unresolved/zombie thread markers surface as findings.
- New `parsers/thread_dump/java_jcmd_json.py` — parses `jcmd <pid>
  Thread.dump_to_file -format=json` output.
- Class histogram parsing (`-XX:+PrintClassHistogram` rows in the
  trailing block); top-N classes exposed as a table.
- Loose detection variants accepted: jstack output without `Full
  thread dump` banner or `nid=` field (JDK 21+); plain
  `traceback.format_stack` text (`Thread ID: <n>`); Node.js
  `Sample #N\Error\  at fn(file:line:col)`; CLR
  `Environment.StackTrace` snapshots without `OS Thread Id:`.
- New heuristic findings (multi_thread_analyzer):
  `THREAD_CONGESTION_DETECTED` (>10% WAITING/LOCK_WAIT/BLOCKED),
  `EXTERNAL_RESOURCE_WAIT_HIGH` (>25% TIMED_WAITING),
  `LIKELY_GC_PAUSE_DETECTED` (>50% blocked on monitors with no app
  owner). Plus `VIRTUAL_THREAD_CARRIER_PINNING` and
  `SMR_UNRESOLVED_THREAD` per detected entry.
- Native-method thread roll-up
  (`tables.native_method_threads`) and embedded G1 heap block parser
  (`{Heap before GC ...}` → `metadata.jvm_heap_block` with heap total /
  used / region / young / metaspace).
- ThreadDumpAnalyzerPage gains a **Dump overview** card (8 metrics)
  and a **JVM signals** tab with sub-tabs for Carrier pinning / SMR /
  Native methods / Class histogram.
- UTF-16 / BOM detection added to `common/file_utils.py`.

### Access log — analyzer overhaul

- Summary expanded from 5 to 22 fields: p50 / p90 / p99 latency,
  total bytes, wall time, avg req/s, avg bytes/s,
  static_count / api_count / static_bytes / api_bytes /
  static_avg_response_ms / api_avg_response_ms /
  api_p95_response_ms, recording bounds, unique URI count.
- New time series: `p50/p90/p99_response_time_per_minute`,
  `status_class_per_minute` (2xx/3xx/4xx/5xx/other),
  `error_rate_per_minute`, `bytes_per_minute`,
  `throughput_per_minute` (req/s + bytes/s), `method_distribution`,
  `request_classification`.
- New per-URL `url_stats` table: count / avg / p50 / p90 / p95 / p99 /
  total bytes / error count / per-status mix; multiple "top N"
  derivative tables (count, avg, p95, bytes, errors).
- Static-vs-API classification by file extension
  (`.js/.css/.png/.jpg/.woff2/...`) and well-known asset paths
  (`/static/`, `/assets/`, `/dist/`, `/img/`, ...).
- New findings: `SLOW_URL_P95` (≥ 1 s p95 with ≥ 5 samples — replaces
  `SLOW_URL_AVERAGE`); `ERROR_BURST_DETECTED` (a single minute ≥ 50%
  error rate with ≥ 5 requests). Schema bumped to 0.2.0.
- Frontend AccessLogAnalyzerPage: 12-metric summary, **URLs** tab
  (sortable by count / avg / p95 / total bytes / errors with
  static / api / all filter and per-row 2·3·4·5xx mix), **Status &
  errors** tab (status families, top status codes, per-minute error
  timeline highlighting any minute ≥ 50% error rate).

### GC log — analyzer deep-dive

- New `gc_log_header.py` extracts JVM/system metadata from the
  recording header: Version, CPUs, Memory, Heap min/initial/max/region,
  Parallel/Concurrent workers, Compressed Oops, Periodic GC,
  CommandLine flags. Surfaced via a new **JVM Info** tab (set as
  default).
- Heap chart: 7 toggleable series (Heap before/after, Heap committed,
  Young before/after, Old before/after, Metaspace before/after);
  optional Pause overlay on a right-axis; data-availability check
  greys out series with no data.
- `D3TimelineChart`: drag-to-zoom-rectangle on the plot, wheel in/out,
  double-click reset, point decimation (max 2 000 per series),
  rAF-throttled transform updates, dynamic x-axis tick format.
- Engine emits new GC series (`young_before_mb`, `old_before/after_mb`
  derived from `heap - young`, `heap_committed_mb`,
  `metaspace_before/after_mb` via unified-format metaspace line
  buffering and G1-legacy `[Metaspace: ...]` parsing).

### Exception analyzer

- New dedicated page replacing the generic placeholder. Paginated +
  filterable event table, click-row Sheet popup with full message,
  signature, stack, and metadata. Top-types chart shows simple class
  names (`NullPointerException`) with full FQN on hover; summary
  card no longer overflows on long FQNs.
- Top stack signatures table with click-to-detail Sheet.

### UI / UX

- "Diagnostics" tab renamed **Parser Report** / **파서 리포트** so
  users don't expect analytical insight from a parser-coverage panel.
- Font system: `font-sans` resolves to Pretendard Variable in both the
  Tailwind v4 `@theme` block and the design-system `--font-sans`,
  guaranteeing consistent rendering in Electron's Chromium.
- DashboardPage hardened against shape-mismatched legacy data (filters
  out non-`access_log`/`profiler_collapsed` types so a stale GC result
  no longer crashes the dashboard).

## [0.2.0-beta] — 2026-05-04

Promotion of the 0.2.0-alpha line to beta. Same feature surface — no
behavior changes, no API changes. Version label propagated through the
Python engine, FastAPI server, React UI, and Electron desktop bundle so
shipped binaries advertise the new tag.

## [0.2.0-alpha] — 2026-05-04

The 0.2.0-alpha release retires the Electron desktop shell, introduces a
local-first FastAPI web app, and ships a complete redesign of the UI
plus six-runtime thread-dump analysis with lock-contention awareness.
Phases 1–7 of the 2026-Q2 cycle are all included.

### Phase 1 — Web pivot (was Electron)

- **Removed** the entire Electron + IPC + PyInstaller-sidecar
  pipeline. `apps/desktop/electron/`, `tsconfig.electron.json`,
  `dist-electron/`, `electron-builder` configuration, and the legacy
  Playwright Electron smoke test are gone.
- **New FastAPI engine layer** (`archscope_engine.web.server`) exposes
  every analyzer, exporter, demo, and settings flow through HTTP under
  `/api/...`, and serves the React build from `/`. Analyzers run
  in-process — no subprocess fan-out.
- **`archscope-engine serve`** CLI subcommand
  (`--host`, `--port`, `--reload`, `--static-dir`, `--no-dev-cors`).
- **HTTP bridge** in the React app (`src/api/httpBridge.ts`) keeps the
  same `window.archscope.*` surface the legacy IPC bridge had, but
  every call is now `fetch('/api/...')`.
- **`scripts/serve-web.sh`** one-shot helper: builds the UI and starts
  the engine. Replaces `build-win.ps1`.

### Phase 2 — Tailwind v4 + shadcn/ui shell

- **Tailwind CSS v4** + `@tailwindcss/vite` + the shadcn/ui token
  sheet. Light/dark/system theme via `ThemeProvider` (persisted to
  `localStorage`).
- **Base shadcn primitives** under `components/ui/`: `button`, `card`,
  `dropdown-menu`, `input`, `separator`, `sheet`, `tabs`, `tooltip`.
- **New AppShell** with a slim TopBar (brand · search · theme ·
  language · settings), a collapsible icon-rail Sidebar, and a sticky
  `FileDock` upload zone (drag-and-drop + click-to-browse).
- **Page migrations** — every page swapped onto the new shell:
  Dashboard, Access Log, Profiler, GC, Threads, Exception, JFR, Demo,
  Export, Chart Studio, Settings.
- **Image export utility** (`lib/exportImage.ts`) with PNG 1×/2×/3×,
  JPEG 2×, SVG vector presets, plus a per-page **"Save all charts"**
  batch export (`lib/batchExport.ts`).
- **D3-based charts** — `D3FlameGraph`, `D3TimelineChart`,
  `D3BarChart`, all wrapped with `D3ChartFrame` (image-export
  dropdown). ECharts panels (legacy) keep working through the same
  `ChartPanel` wrapper.

### Phase 4 — Profiler SVG / HTML inputs

- **`parsers/svg_flamegraph_parser.py`** ingests FlameGraph.pl /
  async-profiler `-o svg` SVG files (auto-detects Brendan default vs.
  icicle layout); XXE-safe via `defusedxml`.
- **`parsers/html_profiler_parser.py`** detects inline-SVG HTML and
  the legacy embedded-tree async-profiler HTML; `UNSUPPORTED_HTML_FORMAT`
  diagnostic for everything else.
- **`profiler analyze-flamegraph-svg|html`** CLI commands and matching
  `flamegraph_svg`/`flamegraph_html` `profileFormat` values on the
  Profiler page; FileDock `accept` adapts to `.svg` / `.html,.htm`.

### Phase 5 — Multi-language thread-dump framework

- **`models/thread_snapshot.py`** — language-agnostic models:
  `ThreadState` (RUNNABLE / BLOCKED / WAITING / TIMED_WAITING /
  NETWORK_WAIT / IO_WAIT / LOCK_WAIT / CHANNEL_WAIT / DEAD / NEW /
  UNKNOWN), `StackFrame`, `ThreadSnapshot`, `ThreadDumpBundle`.
- **Plugin registry** (`parsers/thread_dump/registry.py`) — 4 KB
  header sniffing, format override, mixed-format guard.
- **Six parser plugins** auto-registered with `DEFAULT_REGISTRY`:
  - `java_jstack` — proven jstack parser + per-language enrichment
    (CGLIB / `$$EnhancerByCGLIB$$<hex>` / `$$Proxy<digits>` cleanup;
    `EPoll.epollWait` / `socketRead0` / `FileInputStream.read*` →
    NETWORK_WAIT / IO_WAIT promotion).
  - `go_goroutine` — `runtime.Stack` / panic / debug.Stack;
    framework cleanup (gin / Echo / Chi / fiber receivers, anonymous
    `.func1.func2` chains); state inference (`gopark` /
    `runtime.netpoll` / `sync.(*Mutex).Lock` → CHANNEL / NETWORK /
    LOCK_WAIT).
  - `python_pyspy` — `Process N:` + `Python vX.Y` py-spy banner.
  - `python_faulthandler` — `Thread 0x… (most recent call first):`.
  - `nodejs_diagnostic_report` — `process.report.writeReport()`
    JSON; libuv-driven NETWORK_WAIT / IO_WAIT inference.
  - `dotnet_clrstack` — `dotnet-dump analyze` clrstack output;
    `<…>d__N.MoveNext` async-state-machine cleanup; `Monitor.Enter`
    → LOCK_WAIT, `Socket.Receive` → NETWORK_WAIT.
- **Multi-dump correlator** (`analyzers/multi_thread_analyzer.py`)
  emits language-agnostic findings: `LONG_RUNNING_THREAD`,
  `PERSISTENT_BLOCKED_THREAD`, `LATENCY_SECTION_DETECTED`.
- **CLI**: `archscope-engine thread-dump analyze-multi --input <f>
  --input <f> ... --out <multi.json> [--format <id>]
  [--consecutive-threshold N]`.
- **FastAPI** `/api/analyzer/execute` accepts `type:
  "thread_dump_multi"` with `UNKNOWN_THREAD_DUMP_FORMAT` /
  `MIXED_THREAD_DUMP_FORMATS` error mapping.
- **UI** — bespoke multi-file ThreadDumpAnalyzerPage (cumulative file
  list, Format-override input, Findings / Charts / Per-dump / Threads
  tabs).
- **Bilingual docs**: [`docs/{en,ko}/MULTI_LANGUAGE_THREADS.md`](docs/en/MULTI_LANGUAGE_THREADS.md).

### Phase 6 — GC interactivity + thread→flamegraph + Canvas renderer

- **Interactive GC pause timeline** — `D3TimelineChart` opt-in
  `interactive` prop wires `d3-zoom` (1× – 80×) and `d3-brushX`. The
  GC page renders a 4-stat selection-summary card (count / avg / p95 /
  max pause) when the user brushes a window. Full-GC events expose
  hover payloads with `cause`, `pause_ms`, before/after/committed heap.
- **GC algorithm comparison tab** — per-`gc_type` pause statistics
  (count / avg / p95 / max / total ms) plus two horizontal D3 bar
  charts.
- **Thread → flamegraph batch converter**
  (`analyzers/thread_dump_to_collapsed.py` + CLI `thread-dump
  to-collapsed` + FastAPI `type: "thread_dump_to_collapsed"`). Drives
  the parser registry, applies per-language enrichment, and aggregates
  identical stacks across all input files into a FlameGraph-compatible
  collapsed file.
- **Canvas flamegraph** (`components/charts/CanvasFlameGraph.tsx`) —
  HiDPI-aware Canvas 2D paint of the `FlameGraphNode` tree;
  click-to-zoom, hover tooltip, dedicated "Save PNG" via
  `canvas.toDataURL()`. The Profiler page auto-switches to Canvas
  rendering when the flame tree has ≥ 4 000 nodes.

### Phase 7 — Lock contention analysis

- **`LockHandle(lock_id, lock_class)`** dataclass added to
  `models/thread_snapshot.py`; `ThreadSnapshot.lock_holds` and
  `lock_waiting` populated by `parsers/thread_dump/java_jstack.py`
  (`- locked <0x…>`, `- waiting to lock <0x…>`, `- waiting on <0x…>`,
  `- parking to wait for <0x…>`). Re-entrant locks collapse to
  held-only.
- **`analyzers/lock_contention_analyzer.py`** builds an owner ↔ waiter
  graph keyed by `lock_id`; emits `LOCK_CONTENTION_HOTSPOT` per
  top-N hot lock and `DEADLOCK_DETECTED` per simple cycle (DFS,
  canonical-rotation deduplicated).
- **`GROWING_LOCK_CONTENTION`** finding added to the multi-dump
  correlator — fires when a lock's waiter count strictly increases
  for ≥ N consecutive dumps.
- **CLI**: `archscope-engine thread-dump analyze-locks --input <f>
  ...`. **FastAPI**: `type: "thread_dump_locks"` dispatch on
  `/api/analyzer/execute`.
- **UI** — new "Lock Contention" tab on `ThreadDumpAnalyzerPage`
  (auto-fetched in parallel with the multi-dump analyzer): per-lock
  shadcn table, horizontal D3 bar chart ranking, dedicated red
  severity card per detected deadlock cycle (`T1 → T2 → T3 → T1` plus
  per-edge lock evidence).

### Documentation

- Top-level [`README.md`](README.md), [`README.en.md`](README.en.md),
  [`README.ko.md`](README.ko.md) rewritten for the web app +
  Phase 1–7 capabilities + CLI cheatsheet + ASCII architecture
  diagram.
- New bilingual user guide:
  [`docs/en/USER_GUIDE.md`](docs/en/USER_GUIDE.md) /
  [`docs/ko/USER_GUIDE.md`](docs/ko/USER_GUIDE.md).
- New bilingual reference:
  [`docs/en/MULTI_LANGUAGE_THREADS.md`](docs/en/MULTI_LANGUAGE_THREADS.md) /
  [`docs/ko/MULTI_LANGUAGE_THREADS.md`](docs/ko/MULTI_LANGUAGE_THREADS.md).
- [`ARCHITECTURE.md`](docs/en/ARCHITECTURE.md),
  [`PACKAGING_PLAN.md`](docs/en/PACKAGING_PLAN.md), and
  [`BRIDGE_POC_UX_FLOW.md`](docs/en/BRIDGE_POC_UX_FLOW.md) updated to
  describe the FastAPI HTTP boundary; legacy Electron content kept
  in `Historical:` sections for context.

### Build, test, packaging

- **Test suite**: 268 Python tests passing (was 155 at the start of
  this cycle). UI: `tsc --noEmit` and `vite build` clean; CSS ~67 KB
  / gz 12 KB, page bundle ~316 KB / gz 88 KB, charts chunk ~94 KB /
  gz 31 KB.
- **Engine deps**: FastAPI ≥ 0.110, uvicorn[standard] ≥ 0.27,
  python-multipart ≥ 0.0.9, defusedxml ≥ 0.7 added to
  `engines/python/setup.cfg`.
- **Frontend deps**: Tailwind v4 + `@tailwindcss/vite`,
  `tw-animate-css`, lucide-react, six Radix primitives,
  class-variance-authority, clsx, tailwind-merge, d3, recharts,
  html-to-image, @types/d3.

### Removed / superseded

- **Electron desktop shell** — `apps/desktop/electron/`,
  `tsconfig.electron.json`, `dist-electron/`, `electron-builder`
  config, the legacy Playwright Electron smoke test, and the
  `build-win.ps1` Windows installer script.
- **Legacy `apps/desktop/src/components/{Layout,Sidebar,FileDropZone}.tsx`**
  superseded by `AppShell` / `AppSidebar` / `FileDock`.

---

## [0.1.0] — 2026-04-29

Initial closed-development tag covering the original Electron + React
desktop application with the Phase 1–3 backlog (T-001…T-179).
Superseded by 0.2.0-alpha; no longer publicly distributed.

[0.2.0-beta]: https://github.com/aurakimjh/archscope/releases/tag/v0.2.0-beta
[0.2.0-alpha]: https://github.com/aurakimjh/archscope/releases/tag/v0.2.0-alpha
