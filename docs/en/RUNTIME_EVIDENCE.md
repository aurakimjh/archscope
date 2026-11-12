# Scoped runtime evidence

T-641 uses the common `AnalysisResult` envelope, `runtime_evidence` payload 0.1.0,
and `runtime_core` model 1.0.0. The limited `runtime.evidence-json` importer accepts
normalized JSON. The source assistance UI and `runtime` CLI normalize offline files
first. No collector, arbitrary Python pickle/marshal, heap object graph or online
SQL execution is introduced. Existing result readers remain versioned separately.

## Qualified inputs

| Format | Pinned evidence | Meaning and exclusions |
|---|---|---|
| python-runtime-json | CPython 3.14.4 stdlib API export, python-runtime-export.v1 | asyncio task snapshots, explicit native worker IDs, tracemalloc live bytes/blocks and observed GC intervals. No continuous wait/GIL duration or cumulative allocation rate. |
| node-report-json / node-runtime-json | Node v25.9.0 reportVersion 5 / node-runtime-export.v1 | Reporting thread JS/native stacks, explicit worker reports, libuv handle inventory, perf_hooks event-loop delay and utilization. Open handles do not imply waiting. Runtime worker IDs are not native TIDs. |
| rust-backtrace | rustc 1.94.1 panic/backtrace fixture | Numbered frames and file locations; panic message excluded. No CPU measurement. |
| thread-dump | java_jstack (synthetic JDK 21.0.8 layout), python_pyspy, python_faulthandler | Existing safe text parsers; each format must be explicitly selected. JVM nid is hex native TID; tid/runtime ID is separate. Python heuristics are disclosed separately from observed state. Virtual tasks are not OS threads. |
| ps-l-csv | Synthetic pid,tid,cpu_percent CSV | Explicit process context and measurement basis required. ps lifetime average stays a candidate; interval qualification needs an actual interval-average source. No installed ps command is executed. |
| postgres-explain-json | Synthetic PostgreSQL 18.0 layout, plain/ANALYZE JSON | Estimates separate from actual rows, loops and total ms per loop. Missing actual is null; observed zero remains zero. No SQL, predicates or parent/child time sum. |
| profile-source-set-json | Existing validated ProfileSet | Explicit entity-to-profile mapping with original/model hashes and clock. References only; no copied CPU cost or invented alignment. |
| runtime-evidence-json | Shared model/schema fixture | Strict canonical transport, conflict-refusing merge up to 16 sources. |

Only the bundled pinned fixtures qualify these paths. The Python/Node producers
execute actual standard APIs on a synthetic workload. IDs, paths and clock origins
are sanitized/shifted. PostgreSQL/JVM/CPU fixtures qualify structure and semantics,
not a real database or OS collection. Node versions/report schemas beyond the pinned
fixture, py-spy/faulthandler producer versions, Memray and vendor adapters need their
own captures. Sanitization of technical names/paths is the caller's responsibility.
Raw files can contain secrets: only normalized models should enter cases/indexes.

## Collection context and CPU links

The offline guide and bundled `runtime-evidence-*-options.json` files provide full
examples. Source options are strict JSON, maximum 128 KiB. Declare `format`, producer
version, `clock_id`, process identity and optional entity mappings. A minimal example:

```json
{"format":"ps-l-csv","producer":"manual interval export","producer_version":"1","clock_id":"synthetic-clock","process":{"id":"p1","host_id":"sanitized-host","boot_id":"boot-a","pid_namespace":"ns-a","pid":"700","process_start_ns":"0","runtime":"JVM"},"measurement_basis":"interval_average","from_ns":"0","to_ns":"3000000000","population":"one process"}
```

Raw thread dumps additionally require `thread_format`. Sidecar `entities` can bind
explicit runtime IDs to native IDs, process/thread lifetimes and mapping basis.
Missing fields stay unavailable. The query checks one explicitly selected CPU and
thread observation pair, avoiding an all-pairs join. `qualified_context` requires:
matching host/boot/PID namespace/PID/process start, equivalent native TID encoding,
same known thread start, aligned known source clocks, a containing CPU interval,
covered thread lifetime and an observed/declared mapping. Missing context gives
`candidate`; contradictions give `incompatible`. This verifies declarations, not
source authenticity, causality, GIL contention or CPU attribution to an async task.

Legacy multi-dump payload 0.2.0 joins only records with complete `correlation_context`
(host_id, boot_id, pid_namespace, pid, process_start_ns), runtime/format, runtime
thread ID and explicit thread_start_ns. Otherwise records remain isolated per dump.
Matching names or stacks alone never establish persistence across snapshots. Legacy
thread payload 0.2.0 separates observed state, inferred category and inference basis;
older saved payloads keep their old interpretation.

## Query and reuse

