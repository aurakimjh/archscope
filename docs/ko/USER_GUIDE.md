# 사용자 가이드

이 가이드는 현재 Go/Wails 기반 ArchScope 라인을 기준으로 합니다. 폐기된
Python/FastAPI browser app은 `archive/`에 보관되어 있으며 권장 실행 경로가
아닙니다.

## 빌드와 실행

```bash
cd apps/engine-native
go test ./...
go build ./cmd/archscope-engine ./cmd/archscope-app

cd cmd/archscope-app/frontend
npm ci
npm run build
```

데스크톱 패키징:

```bash
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
cd apps/engine-native/cmd/archscope-app
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
  --in ../../examples/thread-dumps/sample-java-thread-dump.txt \
  --out thread.json

go run ./cmd/archscope-engine trace import \
  --in ../../examples/traces/sample-otlp-traces.jsonl \
  --format auto \
  --out trace.json

go run ./cmd/archscope-engine database-log analyze \
  --in ../../examples/database/sample-postgres.log \
  --format postgres-text \
  --out database.json

go run ./cmd/archscope-engine broker-log analyze \
  --in ../../examples/broker/sample-broker.log \
  --format auto \
  --out broker.json

go run ./cmd/archscope-engine api-contract analyze \
  --openapi ../../examples/api-contract/openapi-orders.json \
  --access-result ../../examples/api-contract/access-result.json \
  --asyncapi ../../examples/api-contract/asyncapi-orders.json \
  --broker-result ../../examples/api-contract/broker-result.json \
  --out contract.json

go run ./cmd/archscope-engine stitch analyze \
  --in ../../examples/stitching/access-result.json \
  --in ../../examples/stitching/trace-result.json \
  --in ../../examples/stitching/database-result.json \
  --time-window-seconds 60 \
  --out stitched.json

go run ./cmd/archscope-engine architecture-docs draft \
  --in contract.json --in stitched.json \
  --out architecture-docs.json

go run ./cmd/archscope-engine report html \
  --in architecture-docs.json \
  --out architecture-docs.html
```

전체 command 목록은 `go run ./cmd/archscope-engine --help`로 확인합니다.
현재 지원 evidence family는 `docs/ko/IMPORTER_SUPPORT_MATRIX.md`에 정리되어
있습니다.

## 네이티브 앱

데스크톱 UI와 패키징 흐름은 `docs/ko/NATIVE_APP.md`를 기준으로
확인하세요. Wails 앱은 profiler 분석과 Go 엔진의 일반 analyzer를 Wails
서비스로 노출합니다. 현재 workspace surface는 Analysis Workspace,
Evidence Board, Incident Timeline, SLO/Golden Signals, Service Flow,
stitched-evidence drilldown state, Export Center, Report Pack, Chart Studio입니다.

## AI 해석

AI 해석은 선택 기능이며 로컬 전용입니다. Go 구현은
`internal/aiinterpretation` 아래에 있으며 evidence 기반 prompt 생성,
민감정보 redaction, evidence reference 검증, localhost Ollama URL 제한을
수행합니다.
