# ArchScope Architecture

ArchScope is an application architecture diagnostic and reporting toolkit. Its core responsibility is to transform raw operational evidence into normalized analysis results and report-ready visualizations.

## System Flow

```text
Raw Data
  -> Parsing
  -> Analysis / Aggregation
  -> Visualization
  -> Report-ready Export
```

## Components

### Desktop UI

The desktop application is built with Electron, React, TypeScript, and Apache ECharts. It provides:

- Analyzer navigation
- File selection workflows
- Chart dashboards
- Chart Studio placeholders
- Export Center placeholders
- English/Korean UI locale switching

The UI reads normalized analysis result JSON rather than raw log files. This keeps UI rendering independent from parser implementation details.

### Python Analysis Engine

The Python engine owns parsing, normalization, aggregation, and export. It is organized around clear module boundaries:

- `parsers`: Convert source files into typed records.
- `analyzers`: Aggregate records into summary, series, and table structures.
- `models`: Shared dataclasses for normalized data.
- `exporters`: Write normalized results to JSON, CSV, HTML, and future formats.
- `common`: Shared file, time, and statistics utilities.

### Result Contract

All analyzers should emit an AnalysisResult-like JSON structure:

```text
type
source_files
created_at
summary
series
tables
charts
metadata
```

Charts are rendered from normalized result fields, not raw log lines.
User-facing labels should come from locale resources so the same normalized result can be shown in English or Korean.

## Extension Model

New diagnostic data types should follow this path:

1. Add record model in `models`.
2. Add streaming parser in `parsers`.
3. Add aggregation logic in `analyzers`.
4. Add JSON export support through the shared exporter.
5. Add chart templates in the desktop chart catalog.
6. Add UI page or extend an existing analyzer page.

## Runtime Scope

Although JVM diagnostics are an early focus, ArchScope should remain runtime-neutral. The model supports Java, Node.js, Python, Go, .NET, and middleware-specific evidence.

## Packaging Direction

Future packaging will use:

- `electron-builder` for the desktop application.
- `PyInstaller` for a distributable Python engine.

The initial skeleton intentionally avoids packaging implementation.
