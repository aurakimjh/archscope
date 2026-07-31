# X-RG1 — HTTP × CPU/Jennifer/Access-Log Correlation Re-Review (T-583)

- **Review group:** X-RG1 (HTTP evidence correlation — Codex engine, generated
  bindings, Claude drilldown/overlay UI)
- **Target task:** T-583
- **Reviewer:** claude-code, acting as the independent reviewer for this group.
  Disclosure: as in the first X-RG1 review, the Claude UI half of this group is
  the same agent family's work. Every claim below is backed by an executed
  command or an executed probe against the shipped code, not by reading the
  remediation commit messages or the plan's completion text. No code, design
  document, or `work_status.md` entry was modified by this review.
- **Date:** 2026-07-31
- **Verdict:** `CONDITIONAL` — **B2 is fully fixed and B1 is substantially
  fixed**, both with engine regressions that hold under independent execution;
  the renderer half of B1 is thorough and well-pinned, and the two UI-scope
  observations (O3, O5) are genuinely closed. But the B1 remediation left a
  **state-leak in `correlateAccess` that reproduces the original defect through
  a narrower door**: loop-local `bestDelta` / `bestClockCompared` are not reset
  when a later request-ID record wins, so an emitted row can carry a
  measured-looking `timestamp_delta_ms` **taken from a different access-log
  record than the one it was paired with**, stamped `aligned`, with the source
  certified `overlay_allowed: true` (B3). This is the same class of unearned
  alignment claim B1 named, and it is blocking on the same reasoning the first
  review used to block B2. T-583 stays `IN_PROGRESS`; remediate B3, then
  re-review. Five new non-blocking observations (O6–O10) are recorded; O1, O2,
  and O4 from the first review remain open.
- **Evidence base:** commits `fbc1a7d` (Codex engine remediation) and `c3383bd`
  (Claude renderer remediation) on top of `7003ab9` / `e9be104`;
  `internal/analyzers/httpcorrelation/analyzer.go` and `analyzer_test.go`;
  `internal/analyzers/accesslog/analyzer.go` and
  `internal/parsers/accesslog/parser.go` (reachability);
  `internal/parsers/profile/v8.go`; `cmd/archscope-app/engineservice.go`;
  `frontend/src/state/httpCorrelation.ts`,
  `components/HttpCorrelationPanel.tsx`, `bridge/types.ts`,
  `pages/AnalysisWorkspacePage.tsx`, `i18n/messages.ts`,
  `state/regression.test.ts`; the paired EN/KO implementation plans §7 X-RG1.

## Scope of this re-review

The first review's stated re-review scope was narrow: B1 and B2 remediated with
engine regressions pinning the new fail-closed behaviour, the matching renderer
handling for an unavailable timestamp delta, and re-runs of the three suites.
O1–O5 did not gate the verdict.

This review executed that scope and, because B1 changed the shape of
`correlateAccess` materially, re-probed the whole access-log path rather than
only the two cases the first review named. B3 was found that way.

## Verification performed

Run from `apps/engine-native` and `cmd/archscope-app/frontend` on darwin/arm64
(macOS 26.0, Go 1.26.3, Node 22). As before, this is not a Windows host;
Windows-specific live-capture behaviour is out of X-RG1 scope and untouched by
either remediation commit. R-RG1 still owes the native Windows acceptance run.

| Command / check | Result |
|---|---|
| `go build ./...` | pass (macOS linker version warnings only) |
| `go vet ./cmd/archscope-app/... ./internal/analyzers/httpcorrelation/...` | clean |
| `go test ./...` | pass — 75 packages ok, zero failures |
| `npm run test:state` | pass (exit 0), including the new B1/O3/O5 regressions |
| `npm run build` (tsc + vite production) | pass; panel still a lazy chunk (`HttpCorrelationPanel-ptQ72CB3.js`, 59.42 kB) |
| Generated bindings | unchanged and correct: the new fields live on `map[string]any` rows, not on a Go struct, so no binding regeneration was owed. `bridge/types.ts` is a hand-written mirror (no DO-NOT-EDIT header) and was updated to match |
| Ownership boundary | `fbc1a7d` touches only engine `.go` + docs; `c3383bd` touches only frontend + docs. The Codex/Claude split held |
| Paired docs | EN §7 `X-RG1 Review — Conditional / Backend Remediation Complete` and KO `X-RG1 리뷰 — 조건부 / 백엔드 보완 완료` both present, with the same §4 group-table row. No drift found |
| Working tree | clean at review time |

