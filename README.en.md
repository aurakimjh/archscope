# ArchScope (English)

[한국어](./README.ko.md) · [Top-level README](./README.md)

ArchScope is a local-first **web** application for application
architects who need to turn raw operational evidence (access logs, GC
logs, profiler outputs, thread dumps, exception stacks, JFR
recordings) into report-ready charts and diagnostic findings — without
sending anything to a third-party SaaS.

## What you get

| Domain | Capabilities |
| --- | --- |
| **Profiler** | async-profiler `collapsed` / Jennifer APM CSV / FlameGraph.pl & async-profiler **SVG** / async-profiler **HTML** with inline-SVG and JS-tree fallbacks; flame drill-down + breakdown; SVG flamegraph for normal sizes, **Canvas flamegraph auto-engaged at ≥4 000 nodes** for converted thread-dump bundles. |
| **GC log** | Pause + heap timelines with **wheel/drag zoom and brush**; per-window count / avg / p95 / max; per-collector comparison tab (G1/ZGC/Shenandoah/Parallel/Serial/CMS); Full-GC event markers with hover payloads (cause, before/after/committed heap, pause). |
| **Thread dumps** | Six auto-detected formats (`java_jstack`, `go_goroutine`, `python_pyspy`, `python_faulthandler`, `nodejs_diagnostic_report`, `dotnet_clrstack`); per-language frame normalization (CGLIB / Express layer aliases / async state machines / `gin.HandlerFunc.func1` / starlette wrappers …); multi-dump correlator emitting `LONG_RUNNING_THREAD`, `PERSISTENT_BLOCKED_THREAD`, `LATENCY_SECTION_DETECTED`. |
| **Thread → flamegraph** | Batch-converts hundreds of dumps from any combination of supported runtimes into a FlameGraph-compatible collapsed file (CLI + HTTP) — feeds straight into the Canvas flamegraph or the standard collapsed pipeline. |
| **Other analyzers** | Access log (NGINX / Apache / OHS / WebLogic / Tomcat / custom regex), JVM exception, JFR (`jfr print --json`), OTel JSONL. |
| **Reports** | HTML / PowerPoint / before-after diff exports per AnalysisResult; per-chart image export (PNG 1×/2×/3×, JPEG 2×, SVG vector); **"Save all charts"** batch export per analyzer page. |
| **UI** | Tailwind v4 + shadcn/ui shell, slim top bar, collapsible sidebar, light/dark/system theme, Korean ↔ English labels, sticky FileDock with drag-and-drop upload. |

## Tech stack

- **Frontend** — React 18 + Vite 8 + TypeScript + Tailwind v4 + shadcn/ui (Radix-based) + lucide icons. Charts: D3 (timeline / bar / flamegraph + Canvas flamegraph) and ECharts (legacy panels). Image export via `html-to-image` + native `canvas.toDataURL()`.
- **Backend** — FastAPI 0.110+ + uvicorn (single in-process Python). Static React build is served at `/` and the analyzer dispatcher lives at `/api/analyzer/execute`.
- **Engine** — Pure Python (`archscope-engine`, Python ≥ 3.9), Typer CLI, defusedxml (XXE-safe SVG parsing), python-multipart for uploads. No subprocess fan-out — analyzers are called in-process.

## Quick start

```bash
# 1. Engine + web server
cd engines/python
python -m venv .venv
source .venv/bin/activate          # or .venv\Scripts\activate on Windows
pip install -e .

# 2. Build UI + start the server (single command helper)
cd ../..
./scripts/serve-web.sh             # builds apps/desktop/dist + starts the server

# 3. Open http://127.0.0.1:8765
```

Development loop with UI hot-reload:

```bash
# Terminal 1 — auto-reloading engine
archscope-engine serve --reload

# Terminal 2 — Vite dev server (proxies /api → :8765)
cd apps/desktop && npm install && npm run dev
# open http://127.0.0.1:5173
```

## CLI cheatsheet

```bash
# Web server
archscope-engine serve [--host 127.0.0.1 --port 8765 --reload \
                        --static-dir apps/desktop/dist --no-dev-cors]

# Profiler
archscope-engine profiler analyze-collapsed       --wall flame.collapsed --out result.json
archscope-engine profiler analyze-flamegraph-svg  --file flame.svg       --out result.json
archscope-engine profiler analyze-flamegraph-html --file flame.html      --out result.json
archscope-engine profiler analyze-jennifer-csv    --file flame.csv       --out result.json

# GC, JFR, exception, access log
archscope-engine gc-log    analyze     --file gc.log     --out result.json
archscope-engine jfr       analyze-json --file jfr.json  --out result.json
archscope-engine exception analyze     --file ex.txt     --out result.json
archscope-engine access-log analyze    --file access.log --format nginx  --out result.json

# Thread dumps
archscope-engine thread-dump analyze       --file dump.txt --out result.json
archscope-engine thread-dump analyze-multi --input d1.txt --input d2.txt --input d3.txt \
                                           --out multi.json
archscope-engine thread-dump to-collapsed  --input d1.txt --input d2.txt \
                                           --output flame.collapsed [--format <id>]

# Reports
archscope-engine report html --input result.json --out report.html
archscope-engine report pptx --input result.json --out report.pptx
archscope-engine report diff --before before.json --after after.json \
                             --out diff.json --html-out diff.html
```

See [`docs/en/USER_GUIDE.md`](docs/en/USER_GUIDE.md) for screenshots-free
walkthroughs of every page and every CLI command, plus the
[multi-language thread-dump reference](docs/en/MULTI_LANGUAGE_THREADS.md).

## Local-first

- All parsing, enrichment, multi-dump correlation, and exporting runs
  inside your local Python process. The engine binds `127.0.0.1` by
  default.
- Uploaded files land in `~/.archscope/uploads/`; settings in
  `~/.archscope/settings.json`. Delete either at any time.
- The optional AI interpretation runs against a **local** Ollama
  instance only. There is no remote LLM call.

## License

MIT — see [LICENSE](./LICENSE).
