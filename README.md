# ArchScope

[한국어](./README.ko.md) · [English](./README.en.md)

ArchScope is a local-first architecture diagnostics toolkit for access
logs, GC logs, profiler outputs, JFR recordings, exception stacks, and
thread dumps across Java, Go, Python, Node.js, and .NET.

The active product line is now **Go + Wails** under
`apps/engine-native/`. The former Python/FastAPI/web implementation is
preserved under `archive/` for reference and migration audits only.

## Quick Start

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-profiler

cd cmd/archscope-profiler-app/frontend
npm ci
npm run build
```

Desktop packaging:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.84
cd apps/engine-native/cmd/archscope-profiler-app
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

- Profiler: async-profiler collapsed, FlameGraph SVG/HTML, Jennifer APM
  CSV, drill-down, diff flamegraph, pprof export.
- Logs: access log, GC log, exception stack, OpenTelemetry log analysis.
- Runtime diagnostics: JFR, native-memory, multi-runtime thread dumps,
  lock contention, multi-dump correlation, thread-dump to collapsed.
- Exporters: JSON, CSV, HTML report, PowerPoint, before/after report diff.
- AI interpretation: evidence-bound local interpretation helpers ported
  to Go under `internal/aiinterpretation`; local Ollama execution only,
  off by default.

## Documentation

- [English documentation index](docs/en/README.md)
- [Korean documentation index](docs/ko/README.md)
- [Architecture](docs/en/ARCHITECTURE.md) · [아키텍처](docs/ko/ARCHITECTURE.md)
- [Native app guide](docs/en/PROFILER_NATIVE.md) · [네이티브 앱 가이드](docs/ko/PROFILER_NATIVE.md)

## Privacy

All parsing, analysis, exporting, and optional AI interpretation run
locally. The Go desktop app does not send input files to remote services.

## License

MIT — see [LICENSE](./LICENSE).
