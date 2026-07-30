# Browser Profile and HTTP Capture Implementation and Review-Gate Plan

- Date: 2026-07-21
- Baseline branch: `main`
- Related designs:
  - [Chrome DevTools CPU Profile Analysis](./CHROME_DEVTOOLS_CPUPROFILE.md)
  - [System-wide HTTP Capture](./SYSTEM_HTTP_CAPTURE.md)
- Korean pair: [브라우저 프로파일·HTTP 캡처 구현 및 리뷰 게이트 계획](../ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md)

## 1. Purpose and Current Decision

This document does not redefine either design. It translates the approved
contracts into an implementation order where related tasks are reviewed as a
group and **the next group cannot start until the current group passes**.

- **Part A — Chrome/V8 profile analysis:** `C-RG1` is **complete — PASS**.
- **Part B — HTTP capture and analysis:** `H-RG1` offline HAR analysis is
  **complete — integrated PASS (2026-07-21)**, including engine and H-SEC1
  re-reviews, the bounded import UI, shared fixtures, and full engine/frontend
  verification. T-571/H-RG2 closed with independent `H-COV1 PASS` on
  2026-07-27, and T-580/H-RG3 closed with independent `H-SEC2 PASS` on
  2026-07-28. T-581/H-RG4 Windows live UI and E2E is **complete — group PASS
  (2026-07-30)**: the narrow independent fourth re-review verified V1–V3
  resolved against the fixture-only replacement artifact, whose privacy
  declaration is now derived from the published output and
  contradiction-checked. T-582/`H-RG5` HTTP session Diff is **complete —
  group PASS (2026-07-30)**: the store-free diff engine, regenerated
  bindings, and the grade-aware comparison UI were verified together, and
  reordered equivalent sessions compare equal end to end. `X-RG1`
  correlation is in progress: the Codex backend emits bounded
  HTTP/CPU/Jennifer/access-log correlation with fail-closed clock grades,
  confidence, provenance, and no-causality diagnostics; the regenerated
  bindings and the Claude drilldown/overlay UI are complete (2026-07-31),
  and the X-RG1 group review remains.

## 2. Ownership

| Area | Primary implementer | Owned surface |
|---|---|---|
| Engine | **Codex** | Go model/parser/analyzer/capturer/store, CLI, Wails API/events, generated bindings, engine tests |
| UI | **Claude** | React pages/state/interactions/copy/accessibility and visual/state regression tests |
| Integration | Codex + Claude | Frozen API contracts, fixture IDs, diagnostics, and acceptance scenarios |
| Review | An independent reviewer other than the author | Group-wide correctness, security, UX, and regression verdict |

Codex commits Wails request/result/event contracts and generated bindings before
the Claude handoff. Claude does not hand-edit generated bindings or hide a
missing engine contract in UI-only logic; contract changes return to Codex.
This boundary is mandatory for the remaining groups: Codex stops after the
backend/engine contract, bindings, fixtures, and engine verification are ready,
and leaves every React/UI/state/i18n/visual-regression change to Claude unless
the user explicitly reassigns that UI work.

## 3. Review-Gate Rules

1. Tasks may have separate commits, but the review unit is the complete group.
2. Group order is `Codex engine contract/implementation -> bindings and fixture
   handoff -> Claude UI -> joint verification -> independent review`.
3. Verdicts are `PASS`, `CONDITIONAL`, or `FAIL`. `CONDITIONAL` is not a pass;
   fixes and re-review are required before the next group starts.
4. Review documents placed in `docs/review/` follow the repository procedure:
   reflect every finding in `work_status.md`, then move the processed review to
   `docs/review/done/`.
5. Each review package names implementation commits, changed contracts, tests,
   known limitations, and explicitly excluded work.
6. Work can be parallel inside an approved group, but UI does not guess an API
   before the engine contract is frozen.

### Mandatory Individual Reviews

Only high-consequence boundaries receive an extra per-item gate:

- `H-SEC1`: malicious-HAR resource limits and JWT/cookie/header/query/body redaction
- `H-COV1`: T-571 ETW/WFP/TCP-owner evidence and the coverage-ratio disposition
- `H-SEC2`: CA lifecycle, upstream TLS verification, and privilege boundaries
- `C-SEM1` (only if promoted): `ph:"X"` `BROWSER_LONG_TASK` semantics and CPU-sample attribution

