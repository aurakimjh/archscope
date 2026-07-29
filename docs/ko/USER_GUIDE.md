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

# Local Lighthouse report (score 보존, URL 리댁션)
go run ./cmd/archscope-engine browser import \
  --in ./lighthouse-report.json \
  --format lighthouse-json \
  --out browser-audit.json

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
| HTTP evidence | 방언 판별·가져오기 시점 리댁션이 있는 HAR 1.2 가져오기(`http_capture`); Windows 실시간 HTTP/1.x metadata 캡처는 구현되었으며 H-RG4 Windows acceptance 대기 중 |
| 언어 중립 evidence | access/edge log, server log, OpenTelemetry log/trace, metrics snapshot, database/broker/platform evidence, OpenAPI, AsyncAPI, stitched evidence, architecture-doc draft |

지원하지 않거나 보류된 범위:

- Static source-code analysis, AST indexing, repository-wide code search,
  code quality scanning, automatic source modification.
- Heap dump parsing(`.hprof`)과 live CPU/RSS/syscall sampling 같은
  process/system monitoring.
- Roadmap에서 Active TO-DO로 승격되지 않은 direct SaaS APM connector.

## Windows 실시간 HTTP 캡처

HTTP Capture 화면에는 T-581 Windows 실시간 캡처 review candidate가 포함되어
있습니다. H-RG4는 2026-07-29 세 번째 `CONDITIONAL` 판정을 받았으므로 릴리스된
capture tier가 아니라 acceptance 작업 용도로 사용합니다.

1. 최초 사용 프록시/CA 경고를 읽고 동의합니다.
2. HTTPS 가로채기가 필요하면 임시 캡처 CA를 설치합니다.
3. 프로세스를 확정하지 못한 트래픽의 리댁션된 metadata가 명시적으로 필요하지
   않다면 미귀속 보존을 끈 상태로 둡니다. 기본값은 drop입니다.
4. 캡처를 시작하고 의도한 test client가 화면에 표시된 loopback proxy를
   사용하도록 설정합니다.
5. 캡처를 중지합니다. ArchScope는 임시 CA를 제거하며 같은 화면에서 종료된
   세션을 일반 `http_capture` 분석 화면으로 불러올 수 있습니다.

실시간 renderer는 최신 metadata-only 행 500개를 유지하며 renderer event가
누락되면 권위 있는 live window를 다시 불러옵니다. 요청·응답 body는 항상
생략하며 SEC-10 crash-dump 제외 preflight가 구현되기 전에는 body capture를
활성화하지 않습니다. 백엔드는 아직 결정되지 않은 MITM progress 행에는
`pending`, passthrough에는 `unsupported`를 사용하며 opaque in-flight 트래픽을
semantic capture로 표시하지 않습니다. Windows CurrentUser root store는 Windows
신뢰 저장소를 사용하는 client에만 적용됩니다. JVM/JSSE와 NSS 기반 client는 별도
CA import가 필요하므로 acceptance harness의 JVM HTTPS에는 JVM truststore가
필수입니다.

중지 후 finalized 분석은 저장된 행에서 capture mode와 최약 fidelity를
계산합니다. mixed/unsupported 세션은 더 이상 HAR-import/foreign-tool/semantic
metadata를 상속하지 않습니다. TLS interception 실패는 attribution이 보존된
`proxy_not_captured` / `unsupported`로, 실패한 명시적 또는 h2-only tunnel은
`proxy_passthrough` / `unsupported`로 기록됩니다. Capture-time redaction count는
manifest에 저장되어 finalized 분석과 acceptance evidence로 전달됩니다.
동일 flush checkpoint에 crash recovery용 capture counter도 저장됩니다. 중지 시
진행 중인 행은 `aborted` / `unsupported`가 되며 progress 전용 `pending` 등급은
finalized transaction에 남지 않습니다. Checkpoint가 없는 구형 store는 백엔드
evidence에서 redaction을 unknown으로 표시하고 persisted 행에서 보수적인 counter를
계산하며, finalized card는 이 상태를 `기록 없음`으로 표시합니다. 리댁션 자체는
모든 기록 전에 수행되므로 요약이 없는 것은 기록이 없는 것이지 "해당 민감 필드가
없었다"로 표시하지 않습니다.

Finalized 캡처 충실도 카드는 엔진이 보고한 provenance에 따라 설명 문구를
선택합니다. 라이브 세션은 프록시가 직접 시간을 측정한 ArchScope 라이브 프록시
캡처로 설명하고, 외부 도구에서 가져온 증거라는 설명은 HAR import 에만
사용합니다. Fidelity, capture mode, 관측 지점, 상세 저장 방식은 두 언어 모두
현지화된 라벨로 표시하며 원본 엔진 토큰은 hover 로 확인할 수 있습니다. 집계
등급 아래에는 트랜잭션별 분포를 함께 표시하므로 `mixed`/`unsupported` 세션도
실제 분포를 확인할 수 있습니다.

`coverage: confirmed`의 의미는 제한적입니다. 동일 client/proxy endpoint tuple과
owner PID를 연속 두 번의 TCP-owner table 조회에서 확인하고 두 번째 조회 전후의
process start time이 동일함을 확인했다는 뜻입니다. Attribution은 HTTP request마다
반복하지 않고 accepted client connection마다 한 번 계산합니다. 전체 트래픽
coverage나 모든 PID 재사용 race 제거를 의미하지 않습니다. 불안정하거나 사라진
row는 `inferred`/`unknown`이며 SEC-17 metadata 보존을 명시적으로 켜지 않으면
drop합니다.

Windows acceptance와 package signature 증거 생성:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/verify-windows-live-capture.ps1 `
  -ProxyAddress 127.0.0.1:43123 `
  -HttpTargetUrl http://127.0.0.1:8080/health `
  -HttpsTargetUrl https://127.0.0.1:8443/health `
  -SessionPath "$env:LOCALAPPDATA\ArchScope\captures\cap-..." `
  -RecoverySessionPath "$env:LOCALAPPDATA\ArchScope\captures\cap-recovered-..." `
  -ArchScopeEngineExe .\bin\archscope-engine.exe `
  -WebViewDebugPort 9223 `
  -JavaTrustStore .\tmp\archscope-t581.jks `
  -ArchScopeExe .\bin\archscope.exe
```

Acceptance 앱은 `t581e2e` tag로 빌드하고
`ARCHSCOPE_E2E_CDP_PORT=9223`으로 실행합니다. Production build는 이 debugging
port를 노출하지 않습니다. 두 target URL은 loopback fixture origin이어야 하며
h2-only probe를 위해 HTTPS fixture는 h2를 지원해야 합니다. Schema-v4 harness는
browser/curl/JVM/Electron HTTP/HTTPS, attribution이 있는 pinning 실패, h2-only
passthrough, 명시적 QUIC/UDP 비가시성, 최소 1,000건 장시간 요청, WebView page
재진입, 별도 실제 crash-recovery session을 필수로 검증합니다. Main UI capture
중지를 기다린 뒤 두 store를 read-only `http-capture acceptance-evidence`로
readback하고 누락·계약 불일치 시 실패합니다. 이 명령은 capture를 시작하지
않으며 metadata-only 증거를 owner-only 권한으로 기록합니다. Artifact는 local
session path를 제외하고 행을 2,000개로 제한하며 loopback fixture privacy scope를
명시합니다. 공개 보관 전 내용을 검토한 뒤 JSON 또는 repository path와 checksum을
기록해야 합니다. 해당 Windows artifact 와 독립 H-RG4 재리뷰 `PASS` 전까지
T-581은 `REVIEW` 상태입니다.

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
