# User Guide

This guide covers the current Go/Wails ArchScope line. The retired
Python/FastAPI browser app is preserved in `archive/` and is no longer
the recommended path.

## Build And Run

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

For desktop packaging:

```bash
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
cd apps/engine-native/cmd/archscope-app
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
  --in ../../examples/thread-dumps/sample-java-thread-dump.txt \
  --out thread.json

go run ./cmd/archscope-engine trace import \
  --in ../../examples/traces/sample-otlp-traces.jsonl \
  --format auto \
  --out trace.json

go run ./cmd/archscope-engine database-log analyze \
  --in ../../examples/database/sample-postgres.log \
  --format postgres-text \
  --out database.json

go run ./cmd/archscope-engine broker-log analyze \
  --in ../../examples/broker/sample-broker.log \
  --format auto \
  --out broker.json

go run ./cmd/archscope-engine api-contract analyze \
  --openapi ../../examples/api-contract/openapi-orders.json \
  --access-result ../../examples/api-contract/access-result.json \
  --asyncapi ../../examples/api-contract/asyncapi-orders.json \
  --broker-result ../../examples/api-contract/broker-result.json \
  --out contract.json

go run ./cmd/archscope-engine stitch analyze \
  --in ../../examples/stitching/access-result.json \
  --in ../../examples/stitching/trace-result.json \
  --in ../../examples/stitching/database-result.json \
  --time-window-seconds 60 \
  --out stitched.json

go run ./cmd/archscope-engine architecture-docs draft \
  --in contract.json --in stitched.json \
  --out architecture-docs.json

go run ./cmd/archscope-engine report html \
  --in architecture-docs.json \
  --out architecture-docs.html
```

Run `go run ./cmd/archscope-engine --help` for the full command list. The
current supported evidence families are summarized in
`docs/en/IMPORTER_SUPPORT_MATRIX.md`.

## Native App

Use `docs/en/NATIVE_APP.md` for the desktop UI and packaging
workflow. The Wails app exposes profiler analysis plus the broader Go
engine analyzers through Wails services. The active workspace surfaces are
Analysis Workspace, Evidence Board, Incident Timeline, SLO/Golden Signals,
Service Flow, stitched-evidence drilldown state, Export Center, Report Pack,
and Chart Studio.

## AI Interpretation

AI interpretation is optional and local-only. The Go implementation under
`internal/aiinterpretation` builds evidence-bound prompts, redacts
sensitive data, validates model findings against registered evidence
references, and accepts only localhost Ollama URLs.
