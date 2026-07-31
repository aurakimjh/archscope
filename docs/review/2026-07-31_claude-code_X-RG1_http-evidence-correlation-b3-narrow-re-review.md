# X-RG1 — B3 Narrow Re-Review (T-583)

- **Review group:** X-RG1 (HTTP evidence correlation — Codex engine, generated
  bindings, Claude drilldown/overlay UI)
- **Target task:** T-583
- **Scope:** narrow. B3 only — the `correlateAccess` state leak in which
  `bestDelta` / `bestClockCompared` survived a later request-ID winner, so an
  emitted row could carry a measured-looking `timestamp_delta_ms` taken from a
  different access-log record than the one it was paired with.
- **Reviewer:** claude-code, acting as the independent reviewer for this group.
  Disclosure: as in the two prior X-RG1 reviews, the Claude UI half of this
  group is the same agent family's work; the B3 remediation itself is engine
  (Codex) work. Every claim below is backed by an executed command or an
  executed probe against the shipped code, not by reading the remediation
  commit message or the plan's completion text. No code, design document,
  `work_status.md` entry, or finding disposition was modified by this review.
- **Date:** 2026-07-31
- **Verdict:** `PASS` for the narrow B3 scope. The remediation does not patch
  the leak in place — it removes the shape it depended on. Candidate selection
  and emitted clock evidence are now separate steps, and the delta and
  `clock_compared` flag are recomputed **exclusively from `used[bestIndex]`**
  after selection, which is the structurally safer of the two options the prior
  re-review offered. The prior review's probe A no longer reproduces; the new
  engine regression is genuine (it fails against the pre-fix analyzer, verified
  by execution); nine adversarial probes over the whole access path found no
  residual or newly introduced misattribution; and the same-class pattern
  elsewhere in the analyzer was swept and is clean. No blocking finding remains
  open in the X-RG1 review chain. One new non-blocking observation (O11) is
  recorded; O1, O2, O4 and O6–O10 remain open as previously stated, three of
  them re-confirmed by execution below.
- **Evidence base:** commit `cb394c2` ("fix X-RG1 access clock provenance") on
  top of `c3383bd` / `fbc1a7d`;
  `internal/analyzers/httpcorrelation/analyzer.go:382-485` and
  `analyzer_test.go:208-250`; `internal/analyzers/httpcorrelation/analyzer.go`
  `correlateProfile` / `correlateJennifer` (same-class sweep);
  `frontend/src/state/httpCorrelation.ts`, `state/regression.test.ts`
  (unchanged, confirmed); the paired EN/KO implementation plans §7 X-RG1.

## What the remediation actually changed

`cb394c2` touches nine lines of `correlateAccess` (`analyzer.go:382-485`):

- `bestDelta` and `bestClockCompared` no longer exist as loop-scoped state. The
  selection loop keeps only `bestIndex`, `bestBasis`, and a renamed
  `bestShapeDelta` whose sole purpose is picking the nearest in-tolerance shape
  candidate (`analyzer.go:394-417`).
- The request-ID branch does no timestamp work at all now. It records the
  index and basis and breaks (`analyzer.go:402-405`).
- After a winner exists, `bestDelta` and `bestClockCompared` are declared fresh
  and computed from `record := used[bestIndex]` alone (`analyzer.go:422-428`).

That is the "structurally safer alternative" the prior re-review named: a row's
delta can no longer come from any record other than the one it was paired with,
because at the moment the delta is computed the only record in scope *is* the
paired one. This is a stronger fix than the minimal reset the finding
required — the minimal reset would have left a correct-by-vigilance invariant
that a future edit could re-break; this one is correct by construction.

The downstream derivations are unchanged and were re-read, not assumed:
`alignment_grade` still requires `bestClockCompared && bestDelta <=
toleranceMS` (`analyzer.go:435-438`), the emit `switch` still omits
`timestamp_delta_ms` entirely when no comparison happened
(`analyzer.go:448-456`), and `allRowsAligned` still gates the source
certification (`analyzer.go:459-470`).

## Verification performed

Run from `apps/engine-native` and `apps/engine-native/cmd/archscope-app/frontend`
on darwin/arm64 (macOS 26.0, Go 1.26.3, Node 25.9.0, npm 11.17.0). As before,
this is not a Windows host; Windows-specific live-capture behaviour is out of
X-RG1 scope and untouched by this commit. R-RG1 still owes the native Windows
acceptance run.

| Command / check | Result |
|---|---|
| `go build ./...` | pass (macOS linker version warnings only) |
| `go vet ./...` | clean, zero diagnostics |
| `go test ./...` | pass — 75 packages ok, zero `FAIL` lines |
| `go test ./internal/analyzers/httpcorrelation/... -count=1 -v` (uncached) | pass, including `TestAnalyzeRequestIdentityDoesNotReusePriorShapeCandidateClock` and both subtests of the B1 regression |
| `npm run test:state` | pass (exit 0) |
| `npm run build` (tsc + vite production) | pass; panel still a lazy chunk (`HttpCorrelationPanel-DY66GojX.js`, 59.42 kB, gzip 13.38 kB) |
| Renderer / bindings | `git diff c3383bd..HEAD -- .../frontend` is empty. No renderer or binding change was made, and per the prior re-review's scope none was owed |
| Ownership boundary | `cb394c2` touches engine `.go` + docs + `work_status.md` only. The Codex/Claude split held |
| Paired docs | EN §7 `X-RG1 Re-Review — Conditional / B3 Remediation Complete` and KO `X-RG1 재리뷰 — 조건부 / B3 보완 완료` both present, with matching §1 status text and matching §4 group-table rows. No EN/KO drift found. The suite claims made in those sections are accurate — I re-ran every one of them |
| Working tree | clean at review time |

