# Offline CPU/GPU execution intervals

GPU execution evidence adds a separate `trace.gpu-execution-json` importer (alias `gpu-execution`)
to the existing `trace_import` AnalysisResult, payload 0.4.0 / gpu_execution_core
1.0.0. Existing trace 0.2.0 and LLM trace 0.3.0 remain unchanged. Parsing,
interval analysis, SVG export and React presentation have separate owners.
Device telemetry remains in MetricHistory; this adapter handles execution traces.

## Actual qualification and limits

| Source format | Actual evidence | Supported meaning |
|---|---|---|
| pytorch-trace-json | PyTorch2.15.0 / Python3.13.13, two local CPU/Gloo ranks, matrix multiplication/all_reduce/relu | CPU operators, user annotations with hashed names, explicit Gloo communication observations, rank/world/backend and producer flow records |
| nsight-sqlite | NVIDIA training report at pinned commit b17c40eef23929d08ec42e66667b29ae336ea8ac; original LFS SHA-256 verified; officially exported with Nsight2024.2.3.38, schema3.11.1 | CUDA runtime API, GPU kernels, memcpy and closed NVTX intervals, serialized process IDs, explicit local-device mapping |
| nsight-summary-csv | Actual official cuda_gpu_kern_sum reporter from that export | Reported counts/durations/shares only; no intervals, stream overlap, launch links or detailed timeline |

The local synthetic two-rank run is not a distributed LLM workload, CUDA/NCCL
qualification, production workload or a cross-host synchronization experiment.
The public NVIDIA report is actual remote GPU evidence, not GPU execution on
this Mac. PyTorch CUDA shapes have structural tests only; exported GPU pid/device
fields are not silently interpreted as CPU process identity. Other versions and
layouts require fixtures, not false version declarations.

The raw NVIDIA report was exported in a development-only network-none container.
A 32KiB reusable SQLite fixture retains real table definitions/rowids and actual
[80,80.5)ms-starting rows, referenced strings and export-version metadata, excluding
environment/session/command-line/GPU-UUID tables. The full 41,431,040-byte export
was also parsed with an explicit interval window, including a CPU API beginning
before the window. Original/export hashes and sanitization are in the bundled
producer-provenance file. The summary CSV describes the full run, not this subset.

## Manual collection and source scope

Use the official tools separately; import never executes them:

```sh
nsys export --type sqlite --output trace.sqlite trace.nsys-rep
nsys stats --report cuda_gpu_kern_sum --format csv --output summary trace.sqlite
archscope-engine gpu-trace inspect-source --in trace.sqlite --options-file capture.json
archscope-engine gpu-trace normalize-source --in trace.sqlite --options-file capture.json --out model.json
archscope-engine evidence import --in model.json --importer trace.gpu-execution-json --options '{}' --out result.json
```

Example capture.json for the qualified SQLite export:

```json
{"format":"nsight-sqlite","producer_version":"2024.2.3.38","capture_id":"local-capture","host_id":"local-host-tag","boot_id":"unique-boot-tag","process_namespace":"nsys-packed","run_id":"local-run","clock_id":"local-export-clock","device_map":{"0":"local-gpu"},"from_ns":"80000000","to_ns":"80500000"}
```

For PyTorch, use format `pytorch-trace-json` / version `2.15.0`, the same explicit
capture/host/boot/process/run/clock fields, and optional time bounds. Rank/world/
backend come from actual distributedInfo, not names. Export CPU profiling with
`torch.profiler.profile(activities=[ProfilerActivity.CPU])` and
`prof.export_chrome_trace(...)`. Source-side raw host/path/args are excluded from
the normalized result; arbitrary annotation names are hashed. Technical operator,
kernel and caller-declared identity strings still need sanitization before sharing.
No tensor values, model weights or GPU memory dump is required or retained.

Select **CPU/GPU execution** in Workspace source assistance, paste the capture
JSON, inspect the original, save a new normalized model and Analyze. Inspection
SHA-256 must still match at normalization. Existing files are refused. A generic
canonical model import accepts only `{}` options. Use `gpu-trace import-source`
for direct CLI result creation and `gpu-trace merge-models --inputs a.json,b.json`
for bounded multi-source models; duplicate source identities are refused.

