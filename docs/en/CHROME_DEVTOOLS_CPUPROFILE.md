# Chrome DevTools / V8 CPU Profile Analysis (Design Note)

> The Korean note (`docs/ko/CHROME_DEVTOOLS_CPUPROFILE.md`) is the normative,
> fully detailed design record; this English pair is the condensed reference.

## Status

All four design gates raised by the 2026-07-18 Codex design review were closed
on 2026-07-20. The note is implementation-ready, and as of 2026-07-21 the core
phases are implemented (see Implementation Status below).

| Gate | Issue | TO-DO | Resolution |
| --- | --- | --- | --- |
| G1 Capture format | Chrome's Performance panel saves a trace (`.json`/`.json.gz`), not `.cpuprofile` | T-558 | Confirmed 2026-07-20. Chrome trace is the first-class input; `.cpuprofile` is supported in the same Phase 1 |
| G2 Measurement unit | `Sample.Value` unit and the `IntervalMS` double-conversion problem | T-559 | Confirmed 2026-07-20. Microseconds `int64`, `samples[]` authoritative, `ValueUnit` branch, invariants `INV-C1`..`INV-C9` |
| G3 Desktop path | New importer and legacy `ProfilerService`/`DiffPage` used different service paths | T-560 | Confirmed 2026-07-20. Option A path unification — `AnalyzeProfileEvidence` is the only canonical desktop path; legacy format switch frozen; staged rollout with AC-1..AC-4 |
| G4 Temporal semantics | CPU samples alone cannot recover browser Long Task boundaries | T-561 | Confirmed 2026-07-20. Pre-collapse `buildSampleRuns` contract, `cpu_sample_runs`/`cpu_activity` outputs, `SAMPLED_CPU_HOTSPOT` finding (100 ms default), `INV-T1`..`T5`. Flame-chart rendering deferred; true Long Task moved to Phase 4 item 21 |

## 1. Background and Non-Goals

ArchScope's runtime profile importer (`internal/parsers/profile`) supports 22
formats, all server-side (JVM, Python, Ruby, PHP, .NET, Go, Rust). Browser
JavaScript profiles had no entry point: a `.cpuprofile` fed to the engine fell
through `detect()` to `generic-async-stack`, or — renamed to `.json` — to
`generic-profile-json`, producing empty results or `PROFILE_EMPTY`. Support was
absent and failure was silent.

Why add it:

1. **Evidence for frontend bottlenecks.** ArchScope narrows response-time
   evidence from access logs to APM to server profiles, but "server says 80 ms,
   user feels 3 s" cases (bundle parsing, re-render storms, long tasks)
   remained guesswork.
2. **Near-zero collection cost.** The DevTools Performance panel ships with
   every Chrome — no agent, permission, or restart required.
3. **High pipeline reuse.** Once samples are normalized to `[]Sample`,
   FlameGraph, Drilldown, Timeline, Classification, Diff, and Report Export
   work unchanged.
4. **Symmetry with Node.js.** Node `--cpu-prof` output uses the same V8
   CpuProfile schema, so Node profile support is a by-product.

Non-goals:

- ArchScope does not replace the DevTools UI. Its role is archival,
  comparison, evidence, and reporting; live interactive profiling stays in
  DevTools. The Browser CPU page is post-hoc only — no live/attach mode.
- Direct Chrome DevTools Protocol (CDP) collection is out of scope.

### Flame graph, not flame chart

DevTools draws a flame chart (X axis = time, each call a separate block).
ArchScope's `buildFlameTree` draws a flame graph (X axis = sample weight; equal
stacks merged via `collapsedStacks`, time order intentionally discarded). This
is by design: diffing two profiles requires the aggregated, time-free
representation. The two views are complementary; the roadmap completes the
flame graph first and treats time-axis output as a separate, bounded contract
(see Temporal Analysis below).

## 2. Supported Formats

