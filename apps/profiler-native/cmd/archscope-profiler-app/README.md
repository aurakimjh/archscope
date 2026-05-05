# ArchScope Profiler — Native (Wails v3)

Wails v3 desktop app over the dependency-light Go profiler core in
`apps/profiler-native/internal/profiler`.

## Status

T-240a + T-240b + T-240c + T-240d + T-240e + Go drill-down (T-239d) + Go SVG parser (T-239b) landed. Visible UI:

- File picker (OS dialog) wired to `ProfilerService.PickProfileFile`.
- Format selector (collapsed stacks / Jennifer flamegraph CSV / FlameGraph SVG)
  — auto-detected from extension.
- Options: sample interval (ms), elapsed seconds, top N, profile kind, timeline
  base method.
- Summary card (`total_samples`, `estimated_seconds`, `interval_ms`,
  `elapsed_seconds`, `profile_kind`, parser).
- **Canvas-rendered flamegraph** (T-240d) — port of T-217 from
  `apps/frontend/src/components/charts/CanvasFlameGraph.tsx`. HiDPI-aware via
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

Wails v3 alpha.84 must be on `PATH` (`go install
github.com/wailsapp/wails/v3/cmd/wails3@latest` puts it under `$GOPATH/bin`).

```bash
# from this directory
PATH="$(go env GOPATH)/bin:$PATH"

# dev mode (hot reload)
wails3 dev

# release build (single binary at bin/archscope-profiler)
wails3 task build

# .app bundle (mac) at bin/archscope-profiler.app
wails3 task package
```

## Binary size (darwin/arm64, alpha.84)

| Output | Size |
|---|---|
| Raw binary | 8.4 MB (8,858,274 bytes) |
| `.app` bundle (binary + Assets.car + icns + Info.plist + ad-hoc signature) | 10 MB |
| Vite frontend bundle | 228.4 KB raw / 70.0 KB gzipped |

Slice-by-slice growth (raw binary):

| Slice | Raw binary |
|---|---|
| T-240a (first slice) | 8,395,330 B |
| + T-240b + T-240d | 8,411,842 B (+16 KB) |
| + T-239b + T-239d + T-240c + T-240e | 8,544,130 B (+148 KB) |
| + Tabs / lost-info recovery / progress UX (T-240f) | 8,560,642 B (+165 KB) |
| + T-239c HTML / T-239e Diff / T-239f pprof + drag-drop + Sidebar + Settings | **8,858,274 B (+463 KB total)** |

Comparison (single distributable):

- Electron desktop shell (current ArchScope): ~120 MB
- Wails v3 alpha.84 (this slice): **8.0 MB / 9.9 MB**

Smaller than the predicted 12–20 MB target, primarily because this slice
ships only the profiler engine + Vite-bundled React (183 KB raw, 57 KB
gzipped) and Wails v3 leans on the macOS WKWebView for the renderer.
