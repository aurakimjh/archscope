# 로드맵

## Phase 1: Foundation

- Repository skeleton
- Desktop UI skeleton
- Python engine skeleton
- Access log parser MVP
- Collapsed profiler parser MVP
- Sample charts
- JSON result format
- English/Korean documentation and UI i18n foundation
- Electron IPC와 Python CLI 기반 Engine-UI Bridge PoC
- Python runtime dependency 및 CLI entry point 명시
- Malformed record용 parser diagnostics
- Encoding fallback correctness
- Access Log와 Profiler용 type-specific `AnalysisResult` contract
- Parser, utility, JSON exporter 중심 테스트 확충

## Phase 2: Report-ready Charts

- Chart Studio
- Theme editor
- ECharts 6 upgrade 평가
- Dark mode 및 dynamic chart theme
- Broken-axis 및 distribution chart option
- PNG/SVG export
- CSV export
- Access log advanced statistics
- Raw chart를 넘어선 Access Log diagnostic findings
- Custom regex parser
- Report label language toggle

## Phase 3: JVM Diagnostics and Distribution

- GC log analyzer
- Java thread dump analyzer
- Java exception analyzer
- JFR recording parser 설계 및 feasibility spike
- Timeline correlation
- Electron supported-version upgrade
- Electron + PyInstaller packaging spike

## Phase 4: Multi-runtime and Observability Inputs

- Timeline correlation `AnalysisResult` 설계
- JDK `jfr` command spike path 기반 JFR recording parser 설계
- Trace/span context mapping을 포함한 OpenTelemetry log input 설계
- Node.js log and stack analyzer
- Python traceback analyzer
- Go panic/goroutine analyzer
- .NET exception/IIS analyzer
- OpenTelemetry log input 및 trace context mapping
- Access log, GC, profiler, thread, JFR, OTel evidence를 아우르는 cross-evidence timeline correlation

## Phase 5: Report Automation

- Before/after diff
- HTML report generation
- PowerPoint export
- Executive summary generator
- AI-assisted interpretation, optional and evidence-bound
- 검증된 evidence reference를 포함한 optional local LLM/Ollama interpretation
- AI interpretation hardening: canonical `evidence_ref` 문법, `InterpretationResult` contract, runtime validator, prompt-injection defense, local-only runtime policy, provenance UI, evaluation gate
