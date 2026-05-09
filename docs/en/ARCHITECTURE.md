# Architecture

ArchScope's active architecture is a single Go module plus a Wails
desktop shell.

```text
Wails desktop UI
  apps/engine-native/cmd/archscope-profiler-app/frontend
        |
        v
Wails services
  apps/engine-native/api
  apps/engine-native/cmd/archscope-profiler-app/*service.go
        |
        v
Go engine
  parsers -> analyzers -> exporters -> AnalysisResult
```

## Repository Boundaries

| Path | Status | Responsibility |
| --- | --- | --- |
| `apps/engine-native/` | Active | Go engine, profiler core, CLIs, Wails app |
| `archive/python-engine/` | Retired | Former Python engine, kept for reference |
| `archive/web-frontend-python/` | Retired | Former browser frontend, kept for reference |
| `docs/en`, `docs/ko` | Active | Current docs |

## Contracts

- `internal/models.AnalysisResult` is the common result envelope used by
  non-profiler analyzers and the Wails renderer.
- `internal/profiler.AnalysisResult` is the profiler-specific typed
  envelope. CLI commands serialize it directly; demo-site integration
  converts it into the common envelope shape.
- Parser diagnostics are emitted with stable JSON keys so the UI and
  report exporters can render partial-success and parse-error states.

## AI Interpretation

Evidence-bound AI interpretation has been ported to Go under
`internal/aiinterpretation`.

- Prompts contain selected evidence as data, not hidden instructions.
- Sensitive values are redacted before prompt construction.
- Findings must cite valid `evidence://...` references.
- Local execution only accepts localhost Ollama URLs and is disabled by
  default.

## Build Model

The former Electron and Python/FastAPI distribution paths are retired.
The target release artifact is the Wails desktop app, with headless Go
CLIs for CI and scripting.

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-profiler

cd cmd/archscope-profiler-app/frontend
npm ci
npm run build
```
