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
- Engine-UI 브릿지 — 초기에는 Electron IPC + Python CLI 기반 PoC. 2026-05 웹 전환(T-206..T-209)에서 FastAPI HTTP 경계(`/api/...`) + in-process 분석기 dispatch로 교체
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
- Chart Studio template preview/edit MVP
- Access log advanced statistics
- Raw chart를 넘어선 Access Log diagnostic findings
- Profiler flamegraph drill-down, Jennifer CSV import, execution breakdown
- Custom regex parser
- Report label language toggle

## Phase 3: JVM Diagnostics and Distribution

- GC log analyzer MVP
- Java thread dump analyzer MVP
- Java exception analyzer MVP
- JFR recording parser 설계 및 feasibility spike
- Timeline correlation
- *(2026-05 웹 전환에서 폐기)* Electron 버전 업그레이드 및 Electron + PyInstaller 패키징 spike — `pip install -e .` + `archscope-engine serve --static-dir`로 교체. [PACKAGING_PLAN](./PACKAGING_PLAN.md) 참고.

## Phase 4: Multi-runtime and Observability Inputs

- Timeline correlation `AnalysisResult` 설계
- JDK `jfr` command spike path 기반 JFR recording parser 설계
- Trace/span context mapping을 포함한 OpenTelemetry log input 설계
- Node.js log and stack analyzer
- Python traceback analyzer
- Go panic/goroutine analyzer
- .NET exception/IIS analyzer
- OpenTelemetry JSONL log analyzer 및 cross-service trace correlation MVP
- OpenTelemetry parent-span service path 분석 및 failure propagation
- 더 넓은 OpenTelemetry envelope ingestion 및 span timing correlation
- Access log, GC, profiler, thread, JFR, OTel evidence를 아우르는 cross-evidence timeline correlation

## Phase 5: Report Automation

- Before/after diff
- HTML report generation
- `AnalysisResult` 및 parser debug JSON용 portable static HTML report MVP
- Profiler result JSON용 static HTML flamegraph rendering
- PowerPoint export
- Minimal PowerPoint `.pptx` report MVP
- Executive summary generator
- AI-assisted interpretation, optional and evidence-bound
- 검증된 evidence reference를 포함한 optional local LLM/Ollama interpretation
- AI interpretation hardening: canonical `evidence_ref` 문법, `InterpretationResult` contract, runtime validator, prompt-injection defense, local-only runtime policy, provenance UI, evaluation gate