## 4. Global Order

| Order | Review group | Status | Entry condition for the next group |
|---:|---|---|---|
| 0 | `PLAN-RG0` execution plan | **Complete** | This plan and `work_status.md` agree |
| 1 | `C-RG1` Chrome/V8 release implementation acceptance | **Complete — PASS (2026-07-21)** | Independent `PASS` |
| 2 | `H-RG1` complete offline HAR analysis | **Complete — integrated PASS (2026-07-21)** | Closed |
| 3 | `H-RG2` Windows coverage proof | **Complete — H-COV1 PASS (2026-07-27)** | Closed |
| 4 | `H-RG3` live-capture engine foundation | **Complete — H-SEC2 PASS (2026-07-28)** | Closed |
| 5 | `H-RG4` live UI and Windows E2E | **Complete — PASS (2026-07-30)** | Closed |
| 6 | `H-RG5` HTTP session Diff | **Complete — group PASS (2026-07-30)** | Closed |
| 7 | `X-RG1` HTTP x profile/server-evidence correlation | In progress — Codex backend, bindings, and Claude UI complete; group review remains | `H-RG5 PASS` |
| 8 | `R-RG1` integrated release acceptance | Planned | `X-RG1 PASS` |

The two features stay in separate commits except for `X-RG1`, whose purpose is
to join them.

## 5. Part A — Chrome/V8 Profile Analysis

### C-RG1 — Accept the Existing Release Implementation

**Status:** complete — `PASS` (2026-07-21). This group approved the completed
T-558 through T-565 work under the new gate process.

#### Codex Engine Review Scope

- [x] Chrome Performance `.json`/`.json.gz` and V8 `.cpuprofile`/gzip normalization
- [x] Microsecond `int64` units, graph/time invariants, hitCount-only policy
- [x] Bounded gzip/JSON streaming, 256 MiB guard, 500k weighted downsampling
- [x] Source-aware frame identity, redaction, category, and color
- [x] Pre-collapse `cpu_sample_runs`/`cpu_activity` and `SAMPLED_CPU_HOTSPOT`
- [x] One `AnalyzeProfileEvidence` path with Diff, Workspace, and Export
- [x] Shared 15-fixture manifest goldens and CLI/Wails parity

#### Claude UI Review Scope

- [x] `BrowserCpuProfilePage` collection guidance and supported file types
- [x] Copy that never presents sampled CPU runs as browser Long Tasks
- [x] Partial/downsample diagnostic visibility
- [x] Flamegraph, drilldown, and Workspace flow
- [x] Fix UI regression or accessibility findings from independent review

#### PASS Criteria

- All 15 manifest fixtures pass format/diagnostic/finding/duration goldens.
- The paired profile and trace report the same 210 ms `renderList` run at 3.1 s.
- Downsampled and hitCount-only inputs make no time-axis claim.
- Frontend state tests/build, Go tests/build, and local packaging smoke are reproducible.
- Review explicitly approves “sampled CPU run is not a browser Long Task” and
  the bounded-result contract.

### C-EXT1 — Accurate Chrome Duration Events (Optional)

This is not a blocker for the current CPU-profile release. Promote it only as a
separate release objective.

- **Codex:** bounded `ph:"X"` modeling, renderer/process selection,
  `RunTask`-based `BROWSER_LONG_TASK`, Layout/Paint intervals, and CPU attribution.
- **Individual gate `C-SEM1`:** approve task boundaries, attribution, and
  downsample suppression before UI work.
- **Claude:** renderer selection and Long Task/Layout/Paint overlays.
- **Group review:** sampled CPU wording and true duration events must remain
  distinct in UI copy and finding codes.

## 6. Part B — HTTP Capture and Analysis

### H-RG1 — Complete Offline HAR Analysis

This promotes Phase 1 from “an MVP exists” to the full design acceptance level.

#### Codex Engine — Complete (2026-07-21)

- [x] Implement the `CaptureTransaction`, timing-state, and fidelity contracts.
- [x] Split import into `detect -> structural validate -> dialect -> normalize ->
  model map -> redact -> analyze`.