### 2.1 Phase 1 input contract (G1 / T-558 — confirmed)

The first-class input is the Chrome Performance trace, because that is what
the DevTools **Save trace** button produces (`.json`, or `.json.gz` — gzip is
the default from Chrome 142). A `.cpuprofile`-first plan was rejected: it
would build a different feature (Node/CDP profile support) and forfeit the
"zero collection cost" rationale.

| Input | Extension | Format id | Notes |
| --- | --- | --- | --- |
| Chrome Performance trace | `.json` | `chrome-trace-json` | Top-level array and `{traceEvents, metadata}` object both accepted |
| Chrome Performance trace (compressed) | `.json.gz` | `chrome-trace-json` | Chrome 142+ default |
| V8 CPU profile | `.cpuprofile` | `v8-cpuprofile` | Node `--cpu-prof`, CDP `Profiler.stop` |
| V8 CPU profile (compressed) | `.cpuprofile.gz` | `v8-cpuprofile` | Same decompression path |

All inputs converge on one V8 CpuProfile normalization core, so `.cpuprofile`
support is nearly free once the trace adapter exists.

The Phase 1 trace adapter is a **filter, not a full trace parser**. It:

- detects and streams gzip decompression;
- accepts both top-level array and `{traceEvents, metadata}` wrappers;
- extracts only `ph:"P"` `Profile`/`ProfileChunk` events, accumulating chunks
  per profile id, and hands the merged result to the V8 normalizer;
- reads Chrome version/process info from `metadata`.

It does **not** load the whole trace into a memory model, interpret `ph:"X"`
duration events (Long Task, Layout, Paint), or reconstruct event correlation.
Duration-event modelling is Phase 4 item 21. NetLog and general trace-event
modelling are out of Phase 1 scope.

Multi-process/multi-thread traces: Phase 1 selects the **largest** profile and
reports the others via diagnostic; a selection UI is deferred to Phase 2.

### 2.2 V8 CpuProfile schema essentials

The schema carries `nodes[]` (parent-to-child call tree with `hitCount`),
`samples[]` (leaf node id per observation), and `timeDeltas[]` (microsecond
gaps). Key consequences:

- `nodes[]` has no `parent` field; a child-to-parent reverse index must be
  built once. Node ids are not guaranteed contiguous — use a map, not array
  indexing.
- `nodes[]`/`hitCount` alone can only produce a flame graph (aggregation).
  `samples[]` + `timeDeltas[]` are the sole source of the time axis
  (`ts[i] = startTime + sum(timeDeltas[0..i])`), so the parser must preserve
  both arrays.
- Per CDP, `samples`, `timeDeltas`, `children`, and `hitCount` are all
  optional. Profiles without `samples[]` are **supported** via an
  aggregation-only path (see the measurement contract): sample count comes
  from `hitCount` sums, the time axis is disabled with a diagnostic, and
  `duration_ms` is not fabricated.
- Synthetic frames `(root)`, `(program)`, `(idle)`, `(garbage collector)`
  need classification rules. `(idle)` is excluded from display by default
  (opt-in `--include-idle`); `(garbage collector)` is always included — GC is
  real cost. Exclusion is a display option, never an accounting option.

### 2.3 Out of scope

`.heapprofile`/`.heapsnapshot` (different axis, separate family), Firefox
Profiler JSON (different schema, revisit on demand), Safari Web Inspector
(closed format). Speedscope-converted profiles already work today via
`speedscope-json`.

## 3. Measurement and Time-Attribution Contract (G2 / T-559 — confirmed)

The draft's combination of microsecond `Sample.Value` plus a back-calculated
`IntervalMS` double-converted every time metric. The confirmed contract:

**Decision 1 — values are microsecond `int64`.** `Sample.Value` holds the CPU
time attributed to the sample in integer microseconds. Nanoseconds would fake
precision the source lacks; millisecond floats would accumulate error and make
tolerance undefinable. Conversion to milliseconds happens exactly once, at the
display boundary.

