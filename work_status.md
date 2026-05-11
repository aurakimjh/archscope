# ArchScope Work Status

Last updated: 2026-05-11

This file is the current execution status for the active ArchScope product line.
The previous long-form history was archived to
`archive/work_status_legacy_2026-05-09.md`.

## Current Baseline

- Active product: unified Go/Wails desktop implementation under
  `apps/engine-native`.
- Active UI: Wails v3 React frontend under
  `apps/engine-native/cmd/archscope-app/frontend`.
- Active engine: Go parser/analyzer/exporter/AI interpretation modules under
  `apps/engine-native/internal`.
- Release baseline: `v0.3.1` is the latest stable GitHub release. The
  `v0.3.1-rc1` prerelease remains available as the Jennifer MSA network-time
  release candidate.
- Current product expansion focus: local-first external trace import,
  Evidence Board, Incident Timeline, SLO/Golden Signals, and service-flow
  topology integration.
- Retired implementation: Python/FastAPI/browser sources are archived under
  `archive/python-engine` and `archive/web-frontend-python`.
- Historical native POC module has been folded into `apps/engine-native`.

## Completed In This Cycle

- Consolidated the Go engine, profiler core, Cobra CLIs, and Wails desktop app
  into `apps/engine-native`.
- Archived the Python engine and browser frontend instead of deleting them.
- Ported the evidence-bound AI interpretation module to Go.
- Updated release packaging around the Go/Wails desktop artifact.
- Rebuilt the `v0.3.0-rc1` release contents and changed the release from
  prerelease to latest.
- Verified representative Go parser/analyzer packages and profiler benchmarks.
- Audited large-file parsing behavior across logs, JSON inputs, profiler files,
  and thread-dump plugins.
- Aligned the active Go baseline with the latest stable Go toolchain
  (`go1.26.3`) and updated CI/release workflows to use `1.26.x`.
- Completed release verification for `v0.3.0-rc1`: local macOS Wails package,
  DMG validation, CLI analysis/export smoke tests, GitHub Release matrix review,
  and release asset checksum validation.
- Changed Jennifer profile analysis tabs so Summary, MSA Timeline, Profile
  Timeline, and Parser Report are visible before a file is analyzed.
- Optimized hot parser/profiler paths for common nginx access logs with
  response time and collapsed-stack flame graph construction.
- Optimized large graph rendering paths by capping displayed MSA timeline bars,
  reducing ECharts animation/resize churn, and using row-bucket hit testing for
  canvas flame graph hover lookup.
- Removed POC-era profiler-suffixed command/app path names from the active
  source and build surface; the desktop app now builds from
  `apps/engine-native/cmd/archscope-app`.
- Added Jennifer MSA service-call network-time summaries and network-aware
  topology placement so internal single-digit millisecond calls and
  gateway/external double-digit millisecond calls separate visually.
- Added `v0.3.1-rc1` and promoted the line to the stable `v0.3.1` desktop
  release.
- Added external trace import MVP support for OTLP JSON-file traces and
  Zipkin v2 JSON traces under `internal/parsers/traceimport` and
  `internal/analyzers/traceimport`.
- Added `archscope-engine trace import --in <file> --format
  auto|otlp-json|zipkin-v2-json` and trace sample fixtures under
  `examples/traces`.
- Added Korean APM import matrix and product expansion TODO documents covering
  trace import, Evidence Board, Incident Timeline, SLO/Golden Signals, service
  flow, and deferred SaaS connectors.

## Current Risk

The Electron-to-Wails migration risk is closed. The highest large-file issue
found in the 2026-05-09 audit has been mitigated: GC log analysis no longer
emits chart series for every event, and access-log/OTel analyzer entrypoints no
longer materialize the full parser record slice before aggregation.

Release verification found no new blocker for the 0.3.x Go/Wails line. Direct
Windows GUI launch was not performed in the local macOS environment; Windows
confidence currently comes from GitHub Actions Windows test/build/package
success plus release artifact checksum and PE inspection.

The MSA timeline discoverability issue is closed: the tab list now renders
before analysis and each tab shows a neutral empty-result state until data is
available. Large MSA timelines are display-capped for browser responsiveness.
Jennifer MSA topology now uses network-time groups to estimate service
distance, while the underlying `AnalysisResult` tables retain detailed call
metrics.