- [x] Add BOM, malformed/deep/oversized JSON and entry/string/body limits with
  deterministic diagnostics.
- [x] Make Chrome/Firefox/Safari/Charles/Fiddler/Proxyman/Insomnia/generic
  dialect normalization a first-class `dialect.go` stage.
- [x] Drive goldens from the 20-fixture shared HAR manifest, including dialect,
  diagnostic, and redaction assertions.
- [x] Apply dedicated redaction to headers, query, cookies, JWT, bodies, and
  process metadata, not only URLs.
- [x] Keep summary/series/tables bounded and define the inline-detail cap and
  truncation diagnostic.
- [x] Verify CLI/Wails parity and the procedure for adding sanitized real
  Chrome/Firefox exports.

#### Individual Gate H-SEC1

Malicious-input resource limits and SEC-4 through SEC-7 must pass, with proof
that secrets do not reappear in diagnostics, findings, exports, or Workspace,
before UI detail/export integration proceeds.

The 2026-07-21 remediation re-review returned `PASS`: all original P1/P2/P3
findings are closed. Phase-2+ SEC-8/10/16/17 implementation measurements remain
assigned to H-SEC2 and do not reopen this offline-import gate.

#### Claude UI — Complete (2026-07-21)

- [x] `(HAR import)` pseudo-process tree and summary cards
- [x] Full timeline, brush selection, and selected-window recomputation
- [x] Transaction list plus request/response/timing/process detail tabs
- [x] Method/status/host/path/MIME/duration/error/fidelity filters
- [x] Dialect, fidelity, redaction, parser diagnostics, and degenerate-time copy
- [x] Bounded rendering, empty/error/partial states, and Workspace regressions

#### PASS Criteria

All 20 manifest fixtures and at least two sanitized real exports pass; the UI
timeline selection and filters use the same denominator; import-only UI makes
no live-capture claim.

The integrated check accepted the deliberately bounded inline-row denominator:
cards, list, and tree use the same filtered rows and the UI explicitly labels
the result as a floor rather than a full-session filtered total. Populated state,
provenance, Workspace, typed component wiring, and production-build evidence
close the Phase-1 UI gate; deeper Wails component fixtures remain non-blocking
hardening. Full Go tests/vet/build and frontend state/build checks passed.

### H-RG2 — Windows Coverage Proof (T-571)

This important evidence group also serves as the `H-COV1` individual review.

#### Codex TODO

- [x] Re-run ETW CAP-1/CAP-4 against a Windows real-NIC target.
- [x] Re-run WFP allow-path attribution on the real-NIC path and record the
  measured-configuration unsupported disposition. ALE audit policy was not
  enabled, so no audit-enabled capability is claimed and WFP is removed as a
  product coverage candidate.
- [x] Replace PowerShell polling with direct `GetExtendedTcpTable` and remeasure CAP-5 CPU cost.
- [x] Update CAP-1 through CAP-6, capability/fidelity matrices, and the source ledger.
- [x] Remove absolute coverage ratios for failed scopes and retain only the five
  self-observed counters.

**PASS:** zero false attribution, an honest reproducibility/evidence-tier record,
and an approved list of values the UI may and may not expose.

The first independent review on 2026-07-27 was `CONDITIONAL`. After remediation,
TCP-owner with `CAP-3: N/A` supports only individual persistent-endpoint
attribution; absolute coverage ratios are forbidden and `counter_fallback: true`
retains only the five self-observed integer counters. Committed observations are
normalized evidence, not a source-level raw package. CAP-6 helper
lifetime/privilege/IPC/install validation is deferred to H-SEC2/H-RG3. The
same-day independent re-review verified all COV-1 through COV-6 remediations
and returned `PASS`.

### H-RG3 — Live-Capture Engine Foundation

#### Codex Engine TODO