## Exact interval and identity rules

- PyTorch ts/dur are exact microseconds converted to integer nanoseconds;
  displayTimeUnit is display only. Nonintegral nanoseconds/overflow are refused.
  Nsight start/end are exact nanoseconds. Values above 2^53 never cross JavaScript
  as measurement numbers. Layout positions are presentation ratios only.
- Source hash/producer/schema/capture/host/boot/process namespace/run/rank/clock,
  device_map, track process/thread/device/context/stream and source positions are
  retained. globalPid/globalTid are serialized Nsight identities; the low thread
  component is removed only for process-scoped CUDA correlation, never labeled
  an authenticated OS PID or process lifetime.
- CUDA links require one runtime API with matching source, serialized process and
  correlation ID. Multiple candidates remain ambiguous; missing candidates stay
  unresolved. One launch may link multiple GPU records. Kernel names never prove
  NCCL, rank, request, prefill/decode or workload attribution.
- Source-scoped flow endpoints preserve explicit IDs. id2 global IDs can cross
  processes within that source; legacy/local IDs remain process-scoped. Orphan,
  repeated or reversed endpoints are disclosed. Endpoint links do not invent
  CPU/kernel event attribution or causal critical paths.
- CPU self is inclusive duration minus the observed same-track direct child
  interval union. This is timing containment, not a supplied call graph. Crossing
  or equal positive intervals make self unavailable for that track. CPU and GPU
  costs are never added. Annotations and communication observations stay separate.
- Per source/device: kernel duration sum, union across streams and kernel event
  extent are different values. Extent is bounded by observed kernels, not a
  complete recording window or device utilization denominator. Memcpy is separate.
- Time filters select intersecting whole intervals in `[from_ns,to_ns)`; zero
  durations are points. No clipping, interpolation or rescaling. A time query
  requires one source clock. Optional clock_group/clock_offset_ns (signed exact
  integer)/clock_uncertainty_ns metadata must be complete and is retained for
  audit; this release still refuses multi-source timeline alignment. No inferred
  NTP offset, cross-rank straggler verdict or critical path.

## Queries, export and saved cases

```sh
archscope-engine gpu-trace query --in result.json --query '{"kind":"events","source_id":"copied-source-id","limit":25}'
archscope-engine gpu-trace query --in result.json --query '{"kind":"busy","limit":25}'
archscope-engine gpu-trace export-svg --in result.json --query '{"kind":"events","source_id":"copied-source-id","limit":25}' --out selected.svg
```

Query kinds: events/tracks/sources/busy/relations/summaries. Source/rank filters
remain explicit. Cursors bind the exact result digest and all selectors. The UI
preview shows returned page rows; the SVG includes **all** selected events,
refusing more than2000 events/64 tracks. SVG keeps exact times, query, only the
selected source provenance and result SHA-256; labels are escaped, no remote
resources are used. Summary-only files cannot export an execution timeline.
JSON/CSV/HTML/report-pack, generic exact tables and durable case snapshots reuse
existing consumers. A saved snapshot is not original-source reimport verification.

Original/expanded input ≤64MiB, canonical JSON model ≤32MiB, model ≤50k events,
4096 tracks/16 sources/4096 summary rows; result ≤64MiB. Earlier byte/record limits
apply. The64MiB raw/expanded byte cap is checked BEFORE selection. A window only
reduces retained event/track counts; it cannot admit an oversized SQLite/PyTorch
file. There is no silent first-N truncation. Parsing has 2 admission slots, a 5-minute deadline and cancellation.
SQLite runs from an immutable read-only private copy, query_only/trusted_schema
restrictions, fixed known table queries and bounded rows. Required metadata/name
tables must be physical; generated columns in selected physical tables and unknown
schema versions are refused. Optional-table presence/absence is disclosed; selected-name views and virtual tables are refused. An empty kernel result is not proof of no GPU activity. Ordinary SQLite files,
.nsys-rep binaries, arbitrary SQL and installed tools are not executed as input.

