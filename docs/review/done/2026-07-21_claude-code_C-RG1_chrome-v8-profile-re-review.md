# C-RG1 — Chrome/V8 Browser CPU Profile Re-Review (Remediation Verification)

- Review ID: **C-RG1** (task **T-578**), re-verification pass
- Date: 2026-07-21
- Reviewer: Claude (same independent reviewer as the first pass; findings-only)
- HEAD reviewed: `b023040`
- First-pass verdict: **CONDITIONAL** (`docs/review/done/2026-07-21_claude-code_C-RG1_chrome-v8-profile-implementation-review.md`)
- Remediation commits: `5db8df8` (A), `ed01dd0` (B), `7b24206` (C), `40c6bde` (D),
  `3249c27` (E), `b023040` (U1–U5); doc updates in `5db8df8`
- **Re-review verdict: PASS** — H-RG1 may start.

> Findings-only, no source modified. Every finding from the first pass was
> re-verified independently at HEAD by reading the changed code and re-running
> tests, not by trusting the commit messages.

---

## 요약 (Korean TL;DR)

1차 리뷰의 CONDITIONAL 사유였던 엔진 A~E(Codex)와 UI U1~U5(Claude)를 현재 HEAD에서
독립 재검증했다. **10개 finding 전부 해소**되었고 회귀 증거(16개 픽스처 골든, 엔진
단위테스트 uncached, `go vet`/`build`, 프런트 state 회귀테스트, EN/KO i18n)가 모두
통과한다. 특히 핵심이던 시간-귀속(A)은 **코드를 계약에 맞춰 전방 귀속으로 수정**하고
(`TimestampUS=ts[i]`, `Value=ts[i+1]-ts[i]`, 마지막 샘플은 `endTime` tail),
골든 허용오차를 **0(정확 일치)** 으로 좁혀 방향을 실제로 고정했다.
**판정: PASS. H-RG1 시작 가능.**

---

## 1. Verification evidence (this session, go1.26.5, HEAD `b023040`)

- `go test -count=1 ./internal/parsers/profile/... ./internal/analyzers/profile/...
  ./internal/analyzers/profileclassification/... ./internal/profiler/...
  ./cmd/archscope-app/... ./cmd/archscope-engine/...` → **all `ok`** (uncached).
- Shared manifest goldens: **16/16 pass** (was 15; a `malformed/trace-negative-delta.json`
  fixture was added for finding C — strictly more coverage).
- `go vet` (profile packages) clean; `go build ./...` succeeds (benign macOS
  linker-version warnings only).
- Frontend `npm run test:state` (`tsc` + Node regression harness) → exit 0;
  the harness now includes explicit `profile_evidence` U1/U2/U4 assertions.
- All page i18n keys resolve in **both `en` and `ko`** (EN/KO parity preserved).

---

## 2. Finding-by-finding re-verification

### Engine (Codex)

**A — Time-attribution direction & golden [P1] → RESOLVED.**
- The parser now uses `v8SampleNormalizer` (`v8.go:174-220`): it advances the
  timestamp, sets `TimestampUS = ts[i]`, and attributes each sample the interval
  **until the next observation** (`Value = ts[i+1] - ts[i]`); the final sample
  owns the `endTime` tail (`Finish`). This is forward attribution —
  `timeDeltas[i]` is now the cost of sample `i-1` — matching the locked contract
  (`CHROME_DEVTOOLS_CPUPROFILE.md` Decision 4). The identical `chrome-trace`
  loop was refactored onto the same normalizer (`v8.go:452,479-484`), so both
  formats share one attribution core.
- `endTime < ts[last]` is clamped to a zero tail with a new
  `PROFILE_END_TIME_BEFORE_LAST_SAMPLE` diagnostic (`v8.go` + `parser.go:169-171`),
  covered by `TestParseV8ClampsEndTimeBeforeLastSample`; the happy path is pinned
  by `TestParseV8AttributesFinalSampleThroughEndTime` (`[200 500]`).
- The golden was tightened from `> 1000µs` tolerance to **exact** equality
  (`fixture_manifest_test.go:220-229`), the manifest field renamed
  `*_approx` → `start_us`/`duration_us`, and the e2e expectation set to the
  forward-attribution values `start_us=3101000, duration_us=210000`. The tolerance
  now equals 0, so the convention is genuinely pinned (the first-pass complaint
  that "the golden does not distinguish the two conventions" no longer holds).
- Docs EN/KO updated to explain the Chrome-consistent direction. Verified: all
  parser/analyzer/manifest tests pass uncached.

**B — Duration decomposition [P2] → RESOLVED.**
- `summary()` now emits `recording_duration_us`, `active_duration_us`,
  `idle_duration_us`, `sampled_duration_us` (and `_ms` forms) via
  `browserProfileDurations` (`analyzer.go:289-345`): `recording = endTime -
  startTime` (display-invariant), idle detected by an `(idle)` leaf. The old
  `total_duration_us` is retained as an explicitly-documented **deprecated alias**
  of `sampled_duration_us` (keeps the existing UI card working). No more silent
  head/tail conflation.

