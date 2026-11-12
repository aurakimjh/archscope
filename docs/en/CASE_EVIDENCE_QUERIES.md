# Case evidence queries and reusable baselines

T-635 adds query contract 1.0.0 without changing the AnalysisResult envelope,
existing profile payload semantics, case schema or saved bytes. New selections
become explicit new results; baseline pins never follow a newer result implicitly.

## Desktop workflow

1. Import an original profile through the T-634 source panel. Add it to Workspace.
2. Open **Canonical profile drilldown and evidence**. Select profiles/metric and
   ordered conjunctive frame-ID filters. Query top frames, tree/flamegraph,
   callers, callees or observations. Click an evidence path to focus it. **Add
   selection as new result** creates an immutable backend result in Workspace.
3. Open **Case search and reusable baselines**. Create/select a case and load its
   current revision. Attach the original as a read-only external reference, select
   that source, and save the current exact result with its source binding. Existing
   snapshot-only case saves are preserved; they cannot assert reimport verification.
4. Search the case, inspect coverage, and open digest-bound original coordinates.
   Select a saved result and its original, then **Reimport and pin new baseline**.
   Choose the immutable baseline and a saved target/original to verify and compare.
   Reload the case after an external mutation or canceled mutation whose commit
   may already have completed. Existing-output/source bytes are never rewritten.

## Canonical profile queries

`profile-query --in result.json --query JSON` and QueryProfileEvidence use the
same ProfileSet math as tree/diff/drilldown/export. A query pins exact selection,
result digest, view, frame/text/window filters, limit and cursor. Result/query
changes invalidate cursors. Responses expose population/selection, exact total,
metric/unit/scale, availability/quality and format-specific original references.
All selected observations contribute to the denominator; text/frame/view filters
only filter displayed rows. Caller/callee neighbors and recursive frame totals
count each selected observation once per distinct frame/neighbor. Tree values
retain path-specific inclusive/self weights.

Time windows apply only to evidence starting timestamps, using exact declared
clock scale and half-open `[from,to)` bounds. Intervals are not clipped and the
selected denominator remains unchanged. Mixed clock kind/unit/scale refuses.
Untimed observations are counted as unavailable for the window; sampled chronology
is never invented. No offsets, correlation or time alignment are inferred.

Paged flamegraph geometry is computed from the full selected tree, so omitted
nodes retain their space. Layout coordinates are bounded approximate fractions;
labels and exported/query values remain exact decimal strings. Zero totals and
signed metrics have no invented positive area. A page has at most 200 rows and
1 MiB JSON. Display labels use a 512-byte UTF-8 prefix, evidence path text about
4 KiB; searches over labels share that disclosed prefix. Canonical IDs/full model
and totals are unchanged. Existing model/tree/reference limits still apply.

```sh
archscope-engine profile-query --in result.json --query '{"selection":{"profile_ids":["profile-0"],"metric_id":"sampled-seconds"},"view":"evidence","limit":25}'
archscope-engine case-query search --store /path/to/cases --request search.json
archscope-engine case-query pin-baseline --store /path/to/cases --request pin.json
archscope-engine case-query compare-baseline --store /path/to/cases --request compare.json
```

## Search index and source links

Search requests require `case_id`, current `revision`, literal `text`, `limit`
(1–200), optional exact `source_id`, `result_id`, `field`, and cursor. Source
`text`, `/summary/<key>` and `profile.observation` fields support IDs/digests and
sanitized text. Typed profile observations reuse the canonical evidence visitor,
including frame IDs, profile IDs, source references and scaled timestamps. Search
index/query version 1.1.0 supports optional `time_from`, `time_to`, `clock` and
`time_unit`; windows require one explicit result_id to avoid aligning relative
clocks across captures. Untimed rows are counted, not assigned timestamps. Arbitrary
log fields/timestamps are not inferred from unparsed text.

The disposable index under the case store's `indexes` directory pins indexer (1.1.0),
redaction, manifest revision, source availability/digests and result digests. Each
query checks original bytes, even on cache hits, and result object integrity.
Missing/corrupt indexes rebuild; cleanup/restart cannot erase snapshots or pins.
Derived text is masked by the separate index-mask 0.2.0 policy before persistence, then bounded
to 2 KiB per snippet. That policy covers known patterns, not every possible secret;
sanitize other sensitive input before intake. Structured IDs/digests/coordinates
remain separate from masked snippets. This is not a new producer parser.

