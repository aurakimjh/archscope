# ArchScope Work Status

Last updated: 2026-05-13

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
- Current execution focus: JFR/async-profiler diagnostics and Evidence Board
  expansion after the trace import desktop workflow, Windows GUI smoke
  workflow, and 0.3.x release hardening pass landed.
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
- Researched product expansion and external APM import priorities around
  local-first trace evidence, Evidence Board, Incident Timeline,
  SLO/Golden Signals, service flow, and deferred SaaS connectors.
- Consolidated product expansion and external APM planning into the English and
  Korean roadmap documents, then removed the former Korean-only planning notes
  so `docs/en` and `docs/ko` Markdown files stay paired.
- Restored and extended GC log memory-space analysis so young/old/metaspace
  event series are emitted again, and added OOM plus long-pause alert findings.
- Connected `trace_import` to the Wails desktop UI with summary cards,
  service dependency and service latency charts, trace/span tables, critical
  path rows, parser diagnostics, and findings.
- Added Elastic APM file import for Elasticsearch `_search` response JSON and
  source-only NDJSON exports.
- Added trace critical-path analysis and MVP findings:
  `SLOW_TRACE_P95`, `CLOCK_SKEW_SUSPECTED`,
  `UNBALANCED_SERVICE_LATENCY`, and `HIGH_ERROR_SERVICE_EDGE`.
- Added the first Evidence Board skeleton with local evidence cards for trace
  findings, service edges, traces, and source metadata.
- Added a Windows GUI smoke workflow that builds the Wails Windows executable
  on `windows-latest`, launches `archscope.exe`, verifies it stays alive, and
  shuts it down cleanly.
- Hardened the 0.3.x release workflow with macOS signing/notarization secret
  preflight, signature/stapler verification, route-level frontend lazy loading,
  modular ECharts bundling, and explicit frontend chunk budgets.

## Current Risk

The Electron-to-Wails migration risk is closed. The highest large-file issue
found in the 2026-05-09 audit has been mitigated: GC log analysis no longer
emits chart series for every event, and access-log/OTel analyzer entrypoints no
longer materialize the full parser record slice before aggregation.

Release verification found no new blocker for the 0.3.x Go/Wails line. Direct
local Windows desktop testing is still environment-dependent, but automated
Windows GUI smoke now runs on a GitHub-hosted Windows Server 2025 runner. The
smoke builds the Wails Windows executable, launches `archscope.exe`, verifies
that the GUI process stays alive for the startup window, and shuts it down.
Windows confidence now includes tests/builds/packages, release artifact
checksum and PE inspection, plus the GUI launch smoke.

Release hardening now fails fast when macOS Developer ID signing or Apple-ID
notarization secrets are only partially configured. The release workflow also
verifies the built app signature and validates the stapled ticket when
notarization credentials are available. The Wails frontend startup shell is
split from the analyzer pages and shared chart runtime, so the initial bundle
is within the documented release budget.

The MSA timeline discoverability issue is closed: the tab list now renders
before analysis and each tab shows a neutral empty-result state until data is
available. Large MSA timelines are display-capped for browser responsiveness.
Jennifer MSA topology now uses network-time groups to estimate service
distance, while the underlying `AnalysisResult` tables retain detailed call
metrics.

Trace import is now a desktop workflow, not only an engine/CLI MVP. OTLP
JSON-file, Zipkin v2 JSON, Elastic APM `_search` JSON, and Elastic source
NDJSON are covered. The UI exposes summary metrics, service dependencies,
traces, spans, critical paths, parser diagnostics, deterministic findings, and
basic Evidence Board capture. Broader Jaeger, SkyWalking, and SaaS connector
items remain roadmap candidates until they are explicitly promoted into the
active TO-DO.

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

1. Improve JFR handling for async-profiler recordings: keep the JDK `jfr`
   CLI as a binary-to-JSON converter, then add ArchScope-native stack/sample
   views instead of trying to clone the full JDK `jfr` tool UI.
2. Expand the Evidence Board beyond Trace Import: add shared "Add to Evidence"
   actions on other analyzer findings/tables and design report export around
   saved evidence cards.
3. Keep release verification healthy before the next 0.3.x cut by repeating
   Windows GUI smoke, macOS signing/notarization validation, and frontend bundle
   budget checks.

## Active TO-DO

