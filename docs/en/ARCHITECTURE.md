# ArchScope Architecture

ArchScope is a local-first application architecture diagnostic and
reporting toolkit. Its core responsibility is to turn raw operational
evidence into normalized `AnalysisResult` JSON and report-ready
visualizations — without sending data to any third-party service.

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
│     gc_log_parser, jfr_parser, exception_parser, otel_parser,  │
│     thread_dump/                                               │
│       registry.py     ← format-id, can_parse(head), parse(path)│
│       java_jstack.py  ← + AOP / network-IO enrichment          │
│       go_goroutine.py ← + framework cleanup + state inference  │
│       python_dump.py  ← py-spy + faulthandler + enrichment     │
│       nodejs_report.py← diagnostic-report JSON + libuv state   │
│       dotnet_clrstack ← + async state machine cleanup          │
│                                                                │
│   analyzers/                                                   │
│     access_log_analyzer, profiler_analyzer (collapsed/SVG/     │
│     HTML/Jennifer), gc_log_analyzer, jfr_analyzer,             │
│     thread_dump_analyzer (single-dump, JVM only),              │
│     multi_thread_analyzer (LONG_RUNNING_THREAD,                │
│         PERSISTENT_BLOCKED_THREAD, LATENCY_SECTION_DETECTED),  │
│     thread_dump_to_collapsed,                                  │
│     exception_analyzer, runtime_analyzer, otel_analyzer,       │
│     ai_interpretation, profiler_breakdown, profiler_drilldown  │
│                                                                │
│   exporters/                                                   │
│     json_exporter, html_exporter, pptx_exporter, report_diff   │
│                                                                │
│   models/                                                      │
│     AnalysisResult contract (single transport boundary),       │
│     FlameNode, ThreadSnapshot + ThreadDumpBundle + ThreadState,│
│     StackFrame, ExceptionRecord, …                             │
│                                                                │
│   web/server.py     ← FastAPI factory + analyzer dispatcher    │
│   cli.py            ← Typer commands (one per analyzer + serve)│
└────────────────────────────────────────────────────────────────┘
```

## Components

### Browser app (`apps/desktop/`)

React 18 served as a static bundle by FastAPI (or by Vite dev server
during development). The `httpBridge` (`src/api/httpBridge.ts`) mounts
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
  (`access_log`, `profiler_collapsed`, `gc_log`, `thread_dump`,
  `thread_dump_multi`, `thread_dump_to_collapsed`, `exception_stack`,
  `jfr_recording`, `flamegraph_svg`, `flamegraph_html`).
  Calls the analyzer **in-process** (no subprocess) and returns the
  full `AnalysisResult` JSON.
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
    schema_version: "0.1.0",
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
| `<repo>/apps/desktop/dist/` | Vite | static React build served by `--static-dir` |
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