- [x] Session state machine and idempotent start/stop/recovery API
- [x] Append-only NDJSON/blob/manifest store, rebuildable index, versioned cursors
- [x] Byte-bounded write/live/aggregate buffers and disk slow/full policy
- [x] Captured/persisted/bodyOmitted/eventSkipped/kernelDropped/parseFailed counters
- [x] Production H1 semantic MITM plus H2 passthrough behind `Proxy`/`Interceptor`
- [x] Direct Windows TCP-owner attribution with short-connection uncertainty
- [x] Live completion-order versus file-replay aggregate parity
- [x] Wails `CaptureService`, sequenced/versioned events, snapshot recovery
- [x] CA lifecycle, verify-always upstream TLS, approved scoped passthrough

Implementation and self-validation completed on 2026-07-27. The Windows
current-user ROOT trust backend rolls back partial installation, removes in
reverse order, and persists only the public-certificate cleanup record so a
post-crash restart can remove it. The proxy binds only to loopback and cannot
disable upstream TLS verification. H2-only ALPN and explicitly approved hosts
produce honest `unsupported` passthrough records. The independent `H-SEC2`
review returned `PASS` on 2026-07-28, so H-RG3 and T-580 are complete and
T-581 / H-RG4 is unblocked.

#### Individual Gate H-SEC2 — `PASS` (2026-07-28)

CA key storage, trust-store rollback/removal/expiry, upstream verification,
pinning diagnosis, passthrough scope/expiry, and privilege IPC passed the
applicable SEC-1 through SEC-16 cases: redaction runs before every persistence
and no plaintext body is stored, the CA private key is memory-only and
non-exportable, session files are owner-only, trust removal is transactional,
upstream TLS is always verified, the proxy is loopback-only, and no CLI/headless
path starts a capture. Two conditions bind the next tier without reopening the
gate: the SEC-10 crash-dump-exclusion preflight must precede any body-capture
tier, and unknown-attribution retention (SEC-17) must sit behind an explicit
metadata-only opt-in before the live UI exposes stored transactions.

#### PASS Criteria

Disk-full, crash recovery, event loss/re-entry, CA failure, pinning,
cancellation, streaming, H2 passthrough, and long-session memory-bound tests pass.

### H-RG4 — Live UI and Windows E2E

**Status:** **Complete — group `PASS` (2026-07-30)**. The third independent
re-review on 2026-07-29 returned the fourth `CONDITIONAL`; the code-side
S1–S9 conditions were verified, and the corrected harness replaced the
rejected artifact. The replacement has 1,012 loopback-only archived rows from
1,023 source rows, records 11 omitted background rows, contains one
confirmed-attribution fixture pinning failure, has an empty contradiction
set, and is pinned by SHA-256
`69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111`.
The narrow independent fourth re-review on 2026-07-30 verified V1–V3 resolved
and closed the gate
(`docs/review/done/2026-07-30_claude-code_H-RG4_windows-live-capture-ui-e2e-fourth-re-review.md`).
T-582 is unblocked; the deferred resolver-cost measurement is owed at
`R-RG1`.

#### Codex Integration

- [x] Supply frozen CaptureService bindings and the Windows E2E harness.
- [x] Supply snapshot/cursor/filter acceptance fixtures.
- [x] Support packaging, signing, and privilege-boundary smoke tests.
- [x] L2: make redaction concurrency-safe and lock it with a stream race test.
- [x] L1: emit honest non-semantic progress fidelity for passthrough and cover
  stop-mid-tunnel behavior.
- [x] L3: bind acceptance fixtures to product constants/transactions and make
  the Windows harness read back capture rows/stats and fail on absent clients.
- [x] L4–L7 backend contracts: bounded progress batches, terminal in-flight
  reconciliation, active SEC-17 policy disclosure, and observed/drop counters.
- [x] L9/L11/L13: enforce platform availability in the manager, avoid fabricated
  CONNECT paths, and document the confirmed-attribution guarantee boundary.
- [x] R1/R5: derive finalized live provenance and weakest fidelity from stored
  transactions, persist the capture-time redaction summary, and keep HAR import
  provenance isolated to the HAR path.
- [x] R3/R4: retain attributed TLS-handshake failures as
  `proxy_not_captured`/`unsupported` and preserve
  `proxy_passthrough`/`unsupported` plus process-derived coverage on tunnel
  failures.