**C — Chrome-trace negative-delta diagnostic parity [P2] → RESOLVED.**
- `parseChromeTraceReader` now counts clamped deltas and sets
  `negative_delta_clamp_count`, so `PROFILE_NEGATIVE_DELTA_CLAMPED` fires for
  traces exactly as for `.cpuprofile`. New golden `malformed/trace-negative-delta.json`
  asserts the diagnostic (manifest `diagnostics: ["PROFILE_NEGATIVE_DELTA_CLAMPED"]`).

**D — Chrome-trace hitCount cross-check [P3] → RESOLVED.**
- The mismatch logic was extracted to `v8HitCountMismatches(nodes, occurrences)`
  and is now invoked on the trace path with occurrences accumulated across
  chunks, so `hit_count_mismatch_nodes` / `PROFILE_HITCOUNT_MISMATCH` (INV-C3) now
  apply to traces too.

**E — First-sample guard vs `startTime==0` [P3] → RESOLVED.**
- The fragile `Value==0 && TimestampUS!=0` guard was replaced by
  `normalizedSampleWeight(index, sample, valueUnit)` (`analyzer.go`), which keys
  on `index==0` and treats zero-width microsecond intervals exactly (`continue`,
  not counted as 1). A `startTime==0` profile no longer miscounts its first sample.

### UI (Claude)

**U1 — Diagnostics visibility [P1] → RESOLVED.**
- The page now mounts `DiagnosticsPanel` reading `result.metadata.diagnostics`
  with an issue-count badge (`BrowserCpuProfilePage.tsx:271-287`,
  `browserCpuProfile.ts:extractProfileDiagnostics`). The empty sampled-run
  timeline is explained by reason — `hitcount_only` / `downsampled` / `empty`
  (`describeTimelineState`, page `:259-266`) — instead of a silent empty table.
  Critically, the hitCount-only case (which the engine does **not** flag as
  `partial_result`) is still detected via the `PROFILE_HITCOUNT_ONLY` code /
  `value_unit=="samples"`, closing the first pass's worst sub-issue. Asserted by
  the regression test (`regression.test.ts:452-455`).

**U2 — Flamegraph & drilldown [P1] → RESOLVED.**
- `CanvasFlameGraph` is rendered from `charts.flamegraph`
  (`extractProfileFlamegraph`/`adaptProfileFlameNode`, page `:171-181`) — the same
  interactive (click-to-zoom) component the other profiler pages use — plus a
  `top_child_frames` table for frame-level drilldown. Workspace/Export flow
  retained (`addWorkspaceResult`, `:54-58`). Flamegraph adaptation asserted by
  the regression test (`:405-422`).

**U3 — `.json.gz` support [P2] → RESOLVED.**
- `accept=".json,.cpuprofile,.gz,.json.gz"` and the native filter
  `*.json;*.cpuprofile;*.json.gz;*.gz` (`:88,95-101`) now advertise the gzip
  support the engine already had.

**U4 — Empty-state, i18n, a11y [P2/P3] → RESOLVED.**
- Pre-result empty-state card (`:125-131`); every page string is now an i18n key
  present in both `en` and `ko`; tables use `<th scope="col">` and an `sr-only`
  `<caption>` (`:190,233,236-244`). Suppressed-timeline copy uses `role="status"`.

**U5 — Frontend regression test [P2] → RESOLVED.**
- `state/regression.test.ts:384-509` adds `profile_evidence` coverage:
  flamegraph adaptation, diagnostics extraction + issue count, and the
  hitCount-only / downsampled / populated timeline-state branches. Runs green.

---

## 3. Re-confirmed PASS criteria (plan §5)

- All manifest fixtures pass format/diagnostic/finding/duration goldens (**16/16**).
- Paired profile↔trace parity holds: the manifest asserts equal
  `total_duration_us` for `e2e-render-hotspot.cpuprofile` and `e2e-chrome-trace.json`.
  Under the corrected forward attribution the shared `renderList` run is pinned
  **exactly** at `start_us=3101000` (one sample interval after the `ph:"X"`
  RunTask boundary at 3.1 s — the honest sampling result), `duration_us=210000`.
- Downsampled and hitCount-only inputs make no time-axis claim (engine suppresses
  `cpu_sample_runs`/`cpu_activity`; UI explains *why* the timeline is empty).
- Frontend state tests/build and Go tests/build are reproducible (run this session).
- The "sampled CPU run is not a browser Long Task" contract and the bounded-result
  contract remain intact and are **explicitly approved**; the Go golden still
  fails any sample-derived finding that uses Long-Task wording, and the UI repeats
  the disclaimer.

---

## 4. Scope note / limitation

This pass verified the code and test evidence for the ten remediation items.
The fixes are confined to parser/analyzer logic and frontend page/state code;
none touch packaging or signing. The local macOS package/DMG smoke recorded on
2026-07-21 for the release slice therefore still stands and was not re-run here.
No behavior in the fixes affects the packaging surface.

---

## 5. Verdict

**PASS.** All first-pass findings (engine A–E, UI U1–U5) are independently
verified resolved at `b023040`, with regression coverage (16-fixture goldens,
engine unit tests, and the frontend state harness) that pins the previously
ambiguous time-attribution contract exactly. Per
`BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §4, **C-RG1 passes and
unblocks H-RG1.**

H-RG1 (offline HAR analysis) may begin. Its own gate — internal **H-SEC1** plus a
full vertical-slice group review — still applies before it, in turn, can unblock
H-RG2.