Trace import is now an engine/CLI MVP, not a full UI workflow. OTLP JSON-file
and Zipkin v2 JSON inputs are covered, but Elastic APM, Jaeger, SkyWalking, and
SaaS connector paths are still pending. Critical-path analysis, richer findings,
and Wails page integration remain the highest trace-import follow-ups.

Remaining large-file risk is concentrated in structured formats that naturally
require object materialization, such as JFR JSON, Node diagnostic reports, jcmd
JSON, and self-contained HTML profiler files. These paths now have documented
guardrails or targeted preflight, but multi-GB structured inputs should still be
filtered before analysis.

## Large-File Audit Snapshot

| Analyzer | Synthetic input | Time | Peak RSS | Output | Status |
|---|---:|---:|---:|---:|---|
| Access log | 30 MB / 300k lines | 0.53s | 19 MB | 10 KB | Streaming parser/analyzer path verified |
| OTel JSONL | 31 MB / 200k lines | 0.25s | 18 MB | 6.3 KB | Streaming parser/analyzer path with trace-detail cap verified |
| GC log | 34 MB / 300k lines | 1.55s | 305 MB | 3.5 MB | Series cap/downsampling verified |
| Collapsed profiler | 29 MB / 500k lines | 0.26s | 16 MB | 6.8 KB | Good baseline |

## Next Execution Queue

1. Wire `trace_import` into the Wails UI: summary cards, service dependency
   table/chart, trace table, span table, and findings panel.
2. Implement Elastic APM importers for Elasticsearch `_search` response JSON
   and `_source` NDJSON exports.
3. Add trace critical-path and richer findings:
   `SLOW_TRACE_P95`, `CLOCK_SKEW_SUSPECTED`,
   `UNBALANCED_SERVICE_LATENCY`, and `HIGH_ERROR_SERVICE_EDGE`.
4. Start the Evidence Board skeleton and define the common evidence-card model
   shared by analyzer findings, chart selections, table rows, and parser
   diagnostics.
5. Perform direct Windows GUI launch smoke-test on a Windows host/VM and
   continue signing/notarization plus frontend bundle-splitting release work.
6. Consider deeper GC event streaming if future real-world logs exceed the
   current 305 MB RSS envelope.

## Active TO-DO

