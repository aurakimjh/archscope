# ArchScope Python Engine

Python engine은 raw diagnostic file을 파싱하고, record를 정규화하며, 통계를 집계한 뒤 AnalysisResult 형식의 JSON 파일을 생성합니다.

## 설치

```bash
cd engines/python
python -m venv .venv
source .venv/bin/activate
pip install -e .
```

## 모듈 구조

```text
archscope_engine/
├── cli.py                    # Typer CLI entrypoint
├── config/                   # 설정 파일 (runtime classification rules, prompt templates)
├── models/                   # 데이터 모델 (dataclass 정의)
│   ├── access_log.py         #   AccessLogRecord
│   ├── analysis_result.py    #   AnalysisResult 공통 모델
│   ├── flamegraph.py         #   FlameNode tree 모델
│   ├── gc_event.py           #   GcEvent
│   ├── otel.py               #   OpenTelemetry record 모델
│   ├── profile_stack.py      #   ProfileStack
│   ├── result_contracts.py   #   TypedDict 기반 result contract
│   ├── runtime_stack.py      #   Multi-runtime stack 공통 모델
│   └── thread_dump.py        #   ThreadDumpRecord
├── parsers/                  # Raw file → typed record 변환
│   ├── access_log_parser.py  #   NGINX/Apache/OHS/WebLogic/Tomcat/regex
│   ├── collapsed_parser.py   #   async-profiler collapsed stack
│   ├── jennifer_csv_parser.py#   Jennifer APM flamegraph CSV
│   ├── gc_log_parser.py      #   HotSpot GC log
│   ├── thread_dump_parser.py #   Java thread dump (jstack)
│   ├── exception_parser.py   #   Java exception stack trace
│   ├── nodejs_stack_parser.py#   Node.js Error stack
│   ├── python_traceback_parser.py # Python traceback
│   ├── go_panic_parser.py    #   Go panic/goroutine dump
│   ├── dotnet_parser.py      #   .NET exception + IIS W3C log
│   ├── otel_parser.py        #   OpenTelemetry JSONL log
│   └── jfr_parser.py         #   JFR print JSON (JDK jfr CLI 출력)
├── analyzers/                # Record → aggregated AnalysisResult
│   ├── access_log_analyzer.py     # Access log 통계/집계
│   ├── profiler_analyzer.py       # Profiler collapsed 분석
│   ├── profiler_drilldown.py      # Flamegraph drill-down
│   ├── profiler_breakdown.py      # Execution breakdown
│   ├── flamegraph_builder.py      # FlameNode tree 구축
│   ├── profile_classification.py  # Stack classification (rule-driven)
│   ├── gc_log_analyzer.py         # GC log 분석
│   ├── thread_dump_analyzer.py    # Thread dump 분석
│   ├── exception_analyzer.py      # Exception stack 분석
│   ├── runtime_analyzer.py        # Multi-runtime stack 분석
│   ├── otel_analyzer.py           # OpenTelemetry trace/service correlation
│   └── jfr_analyzer.py           # JFR recording 분석
├── exporters/                # AnalysisResult → 보고서 artifact
│   ├── json_exporter.py      #   JSON export
│   ├── csv_exporter.py       #   CSV export
│   ├── html_exporter.py      #   Portable HTML report
│   ├── pptx_exporter.py      #   PowerPoint report
│   └── report_diff.py        #   Before/After comparison
├── ai_interpretation/        # Evidence-bound AI 해석 (optional)
│   ├── evidence.py           #   EvidenceRegistry, EvidenceSelector
│   ├── prompting.py          #   PromptBuilder
│   ├── runtime.py            #   OllamaClient, LocalLlmClient
│   ├── validation.py         #   AiFindingValidator
│   ├── privacy.py            #   Prompt 내 개인���보 redaction
│   └── evaluation.py         #   Golden diagnostics 평가
├── common/                   # 공유 유틸리티
│   ├── file_utils.py         #   파일 읽기, encoding fallback
│   ├── time_utils.py         #   시간 파싱, UTC 정규화
│   ├── statistics.py         #   Reservoir sampling, percentile
│   ├── diagnostics.py        #   DiagnosticsCollector
│   ├── redaction.py          #   개인정보 마스킹
│   └── debug_log.py          #   Portable parser debug log
├── demo_site_runner.py       # Demo-site scenario 실행기
└── demo_site_mapping.py      # Analyzer type mapping
```

## CLI

설치 후에는 console script를 사용합니다.

```bash
archscope-engine --help
```

### 주요 명령

```bash
# Access Log 분석
archscope-engine access-log analyze \
  --file <로그파일> --format <nginx|apache|ohs|weblogic|tomcat> --out <출력.json>

# Profiler Collapsed Stack 분석
archscope-engine profiler analyze-collapsed \
  --wall <collapsed파일> --wall-interval-ms 100 --out <출력.json>

# Jennifer APM CSV 분석
archscope-engine profiler analyze-jennifer-csv \
  --file <CSV파일> --out <출력.json>

# Profiler Drill-down / Breakdown
archscope-engine profiler drilldown --wall <파일> --filter <패턴> --out <출력.json>
archscope-engine profiler breakdown --wall <파일> --filter <패턴> --out <출력.json>

# GC Log 분석
archscope-engine gc-log analyze --file <GC로그> --out <출력.json>

# Thread Dump 분석
archscope-engine thread-dump analyze --file <덤프파일> --out <출력.json>

# Exception Stack 분석
archscope-engine exception analyze --file <예외로그> --out <출력.json>

# Multi-runtime
archscope-engine nodejs analyze --file <파일> --out <출력.json>
archscope-engine python-traceback analyze --file <파일> --out <출력.json>
archscope-engine go-panic analyze --file <파일> --out <출력.json>
archscope-engine dotnet analyze --file <파일> --out <출력.json>

# OpenTelemetry
archscope-engine otel analyze --file <JSONL파일> --out <출력.json>

# JFR (JDK jfr print JSON 입력)
archscope-engine jfr analyze-json --file <jfr-print.json> --out <출력.json>

# 보고서 생성
archscope-engine report html --input <분석결과.json> --out <보고서.html>
archscope-engine report diff --before <이전.json> --after <이후.json> --out <비교.json>
archscope-engine report pptx --input <분석결과.json> --out <보고서.pptx>

# Demo-site
archscope-engine demo-site run --manifest-root <경로> --out <출력디렉터리>
```

## 테스트

```bash
cd engines/python
pip install -e ".[test]"
pytest
```

## 개발 모드 실행

설치 전 source tree에서 직접 실행:

```bash
python -m archscope_engine.cli --help
python -m archscope_engine.cli access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out /tmp/result.json
```

## 데이터 흐름

```text
Raw File
  │
  ▼
Parser (parsers/*.py)
  │  Generator[TypedRecord]
  ▼
Analyzer (analyzers/*.py)
  │  Streaming aggregation
  ▼
AnalysisResult (dict)
  │
  ├──▶ JSON Exporter → .json file → Desktop UI (via IPC)
  ├──▶ HTML Exporter → .html standalone report
  ├──▶ PPTX Exporter → .pptx presentation
  └──▶ (Optional) AI Interpretation → InterpretationResult
```
