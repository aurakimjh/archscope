# Local Diagnostics Evolution Design

Status: overall design proposal revised after the 2026-11-11 review. T-628 foundation
is implemented as recorded below; this is not completion or release of the full plan.

This English companion records the same scope, architecture decisions, compatibility
rules, and implementation dependencies as the [Korean design](../ko/LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md).
The Korean document contains the expanded input catalog and investigation examples.
The [work board](../../work_status.md) records review dispositions and decision holds.

## 1 Purpose and scope

ArchScope helps application architects investigate incidents and performance locally.
TA and DBA workflows share its evidence, timeline, and comparison facilities. The
core workflow is import files, inspect measurement quality, narrow a time/entity
selection, drill into source evidence, compare compatible observations, and copy
charts or tables into a technical report.

Reporting improvements cover PNG copying, SVG saving, and HTML/TSV table copying.
PPT authoring, slide editing, and PPTX enhancement are excluded. Heap-dump object
graphs, dominators, retained-size engines, automatic production tuning, and a new
universal live collector are also excluded. Existing exporters remain compatible.

## 2 Current implementation and accuracy gaps

Current components include the source-format registry, runtime thread plugins,
profile import and legacy profiler paths, trace import, metrics snapshots, analysis
workspaces, and chart saving. These do not yet form the proposed unified pipeline.

Code inspection identified profile paths that coerce weights to integers, apply
minimum-one behavior to some inputs, count event closures instead of duration,
or omit later evented profiles. The existing legacy stack map cannot retain all
measurement dimensions. Correctness work must precede additional producer claims.
Current metadata versions vary by result family and path; there is no global
payload version shared by every result.

## 3 Cases, capture context, and lifecycle

A case groups symptoms, source manifests, result revisions, entity/clock mappings,
selections, baselines, annotations, and visual artifacts. Original files are
read-only references by default. Replaced sources invalidate dependent detail
indexes and mark derived views stale without rewriting historical snapshots.

A versioned capture sidecar records file hash/size, producer/version/options,
host/process/device identity, collection start/end, timezone and clock domain,
sampling/loss information, and redaction provenance. Distinguish observed,
user-supplied, and inferred metadata. Never use file mtime as collection time.

Batch ZIP/TAR import validates entries before extraction: traversal, links, depth,
entry count, expanded size, and decompression budget. Group using explicit identity;
retain file-level failures and partial coverage. Duplicate bytes may have distinct
provenance. Support UTF-8, UTF-16, CP949, and EUC-KR through separate fixtures and
preserve original byte/line positions after decoding.

Cases use active, archived, restored, and deleted lifecycle states. Store data in
the OS application-data area with explicit retention and cleanup scopes. Do not
delete cases automatically by default. Case deletion covers managed source copies,
indexes, caches, and artifacts; referenced external originals require a separate
explicit action. Logical deletion does not guarantee physical SSD erasure.
Reusable baselines pin result revisions and comparison conditions.

## 4 Profile breadth and semantics

Prioritize py-spy raw/folded, dump, and Speedscope as distinct capabilities. Extend
through pprof, async-profiler/JFR exports, perf script, constrained BPF outputs,
V8 CPU profiles, and validated Python/.NET/native producer exports. Producer support
requires exact format/version fixtures; parsing a generic container is insufficient.

The internal ProfileSet contains stable frame/stack IDs, multiple measurements,
exact weights and units, timestamp/clock context, process/thread/rank identity,
quality, and source locations. Sample count, CPU time, wall time, duration, bytes,
allocation count, and signed differences remain distinct. Fractional values and
known zero must survive. Unknown intervals and durations remain unavailable.

Speedscope sampled and evented profiles need separate semantics. Evented stacks
produce observed durations; multi-profile files retain all profiles and identities.
An aggregate folded stack has no event chronology. Do not invent a request-level
timeline from an aggregate profile.

Comparisons validate metric, unit, capture conditions, population, and normalization.
Display absolute change and share change separately. Local symbol files and source
maps are optional; unresolved frames and symbol quality remain visible.

## 5 perf, BPF, and conversion

T-637 implementation/qualification is recorded in [System evidence](SYSTEM_EVIDENCE.md).

Separate stack samples, scheduler/on-off-CPU events, and aggregate histograms.
Capture event type, period/frequency, lost records, kernel/user context, CPU and
process identity. Off-CPU time is not automatically lock contention. An aggregated
histogram is not a request event list.

