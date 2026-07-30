# X-RG1 — HTTP × CPU/Jennifer/Access-Log Correlation Group Review (T-583)

- **Review group:** X-RG1 (HTTP evidence correlation — Codex engine, generated
  bindings, Claude drilldown/overlay UI)
- **Target task:** T-583
- **Reviewer:** claude-code, acting as the independent reviewer for this group.
  Disclosure: the Claude UI half of this group (commit `e9be104`) was
  implemented by the same agent family in an earlier session; the engine half
  (commit `7003ab9`) is Codex's. As in H-RG4 and H-RG5, every claim below is
  backed by an executed command or an executed probe against the shipped code,
  not by reading the handoff notes. No code, design document, or
  `work_status.md` entry was modified by this review.
- **Date:** 2026-07-31
- **Verdict:** `CONDITIONAL` — the bounded-input, no-rescan, no-causality, and
  contract-adoption machinery all hold under independent execution, and the
  V8/Jennifer clock handling fails closed exactly as the plan requires. But the
  plan's own X-RG1 review criterion — *"never claim causality across
  incompatible clocks; always show the alignment grade and evidence
  provenance"* — is violated on two paths that were not covered by any test:
  the access-log source reports a **measured-looking `timestamp_delta_ms` of 0
  and an `aligned` grade for pairings where no clock was ever compared** (B1),
  and the CPU-overlap path **certifies `aligned` with a silently defaulted V8
  base timestamp** (B2). Both are affirmative alignment claims the engine did
  not earn. T-583 stays `IN_PROGRESS`; remediate B1 and B2, then re-review.
  Five non-blocking observations (O1–O5) are recorded.
- **Evidence base:** commits `7003ab9` (Codex backend) and `e9be104` (bindings
  + Claude UI); `internal/analyzers/httpcorrelation/analyzer.go` and
  `analyzer_test.go`; `api/api.go`; `cmd/archscope-app/engineservice.go` and
  `engineservice_test.go`; the regenerated bindings under
  `frontend/bindings/...`; `frontend/src/state/httpCorrelation.ts`,
  `components/HttpCorrelationPanel.tsx`, the HttpCapturePage /
  AnalysisWorkspacePage integrations, `i18n/messages.ts`, and
  `state/regression.test.ts`; the paired EN/KO implementation plans §7 X-RG1.

## Scope

The plan (§7, X-RG1) requires:

- **Codex:** bounded session/profile alignment, Jennifer `NETWORK_GAP` checks,
  access-log client/server comparison, confidence, and mismatch diagnostics.
- **Claude:** drilldown and overlays between HTTP transactions, CPU runs, and
  server evidence in the same time window.
- **Review (the PASS criterion):** never claim causality across incompatible
  clocks; always show the alignment grade and evidence provenance.

## Verification performed

Run from `apps/engine-native` and `cmd/archscope-app/frontend` on darwin/arm64
(macOS 26.0, Go 1.26.3, Node 22). Note that this is not the Windows host the
H-RG4/H-RG5 reviews used; Windows-specific live-capture behaviour is out of
X-RG1 scope and unchanged by either commit, but the R-RG1 acceptance run still
owes the native Windows pass.

| Command / check | Result |
|---|---|
| `go build ./...` | pass (macOS linker version warnings only) |
| `go vet ./cmd/archscope-app/... ./internal/analyzers/httpcorrelation/...` | clean |
| `go test ./...` | pass — 75 packages ok, zero failures |
| `go test -count=1 ./internal/analyzers/httpcorrelation/... ./cmd/archscope-app/...` | pass, uncached |
| `npm run test:state` | pass (exit 0), including the new correlation regressions |
| `npm run build` (tsc + vite production) | pass; the panel is a lazy chunk (`HttpCorrelationPanel-BHb2N1kB.js`, 56.69 kB) |
| Binding provenance | `frontend/bindings/.../engineservice.ts` and `.../httpcorrelation/models.ts` carry the generator's DO-NOT-EDIT header and match the Go structs; no hand-edited binding found in `e9be104` |
| Ownership boundary | `7003ab9` touches no frontend source; `e9be104` touches no engine `.go` source. The plan's Codex/Claude split held. |
| Paired docs | EN and KO plans both carry the X-RG1 completion text and the same §4 group-table row; no drift found |

Beyond the shipped tests, four adversarial probes were run against the public
`engineapi.AnalyzeHttpEvidenceCorrelation` entry point from a scratch module
outside the repo (no repo file created or modified). Their raw output is the
evidence for B1, B2, and O1 below.

## What holds

These are the parts the review confirms, independently of the handoff notes:

1. **No source or store rescan.** `Analyze` consumes only `AnalysisResult`
   envelopes; `store_or_file_rescanned: false` appears in both the summary and
   `metadata.http_evidence_correlation`, and the renderer surfaces it. The
   Wails request carries result maps, not paths.
2. **Input validation and the secondary-source minimum.** A non-`http_capture`
   primary is rejected with an explicit type error, and HTTP alone is refused
   by both the engine and the reducer (`hasCorrelationSecondary`).
