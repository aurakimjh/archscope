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

# Chrome Performance trace 또는 V8 .cpuprofile (Node --cpu-prof, CDP)
go run ./cmd/archscope-engine profile import \
  --in ./trace.json.gz \
  --format auto \
  --out browser-profile.json

# 리댁션된 HAR 가져오기 (방언 자동 판별, entry 상한)
go run ./cmd/archscope-engine http-capture analyze \
  --in ./session.har \
  --out http-capture.json

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

## 지원 언어와 Evidence 범위

ArchScope의 지원 범위는 evidence 기반입니다. Runtime artifact, log, profile,
trace, contract를 분석하며, application source code를 정적 분석하거나 직접
수정하지 않습니다.

| 영역 | 현재 지원 |
| --- | --- |
| ArchScope 구현 | Go engine, Wails desktop app, React/TypeScript frontend |
| JVM / Java evidence | GC log, JFR JSON, native-memory event, Java thread dump, jcmd JSON thread dump, Java exception stack, async-profiler/Jennifer profile evidence |
| Go evidence | goroutine dump, panic stack, pprof-compatible profile |
| Python evidence | traceback block, py-spy/faulthandler-style dump, py-spy profile evidence |
| Node.js evidence | diagnostic report, sample trace, JavaScript stack trace |
| .NET evidence | clrstack, Environment.StackTrace, exception/IIS evidence, dotnet-trace speedscope export |
| Ruby / PHP / Swift / native profile evidence | rbspy, StackProf, PHP Excimer/Tideways/Xdebug, Swift/async stack, perf collapsed/native stack을 지원 profile artifact로 제공한 경우 |
| 브라우저 / 프론트엔드 evidence | Chrome Performance trace(`.json`/`.json.gz`), V8 `.cpuprofile`(브라우저, Node `--cpu-prof`, CDP `Profiler.stop`) — sampled CPU run 분석 포함. CPU 샘플만 다루며 네트워크·레이아웃·페인트 귀속은 없음 |
| HTTP evidence | 방언 판별·가져오기 시점 리댁션이 있는 HAR 1.2 가져오기(`http_capture`); live capture는 Windows-first 로드맵 슬라이스 |
| 언어 중립 evidence | access/edge log, server log, OpenTelemetry log/trace, metrics snapshot, database/broker/platform evidence, OpenAPI, AsyncAPI, stitched evidence, architecture-doc draft |

지원하지 않거나 보류된 범위:

- Static source-code analysis, AST indexing, repository-wide code search,
  code quality scanning, automatic source modification.
- Heap dump parsing(`.hprof`)과 live CPU/RSS/syscall sampling 같은
  process/system monitoring.
- Roadmap에서 Active TO-DO로 승격되지 않은 direct SaaS APM connector.

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

이 기능은 source-editing coding agent가 아닙니다. 이미 생성된
`AnalysisResult`를 대상으로 하는 evidence-bound interpretation assistant이며,
deterministic analyzer output이 항상 source of truth입니다.

사용자 관점 흐름:

1. Deterministic analyzer를 실행하고 결과를 Analysis Workspace에 추가합니다.
2. AI interpretation payload가 있으면 Analysis Workspace가 provider, model,
   prompt version, disabled state, finding count, gate status를 표시합니다.
3. AI finding은 별도 AI-assisted panel에 표시되며, evidence gate를 통과한
   경우에만 Evidence Board 또는 Report Pack에 연결됩니다.
4. Ollama 또는 configured model을 사용할 수 없어도 deterministic analysis와
   export는 계속 동작합니다.

로컬 runtime 준비:

```bash
ollama serve
ollama pull qwen2.5-coder:7b
```

초기 정책은 `localhost`, `127.0.0.1`, `::1` Ollama endpoint만 허용합니다.
Model은 사용자가 설치하며 ArchScope desktop package에 번들링하지 않습니다.
전체 gate, redaction, prompt-injection, reporting 정책은
`docs/ko/AI_INTERPRETATION.md`를 기준으로 확인합니다.
