# ArchScope — Native (Wails v3)

Wails v3 desktop app over the dependency-light Go profiler core in
`apps/engine-native/internal/profiler`.

## Status

T-240a + T-240b + T-240c + T-240d + T-240e + Go drill-down (T-239d) + Go SVG parser (T-239b) landed. Visible UI:

- File picker (OS dialog) wired to `ProfilerService.PickProfileFile`.
- Format selector (collapsed stacks / Jennifer flamegraph CSV / FlameGraph SVG)
  — auto-detected from extension.
- Options: sample interval (ms), elapsed seconds, top N, profile kind, timeline
  base method.
- Summary card (`total_samples`, `estimated_seconds`, `interval_ms`,
  `elapsed_seconds`, `profile_kind`, parser).
- **Canvas-rendered flamegraph** (T-240d). HiDPI-aware via
  `devicePixelRatio`, click-to-zoom (with reset), hover tooltip, "Save PNG" via
  `canvas.toDataURL`. Uses `d3-hierarchy` only (full d3 not bundled).
- Parser diagnostics card (parsed records / skipped / warnings / errors).
- Top stacks card (top 5 leaf stacks with sample ratio).
- **Execution breakdown** + **timeline** bar charts (T-240e) — dependency-free
  SVG-style horizontal bar chart wired to `result.series.execution_breakdown`
  and `result.series.timeline_analysis`.
- **Drill-down panel** (T-239d) — filter pattern + filter type
  (include/exclude/regex include/regex exclude) + match mode
  (anywhere/ordered/subtree) + view mode (preserve full path/reroot) +
  case-sensitive checkbox. Stages render breadcrumb chips, removable filter
  chips, stage metrics, and a per-stage Canvas flamegraph.
- en/ko **i18n** (T-240b) — strings extracted into
  `frontend/src/i18n/messages.ts`, locale persisted in `localStorage`,
  `I18nProvider` context.
- **Light / dark / system theme toggle** (T-240c) — `ThemeProvider` mirrors the
  web app's `useTheme` semantics, listens to `prefers-color-scheme`, persists
  under `archscope.profiler.theme`. CSS variables split into a default
  (light) palette and a `.dark` override for every token.

T-239c (HTML parser), T-239e (Diff), T-239f (pprof exporter), drag-drop +
Recent files, and the sidebar / Settings / Compare pages all landed in this
sweep:

- **HTML profiler** input (T-239c) — async-profiler self-contained HTML +
  inline-SVG-wrapped HTML. Format selector + OS dialog filter updated.
- **Profiler diff** (T-239e) — new "Compare" sidebar page with two file
  pickers, normalize toggle, divergent flamegraph (red regression / green
  improvement / gray unchanged), top-10 increase + decrease tables, full
  diff summary card.
- **pprof export** (T-239f) — "Export as pprof" button on the Profiler tab
  strip writes a `.pb.gz` compatible with `go tool pprof`.
- **Drag-and-drop** + **Recent files** — drop overlay on the whole window;
  recent dropdown chip on the file row, persisted in `localStorage`.
- **Collapsible sidebar** — brand mark + nav (Profiler / Compare / Settings)
  with collapse toggle persisted in `localStorage`.
- **Settings page** — language, theme, default sample interval / top-N /
  profile kind (persisted), recent-files viewer with reset, About blurb.

Earlier:


- 5-tab content layout — Summary / Flamegraph / Charts / Drill-down /
  Diagnostics. Tabs auto-disable when there is no result; Diagnostics shows
  a red badge when warnings or errors are present. Active tab snaps back to
  Summary on every successful analysis.
- Spinner + locked options during analyze; option strip auto-collapses after
  a successful run with an "Edit options" / "Hide options" toggle so the
  result surface gets the room it needs.
- Recovered information that the previous single-column layout was hiding:
  Top child frames table (`result.tables.top_child_frames`), Timeline scope
  card (mode / base method / match mode / view mode / base samples / base
  ratio / warnings), Top stacks expanded 5 → 15 with internal scrolling,
  full Diagnostics tab — counts, skipped-reason chips, and severity-coded
  per-sample lists with the raw line preview.

UI cancellation hook is still UI-only (analyzers are CPU-bound and synchronous
on the Go side); a real cancel path will land alongside an `AnalyzeAsync`
service method.

## Build / run

Wails v3 alpha2.117 and Task must be on `PATH`.

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
brew install go-task

# dev mode (hot reload)
wails3 dev

# .app bundle (mac) at bin/archscope.app.
# Codex/sandboxed runs should set GOCACHE to a writable path.
GOCACHE=/tmp/aiservice-go-cache task package
```

## Binary size (darwin/arm64, alpha2.117)

| Output | Size |
|---|---|
| Raw binary | 13.2 MiB |
| `.app` bundle (binary + Assets.car + icns + Info.plist + ad-hoc signature) | 15.0 MiB |
| Vite startup shell JS | 211.3 KB raw / 66.1 KB gzipped |
| Lazy shared ECharts runtime | 698.8 KB raw / 235.6 KB gzipped |
| npm audit | 0 vulnerabilities |

Comparison (single distributable):

- Electron desktop shell (retired): ~120 MB
- Wails v3 alpha2.117 current app: **13.2 MiB raw / 15.0 MiB `.app`**

The Wails frontend now uses route-level lazy loading. The startup shell stays
small, and the chart runtime is loaded only when a chart-backed analyzer page is
opened.