**Decision 2 — `samples[]` is authoritative; `hitCount` is a cross-check.**
Sample count is `len(samples)`. `hitCount` is retained only for invariant
checking (`INV-C3`). `hitCount`-only inputs get counts and ratios but no
time-axis features and no estimated `duration_ms`.

**Decision 3 — the `IntervalMS` back-calculation path is severed for V8.**
The parser sets `Parsed.ValueUnit = "microseconds"` as a first-class field.
The common analyzer branches on it: `"samples"` (the existing 22 formats)
keeps the current interval-based path with zero regression;
`"microseconds"` uses `sum(Value)` directly as duration. The hard-coded
`sample_unit: "samples"` in the unified schema becomes derived from
`ValueUnit`, and time conversion is centralized in a single function.

**Decision 4 — time attribution.** `timeDeltas[0]` is the gap from
`startTime` to the first sample. Timestamps: `ts[i] = startTime +
sum(timeDeltas[0..i])`. Cost of sample `i` = `ts[i+1] - ts[i]`; the last
sample gets `endTime - ts[i]`. Thus **`timeDeltas[i]` is the cost of sample
`i-1`, not sample `i`** — an off-by-one here shifts all attribution, so
golden tests pin it. Negative deltas are clamped to zero with a diagnostic;
`len(timeDeltas) != len(samples)` is an error by default (opt-in tolerant
recovery only). Recording duration and active CPU duration are recorded
separately: `recording_duration_us` (`endTime - startTime`, constant
regardless of display options), `active_duration_us`, `idle_duration_us`,
`sampled_duration_us`.

**Decision 5 — duration invariants**, pinned by golden tests before
implementation (tolerance 0 in integer microseconds except where clamping
occurred):

| ID | Invariant |
| --- | --- |
| INV-C1 | `sampled_duration_us == active_duration_us + idle_duration_us` |
| INV-C2 | `sampled_duration_us == endTime - startTime` (excluding clamp amounts) |
| INV-C3 | Per node, occurrences in `samples[]` == `hitCount` (diagnostic on mismatch) |
| INV-C4 | Per node, `total_us == self_us + sum(child total_us)` |
| INV-C5 | Root `total_us == sampled_duration_us` |
| INV-C6 | `sum(all self_us) == sampled_duration_us` |
| INV-C7 | CLI defaults and Wails defaults produce identical results |
| INV-C8 | Diff baseline/target share identical units regardless of normalization |
| INV-C9 | `recording_duration_us` unchanged by `--include-idle` |

`self_us` is the attributed time of samples where the node is the leaf;
`total_us` covers all samples containing the node in their stack, counted
**once** per stack even under recursion (otherwise INV-C5 breaks for any
recursive profile). INV-C5/C6 exist because INV-C4 alone cannot detect a
dropped subtree.

## 4. Parser Design Highlights

- New files `cpuprofile.go` (and the trace adapter) alongside `parser.go`;
  `detect()` gains a content-based `nodes[]`+`samples[]` signature check
  **before** the `.json` extension branch, plus `.cpuprofile` extension and
  `canonical()` aliases. Regression tests guard existing fixtures against the
  new detection order.
- Stack reconstruction walks the child-to-parent index to build root-first
  stacks, memoized per node id.
- Frame mapping fills `Language: "JavaScript"` and `Runtime: "V8"` explicitly
  so `inferRuntime` (which would misread `v8::` as Rust) never runs. Empty
  function names become `(anonymous)` to avoid collapsing distinct frames.
- **Display name and identity are separated.** `Frame.File` keeps the
  canonical URL (query/fragment stripped); the collapse key includes script
  identity (`scriptId` or canonical URL) so same-basename scripts from
  different origins do not merge; short names are produced only in the UI
  layer. Redaction rules cover URLs, source content, extension ids, and
  internal hostnames.
