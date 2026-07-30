# H-RG5 — HTTP-Specific Session Diff Group Review (T-582)

- **Review group:** H-RG5 (HTTP session Diff — Codex engine, generated
  bindings, Claude comparison UI)
- **Target task:** T-582
- **Reviewer:** claude-code (group review). Disclosure: the Claude UI half of
  this group was implemented by the same agent in the same working session;
  the engine half (commit `66d346a`) was implemented by Codex and is reviewed
  here independently. This mirrors the H-RG4 practice where claude-code
  issued group verdicts on work that included Claude-owned UI, and every UI
  claim below is backed by an executable regression rather than assertion.
- **Date:** 2026-07-30
- **Verdict:** `PASS` — both plan PASS criteria hold under independent
  execution: reordered equivalent sessions produce empty change tables, zero
  findings, and a zero delta; degenerate-timestamp sessions carry explicit
  `rate_unavailable_code` reasons and a `none`/`duration_only` alignment
  verdict that the renderer follows fail-closed; HAR pseudo-process pairs
  disable the process dimension with a disclosed reason at both layers. Four
  non-blocking observations (O1–O4) are recorded. T-582 transitions to
  `DONE`; T-583 / X-RG1 is unblocked.
- **Evidence base:** commits `66d346a` (Codex backend) and `5f679ae`
  (bindings + Claude UI); `internal/analyzers/httpcapture/diff.go` /
  `diff_test.go`; `cmd/archscope-app/engineservice.go` /
  `engineservice_test.go`; the regenerated bindings under
  `frontend/bindings/...`; `frontend/src/state/httpCaptureDiff.ts`,
  `components/HttpCaptureComparisonPanel.tsx`, the HttpCapturePage /
  AnalysisWorkspacePage integrations, `i18n/messages.ts`, and
  `state/regression.test.ts`; the paired EN/KO implementation plans §H-RG5.

## Scope

The plan (§6, H-RG5) requires: versioned URL templates with a bounded
`{other}` projection; endpoint/host/process dimensions with explicit
numerators and denominators; `aligned`/`duration_only`/`none` time grades;
bounded `http_capture_diff` tables and `HTTP_DIFF_*` findings; a store-free
export projection and Workspace routing contract; and a Claude UI with a
compare action, Workspace comparison entry, grade-aware overlay
enablement/suppression, before/after deltas with denominators, unmatched
templates, and cursor drilldown.

**PASS criteria:** reordered equivalent sessions yield no change;
unsupported normalization or dimensions are hidden or explicitly disabled
for degenerate timestamps and HAR pseudo-process sessions.

## Verification performed

Run from `apps/engine-native` and `cmd/archscope-app/frontend` on
windows/amd64 (Windows 11 Pro 10.0.26200, Go native, Node 22).