Six adversarial probes (A–F) were run against the public
`engineapi.AnalyzeHttpEvidenceCorrelation` entry point from a scratch module
outside the repo (no repo file created or modified). Their raw output is the
evidence for B3 and for O6–O10 below.

## Previous findings — disposition verified

### B1 — Access-log matches assert an unmeasured clock alignment → **substantially fixed** (residual: B3)

Verified fixed, independently:

- The request-ID path no longer fabricates `bestDelta = 0`. It compares
  timestamps only when both sides actually have them, and records the outcome
  in a new `clock_compared` flag (`analyzer.go:402-410`).
- Row-level `alignment_grade` is now derived (`analyzer.go:435-438`): `aligned`
  only when `bestClockCompared && bestDelta <= toleranceMS`, else
  `duration_only`.
- `timestamp_delta_ms` is emitted only when a comparison happened; otherwise the
  row carries `timestamp_delta_unavailable_reason` and no delta key at all
  (`analyzer.go:448-456`).
- Source-level grade and `overlay_allowed` are derived from the rows via
  `allRowsAligned` (`analyzer.go:811-821`), which requires **every** emitted row
  to be both `aligned` and `clock_compared: true`. The old "identity **or**
  compatible timestamps" reason string is gone, replaced by two distinct,
  accurate reasons.
- The sort comparator no longer coerces a missing delta to zero
  (`analyzer.go:471-481`).

