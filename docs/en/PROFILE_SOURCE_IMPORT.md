# Exact original profile import

The original-profile importer adds a separate original-source path. It does not change `profile import`,
legacy facades, saved results, or internal ProfileSet 0.2.0 results. Reimport the
original to obtain `profile_evidence` payload 0.4.0; do not relabel saved evidence.
The common AnalysisResult envelope and ProfileSet schema 1.0.0 remain intact.
The reserved `org.archscope.profile_source.v1` extension pins normalizer 1.1.0,
format revision, original metric/profile declarations and source metadata.

## Desktop and CLI

In Analysis Workspace, choose the original file and expand **Original py-spy /
Speedscope / pprof profile**. Inspect it, explicitly select a metric and profiles,
and save a new normalized model. The panel selects `profile.source-set-json`
(alias `profile-source-set`) and sets the exact selection. Press Analyze to run
the common jobs, views, case storage and export flow. Empty profiles remain in
the inventory; no CPU metric, first worker or first value column is auto-selected.

```sh
archscope-engine profile inspect-source --in input.json --format speedscope-json
archscope-engine profile normalize-source --in input.json --format speedscope-json --out new-model.json
archscope-engine evidence import --in new-model.json --importer profile.source-set-json --options '{"profile_ids":["profile-0"],"metric_id":"sampled-seconds"}'
archscope-engine profile import-source --in input.json --format speedscope-json --profile-id profile-0 --metric-id sampled-seconds --out result.json
# Formats without exporter metadata require an explicit declaration to resolve
# a qualified capture policy. Declarations are not producer authentication.
archscope-engine profile inspect-source --in input.raw --format pyspy-raw --producer py-spy --producer-version 0.4.1
archscope-engine profile inspect-source --in heap.pb.gz --format pprof --producer go.runtime/pprof --producer-version go1.26.10
```

`--capture-options` accepts a complete sanitized JSON object. Remove credentials,
environment and command values beforehand; common sensitive keys are refused,
but this is not a general secret detector. Do not include production secrets in
arbitrary fields. Unknown safe keys are retained and compared strictly. Exporter
metadata, when present as `name@version`, must agree with explicit declarations.
Raw/pprof need explicit format; auto recognizes only the Speedscope JSON path.
The old `profile.set-json` transport refuses the new source extension, and the
new transport refuses old models, preserving disjoint result readers.

Inspection returns a bounded inventory, not the model through JavaScript.
Normalization rechecks the original SHA-256 and writes a new file atomically;
changed sources and existing outputs are refused. Source/options changes cancel
stale UI requests. Closing or canceling a request does not publish a partial model.

## Measurement semantics and limits

| Format | Preserved interpretation | Limits / explicit refusals |
|---|---|---|
| py-spy raw / collapsed | Every row, exact decimal/scientific nonnegative weight including zero; aggregate sample count; root-first stack | No inferred sampling duration, CPU time or worker boundaries. Process/thread labels in raw output remain stack frames. Dump text is not this format. |
| Speedscope sampled | Every profile, frame file/line/column, native unit and exact weight including zero | Samples and weights must both exist with equal length; no invented per-sample timestamps or CPU interpretation for time weights. |
| Speedscope evented | Balanced ordered O/C frames, exact exclusive intervals on the active stack; declared start/end | Invalid order, bounds, frame refs, missing frame, unbalanced stacks and non-time units are refused. Unstacked elapsed gaps are explicit `[unattributed]` intervals; total equals the declared span, not event count. |
| pprof | All sample value columns with original type/unit, exact int64 values including zero/signed differences, inline/root ordering, labels/numeric units, period/type/duration metadata | Values are already weighted; never multiply period twice. Known process/thread/worker/rank label tuples form distinct opaque profiles. Unlabeled data stays aggregate. Unknown units are refused. |

pprof source location/mapping/function metadata is retained in the extension,
including addresses and inline provenance. Canonical frame aggregation remains
function/file/line based. Supported units: count/samples/objects, bytes, seconds,
milliseconds, microseconds, nanoseconds and cycles. A negative column becomes an
explicit signed-difference metric. Speedscope units `none` and bytes remain count
and bytes; time becomes duration, not automatically CPU time.

Limits: 64 MiB original and decompressed source, 32 MiB serialized model, 100,000
observations (pprof values count separately), 100,000 frames, depth 1,024, 1,024
profiles and one million distinct-stack frame references. Collapsed lines are
bounded to 1 MiB. Wire string/token/depth limits also apply. File normalization
allows two concurrent operations and a five-minute context deadline. Budgets
refuse the complete operation; no truncated success is returned. The generic
transport retains its independent chunk/record/result/temp ceilings.