Nine adversarial probes (A, A2–A9) were executed against the public
`engineapi.AnalyzeHttpEvidenceCorrelation` entry point from a scratch module
outside the repository (module `b3probe` in a temp directory with a `replace`
onto the engine module — no repo file was created or modified). Their raw
output is the evidence for everything claimed below.

## B3 — disposition verified

### The original defect no longer reproduces

Probe A is the prior re-review's own probe, byte-for-byte: one transaction
(`req-1`, `2026-07-30T01:00:00Z`); access record index 0 is a shape match 50 ms
away with `response_time_ms: 999`; index 1 is the request-ID match with an
unparseable timestamp and `response_time_ms: 80`.

Before (`7049ee8`, quoted from the prior re-review):

```
"match_basis": "request_id", "clock_compared": true,
"timestamp_delta_ms": 50, "alignment_grade": "aligned",
"server_response_ms": 80
```

Now, executed:

```
"match_basis": "request_id", "clock_compared": false,
"alignment_grade": "duration_only", "server_response_ms": 80,
"access_uri": "/api", "request_id": "req-1",
"timestamp_delta_unavailable_reason": "request identity matched, but both observations did not provide parseable absolute timestamps"
```

There is no `timestamp_delta_ms` key at all. The row now describes exactly one
record. The consequences propagate correctly all the way out:

```
"source": "access_log", "alignment_grade": "duration_only",
"confidence": "high", "overlay_allowed": false,
"reason": "request identity paired client and server observations, but at least one pair lacked compatible absolute timestamps"
```

```
"aligned_source_count": 0, "duration_only_source_count": 1, "incompatible_source_count": 0
```

The unearned `aligned` certification, the false "every emitted access-log match
compared compatible absolute timestamps" reason string, and the polluted
summary counter are all gone. The renderer, unchanged, receives a row that
`accessClockComparison` (`httpCorrelation.ts:281-287`) already resolves to
`{ compared: false }`, so the drilldown prints `—` with the engine's reason and
the overlay stays off — which is why no renderer change was owed.

### The new engine regression is genuine, not decorative

`TestAnalyzeRequestIdentityDoesNotReusePriorShapeCandidateClock`
(`analyzer_test.go:208-250`) encodes probe A's shape and asserts all four
things the prior re-review required: the request-ID record is the one selected
(`server_response_ms == 80`), `clock_compared == false`, no `timestamp_delta_ms`
key with the unavailable reason present, `alignment_grade == "duration_only"`,
plus the source diagnostic (`!OverlayAllowed`) and the summary counters.

A passing test proves nothing on its own, so I checked that it fails on the
code it is meant to pin. I copied the engine module to a temp directory,
restored `analyzer.go` from `7049ee8` in the copy only, and ran the new test
there:

```
--- FAIL: TestAnalyzeRequestIdentityDoesNotReusePriorShapeCandidateClock (0.00s)
    analyzer_test.go:237: selected record inherited another candidate's clock evidence:
    map[... "alignment_grade":"aligned", "clock_compared":true,
        "server_response_ms":80, "timestamp_delta_ms":50 ...]
```

The regression reproduces B3 exactly against the pre-fix analyzer and passes
against the current one. It is a real guard.

### Neighbouring paths re-probed for residual and newly introduced misattribution

The prior finding was found by widening the probe after a structural change, so
I did the same again rather than only re-running probe A. All executed:

| Probe | Setup | Result |
|---|---|---|
| **A2** | Earlier in-tolerance shape record (10 ms); later request-ID record with **no `timestamp` key at all** | `clock_compared: false`, no delta, `duration_only`, `overlay_allowed: false`. The missing-key route into the old stale state is closed too |
| **A3** | Earlier in-tolerance shape record (50 ms); later request-ID record with its **own** parseable timestamp skewed 6 h | `timestamp_delta_ms: 21600000` — the winner's own delta, not the 50 ms neighbour's. `duration_only`, no overlay. Provenance correct in the *measured* direction as well, which probe A alone would not have shown |
| **A4** | Two competing shape records (80 ms and 10 ms), no request ID | The 10 ms record wins (`server_response_ms: 90`) and reports `timestamp_delta_ms: 10`. Selection metric and emitted metric agree; `aligned`, `overlay_allowed: true` |
| **A5** | Two transactions: the first measures a delta, the second wins by request ID on an unparseable-timestamp record | Row `h1` → `clock_compared: true`, delta 20; row `h2` → `clock_compared: false`, no delta. No cross-transaction leak (the new state is per-winner, and `bestShapeDelta` is per-transaction) |
| **A6** | B1's original case: request-ID row, both sides timestamped, 6 h skew | Unchanged from the prior re-review — `duration_only`, delta 21600000, no overlay. B1 did not regress |
| **A7** | Request-ID row within tolerance, both sides timestamped | Still `aligned`, `clock_compared: true`, delta 30, `overlay_allowed: true`. The fix did not over-suppress the legitimate case — this was the main false-negative risk and it is not present |
| **A8** | HTTP transaction with **no parseable start**; request-ID record with a good timestamp | `clock_compared: false`, no delta, `duration_only`. Correct (see O8 below on the reason wording) |

