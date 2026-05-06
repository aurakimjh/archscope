# ArchScope Architecture

ArchScope is a local-first application architecture diagnostic and
reporting toolkit. Its core responsibility is to turn raw operational
evidence into normalized `AnalysisResult` JSON and report-ready
visualizations — without sending data to any third-party service.

## Web platform pivot — design decisions (T-206)

Decided **2026-05-06**. **Supersedes T-001** (the original "Electron +
`child_process.execFile` IPC" decision). Locks in the directional choices
that the rest of Phase 1 (T-207–T-209) builds on.

### Runtime

- **FastAPI + uvicorn**, in-process analyzer dispatch (no subprocess).
  Each analyzer is invoked as a normal Python function call; isolating
  it inside a subprocess is rejected because the engine is already
  in-language and the IPC boundary T-001 introduced provided no
  durability win.
- Programmatic `archscope serve` console entry point (T-208) starts
  uvicorn on `127.0.0.1:8765` by default and opens the system browser
  on first start.

### Transport

| Channel | Carries | Why |
|---|---|---|
| **HTTP** (`/api/...`) | Analyzer execute, settings, file dialog, exports, demo runner, file streaming. | Synchronous request/response is the simplest fit for analyzer dispatch and matches the existing client surface in `apps/frontend/src/api/`. |
| **WebSocket** (`/ws/progress`) | Engine progress events, cancellation signals, parser-debug-log streaming. | Long-running analyzers (multi-dump correlation, large GC logs) need to push intermediate state without polling, and the renderer needs a fire-and-forget cancel that's symmetric with the existing Wails track (T-240f). Single-process cancel uses the same task-registry pattern: server keys cancel channels by request id and emits `progress` / `done` / `cancelled` JSON frames. |

### File selection

- **Default — server-side absolute paths via `POST /api/files/select`.**
  The server pops an OS file dialog (the engine binds `127.0.0.1` so
  this is local-only by definition) and returns the absolute path;
  subsequent analyzer calls reference that path directly. Avoids the
  multipart upload round-trip for the common case where the user is on
  the same machine as the engine.
- **Fallback — browser multipart upload via `POST /api/upload`.** Already
  implemented; writes to `~/.archscope/uploads/<uuid>/<orig>`. Kicks in
  when the engine is reachable from a non-localhost origin or when the
  browser sandbox blocks the dialog endpoint.

### Packaging (T-208 directional)

- Single top-level Python distribution `archscope` exposing an
  `archscope` console script. The script wraps uvicorn so
  `pip install archscope && archscope serve` is the entire install path.
- The built React bundle from `apps/frontend/dist/` ships as wheel
  package data, resolved at runtime through `importlib.resources` —
  no copy step at install time, no separate static-file env var.
- The existing Electron desktop shell (`apps/desktop/`) is retired in
  T-207; its files are removed and the React shell is consolidated
  into `apps/frontend/`. The Wails v3 native profiler at
  `apps/profiler-native/` is unaffected (it's a separate track —
  decision recorded under T-242).

### CSP / CORS posture

- **CORS** — `allow_origins=["http://127.0.0.1:5173"]` for the Vite dev
  server. Production serves the React bundle from the same FastAPI
  origin so CORS is effectively unused at runtime. `--no-dev-cors`
  disables the dev allowlist entirely for hardened deployment.
- **CSP** — `default-src 'self'; img-src 'self' data:;
  style-src 'self' 'unsafe-inline'; script-src 'self';
  connect-src 'self' ws://127.0.0.1:8765`. The `connect-src` ws://
  entry is what the renderer needs to subscribe to `/ws/progress`.
  `style-src 'unsafe-inline'` stays for shadcn/ui CSS variables;
  nonce-based hardening is tracked separately under T-052/T-071.

### Apps directory layout (post-T-207)

```text
apps/
├ frontend/         # React shell — single source of truth for the web UI
├ profiler-native/  # Wails v3 native profiler (decided 2026-05-05, T-240a)
└ desktop/          # Removed by T-207
```

The historical `apps/desktop/electron/`, `tsconfig.electron.json`,
`electron-builder` config, `release/`, and `dist-electron/` are
deleted. The new top-level `archscope` Python distribution
(package data + console script) lands at the repo root via T-208.

## Product positioning

ArchScope is positioned as a **privacy-first local professional
diagnostic workbench**:

- the convenience of a browser-based UI,
- the local/offline safety of a desktop analyzer,
- modern report-ready visualizations (D3 + ECharts + Canvas), and
- a normalized evidence contract that supports five runtimes.

The product direction is not to become a general log viewer or a full
observability backend. ArchScope stays focused on turning offline
operational evidence into architecture diagnosis and report artifacts.

## System flow

```text
Raw Data
  → Parsing (per-format parsers + plugin registry)
  → Analysis / Aggregation (per-domain analyzers + multi-dump correlator)
  → Visualization (browser: D3 / Canvas / ECharts)
  → Report-ready Export (HTML / PowerPoint / diff)
```

## Runtime topology

```text
┌────────────────────────────────────────────────────────────────┐
│  Browser (React 18 + Vite + Tailwind v4 + shadcn/ui)           │
│   • AppShell  (TopBar + collapsible Sidebar + Tabs)            │
│   • httpBridge mounts window.archscope onto the FastAPI API    │
│   • Charts: D3 Flame / Canvas Flame / D3 Timeline / D3 Bar /   │
│            legacy ECharts panels                               │
│   • Image export: html-to-image + native canvas.toDataURL()    │
└──────────────────────────┬─────────────────────────────────────┘
                           │  fetch /api/...
                           ▼
┌────────────────────────────────────────────────────────────────┐
│  FastAPI server (`archscope-engine serve`)                     │
│   • POST /api/upload                  multipart upload         │
│   • POST /api/analyzer/execute        dispatcher (per type)    │
│   • POST /api/analyzer/cancel         no-op in single-process  │
│   • POST /api/export/execute          html / pptx / diff       │
│   • GET  /api/demo/list, POST /api/demo/run                    │
│   • GET  /api/files?path=…            stream artifacts back    │
│   • GET/PUT /api/settings             ~/.archscope/settings    │
│   • GET  /                            static React build       │
└──────────────────────────┬─────────────────────────────────────┘
                           │  in-process call (no subprocess)
                           ▼
┌────────────────────────────────────────────────────────────────┐
│  archscope_engine (pure Python)                                │
│                                                                │
│   parsers/                                                     │
│     access_log_parser, collapsed_parser, jennifer_csv_parser,  │
│     svg_flamegraph_parser, html_profiler_parser,               │
│     gc_log_parser + gc_log_header (JVM Info),                  │
│     jfr_recording (binary `.jfr` → JSON via JDK `jfr` CLI),    │
│     jfr_parser (existing JSON path),                           │
│     exception_parser, otel_parser,                             │
│     thread_dump/                                               │
│       registry.py     ← format-id, can_parse(head), parse(path)│
│       java_jstack.py  ← + AOP / network-IO + JDK 21+ no-`nid`  │
│       java_jcmd_json.py ← jcmd JSON.thread_dump_to_file        │
│       go_goroutine.py ← + framework cleanup + state inference  │
│       python_dump.py  ← py-spy / faulthandler                  │
│       python_traceback.py ← Thread ID + File "...", line N     │
│       nodejs_report.py← diagnostic-report JSON + libuv state   │
│       nodejs_sample_trace.py ← Sample # + at fn(file:line:col) │
│       dotnet_clrstack ← + async state machine cleanup          │
│       dotnet_environment_stacktrace ← Environment.StackTrace   │
│                                                                │
│   analyzers/                                                   │
│     access_log_analyzer, profiler_analyzer (collapsed/SVG/     │
│     HTML/Jennifer), profiler_diff (red=slower / blue=faster),  │
│     native_memory_analyzer (alloc/free pairing),               │
│     gc_log_analyzer, jfr_analyzer,                             │
│     thread_dump_analyzer (single-dump, JVM only),              │
│     multi_thread_analyzer (LONG_RUNNING_THREAD,                │
│         PERSISTENT_BLOCKED_THREAD, LATENCY_SECTION_DETECTED,   │
│         GROWING_LOCK_CONTENTION, THREAD_CONGESTION_DETECTED,   │
│         EXTERNAL_RESOURCE_WAIT_HIGH, LIKELY_GC_PAUSE_DETECTED, │
│         VIRTUAL_THREAD_CARRIER_PINNING, SMR_UNRESOLVED_THREAD),│
│     lock_contention_analyzer (owner/waiter graph, DFS deadlock),│
│     thread_dump_to_collapsed,                                  │
│     exception_analyzer, runtime_analyzer, otel_analyzer,       │
│     ai_interpretation, profiler_breakdown, profiler_drilldown  │
│                                                                │
│   exporters/                                                   │
│     json_exporter, html_exporter, pptx_exporter, report_diff,  │
│     pprof_exporter (hand-rolled minimal protobuf, no deps)     │
│                                                                │
│   models/                                                      │
│     AnalysisResult contract (single transport boundary),       │
│     FlameNode (with optional metadata: {a, b, delta} for diff),│
│     ThreadSnapshot + ThreadDumpBundle + ThreadState,           │
│     StackFrame, ExceptionRecord, GcEvent, …                    │
│                                                                │
│   web/server.py     ← FastAPI factory + analyzer dispatcher    │
│   cli.py            ← Typer commands (one per analyzer + serve)│
└────────────────────────────────────────────────────────────────┘
```

## Components

### Browser app (`apps/frontend/`)

React 18 served as a static bundle by FastAPI (or by Vite dev server
during development). The same bundle is also packaged inside the
Electron desktop shell at `apps/desktop/`, where it is loaded via
`file://` and an `apiBase` helper that resolves to the bundled engine
at `127.0.0.1:8765`. The `httpBridge` (`src/api/httpBridge.ts`) mounts
the same `window.archscope.*` surface the legacy Electron build used,
but every call is now an `fetch()` against the FastAPI engine. Pages
never import a parser; they only render normalized `AnalysisResult`
JSON.

The chart layer is split:

- **D3** — the new charts (`D3FlameGraph`, `D3TimelineChart`,
  `D3BarChart`) plus the Canvas-backed `CanvasFlameGraph` that takes
  over when a flame tree exceeds 4 000 nodes.
- **ECharts** — legacy panels still used by the access-log request-rate
  trend and the profiler breakdown donut/bar. `ChartPanel.tsx` wraps
  them with the same shadcn-styled toolbar so the per-chart export
  dropdown works uniformly.

The shell uses Tailwind v4 with the `@tailwindcss/vite` plugin and the
shadcn/ui token sheet (light/dark CSS variables).
`ThemeProvider` toggles `.dark` on `<html>` and persists the choice in
`localStorage`.

### FastAPI engine (`engines/python/archscope_engine/web/`)

`web.server.create_app()` wires:

- `/api/upload` — multipart, writes to `~/.archscope/uploads/<uuid>/`
  and returns the server-side path that subsequent analyzer calls use.
- `/api/analyzer/execute` — single dispatcher that switches on `type`
  (`access_log`, `profiler_collapsed`, `profiler_diff`,
  `profiler_export_pprof`, `gc_log`, `thread_dump`, `thread_dump_multi`,
  `thread_dump_to_collapsed`, `exception_stack`, `jfr_recording`,
  `flamegraph_svg`, `flamegraph_html`).
  Calls the analyzer **in-process** (no subprocess) and returns the
  full `AnalysisResult` JSON. CORS is wide-open
  (`allow_origins=["*"]`) since the engine binds `127.0.0.1` and the
  bundled Electron build loads the UI from `file://`.
- `/api/export/execute` — HTML / PPTX / before-after diff exports.
- `/api/demo/list` and `/api/demo/run` — demo-site fixture runner.
- `/api/files?path=…` — streams arbitrary local files back so the UI
  can open exported reports / artifacts.
- `/api/settings` — JSON object persisted to
  `~/.archscope/settings.json`.
- `/` — static React build (when `--static-dir` is set).

CORS allow-list is enabled by default for `http://127.0.0.1:5173` so
the Vite dev server can talk to the engine; `--no-dev-cors` disables
it for production-style serving.

### Engine package (`engines/python/archscope_engine/`)

Plain Python, no GUI dependencies. Layered:

- **`parsers/`** — read raw files into typed records. The thread-dump
  family is plugin-based: each runtime registers a
  `ThreadDumpParserPlugin` with `format_id`, `can_parse(head: str)`,
  and `parse(path) -> ThreadDumpBundle`. The registry probes the first
  4 KB of every input and dispatches; mixing formats in a single
  multi-dump request raises `MixedFormatError` unless `format_override`
  is passed.
- **`analyzers/`** — turn typed records into `AnalysisResult`s. The
  multi-dump correlator (`multi_thread_analyzer`) is intentionally
  language-agnostic; per-runtime enrichment (CGLIB cleanup, network/IO
  state inference, async state-machine demangling, …) lives inside the
  parser plugin so the correlator only ever consumes `ThreadState` enum
  values.
- **`exporters/`** — `AnalysisResult` → JSON / HTML / PPTX / diff
  artifacts.
- **`models/`** — shared dataclasses. `AnalysisResult` is the single
  transport boundary between engine and UI.

### `AnalysisResult` contract

Every analyzer emits the same envelope:

```text
AnalysisResult {
  type: str                  # e.g. "profiler_collapsed", "thread_dump_multi"
  source_files: list[str]
  created_at: str            # ISO 8601
  summary: dict              # scalar metrics for the metric-card row
  series: dict               # arrays for the chart panels
  tables: dict               # rows for shadcn / D3 tables
  charts:  dict              # raw chart data (e.g. flamegraph trees)
  metadata: {
    parser: str,
    schema_version: "0.2.0",
    diagnostics: ParserDiagnostics,
    findings?: list[Finding],
    drilldown_current_stage?: …,
    detected_html_format?: …, ai_interpretation?: …,
  }
}
```

The browser only renders this shape. New analyzers only need to fit
the contract — no UI plumbing per analyzer.

## Storage and on-disk layout

| Path | Owner | Purpose |
| --- | --- | --- |
| `~/.archscope/uploads/<uuid>/<orig>` | upload endpoint | multipart uploads — input for analyzer dispatch |
| `~/.archscope/uploads/collapsed/<uuid>.collapsed` | thread→flamegraph converter | server-side conversion output |
| `~/.archscope/settings.json` | settings endpoint | engine path / chart theme / locale |
| `<repo>/apps/frontend/dist/` | Vite | static React build served by `--static-dir` (also bundled inside Electron) |
| `<repo>/apps/desktop/dist/` | electron-builder | NSIS installer + portable zip output |
| `<repo>/engines/python/dist/` | PyInstaller | `archscope-engine` single-binary embedded in the Electron package |
| `<repo>/archscope-debug/` | parser debug logs | redacted parse-error context for support |

## What is intentionally out of scope

- **Heap dump / `.hprof` analysis** — out of scope. ArchScope is the
  right tool to ask *why* threads are stuck, not *where allocations
  live*.
- **Live agent / runtime monitoring** — ArchScope ingests captured
  artifacts only.
- **Multi-tenant SaaS / authentication** — the engine binds
  `127.0.0.1` by default and has no auth layer. `--host 0.0.0.0` is
  meant for trusted LANs only.
- **async-profiler 3.x packed-binary HTML** — supported HTML variants
  are inline-SVG and the older embedded-tree JS form. For 3.x, emit
  `--format svg` from `asprof` instead.

For the per-language thread-dump matrix and the detection rules see
[`MULTI_LANGUAGE_THREADS.md`](MULTI_LANGUAGE_THREADS.md).