A case index holds at most 100,000 rows / 16 MiB and source indexing reuses the
existing one-million-line bounded prefix. Per-source/result coverage exposes
indexed/total and missing/decoder/prefix/budget reasons. Snippet truncation is
explicit; text outside prefixes is not silently claimed searchable. Unvisited
sources after a global budget report `index_budget` with unknown total (0), not
complete empty coverage. Source pages retain original byte/decoded offsets and
at most 10 lines when opened here. Out-of-prefix links are refused. Format-specific
pprof/Speedscope references remain structural coordinates, never fabricated text
line numbers. Case/result/source/query/index changes invalidate search cursors.

## Baseline verification and mapping

Pin request: `{case_id,revision,result_id,source_id}`. The source must already be
bound to the saved ResultRef and its hash must equal the embedded model's original
hash. Only original-source profile payload 0.4.0 can currently be replayed; older
snapshots remain readable and need explicit reimport for a qualified new pin.
The backend validates the result, reruns the fixed source normalizer from bounded
original bytes and reconstructed sanitized options, then compares the entire exact
model. Derived source bounds/period fields are recomputed, never trusted from the
request. Present saved selection/options-hash assertions must match reconstruction;
options digest is canonical SourceOptions JSON, not a legacy job-selection hash.

The existing Baseline.conditions JSON stores `profile-baseline` 1.0.0: immutable
result/source/model/options hashes, selection, comparison identity, capture/clock
hashes and compiled policy snapshot including both semantic and incidental option
projections. The context remains bounded to 64 KiB. Imported/edited “verified”
claims grant no capability. A new query repeats original reimport for baseline and target. A process-local
cache (two entries / 128 MiB total / two-minute expiry) can reuse that validated
difference for subsequent pages. Every page checks original/result byte hashes
and the complete case/query/policy binding again and
only then returns `source_verification=verified_by_reimport`. This verifies the
model against those bytes/options, not the publisher, capture authenticity or
process lifetime. Comparison `identity_basis` stays `declared`.

Compare request: `{case_id,revision,baseline_id,target_result_id,target_source_id,
mapping?,context_mapping?,limit?,cursor?}`. Rows are paged (default 50, max 200,
1 MiB response), with exact delta/share/percentage semantics. Canonical T-634
producer/format/version/metric policies govern incidental changes and disclose
both original options. Old generic scalar comparison deliberately remains strict;
the new canonical/baseline UI is the qualified policy consumer.

Entity aliases require a complete one-to-one mapping of selected current tuples:
`{baseline:{id,process,thread,worker},target:{id,process,thread,worker},reason}`.
Only a private comparison copy is mapped; response references describe original
models/options. Stale tuples, collisions and partial mappings refuse. Distinct
capture contexts additionally require `{baseline_sha256,target_sha256,reason}`
bound to both capture hashes. The desktop mapping box accepts an entity array or
`{entities:[],capture:{...}}`. Full capture-clock hashes must still match; explicit
clock alignment is not implemented. Metric/selection/quality/semantic capture
conditions and unknown/no-policy strict equality cannot be bypassed by mapping.

Missing/changed originals, stale sources/encodings/results, edited model or option
claims, changed policy/normalizer or binding reject verification; they do not
rewrite historical pins or upgrade saved hashes. Portable restore preserves pins
but supplies no verification authority: current originals must replay successfully
again. Native Windows/UI acceptance and independent review are separate from
local backend/state/build/cross-build evidence.

## R12 query refinements

Top queries accept `sort: "self" | "inclusive"`, bound into the query hash and
cursor. Desktop Top defaults to self; omitted API sort preserves inclusive
ordering. Search keeps the selected denominator unchanged.

Index-mask 0.2.0 adds common password/secret/token/api-key assignments, JWT and
AWS access-key patterns and stable Authorization placeholders. Policy version
and content digest invalidate old disposable indexes. Legacy snapshot/capture
text-mask 0.1.0 remains unchanged. Only known patterns are covered; inspect input
and snippets before sharing. Source bytes and saved results are never rewritten.

The verified comparison cache lives only in the current service process, is
never loaded from case receipts or disk, and releases on shutdown. A cache miss
or expiry replays originals. New CLI processes remain stateless and perform
reimport for each command. Responses expose cache_hit and verification_scope;
identity_basis remains declared. This cache avoids repeated normalization and
full diff calculation, but still copies a bounded diff and hashes source/result
bytes each page. It does not remove all large-input costs.

Case results are limited to 67,108,864 bytes (64 MiB). High-cardinality profiles
can produce larger results even from much smaller originals. Save/pin is then
unavailable. Errors report actual bytes and limit plus sample count when known;
the pre-decode boundary explicitly reports sample count unavailable. CLI uses
file size before bounded reading. Select fewer profiles/metrics or shorten the
capture, then create a new result. Do not silently coalesce observations or raise
the ceiling. Native WebView/Windows and independent review remain separate.