- [x] R6/R7: resolve TCP-owner attribution once per accepted client connection,
  and confirm both PID and process start time across the second owner-table read.
  - R6 resolver-cost measurement is deferred from the H-RG4 correctness/privacy
    gate to the `R-RG1` Windows integrated performance check. Calls are bounded
    to once per connection and the current long-session acceptance passed; a
    useful measurement requires a dedicated Windows run isolated from UI/client
    load.
- [x] R8 backend: expose a versioned `LiveCaptureContract` for renderer row cap,
  event-skip resync, page re-entry, and finalized handoff; bind the fixture to
  that Go contract. Claude must consume it in production state.
- [x] R12 backend: persist terminal `aborted` rows and include them in final
  stats, aggregate, analysis, and acceptance evidence.
- [x] R2 harness and replacement artifact: require an acceptance WebView2 CDP
  port and a real recovery session, run h2-only and explicit CONNECT pinning
  probes, generate a bounded long session, verify page re-entry, and read every
  scenario back from product stores. The corrected run archived fixture-only
  schema-v4 evidence with an empty contradiction set and updated checksum.
- [x] S2: terminalize every stopped in-flight row as `aborted` /
  `unsupported`; `pending` never survives in a finalized store.
- [x] S3: checkpoint capture stats and the known redaction summary in the same
  store flush lifecycle as persisted rows; legacy manifests fall back to stored
  row counts and explicitly unknown redaction instead of false zero/clean data.
- [x] S5–S8: bind fixture and PowerShell to the schema-v4 harness contract,
  require loopback-only fixture origins, omit local paths, cap rows, apply an
  owner-only artifact ACL, prove explicit-proxy QUIC invisibility, and make the
  page re-entry probe locale-independent with product-row reconciliation.
- [x] V1 harness: run Edge/Electron with ephemeral profiles and background-
  networking suppression, restrict published `capture.rows` to loopback
  fixture rows, and derive `fixtureTrafficOnly`, source/archive/omitted counts,
  and local-path absence from the actual output. The fresh run archived 1,012
  fixture rows, omitted 11 background rows, and contains no local path or raw
  long-session secret.
- [x] S9: remove the unused generic `httpcapture.Build` entry point so live rows
  cannot accidentally re-enter the HAR provenance path.

Codex freezes these backend contracts, generated bindings, fixtures, and engine
verification before UI handoff. The read-only
`http-capture acceptance-evidence` command exports bounded metadata from a
stopped product session with owner-only permissions; the Windows harness
requires HTTP/HTTPS rows for all four clients plus h2-only, pinning,
long-session, page-re-entry, and recovery evidence and fails on missing or
contradictory product data. React/UI/state/i18n remains Claude-owned.

#### Live UI

- [x] Start/stop, session state, CA install/remove, and first-use warning
- [x] Process tree, stable live list, and terminal in-progress reconciliation
- [x] User-scroll-respecting follow mode, batched updates, and row cap
- [x] Persistence/drop/backpressure/disk/recovery status with explicit drop warning
- [x] Honest fidelity, coverage, passthrough, and unattributed warnings
- [x] Same-page finalized-session lazy loading after stop

- [x] R8 renderer: read `LiveCaptureContract` at startup and derive the row cap,
  event-skip resync, page-re-entry restore, and finalized handoff from it;
  reject an unknown schema to the built-in defaults and disclose the mismatch.
- [x] R9: `isDecodedLiveFidelity` gates the live table's fidelity emphasis
  through `liveFidelityTone`, so a grade that never read the exchange is never
  styled as ordinary captured traffic.
- [x] R11: transaction state, session state, CA state, and process attribution
  render through closed EN/KO label maps instead of raw engine tokens.
- [x] S1: the finalized hint is selected by provenance
  (`CAPTURE_PROVENANCE_HINT_KEYS`), so live proxy evidence gets its own
  ArchScope-measured sentence and the HAR/import-only sentence is reachable only
  from a `foreign_tool` observation point.
- [x] S4: finalized mode/fidelity/observation/detail-storage tokens resolve
  through closed EN/KO label maps with the raw token on hover, and the
  `capture_mode_counts`/`fidelity_counts`/`coverage_counts` distributions render
  beneath the aggregate so `mixed` and weakest fidelity are interpretable.
