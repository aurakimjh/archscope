# ArchScope Work Status

Last updated: 2026-11-10

This file is the current execution status for the active ArchScope product line.
The previous long-form history was archived to
`archive/work_status_legacy_2026-05-09.md`.

## Status Overview

**Overall: 202 of 214 tracked tasks are `DONE` (94.4%).** The Windows Wails v3 replacement for IEInspector HTTP Analyzer V7.6.4 Stand-alone has 4 open work items (T-594 through T-613), including the remaining audit findings. T-603 native Windows acceptance completed on 2026-11-09 with actual request/response bodies and verified policy restoration. The 2026-11-09
GC/profiler revalidation confirmed five bugs (T-589 through T-593). Their
current implementation status is recorded in the registry and snapshot below;
T-557 is now DONE after the user requested its Go/CLI/report projection on 2026-11-10.
The 2026-11-10 other-feature audit added five reproduced correctness bugs (T-614 through T-618); current implementation/verification status is recorded in the registry below.
The earlier release acceptance below describes its recorded product baseline
and does not certify these newly identified cases or future remediation.
**T-587 and T-588
closed on 2026-08-11 after their required reviews and the post-change R-RG1
re-run passed.** T-588's Windows forced-crash proof used real request and
response recorders, a live CA key, and `RaiseFailFastException`; Engine Native
run `31488911063` found zero new or modified crash artifacts and no body or
CA-key canary in the dump roots or Application log, then restored the original
`LocalDumps` policy. The activated-tier H-SEC2 review returned `PASS AFTER
REMEDIATION`. The T-587 UI/engine review also returned `PASS AFTER REMEDIATION`
after correcting connection-blocked timing, malformed contract-v3 fail-closed
behavior, and resource-filter option derivation. Post-change R-RG1 acceptance
then passed at product commit `7af0f7a`: Engine Native `31488911063`, general CI
`31488911030`, and Windows GUI Smoke `31489687652` all succeeded. The archived
evidence and review/status commits after that product commit change no buildable
source. **T-580 / H-RG3 is
complete with an independent H-SEC2 `PASS`** on 2026-07-28: the live-capture
engine foundation passed the CA/TLS/privilege gate. Redaction runs before every
persistence, the MITM proxy never stores plaintext bodies, the CA private key is
memory-only and never exported, upstream TLS is always verified against the host
trust store, the proxy binds only to loopback, and no CLI/headless path can start
a capture. Two forward-looking conditions (SEC-10 dump-exclusion preflight before
body capture, SEC-17 explicit unknown-attribution opt-in before the live UI) bind
the next tier and do not reopen this gate. **T-581 / H-RG4 is complete with an
independent `PASS` on 2026-07-30** after a narrow fourth re-review verified the
V1–V3 conditions resolved. The earlier re-reviews confirmed that the original L2 race and the live-table fidelity, batching,
terminal-row, SEC-17, denominator, row-stability, and labeling defects are
resolved. The Codex follow-up now derives finalized live provenance and weakest
fidelity from stored rows (R1), retains honest attributed pinning and failed
tunnel records (R3/R4), persists the capture-time redaction summary (R5),
resolves attribution once per client connection and confirms the process start
time (R6/R7), exposes a versioned renderer contract for Claude (R8 backend),
updates the paired plan (R10), and persists aborted rows (R12). The schema-v4
harness now exercises supported clients, h2-only, pinning, long-session,
WebView re-entry, and a separate recovery store. The first Windows artifact was
archived but later rejected by V1. The corrected replacement now archives 1,012
loopback-only rows from 1,023 source rows, records 11 omitted background rows,
contains confirmed fixture-origin pinning evidence and no contradictions, and
has SHA-256
`2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e`.
This pins the LF-normalized repository bytes; the original Windows CRLF
artifact digest was
`69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111`.
The Claude-owned UI handoff is complete as well: the renderer consumes the
engine's `LiveCaptureContract` for its row
cap, event-skip resync, page-re-entry restore, and finalized handoff and
discloses a contract it cannot honour (R8); `isDecodedLiveFidelity` gates the
live table's fidelity emphasis through `liveFidelityTone` so a grade that never
read the exchange is never styled as ordinary captured traffic (R9); and
transaction, session, CA, and attribution states render through closed EN/KO
label maps instead of raw engine tokens (R11). The full Go
suite, capture race suite, vet/build, Windows app/engine cross-build and vet,
generated bindings, frontend state tests, production frontend build, and the
actual Windows PowerShell harness pass. The Claude-owned finalized card is
implemented as well: its explanatory
prose is selected from the reported provenance so a live session is never
described as imported foreign-tool evidence (S1), its fidelity/mode/observation/
detail-storage tokens resolve through closed EN/KO label maps and are followed
by the per-transaction distributions behind the aggregate grade (S4), and a
`redaction.known=false` recovery state renders as unrecorded rather than as "no
sensitive fields matched" (S3 renderer half). **The 2026-07-29 independent
third re-review returned a fourth `CONDITIONAL`** after finding V1 privacy-scope
drift in the first artifact. The replacement run now enforces that scope,
derives the privacy declaration from its published rows, suppresses client
background networking, proves an attributed fixture pinning failure, and
passes the full contradiction set. V2 is closed by the paired-plan update; V3
explicitly defers the isolated resolver-cost measurement to the R-RG1 Windows
performance check. **The 2026-07-30 narrow fourth re-review independently
verified V1–V3 resolved and returned `PASS`**: every archived row is loopback,
the whole artifact carries no background host, stable identifier, raw secret,
or local path, the privacy declaration is derived and contradiction-checked
rather than asserted, the Go fixture binds the harness mechanisms, and the
checksum matches everywhere it is quoted. H-RG4 is closed and T-582 / H-RG5
is unblocked. **T-582 / H-RG5 backend implementation is complete as of
2026-07-30**: every new `http_capture` result carries a versioned, bounded
Diff source projection; `AnalyzeHttpCaptureDiff` produces the store-free
`http_capture_diff` envelope with explicit rate denominators, alignment grades,
bounded tables and `HTTP_DIFF_*` findings; and the backend publishes the
Workspace routing contract without touching legacy Diff. **The generated Wails
bindings and the Claude-owned grade-aware comparison UI are complete as of
2026-07-30 as well**: a shared comparison panel mounts on HttpCapturePage and
the Analysis Workspace, follows the backend `ResolveWorkspaceComparison`
routing verdict, adopts the versioned diff contract at startup (disabling
comparison with a disclosed mismatch for unimplemented schemas), gates every
time overlay on the backend's `overlay_allowed`+grade verdict with fail-closed
handling of unrecognized grades, exposes explicit numerator/denominator rates
and per-minute denominators, renders unmatched-template sides as `—` rather
than `0`, discloses the disabled HAR pseudo-process dimension with its reason,
and renders reordered-equal sessions as an explicit no-difference state.
**The 2026-07-30 H-RG5 group review returned `PASS`**: reordered equivalent
sessions were independently verified to yield empty change tables, zero
findings, and a zero delta; degenerate timestamps and HAR pseudo-process
pairs are explicitly disabled with reasons at both the engine and renderer
layers; the fail-closed overlay gate, closed token sets, contract adoption,
and result provenance are all pinned by executed regressions. Four
non-blocking observations (O1–O4) were recorded. T-582 is `DONE` and
T-583 / X-RG1 is unblocked. The review is archived at
`docs/review/done/2026-07-30_claude-code_H-RG5_http-session-diff-group-review.md`.
**T-583 / X-RG1 backend implementation is complete as of 2026-07-30**:
`AnalyzeHttpEvidenceCorrelation` consumes bounded `AnalysisResult` projections
only and emits `http_evidence_correlation` with HTTP/CPU overlap rows,
Jennifer `NETWORK_GAP` checks, access-log client/server matches, explicit
alignment/confidence diagnostics, provenance, and mismatch findings. V8
timestamps require an explicit RFC3339 wall-clock anchor before overlays are
allowed; Jennifer's date-less ms-since-midnight evidence remains
`duration_only`; every row forbids causal claims. **The regenerated Wails
bindings and the Claude drilldown/overlay UI are complete as of 2026-07-31
as well**: a shared correlation panel mounts on HttpCapturePage and the
Analysis Workspace, adopts the versioned contract at startup (rejecting
unimplemented schemas and any contract permitting causal claims), gates
every time overlay on the per-source `alignment_diagnostics`
`overlay_allowed`+`aligned` verdict with fail-closed handling of
unrecognized grades, requires the RFC3339 profile wall-clock anchor with
pre-run validation, repeats the no-causality disclosure on every result
surface, resolves grades/confidence/match-bases/sources through closed
EN/KO label maps, and provides cursor drilldown with an aligned-only
time-window overlay. **The 2026-07-31 X-RG1 group review returned
`CONDITIONAL`**. It confirmed the bounded-input, no-rescan, no-causality,
contract-adoption, missing-anchor, Jennifer duration-only, provenance, and
output-limit guarantees, but found two blocking engine defects: B1 request-ID
access-log matches can claim `aligned` with a fabricated zero timestamp delta,
and B2 a missing/non-numeric V8 start timestamp silently defaults to zero.
The matching unavailable-delta renderer state belongs to Claude. O1–O5
(Jennifer match bounds, access host comparison, renderer contract checks,
order-dependent access pairing, and the Workspace selector/top-N path) are
non-blocking follow-up observations. **Codex B1/B2 remediation is complete**:
request identity is independent from measured clock alignment,
incompatible/unavailable deltas fail closed, and invalid V8 timestamp bases
suppress CPU overlays with explicit findings. **The Claude UI/state/i18n
remediation is now complete as well**: the access-log wire row carries
`clock_compared` with an optional delta, `accessClockComparison` fails closed
to unavailable whenever the flag, the delta, or its finiteness disagree, and
both the match table and the drilldown render an unmeasured pairing as `—`
with the engine's reason instead of a measured-looking `0.0ms`; the access
card discloses identity-only pairing above the table; and a new note names
the primary HTTP timeline's own grade beside the three alignment counters,
which cover secondary sources only. The UI-scope non-blocking observations
were taken in the same pass: contract adoption now also rejects
`store_or_file_rescan: true` and any grade/confidence vocabulary the closed
token sets cannot name (O3), and the Workspace mount gates on
`correlationCandidates(entries, "http")` instead of the Diff selector while a
contract-bounded top-N input joins the provenance input identity (O5). O1,
O2, and O4 are engine-scope and remain open as non-blocking. **The 2026-07-31
X-RG1 re-review returned a second `CONDITIONAL`**. It independently verified
B2 fixed, B1 substantially fixed, the Claude renderer remediation complete,
and UI observations O3/O5 closed, but found blocking B3: an earlier shape
candidate's `bestDelta`/`bestClockCompared` state can leak into a later
request-ID winner whose own timestamp is unusable, producing a delta and
overlay certification attributed to the wrong record. O6–O10 are new
non-blocking observations; O1/O2/O4 remain open. **Codex B3 remediation is
complete**: candidate selection no longer owns emitted clock state, and the
delta/clock flag are recomputed only from the final selected record. The
adversarial regression and full Go/frontend verification pass; no B3 renderer
change is required.
**The 2026-07-31 B3 narrow re-review returned `PASS`**: the prior probe no
longer reproduces, the new regression fails against the pre-fix analyzer and
passes now, nine adversarial access-path probes found no residual
misattribution, and the same-class sweep of the profile/Jennifer correlators
is clean. B1/B2/B3 are all closed, X-RG1 has no remaining blocking finding,
and T-583 is `DONE`. O1/O2/O4/O6–O11 remain non-blocking hardening for the
next relevant surface change. T-584 / R-RG1 is now unblocked.
**T-584 / R-RG1 closed with the narrow re-review `PASS` on 2026-08-03**:
the existing platform matrix now names an uncached native Windows
live-capture E2E, measures 50 real `GetExtendedTcpTable` resolver samples with
two owner-table reads and process-start confirmation per sample, and archives
bounded schema-v1 cost evidence without paths or session identifiers. The
same gate now runs frontend state regressions as validation and adds a macOS
offline access-log/Lighthouse import plus Wails package/signature smoke. Local
full Go test/vet/build, frontend state/build, both offline imports, macOS
package/signature verification, and Windows procmap test cross-compilation
pass. Pushed Engine Native run `30601745253` then passed all five jobs:
Windows native Go/race/live-E2E passed and its 50/50 confirmed resolver samples
measured mean 0.539 ms, p50 0.547 ms, p95 0.731 ms, and maximum 0.857 ms. The
parallel CI run `30601745284` passed as well. The first independent review
returned `CONDITIONAL` on release-note coverage, the omitted Windows GUI smoke,
and a CRLF/LF checksum mismatch. The unreleased notes now cover the full
post-0.3.5 wave, Windows GUI smoke run `30809473903` passed at `f5efea8`, and
the evidence sidecar pins the LF-normalized repository artifact while retaining
the original CRLF digest as provenance. The narrow independent re-review
verified all three closures — release notes cross-checked against the full
post-0.3.5 history, GUI smoke run `30809473903` confirmed from GitHub metadata
at `f5efea8` with script-enforced 15-second liveness, and both digest forms
recomputed from the archived bytes — and returned `PASS`. T-584 is `DONE`, no
blocking R-RG1 finding remains, and the version tag / GitHub release is now an
unblocked, separate release decision. No product source was changed by the
gate.
**T-586 is complete**:
the dedicated Lighthouse desktop page now consumes only `AnalyzeBrowserAudit`
and connects the result to Analysis Workspace and Evidence Board. **T-579 / H-RG1
is complete with an integrated `PASS`** after the engine remediation and H-SEC1
re-reviews passed and the bounded UI contract, populated-state regressions,
Workspace/provenance behavior, full Go suite, vet, Go build, frontend state
suite, and production build were rechecked together. **T-571 / H-RG2 is complete
with independent H-COV1 `PASS`** after a `CONDITIONAL` first review and
remediation: ETW is discarded, WFP is removed as a product candidate, direct
TCP-owner supports individual persistent-endpoint attribution only, and
`CAP-3 N/A` forces counter fallback while forbidding absolute ratios. T-585
completed the independent Lighthouse file-first engine slice.

| Status | Count | Meaning |
|---|---:|---|
| `TODO` | 0 | Ready to start; prerequisites are satisfied |
| `IN_PROGRESS` | 0 | Implementation or documentation work is actively underway |
| `REVIEW` | 7 | Review or required remediation is underway; the gate has not passed |
| `PENDING` | 5 | Waiting for a prerequisite, external condition, or activation trigger |
| `DONE` | 202 | Completion criteria and required review gates have passed |

The normal transition is `TODO → IN_PROGRESS → REVIEW → DONE`. A `PENDING`
task returns to `TODO` when its dependency or trigger is satisfied. Task status
is separate from review-group verdicts (`PASS`, `CONDITIONAL`, or `FAIL`). When
a Task Registry status changes, update the counts and Open Work Snapshot in the
same change so this overview remains the status source of truth.

### Desktop-only cleanup (2026-11-10)

T-619 is DONE. The product remains Wails 3.1.0 on macOS, Windows and Linux.
Removed server/Docker-hosting tasks and Dockerfile, unused Android/iOS templates,
mobile CSS fetch and bulk template-regeneration tasks, standalone npm preview,
and unused starter images. Server/mobile build tags now fail compilation;
the frontend imports and mounts App only after detecting the native desktop
transport. CI checks the target/dependency policy and actual server build refusal.
Vite and native WebKit/WebView2/WebKitGTK remain required desktop tooling.

Electron HTTP traffic generation is isolated in an acceptance-only fixture and
requires `-IncludeElectronClient`. Omitting it remains a failure of the full
four-client acceptance contract. Historical evidence and archived source remain
reference-only. Updated paired native/user guides and stale install commands.

Validation: lockfile-based npm install (zero audit vulnerabilities), frontend
state regressions/build, full Go tests/vet, actual macOS `task package` and strict
codesign verification, Windows x86/x64 GUI cross-builds, and server compilation
rejection passed. Native Wails transport cases and browser/Electron rejection
are covered by state regressions. No browser product preview was launched.
This is build/local contract evidence; native GUI interaction and the changed
PowerShell acceptance flow were not re-run on Windows. T-601/T-608/T-611/T-613
retain their existing acceptance requirements.

Local-only cleanup stopped and removed the owned `/private/tmp/ArchScope-UI-Audit.app`
and removed approximately 1.1 GiB of unused Playwright browser/media downloads.
Shared cache metadata and user browsers were retained. Existing unrelated local
HTTP notes and the native verifier were preserved outside this commit.

### macOS UI preflight (2026-11-10)

Native QA preparation fixed duplicate inspector tab IDs, nested option/help
buttons and macOS Command accelerators. Production build checks are separate
from visible-window acceptance. Desktop control twice returned a locked Mac
with failed auto-unlock; window/dialog/splitter interaction awaits an unlocked
remote session. T-605, T-606, T-609 and T-612 are implemented and locally verified; visible native interaction remains open.

### Follow-up implementation verification (2026-11-10)

T-611 local control, T-613 harness preparation, T-601 backend/lifecycle hardening
and T-608 format-evidence preparation are committed separately. Product gates
remain open as recorded in the registry. Full Go tests/vet, focused race checks,
frontend state/build, Mac production build and Windows amd64/386 desktop cross-build
pass locally. The four additional Mac-executable follow-ups are tracked below;
Windows runtime acceptance and genuine-format gates remain separate.

### Mac follow-ups approved on 2026-11-10

| Work | State | Evidence / remaining action |
|---|---|---|
| Windows x86 desktop build | DONE | Guarded temporary Wails module copy/modfile fixes the two overflowing fmt arguments without changing the 2 GiB limit, SDK pin or module cache. Full x86/x64 GUI-subsystem cross-builds pass; CI cross-build matrix added. Native execution/installer/client reach remain unverified. |
| Desktop command wiring | DONE | Shared active-workspace menu/keyboard routing now connects Save/Export, local Help, Find and group collapse/expand with IME/editing/modal/echo guards and single save dialog. State tests and frontend/Mac build pass; native interaction stays in T-613. |
| T-611 shortcut launch | DONE | Implemented read-only IShellLinkW/IPersistFile target/argument/cwd extraction, bounded validation and suspended direct executable launch. Common tests and x86/x64 test compile/vet pass; prepared native Unicode/argument/cwd test awaits Windows. Legacy COM automation remains separate. |
| T-613 resilience checks | DONE | Reproduced/fixed stale-stop trust removal, restart/cleanup race, canceled-start/running cleanup, late hydration rollback and queued canceled automation. Full Go/vet, focused race and renderer tests pass; 60.251s real local-proxy soak completed 72 cycles/18,720 synthetic requests with bounded rows, matching counters and port reuse. Native Windows/multi-hour/reference parity remains unverified. |

The scoped implementation/verification above is complete; this does not close
T-601/T-608/T-611/T-613. Hashed source/build and local validation evidence is in
[`local-validation.json`](docs/review/evidence/2026-11-10-mac-followups/local-validation.json).

### Remaining implementation and Wails 3.1 (2026-11-10)

Final local/cross-build evidence: `docs/review/evidence/2026-11-10-remaining-wails31/local-validation.json`.
Go suite/vet, capture/application race tests, frontend state/build and Windows x86/x64 GUI/COM test compilation pass. Native Windows execution and full replacement parity remain unverified. The COM vtable prefix is guarded at compile time; prepared PowerShell scripts have not been executed here.

- Wails upgrade: DONE locally. Go/CLI v3.1.0 and npm runtime 3.1.0 pinned,
  bindings regenerated, CI/packaging pins updated and obsolete x86 patch removed.
  Full Go and frontend tests/builds plus Mac/Windows 386/amd64 GUI builds pass.
- T-611: native same-user/integrity IDispatch COM adapter, owned per-user registration, metadata Log enumeration and WinINet cleanup implemented. Bundled API DISPIDs/enums recorded. Original dual-IID/typeinfo/events/filter/retention/session-cookie semantics and Windows runtime acceptance remain open.
- T-601: opt-in WinDivert 2.2.2-A IPv4 TCP adapter, original-route/process mapping and HTTP/TLS stream integration implemented and locally tested. Native driver injection/reach remains unverified.
- T-608: proprietary format implementation still requires genuine format evidence;
  no sample is available and no synthetic compatibility claim is permitted.

### T-557 Go drilldown / CLI / report completion (2026-11-10)

The user activated the conditional follow-up. `ProjectJenniferDrilldown` now
projects the existing AnalysisResult with contract 1.0.0; original inputs are
never reopened. Single GUID/TXID and average signature/application selection,
cycle-safe matched descendants, GUID+TXID table joins, one slowest matching
profile per eligible GUID and explicit candidate/sample/excluded counts are
shared by Wails, CLI and exports. The renderer no longer computes subtree
costs or selected-application average edges. Whole-result custom/unprofiled
rollups remain separate from the selected projection; unrecomputed parent
parallelism/custom slices are omitted from subtree metrics.

The MSA tab saves selected JSON/HTML or adds that same projection to Workspace.
Changed selectors/results invalidate export immediately and ignore late replies.
A required common Metadata fix preserves flattened extensions across JSON/Wails
round trips. Full and desktop-compacted edge inputs now produce identical output.

Validation: full Go suite/vet, model/analyzer/exporter/application race tests,
CLI/core/service/JSON/HTML parity, frontend state/build, native Mac build and
Windows x86/x64 cross-build pass. Original renderer comparison passed 2 TXID
scopes / 28 response components / 2 average selections on the existing parser
fixture. Isolated Playwright browser checks used real Go result snapshots with
mocked IPC: pending export disabled, rapid mode changes rejected a stale reply,
and current scope added to Workspace. Browser-only warnings were the Wails
preview notice, existing ECharts deprecation and a missing favicon. These checks
are not native Wails dialog/GUI or Windows execution evidence. Known macOS linker
deployment-target warnings remain. Evidence:
`docs/review/evidence/2026-11-10-t557/local-validation.json`.

### Other-feature fix completion (2026-11-10)

All five fixes passed full Go tests/vet, changed-path/application race tests,
frontend state/build and Mac arm64 / Windows x86+x64 GUI builds. Wails 3.1.0
bindings regenerate without source changes. Isolated real-page browser smoke
shows timeline 120/130 with omitted 10 (critical 1), and SLO targets without
matching evidence explicitly not assessed. Final guards reject unknown legacy path display semantics and retain nginx dash-duration requests as unavailable latency. Native Wails/Windows execution was not performed.
Known Mac deployment-target linker warnings remain. Evidence:
`docs/review/evidence/2026-11-10-other-features/fix-validation.json`.

- T-618 DONE: bounded timeline with full-candidate severity/time summaries and explicit loss accounting in UI/JSON/HTML; both caps, severe late event and empty-input regressions pass.
- T-617 DONE: parser-level latency availability, nullable measured-only statistics with separate traffic denominators, coverage UI and explicit no-evidence SLO status; parser/analyzer/CLI/Flow/SLI/frontend regressions pass.
- T-616 DONE: identity-based logical RPC pairing, defined perspective/error policy, incoming server-peer direction and preserved retry attempts; Go/CLI and Flow/SLI regressions pass.
- T-615 DONE: validated observed-chain elapsed estimate, nullable unavailable paths and separately labeled cumulative work; Go/CLI, deep/malformed graph and frontend/SLO/timeline regressions pass.
- T-614 DONE: rebuild current-source derived measurements and preserve validated saved SLO policies; frontend state regressions and production build pass.

### Other-feature correctness audit (2026-11-10)

Focused review outside HTTP Analyzer/T-557 reproduced five bugs at `406038d`:
Report Pack reuses derived artifacts after their source is removed/replaced;
Trace Import adds inclusive ancestor/descendant durations as critical-path
latency and double-counts a paired CLIENT/SERVER RPC; missing access-log latency
is treated as zero; and Incident Timeline caps can silently omit later severe
events while publishing retained-only severity/time summaries.

Reproductions: a contained 100/80/70 ms span chain gives 250 ms critical path
for a 100 ms trace and two calls for one RPC through the actual Zipkin parser;
one unknown plus one 1000 ms access duration gives average 500/p95 950 ms
without a warning; 130 ordered incident events yield 120 emitted, zero critical
instead of the final critical event; a freshly generated timeline can refer to
REMOVED-SOURCE when the Report Pack lists only CURRENT-SOURCE.

The original reproductions above remain review history; current fixes are recorded in the Task Registry. The pre-fix targeted suites passed without detecting these inputs; the new regressions now cover them.
All five can be implemented and regression-tested on Mac. Preserve the common
AnalysisResult contract and do not infer native desktop or Windows acceptance.
CSV canonical-table export and stronger multi-thread identity were left as
separate design/feature candidates, not promoted as newly proven defects.

Evidence: `docs/review/evidence/2026-11-10-other-features/local-reproductions.json`.
Processed EN/KO reviews: `docs/review/done/2026-11-10_other-features-audit.en.md`
and `docs/review/done/2026-11-10_other-features-audit.ko.md`.

### Open Work Snapshot

Desktop layout across all 21 routes: [Korean design](docs/ko/DESKTOP_CONTENT_LAYOUT.md) · [English design](docs/en/DESKTOP_CONTENT_LAYOUT.md). Start with T-620. The design recheck matched all 21 routes and refined the eight existing tasks to cover conditional views, actual menu dispatch, help pinning, viewport sizing, and form-specific acceptance. The four existing HTTP items can proceed independently as Windows and reference inputs become available.

| Order | ID | Priority | Status | Why it is not `DONE` / next action |
|---:|---|---|---|---|
| 1 | T-620 | P1 | `PENDING` | Measure all 21 views at the default window size. Ready for native baseline measurement. |
| 2 | T-621 | P1 | `REVIEW` | Move global navigation to native menus. After T-620. |
| 3 | T-622 | P1 | `REVIEW` | Compact shared input controls and help. After T-620. |
| 4 | T-623 | P1 | `REVIEW` | Prioritize the HTTP grid and inspector. After T-621, T-622. |
| 5 | T-624 | P1 | `REVIEW` | Prioritize profiler and GC results. After T-621, T-622. |
| 6 | T-625 | P2 | `REVIEW` | Convert nine remaining analysis views. After T-621, T-622. |
| 7 | T-626 | P2 | `PENDING` | Convert eight workspace views and Settings. After T-621, T-622. |
| 8 | T-627 | P2 | `PENDING` | Verify all views in native desktop windows. After T-623–T-626. |
| 9 | T-601 | P1 | `REVIEW` | Deliver the four Windows capture entry points and measure their reach |
| 10 | T-608 | P2 | `PENDING` | Read and write legacy .halog files |
| 11 | T-611 | P3 | `REVIEW` | Support Stand-alone automation and desktop launch switches |
| 12 | T-613 | P3 | `PENDING` | Verify complete Stand-alone parity on Windows |

### Local Windows validation (2026-11-09)

The Windows checkout was fast-forwarded from `d348bcb` to `a9896c3` (12
commits) with no conflicts; the pre-existing untracked `.tmp/` was preserved.
Native Go test/vet/build, frontend state/build, the pinned-Wails Windows build,
and 15-second GUI startup smoke passed. A new native two-client regression
confirms single/multiple PID-plus-start-time selection through persistence.
Resolver measurement confirmed 50/50 samples (mean 3.314 ms, p95 6.116 ms).
The sandboxed failures were rechecked outside the sandbox and passed.

After the user directly acknowledged the notice, native WebView2 testing
verified two real clients, process-scoped live rows, Headers inspection during
new arrivals, the truthful omitted-Body view, active-tab retention, F9/F2 and
stop/reopen detail for five persisted requests. It exposed a stale progress
event overwriting a completed grid row; a monotonic phase merge and failing-
then-passing regression fixed it, and the rebuilt native app retained both
clients' `200 OK` rows.

The follow-up Windows CI also exposed a Clear boundary bug: requests admitted
after Clear could share its wall-clock timestamp and be dropped. Requests now
carry their admission generation through HTTP/HTTPS/tunnel completion; stale
and paused admissions stay excluded. The exact-timestamp regression, HTTPS /
passthrough generation checks, 100 Windows recording-control repetitions and
the full Go/vet/build checks passed. SEC-10 CI cleanup now skips restoration
when policy preparation never ran.

**T-603 is now `DONE`:** the user authorized temporary policy isolation, and
the actual Windows app captured six synthetic POST request/response bodies
from two clients. Process selection, Headers/Body inspection during arrivals,
F9/F2 and stop/reopen retained the expected markers and Korean payload. Both
machine and user registry trees were restored to their original hashes,
including values/subkeys and owner/group/DACL metadata. The policy helpers now
use native registry renaming and have a protected-ACL regression. Evidence:
[`2026-11-09-windows-body`](docs/review/evidence/2026-11-09-windows-body/README.md).
Local race execution needs
a C toolchain, but the first validation commit's Windows CI race/live E2E,
SEC-10 and GUI smoke passed. Existing macOS runner configurations were
unsupported, so the overall Engine Native workflow was cancelled despite its
successful Windows/Linux/frontend jobs. Counts and prerequisite states are
updated for T-603 above. Earlier commands, limits and native screenshots:
[`2026-11-09-windows-local`](docs/review/evidence/2026-11-09-windows-local/README.md).

## Current Baseline

- Active product: unified Go/Wails desktop implementation under
  `apps/engine-native`.
- Active UI: Wails v3 React frontend under
  `apps/engine-native/cmd/archscope-app/frontend`.
- Active engine: Go parser/analyzer/exporter/AI interpretation modules under
  `apps/engine-native/internal`.
- Release baseline: `v0.3.5` is the latest stable GitHub release. The
  `v0.3.1-rc1` prerelease remains available as the Jennifer MSA network-time
  release candidate.