- Graph validation rejects malformed inputs with deterministic diagnostics:
  duplicate/zero/negative node ids, dangling sample or child ids, multiple
  parents, child cycles, `endTime < startTime`, and length mismatches are
  errors; disconnected non-root nodes are warnings handled as orphan roots.
- Metadata recorded in `Parsed.Metadata`: `value_unit`, `start_time_us`/
  `end_time_us`, `duration_ms`, `sample_count`, `node_count`,
  `avg_sample_interval_us`, `idle_ratio`, bounded and redacted `scripts` list.
- Option ownership (`--include-idle`, size/depth caps, sample cap) is defined
  per layer (parser/analyzer/UI) with defaults, metadata recording, and a
  rule that Diff enforces identical options on both sides.

## 5. Desktop Integration Path (G3 / T-560 — confirmed)

The new importer and the legacy desktop pages used different services: the
legacy `ProfilerService` accepts only four formats (collapsed / Jennifer /
SVG / HTML) for both analysis and Diff, and no frontend page called
`AnalyzeProfileEvidence`. "CLI import works" and "desktop/Diff works" were
therefore separate problems.

**Decision — Option A, path unification.** The single normalization point is
`internal/parsers/profile.Parsed` (`[]Sample`). The only canonical desktop
path is `EngineService.AnalyzeProfileEvidence` ->
`internal/analyzers/profile.Build` -> `profile_evidence` `AnalysisResult`.
Once the `chrome-trace-json`/`v8-cpuprofile` parsers land in
`parsers/profile`, CLI, desktop, Diff, and Export all consume them through
the same code. Adding new formats to the legacy `ProfilerService` format
switch (`runAnalyze`/`loadStacks`/`detectFormat`) is **forbidden** — that
switch is frozen and slated for shrinkage.

Staged rollout (each stage independently releasable):

| Stage | Content | Regression surface |
| --- | --- | --- |
| 1. Diff common loader | `stacksFromParsed` loader in Go; `loadStacks` delegates to it; `DiffPage` moves from a fixed 4-format list to auto-detect | Diff page only; golden tests pin equivalence for the existing four formats |
| 2. Browser page | New `BrowserCpuProfilePage` calls `AnalyzeProfileEvidence` only, no legacy dependency | New page only |
| 3. Workspace/Export | Register Browser page results in the Analysis Workspace session registry as `profile_evidence`; Export Center and report pack consume them unchanged — no new export code | One registration call |
| 4. Legacy page migration (follow-up) | `ProfilerAnalyzerPage` moves to `AnalyzeProfileEvidence`; legacy service shrinks to a Jennifer/SVG/HTML adapter. Outside T-560's completion scope | Entire legacy page — done last |

Acceptance criteria: **AC-1** CLI/desktop parity (identical `AnalysisResult`
modulo timestamps, pinned by golden fixtures); **AC-2** Diff works for two
`chrome-trace-json` files and legacy four-format Diff output is unchanged;
**AC-3** Browser page results appear in Workspace and export via Export
Center without pipeline changes; **AC-4** review confirms no new format was
added to the frozen legacy switch.

`ResultType` remains `"profile_evidence"`; the `EvidenceFamilySpec` registry
is unchanged (format addition within an existing family).

## 6. Temporal Analysis Contract (G4 / T-561 — confirmed)

CPU samples provide only the top node and timestamp per observation; browser
task start/end boundaries are not recoverable from them. The draft's plan to
emit "long task" findings from merged samples was semantically wrong, and
structurally impossible anyway: `collapsedStacks` discards `Sample.Labels`,
and `buildTimeline` receives aggregated `FlameNode`s, so the `ts_us` label
never reached the timeline builder.

**Contract (9.3.1).** Before `collapsedStacks` is called,
`analyzers/profile.Build` runs `buildSampleRuns(parsed.Samples)`, which
consumes `Sample.Labels["ts_us"]` (written by the parser in Phase 1). The
aggregation path is untouched.

