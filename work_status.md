# ArchScope Work Status

Last updated: 2026-05-10

This file is the current execution status for the active ArchScope product line.
The previous long-form history was archived to
`archive/work_status_legacy_2026-05-09.md`.

## Current Baseline

- Active product: unified Go/Wails desktop implementation under
  `apps/engine-native`.
- Active UI: Wails v3 React frontend under
  `apps/engine-native/cmd/archscope-profiler-app/frontend`.
- Active engine: Go parser/analyzer/exporter/AI interpretation modules under
  `apps/engine-native/internal`.
- Release baseline: `v0.3.0-rc1` has been rebuilt from the Go/Wails baseline
  and published as the latest GitHub release.
- Retired implementation: Python/FastAPI/browser sources are archived under
  `archive/python-engine` and `archive/web-frontend-python`.
- Historical module: `apps/profiler-native` has been folded into
  `apps/engine-native`.

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

## Current Risk

The Electron-to-Wails migration risk is closed. The highest large-file issue
found in the 2026-05-09 audit has been mitigated: GC log analysis no longer
emits chart series for every event, and access-log/OTel analyzer entrypoints no
longer materialize the full parser record slice before aggregation.

Release verification found no new blocker. Direct Windows GUI launch was not
performed in the local macOS environment; Windows confidence currently comes
from GitHub Actions Windows test/build/package success plus release artifact
checksum and PE inspection.

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

1. Commit/push the Go 1.26.3 and version-metadata alignment, then let CI verify
   the updated workflow definitions on `main`.
2. Perform direct Windows GUI launch smoke-test on a Windows host/VM before the
   next non-RC release.
3. Consider deeper GC event streaming if future real-world logs exceed the
   current 305 MB RSS envelope.
4. Add format-specific streaming for remaining structured thread-dump formats
   when real multi-GB samples are available.
5. Continue signing/notarization and frontend bundle-splitting release work.

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
  `go build ./cmd/archscope-engine ./cmd/archscope-profiler
  ./cmd/archscope-profiler-app`, and `go test ./... -race -count=1` passed
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
