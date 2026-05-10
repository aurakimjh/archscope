# ArchScope Profiler Core

Native Go profiler core inside the unified `apps/engine-native` module.
The unified engine CLI and Wails desktop app share this package.

## Layout

- `internal/profiler/` — analyzer core (collapsed parser, Jennifer CSV parser,
  flame tree, drill-down, breakdown, timeline). No GUI dependencies.
- `cmd/archscope-engine/` — unified CLI; `profiler` subcommands emit
  `AnalysisResult` JSON.
- `cmd/archscope-app/` — Wails v3 desktop app (see its README for
  build/run/size).

The GUI framework (Wails v3) was decided 2026-05-05; see the Active Decision
Queue in `work_status.md`.

## CLI quick start

```bash
go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --interval-ms 100 \
  --elapsed-sec 1336.559 \
  --timeline-base-method Job.execute

go run ./cmd/archscope-engine profiler analyze-jennifer-csv \
  --in ../../examples/profiler/sample-jennifer-flame.csv \
  --interval-ms 100 \
  --elapsed-sec 11.5
```

Input auto-detection lives in the GUI.

## Tests

```bash
go test ./internal/...
```

## Roadmap

The standalone profiler module has been consolidated into
`apps/engine-native`. Current open work is tracked in root
`work_status.md` under the Go/Wails release path.