Run split rules — a run ends when:

1. the `stackKey` differs from the previous sample (script-identity-aware
   key);
2. an idle/GC non-attributable sample appears — idle is never inside a run,
   only a boundary;
3. the gap to the previous sample exceeds `10 x median(timeDeltas)` (guards
   against bridging recording pauses).

Run fields: `start_us`, `end_us`, `duration_us`, `sample_count`,
`top_frame`, `stack_key`, `category`; durations use the same attribution rule
as the measurement contract (`timeDeltas[i]` -> sample `i-1`).

Outputs (the `AnalysisResult` envelope is unchanged; only keys are added,
both bounded to respect the size-stable guardrail):

| Key | Content | Cap |
| --- | --- | --- |
| `tables["cpu_sample_runs"]` | Runs, descending by duration | top-N (default 50) |
| `series["cpu_activity"]` | Fixed-bucket CPU activity time series | <= 1,000 buckets |

Formats without `ts_us` (the 21 collapsed-style formats) get **neither key**
— the time axis is never faked from a uniform-distribution assumption. The
legacy uniform timeline series remains for compatibility, but the UI prefers
`cpu_sample_runs` when present.

**Finding contract.** The finding code is `SAMPLED_CPU_HOTSPOT`. `LONG_TASK`
codes and "Long Task" wording are **banned** for sample-derived findings. The
default threshold is **100 ms**, deliberately different from RAIL's 50 ms
Long Task definition so users cannot equate the two (configurable). Finding
text must state that this is a sampled observation window, not a browser task
boundary. The true Long Task finding (`BROWSER_LONG_TASK`) is built in
Phase 4 item 21 from Chrome trace `RunTask` (`ph:"X"`) boundaries, with CPU
samples attributed into those windows.

Invariants:

- **INV-T1**: runs are time-ordered and non-overlapping.
- **INV-T2**: `sum(duration_us) <= active duration`.
- **INV-T3**: per-stack run duration sums equal the aggregated flame graph's
  time for that stack (cross-check that both derive from the same
  `[]Sample`).
- **INV-T4**: downsampled inputs emit no `cpu_sample_runs`, `cpu_activity`,
  or `SAMPLED_CPU_HOTSPOT` — only a `TIMELINE_SUPPRESSED_DOWNSAMPLED`
  diagnostic, since uniform downsampling distorts time windows.
- **INV-T5**: `hitCount`-only inputs emit no temporal outputs (no ordering
  information exists).

**Flame-chart rendering is deferred** (Phase 3 item 13). Phase 3 ships the
runs table, activity series, and finding; rendering resumes only if demand
for re-viewing archived profiles is confirmed. Until then the timeline view
shows `cpu_activity`/`cpu_sample_runs` data only.

## 7. Browser Frame Classification

The repository had two disconnected classifiers: the JSON-rule-based
`ClassifyStack` (user-editable, but consumed by a single standalone Wails
method) and the hard-coded `classifyFrames` (the actual analysis path).
T-562 requires unifying them behind **one interface that takes source format
and runtime context as inputs** before rules take effect.

Browser vocabulary rules (first match wins, case-insensitive substring;
narrow rules first): `React`, `Vue`, `Bundler Runtime`, `V8 Internal`
(`(garbage collector)`, `(program)`, `(root)`, `(idle)`, `v8::`,
`builtin:`), `Browser API` (`blink::`, `settimeout`,
`requestanimationframe`, `xmlhttprequest`, `fetch`).

Because `v8::` frames are common to both runtimes, ordering alone cannot
separate them: **Node V8 frames must not be reclassified as browser frames.**
The classifier prioritizes `V8 Internal` only in the browser context of the
V8 cpuprofile format; Node context keeps the existing order, pinned by a Node
`--cpu-prof` regression fixture.

