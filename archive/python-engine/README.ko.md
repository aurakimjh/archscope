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
│   ├── collapsed_parser.py   #   async-profiler collapsed stack (per-thread `[Thread]` 인식)
│   ├── jennifer_csv_parser.py#   Jennifer APM flamegraph CSV
│   ├── gc_log_parser.py      #   HotSpot GC log
│   ├── gc_log_header.py      #   GC log 헤더 → JVM Info(Version/CPU/Heap/CommandLine)
│   ├── jfr_recording.py      #   바이너리 .jfr → JSON (JDK `jfr` CLI 자동 호출)
│   ├── jfr_parser.py         #   JFR print JSON (기존 경로)
│   ├── thread_dump/          #   Plugin registry (9 변형, 모두 ThreadDumpBundle 출력)
│   │   ├── registry.py
│   │   ├── java_jstack.py    #   JDK 21+ no-`nid` 변형 포함
│   │   ├── java_jcmd_json.py
│   │   ├── go_goroutine.py
│   │   ├── python_dump.py    #   py-spy / faulthandler
│   │   ├── python_traceback.py # Thread ID + File "...", line N
│   │   ├── nodejs_report.py
│   │   ├── nodejs_sample_trace.py
│   │   ├── dotnet_clrstack.py
│   │   └── dotnet_environment_stacktrace.py
│   ├── exception_parser.py   #   Java exception stack trace
│   ├── nodejs_stack_parser.py#   Node.js Error stack
│   ├── python_traceback_parser.py # Python traceback
│   ├── go_panic_parser.py    #   Go panic/goroutine dump
│   ├── dotnet_parser.py      #   .NET exception + IIS W3C log
│   └── otel_parser.py        #   OpenTelemetry JSONL log
├── analyzers/                # Record → aggregated AnalysisResult
│   ├── access_log_analyzer.py     # 22-메트릭 요약, 분당 시계열, URL classification
│   ├── profiler_analyzer.py       # Profiler collapsed 분석 (per-thread 감지 포함)
│   ├── profiler_diff.py           # 양면 differential flame (red=느려짐 / blue=빨라짐)
│   ├── profiler_drilldown.py      # Flamegraph drill-down
│   ├── profiler_breakdown.py      # Execution breakdown
│   ├── flamegraph_builder.py      # FlameNode tree 구축
│   ├── profile_classification.py  # Stack classification (rule-driven)
│   ├── native_memory_analyzer.py  # JFR alloc/free 페어링 + tail-ratio cutoff
│   ├── gc_log_analyzer.py         # GC log 분석 (9개 힙 시리즈)
│   ├── jfr_analyzer.py            # JFR recording 분석 + 모드/시간/상태 필터
│   ├── thread_dump_analyzer.py    # 단일 덤프 분석 (JVM 전용)
│   ├── multi_thread_analyzer.py   # 멀티 덤프 상관 분석 + 휴리스틱 finding
│   ├── lock_contention_analyzer.py# Owner/waiter 그래프 + DFS 데드락 검출
│   ├── thread_dump_to_collapsed.py# 덤프 → flamegraph collapsed 배치 변환
│   ├── exception_analyzer.py      # Exception stack 분석
│   ├── runtime_analyzer.py        # Multi-runtime stack 분석
│   └── otel_analyzer.py           # OpenTelemetry trace/service correlation
├── exporters/                # AnalysisResult → 보고서 artifact
│   ├── json_exporter.py      #   JSON export
│   ├── csv_exporter.py       #   CSV export
│   ├── html_exporter.py      #   Portable HTML report
│   ├── pptx_exporter.py      #   PowerPoint report
│   ├── pprof_exporter.py     #   Google pprof binary (자체 minimal protobuf)
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

# Profiler Diff (red=느려짐 / blue=빨라짐)
archscope-engine profiler diff \
  --baseline <r1.json> --target <r2.json> --out <diff.json> [--normalize]

# Profiler pprof export (gzipped, Pyroscope/Speedscope/`go tool pprof` 호환)
archscope-engine profiler export-pprof \
  --input <result.json> --output <profile.pb.gz>

# Jennifer APM CSV 분석
archscope-engine profiler analyze-jennifer-csv \
  --file <CSV파일> --out <출력.json>

# Profiler Drill-down / Breakdown
archscope-engine profiler drilldown --wall <파일> --filter <패턴> --out <출력.json>
archscope-engine profiler breakdown --wall <파일> --filter <패턴> --out <출력.json>

# GC Log 분석 (JVM Info + 9개 힙 시리즈)
archscope-engine gc-log analyze --file <GC로그> --out <출력.json>

# Thread Dump 분석
archscope-engine thread-dump analyze --file <덤프파일> --out <출력.json>
archscope-engine thread-dump analyze-multi \
  --input <f1> --input <f2> --input <f3> --out <multi.json> \
  [--consecutive-threshold 3] [--format <plugin-id>]
archscope-engine thread-dump to-collapsed \
  --input <f1> --input <f2> --output <flame.collapsed>

# Exception Stack 분석
archscope-engine exception analyze --file <예외로그> --out <출력.json>

# Multi-runtime
archscope-engine nodejs analyze --file <파일> --out <출력.json>
archscope-engine python-traceback analyze --file <파일> --out <출력.json>
archscope-engine go-panic analyze --file <파일> --out <출력.json>
archscope-engine dotnet analyze --file <파일> --out <출력.json>

# OpenTelemetry
archscope-engine otel analyze --file <JSONL파일> --out <출력.json>

# JFR — 바이너리 .jfr 또는 JDK jfr print JSON 입력
archscope-engine jfr analyze --file <recording.jfr|.json> --out <출력.json> \
  [--event-modes cpu,wall,alloc,lock,gc,exception,io,nativemem] \
  [--time-from "+30s"] [--time-to "-2m"] \
  [--thread-state RUNNABLE,BLOCKED] [--min-duration-ms 5]
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
Parser (parsers/*.py 또는 parsers/thread_dump/<plugin>.py)
  │  Generator[TypedRecord] / ThreadDumpBundle
  ▼
Analyzer (analyzers/*.py)
  │  Streaming aggregation + finding emission
  ▼
AnalysisResult (dict, schema_version 0.2.0)
  │
  ├──▶ JSON Exporter → .json file → Browser UI (via FastAPI /api)
  ├──▶ HTML Exporter → .html standalone report
  ├──▶ PPTX Exporter → .pptx presentation
  ├──▶ pprof Exporter → .pb.gz (profiler 전용)
  └──▶ (Optional) AI Interpretation → InterpretationResult
```

## 웹 서버

```bash
# React UI 빌드 (한 번만)
npm --prefix ../../apps/frontend install
npm --prefix ../../apps/frontend run build

# FastAPI 엔진 + 정적 번들 서빙
archscope-engine serve --static-dir ../../apps/frontend/dist
# http://127.0.0.1:8765 접속
```

엔진은 기본적으로 `127.0.0.1`에 바인딩됩니다. 요청은 모두 로컬이고
번들된 Electron 빌드는 렌더러를 `file://`에서 로드하므로 CORS는
`allow_origins=["*"]`로 풀어둡니다.
