# ArchScope Native App

The native app now lives inside the unified Go engine module:

`apps/engine-native/cmd/archscope-app`

The former native POC module has been folded into
`apps/engine-native/internal/profiler` and the Wails app command tree.

ArchScope ships as a **Wails 3.1.0 native desktop application** on macOS,
Windows and Linux. Start development from the repository root with
`scripts/dev-desktop.sh` (macOS/Linux) or `scripts/dev-desktop.ps1` (Windows).
These launch the native Wails window; Vite only supplies development assets.
Opening the frontend URL in a browser does not initialize the product UI.

Server/Docker hosting, browser preview, Electron shells and mobile targets are
unsupported. The server/mobile build tags fail compilation. Cross-compilation
Docker tooling remains a build tool for native desktop artifacts. WebKit,
WebView2 and WebKitGTK are required native rendering components.

Run `node scripts/verify-desktop-only.mjs --check-server-build` from the root
before packaging. SDK build-asset updates must be reviewed in a temporary
project and ported into the desktop platform files; do not regenerate all
templates over this customized project. Retired Python/browser sources under
`archive/` remain reference-only and are not packaged.

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
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
cd apps/engine-native/cmd/archscope-app
task package
```

## Features

- Profiler inputs: collapsed stacks, Jennifer CSV, FlameGraph SVG/HTML,
  async-profiler HTML, pprof, py-spy, rbspy, speedscope/dotnet-trace, perf
  collapsed, StackProf, PHP profiler exports, Xdebug, Swift/async stacks,
  Pyroscope/Phlare, and Parca snapshots.
- Analyzer inputs: access and edge logs, server logs, OpenTelemetry logs,
  metrics snapshots, observability exports, database slow-query evidence,
  broker logs, Kubernetes/container/cloud evidence, trace imports, GC logs,
  JFR JSON, native memory, exception stacks, and multi-runtime thread dumps.
- Derived workflows: Analysis Workspace, Evidence Board, Incident Timeline,
  SLO/Golden Signals, Service Flow, stitched evidence drilldown, API/event
  contract analysis, and evidence-backed architecture documentation drafts.
- Drill-down, execution breakdown, timeline analysis, profiler diff,
  pprof export, parser diagnostics, debug logs, chart export, report diff,
  evidence packs, and report-pack ZIP export.
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

Recent evidence workflows:

```bash
go run ./cmd/archscope-engine trace import \
  --in ../../examples/traces/sample-otlp-traces.jsonl \
  --format auto \
  --out trace.json

go run ./cmd/archscope-engine stitch analyze \
  --in ../../examples/stitching/access-result.json \
  --in ../../examples/stitching/trace-result.json \
  --in ../../examples/stitching/database-result.json \
  --time-window-seconds 60 \
  --out stitched.json

go run ./cmd/archscope-engine api-contract analyze \
  --openapi ../../examples/api-contract/openapi-orders.json \
  --access-result ../../examples/api-contract/access-result.json \
  --asyncapi ../../examples/api-contract/asyncapi-orders.json \
  --broker-result ../../examples/api-contract/broker-result.json \
  --out contract.json

go run ./cmd/archscope-engine architecture-docs draft \
  --in contract.json --in stitched.json \
  --out architecture-docs.json
```

Use `go run ./cmd/archscope-engine --help` and the
[Importer Support Matrix](./IMPORTER_SUPPORT_MATRIX.md) for the current command
surface.

## CI

`.github/workflows/engine-native.yml` now validates the unified
`apps/engine-native` module and the Wails frontend build.
