# ArchScope Work Status

Last updated: 2026-05-09

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

## Current Risk

The Electron-to-Wails migration risk is closed. The current technical risk is
large-file parser memory pressure in the Go engine.

The highest priority issue is GC log analysis. In the 2026-05-09 synthetic
audit, a 34 MB GC log produced about 1.08 GB peak RSS and a 105 MB result JSON.
The main cause is unbounded timeline/series emission for every parsed GC event.

Collapsed profiler parsing is the positive baseline: a 29 MB / 500k-line input
completed with about 16 MB peak RSS because it already uses streaming parsing.

## Large-File Audit Snapshot

| Analyzer | Synthetic input | Time | Peak RSS | Output | Status |
|---|---:|---:|---:|---:|---|
| Access log | 30 MB / 300k lines | 0.51s | 173 MB | 10 KB | Needs streaming parser/analyzer follow-up |
| OTel JSONL | 31 MB / 200k lines | 0.25s | 147 MB | 6 KB | Needs bounded trace grouping |
| GC log | 34 MB / 300k lines | 1.51s | 1.08 GB | 105 MB | P0 fix required |
| Collapsed profiler | 29 MB / 500k lines | 0.26s | 16 MB | 6.8 KB | Good baseline |

## Next Execution Queue

1. Fix GC log output/memory growth with series caps and downsampling.
2. Add a common streaming text-line API in `internal/textio`.
3. Convert access log, GC log, OTel JSONL, exception, and simple runtime-stack
   parsers away from full file/line retention.
4. Refactor analyzers so they can aggregate from streams instead of requiring
   full `[]Record` or `[]Event` inputs.
5. Add structured large-file guardrails for JFR JSON, Jennifer exports,
   thread-dump plugins, and profiler SVG/HTML inputs.
6. Document the large-file policy and add reproducible performance gates.

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

## Verification Notes

- `go test` passed for representative parser/analyzer packages:
  access log, GC log, OTel, JFR, Jennifer profile, and thread dump.
- `go test -bench . -benchmem ./internal/profiler` passed and confirmed the
  collapsed profiler path remains memory efficient.
- Synthetic large-file measurements were captured with the current
  `cmd/archscope-engine` binary built from `apps/engine-native`.

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