Probe B (the first review's own six-hour-skew case, executed):

```
"match_basis": "request_id", "confidence": "high",
"alignment_grade": "duration_only", "clock_compared": true,
"timestamp_delta_ms": 21600000,
"timestamp_alignment_reason": "request identity matched, but the absolute timestamp delta exceeds the configured tolerance"
```

and the source diagnostic:

```
"source": "access_log", "alignment_grade": "duration_only",
"overlay_allowed": false,
"reason": "request identity paired client and server observations, but at least one pair lacked compatible absolute timestamps"
```

The engine now measures rather than asserts. `TestAnalyzeRequestIdentityDoes
NotClaimUnmeasuredClockAlignment` pins both the large-skew and the
missing-client-timestamp cases including the summary counters.

The renderer half is thorough and, in my judgement, better than the first
review asked for. `accessClockComparison` (`httpCorrelation.ts:281-287`) fails
closed to unavailable if the flag, the delta, or its finiteness disagree — so a
malformed row cannot render as measured. Both the match table's new
`Timestamp Δ` column and the drilldown print `—` with the engine's reason.
`AccessCard` discloses identity-only pairing above the table when the source is
not certified aligned, and `correlationPrimaryAlignment` puts the primary HTTP
timeline's own grade next to the three counters, which answers the first
review's point that `countAlignment` excludes the HTTP source. Seven executed
assertions in `regression.test.ts` pin this, including the two fail-closed
inconsistency cases and the non-finite delta.

The structural invariant the first review wanted also now holds by
construction, not just by disclosure: an access-log row can only be
`clock_compared` when its transaction has a parseable timestamp, so a primary
HTTP timeline with no parseable timestamps can no longer sit under an `aligned`
secondary.

**Residual:** see B3. The delta a row reports can still belong to a different
record than the row was paired with.

### B2 — CPU overlay certifies `aligned` on a silently defaulted V8 base → **fixed**

Verified fixed. `correlateProfile` now resolves the base through
`microseconds()` and fails closed when it is absent or non-numeric
(`analyzer.go:249-253`), exactly as required; `start_us` / `end_us` get the same
treatment (`analyzer.go:256-259`). `optionalNumber` was additionally hardened to
reject NaN and ±Inf (`analyzer.go:728-731`), which closes a route the first
review did not name.

Probe C (valid RFC3339 anchor, `cpu_sample_runs` present, no
`v8_start_time_us`), executed:

```
"http_profile_overlaps": []
"alignment_diagnostics": [... { "source": "profile_evidence",
  "alignment_grade": "none", "confidence": "none", "overlay_allowed": false,
  "reason": "profile metadata is missing a valid numeric parser_metadata.v8_start_time_us timestamp base" }]
"findings": [{ "code": "HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE",
  "message": "... no compatible wall-clock anchor and V8 timestamp base were available." }]
```

`TestAnalyzeSuppressesProfileOverlayWithoutV8TimestampBase` covers both the
missing and the non-numeric sub-case and asserts on the finding message text.
The finding message was correctly widened to name the timestamp base.

The first review's optional suggestion — refusing envelopes whose profile
format is not V8/Chrome — was not taken. That is a legitimate call: the
`v8_start_time_us` requirement is itself the V8-specific gate, and a non-V8
envelope cannot satisfy it. Not charged.

### O3 — Contract adoption does not check `store_or_file_rescan` → **closed**

`isCorrelationContractSupported` (`httpCorrelation.ts:158-178`) now also
requires `store_or_file_rescan === false` and cross-checks the contract's
`alignment_grades` and `confidence_grades` against the renderer's closed token
sets, so a contract advertising a vocabulary the renderer cannot name is
refused rather than rendered as `unknown`. Three executed assertions pin it.
The rationale comment above the function is accurate.

### O5 — Workspace mount reuses the Diff candidate selector → **closed**

`AnalysisWorkspacePage.tsx:129` now gates on
`correlationCandidates(workspace.entries, "http")`. The related `topN` point is
also closed: a contract-bounded Top-N input was added, `correlationTopNState`
validates it against `max_top_n` before the run, and `topN` joined
`HttpCorrelationInputs` so changing it drops the rendered result — the
provenance invariant is preserved rather than widened. Executed assertions
cover the bounds and the drop.

### O1, O2, O4 → **still open** (engine-scope, correctly declared as such)

Re-confirmed by probe, not assumed:

- **O1** — probe E: a 10 ms HTTP transaction still pairs with a 90,000 ms
  Jennifer edge (`duration_delta_ms: 89990`, `confidence: "low"`,
  `alignment_grade: "duration_only"`). No overlay, so nothing false is drawn.
- **O2** — probe F: an access record for `https://other-server.internal/api`
  still matches a transaction on `api.example.com` via
  `method+path_template+status+time`. Note this observation is now *more*
  consequential than when it was written: the cross-host row is graded
  `aligned` and, because it is the only row, the source is certified
  `overlay_allowed: true`. O2 is still non-blocking (the pairing is a
  disclosed, bounded diagnostic and the basis token is shown), but it should be
  raised in priority on the next change to `sameRequestShape` rather than left
  indefinitely.
- **O4** — the greedy per-transaction loop with `claimed` is unchanged, so
  pairing remains order-dependent and undisclosed.

## Blocking finding

### B3 — A request-ID row can report a timestamp delta measured against a different access-log record

This is the residual of B1. `correlateAccess` (`analyzer.go:393-423`) hoists
`bestDelta` and `bestClockCompared` outside the record loop, but the request-ID
branch assigns only `bestIndex` and `bestBasis` before `break`:

```go
bestIndex := -1
bestDelta := math.MaxFloat64
bestBasis := ""
bestClockCompared := false
for index, record := range used {
    if claimed[index] { continue }
    requestID := text(record["request_id"])
    if transaction.RequestID != "" && requestID != "" && strings.EqualFold(transaction.RequestID, requestID) {
        bestIndex, bestBasis = index, "request_id"          // <- bestDelta / bestClockCompared NOT reset
        if recordTime, err := time.Parse(...); err == nil && transaction.HasTime {
            bestDelta = ...
            bestClockCompared = true
        }
        break
    }
    if !sameRequestShape(transaction, record) || !transaction.HasTime { continue }
    ...
    if delta <= toleranceMS && delta < bestDelta {
        bestIndex, bestDelta, bestBasis = index, delta, "method+path_template+status+time"
        bestClockCompared = true                            // <- set here, never cleared
    }
}
```

If a shape-matching record appears **earlier** in the table and sets
`bestDelta` / `bestClockCompared`, and a request-ID record then wins **later**
without a parseable timestamp of its own, the row is emitted with the
*previous* record's delta and the *previous* record's comparison flag.

Probe A, executed. One transaction (`req-1`, `2026-07-30T01:00:00Z`), two
access records: index 0 is a shape match 50 ms away with
`response_time_ms: 999`; index 1 is the request-ID match with an unparseable
timestamp and `response_time_ms: 80`.

```
"match_basis": "request_id", "clock_compared": true,
"timestamp_delta_ms": 50, "alignment_grade": "aligned",
"server_response_ms": 80, "access_uri": "/api", "request_id": "req-1"
```

`server_response_ms: 80` proves the row was paired with record **index 1**;
`timestamp_delta_ms: 50` was computed against record **index 0**. The row mixes
two records' evidence. The consequences propagate exactly as B1's did:

```
"source": "access_log", "alignment_grade": "aligned",
"confidence": "high", "overlay_allowed": true,
"reason": "every emitted access-log match compared compatible absolute timestamps within the configured tolerance"
```

```
"aligned_source_count": 1, "duration_only_source_count": 0, "incompatible_source_count": 0
```

The reason string is false for this result — the emitted match compared no
timestamps at all. The renderer then faithfully prints a measured
`Timestamp Δ 50.0ms`, a green `Aligned` badge, "time overlay enabled", and
suppresses the identity-only note (because the source *is* certified aligned).
Every fail-closed check the renderer gained in `c3383bd` passes, because the
row is internally self-consistent — it is just attributed to the wrong record.
This is an alignment claim the engine did not earn, and a provenance violation:
the row does not describe a single pairing.

**Reachability, stated fairly.** Like B2 before it, this is latent rather than
observed in the current in-product path. Every access-log record constructor in
`internal/parsers/accesslog/parser.go` sets `Timestamp` from a parse that hard-
fails the line on error (`ReasonInvalidTimestamp`), and
`internal/analyzers/accesslog/analyzer.go:481` serialises it with
`Format(time.RFC3339Nano)`, so a record reaching the correlator through
archscope's own access-log analyzer always has a parseable timestamp. The other
route into the stale state — `!transaction.HasTime` — cannot reach it, because
the shape branch is skipped in that case and the flag stays false.

It is blocking on the same three grounds the first review used for B2, all of
which still apply: the analyzer's input is an arbitrary caller-supplied
`map[string]any` rather than a typed in-process value; the contract advertises
the no-unearned-alignment behaviour as a guarantee; and the group's PASS
criterion is specifically that alignment is never claimed without measurement.
It is also strictly worse than B2 in kind — B2 produced a *wrong overlay*, this
produces a *wrong overlay plus a misattributed measurement*.

**Required:** reset `bestDelta` and `bestClockCompared` whenever `bestIndex`
changes. The minimal shape is to set `bestDelta = math.MaxFloat64` and
`bestClockCompared = false` in the request-ID branch before the conditional
comparison. A structurally safer alternative is to compute the delta once,
after the loop, from `used[bestIndex]` alone, so a row's delta cannot come from
any record other than the one it was paired with. Add an engine regression
covering probe A's shape: an earlier in-tolerance shape-matching record plus a
later request-ID record with no usable timestamp must emit
`clock_compared: false`, no `timestamp_delta_ms`, `alignment_grade:
"duration_only"`, and an access source that is not `overlay_allowed`.

## Non-blocking observations (new)

- **O6 — `confidence` and `alignment_grade` share a vocabulary but not a
  meaning.** A request-ID row is stamped `confidence: "high"` regardless of the
  clock outcome, so probe B's six-hour-skew row reads `duration_only` / `high`,
  and `strongestConfidence` propagates `high` to the source diagnostic beside a
  `duration_only` grade. Pairing confidence and clock confidence are different
  axes; the diagnostics card renders them adjacently with the same closed label
  set. The `httpCorrelationAccessIdentityOnlyNote` above the table mitigates
  this, but a reader scanning the diagnostics card alone sees "Access log —
  Duration only — High". Consider naming the field `match_confidence`, or
  scoping the label so it cannot be read as confidence in the alignment.
- **O7 — Whole-source overlay gating may make the access overlay practically
  unreachable.** `allRowsAligned` requires *every* candidate row — including
  rows that `topN` will truncate away — to be aligned before the source is
  certified. On a realistic capture, a single identity-only pairing among
  hundreds suppresses the overlay for all the genuinely measured rows. The
  direction is correct (fail closed) and I would not change it for a per-source
  verdict, but per-row overlay gating would be equally safe and considerably
  more useful. At minimum, the diagnostic could report how many rows were
  `clock_compared` so the reader can see *why* the source was downgraded.
- **O8 — Both new reason strings hardcode "request identity matched".** The
  `switch` at `analyzer.go:448-456` is basis-independent, so
  `timestamp_alignment_reason` would claim request identity on a shape-basis row
  too (today unreachable, since a shape match is always within tolerance and
  therefore `aligned` — but it is one tolerance change from being wrong).
  Separately, `timestamp_delta_unavailable_reason` says "**both** observations
  did not provide parseable absolute timestamps" when the actual condition is
  that *at least one* side did not; that text reaches the user verbatim in the
  drilldown's `Clock comparison` field.
- **O9 — Silently dropped source rows are not disclosed.** `cpu_sample_runs`
  rows with non-integer or unparseable `start_us`/`end_us` are skipped
  (`analyzer.go:256-260`), as are Jennifer edges lacking a gap or a resolvable
  host (`analyzer.go:325-333`), but `rows_used` still reports the bounded input
  count and no counter distinguishes "used" from "usable". This sits oddly next
  to the group's otherwise careful `source_truncated` / `output_truncated` /
  `candidate_rows` disclosure, and it means a profile source can be certified
  `aligned` while most of its runs were discarded. A `rows_skipped` field would
  close it.
- **O10 — `timestamp_delta_ms` saturates instead of failing closed.** The delta
  is computed as `recordTime.Sub(transaction.Start)`, and `time.Duration`
  saturates at ±2^63 ns (~292 years). Probe D, executed with an access record
  timestamped `0001-01-01T00:00:00Z` (the zero `time.Time` a Go producer
  serialises), emits `"timestamp_delta_ms": 9223372036854.775` — a saturation
  sentinel presented as a measurement, with `clock_compared: true`. The grade
  correctly falls to `duration_only`, so nothing is overlaid, and archscope's
  own access-log parser cannot produce this (it rejects unparseable
  timestamps). Worth a bounded-delta guard when B3 is addressed, since both
  live in the same expression.

## Known limitations recorded, not charged against this group

- The isolated resolver-cost measurement remains owed at R-RG1 (H-RG4 V3
  deferral).
- The H-RG4 O1/O2 and H-RG5 O1–O4 observations remain open as optional
  hardening on the next change to those surfaces.
- This review ran on macOS. R-RG1 still owes the native Windows acceptance run.

## Re-review scope

A further re-review needs only: B3 remediated with the engine regression
described above, plus re-runs of `go test ./...`, `npm run test:state`, and
`npm run build`. No renderer change is required for B3 — the existing
`accessClockComparison` fail-closed path renders the corrected row correctly
once the engine stops misattributing the delta. O1, O2, O4, and O6–O10 do not
gate the verdict.
