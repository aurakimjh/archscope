# 아키텍처

ArchScope의 현재 아키텍처는 단일 Go 모듈과 Wails 데스크톱 셸입니다.

```text
Wails 데스크톱 UI
  apps/engine-native/cmd/archscope-app/frontend
        |
        v
Wails 서비스
  apps/engine-native/api
  apps/engine-native/cmd/archscope-app/*service.go
        |
        v
Go 엔진
  parsers -> analyzers -> exporters -> AnalysisResult
```

## 저장소 경계

| 경로 | 상태 | 책임 |
| --- | --- | --- |
| `apps/engine-native/` | 활성 | Go 엔진, profiler core, CLI, Wails 앱 |
| `archive/python-engine/` | 폐기 | 이전 Python 엔진, 참조용 보관 |
| `archive/web-frontend-python/` | 폐기 | 이전 browser frontend, 참조용 보관 |
| `docs/en`, `docs/ko` | 활성 | 현재 문서 |

## 계약

- `internal/models.AnalysisResult`는 non-profiler analyzer와 Wails
  renderer가 공유하는 공통 결과 envelope입니다.
- `internal/profiler.AnalysisResult`는 profiler 전용 typed envelope입니다.
  CLI는 이를 직접 직렬화하고, demo-site 통합은 공통 envelope shape로
  변환합니다.
- Parser diagnostics는 안정적인 JSON 키로 배출되어 UI와 report exporter가
  partial-success와 parse-error 상태를 렌더링할 수 있습니다.

## AI 해석

Evidence 기반 AI 해석 모듈은 Go로 포팅되어
`internal/aiinterpretation`에 있습니다.

- Prompt는 선택된 evidence를 데이터로 전달합니다.
- 민감 값은 prompt 생성 전에 redaction됩니다.
- Finding은 유효한 `evidence://...` reference를 인용해야 합니다.
- 로컬 실행은 localhost Ollama URL만 허용하며 기본값은 비활성입니다.

## 빌드 모델

이전 Electron 및 Python/FastAPI 배포 경로는 폐기되었습니다. 목표 릴리스
산출물은 Wails 데스크톱 앱이며, CI와 스크립팅을 위해 Go CLI를 제공합니다.

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```
