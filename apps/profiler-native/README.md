# ArchScope Profiler Native

Native Go profiler-first slice of ArchScope. Two binaries share the same core.

## Layout

- `internal/profiler/` — analyzer core (collapsed parser, Jennifer CSV parser,
  flame tree, drill-down, breakdown, timeline). No GUI dependencies.
- `cmd/archscope-profiler/` — CLI; emits `AnalysisResult` JSON.
- `cmd/archscope-profiler-app/` — Wails v3 desktop app (see its README for
  build/run/size).

The GUI framework (Wails v3) was decided 2026-05-05; see the Active Decision
Queue in `work_status.md`.

## CLI quick start

```bash
go run ./cmd/archscope-profiler \
  --collapsed ../../examples/profiler/sample-wall.collapsed \
  --interval-ms 100 \
  --elapsed-sec 1336.559 \
  --timeline-base-method Job.execute

go run ./cmd/archscope-profiler \
  --jennifer-csv ../../examples/profiler/sample-jennifer-flame.csv \
  --interval-ms 100 \
  --elapsed-sec 11.5
```

`--collapsed` and `--jennifer-csv` are mutually exclusive; auto-detection lives
in the GUI.

## Tests

```bash
go test ./internal/...
```

## Roadmap

Tracked in root `work_status.md` under "Go/Native Profiler Follow-up". Active
items: T-239b (SVG flamegraph parser), T-239c (HTML profiler parser),
T-239d (drill-down engine), T-240a–f (UI shell, i18n, dark mode, flamegraph,
charts, UX feedback), T-241 (parity tests/benchmarks), T-243 (multi-platform
packaging), T-244 (CI matrix).