- Current execution focus: deliver the V7.6.4 Stand-alone workflow in Windows Wails v3 (T-594 through T-613).
- Prior GC/profiler cycle: T-589 through T-593 are DONE after task-specific
  regressions, final local source review, and whole-engine/frontend validation.
  No active bug TO-DO remains from the GC/profiler audit. T-557 was subsequently activated by the user and completed on 2026-11-10. On 2026-11-09 the user assigned all five fixes, including UI,
  to Codex and requested one commit per completed task, followed by a push only
  after all five are complete. This overrides the prior UI ownership for these
  five fixes only. T-587 and T-588
  are `DONE`; their post-remediation reviews passed, and the post-change R-RG1
  re-run passed at product commit `7af0f7a` in Engine Native `31488911063`,
  general CI `31488911030`, and Windows GUI Smoke `31489687652`. The version
  tag and GitHub release remain a separate decision; the new bug queue and
  verification of any remediation must be considered before a release cut.
  T-583 / X-RG1 closed with `PASS` on 2026-07-31 after the B3 narrow
  re-review verified candidate selection and emitted clock evidence are
  structurally separated, the regression is genuine against the pre-fix
  analyzer, and nine adversarial probes found no residual misattribution.
  The R-RG1 gate ran the full Go/frontend gates, native Windows live E2E
  including the deferred resolver-cost measurement, macOS
  offline-import/package smoke, paired documentation and
  support/security/performance matrices, and honest release notes (reviews
  archived under `docs/review/done/`).
  T-582 /
  H-RG5 HTTP-specific session Diff closed
  with a group `PASS` on 2026-07-30 (backend, regenerated Wails bindings, and
  the Claude-owned grade-aware comparison UI all verified; review archived at
  `docs/review/done/2026-07-30_claude-code_H-RG5_http-session-diff-group-review.md`).
  T-581 / H-RG4 Windows live-capture UI and E2E
  closed with an independent `PASS` on 2026-07-30: the narrow fourth re-review
  verified V1 (the replacement artifact's 1,012 rows are all loopback, the
  privacy declaration is derived from the published output and
  contradiction-checked, Edge/Electron run isolated from background
  networking, and the Go acceptance fixture binds these mechanisms), V2
  (paired plans synchronized), and V3 (resolver-cost measurement deferral to
  R-RG1 recorded). The re-review is archived at
  `docs/review/done/2026-07-30_claude-code_H-RG4_windows-live-capture-ui-e2e-fourth-re-review.md`.
  T-580 / H-RG3 closed with an independent H-SEC2
  `PASS` on 2026-07-28:
  redaction runs before every persistence, the MITM proxy stores no plaintext
  bodies, the CA private key is memory-only and non-exportable, upstream TLS is
  always verified, the proxy is loopback-only, and no CLI/headless path can start
  a capture. SEC-10 dump-exclusion preflight (before body capture) and SEC-17
  explicit unknown-attribution opt-in (before the live UI) bind the next tier and
  do not reopen the gate. T-571 /
  H-RG2 closed with independent H-COV1 `PASS` on 2026-07-27 after the
  `CONDITIONAL` remediation. The real-NIC run
  real-NIC run discarded ETW for process attribution after 197 false
  attributions, found WFP unable to attribute the control flows, and validated
  direct `GetExtendedTcpTable` TCP-owner attribution at 5/5 persistent endpoints
  with zero false attribution, bypass detection, and 4.8%p CAP-5 CPU overhead.
  The approved contract conservatively removes WFP,
  `CAP-3 N/A` forbids absolute coverage ratios, the five counters are retained,
  and CAP-6 production privilege validation is deferred to H-SEC2/H-RG3.
  The parallel local track completed T-585 Lighthouse report JSON engine/CLI/
  Wails-binding support and T-586's desktop UI. The T-586 backend provides a
  versioned UI contract, score provenance disclosure, stable Evidence Board
  source references, export coverage, and a populated/redacted Wails acceptance
  fixture; the frontend now adds a dedicated Lighthouse page that consumes only
  `AnalyzeBrowserAudit`, discloses imported-score provenance, renders
  category/Core Web Vitals/audit/network/resource views and diagnostics,
  registers to Analysis Workspace, captures findings/audit rows to the Evidence
  Board, and exports JSON/HTML reports.
  H-RG1 passed its integrated gate on
  2026-07-21; the engine remediation and H-SEC1 re-reviews are `PASS`, and the
  UI bounded-result behavior and available populated-state evidence were
  accepted together under the Phase 1 contract.
  The engine now includes the canonical transaction/timing/fidelity model,
  staged dialect normalization, resource guards, field-aware redaction,
  manifest-driven shared-fixture goldens, bounded results, and CLI/Wails parity.
  The independent C-RG1 re-review returned
  `PASS` on 2026-07-21 after verifying all engine A-E and UI U1-U5 remediations,
  so T-578 is complete and H-RG1 is unblocked. Close the T-571 Windows real-NIC
  coverage proof before live HTTP capture. Codex owns
  engine work and Claude owns UI work; each review group must pass before the
  next group starts. For T-582 and later, this is a hard execution boundary:
  Codex stops after backend/engine contracts, generated bindings, fixtures, and
  engine verification, while Claude exclusively owns React/UI/state/i18n and
  visual/frontend regression work unless the user explicitly reassigns it.
- Retired implementation: Python/FastAPI/browser sources are archived under
  `archive/python-engine` and `archive/web-frontend-python`.
- Historical native POC module has been folded into `apps/engine-native`.

## Completed In This Cycle

- Consolidated the Go engine, profiler core, Cobra CLIs, and Wails desktop app
  into `apps/engine-native`.
- Archived the Python engine and browser frontend instead of deleting them.
- Ported the evidence-bound AI interpretation module to Go.
- Updated release packaging around the Go/Wails desktop artifact.
- Rebuilt the `v0.3.0-rc1` release contents and changed the release from
  prerelease to latest.
- Verified representative Go parser/analyzer packages and profiler benchmarks.
- Audited large-file parsing behavior across logs, JSON inputs, profiler files,
  and thread-dump plugins.
- Aligned the active Go baseline with the latest stable Go toolchain
  (`go1.26.3`) and updated CI/release workflows to use `1.26.x`.
- Completed release verification for `v0.3.0-rc1`: local macOS Wails package,
  DMG validation, CLI analysis/export smoke tests, GitHub Release matrix review,
  and release asset checksum validation.
- Changed Jennifer profile analysis tabs so Summary, MSA Timeline, Profile
  Timeline, and Parser Report are visible before a file is analyzed.
- Optimized hot parser/profiler paths for common nginx access logs with
  response time and collapsed-stack flame graph construction.
- Optimized large graph rendering paths by capping displayed MSA timeline bars,
  reducing ECharts animation/resize churn, and using row-bucket hit testing for
  canvas flame graph hover lookup.
- Removed POC-era profiler-suffixed command/app path names from the active
  source and build surface; the desktop app now builds from
  `apps/engine-native/cmd/archscope-app`.
- Added Jennifer MSA service-call network-time summaries and network-aware
  topology placement so internal single-digit millisecond calls and
  gateway/external double-digit millisecond calls separate visually.
- Added `v0.3.1-rc1` and promoted the line to the stable `v0.3.1` desktop
  release.
- Added external trace import MVP support for OTLP JSON-file traces and
  Zipkin v2 JSON traces under `internal/parsers/traceimport` and
  `internal/analyzers/traceimport`.
- Added `archscope-engine trace import --in <file> --format
  auto|otlp-json|zipkin-v2-json` and trace sample fixtures under
  `examples/traces`.
- Researched product expansion and external APM import priorities around
  local-first trace evidence, Evidence Board, Incident Timeline,
  SLO/Golden Signals, service flow, and deferred SaaS connectors.
- Consolidated product expansion and external APM planning into the English and
  Korean roadmap documents, then removed the former Korean-only planning notes
  so `docs/en` and `docs/ko` Markdown files stay paired.
- Restored and extended GC log memory-space analysis so young/old/metaspace
  event series are emitted again, and added OOM plus long-pause alert findings.
- Connected `trace_import` to the Wails desktop UI with summary cards,
  service dependency and service latency charts, trace/span tables, critical
  path rows, parser diagnostics, and findings.
- Added Elastic APM file import for Elasticsearch `_search` response JSON and
  source-only NDJSON exports.
- Added trace critical-path analysis and MVP findings:
  `SLOW_TRACE_P95`, `CLOCK_SKEW_SUSPECTED`,
  `UNBALANCED_SERVICE_LATENCY`, and `HIGH_ERROR_SERVICE_EDGE`.
- Added the first Evidence Board skeleton with local evidence cards for trace
  findings, service edges, traces, and source metadata.
- Added a Windows GUI smoke workflow that builds the Wails Windows executable
  on `windows-latest`, launches `archscope.exe`, verifies it stays alive, and
  shuts it down cleanly.
- Hardened the 0.3.x release workflow with macOS signing/notarization secret
  preflight, signature/stapler verification, route-level frontend lazy loading,
  modular ECharts bundling, and explicit frontend chunk budgets.
- Clarified the JFR analyzer contract: the JDK `jfr` CLI is only the `.jfr` to
  `jfr print --json` conversion boundary, while ArchScope focuses on
  report-oriented summaries, stack evidence, hints, and Evidence Board capture.
- Added async-profiler-oriented JFR stack aggregation from `stackTrace.frames`
  into top methods, packages, threads, sample stacks, and a flamegraph-ready
  call tree.
- Added JFR recording UX hints for empty filters, missing stack samples,
  async-profiler-style sample recordings, and sparse stack samples.
- Expanded Evidence Board capture beyond Trace Import to Access Log findings,
  GC findings/alerts, JFR findings/profile rows, and native-memory call sites;
  added local HTML report and JSON evidence-pack export.
- Restored the selected Wails workspace/export parity surfaces: Analysis
  Workspace, Export Center, general report diff, and Chart Studio MVP.
- Added a session Analysis Workspace that retains successful analyzer outputs
  across page navigation and connects those results to Evidence Board,
  Export Center, report diff, and Chart Studio workflows.
- Prepared the stable `v0.3.2` desktop release with app/package metadata,
  Windows executable version metadata, changelog notes, local Windows build,
  and GUI launch smoke verification.
- Added Jaeger QueryService/local trace JSON import and schema-guarded
  SkyWalking GraphQL `queryTrace.spans` validation to the `trace_import`
  parser, analyzer tests, CLI format list, Wails Trace Import selector, and
  English/Korean data-model and roadmap documents.
- Added the Wails Incident Timeline MVP with a common session event model,
  cross-analyzer event mapping, filtering, and Evidence Board capture.
- Added the Wails SLO / Golden Signals MVP with session-wide signal inventory,
  SLI metric normalization, default SLO target evaluation, violation/error
  budget tables, affected-scope breakdowns, and Evidence Board capture.
- Added the Wails Service Flow MVP with a shared Trace Import/Jennifer MSA
  service-edge model, deterministic service-flow findings, Evidence Board
  capture, Mermaid sequence-like export, and JSON export.
- Prepared the stable `v0.3.3` desktop release with updated app/package
  metadata, changelog notes, and 0.3.x Evidence Studio release scope.
- Published the stable `v0.3.4` desktop release with Incident Timeline mapping
  fixes, AI gate hardening, SLO unit/deduplication fixes, report-pack ZIP path
  sanitization, derived export alignment, and frontend state regression tests.
- Published the stable `v0.3.5` desktop release with Mid-Term Plus file-first
  evidence importers, API/event contract analysis, architecture documentation
  drafts, advanced stitching, app/package metadata updates, and changelog
  notes.
- Added the Mid-Term Plus shared ingestion foundations for `v0.3.5`: evidence
  family boundary specs, reusable source-format registry, golden fixture
  diagnostic harness, normalized source metadata, and cross-source correlation
  key model.
- Added the access/edge-log coverage wave as part of `v0.3.5`: Tomcat/Jetty,
  HAProxy HTTP, Envoy/Istio text and JSON, AWS ELB/ALB/CloudFront, GCP/Azure
  edge JSON, IIS W3C, Caddy/Traefik JSON, and Kong/Tyk/API Gateway JSON parsing
  now feed additive access-log summaries, edge dependency tables, Service Flow,
  and Golden Signals.
- Built the T-571 Windows proof-of-capability harness under
  `spikes/t571-windows-coverage` (standalone Go module: loadgen + ETW/WFP/
  TCP-owner probes + a loopback-aware judge that emits the appendix-A ledger
  rows) and ran the first single-machine loopback measurement. Confirmed the
  ETW Kernel-Network event header PID + payload `sport`/`dport` (0 event loss) and that it
  does not observe loopback, and reconfirmed `GetExtendedTcpTable` owner-PID
  attribution at 100% (5/5). `Q-WIN-ETW-PAYLOAD`/`Q-WIN-WFP-ATTR` moved
  open→partial; ETW/WFP real-NIC attribution re-run remains.
- Completed the T-571 Windows real-NIC measurement against a separate LAN
  receiver at `192.168.0.3:18080`, then replaced PowerShell
  `Get-NetTCPConnection` polling with direct IPv4/IPv6
  `iphlpapi!GetExtendedTcpTable`. The direct TCP-owner rerun sustained 30 polls
  in 30 seconds, retained 100% (5/5) PID attribution, zero false attribution,
  and bypass detection, and reduced CAP-5 from 14.4%p to 4.8%p. ETW was
  discarded after 197 false attributions despite zero loss; WFP remained
  unsupported in the measured configuration. The first H-COV1 review was
  `CONDITIONAL`; counter-fallback remediation then passed independent re-review,
  closing T-571/H-RG2.
- Completed the T-574 MITM proxy build-versus-integrate spike (gate 8). Surveyed
  Go/Python proxy candidates by license/maintenance/H2, produced a 4-option
  comparison matrix and a risk register, and **chose option 3 — an own H1
  semantic MVP with H2 passthrough behind a Proxy/Interceptor interface** (option
  4 native-H2 as a later graft, mitmproxy subprocess as a documented fallback).
  De-risked it with `spikes/t574-mitm-proxy-poc`, a pure-stdlib PoC whose offline
  MITM self-test passes and which cross-compiles for Windows.
- Closed the Chrome/V8 release implementation follow-up by wiring the shared
  15-file fixture manifest into Go golden tests, fixing V8 `children` graph and
  Chrome `ProfileChunk` delta handling, supporting hitCount-only and clamped
  negative-delta inputs, suppressing invalid downsampled timelines, refreshing
  Wails bindings, and completing the local release smoke.
- Upgraded the desktop baseline to Wails `v3.0.0-alpha2.117` with the matching
  `@wailsio/runtime` alpha.97 package, refreshed all outdated direct frontend
  dependencies (including React 19 and TypeScript 7), moved Linux CI and
  packaging to GTK4/WebKitGTK 6.0, upgraded official GitHub Actions to their
  Node 24-based majors, and revalidated the signed macOS app and DMG.
- Converted the Chrome/V8 and HTTP-capture designs into a paired English/Korean
  implementation plan with Codex engine ownership, Claude UI ownership,
  sequential group reviews, and mandatory individual gates only for security,
  coverage, CA/privilege, and Long Task semantics.
- Closed C-RG1 with an independent `PASS` after all ten engine/UI remediation
  findings were re-verified against the 16-fixture goldens, engine tests,
  frontend state regressions, `go vet`, and full builds; H-RG1 is unblocked.
- Completed the Codex-owned H-RG1 engine slice: canonical HAR transactions and
  explicit timing states, Chrome/Firefox/Safari/Charles/Fiddler/Proxyman/
  Insomnia/generic dialect normalization, bounded gzip/JSON/body parsing,
  field-aware default-on redaction, bounded AnalysisResult aggregation, HAR
  content detection, CLI/Wails option parity, generated bindings, and the
  shared 20-fixture manifest regression suite.
- Processed the H-SEC1 `CONDITIONAL` review: aligned free-text and structured
  sensitive-key redaction, added Luhn card and WS/SSE non-disclosure regressions,
  closed the live-capture CLI question as "not provided", and synchronized the
  English/Korean contracts for capture scope minimization, crash-dump limits,
  platform CA ACLs, complete NSS-store enumeration, leaf-key non-persistence,
  and honest logical deletion. The independent security re-review returned
  `PASS`; Phase 2+ SEC-8/10/16/17 implementation measurements remain assigned
  to H-SEC2 and do not reopen the offline HAR gate.
- Added the independent T-585 Lighthouse file-first browser-audit engine slice:
  bounded/redacted `lighthouse-json` parsing, preserved Lighthouse category and
  audit scores, Core Web Vitals/audit/resource projections, deterministic
  findings, `browser import`, a distinct `browser_audit_evidence` ingestion
  family, Wails `AnalyzeBrowserAudit`, generated bindings, and paired EN/KO
  data-model/importer/roadmap documentation.
- Completed the T-586 backend handoff while leaving the task `IN_PROGRESS` for
  its frontend scope: the single `AnalyzeBrowserAudit` result now exposes a
  versioned view/evidence/export contract, explicit imported-score provenance,
  stable `source_ref` values, source metadata, and JSON/HTML report coverage.
  A populated sanitized Lighthouse fixture verifies TopN bounds, Wails payload
  shape, Workspace provenance, Evidence Board candidates, and non-disclosure.
- Completed the T-586 frontend scope: a dedicated `LighthouseAuditPage` under
  Browser Performance that consumes only `AnalyzeBrowserAudit` (new
  `bridge/engine.ts` `analyzeBrowserAudit` wrapper and `bridge/types.ts`
  `BrowserAuditRequest`/`BrowserAuditAnalysisResult`/`BrowserAuditContract`
  types) with collection guidance, an imported-score provenance banner that
  discloses scores are preserved and not recomputed, metric cards, category
  scores, Core Web Vitals, bounded audit and network-request tables, resource
  summary, findings, and parser diagnostics. Pure `state/browserAudit.ts`
  selectors keep the logic testable; the result registers to Analysis
  Workspace, findings/audit rows capture to the Evidence Board while preserving
  the frozen `source_ref`, and JSON/HTML reports export from the page. Added
  EN/KO localization, sidebar/App wiring, and populated-state/provenance
  regressions; `npm run test:state` and `npm run build` pass and
  `browser_audit_evidence` stays separate from `profile_evidence`.
- Processed the H-RG1 engine `CONDITIONAL` review and completed its remediation:
  malformed URLs retain only redacted raw values with `HAR_URL_UNPARSABLE`, the
  parser has a non-leaking panic boundary, oversized JSON bodies fail closed
  with `HAR_REDACTION_DEGRADED`, non-string `code`/`auth`/`session` values are
  preserved, and byte/field/timing/UTF-8/body-size regressions cover the P2/P3
  findings. Targeted packages, full `go test ./...`, and `go vet ./...` pass;
  the independent engine remediation re-review returned `PASS`. Its remaining
  non-blocking P3 recommendation was also closed: numeric/boolean values are
  preserved only for the ambiguous analysis keys `code`, `auth`, and `session`,
  while hard-secret keys such as password, SSN, API key, token, and session ID
  are redacted regardless of JSON value type. A dedicated regression covers it.
- Processed the H-RG1 UI `CONDITIONAL` review (U1-U6) and applied its first
  remediation pass: brush/filter now recompute the six summary cards over the same
  filtered rows the list and tree render, with a scope banner that discloses the
  bounded-detail floor (U1); MIME, duration-range, and fidelity filters were
  added alongside method/status/query/errors and compose with the brush window
  (U2); a pure `httpCaptureReducer` owns analysis provenance so selecting a new
  file, starting a run, or a failed run drop any prior result (U3); the
  transaction detail is now request/response/timing/process tabs that surface
  cookies, content type, and body storage/redaction (U4); the timeline gained
  keyboard start/end sliders, rows became keyboard-activatable buttons, and
  `SlideOverPanel` got focus move/trap/restore plus `inert` when closed (U5);
  and new pure state regressions cover the shared denominator, the new filters,
  the A→B provenance reset, and Workspace registration (U6). `npm run test:state`
  and the production build pass with EN/KO key parity. The UI re-review's R1/R2
  were dispositioned by the integrated gate against the Phase 1 contract.
- Re-reviewed the H-RG1 UI remediation at `4c814bb`: U2-U5 closed by code and
  state/build evidence, but R1 (P1) remains because full-session timeline data
  is combined with filters and selection summaries over only `topN` inline
  transaction rows; R2 (P2) remains because synthetic pure-state tests do not
  render populated normal/degenerate/truncated/diagnostic/error fixtures or
  exercise actual Workspace/dialog interactions. The integrated H-RG1 check
  accepted R1 because the bounded `AnalysisResult` contract intentionally caps
  inline rows and the UI uses that same disclosed denominator for cards, list,
  and tree without claiming full-session filtered totals. R2 was accepted for
  Phase 1 based on the populated state derivations, provenance/Workspace
  regressions, typed component wiring, and green production build; deeper Wails
  component fixture coverage remains a non-blocking hardening opportunity.
- Closed T-579 / H-RG1 with an integrated `PASS`: all 20 manifest fixtures and
  sanitized real-export paths remain covered, engine and H-SEC1 re-reviews are
  `PASS`, the import-only UI satisfies the bounded denominator and honesty
  contracts, and `go test ./...`, `go vet ./...`, `go build ./...`, frontend
  `npm run test:state`, and frontend `npm run build` all pass.

## Current Risk

The 2026-11-10 non-HTTP correctness risks (T-614 through T-618) are closed locally after implementation and regressions. Trace elapsed values explicitly remain observed-chain estimates rather than causal critical-path proof. The four HTTP native/reference acceptance tasks remain open. The older audit/gate statements below retain their historical scope.

The 2026-11-09 audit identified these correctness/diagnostic risks; see the
Task Registry for closures and remaining work:
GC measurements lose usable time coordinates, Canvas can drop internal self
weight, drilldown can search display-pruned evidence, legacy lock counters lack
unit-aware duration conversion, and ordered regex errors can be hidden. Their
local reproductions and required closure checks are in the active queue below.
The previous gate closures in this section remain historical baseline evidence;
they do not resolve these newly registered cases.

The C-RG1 risk is closed. Its 2026-07-21 independent re-review returned `PASS`
after re-verifying all ten engine/UI remediation findings and the 16-fixture,
engine, frontend, vet, and build evidence. The Codex H-RG1 engine implementation
now preserves the canonical transaction/timing/fidelity contract, applies
bounded parsing and dedicated redaction, and passes the shared fixture suite.
Its remediation re-review returned `PASS`, and the reviewer-only P3 numeric
hard-secret recommendation is now closed with a regression test. The H-SEC1
security re-review returned `PASS` on 2026-07-21. H-RG1's integrated risk is
closed; the bounded UI denominator is explicitly disclosed and its state,
provenance, Workspace, type, and build evidence passed together. Live HTTP
capture can now proceed to T-580/H-RG3 after T-571's independent H-COV1 `PASS`.

The Electron-to-Wails migration risk is closed. The highest large-file issue
found in the 2026-05-09 audit has been mitigated: GC log analysis no longer
emits chart series for every event, and access-log/OTel analyzer entrypoints no
longer materialize the full parser record slice before aggregation.

Release verification found no new blocker for the 0.3.x Go/Wails line. Direct
local Windows desktop testing is still environment-dependent, but automated
Windows GUI smoke now runs on a GitHub-hosted Windows Server 2025 runner. The
smoke builds the Wails Windows executable, launches `archscope.exe`, verifies
that the GUI process stays alive for the startup window, and shuts it down.
Windows confidence now includes tests/builds/packages, release artifact
checksum and PE inspection, plus the GUI launch smoke.

Release hardening now fails fast when macOS Developer ID signing or Apple-ID
notarization secrets are only partially configured. The release workflow also
verifies the built app signature and validates the stapled ticket when
notarization credentials are available. The Wails frontend startup shell is
split from the analyzer pages and shared chart runtime, so the initial bundle
is within the documented release budget.

The MSA timeline discoverability issue is closed: the tab list now renders
before analysis and each tab shows a neutral empty-result state until data is
available. Large MSA timelines are display-capped for browser responsiveness.
Jennifer MSA topology now uses network-time groups to estimate service
distance, while the underlying `AnalysisResult` tables retain detailed call
metrics.

Trace import is now a desktop workflow, not only an engine/CLI MVP. OTLP
JSON-file, Zipkin v2 JSON, Elastic APM `_search` JSON, Elastic source NDJSON,
Jaeger QueryService/local trace JSON, and schema-guarded SkyWalking GraphQL
`queryTrace.spans` responses are covered. The UI exposes summary metrics,
service dependencies, traces, spans, critical paths, parser diagnostics,
deterministic findings, and basic Evidence Board capture. Evidence Board
capture also now covers selected non-trace analyzers and can export local
HTML/JSON evidence packs. Broader SaaS connectors remain roadmap candidates
until they are explicitly promoted into the active TO-DO.

JFR analysis now has an explicit product boundary and async-profiler-oriented
stack evidence, but direct JFR JSON loading is still an object-materializing
path. Large recordings should still be constrained with `jfr print` event,
time-window, or stack-depth filters before analysis.

Remaining large-file risk is concentrated in structured formats that naturally
require object materialization, such as JFR JSON, Node diagnostic reports, jcmd
JSON, and self-contained HTML profiler files. These paths now have documented
guardrails or targeted preflight, but multi-GB structured inputs should still be
filtered before analysis.

## Large-File Audit Snapshot

| Analyzer | Synthetic input | Time | Peak RSS | Output | Status |
|---|---:|---:|---:|---:|---|
| Access log | 30 MB / 300k lines | 0.53s | 19 MB | 10 KB | Streaming parser/analyzer path verified |
| OTel JSONL | 31 MB / 200k lines | 0.25s | 18 MB | 6.3 KB | Streaming parser/analyzer path with trace-detail cap verified |
| GC log | 34 MB / 300k lines | 1.55s | 305 MB | 3.5 MB | Series cap/downsampling verified |
| Collapsed profiler | 29 MB / 500k lines | 0.26s | 16 MB | 6.8 KB | Good baseline |

## Next Execution Queue

### Completed other-feature correctness follow-ups (2026-11-10)

T-614 through T-618 are implemented in individual commits. Full Go/vet/race, frontend state/build, isolated browser UI checks and Mac/Windows cross-builds pass. Continue with the four open HTTP tasks in the snapshot; their native/reference acceptance remains separate.

### IEInspector V7.6.4 Stand-alone replacement (2026-11-09)

The requested baseline is the Stand-alone EXE. The default desktop view has a
compact command bar, a process-grouped request grid, a lower request/response
inspector and scoped summaries. The first visible result is a request arriving
while the session is recording. Existing analysis cards, HAR import, comparison
and correlation remain accessible as secondary actions. Keep the current Wails
v3 shell, capture services, bounded store, redaction and AnalysisResult contract.

The Open Work Snapshot gives execution order. The Task Registry gives the
prerequisite IDs and an observable completion test for every item. T-603 is the
first end-to-end desktop demonstration. T-613 is the full Stand-alone check.
Features listed only for the browser add-ons (embedded browser panel, live
page grouping, Tamper and IE cache-before/after comparison) are outside this
backlog. T-610 supersedes the earlier Request Builder exclusion. T-611 addresses
automation of the visible desktop application. Platform capture coverage and
legacy file compatibility remain open until measured.

