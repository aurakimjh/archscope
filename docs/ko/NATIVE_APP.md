# ArchScope 네이티브 앱

네이티브 앱은 이제 통합 Go 엔진 모듈 안에 있습니다.

`apps/engine-native/cmd/archscope-app`

기존 네이티브 POC 모듈은 `apps/engine-native/internal/profiler`와 Wails 앱
command tree로 통합되었습니다.

ArchScope는 macOS, Windows, Linux에서 실행하는 **Wails 3.1.0 네이티브
데스크톱 앱**입니다. 저장소 루트에서 macOS/Linux는 `scripts/dev-desktop.sh`,
Windows는 `scripts/dev-desktop.ps1`로 개발을 시작합니다. 이 명령은 Wails
네이티브 창을 열며 Vite는 개발용 자산만 제공합니다. 프런트엔드 URL을
브라우저에서 열어도 제품 UI는 초기화되지 않습니다.

서버/Docker 호스팅, 브라우저 미리보기, Electron 셸, 모바일 빌드는 지원하지
않습니다. 서버/모바일 빌드 태그는 컴파일 단계에서 차단합니다. Docker 교차
컴파일 도구는 네이티브 데스크톱 산출물을 만드는 용도로 유지합니다.
WebKit, WebView2, WebKitGTK는 네이티브 렌더링에 필요한 구성 요소입니다.

패키징 전에 루트에서 `node scripts/verify-desktop-only.mjs --check-server-build`를
실행하세요. SDK 빌드 자산 갱신은 임시 프로젝트에서 검토한 뒤 데스크톱 플랫폼
파일에 반영합니다. 기존 프로젝트에 전체 템플릿을 다시 생성하지 마세요.
`archive/`의 과거 Python/브라우저 소스는 참고용이며 제품에 포함하지 않습니다.

## 빌드

```bash
cd apps/engine-native
go test ./...

cd cmd/archscope-app/frontend
npm ci
npm run build
```

데스크톱 패키징:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
cd apps/engine-native/cmd/archscope-app
task package
```

## 기능

- Profiler 입력: collapsed stack, Jennifer CSV, FlameGraph SVG/HTML,
  async-profiler HTML, pprof, py-spy, rbspy, speedscope/dotnet-trace,
  perf collapsed, StackProf, PHP profiler export, Xdebug, Swift/async stack,
  Pyroscope/Phlare, Parca snapshot.
- Analyzer 입력: access/edge log, server log, OpenTelemetry log, metrics
  snapshot, observability export, database slow-query evidence, broker log,
  Kubernetes/container/cloud evidence, trace import, GC log, JFR JSON,
  native memory, exception stack, multi-runtime thread dump.
- Derived workflow: Analysis Workspace, Evidence Board, Incident Timeline,
  SLO/Golden Signals, Service Flow, stitched evidence drilldown,
  API/event contract analysis, evidence 기반 architecture documentation draft.
- Drill-down, execution breakdown, timeline analysis, profiler diff,
  pprof export, parser diagnostics, debug log, chart export, report diff,
  evidence pack, report-pack ZIP export.
- Light/dark/system theme, 한국어/영어 locale, recent files,
  취소 가능한 비동기 분석.

## CLI

통합 엔진 CLI:

```bash
cd apps/engine-native
go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out result.json
```

최근 evidence workflow 예시:

```bash
go run ./cmd/archscope-engine trace import \
  --in ../../examples/traces/sample-otlp-traces.jsonl \
  --format auto \
  --out trace.json

go run ./cmd/archscope-engine stitch analyze \
  --in ../../examples/stitching/access-result.json \
  --in ../../examples/stitching/trace-result.json \
  --in ../../examples/stitching/database-result.json \
  --time-window-seconds 60 \
  --out stitched.json

go run ./cmd/archscope-engine api-contract analyze \
  --openapi ../../examples/api-contract/openapi-orders.json \
  --access-result ../../examples/api-contract/access-result.json \
  --asyncapi ../../examples/api-contract/asyncapi-orders.json \
  --broker-result ../../examples/api-contract/broker-result.json \
  --out contract.json

go run ./cmd/archscope-engine architecture-docs draft \
  --in contract.json --in stitched.json \
  --out architecture-docs.json
```

현재 CLI surface는 `go run ./cmd/archscope-engine --help`와
[Importer Support Matrix](./IMPORTER_SUPPORT_MATRIX.md)를 기준으로 확인합니다.

## CI

`.github/workflows/engine-native.yml`은 이제 통합된 `apps/engine-native`
모듈과 Wails frontend build를 검증합니다.
