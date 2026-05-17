# ArchScope 네이티브 앱

네이티브 앱은 이제 통합 Go 엔진 모듈 안에 있습니다.

`apps/engine-native/cmd/archscope-app`

기존 네이티브 POC 모듈은 `apps/engine-native/internal/profiler`와 Wails 앱
command tree로 통합되었습니다.

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
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
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
