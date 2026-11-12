# File-only perf and constrained BPF evidence

T-637 adds `system_evidence` 0.2.0 over the exact SystemEvidence 1.1.0 model.
It preserves raw observations in the common AnalysisResult envelope, with separate
parser, analyzer and generic consumers. Existing folded-stack, ProfileSet,
case/index and visual-artifact contracts remain intact. No process attaches BPF,
changes privileges, starts perf, fetches symbols or downloads a converter.

## Supported slices

| Input | Explicit contract and qualification |
|---|---|
| `perf-script` samples | `perf script --ns -F comm,pid,tid,cpu,time,period,event,ip,sym,dso`. Three synthetic software samples were actually exported by Linux perf **6.1.190** from a generated small `perf.data`, without host event collection. PID/TID, CPU, exact nanosecond time, event, period, root-to-leaf stack, module/address and unresolved symbols are retained. |
| `perf-script` scheduler text | Separate fields `comm,pid,tid,cpu,time,period,event,trace`, explicit `sched_switch`/`wakeup` definitions and declared entity map. The constrained Linux tracepoint text grammar has synthetic tests; native scheduler collection/export is **not qualified** by the software-sample fixture. Other layouts/headers are refused. |
| `events-jsonl` | Explicit `system-evidence.v1` collector contract. Records are sample, sched_out/in, wakeup or a complete duration with declared block-I/O/syscall/network event and phase. This is a bounded adapter interface, not support for arbitrary BPF JSON. |
| `bpftrace-jsonl` | bpftrace **0.22.0** `hist` messages, flat or one keyed map. Shape was checked against upstream pinned hist/lhist fixture contracts; the bundled data is synthetic, and no live BPF attach was run. `min`/`max` are inclusive integer bounds; absent ends are open tails. Zero buckets, exact counts and opaque key text are retained. Other map/printf/stats/syscall/diagnostic shapes are refused. |

The sidecar declares `schema: system-evidence.v1`, capture ID, tool/version,
clock, boot ID, PID namespace, unique entity lifetimes with PID/TID and nullable
process start, loss/unwind counts (null means unknown), kernel-symbol quality,
optional frequency, event definitions and histogram definitions. BPF inputs also
require script SHA-256. A duration event fixes one phase (`queue`, `service`,
`total`, `observed_local`, `unclassified`); each record must agree, so queue and
service populations are not silently mixed. Histograms declare unit, population,
cumulative/delta aggregation, reset identity and optional `[from,to)` window.
Metadata is declared, not publisher authentication or proof of source completeness.

Use the bundled `system-evidence-*capture.json` examples and schema. Original
hashes cover compressed bytes; line/byte coordinates refer to decoded text, and
CRLF byte offsets are retained. Binary `perf.data`, folded stack chronology,
unannounced events, ambiguous lifetime mappings and other native producer layouts
require explicit collection-side conversion/qualification. No arbitrary executable
path or arguments in input are run. Supplied export options are provenance only.

## Measurement and scheduler rules

Samples expose **one record count** and an optional **separate event-specific
period**. Cycles, event count and explicitly declared CPU-clock nanoseconds are
different metrics. Frequency never becomes a time multiplier; no multiplex
correction, IPC ratio or cross-event summation is inferred. `[unknown]` frames
keep addresses, and user/kernel/unknown context remains distinct.

Scheduler records are ordered within one capture/clock/entity lifetime, preserving
original coordinates. sched-in→sched-out is an observed on-CPU endpoint span;
sched-out→sched-in is off-CPU; wakeup→sched-in (or runnable sched-out→sched-in)
is run-queue evidence. Duplicate/equal timestamps and inconsistent/open sequences
remain explicit null spans. Derived numeric spans require declared zero loss,
known clock/boot/namespace/lifetime; on-CPU also requires consistent CPU endpoints.
Unknown or nonzero loss refuses those inferred spans. A complete duration emitted
in one record retains its own measured start/end difference under a known clock,
even when other observations may be lost. No gap interpolation is performed.

Off-CPU does not prove lock contention. Runnable, sleeping and unknown states stay
separate. Block-I/O queue/service phases, syscall waits and observed local network
durations are not renamed remote service latency. Costs across parallel entities
or overlapping observations may exceed walltime. Source observations and null
status rows remain available when a numeric profile excludes unqualified spans.

## Desktop workflow

1. In Workspace, choose a source file and open **Perf / constrained BPF evidence**.
   Choose format, supply the sanitized capture JSON, inspect, then save a new
   normalized model. Analyze with `system.evidence-json` and `{}` options.
2. Generic providers show events, intervals, histogram buckets and an event-only
   timeline. Duration cells carry nanosecond units and row availability/reason.
   Aggregate histogram rows do **not** enter the timeline. Choose the histogram
   provider/event and **Show original bucket distribution** for paged exact counts,
   original bounds/window/reset and a full-retained-maximum bar scale. Retained
   counts are not a complete request population; omitted map keys cannot be recovered.
