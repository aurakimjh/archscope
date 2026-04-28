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

async-profiler collapsed 샘플 분석:

```bash
archscope-engine profiler analyze-collapsed \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --wall-interval-ms 100 \
  --elapsed-sec 1336.559 \
  --out ../../examples/outputs/profiler-result.json
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
- JSON result export

GC log, thread dump, exception analysis, packaging, PowerPoint export, large-file optimization은 이후 phase에서 구현합니다.

## License

MIT License
