# ArchScope Profiler Core

Native Go profiler core inside the unified `apps/engine-native` module.
The CLI and Wails desktop app share this package.

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

The standalone profiler module has been consolidated into
`apps/engine-native`. Current open work is tracked in root
`work_status.md` under the Go/Wails release path.
