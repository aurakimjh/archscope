# ArchScope 네이티브 앱

네이티브 앱은 이제 통합 Go 엔진 모듈 안에 있습니다.

`apps/engine-native/cmd/archscope-profiler-app`

기존 `apps/profiler-native` 모듈은
`apps/engine-native/internal/profiler`와 Wails 앱 command tree로 통합되었습니다.

## 빌드

```bash
cd apps/engine-native
go test ./...

cd cmd/archscope-profiler-app/frontend
npm ci
npm run build
```

데스크톱 패키징:

```bash
git clone --depth 1 --branch v3.0.0-alpha.87 https://github.com/wailsapp/wails.git /tmp/wails
(cd /tmp/wails/v3 && go install ./cmd/wails3)
cd apps/engine-native/cmd/archscope-profiler-app
task package
```

## 기능

- Profiler 입력: collapsed stack, Jennifer CSV, FlameGraph SVG,
  async-profiler/inline-SVG HTML.
- Analyzer 입력: access log, GC log, JFR, exception stack,
  OpenTelemetry log, multi-runtime thread dump.
- Drill-down, execution breakdown, timeline analysis, profiler diff,
  pprof export, parser diagnostics, debug log.
- Light/dark/system theme, 한국어/영어 locale, recent files,
  취소 가능한 비동기 분석.

## CLI

Profiler 전용 CLI:

```bash
cd apps/engine-native
go run ./cmd/archscope-profiler \
  --collapsed ../../examples/profiler/sample-wall.collapsed \
  --interval-ms 100 \
  --top-n 20
```

통합 엔진 CLI:

```bash
cd apps/engine-native
go run ./cmd/archscope-engine profiler analyze-collapsed \
  --in ../../examples/profiler/sample-wall.collapsed \
  --out result.json
```

## CI

`.github/workflows/profiler-native.yml`은 이제 통합된 `apps/engine-native`
모듈과 Wails frontend build를 검증합니다.