3. Select one explicit sample/period/qualified-duration metric and all or one
   declared lifetime, then **Create profile in Workspace**. This creates an owned
   immutable `profile_evidence` 0.2.0 result for existing Top/flamegraph,
   caller/callee, evidence/time queries and report images/tables. Unknown-clock
   sample projections are aggregate-only; raw timestamps remain in the parent.
   Comparisons keep existing strict capture equality; no new alias/policy grant.
4. Save a case linked to the original source. Select an event reference on the
   current provider page and **Open verified case original**. It checks current
   case/result/source hashes, reparses the source and verifies that event ID/line
   before opening a bounded text page. Forged coordinates and stale links refuse.
   For a projected profile, open its parent system result for verified originals.
   A gzip source retains decoded coordinates but refuses direct raw-text opening;
   use explicit archive intake first. Source reparse verifies the file link, not
   the producer's collection privileges, script behavior or physical clock accuracy.

JSON preserves saved bytes. Existing generic CSV is summary/availability output;
HTML/Report Pack use retained event/interval/bucket projections. Visual Artifact
exports the selected generic table or existing profile template, not a newly
qualified histogram-image renderer. Native Wails/Windows/Office and live producer
collection remain separate acceptance checks.

## CLI

```sh
archscope-engine system inspect-source --in perf.txt --format perf-script --capture-file perf-capture.json
archscope-engine system normalize-source --in perf.txt --format perf-script --capture-file perf-capture.json --out model.json
archscope-engine evidence import --in model.json --importer system.evidence-json --options '{}' --out result.json
archscope-engine system project-profile --in result.json --metric-id 'period/cpu-clock:HG' --out profile.json
archscope-engine profile-set tree --in profile.json
archscope-engine evidence export --in result.json --export-format report-pack --out report.zip
```

`system import-source` combines normalization and result construction.
`--entity-id` selects declared lifetimes for a projection. All new files refuse
existing destinations and managed paths. Snapshot readers and unsupported inputs
remain explicit, never silently migrated or truncated.

Original/expanded inputs cap at 64 MiB, lines at 1 MiB, models at 16 MiB,
records at 50,000, entity lifetimes at 1,024, stack depth at 1,024, references at
1 million, and histogram buckets at 4,096 per record/100,000 total. Result/case
objects remain 64 MiB. Source normalization admits two calls with five-minute
limits; service operations use common queue/cancellation policy. Canonical models
travel in ≤256 KiB chunks. Synthetic 10k/20k sample results measured about
10.5/21.1 MB; deep stacks/labels can hit byte limits sooner. No admission guarantee
is inferred solely from record count. Package evidence is in
`docs/review/evidence/2026-11-12-t637/`.

Primary contracts: [Linux perf fields](https://raw.githubusercontent.com/torvalds/linux/v6.12/tools/perf/Documentation/perf-script.txt),
[bpftrace 0.22 JSON output](https://bpftrace.org/docs/0.22),
[pinned histogram fixture](https://github.com/bpftrace/bpftrace/blob/v0.22.0/tests/runtime/outputs/hist.json).


## R15 storage and scheduler qualifications

SystemEvidence 1.1.0 stores shared frame and stack tables; records carry bounded
integer `stack_id` references. Result payload 0.2.0 uses this compact storage.
Old 1.0.0 models and saved 0.1.0 result readers remain supported without rewriting
historical bytes. At depth 25, synthetic 20k/24k samples stored compact models of
5,913,811/7,101,811 bytes and results of 17,933,051/21,533,051 bytes in local checks.
Unique/deeper stacks and labels still consume limits. Budget errors disclose
charged bytes, limit, processed records and splitting via `perf script --time`.
Keyed histogram parsing has a separate fresh map and stable malformed-shape code;
its synthetic shape is checked against the pinned 0.22.0 formatter source.

Capture `original_sha256` is declared pre-conversion provenance (for example the
binary perf.data hash); it may differ from the actual imported text/gzip
`source_sha256`. It is not an enforced text-source hash assertion. Waker and idle
entities must currently be declared; system-wide idle TID 0 can produce ambiguous
CPU-sharing rows. An explicit idle/ignore policy remains queued. Scheduler trace
layout has no stacks, so its profile cost is `[unattributed]`; only stack-bearing
event input supports off-CPU call structure. Current off_cpu combines sleeping
and runnable costs, preserves state labels and can overlap runqueue. State-filter
projection is a queued follow-up.

Pinned keyed formatter: [bpftrace 0.22 output.cpp](https://github.com/bpftrace/bpftrace/blob/v0.22.0/src/output.cpp).
