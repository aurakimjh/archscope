# ArchScope

[한국어](./README.ko.md) · [English](./README.en.md)

> Application Architecture Diagnostic & Reporting Toolkit — runs locally
> in your browser, ships report-ready visualizations, never sends your
> data anywhere.

ArchScope ingests middleware logs, GC logs, profiler outputs, and
thread dumps from any of five runtimes (Java, Go, Python, Node.js,
.NET), turns them into normalized analysis results, and renders
interactive charts you can save straight into an architecture report.

## Quick start

```bash
# Install once, run anywhere — single wheel ships the engine + UI.
pip install archscope
archscope serve
# Opens http://127.0.0.1:8765 in your default browser. Ctrl+C to stop.
```

The `archscope` console script (T-208) wraps `uvicorn` and serves the
React bundle that's shipped as wheel package data. Every analyzer
subcommand is also reachable through it (`archscope profiler analyze-…`,
`archscope thread-dump analyze-multi`, etc.) — see `archscope --help`.

To build the wheel from source (e.g. before `pip install`):

```bash
./scripts/build-archscope-wheel.sh
pip install engines/python/dist/archscope-*.whl
```

For UI-side hot-reload during development:

```bash
# Terminal 1 — auto-reloading FastAPI engine
archscope serve --reload --no-browser

# Terminal 2 — Vite dev server (proxies /api → :8765)
cd apps/frontend && npm install && npm run dev

# Open http://127.0.0.1:5173
```

The legacy `archscope-engine` console script is still installed for
backward compatibility; both binaries point at the same Typer app.