3. **Missing-anchor fail-closed for V8.** With no `profileWallClockStart`, the
   profile source grades `none`, `overlay_allowed: false`, emits zero overlap
   rows, and raises `HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE`. Verified by
   the shipped `TestAnalyzeSuppressesProfileOverlayWithoutWallClockAnchor` and
   reproduced independently.
4. **Jennifer stays duration-only.** Date-less ms-since-midnight edges always
   grade `duration_only` with `overlay_allowed: false`, and the renderer prints
   an explicit "compares durations only and proves no temporal overlap" note
   above the table in both locales.
5. **Renderer never judges clocks itself.** `correlationOverlayAllowed` demands
   the backend's `overlay_allowed === true` **and** a grade resolving to
   `aligned`; an unrecognized grade resolves to `unknown` and fails closed.
   Pinned by four executed assertions in `regression.test.ts`.
6. **Contract adoption rejects causal contracts.** The renderer refuses any
   contract whose `schema_version`, `result_type`, or `http_result_type`
   differs, and — correctly — any contract declaring
   `causal_claims_allowed: true`. Pinned by executed assertions.
7. **Race-safe provenance.** The reducer drops the rendered result on any slot,
   anchor, or tolerance change, and a completion that raced past an input change
   never renders. Pinned by executed assertions.
8. **No-causality disclosure and closed token maps.** Every result surface and
   the drilldown repeat the disclosure; grades, confidence, match bases, and
   sources resolve through closed EN/KO maps with the raw token on hover, and
   both locales carry every new key (executed both-locale coverage assertion).
9. **Bounded output with disclosure.** Source rows are capped at 2,000 and
   output at `topN` (default 50, max 500), with `source_truncated`,
   `output_truncated`, `candidate_rows`, `rows_used`, and `output_rows`
   rendered per source.

## Blocking findings

### B1 — Access-log matches assert an unmeasured clock alignment

`correlateAccess` (`analyzer.go:378-449`) has two match paths. The request-ID
path sets `bestDelta = 0` and `bestBasis = "request_id"` without ever comparing
timestamps (`analyzer.go:397-401`); that zero is then emitted verbatim as the
row's `timestamp_delta_ms` (`analyzer.go:429`). Independent of the basis, the
row is stamped `alignment_grade: "aligned"` (`analyzer.go:430`), and if any
match exists the whole source is stamped `aligned`, `overlay_allowed: true`,
with the reason "request identity **or** compatible absolute timestamps align
client and server observations" (`analyzer.go:434-439`).

Probed with a client transaction at `01:00:00Z` and an access record for the
same request ID at `07:00:00Z` — a six-hour server clock skew:

```
"match_basis": "request_id", "confidence": "high",
"alignment_grade": "aligned", "timestamp_delta_ms": 0
```

and the source diagnostic:

```
"source": "access_log", "alignment_grade": "aligned",
"confidence": "high", "overlay_allowed": true,
"reason": "request identity or compatible absolute timestamps align client and server observations"
```

The renderer faithfully prints this: the drilldown shows `Timestamp Δ 0.0ms`
(`HttpCorrelationPanel.tsx:833-836`) and the diagnostics card shows a green
`Aligned` badge plus "time overlay enabled". A reader is told the two clocks
agree to within a millisecond when the engine measured nothing at all. That is
precisely the alignment claim across incompatible clocks the X-RG1 gate
forbids.

A second probe makes it sharper: an HTTP transaction with **no parseable
timestamps whatsoever**, matched by request ID, still produces
`alignment_grade: "aligned"` and `overlay_allowed: true` for the access-log
source, while the primary HTTP source is honestly graded `duration_only`. The
summary tiles then read `Aligned sources: 1 / Duration-only: 0 / Incompatible:
0`, because `countAlignment` (`analyzer.go:746-754`) excludes the HTTP source
from all three counters — so the one grade that would warn the reader is
invisible in the headline metrics.

**Required:** an identity-based pairing must not be reported as clock
alignment. Suggested shape — emit a distinct grade or an
`identity_only`/`clock_not_compared` marker for request-ID rows; omit
`timestamp_delta_ms` (or set an explicit unavailable reason) when no timestamp
comparison happened; derive the source-level `alignment_grade` and
`overlay_allowed` only from rows that actually compared absolute timestamps;
and split the "or" in the diagnostic reason so it states which mechanism was
used. The renderer must render the unavailable delta as `—`, and the summary
tiles should not report a source as aligned when the primary HTTP timeline is
not.

### B2 — The CPU overlay certifies `aligned` on a silently defaulted V8 base

`correlateProfile` maps V8 monotonic microseconds onto wall time as
`anchor + (start_us − v8StartUS)` (`analyzer.go:249-259`), where `v8StartUS`
comes from `metadata.parser_metadata.v8_start_time_us`. `number()` returns `0`
for a missing key, so an envelope without that field is silently treated as
having a V8 base of zero — the anchor is applied to raw, unrebased profile
timestamps. Unlike the missing-*anchor* case, nothing fails closed.

Probed with a valid RFC3339 anchor and a `profile_evidence` envelope carrying
`cpu_sample_runs` but no `v8_start_time_us`:

