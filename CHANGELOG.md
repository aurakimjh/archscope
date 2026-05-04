# Changelog

All notable changes to ArchScope are tracked here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[0.2.0-alpha]: https://github.com/aurakimjh/archscope/releases/tag/v0.2.0-alpha