Semantic categories reuse existing constants (`GC_JVM_RUNTIME`,
`EXTERNAL_API_HTTP`, `FRAMEWORK_MIDDLEWARE`, `APPLICATION_LOGIC`); renaming
is deferred as explicit design debt, with browser-friendly labels applied in
the display layer only. `EXTERNAL_API_HTTP` samples represent the CPU cost of
**initiating** requests, not network latency — CPU profiles do not sample
waiting time, and UI/finding wording must say so. Category/color injection
into flame-graph nodes (currently `null` in `freezeNode`) is an identified
engine-side task so the frontend renderer needs no change.

## 8. Frontend — Browser Performance Analysis Menu

A dedicated sidebar group, "Browser performance analysis"
(`navBrowserTools`), is added between the analysis tools and workspace
groups, initially containing one item: CPU profile (`browser_cpu`,
`pages/BrowserCpuProfilePage.tsx`). Reserved future siblings: Lighthouse
reports and Web Vitals (separate evidence families; a menu group is a
user-facing bundle, not an engine family). Separation rationale: different
users, different collection guidance, different interpretation vocabulary,
different extension direction. The engine — parsers, analyzers,
`AnalysisResult` contract, flame graph builders, CLI — is **fully shared**;
only the entry point and presentation are separate.

Page essentials:

- `BrowserCpuProfilePage` calls `AnalyzeProfileEvidence` only (rollout
  stage 2) and recomposes shared components (`CanvasFlameGraph`,
  `WailsFileDock`, `DrilldownPanel`, etc.); the duplicated `adaptFlameNode`
  adapter is extracted to a shared function rather than copied a fourth time.
- The collection guide leads with DevTools Performance panel -> **Save
  trace**; `.cpuprofile` is documented as the secondary path for Node/CDP
  users.
- File filter: `.json` / `.json.gz` / `.cpuprofile` / `.cpuprofile.gz`.
- Framework color mapping is supplied by the engine via `node.color`
  (frameworks blue, bundler runtime gray, V8 internal/GC orange, browser API
  teal, application code green), with legend and tooltips so color is not the
  only cue.
- The CPU-occupancy view uses the wording "sustained CPU occupancy", never
  "long task"; tooltips state the 100 ms threshold is CPU-based and distinct
  from browser Long Task.
- Source locations are display-only (remote/`webpack://` URLs do not open
  locally); a copy button is provided and redaction is honored.
- Comparison view reuses the `DiffPage` delta-color precedent; frames are
  matched by identity (URL + function + line), and unstable identities are
  shown as unmatched rather than force-matched.
- No live mode: the page analyzes saved files only.

## 9. Large Profile Handling

Implemented boundaries:

- Legacy collapsed path guards: `defaultMaxUniqueStacks = 100_000`,
  `defaultMaxStackDepth = 512`, `defaultMaxRSSMB = 4096`,
  `defaultMaxFlamegraphNodes = 100_000`.
- V8/Chrome inputs use a streaming path selected before the whole-file
  `[]byte` path: gzip is decompressed as a stream with a 256 MiB (or
  caller-set `MaxBytes`) guard, and Chrome traces are walked event-by-event
  with `json.Decoder`, keeping only `ph:"P"` events. Neither the full trace
  document nor a `[]json.RawMessage` copy is materialized. `auto` detection
  reads only a 64 KiB preview.
- Sample cap: profiles above `DefaultMaxProfileSamples = 500,000` samples are
  reduced. Plain uniform sampling with count scaling is **not** used (it
  discards skipped `timeDeltas` and distorts totals); instead, deterministic
  time-ordered buckets anchor the first stack/timestamp and sum all
  microsecond deltas, preserving top-level bottlenecks and total sampled
  duration. The result is a **successful** result carrying
  `parser_metadata.partial_result=true`, original/output sample counts, the
  method, and a `PROFILE_DOWNSAMPLED` warning; the Browser CPU page surfaces
  the partial-result state. Size-limit or malformed-input failures return
  errors, not partial results.