In Evidence, inspect the original source with sanitized context, save a new normalized
model, then import it. Normalize additional sources independently and merge their
models when needed. Select observations/entities, then follow stack/frame IDs to
raw symbol evidence. Counts and exact decimals remain strings across the Go/JS boundary.

For allocations, select exactly one entity and one source; open `live_bytes` or
`live_blocks` as an owned profile. Existing tree, callers/callees, path filters and
visual/export consumers apply. Blocks remain counts, bytes remain bytes. Multiple
capture times are refused; rows describe provided live traced allocations rather
than whole-process retained heap. Closing/resetting or cancelling releases late
owned results. Profile references preserve original identities and do not add costs.

GC pauses use interval union per entity/source/clock/population. Concurrent intervals
are reported separately. Missing clocks/intervals leave coverage unavailable or
incomplete; no pause ratio is manufactured. Existing access/TA/DB analyzers remain
separate; the canonical runtime model can hold explicit latency/pool/lock/database
observations without copying request bodies or inferring database wait from a stack.

```sh
archscope-engine runtime inspect-source --in python.json --options-file python-options.json
archscope-engine runtime normalize-source --in python.json --options-file python-options.json --out model.json
archscope-engine runtime import-source --in model.json --out result.json
archscope-engine runtime query --in result.json --query '{"kind":"observations","limit":25}'
archscope-engine runtime merge-models --inputs model.json,other-model.json --out merged.json
archscope-engine runtime allocation-profile --in result.json --entity-id ENTITY --source-id SOURCE --metric live_bytes --out allocation-result.json
```

Custom pages expose observations, entities/processes/sources, frames/stacks, symbol
basis, relations, profile references and plan nodes. CPU link queries require both
`cpu_observation_id` and `stack_observation_id`. Cursors bind the complete selector
and result digest; stale results/cursors are refused. Common providers, timeline
(only unix clock), CSV/JSON/HTML/report pack and durable cases reuse existing paths.
Only tables are exposed through the existing generic provider pointer contract.

## Bounds and compatibility

Original/expanded source files: 64 MiB; canonical model: 32 MiB; saved result: 64 MiB.
50k observations, 4096 entities, 50k frames/stacks, one million total stack references,
1024 frames per stack, 20k plan nodes, 128 sources, 1024 processes; the byte budget may
be reached first. Custom pages are at most 100 rows (UI 25). Oversized inputs fail
without silent truncation. Two ingestion slots, cancellation checks and a five-minute
source budget apply. Split original sources rather than interpreting partial data.

The service reuses verified immutable model/projection data by digest/version with
a two-entry, two-minute, 96 MiB conservative JSON charge cache. Every request still
acquires ownership and verifies the stored result fingerprint/digest; output DTOs
are copies. CLI commands have independent process lifetimes. No RSS bound or native
Windows/WebView runtime acceptance is claimed by build/static checks.

Rust resolution pins ianlancetaylor/demangle v0.0.0-20261012173340-cbb8aaea4290, with
1024-byte input/output limits. Valid Rust v0/legacy mangling yields encoded language
evidence; .rs/declared language is a separate declared basis. Mere `::` or a substring
`rust` is insufficient. Display resolution never replaces raw symbols, source/build
identity or frame IDs; unresolved symbols remain visible. Dependency code/licenses
ship offline; no symbol server or runtime download is used.


## R18 thread intake and explicit multi-dump context

Actual CPython 3.14.4 faulthandler Thread/Current thread headers with optional
[name] are recognized and names retained. Unrecognized/empty thread input fails
RUNTIME_THREAD_FORMAT_UNRECOGNIZED instead of succeeding with zero snapshots.
Native JVM nid with 0x prefix is hex; without it the decimal OS TID is retained.
Python faulthandler hex runtime IDs and JVM tid pointers are not native TIDs.
Python GC errors now identify /gc_events/N (valid inverted times are included).

CLI `thread-dump analyze-multi` accepts --correlation-context JSON or the bounded
--correlation-context-file. Desktop multi-dump options expose the same JSON field.
Use the bundled runtime-evidence-jdk21-correlation-context.json as a structural
example. Supply schema_version 1.0.0, source_format, host_id, boot_id,
pid_namespace, pid, process_start_ns and threads [{runtime_id,thread_start_ns}].
Starts are canonical uint64 ns in the same declared time domain. A thread cannot
start before its process; unknown/unobserved/duplicate IDs and format mismatches
are refused. This declares identity; it does not authenticate producer lifetimes.
Unmapped snapshots remain isolated. Diagnostics warn how many snapshots/dumps
lack full context and disable persistence findings for those observations; the
multi-dump screen displays the warnings. Names are never a fallback identity.
Actual JDK21 repeated BLOCKED and CPython3.14 fixtures supplement earlier layouts.
