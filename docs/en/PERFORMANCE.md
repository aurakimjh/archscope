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