The Electron desktop shell that previously wrapped the same codebase
was retired in T-207; ArchScope now ships exclusively as a Python wheel
that serves the React bundle from FastAPI on `127.0.0.1`. A native
desktop track lives separately in `apps/profiler-native/` (Wails v3,
profiler-only). Latest binaries land on the [GitHub releases
page](https://github.com/aurakimjh/archscope/releases).

## Highlights

| Area | What you get |
| --- | --- |
| **Profiler** | async-profiler `collapsed`, FlameGraph.pl/async-profiler **SVG**, async-profiler **HTML**, Jennifer APM CSV. Drill-down, execution-breakdown, timeline-segment analysis, **diff flame** (red = slower / blue = faster), **icicle** (inverted) view, **min-width** simplification, **per-thread** filter, **tree view** sortable table, **pprof export** (gzipped, ready for Pyroscope / Speedscope / `go tool pprof`), and either crisp SVG or fast Canvas flamegraphs (auto-switches at ≥ 4 000 nodes) |
| **JFR recording** | **Binary `.jfr` auto-converted** via the JDK `jfr` CLI (PATH / JAVA_HOME / `ARCHSCOPE_JFR_CLI`); JSON output also accepted. Multi-event mode filter (`cpu` / `wall` / `alloc` / `lock` / `gc` / `exception` / `io` / `nativemem`), time-range filter (`+30s`, ISO, HH:MM:SS), thread-state filter, min-duration filter. **Wall-clock heatmap** (drag to set From/To), **native-memory leak detection** with tail-ratio cutoff |
| **GC log** | **JVM Info card** (Version, CPUs, Memory, Heap Min/Initial/Max/Region, Parallel/Concurrent workers, CommandLine flags, Worker-vs-CPU mismatch warning). Pause + heap timelines with **drag-rectangle zoom and brush selection**, per-window stats, per-collector comparison tab, **9 toggleable heap series** (Heap before/after/committed, Young, Old, Metaspace) with optional Pause overlay on a right axis, point decimation for large logs |
| **Access log** | 22-metric summary including p50/p90/p95/p99, throughput (req/s + bytes/s), static-vs-API split. Per-minute series for percentile timeline, status-class breakdown, error rate, throughput. **Sortable URL stats table** (count / avg / p95 / total bytes / errors with API-only / static-only filter and per-row 2·3·4·5xx mix). **Errors-over-time** view with peak-error highlighting |
| **Thread dumps** | Auto-detected formats across 5 runtimes (Java jstack + jcmd JSON, Go goroutine, Python py-spy / faulthandler / traceback, Node.js diagnostic report / sample trace, .NET clrstack / Environment.StackTrace). **Multi-file picker** (drop a folder of dumps in one shot). Per-language frame normalization, multi-dump correlator: **`LONG_RUNNING_THREAD`**, **`PERSISTENT_BLOCKED_THREAD`**, **`LATENCY_SECTION_DETECTED`**, **`GROWING_LOCK_CONTENTION`**, **`THREAD_CONGESTION_DETECTED`**, **`EXTERNAL_RESOURCE_WAIT_HIGH`**, **`LIKELY_GC_PAUSE_DETECTED`**. **Lock-contention** owner/waiter graph + DFS deadlock detection. **JVM signals** tab (Carrier-pinning / SMR-Zombies / Native methods / Class histogram), Dump overview card |
| **Exception logs** | Dedicated page with paginated + filterable event table, click-row Sheet popup with full message + signature + stack. Top types (simple class names, full FQN on hover), top stack signatures |
| **Thread → flamegraph** | Batch-converts hundreds of dumps into a FlameGraph-compatible collapsed file (CLI + HTTP), feed straight into the Canvas flamegraph |
| **Image export** | PNG 1×/2×/3×, JPEG 2×, SVG vector — per chart and **"Save all charts"** batch export per page. **pprof export** for profilers |
| **UI** | Tailwind v4 + shadcn/ui shell, **Pretendard Variable** font (Korean + English), slim top bar, collapsible sidebar, light/dark/system theme, Korean ↔ English labels. Locked page zoom + hardened against legacy localStorage data |

## Architecture

```text
┌────────────────────────────────────────────────────────────────┐
│  Browser (React + Tailwind v4 + shadcn/ui + D3 + ECharts)      │
│                                                                │
│   • AppShell (TopBar + Sidebar + Tabs)                         │
│   • Pages: Dashboard / Access Log / Profiler / GC / Threads /  │
│     Exception / JFR / Demo / Export / Chart Studio / Settings  │
│   • Charts: D3 Flame / Canvas Flame / D3 Timeline / D3 Bar /   │
│     ECharts (legacy)                                           │
└──────────────────────────┬─────────────────────────────────────┘
                           │  fetch /api/...
                           ▼
┌────────────────────────────────────────────────────────────────┐
│  FastAPI engine (`archscope-engine serve`)                     │
│                                                                │
│   • /api/upload          multipart → server-side temp path     │
│   • /api/analyzer/execute     dispatcher (10+ analyzer types)  │
│   • /api/export/execute       HTML / PPTX / diff exports       │
│   • /api/demo/...             demo bundles                     │
│   • /api/files                stream artifacts back            │
│   • /api/settings             persisted under ~/.archscope/    │
│   • Static React build        served at /                      │
└──────────────────────────┬─────────────────────────────────────┘
                           │  in-process call
                           ▼
┌────────────────────────────────────────────────────────────────┐
│  archscope_engine (pure Python, in-process — no subprocess)    │
│                                                                │
│   parsers/  →  per-format parsers                              │
│       access_log, collapsed, jennifer_csv,                     │
│       svg_flamegraph, html_profiler,                           │
│       gc_log + gc_log_header, jfr_recording, jfr_parser,       │
│       exception, otel,                                         │
│       thread_dump/{java_jstack, java_jcmd_json, go_goroutine,  │
│                     python_dump, nodejs_report,                │
│                     dotnet_clrstack, registry}                 │
│                                                                │
│   analyzers/  →  per-domain analyzers                          │
│       access_log, profiler (collapsed / SVG / HTML / Jennifer),│
│       profiler_diff, native_memory_analyzer,                   │
│       gc_log, jfr, thread_dump, multi_thread_analyzer,         │
│       lock_contention_analyzer, thread_dump_to_collapsed,      │
│       exception, runtime, otel                                 │
│                                                                │
│   exporters/  →  json, html, pptx, pprof, report_diff          │
│   models/     →  AnalysisResult contract, FlameNode (with      │
│                  optional metadata for diff), ThreadSnapshot,  │
│                  StackFrame, ThreadState, GcEvent              │
└────────────────────────────────────────────────────────────────┘
```

The browser never imports a parser; the React app only renders normalized
`AnalysisResult` JSON received from FastAPI. Everything (parsing,
enrichment, multi-dump correlation, exports) runs locally in Python —
no data leaves the machine.

## CLI overview

```bash
archscope-engine serve [--host 127.0.0.1 --port 8765 --reload]

# Profiler
archscope-engine profiler analyze-collapsed --wall flame.collapsed --out result.json
archscope-engine profiler analyze-flamegraph-svg --file flame.svg --out result.json
archscope-engine profiler analyze-flamegraph-html --file flame.html --out result.json
archscope-engine profiler analyze-jennifer-csv --file flame.csv --out result.json

# GC, JFR, exception, access log, etc.
archscope-engine gc-log analyze --file gc.log --out result.json
archscope-engine jfr analyze-json --file jfr.json --out result.json
archscope-engine access-log analyze --file access.log --format nginx --out result.json

# Thread dumps (Java/Go/Python/Node.js/.NET — auto-detected)
archscope-engine thread-dump analyze --file dump.txt --out result.json
archscope-engine thread-dump analyze-multi \
    --input d1.txt --input d2.txt --input d3.txt \
    --out multi.json [--consecutive-threshold 3] [--format <id>]
archscope-engine thread-dump to-collapsed \
    --input d1.txt --input d2.txt --output flame.collapsed [--format <id>]

# Reports
archscope-engine report html  --input result.json --out report.html
archscope-engine report pptx  --input result.json --out report.pptx
archscope-engine report diff  --before before.json --after after.json --out diff.json --html-out diff.html
```

## Repository layout

```text
archscope/
  apps/frontend/         React + Vite + Tailwind v4 + Pretendard
                         (served as a static bundle by FastAPI)
  apps/profiler-native/  Wails v3 native profiler track (separate
                         desktop binary, profiler-only — see
                         docs/{en,ko}/PROFILER_NATIVE.md)
  engines/python/        archscope_engine package + FastAPI server.
                         The unified `archscope` wheel (T-208) is built
                         here via scripts/build-archscope-wheel.sh and
                         ships with the React bundle as package data.
  scripts/               serve-web.sh, run-engine.sh, demo runners
  docs/{en,ko}/          Architecture, parser, user guide, …
  examples/              Sample inputs + generated outputs
```

## Documentation

- [English documentation index](docs/en/README.md)
- [Korean documentation index](docs/ko/README.md)
- User guide — [English](docs/en/USER_GUIDE.md) · [한국어](docs/ko/USER_GUIDE.md)
  (includes the [browser support matrix (T-209)](docs/en/USER_GUIDE.md#browser-support-matrix-t-209))
- Multi-language thread dumps — [English](docs/en/MULTI_LANGUAGE_THREADS.md) · [한국어](docs/ko/MULTI_LANGUAGE_THREADS.md)
- Architecture — [English](docs/en/ARCHITECTURE.md) · [한국어](docs/ko/ARCHITECTURE.md)

## Local privacy guarantee

ArchScope is local-first. Every parser, every analyzer, and every
exporter runs in the Python process you started with
`archscope-engine serve`. The only external dependency the FastAPI
server has is the network stack to bind `127.0.0.1`. No telemetry,
no remote LLM call (the optional AI interpretation requires a local
Ollama runtime), no third-party SaaS. Uploaded files live under
`~/.archscope/uploads/` and can be deleted at any time.

## License

MIT — see [LICENSE](./LICENSE).