| ID | Priority | Status | Task | Depends on | Output |
|---|---|---|---|---|---|
| T-393 | P0 | [x] | Added GC analyzer `MaxSeriesPoints` limits and deterministic event-bucket downsampling for pause, heap, metaspace, allocation, and promotion series while preserving exact summary metrics and findings. | Current Go GC analyzer | Bounded `gc_log` result size and lower RSS |
| T-394 | P0 | [x] | Added a reproducible GC large-file regression benchmark that asserts output size stays bounded for synthetic 300k+ event logs. | T-393 | GC performance regression guard |
| T-395 | P1 | [x] | Added a common streaming text API in `internal/textio`, including encoding fallback, line numbers, cancellation hooks, and diagnostic context support without returning `[]string`. | Existing textio encoding support | Shared line streaming primitive |
| T-396 | P1 | [x] | Converted access log, GC log, OTel JSONL, exception, and simple runtime-stack parsers from `IterTextLines`/`ReadAll` to the streaming text API. | T-395 | Parser memory no longer proportional to full line slices |
| T-397 | P1 | [x] | Refactored access log and OTel analyzer entrypoints to aggregate from parser callbacks; OTel trace detail retention is capped per trace while summary counters consume all records. GC log memory risk is reduced by streaming parser input and bounded series output from T-393/T-396. | T-396, T-393 | Memory-bounded analyzer paths |
| T-398 | P2 | [x] | Added JFR JSON large-file controls with direct-load file-size preflight and guidance to use event/time/stack-depth filters before analysis. | T-395 | Safer JFR JSON ingestion |
| T-399 | P2 | [x] | Replaced Jennifer profile global `ReadFile` and TXID index splitting with streaming block segmentation. | T-395 | Bounded Jennifer profile parser |
| T-400 | P2 | [x] | Converted Java jstack high-volume section scanning to stream lines instead of materializing the whole file; remaining full-read thread dump formats are documented as structured-format guardrail targets. | T-395 | Large thread-dump safety pass |
| T-401 | P2 | [x] | Reduced profiler SVG memory copies and added size preflight for self-contained HTML payloads. | T-395 | Lower parser copy overhead for visual profiler inputs |
| T-402 | P2 | [x] | Updated English/Korean parser and performance docs with warning thresholds, stream-only paths, max-lines/time filters, output downsampling, and UI messaging for large inputs. | T-393, T-397 | Current large-file operations guide |
| T-403 | P0 | [x] | Updated local Go to `go1.26.3`, set `go.mod`/`go.work` to Go 1.26.3, moved CI/release workflows to `go-version: 1.26.x`, aligned release Node setup to 22, added the missing `0.3.0-rc1` changelog entry, and corrected Android build metadata from `1.0` to `0.3.0-rc1`. | Release verification request | Version/toolchain baseline aligned |
| T-404 | P0 | [x] | Verified the `v0.3.0-rc1` release path locally and remotely: Go vet/build/race tests, frontend build, Wails macOS package, app launch smoke, code signature check, DMG checksum, CLI analysis/export smoke tests, GitHub Release matrix review, and release asset SHA256 validation. | T-403 | Release verification evidence captured |
| T-405 | P1 | [x] | Made Jennifer profile tabs visible before analysis and added a consistent empty-result state for Summary, MSA Timeline, Profile Timeline, and Parser Report. | Jennifer profile UI | MSA timeline menu discoverable before analysis |
| T-406 | P1 | [x] | Added a fast path for common nginx access-log lines with trailing response time, preserved fallback parsing, and reduced collapsed profiler stack-splitting/whitespace-scan allocations. | Large-log audit findings | Faster parser/profiler hot paths |
| T-407 | P1 | [x] | Optimized graph rendering for large results: MSA timeline display cap and row-index map, ECharts canvas/no-animation/lazy resize updates, and row-bucket hit testing for canvas flame graph hover. | T-405, graph UI components | Lower browser render and interaction cost |
| T-408 | P1 | [x] | Renamed the Wails app command tree to `cmd/archscope-app`, retired the duplicate POC CLI path in favor of `archscope-engine profiler ...`, renamed the native workflow/docs, and regenerated Wails bindings under the new path. | Official ArchScope naming | Build/source paths no longer expose POC-era profiler suffixes |
| T-409 | P1 | [x] | Added Jennifer MSA service-call network summaries and network-time topology grouping so internal and gateway/external call distances are visually separated. | Jennifer MSA result contract | Service position inference from network-time bands |
| T-410 | P2 | [x] | Added sortable table header support for dense frontend tables. | Wails frontend table UX | Reusable table sorting hook |
| T-411 | P1 | [x] | Added external trace import MVP for OTLP JSON-file and Zipkin v2 JSON inputs, including canonical span parsing, `trace_import` analysis result, samples, and CLI command. | Product expansion import matrix | Local-first APM trace evidence import |
| T-412 | P1 | [x] | Added APM import matrix and product expansion TODO docs covering standard file imports, deferred SaaS connectors, Evidence Board, Incident Timeline, and SLO/golden signals. | Product expansion research | Prioritized product expansion backlog |
| T-413 | P0 | [x] | Promoted desktop metadata and GitHub release line to `0.3.1` after the `0.3.1-rc1` Jennifer MSA prerelease. | T-409, T-411 | Stable 0.3.1 desktop release baseline |
| T-414 | P1 | [ ] | Connect `trace_import` to the Wails UI with summary cards, service dependency view, trace table, span table, and findings panel. | T-411 | Trace Import desktop workflow |
| T-415 | P1 | [ ] | Add Elastic APM `_search` response and source-only NDJSON importers. | T-411, APM import matrix | Elastic trace evidence import |
| T-416 | P1 | [ ] | Add trace critical-path analysis and expanded trace findings. | T-411 | Root-cause oriented trace diagnostics |
| T-417 | P1 | [ ] | Design and build the Evidence Board skeleton around reusable evidence cards. | T-412, analyzer result contracts | Cross-analyzer evidence pack foundation |
| T-418 | P1 | [ ] | Run direct Windows GUI launch smoke-test for the 0.3.1 line on a Windows host/VM. | 0.3.1 release assets | Native Windows confidence beyond CI/package success |

## Verification Notes