Prefer conversion in the collection environment using the compatible perf, JDK,
Nsight, or .NET tooling. Optional local conversion uses an explicitly configured
installed executable, bounded execution, cancellation, sanitized arguments, and
recorded version/options/hash. Do not auto-download tools. Existing binary JFR
loading invokes `jfr print --json`; a native Go binary decoder is deferred.

## 6 Drilldown and evidence

Flamegraphs share metric selection, search, focus, caller/callee, top stacks,
inclusive/self values, comparison, source evidence, and quality disclosure. Frame
identity is independent of display labels. All cards, trees, tables, and exported
images use the same selection revision and denominator.

Detail queries are bounded, cancellable, paged, and tied to result revision/query
hash. Stale cursors and late responses cannot replace the current selection.
Node/display budgets do not silently change aggregate values. Case-wide search
reuses the detail index for IDs, time windows, fields, and sanitized text, returning
source locations and explicit indexing coverage.

## 7 Runtime stacks

JVM analysis distinguishes observed lock relationships, thread states, and CPU
evidence. Map top-H/ps-L TID to JVM nid only after checking host, namespace, process
lifetime, collection interval, and numeric representation. RUNNABLE is not proof
of CPU consumption; snapshot stacks do not cover an entire CPU measurement interval.

Python adds asyncio task exports, ASGI/WSGI and queue-worker context, and allocation
summaries from validated memray/tracemalloc exports. CPU sampling cannot enumerate
all coroutine waits. Node/TypeScript connects V8, reports, worker/event-loop metrics,
and local source maps. Go and .NET preserve their runtime-specific task/thread scope.

