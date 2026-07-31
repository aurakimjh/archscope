# Performance

Performance measurement now targets the Go engine and Wails desktop
build.

## Baseline Commands

```bash
cd apps/engine-native
go test ./...
go test -bench=. -run=^$ ./internal/profiler
go build -trimpath -ldflags="-s -w" ./cmd/archscope-engine ./cmd/archscope-app
```

Frontend build size:

```bash
cd apps/engine-native/cmd/archscope-app/frontend
npm ci
npm run build
```

## Windows Resolver Cost

R-RG1 measures the real Windows endpoint-owner path separately from unit tests.
The measurement keeps one accepted loopback TCP connection open and performs
50 attribution samples. Each sample includes the two immediate
`GetExtendedTcpTable` reads and process start-time confirmation used by the
production resolver.

```powershell
$env:ARCHSCOPE_RUN_RESOLVER_PERF = "1"
$env:ARCHSCOPE_RESOLVER_PERF_OUT = "$env:TEMP\archscope-resolver-cost.json"
go test -count=1 -v ./internal/capture/procmap `
  -run '^TestWindowsResolverCostMeasurement$'
```

The schema-v1 artifact reports sample count, confirmed count, mean, p50, p95,
and maximum duration without process paths or session identifiers. R-RG1 did
not predeclare a product latency SLO, so this gate records cost and verifies
correct attribution rather than inventing a pass threshold after measurement.
The production capture path resolves once per accepted connection, not once
per HTTP request.

The first native R-RG1 run on 2026-07-31
([GitHub Actions run 30601745253](https://github.com/aurakimjh/archscope/actions/runs/30601745253))
confirmed all 50/50 samples on Windows amd64. It measured mean `0.539 ms`,
p50 `0.547 ms`, p95 `0.731 ms`, and maximum `0.857 ms`. These values are a
runner-specific baseline, not a product SLO.

## Budget

- Keep the desktop binary small enough for direct field distribution.
- Avoid reintroducing Electron or an HTTP server into the release binary.
- Prefer streaming parsers and bounded diagnostics for large profiler,
  GC, access-log, and thread-dump inputs.

## Large-File Policy

The active Go engine treats large inputs as an offline field workload, not as a
browser upload workload.

- Text log parsers should use `internal/textio.ForEachTextLine` so files are
  decoded line-by-line instead of through `ReadAll`.
- GC log chart series are capped by `MaxSeriesPoints` and downsampled
  deterministically; summary metrics and findings still use all parsed events.
- Access-log and OTel analyzer entrypoints aggregate from parser callbacks.
  OTel keeps exact summary counters but caps retained per-trace detail rows.
- JFR JSON direct loading has a file-size preflight. Large recordings should be
  exported with `jfr print --events`, time windows, or stack-depth filters
  before analysis.
- Jennifer profile exports are segmented by TXID blocks while streaming so one
  transaction block can be parsed and released at a time.
- Java jstack section parsing streams lines. Structured thread-dump formats
  such as jcmd JSON, Node diagnostic reports, and .NET clrstack should keep
  size preflight or format-specific streaming before multi-GB use.
- HTML profiler inputs are size-checked before direct parsing; SVG parsing uses
  a byte reader to avoid an extra whole-file string copy.
- Browser/V8 profile inputs (`chrome-trace-json`, `v8-cpuprofile`, including
  `.json.gz`/`.cpuprofile.gz`) stream with a 256 MiB byte guard and a
  500,000-sample cap. Overflow triggers deterministic time-weighted bucket
  downsampling recorded via `PROFILE_DOWNSAMPLED` and
  `metadata.partial_result`; time-axis outputs (`cpu_sample_runs`,
  `cpu_activity`, `SAMPLED_CPU_HOTSPOT`) are suppressed for downsampled
  inputs because uniform downsampling distorts time windows.

Recommended warning thresholds for UI/CLI messaging:

| Input size | Policy |
|---:|---|
| 100 MB+ | Show a large-file notice and surface available filters. |
| 500 MB+ | Prefer `max_lines`, event filters, or time windows where the format supports them. |
| 1 GB+ | Use stream-only paths; avoid direct JSON/HTML ingestion unless explicitly filtered. |