```
"http_profile_overlaps": [{ "overlap_ms": 200, "overlap_ratio": 1,
  "confidence": "high", "alignment_grade": "aligned",
  "match_basis": "explicit_wall_clock_anchor+time_overlap",
  "cpu_started_at": "2026-07-31T01:00:00.1Z" }]
"alignment_diagnostics": [... { "source": "profile_evidence",
  "alignment_grade": "aligned", "overlay_allowed": true,
  "reason": "explicit profile wall-clock anchor maps V8 monotonic timestamps to HTTP wall time" }]
"findings": []
```

A 100 % overlap at `high` confidence is emitted, the source is certified
`aligned`, and `TimeOverlapBlock` (`HttpCorrelationPanel.tsx:892-952`) renders
the time-window chart — all from a mapping whose second term was invented.

Reachability, stated fairly: in the current in-product path this does not fire.
`cpu_sample_runs` is emitted only for `ValueUnit == "microseconds"`
(`analyzers/profile/analyzer.go:98-101`), which only the V8 parser produces
(`parsers/profile/v8.go:353`), and both V8 construction paths always set
`v8_start_time_us` (`v8.go:245`, `v8.go:344`). So this is a latent, not an
observed, mis-overlay. It is still blocking because the analyzer's input is an
arbitrary caller-supplied `map[string]any` from the Workspace rather than a
typed in-process value, the contract advertises
`requires_profile_wall_clock_anchor: true` as a fail-closed guarantee, and the
group's whole design principle is that a missing clock basis suppresses the
overlay. Half of the mapping fails closed; the other half defaults to zero.

**Required:** treat a missing or non-numeric `v8_start_time_us` the same as a
missing anchor — grade `none`, `overlay_allowed: false`, zero rows, and a
`HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE` finding naming the absent base —
and add an engine regression for it. Consider also refusing envelopes whose
profile format is not a V8/Chrome profile, since the anchor's semantics are
V8-specific.

## Non-blocking observations

- **O1 — Jennifer nearest-duration matching is unbounded.** `correlateJennifer`
  (`analyzer.go:332-345`) picks the nearest host-matching transaction by
  duration with no rejection threshold. Probed: a 10 ms HTTP transaction was
  paired with a 90,000 ms Jennifer edge, emitting a row with
  `duration_delta_ms: 89990` and `confidence: "low"`. The grade is
  `duration_only` and the overlay is suppressed, so nothing false is overlaid,
  but the row is still presented as a "check". Consider rejecting matches
  beyond a bounded multiple of the tolerance, or marking them `unmatched` with
  a reason.
- **O2 — The access-log shape match never compares the server host.**
  `sameRequestShape` (`analyzer.go:527-532`) builds the record's endpoint
  template from `transaction.Host`, so the record's own host is structurally
  excluded from the comparison. A record from a different server with the same
  method, status, and path template inside the tolerance will match. The basis
  token `method+path_template+status+time` therefore overstates what was
  compared.
- **O3 — Contract adoption does not check `store_or_file_rescan`.**
  `isCorrelationContractSupported` (`state/httpCorrelation.ts:53-63`) validates
  the schema version, result types, and `causal_claims_allowed`, but not
  `store_or_file_rescan`, even though the renderer surfaces "no source file or
  capture store was reopened" as a safety disclosure. A future contract
  flipping that flag would be adopted silently. The grade and confidence
  vocabularies in the contract are likewise not cross-checked against the
  renderer's closed token sets.
- **O4 — Access-log pairing is order-dependent.** The greedy per-transaction
  loop with a `claimed` set (`analyzer.go:388-417`) makes the assignment depend
  on table order; with several same-shape transactions inside the tolerance,
  the pairing is not the globally nearest one. Acceptable for a bounded
  diagnostic, but undisclosed.
- **O5 — Workspace mount reuses the Diff candidate selector.**
  `AnalysisWorkspacePage.tsx:126` gates the correlation panel on
  `diffCandidateEntries(...)` from `state/httpCaptureDiff.ts`. The filter is
  equivalent today (`result_type === "http_capture"`), but the correlation
  panel should not depend on the Diff feature's selector; `correlationCandidates
  (entries, "http")` already exists and expresses the intent.
  Related, minor: the panel never sends `topN`, so the contract's
  `default_top_n` / `max_top_n` range is unreachable from the UI.

## Known limitations recorded, not charged against this group

- The isolated resolver-cost measurement remains owed at R-RG1 (H-RG4 V3
  deferral).
- The H-RG4 O1/O2 and H-RG5 O1–O4 observations remain open as optional
  hardening on the next change to those surfaces.
- This review ran on macOS. R-RG1 still owes the native Windows acceptance run.

## Re-review scope

A re-review needs only: B1 and B2 remediated with engine regressions that pin
the new fail-closed behaviour (a request-ID pairing under a large clock skew,
and a profile envelope missing `v8_start_time_us`), the matching renderer
handling for an unavailable timestamp delta, and re-runs of `go test ./...`,
`npm run test:state`, and `npm run build`. O1–O5 do not gate the verdict.
