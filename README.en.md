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
| **Profiler** | async-profiler `collapsed` / Jennifer APM CSV / FlameGraph.pl & async-profiler **SVG** / async-profiler **HTML** with inline-SVG and JS-tree fallbacks. **Differential flame** (red = slower, blue = faster), **icicle** (inverted) view, **min-width** simplification, **per-thread filter** for `-t` collapsed, **tree-view** sortable table, **pprof export** (gzipped) ready for Pyroscope / Speedscope / `go tool pprof`. SVG flamegraph for normal sizes, Canvas flamegraph auto-engaged at ≥ 4 000 nodes. |
| **JFR recording** | Binary `.jfr` auto-converted via the JDK `jfr` CLI (PATH / `JAVA_HOME` / `ARCHSCOPE_JFR_CLI`); legacy JSON path also accepted. **Multi-event mode** (`cpu` / `wall` / `alloc` / `lock` / `gc` / `exception` / `io` / `nativemem`), **time-range filter** (ISO / HH:MM:SS / `+30s` / `-2m` / `500ms`), **thread-state filter**, **min-duration** filter. **Wall-clock heatmap** (drag to set From/To). **Native-memory leak detection** with tail-ratio cutoff. |
| **GC log** | **JVM Info** card (Version / CPUs / Memory / Heap Min/Initial/Max / Region size / Parallel & Concurrent workers / Compressed Oops / CommandLine flags) with worker-vs-CPU mismatch warning. Pause + heap timelines with **drag-rectangle zoom** and brush; per-window count / avg / p95 / max; per-collector comparison tab; **9 toggleable heap series** (Heap before/after/committed, Young, Old, Metaspace) with optional Pause overlay on a right axis; point decimation for huge logs. |
| **Access log** | 22-metric summary: total / errors / **p50 / p90 / p95 / p99**, **throughput** (req/s, bytes/s), **static-vs-API** split. Per-minute series for percentile timeline, status-class breakdown, error rate, throughput. **Sortable URL stats table** (count / avg / p95 / total bytes / errors with API-only / static-only filter and per-row 2·3·4·5xx mix). **Errors over time** view that highlights any minute ≥ 50% error rate. |
| **Thread dumps** | Auto-detected across 5 runtimes — `java_jstack` (incl. JDK 21+ no-`nid` variant), `java_jcmd_json`, `go_goroutine`, `python_pyspy` / `python_faulthandler` / `python_traceback`, `nodejs_diagnostic_report` / `nodejs_sample_trace`, `dotnet_clrstack` / `dotnet_environment_stacktrace`. **Multi-file picker** (drop a folder of dumps in one shot). Per-language frame normalization. Multi-dump correlator findings: `LONG_RUNNING_THREAD`, `PERSISTENT_BLOCKED_THREAD`, `LATENCY_SECTION_DETECTED`, `GROWING_LOCK_CONTENTION`, `THREAD_CONGESTION_DETECTED`, `EXTERNAL_RESOURCE_WAIT_HIGH`, `LIKELY_GC_PAUSE_DETECTED`, `VIRTUAL_THREAD_CARRIER_PINNING`, `SMR_UNRESOLVED_THREAD`. **Lock-contention** owner/waiter graph + DFS deadlock detection. **JVM signals** tab (Carrier-pinning / SMR / Native methods / Class histogram). UTF-16 / BOM detection. |
| **Exception logs** | Dedicated page with paginated + filterable event table; click-row Sheet popup with full message, signature, stack. Top types (simple class names, full FQN on hover), top stack signatures. |
| **Thread → flamegraph** | Batch-converts hundreds of dumps from any combination of supported runtimes into a FlameGraph-compatible collapsed file (CLI + HTTP). |
| **Reports** | HTML / PowerPoint / before-after diff exports per AnalysisResult; per-chart image export (PNG 1×/2×/3×, JPEG 2×, SVG vector); **"Save all charts"** batch export. **pprof** for profilers. |
| **UI** | Tailwind v4 + shadcn/ui shell, **Pretendard Variable** font, slim top bar, collapsible sidebar, light/dark/system theme, Korean ↔ English labels, FileDock with drag-and-drop multi-file upload. |

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
./scripts/serve-web.sh             # builds apps/frontend/dist + starts the server

# 3. Open http://127.0.0.1:8765
```

Development loop with UI hot-reload:

```bash
# Terminal 1 — auto-reloading engine
archscope-engine serve --reload

# Terminal 2 — Vite dev server (proxies /api → :8765)
cd apps/frontend && npm install && npm run dev
# open http://127.0.0.1:5173
```

## CLI cheatsheet

```bash
# Web server
archscope-engine serve [--host 127.0.0.1 --port 8765 --reload \
                        --static-dir apps/frontend/dist --no-dev-cors]

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
