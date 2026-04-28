# ArchScope

Application Architecture Diagnostic & Reporting Toolkit

## What is ArchScope?

ArchScope parses middleware logs, GC logs, profiler outputs, thread dumps, and exception stack traces, then converts them into report-ready statistics, charts, and diagnostic evidence.

ArchScope is intended for application architects who need to turn raw operational and performance data into architecture diagnosis evidence.

## Korean Overview

ArchScope는 미들웨어 로그, GC 로그, 프로파일링 결과, 스택 트레이스 등을 파싱하여 애플리케이션 아키텍트가 성능 진단 보고서에 바로 사용할 수 있는 통계, 그래프, 진단 근거로 변환하는 도구입니다.

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

- [English documentation](docs/README.md#english)
- [Korean documentation](docs/README.md#korean)

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
python -m archscope_engine.cli --help
```

Run the sample access log analysis:

```bash
python -m archscope_engine.cli access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json
```

Run the sample async-profiler collapsed analysis:

```bash
python -m archscope_engine.cli profiler analyze-collapsed \
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
