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
# 1. Python engine + web server
cd engines/python
python -m venv .venv
source .venv/bin/activate          # or .venv\Scripts\activate on Windows
pip install -e .

# 2. Build the React UI once and serve it
cd ../..
./scripts/serve-web.sh             # macOS / Linux
# Equivalent: npm --prefix apps/desktop run build && \
#             archscope-engine serve --static-dir apps/desktop/dist

# 3. Open http://127.0.0.1:8765 in your browser.
```

For UI-side hot-reload during development:

```bash
# Terminal 1 — auto-reloading FastAPI engine
archscope-engine serve --reload

# Terminal 2 — Vite dev server (proxies /api → :8765)
cd apps/desktop && npm install && npm run dev

# Open http://127.0.0.1:5173
```

## Highlights

| Area | What you get |
| --- | --- |
| **Profiler** | async-profiler `collapsed`, FlameGraph.pl/async-profiler **SVG**, async-profiler **HTML**, Jennifer APM CSV — drill-down, breakdown, and either crisp SVG or fast Canvas flamegraphs (auto-switches at ≥4 000 nodes) |
| **GC log** | Pause + heap timelines with **wheel/drag zoom and brush selection**, per-window stats (count / avg / p95 / max), per-collector comparison tab, area-fill on heap_before, Full-GC event markers with hover payloads |
| **Thread dumps** | Six auto-detected formats — `java_jstack`, `go_goroutine`, `python_pyspy`, `python_faulthandler`, `nodejs_diagnostic_report`, `dotnet_clrstack` — with per-language frame normalization (CGLIB/AOP, Express layer aliases, async state machines, …) and a multi-dump correlator that fires **`LONG_RUNNING_THREAD`**, **`PERSISTENT_BLOCKED_THREAD`**, and **`LATENCY_SECTION_DETECTED`** findings |
| **Thread → flamegraph** | Batch-converts hundreds of dumps into a FlameGraph-compatible collapsed file (CLI + HTTP), feed straight into the Canvas flamegraph |
| **Image export** | PNG 1×/2×/3×, JPEG 2×, SVG vector — per chart and **"Save all charts"** batch export per page |
| **UI** | Tailwind v4 + shadcn/ui shell, slim top bar, collapsible sidebar, light/dark/system theme, Korean ↔ English labels |

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
│  archscope_engine (pure Python — no Electron, no subprocess)   │
│                                                                │
│   parsers/  →  per-format parsers                              │
│       access_log, collapsed, jennifer_csv,                     │
│       svg_flamegraph, html_profiler,                           │
│       gc_log, jfr, exception, otel,                            │
│       thread_dump/{java_jstack, go_goroutine, python_dump,     │
│                     nodejs_report, dotnet_clrstack, registry}  │
│                                                                │
│   analyzers/  →  per-domain analyzers                          │
│       access_log, profiler (collapsed / SVG / HTML / Jennifer),│
│       gc_log, jfr, thread_dump, multi_thread_analyzer,         │
│       thread_dump_to_collapsed, exception, runtime, otel       │
│                                                                │
│   exporters/  →  json, html, pptx, report_diff                 │
│   models/     →  AnalysisResult contract, FlameNode,           │
│                  ThreadSnapshot, StackFrame, ThreadState       │
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
  apps/desktop/          React + Vite + Tailwind v4 frontend
                         (served as a static bundle by FastAPI)
  engines/python/        archscope_engine package + FastAPI server
  scripts/               serve-web.sh, run-engine.sh, demo runners
  docs/{en,ko}/          Architecture, parser, user guide, …
  examples/              Sample inputs + generated outputs
```

## Documentation

- [English documentation index](docs/en/README.md)
- [Korean documentation index](docs/ko/README.md)
- User guide — [English](docs/en/USER_GUIDE.md) · [한국어](docs/ko/USER_GUIDE.md)
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
