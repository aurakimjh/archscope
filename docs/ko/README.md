# ArchScope 문서

이 디렉터리는 ArchScope의 한국어 문서를 포함합니다.

## 읽기 순서 안내

| 독자 | 추천 시작 문서 |
|------|----------------|
| 처음 사용하는 분 | [사용자 가이드](./USER_GUIDE.md) |
| 아키텍처 이해가 필요한 개발자 | [아키텍처](./ARCHITECTURE.md) → [데이터 모델](./DATA_MODEL.md) |
| Parser/Analyzer 확장 개발자 | [파서 설계](./PARSER_DESIGN.md) → [런타임 분류](./RUNTIME_CLASSIFICATION.md) |
| UI/차트 개발자 | [차트 설계](./CHART_DESIGN.md) → [Chart Studio 설계](./CHART_STUDIO_DESIGN.md) |
| 성능 최적화 담당 | [성능 측정](./PERFORMANCE.md) |

## 사용자 문서

- [**사용자 가이드**](./USER_GUIDE.md) — Desktop App 화면별 상세 사용법

## 설계 문서

- [아키텍처](./ARCHITECTURE.md) — 시스템 구성, 컴포넌트 경계, IPC 설계
- [데이터 모델](./DATA_MODEL.md) — AnalysisResult 공통 contract, TypeScript/Python 타입 정의
- [파서 설계](./PARSER_DESIGN.md) — Parser 책임, 에러 처리, 대용량 파일 전략
- [차트 설계](./CHART_DESIGN.md) — Chart template, factory 패턴, 성능 가이드라인
- [Chart Studio 설계](./CHART_STUDIO_DESIGN.md) — Chart 미리보기, option persistence
- [런타임 분류](./RUNTIME_CLASSIFICATION.md) — Profiler stack classification rule 설계
- [보고서 Export 설계](./REPORT_EXPORT_DESIGN.md) — HTML/PPTX/Diff export 파이프라인
- [고급 진단 설계](./ADVANCED_DIAGNOSTICS.md) — Timeline correlation, JFR, OpenTelemetry
- [AI 보조 해석 설계](./AI_INTERPRETATION.md) — Evidence-bound AI interpretation, Ollama 연동

## 운영 문서

- [로드맵](./ROADMAP.md) — Phase별 개발 계획
- [패키징 계획](./PACKAGING_PLAN.md) — PyInstaller sidecar, Electron 빌드, 코드 서명
- [성능 측정](./PERFORMANCE.md) — 벤치마크, 프로파일링, 메모리 측정

## UX / 기능 문서

- [Bridge PoC UX Flow](./BRIDGE_POC_UX_FLOW.md) — Engine-UI 연동 최소 UX 정의
- [Demo-site 데이터 실행 흐름](./DEMO_SITE_DATA.md) — Demo 시나리오 생성 및 검증

---

영문 문서는 [`../en`](../en/README.md)에서 확인할 수 있습니다.
