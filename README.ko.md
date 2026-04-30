# ArchScope

[English](./README.md)

애플리케이션 아키텍처 진단 및 보고서 작성 Toolkit

## ArchScope란?

ArchScope는 미들웨어 access log, GC log, profiler output, thread dump, exception stack trace를 파싱하여 보고서에 바로 사용할 수 있는 통계, 차트, 진단 근거로 변환하는 도구입니다.

ArchScope는 운영 및 성능 데이터를 애플리케이션 아키텍처 진단 근거로 정리해야 하는 애플리케이션 아키텍트를 위한 desktop 기반 utility입니다.

## 핵심 목표

- 운영 및 성능 데이터 파싱
- Raw data를 공통 모델로 정규화
- 통계 및 집계 생성
- 보고서용 chart 시각화
- Architecture report를 위한 chart와 table export
- 여러 runtime과 middleware platform 지원
- English/Korean 문서와 UI label 지원

## 진단 흐름

```text
Raw Data -> Parsing -> Analysis / Aggregation -> Visualization -> Report-ready Export
```

ArchScope는 단순 log viewer가 아니라 Architecture Evidence Builder를 지향합니다.

## 초기 모듈

- Access Log Analyzer
- GC Log Analyzer
- Profiler Analyzer
- Thread Dump Analyzer
- Exception Analyzer
- Chart Studio
- Export Center

## 기술 스택

- Electron
- React
- TypeScript
- Apache ECharts
- Python
- Typer
- pandas, optional

## Repository 구조

```text
archscope/
  apps/desktop/        Electron + React desktop skeleton
  engines/python/      Python parser and analysis engine
  docs/                Product and architecture design documents
  examples/            Sample input data and generated outputs
  scripts/             Development helper scripts
```

## 문서

- [English documentation](docs/en/README.md)
- [한국어 문서](docs/ko/README.md)

## 개발

### Desktop UI

```bash
cd apps/desktop
npm install
npm run dev
```

Desktop app은 Electron shell을 실행하고 Vite React UI를 로드합니다.
현재 UI는 navigation, dashboard label, analyzer skeleton page, chart label에 대해 English/Korean language selector를 제공합니다.

### Python Engine

```bash
cd engines/python
python -m venv .venv
source .venv/bin/activate
pip install -e .
archscope-engine --help
```

Access log 샘플 분석:

```bash
archscope-engine access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json
```

Malformed record가 skip되면 parser debug log가 자동 생성됩니다. Debug log는 기본 redaction을 적용하며, 강제 생성 또는 저장 위치 지정도 가능합니다.

```bash
archscope-engine access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json \
  --debug-log \
  --debug-log-dir ./archscope-debug
```

async-profiler collapsed 샘플 분석:

```bash
archscope-engine profiler analyze-collapsed \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --wall-interval-ms 100 \
  --elapsed-sec 1336.559 \
  --out ../../examples/outputs/profiler-result.json
```

Jennifer APM flamegraph CSV 샘플 분석:

```bash
archscope-engine profiler analyze-jennifer-csv \
  --file ../../examples/profiler/sample-jennifer-flame.csv \
  --out ../../examples/outputs/profiler-jennifer-result.json
```

Profiler drill-down 및 execution breakdown:

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

`AnalysisResult` JSON 또는 parser debug JSON으로 portable HTML report 생성:

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

Multi-runtime analyzer MVP 명령:

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

## 현재 범위

현재 repository는 foundation 단계입니다.

- Public repository skeleton
- 설계 문서
- Electron + React + TypeScript UI skeleton
- ECharts sample dashboard
- Python engine skeleton
- Minimal NGINX-like access log parser
- Minimal async-profiler collapsed parser
- Jennifer APM flamegraph CSV import
- Profiler flamegraph drill-down 및 execution breakdown
- JVM GC log, thread dump, exception stack analyzer MVP
- Node.js, Python traceback, Go panic/goroutine, .NET/IIS analyzer MVP
- OpenTelemetry JSONL log analyzer 및 cross-service trace correlation MVP
- 현장 parser 수정을 위한 portable redacted parser debug log
- JSON result export
- 결과/debug JSON 기반 portable HTML report export
- Before/after comparison result export
- Minimal PowerPoint report export
- Profiler result JSON용 static HTML flamegraph rendering
- 제목, renderer, theme, option JSON을 조정하는 Chart Studio template preview

Packaging polish, interactive HTML chart bundle, 더 깊은 OTel trace context mapping, 광범위한 large-file optimization은 이후 phase 작업으로 남겨둡니다.

## License

MIT License
