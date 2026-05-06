# ArchScope User Guide (English)

[한국어](../ko/USER_GUIDE.md)

This guide walks through every page and CLI command of the local-first
ArchScope web app. If you only have a minute, the
[top-level README](../../README.md) has a one-screen quick start.

---

## Table of contents

1. [Install & run](#install--run)
2. [UI tour](#ui-tour)
3. [Dashboard](#dashboard)
4. [Access Log Analyzer](#access-log-analyzer)
5. [Profiler Analyzer](#profiler-analyzer)
6. [GC Log Analyzer](#gc-log-analyzer)
7. [Thread Dump Analyzer](#thread-dump-analyzer)
8. [Exception Analyzer](#exception-analyzer) · [JFR Analyzer](#jfr-analyzer)
9. [Demo Data Center](#demo-data-center)
10. [Export Center](#export-center)
11. [Chart Studio](#chart-studio)
12. [Settings](#settings)
13. [AI Interpretation (Optional)](#ai-interpretation-optional)
14. [Image export & "Save all charts"](#image-export--save-all-charts)
15. [Thread → flamegraph conversion](#thread--flamegraph-conversion)
16. [CLI reference](#cli-reference)
17. [Troubleshooting FAQ](#troubleshooting-faq)
18. [Browser support matrix (T-209)](#browser-support-matrix-t-209)

---

## Install & run

### Requirements

| Item | Required |
| --- | --- |
| OS | macOS / Linux / Windows |
| Python | 3.9 or newer (only when running the engine from source) |
| Node.js | 18+ (only when building the UI from source) |
| JDK | 11+ on PATH (optional — only needed for binary `.jfr` ingestion) |
| RAM | 4 GB minimum, 8 GB recommended for large flamegraphs |
| Disk | ~500 MB |

ArchScope also ships a Windows installer (NSIS) and portable zip with
the Python engine PyInstaller-bundled inside, so end users on Windows
don't need to install Python, Node, or build anything. The "from
source" steps below remain the standard developer flow.

### Install the engine

```bash
cd engines/python
python -m venv .venv
source .venv/bin/activate          # Windows: .venv\Scripts\activate
pip install -e .
```

`pip install -e .` registers the `archscope-engine` console script and
installs FastAPI / uvicorn / defusedxml / Typer / python-multipart.

### Build the UI and start the server

The repository ships with a helper that does both:

```bash
./scripts/serve-web.sh             # macOS / Linux
```

Equivalent manual steps:

```bash
cd apps/frontend
npm install
npm run build                      # produces apps/frontend/dist
cd ../..
archscope-engine serve --static-dir apps/frontend/dist
```

Open `http://127.0.0.1:8765`. ArchScope binds `127.0.0.1` by default;
pass `--host 0.0.0.0` only if you knowingly want to expose it on a
trusted network.

### Hot-reload development loop

```bash
# Terminal 1 — auto-reloading FastAPI engine
archscope-engine serve --reload

# Terminal 2 — Vite dev server (proxies /api → :8765)
cd apps/frontend && npm run dev     # http://127.0.0.1:5173
```

---

## UI tour

The shell consists of three persistent regions:

```text
┌────────────────────────────────────────────────────────┐
│  TopBar — brand · search · theme · locale · settings   │
├────────────┬───────────────────────────────────────────┤
│ Sidebar    │  Main content                             │
│ (collap-   │  ┌─────────────────────────────────────┐  │
│  sible)    │  │ Sticky FileDock (drag & drop)       │  │
│            │  ├─────────────────────────────────────┤  │
│            │  │ Tabs (per page)                     │  │
│            │  │   summary / charts / tables / …     │  │
│            │  └─────────────────────────────────────┘  │
└────────────┴───────────────────────────────────────────┘
```

- **TopBar** — light/dark/system theme picker (persisted), language
  toggle (English / 한국어), settings shortcut.
- **Sidebar** — collapsible icon rail. Collapsed state is persisted to
  `localStorage`. Sections: Analyzers, Workspace, Settings.
- **FileDock** — every analyzer page starts with a sticky upload card.
  Drag a file in or click "Browse". The file is uploaded to
  `~/.archscope/uploads/` and the resulting server-side path is what
  the analyzer uses.
- **Tabs** — every analyzer page splits results across tabs (Summary /
  Charts / per-domain tabs / Diagnostics) so the page never has to
  scroll past unrelated information.

Most pages also expose a **"Save all charts"** button on the right
side of the tab list — see [Image export](#image-export--save-all-charts).

---

## Dashboard

The Dashboard remembers the last analysis result you ran (any analyzer
that calls `saveDashboardResult` does this) and renders the access-log
metric cards plus the standard chart panels.

- **Empty state** — until you run an analyzer at least once, the
  Dashboard shows a "no analysis results yet" hint. Run any analyzer
  and come back.
- **Save all charts** — a single click bundles every chart on the
  Dashboard into 2× PNG files for a report.

---

## Access Log Analyzer

### Inputs

Drop a `.log` / `.txt` access log into the FileDock. Then pick a format
in the **Analyzer options** card:

- `nginx` (default), `apache`, `ohs`, `weblogic`, `tomcat`,
  `custom-regex` (fallback parser, slower).

Optional fields:

- **Max lines** — sample the first N lines for speed.
- **Start time / End time** — ISO `datetime-local`; the engine windows
  the analysis to this range.

Click **Analyze**; **Cancel** appears while the run is in flight.

### Tabs

| Tab | What's there |
| --- | --- |
| Summary | 12-metric grid: total requests, error rate, avg + **p50 / p90 / p95 / p99** response time, total bytes, avg req/s, avg throughput (bytes/s), static request count, API request count. |
| Charts | Request-count trend (ECharts panel — supports the same image-export dropdown as D3 charts). |
| URLs | **Sortable** per-URL stats table backed by `url_stats`. Pick the sort axis (count / avg response / p95 response / total bytes / errors), filter by classification (all / API only / static only). Each row shows method, URI, classification badge, count, avg ms, p95 ms, total bytes, error count, and a per-row `2·3·4·5xx` mix. |
| Status & errors | Status families table, top status codes (full code, not just family), and a **per-minute timeline** of 2xx/3xx/4xx/5xx counts + error rate. Any minute with ≥ 50% error rate is highlighted in rose; the card header surfaces the peak ("Peak: 75% errors at 14:23 (15/20 requests)") so you can pinpoint when failures started. |
| Parser Report | Parser diagnostics — counts of skipped lines and a sample of failed lines for debugging custom formats. |

The analyzer also emits new findings: `SLOW_URL_P95` (≥ 1 s p95 with ≥ 5
samples) and `ERROR_BURST_DETECTED` (a single minute crossing 50% error
rate with ≥ 5 requests).

---

## Profiler Analyzer

### Inputs

The **Profile format** selector controls the parser:

| `profileFormat` | Accepts | When to pick it |
| --- | --- | --- |
| `collapsed` | `*.collapsed`, `*.txt` | async-profiler `-o collapsed`, perf `stackcollapse-perf.pl`, jstack converter output (see [Thread → flamegraph](#thread--flamegraph-conversion)) |
| `jennifer_csv` | `*.csv` | Jennifer APM flamegraph CSV |
| `flamegraph_svg` | `*.svg` | FlameGraph.pl or async-profiler `-o svg` |
| `flamegraph_html` | `*.html`, `*.htm` | async-profiler self-contained HTML (inline-SVG or embedded JS tree) |

The FileDock's `accept` adapter changes automatically when you switch
formats so the OS file picker only shows the relevant extensions.

For collapsed input, **Profile kind** further selects `wall`, `cpu`, or
`lock` — this only changes labels and findings, not parsing.

Other knobs:

- **Interval (ms)** — sampling interval used to convert samples → time.
- **Elapsed seconds** — wall-clock window length, optional.
- **Top N** — number of top stacks shown in the breakdown.

### Tabs

| Tab | What's there |
| --- | --- |
| Summary | Total samples · interval · estimated CPU/wall time + drill-down stage metrics (matched samples / estimated seconds / total ratio / parent stage ratio). |
| Flame Graph | The interactive flamegraph with the **Display toolbar** above it (highlight regex, simple class names, normalize lambdas, dotted package, **Icicle** inverted view, **Min width** %). When the tree carries thread brackets (async-profiler `-t`), a **Filter by thread** dropdown re-roots the flame on a single thread without a server round-trip. Above the chart, the **Export pprof** button downloads a gzipped pprof file ready for Pyroscope / Speedscope / `go tool pprof`. SVG renderer by default; ≥ 4 000 nodes auto-switch to Canvas. Click a frame to zoom in; double-click resets. Hover for sample count + ratio + category. |
| **Tree** | Hierarchical expandable table of the same flame data, sorted by samples desc. Columns: Frame · Samples · Self · % of total. Honors the same display + highlight options as the flame graph. |
| **Diff** | Two FileDocks (Baseline / Target) + format selectors per side + Normalize toggle. After running, shows a divergent-color flame (red = increased, blue = decreased) and two tables: "Biggest increases" (slower) and "Biggest decreases" (faster). The diff tab also lists the **Recent profile files** (last 10 analyzed files persisted to localStorage) — click to set baseline, Shift+click to set target, for continuous-session workflows. |
| Drill-down | Stack-frame filters with `include_text` / `exclude_text` / `regex_include` / `regex_exclude`, three match modes (`anywhere` / `ordered` / `subtree`), and two view modes (`preserve_full_path` / `reroot_at_match`). Apply to push a new stage; Reset returns to the original tree. |
| Timeline Analysis | Stacked composition and evidence table that converts flamegraph samples into an estimated execution composition: startup/framework, internal methods, SQL, DB network wait, external call, external network wait, pool wait, lock wait, and other waits. It is sample-based composition, not timestamp-ordered tracing. |
| Breakdown | ECharts donut + bar showing how samples distribute across "execution categories" (SQL / external API / network I/O / connection-pool wait / others). The breakdown table shows the top method per category. |
| Top Stacks | Sorted table of the top N stacks (samples · estimated seconds · ratio). |
| Parser Report | Parser diagnostics. |

### Saving the flamegraph

- **SVG renderer** — the chart frame's "Save image" dropdown supports
  PNG 1×/2×/3×, JPEG 2×, and SVG vector.
- **Canvas renderer** — adds a separate **"Save PNG"** action that uses
  `canvas.toDataURL()` for a pixel-perfect snapshot at the current
  device pixel ratio. The standard dropdown (PNG/JPEG/SVG via
  `html-to-image`) still works.
- **Export pprof** — downloads `*.pb.gz` (gzipped Google pprof binary)
  for use with `go tool pprof`, Pyroscope, Speedscope, or `pprof.dev`.

---

## GC Log Analyzer

### Inputs

Drop a `*.log` / `*.txt` / `*.gz` HotSpot unified GC log into the
FileDock and click **Analyze**. The engine extracts pause / heap /
allocation / promotion timelines plus per-collector counts.

### Tabs

| Tab | What's there |
| --- | --- |
| **JVM Info** (default) | JVM and system metadata extracted from the recording header: Version, CPUs (total/available), Memory, Heap Min/Initial/Max/Region size, Parallel/Concurrent/Concurrent-refinement workers, Compressed Oops, NUMA, Pre-touch, Periodic GC. The full **CommandLine flags** are shown verbatim (one flag per line, with a **Copy** button) and the raw header lines are preserved for capture. A **worker-vs-CPU mismatch warning** banner highlights configurations like "GC workers limited to 1 on a 9-CPU host". |
| Summary | 15-stat metric grid (throughput, p50/p95/p99/max/avg/total pause, young/full GC count, allocation/promotion rate, humongous count, concurrent-mode failures, promotion failures) plus a findings card. |
| GC Pauses | **Interactive timeline** — drag inside the plot to draw a visible blue selection rectangle and zoom into that range; wheel zooms in/out; double-click resets. Brushing populates a 4-stat selection summary (events in window / avg / p95 / max pause). Full-GC events are orange dashed verticals (hover for `cause`, `pause_ms`, before/after/committed heap). Point decimation (max 2 000 per series) keeps huge logs interactive. |
| Heap Usage | **9 toggleable series** (Heap before/after/committed, Young before/after, Old before/after, Metaspace before/after) with optional **Pause overlay on a right axis**. Series with no data in the recording are greyed out automatically. Same drag-zoom / wheel-zoom as the Pauses tab. |
| Algorithm comparison | Frontend-aggregated table of pause stats per `gc_type` (count / avg / p95 / max / total ms) plus two horizontal D3 bar charts (avg + max). Lets you compare G1Young / G1Mixed / FullGC / ZGC / Shenandoah collectors side by side. |
| Breakdown | D3 horizontal bars for `gc_type_breakdown` and `cause_breakdown`. |
| Events | Up to 200 events shown in a shadcn table (timestamp · uptime · type · cause · pause · heaps). |
| Parser Report | Parser diagnostics. |

---

## Thread Dump Analyzer

The thread-dump page is the only "multi-file" analyzer in ArchScope and
the most heavily TDA-inspired one.

### Adding files

The FileDock supports **multi-file selection** — `Ctrl/Shift+Click` in
the OS picker, drag-drop a folder of dumps, or repeat the upload to
append. Each successful upload appends to the cumulative file list
right below. Each row carries a numeric index, the original file name,
the upload size, and an `X` button to remove it.

You can mix files from any supported runtime (Java, Go, Python, Node.js,
.NET). The parser registry sniffs the first 4 KB of every file and
matches across **9 plugin variants** (`java_jstack`, `java_jcmd_json`,
`go_goroutine`, `python_pyspy`, `python_faulthandler`,
`python_traceback`, `nodejs_diagnostic_report`, `nodejs_sample_trace`,
`dotnet_clrstack`, `dotnet_environment_stacktrace`). If two files
resolve to different formats the request fails fast with
`MIXED_THREAD_DUMP_FORMATS`. To force one parser, type its `format_id`
into the **Format override** input (e.g. `java_jstack`) — auto-detection
is skipped. UTF-16 / BOM-encoded dumps are detected and decoded on the
fly.

The **Consecutive-dump threshold** input (default `3`) controls when the
correlator emits persistence findings.

Click **Correlate dumps** to run the multi-dump analyzer.

### Tabs

| Tab | What's there |
| --- | --- |
| **Overview** | Dump-level metadata card (count / time span / unique thread name / total observations / dominant runtime / parser format) and a quick distribution of states aggregated across all dumps. |
| Findings | Severity-colored cards for every finding (see catalog below). Full `LONG_RUNNING_THREAD` and `PERSISTENT_BLOCKED_THREAD` tables appear below the cards. |
| Charts | D3 vertical bar chart of thread count per dump + D3 horizontal sorted bar chart of top threads by observation count. |
| Per dump | Per-dump state distribution table. |
| Threads | Sorted table of every thread name and how many dumps it appeared in. |
| **Lock contention** | Owner / waiter graph (one node per `lock_addr`) plus the **deadlock cycles** detected by DFS over the wait graph. |
| **JVM signals** | Four sub-tabs that surface JVM/runtime-specific evidence: **Carrier-pinning** (virtual-thread carriers blocked on monitors), **SMR / Zombies** (SafeMemoryReclamation un-resolved threads), **Native methods** (top JNI frames per thread), **Class histogram** (most-referenced classes per dump if jcmd JSON is included). |

### Findings catalog

- **`LONG_RUNNING_THREAD`** *(warning)* — a thread name kept the same
  RUNNABLE stack for ≥ N consecutive dumps.
- **`PERSISTENT_BLOCKED_THREAD`** *(critical)* — a thread stayed
  BLOCKED / LOCK_WAIT for ≥ N consecutive dumps.
- **`LATENCY_SECTION_DETECTED`** *(warning)* — a thread stayed in
  `NETWORK_WAIT`, `IO_WAIT`, or `CHANNEL_WAIT` for ≥ N consecutive
  dumps. Wait categories come from per-language enrichment
  (`EPoll.epollWait` → `NETWORK_WAIT`, `gopark` → `CHANNEL_WAIT`,
  `Monitor.Enter` → `LOCK_WAIT`, …).
- **`GROWING_LOCK_CONTENTION`** *(warning)* — the same lock address
  attracted strictly more waiters across consecutive dumps.
- **`THREAD_CONGESTION_DETECTED`** *(warning)* — runnable thread count
  exceeds CPU count by an order of magnitude in a single dump.
- **`EXTERNAL_RESOURCE_WAIT_HIGH`** *(warning)* — > 30% of threads sit
  in `NETWORK_WAIT` / `IO_WAIT` simultaneously.
- **`LIKELY_GC_PAUSE_DETECTED`** *(warning)* — most threads are
  RUNNABLE and a VM internal thread (`VM Thread` / `GC task thread`)
  carries a GC frame.
- **`VIRTUAL_THREAD_CARRIER_PINNING`** *(warning)* — Loom carrier
  thread is pinned because the virtual thread holds a monitor.
- **`SMR_UNRESOLVED_THREAD`** *(warning)* — `_threads` SMR list still
  references a thread that doesn't appear in this dump.

For the full per-language enrichment matrix and detection signatures,
see [`MULTI_LANGUAGE_THREADS.md`](MULTI_LANGUAGE_THREADS.md).

---

## Exception Analyzer

A dedicated full page (no longer the placeholder shell). Drop a Java
exception stack file (`*.txt` / `*.log`) and ArchScope produces:

- **Summary metrics** — total events, unique types, unique signatures,
  first/last seen timestamps.
- **Top types** — bar chart + table of exception class counts. The
  table shows the **simple class name** (e.g. `NullPointerException`)
  with the full FQN on hover, so deep packages don't blow up the
  layout.
- **Top stack signatures** — top 10 normalized stack signatures across
  all events.
- **Events table** — paginated + filterable (search box matches
  message, type, or signature). Clicking a row opens a `Sheet` popup
  with the full message, signature, and stack trace, formatted for
  copy-paste into an issue tracker.
- **Parser Report** — diagnostics from the parser.

The page is bounded vertically — long event lists paginate instead of
extending the page indefinitely.

---

## JFR Analyzer

Drop a binary `.jfr` recording **or** the JSON output of
`jfr print --json recording.jfr`. ArchScope detects the binary header
(`FLR\0`) and auto-runs the JDK `jfr` CLI to convert it on the fly.
The CLI is resolved in this order:

1. `ARCHSCOPE_JFR_CLI` env var (full path to `jfr` / `jfr.exe`).
2. `JAVA_HOME/bin/jfr`.
3. `jfr` on the system `PATH`.

If no CLI is found, the analyzer reports a clean error suggesting JDK
11+ as a prerequisite.

### Filters (URL-style query params on the analyzer call)

| Param | Value | Effect |
| --- | --- | --- |
| `event_modes` | comma-separated `cpu` / `wall` / `alloc` / `lock` / `gc` / `exception` / `io` / `nativemem` | Limit which event categories are aggregated. |
| `time_from` / `time_to` | ISO timestamp, `HH:MM:SS`, relative (`+30s`, `-2m`, `500ms`) | Window the recording. |
| `thread_state` | comma-separated states | Only count samples in those states (`RUNNABLE`, `BLOCKED`, `WAITING`, …). |
| `min_duration_ms` | number | Drop events below the threshold. |

The page exposes these as form inputs at the top.

### Tabs

| Tab | What's there |
| --- | --- |
| Summary | Event count, distinct event types, time span, top thread by samples. |
| Event distribution | D3 horizontal bar chart of event-type counts. |
| Top samples | Top 50 execution samples (thread / state / sample count / top frame). |
| **Heatmap** | 1D wall-clock density strip — drag a region to populate `time_from` / `time_to` for the next analyze run. |
| GC summary | If `gc` events are included, pause percentile summary. |
| **Native memory** | If native-memory events are present, alloc/free pairing — sites whose allocations are not matched by frees within the recording are flagged with a tail-ratio cutoff. Byte-weighted flame view available. |
| Parser Report | Parser diagnostics. |

---

## Demo Data Center

If you don't have your own logs handy, the Demo Data Center runs the
bundled fixture set at `projects-assets/test-data/demo-site/`:

- Browse to or paste a manifest root path.
- Pick **All / synthetic / real** as the data source filter.
- Optionally pick a single scenario from the dropdown.
- Click **Run demo data**.

The results card shows aggregate stats plus per-scenario detail cards
with artifact tables. Clicking **Send to Export Center** on a JSON
artifact opens the Export Center pre-populated.

---

## Export Center

Renders a finished `AnalysisResult` JSON into a final report:

| Format | Output |
| --- | --- |
| `html` | Single-file portable HTML report (charts + tables + findings). |
| `pptx` | Minimal PowerPoint deck with summary + key charts. |
| `diff` | Before-vs-after comparison: emits both a JSON diff and an HTML diff side-by-side. |

Pick a format, browse to the JSON file(s), optionally type a custom
title or diff label, click **Run export**. The result panel lists every
file written by the engine.

---

## Chart Studio

Lets you preview every built-in chart template against a sample
`AnalysisResult`. Useful for:

- Inspecting which chart layouts the engine ships.
- Toggling SVG vs. canvas renderer per template.
- Editing the underlying ECharts option JSON in the inline editor (Edit
  → Apply / Reset).
- **Loading external JSON** — select an analyzer JSON from disk and
  re-render the current template against it. The file is fetched via
  `/api/files?path=...`.

---

## Settings

- **Engine path** — leave blank to use the bundled engine. Set it only
  if you ship a separate `archscope-engine` binary.
- **Theme** — three-button picker (Light / Dark / System) wired to the
  global ThemeProvider; the choice is persisted in `localStorage`.
- **Default chart theme** — light or dark default for ECharts panels.
- **Locale** — English or 한국어 (also persisted).
- **Save settings** writes to `~/.archscope/settings.json`. **Reset to
  default** zeros every field client-side; click Save to persist.

---

## AI Interpretation (Optional)

ArchScope can optionally use a **local LLM** to produce AI-assisted
findings on top of the deterministic analysis. AI output is always
evidence-bound and displayed separately from rule-based findings. No
cloud API call is required — the feature runs entirely on your machine
via [Ollama](https://ollama.com/).

### Installing Ollama

#### Windows

1. Download the Windows installer from <https://ollama.com/download/windows>.
2. Run `OllamaSetup.exe` and follow the wizard. Ollama is installed as
   a background service and starts automatically.
3. Open **PowerShell** or **Command Prompt** and verify:

   ```powershell
   ollama --version
   ```

> **Note:** Ollama on Windows requires Windows 10 version 22H2 or
> newer. A GPU with updated drivers is recommended for reasonable
> inference speed, but CPU-only mode also works (slower).

#### macOS

```bash
brew install ollama            # Homebrew
# — or download the .dmg from https://ollama.com/download/mac
ollama serve                   # starts the local server (if not using the app)
```

#### Linux

```bash
curl -fsSL https://ollama.com/install.sh | sh
systemctl start ollama         # or: ollama serve
```

### Pulling the recommended model

ArchScope suggests `qwen2.5-coder:7b` as a starter model (requires
~5 GB disk, 16 GB RAM recommended):

```bash
ollama pull qwen2.5-coder:7b
```

You can use any Ollama-compatible model. To change the model, set
`ai.model` in `~/.archscope/settings.json` or update it in the
Settings page when AI settings are exposed in the UI.

### Verifying the connection

After Ollama is running, ArchScope auto-detects it at
`http://localhost:11434`. Run any analyzer — if a local model is
available, the results page shows an **AI Interpretation** panel below
the deterministic findings. If Ollama is not running or the model is
not pulled, AI interpretation is silently disabled and the rest of the
analysis works normally.

### Windows-specific tips

- **Firewall:** Ollama binds `127.0.0.1:11434` by default. No
  inbound firewall rule is needed unless you changed the bind address.
- **GPU acceleration:** Ollama auto-detects NVIDIA (CUDA) and AMD
  (ROCm) GPUs on Windows. Make sure you have the latest GPU driver
  installed. If GPU is not detected, inference runs on CPU.
- **Running as a service:** The Windows installer registers Ollama as
  a startup service. You can manage it from **Task Manager → Startup
  apps** or via `sc stop ollama` / `sc start ollama` in an
  administrator prompt.
- **Proxy / air-gapped environments:** If the machine has no internet
  access, you can copy model files from another machine. Pull the
  model on an internet-connected machine, then copy the
  `%USERPROFILE%\.ollama\models` directory to the air-gapped host.

For the full AI interpretation design (evidence requirements, prompt
structure, validation rules), see
[`AI_INTERPRETATION.md`](AI_INTERPRETATION.md).

---

## Image export & "Save all charts"

### Per-chart export

Every chart frame (D3 + ECharts + Canvas) shows a download icon in the
top-right of its header. The dropdown matrix:

| Preset | Format | Pixel ratio | Notes |
| --- | --- | --- | --- |
| `PNG · 1x` | PNG | 1 × | Smallest file, screen-resolution. |
| `PNG · 2x` | PNG | 2 × | Default for slide decks. |
| `PNG · 3x` | PNG | 3 × | Print-resolution. |
| `JPEG · 2x` | JPEG | 2 × | Smaller than PNG; useful for email. |
| `SVG (vector)` | SVG | n/a | True vector — best for editing in Figma / Illustrator. ECharts panels and D3-SVG charts are rendered as native SVG; Canvas charts are rasterized to SVG via `html-to-image`. |

The Canvas flamegraph adds a dedicated **"Save PNG"** button that uses
the native `canvas.toDataURL()` path — fastest and pixel-perfect at
the device pixel ratio.

### Batch export

Most analyzer pages add a **"Save all charts"** button next to the
TabsList. Clicking it walks every chart inside the current page, writes
each one as a 2× PNG, and uses a per-page filename prefix
(e.g. `gc-log-…`, `profiler-…`, `thread-dump-multi-…`,
`access-log-…`).

---

## Thread → flamegraph conversion

To inspect a long-running incident as a flamegraph, batch-convert the
dumps into a collapsed file:

```bash
archscope-engine thread-dump to-collapsed \
    --input dump-2025-05-04T10-00.txt \
    --input dump-2025-05-04T10-05.txt \
    --input dump-2025-05-04T10-10.txt \
    --output incident-2025-05-04.collapsed \
    [--format <plugin-id>] \
    [--no-thread-name]
```

What the converter does:

1. Drives the parser registry (Java / Go / Python / Node.js / .NET
   detected automatically; `--format` forces one).
2. Applies all per-language enrichment — CGLIB / Express layer aliases
   / async state machines / framework wrappers are normalized.
3. Reverses each stack so the root frame is on the left (collapsed
   convention).
4. Prepends the (sanitized) thread name as a synthetic root frame so
   threads with the same stack don't merge — pass `--no-thread-name`
   to merge aggressively.
5. Aggregates identical stacks across all input files into the count
   column.

Then feed the result back into the profiler page:

```bash
archscope-engine profiler analyze-collapsed \
    --wall incident-2025-05-04.collapsed \
    --wall-interval-ms 1 \
    --out incident-flame.json
```

The web UI exposes the same conversion via `POST /api/analyzer/execute`
with `type: "thread_dump_to_collapsed"`. The response includes the
`outputPath` written under `~/.archscope/uploads/collapsed/` and the
total `uniqueStacks`.

---

## CLI reference

```bash
# ---------- web server ----------
archscope-engine serve [--host 127.0.0.1] [--port 8765]
                       [--static-dir apps/frontend/dist] [--reload]
                       [--no-dev-cors]

# ---------- access log ----------
archscope-engine access-log analyze --file <log> --format <name> --out <result.json>
    [--max-lines N] [--start-time ISO] [--end-time ISO]

# ---------- profiler ----------
archscope-engine profiler analyze-collapsed --wall <collapsed> --out <result.json>
    [--wall-interval-ms 100] [--elapsed-sec N] [--top-n 20] [--profile-kind wall|cpu|lock]
archscope-engine profiler analyze-flamegraph-svg  --file <svg>  --out <result.json>
archscope-engine profiler analyze-flamegraph-html --file <html> --out <result.json>
archscope-engine profiler analyze-jennifer-csv    --file <csv>  --out <result.json>
archscope-engine profiler diff   --baseline <r1.json> --target <r2.json> --out <diff.json>
                                  [--normalize] [--top-n 50]
archscope-engine profiler export-pprof --input <result.json> --output <profile.pb.gz>
archscope-engine profiler drilldown   ...           # see --help
archscope-engine profiler breakdown   ...

# ---------- GC ----------
archscope-engine gc-log analyze --file <gc.log> --out <result.json> [--top-n 20]

# ---------- JFR ----------
archscope-engine jfr analyze      --file <jfr|jfr.json> --out <result.json>
    [--event-modes cpu,wall,alloc,lock,gc,exception,io,nativemem]
    [--time-from <ts>] [--time-to <ts>] [--thread-state RUNNABLE,BLOCKED,...]
    [--min-duration-ms N] [--top-n 20]
archscope-engine jfr analyze-json --file <jfr.json> --out <result.json> [--top-n 20]

# ---------- thread dump ----------
archscope-engine thread-dump analyze       --file <dump> --out <result.json>
archscope-engine thread-dump analyze-multi --input <f> --input <f> ... --out <multi.json>
    [--format <plugin-id>] [--consecutive-threshold 3] [--top-n 20]
archscope-engine thread-dump to-collapsed  --input <f> --input <f> ... --output <collapsed>
    [--format <plugin-id>] [--no-thread-name]

# ---------- exception / language stacks ----------
archscope-engine exception        analyze --file <ex>    --out <result.json>
archscope-engine nodejs           analyze --file <stack> --out <result.json>
archscope-engine python-traceback analyze --file <stack> --out <result.json>
archscope-engine go-panic         analyze --file <stack> --out <result.json>
archscope-engine dotnet           analyze --file <stack> --out <result.json>

# ---------- OpenTelemetry ----------
archscope-engine otel analyze --file <events.jsonl> --out <result.json>

# ---------- reports ----------
archscope-engine report html --input <result.json> --out <report.html> [--title "..."]
archscope-engine report pptx --input <result.json> --out <report.pptx> [--title "..."]
archscope-engine report diff --before <before.json> --after <after.json>
                             --out <diff.json> [--label "..."] [--html-out <diff.html>]

# ---------- demo bundles ----------
archscope-engine demo-site mapping [--manifest-root <dir>]
archscope-engine demo-site run     --manifest-root <dir> --out <bundle-dir>
                                   [--scenario name] [--data-source real|synthetic] [--no-pptx]
```

`archscope-engine --help` lists every command; each subcommand also
supports `--help` for its own flags.

---

## Troubleshooting FAQ

**The browser opens to "ArchScope API is running" instead of the UI.**
You started the engine without a built React bundle. Either run
`./scripts/serve-web.sh`, or run `npm --prefix apps/frontend run build`
once and pass `--static-dir apps/frontend/dist` to `archscope-engine
serve`. The dev loop (`archscope-engine serve --reload` plus `npm run
dev`) opens the UI on `:5173` instead.

**`UNKNOWN_THREAD_DUMP_FORMAT` when uploading a thread dump.**
The first 4 KB of the file did not match any registered parser. Check
that you uploaded the dump itself and not its enclosing log file. If
the dump is a known format with the header stripped, type the
`format_id` (e.g. `java_jstack`) into the **Format override** input on
the Thread Dump page.

**`MIXED_THREAD_DUMP_FORMATS` when running multi-dump analysis.**
Two of the uploaded files resolved to different parsers. Either remove
the odd one out or set the **Format override** to the parser you want
applied to every file.

**The flamegraph is sluggish even with the SVG renderer.**
Switch the data into a thread-dump-style bundle (run
`thread-dump to-collapsed`) and feed that as `flamegraph collapsed`.
With ≥ 4 000 nodes the page automatically renders via Canvas. As an
extra simplification, raise **Min width** in the display toolbar to
e.g. 0.5 % so frames below that ratio collapse into a `…` aggregator.

**Binary `.jfr` analyzer reports `JFR_CLI_NOT_FOUND`.**
ArchScope shells out to the JDK `jfr` CLI to convert `.jfr` →
events JSON. Install JDK 11+, set `JAVA_HOME` (so `JAVA_HOME/bin/jfr`
resolves), or point `ARCHSCOPE_JFR_CLI` directly at the executable.
Alternatively, pre-convert with `jfr print --json recording.jfr > out.json`
and analyze that JSON instead.

**Image export downloads a blank PNG.**
The chart hadn't finished rendering. Wait until the data settles
(charts use a `ResizeObserver`) and try again. For very large Canvas
flamegraphs, prefer the dedicated **Save PNG** button — it uses
`canvas.toDataURL()` directly and avoids `html-to-image` rasterization.

**The dark theme looks broken on the legacy ECharts panels.**
Toggle the ECharts theme in **Settings → Default chart theme**. The
ECharts panels do not pick up the global theme automatically; only the
new D3 charts do.

**AI interpretation panel does not appear after analysis.**
Ollama is either not running or the configured model is not pulled.
Open a terminal and run `ollama list` to check available models. If
the list is empty, run `ollama pull qwen2.5-coder:7b`. On Windows,
check that the Ollama service is running in Task Manager or start it
with `ollama serve` in PowerShell. ArchScope checks
`http://localhost:11434` — if Ollama is bound to a different address,
update `ai.provider_url` in `~/.archscope/settings.json`.

**AI interpretation is very slow on Windows.**
If Ollama is running in CPU-only mode (no GPU detected), a 7B model
can take 30+ seconds per interpretation. Install the latest NVIDIA or
AMD GPU driver so Ollama can offload to the GPU. Alternatively, use a
smaller model such as `qwen2.5-coder:3b` (less accurate but faster).
You can also increase the timeout in `~/.archscope/settings.json` by
setting `ai.timeout_seconds` to a higher value (default: 30).

**Where do my uploads live?**
`~/.archscope/uploads/<uuid>/<original-name>`. Delete the directory
whenever you want; ArchScope re-creates it on the next upload.

**How do I expose the engine to my coworkers on the LAN?**
Pass `--host 0.0.0.0`. Be aware: the engine has no auth — only do this
on a trusted network. Never put it on the open internet.

---

## Browser support matrix (T-209)

ArchScope is a localhost web app. After T-207/T-208, it serves the
React UI from FastAPI, so the browser running the UI also has to honor
the contract decided in T-206 (`POST /api/files/select`,
`WebSocket /ws/progress`, drag-drop with `File.path` fallback, ECharts +
D3 rendering, image export via `html-to-image` / `canvas.toDataURL`).
This section is the **smoke matrix** the project signs off on for each
release; rows marked `☐` have not yet been verified on that browser.

### Test setup

1. Install the wheel: `pip install archscope`
   (or build from source: `./scripts/build-archscope-wheel.sh`).
2. Start the server: `archscope serve`. It opens
   `http://127.0.0.1:8765` in the default browser.
3. Repeat the manual checks below in each target browser. Open
   `archscope serve --no-browser` so you can paste the URL into the
   specific browser instead of relying on the OS default.
4. Record the result by changing `☐ Not tested` to `✅ OK`,
   `⚠️ <caveat>`, or `❌ <bug + linked issue>`.

The fixtures used for analyzer end-to-end checks live under
`examples/profiler/`, `examples/access-log/`, and
`examples/thread-dump/`. Any reasonably small file works.

### Target browsers

| Engine | Latest channel target | Min version |
|---|---|---|
| Chromium (Chrome / Edge / Opera / Brave / Arc) | latest stable | 120 |
| WebKit (Safari, macOS / iOS) | latest stable | Safari 17.4 / iOS Safari 17.4 |
| Gecko (Firefox / Firefox ESR) | latest stable | 122 |

Older versions are not actively tested. The lower bound is set by ECharts
6 + the modern ESM bundle Vite 5 produces.

### Verification matrix

Each cell records the result on a fresh profile/session. `☐ Not tested`
means a release is blocked from claiming support on that browser until
someone fills it in.

| Check | Chrome | Edge | Firefox | Safari |
|---|---|---|---|---|
| Static UI loads at `/` (no console errors) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| File picker (`Pick file` button → server-side path) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Drag-and-drop a file onto the drop zone | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Multipart upload fallback (`/api/upload`) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Run one analyzer end-to-end (Profiler collapsed) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Tab navigation (Summary / Flamegraph / Charts / Drill-down / Diagnostics) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| WebSocket `/ws/progress` connects + receives `ready` | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Cancel button signals `analyze:cancelled` | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| ECharts panel renders (Access Log request rate) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| D3 SVG flamegraph (small profile) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Canvas flamegraph auto-switch (≥4 000 nodes) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| D3 timeline + bar charts | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Save image PNG 1× / 2× / 3× | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Save image JPEG 2× | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Save image SVG (vector) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| "Save all charts" batch export | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| pprof `.pb.gz` download | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| HTML report download (Export Center) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| PPTX report download (Export Center) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Locale switch en ↔ ko | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Theme toggle light / dark / system | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Sidebar collapse + persistence | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Keyboard shortcuts (`/` for search, etc.) | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Settings page persists across reload | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |
| Demo Data Center runs a scenario | ☐ Not tested | ☐ Not tested | ☐ Not tested | ☐ Not tested |

### Known browser caveats

This section is filled in as bugs surface. Each entry should link the
issue, the affected browser/version, the workaround (if any), and the
fix-or-defer disposition.

- **Chrome** — *(none recorded yet)*
- **Edge** — *(none recorded yet)*
- **Firefox** — *(none recorded yet)*
- **Safari** — *(none recorded yet)*

### How to record a result

The matrix above lives in `docs/en/USER_GUIDE.md` and `docs/ko/USER_GUIDE.md`.
When a row passes:

1. Edit the cell to `✅ OK` plus the version you tested
   (e.g. `✅ Chrome 134`).
2. Commit the doc edit on the same PR that fixes the underlying bug,
   or as a standalone documentation commit if no code changed.

When a row fails:

1. File an issue and edit the cell to
   `❌ #<issue> — short reason`.
2. Add a "Known browser caveats" entry above with the workaround.

A release is allowed to ship with `⚠️` rows (caveat noted) but not with
`❌` rows on Chrome / Edge / Safari. Firefox `❌` rows are documented
but do not block the release because the project tier-1 platforms are
the macOS-default Safari and the cross-platform Chromium family.