| Command / check | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./cmd/archscope-app/...` | clean |
| `go test ./...` | pass — 74 packages ok, zero failures |
| `go test -count=1 ./cmd/archscope-app/... ./internal/analyzers/httpcapture/...` | pass, uncached |
| `npm run test:state` | pass, including the ~40 new diff regressions |
| `npm run build` (tsc + vite production) | pass; the comparison panel is a lazy chunk (`HttpCaptureComparisonPanel-*.js`) |
| Binding regeneration | `wails3 generate bindings -ts` with the module-pinned CLI (`go run`, alpha2.117); `AnalyzeHttpCaptureDiff` / `GetHttpCaptureDiffContract` / `ResolveWorkspaceComparison` plus `HttpCaptureDiffRequest` / `WorkspaceComparisonRequest` / contract models present in `engineservice.ts` / `models.ts`; an earlier stray run with a globally installed alpha.87 CLI was fully reverted before the pinned regeneration |
| Projection attachment | `buildParsed` attaches `http_capture_diff_source` + `http_capture_diff_contract` to **every** result; `BuildLive` routes through the same emit path, and `captureservice.go:213` binds the finalized live snapshot via `SetDiffCaptureSessionRef` without a store rescan |

### Backend behaviors verified against the emitted envelope

- **Reordered equality (PASS criterion 1):**
  `TestAnalyzeDiffReorderedEquivalentSessionsCompareEqual` builds two results
  from a shuffled copy of the same entries and asserts all five change tables
  are empty, `Metadata.Findings` is empty, and the summary delta is zero. Run
  and passed uncached.
- **URL template v1:** `TestURLTemplateVersionOneRulesAndQueryKeys` pins
  `{id}`/`{uuid}`/`{hash}`/`{token}`/`{email}` segment folding and
  sorted-query-key-only templating; `AnalyzeDiff` rejects mismatched or
  unsupported `url_template_version` and mismatched template limits with
  re-analyze guidance.
- **Explicit denominators:** every side metric carries
  `error_rate`/`traffic_share` as `{numerator, denominator, value}`;
  `count_per_minute` exists only when the timeline is trusted, otherwise
  `rate_unavailable_code` (`timestamps_degenerate` /
  `capture_duration_unavailable`) says why (PASS criterion 2, engine half).
- **Process dimension:** HAR imports and sessions without real attribution
  set `process_available: false` with a reason;
  `TestAnalyzeDiffLiveProcessDimensionAndDurationOnlyGrade` covers the live
  side and the `duration_only` grade.
- **Bounds and validation:** source projections are bounded at ≤1,000 rows
  +`{other}` with fold disclosure; `validateDiffProjection` cross-checks
  endpoint/host/process totals against the transaction total; tables are
  bounded by `table_limit` (≤500); finding evidence rows are capped at 10;
  `store_rescanned: false` and `export_projection:
  analysis_result_envelope` are recorded in the envelope.
- **Workspace routing:** `ResolveWorkspaceComparison` returns a supported
  route only for an `http_capture`/`http_capture_diff` pair and an explicit
  reason otherwise; legacy Diff (`DiffPage`, `ProfilerService.Diff`) is
  untouched by both commits; no new NavKey exists.

### Claude UI behaviors verified by executed regressions

All of the following are pinned in `state/regression.test.ts` (run above),
not just implemented:

- **Contract adoption (H-RG4 R8 pattern):** a v1 contract enables
  comparison; schema 99, a foreign `result_type`, and a missing contract all
  disable it and flag the mismatch, which the panel renders as a disclosure.
- **Grade-aware overlay gating, fail closed:** `aligned` → duration and
  per-minute overlays; `duration_only` → durations only; `none`,
  `overlay_allowed: false`, an unrecognized grade, and a missing alignment
  block → nothing renders. The renderer follows the backend verdict and
  never upgrades one it cannot interpret.
- **Closed token sets:** alignment grades, change tokens, rate-unavailable
  codes, and source kinds each resolve unrecognized wire values to `unknown`
  labels ("unrecognized …") instead of leaking raw tokens; raw tokens stay
  reachable on hover.
- **Result provenance:** changing either selection, swapping, or a failed
  comparison drops the rendered result; a comparison that resolves after a
  selection change (raced) never renders under the new pair.
- **Preconditions:** only `http_capture` entries are candidates; a result
  without `http_capture_diff_source` (analyzed before the backend handoff)
  is blocked up front with a re-analyze notice — matching the backend's
  `errDiffSourceMissing` contract instead of surfacing it as a failure.
- **No-difference state:** all-empty change tables render an explicit "no
  differences" sentence (PASS criterion 1, renderer half).
- **Unmatched templates:** `endpoints_added`/`endpoints_removed` render as
  their own tables and the absent side renders `—`, never `0`.
- **Locale parity:** every new message key is asserted non-empty in both EN
  and KO, and the global en/ko key-parity check covers the additions.

Manual code inspection confirmed the drilldown slide-over exposes per-side
numerator/denominator pairs, per-minute denominators in minutes, and
duration-sample counts, and that the disabled process dimension renders as
an explicit card with the engine's reason string (PASS criterion 2,
renderer half).

## Non-blocking observations

None blocks the gate; recorded for future hardening.

- **O1 (L):** at `duration_only` grade the summary's per-minute row prints
  the closed "unavailable" label for **both** sides, including a side that
  privately has a trusted per-minute value. This is deliberate suppression
  of a non-comparable rate in a comparison table (the per-side value remains
  reachable in the drilldown), but a future refinement could distinguish
  "suppressed for comparability" from "unavailable".
- **O2 (L):** removing a Workspace entry that is currently selected as A or
  B leaves the stale id in the shared store; the compare action correctly
  disables (the entry no longer resolves) and the select falls back to its
  placeholder, but the selection is not auto-cleared.
- **O3 (L):** the Workspace per-entry slot buttons are labeled "A"/"B" with
  the full action name only in the `title` tooltip; an `aria-label` would
  serve screen readers better.
- **O4 (L):** `engineservice_test.go` covers the contract, routing, and
  store-free analysis at the Wails layer in one test
  (`TestEngineService_HttpCaptureDiffContractRoutingAndStoreFreeAnalysis`);
  per-error-path request validation (nil before/after) is exercised only via
  the analyzer's own rejection tests.

## Verdict

`PASS`. Both plan PASS criteria are satisfied by independently executed
regressions on the engine and the renderer, the ownership boundary was
respected (no engine source changed in the UI commit; no frontend source
changed in the backend commit), the bindings were regenerated with the
module-pinned CLI, and the paired EN/KO plans record the completed handoffs.
T-582 transitions to `DONE`; T-583 / X-RG1 is unblocked. The only H-RG4
carry-forward obligation (isolated resolver-cost measurement) remains owed
at R-RG1 and is unaffected by this group.
