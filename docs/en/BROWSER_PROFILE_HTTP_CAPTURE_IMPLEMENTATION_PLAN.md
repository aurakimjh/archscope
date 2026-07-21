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

- **Part A — Chrome/V8 profile analysis:** the engine, desktop UI, Diff,
  Workspace, paired docs, fixtures, and release smoke are implemented. The new
  process still requires an independent implementation review, so `C-RG1` is
  **implemented / awaiting review**.
- **Part B — HTTP capture and analysis:** Phase-0 design gates, the proxy PoC,
  and deterministic aggregation are complete. A HAR parser/analyzer and minimal
  import page exist, but manifest-driven dialect/security goldens, dedicated
  redaction and resource limits, timeline/brush, detail, and filter UX remain.
  Windows live capture cannot start until the T-571 real-NIC evidence closes.

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
| 2 | `H-RG1` complete offline HAR analysis | **In progress — engine/H-SEC1 remediation complete; re-review/UI pending** | `C-RG1 PASS`; internal `H-SEC1 PASS` |
| 3 | `H-RG2` Windows coverage proof | Waiting | `H-RG1 PASS`; real-NIC evidence and `H-COV1 PASS` |
| 4 | `H-RG3` live-capture engine foundation | Planned | `H-RG2 PASS`; internal `H-SEC2 PASS` |
| 5 | `H-RG4` live UI and Windows E2E | Planned | `H-RG3 PASS` |
| 6 | `H-RG5` HTTP session Diff | Planned | `H-RG4 PASS` |
| 7 | `X-RG1` HTTP x profile/server-evidence correlation | Planned | `H-RG5 PASS` |
| 8 | `R-RG1` integrated release acceptance | Planned | `X-RG1 PASS` |

The two features stay in separate commits except for `X-RG1`, whose purpose is
to join them.

## 5. Part A — Chrome/V8 Profile Analysis

### C-RG1 — Accept the Existing Release Implementation

**Status:** implemented, awaiting independent review. This group approves the
completed T-558 through T-565 work under the new gate process.

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
- [ ] Fix UI regression or accessibility findings from independent review

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

The 2026-07-21 independent review returned `CONDITIONAL`; all P1/P2/P3
remediation is now reflected in code and the paired contracts. The gate is
**awaiting independent re-review** and is not closed until that review returns
`PASS`.

#### Claude UI TODO

- [ ] `(HAR import)` pseudo-process tree and summary cards
- [ ] Full timeline, brush selection, and selected-window recomputation
- [ ] Transaction list plus request/response/timing/process detail tabs
- [ ] Method/status/host/path/MIME/duration/error/fidelity filters
- [ ] Dialect, fidelity, redaction, parser diagnostics, and degenerate-time copy
- [ ] Bounded rendering, empty/error/partial states, and Workspace regressions

#### PASS Criteria

All 20 manifest fixtures and at least two sanitized real exports pass; the UI
timeline selection and filters use the same denominator; import-only UI makes
no live-capture claim.

### H-RG2 — Windows Coverage Proof (T-571)

This important evidence group also serves as the `H-COV1` individual review.

#### Codex TODO

- [ ] Re-run ETW CAP-1/CAP-4 against a Windows real-NIC target.
- [ ] Re-run WFP allow-path attribution after ALE audit configuration.
- [ ] Replace PowerShell polling with direct `GetExtendedTcpTable` and remeasure CAP-5 CPU cost.
- [ ] Update CAP-1 through CAP-6, capability/fidelity matrices, and the source ledger.
- [ ] Remove absolute coverage ratios for failed scopes and retain only the five
  self-observed counters.

**PASS:** zero false attribution, reproducible raw evidence, and an approved list
of values the UI may and may not expose.

### H-RG3 — Live-Capture Engine Foundation

#### Codex Engine TODO

- [ ] Session state machine and idempotent start/stop/recovery API
- [ ] Append-only NDJSON/blob/manifest store, rebuildable index, versioned cursors
- [ ] Byte-bounded write/live/aggregate buffers and disk slow/full policy
- [ ] Captured/persisted/bodyOmitted/eventSkipped/kernelDropped/parseFailed counters
- [ ] Production H1 semantic MITM plus H2 passthrough behind `Proxy`/`Interceptor`
- [ ] Direct Windows TCP-owner attribution with short-connection uncertainty
- [ ] Live completion-order versus file-replay aggregate parity
- [ ] Wails `CaptureService`, sequenced/versioned events, snapshot recovery
- [ ] CA lifecycle, verify-always upstream TLS, approved scoped passthrough

#### Individual Gate H-SEC2

CA key storage, trust-store rollback/removal/expiry, upstream verification,
pinning diagnosis, passthrough scope/expiry, and privilege IPC must pass the
applicable SEC-1 through SEC-15 cases before live-control UI handoff.

#### PASS Criteria

Disk-full, crash recovery, event loss/re-entry, CA failure, pinning,
cancellation, streaming, H2 passthrough, and long-session memory-bound tests pass.

### H-RG4 — Live UI and Windows E2E

#### Codex Integration TODO

- [ ] Supply frozen CaptureService bindings and the Windows E2E harness.
- [ ] Supply snapshot/cursor/filter acceptance fixtures.
- [ ] Support packaging, signing, and privilege-boundary smoke tests.

#### Claude UI TODO

- [ ] Start/stop, session state, CA install/remove, and first-use warning
- [ ] Process tree, stable live list, and in-progress transaction state
- [ ] User-scroll-respecting follow mode, batched updates, and row cap
- [ ] Persistence/drop/backpressure/disk/recovery status
- [ ] Honest fidelity, coverage, passthrough, and unattributed warnings
- [ ] Same-page finalized-session lazy loading after stop

**PASS:** Windows supported-tier scenarios for browser/curl/JVM/Electron, page
re-entry, long sessions, and recovery pass E2E; H2/QUIC/pinning limitations never
look like successful semantic capture.

### H-RG5 — HTTP-Specific Session Diff

#### Codex Engine TODO

- [ ] Versioned URL templates and bounded `{other}` projection
- [ ] Endpoint/host/process dimensions with explicit numerators/denominators
- [ ] `aligned`/`duration_only`/`none` grades
- [ ] Bounded `http_capture_diff` and `HTTP_DIFF_*` findings
- [ ] Store-free export projection and Workspace routing contract

#### Claude UI TODO

- [ ] HttpCapturePage compare action and Workspace comparison entry
- [ ] Grade-aware overlay enablement/suppression
- [ ] Before/after deltas, denominators, unmatched templates, cursor drilldown

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

### R-RG1 — Integrated Release Acceptance

- Run full Go test/vet/build, frontend state tests/build, Windows GUI/live E2E,
  and macOS offline-import/package smoke.
- Align paired docs, importer matrix, and user/security/performance guides.
- Release notes distinguish offline HAR, Windows live tiers, H2/QUIC/pinning,
  and coverage limitations.
- Do not create a version tag or GitHub release before `R-RG1 PASS`.

## 8. First Execution Point

The next action is an independent `C-RG1` review of the Chrome/V8 implementation
and its 2026-07-21 release smoke. No `H-RG1` code starts before that review
passes. Review fixes stay in the same group: Codex owns engine fixes, Claude owns
UI fixes, followed by re-verification and re-review.