| ID | Priority | Status | Task | Depends on | Output |
|---|---|---|---|---|---|
| T-414 | P1 | [x] | Connect `trace_import` to the Wails UI with summary cards, service dependency view, trace table, span table, and findings panel. | Trace import MVP | Completed 2026-05-13: Trace Import desktop workflow |
| T-415 | P1 | [x] | Add Elastic APM `_search` response and source-only NDJSON importers. | Trace import MVP | Completed 2026-05-13: Elastic trace evidence import |
| T-416 | P1 | [x] | Add trace critical-path analysis and current MVP findings: `SLOW_TRACE_P95`, `CLOCK_SKEW_SUSPECTED`, `UNBALANCED_SERVICE_LATENCY`, and `HIGH_ERROR_SERVICE_EDGE`. | Trace import MVP | Completed 2026-05-13: Root-cause oriented trace diagnostics |
| T-417 | P1 | [x] | Design and build the Evidence Board skeleton around reusable evidence cards. | Analyzer result contracts | Completed 2026-05-13: Cross-analyzer evidence pack foundation |
| T-418 | P1 | [x] | Run direct Windows GUI launch smoke-test for the 0.3.1 line on a Windows host/VM. | 0.3.1 release assets | Completed 2026-05-13: Windows runner GUI process launch smoke |
| T-419 | P2 | [x] | Continue 0.3.x release hardening for signing/notarization and frontend bundle splitting. | 0.3.1 release baseline | Completed 2026-05-13: signing/notarization preflight plus frontend bundle split |
| T-420 | P1 | [ ] | Clarify the JFR analyzer contract: use the JDK `jfr` CLI only for `.jfr` to `jfr print --json` conversion, and avoid reimplementing the full JDK `jfr view` / `jfr summary` feature set in the desktop UI. | Existing JFR parser bridge | JFR scope note and UI copy |
| T-421 | P1 | [ ] | Add async-profiler-oriented JFR stack analysis by aggregating `stackTrace.frames` from sample events into top methods, top packages, top threads, and call-tree/flamegraph-ready rows. | JFR JSON parser, profiler flamegraph components | Useful CPU/wall/alloc sample diagnostics for async-profiler JFR |
| T-422 | P2 | [ ] | Add JFR recording UX hints that detect sparse or mode-specific recordings, especially async-profiler JFR files with mostly `jdk.ExecutionSample`, `jdk.NativeMethodSample`, `jdk.ObjectAllocationSample`, or lock events. | T-421 preferred | Clear empty-state and capture-mode guidance |
| T-423 | P2 | [ ] | Expand Evidence Board capture beyond Trace Import and add report export around saved evidence cards. | T-417 | Evidence-driven report pack workflow |

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
- `npm ci` and `npm run build` passed for the Wails frontend during the
  2026-05-10 release pass. That pass exposed the former main-chunk warning
  that was later addressed by T-419.
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
  frontend and reported the former >500 KB main chunk warning that was later
  addressed by T-419.
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
- 2026-05-11 documentation consolidation check passed: `git diff --check`
  succeeded, and `docs/en` and `docs/ko` Markdown file lists match 1:1 after
  removing the former single-language planning notes.
- 2026-05-13 GC alert verification passed in
  `apps/engine-native/internal/analyzers/gclog`: young/old/metaspace series,
  long-pause flags, and `OUT_OF_MEMORY_ERROR` findings are covered by tests.
- 2026-05-13 trace import verification passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine ./api ./cmd/archscope-app`. The Wails app test build
  still emits the known macOS linker warning.
- 2026-05-13 full Go verification also passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./...`.
- 2026-05-13 Wails frontend release-hardening build passed: `npm run build`.
  Startup shell chunk is 156.77 KB raw / 50.90 KB gzip; the lazy shared
  ECharts runtime is 668.49 KB raw / 221.26 KB gzip and stays under the
  documented 700 KB chart-runtime budget. The previous Vite large-chunk warning
  is no longer emitted.
- 2026-05-13 release workflow hardening updated macOS signing secret preflight
  and codesign/stapler validation. The workflow change was not exercised by a
  local release-tag run. Local YAML parsing and `git diff --check` passed.
- 2026-05-13 CLI smoke passed for Elastic APM auto-detect:
  `go run ./cmd/archscope-engine trace import --in
  ../../examples/traces/sample-elastic-apm-search.json --format auto --top-n
  10` emitted `source_format = elastic-apm-search-json` and the expected
  trace findings.
- 2026-05-13 Windows GUI smoke run `25800619784` passed on Microsoft Windows
  Server 2025. It built `apps/engine-native/cmd/archscope-app/bin/archscope.exe`
  with `task windows:build ARCH=amd64`, confirmed a 14,060,544-byte executable,
  launched it, observed the GUI process alive for 15 seconds, and requested
  graceful shutdown.

## Decisions

- Go/Wails is the primary active runtime and distribution path.
- Wails service bindings are the active engine/UI boundary.
- Local HTTP/FastAPI/browser serving is retired and retained only in `archive/`.
- Python sources are retained for behavior reference and audits only.
- Large-file safety work should happen in Go first; archived Python code should
  not receive new product features.
- Roadmap candidates are not automatically promoted into `Active TO-DO`; they
  should be added only after explicit review and request.

## Archive

- Old work status: `archive/work_status_legacy_2026-05-09.md`
- Archived Python engine: `archive/python-engine`
- Archived browser frontend: `archive/web-frontend-python`
