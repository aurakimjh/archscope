# ArchScope Profiler Native POC

This workspace is the Go/Fyne-native profiler experiment.

The first slice intentionally has no external dependency: it ports the
collapsed profiler analyzer core and CLI so output can be compared with the
Python `profiler_collapsed` result contract before Fyne widgets are added.

## Run

```powershell
go run ./cmd/archscope-profiler `
  --collapsed ..\..\examples\profiler\sample-wall.collapsed `
  --interval-ms 100 `
  --elapsed-sec 1336.559 `
  --timeline-base-method Job.execute
```

## Next

- add Fyne UI under `cmd/archscope-profiler-app`
- port SVG / HTML / Jennifer CSV inputs
- port drill-down, diff, and pprof export
- add golden JSON parity tests against the Python engine
