# ArchScope Profiler — Native Build (Wails v3)

A self-contained desktop profiler that runs the same analyzers as the web
build but ships as a single ~10 MB executable. Lives under
`apps/profiler-native/`.

## What you get

- **5 input formats** — async-profiler `*.collapsed`, Jennifer APM
  flamegraph CSV, FlameGraph SVG (Brendan default + icicle), async-profiler
  self-contained HTML, inline-SVG-wrapped HTML.
- **Drill-down engine** — include / exclude / regex include / regex
  exclude × anywhere / ordered (`a > b > c`) / subtree × preserve full
  path / re-root at match. ReDoS-safe (RE2 + 500-char pattern cap).
- **Profiler diff** — A/B comparison with optional totals normalisation,
  divergent flamegraph palette (red regression / green improvement /
  gray unchanged), top-30 increase / decrease tables.
- **pprof export** — `.pb.gz` compatible with `go tool pprof`.
- **Portable parser debug log** — single redacted JSON artifact a field
  user can ship; mirrors the Python implementation's shape so cross-engine
  debug logs can be diffed.
- **Light / dark / system theme**, **en/ko locale**, **drag-and-drop file
  input**, **Recent files (last 5, persisted)**, **collapsible sidebar**,
  **Settings page** (defaults for sample interval / top-N / profile kind).
- **Async analyze with cancel** — long runs can be aborted from the
  spinner overlay.

## Install (developer mode)

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.84
cd apps/profiler-native/cmd/archscope-profiler-app
npm install --prefix frontend
wails3 task package
open bin/archscope-profiler.app   # macOS
# Linux: ./bin/archscope-profiler
# Windows: bin\archscope-profiler.exe
```

The CI matrix in `.github/workflows/profiler-native.yml` produces ready-to-
download artifacts per push (`archscope-profiler-darwin-arm64` /
`darwin-amd64` / `windows-amd64` / `linux-amd64`).

## System prerequisites

| Platform | Required |
|---|---|
| macOS 12+ | None — WKWebView ships with the OS. |
| Windows 11 | None — WebView2 ships with the OS. |
| Windows 10 | WebView2 Evergreen runtime (Microsoft installer; the NSIS bootstrapper handles this automatically). |
| Linux | `libgtk-3-dev` + `libwebkit2gtk-4.1-0` (Debian/Ubuntu) or `webkit2gtk4.1` (Fedora). |

## Layout

```
Sidebar (collapsible)        Topbar (theme + locale)
├ Profiler  ─┐
├ Compare   │  File strip (path + format + interval/top-N/elapsed/profile-kind)
└ Settings  │  Tabs: Summary · Flamegraph · Charts · Drill-down · Diagnostics
```

Tabs auto-disable until the first analysis succeeds; Diagnostics shows a
red badge when warnings or errors are present.

## Profiler tab

1. Drop a file on the window or click **Pick file** (or paste an absolute
   path). Format auto-detects from extension; override with the dropdown.
2. Tweak sample interval, elapsed seconds, top-N, profile kind, optional
   timeline base method.
3. Click **Analyze**. The spinner overlay surfaces a **Cancel** button
   that aborts the underlying goroutine task; the option strip locks
   during a run and auto-collapses on success (use **Edit options** /
   **Hide options** to control visibility).

### Summary

- Total samples, estimated seconds, interval (ms), elapsed (s),
  profile kind, parser.
- Timeline scope card — mode / base method / match mode / view mode /
  base samples / base ratio / warnings (e.g. `TIMELINE_BASE_METHOD_NOT_FOUND`).
- Top child frames table.
- Top stacks (15 entries with internal scrolling).

### Flamegraph

Canvas-rendered, HiDPI-aware, click-to-zoom (with **Reset zoom**), hover
tooltip, **Save PNG** via `canvas.toDataURL`.

### Charts

- Execution breakdown horizontal bars (samples + ratio per category).
- Timeline analysis horizontal bars (segment ratio).

### Drill-down

Add a filter (pattern + filter type + match mode + view mode +
case-sensitive). Stages render breadcrumb + filter chips (with remove
buttons) + stage metrics + a per-stage Canvas flamegraph.

Filter types: `include_text`, `exclude_text`, `regex_include`,
`regex_exclude`. Match modes: `anywhere`, `ordered` (a > b > c),
`subtree`. View modes: `preserve_full_path`, `reroot_at_match`.

### Diagnostics

- Aggregated counts + skipped-reason chip cloud.
- Severity-coded sample list (errors / warnings / samples) with raw line
  preview.
- **Save debug log** writes a portable redacted JSON under
  `<cwd>/archscope-debug/`; the verdict ladder is
  `CLEAN → PARTIAL_SUCCESS → MAJORITY_FAILED → FATAL_ERROR`.

## Compare tab

Pick two files (any combination of the 5 formats), optional **Normalize
totals**, **Run diff**. Output: divergent flamegraph + summary card with
biggest regression / improvement, top-10 increase / decrease tables.

## Settings tab

- Language (English / 한국어).
- Theme (Light / Dark / System) — listens to `prefers-color-scheme`.
- Default sample interval / top-N / profile kind (persisted under
  `archscope.profiler.defaults` in `localStorage`).
- Recent files viewer with **Clear**.

## Export targets

- **PNG** — flamegraph (Save PNG button on the Canvas).
- **pprof `.pb.gz`** — Export as pprof button on the Profiler tab strip.
  Open with `go tool pprof bin/archscope-profiler-export.pb.gz`.
- **Portable debug log** — Save debug log button on the Diagnostics tab.

## CLI

The same Go core is exposed via `cmd/archscope-profiler` for scripting and
parity testing:

```bash
go run ./cmd/archscope-profiler \
  --collapsed examples/profiler/sample-wall.collapsed \
  --interval-ms 100 --elapsed-sec 1336.559 \
  --timeline-base-method Job.execute \
  --top-n 20 \
  --debug-log --debug-log-dir /tmp/archscope-debug
```

`--collapsed`, `--jennifer-csv`, `--flamegraph-svg`, `--flamegraph-html`
are mutually exclusive.

## Troubleshooting

| Symptom | Fix |
|---|---|
| macOS "executable is missing" on launch | The `.app` bundle's `CFBundleExecutable` must match the binary in `Contents/MacOS/`. Re-run `wails3 task package` after editing `build/config.yml` (or run `wails3 task common:update:build-assets` to regenerate `Info.plist`). |
| Windows 10 launch error mentioning WebView2 | Install the WebView2 Evergreen runtime (or use the NSIS installer which bundles the bootstrapper). |
| Linux launch fails with `libwebkit2gtk-4.1.so.0 not found` | `apt install libwebkit2gtk-4.1-0` (Debian/Ubuntu) or `dnf install webkit2gtk4.1` (Fedora). |
| Drag-drop doesn't pre-fill the path | Some webviews don't expose `File.path`; the drop falls back to the filename. Click **Pick file** instead, or paste the path. |
| Browser quarantine on macOS (`xattr com.apple.quarantine`) | `xattr -dr com.apple.quarantine /path/to/archscope-profiler.app`. |

## Cross-references

- Multi-platform packaging — `apps/profiler-native/cmd/archscope-profiler-app/PACKAGING.md`.
- CI matrix — `.github/workflows/profiler-native.yml`.
- Shared analyzers (Python parity reference) —
  `engines/python/archscope_engine/analyzers/profiler_*.py`.
- Roadmap — `work_status.md` "Go/Native Profiler Follow-up" section.
