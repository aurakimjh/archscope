# ArchScope

[한국어](./README.ko.md) · [English](./README.en.md)

ArchScope is a local-first architecture diagnostics toolkit for operational
evidence: access and edge logs, server logs, OpenTelemetry logs, database and
broker logs, platform/cloud evidence, metrics snapshots, traces, runtime
profiles, JFR recordings, GC logs, exception stacks, and thread dumps across
Java, Go, Python, Node.js, and .NET.

The active product line is now **Go + Wails** under
`apps/engine-native/`. The former Python/FastAPI/web implementation is
preserved under `archive/` for reference and migration audits only.

## Quick Start

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

Desktop packaging:

```bash
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
cd apps/engine-native/cmd/archscope-app
task package
```

## Current Layout

```text
archscope/
  apps/engine-native/        Go engine, profiler core, Cobra CLIs,
                             and Wails desktop app
  archive/python-engine/     Retired Python engine source
  archive/web-frontend-python/
                             Retired browser frontend source
  docs/{en,ko}/              Current design and user documents
  examples/                  Sample input data
  scripts/                   Go/Wails helper scripts
```

## Capabilities

- Evidence importers: access/edge logs, application and web server logs,
  OpenTelemetry logs, Prometheus/OpenMetrics snapshots, Loki/Tempo/Grafana
  exports, database slow-query evidence, broker logs, Kubernetes/container
  evidence, cloud audit logs, and portable trace exports.
- Runtime diagnostics: GC logs, JFR JSON, native memory, exception stacks,
  multi-runtime thread dumps, lock contention, multi-dump correlation, Node.js,
  Python, Go, and .NET runtime stack evidence.
- Profiling: async-profiler collapsed/HTML, FlameGraph SVG/HTML, Jennifer APM
  CSV, pprof, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, StackProf,
  PHP profiler exports, Xdebug, Swift/async stacks, Pyroscope/Phlare, and Parca
  snapshots.
- Evidence Studio: Analysis Workspace, Evidence Board, Incident Timeline,
  SLO/Golden Signals, Service Flow, stitched evidence drilldowns, API/event
  contract analysis, and evidence-backed architecture documentation drafts.
- Exporters: JSON, CSV, HTML report, PowerPoint, before/after report diff,
  chart exports, evidence packs, and report-pack ZIP bundles.
- AI interpretation: evidence-bound local interpretation helpers under
  `internal/aiinterpretation`; localhost Ollama execution only, off by default,
  with redaction and evidence-reference validation.

## Documentation

- [English documentation index](docs/en/README.md)
- [Korean documentation index](docs/ko/README.md)
- [Architecture](docs/en/ARCHITECTURE.md) · [아키텍처](docs/ko/ARCHITECTURE.md)
- [Native app guide](docs/en/NATIVE_APP.md) · [네이티브 앱 가이드](docs/ko/NATIVE_APP.md)

## Privacy

All parsing, analysis, exporting, and optional AI interpretation run
locally. The Go desktop app does not send input files to remote services.

## License

MIT — see [LICENSE](./LICENSE).
