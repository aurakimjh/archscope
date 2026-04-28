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

Run the sample async-profiler collapsed analysis:

```bash
archscope-engine profiler analyze-collapsed \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --wall-interval-ms 100 \
  --elapsed-sec 1336.559 \
  --out ../../examples/outputs/profiler-result.json
```

## Current Scope

This repository currently contains the foundation only:

- Public repository skeleton
- Design documents
- Electron + React + TypeScript UI skeleton
- ECharts sample dashboard
- Python engine skeleton
- Minimal NGINX-like access log parser
- Minimal async-profiler collapsed parser
- JSON result export

GC log, thread dump, exception analysis, packaging, PowerPoint export, and large-file optimization are intentionally left for later phases.

## License

MIT License
