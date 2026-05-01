# ArchScope

[한국어](./README.ko.md)

Application Architecture Diagnostic & Reporting Toolkit

## What is ArchScope?

ArchScope parses middleware logs, GC logs, profiler outputs, thread dumps, and exception stack traces, then converts them into report-ready statistics, charts, and diagnostic evidence.

ArchScope is intended for application architects who need to turn raw operational and performance data into architecture diagnosis evidence.

## Key Goals

- Parse operational and performance data
- Normalize raw data into common models
- Generate statistics and aggregations
- Visualize results with report-ready charts
- Export charts and tables for architecture reports
- Support multiple runtimes and middleware platforms
- Support English/Korean documentation and UI labels

## Diagnostic Flow

```text
Raw Data -> Parsing -> Analysis / Aggregation -> Visualization -> Report-ready Export
```

ArchScope is not just a log viewer. It is designed as an Architecture Evidence Builder.

## Initial Modules

- Access Log Analyzer
- GC Log Analyzer
- Profiler Analyzer
- Thread Dump Analyzer
- Exception Analyzer
- Chart Studio
- Export Center

## Tech Stack

- Electron
- React
- TypeScript
- Apache ECharts
- Python
- Typer
- pandas, optional

## Repository Layout

```text
archscope/
  apps/desktop/        Electron + React desktop skeleton
  engines/python/      Python parser and analysis engine
  docs/                Product and architecture design documents
  examples/            Sample input data and generated outputs
  scripts/             Development helper scripts
```

## Documentation

- [English documentation](docs/en/README.md)
- [Korean documentation](docs/ko/README.md)

## Development

### Desktop UI

```bash
cd apps/desktop
npm install
npm run dev
```

The desktop app starts an Electron shell and loads the Vite React UI.
The current UI includes an English/Korean language selector for navigation, dashboard labels, analyzer skeleton pages, and chart labels.

### Python Engine

```bash
cd engines/python
python -m venv .venv
source .venv/bin/activate
pip install -e .
archscope-engine --help
```

Run the sample access log analysis:

```bash
archscope-engine access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json
```

Parser debug logs are generated automatically when malformed records are skipped. They are redacted by default and can be forced or redirected:

```bash
archscope-engine access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json \
  --debug-log \
  --debug-log-dir ./archscope-debug
```

Run the sample async-profiler collapsed analysis:

```bash
archscope-engine profiler analyze-collapsed \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --wall-interval-ms 100 \
  --elapsed-sec 1336.559 \
  --out ../../examples/outputs/profiler-result.json
```

Run the sample Jennifer APM flamegraph CSV analysis:

```bash
archscope-engine profiler analyze-jennifer-csv \
  --file ../../examples/profiler/sample-jennifer-flame.csv \
  --out ../../examples/outputs/profiler-jennifer-result.json
```

Profiler drill-down and execution breakdown are available through:

```bash
archscope-engine profiler drilldown \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --filter oracle.jdbc \
  --out ../../examples/outputs/profiler-drilldown-result.json

archscope-engine profiler breakdown \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --filter RestTemplate \
  --out ../../examples/outputs/profiler-breakdown-result.json

archscope-engine gc-log analyze \
  --file ../../examples/gc-logs/sample-hotspot-gc.log \
  --out ../../examples/outputs/gc-log-result.json

archscope-engine thread-dump analyze \
  --file ../../examples/thread-dumps/sample-java-thread-dump.txt \
  --out ../../examples/outputs/thread-dump-result.json

archscope-engine exception analyze \
  --file ../../examples/exceptions/sample-java-exception.txt \
  --out ../../examples/outputs/exception-result.json
```

Generate a portable HTML report from any `AnalysisResult` JSON or parser debug JSON:

```bash
archscope-engine report html \
  --input ../../examples/outputs/access-log-result.json \
  --out ../../examples/outputs/access-log-report.html

archscope-engine report diff \
  --before ../../examples/outputs/access-log-result.json \
  --after ../../examples/outputs/access-log-result.json \
  --out ../../examples/outputs/access-log-diff.json \
  --html-out ../../examples/outputs/access-log-diff.html

archscope-engine report pptx \
  --input ../../examples/outputs/access-log-result.json \
  --out ../../examples/outputs/access-log-report.pptx
```

Multi-runtime analyzer MVP commands:

```bash
archscope-engine nodejs analyze \
  --file ../../examples/runtime/sample-nodejs-stack.txt \
  --out ../../examples/outputs/nodejs-stack-result.json

archscope-engine python-traceback analyze \
  --file ../../examples/runtime/sample-python-traceback.txt \
  --out ../../examples/outputs/python-traceback-result.json

archscope-engine go-panic analyze \
  --file ../../examples/runtime/sample-go-panic.txt \
  --out ../../examples/outputs/go-panic-result.json

archscope-engine dotnet analyze \
  --file ../../examples/runtime/sample-dotnet-iis.txt \
  --out ../../examples/outputs/dotnet-iis-result.json

archscope-engine otel analyze \
  --file ../../examples/otel/sample-otel-logs.jsonl \
  --out ../../examples/outputs/otel-logs-result.json
```

### Demo-Site Report Bundles

Shared demo-site manifests live in `../projects-assets/test-data/demo-site`.
ArchScope reads the manifest and analyzer type mapping from that asset
repository, then writes generated bundles locally:

```bash
./scripts/run-demo-site-data.sh

python -m archscope_engine.cli demo-site run \
  --manifest-root ../projects-assets/test-data/demo-site \
  --data-source synthetic \
  --scenario gc-pressure \
  --out /tmp/archscope-demo-bundles
```

Each scenario bundle contains `run-summary.json`, `index.html`, analyzer
JSON/HTML/PPTX outputs, and baseline comparison reports where matching
`analyzer_type` outputs exist. The canonical manifest mapping is
`../projects-assets/test-data/demo-site/analyzer_type_mapping.json`; inspect it
with `python -m archscope_engine.cli demo-site mapping --manifest-root ...`.

## Current Scope

This repository currently contains the foundation only:

- Public repository skeleton
- Design documents
- Electron + React + TypeScript UI skeleton
- ECharts sample dashboard
- Python engine skeleton
- Minimal NGINX-like access log parser
- Minimal async-profiler collapsed parser
- Jennifer APM flamegraph CSV import
- Profiler flamegraph drill-down and execution breakdown
- JVM GC log, thread dump, and exception stack analyzer MVPs
- Node.js, Python traceback, Go panic/goroutine, and .NET/IIS analyzer MVPs
- OpenTelemetry JSONL log analyzer and cross-service trace correlation MVP
- Portable redacted parser debug logs for field parser fixes
- JSON result export
- Portable HTML report export from result/debug JSON
- Before/after comparison result export
- Minimal PowerPoint report export
- Static HTML flamegraph rendering for profiler result JSON
- Chart Studio template preview with title, renderer, theme, and option JSON controls
- Demo Data Center and demo-site report bundle generation from shared manifests

Packaging polish and broad large-file optimization remain later-phase work. The desktop package now includes a Playwright/Electron smoke check for the Demo Data Center flow.

Current implementation work for this project is closed at the present scope. There are no active `T-xxx` backlog items in `work_status.md`; future work should start only from newly submitted review documents, explicit roadmap reopening, or verification/documentation maintenance.

## License

MIT License