The official references distinguish editions and describe V7 comments, connection
IDs, content types, automation and file formats. [Stand-alone and Add-on features](https://www.ieinspector.com/httpanalyzer/features) · [Changes in V7](https://www.ieinspector.com/httpanalyzer/newinv7) · [Export formats and launch switches](https://www.ieinspector.com/httpanalyzer/manual/commandlineinterface.htm)

#### Evidence from the initial desktop UI audit

The Windows Wails v3 desktop app is the acceptance target. The official
[V7.6.4 distribution](https://www.ieinspector.com/httpanalyzer/download),
[Stand-alone screen tour](https://www.ieinspector.com/httpanalyzer/tour) and
[session inspection walkthrough](https://www.ieinspector.com/httpanalyzer/manual/gettingstarted.htm)
describe inspecting a selected request while logging. The project's own
[screen and control specification](docs/ko/SYSTEM_HTTP_CAPTURE.md)
covers the process tree, grid and lower detail pane in sections 8.2–8.5.

At `6749406`, the production Wails target built locally against
`v3.0.0-alpha2.117`. On macOS the native live mode reports unavailable.
A local browser run exercised the actual React screen with synthetic Wails
service responses for two distinct processes and two requests. At 1280×800
with the sidebar expanded, the first running-session request started at y=822.
Clicking a process left both requests visible. Clicking a live row opened zero
detail tabs. After Stop and Load finalized analysis, selecting the same row
showed both headers and the supplied response body. Changing rows from Body
selected Raw again. The path cell was 17 px wide for a short path requiring
51 px. These measurements apply to the browser renderer. The local Mac was
locked when native interaction was attempted. No current Windows capture run
was performed. Archived Windows WebView2 evidence covers live row restoration
on re-entry; it does not exercise these selections. The targeted Go capture
service/fixture tests and frontend state regression suite passed on this baseline.

1. **T-594 (P1).** Combine capture controls, streaming rows and lower detail.
   Collect first-use options before recording, then leave a compact status and
   stop control visible. Pass when a Windows Wails window at 1280×800 can
   select a completed request without stopping capture or scrolling the page,
   inspect both headers and any permitted retained body, and keep the same
   selection as new rows arrive. Show pending, omitted and truncated states.
   The live ring/events strip payloads in `stream/pipeline.go:metadataOnly`.
   Fetch only the selected permitted detail and keep existing caps and redaction.
2. **T-595 (P1).** Use the existing process instance key (PID and start time)
   to select or expand a process group in the main grid. Cover two processes,
   same-name instances, All and unattributed rows. Verify count/byte summaries
   and incoming rows follow display scope while recording continues. HAR origin
   groups must remain visibly distinct from real OS processes.
3. **T-596 (P1).** The Wails start request and engine capture config have no
   target-process selector. Provide explicit single/multiple-process recording
   scope for supported proxy traffic and distinguish it from display filtering.
   Verify with two Windows loopback clients that excluded rows never reach the
   store. Preserve unknown-attribution and body retention policy.
4. **T-597 (P1).** Address the narrow path column and tab reset in
   `HttpAnalyzerView.tsx`. Verify that two short paths can be distinguished at
   default width and that selecting adjacent rows refreshes Headers or Body in
   place without displaying a previous request's values.
5. **T-598 (P2).** Extend the current text preview with decoded query pairs,
   form fields and a JSON tree first. Cover ordered duplicate keys, encoding,
   unavailable bodies and truncated JSON. Review safe XML, image and byte views
   against what the retained evidence actually contains.

Each item can start with the existing contracts/components. Final integrated
verification of the live workflow requires T-594 and T-595 together in a
Windows desktop session. The source and live panels, state and bindings are
under `apps/engine-native/cmd/archscope-app`.

The following work extends the five findings into a replacement plan.

| Surface | Current ArchScope behavior | Remaining work |
|---|---|---|
| Window and commands | Recording controls and finalized analysis use separate panels. The Wails shell already provides the desktop host. | Match reference screen states, implement the command bar and demonstrate the complete recording workflow (T-599, T-600, T-603). |
| Capture coverage | Windows HTTP/1.x proxy with process attribution, partial stages and explicit coverage grades. | Extend capture modes and exchange evidence, including proxy-independent Windows cases (T-601, T-602). |
| Navigation and timing | Fixed columns, basic filters, a finalized waterfall and session metrics. | Field grouping, search, configurable columns, live phase charts and process totals (T-604, T-605). |
| Details and diagnostics | Six detail tabs in the finalized grid, text preview, reconstructed Raw and session findings. | Content/stream viewers, per-row hints, annotations and legacy decoders (T-606, T-609, T-612). |
| Session files | HAR input and an internal capture store. | Desktop save/export, legacy log exchange and format-specific round trips (T-607, T-608). |
| Companion commands | Request Builder and desktop automation were outside the previous implementation. | Add explicit request composition and Windows desktop automation (T-610, T-611). |
| Acceptance | Historical Windows tests cover capture/re-entry. The UI audit above used synthetic renderer data. | Compare native screenshots and observed actions with the Stand-alone reference (T-599, T-603, T-613). |

### Completed GC / profiler cycle

1. **T-589 (DONE; P1, Codex engine and UI):** repair GC time extraction and
   measurement availability. Define the observation window before changing
   throughput/rate arithmetic, then align absolute versus elapsed chart axes.
2. **T-590 (DONE; P1, Codex UI):** preserve inclusive and self sample accounting in
   the shared Canvas flamegraph. Verify ordinary prefix stacks independently of
   pruning, then cover the pruned/hidden-weight representation with T-591.
3. **T-591 (DONE; P1, Codex engine/contracts and UI):** apply
   drilldown filters to canonical evidence before bounded display pruning.
   Preserve the node/payload limits and distinguish hidden descendants from
   actual self samples; do not solve this by disabling memory guards.
4. **T-592 (DONE; P1, Codex engine/contracts/export and UI):** establish explicit
   counter-unit and aggregation provenance for legacy profiles before deriving
   durations. Keep the existing typed V8/profile-evidence units intact.
5. **T-593 (DONE; P2, Codex engine):** prevalidate/precompile ordered regex terms and
   return the same diagnostics as the non-ordered path. This is independent of
   the P1 fixes and can be implemented without changing filter semantics.

All five tasks above are DONE closure records. No active bug implementation
remains from this audit; T-557 retained its external usage trigger at that audit; its later completion is recorded above. This user-requested fix
cycle uses targeted regressions, relevant engine/frontend checks, and a local
source review for closure; no independent-review or platform-E2E verdict is
claimed. Prior independently reviewed release gates retain their own scope. T-557 was still pending its usage trigger at that audit; it was activated and completed on 2026-11-10. That earlier audit did not activate a new feature range.

### GC / Profiler Bug Revalidation — 2026-11-09

The findings below are the original pre-fix reproduction record; current fix
status and validation are in the Task Registry. They are not current failures
for tasks subsequently marked DONE.

The current source was rechecked with temporary synthetic Go API probes and
the actual Canvas hierarchy expression evaluated with the installed
`d3-hierarchy`. Temporary probes were removed. These are local semantic
reproductions, not desktop GUI, real-JVM recording, Windows, or release E2E
evidence. No unprocessed review document was present in `docs/review/`.

- **T-589 — confirmed, fix required.** Unified GC records at `[1.000s]` and
  `[11.000s]` parse two events but leave `uptime_sec` unavailable; their
  timeline becomes `"0"`, `"1"`, and wall time, throughput, and allocation
  rate all report zero. Adding dates preserves the displayed timestamps but
  still leaves the rate denominator zero; date-only input has the same rate
  defect. The input contains a measurable ten-second span and 30 ms of pauses,
  so zero is not an honest unavailable state. Sources:
  `internal/parsers/gclog/parser.go:712`,
  `internal/analyzers/gclog/analyzer.go:566`, and
  `frontend/src/charts/gcLogCharts.ts:175` (paths relative to the active engine
  or its desktop app). Acceptance: uptime-only, date+uptime, date-only,
  single-event, missing-time, and clock-reset inputs; explicit observation
  window/numerator rules; correct axis units; unavailable metrics carry a
  reason instead of measured-looking zeros; paired EN/KO contract docs and
  engine/UI regressions. Oracle documents uptime as a default unified-log
  decorator: [Java 21 logging decorations](https://docs.oracle.com/en/java/javase/21/docs/specs/man/java.html#decorations).
- **T-590 — confirmed, fix required.** With inclusive `All=100`, `A=100`,
  `A/B=40`, the current Canvas expression returns a total of 40 and presents B
  as 100%, losing A's 60 self samples. A leaf-only control totaling 40 remains
  correct, isolating the internal-self case. Source:
  `frontend/src/components/CanvasFlameGraph.tsx:155`. Acceptance: render total
  100 and B at 40% of the original total, preserve A's self weight, and avoid
  double counting on prefix, recursive, focused, searched, and pruned trees.
  Include ordinary non-pruned regressions so T-591 cannot mask this bug.
- **T-591 — confirmed, fix required.** Input stacks `a;hot=100` and
  `a;rare=10`, with `MaxFlamegraphNodes=3`, produce zero matches for an include
  filter on `rare`; the unpruned control produces 10. The desktop service feeds
  `result.Charts.Flamegraph` into the filter after analysis has pruned it.
  The small budget exercises the same branch used beyond the default 100,000
  node cap. At budget exhaustion the pruner can also omit the promised
  synthetic `other` child, leaving hidden descendant weight indistinguishable
  from real self weight. Sources: `cmd/archscope-app/profilerservice.go:392`,
  `internal/profiler/analyzer.go:340`, and
  `internal/profiler/flamegraph.go:245`. Acceptance: a retained canonical stack
  matches with the same count regardless of display budget; filtering precedes
  display pruning; hidden/omitted weight is conserved and explicitly disclosed;
  every returned stage remains bounded. Parser-level input truncation remains
  a distinct disclosed limitation, not a promise to recover discarded input.
- **T-592 — confirmed unit-contract bug, fix required with narrower scope.**
  A legacy `lock` counter of 1,000,000, explicitly interpreted as total
  nanoseconds in the probe, is converted with `IntervalMS=100` into 100,000
  seconds rather than 0.001 seconds. The legacy request has no counter-unit or
  aggregation-mode field. This does **not** mean every lock counter is
  nanoseconds: async-profiler supports sample-count and total-metric output.
  Sources: `internal/profiler/analyzer.go:313`,
  `internal/profiler/drilldown.go:214`, and `internal/profiler/pprof.go:65`.
  Acceptance: explicit sample-count versus total-time provenance, direct
  conversion for a known time unit, no interval multiplication of weighted
  durations, unavailable duration when provenance is insufficient, consistent
  summary/drilldown/export units, and unchanged supported CPU/wall/V8 semantics.
  Resolve the contract before updating generated bindings or EN/KO labels.
  References: [aggregation options](https://github.com/async-profiler/async-profiler/blob/master/docs/ProfilerOptions.md),
  [lock profiling](https://github.com/async-profiler/async-profiler/blob/master/docs/ProfilingModes.md#lock-profiling).
- **T-593 — confirmed, fix required.** Regex `[` in `anywhere` mode emits
  `INVALID_REGEX`; ordered pattern `a>[` instead returns zero matches with
  `diagnostics=null`. Ordered terms are compiled inside the frame loop and
  their errors are discarded. Sources: `internal/profiler/drilldown.go:331`
  and `:368`. Acceptance: validate all ordered terms before traversal,
  including unreachable terms, apply consistent invalid/unsafe-pattern guards,
  compile each term once per filter, preserve existing matching/reroot behavior,
  and verify both include and exclude error cases cannot masquerade as valid
  matches or non-matches.

**Items deliberately not promoted to bug TO-DO:**

- Peak-preserving GC downsampling is an optional visualization improvement.
  A six-event/two-bucket probe retains 1 ms line points while the exact summary
  still reports 1,000 ms and the critical alert retains the omitted peak's
  timestamp/value. This follows the documented `event_bucket_last` policy;
  the earlier suggestion alone does not establish lost summary/alert evidence.
- Drilldown stale-result/race behavior remains a static-code concern; no
  component lifecycle or desktop reproduction was executed in this audit.
  Reproduce input changes, failed requests, and reversed completion order
  before registering a separate confirmed bug.
- GC interval analysis, collector-specific coverage expansion, streaming GC
  aggregation, and profile caching remain optional feature/performance work.
  Synthetic heap-less phase rejection proves a coverage limit, not complete
  ZGC/Shenandoah support or an independently validated collector regression.

### Prior Release Closure and Roadmap Notes

The following records retain the earlier gates and roadmap decisions. They are
historical context, not tasks ahead of the active bug queue above.

1. **T-584 / R-RG1 closure record:** the first review returned `CONDITIONAL`
   on R1–R3. The remediation completed the unreleased feature notes for the
   whole post-0.3.5 wave, restored and ran the Windows GUI smoke (run
   `30809473903` at `f5efea8`), and re-pinned the archived evidence to the
   repository's LF-normalized bytes with the original CRLF digest retained as
   provenance. The 2026-08-03 narrow independent re-review verified all three
   closures and returned `PASS`: the platform gates from the first review
   carry over because the remediation commit changes no buildable surface.
   T-584 is `DONE`, and the version tag / GitHub release is now a separate,
   unblocked release decision (`v0.4.0` remains reserved for the Evidence
   Studio roll-up). X-RG1 O1/O2/O4/O6–O11, H-RG4 O1/O2, H-RG5 O1–O4, and
   R-RG1 O1–O4/O6–O7 remain optional hardening. Reviews archived at
   `docs/review/done/2026-07-31_claude-code_R-RG1_integrated-release-acceptance-review.md`
   and
   `docs/review/done/2026-08-03_claude-code_R-RG1_integrated-release-acceptance-narrow-re-review.md`.
2. **T-581 / H-RG4 closure record:**
   - **Fourth re-review verdict (`PASS`, 2026-07-30):** V1 resolved — the
     replacement artifact's 1,012 archived rows are all loopback with zero
     background hosts, stable identifiers, raw secrets, or local paths in the
     whole file; the harness filters published rows through
     `Test-FixtureArtifactRow`, derives
     `trafficScope`/`fixtureTrafficOnly`/row counters/`localPathsOmitted`
     from the actual output, adds fail-closed scope contradictions, and runs
     Edge/Electron isolated from background networking; the Go acceptance
     fixture asserts these mechanisms remain connected; the checksum
     `2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e`
     matches the sidecar and every record. V2 resolved — paired plans
     synchronized in both languages. V3 resolved — the R6 measurement deferral
     to R-RG1 is recorded with rationale. The session counters reconcile the
     archived/omitted split (1,023 = 1,012 + 11), the scenario matrix is
     complete including the fixture-origin attributed pinning failure, and
     `go build`/`go vet`/full `go test` plus uncached capture/app suites pass
     natively on Windows.
   - **Codex backend — implemented:** R1 transaction-derived finalized
     provenance/weakest fidelity; R3/R4 attributed
     `proxy_not_captured`/`proxy_passthrough` terminal failures; R5 persisted
     redaction summary; R6 connection-scoped resolver caching; R7 PID and start
     time confirmation; R8 versioned Wails renderer contract and bound fixture;
     R10 paired plan update; R12 persisted aborted rows with final stats and
     analysis parity.
   - **Third review backend follow-up — implemented:** Codex terminalized aborted-row
     fidelity (S2), crash-safe incremental redaction/stats checkpoints (S3),
     artifact privacy and harness-contract hardening (S5/S6), explicit QUIC
     invisibility evidence and locale-independent re-entry checks (S7/S8), and
     removal of the dead generic analyzer entry point (S9).
   - **Third review UI follow-up — implemented:** the finalized capture-fidelity
     card selects its explanatory sentence from the reported provenance, so a
     live proxy session is described as an ArchScope capture with
     proxy-measured timings and the imported-evidence sentence is reachable only
     from a `foreign_tool` observation point (S1); its fidelity, capture mode,
     observation point, and detail storage resolve through closed EN/KO label
     maps with the raw engine token on hover, and the
     `capture_mode_counts`/`fidelity_counts`/`coverage_counts` distributions
     render beneath the aggregate so a `mixed` / `unsupported` session shows how
     its transactions actually divide (S4); and a `redaction.known=false`
     recovery state renders as unrecorded with caution styling instead of "no
     sensitive fields matched" (S3 renderer half). The per-transaction detail
     panel resolves its fidelity token through the same map. State regressions
     assert live-versus-HAR hint selection, every closed token set, distribution
     ordering and unknown-bucket merging, both-locale coverage of every new key,
     the absence of any unconditional HAR hint, and the unknown/known/legacy
     redaction states.
   - **Initial Windows integration — superseded:** the schema-v4 harness ran
     from a `t581e2e` build and wrote its first artifact at
     `docs/review/evidence/2026-07-29_t581_windows-live-capture-schema-v4.json`
     but V1 rejected its privacy declaration. The repository copy now contains
     only the 1,010 loopback rows, records 57 omitted non-fixture rows, uses
     SHA-256
     `0b2bfc99d8e54d15ec98278d500c7691f7fed79c149f33f2320343a906d44975`,
     and intentionally reported that fixture-origin pinning evidence had to be
     regenerated. It was a safe non-passing artifact, not R2 closure.
   - **Corrected Windows replacement — completed:** the fresh `t581e2e` run
     archived 1,012 loopback-only rows from 1,023 source rows, omitted 11
     background rows, proved one confirmed-attribution fixture pinning failure,
     H2-only passthrough, QUIC invisibility, 1,000/1,000 long-session requests,
     500 restored rows, recovery readback, redaction, and local-path omission.
     Its contradiction set is empty and SHA-256 is
     `2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e`.
   - **Claude UI handoff — implemented:** the panel reads
     `GetLiveCaptureContract` at startup and derives its row cap, event-skip
     resync, page-re-entry restore, and finalized handoff from it, rejecting an
     unknown schema to the built-in defaults and disclosing that mismatch (R8);
     `liveFidelityTone` puts `isDecodedLiveFidelity` on the product path so a
     non-decoded grade carries caution emphasis instead of reading like ordinary
     captured traffic (R9); and transaction, session, CA, and process-attribution
     states render through closed EN/KO label maps (R11). No engine source is
     changed by Claude.
   - **Validation — passed:** full Go test/vet/build, capture race suite,
     Windows `t581e2e` app and engine cross-build, Windows capture vet and
     procmap test compilation, generated Wails bindings, frontend state tests,
     frontend production build, and the actual Windows PowerShell harness.
     The third re-review re-ran the full suite (including the app package,
     capture race suite, Windows cross-vet/build, frontend state tests, and
     production build) and confirmed every claim except the first artifact's
     privacy declaration (V1). The 2026-07-30 fourth re-review verified the
     corrected artifact against the V1 scope and returned the group `PASS`;
     T-581 is `DONE`.
3. **T-583 / X-RG1 closure record:** the final B3 narrow re-review returned
   `PASS` on 2026-07-31, so T-583 is `DONE` and T-584 / R-RG1 is ready. Do
   not create a tag or release before the remaining R-RG1 gate passes.
4. Review the long-term external APM import roadmap before assigning any new
   external-connector implementation range. Local file imports already cover
   OTLP, Zipkin, Elastic APM, Jaeger, and SkyWalking-style trace evidence;
   direct SaaS and product-specific connectors remain deferred until token
   storage, redaction, compliance, connector testability, and source-evidence
   contracts are clear.
5. Keep security/compliance evidence as a roadmap candidate, not an active
   TO-DO range, until it is selected against the external APM roadmap and the
   next release objective.
6. Continue pairing English/Korean docs whenever importer, analyzer, UI, or
   report-pack surfaces change.
7. Reserve `v0.4.0` for a broader Evidence Studio roll-up after the remaining
   roadmap candidates are prioritized and the chosen source families stabilize
   as one coherent workflow.
8. Keep release verification healthy before each release cut by repeating
   Windows GUI smoke, macOS signing/notarization validation, frontend bundle
   budget checks, and representative real-export fixture updates.

## Browser Profile / HTTP Capture Review Groups

The authoritative paired plan is
`docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` and
`docs/ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`.

| Order | Group | Owners | Status | Gate |
|---:|---|---|---|---|
| 1 | C-RG1 Chrome/V8 release implementation acceptance | Codex engine, Claude UI fixes | DONE — verdict `PASS`, 2026-07-21 | Closed |
| 2 | H-RG1 offline HAR analysis completion | Codex engine, Claude UI | DONE — integrated `PASS`, 2026-07-21; engine and H-SEC1 re-reviews `PASS`, UI R1/R2 dispositioned under the bounded Phase 1 contract | Closed |
| 3 | H-RG2 Windows coverage proof | Codex | DONE — H-COV1 `PASS`, 2026-07-27 | Closed |
| 4 | H-RG3 live-capture engine foundation | Codex | DONE — H-SEC2 `PASS`, 2026-07-28 | Closed |
| 5 | H-RG4 live UI and Windows E2E | Claude UI, Codex integration | DONE — group `PASS`, 2026-07-30 (narrow fourth re-review verified V1–V3 resolved) | Closed |
| 6 | H-RG5 HTTP session Diff | Codex engine, Claude UI | DONE — group `PASS`, 2026-07-30 | Closed |
| 7 | X-RG1 HTTP/profile/server-evidence correlation | Codex engine, Claude UI | DONE — narrow B3 re-review `PASS`, 2026-07-31; B1/B2/B3 closed | Closed |
| 8 | R-RG1 integrated release acceptance | Codex + Claude + reviewer | DONE — original PASS 2026-08-03; post-T-587/T-588 re-run PASS 2026-08-11 | Closed |

Group verdicts are PASS, CONDITIONAL, or FAIL. CONDITIONAL does not unblock the
next group. Individual reviews are limited to H-SEC1 (HAR security), H-COV1
(coverage evidence), H-SEC2 (CA/TLS/privilege), and optional C-SEM1 (`ph:"X"`
Long Task semantics).

## Review Intake Decisions

| Review | Decision | Follow-up |
|---|---|---|
| 2026-11-09 GC / flamegraph bug revalidation and requested implementation | Five confirmed bugs implemented by Codex under the user's explicit end-to-end assignment. Local regression/source review passed; independent reviewer, actual desktop E2E and fresh Windows evidence were not claimed. Optional downsampling and unexecuted lifecycle concerns remain separate. | T-589 through T-593 are `DONE`; original reproduction and current validation remain traceable in this file. |
| 2026-05-16 mid-term roadmap review P0-1 through P0-6 | Accepted | T-452 through T-457 |
| P1-7 UI-heavy derived analysis and P1-8 AnalysisResult drift | Accepted as phased remediation, not a single rewrite | T-458, T-459 |
| P1-9 exporter recomputes analysis | Accepted | T-460 |
| P1-10 service-edge identity matching | Accepted | T-461 |
| P1-11 AI provenance depth | Accepted | T-462 |
| P1-12 AI/deterministic visual separation after Evidence Board capture | Accepted | T-463 |
| P2 determinism, timestamp, runtime-stack, testing, shared utility, Mermaid, narrative sorting, and dedupe issues | Accepted and grouped by risk/dependency | T-464 through T-467 |
| Full immediate migration of all Wails state projections to Go analyzers | Deferred as too broad for one patch; start with Service Flow and migrate the remaining projections after the contract pattern is proven | T-458, T-459 |
| Korean PII allow-list coverage | Accepted as a review point inside the privacy hardening task; exact rule set needs implementation-time examples | T-454 |
| 2026-07-18 Chrome DevTools CPU profile design review P0-1 through P0-4 | Accepted as implementation gates; G1/G2 closed on 2026-07-20 with Chrome trace chosen as the first input and a microsecond `int64` measurement contract | T-558 through T-561 |
| Chrome/V8 classification, parser validation and frame identity, large-file/trace streaming findings | Accepted as dependent design hardening | T-562 through T-564 |
| Chrome profile end-to-end fixtures and English/Korean documentation parity | Accepted after the P0 contracts stabilize | T-565 |
| 2026-07-19 system HTTP capture design review P0-1 through P0-5 | Accepted; design note revised on 2026-07-19 to withdraw the per-process `bytesSystem` coverage premise, the raw-wire header guarantee, the single timing model, the size-threshold `AnalysisResult` contract, and automatic TLS passthrough | T-566 through T-571 |
| Proxy build-versus-integrate decision, capture lifecycle/recovery, HAR dialect identity and fixture corpus, capability tiers | Accepted as design completeness work reflected in the revised note | T-572 through T-575 |
| Repository-relative paths, cross-reference numbering, and scoped competitive claims | Accepted and applied directly to `docs/ko/SYSTEM_HTTP_CAPTURE.md`; the source ledger was filled on 2026-07-20 and the gate now reads "no design decision rests on an unverified row" | T-566 |
| 2026-07-20 Phase 0 design-gate authoring | Gates 1 through 5 written into `docs/ko/SYSTEM_HTTP_CAPTURE.md` and renumbered 1-11 with design gates separated from measurement gates; Phase 1 unblocked | T-566 through T-570 |
| English documentation parity for the HTTP capture note | Deferred until the Phase 1 contracts are approved, to avoid translating a design that is still moving | T-567 through T-571 |
| 2026-07-21 C-RG1 Chrome/V8 release implementation review | Accepted `CONDITIONAL`; Codex A-E and Claude U1-U5 remediation completed and were submitted for independent re-review | T-578 |
| 2026-07-21 C-RG1 Chrome/V8 remediation re-review | Accepted `PASS`; all ten A-E/U1-U5 findings independently verified resolved with 16-fixture goldens, engine/frontend regressions, vet, and build evidence | T-578 completed; H-RG1/T-579 unblocked |
| 2026-07-21 H-SEC1 HTTP-capture security independent review | Processed `CONDITIONAL`; no P0. Remediation completed: P2-5 free-text `token/session/code/authToken/*token` coverage aligned with structured fields plus Luhn and WS/SSE regressions; P1-1 closed by prohibiting CLI/headless live-capture start and adding SEC-16; P2-1/2/3/4/6 resolved in paired contracts through explicit crash-dump fail-closed mechanisms and honest Go zeroization limits, full-delete JWT/cookie defaults, platform owner-only CA ACLs, all discovered OS/NSS store enumeration, and pre-storage host/process scope filtering (SEC-17). P3 leaf-key memory-only, WS/SSE non-disclosure invariant, and logical-delete wording also reflected. Original review archived at `docs/review/done/2026-07-21_claude-code_H-SEC1_http-capture-security-review.md`. | Independent re-review `PASS`; H-SEC1 closed |
| 2026-07-21 H-RG1 HTTP-capture UI independent review | Accepted `CONDITIONAL`; no P0. P1: selected window/filter changes only bounded list/tree while summary cards retain the full-result denominator; MIME/duration/fidelity filters are missing; file changes and failed analyses can leave the previous result visible under the new source/error. P2: implement the contracted request/response/timing/process tabs plus cookies, keyboard-accessible timeline/rows and correct dialog focus, and fixture-driven populated-state/Workspace regressions. Engine implementation is explicitly outside this review and will be reviewed separately by Claude. | U1-U6 remediation implemented at `4c814bb`; original review archived at `docs/review/done/2026-07-21_codex_H-RG1_http-capture-ui-review.md` |
| 2026-07-21 H-RG1 HTTP-capture UI remediation re-review | Initial re-review was `CONDITIONAL`; U2-U5 closed. Integrated H-RG1 disposition accepted R1 because Phase 1 deliberately exposes a bounded `AnalysisResult`, all filtered cards/list/tree share that disclosed inline-row denominator, and the UI makes no full-session filtered-total claim. R2 was accepted for Phase 1 from populated state derivations, provenance/Workspace regressions, typed component wiring, and a green production build; deeper Wails component fixtures remain non-blocking hardening. | T-579/H-RG1 integrated `PASS`; archived at `docs/review/done/2026-07-21_codex_H-RG1_http-capture-ui-remediation-rereview.md` |
| 2026-07-21 H-RG1 HTTP-capture engine implementation review | Processed `CONDITIONAL`; no P0. Remediation completed: malformed URL parse failures no longer panic and emit `HAR_URL_UNPARSABLE`; the parser-level recovery boundary converts unexpected panics to non-leaking structural errors; oversized JSON bodies are fail-closed with `HAR_REDACTION_DEGRADED`; structured numeric/boolean `code`, `auth`, and `session` values remain typed; plain-byte/field/timing guards, request decoded-size semantics, dead body-storage branching, and UTF-8 preview truncation are covered. Targeted and full Go tests plus vet pass. Original review archived at `docs/review/done/2026-07-21_claude-code_H-RG1_http-capture-engine-implementation-review.md`. | T-579 engine remediation complete; independent re-review `PASS` |
| 2026-07-21 H-RG1 HTTP-capture engine remediation re-review | Accepted `PASS`; all original P1×2, P2×3, and P3×3 findings were independently verified resolved with targeted/full Go tests, vet, and build evidence. The sole non-blocking P3 recommendation was also closed during intake: numeric/boolean values under hard-secret JSON keys are now redacted, while typed `code`/`auth`/`session` analysis values remain preserved. | Engine gate closed; archived at `docs/review/done/2026-07-21_claude-code_H-RG1_http-capture-engine-re-review.md` |
| 2026-07-21 H-SEC1 HTTP-capture security re-review | Accepted `PASS`; all eight original findings (P1×1, P2×6, P3×3) independently re-verified closed at `1a1b409`. The two code defects (P2-5 free-text denylist alignment, P3-2 WS/SSE non-disclosure invariant) are fixed and locked with regression tests that pass on independent execution; the six design-contract findings (P1-1 no CLI/headless capture start + SEC-16, P2-1 crash-dump fail-closed with honest Go zeroization limits, P2-3 platform owner-only CA ACLs, P2-4 full discovered OS/NSS store enumeration, P2-6 pre-storage scope filter + SEC-17, P3-1 memory-only leaf keys, P3-3 logical-delete honesty) are symmetrically reflected in `docs/ko`/`docs/en`. `go test`/`go vet` pass for the redact and httpcapture packages. Scope note: SEC-8/10/16/17 contracts for the unimplemented capture/CA/helper surfaces are re-measured at implementation time under H-SEC2. Archived at `docs/review/done/2026-07-21_claude-code_H-SEC1_http-capture-security-re-review.md` | H-SEC1 gate closed; included in H-RG1 integrated `PASS` |

| 2026-07-27 H-COV1 T-571 Windows coverage review | Accepted `CONDITIONAL`; COV-1~COV-6 required CAP-3 N/A ratio suppression, evidence-tier honesty, CAP-6 deferral, ETW header-PID wording, WFP N/A/removal parity, and judge safety regressions. | Remediation completed; original review archived at `docs/review/done/2026-07-27_codex_H-COV1_t571_windows-coverage-review.md` |
| 2026-07-27 H-COV1 T-571 remediation re-review | Accepted `PASS`; all COV-1~COV-6 independently verified closed with no residual findings. The approved exposure is five self-observed counters plus successful individual persistent-endpoint attribution; absolute coverage ratios remain forbidden. | T-571/H-RG2 closed; T-580/H-RG3 unblocked. Re-review archived at `docs/review/done/2026-07-27_codex_H-COV1_t571_windows-coverage-rereview.md` |
| 2026-07-28 H-SEC2 T-580 live-capture engine security review | Accepted `PASS`; SEC-1/2/3/6/7 redaction runs before every persistence and the MITM proxy stores no plaintext bodies; SEC-8/9 the CA private key is memory-only, non-exportable, per-process and zeroized on close; SEC-11 session dirs `0700`/files `0600` with path-traversal-validated IDs; SEC-12 transactional trust-store rollback/removal; SEC-13/14 no auto-bypass and per-request upstream-failure handling; SEC-16 no CLI/headless capture-start; upstream TLS verify-always against the host trust store with loopback-only bind. SEC-10 (dump-exclusion preflight before body capture) and SEC-17 (explicit unknown-attribution opt-in before the live UI) are recorded as binding conditions on the next tier because this slice never stores plaintext bodies and holds no long-lived secret on disk. `go test ./internal/capture/...`, `go test ./cmd/archscope-app/...`, `go vet`, and `go build ./...` pass. | T-580/H-RG3 closed; T-581/H-RG4 unblocked. Review archived at `docs/review/done/2026-07-28_claude-code_H-SEC2_http-capture-live-engine-security-review.md` |
| 2026-07-28 H-RG4 T-581 Windows live-capture UI/E2E review | Accepted `CONDITIONAL`. Blocking: L1 passthrough progress rows claim semantic fidelity, L2 `redact.Policy` has a confirmed concurrent-map race, L3 the E2E/harness/fixture package does not read back or prove real ArchScope captures. High: L4 unbatched progress IPC, L5 unresolved in-flight rows, L6 SEC-17 opt-in disclosure is lost on re-entry, L7 deliberate unattributed drops lack an honest warning/denominator. Medium/low L8–L14 cover row-position churn, renderer-only platform gating, JVM/NSS trust disclosure, fabricated CONNECT paths, duplicate started reset, attribution-boundary wording, and drop-label ambiguity. Sound behavior retained: SEC-17 enforcement below UI, unconditional body omission, same-ID replacement, store-root protection, CA cleanup, and event-skip resync. | T-581 stays `REVIEW`; remediate L1–L3, resolve or accept L4–L7, disposition L8–L14, produce product-readback Windows evidence, then request independent re-review. T-582 remains blocked. Review archived at `docs/review/done/2026-07-28_claude-code_H-RG4_windows-live-capture-ui-e2e-review.md` |
| 2026-07-28 H-RG4 T-581 Claude UI/state/i18n disposition | L1 fixed in the renderer: `resolveLiveFidelity` maps a closed token set and sends every unrecognized value to `unknown`, which renders as "not yet determined"; the raw grade survives only as a tooltip and `isDecodedLiveFidelity` gates any positive claim. L4 fixed: the reducer consumes the engine's batched `items` in one dispatch. L5 fixed: a state column, in-flight badge and count, and `—` placeholders keep unresolved rows from printing `0`/`0 ms` as measured values. L6 fixed: `activeUnattributedPolicy` shows the running session's `retainUnattributedMetadata`, so page re-entry states the enforced policy rather than the unchecked default. L7 fixed: an `observed` denominator tile, a drop-share line, and localized drop/unattributed/kernel-loss warnings. L8 fixed: `boundedDistinct` replaces a row at its first-seen position, so finalization no longer moves rows to the tail. L12 fixed: a duplicate `started` dispatch for the same session is idempotent and the follow preference survives every start. L14 fixed: "Discarded by policy" and "Lost before capture" are separate labels with separate warnings. State regressions cover each finding and assert en/ko key parity. | T-581 stays `REVIEW` pending real Windows evidence and independent re-review. One backend item returned to Codex under the ownership boundary: `Pipeline.abortInflight` marks only `TxRequestSent` rows, leaving `receiving` rows non-terminal at stop. |
| 2026-07-29 H-RG4 T-581 Windows live-capture UI/E2E re-review | Accepted a second `CONDITIONAL`. Original L2 and the live-table L1/L4–L8/L12/L14 behavior are independently verified closed. Blocking R1: finalized live sessions are still mislabeled `semantic` / `har_import` / `foreign_tool`. Blocking R2: the asserted Windows runs have no inspectable archived artifact and the harness omits unsupported, page re-entry, long-session, and recovery coverage. High R3–R5: pinning failures lose attribution and are dropped, failed passthrough tunnels finalize as MITM/pending with inconsistent coverage, and finalized redaction disclosure falsely reports no applied redaction. R6–R8 cover resolver cost, start-time confirmation honesty, and unbound renderer fixture contracts; R9–R12 cover a dead claimed gate, stale paired plan, raw state localization, and non-persisted aborted rows. | T-581 remains `REVIEW`; close R1–R5, resolve or accept R6–R8, record R9–R12 disposition, rerun validation, and request another independent re-review. T-582 remains blocked. Review archived at `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-re-review.md` |
| 2026-07-29 H-RG4 T-581 Codex backend disposition after second re-review | R1 fixed with live-only transaction-derived mode, proxy provenance, weakest fidelity and per-token counts; R3/R4 fixed with attributed `proxy_not_captured` pinning failures and truthful passthrough terminal failures; R5 fixed by persisting the redaction summary into the manifest and finalized analysis/evidence; R6/R7 fixed with connection-scoped resolution and second-read PID/start-time equality; R8 backend fixed with a versioned Wails contract and fixture binding; R10 paired plan updated; R12 aborted rows persisted and counted. The schema-v3 R2 harness now runs supported, h2-only, pinning, long-session, WebView re-entry, and recovery readbacks. Full Go/race/vet/build, Windows app/engine cross-build and capture vet, procmap Windows test compilation, bindings, and frontend test/build validation pass. | Codex backend handoff ready. Still required: execute the PowerShell harness and archive a fresh Windows artifact (R2), Claude production R8 contract consumption plus R9/R11, and another independent re-review. T-581 remains `REVIEW`; T-582 remains blocked. |
| 2026-07-29 H-RG4 T-581 Claude UI/state/i18n disposition after second re-review | R8 fixed on the renderer side: the panel reads `GetLiveCaptureContract` at startup, the reducer carries the contract, and the row cap, event-skip resync, page-re-entry restore, and finalized-analysis handoff are all derived from it, so the acceptance fixture's `renderer` block now describes real behavior instead of a literal that can drift from `LIVE_TRANSACTION_ROW_CAP`. A contract whose `schemaVersion` this build does not implement, or whose fields are malformed, falls back to the built-in defaults and raises a visible mismatch notice rather than half-honoring an unknown contract. R9 fixed by putting `isDecodedLiveFidelity` on the product path through `liveFidelityTone`: the live table's fidelity cell now takes a caution emphasis for `unsupported`/`passthrough`, a muted one for undetermined grades, and the plain style only for genuinely decoded ones, so the claimed positive-capture gate exists rather than being asserted. R11 fixed by resolving transaction state, session state, CA state, and process attribution through closed EN/KO label maps; unrecognized tokens say "unrecognized state" instead of leaking the wire value. The paired implementation plans record the completed handoff. State regressions cover contract adoption, rejection, contract-driven row cap, disabled resync, tone separation, and every new token map, plus en/ko key parity. `npm run test:state`, `npm run build` (tsc + vite), `go build ./...`, `go vet ./cmd/archscope-app/...`, and `go test ./cmd/archscope-app/...` pass. | R8/R9/R11 closed. T-581 remains `REVIEW`: the only outstanding H-RG4 condition is the R2 archived Windows schema-v3 artifact, after which an independent re-review is required. T-582 remains blocked. |
| 2026-07-29 H-RG4 T-581 Windows live-capture UI/E2E second re-review | Accepted a third `CONDITIONAL`. Ten R-findings are resolved and R2 is resolved only in mechanism. Blocking S1: the finalized live card still renders a fixed HAR/import hint. Blocking R2: no inspectable Windows schema-v3 artifact/checksum exists. High S3: crash recovery can read persisted rows beside absent redaction and zero stats because checkpoints are written only during graceful stop. Medium S2/S4/S5: aborted rows retain `pending`, finalized tokens/distributions are not localized, and the required public artifact lacks a loopback-only privacy boundary. Low S6-S9: the harness contract can drift, QUIC is declared but unprobed, re-entry is locale/DOM-fragile, and generic `httpcapture.Build` is dead. | Codex: S2/S3/S5-S9. Claude: S1/S4. R2 requires a fresh privacy-reviewed Windows artifact after both handoffs. T-581 remains `REVIEW`; T-582 remains blocked. Review archived at `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-second-re-review.md`. |
| 2026-07-29 H-RG4 T-581 Codex disposition after third `CONDITIONAL` | S2 fixed: every stopped in-flight row persists as `aborted` / `unsupported`, so `pending` cannot grade a finalized session. S3 backend fixed: row flushes carry capture/redaction checkpoints, recovery reconciles counters with stored rows, and missing/partial legacy redaction is explicitly `known=false`. S5/S6 fixed with a shared schema-v4 harness contract, loopback-only fixtures, omitted local paths, 2,000-row bound, privacy metadata, and owner-only artifact ACL. S7 adds a fail-closed UDP/QUIC invisibility probe. S8 removes locale text lookup and reconciles restored rows with product readback. S9 removes generic `httpcapture.Build`. | Codex handoff complete after validation. Remaining: Claude S1/S4 and the S3 unknown-state label, execute and archive the privacy-reviewed Windows schema-v4 artifact for R2, then independent re-review. T-581 remains `REVIEW`; T-582 remains blocked. |
| 2026-07-29 H-RG4 T-581 Claude UI/state/i18n disposition after third `CONDITIONAL` | S1 fixed: the finalized card's explanatory sentence is selected from the result's own provenance through `CAPTURE_PROVENANCE_HINT_KEYS`, so a stopped live session is described as an ArchScope live proxy capture with proxy-measured timings and omitted bodies; the imported-foreign-tool sentence is now reachable only from a `foreign_tool` observation point, an unrecognized point claims neither origin, and the previously unconditional `httpCaptureFidelityHint` key is removed so it cannot be printed again. S4 fixed: fidelity, capture mode, observation point, and detail storage resolve through closed EN/KO label maps with the raw engine token preserved on hover, unrecognized tokens say so instead of leaking the wire value, and the `capture_mode_counts`/`fidelity_counts`/`coverage_counts` distributions render beneath the aggregate so a `mixed` / `unsupported` session shows that (for example) 200 of 201 rows decoded; the per-transaction detail panel uses the same fidelity map. S3 renderer half fixed: `redaction.known=false` renders as "not recorded" with caution styling and states that redaction still ran before every write, so an absent recovery summary is never presented as "no sensitive fields matched"; the rule chips are suppressed for that state and payloads predating the flag keep their determined summary. The token→message-key maps live in `state/httpCapture.ts` so the Node state harness asserts hint selection directly, and the records are exhaustive over their token unions so a new engine enum fails type-check rather than reaching the screen untranslated. State regressions cover live-versus-HAR-versus-unknown hint selection, each closed token set, distribution ordering and unknown-bucket merging, both-locale coverage of every new key, and the unknown/known/legacy redaction states. `npm run test:state`, `npm run build` (tsc + vite), `go build ./...`, `go vet ./cmd/archscope-app/...`, and `go test ./cmd/archscope-app/... ./internal/analyzers/httpcapture/...` pass. Paired EN/KO user guides and implementation plans record the closed handoff. | S1/S4 and the S3 renderer half closed; no engine source changed by Claude. T-581 remains `REVIEW`: the only outstanding H-RG4 condition is the R2 privacy-reviewed Windows schema-v4 artifact, after which an independent re-review is required. T-582 remains blocked. |
| 2026-07-29 H-RG4 T-581 Windows schema-v4 acceptance execution | The real Windows `t581e2e` app and engine completed the loopback-only schema-v4 harness with an empty contradictions array. Product readback covers curl/browser/JVM/Electron over HTTP and HTTPS, explicit h2-only passthrough, certificate-pinning failure, QUIC invisibility, 1,000/1,000 long-session requests, 500 restored rows after WebView re-entry, and a separately recovered crash store. The final session contains 1,067 rows and its redaction counters total 4,113. The archived JSON contains neither the raw test secret nor a local Windows user path and is checksum-pinned at `e1bdfafe8e610507f42a04f65f5d2eb04b94dfdb29a5a3163778d1fb676d4f8a`. The run exposed and closed a CONNECT shutdown hang by closing hijacked tunnels before graceful server shutdown, plus harness defects in loopback bypass, JVM H2 socket typing, CDP result handling, re-entry polling, artifact ACL idempotence, and local-path omission. All temporary fixture and ArchScope capture CAs were removed from `CurrentUser\Root` after the run. | R2 execution and archival closed at `docs/review/evidence/2026-07-29_t581_windows-live-capture-schema-v4.json`. T-581 remains `REVIEW` solely for independent H-RG4 re-review; T-582 remains blocked. |
| 2026-07-29 H-RG4 T-581 Windows live-capture UI/E2E third re-review | Accepted a fourth `CONDITIONAL`. Independently verified closed: S1 (provenance-selected hint, unconditional key deleted, live/HAR/unknown regressions), S3 backend and renderer (per-flush capture/redaction checkpoints, recovery counter reconciliation, `known=false` renders as "not recorded"; the artifact's real crash-recovery block reads back `1/1/1` stats beside a checkpointed summary), S2 (aborted rows persist `unsupported`; a 200-decoded+1-aborted probe grades `unsupported` with interpretable distributions), S4 (closed EN/KO maps, hover raw tokens, rendered distributions), S6 (shared harness-contract JSON DeepEqual-bound in Go and referenced by the script), S7 (fail-closed QUIC invisibility probe plus product-readback contradiction), S8 (aria-current re-entry lookup with product reconciliation), S9 (generic `Build` removed). The artifact checksum matches, its contradictions array is empty, all scenarios have product readback, the raw long-session secret is absent, and redaction counts exceed the long session. Full Go suite (incl. app package), capture race suite ×2, Windows cross-vet/engine build, frontend state tests, and production build re-run and pass. Blocking V1: the artifact violates its own hardcoded `fixtureTrafficOnly: true` declaration — 57 operator Edge/Electron background rows are archived, 26 decoded with full query strings including a stable sync `client_id` and MSA identity endpoints, and no contradiction check examines archived-row hosts, so the privacy scope is asserted rather than enforced. Low V2: paired EN/KO plans still record the artifact as nonexistent. Low V3: R6's allowed measurement deferral has no recorded decision. | T-581 remains `REVIEW`; enforce the artifact privacy scope in the harness (derive `fixtureTrafficOnly` or isolate client background traffic), regenerate and re-checksum the Windows artifact, record V2/V3, then request a narrow independent re-review limited to V1–V3. T-582 remains blocked. Review archived at `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-third-re-review.md`. |
| 2026-07-30 H-RG4 T-581 V1–V3 disposition | V1 harness remediation filters published evidence rows to loopback fixture hosts, derives `fixtureTrafficOnly` plus source/archive/omitted row counts from the output, derives local-path absence, and launches Edge/Electron with ephemeral profiles and background-networking suppression. The Go acceptance fixture now asserts these mechanisms remain connected. The unsafe artifact copy was immediately filtered to 1,010 loopback rows; 57 non-fixture rows were omitted, the quoted stable ID was redacted from the review, and checksum `0b2bfc99d8e54d15ec98278d500c7691f7fed79c149f33f2320343a906d44975` pins the safe copy. It intentionally contains a contradiction requiring a fresh fixture-origin pinning proof, so it cannot be mistaken for PASS evidence. V2 is closed by synchronizing the paired EN/KO implementation plans with the existing-but-rejected artifact and current remediation state. V3 explicitly defers isolated resolver-cost measurement to the R-RG1 Windows integrated performance check: resolution is already bounded to once per accepted connection, the long-session acceptance passed, and meaningful profiling must separate resolver cost from UI/client load. | Review archived. T-581 remains `REVIEW`; rerun the corrected Windows harness, replace/checksum the artifact, then request a narrow V1–V3 re-review. T-582 remains blocked. |
| 2026-07-30 H-RG4 T-581 corrected Windows replacement execution | The privacy-hardened schema-v4 harness completed from a fresh `t581e2e` session with an empty contradiction set. It archived 1,012 loopback fixture rows from 1,023 product rows, recorded 11 omitted background rows, and derived `fixtureTrafficOnly: true` plus local-path absence from the published output. A deterministic Java explicit-CONNECT pinning probe replaced the loopback-bypassing/non-terminal client probes and produced one `proxy_not_captured` / `unsupported` fixture row with confirmed process attribution. H2-only passthrough, QUIC invisibility, curl/browser/JVM/Electron HTTP+HTTPS, 1,000/1,000 long-session requests, 500-row page re-entry, crash recovery, redaction, package metadata, raw-secret absence, and checksum parity passed. The LF-normalized repository artifact SHA-256 is `2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e`; the original Windows CRLF artifact SHA-256 was `69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111`. | V1 replacement execution is complete and V2/V3 remain recorded. T-581 stays `REVIEW` only for the narrow independent V1–V3 verdict; T-582 remains blocked. |
| 2026-07-30 H-RG4 T-581 Windows live-capture UI/E2E fourth re-review (narrow V1–V3) | Accepted `PASS`. V1 independently verified resolved: the replacement artifact's Windows CRLF checksum `69565684…30111` matched the then-recorded sidecar and every record; the 2026-08-03 R-RG1 correction pins the repository's LF-normalized bytes as `27371338…7218c7e` and retains the CRLF digest as provenance. All 1,012 archived rows plus the recovery row are loopback; whole-file scans find zero non-loopback URLs, zero background hosts, zero stable identifiers, no raw `t581-secret`, and zero local Windows paths; the privacy block is derived from the published output (`1,023 = 1,012 + 11` reconciles across every counter family) and the fail-closed scope/local-path contradictions plus Edge/Electron background isolation are present in the harness and bound by the Go acceptance fixture; the scenario matrix is complete including the fixture-origin attributed `proxy_not_captured` pinning row, h2-only passthrough, QUIC absence, 1,000/1,000 long session, 500-row re-entry, and the checkpointed recovery block. V2 verified: paired EN/KO plans record the artifact lifecycle truthfully. V3 verified: the R6 measurement deferral to R-RG1 is recorded with rationale. `go build`/`go vet`/full `go test` plus uncached capture/app suites pass natively on Windows; no product source changed since the third re-review, so its race-suite pass stands. Non-blocking observations O1–O4 recorded (tautological `fixtureTrafficOnly` derivation, unfiltered recovery rows, literal `reviewBeforeArchive`, reused crash store). | H-RG4 closed with group `PASS`; T-581 `DONE`; T-582/H-RG5 unblocked. R-RG1 owes the deferred resolver-cost measurement. Review archived at `docs/review/done/2026-07-30_claude-code_H-RG4_windows-live-capture-ui-e2e-fourth-re-review.md`. |
| 2026-07-30 H-RG5 T-582 Codex backend handoff | Implemented versioned deterministic URL templating (`{id}`/`{uuid}`/`{hash}`/`{token}`/`{email}` plus sorted query keys only), bounded top-1,000 endpoint/host/process source projections with `{other}` disclosure, explicit numerator/denominator error/share/per-minute rates, independent p50/p95/p99 session projections, `aligned`/`duration_only`/`none` time grades, bounded `http_capture_diff` tables and five `HTTP_DIFF_*` finding families, real-process-only comparison with explicit HAR disablement, cross-dimension validation, store-free export metadata, and backend Workspace routing that leaves legacy Diff untouched. Wails methods are `AnalyzeHttpCaptureDiff`, `GetHttpCaptureDiffContract`, and `ResolveWorkspaceComparison`. Reordered equivalent sessions produce empty change tables and zero findings. Full `go test ./...`, `go vet ./...`, and `go build ./...` pass; targeted uncached analyzer/app tests pass. Windows `-race` was not runnable because the active Go environment has CGO disabled. | Codex backend complete. T-582 remains `IN_PROGRESS` for generated bindings, Claude comparison UI, and H-RG5 group review; no frontend source or generated binding was changed in this handoff. |
| 2026-07-30 H-RG5 T-582 Claude UI handoff | Regenerated the Wails bindings with the module-pinned CLI (alpha2.117, `-ts`), adding `AnalyzeHttpCaptureDiff`/`GetHttpCaptureDiffContract`/`ResolveWorkspaceComparison` and their request/contract models. Implemented the Claude-owned comparison UI: a shared `HttpCaptureComparisonPanel` mounts on HttpCapturePage (compare action seeded from the just-analyzed session) and on the Analysis Workspace (per-entry A/B slots), with the selection/run lifecycle in a pure reducer + module store (`state/httpCaptureDiff.ts`) so both surfaces share one A/B pair and a result renders only with the pair that produced it (race-safe). The renderer never routes comparisons itself — it follows the backend `ResolveWorkspaceComparison` verdict — and adopts `GetHttpCaptureDiffContract` at startup, disabling comparison with a disclosed mismatch for unimplemented schema versions. Overlays follow only `overlay_allowed`+grade with unrecognized grades failing closed; per-minute rates render only at `aligned`. Error rate and traffic share render numerator/denominator pairs, per-minute rates disclose their minute denominator, rate-unavailable codes resolve through closed EN/KO maps, added/removed unmatched-template sides render `—` not `0`, the disabled HAR pseudo-process dimension shows its reason, results without the diff source projection are blocked with a re-analyze notice, cursor drilldown opens full per-side metrics, and empty change tables render an explicit no-difference state. State regressions cover contract adoption/rejection, closed token sets, the fail-closed overlay gate, provenance racing, candidate filtering, the projection precondition, and both-locale coverage of every new key. `npm run test:state`, `npm run build`, `go build ./...`, `go vet ./cmd/archscope-app/...`, and `go test ./cmd/archscope-app/... ./internal/analyzers/httpcapture/...` pass. | Claude UI handoff complete; no engine source changed. T-582 remains `IN_PROGRESS` solely for the independent H-RG5 group review. |
| 2026-07-30 H-RG5 T-582 HTTP session Diff group review | Accepted `PASS`. Engine half verified independently: `TestAnalyzeDiffReorderedEquivalentSessionsCompareEqual` proves shuffled equivalent sessions yield empty change tables, zero findings, and a zero delta; URL template v1 rules and sorted-query-key folding are pinned; every rate carries an explicit numerator/denominator with `rate_unavailable_code` for untrusted timelines; HAR pseudo-process pairs disable the process dimension with a reason; projections are bounded (≤1,000 + `{other}`) with cross-dimension total validation; `store_rescanned: false` and Workspace routing leave legacy Diff untouched; `buildParsed` attaches the projection on both the HAR and live paths and `SetDiffCaptureSessionRef` binds the finalized snapshot. Renderer half verified by executed regressions: contract adoption/rejection, fail-closed grade-gated overlays, closed EN/KO token maps, race-safe result provenance, projection precondition blocking, unmatched-template `—` rendering, explicit no-difference state, and both-locale key coverage. Full Go suite (74 packages), uncached app/analyzer tests, `npm run test:state`, and the production build pass. Non-blocking O1–O4: duration_only per-minute suppression label precision, stale removed-entry selection, Workspace slot-button aria-labels, single Wails-layer diff test. | H-RG5 closed with group `PASS`; T-582 `DONE`; T-583 / X-RG1 unblocked. R-RG1 still owes the H-RG4 resolver-cost measurement. Review archived at `docs/review/done/2026-07-30_claude-code_H-RG5_http-session-diff-group-review.md`. |
| 2026-07-30 X-RG1 T-583 Codex backend handoff | Added the versioned `http_evidence_correlation` analyzer and Wails methods `AnalyzeHttpEvidenceCorrelation` / `GetHttpEvidenceCorrelationContract`. The analyzer consumes bounded `AnalysisResult` rows only (no source/store rescan), reuses HTTP Diff endpoint templating, and emits bounded HTTP/CPU overlap, Jennifer network-gap, access-log client/server, and alignment diagnostic tables with source provenance. V8 CPU timestamps require an explicit RFC3339 profile-start wall-clock anchor before `aligned` overlay output is allowed; missing anchors fail closed. Jennifer evidence remains `duration_only` because its edge timestamps have no date/offset. Access-log matches prefer request ID, then method/path-template/status/time. Confidence, mismatch findings, truncation disclosure, and `causal_claim_allowed: false` are explicit. Full `go test ./...`, `go vet ./...`, and `go build ./...` pass; targeted analyzer/httpcapture/app tests also pass uncached. | Codex backend complete. T-583 remains `IN_PROGRESS` for generated Wails bindings, Claude drilldown/overlay UI, frontend verification, and X-RG1 group review; no frontend source or generated binding changed in this handoff. |
| 2026-07-31 X-RG1 T-583 Claude UI handoff | Regenerated the Wails bindings with the module-pinned CLI (alpha2.117, `-ts`), adding `AnalyzeHttpEvidenceCorrelation` / `GetHttpEvidenceCorrelationContract` and their request/contract models. Implemented the Claude-owned drilldown/overlay UI: a shared `HttpCorrelationPanel` mounts on HttpCapturePage (with a "use in correlation" seed action on the analyzed session) and the Analysis Workspace; slot selection (HTTP required, profile/Jennifer/access-log optional with at least one), the RFC3339 profile wall-clock anchor with pre-run validation, and the time tolerance live in a pure reducer + module store (`state/httpCorrelation.ts`) whose provenance invariant drops the rendered result on any input change, including raced completions. The panel adopts the versioned contract at startup and rejects unimplemented schemas — or any contract declaring causal claims allowed — with a disclosed mismatch. Time overlays render only for a source whose `alignment_diagnostics` row certifies `overlay_allowed` at an `aligned` grade; unrecognized grades fail closed; Jennifer tables carry an explicit duration-only no-overlap note; a suppressed CPU overlay shows the engine's reason. Every result surface repeats the no-causality disclosure; grades, confidence, match bases, and sources resolve through closed EN/KO label maps with raw tokens on hover; per-source diagnostics disclose rows used/output rows/truncation; cursor drilldown shows full per-row evidence plus an aligned-only time-window overlay. State regressions pin contract adoption/rejection, the fail-closed overlay gate, anchor validation, slot candidate filtering, the secondary-source minimum, race-safe provenance, and both-locale coverage of every new key. `npm run test:state`, `npm run build`, `go build ./...`, `go vet ./cmd/archscope-app/...`, and uncached `go test ./cmd/archscope-app/... ./internal/analyzers/httpcorrelation/...` pass. | Claude UI handoff complete; no engine source changed. T-583 remains `IN_PROGRESS` solely for the independent X-RG1 group review. |
| 2026-07-31 X-RG1 T-583 HTTP evidence correlation group review | Accepted `CONDITIONAL`. Independently verified: bounded `AnalysisResult`-only input with no file/store rescan, secondary-source validation, missing-anchor V8 fail-closed behavior, Jennifer `duration_only`, renderer grade-gated overlays and causal-contract rejection, race-safe provenance, closed EN/KO token maps, no-causality disclosure, and bounded output/truncation diagnostics. Blocking B1: request-ID access-log pairings can emit a fabricated `timestamp_delta_ms: 0`, `aligned`, and overlay permission without comparing clocks, even when the HTTP timeline is unavailable or six hours apart. Blocking B2: missing/non-numeric `metadata.parser_metadata.v8_start_time_us` defaults to zero and can certify a false CPU overlap. O1–O5 record non-blocking Jennifer match bounds, access host omission, renderer contract checks, greedy pairing order, and Workspace selector/top-N observations. | T-583 remains `IN_PROGRESS`. Codex: B1/B2 plus engine regressions. Claude: unavailable timestamp-delta renderer state (`—`). Then rerun the narrow B1/B2 review, full Go tests, frontend state tests, and frontend build. Review archived at `docs/review/done/2026-07-31_claude-code_X-RG1_http-evidence-correlation-group-review.md`. |
| 2026-07-31 X-RG1 T-583 Codex B1/B2 remediation handoff | B1 now treats request ID only as a pairing basis. Each pair separately records `clock_compared`; `aligned` requires parseable absolute timestamps and a measured delta within tolerance. Missing clocks omit `timestamp_delta_ms`, provide `timestamp_delta_unavailable_reason`, grade `duration_only`, and suppress overlays; measured out-of-tolerance deltas retain their real value plus a mismatch reason. Source overlay permission fails closed unless every emitted row has compatible clock evidence, so a duration-only primary timeline cannot produce an aligned access source. B2 validates `metadata.parser_metadata.v8_start_time_us` as a finite integral numeric timestamp base; missing/non-numeric values emit zero overlap rows, grade `none`, suppress overlay, and raise `HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE` naming the field. Regressions cover six-hour request-ID skew, missing client time, missing/non-numeric V8 bases, and the valid request-ID-plus-clock path. Targeted uncached analyzer tests and full `go test ./...`, `go vet ./...`, and `go build ./...` pass (macOS linker-version warnings only). | Codex backend remediation complete with no frontend or generated-binding change. Claude must render omitted timestamp deltas as `—` with the unavailable reason; then run the narrow X-RG1 re-review and frontend verification. |
| 2026-07-31 X-RG1 T-583 Claude B1 renderer remediation handoff | B1 renderer half: `HttpCorrelationAccessRow` now carries `clock_compared` with an optional `timestamp_delta_ms` plus the engine's `timestamp_delta_unavailable_reason` / `timestamp_alignment_reason`, and the new pure selector `accessClockComparison` fails closed to "unavailable" whenever the flag, the delta, or its finiteness disagree — a delta without the flag, a flag without a delta, and a non-finite delta all render unmeasured. The access match table gained a Timestamp Δ column that prints `—` with the reason on hover for identity-only pairings, and the drilldown replaces the old unconditional `0.0ms` with the same `—` plus an explicit clock-comparison field. An access source the engine did not certify `aligned` now discloses identity-only pairing above its table. Because the summary's three alignment counters exclude the primary source, a new note names the primary HTTP timeline's own grade beside them (`correlationPrimaryAlignment`, failing closed to `unknown`). UI-scope observations closed in the same pass: contract adoption additionally rejects `store_or_file_rescan: true` and any `alignment_grades`/`confidence_grades` token the closed renderer sets cannot name (O3); the Workspace mount gates on `correlationCandidates(entries, "http")` instead of the Diff selector, and a contract-bounded top-N input (`correlationTopNState`, rejecting non-integers, zero, and values above `max_top_n`) joins the provenance input identity so changing it drops the rendered result (O5). New EN/KO keys carry both-locale coverage. Verified against the shipped engine with an out-of-repo probe: the six-hour request-ID skew emits a real 21,600,000 ms delta at `duration_only`, and an identity match with no parseable client timestamps omits the delta entirely with `clock_compared: false`. `npm run test:state`, `npm run build`, `go build ./...`, and `go test ./...` pass. | Claude renderer remediation complete with no engine source change. T-583 remains `IN_PROGRESS` solely for the narrow X-RG1 re-review; engine-scope O1/O2/O4 stay non-blocking. |
| 2026-07-31 X-RG1 T-583 HTTP evidence correlation re-review | Accepted a second `CONDITIONAL`. B2 is independently verified fixed; B1's six-hour skew, missing-time, source-level fail-closed, summary disclosure, and Claude table/drilldown handling all hold; O3 contract rescan/vocabulary checks and O5 correlation-owned Workspace selector/top-N provenance are closed. Blocking B3: when an earlier shape candidate sets `bestDelta`/`bestClockCompared` and a later request-ID record wins without a usable timestamp, the emitted request-ID row can inherit the earlier record's delta, grade `aligned`, and enable overlay. This mixes evidence from two access records. New non-blocking O6–O10 cover match-vs-clock confidence wording, whole-source overlay utility, basis-specific reason wording, skipped-row disclosure, and saturated extreme deltas. O1/O2/O4 remain open; O2 should be prioritized on the next `sameRequestShape` change. | T-583 remains `IN_PROGRESS`. Codex must recompute/reset clock evidence from the selected access record and add the adversarial B3 regression; no renderer change is required. Then rerun `go test ./...`, frontend state/build checks, and one more narrow re-review. Review archived at `docs/review/done/2026-07-31_claude-code_X-RG1_http-evidence-correlation-re-review.md`. |
| 2026-07-31 X-RG1 T-583 Codex B3 remediation handoff | Candidate selection and emitted clock evidence are now structurally separated. Shape candidates use `bestShapeDelta` only to choose the closest record; after the winner is fixed, `timestamp_delta_ms` and `clock_compared` are recomputed exclusively from `used[bestIndex]`. The exact adversarial case—an earlier in-tolerance shape record followed by a request-ID winner with an invalid timestamp—now selects the request-ID row but emits `clock_compared: false`, no delta, `duration_only`, and no source overlay. Targeted uncached analyzer tests, full `go test ./...`, `go vet ./...`, `go build ./...`, `npm run test:state`, and `npm run build` pass (macOS linker-version warnings only). | Codex B3 remediation complete with no frontend or binding change. One narrow B3 re-review remains; O1/O2/O4/O6–O10 do not gate it. |
| 2026-07-31 X-RG1 T-583 B3 narrow re-review | Accepted `PASS`. The remediation structurally separates candidate selection from emitted clock evidence and recomputes delta/`clock_compared` exclusively from `used[bestIndex]`. The original B3 probe no longer reproduces; the new regression demonstrably fails against the pre-fix analyzer and passes now; nine adversarial access-path probes cover missing/skewed/valid request-ID timestamps, competing shape candidates, cross-transaction isolation, and legitimate alignment without residual misattribution; a same-class sweep found the profile and Jennifer correlators clean. Full Go test/vet/build, uncached correlation tests, frontend state tests, and production build pass. No blocking X-RG1 finding remains. O11 adds non-blocking coverage hardening for two-shape-candidate selection/emission agreement; O1/O2/O4/O6–O10 remain non-blocking. | X-RG1 closed with `PASS`; T-583 `DONE`; T-584/R-RG1 unblocked. Review archived at `docs/review/done/2026-07-31_claude-code_X-RG1_http-evidence-correlation-b3-narrow-re-review.md`. |
| 2026-08-03 R-RG1 T-584 conditional remediation handoff | R1: `[Unreleased]` now covers Chrome/V8 CPU analysis, Lighthouse, HAR, Windows live capture, HTTP Diff, and HTTP evidence correlation while retaining explicit live-tier security and fidelity limits; paired importer matrices no longer claim a stale v0.3.5-only scope. R2: manual Windows GUI smoke run `30809473903` built an 18,909,184-byte Wails executable from release-candidate commit `f5efea8` and kept it alive for the 15-second smoke window. R3: the evidence sidecar and active records now pin the repository's LF-normalized artifact digest `2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e`, while the original Windows CRLF digest `69565684…30111` remains recorded as provenance. | R1–R3 remediation complete without product source, tag, or release changes. T-584 remains `REVIEW` solely for a narrow independent R-RG1 re-review. |
| 2026-08-03 R-RG1 T-584 narrow re-review | Accepted `PASS`. R1: the six `[Unreleased]` feature entries were cross-checked against `git log v0.3.5..HEAD` (138 non-merge commits) and cover every user-facing family; the Security paragraph and narrow live-tier qualifiers are retained, and the stale importer-matrix scope (O5) is fixed in both languages. R2: run `30809473903` was verified from GitHub metadata as a successful manually dispatched Windows GUI Smoke run on `f5efea8` whose pinned script enforces the ≥1 MiB size floor, launch, and 15-second liveness; the restored plan/registry scope matches, and the only commit after the smoked one is documentation, so the smoked binary matches the shipping product source. R3: the repository artifact recomputes to the pinned LF digest `27371338…7218c7e`, CRLF re-expansion recomputes the original `69565684…30111`, provenance notes accompany every pin, and the closed H-RG4 fourth re-review carries a dated correction note instead of a rewrite. `b48511d` changes no buildable surface, so the first review's platform verification carries over. R-RG1 O1–O4/O6–O7 remain non-blocking. | R-RG1 closed with `PASS`; T-584 `DONE`; the tag/release constraint is lifted as a separate decision. Review archived at `docs/review/done/2026-08-03_claude-code_R-RG1_integrated-release-acceptance-narrow-re-review.md`. |
| 2026-08-10 T-587 HTTP Analyzer surface and capture-detail rework | Accepted as implementation-sourced decisions, recorded retroactively. The work closed three design-implementation gaps that needed no new decision (ko §8 packet-grid/detail screen grammar, §6.3 timing perspectives, §6.4 body-storage vocabulary) and made four decisions that did: `RD-130` originally made bodies a per-session opt-in with `bodyCaptureGate = "sec10_preflight_pending"`; `RD-131` split the transaction row budget from aggregate top-N; `RD-132` moved `LiveCaptureContract` to schema v2; `RD-133` kept resource-kind taxonomy renderer-side. **Process note: the decisions were made during implementation rather than registered as a `TODO` and written into the design notes first.** | Historical handoff, superseded for body enablement by the H-SEC2 remediation row below and `RD-134`. T-587 remains `REVIEW` for the independent UI/engine review and later R-RG1 re-run. |
| 2026-08-10 T-587 H-SEC2 body-tier re-review and remediation | Accepted `PASS AFTER REMEDIATION`, scoped to a dormant/unavailable tier. B1 (P0): `sec10_preflight_pending` was descriptive only and allowed plaintext retention despite H-SEC2 C1's preflight-before-body requirement. B2 (P1): gzip/deflate expansion could retain up to a separate 8 MiB bound instead of the advertised per-message cap. B3 (P2): the recorder retained its unredacted wire buffer after canonicalization. Remediation publishes `bodyCaptureAvailable = false`, makes renderer fallback fail closed, rejects `CaptureBodies = true` in `session.Manager.Start` before CA/session allocation and independently in `proxy.New`, uses the per-message cap for decoded expansion, and zeroes/resets the recorder buffer after apply. | H-SEC2 body gate closed for the unavailable state; review archived at `docs/review/done/2026-08-10_codex_H-SEC2_body-tier-security-rereview.md`. SEC-10 implementation plus another H-SEC2 review is mandatory before activation. T-587 remains `REVIEW` for independent UI/engine review, then R-RG1 re-run. |

## Mid-Term Plus Intake Plan

| Classification | Priority | TO-DO range | Target release | Execution rule |
|---|---|---|---|---|
| Completed Mid-Term Plus roll-up | P1/P2 | T-468 through T-555 | v0.3.5 | Shipped as one stable release covering ingestion foundations, access/edge, server, observability, database, broker, platform, profiling, stitching, API/event contracts, architecture docs, and documentation alignment. |
| Long-term external APM import | P2/P3 | Not assigned | Roadmap candidate | Keep direct SaaS/product connectors deferred until local token policy, redaction, compliance, connector fixtures, and evidence contracts are ready. |
| Security/compliance evidence | P1/P2 | Not assigned | Roadmap candidate | Promote only evidence-backed checks after the external APM roadmap and next release objective are reviewed. |
| Evidence Studio stabilization | P1/P2 | Not assigned | v0.4.0 candidate | Treat the full importer, contract, stitching, report-pack, and AI-gated interpretation workflow as one user story before a broader roll-up release. |

## Version Milestone Plan

| Release | Complete through | Release type | Required contents before cut |
|---|---|---|---|
| v0.3.5 | T-468 through T-555 | Stable Evidence Studio expansion | Released 2026-05-17 with Mid-Term Plus importers, API/event contract analysis, architecture docs drafts, advanced stitching, version metadata, changelog, release tag, and release workflow verification. |
| Next candidate | T-560 through T-565 and T-578 complete | Chrome browser/V8 profile slice | C-RG1 returned independent `PASS` on 2026-07-21. Candidate version/tag selection remains a separate release decision; no release is cut by this status transition. |
| Future candidate | T-571 and T-580 through T-584 review-gated | Windows-first HTTP capture and cross-OS evidence analysis slice | C-RG1/T-578, H-RG1 through H-RG5 (T-579, T-571, T-580, T-581, T-582), X-RG1/T-583, and R-RG1/T-584 integrated release acceptance have all passed; the review-group ladder is closed. Candidate version/tag selection remains a separate release decision. Analyze Linux/macOS-generated supported evidence in the Windows UI. |
| v0.4.0 candidate | Not assigned | Evidence Studio roll-up | Full local evidence workflow is smoke-tested as one product story with sample packs, report exports, regression tests, AI gate checks, and release notes that present the expanded capability coherently. |

## Task Registry

Post-T-555 work includes the Jennifer MSA drilldown follow-up, the Chrome
DevTools/V8 CPU profile design gate reviewed on 2026-07-18, and the system HTTP
capture design gate reviewed on 2026-07-19. The Chrome profile acquisition-format
and measurement-unit contracts were resolved on 2026-07-20 (T-558, T-559), so
Chrome parsing, bounded streaming, desktop integration, and sampled CPU run
projections are now implemented, and the T-565 fixture/documentation parity
set was completed on 2026-07-21 (browser-profile fixture corpus plus paired
English documentation). Go golden-test wiring to the fixture manifest and the
local release smoke were completed on 2026-07-21.
The 2026-07-21 C-RG1 review first returned `CONDITIONAL`, then returned `PASS`
on independent re-review after all A-E engine and U1-U5 UI findings were
resolved. The exact sampled-run golden distinguishes attribution direction;
endTime tail and four duration fields are emitted; trace negative-delta and
hitCount diagnostics match cpuprofile; `startTime == 0` is safe; diagnostics,
timeline suppression, flamegraph drilldown, gzip discovery, i18n/a11y, and
frontend regressions are present; and the shared corpus has 16 fixtures. T-578
is complete and H-RG1 is unblocked.
HTTP work began with bounded, sanitized HAR import. Phase 1 HAR import passed
H-SEC1 and the full H-RG1 group gate on 2026-07-21. Live MITM capture is
Windows-first; T-571/H-RG2 passed on 2026-07-27, T-580/H-RG3 passed on
2026-07-28, and T-581/H-RG4 passed on 2026-07-30 after a narrow fourth
re-review verified the V1–V3 conditions resolved.
Linux/macOS-generated supported
HAR, profiles, and logs remain valid offline inputs to the Windows UI; Linux/macOS
live-capture parity is not a first-release gate. The Korean design note was
revised on 2026-07-19 to absorb the review: it now states capability tiers
instead of an "all processes" requirement, splits fidelity into
semantic/decoded_wire/raw_wire, separates timing perspectives with explicit
known/not-applicable/unknown states, keeps `AnalysisResult` size-stable behind a
versioned session store, defines backpressure and loss accounting, and hardens
the security defaults. A further 2026-07-20 revision wrote the Phase 0 design
gates themselves: the source ledger, the Windows capability/fidelity matrix, the
transaction/time contract, the bounded session store, and the security model
(T-566 through T-570). With those five closed, **no design gate blocks Phase 1
HAR import**; T-571 keeps its contract half only, because proving Windows
coverage signals needs a real-NIC measurement run on Windows. T-572 through
T-575 are complete: fixture corpus, aggregator/recovery protocol, proxy spike,
and HTTP-specific Diff contract. Other roadmap candidates,
including long-term external APM import and security/compliance evidence, remain
deferred until explicitly promoted.

| ID | Priority | Status | Task | Depends on | Output |
|---|---|---|---|---|---|
| T-614 | P1 | DONE | Prevent stale derived artifacts from silently entering a current Report Pack | Existing Report Pack/timeline/SLO/service-flow contracts | Completed 2026-11-10: Report Pack rebuilds timeline, SLO metrics and service flow from current source results on every generation. Only validated saved SLO target settings are retained, including an intentional empty policy. Source replacement/removal/addition, same-ID content changes and customized threshold regressions plus frontend state/build pass. |
| T-615 | P1 | DONE | Correct Trace Import elapsed critical-path calculation | Existing Trace Import and downstream latency contracts | Completed 2026-11-10: observed ancestry-chain elapsed estimate uses validated contained intervals, with inclusive work separate. New semantics explicitly do not prove a causal critical path. Missing/skewed/duplicate/cyclic input emits null/reason and availability counts; 20k-depth iterative graph, nested/sequential/parallel/zero and downstream SLO/timeline regressions pass with Go/CLI tests and frontend state/build. |
| T-616 | P1 | DONE | Avoid double-counting one RPC's CLIENT and SERVER spans | Existing Trace Import dependency and Service Flow/SLI contracts | Completed 2026-11-10: one-to-one explicit CLIENT/SERVER pairs use one logical call, client elapsed time and either-side error; single-sided exports, cross-trace identities, separate retry pairs and multiple server attempts remain distinct. SERVER remote peers use incoming direction. Go/CLI tests and frontend Flow/SLI counter regressions pass. |
| T-617 | P1 | DONE | Preserve missing access-log response time as unavailable rather than zero | Access-log parser/analyzer and SLI/SLO contracts | Completed 2026-11-10: all supported parsers preserve response-time availability, including true zero; unknown samples are excluded from total/static/API/URL/minute/route/edge latency while full request/error/byte counts remain. Null empty stats, measured/unknown counts, warning finding, sample coverage UI and measured-only Flow weighting propagate to SLI/SLO; targets without evidence are explicitly not assessed. Parser/analyzer/CLI and frontend state/build regressions pass. |
| T-618 | P2 | DONE | Disclose Incident Timeline truncation and qualify severity/time summaries | Existing Incident Timeline/Report Pack contracts | Completed 2026-11-10: 120/source and 1000/session row caps remain, with deterministic documented selection, total/emitted/omitted and per-source/severity loss counts. Severity/time/ranged/correlation summaries use all candidates; groups/narrative explicitly remain retained-only. Page/JSON/Report Pack HTML disclose partial output and omitted critical events. Both-cap, late-critical, empty-input and export regressions plus frontend state/build pass. |
| T-620 | P1 | PENDING | Measure all 21 views at the default window size | — | Prepared opt-in native layoutaudit build and 21-route collector. Locked Mac produced no ready report; no native geometry or screenshot claimed. User deferred interactive checks until evening on 2026-11-10; implementation proceeds using the source baseline (224px sidebar, at least 54px topbar). |
| T-621 | P1 | REVIEW | Move global navigation to native menus | T-620 | Implemented one native menu for all 21 routes, localized theme/locale/About, active open capability, retained tool state and macOS drag strip. Profiler Open/Settings no-op callbacks replaced, native accelerator duplication avoided. App Go tests and frontend state/build pass. Native menu interaction awaits evening unlock. |
| T-622 | P1 | REVIEW | Compact shared input controls and help | T-620 | Implemented compact source rows, inline options, collapsed recent files, viewport-positioned help with click pinning and delayed pointer transfer, and modal-aware Escape. Frontend production build and state regressions pass; hover/focus interaction and native clipping checks await evening unlock. |
| T-623 | P1 | REVIEW | Prioritize the HTTP grid and inspector | T-621, T-622 | Implemented flexible imported/live HTTP grid height, 160px default inspector, grid row budget, collapsed file tools/advanced filters and bounded configuration drawers. Search opens filters and hidden-page shortcuts are scoped. Frontend state/build pass; ten-row/four-line and recording interaction acceptance awaits unlocked Mac/Windows. |
| T-624 | P1 | REVIEW | Prioritize profiler and GC results | T-621, T-622 | Profiler now opens available flamegraph after analysis with summary fallback. GC Summary shows adaptive pause trend beside a shared ten-event table preview and collapses the long metric list. Input/options spacing reduced. Frontend build/state regressions pass. Real frame hit testing, narrow window geometry and no-clock fixtures await unlocked native verification. |
| T-625 | P2 | REVIEW | Convert nine remaining analysis views | T-621, T-622 | Applied dense result layouts to the nine remaining analyzers, compact shared metric/card rows and preserved conditional tabs, source labels and errors. Lighthouse guidance is on demand with imported-score origin/recomputed status visible. Frontend state/build pass; all-route/subview native row and clipping measurements await evening verification. |
| T-626 | P2 | PENDING | Convert eight workspace views and Settings | T-621, T-622 | Check eight workspace views and Settings. Measure data-first layouts and test registration, evidence and export handoff. Verify chart template/renderer, output destination and Settings save at small sizes. |
| T-627 | P2 | PENDING | Verify all views in native desktop windows | T-623–T-626 | Compare matching native before/after cases for all routes and inventoried subviews. Record ratio/row/plot or form verdict, keyboard/locale and resize checks. Validate OS menu navigation and actual Windows live re-entry separately. |
| T-619 | P1 | DONE | Enforce the Wails 3.1.0 desktop-only product and remove unused launch templates | Existing native desktop packaging | Completed 2026-11-10: removed server hosting/mobile/preview paths, added compiler and frontend native-transport guards with CI enforcement, isolated opt-in external Electron HTTP probe without weakening full acceptance, and refreshed paired guides. Full Go/vet and frontend state/build, native macOS package/signature, Windows x86/x64 GUI builds and expected server build refusal pass; Windows runtime acceptance remains separate. |
| T-599 | P1 | DONE | Establish the V7.6.4 Stand-alone visual and command reference | Existing baseline | Completed 2026-11-09. Official screen, V7 additions, complete manual field inventory, state behavior, shortcuts and edition exceptions recorded in paired SYSTEM_HTTP_CAPTURE references. The tour image is explicitly V4; native Windows acceptance remains T-603. |
| T-594 | P1 | DONE | Keep capture controls, live grid and selected detail in one window | Existing baseline | Completed 2026-11-09. Live rows use the shared grid and inspector, one-ID redacted store fetch stays available during recording, and async detail ignores obsolete selections. Real local proxy/service regression and frontend build passed; browser renderer at 1280x800 showed selected headers/body while active. Native Windows integration remains T-603. |
| T-595 | P1 | DONE | Make process grouping the default in the request grid | Existing baseline | Completed 2026-11-09. Shared virtual grid defaults to expandable process groups with request/byte totals; live process selector scopes incoming rows by PID plus start time and clears obsolete detail. Same-name PID reuse and collapse regressions, frontend state suite and production build passed. Native integration remains T-603. |
| T-596 | P1 | DONE | Select single or multiple running processes before recording | Existing baseline | Completed 2026-11-09. Windows process snapshot exposes PID/start-time targets through Wails; selected keys enforce a pre-store and pre-publication scope. Unknown owners and PID reuse are rejected within selected scope. Stream/session regressions and frontend build passed. Native two-client selection verification remains part of T-603. |
| T-597 | P1 | DONE | Make paths readable and retain the active detail tab | Existing baseline | Completed 2026-11-09. Explicit path width and horizontal grid scrolling preserve readable URLs. The active detail tab survives adjacent row changes; splitter height persists and clamps to available panel space. Production build and browser checks confirmed 224px path cells, retained Body tab and process-scoped selection. |
| T-600 | P1 | DONE | Rebuild the desktop menu, recording toolbar and status bar | T-599, T-594 | Completed 2026-11-09. Native Wails HTTP Capture menu, F9/F2/Ctrl+L, toolbar and recording status share guarded commands. Engine pause and clear preserve the proxy address, reset log rows/totals and reject old cursors/events. Race capture/app suites, real proxy service control test, frontend state/build and browser controls passed. Native Windows command acceptance remains T-603. |
| T-601 | P1 | REVIEW | Deliver the four Windows capture entry points and measure their reach | T-596 | Added an explicit registry/factory/capability boundary and desktop backend selector; unsupported transparent/unknown backends fail before launch, CA/body/store allocation with no proxy fallback. Backend choice persists in session/manifest. Partial-start cleanup releases owned resources; synchronized suspended-child handles prevent double resume/abort and reused-handle termination. Local failure/race tests and Windows x86/x64 compilation pass. Added opt-in hash-verified WinDivert 2.2.2-A IPv4 TCP 80/443 redirection, bounded 5-tuple/process/scope mapping, original-route HTTP/TLS integration and driver failure/cleanup path. Local mapping/stream/race tests and Windows compilation pass; actual injection, proxy-ignoring/service reach and driver-policy compatibility remain unverified. |
| T-602 | P1 | DONE | Preserve the HTTP exchange and its intermediate responses | Existing baseline | Completed 2026-11-10. Accepted connections and admissions carry stable connection IDs and monotonic request numbers. Bounded 1xx evidence, redirect targets, receiving state and stream provenance survive model/store/analyzer/Wails detail. Incomplete body transfers fail explicitly. Redirect/intermediate/connection, HTTPS receiving-state and pre-store redaction regressions plus frontend state/build passed locally. |
| T-603 | P1 | DONE | Prove the everyday inspection workflow in the packaged Windows app | T-599, T-594, T-595, T-596, T-597, T-600 | Completed 2026-11-09 on native Windows Wails/WebView2: six synthetic POSTs from two PID/start-time clients; requested 1280×800 window (1266×793 bounded screenshots); process filtering; live Headers and actual request/response Body during arrivals; F9/F2; stopped-store reopening with identical markers and Korean text. Temporary dump policy and user WER exclusions restored with exact original tree hashes, including ACL metadata. Evidence: `docs/review/evidence/2026-11-09-windows-body/README.md`. |
| T-598 | P2 | DONE | Add decoded query, POST form and JSON inspection | Existing baseline | Completed 2026-11-10. Added ordered Query String and Post Data inspectors, raw/decoded parameter columns, bounded JSON tree navigation and explicit malformed/truncated/unavailable states. Redaction now preserves benign encoded field order and spelling. State/parser/redaction regressions, frontend build and actual renderer query/form/JSON interactions passed. |
| T-604 | P2 | DONE | Configure columns, group and filter requests, and search the entire session | T-599, T-595 | Completed 2026-11-10. Migrating column preferences support visibility, width and order; process/host/method/status/MIME grouping and generic field comparisons share stable virtual rows. Bounded store-backed search covers URL, headers, retained bodies, cookies, query, POST and reconstructed streams beyond the live ring, with snapshot paging and selected-only payload fetch. State/migration, native service boundary tests, frontend build and renderer interactions passed. |
| T-605 | P2 | DONE | Complete live timing charts and process/session summaries | T-595, T-602 | Added displayed/store-wide/process summary scopes, measured/unknown byte totals, compression pairs, deduplicated TCP/DNS, separate phase perspectives and process-relative bounded waterfall with status-to-grid filtering. Store-wide boundary fixture, counts/timing/PID reuse regressions and frontend state/build pass locally; visible native acceptance remains explicitly unverified. |
| T-606 | P2 | DONE | Extend Headers, Cookies, Stream and Content viewers | T-598, T-602 | Added ordered cookie attributes with pre-store value masking, sent/received/combined stream, bounded syntax/XML/SOAP and UTF-8 hex views, inert formatting-only HTML, and opt-in bounded re-encoded PNG previews for live/HAR/Builder. Pixel data is explicitly unredacted; originals, SVG and oversized/truncated rasters are not retained. Metadata/event/overflow/compact paths omit previews. Capture/parser/session, safety fixtures and frontend state/build pass; native visual acceptance remains unverified. |
| T-607 | P2 | DONE | Open, save, clear and export capture sessions from the desktop | Existing baseline | Desktop open/save/export now supports canonical JSON, HAR 1.2, full/compact XML, URL/text/HTML, BIFF8 XLS and response-preview directories; explicit all-store/displayed-row scope, preserved identity/selection, redacted round trips and cancellation pass. XLS independently verified with xlrd including Korean/1000 rows; Windows native dialogs remain T-613 acceptance. |
| T-608 | P2 | PENDING | Read and write legacy .halog files | T-607 | No genuine sanitized V7.6.4 sample is available (confirmed by user). Official installer directory and nested automation ZIP now inspected without execution; TLBs and bundled 2009/library-5.0 Delphi contract metadata found, with no .halog entries in either listing; no codec inferred. Added bounded metadata-only container/hash probe and pair evidence, CLI/desktop access, synthetic reference-traffic generator and prevention of ArchScope JSON/XML masquerading as .halog. Local refusal/immutability/probe/fixture checks prepared; proprietary decoder/encoder and both reference directions remain unimplemented/unverified pending genuine samples. |
| T-609 | P2 | DONE | Integrate row hints, status help, stars and comments | T-594, T-607 | Added request-ID hints/status help, explicit evidence links, star/comment columns and starred-only filtering. Live annotations use an atomic bounded sidecar, verified IDs, redacted comments and Clear reset; finalized/native JSON/XML/ArchScope HAR preserve notes, while opened-file edits stay explicit export overrides. ID sorting, service and reopen regressions plus frontend state/build pass; native visual acceptance remains unverified. |
| T-610 | P2 | DONE | Add the Stand-alone HTTP Request Builder | T-594 | Desktop Request Builder supports fresh requests, selected-row copies and a fixed drag/drop target, editable headers/raw/form/multipart files, explicit send/cancel and separate response inspection. Local raw/form/multipart/redirect/timeout/TLS/gzip fixtures, race detector, browser mouse copy/edit/send/drag with original-row preservation, Go suite/vet/build and frontend state/build pass; native Windows acceptance remains T-613. |
| T-611 | P3 | REVIEW | Support Stand-alone automation and desktop launch switches | T-596, T-607 | Implemented visible desktop launch options, bounded authenticated opt-in loopback control/client, PID/start-time attach contract, timer-stop/export, stored reads and resynchronizable payload-free events. Local real-proxy start/pause/resume/stop/save/reload, auth/retry/origin/descriptor tests and frontend build pass; Windows x86/x64 compile checked. Added bounded Windows .lnk executable resolution, verbatim link arguments plus quoted extra args, working directory and refusal of unsupported shell activation before allocation. Common resolution/refusal tests and x86/x64 test compile/vet pass; Added same-user/integrity native late-bound COM under an ArchScope-only ProgID, controller session ownership, log metadata/enumeration and selective WinINet cleanup. Bundled reference DISPIDs/export enums are implemented without claiming current TLB/binary parity. Original dual-IID/typeinfo/events, filter/retention and cross-process session-cookie semantics remain unimplemented; native Windows activation/client/shortcut/cache acceptance remains unverified. |
| T-612 | P3 | DONE | Restore legacy content inspectors | T-606 | Added bounded inert AMF0/AMF3 and remoting-packet readers, ViewState v1 token trees/text, references/traits/vectors/dictionaries, form/HTML chunk extraction and explicit invalid/limited/unsupported states. Classes/converters/binary objects are never instantiated; MAC/encryption remain unverified, opaque bytes omitted and secret trees/raw ViewState sources masked. Capture/HAR/Builder/portable paths, known-byte fixtures and 15-second fuzz pass; native/reference-EXE visual interoperability remains T-613. |
| T-613 | P3 | PENDING | Verify complete Stand-alone parity on Windows | T-601, T-602, T-603, T-604, T-605, T-606, T-607, T-608, T-609, T-610, T-611, T-612 | Prepared a 16-case evidence contract and native Windows fixture/control harness with source/binary provenance, hashed artifacts, isolated instance store, loopback traffic and private descriptor cleanup. Report contract/syntax tests pass on Mac; control data cannot count as native visual/reference evidence. Local regression/race tests now cover interruption, cleanup ownership, re-entry, controller retries and bounded replay; a 60.251s real local-proxy soak passed 72 cycles/18,720 synthetic requests. Windows execution/screenshots, multi-hour native soak, COM and genuine .halog remain unverified; overall acceptance is not complete. |
| T-589 | P1 | DONE | Restore unified GC uptime/date coordinates and valid throughput/allocation/promotion rate denominators; define observation-window rules, expose unavailable values honestly, and align absolute versus elapsed UI axes. | None; existing GC parser/analyzer and AnalysisResult envelope | Completed 2026-11-09: unified date/uptime parsing, monotonic observed-window accounting, nullable unavailable metrics with reasons, elapsed/absolute axes and EN/KO disclosure. GC parser/analyzer regressions, frontend state suite and production build passed; local source review completed (no independent or desktop E2E claim). |
| T-590 | P1 | DONE | Preserve internal-frame self samples in the shared Canvas flamegraph so totals, widths, tooltips, and search use the original inclusive weights without double counting. | None; coordinate hidden-weight fixtures with T-591 | Completed 2026-11-09: Canvas derives self weights from inclusive nodes; shared layout/search helpers preserve prefix-stack totals and union recursive matches. Prefix, leaf, recursion, focus/search and synthetic-hidden-weight regressions plus frontend state suite/build passed; local source review completed. |
| T-591 | P1 | DONE | Apply profiler drilldown filters to canonical evidence before display pruning; conserve/disclose omitted subtree weight while keeping every result stage within node/payload limits. | None; coordinate Canvas accounting with T-590 | Completed 2026-11-09: filters consume canonical Go-only evidence before each bounded display stage; pruning preserves explicit hidden-descendant weight without mutating later filter input. Service tests verify rare=10 at node budgets 1, 3 and 100; profiler budget/accounting tests, app tests, generated Wails bindings and frontend state/build passed; local source review completed. |
| T-592 | P1 | DONE | Resolve legacy profile counter-unit and sample-count/total-metric provenance before converting lock weights to durations; keep summary, drilldown, and pprof export units consistent and disclose insufficient provenance. | None; source-unit contract must precede binding/UI changes within this task | Completed 2026-11-09: explicit counter units and aggregation provenance; known time weights convert directly, lock counts and unknown/non-time weights emit nullable durations. Summary, timeline, drilldown, Canvas labels and pprof agree. Unit/pprof-file tests, real CLI invocation, Wails propagation, CPU/wall/V8 regressions, generated bindings and frontend state/build passed; local semantic review completed. |
| T-593 | P2 | DONE | Prevalidate and precompile every ordered-regex term; propagate INVALID_REGEX/UNSAFE_REGEX consistently instead of silently returning no matches or accepting invalid excludes. | None; existing filter behavior and RE2 guards | Completed 2026-11-09: validates the full ordered-regex budget and precompiles every term before traversal; invalid or unsafe include/exclude filters emit diagnostics even on unreachable terms and empty input. Ordered/case/reroot/exclude regressions passed. Whole Go test suite, vet/build and frontend state/build passed; local source review completed. |
| T-577 | P0 | DONE | Read the Chrome/V8 and system HTTP-capture designs and completed reviews, reconcile them with current code, and publish a paired English/Korean implementation plan with Codex engine ownership, Claude UI ownership, grouped review gates, and narrowly scoped individual security/semantic reviews. | T-558 through T-576 | Completed 2026-07-21: paired `BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`; stale T-565 documentation status corrected; work status and execution queue aligned |
| T-578 | P0 | DONE | Resolve the C-RG1 `CONDITIONAL` verdict and obtain independent re-review `PASS`. Codex A-E: reconcile the time-attribution contract with real Chrome semantics and add an exact convention-pinning golden plus tail handling; emit separate recording/active/idle/sampled durations without conflating `total_duration_us`; add Chrome-trace negative-delta diagnostic fixture/parity and hitCount cross-check; make first-sample handling independent of `startTime`. Claude U1-U5: render engine diagnostics and explicit suppressed/aggregated timeline states; add flamegraph/drilldown; expose `.json.gz`; add empty-state/i18n/table a11y; add Browser CPU regression tests for diagnostics, suppression, and the non-Long-Task wording. | T-565, T-577 | Completed 2026-07-21. Codex A-E landed in `5db8df8`, `ed01dd0`, `7b24206`, `40c6bde`, and `3249c27`; Claude U1-U5 landed in `b023040`; shared exact/negative-trace fixtures are in projects-assets `de7cf07` and `5531036`. Independent re-review at `b023040` returned `PASS` after verifying all ten findings, 16-fixture goldens, engine tests, frontend state tests, EN/KO i18n parity, `go vet`, and full builds. H-RG1/T-579 is unblocked. |
| T-579 | P0 | DONE | Complete and review H-RG1 offline HAR analysis. Codex: canonical transaction/timing/fidelity model, staged dialect normalizer, resource limits, shared manifest goldens, dedicated redaction, bounded result and CLI/Wails parity. Claude: pseudo-process tree, timeline/brush, list/detail/filter, fidelity/diagnostics/redaction UX and Workspace regression. | T-578 PASS; T-568 through T-573 | Completed 2026-07-21 with integrated `PASS`. Engine remediation and H-SEC1 re-reviews are `PASS`; all 20 manifest fixtures, bounded/redacted `AnalysisResult`, CLI/Wails parity, and UI import-only behavior are retained. UI R1 was accepted under the disclosed bounded Phase 1 denominator contract; R2 was accepted from populated state, provenance/Workspace, type, and production-build evidence with deeper component fixtures left as non-blocking hardening. Full `go test ./...`, `go vet ./...`, `go build ./...`, frontend `npm run test:state`, and frontend `npm run build` pass. |
| T-580 | P0 | DONE | Implement and review H-RG3 live-capture engine foundation: session lifecycle/recovery, versioned NDJSON/blob store and cursor API, bounded streaming/backpressure/loss counters, H1 semantic MITM with H2 passthrough, Windows process attribution, Wails snapshot/event recovery, and CA/TLS policy. | T-571/H-RG2 PASS, T-579 PASS | Completed 2026-07-28 with independent H-SEC2 `PASS`. Implemented 2026-07-27 with append-only recovery/index, bounded pipeline and loss counters, H1 verified-upstream MITM, honest H2 passthrough, direct Windows TCP-owner attribution, generated CaptureService bindings, and transactional current-user CA trust lifecycle. H-SEC2 verified SEC-1/2/3/6/7 redaction-before-persistence with no plaintext-body storage, SEC-8/9 memory-only non-exportable CA key, SEC-11 `0700`/`0600` perms with path-traversal-validated IDs, SEC-12 transactional trust rollback/removal, SEC-13/14 no auto-bypass, SEC-16 no CLI/headless start, verify-always upstream TLS, and loopback-only bind. SEC-10 dump-exclusion preflight (before body capture) and SEC-17 explicit unknown-attribution opt-in (before the live UI) bind the next tier without reopening the gate. `go test ./internal/capture/...`, `go test ./cmd/archscope-app/...`, `go vet`, and `go build ./...` pass. Review archived at `docs/review/done/2026-07-28_claude-code_H-SEC2_http-capture-live-engine-security-review.md`. |
| T-581 | P1 | DONE | Implement and review H-RG4 Windows live-capture UI and E2E. Claude: capture controls, CA lifecycle UX, process tree, stable live rows, recovery/backpressure/coverage/fidelity states. Codex: frozen bindings, acceptance fixtures, Windows E2E and packaging support. Carry the H-SEC2 binding conditions: gate unknown-attribution retention behind an explicit metadata-only opt-in (SEC-17) before exposing stored transactions, and require the SEC-10 dump-exclusion preflight before enabling body capture. | T-580 PASS | Completed 2026-07-30 with independent H-RG4 group `PASS` (narrow fourth re-review of V1–V3). The fixture-only replacement artifact has 1,012 loopback-only archived rows, 11 explicitly omitted background rows, confirmed fixture pinning evidence, no contradictions, and LF-normalized repository checksum `27371338…7218c7e` (original Windows CRLF checksum `69565684…30111`); the harness derives its privacy declaration from the published output and the Go acceptance fixture binds the enforcement mechanisms. SEC-17 is enforced below the UI; SEC-10 continues to bind any future body-capture tier (this slice stores no bodies). The V3 resolver-cost measurement is owed at R-RG1. Review archived at `docs/review/done/2026-07-30_claude-code_H-RG4_windows-live-capture-ui-e2e-fourth-re-review.md`. |
| T-582 | P1 | DONE | Implement and review H-RG5 HTTP-specific session Diff with versioned URL templates, bounded dimensions, explicit rate denominators, time-alignment grades, `http_capture_diff` findings, Workspace routing, and grade-aware comparison UI. | T-581 PASS, T-575 | H-RG5 group `PASS` 2026-07-30; reordered equivalent sessions compare equal end to end; O1–O4 non-blocking observations recorded |
| T-583 | P1 | DONE | Implement and review X-RG1 HTTP correlation with Chrome/V8 CPU runs, Jennifer network-gap evidence, and access logs, including bounded alignment/confidence diagnostics and provenance-aware UI drilldown/overlay. | T-582 PASS (2026-07-30) | Completed 2026-07-31 with B3 narrow re-review `PASS`. B1/B2/B3 are verified closed across engine and renderer; O1/O2/O4/O6–O11 remain non-blocking and O3/O5 are closed. Review archived at `docs/review/done/2026-07-31_claude-code_X-RG1_http-evidence-correlation-b3-narrow-re-review.md` |
| T-584 | P0 | DONE | Run R-RG1 integrated release acceptance across Go test/vet/build, frontend test/build, Windows GUI/live E2E, macOS offline import/package smoke, paired documentation, support/security/performance matrices, and honest release notes. | T-583 PASS (2026-07-31) | Backend/platform acceptance passed locally and in Actions run `30601745253`; Windows live E2E and GUI smoke run `30809473903` passed, resolver evidence confirmed 50/50 samples (mean 0.539 ms, p95 0.731 ms, max 0.857 ms), and the R1–R3 remediation passed the narrow independent re-review. R-RG1 `PASS` (2026-08-03); no tag or release was created by this gate |
| T-585 | P1 | DONE | Implement the file-first Lighthouse report JSON engine slice as the browser-performance roadmap task that can proceed while T-571 waits for hardware: separate parser/analyzer packages, bounded/redacted projections, preserved report scoring, CLI, ingestion-family contract, Wails binding, tests, and paired documentation. | T-565; independent of T-571 | Completed 2026-07-21: `browser import --format lighthouse-json`, `browser_audit_evidence`, `AnalyzeBrowserAudit`, generated TypeScript bindings, Core Web Vitals/category/audit/resource outputs, report-authored score findings, 64 MiB default input guard, URL redaction, bounded audit/network tables, and targeted Go tests. |
| T-586 | P1 | DONE | Add the dedicated Lighthouse desktop page under Browser Performance, consuming only `AnalyzeBrowserAudit`; provide collection guidance, score/metric cards, category/audit/resource views, diagnostics, Analysis Workspace registration, Evidence Board capture, report export, and populated-state/provenance regressions. | T-585 | Backend handoff completed 2026-07-22: versioned `browser_audit_contract`, explicit imported-score/non-recalculation disclosure, stable row/finding `source_ref` values, source metadata, declared view/evidence/export surfaces, sanitized populated fixture, Wails payload/redaction regression, and JSON/HTML export regression. Frontend completed 2026-07-22: `LighthouseAuditPage` + `bridge/engine.ts` `analyzeBrowserAudit` wrapper + `state/browserAudit.ts` pure selectors, sidebar/App route wiring, EN/KO localized guidance/provenance/views, score-provenance disclosure banner, category/Core Web Vitals/audit/network/resource views, diagnostics, Analysis Workspace registration, Evidence Board finding/audit-row capture preserving `source_ref`, JSON/HTML report export, and populated-state/provenance regressions in `state/regression.test.ts`. `npm run test:state`, `npm run build`, and the Lighthouse Go tests pass; `browser_audit_evidence` stays separate from `profile_evidence`. |
| T-587 | P0 | DONE | Rework HTTP capture presentation and capture detail so the desktop surface matches the §8 HTTPAnalyzer screen grammar: record proxy-upstream/proxy-internal timing phases in the live proxy, give the transaction table its own row budget so resource requests are not dropped, add resource-type classification and filtering, add an HTTP Analyzer-style packet grid over a resizable detail pane with a reconstructed raw-protocol tab, and prepare a body-processing tier without violating SEC-10. | T-581 PASS, T-582 PASS, T-584 PASS | Completed 2026-08-11. The UI/engine review closed connection-blocked timing, malformed contract-v3 fail-closed handling, and resource-filter option derivation, then returned `PASS AFTER REMEDIATION`. T-588 supplied the reviewed SEC-10 activation and the post-change R-RG1 re-run passed. Reviews: `docs/review/done/2026-08-10_codex_H-SEC2_body-tier-security-rereview.md` and `docs/review/done/2026-08-11_codex_T-587_http-analyzer-surface-review.md`. |
| T-588 | P0 | DONE | Implement the SEC-10 crash-dump exclusion preflight and activate the dormant body-capture tier, so the HTTP Analyzer raw/body view can show an actual payload. Windows only, because live capture is Windows only. Engine: a `capture/dumppolicy` package following the existing `_windows.go`/`_other.go` split that reads and verifies WER policy before authority generation or plaintext retention; both engine entry points bind the verified result; contract v3 exposes a fail-closed renderer state; and a forced-crash harness scans Windows artifacts and the Application log. | T-587 H-SEC2 body-tier `PASS AFTER REMEDIATION` (2026-08-10) | Completed 2026-08-11. Engine Native run `31488911063` forced a fail-fast crash while real request/response recorders and live CA keys were resident, found zero new or modified artifacts and zero canary matches, restored the pre-existing `LocalDumps` policy, and archived checksummed evidence. The activated-tier H-SEC2 review returned `PASS AFTER REMEDIATION`; post-change R-RG1 passed. Review: `docs/review/done/2026-08-11_codex_T-588_H-SEC2_activated-body-tier-review.md`; evidence: `docs/review/evidence/2026-08-11_t588_sec10-dump-exclusion.json`. Residual scope remains active debugger access, mid-session administrator policy changes, kernel/hypervisor acquisition, and pagefile/hibernation exposure. |
| T-566 | P0 | DONE | Add a reproducible source ledger for every HTTP-capture research claim marked `[V]`, including official URL or source commit, retrieval date, verified fact, and design impact; scope absolute competitive-gap claims to what was actually inspected. | None | Completed 2026-07-20: appendix A rewritten as a filled ledger — claim-ID scheme (`V`/`I`/`N`/`Q` × area × topic), `fixed`/`partial`/`open` status values, four ledger tables (platform/spec, Windows capture, HTTPAnalyzer, competitors), and a refresh procedure. Gate condition restated as "no design decision rests on an `open` row" rather than "all rows verified"; `Q-COMP-MITM-PID` and the 1.5.4 absolutes are recorded as `open` and withdrawn from use |
| T-567 | P0 | DONE | Replace the global “all processes” requirement with a Windows-first platform/mode capability and fidelity matrix covering process attribution, HTTP versions, header/body fidelity, timing perspective, privileges, and explicit unsupported cases; separate Windows runtime support from cross-OS offline evidence compatibility. | T-566 | Completed 2026-07-20: §9.3 adds six per-mode matrices (attribution, HTTP versions, fidelity, timing perspective, privileges, explicit unsupported) across proxy/HAR/ETW/WFP/Npcap/keylog with `미검증` cells reserved for the T-571 spike; §9.4 splits runtime support (Windows only) from evidence compatibility (all OSes) with separate acceptance criteria, and records that HAR dialect differences are a function of the generating tool, not the generating OS |
| T-568 | P0 | DONE | Redesign the HTTP capture model so unknown/not-applicable/zero timings, client-proxy versus proxy-upstream perspectives, transaction state, connection reuse, header semantics, decoded versus wire bytes, and fidelity are represented without overclaiming Go `net/http` guarantees. | T-567 | Completed 2026-07-20: §6.3.1 fixes transaction boundaries (start = first request byte, end = last response byte/terminal state) with eight ambiguous cases resolved; §6.3.2 separates monotonic durations from wall-clock instants and locks ms/`float64`; §6.3.3 adds the state machine with transitions and aggregation rules; §6.3.4 adds versioned invariants `INV-1`~`INV-7` plus opposing H1/H2 golden sets (`INV-H1-1` non-overlap vs `INV-H2-1` overlap-permitted) |
| T-569 | P0 | DONE | Define one bounded session-store and `AnalysisResult` boundary: keep the envelope size-stable, page details through a versioned cursor API, specify capture lifecycle/crash recovery, and define disk-slow/full backpressure and loss accounting. | T-567 | Completed 2026-07-20: §7.6 defines `CaptureSessionStore` — append-only NDJSON layout with rebuildable offset index, three independent version fields, per-tier memory ceilings including truncated live-window records and non-resident offset index, eviction policy with chunked lazy loading and store-side filtering, disk spill limits with `stop`/`body_only`/`rotate` policies defaulting to `stop`, and the interface contract with snapshot-versioned cursors and single-writer semantics |
| T-570 | P0 | DONE | Complete the HTTP capture threat model and safe defaults for imported HAR, CA keys, TLS passthrough, JWT/cookies, blob identifiers, process metadata, user redaction rules, manifests, exports, and resource limits. | T-567 | Completed 2026-07-20: §11.2.1 adds the CA lifecycle state machine (`none`/`generated`/`trusted`/`expired`) with per-store partial-failure rules, rollback on partial install, and explicit "app removal does not remove the CA"; §11.6 consolidates the privilege contract around what elevated code may do (no resident elevated process, helper captures only, peer-credential IPC); §11.7 adds the `SEC-1`~`SEC-15` adversarial matrix, with `SEC-4`~`SEC-7` applying from Phase 1 |
| T-571 | P0 | DONE | Prove or remove Windows coverage signals first, distinguishing TCP endpoint ownership, ETW event attribution, WFP/Npcap observations, and adapter counters by real scope; do not expose `bytesObserved/bytesSystem` as a process ratio without same-scope evidence. Defer Linux/macOS live-capture proofs without blocking Windows. | T-567 | Completed 2026-07-27 with independent H-COV1 `PASS` after `CONDITIONAL` remediation. ETW is discarded after 197 distinct false PID/port pairs despite zero loss. WFP had no relevant ALLOW events and is removed as a product candidate without claiming audit-enabled capability. Direct IPv4/IPv6 `GetExtendedTcpTable` retained 5/5 persistent-endpoint attribution, zero false pairs, bypass detection, and improved CAP-5 from 14.4%p fail to 4.8%p pass. Because CAP-3 is N/A, only individual endpoint attribution plus the five self-observed counters are allowed; absolute ratios are forbidden. Committed ground truth/observations/reports are normalized evidence, not a complete source-level raw package. |
| T-558 | P0 | DONE | Decide whether the first supported browser input is the current Chrome Performance trace (`.json`/`.json.gz`) or direct V8 `.cpuprofile`, then align the design title, collection guide, phase order, and menu copy with that decision. | None | Completed 2026-07-20: **Chrome Performance trace is the first input**, with `.cpuprofile` supported in the same Phase 1 because the V8 normalization core is shared. Narrowing the Phase 1 trace adapter to a `ph:"P"` filter removes most of the cost objection — duration-event modelling stays in Phase 4 under G4. Scoping `.cpuprofile` first was rejected as building a different feature, not a cheaper one: it fails both stated motivations (frontend bottleneck evidence, zero collection cost). Title kept, menu copy set to "브라우저 성능 분석", collection guide leads with Save trace, matrix entries registered as `chrome-trace-json`/`v8-cpuprofile`, Phase 1/4 items revised |
| T-559 | P0 | DONE | Define one value/time-unit contract for V8 samples, including delta attribution, `IntervalMS`, idle accounting, summary fields, Diff normalization, and CLI/Wails defaults; lock it with duration parity tests. | T-558 | Completed 2026-07-20 (§4.3.1): `Sample.Value` is **microseconds `int64`** — nanoseconds rejected as precision the source lacks, float ms rejected because accumulation error makes the tolerance undefinable; ms conversion happens once at display. `samples[]` is authoritative and `hitCount` becomes a cross-check (`INV-C3`), with `hitCount`-only inputs served by an aggregate path that reports no duration rather than estimating one. The `IntervalMS` back-calculation is replaced by a `Parsed.ValueUnit` branch, leaving the 22 existing formats unchanged. Cost attribution assigns `timeDeltas[i]` to sample `i-1`, and recording/active/idle durations are recorded separately. Invariants `INV-C1`~`INV-C9` cover `total = self + Σ children`, the two cross-checks that catch dropped subtrees, no double-counting under recursion, and CLI/Wails parity |
| T-560 | P0 | DONE | Define and implement one desktop integration path for profile evidence so Browser CPU analysis, the existing Profiler policy, Analysis Workspace, Diff, and Report Export all consume the same normalized parser output. | T-558, T-559 | Completed 2026-07-20: Browser CPU uses `AnalyzeProfileEvidence`, adds its result to Analysis Workspace/Export flow, and Diff loads V8/Chrome through `parsers/profile.Parsed` rather than adding formats to the legacy switch. |
| T-561 | P0 | DONE | Redesign Phase 3 temporal analysis: use Chrome trace task boundaries for true Long Task findings, or rename cpuprofile-only output to sampled CPU runs; define a pre-collapse ordered-series contract. | T-558, T-559 | Completed 2026-07-20: pre-collapse samples produce bounded `cpu_sample_runs` and `cpu_activity`; runs split on stack/idle/gap, emit only `SAMPLED_CPU_HOTSPOT` at 100ms+, and UI explicitly states that they are not browser Long Tasks. |
| T-556 | P0 | DONE | Add Jennifer MSA application drilldown so a user can choose a middle-tier application/TXID or average-transaction application URL and recalculate response-time composition from that application and its downstream calls instead of the whole GUID root. | Existing Jennifer MSA `guid_groups`, `profiles`, `msa_edges`, `slow_sql_events` contracts | Completed 2026-05-28: single TXID selector plus average-mode unique application URL selector, scoped response breakdown, scoped topology/timeline/treemap, scoped transaction and slow SQL tables |
| T-572 | P1 | DONE | Build the Phase 1 HAR normalization and security fixture corpus for Chrome, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic, malformed, oversized, and secret-bearing inputs in the shared asset repository, including Linux/macOS-generated fixtures consumed by the Windows UI, with provenance and expected diagnostics. | T-568, T-570 | Completed 2026-07-20: synthetic-first corpus at `../projects-assets/test-data/har-fixtures/` — 10 dialect fixtures (Chrome H1/H2/sensitive, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic), 8 malformed (BOM, schema violation, absent bodies, base64-no-encoding, sizes -1, degenerate timestamps, truncated, NetLog rejection), 2 adversarial (secrets-everywhere redaction golden, deep nesting) plus generators for oversized/gzip/extreme-nesting inputs that are never committed. Per-directory manifests fix provenance, basis claims, and expected dialect/diagnostic/redaction assertions; fixtures are deterministic script outputs. Honest limitation recorded: all synthetic, so CreatorMatch strings stay provisional until the real-export collection plan in the corpus README is executed (Chrome/Firefox first; Fiddler alongside the T-574 Windows spike) |
| T-573 | P1 | DONE | Define a deterministic bounded aggregator and recoverable Wails snapshot/event protocol, then prove live completion-order and offline start-order parity with property and long-session tests. | T-568, T-569 | Completed 2026-07-20: HAR import records the shared aggregate snapshot; sequence/snapshot recovery plus permutation and 50k-entry long-session live/offline parity tests are fixed. |
| T-574 | P1 | DONE | Run a Windows-first build-versus-integrate spike for the MITM proxy, comparing vetted libraries, a subprocess adapter, an H1-limited MVP, and a native H1/H2 implementation on licensing, Windows packaging/signing, updates, PID hooks, fidelity, and crash isolation. | T-568, T-569, T-570 | Completed 2026-07-20: library survey (goproxy BSD-3 maintained but H2-limited; martian archived; gomitmproxy GPL-3.0/no-H2/stale; mitmproxy MIT but Python-bundle burden), comparison matrix across the 4 options, and a measured risk register (R-574-1..7) written into `docs/ko/SYSTEM_HTTP_CAPTURE.md` build-vs-integrate 결정. **Decision: option 3 (own H1 semantic MVP + H2 passthrough)** behind a Proxy/Interceptor interface, with option 4 (native H2) as a later graft and option 2 (mitmproxy subprocess) as a documented fallback. Proven by `spikes/t574-mitm-proxy-poc` — pure-stdlib PoC (CA + on-the-fly leaf + CONNECT + ALPN-peek passthrough + H1 terminate/forward/capture) whose offline self-test passes and which cross-compiles for windows/amd64; PID attribution reuses the T-571-validated `GetExtendedTcpTable` path. Gate 8 closed |
| T-575 | P1 | DONE | Define HTTP-specific report and session Diff projections plus the independent current-code navigation integration; do not claim generic Diff or a not-yet-implemented browser menu provides these paths automatically. | T-568, T-569 | Completed 2026-07-20 (§12.4.1): versioned deterministic URL templating (`{id}`/`{uuid}`/`{hash}`/`{token}`/`{email}`, query keys only, top-K template cap with `{other}` bucket); comparison dimensions endpoint/host/process with process disabled for HAR pseudo-process sessions and cross-dimension sum checks; explicit numerator/denominator on every rate, per-minute normalization skipped for degenerate-timestamp sessions, percentiles computed per session and never averaged; time-alignment grades `aligned`/`duration_only`/`none` with overlay views suppressed at `none`; bounded `http_capture_diff` AnalysisResult (top-K 50 tables, HTTP_DIFF_* findings) whose drilldown uses per-session store cursors and whose export never rescans stores (T-460 principle); Wails entry via HttpCapturePage compare action and Workspace comparison routing — no new NavKey, no browser-menu assumption, legacy DiffPage untouched. Implementation is Phase 4 item 31 |
| T-562 | P1 | DONE | Unify the JSON and hard-coded profiler classifiers behind a source-format-aware interface, then define where FlameNode category/color enrichment occurs without reclassifying Node V8 frames as browser frames. | T-559 | Completed 2026-07-20: analyzer-stage flamegraph enrichment assigns browser application/runtime/dependency versus Node runtime categories and colors from source format. |
| T-563 | P1 | DONE | Harden V8 parsing with graph validation, optional-field policy, timestamp reconciliation, canonical per-frame script identity, redaction, and explicit parser/analyzer/Wails options. | T-558, T-559 | Completed 2026-07-20: node/parent graph validation, timestamped microsecond samples, canonical frame identity with URL redaction, and max-bytes/max-samples options flow through parser, analyzer, and Wails. |
| T-564 | P1 | DONE | Replace whole-file/double-unmarshal assumptions with a size/gzip/streaming strategy for direct profiles and Chrome traces; define weighted downsampling and partial-result semantics. | T-558, T-563 | Completed 2026-07-21: V8/Chrome path selects a capped gzip reader before whole-file loading; Chrome trace walks `traceEvents` token-by-token and only decodes `ph:"P"` chunks (with a count-only first pass when bounded output is needed). Default 500k deterministic time-weighted buckets preserve total microsecond duration, emit `partial_result`/source-and-output counts plus `PROFILE_DOWNSAMPLED`, and Browser CPU visibly labels aggregated output. Gzip-stream, weighted-duration, and decompressed-limit regressions pass. |
| T-557 | P1 | DONE | Promote Jennifer MSA drilldown calculation into a shared Go analyzer contract for desktop, CLI and reports. | T-556; user activation 2026-11-10 | Completed 2026-11-10: versioned existing-result projection for GUID/TXID and signature/application, cycle-safe downstream scope, representative-per-GUID average and explicit denominators; scoped canonical tables and synthetic average edges, Wails UI/Workspace/JSON/HTML and CLI wiring. Metadata extensions survive JSON/Wails round trips. Go/vet/race, legacy calculation parity, CLI/service/export integration, frontend state/build, isolated browser interaction and Mac/Windows cross-build checks pass. Native desktop interaction was not exercised for this change. |
| T-576 | P2 | DONE | Pair the revised Korean HTTP capture note with English documentation, an importer/dialect support matrix, a data-model reference, and a security guide once the Phase 1 contracts are approved; keep the source ledger appendix filled as claims are re-verified. | T-566 through T-571 | Completed 2026-07-21: `docs/en/SYSTEM_HTTP_CAPTURE.md` added as the condensed English pair with dedicated sections for the data-model reference (bounded `http_capture` envelope, `CaptureSessionRef`, versioned `CaptureSessionStore`), the importer/dialect support matrix (7-step pipeline, verified dialect quirks, `HAR_*` diagnostic codes, NetLog rejection, T-572 corpus pointer), and the security guide (import-time redaction rationale with the Okta incident, value-preserving techniques, CA lifecycle, privilege contract, SEC-1~15); also covers the transaction/time model, capability tiers, T-575 Diff contract, phase status, and the T-571 measurement state. The Korean note stays normative and Appendix A remains the auditable claim ledger — English §8 must be refreshed when the T-571 real-NIC rerun lands |
| T-565 | P2 | DONE | Add sanitized Chrome/Node/CDP malformed and end-to-end fixtures, then pair the Korean design with English documentation and update importer matrix, data model, user guide, and performance docs after the P0 contracts stabilize. | T-558 through T-564 | Completed 2026-07-21: synthetic browser-profile corpus at `../projects-assets/test-data/browser-profile-fixtures/` (6 valid — V8/Node/CDP/trace object/array/gzip; 7 malformed — broken children reference, hitCount-only, negative delta, unknown node, no-profile-chunk trace, truncated, empty; 2 e2e reproducing the Phase 3 acceptance scenario of a 210ms renderList run at 3.1s in both cpuprofile and trace form, doubling as the AC-1 parity input and reserving the trace `RunTask` as the Phase 4 `BROWSER_LONG_TASK` golden) with manifest-fixed expected formats/diagnostics/findings; `docs/en/CHROME_DEVTOOLS_CPUPROFILE.md` added as the condensed English pair (Korean note stays normative); importer matrix, data model, user guide, and performance docs updated in both languages with the implemented formats, 256 MiB/500k-sample guards, downsample suppression, and `http_capture` MVP envelope; CLI `profile import --format` help lists `v8-cpuprofile`/`chrome-trace-json`; Go golden tests now consume all 15 manifest entries and enforce Chrome trace/cpuprofile duration parity, diagnostics, findings, sampled-run location/duration, and hitCount-only temporal suppression. |
| T-414 | P1 | DONE | Connect `trace_import` to the Wails UI with summary cards, service dependency view, trace table, span table, and findings panel. | Trace import MVP | Completed 2026-05-13: Trace Import desktop workflow |
| T-415 | P1 | DONE | Add Elastic APM `_search` response and source-only NDJSON importers. | Trace import MVP | Completed 2026-05-13: Elastic trace evidence import |
| T-416 | P1 | DONE | Add trace critical-path analysis and current MVP findings: `SLOW_TRACE_P95`, `CLOCK_SKEW_SUSPECTED`, `UNBALANCED_SERVICE_LATENCY`, and `HIGH_ERROR_SERVICE_EDGE`. | Trace import MVP | Completed 2026-05-13: Root-cause oriented trace diagnostics |
| T-417 | P1 | DONE | Design and build the Evidence Board skeleton around reusable evidence cards. | Analyzer result contracts | Completed 2026-05-13: Cross-analyzer evidence pack foundation |
| T-418 | P1 | DONE | Run direct Windows GUI launch smoke-test for the 0.3.1 line on a Windows host/VM. | 0.3.1 release assets | Completed 2026-05-13: Windows runner GUI process launch smoke |
| T-419 | P2 | DONE | Continue 0.3.x release hardening for signing/notarization and frontend bundle splitting. | 0.3.1 release baseline | Completed 2026-05-13: signing/notarization preflight plus frontend bundle split |
| T-420 | P1 | DONE | Clarify the JFR analyzer contract: use the JDK `jfr` CLI only for `.jfr` to `jfr print --json` conversion, and avoid reimplementing the full JDK `jfr view` / `jfr summary` feature set in the desktop UI. | Existing JFR parser bridge | Completed 2026-05-13: JFR scope note, metadata contract, and UI copy |
| T-421 | P1 | DONE | Add async-profiler-oriented JFR stack analysis by aggregating `stackTrace.frames` from sample events into top methods, top packages, top threads, and call-tree/flamegraph-ready rows. | JFR JSON parser, profiler flamegraph components | Completed 2026-05-13: async-profiler stack evidence tables and flamegraph-ready tree |
| T-422 | P2 | DONE | Add JFR recording UX hints that detect sparse or mode-specific recordings, especially async-profiler JFR files with mostly `jdk.ExecutionSample`, `jdk.NativeMethodSample`, `jdk.ObjectAllocationSample`, or lock events. | T-421 preferred | Completed 2026-05-13: empty-filter, missing-stack, async-profiler-style, and sparse-stack hints |
| T-423 | P2 | DONE | Expand Evidence Board capture beyond Trace Import and add report export around saved evidence cards. | T-417 | Completed 2026-05-13: non-trace evidence capture plus HTML/JSON evidence export |
| T-424 | P1 | DONE | Restore a Wails Export Center page and menu entry using the existing `ExportJSON`, `ExportHTML`, `ExportPPTX`, `ExportCSV`, and `ExportCSVDir` engine bindings for full `AnalysisResult` exports. | Engine exporter bindings | Completed 2026-05-13: desktop Export Center for JSON/HTML/PPTX/CSV artifacts |
| T-425 | P1 | DONE | Add an Analysis Workspace / Recent Results surface that stores successful analyzer results across page navigation and lets users send them to Evidence Board, Export Center, or comparison workflows. | T-424 preferred, current analyzer pages | Completed 2026-05-13: reusable session result registry and workspace UI |
| T-426 | P2 | DONE | Add a general AnalysisResult comparison workflow around the existing `DiffReports` engine binding so non-profiler JSON results can be compared before/after or release-to-release. | T-425 preferred | Completed 2026-05-13: normalized AnalysisResult JSON comparison page |
| T-427 | P2 | DONE | Reintroduce a focused Chart Studio MVP for Wails with chart template selection, analyzer-result preview, and PNG/SVG/CSV-style chart export before deeper custom chart editing. | T-425 preferred, chart/export foundation | Completed 2026-05-13: report-ready chart preview/export workflow |
| T-428 | P1 | DONE | Decide and implement the Jaeger trace import contract for the file-first compatibility pass. | Trace import MVP | Completed 2026-05-15: Jaeger QueryService/local trace JSON importer, auto-detect, fixtures, CLI/UI format option |
| T-429 | P1 | DONE | Add SkyWalking schema/version handling before widening import support. | Trace import MVP | Completed 2026-05-15: schema-guarded SkyWalking GraphQL `queryTrace.spans` importer diagnostics and fixture coverage |
| T-430 | P2 | DONE | Update trace compatibility fixtures, tests, and English/Korean docs after Jaeger/SkyWalking work. | T-428, T-429 | Completed 2026-05-15: parser/analyzer tests, data-model docs, roadmap, Trace Import UI/CLI format list |
| T-431 | P1 | DONE | Define a common Incident Timeline event model for timestamp/range, source analyzer, severity, label, and evidence reference. | Analysis Workspace, Evidence Board | Completed 2026-05-15: reusable Wails session timeline event model |
| T-432 | P1 | DONE | Map existing analyzer results into Incident Timeline events across findings, access-log bursts/latency, GC alerts, JFR pauses/notable events, exceptions, thread dumps, and trace import. | T-431 | Completed 2026-05-15: cross-analyzer event builder from Analysis Workspace results |
| T-433 | P1 | DONE | Add a Wails Incident Timeline workspace page with event filters and Evidence Board capture. | T-431, T-432 | Completed 2026-05-15: desktop Incident Timeline MVP under Workspace |
| T-434 | P1 | DONE | Promote the Wails session Incident Timeline into an engine-level or exportable `AnalysisResult` when report-pack generation needs persisted timeline data. | T-431, T-432, T-433 | Completed 2026-05-16: exportable `incident_timeline` AnalysisResult projection |
| T-435 | P2 | DONE | Add richer Incident Timeline event ranges, correlation IDs, and timeline grouping for multi-file incidents. | T-434 | Completed 2026-05-16: event ranges, correlation IDs, group metadata, grouped UI filters, and exportable `tables.groups` |
| T-436 | P2 | DONE | Add Incident Timeline narrative support that explains what happened and in what order during an incident. | T-435, T-446 preferred | Completed 2026-05-16: deterministic evidence-linked narrative steps in UI, exportable timeline, and report packs |
| T-437 | P1 | DONE | Build a Golden Signals inventory across access logs, trace import, Jennifer MSA, exceptions, GC, JFR, thread dumps, and JVM signals. | T-425, T-433 | Completed 2026-05-16: session-wide Golden Signals inventory model |
| T-438 | P1 | DONE | Define SLI metrics for latency, traffic, errors, and saturation from the Golden Signals inventory. | T-437 | Completed 2026-05-16: normalized latency, traffic, error, and saturation SLI metric model |
| T-439 | P1 | DONE | Add SLO target configuration, violating-window detection, error-budget burn tables, and affected service/endpoint breakdowns. | T-438 | Completed 2026-05-16: session SLO targets, violations, error-budget, and affected-scope breakdowns |
| T-440 | P1 | DONE | Add a Wails SLO / Golden Signals workspace page with signal inventory, SLO violations, and Evidence Board capture. | T-437, T-438, T-439 | Completed 2026-05-16: desktop SLO / Golden Signals workspace workflow |
| T-441 | P1 | DONE | Unify Jennifer MSA topology and trace-import dependency models. | T-427, T-430 | Completed 2026-05-16: shared service-flow input model for Trace Import and Jennifer MSA |
| T-442 | P1 | DONE | Define a common service-edge schema with caller, callee, call count, average/max/total latency, error count, and network gap. | T-441 | Completed 2026-05-16: normalized service-edge contract with latency, error, and network-gap aggregation |
| T-443 | P1 | DONE | Normalize unmatched calls, missing parents, and network gaps into service-edge findings. | T-442 | Completed 2026-05-16: deterministic service-flow findings for unmatched calls, missing parents, and high network gaps |
| T-444 | P2 | DONE | Add C4 dynamic view or sequence-like export for service-flow evidence. | T-442, T-443 | Completed 2026-05-16: Wails Service Flow page with Mermaid sequence-like and JSON export |
| T-445 | P1 | DONE | Generate report-ready HTML, ZIP, and later PPTX/PDF outputs from Evidence Board content. | T-423, T-434 preferred | Completed 2026-05-16: Evidence Board report pack HTML/ZIP export with session artifacts |
| T-446 | P1 | DONE | Preserve source metadata, analyzer options, captured evidence, deterministic findings, and optional AI interpretation provenance in report packs. | T-445 | Completed 2026-05-16: report pack provenance contract for source results, evidence, findings, derived artifacts, and AI provenance |
| T-447 | P2 | DONE | Support customer-facing summaries without hiding raw evidence behind conclusions. | T-445, T-446 | Completed 2026-05-16: evidence-linked customer summary with raw evidence appendix |
| T-448 | P2 | DONE | Surface AI interpretation provenance in the UI. | AI interpretation module, T-425 | Completed 2026-05-16: Analysis Workspace AI provenance status with provider/model/prompt metadata |
| T-449 | P2 | DONE | Keep AI findings visually separate from deterministic findings. | T-448 | Completed 2026-05-16: separate AI-assisted findings panel with confidence, reasoning, limitations, and evidence refs |
| T-450 | P1 | DONE | Add AI evaluation gates using golden diagnostics, evidence-reference integrity, quote-to-source matching, low-confidence filtering, and hallucination review. | T-448, T-449 | Completed 2026-05-16: Go/UI AI gate for schema, evidence refs, quote matching, confidence, and hallucinated refs |
| T-451 | P1 | DONE | Connect AI interpretation to Evidence Board and report generation only when every generated claim has valid evidence references. | T-446, T-450 | Completed 2026-05-16: gated AI Evidence Board capture and Report Pack AI findings |
| T-452 | P0 | DONE | Fix Incident Timeline cross-analyzer mappings so thread dump, lock contention, multi-thread dump, and exception events read the table keys actually emitted by Go analyzers, or add the missing analyzer tables where that is the better contract. Add regression fixtures for each corrected mapping. | T-431, T-432 | Completed 2026-05-16: Incident Timeline now reads `thread_dump`, `thread_dump_multi`, and `thread_dump_locks` tables emitted by Go analyzers; lock-contention UI accepts the Go result type |
| T-453 | P0 | DONE | Make AI interpretation gating authoritative through the Go `internal/aiinterpretation` evaluator exposed via Wails bindings; keep TypeScript as presentation logic only, require non-empty model/summary/reasoning fields, require evidence quotes when quote matching is enabled, make minimum confidence configurable, and preserve validation issues with provenance. | T-448, T-449, T-450, T-451 | Completed 2026-05-16: exposed `EvaluateAiInterpretation` through EngineService, aligned the TS gate issue codes with Go, required model/summary/reasoning, configurable confidence, and quote-required diagnostics |
| T-454 | P0 | DONE | Harden privacy redaction before any LLM dispatch by adding allow-list or stronger deny-list coverage for stack traces, hostnames, IPv6, bearer tokens, JWTs, SQL fragments, and Korean PII patterns, with Go tests for redaction edge cases. | T-453 preferred | Completed 2026-05-16: redacts bearer/JWT secrets, host/URL targets, IPv6, filesystem paths in stack traces, SQL literals, Korean phone/RRN, and existing email/IP/token patterns before prompt construction |
| T-455 | P0 | DONE | Normalize SLO error-rate units across access-log and trace-import signals, add explicit fraction/percent conversion at the SLI boundary, and fix error-budget burn formulas so objective percent and threshold semantics are applied correctly. | T-437, T-438, T-439, T-440 | Completed 2026-05-16: percent signals now carry explicit `rate_unit`, trace-import fractions are converted to percent at signal normalization, and budget consumption uses target semantics for upper/lower percent bounds |
| T-456 | P0 | DONE | Prevent Trace Import and Jennifer MSA traffic double-counting in Golden Signals by making aggregation source-aware and deduplicating equivalent caller-to-callee traffic where both analyzers describe the same edge. | T-441, T-455 | Completed 2026-05-16: equivalent service-edge traffic signals share a metric family and use source-aware max aggregation across analyzer totals instead of summing trace-import and Jennifer MSA observations |
| T-457 | P0 | DONE | Sanitize report-pack ZIP entry paths with a shared path-normalization helper that rejects absolute, parent-directory, and platform-specific traversal forms before writing archive headers. | T-445 | Completed 2026-05-16: report-pack ZIP entries now pass through `normalizeZipEntryPath`, rejecting absolute paths, drive/UNC forms, parent/current directory segments, empty segments, NUL bytes, encoded dot segments, and duplicate archive names before writing headers |
| T-458 | P1 | DONE | Move Service Flow toward the first Go analyzer/exportable `AnalysisResult` pattern for derived Evidence Studio workflows, including parser/findings/diagnostics metadata, CLI/exporter compatibility, service-edge normalization, and fixtures. | T-441, T-442, T-443, T-444 | Completed 2026-05-16: Service Flow export now uses a `service_flow` AnalysisResult envelope with `summary/series/tables/charts/metadata`, parser/diagnostics/findings metadata, source-result extras, and compatibility top-level edge/finding fields |
| T-459 | P1 | DONE | Apply the proven derived-analysis contract pattern to Incident Timeline and SLO/Golden Signals projections so exported payloads satisfy the common `AnalysisResult` envelope and remain readable by Go exporters. | T-458, T-452, T-455 | Completed 2026-05-16: Incident Timeline metadata now includes parser/diagnostics/findings/extra fields; SLO / Golden Signals now has an exportable `slo_golden_signals` AnalysisResult envelope with tables for signals, metrics, targets, violations, budget rows, and affected breakdown |
| T-460 | P1 | DONE | Stop report-pack export from recomputing analysis at export time; report packs should consume persisted Analysis Workspace results and derived `AnalysisResult` artifacts produced earlier in the workflow. | T-458, T-459, T-445 | Completed 2026-05-16: report-pack artifact resolution now prefers existing `incident_timeline`, `slo_golden_signals`, and `service_flow` workspace AnalysisResults and filters derived artifacts out of source-result provenance, with one compatibility fallback for sessions that have not persisted derived artifacts yet |
| T-461 | P1 | DONE | Add service identity normalization for Service Flow and SLO aggregation, including case, hyphen/underscore, display-name, and alias-table matching for Trace Import and Jennifer MSA service names. | T-441, T-458 | Completed 2026-05-16: Service Flow and SLO edge/service grouping now use a shared canonical service identity that handles case, camel/display names, hyphen/underscore variants, URL hosts, ports, and an alias-table hook |
| T-462 | P1 | DONE | Strengthen AI provenance in Evidence Board and report packs with prompt hash, generation parameters, token counts when available, source/evidence integrity hashes, and validation status. | T-453, T-446 | Completed 2026-05-16: report-pack AI provenance now records prompt hash/params/token counts when present, source integrity hashes, evidence integrity hashes, and validation status alongside gate issues |
| T-463 | P1 | DONE | Preserve AI-assisted versus deterministic finding separation after Evidence Board capture and in report-pack rendering, including visible AI badges, confidence, limitations, and evidence-reference status. | T-449, T-451 | Completed 2026-05-16: report-pack rendering keeps deterministic findings separate from accepted AI findings and shows AI-assisted badges, validation status, confidence, limitations, and evidence quote status |
| T-464 | P2 | DONE | Make derived workflow exports deterministic by injecting `now` explicitly, supporting fixed ZIP timestamps, parsing Unix-second timestamps, replacing locale-dependent formatting, stabilizing dedupe keys, and optimizing narrative sorting with precomputed event maps. | T-457, T-459 | Completed 2026-05-16: Incident Timeline parses Unix-second timestamps and uses stable ISO labels, narrative sorting uses precomputed event maps, report-pack ZIP builders support fixed timestamps, and report formatting now avoids locale-default number formatting in updated derived surfaces |
| T-465 | P2 | DONE | Expand SLO and runtime signal robustness with configurable SLO target overrides, safe non-spread max scans for large series, and runtime-stack signal extraction for Node.js, Python, Go panic, and .NET analyzer result types. | T-455 | Completed 2026-05-16: added SLO target override merging, non-spread max/min scans, and Golden Signal extraction for `nodejs_stack`, `python_traceback`, `go_panic`, and `dotnet_exception_iis` runtime results |
| T-466 | P2 | DONE | Add frontend state regression tests for Incident Timeline, SLO/Golden Signals, Service Flow, Report Pack, and AI gate behavior, plus Go AI redaction/evidence-reference tests for the new hardening paths. | T-452, T-453, T-454, T-455, T-456, T-457 | Completed 2026-05-16: added `npm run test:state` with frontend state regressions for service identity, SLO percent conversion/dedupe, runtime signals, Mermaid source-only findings, Unix timestamps, and AI quote enforcement; Go AI hardening tests were added with T-453/T-454 |
| T-467 | P2 | DONE | Deduplicate shared frontend state utilities and fix Mermaid sequence export edge cases, including empty-service/source-only findings and quoted labels. | T-458, T-461 | Completed 2026-05-16: service identity canonicalization is shared by Service Flow and SLO, Mermaid source-only findings now get a fallback participant, quoted labels continue to use JSON escaping, and large-series max scans avoid spread-based limits |
| T-468 | P1 | DONE | Define the Mid-Term Plus ingestion architecture boundary: parser package layout, analyzer package layout, CLI command naming, Wails binding pattern, and `AnalysisResult` naming for new evidence families. | Roadmap Mid-Term Plus | Completed 2026-05-16: `internal/ingestion` evidence family specs define parser/analyzer packages, CLI leaves, Wails bindings, and result types for Mid-Term Plus source families |
| T-469 | P1 | DONE | Add a reusable source-format registry and auto-detect contract that can be shared by access logs, trace import, server logs, database logs, broker logs, and platform evidence. | T-468 | Completed 2026-05-16: reusable bounded-probe `FormatRegistry`, `SourceFormat`, detector, and unknown-format contract |
| T-470 | P1 | DONE | Create a golden fixture and parser diagnostic harness for new importers, including valid, partial, malformed, unknown-format, and large-file samples. | T-468 | Completed 2026-05-16: parser-agnostic fixture expectation/check harness covers valid, partial, malformed, unknown-format, and large-file diagnostic assertions |
| T-471 | P1 | DONE | Normalize source metadata fields for new evidence families, including source kind, source format, product, version, host, service, environment, and sanitized file identity. | T-468 | Completed 2026-05-16: `SourceMetadata`/`FileIdentity` contract plus additive access-log and trace-import `metadata.source_metadata` |
| T-472 | P1 | DONE | Define the cross-source correlation-key model for trace ID, span ID, request ID, tenant/customer ID, container ID, pod UID, host ID, PID, and source timestamp windows. | T-468, T-471 | Completed 2026-05-16: `CorrelationKeys` with normalized stable IDs and timestamp windows; trace import now emits bounded correlation-key samples |
| T-473 | P1 | DONE | Add Tomcat access valve and Jetty NCSA request-log parsers with format auto-detection fixtures. | T-469, T-470 | Completed 2026-05-16: Tomcat and Jetty now use the combined/common access parser path with source-format tagging and auto-detect coverage |
| T-474 | P1 | DONE | Map Tomcat and Jetty access records into the existing access-log `AnalysisResult` contract without breaking nginx/apache fields. | T-473 | Completed 2026-05-16: Tomcat/Jetty records flow through existing access-log summaries, series, sample records, and diagnostics without changing required nginx/apache fields |
| T-475 | P1 | DONE | Add HAProxy default and HTTP log parsers with backend/server timing fields and diagnostics for partial timing data. | T-469, T-470 | Completed 2026-05-16: HAProxy HTTP parser extracts backend/server, queue/connect/response timings, termination state, retry count, and classified parse diagnostics |
| T-476 | P1 | DONE | Analyze HAProxy access evidence for latency, status, backend saturation, retry, termination-state, and error-burst findings. | T-475 | Completed 2026-05-16: access-log analyzer now summarizes gateway latency/retries/termination errors and emits HAProxy/edge findings plus service-edge tables |
| T-477 | P1 | DONE | Add Envoy and Istio default text plus JSON access-log parsers, including service-mesh trace headers and upstream cluster metadata. | T-469, T-470, T-472 | Completed 2026-05-16: Envoy/Istio text and JSON parsers capture upstream cluster/service, route, response flags, TLS, trace ID, and request ID metadata |
| T-478 | P1 | DONE | Feed Envoy/Istio upstream service, trace ID, TLS, response flag, and route data into access-log summaries, Service Flow, and Golden Signals. | T-477 | Completed 2026-05-16: upstream service distributions, route stats, service dependency rows, access edge Service Flow edges, and access edge Golden Signals are emitted |
| T-479 | P2 | DONE | Add AWS ELB/ALB classic, ALB v2, and CloudFront standard log parsers with cloud source metadata. | T-469, T-470, T-471 | Completed 2026-05-16: AWS ELB, ALB, and CloudFront access rows parse with AWS cloud provider and edge/location or load-balancer metadata |
| T-480 | P2 | DONE | Add GCP HTTP(S) Load Balancer JSON and Azure App Service/Front Door JSON access-log parsers. | T-469, T-470, T-471 | Completed 2026-05-16: GCP HTTP LB JSON and Azure App Service/Front Door JSON parse request, status, latency, host/service, backend, route, and trace fields |
| T-481 | P2 | DONE | Add IIS W3C extended log and Caddy/Traefik JSON access-log parsers. | T-469, T-470 | Completed 2026-05-16: IIS W3C field headers, Caddy JSON, and Traefik JSON parse into normalized access-log records with route/upstream/backend metadata where available |
| T-482 | P2 | DONE | Add Kong, Tyk, and AWS API Gateway access-log parsers with route, consumer, upstream, and gateway-latency fields. | T-469, T-470, T-472 | Completed 2026-05-16: Kong, Tyk, and AWS API Gateway JSON access logs parse gateway latency, route, consumer, upstream service, request ID, and trace ID fields |
| T-483 | P1 | DONE | Wire all new access/edge formats into access-log auto-detect dispatch and per-source parser diagnostics. | T-473, T-475, T-477 | Completed 2026-05-16: `auto` dispatch covers the added access/edge formats and `metadata.source_format_diagnostics` reports selected format, parsed-by-format counts, and skipped reasons |
| T-484 | P1 | DONE | Extend the access-log contract and tests for upstream service, mesh trace ID, gateway latency, TLS, route, and cloud-edge metadata while keeping existing exports compatible. | T-474, T-476, T-478, T-483 | Completed 2026-05-16: optional access/edge summary, series, service dependency, route, and sample-record metadata are documented and covered by parser/analyzer/frontend state tests |
| T-485 | P1 | DONE | Define the server-log `AnalysisResult` contract and finding taxonomy for startup, deployment, datasource pool, stuck thread, hung thread, severe error, and worker error events. | T-468, T-471, T-472 | Completed 2026-05-16: `server_log` contract, tables, summaries, source metadata, correlation keys, and finding taxonomy are documented and implemented |
| T-486 | P1 | DONE | Add Tomcat catalina.out and Jetty server-log parsers/analyzers with startup, deployment, datasource, stuck-thread, and severe-error findings. | T-485, T-470 | Completed 2026-05-16: Tomcat and Jetty server-log selectors parse into normalized server events and analyzer findings |
| T-487 | P2 | DONE | Add JBoss/WildFly server-log parsing and findings for deployment failures, datasource warnings, thread pool pressure, and severe errors. | T-485, T-470 | Completed 2026-05-16: JBoss/WildFly log selectors parse deployment, datasource, thread-pool, and error evidence |
| T-488 | P2 | DONE | Add WebLogic AdminServer/ManagedServer log parsing and findings for deployment, stuck thread, JDBC pool, and managed-server health events. | T-485, T-470 | Completed 2026-05-16: WebLogic angle-bracket server logs map JDBC, deployment, thread, and managed-server health events |
| T-489 | P2 | DONE | Add WebSphere SystemOut/SystemErr parsing and findings for hung threads, application lifecycle events, datasource warnings, and severe errors. | T-485, T-470 | Completed 2026-05-16: WebSphere SystemOut/SystemErr rows parse severity, component, timestamp, hung-thread, lifecycle, datasource, and error events |
| T-490 | P2 | DONE | Add GlassFish/Payara server-log parsing and findings for deployment, JDBC pool, thread pool, and severe application-server errors. | T-485, T-470 | Completed 2026-05-16: GlassFish/Payara bracketed logs parse into server events with deployment/JDBC/thread-pool/error taxonomy |
| T-491 | P1 | DONE | Add nginx and Apache error-log parsers/analyzers so worker errors can be correlated with access-log request evidence. | T-485, T-472 | Completed 2026-05-16: nginx and Apache error logs emit worker/upstream error rows plus request/trace correlation candidates |
| T-492 | P1 | DONE | Map server-log and web-server error findings into Incident Timeline, Evidence Board, and report-pack artifacts with access/error correlation where keys are available. | T-486, T-491 | Completed 2026-05-16: server-log findings flow through generic Evidence Board/report-pack paths, server-log events feed Incident Timeline, and Golden Signals captures server error/thread saturation counts |
| T-493 | P1 | DONE | Define and implement OpenTelemetry Logs OTLP JSON and NDJSON parser contracts with severity, body, attributes, resource metadata, and trace/span references. | T-468, T-470, T-472 | Completed 2026-05-16: OTel parser now accepts JSONL/NDJSON records and OTLP Logs `resourceLogs` with attributes/resource metadata and trace/span references |
| T-494 | P1 | DONE | Add the OpenTelemetry Logs analyzer with severity bursts, error signatures, resource grouping, and trace/span correlation diagnostics. | T-493 | Completed 2026-05-16: OTel analyzer emits resource groups, error signatures, severity bursts, and existing trace/span topology diagnostics |
| T-495 | P2 | DONE | Add Prometheus snapshot and OpenMetrics import for offline metrics evidence and Golden Signals enrichment. | T-468, T-470 | Completed 2026-05-16: `metrics_snapshot` parser/analyzer imports Prometheus/OpenMetrics text and exposes Golden Signal candidate rows |
| T-496 | P2 | DONE | Add Loki query JSON export and Tempo trace JSON export importers so LGTM stack exports become first-class local evidence sources. | T-493, Trace import MVP | Completed 2026-05-16: `observability_evidence` imports Loki query JSON and Tempo trace JSON as local records with trace references |
| T-497 | P2 | DONE | Add Grafana dashboard JSON ingestion that lets Evidence Board cards reference dashboard panels without treating dashboards as raw metrics evidence. | T-468, T-471 | Completed 2026-05-16: Grafana dashboard JSON panels import as reference records and findings rather than metric samples |
| T-498 | P1 | DONE | Map OTel logs, metrics snapshots, Loki, Tempo, and Grafana references into Incident Timeline, SLO/Golden Signals, Evidence Board, and report packs. | T-494, T-495, T-496, T-497 | Completed 2026-05-16: observability findings flow through Evidence Board/report packs, observability records feed Incident Timeline, and metrics/observability summaries feed Golden Signals |
| T-499 | P1 | DONE | Define the slow-query and database engine-log `AnalysisResult` contract with query fingerprint, latency percentiles, lock wait, error, row count, database, schema, and sanitized SQL fields. | T-468, T-471, T-472 | Completed 2026-05-16: `database_slow_query` contract emits sanitized fingerprints, latency percentiles, lock waits, errors, row counts, database/schema, source metadata, and dependency rows |
| T-500 | P1 | DONE | Add PostgreSQL csvlog and text log parsers/analyzers for slow queries, errors, lock waits, checkpoints, autovacuum signals, and top query fingerprints. | T-499, T-470 | Completed 2026-05-16: PostgreSQL text and csvlog duration/error/lock evidence parse into query and fingerprint tables |
| T-501 | P1 | DONE | Add MySQL/MariaDB slow query log parsing and analysis for query fingerprints, p95/p99 latency, lock time, rows examined, and top query findings. | T-499, T-470 | Completed 2026-05-16: MySQL/MariaDB slow-query blocks parse Query_time, Lock_time, Rows_examined, query text, and fingerprints |
| T-502 | P2 | DONE | Add MongoDB profiler and diagnostic.data export parsing for slow operations, collection/index hints, lock/queue indicators, and error counts. | T-499, T-470 | Completed 2026-05-16: MongoDB profiler-style JSON imports operation, namespace, command/query text, millis, and row counts |
| T-503 | P2 | DONE | Add Redis slowlog get output parsing and analysis for command fingerprints, latency, key-pattern sanitization, and top slow commands. | T-499, T-470 | Completed 2026-05-16: Redis slowlog text imports command fingerprints and microsecond latency normalized to milliseconds |
| T-504 | P2 | DONE | Add SQL Server extended events JSON parsing for slow query, wait, deadlock, and error evidence. | T-499, T-470 | Completed 2026-05-16: SQL Server extended-event JSON imports statement/sql_text, duration, waits, database name, and error fields |
| T-505 | P1 | DONE | Add PostgreSQL and MySQL EXPLAIN plan importers and Evidence Board plan cards linked to slow-query fingerprints. | T-499, T-500, T-501 | Completed 2026-05-16: PostgreSQL/MySQL EXPLAIN JSON rows are flagged as plan evidence and linked through query fingerprints and plan summaries |
| T-506 | P1 | DONE | Integrate database slow-query evidence into Incident Timeline, SLO/Golden Signals, Service Flow enrichment, and report packs. | T-500, T-501, T-505 | Completed 2026-05-16: database findings flow through Evidence Board/report packs, query rows feed Incident Timeline, latency/error/lock summaries feed Golden Signals, and dependency rows feed Service Flow |
| T-507 | P1 | DONE | Define the broker/streaming middleware `AnalysisResult` contract and finding taxonomy for rebalance, replication, queue pressure, dead letter, partition, and broker health events. | T-468, T-471, T-472 | Completed 2026-05-16: `broker_log` contract normalizes broker events, dependency rows, source metadata, and broker finding taxonomy |
| T-508 | P1 | DONE | Add Kafka broker, controller, and state-change log parsing with ISR change, rebalance, under-replicated partition, log compaction, and KRaft quorum findings. | T-507, T-470 | Completed 2026-05-16: Kafka-style log lines classify ISR/replication, rebalance, partition, compaction, and KRaft quorum events |
| T-509 | P1 | DONE | Add RabbitMQ server-log parsing plus `rabbitmq-diagnostics` JSON import for connection churn, queue length, dead-letter, partition, and node health findings. | T-507, T-470 | Completed 2026-05-16: RabbitMQ text and diagnostics JSON import queue pressure, connection churn, dead-letter, and broker-health evidence |
| T-510 | P2 | DONE | Add Pulsar broker-log parsing for topic, ledger, bookie, broker health, and backlog-related findings. | T-507, T-470 | Completed 2026-05-16: Pulsar-style log lines classify topic/ledger/bookie/backlog and health events under the broker taxonomy |
| T-511 | P2 | DONE | Add NATS server-log parsing for connection churn, slow consumers, cluster events, JetStream pressure, and authorization failures. | T-507, T-470 | Completed 2026-05-16: NATS/JetStream log lines classify connection churn, slow consumers, pressure, cluster/partition, and auth failures |
| T-512 | P2 | DONE | Add ActiveMQ broker-log parsing for queue pressure, consumer churn, dead letter, store usage, and broker health events. | T-507, T-470 | Completed 2026-05-16: ActiveMQ-style log lines classify queue pressure, dead letters, consumer churn, store usage, and health events |
| T-513 | P1 | DONE | Surface broker findings in Incident Timeline and Service Flow beside trace-import dependencies and service-edge evidence. | T-508, T-509, T-472 | Completed 2026-05-16: broker findings flow to report/Evidence Board paths, broker events feed Incident Timeline, broker summaries feed Golden Signals, and dependency rows feed Service Flow |
| T-514 | P2 | DONE | Define the Kubernetes/container evidence contract and identity model for cluster, namespace, workload, pod, container, node, image, restart count, and owner references. | T-468, T-471, T-472 | Completed 2026-05-16: `kubernetes_evidence` preserves platform identity fields, restart counts, image/node/pod/container context, and source metadata |
| T-515 | P2 | DONE | Add `kubectl get events` and `describe pod` JSON importers with OOMKilled, restart, eviction, scheduling, image pull, and readiness findings. | T-514, T-470 | Completed 2026-05-16: Kubernetes event/pod JSON import OOMKilled, restart, eviction, scheduling, image pull, and readiness/probe evidence |
| T-516 | P2 | DONE | Add kubelet log parsing for pod lifecycle, eviction, probe, image, runtime, and node-pressure events. | T-514, T-470 | Completed 2026-05-16: kubelet text logs classify lifecycle, eviction, probe/readiness, image, runtime, and node-pressure events |
| T-517 | P2 | DONE | Add container runtime log parsing for containerd, CRI-O, and Docker daemon events that accompany application evidence. | T-514, T-470 | Completed 2026-05-16: container-runtime text logs classify containerd/CRI-O/Docker events with pod/container/node identity hints |
| T-518 | P2 | DONE | Add cloud audit/log importers for AWS CloudTrail JSON, GCP Cloud Audit Logging JSON, and Azure Activity Logs JSON with security-incident evidence fields. | T-468, T-470, T-471 | Completed 2026-05-16: AWS CloudTrail, GCP audit, and Azure Activity JSON map actor, operation, resource, provider, and security category fields |
| T-519 | P2 | DONE | Map Kubernetes, container, and cloud audit evidence into Incident Timeline, Evidence Board, SLO saturation signals, and security-oriented report sections. | T-515, T-516, T-517, T-518 | Completed 2026-05-16: platform findings flow through Evidence Board/report packs, platform rows feed Incident Timeline, and restart/OOM/node-pressure/security counts feed Golden Signals |
| T-520 | P1 | DONE | Define a unified profile evidence schema with language-tagged frames, native-versus-managed split, async frame markers, source runtime, and flamegraph rollup metadata. | T-468, existing profiler analyzers | Completed 2026-05-16: `profile_evidence` normalizes language/runtime-tagged frames, native/managed/async markers, and flamegraph rollup metadata |
| T-521 | P1 | DONE | Add a generic pprof binary `.pb.gz` importer shared by Go pprof, Datadog Ruby/PHP profiler exports, py-spy pprof-compatible output, and other pprof runtimes. | T-520, T-470 | Completed 2026-05-16: pprof `.pb.gz` imports use `github.com/google/pprof/profile` and map samples into the unified profile schema |
| T-522 | P2 | DONE | Add py-spy raw output and rbspy raw output importers with Python/Ruby frame tagging and thread/process metadata. | T-520, T-470 | Completed 2026-05-16: py-spy and rbspy raw text stacks import with runtime/language labels and thread block metadata |
| T-523 | P1 | DONE | Add async-profiler `.html` and collapsed input parity for the unified profile path while preserving existing collapsed/JFR behavior. | T-520, existing profiler analyzers | Completed 2026-05-16: unified profile import reuses existing async-profiler collapsed and HTML parser paths |
| T-524 | P2 | DONE | Add dotnet-trace `.nettrace` and speedscope output importers with .NET frame tagging and managed/native separation. | T-520, T-470 | Completed 2026-05-16: speedscope JSON, including dotnet-trace speedscope exports, maps into unified profile frames with .NET/runtime tagging |
| T-525 | P2 | DONE | Add perf script collapsed import for Rust/native profiling evidence with symbol, thread, and process metadata. | T-520, T-470 | Completed 2026-05-16: perf collapsed stacks import through the profile path with native/Rust-oriented frame classification |
| T-526 | P2 | DONE | Add Ruby stackprof plus PHP Excimer, Tideways CLI export, and Xdebug profile importers where stable file contracts are available. | T-520, T-470 | Completed 2026-05-16: StackProf JSON, generic PHP profiler JSON, and Xdebug cachegrind text map into profile samples |
| T-527 | P3 | DONE | Add Swift backtrace and generic async stack trace parsing after the primary server-side runtime profiler inputs stabilize. | T-520, T-470 | Completed 2026-05-16: Swift/generic async text stacks import with async frame markers |
| T-528 | P1 | DONE | Promote collapsed and JFR profiler outputs into the unified multi-language profile analyzer with cross-language flamegraph rollups and Evidence Board capture. | T-521, T-523, T-520 | Completed 2026-05-16: unified analyzer rolls imported samples into existing flamegraph/drilldown paths and Golden Signal capture |
| T-529 | P1 | DONE | Implement the first evidence-stitching pass that joins access logs, traces, runtime stacks, broker logs, and database slow logs by correlation key. | T-472, T-484, Trace import MVP, T-506, T-513 | Completed 2026-05-16: `stitched_evidence` reads AnalysisResult JSON files and joins table rows by trace/span/request/tenant/container/host/PID keys |
| T-530 | P1 | DONE | Surface correlation-gap findings for missing trace ID, dropped parent span, unmatched request log, unmatched database call, and unmatched broker event. | T-529 | Completed 2026-05-16: stitching emits `MISSING_TRACE_ID`, `DROPPED_PARENT_SPAN`, `UNMATCHED_REQUEST_LOG`, `UNMATCHED_DATABASE_CALL`, and `UNMATCHED_BROKER_EVENT` findings |
| T-531 | P2 | DONE | Add stitched evidence views in Incident Timeline, Evidence Board, Service Flow, and report packs with raw source evidence preserved. | T-529, T-530 | Completed 2026-05-16: stitched gaps/matches feed Incident Timeline, findings remain Evidence Board/report-pack compatible, and stitched dependencies feed Service Flow |
| T-532 | P2 | DONE | Add Grafana Pyroscope/Phlare snapshot export importer and map it into the unified profile evidence schema. | T-520, T-470 | Completed 2026-05-16: profile importer accepts Pyroscope/Phlare-style JSON snapshots and maps names/stacks into profile samples |
| T-533 | P2 | DONE | Add Polar Signals Parca snapshot export importer and map it into the unified profile evidence schema. | T-520, T-470 | Completed 2026-05-16: profile importer accepts Parca-style profile JSON snapshots through the generic continuous-profile path |
| T-534 | P2 | DONE | Route continuous-profiling snapshots through the existing flamegraph analyzer, Evidence Board capture, and report-pack export paths. | T-532, T-533, T-528 | Completed 2026-05-16: continuous profile snapshots roll up through `profile_evidence` flamegraph charts, findings, Golden Signals, and generic report-pack capture |
| T-535 | P3 | DONE | Track the OpenTelemetry Profiles signal lifecycle and add a decision note for when OTLP Profiles should move from long-term radar to active ingestion work. | T-520 | Completed 2026-05-16: added English/Korean OTLP Profiles decision notes with activation criteria |
| T-536 | P2 | DONE | Keep English/Korean roadmap, data-model notes, examples, and importer support matrices current as Mid-Term Plus tasks are implemented. | T-468 through T-535 | Completed 2026-05-16: updated data model/parser docs, examples, and English/Korean importer support matrices |
| T-537 | P1 | DONE | Add OpenAPI import for endpoint/method/operation metadata and expose it through the common parser/analyzer boundary. | T-536, T-470 | Completed 2026-05-17: OpenAPI JSON and conservative YAML subset import into `api_contract_analysis` operations |
| T-538 | P1 | DONE | Compare OpenAPI operations with access-log evidence to identify observed, undocumented, and unused API surfaces. | T-537, access-log evidence | Completed 2026-05-17: access-log result tables are normalized to route templates and compared with OpenAPI operations |
| T-539 | P1 | DONE | Detect slow and high-error APIs by combining API contract matches with access-log latency/error evidence. | T-538 | Completed 2026-05-17: slow/high-error operation tables and findings use configurable latency/error thresholds |
| T-540 | P2 | DONE | Add AsyncAPI import for channel, direction, operation, and message metadata. | T-536, broker evidence | Completed 2026-05-17: AsyncAPI JSON and conservative YAML subset import channel and message metadata |
| T-541 | P2 | DONE | Compare AsyncAPI channels with broker/event evidence to find undocumented and unused event channels. | T-540, T-513 | Completed 2026-05-17: broker result tables are matched to AsyncAPI channels with undocumented/unused channel findings |
| T-542 | P2 | DONE | Surface API/event contract findings in Incident Timeline, SLO/Golden Signals, Evidence Board, examples, and support docs. | T-537 through T-541 | Completed 2026-05-17: API contract summaries feed Golden Signals, findings/tables feed generic Evidence Board/report paths, and key findings feed Incident Timeline |
| T-543 | P1 | DONE | Define the architecture-documentation `AnalysisResult` contract for arc42 Context, Runtime View, Deployment View, Quality Requirements, Risks, and ADR draft sections. | T-536 | Completed 2026-05-17: `architecture_docs` contract emits arc42 sections, context/runtime/deployment rows, quality requirements, risks, and ADR drafts |
| T-544 | P1 | DONE | Generate arc42 Context and System Scope draft inputs from service-flow, API contract, and stitched evidence. | T-543, T-542, T-531 | Completed 2026-05-17: context services and interface rows are inferred from service dependencies, API contracts, and stitched evidence |
| T-545 | P1 | DONE | Generate Runtime View draft inputs from Incident Timeline, trace, broker, database, and stitched evidence. | T-543, T-531 | Completed 2026-05-17: runtime view rows capture service dependencies, matches, critical paths, and correlation gaps |
| T-546 | P2 | DONE | Generate Deployment View draft inputs from Kubernetes/platform, cloud audit, host/container, and service metadata evidence. | T-543, T-519 | Completed 2026-05-17: deployment rows preserve namespace, workload/pod, container, node, image, and platform risk hints |
| T-547 | P1 | DONE | Generate Quality Requirement and Risk sections from deterministic findings across latency, errors, saturation, security, and correlation gaps. | T-543, T-542, T-531 | Completed 2026-05-17: quality requirements and risks are derived from deterministic findings with severity, impact, mitigation, and evidence refs |
| T-548 | P2 | DONE | Add ADR draft rows with decision, context, alternatives, tradeoffs, consequences, and evidence references. | T-543, architecture evidence | Completed 2026-05-17: ADR draft rows are generated from highest-priority risk and quality evidence |
| T-549 | P1 | DONE | Enhance evidence stitching with timestamp-window matching so nearby records can correlate even when only request/service identity is shared. | T-529 | Completed 2026-05-17: `stitch analyze` adds configurable timestamp-window matching with `time_window_service_alias` match rows |
| T-550 | P1 | DONE | Add service alias normalization for gateway, database, broker, Kubernetes workload, and application naming variants in stitched evidence. | T-549, T-531 | Completed 2026-05-17: service aliases normalize service/svc/deployment suffixes, database/broker prefixes, and Kubernetes pod replica suffixes |
| T-551 | P2 | DONE | Add trace-profile linkage by joining profile evidence with trace/service/runtime labels when correlation metadata is present. | T-549, T-528 | Completed 2026-05-17: `profile_evidence.profile_samples.labels.trace_id` participates in trace/profile stitching |
| T-552 | P2 | DONE | Add stitched drilldown tables for match confidence, time-window reason, alias reason, and raw evidence references. | T-549, T-550, T-551 | Completed 2026-05-17: `match_drilldowns` preserves confidence, reason, alias, source node IDs, evidence refs, and bounded raw source rows |
| T-553 | P2 | DONE | Surface advanced stitched-evidence signals in Incident Timeline, Service Flow, and Golden Signals. | T-552 | Completed 2026-05-17: time-window and trace-profile matches feed Incident Timeline labels, Service Flow match status, and Golden Signal summaries |
| T-554 | P2 | DONE | Add a stitched-evidence detail view or drilldown-ready state projection for the desktop UI. | T-552 | Completed 2026-05-17: desktop state can drill from match events into `match_drilldowns` and raw source node rows |
| T-555 | P2 | DONE | Update English/Korean docs, examples, and support matrix for architecture docs and advanced stitching work. | T-543 through T-554 | Completed 2026-05-17: updated data model, parser design, roadmap, importer matrix, and stitching examples |

## Verification Notes

- 2026-11-09 fix-cycle integration passed `go test ./...`, `go vet ./...`,
  `go build ./...`, frontend `npm run test:state`, and production `npm run build`.
  Native macOS linking retains the existing SDK-versus-deployment-target
  warning; all commands exited successfully. Evidence covers local tests and
  builds, not actual desktop interaction, fresh Windows E2E or an independent
  review verdict. The final source review also added a passing partial-uptime
  regression so GC rates use one common clock throughout each result; the
  complete validation set passed again after that change. Task-specific closure
  details are in T-589 through T-593.

- 2026-11-09 GC/profiler revalidation: uncached
  `go test -count=1 ./internal/parsers/gclog ./internal/analyzers/gclog ./internal/profiler`
  passed under `apps/engine-native`. Temporary public-API probes reproduced
  unified GC time/rate loss, display-pruned versus full-tree filter divergence,
  an explicitly nanosecond-weighted lock conversion error, and ordered-regex
  diagnostic loss. The actual Canvas hierarchy expression, evaluated with the
  installed `d3-hierarchy`, reproduced internal self-sample loss with a leaf-only
  control. The peak probe also verified exact summary and retained critical
  alert evidence, so peak-preserving downsampling was not classified as a
  confirmed correctness bug. Existing tests passing is baseline evidence only;
  it does not close T-589 through T-593. No product source, persistent test,
  desktop lifecycle test, shared asset, release, commit, or push changed here.
- 2026-07-21 Chrome/V8 release finishing passed the shared manifest golden test
  across 15 valid, malformed, and E2E fixtures, including 3.1s/210ms
  `renderList` parity between `.cpuprofile` and Chrome trace inputs.
- 2026-07-21 release smoke passed `go test ./...`, `go vet ./...`, and
  `go build ./cmd/archscope-engine ./cmd/archscope-app`; the known macOS
  26.0-to-11.0 linker warning remains non-blocking. Frontend
  `npm run test:state` and `npm run build` passed after resolving the direct
  dependency advisories with ECharts 6.1.0 and Vite 8.1.5; `npm audit` reports
  zero vulnerabilities. The shared ECharts chunk is 698.80 KB raw / 235.56 KB
  gzip, within the 700 KB raw budget.
- 2026-07-21 local Wails packaging produced the ad-hoc-signed
  `bin/archscope.app` and `bin/archscope-arm64.dmg`; strict deep codesign
  verification and `hdiutil verify` both passed.
- `go test ./...` passed under `apps/engine-native`. The Wails app test build
  still emits the known macOS linker warning about object files built for newer
  macOS 26.0 than the 11.0 link target.
- `go test -bench BenchmarkBuildLargeSyntheticGCLog -benchtime=1x -benchmem
  ./internal/analyzers/gclog` passed with a 300k-event JSON payload of about
  1.9 MB.
- Synthetic large-file measurements were captured with the current
  `cmd/archscope-engine` binary built from `apps/engine-native`.
- 2026-05-10 release verification ran with local `go1.26.3`. `go vet ./...`,
  `go build ./cmd/archscope-engine ./cmd/archscope-app`, and
  `go test ./... -race -count=1` passed
  under `apps/engine-native` using `/tmp` Go caches. The race test required
  loopback permission for `httptest`.
- `npm ci` and `npm run build` passed for the Wails frontend during the
  2026-05-10 release pass. That pass exposed the former main-chunk warning
  that was later addressed by T-419.
- `task package` and `task darwin:package:dmg ARCH=arm64` produced
  `bin/archscope.app` and `bin/archscope-arm64.dmg`. The app launched without
  immediate crash, ad-hoc `codesign --verify --deep --strict` passed,
  `CFBundleShortVersionString`/`CFBundleVersion` are `0.3.0-rc1`, and
  `hdiutil verify` reported a valid DMG checksum.
- CLI smoke tests produced access-log and profiler `AnalysisResult` JSON plus
  HTML/CSV report outputs in `/tmp`.
- GitHub Actions run `25602445878` completed the `v0.3.0-rc1` Release workflow
  successfully across darwin-arm64, windows-amd64, and linux-amd64 packaging
  plus GitHub Release creation. Latest main CI runs `25603594053` and
  `25603594056` were also successful, including Windows `go vet`, `go build`,
  and `go test`.
- Downloaded release assets matched `SHA256SUMS`; Windows zip contains an
  x86-64 `archscope.exe`, while the NSIS installer wrapper is a 32-bit PE
  self-extractor as expected.
- 2026-05-10 UI/performance pass: `npm run build` passed for the Wails
  frontend and reported the former >500 KB main chunk warning that was later
  addressed by T-419.
- `go test ./internal/parsers/accesslog ./internal/profiler` and
  `go test ./...` passed under `apps/engine-native` using `/tmp` Go caches.
- Parser/profiler benchmarks after the hot-path changes:
  `BenchmarkParseLineNginxWithResponseTime` 165.4 ns/op, 144 B/op, 1 alloc/op;
  `BenchmarkAnalyzeCollapsedSampleWall` 86.7 ms/op, 53.6 KB/op, 549 allocs/op;
  `BenchmarkAnalyzeJenniferSample` 41.5 us/op, 41.3 KB/op, 422 allocs/op.
- 2026-05-10 naming cleanup: `go test ./...`, `go build
  ./cmd/archscope-engine ./cmd/archscope-app`, `npm run build`, `task
  package`, `task darwin:package:dmg ARCH=arm64`, `codesign --verify --deep
  --strict bin/archscope.app`, and `hdiutil verify bin/archscope-arm64.dmg`
  passed from the renamed `cmd/archscope-app` tree.
- 2026-05-10 `v0.3.1-rc1` prerelease was created from the Jennifer MSA
  network-time grouping work. GitHub Release workflow completed successfully
  across darwin-arm64, windows-amd64, linux-amd64, and release creation.
- 2026-05-11 `v0.3.1` is listed as the latest GitHub Release. It promotes the
  0.3.1 line to stable and includes the trace import MVP plus the Jennifer MSA
  network-time improvements carried forward from `v0.3.1-rc1`.
- 2026-05-11 local trace import verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test
  ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine`.
- 2026-05-11 documentation consolidation check passed: `git diff --check`
  succeeded, and `docs/en` and `docs/ko` Markdown file lists match 1:1 after
  removing the former single-language planning notes.
- 2026-05-13 GC alert verification passed in
  `apps/engine-native/internal/analyzers/gclog`: young/old/metaspace series,
  long-pause flags, and `OUT_OF_MEMORY_ERROR` findings are covered by tests.
- 2026-05-13 trace import verification passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine ./api ./cmd/archscope-app`. The Wails app test build
  still emits the known macOS linker warning.
- 2026-05-13 full Go verification also passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./...`.
- 2026-05-13 Wails frontend release-hardening build passed: `npm run build`.
  Startup shell chunk is 156.77 KB raw / 50.90 KB gzip; the lazy shared
  ECharts runtime is 668.49 KB raw / 221.26 KB gzip and stays under the
  documented 700 KB chart-runtime budget. The previous Vite large-chunk warning
  is no longer emitted.
- 2026-05-13 release workflow hardening updated macOS signing secret preflight
  and codesign/stapler validation. The workflow change was not exercised by a
  local release-tag run. Local YAML parsing and `git diff --check` passed.
- 2026-05-13 CLI smoke passed for Elastic APM auto-detect:
  `go run ./cmd/archscope-engine trace import --in
  ../../examples/traces/sample-elastic-apm-search.json --format auto --top-n
  10` emitted `source_format = elastic-apm-search-json` and the expected
  trace findings.
- 2026-05-13 Windows GUI smoke run `25800619784` passed on Microsoft Windows
  Server 2025. It built `apps/engine-native/cmd/archscope-app/bin/archscope.exe`
  with `task windows:build ARCH=amd64`, confirmed a 14,060,544-byte executable,
  launched it, observed the GUI process alive for 15 seconds, and requested
  graceful shutdown.
- 2026-05-13 JFR async-profiler verification passed:
  `env GOCACHE=/tmp/archscope-go-cache GOMODCACHE=/tmp/archscope-go-mod-cache
  go test ./internal/analyzers/jfr ./internal/parsers/jfr ./cmd/archscope-app`.
  The Wails app test build still emits the known macOS linker warning.
- 2026-05-13 Wails frontend verification passed after JFR/Evidence Board
  expansion: `npm run build`. Startup shell chunk is 156.78 KB raw / 50.91 KB
  gzip; the lazy shared ECharts runtime remains 668.49 KB raw / 221.26 KB gzip.
- 2026-05-13 `git diff --check` passed after the T-420 through T-423 changes.
- 2026-05-13 Wails frontend verification passed after T-424 through T-427:
  `npm run build`. Startup shell chunk is 159.76 KB raw / 51.58 KB gzip; the
  lazy shared ECharts runtime is 689.89 KB raw / 229.49 KB gzip and remains
  under the documented 700 KB chart-runtime budget.
- 2026-05-15 trace compatibility verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test
  ./internal/parsers/traceimport ./internal/analyzers/traceimport
  ./cmd/archscope-engine`.
- 2026-05-15 full Go verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...`. The Wails app test
  build still emits the known macOS linker warning.
- 2026-05-15 Wails frontend verification passed after adding Jaeger and
  SkyWalking format options: `npm run build`. Startup shell chunk is
  159.75 KB raw / 51.58 KB gzip; the lazy shared ECharts runtime is
  689.89 KB raw / 229.49 KB gzip and remains under the documented 700 KB
  chart-runtime budget.
- 2026-05-15 CLI smoke passed for Jaeger and SkyWalking auto-detect:
  `go run ./cmd/archscope-engine trace import --format auto` emitted
  `source_format = jaeger-query-json` with three spans and
  `source_format = skywalking-graphql-json` with two spans.
- 2026-05-15 Wails frontend verification passed after T-431 through T-433:
  `npm run build`. Startup shell chunk is 160.48 KB raw / 51.79 KB gzip; the
  lazy shared ECharts runtime is 689.89 KB raw / 229.50 KB gzip and remains
  under the documented 700 KB chart-runtime budget.
- 2026-05-16 Wails frontend verification passed after T-437 through T-440:
  `npm run build`. Startup shell chunk is 161.04 KB raw / 51.95 KB gzip; the
  SLO / Golden Signals page is a lazy 41.99 KB raw / 9.17 KB gzip route chunk,
  and the lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
  `git diff --check` passed.
- 2026-05-16 Wails frontend verification passed after T-441 through T-444:
  `npm run build`. Startup shell chunk is 161.40 KB raw / 52.02 KB gzip; the
  Service Flow page is a lazy 22.30 KB raw / 5.84 KB gzip route chunk, and the
  lazy shared ECharts runtime remains 689.89 KB raw / 229.49 KB gzip.
- 2026-05-16 Wails frontend verification passed after T-447:
  `npm run build`. Startup shell chunk is 161.45 KB raw / 52.07 KB gzip; the
  Evidence Board page is a lazy 17.92 KB raw / 5.63 KB gzip route chunk, and
  the lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
- 2026-05-16 Wails frontend verification passed after T-448:
  `npm run build`. Startup shell chunk is 161.45 KB raw / 52.07 KB gzip; the
  Analysis Workspace page is a lazy 6.14 KB raw / 2.21 KB gzip route chunk,
  and the lazy shared ECharts runtime remains 689.89 KB raw / 229.49 KB gzip.
- 2026-05-16 Wails frontend verification passed after T-449:
  `npm run build`. Startup shell chunk is 161.45 KB raw / 52.06 KB gzip; the
  Analysis Workspace page is a lazy 10.15 KB raw / 3.15 KB gzip route chunk,
  and the lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
- 2026-05-16 AI interpretation gate verification passed after T-450:
  `env GOCACHE=/tmp/archscope-go-cache go test ./internal/aiinterpretation`
  passed under `apps/engine-native`; loopback permission was required for the
  existing Ollama `httptest` client case. Wails frontend `npm run build` also
  passed. Startup shell chunk is 161.45 KB raw / 52.05 KB gzip; the Analysis
  Workspace page is a lazy 13.42 KB raw / 4.20 KB gzip route chunk, and the
  lazy shared ECharts runtime remains 689.89 KB raw / 229.50 KB gzip.
- 2026-05-16 AI report integration verification passed after T-451:
  `env GOCACHE=/tmp/archscope-go-cache go test ./internal/aiinterpretation`
  passed under `apps/engine-native`, and Wails frontend `npm run build` passed.
  Startup shell chunk is 161.50 KB raw / 52.08 KB gzip; the Analysis Workspace
  page is a lazy 9.22 KB raw / 2.78 KB gzip route chunk, Evidence Board is a
  lazy 19.97 KB raw / 6.11 KB gzip route chunk, and the shared
  `aiInterpretation` helper is a lazy 5.15 KB raw / 1.88 KB gzip chunk.
- 2026-05-16 Wails frontend verification passed after T-435:
  `npm run build`. Startup shell chunk is 161.50 KB raw / 52.08 KB gzip; the
  Incident Timeline page is a lazy 9.97 KB raw / 2.62 KB gzip route chunk, the
  shared `incidentTimeline` helper is 11.38 KB raw / 3.79 KB gzip, and
  `git diff --check` passed.
- 2026-05-16 Wails frontend verification passed after T-436:
  `npm run build`. Startup shell chunk is 161.50 KB raw / 52.08 KB gzip; the
  Incident Timeline page is a lazy 11.18 KB raw / 2.80 KB gzip route chunk,
  the shared `incidentTimeline` helper is 12.70 KB raw / 4.20 KB gzip, and
  `git diff --check` passed.
- 2026-05-16 `v0.3.3` release-prep verification passed: Wails build assets
  were regenerated from `build/config.yml`, Windows `syso` generation passed
  for the 0.3.3 metadata, `env GOCACHE=/tmp/archscope-go-cache go test ./...`
  passed under `apps/engine-native`, `npm run build` passed for the Wails
  frontend, and `git diff --check` passed. The Wails app test build still
  emits the known macOS linker warning. Startup shell chunk is 161.50 KB raw /
  52.08 KB gzip, and the lazy shared ECharts runtime is 689.89 KB raw /
  229.49 KB gzip.
- 2026-05-16 `v0.3.4` release-prep verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...` passed under
  `apps/engine-native` with the known macOS linker warning,
  `npm run test:state` passed for derived frontend state regressions,
  `npm run build` passed for the Wails frontend, and `git diff --check`
  passed. Frontend package/build metadata is set to 0.3.4.
- 2026-05-17 `v0.3.5` release-prep verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...` passed under
  `apps/engine-native` with the known macOS linker warning,
  `npm run test:state` passed for derived frontend state regressions,
  `npm run build` passed for the Wails frontend, and `git diff --check`
  passed. Frontend package/build metadata is set to 0.3.5.
- 2026-05-17 `v0.3.5` GitHub release workflow completed successfully after the
  `Prepare v0.3.5 release` commit and `v0.3.5` tag were pushed. The release is
  the latest stable release; GitHub workflow annotations were limited to
  non-blocking Node.js 20 deprecation notices.
- 2026-05-16 T-468 through T-472 verification passed:
  `env GOCACHE=/tmp/archscope-go-cache go test ./...` passed under
  `apps/engine-native` with the known macOS linker warning, and
  `git diff --check` passed.
- 2026-05-17 documentation current-state verification passed after source/doc
  alignment: `git diff --check`, `bash -n scripts/run-demo-site-data.sh`,
  `env GOCACHE=/tmp/archscope-go-cache go test ./cmd/archscope-engine`, CLI
  help checks for report/demo-site/thread-dump/api-contract/stitch/
  architecture-docs/database-log/broker-log, and sample executions for API
  contract, stitching, thread-dump, trace import, database-log, broker-log, and
  architecture-docs workflows.
- 2026-05-28 Wails frontend verification passed after Jennifer MSA application
  drilldown: `npm run build`.

## Decisions

- Go/Wails is the primary active runtime and distribution path.
- Wails service bindings are the active engine/UI boundary.
- Local HTTP/FastAPI/browser serving is retired and retained only in `archive/`.
- Python sources are retained for behavior reference and audits only.
- Large-file safety work should happen in Go first; archived Python code should
  not receive new product features.
- Roadmap candidates are not automatically promoted into `Active TO-DO`; they
  should be added only after explicit review and request.

## Archive

- Old work status: `archive/work_status_legacy_2026-05-09.md`
- Archived Python engine: `archive/python-engine`
- Archived browser frontend: `archive/web-frontend-python`