- `go test ./...` passed under `apps/engine-native`. The Wails app test build
  still emits the known macOS linker warning about object files built for newer
  macOS 26.0 than the 11.0 link target.
- `go test -bench BenchmarkBuildLargeSyntheticGCLog -benchtime=1x -benchmem
  ./internal/analyzers/gclog` passed with a 300k-event JSON payload of about
  1.9 MB.
- Synthetic large-file measurements were captured with the current
  `cmd/archscope-engine` binary built from `apps/engine-native`.
- 2026-05-10 release verification ran with local `go1.26.3`. `go vet ./...`,
  `go build ./cmd/archscope-engine ./cmd/archscope-app`, and
  `go test ./... -race -count=1` passed
  under `apps/engine-native` using `/tmp` Go caches. The race test required
  loopback permission for `httptest`.
- `npm ci` and `npm run build` passed for the Wails frontend. Vite still warns
  that the main JS chunk is larger than 500 KB, matching the existing
  bundle-splitting follow-up.
- `task package` and `task darwin:package:dmg ARCH=arm64` produced
  `bin/archscope.app` and `bin/archscope-arm64.dmg`. The app launched without
  immediate crash, ad-hoc `codesign --verify --deep --strict` passed,
  `CFBundleShortVersionString`/`CFBundleVersion` are `0.3.0-rc1`, and
  `hdiutil verify` reported a valid DMG checksum.
- CLI smoke tests produced access-log and profiler `AnalysisResult` JSON plus
  HTML/CSV report outputs in `/tmp`.
- GitHub Actions run `25602445878` completed the `v0.3.0-rc1` Release workflow
  successfully across darwin-arm64, windows-amd64, and linux-amd64 packaging
  plus GitHub Release creation. Latest main CI runs `25603594053` and
  `25603594056` were also successful, including Windows `go vet`, `go build`,
  and `go test`.
- Downloaded release assets matched `SHA256SUMS`; Windows zip contains an
  x86-64 `archscope.exe`, while the NSIS installer wrapper is a 32-bit PE
  self-extractor as expected.
- 2026-05-10 UI/performance pass: `npm run build` passed for the Wails
  frontend; Vite still reports the existing >500 KB main chunk warning.
- `go test ./internal/parsers/accesslog ./internal/profiler` and
  `go test ./...` passed under `apps/engine-native` using `/tmp` Go caches.
- Parser/profiler benchmarks after the hot-path changes:
  `BenchmarkParseLineNginxWithResponseTime` 165.4 ns/op, 144 B/op, 1 alloc/op;
  `BenchmarkAnalyzeCollapsedSampleWall` 86.7 ms/op, 53.6 KB/op, 549 allocs/op;
  `BenchmarkAnalyzeJenniferSample` 41.5 us/op, 41.3 KB/op, 422 allocs/op.
- 2026-05-10 naming cleanup: `go test ./...`, `go build
  ./cmd/archscope-engine ./cmd/archscope-app`, `npm run build`, `task
  package`, `task darwin:package:dmg ARCH=arm64`, `codesign --verify --deep
  --strict bin/archscope.app`, and `hdiutil verify bin/archscope-arm64.dmg`
  passed from the renamed `cmd/archscope-app` tree.
- 2026-05-10 `v0.3.1-rc1` prerelease was created from the Jennifer MSA
  network-time grouping work. GitHub Release workflow completed successfully
  across darwin-arm64, windows-amd64, linux-amd64, and release creation.
- 2026-05-11 `v0.3.1` is listed as the latest GitHub Release. It promotes the
  0.3.1 line to stable and includes the trace import MVP plus the Jennifer MSA
  network-time improvements carried forward from `v0.3.1-rc1`.
- 2026-05-11 local trace import verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test
  ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine`.

## Decisions

- Go/Wails is the primary active runtime and distribution path.
- Wails service bindings are the active engine/UI boundary.
- Local HTTP/FastAPI/browser serving is retired and retained only in `archive/`.
- Python sources are retained for behavior reference and audits only.
- Large-file safety work should happen in Go first; archived Python code should
  not receive new product features.

## Archive

- Old work status: `archive/work_status_legacy_2026-05-09.md`
- Archived Python engine: `archive/python-engine`
- Archived browser frontend: `archive/web-frontend-python`
