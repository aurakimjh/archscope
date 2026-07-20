# engine-native

`apps/engine-native` is the active ArchScope implementation. It contains
the Go analysis engine, the consolidated profiling core, the Cobra CLI,
and the Wails desktop app.

The retired Python implementation has been moved to `archive/`; it is no
longer the shipping path.

## Layout

```text
apps/engine-native/
  api/                         Wails service API bindings
  cmd/
    archscope-engine/          Headless Cobra CLI for all analyzers
    archscope-app/             Wails desktop app + React frontend
  internal/
    aiinterpretation/          Evidence-bound local AI interpretation
    analyzers/                 Access log, GC, JFR, thread dump, etc.
    demosite/                  Demo manifest runner
    diagnostics/               Parser diagnostics contract
    exporters/                 JSON, CSV, HTML, PPTX, report diff
    models/                    Common AnalysisResult envelope
    parsers/                   Input parsers
    profiler/                  Collapsed/SVG/HTML/Jennifer profiler core
    threaddump/                Multi-runtime thread dump registry/plugins
```

## Build And Test

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

Desktop packaging requires the Wails v3 CLI and Task:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.117
brew install go-task
cd apps/engine-native/cmd/archscope-app
GOCACHE=/tmp/aiservice-go-cache task package
```

Current local packaging baseline (2026-07-21):

- `task` 3.50.0
- `wails3` v3.0.0-alpha2.117
- `npm audit`: 0 vulnerabilities after Vite 8 / React plugin 6 update
- `bin/archscope`: 13.2 MiB
- `bin/archscope.app`: 15.0 MiB

## Notes

- `internal/models.AnalysisResult` remains the common engine/UI
  contract.
- `internal/profiler.AnalysisResult` is preserved for the profiler core
  and is serialized directly by profiler CLI commands.
- AI interpretation is implemented under `internal/aiinterpretation` and
  accepts only localhost LLM URLs.
- `.github/workflows/engine-native.yml` now validates this unified Go
  module and Wails frontend.