- [x] S3 renderer follow-up: `redaction.known=false` renders as unrecorded
  recovery metadata with its own caution styling, never as "no sensitive fields
  matched."

L10 is closed in the paired user guides and JVM truststore harness contract.

SEC-17 is enforced below the renderer: unknown attribution is dropped by
default before persistence or progress exposure, and the explicit opt-in keeps
redacted metadata only. Bodies remain unconditionally omitted, so SEC-10 still
gates any future body-capture tier. The acceptance package is
`cmd/archscope-app/testdata/t581_live_capture_acceptance.json`,
`capture_windows_e2e_test.go`, and
`scripts/verify-windows-live-capture.ps1`, with its shared contract at
`scripts/t581-live-capture-harness-contract.json`.

**PASS:** Windows supported-tier scenarios for browser/curl/JVM/Electron, page
re-entry, long sessions, and recovery pass E2E; H2/QUIC/pinning limitations never
look like successful semantic capture.

### H-RG5 — HTTP-Specific Session Diff

#### Codex Engine — Complete (2026-07-30)

- [x] Versioned URL templates and bounded `{other}` projection
- [x] Endpoint/host/process dimensions with explicit numerators/denominators
- [x] `aligned`/`duration_only`/`none` grades
- [x] Bounded `http_capture_diff` and `HTTP_DIFF_*` findings
- [x] Store-free export projection and Workspace routing contract

The analyzer attaches a versioned top-1,000 source projection to each
`http_capture` result and compares only those projections. The Wails backend
exposes `AnalyzeHttpCaptureDiff`, `GetHttpCaptureDiffContract`, and
`ResolveWorkspaceComparison`; legacy Diff is explicitly unsupported for these
inputs and no new NavKey is required. Regression coverage fixes URL-template
rules, cross-dimension totals, process disablement for HAR, explicit rate
denominators, alignment behavior, top-K result bounds, store-free JSON export,
and reordered-session equality. Full Go test/vet/build passes. Generated
renderer bindings were assigned to the Claude UI handoff and are complete
below.

#### Claude UI — Complete (2026-07-30)

- [x] HttpCapturePage compare action and Workspace comparison entry
- [x] Grade-aware overlay enablement/suppression
- [x] Before/after deltas, denominators, unmatched templates, cursor drilldown

The renderer bindings were regenerated with the module-pinned Wails CLI
(alpha2.117) and include the three methods plus their request/contract
models. A shared comparison panel
(`components/HttpCaptureComparisonPanel.tsx`) mounts on both HttpCapturePage
and the Analysis Workspace; the selection/run lifecycle lives in a pure
reducer plus module store (`state/httpCaptureDiff.ts`) so both surfaces share
one A/B selection. Routing is never guessed by the renderer — it follows the
`ResolveWorkspaceComparison` verdict — and the panel adopts
`GetHttpCaptureDiffContract` at startup, disabling comparison with a
disclosed mismatch for any schema version this build does not implement
(the H-RG4 R8 pattern). Overlays follow only the backend's
`overlay_allowed`+grade verdict and an unrecognized grade fails closed. The
summary and cursor drilldown expose error-rate/traffic-share
numerator/denominator pairs, per-minute denominators in minutes, and
duration-sample counts; sessions without a trustable rate show the closed
EN/KO label for their reason code. The absent side of added/removed
(unmatched-template) rows renders `—`, never `0`, and the process dimension
of a HAR pseudo-process pair renders as a disabled card with its reason.
Results predating the diff source projection are blocked up front with a
re-analyze notice, and the empty change tables of reordered equivalent
sessions render an explicit no-difference state. State regressions pin
contract adoption/rejection, the closed token sets, the fail-closed overlay
gate, race-safe result provenance, candidate filtering, the projection
precondition, and both-locale coverage of every new key. `npm run
test:state`, `npm run build` (tsc + vite), `go build ./...`, `go vet
./cmd/archscope-app/...`, and `go test ./cmd/archscope-app/...
./internal/analyzers/httpcapture/...` pass. No engine source was changed by
Claude.

**PASS:** reordered equivalent sessions yield no change; unsupported normalization
or dimensions are hidden or explicitly disabled for degenerate timestamps and
HAR pseudo-process sessions.

## 7. Cross-Feature Work and Release

