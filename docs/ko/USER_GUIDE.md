# 사용자 가이드

이 가이드는 현재 Go/Wails 기반 ArchScope 라인을 기준으로 합니다. 폐기된
Python/FastAPI browser app은 `archive/`에 보관되어 있으며 권장 실행 경로가
아닙니다.

## 빌드와 실행

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-profiler

cd cmd/archscope-profiler-app/frontend
npm ci
npm run build
```

데스크톱 패키징:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.84
cd apps/engine-native/cmd/archscope-profiler-app
task package
```

## CLI 예시

```bash
cd apps/engine-native

go run ./cmd/archscope-engine access-log analyze \
  --in ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out access.json

go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out profiler.json

go run ./cmd/archscope-engine thread-dump analyze \
  --in ../../examples/thread-dumps/java-jstack-sample.txt \
  --out thread.json
```

## 네이티브 앱

데스크톱 UI와 패키징 흐름은 `docs/ko/PROFILER_NATIVE.md`를 기준으로
확인하세요. Wails 앱은 profiler 분석과 Go 엔진의 일반 analyzer를 Wails
서비스로 노출합니다.

## AI 해석

AI 해석은 선택 기능이며 로컬 전용입니다. Go 구현은
`internal/aiinterpretation` 아래에 있으며 evidence 기반 prompt 생성,
민감정보 redaction, evidence reference 검증, localhost Ollama URL 제한을
수행합니다.
