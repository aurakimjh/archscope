# Roadmap

## Phase 1: Foundation

- Repository skeleton
- Desktop UI skeleton
- Python engine skeleton
- Access log parser MVP
- Collapsed profiler parser MVP
- Sample charts
- JSON result format
- English/Korean documentation and UI i18n foundation
- Engine-UI Bridge PoC with Electron IPC and Python CLI
- Explicit Python runtime dependencies and CLI entry point
- Parser diagnostics for malformed records
- Encoding fallback correctness
- Type-specific `AnalysisResult` contracts for Access Log and Profiler
- Focused parser, utility, and JSON exporter tests

## Phase 2: Report-ready Charts

- Chart Studio
- Theme editor
- ECharts 6 upgrade evaluation
- Dark mode and dynamic chart themes
- Broken-axis and distribution chart options
- PNG/SVG export
- CSV export
- Access log advanced statistics
- Access log diagnostic findings beyond raw charts
- Profiler flamegraph drill-down, Jennifer CSV import, and execution breakdown
- Custom regex parser
- Report label language toggle

## Phase 3: JVM Diagnostics and Distribution

- GC log analyzer
- Java thread dump analyzer
- Java exception analyzer
- JFR recording parser design and feasibility spike
- Timeline correlation
- Electron supported-version upgrade
- Electron + PyInstaller packaging spike

## Phase 4: Multi-runtime and Observability Inputs

- Timeline correlation `AnalysisResult` design
- JFR recording parser design using the JDK `jfr` command spike path
- OpenTelemetry log input design with trace/span context mapping
- Node.js log and stack analyzer
- Python traceback analyzer
- Go panic/goroutine analyzer
- .NET exception/IIS analyzer
- OpenTelemetry log input and trace context mapping
- Cross-evidence timeline correlation across access logs, GC, profiler, thread, JFR, and OTel evidence

## Phase 5: Report Automation

- Before/after diff
- HTML report generation
- PowerPoint export
- Executive summary generator
- AI-assisted interpretation, optional and evidence-bound
- Optional local LLM/Ollama interpretation with validated evidence references
- AI interpretation hardening: canonical `evidence_ref` grammar, `InterpretationResult` contract, runtime validator, prompt-injection defense, local-only runtime policy, provenance UI, and evaluation gates