- Depth truncation reuses `MaxStackDepth = 512` with `OverDepthRecords`
  diagnostics; flame-graph pruning (`MaxFlamegraphNodes`, per-level top-K +
  `"...other"`) already exists and needs no change.

Interaction rule: downsampling must land before temporal analysis is enabled
for an input — INV-T4 suppresses all time-axis claims for downsampled
profiles.

## 10. Implementation Phases and Status

Execution ownership and the new review-group gates are tracked in
[Browser Profile and HTTP Capture Implementation and Review-Gate Plan](./BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md).

| Phase | Scope | Status (2026-07-21) |
| --- | --- | --- |
| 1 | Normalization core (`parseV8CpuProfile`), unit contract (`ValueUnit`, INV-C1..C9 golden tests first), `ph:"P"` trace adapter with gzip streaming, largest-profile selection, `detect()`/`canonical()` for `chrome-trace-json` and `v8-cpuprofile`, fixtures, CLI help | Implemented |
| 2 | Browser classification rules and tokens, Node `--cpu-prof` regression fixture, pre/post-deploy Diff validation | Implemented |
| 3 | `buildSampleRuns`, `cpu_sample_runs`/`cpu_activity`, `SAMPLED_CPU_HOTSPOT` (100 ms), INV-T1..T5; flame-chart rendering deferred | Implemented (rendering deferred by decision) |
| 4 | Size guards, downsampling + diagnostics, depth truncation, Browser sidebar group and page, Diff common loader with auto-detect, Workspace/Export registration; `ph:"X"` duration events and `BROWSER_LONG_TASK` remain in this phase's item 21 | Release scope implemented; optional duration-event extension deferred |

`AnalyzeProfileEvidence` generates `cpu_sample_runs` and `cpu_activity` from
pre-collapse V8 samples, records only runs of 100 ms or more as
`SAMPLED_CPU_HOTSPOT`, and the Browser CPU UI labels them as distinct from
browser Long Tasks. V8/Chrome Diff uses the same `parsers/profile.Parsed`
normalization path.

**T-565 is complete (2026-07-21):** the paired shared fixture corpus and
manifest-driven goldens cover V8/Node/CDP, object/array/gzip trace wrappers,
malformed graphs, hitCount-only input, negative deltas, temporal suppression,
and the paired 3.1 s/210 ms acceptance scenario. The importer matrix, data
model, user guide, and performance guide are updated in English and Korean.

The remaining `ph:"X"` duration-event work (`BROWSER_LONG_TASK`, Layout, and
Paint) is an **optional extension**, not unfinished T-565 release work. If it is
promoted, it follows `C-EXT1` and the mandatory `C-SEM1` semantic review in the
implementation plan.

## 11. Constraints and Future Extensions

Constraints (must be surfaced in user documentation):

- **Sampling limits.** V8 samples at roughly 1 ms; shorter functions appear
  only statistically and may be missing entirely from short recordings.
  Absence does not mean "did not run".
- **Inlining.** V8-inlined functions are absorbed into their callers and
  never appear as frames; interpret self time accordingly.
- **No source maps.** Production bundles show `main.a1b2c3.js:1:48213`;
  restoring original names requires profiling a development build.
- **CPU only.** Network waits, layout, paint, and compositing do not appear
  in a CPU profile; it explains only part of frontend performance.
- **Single thread.** Workers and the main thread are separate files; no
  merged view.

Future extensions: full Chrome trace duration events (long task, layout,
paint alongside the timeline), source-map resolution, Lighthouse JSON and Web
Vitals as separate evidence families (menu slots reserved), Performance
Insights once Chrome's export format stabilizes, direct CDP collection, RUM
ingestion, and server-client correlation via trace ids — the last being
ArchScope's distinctive goal, enabled but not implemented by this work.
