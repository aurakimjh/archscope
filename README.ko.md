# ArchScope (한국어)

[English](./README.en.md) · [최상위 README](./README.md)

ArchScope는 이제 **Go/Wails 기반 로컬 데스크톱 애플리케이션**입니다.
Access/edge log, server log, OpenTelemetry log, database/broker log,
platform/cloud evidence, metrics snapshot, trace, runtime profile, JFR, GC log,
exception stack, thread dump를 분석해 공통 `AnalysisResult`, 차트, 진단,
contract/risk view, architecture-doc draft, 보고서 산출물로 변환하며 원격
서비스로 데이터를 보내지 않습니다.

## 현재 스택

- `apps/engine-native/` — Go 엔진, parser/analyzer/exporter,
  profiler core, Cobra CLI, Wails 앱.
- `apps/engine-native/cmd/archscope-engine` — CI와 스크립팅용 headless CLI.
- `apps/engine-native/cmd/archscope-app` — Wails 데스크톱 앱.
- `archive/python-engine/`, `archive/web-frontend-python/` — 폐기된
  Python/FastAPI/browser 구현. 참조와 이관 검증 용도로만 보관.

## 빌드

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

## 기능 범위

| 영역 | 기능 |
| --- | --- |
| Evidence import | access/edge log, server log, OpenTelemetry log, metrics snapshot, observability export, database slow-query evidence, broker log, Kubernetes/container/cloud audit evidence, trace import |
| Runtime diagnostics | GC log, JFR, native memory, Java thread dump, lock contention, multi-dump correlation, exception stack, Node.js/Python/Go/.NET runtime stack evidence |
| Profiling | collapsed stack, Jennifer CSV, FlameGraph SVG/HTML, pprof, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, StackProf, PHP profiler export, Xdebug, Swift/async stack, Pyroscope/Phlare, Parca |
| Evidence Studio | Analysis Workspace, Evidence Board, Incident Timeline, SLO/Golden Signals, Service Flow, stitched-evidence drilldown, API/event contract analysis, architecture docs draft |
| 내보내기 | JSON, CSV, HTML report, PPTX, report diff, chart export, evidence pack, report-pack ZIP |
| AI | evidence 기반 로컬 해석 헬퍼, redaction, evidence-reference validation, localhost Ollama 전용 |

## 문서

- [아키텍처](docs/ko/ARCHITECTURE.md)
- [네이티브 앱 가이드](docs/ko/NATIVE_APP.md)
- [AI 보조 해석](docs/ko/AI_INTERPRETATION.md)
- [멀티 언어 thread dump](docs/ko/MULTI_LANGUAGE_THREADS.md)
- [Importer support matrix](docs/ko/IMPORTER_SUPPORT_MATRIX.md)
- [데이터 모델](docs/ko/DATA_MODEL.md)

## 로컬 우선

데스크톱 앱과 CLI는 로컬에서 실행됩니다. 선택적 AI 해석은 localhost
Ollama endpoint만 허용하고, 생성된 finding은 evidence reference 검증을
통과해야 결과로 인정됩니다.
