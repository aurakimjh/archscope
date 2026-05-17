# ArchScope (English)

[한국어](./README.ko.md) · [Top-level README](./README.md)

ArchScope is now a **Go/Wails local desktop application**. It analyzes
operational evidence and produces normalized `AnalysisResult` payloads,
charts, diagnostics, contract/risk views, architecture-documentation drafts,
and report exports without sending data to a remote service.

## Active Stack

- `apps/engine-native/` — Go engine, parsers, analyzers, exporters,
  profiler core, Cobra CLIs, and Wails app.
- `apps/engine-native/cmd/archscope-engine` — headless CLI for analysis,
  CI, and scripted workflows.
- `apps/engine-native/cmd/archscope-app` — Wails desktop app.
- `archive/python-engine/` and `archive/web-frontend-python/` — retired
  Python/FastAPI/browser implementation kept for reference only.

## Build

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

## Feature Surface

| Domain | Capabilities |
| --- | --- |
| Evidence import | access/edge logs, server logs, OpenTelemetry logs, metrics snapshots, observability exports, database slow-query evidence, broker logs, Kubernetes/container/cloud audit evidence, trace imports |
| Runtime diagnostics | GC logs, JFR, native memory, Java thread dumps, lock contention, multi-dump correlation, exception stacks, Node.js/Python/Go/.NET runtime stack evidence |
| Profiling | collapsed stacks, Jennifer CSV, FlameGraph SVG/HTML, pprof, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, StackProf, PHP profiler exports, Xdebug, Swift/async stacks, Pyroscope/Phlare, Parca |
| Evidence Studio | Analysis Workspace, Evidence Board, Incident Timeline, SLO/Golden Signals, Service Flow, stitched-evidence drilldowns, API/event contract analysis, architecture docs drafts |
| Export | JSON, CSV, HTML report, PPTX, report diff, chart export, evidence packs, report-pack ZIP |
| AI | Evidence-bound local interpretation helpers, redaction, evidence-reference validation, local Ollama only |

## Docs

- [Architecture](docs/en/ARCHITECTURE.md)
- [Native app guide](docs/en/NATIVE_APP.md)
- [AI interpretation](docs/en/AI_INTERPRETATION.md)
- [Multi-language thread dumps](docs/en/MULTI_LANGUAGE_THREADS.md)
- [Importer support matrix](docs/en/IMPORTER_SUPPORT_MATRIX.md)
- [Data model](docs/en/DATA_MODEL.md)

## Local-first

The desktop app and CLIs run locally. Optional AI interpretation is
designed for a localhost Ollama endpoint and validates evidence
references before accepting generated findings.
