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

# Chrome Performance trace or V8 .cpuprofile (Node --cpu-prof, CDP)
go run ./cmd/archscope-engine profile import \
  --in ./trace.json.gz \
  --format auto \
  --out browser-profile.json

# Local Lighthouse report (scores are preserved, URLs are redacted)
go run ./cmd/archscope-engine browser import \
  --in ./lighthouse-report.json \
  --format lighthouse-json \
  --out browser-audit.json

# Redacted HAR import (dialect auto-detection, bounded entry cap)
go run ./cmd/archscope-engine http-capture analyze \
  --in ./session.har \
  --out http-capture.json

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

## Supported Languages And Evidence

ArchScope support is evidence-based. It analyzes runtime artifacts, logs,
profiles, traces, and contracts; it does not perform static source-code review
or modify application source code.

| Area | Current support |
| --- | --- |
| ArchScope implementation | Go engine, Wails desktop app, React/TypeScript frontend |
| JVM / Java evidence | GC logs, JFR JSON, native-memory events, Java thread dumps, jcmd JSON thread dumps, Java exception stacks, async-profiler/Jennifer profile evidence |
| Go evidence | goroutine dumps, panic stacks, pprof-compatible profiles |
| Python evidence | traceback blocks, py-spy/faulthandler-style dumps, py-spy profile evidence |
| Node.js evidence | diagnostic reports, sample traces, JavaScript stack traces |
| .NET evidence | clrstack, Environment.StackTrace, exception/IIS evidence, dotnet-trace speedscope exports |
| Ruby / PHP / Swift / native profile evidence | rbspy, StackProf, PHP Excimer/Tideways/Xdebug, Swift/async stacks, perf collapsed/native stacks when supplied as supported profile artifacts |
| Browser / frontend evidence | Chrome Performance traces (`.json`/`.json.gz`), V8 `.cpuprofile` (browser, Node `--cpu-prof`, CDP `Profiler.stop`) with sampled CPU run analysis; note these are CPU samples only — no network, layout, or paint attribution |
| HTTP evidence | HAR 1.2 imports with dialect detection and import-time redaction (`http_capture`); live capture is a Windows-first roadmap slice |
| Language-neutral evidence | access/edge logs, server logs, OpenTelemetry logs/traces, metrics snapshots, database/broker/platform evidence, OpenAPI, AsyncAPI, stitched evidence, architecture-doc drafts |

Unsupported or deferred:

- Static source-code analysis, AST indexing, repository-wide code search, code
  quality scanning, and automatic source modification.
- Heap dump parsing (`.hprof`) and process/system monitoring such as live CPU,
  RSS, or syscall sampling.
- Direct SaaS APM connectors unless promoted from the roadmap into an active
  implementation slice.

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

This feature is not a source-editing coding agent. It is an evidence-bound
interpretation assistant for already-produced `AnalysisResult` data. The
deterministic analyzer output remains the source of truth.

User-facing workflow:

1. Run one or more deterministic analyzers and add the results to Analysis
   Workspace.
2. If an AI interpretation payload is present, Analysis Workspace shows provider,
   model, prompt version, disabled state, finding count, and gate status.
3. AI findings are rendered in a separate AI-assisted panel and can be added to
   Evidence Board or Report Pack only when the evidence gate passes.
4. If Ollama or the configured model is unavailable, deterministic analysis and
   exports still work.

Local runtime requirements:

```bash
ollama serve
ollama pull qwen2.5-coder:7b
```

The initial policy allows only `localhost`, `127.0.0.1`, or `::1` Ollama
endpoints. Models are user-installed and are not bundled with ArchScope. See
`docs/en/AI_INTERPRETATION.md` for the full gate, redaction, prompt-injection,
and reporting policy.