Rust keeps raw and demangled symbols and does not identify the runtime solely from
`::` or strip hashes into collisions. Use producer/binary evidence and confidence.
Panic/backtrace text is error evidence; async poll/wake relationships require actual
events. Resolver fixtures follow the [rustc symbol documentation](https://doc.rust-lang.org/rustc/symbol-mangling/index.html).

## 8 GC and memory pressure

Maintain collector/JDK/log-option matrices and equivalent per-runtime contracts.
STW interval union differs from concurrent-cycle duration. Allocation rate requires
appropriate observations, rather than a heap before/after subtraction. Preserve
restart, rotation, missing events, and observation-window limits. Distinguish heap,
native, container, and GPU OOM. Allocation profiles and text histograms are in scope;
object-reference heap-dump engines are not.

## 9 Access, database, system, and workload evidence

Access analysis preserves latency availability, endpoint populations, streaming
failures, retries, upstream/client observation boundaries, and rotation/encoding.
Do not average percentiles or add overlapping durations. Database analysis connects
slow queries, plans, pool waits, and observed execution statistics without executing
queries against a database.

TA candidates include sar/pidstat/iostat/vmstat, PSI/cgroup, kernel/Kubernetes events,
and resource exports. Workload exports from JMeter, nGrinder, k6, Gatling,
GenAI-Perf, and llmperf become comparison evidence only after fixture validation.
Preserve arrival model, concurrency, warmup, client errors, and token distributions.
Text hs_err_pid analysis is a bounded follow-on; it is not a core-dump engine.
Commercial DB reports, DB snapshots, Windows exports, network counters, and APM
exports remain demand/fixture-dependent candidates.

## 10 Metric history and OTLP GenAI metrics

Keep `metrics_snapshot` compatible and introduce `metrics_timeseries`. Accept
timestamped snapshot sequences, range-query exports with query provenance, and
validated OTLP metric files. A single untimed snapshot cannot produce rates or
time-window percentiles. Retain counter resets, gaps, histogram schemas, and scope.

OTLP metrics are first-class inputs independent of trace availability. Preserve
resource, instrumentation scope/schema URL, attributes, type, unit, start/end time,
and temporality. Difference cumulative sums within the same lifetime; do not
difference delta sums again. Validate gaps, overlap, order, and restarts for both.
Non-monotonic sums do not use counter-reset rules.

Explicit and exponential histograms have separate decoders and aggregation rules.
Preserve exponential scale/offset, positive/negative buckets, and the zero region;
do not treat them as Prometheus native histograms. Deduplicate only with supported
export provenance, identity, interval, and point evidence. Never sum OTLP and
Prometheus copies of the same instrumentation automatically. Follow the
[OTel metric model](https://opentelemetry.io/docs/specs/otel/metrics/data-model/).

GenAI aliases require instrumentation/version fixtures. Client/server populations
remain separate; exemplars and trace IDs provide optional explicit links.

## 11 GPU evidence

T-639 implementation and its explicit scope/producer gates: [Offline GPU and inference serving evidence](INFERENCE_EVIDENCE.md).

Start with NVIDIA telemetry/errors, then PyTorch/Nsight execution evidence. Device,
host, process, container, rank, and MIG identities require explicit mapping. Keep
device utilization, memory capacity/usage, framework allocated/reserved memory,
and engine KV cache distinct. pmon/dmon, CUDA OOM text, framework summaries, and
Kubernetes allocations are candidates requiring producer/driver fixtures.

Apple Silicon is in product scope. Its shared-memory semantics and host/device
identity enter the common model early; NVIDIA UUID/MIG/Xid and dedicated-VRAM
assumptions are not universal fields. Correlate available GPU observations with
system memory pressure/swap and thermal/power evidence without inventing metrics.
See [Apple's memory model](https://developer.apple.com/documentation/metal/choosing-a-resource-storage-mode-for-apple-gpus).
The actual Mac engine, macOS, collection method, and export remain decision holds.
MLX/MPS/Metal/Ollama are candidates, not verified adapters.

GPU utilization is not proof of useful work. Overlapping kernels cannot be summed
as elapsed time. Nsight exports require versioned schema inspection; summary CSV
supports summary analysis only. Hardware instruction tuning remains external.

## 12 Serving and LLM application traces

Use an engine-neutral `inference_serving` semantic layer with vLLM first. Declare
each signal's unit, scope, boundary, source, and statistical form: gauge, counter,
histogram, request event, or unavailable. Queue/running, TTFT, ITL/TPOT, throughput,
cache, and phase observations are comparable only with matching semantics.
Store original metrics once and reference them from serving projections.

Client first-token time, gateway observations, and server TTFT remain distinct.
Network chunks are not necessarily tokens. Request TPOT requires at least two
output tokens and observed first/last-token boundaries. Do not add phase p95s into
end-to-end p95. Keep model/revision, quantization, input/output lengths, parallelism,
batch/cache conditions, and warmup in comparison context.

Langfuse/LangSmith/OTel GenAI/OpenInference exports map into a common LLM execution
layer on `trace_import`: LLM, embedding, retrieval, reranking, tool, chain/agent,
model, token accounting, timing, errors, attempts, parent and link identities.
OTLP sources reuse the existing trace parser with semantic adaptation. Product JSON
adapters wait for actual export shapes. Preserve orphan/missing/duplicate spans and
avoid summing nested durations. Prompt/completion/tool bodies are excluded by
default and sanitized before managed copies/indexes when explicitly selected.

Model load, warmup, replacement, scale-from-zero, and recovery define separate
populations. Show exclusions rather than silently removing requests. Gateway logs
map routing, rate limits, retry/fallback, tokens, and logical requests versus attempts.
Other serving engines remain fixture-driven extensions; no online API is required.

## 13 Correlation and clocks

Distinguish explicit ID links, verified entity/time mappings, and temporal
coincidence. Preserve wall/monotonic/device clocks, offsets, uncertainty, and user
adjustments. A common timeline aids investigation but does not prove causality.
Do not interpolate low-resolution telemetry into exact request or kernel timing.

## 14 Visual artifacts and renderer boundary

Implementation T-636 and its qualified template/CLI/native acceptance scope are documented in [Pinned report images and tables](VISUAL_ARTIFACTS.md).

A visual spec pins result/hash, selection/query, metric/unit/denominator, annotations,
redaction, and template/renderer/font versions. Generate high-resolution PNG,
SVG files, and tables with readable labels, source, time range, quality, and scope.
Actual PowerPoint paste tests on supported Windows/macOS versions are required.

The first renderer is a separate desktop WebView render surface using versioned
components, independent of viewport/scroll/hover. `SaveChartArtifact` continues to
save already-rendered bytes. `RenderVisual` requires a desktop renderer capability,
font readiness, timeout, and cancellation.

CLI supports normalized JSON/CSV, spec validation/storage, and existing static HTML.
New PNG/SVG rendering and clipboard output are initially unsupported in CLI.
A headless worker is a separate future decision; bundled Chromium is not mandatory.
Existing exporters consume `AnalysisResult`; analyzer/query-produced immutable
views supply selections. HTML/report packs may embed previously rendered artifacts,
specs, captions, hashes, and references without reparsing logs or recalculating data.

Saved image bytes remain historical snapshots. Exact-version regeneration requires
all pinned inputs and rendering dependencies; otherwise display the snapshot or
create an explicitly upgraded revision. Do not overwrite the old spec. Numeric and
layout reproducibility differ from byte identity across platform rasterizers.

## 15 Input assistance and optional AI

Explain detected candidates, limits, missing capture context, and local conversion
steps. Declarative field previews show source-to-normalized values before import.
Optional local AI may explain bounded evidence and propose investigation steps,
but cannot invent measurements or replace deterministic findings. Preserve model,
prompt-template, evidence, and redaction provenance. No external model call is
required for core analysis or export.

## 16 Extension and compatibility contracts

The T-628 pre-implementation specification is in [Parser Design](./PARSER_DESIGN.md#t-628-detailed-design-scope-and-baseline)
and [Data Model](./DATA_MODEL.md#t-628-detailed-measurement-and-compatibility-contract).
Those sections detail registration, selection, jobs, precision, compatibility and
test handoff. Authoring them does not complete T-628 or release its dependencies.

### 16.1 Ownership and registration

`internal/ingestion` owns descriptors/probes/records; `internal/catalog` explicitly
assembles compiled importers and analyzers; `internal/profilemodel` owns the common
profile model; parser/analyzer families decode and analyze separately;
`internal/analysis` orchestrates jobs and queries. These new locations are proposals.
Existing thread plugins are wrapped without lifting mixed-runtime restrictions.

An ImporterDescriptor declares stable ID/aliases/family, producer and verified
versions, adapter version, probe budgets, input/option schemas, record family,
measurements/capabilities, limitations/converter requirements, fixture manifest/hash,
conformance version, and compiled parser/adapter/analyzer handles. Duplicate IDs
fail. Ambiguous detection presents candidates; registry ordering is deterministic.

The flow is Probe → Detect → Validate → Parse to bounded sink → Normalize → Analyze.
Progress, cancellation, backpressure, resource budgets, and partial status propagate.
Package + fixtures + one registration is the acceptance goal **only when existing
semantic families and renderers suffice**. A new semantic model or visualization
still requires explicit model/provider/renderer work.

### 16.2 One profile core and legacy migration

New profile paths use ProfileSet → profile analyzer → common `AnalysisResult` with
`profile_evidence`. Legacy profiler DTOs/endpoints remain compatibility facades.
Migration order: freeze legacy goldens; add the model/metadata wrappers; migrate
tree calculations; migrate diff/drilldown/export; remove redundant dispatch paths.

LegacyStackProjection may aggregate dimensions with disclosed losses, but numeric
conversion must be exact at a declared scale and fit the target integer range.
Reject rounding, minimum-one substitution, unsupported signed values, identity
collisions, and overflow. Operations unable to honor scale use the new path.
Do not force new results through the old typed DTO or introduce another envelope.

### 16.3 Metadata and semantic versions

Keep existing scalar summary values. `metadata.measurement` maps JSON Pointer paths
to descriptors for kind, unit, scope, aggregation, availability, and evidence.
Collection descriptors identify allowed columns; per-row availability is explicit.
Do not relabel total_samples as duration; introduce total_weight and preserve count.

Proposed profile_evidence 0.2.0 covers corrected fractional/zero weights, evented
duration, multi-profile population, and pprof multi-measurement semantics. Reanalyze
raw sources into a new revision; do not relabel old values or silently compare
incompatible semantic revisions. The proposed version is not shipped by this edit.

Nullable targets are unknown sample count, estimated/elapsed/duration seconds,
seconds-per-weight, and sampling interval in the new contract. Null is not global:
only schema-declared fields allow it. UI shows unavailable with a reason; charts
show gaps; CSV carries blank plus availability; diff propagates unavailable.
Known zero remains zero. Unsupported schemas cannot feed numeric computations.

Legacy profiler metadata needs unknown-key preservation through typed-key precedence
and `map[string]json.RawMessage` marshal/unmarshal tests. Existing binaries cannot
be assumed to preserve new fields. The common path preserves extension blocks.

| Version axis | Owner and mismatch behavior |
|---|---|
| Envelope | models; proposed metadata.envelope_schema_version; absent means legacy; unsupported range blocks execution/resave |
| Result payload/semantics | analyzer metadata.schema_version; type-specific reader ranges; incompatible computations/diffs blocked |
| Extension block | metadata.extensions; unknown optional blocks preserved and disabled; required unsupported blocks block operations |
| Importer/adapter | descriptor; retain snapshots and create new revisions on reanalysis |
| Knowledge pack | manifest ID/version/hash; new jobs only, no retroactive result changes |
| Detail index | index header/indexer; rebuild derived cache; no original means detail unavailable |
| Case manifest | case_format_version; unsupported major rejected, compatible optional fields retained |
| Portable bundle | bundle_format_version; validate before extraction, preserve unsupported original |
| Capture sidecar | capture_schema_version; preserve unknown data, limit uncertain clock/entity capabilities |
| Visual spec | spec schema/version; supported migration creates a copy |
| Template/renderer/font | pinned IDs/versions/hashes; old snapshot or explicit new revision if dependencies unavailable |

### 16.4 Results and consumers

GPU telemetry is a semantic view over `metrics_timeseries`, not a duplicate
`gpu_metrics` store. Detailed GPU execution may require `gpu_profile_evidence`;
serving uses `inference_serving`; LLM application semantics extend `trace_import`.
A dedicated screen alone does not justify a new canonical result type.

ResultDescriptor registers payload readers, generic projections, timeline/SLI,
comparison, and detail providers. Capabilities declare ID/version/availability/reason,
payload path and schema. Validate payloads before use; imported strings are not
trusted capabilities. Generic import/analyze services wrap existing endpoints.
Consumers enumerate providers instead of adding per-format switches. A bounded
generic summary/table/diagnostic renderer precedes new domain types.

## 17 Offline extension, storage, and distribution

Evaluate new inputs in this order: declarative mapping, verified external conversion,
then compiled importer. Declarative packs use a constrained schema/interpreter;
they cannot execute scripts, arbitrary SQL, shell, or template JavaScript.

Offline packs are data-only archives with manifest IDs/versions, compatible reader
ranges, dependencies, licenses, file hashes, and fixtures. Detached Ed25519 signatures
cover exact manifest bytes and use application/admin-trusted keys; hashes alone do
not authenticate. Offline revocation metadata is versioned. Validate in staging,
run conformance, activate atomically, pin running jobs, and allow explicit rollback.
Unsigned local presets have explicit local provenance and the same bounded parser.
Manual collection scripts are a separate artifact from data-only knowledge packs.

Package documentation, schemas, verified fixtures, chart assets and licensed pinned
CJK fonts for offline use. Windows WebView2 deployment must provide a supported
offline/fixed-runtime path. No updater, telemetry, symbols, fonts, or documentation
may trigger outbound requests in offline mode. Verify attempts, not only failures.

Use case manifests/result JSON plus a rebuildable detail index and bounded chunks.
SQLite is a candidate, not an existing dependency claim. Prototype a pure-Go option
first; adopting cgo requires an explicit OS/architecture packaging decision.
Release CI requires a pinned conformance bundle and fails when absent, independently
of sibling-workspace availability. Runtime must not depend on sibling paths.

Reusable sanitized fixtures, mappings, collection recipes, and rules belong in
`projects-assets` with ASSET_INDEX updates; ArchScope-specific implementation stays
here. This revision references assets without creating new shared executable rules.
Untrusted HTML/SVG, archives, SQLite, and exported identifiers retain privacy and
resource limits across cache, UI, AI, and clipboard paths.

## 18 Acceptance evidence

Required implementation checks include legacy/new profile goldens; exact/unsupported
projection; registry ambiguity/cancellation; unknown metadata roundtrips; reader
version gates; archive/encoding/lifecycle/baseline cases; OTLP temporality/histograms/
duplicates; GPU vendor identity; Rust and thread identity; pack tampering/rollback;
clean offline installation and zero outbound attempts; desktop rendering and CLI
unsupported responses; actual Office paste behavior and provenance consistency.

Benchmark targets remain proposals: initial progress within one second, typical
cancellation within two seconds, indexed million-sample query p95 around one second,
5 GiB text processing with engine peak RSS around 2 GiB, and ordinary 2400 px chart
copy around three seconds on a recorded 8-core/16 GiB/SSD environment. Measure each
format/platform independently, including cold cache, unique-stack cardinality,
disk exhaustion, large records, partial results, and bridge payload limits.

## 19 Dependency-ordered implementation scope

B0 means foundation, B1 core value, B2 additional breadth, distinct from review
severity. At the user's request on 2026-11-11, the eighteen items were registered
as T-628–T-645 in the [implementation backlog](../../work_status.md#dependency-ordered-implementation-backlog).
D identifiers remain design references; the Task Registry owns execution status.
Registration does not assert implementation or set a release schedule.

| ID | Scope | Band | Dependencies |
|---|---|---|---|
| D01 | Importer, envelope, measurement, version contracts | B0 | Existing goldens and compatibility inventory |
| D02 | ProfileSet and legacy facade migration | B0 | D01 |
| D03 | Case/capture, clock/entity, redaction, vendor-neutral identity | B0 | D01 |
| D04 | Result capabilities and generic renderer | B0 | D01 |
| D05 | Packs, offline packaging, mandatory fixture bundle | B0 | D01 |
| D06 | Input assistance and manual collection guidance | B1 | D03, D05 |
| D07 | py-spy/Speedscope/pprof correctness and producer matrix | B1 | D02, D04 |
| D08 | Drilldown, detail index, case search, baseline | B1 | D02, D03, D04 |
| D09 | Image/table copying and regeneration | B1 | D05, D07, D08 |
| D18 | perf events and constrained BPF adapters | B1 | D02, D03, D08; loss/clock/identity and on/off-CPU semantics |
| D10 | Prometheus/OTLP history and histograms | B1 | D01, D03, D04; actual OTLP format Q04 |
| D11 | NVIDIA and vLLM basic analysis | B1 | D05, D10, Q03 |
| D12 | LLM app trace, gateway, cold start | B1 | D03, D04, D10; product export Q01 |
| D13 | Python/Node/Rust, thread/GC/access, TA/DB depth | B1 | D02, D03, D08 |
| D14 | Workload export, crash text, comparison context | B2 | D03, D08, D10 |
| D15 | Actual Apple Silicon adapter | B2 | D03, D10, Q02 |
| D16 | PyTorch/Nsight and distributed inference detail | B2 | D08, D10, D11 |
| D17 | Additional profile/serving/DB/OS/APM adapters | B2 | D01–D05 and actual demand/fixtures |

The first useful complete slice is profile correctness → consistent drilldown and
comparison → image/table copying. Do not postpone visuals until every input exists.
Apple identity enters D03; application traces and OTLP metrics enter B1.

## 20 Decisions awaiting actual inputs

The review records vLLM/NVIDIA emphasis, MacBook GPU needs, Python/Rust/Node, and
OTel AI metrics. Accept those needs while leaving four exact input decisions open.

| ID | DECISION_PENDING | Work that can proceed |
|---|---|---|
| Q01 | Langfuse/LangSmith hosting, export route/schema | Common LLM trace, OTLP semantics, privacy |
| Q02 | Mac engine/framework, macOS and available GPU export | Vendor-neutral identity and shared-memory model |
| Q03 | vLLM versions, configuration and metric/log fixtures | Serving capability and metric history contracts |
| Q04 | OTel metric export route (OTLP/Prometheus/range export), encoding/container and instrumentation | Data model, series identity and aggregation rules |

These are not implementation TODO states. They block only dependent adapters.
Personal diagnostic libraries remain later opt-in sanitized patterns with explicit
provenance and deletion/retention; incidents do not become shared rules automatically.

## 21 Related contracts

See [Data Model](./DATA_MODEL.md), [Parser Design](./PARSER_DESIGN.md),
[Importer Support Matrix](./IMPORTER_SUPPORT_MATRIX.md),
[Packaging Plan](./PACKAGING_PLAN.md), and [Report Export Design](./REPORT_EXPORT_DESIGN.md).
External specifications were consulted on 2026-11-11. Implementation must pin
producer, adapter/schema, and fixture versions rather than treat latest/main
documentation as a permanent support guarantee.

## T-628 implementation status (2026-11-11)

The common importer/job/measurement/version foundation and existing Tomcat pilot
are implemented. See [parser implementation](./PARSER_DESIGN.md#t-628-implementation-record-2026-11-11)
and [wire conformance](./DATA_MODEL.md#t-628-wire-implementation-and-conformance-2026-11-11)
for paths, resource limits and compatibility. A sealed result JSON revision rename
is this stage's atomic marker; separate case manifests/cache lifecycle remain T-630.
Tests are not independent review PASS. T-629 onward and undecided Q01–Q04 remain separate.

### T-628 review gate and remaining lifecycle boundary

The original independent T-628 verdict remains CONDITIONAL. R3 confirmed the
R2 findings resolved with no P1/P2 and explicitly permitted DONE; work_status.md
records closure and the final locally verified P3 follow-ups without claiming
an independent PASS for those changes. T-629 now has its own CONDITIONAL review
and R4 remediation checkpoint. T-630 may proceed with case lifecycle work;
bounded backend retention/deduplication, administrative deletion and store locking
are only its foundation. Unsafe public Wails release is removed until ownership
exists. Durable references, reference ownership, migration/recovery,
deletion/restore and indexes remain mandatory. T-631 needs load cancellation and
generic UI fallback; T-632 retains offline distribution acceptance. Full producer
measurement correction remains T-634. Q01–Q04 stay DECISION_PENDING. Actual native
WebView flows, Windows execution and hosted CI are separate from local checks.

### T-629 core implementation checkpoint

The ProfileSet core, exact tree/selection/diff/drilldown/pprof path and legacy
compatibility composition are implemented. See the paired DATA_MODEL and
PARSER_DESIGN contracts for versions, resource bounds and refusal behavior.
`profile_evidence` 0.2.0 is enabled only for validated canonical models; existing
0.1.x producer paths retain legacy semantics. Actual producer corrections and
qualification remain T-634, generic screens T-631, and case/baseline context
T-630/T-635. Local checks and author verification do not grant independent PASS.

R4 review remediation keeps the same semantic model while transporting <=32 MiB
JSON via bounded chunks. Exact tree-byte accounting replaces the old conservative
8 MiB estimate. Comparison conditions separate quality categories from measured
counts/ratios and require explicit mapping for differing entities; references
preserve both quality snapshots. Content validation is cached within an immutable
registry, including the checked metadata.quality copy. DATA_MODEL states effective
joint limits and measured amplification. T-634 producer corrections, T-635 entity
mapping and T-631 actual generic screens remain separate.


## T-630 local implementation checkpoint (2026-11-11)

Case/capture storage, safe intake, original coordinates, immutable baseline refs,
caller leases and cache recovery now have a local implementation. The exact
contracts and operational limits are recorded in [DATA_MODEL](DATA_MODEL.md#t-630-case-and-capture-storage-contract-2026-11-11)
and [PARSER_DESIGN](PARSER_DESIGN.md#t-630-safe-intake-and-adapter-boundary-2026-11-11).
The task remains REVIEW: local Go/CLI/bridge/state tests and native/cross-builds
are distinct from independent approval, actual new WebView interaction and Windows
runtime. Generic presentation/discovery/search remain T-631/T-633/T-635. This
checkpoint does not activate optional producer decisions or the entire design.

## T-640 implementation boundary (2026-11-12)

The limited `trace.llm-json` importer normalizes LLMTrace 1.0.0 into existing trace_import 0.3.0 / llm_core 1.0.0. Legacy trace 0.2.0 remains unchanged. Parent/link, observed child-union durations, explicit logical/attempt accounting, token boundaries and lifecycle populations stay separate. Body-free normalization precedes case copies/indexing. Exact bounded query/UI and existing providers/exports are reused. Q01 vendor JSON adapters stay gated. See [LLM trace evidence](LLM_TRACE_EVIDENCE.md). T-639 R16 adds explicitly declared untyped range metadata and user event-ID normalization, with 1.0/1.1 model and 0.1/0.2 result readers preserved; see [inference evidence](INFERENCE_EVIDENCE.md).


## T-641 runtime evidence

Runtime evidence uses the separate bounded canonical model within AnalysisResult. See [RUNTIME_EVIDENCE.md](RUNTIME_EVIDENCE.md) for pinned producer formats, identity/window qualification, live allocation profile reuse, GC union, plan per-loop semantics and saved-reader boundaries.


## T-642 execution evidence

Bounded benchmark/crash context uses execution_core 1.0.0 inside common AnalysisResult, execution_evidence 0.1.0. Pinned k6/LLMPerf and structural JTL/actual sanitized HotSpot inputs normalize before generic providers/cases/exports. Comparison is explicitly declared-condition snapshot verification, separate from profile original-reimport baselines. See [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md).


T-645 bounded actual DB/OS snapshot batch: [Manual database and socket snapshots](SYSTEM_DB_SNAPSHOTS.md). Existing MetricHistory transport, exact epochs/gauges and common consumers; other adapters remain fixture-gated.
