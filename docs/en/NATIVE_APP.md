# ArchScope Native App

The native app now lives inside the unified Go engine module:

`apps/engine-native/cmd/archscope-app`

The former native POC module has been folded into
`apps/engine-native/internal/profiler` and the Wails app command tree.

## Build

```bash
cd apps/engine-native
go test ./...

cd cmd/archscope-app/frontend
npm ci
npm run build
```

Package the desktop app:

```bash
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
cd apps/engine-native/cmd/archscope-app
task package
```

## Features

- Profiler inputs: collapsed stacks, Jennifer CSV, FlameGraph SVG, and
  async-profiler/inline-SVG HTML.
- Analyzer inputs: access logs, GC logs, JFR, exception stacks,
  OpenTelemetry logs, and multi-runtime thread dumps.
- Drill-down, execution breakdown, timeline analysis, profiler diff,
  pprof export, parser diagnostics, and debug logs.
- Light/dark/system theme, Korean/English locale, recent files, and
  cancellable async analysis.

## CLI

Unified engine CLI:

```bash
cd apps/engine-native
go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out result.json
```

## CI

`.github/workflows/engine-native.yml` now validates the unified
`apps/engine-native` module and the Wails frontend build.