A7 matters as much as A: a fix of this shape can easily be over-tightened into
refusing all alignment, which would have made the feature useless while looking
"safe". It was not.

### Same-class sweep of the rest of the analyzer

B3 was an instance of a general shape — hoisted `best*` state where one branch
updates a subset. I swept the other two correlators for it:

- `correlateJennifer` (`analyzer.go:335-353`) declares `bestIndex` /
  `bestDelta` **inside** the per-edge loop and every assignment writes both in
  the same statement (`bestDelta, bestIndex = delta, index`), so the emitted
  `duration_delta_ms` always belongs to `httpRows[bestIndex]`. Clean.
- `correlateProfile` (`analyzer.go:274-305`) emits per-overlap rows with no
  best-candidate state at all. Clean.

`correlateAccess` was the only site, and it is now the only one of the three
that computes its emitted evidence after selection rather than during it.

## Previously open observations — re-confirmed by execution, still open

These do not gate this verdict and were not re-argued; I only checked whether
the B3 fix changed their status.

- **O7 (whole-source overlay gating)** — unchanged in code, but the fix makes
  it bite more often, by design: probe A's single identity-only row now drops
  the entire access source to `duration_only` / `overlay_allowed: false`. That
  is the correct fail-closed direction and exactly what B3 asked for. It does
  strengthen the case for per-row overlay gating, or at minimum for reporting a
  `clock_compared` row count so a reader can see why a source was downgraded.
- **O8 (reason wording)** — still open and now the more visible of the two
  halves. Probe A8: the HTTP side had no parseable start, the access record had
  a perfectly good timestamp, and the emitted text is still "request identity
  matched, but **both** observations did not provide parseable absolute
  timestamps". That reaches the user verbatim in the drilldown's
  `Clock comparison` field and is factually wrong about which side was missing.
  Cheap to fix; worth doing on the next touch of this function.
- **O10 (delta saturation)** — still open, unchanged by the fix. Probe A9, with
  an access record timestamped `0001-01-01T00:00:00Z`, still emits
  `"timestamp_delta_ms": 9223372036854.775` with `clock_compared: true` — a
  `time.Duration` saturation sentinel presented as a measurement. The grade
  correctly falls to `duration_only` so nothing is overlaid, and archscope's own
  access-log parser cannot produce this input. The prior review suggested
  folding a bounded-delta guard into the B3 change; the remediation was kept
  minimal instead, which is a defensible scoping call and is not charged here.
- **O1, O2, O4, O6, O9** — untouched by this commit and unaffected by it.
  O2's elevated consequence noted in the prior re-review (a cross-host shape
  row can be the sole row and certify the source) is unchanged.

## New non-blocking observation

- **O11 — the shape-basis path's selection/emission agreement is unpinned.**
  The delta is now computed twice for a shape-basis winner: once in the loop to
  choose the nearest candidate (`bestShapeDelta`), once after selection to emit
  (`bestDelta`). Today the two agree by construction — same record, same parse,
  same expression. But no engine regression asserts that a shape-basis row's
  *emitted* delta corresponds to the record it selected; the new test covers
  only the request-ID winner, and
  `TestAnalyzeCorrelatesBoundedProfileJenniferAndAccessEvidence` has a single
  access candidate. Probe A4 confirms the behaviour is correct now, but a
  future change to either expression (an offset, a signed delta, a different
  tolerance basis) could silently diverge them and re-open a narrower B3
  without any test noticing. A two-shape-candidate regression asserting the
  winner's `server_response_ms` and `timestamp_delta_ms` together would close
  this for a few lines of test. Non-blocking: the current code is correct, and
  this is about keeping it that way.

## Known limitations recorded, not charged against this group

- The isolated resolver-cost measurement remains owed at R-RG1 (H-RG4 V3
  deferral).
- The H-RG4 O1/O2 and H-RG5 O1–O4 observations remain open as optional
  hardening on the next change to those surfaces.
- This review ran on macOS. R-RG1 still owes the native Windows acceptance run.

## Re-review scope

None. B3 is closed and no blocking finding remains open across the X-RG1 review
chain (B1, B2, B3 all verified fixed by execution). O1, O2, O4, and O6–O11 are
non-blocking and do not require a further review pass; they should be picked up
on the next change to the surfaces they name. Recording the disposition of
these findings, and the resulting status of T-583 / X-RG1, is outside this
reviewer's role and is left to the implementing agent.
