# ArchScope (English)

[한국어](./README.ko.md) · [Top-level README](./README.md)

ArchScope is now a **Go/Wails local desktop application**. It analyzes
operational evidence and produces normalized `AnalysisResult` payloads,
charts, diagnostics, and report exports without sending data to a remote
service.

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
| Profiler | collapsed stacks, Jennifer CSV, FlameGraph SVG/HTML, drill-down, execution breakdown, timeline analysis, diff flamegraph, pprof export |
| JVM | GC log, JFR, native memory, Java thread dumps, lock contention, multi-dump correlation |
| Multi-runtime | Go goroutine, Python dump/traceback, Node.js diagnostic reports, .NET clrstack/environment stacktrace |
| Logs | Access logs, exception stacks, OpenTelemetry logs |
| Export | JSON, CSV, HTML report, PPTX, report diff |
| AI | Evidence-bound local interpretation helpers, local Ollama only |

## Docs

- [Architecture](docs/en/ARCHITECTURE.md)
- [Native app guide](docs/en/NATIVE_APP.md)
- [AI interpretation](docs/en/AI_INTERPRETATION.md)
- [Multi-language thread dumps](docs/en/MULTI_LANGUAGE_THREADS.md)

## Local-first

The desktop app and CLIs run locally. Optional AI interpretation is
designed for a localhost Ollama endpoint and validates evidence
references before accepting generated findings.
