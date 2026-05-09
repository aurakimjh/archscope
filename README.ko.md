# ArchScope (한국어)

[English](./README.en.md) · [최상위 README](./README.md)

ArchScope는 이제 **Go/Wails 기반 로컬 데스크톱 애플리케이션**입니다.
운영 증거(access log, GC log, profiler output, JFR, exception stack,
thread dump)를 분석해 공통 `AnalysisResult`, 차트, 진단, 보고서 산출물로
변환하며 원격 서비스로 데이터를 보내지 않습니다.

## 현재 스택

- `apps/engine-native/` — Go 엔진, parser/analyzer/exporter,
  profiler core, Cobra CLI, Wails 앱.
- `apps/engine-native/cmd/archscope-engine` — CI와 스크립팅용 headless CLI.
- `apps/engine-native/cmd/archscope-profiler` — profiler 전용 CLI.
- `apps/engine-native/cmd/archscope-profiler-app` — Wails 데스크톱 앱.
- `archive/python-engine/`, `archive/web-frontend-python/` — 폐기된
  Python/FastAPI/browser 구현. 참조와 이관 검증 용도로만 보관.

## 빌드

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-profiler

cd cmd/archscope-profiler-app/frontend
npm ci
npm run build
```

## 기능 범위

| 영역 | 기능 |
| --- | --- |
| Profiler | collapsed stack, Jennifer CSV, FlameGraph SVG/HTML, drill-down, execution breakdown, timeline analysis, diff flamegraph, pprof export |
| JVM | GC log, JFR, native memory, Java thread dump, lock contention, multi-dump correlation |
| 멀티 런타임 | Go goroutine, Python dump/traceback, Node.js diagnostic report, .NET clrstack/environment stacktrace |
| 로그 | Access log, exception stack, OpenTelemetry log |
| 내보내기 | JSON, CSV, HTML report, PPTX, report diff |
| AI | evidence 기반 로컬 해석 헬퍼, localhost Ollama 전용 |

## 문서

- [아키텍처](docs/ko/ARCHITECTURE.md)
- [네이티브 앱 가이드](docs/ko/PROFILER_NATIVE.md)
- [AI 보조 해석](docs/ko/AI_INTERPRETATION.md)
- [멀티 언어 thread dump](docs/ko/MULTI_LANGUAGE_THREADS.md)

## 로컬 우선

데스크톱 앱과 CLI는 로컬에서 실행됩니다. 선택적 AI 해석은 localhost
Ollama endpoint만 허용하고, 생성된 finding은 evidence reference 검증을
통과해야 결과로 인정됩니다.
