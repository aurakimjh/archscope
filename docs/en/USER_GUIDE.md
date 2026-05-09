# User Guide

This guide covers the current Go/Wails ArchScope line. The retired
Python/FastAPI browser app is preserved in `archive/` and is no longer
the recommended path.

## Build And Run

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-profiler

cd cmd/archscope-profiler-app/frontend
npm ci
npm run build
```

For desktop packaging:

```bash
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
cd apps/engine-native/cmd/archscope-profiler-app
task package
```

## CLI Examples

```bash
cd apps/engine-native

go run ./cmd/archscope-engine access-log analyze \
  --in ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out access.json

go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out profiler.json

go run ./cmd/archscope-engine thread-dump analyze \
  --in ../../examples/thread-dumps/java-jstack-sample.txt \
  --out thread.json
```

## Native App

Use `docs/en/PROFILER_NATIVE.md` for the desktop UI and packaging
workflow. The Wails app exposes profiler analysis plus the broader Go
engine analyzers through Wails services.

## AI Interpretation

AI interpretation is optional and local-only. The Go implementation under
`internal/aiinterpretation` builds evidence-bound prompts, redacts
sensitive data, validates model findings against registered evidence
references, and accepts only localhost Ollama URLs.