### X-RG1 — HTTP x CPU/Jennifer/Access-Log Correlation

- **Codex:** bounded session/profile alignment, Jennifer `NETWORK_GAP` checks,
  access-log client/server comparison, confidence, and mismatch diagnostics.
- **Claude:** drilldown and overlays between HTTP transactions, CPU runs, and
  server evidence in the same time window.
- **Review:** never claim causality across incompatible clocks; always show the
  alignment grade and evidence provenance.

#### Claude UI — Complete (2026-07-31)

The bindings were regenerated with the module-pinned Wails CLI and expose
`AnalyzeHttpEvidenceCorrelation` / `GetHttpEvidenceCorrelationContract`. A
shared correlation panel (`components/HttpCorrelationPanel.tsx`) mounts on
HttpCapturePage (with a "use in correlation" seed action) and the Analysis
Workspace; slot selection (HTTP required, profile/Jennifer/access-log
optional with at least one), the RFC3339 profile wall-clock anchor, and the
time tolerance live in a pure reducer + module store
(`state/httpCorrelation.ts`) whose provenance invariant drops the rendered
result whenever any input changes, including raced completions. The panel
adopts the versioned contract at startup and rejects unimplemented schemas —
or any contract claiming causal claims are allowed — with a disclosed
mismatch. Time overlays render only for a source whose
`alignment_diagnostics` row certifies `overlay_allowed` at an `aligned`
grade; unrecognized grades fail closed, Jennifer tables carry an explicit
duration-only no-overlap note, and a suppressed CPU overlay shows the
engine's reason (e.g. missing anchor). Every result surface repeats the
no-causality disclosure, alignment grades / confidence / match bases /
sources resolve through closed EN/KO label maps with raw tokens on hover,
per-source diagnostics disclose rows used, output rows, and truncation, and
the cursor drilldown shows full per-row evidence plus an aligned-only
time-window overlay. State regressions pin contract adoption/rejection
(including the causal-claims rejection), the fail-closed overlay gate,
anchor validation, slot candidate filtering, the secondary-source minimum,
race-safe provenance, and both-locale coverage of every new key. `npm run
test:state`, `npm run build`, `go build ./...`, `go vet`, and uncached
app/correlation Go tests pass. No engine source was changed by Claude.

### R-RG1 — Integrated Release Acceptance

- Run full Go test/vet/build, frontend state tests/build, Windows GUI/live E2E,
  and macOS offline-import/package smoke.
- Align paired docs, importer matrix, and user/security/performance guides.
- Release notes distinguish offline HAR, Windows live tiers, H2/QUIC/pinning,
  and coverage limitations.
- Do not create a version tag or GitHub release before `R-RG1 PASS`.

## 8. First Execution Point

T-580 / `H-RG3` entered `REVIEW` on 2026-07-27 and passed the independent
`H-SEC2` CA/TLS/privilege gate on 2026-07-28. T-581 / H-RG4 closed with the
independent group `PASS` on 2026-07-30: the corrected Windows run archived a
fixture-only replacement artifact and checksum with no contradictions, and
the narrow fourth re-review verified V1–V3 resolved. T-582 / `H-RG5` HTTP
session Diff closed with a group `PASS` on 2026-07-30: the Codex engine
slice, regenerated bindings, and the Claude grade-aware comparison UI were
verified together, with reordered equivalent sessions comparing equal end to
end (review archived at
`docs/review/done/2026-07-30_claude-code_H-RG5_http-session-diff-group-review.md`).
T-583 / `X-RG1` is now in progress. The Codex backend handoff adds the
versioned `http_evidence_correlation` result, `AnalyzeHttpEvidenceCorrelation`
and `GetHttpEvidenceCorrelationContract`, bounded HTTP/CPU overlaps, Jennifer
network-gap checks, access-log client/server matches, and explicit
alignment/confidence/provenance diagnostics. Incompatible clocks fail closed:
V8 overlays require an explicit RFC3339 profile-start wall-clock anchor, and
Jennifer's date-less ms-since-midnight evidence remains `duration_only`.
Every output row forbids causal claims. Generated bindings, the Claude
drilldown/overlay UI, full verification, and X-RG1 group review remain.