Evidence: actual producer fixtures and independent integer interval calculations,
source/cursor/ownership/tamper tests, full race suite, common consumers, packaged
resources and disconnected CLI checks. Native Windows/WebView, native PyTorch
CUDA/NCCL, other export versions and hardware tuning remain separate gates.

Primary references: [PyTorch profiler](https://docs.pytorch.org/docs/2.15/profiler.html),
[distributed package](https://docs.pytorch.org/docs/2.15/distributed.html),
[Nsight export/schema](https://docs.nvidia.com/nsight-systems/AnalysisGuide/index.html#sqlite-schema-reference),
[NVIDIA training report](https://github.com/NVIDIA/nsight-training/tree/b17c40eef23929d08ec42e66667b29ae336ea8ac/cuda/2021_gtc/x-ac-03-v1/task1/task/nsys/precollected_reports).

The Nsight adapter selects runtime API/kernel/memcpy/closed NVTX tables only; OS runtime, driver API, memset, marker and hardware-counter tables are outside this slice. Memcpy supplies intervals, not byte/direction-derived bandwidth.


<a id="large-original-capture-workflow-r21-01"></a>

## Large original capture workflow

For a standalone qualified Nsight SQLite larger than64MiB, run the explicit
manual helper before import (Python standard-library sqlite3 only):

```sh
python3 scripts/subset-nsight-sqlite.py --in full.sqlite --from-ns 80000000 --to-ns 80500000 --out selected.sqlite
archscope-engine gpu-trace inspect-source --in selected.sqlite --options-file capture.json
```

The helper bounds original files at2GiB/300s, SQLite cache4MiB, selected whole
intersecting intervals50k and output64MiB. It preserves original rowids and only
supported integer columns, three version metadata entries and referenced technical
names. Source DDL, environment/session/command line and NVTX text are excluded.
Views/virtual/generated columns in selected tables, sidecars, unknown versions,
changed source and existing output/receipt paths refuse. It emits an adjacent
receipt with original/subset hashes, window and per-table retained counts. Review
technical names before sharing. The imported source SHA identifies the new subset,
not the large original; keep the receipt for that relationship. The app never runs
this helper or bypasses its immutable64MiB intake. Above2GiB, acquire a shorter
producer capture before preprocessing; nsys export is not assumed to provide a
window option. Python/tool installation is a separate manual prerequisite.

For PyTorch, use profiler schedule/activities to capture a shorter active phase
and disable unnecessary stacks/record_shapes when appropriate. For example,
`torch.profiler.schedule(wait=1,warmup=1,active=2,repeat=1)` with `prof.step()` and
`on_trace_ready` exports each bounded active phase separately. Recheck each file's
actual size; schedule does not guarantee a byte ceiling. Within64MiB, ignored
metadata/python_function records do not consume the50k retained supported-event
budget; the window is applied before that event budget. Supported events still
preserve all whole intervals and refuse at the first overflow (`at_least`). Table-presence disclosure and structural flow qualification are described below.

The disconnected release bundle also ships the same manual helper at `resources/tools/subset-nsight-sqlite.py`; invoke it explicitly with Python.

Nsight import records present/absent status for each supported runtime, kernel, memcpy and NVTX table. An absent table means unavailable coverage, never proven zero activity. Any selected table name backed by a view or virtual table is refused before reading it; generated columns are also refused. Historical saved models retain their original warnings.

Additional structural regression fixtures distinguish overlapping streams (kernel sum 77,888 ns, union 51,968 ns), duplicate launch candidates and ambiguous/orphan/valid flow endpoints. They are deterministic mutations of an actual sanitized SQLite export and synthetic Chrome flow records, not new CUDA/NCCL runtime qualification.

Qualified versions remain the defaults. For explicit structural-only opt-in, actual/unknown version preservation and qualification limits, see [Unverified producer versions](INPUT_ASSISTANCE.md#explicit-unverified-producer-versions).
