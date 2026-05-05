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
8. [Exception / JFR analyzers](#exception--jfr-analyzers)
9. [Demo Data Center](#demo-data-center)
10. [Export Center](#export-center)
11. [Chart Studio](#chart-studio)
12. [Settings](#settings)
13. [Image export & "Save all charts"](#image-export--save-all-charts)
14. [Thread → flamegraph conversion](#thread--flamegraph-conversion)
15. [CLI reference](#cli-reference)
16. [Troubleshooting FAQ](#troubleshooting-faq)

---

## Install & run

### Requirements

| Item | Required |
| --- | --- |
| OS | macOS / Linux / Windows |
| Python | 3.9 or newer |
| Node.js | 18+ (only when building the UI from source) |
| RAM | 4 GB minimum, 8 GB recommended for large flamegraphs |
| Disk | ~500 MB |

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
cd apps/desktop
npm install
npm run build                      # produces apps/desktop/dist
cd ../..
archscope-engine serve --static-dir apps/desktop/dist
```

Open `http://127.0.0.1:8765`. ArchScope binds `127.0.0.1` by default;
pass `--host 0.0.0.0` only if you knowingly want to expose it on a
trusted network.

### Hot-reload development loop

```bash
# Terminal 1 — auto-reloading FastAPI engine
archscope-engine serve --reload

# Terminal 2 — Vite dev server (proxies /api → :8765)
cd apps/desktop && npm run dev     # http://127.0.0.1:5173
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
| Summary | Total requests · avg / p95 response · error rate metric cards. |
| Charts | Request-count trend (ECharts panel — supports the same image-export dropdown as D3 charts). |
| Top URLs | Sorted shadcn table of the slowest URIs (URI · count · avg ms). |
| Diagnostics | Parser diagnostics — counts of skipped lines and a sample of failed lines for debugging custom formats. |

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
| Flame Graph | The interactive flamegraph. **SVG renderer** is the default; once the tree exceeds **4 000 nodes** the page automatically swaps to the **Canvas renderer** so converted thread-dump bundles stay smooth. Click a frame to zoom in; "Reset zoom" appears when you're not at the root. Hover any frame for a tooltip with sample count + ratio + category. |
| Drill-down | Stack-frame filters with `include_text` / `exclude_text` / `regex_include` / `regex_exclude`, three match modes (`anywhere` / `ordered` / `subtree`), and two view modes (`preserve_full_path` / `reroot_at_match`). Apply to push a new stage; Reset returns to the original tree. The current stage's drill-down breadcrumb appears at the top. |
| Timeline Analysis | Stacked composition and evidence table that converts flamegraph samples into an estimated execution composition: startup/framework, internal methods, SQL, DB network wait, external call, external network wait, pool wait, lock wait, and other waits. It is sample-based composition, not timestamp-ordered tracing. |
| Breakdown | ECharts donut + bar showing how samples distribute across "execution categories" (SQL / external API / network I/O / connection-pool wait / others). The breakdown table shows the top method per category. |
| Top Stacks | Sorted table of the top N stacks (samples · estimated seconds · ratio). |
| Diagnostics | Parser diagnostics. |

### Saving the flamegraph

- **SVG renderer** — the chart frame's "Save image" dropdown supports
  PNG 1×/2×/3×, JPEG 2×, and SVG vector.
- **Canvas renderer** — adds a separate **"Save PNG"** action that uses
  `canvas.toDataURL()` for a pixel-perfect snapshot at the current
  device pixel ratio. The standard dropdown (PNG/JPEG/SVG via
  `html-to-image`) still works.

---

## GC Log Analyzer

### Inputs

Drop a `*.log` / `*.txt` / `*.gz` HotSpot unified GC log into the
FileDock and click **Analyze**. The engine extracts pause / heap /
allocation / promotion timelines plus per-collector counts.

### Tabs

| Tab | What's there |
| --- | --- |
| Summary | 15-stat metric grid (throughput, p50/p95/p99/max/avg/total pause, young/full GC count, allocation/promotion rate, humongous count, concurrent-mode failures, promotion failures) plus a findings card. |
| GC Pauses | **Interactive timeline** with wheel/drag zoom (1× – 80×) and a **brush selection band** below the plot. Brushing populates a 4-stat selection summary card (events in window / avg / p95 / max pause). Full-GC events appear as orange dashed verticals — hover within ~6 px of one to see its `cause`, `pause_ms`, `heap_before_mb`, `heap_after_mb`, `heap_committed_mb`. The **Reset zoom** button appears in the chart frame once you start zooming. The allocation-rate timeline below uses an area fill on `allocation` with a line for `promotion`. |
| Heap Usage | Heap-before (area) + Heap-after (line), with the same Full-GC event markers. |
| Algorithm comparison | Frontend-aggregated table of pause stats per `gc_type` (count / avg / p95 / max / total ms) plus two horizontal D3 bar charts (avg + max). Lets you compare G1Young / G1Mixed / FullGC / ZGC / Shenandoah collectors side by side. |
| Breakdown | D3 horizontal bars for `gc_type_breakdown` and `cause_breakdown`. |
| Events | Up to 200 events shown in a shadcn table (timestamp · uptime · type · cause · pause · heaps). |
| Diagnostics | Parser diagnostics. |

---

## Thread Dump Analyzer

The thread-dump page is the only "multi-file" analyzer in ArchScope.

### Adding files

The sticky FileDock acts as an **adder**: each successful upload appends
to the cumulative file list shown right below. Each row carries a
numeric index, the original file name, the upload size, and an `X`
button to remove it.

You can mix files from any supported runtime (Java, Go, Python, Node.js,
.NET) — the parser registry sniffs the first 4 KB of every file. If two
files resolve to different formats the request fails fast with
`MIXED_THREAD_DUMP_FORMATS`. To force one parser, type its `format_id`
into the **Format override** input (e.g. `java_jstack`) — auto-detection
is skipped.

The **Consecutive-dump threshold** input (default `3`) controls when the
correlator emits the persistence findings.

Click **Correlate dumps** to run the multi-dump analyzer.

### Tabs

| Tab | What's there |
| --- | --- |
| Findings | 6-stat summary card (total dumps / unique threads / long-running / persistent blocked / latency sections / threshold) plus severity-colored cards for every finding. The full **`LONG_RUNNING_THREAD`** and **`PERSISTENT_BLOCKED_THREAD`** tables appear below. |
| Charts | D3 vertical bar chart: thread count per dump. D3 horizontal sorted bar chart: top threads by observation count across all dumps. |
| Per dump | Per-dump state distribution table. |
| Threads | Sorted table of every thread name and how many dumps it appeared in. |

The findings catalog:

- **`LONG_RUNNING_THREAD`** *(warning)* — a thread name kept the same
  RUNNABLE stack for ≥ N consecutive dumps.
- **`PERSISTENT_BLOCKED_THREAD`** *(critical)* — a thread stayed
  BLOCKED / LOCK_WAIT for ≥ N consecutive dumps.
- **`LATENCY_SECTION_DETECTED`** *(warning)* — a thread stayed in
  `NETWORK_WAIT`, `IO_WAIT`, or `CHANNEL_WAIT` for ≥ N consecutive
  dumps. The wait categories are populated by the per-language
  enrichment plugins (e.g. `EPoll.epollWait` → `NETWORK_WAIT`,
  `gopark` → `CHANNEL_WAIT`, `Monitor.Enter` → `LOCK_WAIT`).

For the full per-language enrichment matrix and the detection
signatures, see
[`MULTI_LANGUAGE_THREADS.md`](MULTI_LANGUAGE_THREADS.md).

---

## Exception / JFR analyzers

Both exception-stack and JFR analyzers run on the **PlaceholderPage**
shell — they share the same FileDock + Tabs treatment as the heavier
analyzers but with a smaller surface:

- **Exception** — drops a Java exception stack file (`*.txt` / `*.log`),
  produces summary metrics, an event-style table preview, and parser
  diagnostics.
- **JFR** — drops the JSON output of `jfr print --json recording.jfr`
  and surfaces event distribution, top execution samples, and GC pause
  summary.

Inputs go through the same upload + analyze flow as the other pages.

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
                       [--static-dir apps/desktop/dist] [--reload]
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
archscope-engine profiler drilldown   ...           # see --help
archscope-engine profiler breakdown   ...

# ---------- GC ----------
archscope-engine gc-log analyze --file <gc.log> --out <result.json> [--top-n 20]

# ---------- JFR ----------
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
`./scripts/serve-web.sh`, or run `npm --prefix apps/desktop run build`
once and pass `--static-dir apps/desktop/dist` to `archscope-engine
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
With ≥ 4 000 nodes the page automatically renders via Canvas.

**Image export downloads a blank PNG.**
The chart hadn't finished rendering. Wait until the data settles
(charts use a `ResizeObserver`) and try again. For very large Canvas
flamegraphs, prefer the dedicated **Save PNG** button — it uses
`canvas.toDataURL()` directly and avoids `html-to-image` rasterization.

**The dark theme looks broken on the legacy ECharts panels.**
Toggle the ECharts theme in **Settings → Default chart theme**. The
ECharts panels do not pick up the global theme automatically; only the
new D3 charts do.

**Where do my uploads live?**
`~/.archscope/uploads/<uuid>/<original-name>`. Delete the directory
whenever you want; ArchScope re-creates it on the next upload.

**How do I expose the engine to my coworkers on the LAN?**
Pass `--host 0.0.0.0`. Be aware: the engine has no auth — only do this
on a trusted network. Never put it on the open internet.