The existing exact tree/diff/drilldown/export operations consume the selected
model. pprof export is an explicit selected aggregate projection with tick scale
and aggregated-dimension disclosure, not a byte-identical original reconstruction.
Fractional weights require an exact tick scale; inexact/overflowing export is
refused. A pprof export with `ticks` units is not accepted by this original-source
reader, because its original dimensional interpretation requires the projection
metadata. Keep the saved AnalysisResult for lossless model reuse.

<a id="qualified-capture-option-policies-r5-02"></a>

## Qualified capture-option policies

Only compiled bindings resolve a policy. The snapshot includes policy ID, revision
1.0.0 and hash, producer/version/format/normalizer/metric, complete sanitized
original options, semantic projection, incidental projection and strict unknown
keys. Input-supplied whitelist/ignore/policy declarations have no authority.
Missing, null and zero remain distinct; both originals and incidental changes
are disclosed in canonical comparison references. Entity, clock, metric,
selection, semantic revision and quality-category gates still apply. A declared
non-null capture PID also binds the profile process condition; changing PID
requires explicit entity mapping, which this task does not invent.

| Qualified binding | Semantic keys | Incidental keys |
|---|---|---|
| py-spy 0.4.1 raw count and sampled Speedscope seconds | rate, gil, native, idle, subprocesses, duration, sampling_mode | pid, output_path |
| Go runtime/pprof go1.26.10 heap alloc_objects/count and alloc_space/bytes | sample_rate, period, period_type, profile_type, source_period, source_period_type, duration, source_duration | pid, output_path |
| Same Go heap inuse_objects/count and inuse_space/bytes | sample_rate, period, period_type, profile_type, source_period, source_period_type | pid, output_path, duration, source_duration |

Speedscope source profile bounds are derived and strict; user overrides of
reserved source fields are refused. Other versions/producers/metrics, including
Go CPU profiles and evented Speedscope, parse where structurally supported but
receive no relaxed policy. Every unclassified option remains strict. There is no
global PID/duration ignore rule. Source identity is declared or file-exporter
based, never original-source authentication.

New canonical comparison identity uses `profile-conditions-v4`; existing model
v2 hashes remain unchanged. Saved 0.4.0 metadata is recomputed and checked against
the embedded model/policy. Generic scalar comparison deliberately retains opaque
strict capture equality and discloses this scope; use canonical ProfileSet diff
for qualified incidental differences. Durable baseline binding and declared entity mapping are implemented in
[Case evidence queries](CASE_EVIDENCE_QUERIES.md); the older generic scalar path
retains strict capture equality.

## Qualification evidence

The digest-pinned `diagnostics.profile-producers.v1` reusable corpus contains
actual py-spy 0.4.1 synthetic subprocess recordings (Speedscope and raw) and an
actual Go go1.26.10 synthetic heap recording, with documented path/time
sanitization. Byte-identical fixture copies live in the parser testdata, so tests
and runtime do not require a sibling checkout. This qualifies those fixture
slices, not all tool versions, platforms or options. Synthetic tests separately
cover fractions/zero, mixed profiles, evented gaps/nesting/refusals, pprof multiple
columns/units/signed values/large integers/labels, policy safety and saved/Wails
round trips. Native Windows UI execution and independent review are separate.

Format references: [Speedscope specification](https://github.com/jlfwong/speedscope/blob/main/src/lib/file-format-spec.ts),
[py-spy 0.4.1 exporter](https://github.com/benfred/py-spy/blob/v0.4.1/src/speedscope.rs),
[pprof proto](https://github.com/google/pprof/blob/main/proto/profile.proto).

<a id="r11-revision-boundary-2026-11-12"></a>

## revision boundary (2026-11-12)

Normalizer 1.1.0 / payload 0.4.0 adds original source type/unit to every source
metric, inventory, UI and result summary. Column IDs remain stable within the
file, but identity also binds the original type/unit and format revision without
requiring a capture policy. `alloc_space` and `inuse_space` cannot compare under
an undeclared or unqualified producer. Old 0.3.0 snapshots keep their original
v2/v3 hashes and can be read, saved, queried and exported; comparison requires
explicit original reimport into a new result. No old result is relabeled.

Speedscope `none` maps to generic count-weight (`kind=count`), preserves
`source_unit=none` and never asserts total_samples. New policy snapshots expose
basis=declared/file_exporter and comparisons warn capture_policy_uses_declared_producer.
Replay checks bytes/options/model, not producer authenticity. The 100k observation
ceiling now reports actual/limit values for observations/frames/depth. At 100 Hz,
100k sampled observations are about 1,000 seconds for one worker or 250 seconds
for four equally sampled workers; pprof columns each consume an observation.
No automatic coalescing loses original observation references. See
[Case evidence queries](CASE_EVIDENCE_QUERIES.md) for verified baseline reuse.
