# ArchScope Work Status

Last updated: 2026-11-13

This file is the current execution status for the active ArchScope product line.
The previous long-form history was archived to
`archive/work_status_legacy_2026-05-09.md`.

## Status Overview

**Overall: 239 of 249 tracked tasks are `DONE` (96.0%).** The user activated the local diagnostics backlog T-628–T-645 on 2026-11-11. T-628 is DONE after independent R3 confirmed R2 N-01–N-04 resolved and explicitly permitted closure with only nonblocking P3 findings; R3-01–R3-03 are now locally remediated and verified. The independent verdict remains CONDITIONAL, not PASS, and does not cover the final P3 fixes. T-629 is DONE under independent R2 closure permission: R4-01–R4-05 are independently resolved, R5-01 is locally fixed, and R5-02 is explicitly scoped to T-634/T-635; the original verdict remains CONDITIONAL. T-630 is DONE under independent R2 closure permission after R6-01–R6-07 were verified resolved; nonblocking R7-01 is locally remediated. The original independent verdict remains CONDITIONAL. Actual new case UI/Windows lifecycle evidence remains separate. T-631 is DONE under the independent review's explicit permission to close after R8-01 remediation: R8-01–R8-04 are locally fixed and verified, including 8k/16k semantics, 100k-row cache tests, exact exports, typed refusals and native Mac display/target eligibility. The original verdict remains CONDITIONAL; final fixes have not been independently re-reviewed. T-632 is DONE under its independent review's explicit after-R9-01-remediation closure permission: R9-01–R9-04 are locally fixed and verified. The original verdict remains CONDITIONAL; the final fixes have no independent re-review and native Windows offline/WebView observation remains separate. T-633 is DONE under its independent review's explicit after-R10-01-remediation closure permission. R10-01–R10-05 are locally fixed and verified, including a 10.5 MB native JFR expanding to about 530 MB JSON and successful legacy binary analysis. The original verdict remains CONDITIONAL; final independent re-review and native UI/Windows execution remain separate. T-634 is DONE under independent R11 review permission after R11-01 remediation. R11-01–R11-04 are locally fixed and verified: source semantics 0.4.0 / normalizer 1.1.0 binds original sample type/unit independently of policy, preserves old 0.3.0 bytes/hashes with reimport comparison gate, exposes policy basis and generic none weights. Original CONDITIONAL is preserved. T-635 is DONE under independent R12 explicit closure permission after queueing its four nonblocking P3 findings. R12-01–R12-04 are now locally fixed and verified, preserving CONDITIONAL. T-638 is DONE under R13 explicit after-P2-remediation closure permission: selected-series compact payload 0.2.0, minimal projections, compact CLI and 100k-point saved-result readback are locally verified; 0.1.0 readers remain. R13-02–R13-07 are now locally fixed/verified, including source diagnostics, interval CSV and paged selection and the independent verdict remains CONDITIONAL. T-636/T-637/T-640/T-642/T-644/T-645 keep their original DONE closure permissions and independent CONDITIONAL verdicts. The 2026-11-13 review batch locally remediates R14-02–07, R15-03–05, R17-05, R19-02–04, R20-02/03, R21-02/04 and R22-03/01/02. R20-04 is EVIDENCE_PARTIAL: actual multi-load is covered, but authorized natural nonzero-swap/elevated-pressure evidence is still missing; raw pressure code1 is not an elevated-pressure claim. No final independent PASS is asserted. Native acceptance remains separate. Q01 vendor JSON and Q04/GenAI aliases remain gated. After comprehensive-review intake, 10 tasks remain open: the four existing acceptance items plus 6 open follow-ups in T-646–T-662 (11 DONE / 0 TODO / 0 IN_PROGRESS / 6 PENDING). RS-01 registration is complete; RS-02–RS-10 remain tracked, with the immediate RS-09 documentation contradiction corrected. The independent comprehensive verdict remains CONDITIONAL. T-623 Windows live/process/reentry and T-608 native HALOG codec/reference interoperability completed on 2026-11-11. The four existing acceptance items are T-601, T-611, T-613 and T-627. The private 675-entry sample round-trips byte-for-byte; decoded fields/bodies and redacted exports are verified against original V7.6.4.508. The Windows Wails v3 replacement for IEInspector HTTP Analyzer V7.6.4 Stand-alone has 3 open work items (T-594 through T-613), including the remaining audit findings. T-603 native Windows acceptance completed on 2026-11-09 with actual request/response bodies and verified policy restoration. The 2026-11-09
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
| `REVIEW` | 3 | Review or required remediation is underway; the gate has not passed |
| `PENDING` | 7 | Waiting for a prerequisite, external condition, or activation trigger |
| `DONE` | 239 | Completion criteria and required review gates have passed |

The normal transition is `TODO → IN_PROGRESS → REVIEW → DONE`. A `PENDING`
task returns to `TODO` when its dependency or trigger is satisfied. Task status
is separate from review-group verdicts (`PASS`, `CONDITIONAL`, or `FAIL`). When
a Task Registry status changes, update the counts and Open Work Snapshot in the
same change so this overview remains the status source of truth.

### Desktop layout current acceptance (2026-11-11)

Mac implementation and detailed native functional verification are complete.
T-620/T-621/T-622/T-623/T-624/T-625/T-626 are DONE. T-623's Windows
live/process/reentry gate is recorded in the 2026-11-11 evidence below.
T-627 retains the complete Windows native menu/OS scaling matrix. Original native
21-route baseline and current 21-route navigation were observed. Deep profiler
71.2%, GC 60-event stress input 72.5%, selected HTTP inspector 70.7% and
Chart Studio 72.3% visible-data union are recorded with exact observed sizes;
sparse results and Settings/Export forms have explicit exclusions. GC baseline
and final stress row counts use different inputs and are not a controlled
row-count comparison.

The production Wails 3.1.0 app uses the macOS top menu bar and has no Layout QA
menu. The sidebar screenshot was the d98a958 original reference app. No browser
or Electron UI was used. Native production import confirms global request
numbers and selected headers with the default 184px inspector.

Fixed long-path flex, drawer focus, early task-completion event loss, HAR
sequence/unknown bytes, native chart output and Settings draft validation.
All nine analyzer and eight workspace functional matrices, conditional/empty/
error recovery cases, chart templates/renderers and export formats were checked.
Full Go/vet, frontend state/build, tagged tests, macOS package/signature, desktop-
only checks and Windows x86/x64 GUI cross-builds pass; no new Windows runtime
acceptance is claimed.

Evidence: [final validation](docs/review/evidence/2026-11-10-layout/mac-detail-validation.json),
[final geometry](docs/review/evidence/2026-11-10-layout/mac-detail-geometry.json),
[original baseline](docs/review/evidence/2026-11-10-layout/mac-original-baseline.json).
Earlier reports remain historical; the final follow-up supersedes their
incomplete-Mac notes. Paired DESKTOP_CONTENT_LAYOUT documents give case limits.

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

### Local diagnostics improvement design (2026-11-11)

[Korean design](docs/ko/LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md) ·
[English companion](docs/en/LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md).
Scope remains local incident/performance analysis for application architects and
TA/DBA, broad profile inputs including py-spy, GPU/AI evidence, and report-ready
PNG/SVG/table copying. PPT authoring/PPTX enhancement and heap-dump object-graph
engines remain excluded.

The 2026-11-11 Claude review is processed as a design review. Its original verdict
is **CONDITIONAL** and is not upgraded to independent PASS by this remediation.
All twelve findings are accepted at the design level, with the qualifications
below. Code inspection confirmed the registry/dispatch gap, integer legacy stack
core, fixed profiler metadata, JDK JFR conversion dependency, WebView-rendered chart
saving, and release fixture availability gap. No runtime implementation or support
claim is made by these document edits.

Source review: [processed original](docs/review/done/2026-11-11_claude-code_local-diagnostics-evolution-design-review.ko.md).
Reviewed design SHA-256: `72bb45064bfe96fd933dc1deece372b9bd770d1a5e1a234592ca79c5e24793d9`.
Original review SHA-256: `15e65b520d3e9fd4791020646824499622ef2f5deb99b18dc9ec721166e16e07`.
The original was moved unchanged after its dispositions and follow-ups were recorded
here; the archived SHA-256 matches. No unprocessed review documents remain.

#### Finding dispositions

Section references below use the Korean design; corresponding English contracts
and paired DATA_MODEL, PARSER_DESIGN, PACKAGING_PLAN, REPORT_EXPORT_DESIGN were aligned.

| Finding | Decision and concrete remediation | Follow-up design reference |
|---|---|---|
| F-01 / P1 | Accept with scope qualification: §16.6 defines descriptor, catalog, parse sink and conformance. Package + fixture + registration is guaranteed only for existing semantic families/renderers, not every possible new visualization. | D01, D04 |
| F-02 / P1 | Accept: §16.7 chooses ProfileSet → profile_evidence as the new canonical path, staged golden-backed migration, legacy facades, exact-scale projection or explicit refusal; no third envelope. | D02, D07 |
| F-03 / P1 | Accept: §16.8 defines key-addressed measurement descriptors, legacy unknown-key roundtrips, explicit nullable fields and semantic-change versions/reanalysis. Correct DATA_MODEL's global-version wording. | D01, D02 |
| F-04 / P1 | Accept: §17.3–17.4 specifies data-only signed packs, staged activation/rollback, offline docs/fonts/runtime, conversion location, SQLite tradeoff and mandatory release fixture bundle. | D05 |
| F-05 / P2 | Accept: §16.10 adds ResultDescriptor providers, generic service wrappers and payload-validated capability consumers; generic fallback precedes new domain types. PARSER_DESIGN gets the extension checklist. | D04 |
| F-06 / P2 | Accept: §16.9 names eleven version axes, owners, declarations and reject/read-only/recompute rules; snapshots stay immutable, indexes rebuild. | D01, D03, D09 |
| F-07 / P2 | Accept: §3.3 defines capture sidecar, lifecycle/deletion, safe archive batch, CP949/EUC-KR and immutable baseline context; §18.5 adds tests. | D03, D08 |
| F-08 / P2 | Accept: §17.3 evaluates declarative mapping → external conversion → compiled importer; bounded data packs cannot execute arbitrary code. | D01, D05 |
| F-09 / P2 | Accept: §19 adds missing foundation, identity/clock/privacy, case, input assistance and TA work; B0/B1/B2 design bands distinguish review severity. Dependencies are explicit below. | D01–D18 |
| F-10 / P2 | Accept: §14.6 selects desktop WebView rendering initially, explicit CLI image-render limitation, existing exporter relationship, bundled artifacts and old-template snapshot/new-revision rules. | D09 |
| F-11 / P3 | Accept the useful English companion and indexes. Qualification: language-specific-file guidance does not by itself require a complete translation of every document. The new English companion covers operative scope/contracts; expanded catalogs remain in Korean. | Applied in documentation |
| F-12 / P3 | Accept: §16.2 stores GPU telemetry in metrics_timeseries with semantic projections; LLM app traces extend trace_import. A dedicated screen alone does not justify another canonical type. | D10–D12 |

#### Optional feature dispositions

These are scope choices, not defects or promises of already-supported inputs.
Review-recorded user needs (vLLM/NVIDIA, MacBook GPU, Python/Rust/Node and OTel AI
metrics) are accepted. Actual file/version choices remain separate decision holds.

| Proposal | Disposition and boundary | Design reference |
|---|---|---|
| G-01 Workload exports | Accept bounded comparison evidence; validate producer files, warmup/client scope; do not build a load generator. | §9.4 / D14 |
| G-02 OS CPU + thread dump | Accept; TID/nid conversion requires host/namespace/lifetime/window evidence, not number equality alone. | §7.4 / D13 |
| G-03 Offline collection kit | Accept manual recipes/sidecar generation, separate from data-only packs; no auto-attach or remote execution. | §17.4 / D06 |
| G-04 AWR/ASH/Tibero TPR | Defer concrete adapters until demand, permitted sanitized files and versions are available. | §9.2–9.4 / D17 |
| G-05 Case-wide search | Accept bounded indexed search with source positions, coverage and sanitization. | §13.4 / D08 |
| G-06 hs_err_pid | Accept bounded text crash evidence after core inputs; no core/heap engine or automatic root-cause claim. | §9.4 / D14 |
| G-07 Native JFR decoder | Partially accept: document existing JDK conversion and failure guidance now; defer pure-Go binary decoder. | §17.4 / D06 |
| G-08 DB stats snapshots | Retain demand/fixture-dependent candidate using common counter/identity rules. | §9.4 / D17 |
| G-09 Windows server exports | Retain candidate for PerfMon/event files; existing IIS support is not new adapter evidence. | §9.4 / D17 |
| G-10 Network counters | Retain bounded ss/netstat export candidate; no pcap engine. | §9.4 / D17 |
| G-11 Other inference engines | Superseded by G-15; no separate implementation item. | §12.7 |
| G-12 APM exports | Defer platform adapters until real export shape/version fixtures; reuse trace/import mechanisms. | §9.4 / D17 |
| G-13 Personal knowledge | Defer opt-in sanitized pattern library with provenance/retention; never auto-promote incident contents into shared rules. | §20.2 / later candidate |
| G-14 LLM application traces | Accept common execution semantic layer in core scope; reuse trace_import/OTLP, exclude bodies by default; product adapters wait for Q01. | §12.8 / D12 |
| G-15 Engine-neutral serving | Accept; vLLM first, capability/statistical-form contracts; other engines require actual files. | §12.7 / D11 |
| G-16 Cold start/lifecycle | Accept observed lifecycle events and explicit warm/cold/recovery populations. | §12.9 / D12 |
| G-17 Gateway | Accept routing/retry/fallback/token evidence through access/trace mapping; logical request and attempt remain distinct. | §12.9 / D12 |
| G-18 Python and other runtimes | Accept Python/Node depth, async/worker evidence and allocation summaries; no heap object graph. | §7.4 / D13 |
| G-19 NVIDIA process/memory/allocation | Accept candidates with producer/driver/process mapping fixtures; do not assume identical metric availability. | §11.6 / D11 |
| G-20 Apple Silicon | Accept product scope and vendor-neutral model now; actual adapter waits for Q02. No NVIDIA identity/VRAM assumptions or automatic privileged collection. | §11.6 / D03, D15 |
| G-21 Rust | Accept demangle, producer-based classification, panic/backtrace; preserve raw identity and reject blind hash stripping; async tasks need evidence. | §7.4 / D13 |
| G-22 OTel GenAI metrics | Accept first-class metric path with temporality/start time/resource/scope and histogram fidelity. Refine review wording: delta is not differenced again but gaps/restarts still need validation; exponential is not Prometheus native format. Aliases wait for Q04 fixtures. | §10.3 / D10 |

#### Dependency-ordered implementation backlog

At the user's request on 2026-11-11, all eighteen D items are registered as
T-628–T-645. T-628 is `DONE` under R3's explicit closure permission; P3 follow-ups are locally verified and the independent verdict remains CONDITIONAL. T-629 is `DONE` under R2's explicit closure permission; T-630 is DONE under R2 explicit closure permission; T-631 is DONE under the explicit R8-01 remediation closure condition, with all R8 findings locally verified; T-632 is DONE under the explicit R9-01 remediation condition; R9-01–R9-04 are locally fixed and verified, without a new independent PASS. T-633 is DONE under the explicit R10-01 remediation condition; all R10 findings are locally fixed and verified, without a new independent PASS. Native acceptance remains separate. T-634 is DONE under independent R11 after-remediation closure permission; T-635 is DONE under R12 explicit closure permission (four P3 findings locally fixed); neutral T-638 is DONE under R13 explicit P2-remediation closure permission (P3 follow-ups queued; CONDITIONAL preserved). T-636 is DONE under R14 explicit closure permission after R14-01 local remediation; six P3 findings remain queued and the original verdict is CONDITIONAL. Native Office/Windows acceptance remains separate.
T-637 is DONE under R15 explicit P2-remediation closure permission, with R15-03–R15-05 queued and R15-06 locally fixed; original CONDITIONAL/native boundaries are preserved. T-639 is DONE under R16 explicit P2-remediation closure permission; all six findings are locally fixed/verified and original CONDITIONAL is preserved. T-640 is DONE under R17 explicit closure permission after R17-01 remediation; R17-02–04 are also locally fixed and R17-05 remains queued. T-641 is DONE under R18 explicit P2-remediation closure permission; all four R18 findings are locally fixed. T-642 is DONE under R19 closure permission after R19-01; P3 follow-ups remain queued and CONDITIONAL is preserved. T-643 is DONE under R20 closure permission after selection-coverage remediation; R20 P3 follow-ups remain queued. T-644 is DONE under R21 after-P2-remediation permission, with R21-03 fixed and R21-02/04 queued. T-645 is DONE under R22 explicit no-blocker closure permission with its three P3 findings queued; all other D17 candidates retain their individual demand/input/version gates. Q04 still gates actual OTel/GenAI aliases. Q01–Q04 remain separate
`DECISION_PENDING` entries. Task registration does not claim implementation or
independent design PASS. Existing acceptance task statuses are preserved.

B0/B1 items use execution priority P1 and B2 items P2. These are task priorities,
not the source review's severity. Follow dependency order within each priority;
D identifiers remain stable design references. Acceptance gates are in design §18
and observable outputs are in the Task Registry.

| Order | Design | Task | Band | Priority | Status | Next work | Dependencies / scoped trigger |
|---:|---|---|---|---|---|---|---|
| 1 | D01 | T-628 | B0 | P1 | `DONE` | Implement importer, envelope, measurement and version contracts | Existing golden/compatibility inventory (part of this task) |
| 2 | D02 | T-629 | B0 | P1 | `DONE` | Migrate profiles to ProfileSet with legacy facades | T-628 |
| 3 | D03 | T-630 | B0 | P1 | `DONE` | Implement case lifecycle, capture context and safe evidence intake | T-628 |
| 4 | D04 | T-631 | B0 | P1 | `DONE` | Add result capability providers and generic consumers | T-628 |
| 5 | D05 | T-632 | B0 | P1 | `DONE` | Deliver declarative packs and verifiable offline distribution | T-628 |
| 6 | D06 | T-633 | B1 | P1 | `DONE` | Guide input discovery, manual collection and local conversion | T-630, T-632 |
| 7 | D07 | T-634 | B1 | P1 | `DONE` | Correct py-spy, Speedscope and pprof measurement semantics | T-629, T-631 |
| 8 | D08 | T-635 | B1 | P1 | `DONE` | Unify drilldown, evidence indexing, case search and baselines | T-629, T-630, T-631 |
| 9 | D09 | T-636 | B1 | P1 | `DONE` | Copy reproducible report images and tables from the desktop | T-632, T-634, T-635 |
| 10 | D18 | T-637 | B1 | P1 | `DONE` | Add perf events and constrained BPF evidence adapters | T-629, T-630, T-635 |
| 11 | D10 | T-638 | B1 | P1 | `DONE` | Implement metric history and OTLP histogram semantics | T-628, T-630, T-631; Q04 gates actual OTel export/GenAI aliases |
| 12 | D11 | T-639 | B1 | P1 | `DONE` | Analyze NVIDIA and vLLM through neutral device/serving contracts | T-632, T-638; Q03 gates version-specific vLLM support |
| 13 | D12 | T-640 | B1 | P1 | `DONE` | Normalize LLM application traces, gateway attempts and lifecycle | T-630, T-631, T-638; Q01 gates Langfuse/LangSmith adapters |
| 14 | D13 | T-641 | B1 | P1 | `DONE` | Deepen Python, Node, Rust and existing runtime/log diagnostics | T-629, T-630, T-635 |
| 15 | D14 | T-642 | B2 | P2 | `DONE` | Import workload results and text crash context for comparisons | T-630, T-635, T-638 |
| 16 | D15 | T-643 | B2 | P2 | `DONE` | Implement the selected Apple Silicon evidence adapter | T-630, T-638; Q02 and actual Apple input fixtures |
| 17 | D16 | T-644 | B2 | P2 | `DONE` | Deliver the bounded first batch of PyTorch/Nsight execution detail | T-635, T-638, T-639 |
| 18 | D17 | T-645 | B2 | P2 | `DONE` | Qualify the bounded first DB/OS snapshot adapter batch | T-628, T-629, T-630, T-631, T-632; Demand, permitted sanitized files and selected producer/version |

#### Comprehensive-review follow-up backlog (2026-11-13)

The independent [comprehensive review](docs/review/done/2026-11-13_claude-code_T-628-T-645_comprehensive-review.ko.md)
returned CONDITIONAL (P2 3 / P3 7; no P1) at `f69daa2`. It explicitly permits
existing DONE closures to stand, provided remaining scope and policy work are
registered. RS-01 is satisfied by T-654–T-662 and the bounded-first-batch T-644 /
T-645 titles. Registration does not implement those adapters or qualify new
producer versions. Existing AnalysisResult and saved-reader contracts remain.

| Order | Finding / scope | Task | Priority | Status | Dependency / activation and acceptance |
|---:|---|---|---|---|---|
| 1 | RS-03 + RS-10 | T-646 | P2 | `DONE` | Existing T-632 baseline; enforce import/layer rules with explicit legacy exceptions, Linux offline syscall CI with positive control, and localconvert denial before execution. |
| 2 | RS-02 | T-647 | P2 | `DONE` | Existing version/qualification contracts; specify and implement explicit unverified-version policy across nine pinned paths; propagate actual/unknown versions and qualification through results, exports and comparison; retain strict defaults and old readers. No automatic version-range expansion. |
| 3 | RS-04 | T-648 | P3 | `DONE` | Existing model contracts; single caller-owned JSON byte limit, domain-correct errors and shared validation helpers; reproduce 42 MiB k6 rejection, test exact boundaries and preserve strict parsing/old identities. |
| 4 | RS-05 | T-649 | P3 | `DONE` | T-631 capability baseline; backend-owned panel/action capability and registry-to-renderer contract coverage for every supported type/payload, including execution_evidence 0.2.0 and unknown-version refusal. |
| 5 | RS-07 | T-650 | P3 | `DONE` | Current model/parser/exporter contracts; six model boundary suites, bounded text/CSV/JSON parser fuzz and SVG XML/escaping tests; retain adversarial inputs and request independent R13–R17 follow-up revalidation separately. |
| 6 | RS-09 | T-651 | P3 | `DONE` | Immediate EN/KO execution-guide contradiction fixed at intake; finish current-behavior cleanup of user guides and bundled mirrors, move task history to tracker/reviews, and add the missing EN MSA_SERVLET_DISPATCH counterpart. |
| 7 | RS-08 | T-652 | P3 | `DONE` | T-648, T-649; extract bounded acquisition/lease and digest-bound cursor/page components without domain mixing; demonstrate one family migration preserving contracts and require existing-result reuse assessment before a new family. |
| 8 | RS-06 | T-653 | P3 | `DONE` | Existing legacy golden results; inventory interpolation/floor/nearest-rank/reservoir policies and annotate percentile_method with versioned compatibility; independently measure cross-family examples and avoid rewriting saved numbers. |
| 9 | RS-01 / D16 remainder | T-654 | P2 | `PENDING` | T-644; activate only with permitted sanitized actual CUDA/NCCL distributed-inference inputs, selected PyTorch/Nsight/tool versions and available hardware; distinguish actual execution from structural fixtures and rank clocks. |
| 10 | RS-01 / D17 profile | T-655 | P2 | `DONE` | T-645; select an unmet profile format after importer-matrix review, actual sanitized input, demand and producer/version; reuse ProfileSet/providers and pass old-reader/common-consumer acceptance. |
| 11 | RS-01 / D17 serving, Q03 | T-656 | P2 | `PENDING` | T-639, T-645; choose serving adapter/version/config and permitted startup/runtime/metric captures, including Q03 for vLLM; verify neutral contracts and disclose qualification. |
| 12 | RS-01 / D17 DB | T-657 | P2 | `DONE` | T-645; choose additional DB/vendor (e.g. AWR/ASH/Tibero only if requested), version and permitted sanitized export; declare counter/reset semantics and exact common projections. |
| 13 | RS-01 / D17 OS | T-658 | P2 | `DONE` | T-637, T-645; select OS/tool version and permitted actual evidence beyond the delivered ss summary; declare scope/clock/units and reuse existing results where possible. |
| 14 | RS-01 / D17 APM | T-659 | P2 | `PENDING` | T-645; select additional APM vendor/export/version and permitted sanitized input after existing-importer review; preserve common AnalysisResult and explicit alignment/coverage limits. |
| 15 | RS-01 / Q01 | T-660 | P2 | `PENDING` | T-640; confirm Langfuse/LangSmith hosting, export route/schema and actual sanitized producer/version fixture; exclude bodies/secrets and verify LLMTrace consumers. |
| 16 | RS-01 / Q04 | T-661 | P2 | `PENDING` | T-638, T-640; confirm actual OTel/GenAI export route/encoding, scope/schema URL, temporality and producer version before alias activation; test missing/zero and incompatible comparisons. |
| 17 | RS-01 / R20-04 | T-662 | P3 | `PENDING` | T-643; obtain owner-authorized natural nonzero-swap/elevated-pressure capture with collector/platform version and independent values; no forced memory stress. Existing multi-load is partial evidence, raw pressure code1 is not elevated-pressure proof. |

Execution order is T-646 → T-647 → T-648 → T-649/T-650 → T-651, then
T-652 after its dependencies and T-653 as the legacy-policy stream. PENDING
adapter/evidence tasks enter TODO only when their explicit inputs are available;
within activated work, P2 precedes P3 and dependency order takes precedence.
All ten RS findings are accounted for. RS-01 is locally addressed by registration;
RS-09 is partially addressed. Remaining tasks are open, and this disposition does
not replace independent review with PASS or certify new native/runtime evidence.

#### Decisions pending input confirmation

Q01/Q03/Q04 remain **DECISION_PENDING**, separate from task statuses/counts. Q02 is **SCOPED_SELECTED** for the first implementation fixture only; the user production engine remains undeclared.
They block only dependent product adapters, not structural design work. No values
are assumed and no support is declared from a product name alone.

| ID | Unresolved input decision | Evidence needed |
|---|---|---|
| Q01 | Langfuse/LangSmith hosting, export route and schema | Sanitized actual export and producer/version |
| Q02 | SCOPED_SELECTED: Ollama 0.40.16 / macOS 27.2.1 (26B131), Apple M5 Max | Actual synthetic 8-token run plus manual read-only API/macOS snapshots, now including matched/excluded multi-load 1/1, unmatched 0/2 and empty 0/0 selection coverage. GPU residency estimates, shared system bytes/pressure/swap and explicit unavailable GPU activity/power/thermal remain separate. Other engine/version, nonzero swap/pressure and user production choices remain unqualified. |
| Q03 | Target vLLM version range | Version/config, startup/runtime logs and timestamped metrics |
| Q04 | OTel AI metric export route (OTLP/Prometheus/range), encoding and instrumentation | Actual file, scope/schema URL, temporality and producer version |

Design-remediation validation: checked 15 scoped Markdown documents, 65 new/changed local
links and anchors, 39 tables in the new designs, balanced fences, the JSON example,
and KO/EN/board D01–D18 plus Q01–Q04 coverage. `git diff --check` and new-file whitespace
checks pass. Review archive integrity matches the original hash. Runtime tests,
offline packaging, migration, GPU adapters and Office pasting are future acceptance
work, not evidence from this edit. Shared assets are referenced only; no new reusable
rule/schema/fixture was created. Unrelated HTTP-capture edits remain outside this work.

Backlog activation validation (2026-11-11): all 232 registry rows reconcile with
210 DONE, 1 TODO, 18 PENDING and 3 REVIEW; T-628–T-645 IDs are unique, dependency
references resolve in order, and existing 214 registry rows are unchanged. The
four input decisions remain outside task counts. Documentation links/anchors and
whitespace checks are rerun before publication; no runtime code changed.

#### T-628 design authoring checkpoint (historical, 2026-11-11)

This records the documentation-only checkpoint at eb3e782. The implementation
completion record below supersedes its runtime status.

The design contents were already registered in T-628–T-645 by commit `83b8ff9`.
This follow-up fulfills the request to progress design authoring: the **design
portion of T-628 is written**, while its implementation task remains `TODO`.
No new task is counted and no downstream dependency is released by documentation.

Detailed contracts: [Korean parser](docs/ko/PARSER_DESIGN.md#t-628-상세-설계-범위와-현재-기준),
[Korean data model](docs/ko/DATA_MODEL.md#t-628-측정과-호환-상세-계약),
[English parser](docs/en/PARSER_DESIGN.md#t-628-detailed-design-scope-and-baseline),
[English data model](docs/en/DATA_MODEL.md#t-628-detailed-measurement-and-compatibility-contract).
The integration design links to these details instead of duplicating them.

| Portion | State | Deliverable / remaining evidence |
|---|---|---|
| Design authoring | Written, documentation only | Registry identity/alias/schema ownership; verified-vs-hint selection; source leases, bounded sinks, job/cancellation/partial/commit rules; stable proposed errors |
| Wire/compatibility design | Written, documentation only | Pointer-addressed measurements, numeric encoding and null/zero/absent; exact unknown metadata, Go/JS bridge boundaries, reader matrix and explicit migration |
| C1–C4 implementation | Not started | Registry/wrappers, job orchestration, decoder/metadata, one importer through CLI/Wails/export with conformance evidence |
| W01–W10 cases | Specified, not executed | Large integers/decimals, typed-key precedence, receiver reuse, malformed JSON, pointer validation, availability, version gating and bridge fidelity |

Code inspection found that common metadata retains unknown fields but uses default
JSON number decoding, which does not establish exact large-integer/decimal fidelity;
legacy profiler metadata still lacks unknown-key storage. The new design covers
both and explicitly avoids treating a Go serializer change as proof of UI/export
roundtrip precision. Existing confidence-based detection and thread override behavior
remain compatibility baselines until their wrappers pass tests.

Validation scope is documentation structure, links, mirrored examples/contracts and
Task Registry consistency. No runtime tests, production support or independent PASS
are claimed. The source review remains CONDITIONAL; Q01–Q04 remain unresolved and
separate from task statuses. Counts remain 210 DONE / 1 TODO / 18 PENDING / 3 REVIEW
out of 232. Unrelated HTTP-capture work is preserved outside this change.

### Open Work Snapshot

Desktop layout: [Korean design](docs/ko/DESKTOP_CONTENT_LAYOUT.md) · [English design](docs/en/DESKTOP_CONTENT_LAYOUT.md).
Mac implementation and detailed native validation are complete. Remaining work
includes transparent capture reach, the remaining legacy automation contracts, the
complete Windows menu/OS scaling matrix and full parity.
The table below retains those four acceptance items. The eighteen new local
diagnostics tasks are listed in the [dependency-ordered implementation backlog](#dependency-ordered-implementation-backlog):
T-628 is DONE after independent R3 accepted the R2 fixes with no P1/P2 blockers;
R3 P3 follow-ups are locally remediated, without renaming CONDITIONAL to PASS.
T-629 is DONE after independent R2 confirmed R4 remediation and allowed closure; R5 follow-ups are fixed or explicitly assigned to T-634/T-635, not renamed PASS. T-630 is DONE under independent R2 closure permission (R7-01 locally fixed; original verdict CONDITIONAL). T-631 is DONE under the independent review's explicit R8-01 remediation condition; all R8 findings are locally fixed and verified. T-632 is DONE under the explicit R9-01 remediation condition; all R9 findings are locally fixed and verified. T-633 is DONE under the explicit R10-01 remediation condition, with R10-01–R10-05 locally verified. T-634 is DONE under R11 closure permission after remediation; T-635 is DONE under R12 closure permission; neutral T-638 is DONE under R13 P2-remediation closure permission; T-636 is DONE under R14 P2-remediation closure permission with six P3 findings queued; T-637 is DONE under R15 P2-remediation closure permission; T-639 is DONE under R16 explicit P2-remediation closure permission; all six findings are locally fixed/verified. T-640 is DONE under R17 explicit P2-remediation closure permission, with R17-05 queued. T-641 is DONE under R18 explicit P2-remediation closure permission, all four findings locally fixed. T-642 is DONE under R19 closure permission after R19-01. T-643 is DONE under R20 closure permission after R20-01. Their nonblocking P3 queues and original CONDITIONAL verdicts remain. T-644 is DONE under R21 explicit P2-remediation permission; original CONDITIONAL and R21-02/04 queues remain. T-645 is DONE under R22 explicit no-blocker closure permission, with its three P3 findings queued. Lifecycle/native UI conditions remain below. The numbered-task open snapshot now contains 10 tasks: the four acceptance items below and 6 open follow-ups in the comprehensive-review backlog. Current review follow-up status supersedes the earlier queued descriptions:20 locally remediated, R20-04 EVIDENCE_PARTIAL; see the 2026-11-13 batch receipt. No independent final-fix PASS is claimed.

| Order | ID | Priority | Status | Why it is not `DONE` / next action |
|---:|---|---|---|---|
| 1 | T-601 | P1 | `REVIEW` | Windows capture entry points and reach measurement. |
| 2 | T-627 | P2 | `REVIEW` | Windows production native-menu entry to all 21 routes, locale switching and saved HTTP Headers/Body/reentry pass on 2026-11-12. Full populated/conditional/error, small-window and OS DPI acceptance remains. MSA mixed-language labels and raw file-dialog cancellation text are recorded follow-ups. |
| 3 | T-611 | P3 | `REVIEW` | Physical retention, pre-storage admission cancellation, owned custom-IID registration, nested normalized filters and common-engine soak tooling implemented. Mac and hosted race/vet/frontend/package plus both GUI cross-builds pass; amd64-race/386 native COM, including custom-IID RPC and explicit registration replacement/cleanup, pass. Current vendor early-bound compatibility, unobserved native filter/cache-origin evidence and foreign-process in-memory session-cookie integration remain. |
| 4 | T-613 | P3 | `PENDING` | Overall Windows parity depends on capture, automation and native acceptance gates. |

### Native Windows follow-up (2026-11-11)

T-623 is DONE. The production Wails/WebView2 app captured ten synthetic POSTs
from two real PID/start-time clients, preserved selected Headers/Body while new
rows arrived, filtered process B to 4/8 rows, and retained the selection with
follow disabled. Native Analyze/Capture menus navigated away and back: the same
running session and all ten rows restored. Page-local filters/selection/follow
reset on remount. F2 stopped recording; finalized analysis reopened the same
request and headers. Body capture was off; this does not repeat T-603's earlier
payload/policy acceptance.

Native execution exposed two Wails 3.1.0 x86 startup defects hidden by cross-builds:
an eight-byte POINT callback argument and nonexistent Win32 Ptr API imports.
The build wrapper now uses source-hash/version-guarded temporary SDK/modfile
copies, leaving the pinned dependency, module cache and amd64 path intact.
Both x86/x64 15-second startup and authenticated control checks pass. CI GUI
smoke now runs both architectures. PowerShell COM property reads without
ITypeInfo needed explicit IDispatch invocation; native x86/x64 COM lifecycle,
enumeration, attribution and save pass after fixing the harness. Unicode shortcut
execution passes. Native Go test/vet, x86 capture tests, frontend state/build and
desktop-only refusal pass. A 120.998s service/proxy soak passed 98 cycles and
25,480 requests; this is not a multi-hour desktop soak. Local race execution
remains unavailable without a C toolchain.

The user supplied a real legacy log. T-608 is now DONE: the bounded native
format-6/schema-7 codec preserves all 675 entries byte-for-byte. Direct reference
COM comparison verifies decoded fields, timings and retained bodies, and original
read/write interoperability verifies redacted semantic exports. The malformed
private Full XML export is not used as an oracle. Encoding, timezone and retained
text limits are documented in `docs/review/evidence/2026-11-11-windows/HALOG_CODEC.md`.
Raw logs/XML, descriptors and tokens
are excluded from Git. T-601 still needs a verified driver plus a peer fixture
for transparent/service reach; T-611 still needs its remaining vendor contract.
T-627 retains the full OS DPI/menu matrix: Computer Use returned
"Computer Use was not approved to use archscope-audit"; production UI checks
continued, without claiming that denied audit matrix. T-613 remains PENDING.

Evidence: [Windows follow-up](docs/review/evidence/2026-11-11-windows/README.md).
At that native follow-up checkpoint, counts agreed with all 214 registry rows; the old 202/214 opening headline
was stale relative to the previous 208 DONE rows.

Post-push follow-up: `cd01dff9` fixes read-only temporary SDK directory cleanup
on Unix; `8ded194e` accepts native 8.3 shortcut aliases by filesystem identity
while preserving argument checks. Engine Native run `42848607855` passed the
Windows full race/live/body/SEC-10/restoration checks, Linux checks, frontend and
both GUI cross-builds. Windows GUI Smoke `42848207160` passed amd64/386; general
CI `42848607891` passed. Engine Native's overall conclusion is cancelled because
two macOS jobs report unsupported `macos-14-arm64-us-default` before starting.
Exact scoped verdicts: [CI evidence](docs/review/evidence/2026-11-11-windows/ci-validation.json).

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

The 2026-11-10 non-HTTP correctness risks (T-614 through T-618) are closed locally after implementation and regressions. Trace elapsed values explicitly remain observed-chain estimates rather than causal critical-path proof. The three HTTP native/reference acceptance tasks T-601/T-611/T-613 remain open after T-608 completion. The older audit/gate statements below retain their historical scope.

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

### Local diagnostics implementation (2026-11-11)

T-628 is DONE under R3's independent closure permission. R3's nonblocking P3
follow-ups are locally verified; the recorded verdict remains CONDITIONAL. **T-629 is DONE under R2 closure permission**; R5-01 has local boundary/error
regressions and R5-02 is a scoped T-634/T-635 acceptance requirement. Actual UI/
Windows/CI evidence remains separate.
T-630 is DONE under R2's explicit closure permission after R6-01–R6-07 were
independently confirmed; R7-01 is locally fixed and the original verdict remains
CONDITIONAL. T-631 is DONE under the independent review's explicit R8-01-remediation closure
condition. R8-01–R8-04 are locally fixed with scale, cache lifecycle, cancellation,
exact-export, typed-refusal and scoped native Mac UI evidence. The original verdict
remains CONDITIONAL; no independent verification of the final changes is claimed.
T-632 is DONE under the independent review's explicit after-R9-01-remediation
closure permission; all R9-01–R9-04 fixes are locally verified. The original
CONDITIONAL is preserved, with no independent re-review of the final fixes.
T-633 is DONE under its independent review's explicit R10-01 remediation closure
condition; R10-01–R10-05 are locally fixed and verified. The original CONDITIONAL
is preserved; independent final re-review and new native UI/Windows acceptance
remain separate. T-634 is DONE under R11 closure permission after R11-01 remediation;
all four findings are locally fixed, preserving CONDITIONAL. T-635 is DONE under R12
explicit closure permission; all R12 findings are locally fixed and verified. T-638 is DONE; R13-02–07 are locally fixed. T-636/T-637/T-639/T-640/T-641/T-642/T-643/T-644/T-645 retain their original DONE permissions and CONDITIONAL verdicts. The 2026-11-13 batch remediates20 of21 follow-ups in order, with one commit per item. R20-04 has actual multi-load evidence but still needs an authorized natural nonzero-swap/elevated-pressure capture; other environments and long-duration qualification are disclosed separately. No forced memory stress is authorized or used. The 2026-11-13 user instruction now authorizes commit and push of the review disposition and existing committed batch; the earlier publication hold is superseded, while missing R20-04 evidence remains PENDING as T-662. The four native Windows acceptance tasks remain separate.
T-646–T-653 are locally complete. Continue selected actual-producer qualification in T-655–T-661; T-654/T-662 retain hardware/natural-evidence gates. T-652 prerequisites are satisfied and it is now TODO after the earlier queue. Follow the
[comprehensive-review backlog](#comprehensive-review-follow-up-backlog-2026-11-13)
for T-648–T-653 and the explicit activation gates on T-654–T-662. There are 5
TODO tasks and 10 new PENDING tasks; existing DONE verdicts remain unchanged.
Q01–Q04 and real-producer fixtures still gate their adapter-specific claims.
T-635 must preserve indexed_lines/index_truncated. Existing Windows/CI evidence
at 5b58374 is preserved and does not cover these new R8 changes. Follow the
[ordered backlog](#dependency-ordered-implementation-backlog).
The first complete user workflow remains profile correctness → drilldown/comparison
→ image/table copying. Move a task from PENDING to TODO only when its registered
prerequisites pass; Q01–Q04 gate only the corresponding input-specific adapters.
Additional D17 batches T-655–T-659 need selected producers and sanitized fixtures before activation.
Existing HTTP/desktop acceptance tasks remain a separate stream; they do not block
T-628 and are not closed by this documentation change.

### Completed other-feature correctness follow-ups (2026-11-10)

T-614 through T-618 are implemented in individual commits. Full Go/vet/race, frontend state/build, isolated browser UI checks and Mac/Windows cross-builds pass. The four open HTTP acceptance tasks remain in the snapshot alongside the newly registered diagnostics backlog; their native/reference acceptance remains separate.

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
| 2026-11-12 T-633 independent implementation review (claude-code) | Verdict CONDITIONAL, no P1. Discovery is snapshot/hash based with no adjacent-file scan; sidecars bind only on exact hash+size and leave undeclared context missing; drafts stay unknown; first-row mapping preview is sample-only (12.50 -> 0.0125 seconds); manual kit is inert; real CLI conversion with Temurin 11/17/21 succeeds, SIGINT kills the converter process tree and cleans scratch, offline refuses conversion before executable access, legacy binary JFR requires explicit ARCHSCOPE_JFR_CLI; verified at 3d443d3 (102 Go packages race, vet, Windows amd64/386 cross-vet, frontend tsc/state, 88 evidence hashes). P2: R10-01 the 64 MiB conversion output cap trips on ordinary recordings (jfr print --json expands 5x-30x: a 10.5 MB 40 s profile recording yields 310 MB JSON and fails CONVERTER_OUTPUT_LIMIT in both jfr convert and legacy jfr analyze-*, while the same JSON analyzed after manual conversion succeeds) and the error text blames the JDK rather than size. P3: R10-02 provenance hard-codes tool jdk.jfr for any absolute executable (JDK 11/17 report version 1.0), R10-03 >16 KiB stderr fails a successful conversion, R10-04 output collision found only after full conversion with internal temp path leaked and ~4.4x RSS, R10-05 PARSER_DESIGN still documents PATH/JAVA_HOME discovery. Not verified: real WebView UI, Windows Job Object native execution, desktop network observation, hosted CI, real producers. Review document: docs/review/done/2026-11-12_claude-code_T-633_implementation-review.ko.md. Processed before unchanged archival. R10-01–R10-05 are now locally remediated and verified; DONE under explicit after-R10-01 permission, original CONDITIONAL retained with no final independent re-review. Evidence: docs/review/evidence/2026-11-12-t633-r10/local.json. |
| 2026-11-12 T-632 independent implementation review (claude-code) | Verdict CONDITIONAL, no P1. Exact-manifest-byte Ed25519 verification with trust only from the registered store, closed ZIP/mapping vocabulary with exact decimals (probes: precision preserved, null/negative counter/1e-400 refused), sequential trust updates, revision-guarded atomic activation, hash/undeclared-file-rejecting offline bundle, three-importer mandatory fixtures and seven guarded app network/exec entry points verified at 34dbd8f (99 Go packages race, vet, Windows amd64/386 cross-vet, frontend tsc/state, 170 evidence hashes, real CLI packs/offline build/verify/tamper). P2: R9-01 revoking the bundled bootstrap key (a valid signed rotation, reachable from Settings and packs update-trust) makes OpenDefault fail its bundled-pack reinstall with PACK_SIGNER_UNTRUSTED, so the app evidence service and every packs CLI command (including recovery commands) fail on the next start; a revoked active-pack key also blocks all new imports. P3: R9-02 application-scope activation is not checked against existing user/case scopes (user scope later fails with PACK_ALIAS_CONFLICT), R9-03 Snapshot loads/decompresses/revalidates every active pack three times per call (93 ms for 12 MiB, ~0.25 s at the 32 MiB cap) under the manager mutex, R9-04 mapped metric names pass BiDi/control characters (same class as R8-03). Not verified: real WebView UI, Windows native Fixed Version launch/offline browser observation, desktop network observation, hosted CI, real producers. Review document: docs/review/done/2026-11-12_claude-code_T-632_implementation-review.ko.md. Processed into the ordered R9 queue below before unchanged archival. Intake was separate from T-631. R9-01–R9-04 are now locally remediated and verified; T-632 DONE under the explicit R9-01 closure condition, without changing the original CONDITIONAL or claiming final independent re-review. Evidence: docs/review/evidence/2026-11-12-t632-r9/local.json. |
| 2026-11-11 T-631 independent implementation review (claude-code) | Verdict CONDITIONAL, no P1. Capabilities come only from compiled providers (forged imported strings grant nothing), exact values/null/zero stay text, page/column/cell bounds hold, comparison refusals carry no delta and declared identity; verified at ff06c54 (96 Go packages, vet, frontend tsc/state, Windows cross-vet pass; real CLI describe/query/compare). P2: R8-01 timeline/SLI payload validation is quadratic (referenced() re-marshals the id list per row; 2k rows 1.2 s, 8k 19 s, 16k 75 s per Capabilities call, repeated on every describe/page) — latent until a producer registers a timeline/SLI provider. P3: R8-02 every describe/page re-reads and re-scans the whole result (~0.5 s per 25-row page at 100k rows), R8-03 BiDi/control characters displayed raw, R8-04 incompatible comparison target returns an untyped error instead of a typed refusal. Processed into the ordered R8-01, R8-02, R8-04, R8-03 follow-up queue before unchanged archival. R8-01–R8-04 locally fixed/verified on 2026-11-12; DONE under this review's explicit after-R8-01-remediation permission. Original verdict CONDITIONAL; no independent re-review/PASS for final fixes. | docs/review/done/2026-11-11_claude-code_T-631_implementation-review.ko.md |
| 2026-11-11 T-630 re-review R2 (claude-code) | R6-01–R6-07 independently resolved; no P1/P2 and explicit DONE return permission. Nonblocking R7-01 locally fixed: preserve busy code and show corruption cause/opaque handle. Original verdict CONDITIONAL; no independent verification of final P3. | docs/review/done/2026-11-11_claude-code_T-630_implementation-re-review-R2.ko.md |
| 2026-11-11 T-630 independent implementation review (claude-code) | Verdict CONDITIONAL, no P1. Exact numeric/null/zero snapshots survive save→export and portable pack→restore byte-for-byte; revision/HEAD, single-writer, token-gated purge and shared-asset digests verified at 82bc6a8 (96 Go packages, frontend tsc/state, Windows cross-vet pass). P2: R6-01 export-destination guard is bypassed by a case-differing path on case-insensitive volumes (macOS verified) and can overwrite a managed HEAD.json, making the case unreadable; R6-02 the shared evidence-cache moved to permanent app-data has no release/cleanup path and, when full (1,024 results/512 MiB), blocks new imports and reopening saved cases. P3: R6-03 repeated save duplicates result refs, R6-04 corrupt manifest/HEAD cannot be planned or purged, R6-05 fail-closed cache migration has no recovery path, R6-06 source page decodes the whole file per call and refuses >1M lines, R6-07 colon in a local file name is rejected. Processed R6-01–R6-07 into the ordered T-630 follow-up queue before unchanged archival; R6-01–R6-07 are now locally remediated and verified, pending independent re-review. T-631 is explicitly not blocked. Original verdict remains CONDITIONAL; WebView IPC/UI, Windows native execution and hosted CI remain unverified. | docs/review/done/2026-11-11_claude-code_T-630_implementation-review.ko.md |
| 2026-11-11 T-629 re-review R2 (claude-code) | Verified R4-01–R4-05 remediation at 27fcd3d: 4.9 MB and 32.1 MB models now import (chunked transport; typed budget detail), lost-only profile comparisons succeed with quality-difference warnings, metadata.quality tampering rejected, validation cache cuts 7 MB Check from ~387 ms to ~25 ms. No P1/P2. P3: R5-01 (single string >4 MiB rejected as FORMAT_MISMATCH without budget detail), R5-02 (capture_options compared wholesale; pid/duration differences refuse). Processed before unchanged archival: R5-01 fixed/locally verified; R5-02 contract and acceptance assigned to T-634/T-635, runtime remains strict. T-629 DONE under explicit R2 permission; verdict remains CONDITIONAL. WebView IPC/UI, Windows native execution and hosted CI remain unverified. | docs/review/done/2026-11-11_claude-code_T-629_implementation-re-review-R2.ko.md |
| 2026-11-11 T-629 independent implementation review (claude-code) | Verdict CONDITIONAL, no P1. Legacy CLI output identical to 63d3b6a; exact numeric/null/zero handling and recomputed-result validation verified. P2: R4-01 effective capacity far below documented budgets (1 MiB single-record cap, ~25k-node tree serialization cap; 4.9 MB normalized model fails with generic IMPORT_BUDGET_EXCEEDED), R4-02 comparison identity includes measured quality counts and process identity so differing captures are refused. P3: R4-03 per-Check full recomputation/result size, R4-04 metadata.quality mirror unverified, R4-05 >64 KiB models not auto-detected. Processed R4-01–R4-05 before unchanged archival; local fixes and scale/tamper/cache/compare regressions pass. R2 independently confirmed R4 resolution and permitted T-629 DONE; see latest disposition. WebView IPC/UI, Windows native execution and hosted CI remain unverified. | docs/review/done/2026-11-11_claude-code_T-629_implementation-review.ko.md |
| 2026-11-11 T-628 re-review R3 (claude-code) | Verified R2 N-01–N-04 remediation at 6c3717d: different-source scalar comparison now works (delta 1, 100/3 percent, ordered baseline/target evidence), public Release removed, max_lines EOF lookahead, projection budgets. No new P1/P2; P3 R3-01 (undifferentiated comparison refusal reason), R3-02 (declared, unverified comparison identity), R3-03 (max_lines marker on blank-only suffix). Processed R3-01–R3-03 before archiving the unchanged review; locally remediated with regression evidence. T-628 DONE under explicit R3 permission; CONDITIONAL not renamed PASS. WebView IPC/UI, Windows native execution and hosted CI remain unverified. | docs/review/done/2026-11-11_claude-code_T-628_implementation-re-review-R3.ko.md |
| 2026-11-13 T-628-T-645 comprehensive cross-cutting review (claude-code) | Verdict CONDITIONAL (P2 3/P3 7, no P1) at f69daa2. No computation or contract defect found: parsers never import analyzers/exporters, T-628+ analyzers never import parsers, new model/parser/analyzer packages use no float64 measurement arithmetic, go list -deps shows no net/http outside legacy httpcapture packages, doc mirrors byte-identical, full go test ./... 120 packages ok. Re-ran saved repro inputs against the 2026-11-13 batch: R19-02/03/04, R20-02/03, R21-02, R22-01/02 confirmed fixed. P2: RS-01 unfinished scope (D16 CUDA/NCCL and distributed inference, D17 remaining adapter classes, Q01, Q04, R20-04 partial) has no open task ID while the table shows 0 TODO; RS-02 nine exact patch-version pins (Nsight, PyTorch, PostgreSQL, iproute2, Ollama, k6, JMeter) with an unverified-version opt-in only for JMeter; RS-03 air-gap guarantee is not regression-protected (cooperative netpolicy, no import-boundary test, offline syscall verification and browser QA not in CI). P3: RS-04 every model Strict delegates to metricmodel.Strict so a 42 MiB k6 summary inside the documented 64 MiB is refused with METRIC_BUDGET_EXCEEDED at 32 MiB, plus 9 duplicated helper sets; RS-05 renderer hard-codes result type/payload versions (already caused the execution_evidence 0.2 panel miss) and has one frontend test file; RS-06 four unlabeled percentile definitions in legacy analyzers; RS-07 six model packages and the SVG exporter have no tests, no fuzzing of new parsers, R13-R17 fixes not independently re-verified; RS-08 adding a family touches about 70-110 files with per-family cursor/CLI/service/component copies; RS-09 user docs keep stale review history contradicting current behaviour (EXECUTION_EVIDENCE en:164-166, ko:142); RS-10 two exporter-to-analyzer imports and no layering test. Existing DONE states stay; -race full suite, WebView/Windows, offline syscall rerun not executed. | docs/review/2026-11-13_claude-code_T-628-T-645_comprehensive-review.ko.md |
| 2026-11-12 T-645 independent implementation review (claude-code) | Verdict CONDITIONAL (P3 3, no P1/P2, nonblocking). Independently verified with my own disposable network-none PostgreSQL 16.14 server (different workload and order, shipped read-only SQL): 4 snapshots import to 15 series/60 points/14 derived intervals; unknown-reset intervals give no delta, post-reset deltas for all 14 counters equal snapshot differences and rates equal exact rationals (xact_commit 6 -> 250000/96429); 18 tampered captures (future reset, wrong version/settings, overlap, 2^64 overflow, path/connection-string tags, unknown field, start>end) refused, a counter drop without reset blocks only that counter. My own iproute2-6.1.0 ss -s capture (IPv4/IPv6 listeners, UDP, 5 connection pairs) yields 21 series/84 points, 84/84 values equal an independent parse; extra column, locale, netstat layout, 17 KiB summary, other version refused. T-644 R21-01 confirmed (73.5 MB padded Nsight export -> 24 KB subset with receipt, 14 kernels, sum 77,888 / union 57,696 ns equal sqlite3) and R21-03 confirmed (100,500 ignored records do not consume the supported-event budget; at_least wording). P3: R22-01 first known reset after an unknown epoch gets no rate and a start_time_unavailable status although its start is known, while a known-to-known reset gets a rate; R22-02 numbackends=0 and a backwards-moving stats_reset are accepted; R22-03 a never-reset database (stats_reset NULL) yields no deltas with no documented reason/procedure and no shipped PostgreSQL collection helper. Full 118-package suite, WebView/Windows, offline syscall rerun, other PostgreSQL/iproute2 versions and real production servers not executed. | docs/review/done/2026-11-12_claude-code_T-645_implementation-review.ko.md |
| 2026-11-12 T-644 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 1/P3 3, no P1). Independently verified with a freshly installed PyTorch 2.15.0 (own two-rank CPU/Gloo workload): rank-0 trace imports 228 events = 216 cpu_op + 12 annotations, exact ts/dur to integer ns, 216/216 CPU self values equal an independent containment computation, annotation names and host_name absent, rank/clock isolation (duplicate source, time query and SVG without a source clock refused), window import of a 59.8k-event trace returns the independently expected 30,049 events; Nsight bundled SQLite kernel sum/union equal sqlite3, and an overlapping-stream variant gives sum 77,888 / union 57,696 ns exactly; end<start, wrong export version and missing device map refused; SVG well-formed with only the selected source provenance. T-643 R20-01 fix confirmed on real Ollama (1 matched/1 excluded and 0/2; models unloaded, /api/ps empty). R21-01 (P2): the 64 MiB raw-byte and 100k raw-event limits apply before the time window, so a 73.5 MB Nsight SQLite with an explicit window and a 103.5k-event PyTorch trace are refused although the docs say large exports need a window. P3: R21-02 missing or view-shadowed optional Nsight tables are silently skipped with no disclosure (docs say views are refused), R21-03 ambiguous budget errors (TORCH_SCHEMA_OR_RECORD_BUDGET has no counts, observed=50001 is not an exact count), R21-04 bundled Nsight fixture has sum == union and no ambiguous-correlation/flow input. Closure allowed after R21-01; real nsys, CUDA/NCCL, full 118-package suite, WebView/Windows not executed. | docs/review/done/2026-11-12_claude-code_T-644_implementation-review.ko.md |
| 2026-11-12 T-643 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 1/P3 3, no P1). Independently verified on this Apple Silicon Mac with real Ollama 0.40.16: the bundled collector run empty, with gemma:2b loaded (size = size_vram = 1799817788, matches ollama ps; model unloaded afterwards, /api/ps back to empty), imports to metrics_timeseries 0.2.0; strict-input refusals (22 mutations), GPU/power numeric values refused, exact uint64 kept above 2^53, fixture hash matches manifest. R19-01 closure confirmed: 60k-row 3-label JTL, 27 statistics equal a Python nearest-rank computation, 1,000 rows stay core 1.0.0 and 1,001 switch to 1.1.0 distributions, 1.0.0 vs 1.1.0 compare works, 11 tamper variants refused, 1M rows in 1.2 s / 86 MB. R20-01 (P2): the collector filters /api/ps to the selected model name and records nothing about excluded models; with gemma:2b and nomic-embed-text both loaded only gemma appears, and a mistyped --model yields the same empty models as a genuinely unloaded model. P3: R20-02 importer checks syntax not invariants (size_vram > size, vm_free > capacity, path-like host_id, uppercase digest accepted), R20-03 the 64 MiB input limit binds before the documented 1M rows (about 440k rows with default JMeter columns) with no hint in the error, R20-04 single-machine/version/synthetic-request qualification, no multi-model or swap/pressure capture. Closure allowed after R20-01; full 116-package suite, WebView/Windows, offline syscall rerun not executed. | docs/review/done/2026-11-12_claude-code_T-643_implementation-review.ko.md |
| 2026-11-12 T-642 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 1/P3 3, no P1). Verified independently: 27 JTL statistics (3 labels x count/errors/min/max/mean/p50/p90/p95/p99 on a 30k-row JMeter-layout CSV incl. comma labels and multiline failure messages) match a Python nearest-rank computation exactly; a real OpenJDK 21.0.10 hs_err (1,129 lines, SIGSEGV) leaks no host, user, path, environment or command line; baseline comparison blocks mixed/unknown arrival, concurrency, warmup, phase and target differences and reports exact differences (-500, -100/3, null ratio on zero baseline), unavailable rows and tamper refusal; k6 ms->s conversions exact; T-641 R18-01..R18-04 fixes confirmed with real faulthandler/jstack (context flag restores persistent_blocked 1/unique 23, missing context now warns). R19-01 (P2): every JTL sample is kept as a ~1 KB record with a 50k-row cap, so a 60k-row run is refused (no counts) and split files cannot have percentiles merged. P3: R19-02 real hs_err elapsed time is null (expects a line-start), native symbols are cut at the first space and Java frames repeat across sections unlabelled, R19-03 no JTL plausibility checks (epoch seconds accepted as ms, Latency > elapsed), R19-04 other JMeter versions are refused with no override. Closure allowed after R19-01; real k6/JMeter/LLMPerf runs and native UI/Windows not observed. | docs/review/done/2026-11-12_claude-code_T-642_implementation-review.ko.md |
| 2026-11-12 T-641 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 2/P3 2, no P1). Independent inputs were produced by running the pinned CPython 3.14.4, Node v25.9.0, rustc 1.94.1 and OpenJDK 21 on the review host: tracemalloc 3,099,483 bytes/1,124 blocks match the allocation profile total exactly, GC callback pause sum 1,799,000 ns equals pause_union_ns (overlaps unioned, concurrent separate), Node event-loop delay/utilization preserved as exact decimals, Rust v0 backtraces restored 14/14 and 41/41 frames without the panic body, real jstack states match, CPU-thread links classify qualified/candidate/incompatible exactly as documented; T-640 R17-01/R17-02 fixes confirmed. R18-01 (P2): a real Python 3.14.4 faulthandler dump (Thread 0x... [name] header) imports with success and zero threads while the same file without the [name] imports 3, so format mismatch and empty dump are indistinguishable. R18-02 (P2): analyze-multi payload 0.2.0 joins dumps only with a correlation_context that nothing in the repository sets, so two real jstacks go from persistent_blocked 1/unique 23 (0.1.0) to 0/46 with no warning. P3: R18-03 one reversed GC interval rejects the whole file without coordinates, R18-04 jstack nid is decimal on macOS but always labelled jvm_nid_hex. Closure allowed after R18-01 and R18-02; real PostgreSQL/py-spy/Memray and native UI/Windows not observed. | docs/review/done/2026-11-12_claude-code_T-641_implementation-review.ko.md |
| 2026-11-12 T-640 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 1/P3 4, no P1). Verified with the real CLI: body-free normalization (12 sensitive locations -> 0 occurrences, 13 omitted fields counted), observed child-union self time exact (100-(50+10)=40 ms) with out-of-parent children left unqualified, cycle/duplicate/missing-parent audit, TTFT 200 ms/TPOT 250 ms exact with not-distinct/reversed/1-token statuses, retry/fallback chains, previous-attempt cycles, client/server pairs with explicit token authority, raw LLM carriers refused in legacy trace import and case source (plain/gzip/unicode-escaped); T-639 R16-01/02/03 fixes confirmed (declared range metadata, generated event IDs, [N/A] and trailing blank). R17-01 (P2): inference Observations convert() parses rates with Number(), which rejects fraction strings such as 20/3, so non-integer counter/precomputed rates become null while rate_meaning stays set (missed in the T-639 review; fixtures only have integer rates). P3: R17-02 raw-carrier gate limited to gen_ai./openinference./diag.* so Traceloop/Langfuse bodies are copied into case objects in clear, R17-03 per-page ProjectLLM + full-row serialization (3.9 s/page at 20k spans), R17-04 documented 50k-span limit reached near 35k by the 32 MiB model, R17-05 contradictory token boundaries classified inconsistently and real GenAI spans carry no first/last-token fields. Closure allowed after R17-01; real Langfuse/LangSmith exports and native UI/Windows not observed. | docs/review/done/2026-11-12_claude-code_T-640_implementation-review.ko.md |
| 2026-11-12 T-639 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 2/P3 4, no P1). Verified with the real CLI: NVIDIA CSV conversions exact (75%->0.75, 8192 MiB->8589934592 bytes), N/A/[Not Supported]/[Insufficient Permissions] kept distinct with null values, out-of-range values nulled with raw kept, request TPOT (1.8-1.2)/(3-1)=300000000 ns and no TPOT for 1 token, scope/unit mismatches refused, no replication or summing; T-637 R15-01 (keyed hist), R15-02 (20k/24k perf samples with repeated stacks) and R15-06 (decimals) fixes confirmed. R16-01 (P2): MetricHistory from Prometheus range (always gauge, unit empty) cannot be bound to any signal (4 attempts fail with bare unit/counter mismatch codes). R16-02 (P2): structured events (runtime OOM/Xid and request TTFT/TPOT) need a self-hash event ID that only the fixture generator can compute; blank or wrong IDs fail with INFERENCE_EVENT_INVALID. P3: R16-03 [N/A] and a trailing blank line abort the whole CSV, R16-04 result capacity ~80k points with a 'select fewer series' hint that this path cannot act on, R16-05 unique-frame perf input still hits the model limit near 4k samples (document), R16-06 first==last multi-token request reports TPOT 0. Closure allowed after R16-01 and R16-02; real NVIDIA/vLLM collection and native UI/Windows not observed. | docs/review/done/2026-11-12_claude-code_T-639_implementation-review.ko.md |
| 2026-11-12 T-637 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 2/P3 4, no P1). Verified with the real CLI: scheduler spans (on-CPU 500 ms, sleeping off-CPU 600 ms, wakeup run-queue 100 ms, preempted runnable 200 ms) pair exactly; unpaired/ambiguous/loss-affected spans stay null; binary perf.data, --header, wrong layouts, overlapping/unsupported BPF messages and forged positions are refused; scheduler projections feed existing profile paths; T-636 R14-01 table fix confirmed (paths/URLs/headers unchanged). R15-01 (P2): keyed-map bpftrace histogram, documented as supported, always fails with a raw Go JSON error (map decoded into a map that still holds the raw entry). R15-02 (P2): with realistic 25-frame stacks the 16 MiB model limit trips at ~4.3k samples (3k ok at 13.9 MB; 6k/20k SYSTEM_MODEL_BUDGET without counts or guidance); documented 10k/20k figures are for 2-3 frame stacks. P3: R15-03 capture original_sha256 never compared to the source, R15-04 every waker/idle PID/TID must be declared and idle TID 0 yields ambiguous rows (9/21), R15-05 scheduler projections are a single [unattributed] frame and off-CPU merges sleeping+preempted, R15-06 R14-01 fix makes decimals like 0.1/1.5/3.14 text in TSV. Closure allowed after R15-01 and R15-02; real perf/BPF collection and native UI/Windows not observed. | docs/review/done/2026-11-12_claude-code_T-637_implementation-review.ko.md |
| 2026-11-12 T-636 independent implementation review (claude-code) | Verdict CONDITIONAL (P2 1/P3 6, no P1). Verified on real pprof results: Top view values/denominator match profile-query, flamegraph/diff views pin populations and quality, spec/view binding tampering is refused, imported HTML is rebuilt from the view, CLI refuses new render/clipboard and exports saved bytes unchanged; T-638 R13-01 0.2.0 storage fix confirmed (90k points/10 series: 3 series 17.7 MB, all 59.0 MB). R14-01 (P2): table copy/TSV/HTML prefix an apostrophe to every string containing '/', altering Go package/path frame names and headers (HTML already requests text format); date-like strings stay unprotected. P3: R14-02 diff rows in node-ID order (arbitrary first image, zero-delta rows), R14-03 profile offset silently ignored, R14-04 >5-image presentations pass create-spec/validate, R14-05 ZIP/validate are self-attesting (consistently re-hashed PNG accepted), R14-06 10.4 MB font per SVG (5-image bundle 33 MB; store ~16 bundles; Report Pack fails at 3 five-image bundles), R14-07 generic metric tables now only point_id/status. Closure allowed after R14-01; native WebView/clipboard/Office/Windows not observed. | docs/review/done/2026-11-12_claude-code_T-636_implementation-review.ko.md |
| 2026-11-12 T-638 independent implementation review (claude-code) | P2 R13-01 locally remediated with compact payload 0.2.0/selected model/minimal projections/compact CLI; 100k-point saved readback verified. T-638 DONE under original explicit closure condition; R13-02–R13-07 are now locally fixed (original findings below preserved). Verdict CONDITIONAL (P2 1/P3 6, no P1). Interval arithmetic verified: reset/overlap/unannounced-decrease/delta-overlap statuses, exact rate fractions, explicit/exponential histogram deltas with schema-change refusal, [from,to) windows, four tamper refusals, provenance dedup and gzip-bomb refusal; T-635 R12-02–R12-04 fixes confirmed. R13-01 (P2): the saved result embeds the whole normalized model plus two interval tables, so the practical capacity is about 34k points rather than 100k and selecting fewer series does not shrink the model (90k-point export: 1 series ok at 45.8 MB, 3 series fails with an unexplained budget error). R13-02 per-page decode/validate cost (~2.7 s at 90k points), R13-03 range step never reports gaps, R13-04 numeric/format errors lack coordinates, R13-05 identical cross-export duplicates labelled conflicting, R13-06 CSV has no interval rows, R13-07 series picker scale are P3. Closure allowed after R13-01; native UI/Windows not observed. | docs/review/done/2026-11-12_claude-code_T-638_implementation-review.ko.md |
| 2026-11-12 T-635 independent R12 review (claude-code) | CONDITIONAL, P3 only. All four findings queued before unchanged archival, then locally remediated/tested during T-638. Explicit review permission allows T-635 DONE. Cache trust/byte rechecks, self sorting, isolated index masking and actionable result limits are recorded; no final independent PASS or native acceptance is inferred. | docs/review/done/2026-11-12_claude-code_T-635_implementation-review.ko.md; evidence/2026-11-12-t638/local.json |
| 2026-11-11 T-628 re-review R2 (claude-code) | Verified R-01–R-14 remediation at 63d3b6a (R-01 P1 resolved; deep 256–4000 frame CLI/evidence exports pass). R-04 only partially resolved. New findings: N-01 (P2, population identity includes source hash so different-source before/after scalar comparison is always refused), N-02–N-04 (P3). Verdict CONDITIONAL; T-629 can start; T-628 DONE return pending N-01. Processed into the ordered R2 intake; N-01–N-04 now locally remediated with regression evidence, pending independent closure. WebView IPC/UI, Windows native execution and hosted CI remain unverified. | docs/review/done/2026-11-11_claude-code_T-628_implementation-re-review-R2.ko.md |
| 2026-11-11 T-628 independent implementation review (claude-code) | Verdict CONDITIONAL: R-01 (P1, depth-64 JSON limit breaks 31+ frame profiler results in report/evidence export; regression vs eb3e782), R-02–R-04 (P2), R-05–R-14 (P3). T-628 DONE held. Processed R-01–R-14, archived the original unchanged, and locally verified fixes. T-628 remains REVIEW; T-629 is ready under the review's R-01 regression condition. No independent re-review PASS is claimed. WebView IPC/UI and Windows native execution remain unverified. | docs/review/done/2026-11-11_claude-code_T-628_implementation-review.ko.md |
| 2026-11-11 local diagnostics evolution design review | Processed original `CONDITIONAL`; F-01–F-12 accepted with documented qualifications, 21 optional proposals dispositioned (G-11 superseded). KO/EN design and companion contracts revised. This is design remediation, not independent PASS or runtime completion. | See Local diagnostics improvement design above: D01–D18 mapped to T-628–T-645 and separate Q01–Q04 `DECISION_PENDING`; original archived unchanged under `docs/review/done/`. |
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

The 2026-11-11 local diagnostics review follow-up is registered as T-628–T-645
at the user's request. The new rows are ordered by priority and prerequisite;
Q01–Q04 are input decisions, not task IDs. Registration leaves the original review
verdict CONDITIONAL and does not claim runtime completion.

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
| T-646 | P2 | DONE | Enforce architecture and offline regression gates | T-632 | RS-03 + RS-10: Existing T-632 baseline; enforce import/layer rules with explicit legacy exceptions, Linux offline syscall CI with positive control, and localconvert denial before execution. Locally implemented and verified in e27e4ab; hosted CI and independent re-review remain unrun. |
| T-647 | P2 | DONE | Unify explicit unverified producer-version policy | T-628, T-642 | RS-02: Existing version/qualification contracts; specify and implement explicit unverified-version policy across nine pinned paths; propagate actual/unknown versions and qualification through results, exports and comparison; retain strict defaults and old readers. No automatic version-range expansion. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-648 | P3 | DONE | Unify strict JSON validation and effective byte budgets | T-628 | RS-04: Existing model contracts; single caller-owned JSON byte limit, domain-correct errors and shared validation helpers; reproduce 42 MiB k6 rejection, test exact boundaries and preserve strict parsing/old identities. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-649 | P3 | DONE | Bind renderer panels/actions to result capabilities | T-631 | RS-05: T-631 capability baseline; backend-owned panel/action capability and registry-to-renderer contract coverage for every supported type/payload, including execution_evidence 0.2.0 and unknown-version refusal. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-650 | P3 | DONE | Close model, parser fuzz, SVG and independent revalidation gaps | T-628–T-645 | RS-07: Current model/parser/exporter contracts; six model boundary suites, bounded text/CSV/JSON parser fuzz and SVG XML/escaping tests; retain adversarial inputs and request independent R13–R17 follow-up revalidation separately. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-651 | P3 | DONE | Reconcile current user guides and EN/KO coverage | T-632, T-642 | RS-09: Immediate EN/KO execution-guide contradiction fixed at intake; finish current-behavior cleanup of user guides and bundled mirrors, move task history to tracker/reviews, and add the missing EN MSA_SERVLET_DISPATCH counterpart. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-652 | P3 | DONE | Extract bounded acquisition and digest-bound query components | T-648, T-649 | RS-08: T-648, T-649; extract bounded acquisition/lease and digest-bound cursor/page components without domain mixing; demonstrate one family migration preserving contracts and require existing-result reuse assessment before a new family. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-653 | P3 | DONE | Disclose legacy percentile policies without changing saved values | Legacy analyzer contracts | RS-06: Existing legacy golden results; inventory interpolation/floor/nearest-rank/reservoir policies and annotate percentile_method with versioned compatibility; independently measure cross-family examples and avoid rewriting saved numbers. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-654 | P2 | PENDING | Qualify actual CUDA/NCCL distributed-inference execution | T-644; actual CUDA/NCCL inputs and hardware | RS-01 / D16 remainder: T-644; activate only with permitted sanitized actual CUDA/NCCL distributed-inference inputs, selected PyTorch/Nsight/tool versions and available hardware; distinguish actual execution from structural fixtures and rank clocks. Registered from the 2026-11-13 comprehensive review; implementation/qualification remains open. |
| T-655 | P2 | DONE | Qualify an additional requested profile adapter | T-645; demand/input/version | RS-01 / D17 profile: T-645; select an unmet profile format after importer-matrix review, actual sanitized input, demand and producer/version; reuse ProfileSet/providers and pass old-reader/common-consumer acceptance. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-656 | P2 | PENDING | Qualify an additional serving adapter and Q03 version | T-639, T-645; Q03/input/version | RS-01 / D17 serving, Q03: T-639, T-645; choose serving adapter/version/config and permitted startup/runtime/metric captures, including Q03 for vLLM; verify neutral contracts and disclose qualification. Registered from the 2026-11-13 comprehensive review; implementation/qualification remains open. |
| T-657 | P2 | DONE | Qualify an additional requested DB adapter | T-645; DB vendor/input/version | RS-01 / D17 DB: T-645; choose additional DB/vendor (e.g. AWR/ASH/Tibero only if requested), version and permitted sanitized export; declare counter/reset semantics and exact common projections. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-658 | P2 | DONE | Qualify an additional requested OS adapter | T-637, T-645; OS input/version | RS-01 / D17 OS: T-637, T-645; select OS/tool version and permitted actual evidence beyond the delivered ss summary; declare scope/clock/units and reuse existing results where possible. Locally implemented and verified; see the dated task receipt. Original comprehensive verdict remains CONDITIONAL. |
| T-659 | P2 | PENDING | Qualify an additional requested APM adapter | T-645; APM input/version | RS-01 / D17 APM: T-645; select additional APM vendor/export/version and permitted sanitized input after existing-importer review; preserve common AnalysisResult and explicit alignment/coverage limits. Registered from the 2026-11-13 comprehensive review; implementation/qualification remains open. |
| T-660 | P2 | PENDING | Select and qualify Q01 Langfuse/LangSmith export adapter | T-640; Q01 actual export decision | RS-01 / Q01: T-640; confirm Langfuse/LangSmith hosting, export route/schema and actual sanitized producer/version fixture; exclude bodies/secrets and verify LLMTrace consumers. Registered from the 2026-11-13 comprehensive review; implementation/qualification remains open. |
| T-661 | P2 | PENDING | Select and qualify Q04 OTel/GenAI export aliases | T-638, T-640; Q04 actual export decision | RS-01 / Q04: T-638, T-640; confirm actual OTel/GenAI export route/encoding, scope/schema URL, temporality and producer version before alias activation; test missing/zero and incompatible comparisons. Registered from the 2026-11-13 comprehensive review; implementation/qualification remains open. |
| T-662 | P3 | PENDING | Complete R20-04 natural swap/pressure qualification | T-643; authorized natural pressure/swap evidence | RS-01 / R20-04: T-643; obtain owner-authorized natural nonzero-swap/elevated-pressure capture with collector/platform version and independent values; no forced memory stress. Existing multi-load is partial evidence, raw pressure code1 is not elevated-pressure proof. Registered from the 2026-11-13 comprehensive review; implementation/qualification remains open. |
| T-628 | P1 | DONE | Implement importer, envelope, measurement and version contracts (D01) | Existing parser/profiler goldens and compatibility inventory | C1–C4, R-01–R-14 and R2 N-01–N-04 remediation implemented and locally verified: separate complete-result budgets, reclaimed job history, schema/result composition, population identity, patch compatibility, isolated detector diagnostics, bounded/deduplicated store and Load lifetime, legacy parity, executable fixture/schema assertions, export preservation and checked profiler TS facade. R2 adds source-independent compatibility with separate baseline/target references, removes unsafe public shared-handle deletion, bounds availability projection and corrects max_lines exact EOF. R3 independently confirmed N-01–N-04 resolved, no P1/P2, and explicitly permitted DONE. R3-01–R3-03 are locally addressed: stable refusal codes/JSON causes, declared identity basis, whitespace-only suffix coverage. Original R3 verdict remains CONDITIONAL; these final P3 fixes have no independent re-review. R3 evidence: docs/review/evidence/2026-11-11-t628-r3/local.json. R2 evidence: docs/review/evidence/2026-11-11-t628-r2/local.json. Earlier evidence: docs/review/evidence/2026-11-11-t628-remediation/local.json. Actual new WebView IPC/UI, Windows native execution and hosted CI remain unexecuted. |
| T-629 | P1 | DONE | Migrate profiles to ProfileSet with legacy facades (D02) | T-628 / R-01 scoped condition met | Implemented from 63d3b6a: exact ProfileSet/identity/clock/quality and explicit selections; common trie and legacy compatibility catalog; canonical profile_evidence 0.2.0 importer/analyzer with disjoint version readers; exact tree/diff/drilldown and lossless tick-scaled pprof; CLI and opaque Wails handle paths; immutable saved-model validation and legacy normalization wrapper. Existing goldens and five baseline CLI comparisons pass. Local race: 95 packages ok / 5 without tests; frontend/bindings, Mac package/signature, Windows amd64/386 cross-vet/build and desktop guard pass. Independent review, actual new WebView UI and Windows native execution are not claimed. R4-01–R4-05 locally remediated: chunked 32 MiB transport with 54,001-node scale regression, exact tree-byte accounting and resource details; quality values preserved separately from comparison categories, entity mapping refusal codes, content-validation cache and checked quality copy; 64 KiB detection boundary documented. Independent R2 confirmed all R4 findings resolved with no P1/P2 and explicitly allowed DONE. R5-01 typed wire budgets/4 MiB boundary are locally fixed; R5-02 capture-key policy is specified for T-634/T-635 with current strict comparison retained. Original verdict remains CONDITIONAL; final R5 changes have no independent re-review. R2 evidence: docs/review/evidence/2026-11-11-t629-r2/local.json. Earlier review evidence: docs/review/evidence/2026-11-11-t629-review/local.json; original implementation evidence: docs/review/evidence/2026-11-11-t629/local.json. Producer corrections remain T-634; generic views T-631. |
| T-630 | P1 | DONE | Implement case lifecycle, capture context and safe evidence intake (D03) | T-628 | Deliver versioned capture/case manifests, clock/entity provenance, vendor-neutral device identity, archive validation, encoding/source-position preservation, redaction and scoped deletion/restore. Cover CP949/EUC-KR and baseline/stale references. Add R-08 acceptance: durable saved-case references, cache migration/recovery, reference ownership before deduplicated-handle release, user retention/deletion/restore and index lifecycle. (§3, §11.6, §13, §17.1; F-07). C1–C4 locally implemented and verified; 96 Go packages/CLI/bridge/state/build checks pass. The original independent verdict is CONDITIONAL; actual case-lifecycle WebView UI and Windows runtime remain separate; evidence: docs/review/evidence/2026-11-11-t630/local.json. Independent review at 82bc6a8 returned CONDITIONAL: R6-01/R6-02 (P2) and R6-03–R6-07 (P3) are locally remediated: filesystem-identity export guards; explicit owner-aware cleanup and bounded owned reopen reserve; idempotent save; tree-token corrupt deletion; actionable migration recovery; compact cached source pages with prefix disclosure; independent display-name rules. Full local Go race/vet/build, actual CLI, bridge/state/build/package and Windows cross-vet pass. Independent R2 verified R6-01–R6-07 resolved and explicitly permitted DONE. R7-01 is locally fixed; final P3 has no independent re-review. R6 evidence: docs/review/evidence/2026-11-11-t630-r6/local.json. |
| T-631 | P1 | DONE | Add result capability providers and generic consumers (D04) | T-628 | Compiled, payload-validated summary/table/detail/timeline/SLI/compare provider contracts; bounded fallback, exact text cells, typed comparison refusals and declared identity. Generic CLI/Wails services and Workspace import/job/selection/cancel/load/query/compare/export/reopen UI implemented. Full Go race/vet/build, bindings, state/build and scoped real macOS WebView import/detail/comparison/fallback/pagination/Workspace checks pass; final focused checks cover output-budget hardening. Independent review remains CONDITIONAL and explicitly allows DONE after R8-01 remediation. R8-01–R8-04 are now locally fixed and verified: linear reference sets, private digest-bound bounded projection/index cache, escaped display controls with exact exports, typed result_unsupported refusals and disabled incompatible targets. Full Go race/vet/build, 8k/16k semantic regressions, 100k-row cache/lifetime/tamper/cancellation tests, actual CLI and scoped native Mac UI pass. Final fixes have no independent re-review; Windows-PC generic UI/runtime evidence remains separate. R8 evidence: docs/review/evidence/2026-11-12-t631-r8/local.json. Evidence: docs/review/evidence/2026-11-11-t631/local.json. |
| T-632 | P1 | DONE | Deliver declarative packs and verifiable offline distribution (D05) | T-628 | Locally implemented exact-byte Ed25519 data-only packs, closed bounded mapping interpreter, conformance/hash/compatibility gates, atomic activation/rollback, sequential offline trust rotation/revocation, explicit local user/case presets and alias choices, immutable job/catalog/result provenance, CLI/Wails/Settings management. Mandatory pinned standalone fixture ZIP covers all 3 registered importers and gates release; docs/fonts/notices are local and hashed. New disconnected Linux CLI process trees show zero IPv4/IPv6 syscalls with failed-network positive control. Scoped native macOS UI observes offline/font/activation revisions. Fixed WebView2 path/PE/inventory preflight implemented; actual Windows runtime/browser observation remains unexecuted here. Measured pure-Go SQLite spike retains JSON/index production storage; no new production SQLite dependency. Independent review remains CONDITIONAL and explicitly permits DONE after R9-01 remediation. R9-01–R9-04 are now locally fixed and verified: recovery survives revoked/missing bootstrap trust, dormant packs/dependents are isolated, exact-byte impact review is bound to state revision, application activation checks all active scopes atomically, a bounded verified cache still hashes each access, and new mapped metric names reject shared C0/C1/BiDi controls while old saved results remain readable. Full race/CLI/bridge/state/build/package, Windows cross-vet/build and disconnected CLI observation pass. New native Mac UI was blocked by screen lock; Windows native execution and independent final re-review are not claimed. R9 evidence: docs/review/evidence/2026-11-12-t632-r9/local.json. Evidence: docs/review/evidence/2026-11-11-t632/local.json; details docs/en/OFFLINE_PACKS.md and docs/ko/OFFLINE_PACKS.md. |
| T-633 | P1 | DONE | Guide input discovery, manual collection and local conversion (D06) | T-630, T-632 | Show detected formats, missing context and supported capabilities; ship manual collection recipes/sidecars separately from data packs. Bound existing converter execution, cancellation and diagnostics, including the JDK JFR path; no automatic attach/download (§4.6, §15.1, §17.4; G-03/G-07). Locally implemented snapshot-backed 64 KiB hints and explicit file-bound sidecar review/drafts; Workspace capabilities/missing-context guidance, standalone manual ZIP, fixed-argv installed-JDK conversion with streaming/time/concurrency limits, process-tree cancellation, safe refusals and exact provenance. CLI/bridge/state/build/race and actual installed Temurin 11/17/21 synthetic JFR conversion plus both legacy analysis paths pass. Offline conversion remains denied; disconnected CLI inspection/kit and denied conversion are observed separately from desktop/Windows. Original independent review is CONDITIONAL and explicitly permits DONE after R10-01 remediation. R10-01–R10-05 are locally fixed: 512 MiB independent expanded JSON ceiling, streaming validation/hash/publication, actionable size/I/O errors, preflighted output pair, non-fatal discarded stderr, honest declared_unverified tool provenance (including legacy metadata), and consistent EN/KO docs. A 10,521,805-byte native JFR converts to 529,956,916-byte JSON and passes legacy binary analysis. Full race plus focused legacy identity regression, CLI/bridge/state/build/package, Windows cross-vet/build and offline observation pass. No final independent re-review, broad producer qualification or native decoder is claimed. R10 evidence: docs/review/evidence/2026-11-12-t633-r10/local.json. Native Mac UI could not be observed while screen locked; native Windows remains separate. Evidence: docs/review/evidence/2026-11-12-t633/local.json. |
| T-634 | P1 | DONE | Correct py-spy, Speedscope and pprof measurement semantics (D07) | T-629, T-631 | Preserve fractional/zero weights, evented duration, every profile/worker and pprof value/unit choices through import, comparison and export. R5-02 acceptance: producer/version/format/metric-bound trusted capture-option key policy; keep unknown/no-policy options strict, retain sanitized originals and incidental-change disclosures, and test semantic mismatch/presence/null/zero/forged whitelists plus saved/CLI/Wails parity before each producer promotion. Do not globally ignore PID/duration or bypass entity/clock gates. Validate actual producers and update the support matrix; create new semantic revisions rather than relabel old results (§2.1, §4, §18). Locally implemented separate exact original-source CLI/Wails normalization and explicit selection; source extension 1.0.0 / result 0.3.0, disjoint importer, all sampled profiles/weights, evented exclusive elapsed intervals and unattributed gaps, pprof value/unit/label/inline preservation and exact tick export. Actual py-spy 0.4.1 subprocess and Go go1.26.10 heap fixtures are digest pinned; compiled capture policy preserves strict unknown/missing/null/zero semantics and entity/clock gates with original-options/incidental-change disclosure. Full race and final focused regressions, CLI/bridge/state/build/package and Windows cross-vet/build pass. Independent review and native Windows/UI remain separate; generic scalar/baseline policy adoption remains T-635. Evidence: docs/review/evidence/2026-11-12-t634/local.json. Independent R11 verdict CONDITIONAL explicitly allows DONE after R11-01 remediation. R11-01–R11-04 are now locally fixed: new normalizer 1.1.0 / payload 0.4.0 original type/unit identity and policy-independent comparison; original 0.3.0 snapshot hashes/read/query/export preserved with reimport comparison gate; source policy basis/warning, numeric observation budgets and none generic-count provenance. Actual four-column Go fixture and no-policy alloc_space vs inuse_space rejection, old-snapshot regression and full race/CLI/build/cross-validation pass. No new independent PASS or native UI/Windows claim. Follow-up evidence: docs/review/evidence/2026-11-12-t635/local.json. |
| T-635 | P1 | DONE | Unify drilldown, evidence indexing, case search and baselines (D08) | T-629, T-630, T-631 | Keep all views on the same selection/denominator; provide bounded revision-bound queries, source links, cancellation and case-wide search with coverage. Distinguish declared identity from original reimport verification; bind originals/options/result digests to durable baseline refs before asserting verified status (R3-02). Consume only qualified capture-key policy revisions from T-634 (R5-02); pin policy/semantic projection and both original sanitized option refs, preserve old v2 hashes, and require reimport for new policy semantics. Validate immutable baseline reuse, stale cursors, missing originals and index rebuilds (§6, §13.4, §16.3–16.4; G-05). Locally implemented canonical profile query 1.0.0 with selection/result/query-bound cursor, exact denominator, top/caller/callee/evidence/time-window and paged flamegraph positions independent of display budget; immutable selected-result materialization. Case search reuses bounded source coordinates and canonical typed observations/time windows, masks snippets before durable disposable indexing, discloses prefix/budget/missing coverage and invalidates changed sources/results/cursors. Immutable baseline bindings pin result/source/options/model/selection/policy/capture digests, replay both originals at query start, reuse a process-local validated diff for two minutes, and recheck original/result bytes plus full case bindings on every page, reject edited/absent/stale inputs and preserve declared identity; complete explicit entity and capture-context mappings cannot bypass strict clock/metric/quality/unknown conditions. CLI, opaque Wails and Workspace consumers are implemented. Full 1493-test race plus CLI-refusal regression, actual producer type check, bridge/state/build/package and Windows cross-validation pass; native UI/Windows and independent review are separate. Evidence: docs/review/evidence/2026-11-12-t635/local.json. R12-01 cache, R12-02 self/inclusive query sort, R12-03 index-mask 0.2.0 and R12-04 numeric save-limit guidance are locally fixed/tested. DONE under explicit R12 nonblocking-queue closure permission; original CONDITIONAL preserved. Current evidence: docs/review/evidence/2026-11-12-t638/local.json. |
| T-636 | P1 | DONE | Copy reproducible report images and tables from the desktop (D09) | T-632, T-634, T-635 | Implemented fixed separate desktop WebView rendering, pinned view/spec/font/query/quality, PNG/self-contained SVG, native PNG/DIBV5/TIFF and HTML/TSV clipboard formats, split images, immutable visual library/ZIP and visual Report Pack. CLI spec/validation/saved-byte exports and explicit new-render/clipboard refusals. Local contract, Chromium raster/DPR, race/vet/frontend/package and Windows cross-build checks pass. DONE under R14 explicit P2-remediation closure permission: R14-01 protects actual spreadsheet formula/numeric/date risks, preserves ordinary paths/URLs/headers, distinguishes HTML/TSV and discloses changed-cell counts. Local regression/full verification passes. R14-02–R14-07 remain queued; original verdict CONDITIONAL. Native Office/Windows/WebView acceptance remains separate; no native paste claim. Evidence: docs/review/evidence/2026-11-12-t637/local.json and prior t636/local.json; docs/en/VISUAL_ARTIFACTS.md.  2026-11-13 batch: follow-ups locally remediated except R20-04 EVIDENCE_PARTIAL; see docs/review/evidence/2026-11-13-review-followups/local.json. Original verdict/native boundaries preserved. |
| T-637 | P1 | DONE | Add perf events and constrained BPF evidence adapters (D18) | T-629, T-630, T-635 | Implemented bounded SystemEvidence 1.1.0 / system_evidence 0.2.0 with perf text, explicit event JSONL and pinned bpftrace histogram JSONL; exact timestamps/periods/bounds/counts, loss/clock/lifetime-aware nullable scheduler intervals, explicit ProfileSet projection, digest-bound histogram paging and verified case-original coordinates. CLI, Wails bridge and Workspace controls reuse existing providers/report paths; aggregate histograms never become request timelines. Actual perf 6.1.190 text conversion of a synthetic perf.data fixture, 10k/20k scale, full race/vet/frontend/package, Windows cross-builds, isolated React mock IPC and network-none CLI checks pass. Independent review, native Linux sampling/scheduler/BPF attachment and Wails/Windows execution remain separate. R15-01 keyed map and R15-02 shared frame/stack storage locally fixed/tested: model 1.1.0/result 0.2.0, old 1.0.0/0.1.0 saved readers retained, actual 20k/24k 25-frame CLI normalize/import/project/tree pass. R15-06 decimal table protection corrected with old receipt policy reader. DONE under original explicit R15 P2-remediation closure permission; R15-03–R15-05 queued and CONDITIONAL/native scope preserved. Evidence: docs/review/evidence/2026-11-12-t639/local.json and prior t637/local.json; docs/en/SYSTEM_EVIDENCE.md.  2026-11-13 batch: follow-ups locally remediated except R20-04 EVIDENCE_PARTIAL; see docs/review/evidence/2026-11-13-review-followups/local.json. Original verdict/native boundaries preserved. |
| T-638 | P1 | DONE | Implement metric history and OTLP histogram semantics (D10) | T-628, T-630, T-631; Q04 gates actual OTel export/GenAI aliases | DONE under R13 explicit after-P2-remediation closure permission. MetricHistory 1.0.0 / payload 0.2.0 stores selected-series model plus minimal point/time projections, retains interval context and 0.1.0 saved readers. Actual 100k-point/10-series CLI import/readback fits 64 MiB; one-series storage reduction and compact JSON/size errors verified. Original CONDITIONAL preserved; R13-02 is locally fixed/verified with owned warm model/interval indexes, byte rehash every request, bounded LRU/expiry and measured100k warm reuse. R13-03/05/06/07 are locally fixed with explicit normalizer compatibility, interval CSV and searchable paged pickers; R13-04 now supplies bounded source codes/paths/rules while retaining whole-file refusal. Q04/user export/GenAI and native acceptance stay separate. Cache evidence: docs/review/evidence/2026-11-12-metric-cache/local.json. Evidence: docs/review/evidence/2026-11-12-t636/local.json and prior t638/local.json. |
| T-639 | P1 | DONE | Analyze NVIDIA and vLLM through neutral device/serving contracts (D11) | T-632, T-638; Q03 gates version-specific vLLM support | R16 original CONDITIONAL explicitly permits closure after P2 remediation. R16-01–R16-06 locally fixed/verified: declared range metadata in InferenceEvidence 1.1.0 without original identity mutation, input-only missing event ID generation, CSV [N/A]/trailing blanks/field errors, retained-series selection with full interval context, unique-stack budget guidance, distinct-boundary TPOT in payload 0.2.0. Legacy model 1.0.0/result 0.1.0 readers preserved. Original verdict and native GPU/vLLM/Windows gates remain. Evidence: docs/review/evidence/2026-11-12-t640/local.json; docs/en/INFERENCE_EVIDENCE.md. |
| T-640 | P1 | DONE | Normalize LLM application traces, gateway attempts and lifecycle (D12) | T-630, T-631, T-638; Q01 gates Langfuse/LangSmith adapters | Implemented limited trace.llm-json / LLMTrace 1.0.0 / existing trace_import 0.3.0 with llm_core, actual OTel SDK carrier and selected semantic profiles. Body-free normalization before case copy/indexing; parent/link and duplicate/missing/cyclic graphs, exact observed child-union duration, explicit logical/client-server attempt accounting, retry/fallback predecessor audit, token boundaries and lifecycle populations. Bounded CLI/Wails/Workspace query, existing providers/timeline/exports/cases reused; legacy trace 0.2.0 retained. Local race/vet/frontend/package/cross-build/CLI/browser mock/offline checks pass; independent/native Windows/vendor export evidence remains separate. Evidence: docs/review/evidence/2026-11-12-t640/local.json; docs/en/LLM_TRACE_EVIDENCE.md. R17 closure: exact rational inference 0.3.0 and R17-02–04 locally verified; R17-05 queued, original CONDITIONAL preserved. Evidence: docs/review/evidence/2026-11-12-t641/local.json.  2026-11-13 batch: follow-ups locally remediated except R20-04 EVIDENCE_PARTIAL; see docs/review/evidence/2026-11-13-review-followups/local.json. Original verdict/native boundaries preserved. |
| T-641 | P1 | DONE | Deepen Python, Node, Rust and existing runtime/log diagnostics (D13) | T-629, T-630, T-635 | Implemented bounded runtime_evidence 0.1.0 / runtime_core 1.0.0 with pinned Python/Node/Rust API fixtures, explicit CPU/native-thread process/lifetime/window context, separate observed/inferred states, conservative multi-dump identity, live allocation profile reuse, GC interval union and PostgreSQL estimate/actual per-loop plans. CLI/Wails/Workspace pages and common providers/exports/cases reused. Local verification recorded in docs/review/evidence/2026-11-12-t641/local.json; independent/native producer/Windows acceptance remains separate. R18 closure: actual Python 3.14 named headers/empty-source refusal, CLI/desktop correlation sidecar and isolation warnings, GC event coordinates and decimal nid are locally verified. Original CONDITIONAL preserved. Evidence: docs/review/evidence/2026-11-12-t642/local.json. |
| T-642 | P2 | DONE | Import workload results and text crash context for comparisons (D14) | T-630, T-635, T-638 | Implemented execution_evidence 0.1.0 / execution_core 1.0.0 inside common AnalysisResult. Pinned k6/LLMPerf, structural JTL and actual sanitized HotSpot crash inputs normalize before common providers/exports/cases; ordered baseline comparison requires matching known workload/client/target/arrival/concurrency/token/warmup conditions and preserves unavailable/zero values. Bounded CLI/Wails/Workspace pages, source pinning and comparison-page provenance. No load generator/core/heap engine or causal/root-cause verdict. Evidence: docs/review/evidence/2026-11-12-t642/local.json; docs/en/EXECUTION_EVIDENCE.md. R19-01: exact large-JTL distributions 1.1.0 / new projection 0.2.0, 60k independent-stat/bounded tests and common consumers verified; DONE under original explicit closure permission, original CONDITIONAL preserved; R19-02–04 queued. Evidence: docs/review/evidence/2026-11-12-t643/local.json.  2026-11-13 batch: follow-ups locally remediated except R20-04 EVIDENCE_PARTIAL; see docs/review/evidence/2026-11-13-review-followups/local.json. Original verdict/native boundaries preserved. |
| T-643 | P2 | DONE | Implement the selected Apple Silicon evidence adapter (D15) | T-630, T-638; Q02 and actual Apple input fixtures | Actual Ollama 0.40.16 / Apple M5 Max fixture, MetricHistory/AnalysisResult and manual read-only collection. R20-01 locally fixed/verified: collector 1.1.0 selected-match/excluded counts and selector hash; legacy 1.0 saved normalization unchanged with read-time unknown-coverage notice. Actual matched multi-load, unmatched selector and empty states verified, restoring unloaded baseline. DONE under original R20 explicit P2-remediation permission; CONDITIONAL preserved, R20-02–04 queued (multi-load partly covered). Evidence: docs/review/evidence/2026-11-12-t644/local.json; docs/en/APPLE_SILICON_EVIDENCE.md.  2026-11-13 batch: follow-ups locally remediated except R20-04 EVIDENCE_PARTIAL; see docs/review/evidence/2026-11-13-review-followups/local.json. Original verdict/native boundaries preserved. |
| T-644 | P2 | DONE | Deliver the bounded first batch of PyTorch/Nsight execution detail (D16) | T-635, T-638, T-639 | Implemented gpu_execution_core1.0.0 / trace_import0.4.0 with actual PyTorch2.15 CPU/Gloo two-rank traces and NVIDIA training report exported by official Nsight2024.2.3.38/schema3.11.1. Exact ns, isolated source/rank clocks, CPU inclusive/self, separate kernel sum/union and scoped flow/correlation; unresolved links remain explicit. Bounded SQLite/JSON/summary CSV intake, paged CLI/Wails/React detail, selected-source SVG, common consumers/cases. Actual 63-event corpus and separate 400-entry summary; full 118-package race plus final focused checks, clean-install offline and browser/mock-IPC checks pass. DONE under original R21 explicit P2-remediation permission: raw64MiB pre-window restriction documented, manual bounded SQLite subset helper verified on a114.9MB padded actual export to20 whole intervals/24KiB with original/subset hashes. Ignored PyTorch metadata no longer consumes retained event budget; schema error and at_least overflow are explicit. R21-01/03 locally fixed; R21-02/04 queued, CONDITIONAL preserved. Final fixes have no independent PASS; no native PyTorch CUDA/NCCL, causal critical path, hardware utilization or local NVIDIA/Windows runtime claim. Evidence: docs/review/evidence/2026-11-12-t645/local.json; initial docs/review/evidence/2026-11-12-t644/local.json; docs/en/GPU_EXECUTION_EVIDENCE.md.  2026-11-13 batch: follow-ups locally remediated except R20-04 EVIDENCE_PARTIAL; see docs/review/evidence/2026-11-13-review-followups/local.json. Original verdict/native boundaries preserved. |
| T-645 | P2 | DONE | Qualify the bounded first DB/OS snapshot adapter batch (D17) | T-628, T-629, T-630, T-631, T-632; User-activated bounded batch, actual sanitized inputs and selected versions | First actual batch implemented: PostgreSQL16.14 pg_stat_database15 series/75 points with unknown/reset epochs and delayed counter semantics; Linux iproute2-6.1.0/kernel7.0.14-linuxkit ss summary21 gauges/63 points, actual established0/8/0 and shared-kernel scope disclosure. Closed4MiB/256-snapshot adapters reuse MetricHistory1.0.0/metrics_timeseries0.2.0 and common source pinning, CLI/Wails/React queries/exports/cases; importer never collects or runs SQL/tools. Shared schema/provenance/hash corpus and offline mirrors. Local full118-package race plus final affected checks, actual CLI/offline and browser/mock-IPC pass; DONE under R22 explicit no-blocker permission; R22-03/01/02 queued after older P3 work, original CONDITIONAL preserved. Actual independent PostgreSQL14 counter deltas/ss84 values and R21 remediation verified. Evidence follow-up: docs/review/evidence/2026-11-12-metric-cache/local.json. Other profile/serving/DB/OS/APM vendors and Q01/Q04 remain demand/input/version-gated, not automatically activated. Evidence: docs/review/evidence/2026-11-12-t645/local.json; docs/en/SYSTEM_DB_SNAPSHOTS.md.  2026-11-13 batch: follow-ups locally remediated except R20-04 EVIDENCE_PARTIAL; see docs/review/evidence/2026-11-13-review-followups/local.json. Original verdict/native boundaries preserved. |
| T-614 | P1 | DONE | Prevent stale derived artifacts from silently entering a current Report Pack | Existing Report Pack/timeline/SLO/service-flow contracts | Completed 2026-11-10: Report Pack rebuilds timeline, SLO metrics and service flow from current source results on every generation. Only validated saved SLO target settings are retained, including an intentional empty policy. Source replacement/removal/addition, same-ID content changes and customized threshold regressions plus frontend state/build pass. |
| T-615 | P1 | DONE | Correct Trace Import elapsed critical-path calculation | Existing Trace Import and downstream latency contracts | Completed 2026-11-10: observed ancestry-chain elapsed estimate uses validated contained intervals, with inclusive work separate. New semantics explicitly do not prove a causal critical path. Missing/skewed/duplicate/cyclic input emits null/reason and availability counts; 20k-depth iterative graph, nested/sequential/parallel/zero and downstream SLO/timeline regressions pass with Go/CLI tests and frontend state/build. |
| T-616 | P1 | DONE | Avoid double-counting one RPC's CLIENT and SERVER spans | Existing Trace Import dependency and Service Flow/SLI contracts | Completed 2026-11-10: one-to-one explicit CLIENT/SERVER pairs use one logical call, client elapsed time and either-side error; single-sided exports, cross-trace identities, separate retry pairs and multiple server attempts remain distinct. SERVER remote peers use incoming direction. Go/CLI tests and frontend Flow/SLI counter regressions pass. |
| T-617 | P1 | DONE | Preserve missing access-log response time as unavailable rather than zero | Access-log parser/analyzer and SLI/SLO contracts | Completed 2026-11-10: all supported parsers preserve response-time availability, including true zero; unknown samples are excluded from total/static/API/URL/minute/route/edge latency while full request/error/byte counts remain. Null empty stats, measured/unknown counts, warning finding, sample coverage UI and measured-only Flow weighting propagate to SLI/SLO; targets without evidence are explicitly not assessed. Parser/analyzer/CLI and frontend state/build regressions pass. |
| T-618 | P2 | DONE | Disclose Incident Timeline truncation and qualify severity/time summaries | Existing Incident Timeline/Report Pack contracts | Completed 2026-11-10: 120/source and 1000/session row caps remain, with deterministic documented selection, total/emitted/omitted and per-source/severity loss counts. Severity/time/ranged/correlation summaries use all candidates; groups/narrative explicitly remain retained-only. Page/JSON/Report Pack HTML disclose partial output and omitted critical events. Both-cap, late-critical, empty-input and export regressions plus frontend state/build pass. |
| T-620 | P1 | DONE | Measure all 21 views at the default window size | — | Completed same native 1280x800/100%/Korean 21-route original baseline at d98a958 using native sidebar and identical sanitized deep-profile, GC-pressure and 30-request HAR inputs. Original profiler first data y=484 (29.6% union); GC Events y=410, six full rows; HTTP zero full request rows. Evidence: docs/review/evidence/2026-11-10-layout/mac-original-baseline.json. |
| T-621 | P1 | DONE | Move global navigation to native menus | T-620 | Actual macOS native menus reached all 21 routes; Open dialog, Cmd+, locale/theme and About verified. Full-width flex regression fixed; menu-context and build checks pass. Windows acceptance remains in T-627. |
| T-622 | P1 | DONE | Compact shared input controls and help | T-620 | Completed Mac native long Korean filename (182 UTF-8 bytes, 488-character path), 1024x768/125% viewport, pointer enter/transfer/leave, click pin/outside/Escape, Option-Tab/Enter/Escape and drawer close-focus restoration. Fixed unlayered flex:none overriding the file dock and macOS document-focus restoration. Evidence: mac-detail-T622.json. |
| T-623 | P1 | DONE | Prioritize the HTTP grid and inspector | T-621, T-622 | Completed 2026-11-11: Mac imported HAR density/ten-tab acceptance plus native Windows production live capture. Two real PID/start-time clients produced ten complete persisted exchanges; process filter, selected Headers/Body during arrivals, disabled follow, native menu navigation/reentry and F2 stop/finalized detail passed. Page-local selection/filter/follow reset on remount; session identity and all ten rows restore. Evidence: docs/review/evidence/2026-11-11-windows/README.md. Complete Windows menu/OS scaling remains T-627. |
| T-624 | P1 | DONE | Prioritize profiler and GC results | T-621, T-622 | Completed Mac native detailed acceptance: deep profiler first data y=172, 71.2% visible-data union, resized frame zoom/reset and matched/empty drilldown. GC first data y=147, 72.5%, 26 full preview rows; no-clock and all seven tabs verified. Compact 32-row preview exposes total count and full 15-column Events retains explicit 50/60 top-N count. Evidence: mac-detail-T624.json. |
| T-625 | P2 | DONE | Convert nine remaining analysis views | T-621, T-622 | Completed Mac native matrix: all nine analyzers, normal and conditional tabs, sparse/empty states, malformed input and recovery, comparison and detail drawers. Fixed async completion-before-ID race with repeated native completion and error-retry checks. Shared long-path/keyboard/resize evidence is T-622; sparse fixtures do not prove a universal 70% area ratio. Evidence: mac-detail-T625.json and mac-detail-async.json. |
| T-626 | P2 | DONE | Convert eight workspace views and Settings | T-621, T-622 | Completed Mac eight-route/result handoff matrix, all eight chart templates, Canvas/SVG cross-format native PNG/SVG/CSV writes, Export Center HTML/PPTX/JSON/CSV/directory/cancel/error recovery and invalid Settings drafts with restored defaults. Chart Studio first data y=184 and 72.3% visible-data union at observed 1281x801; forms/sparse results excluded with reasons. Evidence: mac-detail-T626.json and mac-detail-exports.json. |
| T-627 | P2 | REVIEW | Verify all views in native desktop windows | T-623–T-626 | Completed Mac original/current 21-route baseline and detailed functional/layout checks. Windows production native menus reached all 21 empty/form routes on 2026-11-12; locale switching, synthetic saved HTTP Headers/Body and menu reentry passed. Full populated/conditional/error, small-window, exact viewport/area and OS DPI acceptance remains; mixed MSA labels and raw dialog-cancel text are recorded. Evidence: docs/review/evidence/2026-11-12-windows/README.md; earlier Mac mac-detail-validation.json. |
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
| T-608 | P2 | DONE | Read and write legacy .halog files | T-607 | Completed 2026-11-11: bounded format-6/schema-7 reader/writer; private 675-entry/35,463,916-byte sample re-encodes byte-identically. Original 7.6.4.508 COM validates decoded metadata/timing and 669 request/378 response text bodies. Desktop/CLI/automation apply redaction and export reconstructed retained-text streams. Native reference reopening/re-save/reimport, large chunk boundaries, duplicate headers, Korean text/metadata, redirect, failed/pending requests, malformed input/resource limits, x86/x64 Go/frontend/GUI validation pass. ANSI codepage/host timezone and unavailable binary/encoded content limits are explicit. Evidence: docs/review/evidence/2026-11-11-windows/HALOG_CODEC.md. |
| T-609 | P2 | DONE | Integrate row hints, status help, stars and comments | T-594, T-607 | Added request-ID hints/status help, explicit evidence links, star/comment columns and starred-only filtering. Live annotations use an atomic bounded sidecar, verified IDs, redacted comments and Clear reset; finalized/native JSON/XML/ArchScope HAR preserve notes, while opened-file edits stay explicit export overrides. ID sorting, service and reopen regressions plus frontend state/build pass; native visual acceptance remains unverified. |
| T-610 | P2 | DONE | Add the Stand-alone HTTP Request Builder | T-594 | Desktop Request Builder supports fresh requests, selected-row copies and a fixed drag/drop target, editable headers/raw/form/multipart files, explicit send/cancel and separate response inspection. Local raw/form/multipart/redirect/timeout/TLS/gzip fixtures, race detector, browser mouse copy/edit/send/drag with original-row preservation, Go suite/vet/build and frontend state/build pass; native Windows acceptance remains T-613. |
| T-611 | P3 | REVIEW | Support Stand-alone automation and desktop launch switches | T-596, T-607 | Physical retention, pre-storage admission cancellation, owned custom-IID registration, nested normalized filters and common-engine soak tooling implemented. Mac and hosted race/vet/frontend/package plus both GUI cross-builds pass; amd64-race/386 native COM, including custom-IID RPC and explicit registration replacement/cleanup, pass. Current vendor early-bound compatibility, unobserved native filter/cache-origin evidence and foreign-process in-memory session-cookie integration remain. Follow-up hosted evidence: docs/review/evidence/2026-11-11-five-followups/ci.json. |
| T-612 | P3 | DONE | Restore legacy content inspectors | T-606 | Added bounded inert AMF0/AMF3 and remoting-packet readers, ViewState v1 token trees/text, references/traits/vectors/dictionaries, form/HTML chunk extraction and explicit invalid/limited/unsupported states. Classes/converters/binary objects are never instantiated; MAC/encryption remain unverified, opaque bytes omitted and secret trees/raw ViewState sources masked. Capture/HAR/Builder/portable paths, known-byte fixtures and 15-second fuzz pass; native/reference-EXE visual interoperability remains T-613. |
| T-613 | P3 | PENDING | Verify complete Stand-alone parity on Windows | T-601, T-602, T-603, T-604, T-605, T-606, T-607, T-608, T-609, T-610, T-611, T-612 | Windows x86/x64 production builds and 15-second startup/control intervals pass. Four of sixteen parity-contract cases pass per architecture; aggregate verdict remains UNVERIFIED. Native live/process/reentry and finalized Headers pass; latest native-host service/proxy soak passed 603.731s, 172 cycles/44,720 requests with at most 64 retained rows. T-608 native codec round trips are complete. Multi-hour desktop soak, full native feature matrix, transparent/service reach and remaining vendor automation remain open. Evidence: docs/review/evidence/2026-11-12-windows/README.md; earlier native live evidence: 2026-11-11-windows. |
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

### Mac detailed acceptance follow-up: original baseline

The follow-up user request enabled the separate original-UI app. All 21 native
sidebar routes were measured at 1280x800, Korean, 100% WebView scale. Populated
original profiler, GC Events and HTTP HAR use the same sanitized inputs as the
improved app. Original profiler first-data y=484 and union ratio 0.296; original
GC Events first-data y=410 with six full rows; original HTTP has zero full rows.
HTTP source picking scrolls the original page, so its recordings include both
after-load and subsequent scroll observations rather than a pristine top reset.
T-620 is DONE; earlier approval-denied notes are historical, not the current blocker.

### Mac detail follow-up: fast native completion race

Reproduced a multi-thread dump stuck in Running because `engine:done` preceded
the IPC task-ID reply. Thread-dump and profiler now buffer only handshake events,
replay against the bound ID and discard cancelled/superseded starts. Mode changes,
input replacement and unmount invalidate old runs. Pure ordering/cancel/stale-ID
regressions pass. Native one/two-dump, lock-contention, three quick profile runs,
three two-dump runs, missing-file error and recovery pass without a stuck spinner.

### Mac Settings edge validation (2026-11-11)

Native negative interval and fractional Top N drafts now remain editable and are rejected at Save, before integer Go bindings. Saved defaults are normalized and storage failures no longer claim success. Original 100 ms / Top 20 values were restored and saved. State regressions and frontend build pass; T-626 final matrix is recorded in the follow-up evidence.

### Native chart file validation (2026-11-11)

Chart Studio now saves PNG/SVG/CSV through native dialogs and an explicit engine write service; both on-screen renderers support both image formats. All eight chart templates produced valid SVG/CSV, both renderers produced PNG/SVG, and the profiler PNG was decoded. Export Center HTML/PPTX/JSON/CSV, directory output, cancel and write-error recovery were exercised. Artifact checksums and structural checks: mac-detail-exports.json. Go export tests, full Go/vet and frontend tests/build pass.

### HTTP density and capture order follow-up (2026-11-11)

Combined collapsed file/filter controls leave the grid and selected inspector visible. Default detail height is 184px with double-click/Home reset. Missing HAR byte counts no longer display negative sizes. HAR request sequence now follows global 1-based capture order while retaining separate connection-reuse timing. Parser/analyzer and state regressions pass; production Wails native import confirms increasing request numbers and selected Headers. Evidence: mac-detail-T623.json.

### Profiler and GC detailed acceptance (2026-11-11)

T-624 is DONE. Recent/progress tools share the input row, duplicate plot headings are removed and drilldown places its graph before composition. Empty matching results have specific recovery text. The GC summary uses a compact preview with full detail retained on Events; preview and retained/total counts are explicit. Native 1280x800 results exceed the populated-view 70% target (profiler 71.2%, GC 72.5%). No-clock, sparse and all result-tab cases are recorded separately. Evidence: mac-detail-T624.json and mac-detail-geometry.json.

### Nine-analyzer detailed acceptance (2026-11-11)

T-625 is DONE for implementation and the Mac native functional matrix. All nine routes and their populated/conditional tabs were exercised, including multi-dump results, native-memory JFR, exception detail, imported-browser provenance and invalid-to-valid recovery. Sparse fixture geometry is not reported as a filled-area pass. Windows menu/scale acceptance remains T-627.

### Workspace and Settings detailed acceptance (2026-11-11)

T-626 is DONE for implementation and Mac native acceptance. All eight workspace routes, result handoffs, chart templates/renderers/formats and Settings edge cases were exercised. Native file outputs were checked structurally; file hashes, row counts and dimensions are recorded without committing generated binaries. Forms and sparse results have explicit geometry exclusions.

### macOS runner migration (2026-11-11)

Moved Engine Native test/package and Release arm64 packaging from retired macos-14 to macos-15. GitHub retired all macOS 14 Actions images on 2026-11-02: https://github.blog/changelog/2026-10-01-github-actions-macos-14-runner-image-retirement/. Existing architecture, tests, signature checks and release conditions are retained. Workflow structure checked locally; actual hosted execution follows the final push and is not inferred from the edit.

### T-611 filter and retained automation view (2026-11-11)

Implemented bounded original TPF0 preset reading, FilteredCount/FilteredItem and
entry IsFiltered; 16/17 official presets parse, with cache-origin filtering
explicitly unavailable because source evidence is absent. MaxLogSize counts
latest rows in the automation/SaveLog view (0..100000); evicted IDs do not return
when the limit rises. Durable session evidence/byte caps stay independent.
StillCaptureDataWhenHide pauses/resumes only its own automatic visibility pause.
Common/race tests and Windows amd64/386 test cross-builds pass. Native Windows
reference verification remains open; T-611 stays REVIEW with its other COM and
foreign-process cookie requirements. Paired SYSTEM_HTTP_CAPTURE guides document
the supported encoding and limits.

### T-611 declared dual ABI and ITypeInfo (2026-11-11)

Implemented custom-IID/inherited vtable shapes and HRESULT/out-result conversion
from the factual 28-interface/225-method declarations, with native same-user gates.
Added in-memory dual type descriptions and native ABI/metadata tests. Common
race tests and Windows amd64/386 compilation/vet pass; Windows execution, clean-
host custom-IID marshaling and current-reference early-bound interoperability
remain unverified. Original ProgIDs/CLSID registrations are untouched. T-611
stays REVIEW; event/ref-cancel and foreign-process cookie work is separate.

### T-611 connection events and cleanup commands (2026-11-11)

Added live log references and bounded connection-point subscriptions, STA
polling/new/update dispatch, by-ref discard in the automation/export view,
source-epoch checks, subscription enumeration and reference cleanup. Original
pre-storage discard semantics remain separate; durable evidence is preserved.
Single-URL cache/persistent-cookie cleanup uses validated targets and the
existing rechecked plans. Foreign-process session-cookie reset returns an
explicit unavailable error, without pretending persistent cleanup is equivalent.
Common/race and Windows x86/x64 compile/vet pass; native callback/metadata tests
and the updated typed SaveLog smoke await Windows execution. T-611 stays REVIEW.

### Mac T-611 implementation verification and native-test handoff (2026-11-11)

Full Go race/vet, frontend state/build, macOS packaging/strict signature, desktop-
only/server refusal, Windows amd64/386 test compilation/vet and both guarded
production GUI cross-builds pass. The server's own generated typelib now has a
temporary backing file for remote metadata lookup. Added a separate-process,
random-test-CLSID IDispatch/ITypeInfo test and native x86 COM CI execution. These
Windows-only tests are prepared, not executed on Mac. Hosted results follow the
final push. Evidence: docs/review/evidence/2026-11-11-mac-automation/validation.json.
T-611 stays REVIEW for actual new Windows execution and the explicitly remaining
custom marshal/reference, pre-storage cancel/physical retention, cache-origin
filter and target-process session-cookie contracts. Counts remain 210 DONE,
3 REVIEW and 1 PENDING out of 214.

### Native COM CI remediation (2026-11-11)

The new Windows execution reproduced two defects hidden by cross-builds: an
x/sys interior-SID pointer conversion tripped race/checkptr during the integrity
check, and dual type construction rejected property naming. Integrity now reads
the bounded copied SID in its original allocation; type flags are finalized
after member definitions, property-put value names are omitted, and SYS_WIN32
is used for 386. Added a dedicated amd64-race/386 COM workflow for focused native
feedback. First run 42908152263 passed both macOS jobs and GUI cross-builds but
failed Windows COM; replacement native results are required before claiming it
fixed. Existing same-user/integrity comparisons remain enforced.

Native follow-up: the focused workflow confirmed the SID copy also needs
pointer-aligned storage under amd64 race instrumentation. It now uses a
uintptr-backed buffer. Full function-name metadata includes the [out, retval]
parameter name while property-put's final value remains unnamed; repeated
Windows results are still required.

### Final hosted verification (2026-11-11)

Product commit 7709787 passes Engine Native 42909617678, native Windows COM
42909617794 and general CI 42909617793. The retired macOS runners are replaced
and both macOS test/package jobs now complete. Native amd64 race and 386 COM
suites pass real IDispatch/ITypeInfo remoting plus metadata/lifetime/event ABI
checks after correcting the reproduced token buffer and retval naming defects.
The final records are documentation-only after that product commit. T-611 remains
REVIEW for the explicitly listed source-evidence/reference/target-process gaps.
Full parity is not inferred from these fixture tests. Existing unrelated local
changes are preserved separately. Counts remain 210 DONE / 3 REVIEW / 1 PENDING.

### Five Mac follow-ups (2026-11-11)

| Order | Scope | Status | Evidence / next action |
|---:|---|---|---|
| 1 | Physical row retention | DONE | Immutable row files plus atomic selection, bounded eviction/shared-blob reclamation, crash/reopen/Clear and stale-cursor tests; real local-proxy analysis/portable export pass. Cumulative and retained counters remain distinct. |
| 2 | Pre-storage cancellation | DONE | Redacted completion admission before Append, bounded 500 ms STA queue, fail-closed discard/error/deadline and lifecycle/generation cancellation; common race and local-proxy Stop/Clear/Pause pass. Native amd64-race/386 STA discard and reentrant-stop tests pass. |
| 3 | Clean-host custom-IID / early-bound COM | DONE | Explicit per-user owned interface/proxy/type-library mappings, both target architectures, collision refusal and cleanup; separate-process typed custom-IID calls and owned-registry tests added. Mac cross-build/vet and hosted amd64-race/386 custom-IID RPC plus explicit public register/replace/unregister tests pass. |
| 4 | Extended filter engine | DONE | Explicit versioned JSON with nested AND/OR/NOT, typed numeric/text/RE2/LIKE operators, unknown-value logic, strict keys and bounded complexity. Race tests and private 16-supported/1-refused original preset regression pass. Unknown vendor group encodings remain unsupported. |
| 5 | Long-run verification automation | DONE | Cross-platform Node runner/manual macOS+Windows workflow (2s..6h), bounded real-proxy retention/admission/reopen/export loop and scoped resource metrics. Mac race soak: 120.56 s, 58 cycles, 15,080 requests, 64 retained rows; full common suite/package and GUI cross-builds pass. Native GUI/driver long-run acceptance remains T-613. |

These are T-611/T-613 implementation follow-ups, not five new registry IDs.
T-611 remains REVIEW until its remaining native/reference contracts pass.
Task 1 also exposes retention in the existing compact status line and portable
metadata. MaxLogSize changes require a stopped capture; Windows native acceptance
and original format metadata limitations remain explicit.

Follow-up verification: Mac race soak completed 58 cycles / 15,080 synthetic
requests in 120.56 seconds with a 64-row physical bound. Full Go race/vet,
frontend state/build, regenerated Wails bindings, macOS package/strict signature,
desktop-only/server refusal and guarded Windows amd64/386 GUI cross-builds pass.
No native UI or driver run is inferred. Hosted Windows COM and common-engine soak results are recorded below.
Local evidence: docs/review/evidence/2026-11-11-five-followups/local.json.

### Five-follow-up hosted acceptance (2026-11-11)

Product 3209bcc passes Engine Native 42928859923, general CI 42928859913 and
native COM 42928859864. Manual common-engine soak 42928876605 also passes:
macOS 70 cycles / 18,200 requests; Windows 37 cycles / 9,620 requests; both
retain 64 physical rows and verify cancellation/reopen/export invariants.
Test-only 4feed2d additionally passes native COM 42929282301 on amd64 race and
386, including the explicit public Register/replace/Unregister lifecycle, owned
file/registry cleanup and separate-process custom-IID typed calls (not skipped).
These do not establish current vendor-client or native GUI/driver parity.
Evidence: docs/review/evidence/2026-11-11-five-followups/ci.json. Registry remains
210 DONE / 3 REVIEW / 1 PENDING; all five implementation sub-tasks are DONE.

### T-628 implementation started (2026-11-11)

C1–C4 implementation started from eb3e782. Existing HTTP documentation changes,
local verifier and unrelated work-status notes are preserved separately and
will be restored after the scoped commit. Q01–Q04 remain undecided; T-629,
T-631 and T-634 semantic/renderer migrations are outside this change.

### T-628 implementation completed (2026-11-11)

C1–C4 and W01–W10 are implemented and verified locally from eb3e782. New code
lives in ingestion/catalog/analysis/wirejson/resultcontract; legacy profile
metadata gains RawMessage preservation without changing profile quantities.
The existing Tomcat adapter is the limited pilot. New same-family adapters need
parser factory + schema/fixture + composition registration, demonstrated without
service/CLI/UI format-switch edits. Legacy confidence, CLI aliases, profile
goldens and thread mixed-format/override tests remain intact.

Actual CLI import/reopen/export, direct Wails Go-method integration, opaque
JSON transport simulation, TS state/Report Pack source appendix and frontend
production build pass. Full Go race/vet/build, macOS package/strict signature,
desktop-only guard, Windows amd64/386 guarded GUI cross-build and full cross-vet
pass. A pre-existing large integer literal in a thread test was made int64
without changing its JSON value, removing its 386 compile failure.

Evidence levels are distinct. No new native WebView IPC/UI session, Windows
native execution or hosted CI run was performed for T-628; a new generic job
screen remains T-631. No independent review was performed or declared PASS.
Atomic publication uses a sealed result JSON revision as its marker; separate
case/index manifests and persistent cache lifecycle remain T-630. The pilot
is plain UTF-8/uncompressed with a verified probe record. ProfileSet (T-629),
profile semantic correction (T-634), generic consumers (T-631) and Q01–Q04 remain
outside this task. Implementation and conformance details are mirrored in the
EN/KO parser/data-model/evolution documents.

Shared synthetic manifest diagnostics.ingestion.foundation.v1 and measurement
schema/index were updated; bundled copies have verified hashes. Evidence:
docs/review/evidence/2026-11-11-t628/local.json. Registry totals: 211 DONE,
4 TODO, 14 PENDING, 3 REVIEW, 0 IN_PROGRESS out of 232. T-629–T-632 are ready.

T-628 supporting asset commit: 8936c2b (shared synthetic fixture/schema/index).
Binding generation now watches internal/API/module inputs, avoiding stale DTOs
when shared job/result types change outside the desktop entry-point directory.

### T-628 independent review disposition and remediation (2026-11-11)

Read the complete claude-code review of a698d26. Accept CONDITIONAL; reopen
T-628 to REVIEW and hold T-629 for R-01 verification. All findings are recorded
before archiving the original report unchanged. This is remediation, not a new
independent PASS. Existing unrelated HTTP changes remain preserved separately.

| Order | Finding | Priority | Action | State |
|---:|---|---|---|---|
| 1 | R-01 | P1 | Separate opaque/record and complete-result JSON budgets; deep profiler CLI/bridge regressions and documented byte ceilings. | LOCAL_VERIFIED |
| 2 | R-02 | P2 | Reclaim terminal job history without evicting active jobs or invalidating result revisions. | LOCAL_VERIFIED |
| 3 | R-03 | P2 | Cross-check schema roles/families/result types and result registry at composition; execute conformance samples. | LOCAL_VERIFIED |
| 4 | R-04 | P2 | Define source/selection/option/semantic population identity; compare independent imports without trusting labels. | LOCAL_VERIFIED |
| 5 | R-06, R-09, R-13 | P3 | Consistent extension patch policy; accurate retry/error classification; separate queue and execution budgets. | LOCAL_VERIFIED |
| 6 | R-07 | P3 | Isolate unrelated detector failures and surface bounded diagnostics. | LOCAL_VERIFIED |
| 7 | R-05, R-10 | P3 | Define max_lines coverage/parity and restore correlation metadata parity. | LOCAL_VERIFIED |
| 8 | R-08 | P3 | Bound/deduplicate retained results and Load work; owned staging cleanup and stale-handle lifecycle contract. | LOCAL_VERIFIED |
| 9 | R-11 | P3 | Execute manifest expectations and align shared scalar/column/collection schemas with runtime contracts. | LOCAL_VERIFIED |
| 10 | R-12, R-14 | P3 | Preserve export series on collisions; project collection availability; restore checked profiler metadata TS usage. | LOCAL_VERIFIED |

Review source: docs/review/done/2026-11-11_claude-code_T-628_implementation-review.ko.md.
Actual WebView IPC/UI, Windows native execution and hosted CI remain separate
from this Mac implementation/test work.


#### Remediation verification and disposition

All fourteen findings have scoped code changes and regression coverage. The table's
LOCAL_VERIFIED means author-run local verification, not an independent PASS. Original
review SHA-256: `18cf079c1120493f74d37db8a2ddbc2cc4054d9658b698c6ff1156b835977d78`.
R-01 now passes 256-frame actual CLI report json/html/csv/pptx/diff and evidence
exports, Service.Load/Read/Export and direct Wails LoadEvidenceResult; a 65 MiB
valid JSON regression verifies the separate legacy byte ceiling. Explicit smaller
tree/byte policies and deep opaque metadata still refuse.

| Evidence class | Final local result |
|---|---|
| Go unit/integration + existing profile golden, metadata and mixed-thread compatibility | `go test -race ./... -count=1`: 94 packages ok, 4 without tests, zero failures/races. Includes 200-job history/active preservation, composition mismatches, independent import populations, queue/execution clocks, shutdown/load/store limits and parser I/O classification. |
| CLI and bridge | Actual built CLI deep-profile reports/evidence exports pass. Direct Wails method import/load/JSON/CSV/HTML/pack/scalar checks pass. These are not actual WebView IPC execution. |
| Frontend and bindings | Forced Wails binding generation, `npm run test:state` (runtime and negative type checks), production frontend build pass; profiler/diff/drilldown metadata consumers use checked manual facades. |
| Native build/static checks | Mac `go vet ./...`, `go build ./...`, Darwin package and strict deep codesign verify pass; Windows amd64/386 cross-vet and guarded GUI-subsystem builds pass. Desktop-only/server-build guard passes. |
| Docs/assets | Paired KO/EN contracts updated. Original review archived byte-for-byte after status intake; bundled shared fixture/schema bytes match. Supporting asset commit `56255e2` updates manifest/schema 1.0.1 and ASSET_INDEX. |
| Unexecuted | New generic WebView IPC/UI flows (no T-631 screen), Windows native lock/rename/cleanup/cache behavior, hosted CI, real producer logs and long-session production workloads. Cross-build/package evidence does not substitute for these. |

R-08's minimum retention/admission/deduplication/release and owned startup cleanup
are implemented. Full durable case/reference ownership, cache clearing/recovery,
migration, user deletion/restore and indexing remain explicit T-630 acceptance.
Load joins bounded service lifetime and shutdown cancellation; a per-load renderer
cancel action remains T-631. No ProfileSet migration, generic screen, corrected
profile numerics, or Q01–Q04 product/format selection is claimed.

T-628 remains REVIEW pending independent closure; no new PASS is declared.
The original reviewer explicitly permits T-629 once R-01 is corrected and deep
profile regressions pass, so T-629 is returned to TODO without adding a new approval
condition. Current totals: 210 DONE / 4 TODO / 14 PENDING / 4 REVIEW = 232.
No push is part of this review-processing request. Unrelated HTTP work is restored
after the scoped implementation commit.

### T-629 implementation started (2026-11-11)

Work starts from 63d3b6a in the isolated codex/t629-profileset worktree while
Claude re-reviews T-628. Preserve the original checkout and unrelated HTTP work.
Implement the exact ProfileSet core, tree/selection/diff/drilldown/pprof paths and
legacy compatibility facades. Producer-specific semantic corrections remain T-634;
no independent PASS or native UI evidence is inferred.


### T-629 implementation locally completed (2026-11-11)

Implemented in isolated `codex/t629-profileset` from 63d3b6a while the original
checkout remains available to Claude's T-628 re-review. Existing unrelated HTTP
changes are excluded from this implementation and preserved on integration.
T-628's independent CONDITIONAL verdict is not upgraded here.

| Stage | Delivered behavior and verification |
|---|---|
| Model/legacy wrappers | Versioned ProfileSet with stable original frame/stack identity, exact decimal weights/scales, multiple profiles/metrics, explicit process/thread/worker and clock/quality. SourceLease-backed legacy normalization records its limitations instead of claiming corrected producer support. |
| Tree/selection | Shared generic trie under exact model operations and legacy integer/float view adapters. Recursive/colliding frames remain distinct. Known zero, unavailable selection, scalar null and timing absence/null/zero are separated; model and selections are frozen. |
| Diff/drilldown/export | Exact physical-unit arithmetic, rational percentages/shares and source/profile/producer/quality/capture/selection compatibility. Immutable drilldown stages, lossless native-int projections or refusal, and explicit int64/tick pprof with identity/provenance comments and zero records. |
| Canonical and compatibility entry points | profile.set-json/profile-set internal transport through the common catalog/job/sink/atomic store to profile_evidence 0.2.0; CLI tree/diff/drilldown/pprof/normalize-legacy and Wails JSON-text/handle methods. Legacy service switches replaced by a compatibility catalog; existing 0.1.x/profile typed numeric behavior stays unchanged. |
| Version/fidelity | Multiple disjoint readers per result type; overlapping ranges refused. 0.2.0 requires a retained valid model and recomputed known projections. Forged summaries or schema-only relabeling fail. JSON/source pack retain the model and unknown metadata; selected pprof is explicitly not a full multimetric backup. |
| Reusable assets | Shared ProfileSet schema/fixture/manifest/ASSET_INDEX at e98fd96; synthetic decimals, zero, large integer, recursion, colliding labels and multiple metrics/workers. Bundled copies match and expected counters/weights are executed. |

Final local checks:

- `go test -race ./... -count=1`: 95 packages ok, 5 without tests; zero failures/races.
  Existing profile golden, metadata roundtrip and mixed-thread tests are retained.
- Actual CLI comparison with 63d3b6a: collapsed, Jennifer, SVG, HTML and drilldown
  output equal after excluding only created_at and progress_log_path. Existing
  sanitized collapsed fixture also passes normalize-legacy → model import → exact
  result (32629) end-to-end. Benchmarks are local observations, not production
  throughput guarantees; the legacy diff allocation count remains 279 in the sample.
- Direct Wails method tests cover import, opaque tree/diff/drilldown, save/reopen and
  pprof. Large integers remain text across the new bridge. Generated bindings,
  frontend state/type checks and production build pass.
- Mac vet/build/package/strict deep signature, Windows amd64/386 cross-vet and
  guarded GUI builds, and desktop-only/server-build guard pass.
- Paired KO/EN model/parser/architecture/support documentation, registry counts,
  local link targets and bundled/shared asset hashes checked.

Unexecuted: independent T-629 review, actual new WebView IPC/UI interaction,
Windows native execution, hosted CI, and new real-producer/large-production-input
qualification. T-634 owns corrected py-spy/Speedscope/pprof producer decoding and
support claims; T-631 owns generic result screens. T-630/T-635 own broader case,
entity/time/workload baseline policy. Q01–Q04 remain undecided. Existing large
legacy paths retain a shared-trie compatibility fast path rather than materializing
a second full model; their retained decoder limitations are not called corrected.

T-629 moves to REVIEW for independent closure, not DONE/PASS. Current totals are
210 DONE / 3 TODO / 14 PENDING / 5 REVIEW / 0 IN_PROGRESS = 232 (22 open).
T-630–T-632 retain scoped readiness. Commit and push are authorized for this task.


### T-628 R2 re-review intake after T-629 integration (2026-11-11)

Read the full Claude re-review committed at 9f976b1 against 63d3b6a. Original
verdict is CONDITIONAL (most findings resolved); it explicitly permits T-629.
R-01–R-14 are reflected, with R-08's minimum scope and the N-01 comparison limitation
retained. No independent PASS is inferred for T-628 or the new T-629 implementation.
Record all new actions before archiving the original unchanged.

| Priority/order | Finding | Required follow-up | Status/dependency |
|---|---|---|---|
| 1 / P2 | N-01 | Separate comparison compatibility from source provenance; deliver an explicit baseline/target comparison contract and different-source nonzero-delta regressions. Initial same-input-only scope is superseded by the R2 implementation below; existing numbers and old snapshot rules are preserved. | LOCALLY REMEDIATED: source-independent versioned selection/coverage policy; ordered baseline/target handles and source/result digests; nonzero different-source scalar/ProfileSet regressions. Independently confirmed resolved in R3. T-635 still owns durable baseline management. |
| 2 / P3 | N-02 | Define caller/Workspace ownership of deduplicated handles before releasing shared revisions; either lease/refcount or remove public release until ownership is wired. | SAFEGUARD IMPLEMENTED: remove ReleaseEvidenceResult from Wails and generated bindings; reflection/shared-load regression. Administrative backend Release remains internal to engine callers. Full reference ownership remains T-630. |
| 3 / P3 | N-04 | Bound or compact rows × columns availability projection; explicit resource refusal on overflow. | LOCALLY REMEDIATED: preflight 100,000-entry ceiling and 16 MiB appendix budget; typed explicit refusal, no source mutation. T-631 consumes this bound. |
| 4 / P3 | N-03 | Clarify max_lines as an applied option (not proof of truncation), or perform bounded lookahead before recording a truncation marker; cover exact-EOF equality. | LOCALLY REMEDIATED: one-byte lookahead; below/equal/above physical line count with/without final newline covered. Numerical legacy parity retained. |

The initial N-01 support clarification was documentation-only. It is superseded
by the verified R2 implementation below; independent closure is still separate.
T-628 remains REVIEW for explicit closure. T-629 remains REVIEW after local
implementation checks. Totals remain 210 DONE / 3 TODO / 14 PENDING / 5 REVIEW.
Original R2 SHA-256: `2987d8db661af78c613ec0ac62ecbb023995b6c0b9e86cad3c4da40cf5aff392`.
T-629 code commit f06673a and main integration 8b5a3e2 preserve the review commit.
The original unrelated HTTP modifications stay outside these commits.

### T-628 R2 remediation started (2026-11-11)

User requested implementation and commit/push of the processed R2 follow-ups.
N-01 (P2), N-02, N-04 and N-03 (P3) are IN_PROGRESS in that order.
Implement explicit baseline/target provenance and compatible selection policies,
remove unsafe public shared-handle release until T-630 ownership exists, bound
availability projections, and cover max_lines at exact EOF. Preserve the archived
review unchanged; independent review and real UI/Windows execution are separate.
Current implementation totals: 210 DONE / 3 TODO / 14 PENDING / 4 REVIEW /
1 IN_PROGRESS = 232. Existing HTTP changes are preserved in a named stash.

### T-628 R2 remediation completed locally (2026-11-11)

Implemented N-01, N-02, N-04 and N-03 from the archived independent R2 review.
T-628 returns from the implementation checkpoint to REVIEW; this is not an
independent PASS. The original review bytes and verdict remain unchanged.

- N-01: preserve observation population/source identity and add a versioned,
  source-independent selection/coverage policy. Ordered compare handles are the
  explicit baseline/target; response references retain source hashes, exact saved
  result digests and handles. Different-log scalar delta 1 (100/3 percent) and
  different-source ProfileSet delta 0.05 are verified. Different selection,
  semantic/coverage/metric/capture conditions, unknown policies and partial
  cross-source scalar imports fail closed. Profile comparison keeps producer,
  profile/entity, quality, scale and frame-selection checks. Existing policy-free
  snapshots remain readable/exportable and same-input compatible; no schema
  number is changed to reinterpret prior numbers. Saved models can be checked
  directly; durable baseline storage and entity/time matching remain T-630/T-635.
- N-02: remove unsafe ReleaseEvidenceResult from public Wails and generated
  bindings. Two consumers can load the same deduplicated handle and continue
  reading it. Engine Release is administrative deletion only; reference ownership,
  retention and durable Workspace recovery remain T-630, not claimed complete.
- N-04: preflight at most 100,000 availability entries and cap serialized appendix
  JSON at 16 MiB. CSV/HTML fail with typed EXPORT_PROJECTION_BUDGET_EXCEEDED rather
  than truncating or mutating canonical evidence. JSON/pack source bytes survive.
- N-03: bounded one-byte lookahead marks max_lines selection only if a suffix is
  excluded. Below/equal/above EOF, final newline present/absent, completed state
  and numerical parity are covered without parsing/counting another physical line.

Executed on Mac: full go test -race ./... -count=1 (95 passed packages, 5 without
tests, 0 failures/races), go vet ./..., go build ./..., regenerated Wails bindings,
frontend state tests and production build, desktop-only guard with server-build
check, macOS production packaging plus codesign verification, Windows amd64/386
cross-vet and guarded GUI cross-builds. Actual built CLI import/save/reopen/diff
confirms different-source delta 0.05 and both exact saved-result hashes; CSV/HTML/
Report Pack exports pass. Existing profile goldens, metadata and thread mixed-format
regressions stay in the full suite. The first ad-hoc CLI export used the wrong
flag; rerunning with the documented --export-format passed (harness-only correction).
Paired EN/KO contract/support docs are updated. Evidence and source/artifact/log
hashes: docs/review/evidence/2026-11-11-t628-r2/local.json.

Not executed: independent review of these fixes, actual new WebView IPC/UI,
Windows native execution, hosted CI, real new producer qualification and long-term
production-load measurement. Direct Go Wails method calls and wire tests do not
stand in for UI execution. No new product/format decision is made for Q01–Q04.
T-629 remains REVIEW; T-630–T-632 retain their scoped ready work. Totals remain
210 DONE / 3 TODO / 14 PENDING / 5 REVIEW / 0 IN_PROGRESS = 232 (22 open).

### T-628 R3 intake and remediation started (2026-11-11)

Read the entire independent R3 against 6c3717d, committed by Claude at 2bd5e10.
Verdict remains CONDITIONAL with no P1/P2; R2 N-01–N-04 independently confirmed
resolved. R3 explicitly permits T-628 DONE and does not block T-629–T-631.
Record all actions before archiving the original unchanged.

| Priority/order | Finding | Action / dependency | State |
|---|---|---|---|
| 1 / P3 | R3-01 | Expose stable comparison refusal codes (selection, coverage, partial, semantic revision, missing/unsupported policy) in typed Go errors and JSON error causes before T-631 compare provider. | LOCALLY REMEDIATED: typed codes plus Go/JSON cause regressions. |
| 2 / P3 | R3-02 | Mark response comparison identity as declared, never imply result SHA validates original sources/options; bind durable reimport verification in T-635. Test edited saved-file declarations. | LOCALLY REMEDIATED: backend-owned declared flag; edited saved-file regression. T-635 keeps source verification binding. |
| 3 / P3 | R3-03 | Inspect whitespace-only suffix without parsing/counting more source lines; preserve existing limits/cancellation/error semantics and exact EOF behavior. | LOCALLY REMEDIATED: Unicode-aware streaming suffix check, fixed memory, source-byte/deadline bounds; EOF/blank/nonblank/error/cancel regressions. |

Implementation checkpoint totals: 210 DONE / 3 TODO / 14 PENDING / 4 REVIEW /
1 IN_PROGRESS = 232. R3 original SHA-256: `4f99931495fc292207307bbaf4df4200d7a3921c86ad8191e4d61781789ea312`.
Actual new WebView IPC/UI, Windows native execution, hosted CI and T-629
independent review remain outside this closure; CONDITIONAL is not renamed PASS.

### T-628 R3 disposition and closure (2026-11-11)

Accepted the independent R3's explicit DONE permission: R2 N-01–N-04 are
independently resolved with no P1/P2 findings. The original CONDITIONAL verdict
and archived bytes are preserved, not renamed PASS. R3's three P3 follow-ups
are additionally remediated and locally verified; no independent review of
these final changes is claimed. This closure does not close T-629's review.

- R3-01: stable ComparisonError.code/message, errors.As for Go and JSON error
  cause.code for Wails' pinned default marshaler. Selection/coverage/partial/
  semantic-revision/missing-or-unsupported-policy reasons are distinct, with
  identity/measurement refusal codes too. Ordered checks keep same-input
  compatibility; rejection grants no numeric delta. Consumers in T-631 must
  use codes rather than parsing prose. Direct bridge and JSON wire tests pass;
  actual WebView IPC is not claimed.
- R3-02: scalar and ProfileSet comparison responses explicitly set
  identity_basis=declared, even for fresh imports. A saved verification label
  cannot promote this basis. An edited saved-file test confirms its exact digest
  and comparison are still labelled declared, while JSON export preserves bytes.
  Schema/model consistency and result hashes do not authenticate original sources
  or recompute options binding. T-635 must bind original reimport/options/results
  to durable baselines before claiming verified_by_reimport; that path is absent.
- R3-03: stream remaining Unicode whitespace with fixed memory under the leased
  input-byte ceiling and cooperative cancellation/deadline. Stop at the first
  nonblank rune; parse/count no further physical lines. Blank/CRLF/Unicode and
  >1 MiB tails retain complete_records; nonblank tails remain selected. Invalid
  UTF-8 is nonblank; I/O failures and cancellation are preserved. Existing saved
  coverage is never rewritten; cross-source coverage correction uses reimport.

Executed: go test -race ./... -count=1 (95 packages passed, 5 without tests,
0 failures/races), go vet ./..., go build ./..., Wails binding generation,
frontend state tests and production build, desktop-only/server-build guard,
macOS package/strict deep signature, Windows amd64/386 cross-vet and guarded
GUI cross-builds. Actual built CLI imports verify exact/blank/Unicode suffix
coverage, plus ProfileSet JSON identity_basis=declared. Existing profile goldens,
metadata roundtrip and mixed-thread suites remain in the full regression run.
Paired EN/KO DATA_MODEL, PARSER_DESIGN and IMPORTER_SUPPORT_MATRIX are updated.
Evidence: docs/review/evidence/2026-11-11-t628-r3/local.json.

Unexecuted: independent review of these P3 fixes, actual new WebView IPC/UI,
Windows native runtime, hosted CI, original-source reimport verification,
new-producer qualification and long-term/production-load measurement. Reference
ownership stays T-630; generic consumers T-631; verified durable baselines T-635.
T-628 is DONE; T-629 remains REVIEW; next ready implementation is T-630, followed
by T-631/T-632 within scoped dependencies. Q01–Q04 remain undecided. Totals:
211 DONE / 3 TODO / 14 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232 (21 open).
Existing unrelated HTTP changes remain excluded from the commit and preserved.

### T-629 independent review intake and remediation started (2026-11-11)

Read the entire Claude review c7a1570 of f06673a and follow-ups through a320881.
Verdict CONDITIONAL: no P1, two P2 and three P3 findings. Preserve that verdict;
T-629 is IN_PROGRESS for remediation, then REVIEW pending independent closure.
Record all actions before archiving the unchanged review.

| Priority/order | Finding | Action / dependency | State |
|---|---|---|---|
| 1 / P2 | R4-01 | Remove whole-document 1 MiB transport bottleneck via bounded chunks; align actual tree serialization accounting and state effective input/result limits. Expose numeric resource refusal detail. Verify >1 MiB and >=50,000-node cases before T-634. | LOCALLY REMEDIATED: <=256 KiB chunks, original policy preserved, lowered stack/record limits honored, exact 64 MiB tree counting, numeric refusal detail; 4.96 MB / 54,001-node core and 5.48 MB CLI case pass. |
| 2 / P2 | R4-02 | Separate measured quality values from comparison categories; preserve both qualities and change disclosure. Reject entity mismatches with mapping-required code before T-635, without inventing entity mapping. | LOCALLY REMEDIATED: measured values in both references, change disclosure, typed category/selection/entity/capture refusals; lost-only difference compares and PID change requires mapping. |
| 3 / P3 | R4-03 | Bound validation cache by actual result content per immutable registry, preserve tamper/operation checks, and measure size amplification/validation reuse with R4-01. | LOCALLY REMEDIATED: at most 64 successful content-digest validations per registry; every call hashes content and still checks operation blocks; eviction/tamper/cache tests. Size amplification documented. |
| 4 / P3 | R4-04 | Validate metadata.quality copy against retained model on every uncached validation; test edited quality rejection across operations. | LOCALLY REMEDIATED: metadata.quality in authoritative-model projection validation; mutated saved-file compute/compare/report exports rejected without poisoning original cached result. |
| 5 / P3 | R4-05 | Document <=64 KiB whole-document verified detection vs larger hint/manual selection; test the boundary without granting verification to truncated JSON. | DOCUMENTED AND VERIFIED: whole-model verified detection only within 64 KiB; larger valid JSON stays a hint requiring explicit selection/full input validation. |

Checkpoint totals: 211 DONE / 3 TODO / 14 PENDING / 3 REVIEW / 1 IN_PROGRESS = 232.
R4-01 gates T-634 producer promotion; R4-02 gates T-635 baseline consumers.
T-630/T-631 are not blocked by this review. No new producer/input/Q01–Q04 decision.
Original review SHA-256: `b7e48b23b48e8f45c4a4d6af49a1220dee075b37991a6d916f8703ab89d75ee5`.

### T-629 independent-review remediation completed locally (2026-11-11)

R4-01–R4-05 from Claude's c7a1570 review are processed; T-629 returns to REVIEW.
The original CONDITIONAL verdict is preserved and no independent PASS of these
fixes is claimed. Review source was archived byte-for-byte only after intake.

- R4-01: internal adapter 1.0.1 transports a <=32 MiB model as <=256 KiB raw
  base64/offset chunks through the existing synchronous bounded sink. No policy
  ceiling is raised: serialized record 1 MiB, normalized/result 64 MiB each, temp
  160 MiB. Chunk order/size, reassembly, cancellation and caller-lowered stack/
  record ceilings are checked; failure publishes no result and cleans staging.
  Tree JSON counts exact node-shell bytes and separators up to 64 MiB instead
  of a conservative 8 MiB estimate. Independent model/cardinality/string/result/
  combined-drilldown/projection ceilings are documented jointly, not promised
  simultaneously. Numeric resource detail is carried through JobProblem and CLI
  for transport, input, record, normalized/result/temp, model cardinality, tree and
  string refusals. Detail actual/limit use exact decimal strings, including values
  beyond JavaScript safe integers; no new lossy wire conversion is introduced.
- R4-02: measured lost/unknown/truncated/resolution values do not gate comparison.
  Preserve nullable quality in both references and list changed fields/warnings;
  no arbitrary diagnostic threshold is introduced. Producer/normalizer, metric,
  capture/selection, clock/symbol/conversion categories still gate the operation.
  Differing profile/process/thread/worker requires entity_mapping_required; no
  inferred mapping is implemented. Other mismatch codes distinguish categories,
  clocks, selection, capture and revision. Source/population identity is unchanged.
  A new derived conditions hash is separate from provenance. Old saved policies
  are exactly recomputed/accepted without rewriting or promoting their scalar
  policy; model-derived comparisons can inspect old retained models directly.
- R4-03: successful validation is cached by actual serialized content SHA-256
  within the immutable registry (64 digests; no result/model retention). Changed
  content misses; failures are not cached; per-operation/version/required-block
  checks still execute. Cache reuse, eviction, tamper and operation gates pass.
  Original model/tree/frame/weight-row projections remain for compatibility and
  their size amplification is disclosed. Concurrent cold misses may revalidate.
- R4-04: metadata.quality is compared to the retained model before validation
  caching; edited quality copies fail normal JSON/CSV/HTML/pack export, compute
  and compare. Original bytes/cache remain valid; raw recovery stays separate.
- R4-05: intentionally keep bounded 64 KiB whole-model verified detection; larger
  JSON is only a hint and requires explicit importer plus profile/metric selection
  and complete input validation. Boundary unit and actual CLI cases pass.

Scale evidence: shared synthetic 3,000 stacks at depth 18 yields a 4,961,619-byte
core model / 28,894,739-byte result (5.82x) and 54,001 tree nodes, total 3,000.
Actual CLI normalize-legacy -> import -> save/reopen/pack also passes depth 14/18:
4,603,851 / 5,480,531-byte models, 42,001 / 54,001 nodes, total 3,000; CLI results
are 23,452,268 / 29,561,628 bytes (5.09x / 5.39x). These are synthetic local
observations, not producer conformance, production throughput or peak-memory proof.
Full large HTML is explicitly refused by the existing 100,000-entry appendix
ceiling; JSON/pack works. No large HTML/CSV success is claimed or truncation hidden.

Verification: full go test -race ./... -count=1 (95 passed packages, 5 without
tests, no failures/races), plus final race rechecks of ingestion/analysis/app and
R4 catalog/analysis after the last budget-metadata changes; vet/build, Wails
bindings, frontend state/production build, desktop-only/server-build guard,
macOS package/strict deep codesign and Windows amd64/386 cross-vet/build pass.
Existing profile goldens, metadata roundtrip and mixed-thread regressions remain.
Direct Go Wails calls verify quality differences and JSON error causes; actual
new WebView IPC/UI and Windows runtime remain unexecuted, as does hosted CI.
The initial new test mistakenly expected nil rather than the fixture's observed
lost=0; that assertion was corrected and all final regressions pass.

Paired EN/KO DATA_MODEL/PARSER_DESIGN/support/architecture docs are updated.
Reusable synthetic scale cases/generator and manifest/index are in shared assets
commit d3a1456; bundled manifest/cases match byte-for-byte. No huge generated sample
or binary is committed. New producer semantics and Q01–Q04 remain undecided.
Evidence: docs/review/evidence/2026-11-11-t629-review/local.json.
T-630/T-631 remain ready; R4-01 before T-634 and R4-02 before T-635 are locally
met, pending independent confirmation. Totals remain 211 DONE / 3 TODO /
14 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232 (21 open). Existing unrelated HTTP
changes are preserved outside these commits. Commit/push are authorized.

### T-629 R2 intake and disposition started (2026-11-11)

Read the full independent R2 at f08869e against 27fcd3d. R4-01–R4-05 are
independently confirmed resolved; no new P1/P2. Original verdict CONDITIONAL
(P3 only) explicitly permits T-629 DONE and does not itself block T-634/T-635.
Record actions before archiving the unchanged source.

| Priority/order | Finding | Action / dependency | State |
|---|---|---|---|
| 1 / P3 | R5-02 | Define producer/version/metric-bound semantic capture-option keys in T-634; preserve incidental values in both comparison references with disclosure. Unregistered opaque options remain strict; no arbitrary producer key list is chosen by T-629. Include T-635 baseline/mapping acceptance. | SCOPED TO T-634/T-635; document contract and test current fail-closed behavior. |
| 2 / P3 | R5-01 | Document per-string/key decoded UTF-8 limit of 4 MiB; propagate typed wirejson budget errors through ProfileSet to JobProblem detail instead of format mismatch. Check exact boundary and cleanup. | LOCALLY FIXED: typed JSON limits -> model budget -> job detail, exact UTF-8 byte boundary/format/cleanup and actual CLI regressions pass. |

Checkpoint totals: 211 DONE / 3 TODO / 14 PENDING / 3 REVIEW / 1 IN_PROGRESS = 232.
Review SHA-256: `31486b1464e33f7b5a66d94823e6853871402555139f44e4f812863d983707eb`.
Actual WebView IPC/UI, Windows native runtime, hosted CI and real producer
qualification remain separate; CONDITIONAL is not renamed PASS.

### T-629 R2 disposition and closure (2026-11-11)

Independent R2 confirms R4-01–R4-05 resolved, no new P1/P2, and explicitly permits
T-629 DONE. Accept that scoped closure without renaming CONDITIONAL to PASS.
The original review is archived unchanged; final R5 changes are locally verified,
not independently re-reviewed.

- R5-01 IMPLEMENTED: wirejson emits typed LimitError for input bytes, depth,
  individual decoded UTF-8 strings/object keys, array items and key cardinality.
  Existing messages and limits stay stable except that oversized keys are now
  correctly resource-classified. ProfileSet maps wire limits to model budget
  errors, so valid JSON with a >4 MiB string returns IMPORT_BUDGET_EXCEEDED,
  detail.resource=json_string_bytes, reasonCode=profile_json_string_bytes and
  exact decimal-string actual/limit. Format/duplicate-key/invalid-UTF-8 errors
  retain their classification; failed decode does not mutate its destination.
  Unit tests cover structural resources, UTF-8 bytes, exact 4 MiB +/-1, and job
  error detail/no publication/staging cleanup. Actual CLI accepts 4,194,303 and
  4,194,304 bytes with exact values and refuses 4,194,305 with item/limit detail.
- R5-02 SCOPED, NOT IMPLEMENTED HERE: T-634 must qualify trusted producer/version/
  normalizer/metric-bound semantic vs incidental capture-option keys. T-635
  consumes pinned policy/projections and both sanitized original option refs.
  Missing/unknown policies and keys stay strict; input whitelists have no
  authority. Do not globally ignore duration/PID or bypass entity/clock/quality
  checks. Duration/denominator meaning, presence/null/zero and exact numbers need
  actual producer fixtures. New policy revisions require explicit reimport or
  preserved old semantics, never silent v2-hash rewrites. Current opaque equality
  and capture_options_mismatch remain, with a regression for forged ignore lists.
  The detailed acceptance contract and tests are in paired DATA_MODEL plus the
  existing T-634/T-635 rows. No actual producer/key list or Q01–Q04 is invented.

Executed: full go test -race ./... -count=1 (95 packages passed, 5 without tests,
zero failures/races), go vet ./..., go build ./..., frontend state/production
build, desktop-only/server-build guard, macOS package/strict deep signature,
Windows amd64/386 cross-vet and guarded GUI builds. Actual CLI boundary imports
pass; existing profile goldens, metadata roundtrip and mixed-thread tests remain
in the full suite. Paired EN/KO model/parser/support docs disclose individual
4 MiB vs aggregate/document bounds and distinguish planned policy from runtime.
Evidence: docs/review/evidence/2026-11-11-t629-r2/local.json.

Not executed: independent review of R5-01, actual new WebView IPC/UI, Windows
native runtime, hosted CI, producer/key-policy qualification, original-source
reimport verification and production/long-term memory/latency measurement.
Existing large HTML/CSV projection limits remain; no blanket export success.
T-629 is DONE; next ready implementation is T-630 (T-631/T-632 also scoped ready).
T-634/T-635 still retain their other registered prerequisites. Totals:
212 DONE / 3 TODO / 14 PENDING / 3 REVIEW / 0 IN_PROGRESS = 232 (20 open).
Original unrelated HTTP changes are preserved outside this commit. Commit/push
are authorized by the current user request.

### T-630 implementation started (2026-11-11)

Implement the scoped case/capture lifecycle in design §3.3, §11.6, §13, §17.1
and R-08/N-02 ownership acceptance. Preserve AnalysisResult and prior source
semantics; no new producer/Q01–Q04 decision or generic renderer is introduced.

| Stage | Deliverable / acceptance | Status |
|---|---|---|
| C1 | Versioned capture context, neutral device/entity/clock declarations with basis; case manifest/revisions, source/result/artifact/baseline references and stale detection. | LOCAL_VERIFIED |
| C2 | App-data durable case storage, atomic revisions/recovery, explicit lifecycle/retention/deletion scope, portable archive/restore and rebuildable index. | LOCAL_VERIFIED |
| C3 | Bounded ZIP/TAR preview/intake; path/link/duplicate/nested/budget/hash checks; per-source status, encoding and original line/byte mapping, explicit sanitized copies. | LOCAL_VERIFIED |
| C4 | Caller-owned backend result leases, cache migration/reopen from saved case, CLI/Wails/Workspace persistence connection and regression evidence. | LOCAL_VERIFIED |

Checkpoint: 212 DONE / 2 TODO / 14 PENDING / 3 REVIEW / 1 IN_PROGRESS = 232.
Actual new WebView interaction, Windows runtime and independent review remain
separate evidence gates; only verified work will be committed and pushed to main.


### T-630 local implementation and validation (2026-11-11)

T-630 is `REVIEW`, not independently approved or DONE. C1–C4 are implemented and
locally verified. Preserve the initial IN_PROGRESS checkpoint above as history.

- C1: case/capture 1.0.0, claim basis/null/absence, complete-path/hash/size/entity
  declarations, neutral Apple/unified device identity and hierarchy checks,
  explicit header conflicts without overwrite, sanitized allowlisted descriptive
  options. Immutable result/source/capture/options/selection/version references,
  pinned baselines and artifact specs; source/encoding/omission changes mark
  dependent artifacts stale. Matching original hashes is unverified provenance,
  not successful reimport/comparison (T-635).
- C2: app-data case store with kernel lock, immutable objects/revision journals,
  revision CAS, synced atomic HEAD, interrupted-stage recovery, hash validation,
  user retention reminders and no automatic deletion. Archive/restore/logical
  delete/purge with revision-bound scope token; cache/index rebuild never deletes
  authority. Emergency deletion remains possible at logical byte/history limits.
  Portable ZIP is selection/review-gated, removes external paths, defaults source
  and Workspace to omitted, validates digest/version/envelope before one new-case
  restore and clears sharing approvals. Export rejects managed store destinations
  and current external originals/aliases.
- C3: bounded ZIP/TAR/tar.gz preview and SHA-bound intake; central directory
  preflight, path/depth/Unicode-case collisions, links/special/extended/sparse,
  nested archive, size/CRC/trailing payload limits. Structural failure/cancel
  publishes no new HEAD; declared hash failures remain individual failed sources.
  Separate UTF-8/UTF-16LE/BE/CP949/strict EUC-KR fixtures; explicit/BOM/validated
  UTF-8/permitted sniff selection, ambiguity refusal, opt-in replacement counts,
  original byte/line positions and revision-bound rebuilt indexes. Sanitized
  UTF-8 copy has parent digest and copy coordinates, never invented original
  offsets. External inputs are bounded regular files, double-read/hash/stat
  verified, and never extracted/executed in place.
- C4/R-08/N-02: exclusive legacy cache migration to app data with completion
  marker/retry/dedup/source preservation; saved cases recover exact bytes after
  cache loss. Opaque caller leases prevent administrative deletion of shared
  snapshots; public ReleaseEvidenceResult remains absent. CLI case lifecycle/
  intake/pack/reopen/export, Wails JSON-string methods/generated bindings and
  compact Workspace save/open menu are wired. Workspace releases only its own
  leases and rejects late reopen after clear. Backend-less legacy Workspace
  projections remain in-memory; saving them as exact evidence is refused.

Executed: full Go race suite (96 packages passed, 5 without tests; zero failures/races), vet/build; targeted case/analysis/Go bridge tests;
actual isolated CLI import → save/source/baseline → archive/restore → portable
snapshot-only restore → byte-identical export → cache rebuild/cleanup → scoped
purge, including export overwrite refusals. Frontend state/production build,
Wails binding generation, desktop-only/server refusal, ad-hoc signed macOS package and
strict verification, Windows amd64/386 cross-vet and guarded GUI builds pass.
Existing profile goldens, metadata roundtrip and mixed-format thread tests remain
in the full suite. Shared capture schema/encoding fixtures are bundled/pinned and
ASSET_INDEX is updated. Paired EN/KO DATA_MODEL/PARSER_DESIGN record scope/limits.
Evidence: docs/review/evidence/2026-11-11-t630/local.json.

Not executed/approved: independent review, actual new WebView IPC/UI interaction,
Windows native runtime, hosted CI and production/long-term I/O performance. Native
packaging and Go service calls do not substitute for those gates. The existing
macOS SDK deployment-target linker warning remains. No Q01–Q04 decision, producer
qualification, capture-key policy change, generic case renderer, SQLite/index
engine, automatic retention or complete physical SSD/backup erasure is claimed.
Generic consumers T-631, offline packs T-632 and source discovery/search/baseline
verification T-633/T-635 retain their registered scope and dependencies.

Current counts: 212 DONE / 2 TODO / 14 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(20 open). Next ready work is T-631 (T-632 is also scoped ready); independently
review T-630 before closure. Original unrelated HTTP changes remain outside this
commit. Main-branch commit and push are authorized by the user request.

### T-631 implementation started (2026-11-11)

User requested implementation and a completion commit. Windows native tests are
running separately on the user Windows PC. Implement descriptor-owned providers,
bounded generic consumers, compare refusal/declared identity, per-call load
cancellation, stale-handle recovery and job UI without changing AnalysisResult.
Existing HTTP documentation/verifier and its work-status hunk remain unrelated.
Current counts: 212 DONE / 1 TODO / 14 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-630 independent review intake during T-631 (2026-11-11)

Independent review of `82bc6a8`, committed separately as `29eec49`, returned
CONDITIONAL (no P1; two P2 and five P3 findings). T-630 remains REVIEW; no
remediation or independent PASS is claimed. The review explicitly does not block
T-631/T-633/T-635 implementation entry; R6-02 must close before T-635 case reuse.
Its suggested earlier timing for R6-03/R6-04 is retained below; this T-631 change
adds result consumption and uses the existing case reopen path, without adding
new case mutation/deletion controls or claiming those lifecycle defects fixed.

| Order | Finding | Priority | Action / dependency | Disposition |
|---:|---|---|---|---|
| 1 | R6-01 | P2 | Protect managed exports by filesystem identity/ancestor checks, including case aliases and hardlinks; regress overwrite refusal. | LOCALLY REMEDIATED: filesystem ancestor/file identity checks; real case-alias and hardlink regressions pass. Independent re-review remains. |
| 2 | R6-02 | P2 | Make saved-case reopen recover from evidence-cache capacity, provide safe owner-aware cleanup and actionable bounds/documentation. Test full-store reopen and import. | LOCALLY REMEDIATED: explicit unowned cleanup plus atomic lease-owned saved-case reserve (32 results/64 MiB); full-cache reopen/new-import and shared-owner tests pass. No automatic ordinary eviction. |
| 3 | R6-03 | P3 | Deduplicate identical saved-result references or block repeated UI saves; preserve selection/source/options identity. | LOCALLY REMEDIATED: complete-identity save deduplication and repeat-click Workspace reuse; no revision/slot growth for identical saves. |
| 4 | R6-04 | P3 | Provide explicit scoped deletion/recovery for corrupt manifest/HEAD cases, or document a bounded manual recovery path. | LOCALLY REMEDIATED: separate corrupt-tree plan/token/purge endpoints; changed tree refuses old token, readable cases and external originals remain protected. |
| 5 | R6-05 | P3 | Add explicit recovery/skip guidance for incomplete/corrupt legacy migration; never silently discard originals. | LOCALLY REMEDIATED: missing empty directories initialize; typed recovery guidance names legacy/durable locations and preserve/rename/restart steps. Corrupt originals and incomplete marker are preserved. |
| 6 | R6-06 | P3 | Cache revision-bound source indexes or read bounded byte windows; disclose line-index limits and measure allocation. | LOCALLY REMEDIATED: one compact revision-bound index, streaming source verification on hits, total/indexed/truncated counts. 60 MiB cached-page measurements recorded. T-635 must honor prefix coverage before wider source-link claims. |
| 7 | R6-07 | P3 | Separate local display names from portable/archive name restrictions; explain name override for colon-containing local filenames. | LOCALLY REMEDIATED: UTF-8 display labels are separate from archive/capture path safety. Colon filenames and portable digest paths pass; control characters and unsafe archive paths remain refused. |

The original is archived unchanged only after these dispositions were recorded.
Source: `docs/review/done/2026-11-11_claude-code_T-630_implementation-review.ko.md`.
Original review SHA-256: `bc0eb4ecba7fff8c166bda482ad8ffcd6ada46150a7025608e060b5d0abae3d1`.

### T-631 local implementation and verification (2026-11-11)

T-631 is REVIEW: implementation and local acceptance are complete; independent
review and the user's separate Windows-PC execution remain outstanding.

- C1: compiled/frozen versioned providers with explicit ID/kind/path/schema and
  availability/reason; descriptor/envelope/extension validation gates semantic
  capabilities. Timeline time/clock/evidence and SLI numerator/denominator/unit/
  population conformance is checked, never inferred from imported strings.
  Server events and ProfileSet frame weights register detail providers. Unknown/
  unsupported results retain read-only summary/table/diagnostic/source views;
  unknown chart scripts never execute. No canonical AnalysisResult change.
- C2: generic Describe/Query/Compare service, Wails bridge/generated bindings and
  CLI describe/query/compare plus analyze alias. Existing Analyze* wrappers stay
  compatible. Same-family fixture importer reuses detail provider/query without
  service/CLI/UI switches. Comparisons retain exact values, ordered digests,
  typed refusal codes and declared identity without source-verification claims.
- C3: Workspace import/options/explicit partial opt-in, ambiguous selection,
  job state/cancel, saved-result open, provider views/pagination, comparison,
  exact JSON export and explicit Workspace insertion. Per-load Wails context
  cancellation, superseded-response rejection, late Start cancellation, owned
  lease release, revision/digest checks and stale/corrupt handle saved-file/case
  reopen are wired. Workspace previews keep exact strings and source labels
  separate from calculation payloads; clear-generation checks prevent late adds.
- C4: independent output ceilings (128 providers/128 KiB descriptions, 32 source
  labels, 100 rows/32 stable columns, 2 KiB per text field, 256 KiB per page),
  honest truncation and oversized-first-row refusal prevent nonadvancing paging.
  Null/zero/absent, unsafe integers, malformed semantic providers, stale/canceled
  loads, compare refusal, fallback and same-family extension regressions pass.

Executed: full Go race suite (96 packages, 1,380 passed test cases, 5 expected
fixture/tool skips; no failures/races), Go vet/build, actual isolated CLI process
integration, Wails binding generation, frontend state tests and production build,
desktop-only/server-build refusal, ad-hoc macOS production package/strict signature.
Final changes were checked with focused race tests for resultcontract/catalog/
analysis/desktop bridge, and final frontend state/build/package checks. This adds
one further oversized-row regression after the full-suite test-count snapshot.

Actual native Wails/macOS Computer Use: import the bundled synthetic Tomcat log,
view total_events=3 and INFO/WARN/ERROR detail rows, insert Workspace entry, select
registered scalar comparison and observe exact self-delta 0 / 0% with ordered
same-result digests. Open a synthetic unknown result with imported fake capabilities:
show read-only fallback, exact 9007199254740993, null as em dash and zero as 0;
chart is disabled and no compare/timeline capability appears. Native table pages
show 1–25/61 and 26–50/61. The final rebuilt package additionally confirms Workspace
source label and exact summary previews. This is scoped actual WebView IPC/UI
execution, not exhaustive UI/cancellation/profile/OS coverage.

Not executed here: Windows native runtime (owned by the user's Windows PC),
hosted CI, independent T-631 review, exhaustive native cancellation/error/producer
matrices and production performance qualification. Existing macOS SDK linker
warnings persist. R6-01–R6-07 remain T-630 follow-ups; cache-full saved-case recovery
is not certified by the normal reopen path. T-633 discovery, T-634 producer
semantics/capture-key policy, T-635 indexes/search/verified durable baselines and
T-632 offline packs remain separate. No unrequested push.

Evidence: `docs/review/evidence/2026-11-11-t631/local.json`.
Current counts: 212 DONE / 1 TODO / 14 PENDING / 5 REVIEW / 0 IN_PROGRESS = 232
(20 open). T-632 is the next ready implementation; T-630's ordered review queue
must be addressed before its closure and R6-02 before T-635.

### T-630 R6 remediation started (2026-11-11)

User requested review remediation, commit and push. R6-01–R6-07 are being handled
in the existing priority/dependency order. T-630 remains REVIEW; the independent
CONDITIONAL verdict is not renamed PASS by author changes. Windows native tests
remain owned by the separate Windows PC. Existing HTTP documentation/verifier
and the HTTP-only work-status hunk remain outside this commit.

R6-02 uses explicit owner-aware ordinary-cache cleanup plus a separate bounded,
lease-owned saved-case reopen reserve. It does not evict unsaved ordinary results
automatically. R6-04 adds a separate managed-tree token for explicit corrupt-case
deletion, rather than trusting unreadable manifests. R6-06 caches compact source
coordinates and exposes the bounded prefix with total/indexed line counts.
Task totals remain 212 DONE / 1 TODO / 14 PENDING / 5 REVIEW / 0 IN_PROGRESS.

### Windows HTTP follow-up: three reproduced failures fixed (2026-11-11)

Fixed the embedded catalog/case contracts' CRLF checkout with scoped LF Git
attributes, published the shortcut fixture's JSON marker by same-directory rename,
and shared the guarded Wails 3.1.0 x86 SDK/modfile patches between production
builds and native tests. Added `test.mjs`, `task windows:test` and a native 386
full-suite CI job. Runtime hash checks and dependency pins remain unchanged.

Physical Windows amd64 and 386 full Go suites both pass: 1,208 top-level tests
pass, zero fail, 13 explicit skips per architecture. Both full vet runs pass.
The shortcut regression passes 100/100 repetitions on each architecture. Both
production builds and authenticated desktop harnesses pass their four automated
cases; full parity stays UNVERIFIED. The native x86 Task entry point, twelve
contract files' LF conversion with autocrlf=true, YAML/JS syntax and desktop-only
server refusal pass. Original SDK cache hashes are unchanged.

These are local working-tree results, not hosted CI or an independent review.
New generic WebView UI, driver/body opt-in, local race (no C compiler), vendor
client/clean-host COM and multi-hour GUI/DPI acceptance remain separate. No
registry task is promoted by this follow-up. Evidence:
`docs/review/evidence/2026-11-11-windows-http-fixes/README.md` and `validation.json`.

### T-630 R6 remediation locally completed (2026-11-11)

R6-01–R6-07 are locally remediated in the ordered table above. T-630 remains
REVIEW; original independent CONDITIONAL is unchanged and no author PASS is
substituted for independent re-review. Current counts remain 212 DONE / 1 TODO /
14 PENDING / 5 REVIEW / 0 IN_PROGRESS = 232 (20 open).

- R6-01: actual ancestor and managed-file identity protect case-differing paths
  and external hardlinks; existing external-original checks remain.
- R6-02: no implicit ordinary-cache eviction. Explicit CLI/Wails/Workspace cleanup
  removes only unleased copies and reports counts/bytes/owners. Saved-case reopen
  acquires its lease atomically and can use a separate 32-result/64 MiB transient
  reserve at the ordinary 1,024-result/512 MiB ceiling. Final owner release removes
  reserve copies only; startup reclaims crash-left copies. Full reserve gives an
  actionable bounded refusal. Shutdown waits for maintenance and serializes lock
  release; ordinary publishing never inherits a reserve handle's short lifetime.
- R6-03: same result/source hashes/capture/selection/options/versions/sharing state
  returns existing refs/revision, including when a different result was saved later.
  Workspace repeat clicks reuse their existing saved reference.
- R6-04: a separate corrupt-case plan enumerates exact managed scope, counts/bytes
  and exclusions; token binds current tree content. Readable cases use normal
  logical deletion, stale tokens refuse, and external originals/other cases remain.
- R6-05: empty/partial legacy structures initialize; corrupt data still fails closed
  with a typed preserve-both-directories/rename-legacy-backup/restart path. Tests
  execute that recovery and retain the corrupt original and absent completion marker.
- R6-06: Wails uses the compact cached SourcePage path. A revision/source/options
  key prevents reuse after edits; every hit stream-verifies content rather than
  trusting size/mtime. A one-million-line prefix is disclosed while the entire
  bounded input is validated/counted; it no longer refuses all early pages. Source
  coordinates retain prior encoding identity and fixed page limits. Derived text
  is discarded on mutation/deletion/cache cleanup/close.
- R6-07: local display names allow colons and never become archive paths. Portable
  objects retain digest paths; strict ZIP/TAR/capture path rules are unchanged.

Executed: full Go race suite, 96 packages / 1,400 tests passed, zero failures/races.
Six explicit skips include the opt-in large-page test, separately executed here.
Full vet/build, existing goldens and ownership/migration regressions, new bridge
full-cache saved reopen → explicit cleanup → real fresh import, frontend state/build,
Wails 3.1 bindings, desktop-only/server refusal, native macOS production package and
strict ad-hoc signature pass. Actual isolated CLI covers duplicate saves, native
case-alias export refusal, corrupt plan/purge and explicit evidence-cache cleanup.
Windows amd64/386 cross-vet passes; this is not new native Windows execution.

Synthetic 62,914,488-byte source: 806,596 lines; first compact page 339.400 ms,
three subsequent pages 66.375 ms combined and 524,120 allocated bytes combined;
12,905,536 used compact offset bytes. Final source-page race tests pass after
copying bounded page strings so returned rows do not retain the full decoded cache.
These are one local measurement, not a
production guarantee or controlled before/after benchmark. >1,000,000-line tests
confirm readable prefix and explicit coverage limits. T-635 must honor that limit
or add byte windows before claiming wider source access.

Remote Windows commit `97e518d` was integrated by merge `2e54e34` without rewriting
its history or treating its earlier native results as execution of this new R6 code.
Only work-status additions conflicted; both the existing T-631/R6 history and the
Windows follow-up were preserved. Windows packaging-script syntax/targeted vet are
checked separately after integration. No private evidence or user cache cleanup is
performed against the default live store.

Evidence: `docs/review/evidence/2026-11-11-t630-r6/local.json`.
Not claimed: independent re-review PASS/DONE, hosted CI result, actual new case/cache
maintenance UI execution, new Windows runtime/driver/native-case acceptance or
production workload qualification. Existing macOS SDK linker warnings persist.
Original HTTP dirty changes remain excluded. Commit and main-branch push are
explicitly authorized by the user request.

### T-630 R2 intake and T-632 start (2026-11-11)

Independent R2 at e2c1c7d confirmed R6-01–R6-07 resolved with no P1/P2 and
explicitly permitted T-630 DONE. Its original verdict remains CONDITIONAL.
Nonblocking R7-01 is accepted and locally remediated: legacy lock contention keeps
IMPORT_STORE_BUSY and does not suggest renaming; recovery errors include the
underlying cause and opaque failing legacy handle. Regression verification follows.
Original R2 is archived unchanged after this disposition. T-635 must still honor
indexed_lines/index_truncated; new native UI/Windows/hosted evidence remains separate.

User activated T-632 implementation, commit and push. Implement data-only signed
packs, isolated validation/conformance, atomic activation/rollback and pinned jobs;
mandatory offline resources/fixture bundle, fixed-runtime preflight, outbound-attempt
observation and a measured pure-Go SQLite packaging decision. Preserve existing
AnalysisResult/legacy endpoints and HTTP dirty files. T-632 is IN_PROGRESS.
Current totals: 213 DONE / 0 TODO / 14 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232
(19 open). No independent T-632 PASS or Windows runtime result is claimed.
### Pulled T-630 R6 / T-631 Windows verification and CI remediation (2026-11-11)

Validated pulled commit e2c1c7d9 on the physical Windows host. Reproduced two CLI
test launch failures caused by extensionless Windows executables; both now reuse
the existing OS-aware buildBinary helper. CI for 97e518dd and e2c1c7d9 also exposed
the 54,001-node profile regression reusing one context across import wait, saved
load and a later refusal wait. Independent per-operation contexts preserve all
existing scale/byte/record-limit assertions and product budgets.

Local amd64 full Go suite: 1,224 pass / zero fail / 14 explicit skips. Native 386
app/CLI/analysis/cases/catalog/resultcontract suites: 151 pass / zero fail / five
skips. Final three regressions pass on both architectures after the final load
context split. Both full vet runs, frontend state/build, both production builds
and the four-case desktop controller harness pass; parity remains UNVERIFIED.

Fix commit 5b583745 passes Engine Native 43030540562 (all eight jobs) and General
CI 43030540850. Hosted checks include Windows full race/live/body/SEC-10 and
policy restoration, native 386 full tests, Linux/macOS full race, both Windows
GUI cross-builds, frontend and macOS package smoke. This is actual hosted
execution, distinct from the earlier Mac-only T-630/T-631 evidence.

No independent review/task closure, complete generic native UI/DPI acceptance,
driver reach or production long-run acceptance is inferred. Local race remains
unrun without a C compiler. Evidence:
`docs/review/evidence/2026-11-11-windows-pull-ci/README.md` and `validation.json`.

### T-631 independent review intake during T-632 (2026-11-11)

Read the complete independent review at ff06c54 (CONDITIONAL; no P1, one P2,
three P3). T-631 remains REVIEW: closure is permitted only after R8-01 is fixed.
The original review is archived unchanged after recording this disposition.
These findings are queued separately from the user-activated T-632 pack slice:

| Order | Finding | Priority | Action / dependency |
|---|---|---|---|
| 1 | R8-01 | P2 | Precompute reference sets and cache provider validation by digest; add 8k/16k timeline and SLI scale/cancellation regressions. Must close before first timeline/SLI producer in T-634/T-638/T-640. |
| 2 | R8-02 | P3 | Share bounded decoded-result/provider/column caches with handle/lease lifetime; avoid full-file decode and full-table scan for every page. Coordinate cache design with R8-01. |
| 3 | R8-04 | P3 | Return structured compatibility refusal codes without numeric delta; disable incompatible UI comparison targets. |
| 4 | R8-03 | P3 | Escape BiDi/C0/C1 controls only in display DTOs, disclose escaping and preserve original export bytes. |

No R8 remediation, independent PASS, new producer qualification or task closure
is inferred from T-632 implementation. T-635 is not blocked by this review.

### T-632 local implementation and offline acceptance (2026-11-11)

Implemented and locally verified the T-632 data-only pack slice. Exact-byte Ed25519
signatures use separately registered public trust; signed offline rotations advance
one sequence and revocation blocks future activation/rollback. ZIP/path/size/hash,
closed schema, app/interpreter/extension compatibility, pinned dependency and
positive/negative conformance gates precede immutable publication. A kernel-locked
sync/rename state switch and retained versions support activation/rollback; restart
reclaims interrupted staging/orphans. This is process-recovery evidence, not native
power-loss durability. Local unsigned presets require explicit user/case scope,
locally_authored provenance and explicit collision winners; canonical IDs cannot
be shadowed. Dormant choices grant no precedence without an actual alias collision.
Jobs freeze the full catalog, schema, selected pack digests and trust sequence;
mid-selection deactivation tests prove old jobs finish while new catalogs change.

A fixed JSONL interpreter preserves exact decimals/zero/large values, literal
producer/version guards, aliases, units and gauge/counter semantics. It emits
metrics_snapshot 0.1.1 with common envelope/measurement/population/provenance;
legacy metrics_snapshot 0.1.0 and AnalysisResult stay intact. Synthetic fixtures
qualify only synthetic.metrics 1.0.0, not real products or time-series support.
CLI/Wails/Settings expose install, activation/rollback/deactivation, public-key
registration, signed offline trust updates and export. Generic import can opt
into user/case presets and shows immutable job pack pins.

The required standalone fixture ZIP pins every member hash and all three current
registered importers; executable release/CI gates reject missing/altered resources
or uncovered importer IDs without a sibling checkout. Documents, Noto Sans KR,
original OFL/Inter/dependency notices and schemas are local. Font/license/notice
hashes and source-lock parity are release checked. Byte attributes preserve hashes
on Windows; ZIP generation uses stored entries and a fixed creator/timestamp.
The clean-bundle inventory verifies platform/executable/resources; Windows desktop
requires a supplied complete Fixed Version WebView2 path, PE architecture and
runtime inventory before Wails creation, without download/Evergreen fallback.
macOS still depends on OS WKWebView; Linux desktop is not qualified by CLI tests.

Explicit offline sessions guard app networking and unqualified JFR subprocess
execution before allocation, apply a same-origin renderer policy and report
attempted operations without endpoints/payloads. Linux arm64 CLI clean-install
process trees ran under network=none with strace: standalone conformance, format
listing, actual mapped import and pack-state listing each had zero IPv4/IPv6
syscalls and zero app denials. A failed reserved-address connection produced two
observed INET syscalls, proving failed attempts are visible. This does not assert
zero desktop/WebView/Windows traffic. The scoped real macOS WebView observed
Offline session, local Korean font loaded, active signed pack, deactivation
revision 3→4 and reactivation 4→5 in an isolated diagnostics root. No exported UI
shutdown audit file is claimed; the displayed app-operation counter was zero.

The separate pure-Go SQLite v1.53.0 spike and 100k-row JSONL/index baseline were
measured and Windows 386 cross-built. Retain JSON/compact indexes in production;
this task adds no production SQLite dependency. Measurements/limits and user
commands are paired in docs/en/OFFLINE_PACKS.md and docs/ko/OFFLINE_PACKS.md.
Shared sanitized schemas/fixtures/font provenance are committed and pushed in
projects-assets at 4f0e9789ae95aee863c89e5d3c57c2c6c4c51d5f.

Validation: full uncached Go race: 99 packages, 1,421 tests pass, six explicit
skips, zero fail; final focused race covers final resource validation additions.
Host vet/build, generated bindings, frontend state/build, desktop-only gate,
production Mac package/signature and Windows amd64/386 cross-build/vet pass.
Existing Mac SDK linker warnings remain. Windows native Fixed Version launch,
font/layout and browser-process outbound observation, independent T-632 review
and hosted validation of these new changes are not claimed.

T-630 R7-01 is committed as 1fdb84b; R2 explicitly permits DONE while its original
verdict remains CONDITIONAL. T-631 R8 findings are recorded separately and pending.
T-632 is REVIEW, without self-awarded PASS/DONE. Current totals: 213 DONE / 0 TODO /
14 PENDING / 5 REVIEW / 0 IN_PROGRESS = 232 (19 open). The next work is independent
T-632 review / Windows offline acceptance and the ordered T-631 R8 remediation;
T-633 follows its registered prerequisites. User-authorized commit/push will
preserve the existing HTTP docs/script and the original HTTP work-status paragraph.
Evidence: docs/review/evidence/2026-11-11-t632/local.json.

### T-631 R8 remediation activated; T-632 review intake (2026-11-12)

The user requested git pull before T-631 R8 work, then validation, commit and push.
Pull completed before edits (34dbd8f current). Preserve unrelated HTTP files and
the original work-status HTTP paragraph. Implement R8-01 → R8-02 → R8-04 → R8-03:
linear reference lookup, private bounded projection/column caches, structured
compatibility refusals and display-only control escaping. Original T-631 review
is CONDITIONAL and remains preserved; no final independent re-review is claimed.

During implementation a new T-632 independent review was committed at e328d8d.
Read the entire document and recorded its findings before unchanged archival.
T-632 remains REVIEW; these findings do not block T-631 or T-633 and are queued
for a separate pack-remediation task:

| Order | Finding | Priority | Action / dependency |
|---|---|---|---|
| 1 | R9-01 | P2 | Keep pack administration/recovery usable after bundled signer revocation; isolate or clearly explain revoked active packs, show affected packs and add restart/recovery tests and documents. Required before T-632 DONE. |
| 2 | R9-02 | P3 | Validate application activation against all active user/case scopes before atomic commit; test both activation orders. Before T-636/T-639 rely on active packs. |
| 3 | R9-03 | P3 | Load each revision once per snapshot; bound verified revision caching by archive hash and trust sequence. |
| 4 | R9-04 | P3 | Resolve metric-name BiDi/control display using the R8-03 display boundary or explicit input rejection; test mapped names and preserve intended exact provenance. Generic display coverage is not an input-policy change. |

No pack/trust/storage runtime changes or R9 closure are included in T-631 scope.

### T-631 R8 remediation verified and closed under conditional permission (2026-11-12)

R8-01: clock/evidence/population lists become sets once per validation; malformed
lists grant no references. Timeline/SLI rows use constant-time lookups. Full race
run measured 8k/16k timeline+SLI at 217/423 ms. Cancellation is checked during
reference preparation, semantic rows, queued index acquisition and index scans;
failed/canceled preparation never publishes a partial projection or index.

R8-02: provider service requests share a private decoded model and verified
capabilities/measurements by handle plus original digest. Stable collection columns
and summary keys are prepared lazily once. Warm pages format only their slice.
The cache retains at most four entries/512 MiB of conservative structural admission
units, not an RSS guarantee; oversized entries run uncached. LRU eviction never
removes a leased result. Final lease release, Release, explicit cleanup and Close
clear matching entries. Cold/changed fingerprints rehash original bytes; warm
hits check identity/size/mtime/native change time, with full-hash fallback when
change time is unavailable. A hit is the already verified immutable snapshot,
not a fresh disk cryptographic attestation. Original exports still fully hash
and read the stored file. No persistent source/search index is introduced.
A separate 100k-row 19,689,335-byte warm benchmark measured describe+last 25-row
page at 108,454 ns/op, 119,052 B/op, 749 allocs/op over 20 iterations on this host.
No cold-load, Windows or general workload performance guarantee is inferred.

R8-04: Registry.Compare and the provider service return result_unsupported with
an explicit compatibility reason and no numeric delta for unknown/unsupported
results. Valid comparisons preserve ordered original digests and declared identity.
Descriptions expose payload_version; UI checks candidate descriptions with two
cancellable workers and disables incompatible/missing version/provider targets.
Eligibility is bound to the current baseline/digest/provider/entry set so late
answers cannot enable a changed selection. Backend selection/population/unit
refusals remain authoritative.

R8-03: display values, metadata labels, columns and source labels visibly escape
C0/C1/BiDi controls, then enforce output limits. Cell.escaped/Page.escaped and UI
notices disclose the display transformation; exact integer text, null/zero/absence
and backend export bytes are preserved. UI labels are escaped and value direction
is isolated. No parser/analyzer record, metric identity or original result changes.
The R9 metric input policy remains separately queued.

Validation: uncached full Go race has 99 passing packages / 1,432 passing tests /
six explicit skips / zero failures. Host vet/build, Windows amd64/386 vet and
production cross-builds, generated Wails bindings, frontend state/build, production
Mac package/signature, mandatory offline fixture parity and desktop-only guard pass.
Actual CLI verifies unknown comparison refusal, exact numeric/escaped fallback and
self delta zero. Scoped native macOS WebView observed visible U+202E/U+0007 markers,
9007199254740993/null/zero, escaped source/Workspace labels, the unknown target
disabled with unknown_result_type, a valid target enabled, and self comparison
Δ0 / 0% with equal ordered digests. New Windows native execution, hosted CI and
independent re-review of these fixes are not claimed; existing SDK linker warnings
remain. Evidence: docs/review/evidence/2026-11-12-t631-r8/local.json.

The preserved independent review explicitly says T-631 DONE is possible after
R8-01 remediation. That local condition is verified, and R8-02–R8-04 are also fixed;
T-631 is DONE under this permission. The original verdict remains CONDITIONAL,
not PASS, and does not independently verify the final changes. T-632 remains
REVIEW under newly recorded R9. Its review explicitly does not block T-633.
T-633/T-634/T-635/neutral T-638 are TODO; specific producer/decision gates remain.
Current totals: 214 DONE / 4 TODO / 10 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(18 open). Preserve existing HTTP dirty files and the original HTTP status paragraph.
User-authorized commit/push follows verification and remote refresh.


### T-632 R9 remediation and conditional closure (2026-11-12)

User activated R9 remediation, verification, commit and push. Ran git pull before
edits at 0f7e41d (already current). Read the preserved original independent review
in full; no new unprocessed review was present. R9 was already reflected in the
tracker before its unchanged archival during T-631 work. The ordered findings
above are historical intake; all four now have these local dispositions:

| Finding | Local remediation / evidence |
|---|---|
| R9-01 (P2) | OpenDefault skips only the untrusted bundled bootstrap installation and keeps management open without restoring removed/revoked keys or rewriting desired activation. Snapshots exclude dormant signer/dependency mappings while preserving builtin/unaffected importers and already pinned jobs/results. Status reports dormant packs and scope problems; retained-byte backup still verifies SHA and grants no activation trust. Settings/CLI show impact before applying; review SHA binds exact update/signature and current state revision. Wrong/stale review, changed files, signature/sequence violations and revoked rollback are refused. Restart, missing key, owner registration/replacement, export/deactivate, actual CLI process reopening, bridge and real builtin import regressions pass. |
| R9-02 (P3) | Application activation/rollback validates every desired active user/case scope before atomic commit. Both activation orders reject conflicting canonical mappings without changing state; a new application dependency cannot silently suspend a previously usable local dependent. |
| R9-03 (P3) | One decoded revision per selected pack in each Snapshot; dependencies/cycles reuse it. Successful full verification has a bounded 64-entry/80 MiB admission-charge LRU keyed by scope, archive digest, trust sequence and signer fingerprint. Every access still reads/hashes compressed bytes. Trust commit/Close clear cache; copied mapping fields prevent caller mutation. Corruption, alternate candidate trust with equal sequence, cancellation and retention tests pass. A 12.7 MiB expanded synthetic fixture measured 20 warm snapshots in about 4 ms under race on this host; not an RSS/Windows/general workload guarantee. |
| R9-04 (P3) | Mapped metric names reject C0/C1/BiDi through the shared R8 display predicate. Ordinary Korean/accent/ZWJ names and exact decimals are preserved. Integration tests confirm new unsafe input fails, while older saved unsafe metric payloads remain readable, visibly escaped in provider pages and unchanged on raw export. |

Validation and limitations are recorded at
`docs/review/evidence/2026-11-12-t632-r9/local.json`. Full Go race, host vet/build,
Windows amd64/386 vet and production cross-builds, generated Wails bindings,
frontend state/build, signed Mac package, mandatory offline asset parity and
desktop-only guard pass. Fresh disconnected Linux CLI fixture/formats/import/list
process trees show zero INET syscalls; the failed-network positive control records
two INET calls. This is no desktop/browser outbound claim. New native Mac review
UI observation was attempted but blocked by the locked screen; bridge integration
is tested separately. Native Windows runtime/UI, hosted CI, real producers and
independent re-review of the final R9 fixes remain unexecuted here.

The original independent review explicitly permits T-632 DONE after R9-01 is
fixed. That condition is verified; R9-02–R9-04 are fixed too. T-632 is DONE under
that permission, with original CONDITIONAL retained and no self-awarded independent
PASS. T-631 stays DONE. Next implementation is T-633; T-634/T-635/neutral T-638
remain TODO. Totals: 215 DONE / 4 TODO / 10 PENDING / 3 REVIEW / 0 IN_PROGRESS =
232 (17 open). Preserve the unrelated HTTP documents/script and original HTTP
status paragraph outside this commit.


### T-633 input assistance implementation activated (2026-11-12)

User requested implementation, commit and push. Ran git pull before changes at
0ce79e7; already up to date. No unprocessed review document was present. Preserve
unrelated HTTP documents/script and the original HTTP tracker paragraph. T-633
is IN_PROGRESS: inspect catalog-backed input hints and explicit capture sidecars,
ship a separate inert collection kit, and bound explicitly chosen installed JDK
conversion with cancellation/provenance. No new producer qualification, automatic
attach/download or native binary JFR decoder is implied. Checkpoint totals:
215 DONE / 3 TODO / 10 PENDING / 3 REVIEW / 1 IN_PROGRESS = 232 (17 open).


### T-633 local implementation and validation (2026-11-12)

T-633 is REVIEW after implementing input assistance, separate manual collection
assets and explicit installed JDK conversion. This is local implementation
acceptance, without an independent DONE/PASS or new native Windows acceptance.

- Input inspection uses an owned bounded source snapshot and the current scoped
  generic catalog. It hashes full selected bytes, probes at most 64 KiB, returns
  candidate grades/capabilities/limitations, and distinguishes dedicated JFR hints
  from registered generic support. Up to four first-row declarative previews show
  exact source → normalized text/unit/kind with pack SHA using the same interpreter.
  Preview is sample-only, not full import/conformance; display escaping is disclosed.
  Inspection does not execute a tool or analyze a result.
- Explicit optional sidecars are bounded/validated and must match selected file
  hash and size. Only entity IDs linked to those bytes fill identity gaps. Missing
  clock/producer/process context stays unknown; mtime never becomes collection
  time. New drafts contain observed file identity and unknown capture fields.
- Workspace supports inspect/select-importer, explicit sidecar/clear, new draft,
  manual collection ZIP, explicit installed jfr selection and cancellable conversion.
  Late answers/file-dialog selections cannot update a changed input scope. Imported
  display strings use R8 escaping. Conversion output is opened in the dedicated
  JFR view; no generic JFR importer or producer qualification is invented.
- The inert manual kit includes paired manuals, shared manual-only recipes and
  capture schema, separate from data packs. Reusable recipes and index are in
  projects-assets commit 5801ce822106b4d89b2f949d6200d403c35ff97e; embedded copies
  pass exact-byte parity and require no sibling checkout at runtime.
- The converter requires an explicit absolute installed tool, fixed version/print
  argv and private .jfr-suffixed source snapshot. It bounds whole/version time,
  source/output/stderr bytes and concurrency; hashes executable/input/output and
  records sanitized argv/tool version. No shell, automatic attach/download or
  arbitrary options. The recorded jfr tool version may be 1.0 on older JDKs;
  it is not a separately measured whole JDK distribution version.
- Unix process groups and suspended Windows Job Objects prevent descendant escape
  on cancellation. Stderr/environment values are omitted from refusals, Java
  injection environment variables are excluded, changed/cancelled/invalid output
  is discarded. The chosen external executable is trusted owner input, not an
  execution sandbox or full-JDK cryptographic qualification.
- New JSON plus .conversion.json publication refuses existing files and managed
  state paths. Failure to publish provenance removes the owned JSON. Legacy binary
  JFR paths share the bounded/context-aware runner and preserve common result
  contracts and JSON readers; PATH/JAVA_HOME discovery alone no longer executes.

Validation: full Go race has 102 passing packages / 1,458 passing tests / six
explicit skips / zero failures. Host vet/build, Windows amd64/386 vet/production
cross-builds, Wails bindings, frontend state/build, signed Mac production package,
embedded manual/offline parity, desktop-only guard and real CLI/bridge flows pass.
Actual installed Temurin JDK 11/17/21 convert one synthetic custom-event JDK 11
recording and analyze JSON plus both legacy binary JFR/native-memory paths with
provenance. This is not broad event/producer/Windows qualification. Fresh network-none
Linux CLI trees include discovery/kit and a denied converter call: zero INET
calls, exactly one app converter denial, and two INET positive-control calls.
Native Mac UI observation was attempted but the screen is locked. Independent
review, hosted CI, Windows process-tree/runtime/UI and desktop outbound observation
remain unexecuted here. Shared full asset validation hits the pre-existing
profile-core-v1 manifest dict/list error; validator and offending manifest match
unchanged HEAD. New manual assets/index and embedded parity pass separately.

Counts: 215 DONE / 3 TODO / 10 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232 (17 open).
Next implementation is T-634; T-635 and neutral T-638 are also TODO. Preserve the
unrelated HTTP docs/script and original HTTP status block outside this commit.


### T-633 independent R10 review intake and remediation (2026-11-12)

User requested review processing, commit and push. Pulled before edits at e1a0340;
already current. Read the entire unprocessed T-633 review. Original verdict is
CONDITIONAL, no P1; explicit DONE permission follows R10-01 remediation. Existing
local review-index/summary removals are reconciled into this intake. Preserve the
unrelated HTTP documents/script and original HTTP status block. T-633 remains
REVIEW during remediation; the review does not block T-634 and later ready work.

| Order | Finding | Priority | Action / closure condition |
|---|---|---|---|
| 1 | R10-01 | P2 | Align bounded disk-spooled JSON output with the legacy JSON reader, distinguish input/output/time refusal messages and exercise >100 MiB plus an event-rich ~10 MB JFR. Required before T-633 DONE. |
| 2 | R10-04 | P3 | Preflight both output names before conversion, retain atomic no-replace at publication, hide internal temp paths and stream validation/hash/publication. |
| 3 | R10-03 | P3 | Drain/discard stderr without cancelling successful conversion; retain bounded diagnostics/counts and exit/timeout authority. |
| 4 | R10-02 | P3 | Identify owner-selected executables honestly; version/hash is not verified JDK publisher or full distribution identity. |
| 5 | R10-05 | P3 | Correct old EN/KO JFR discovery text and update manual/UI budgets and diagnostics. |

Findings are recorded before unchanged archival of the original review at
`docs/review/done/2026-11-12_claude-code_T-633_implementation-review.ko.md`.
No independent final re-review or native Windows execution is claimed.


### T-633 R10 remediation verified and conditionally closed (2026-11-12)

All R10 findings are locally addressed in the intake priority/dependency order.

| Finding | Local disposition / evidence |
|---|---|
| R10-01 (P2) | Independent input 64 MiB / expanded JSON 512 MiB disk-spool ceilings, aligned with the legacy JFR JSON reader. Binary size cannot predict JSON expansion. A generated native JDK 21 recording is 10,521,805 bytes and expands to 529,956,916 bytes; explicit conversion and legacy binary native-memory analysis succeed. >100 MiB exact-hash publication and >512 MiB refusal regressions pass. Input/output/structural/time/disk messages identify their actual causes, without suggesting an incompatible JDK for size errors. |
| R10-04 (P3) | Both output and provenance names are checked before any tool execution, then rechecked with atomic no-replace publication. Stable OUTPUT_EXISTS/OUTPUT_MANAGED_PATH/OUTPUT_CHANGED/OUTPUT_UNAVAILABLE messages hide internal temporary paths. Lexical scalar bounds plus streaming JSON token validation retain no document tree; hash/copy/publication use bounded buffers. A ~108 MB synthetic tool output uses about 27 MB maximum RSS in this one-run CLI process-tree measurement. Existing outputs and races preserve previous bytes; cancellation/failure cleans owned scratch. |
| R10-03 (P3) | Stderr is drained/discarded without a content-size failure, with only byte count and omission flag in provenance. A successful tool emitting >20 KB warnings succeeds; nonzero exit, stdout limit and deadlines remain authoritative. No stderr content is exposed. |
| R10-02 (P3) | Provenance identifies the invocation profile as jfr.print-json.v1 while tool identity remains declared_unverified with owner-selected hash basis. Tool --version is not publisher authentication or full-JDK identity. Legacy jfr_cli metadata retains that unverified identity too; hash and exact sanitized arguments remain recorded. |
| R10-05 (P3) | Both old EN/KO JFR design paragraphs now require explicit absolute ARCHSCOPE_JFR_CLI and reject PATH/JAVA_HOME-only authorization. Paired manuals/embedded copies/UI disclose independent limits, disk requirements, streaming structural budgets, omitted stderr and unverified tool identity. |

Verification: 102 packages / 1,465 tests pass in full uncached Go race, with six
explicit skips and no failures. Focused race regressions additionally verify
legacy metadata identity and version-output budget diagnostics. Host vet/build, Windows amd64/386 vet
and production cross-builds, Wails bindings, frontend state/build, signed Mac
package, offline/manual parity and desktop-only guard pass. Fresh network-none
Linux CLI observation repeats all seven operations, including conversion refusal,
with zero INET calls and a working failed-network positive control.

The ~530 MB actual JFR conversion takes about 6.6 seconds in one local CLI run;
its process-tree maximum RSS is about 326 MB (including the installed tool).
The legacy analyzer still materializes JSON and uses about 2.7 GB in this case;
this change makes conversion/publication streaming, not legacy analysis. These
are scoped synthetic native-JFR measurements, not broad workload/platform guarantees.
The Mac is locked, so new native UI remains unobserved. Native Windows Job Object/
WebView/UI execution, hosted CI and final independent re-review are not claimed.
Evidence: docs/review/evidence/2026-11-12-t633-r10/local.json.

The preserved review explicitly permits T-633 DONE after R10-01 remediation.
That condition is verified and all P3 findings are fixed too; T-633 is DONE under
this permission. Original CONDITIONAL stays unchanged; no independent PASS is
self-awarded. Next ready implementation is T-634, with T-635 and neutral T-638
also TODO. Counts: 216 DONE / 3 TODO / 10 PENDING / 3 REVIEW / 0 IN_PROGRESS = 232
(16 open). The unrelated HTTP documents/script and original HTTP tracker block
are preserved outside this commit.


### T-634 producer semantics implementation activated (2026-11-12)

User requested implementation, commit and push. Ran git pull before edits at
e650b53; already current. No unprocessed review was present. Preserve unrelated
HTTP documents/script and original HTTP status block. T-634 is IN_PROGRESS.
Implement original-source exact py-spy collapsed/Speedscope/pprof normalization,
explicit profile and metric selection, new semantic result revision, exact export
and compiled producer-bound capture comparison policy. Preserve old facades, saved
bytes and v2 identities; no global pid/duration whitelist. Qualify only concrete
producer versions backed by actual sanitized fixtures. Current totals:
216 DONE / 2 TODO / 10 PENDING / 3 REVIEW / 1 IN_PROGRESS = 232 (16 open).

### T-634 exact original profile semantics implemented (2026-11-12)

T-634 is REVIEW, not self-awarded DONE/PASS. Pulled before edits and again before
commit; both checkouts were already current. No unprocessed project review was
present. The unrelated HTTP documents/script and original tracker block remain
outside this commit.

- New original-source parser owns bounded snapshot/gzip/protobuf/JSON decoding;
  ProfileSet owns exact model/selection/compiled comparison policy; analyzer emits
  the unchanged common AnalysisResult envelope with new payload 0.3.0. Old legacy
  facades and internal 0.2.0 results/v2 comparison hashes remain unchanged.
- Fractional/scientific/zero weights and every Speedscope profile survive.
  Sampled time is duration, not inferred CPU time; evented O/C stacks become
  exclusive elapsed intervals with explicit unattributed gaps. Malformed clocks,
  missing weights/frame refs, unbalanced events and budgets refuse success.
- pprof keeps all type/unit columns, large int64/zero/signed values, worker-label
  groups, inline/root ordering, string/numeric labels and original period metadata.
  Sampling period is not applied twice. Exact tick-scaled selected pprof projection
  remains explicit; inexact output is refused and tick units are not silently
  reinterpreted by the original reader.
- R5-02 canonical implementation: pinned py-spy 0.4.1 raw/sampled-seconds and Go
  go1.26.10 heap policies are compiled and bound to normalizer/format/metric. Unknown
  keys and unqualified bindings remain strict. Missing/null/zero and forged
  whitelists are tested. Both sanitized original option snapshots, policy hashes,
  semantic/incidental projections and incidental changes are retained. PID also
  binds process identity, so incidental classification cannot bypass entity mapping.
  Generic scalar comparison stays strict pending T-635 durable baseline/consumer
  wiring; declared identity is never upgraded to original-source authentication.
- Actual synthetic producer corpus: py-spy 0.4.1 three-process Speedscope plus raw,
  and Go runtime/pprof go1.26.10 four-column heap. Path/time sanitization and hashes
  are documented in reusable asset diagnostics.profile-producers.v1, shared commit
  08d3752; byte-identical test copies need no sibling checkout at runtime.
- CLI inspect/normalize/import-source and Workspace source assistance require
  explicit metric/profiles. Original hash is rechecked before atomic new-file
  publication. Saved-result, canonical diff, drilldown and exact export reuse the
  common backend; new/old generic transports enforce disjoint semantic revisions.

Validation: full uncached race reports 102 packages ok, 5 without tests, 1479 tests
passed and six explicit skips, no failures. Final focused race additionally covers
exact large capture-number identities, independent actual producer totals, expanded
input/observation ceilings, source transport fixture and Wails saved/export flow.
Host vet/build, Windows amd64/386 cross-vet and production build, frontend state/
TypeScript/production build, generated bindings, signed Mac package, desktop-only
and offline resource guards pass. A disconnected clean Linux CLI bundle verifies
all four registered importers and zero IPv4/IPv6 attempts with a failed-network
positive control. Native Mac screen was locked, so the new UI was not observed;
native Windows execution and independent review are unexecuted here.

The shared corpus passes targeted manifest/hash/byte-parity checks. The shared
repository's full validator still crashes on its pre-existing map-style
profile-core-v1 manifest; no broad validator PASS is claimed and that unrelated
asset was not changed. Details and artifact/source digests are in
`docs/review/evidence/2026-11-12-t634/local.json`.

Current totals: 216 DONE / 2 TODO / 10 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(16 open). Next implementation is T-635; neutral T-638 also remains ready.
T-636 still requires both T-634 and T-635 completion. Existing review verdicts,
Q01–Q04 and native HTTP/Windows acceptance remain separate.

### T-635 case search, canonical queries and verified baseline work activated (2026-11-12)

User requested implementation and commit. Pull before edits reports already up
to date at eaa8c54. No unprocessed project review is present. Preserve unrelated
HTTP files/script and original tracker block. Implement revision/query-bound
canonical profile views and evidence coordinates, bounded sanitized case search
with explicit coverage and rebuildable indexes, and durable immutable baseline
bindings whose original-source reimport is checked before verified reuse. Keep
mapping declarations separate from source verification, old saved bytes/v2
identities and unknown capture policies strict. T-635 is IN_PROGRESS.
Current totals: 216 DONE / 1 TODO / 10 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-634 independent R11 review intake during T-635 (2026-11-12)

Independent review arrived at 1847b77 while T-635 was underway. Recorded before
archival. Original verdict CONDITIONAL is unchanged; explicit permission allows
T-634 DONE after R11-01 remediation. Priority/dependency order:

| Priority | Finding | Action / disposition |
|---|---|---|
| 1 / P2 | R11-01 | Block baseline qualification until original pprof sample type/unit are visible in inventory/UI/result and policy-independent comparison identity. Emit a new normalizer/payload revision; preserve saved bytes and old hashes. Add alloc_space vs inuse_space no-policy regressions. |
| 2 / P3 | R11-02 | Clarify numeric observation ceiling and duration implications; retain fail-closed 100k row budget and avoid silently coalescing original references. |
| 3 / P3 | R11-03 | Expose declared/file-exporter policy basis and warn in new canonical/baseline consumers; source replay does not authenticate producer. |
| 4 / P3 | R11-04 | Expose original Speedscope none unit and avoid presenting unitless arbitrary weights as proven sample count in the new revision. |

This prerequisite correction is included before T-635 baseline validation. Native
Windows/UI and final independent review are not inferred from author validation.

### T-635 implemented; prerequisite R11 findings locally remediated (2026-11-12)

T-635 is REVIEW, not self-awarded DONE/PASS. T-634 is DONE under the independent
review's explicit after-R11-01 closure permission. All R11 findings are locally
fixed; original CONDITIONAL remains unchanged and final independent re-review is
not claimed. Intake was recorded above before unchanged archive at
`docs/review/done/2026-11-12_claude-code_T-634_implementation-review.ko.md`.

| Finding | Local resolution |
|---|---|
| R11-01 / P2 | Source normalizer 1.1.0 / result 0.4.0 carries original type/unit in metric, inventory, UI, result summary and comparison identities independently of producer policy. Actual Go heap alloc_space vs inuse_space with undeclared producer now refuses. Old 0.3.0 bytes and v2/v3 hashes remain readable/saveable/queryable/exportable; comparisons require explicit original reimport. Old internal 0.2.0 and legacy facades retain their separate semantics. |
| R11-02 / P3 | Budget message reports observation/frame/depth actuals and limits, with reduction/splitting guidance; paired manuals quantify 100k rows at 100 Hz (about 1,000 seconds single-worker / 250 seconds for four). No lossy implicit coalescing or budget increase. |
| R11-03 / P3 | New compiled policy snapshots expose declared/file_exporter basis and canonical diffs warn capture_policy_uses_declared_producer. Source reimport proves byte/options/model consistency, not producer authenticity or process lifetime. Old policy snapshots remain unchanged. |
| R11-04 / P3 | New Speedscope none weights use generic count with original source_unit=none and total_samples unavailable. Source values remain exact. |

T-635 delivery:
- One canonical ProfileSet selection drives top/tree/flamegraph, caller/callee,
  focus, evidence and exact new-result materialization. Text/time-window views
  filter rows, not denominator. Exact scale and half-open timestamp windows,
  unavailable timestamps, quality and source references remain explicit. Paged
  geometry uses the full canonical denominator; display truncation does not
  renormalize the picture. Cursors pin result/query/selection and stale pages fail.
- Case-wide literal source/summary/typed-profile search uses persistent rebuildable versioned
  indexes bounded to 100k rows/16 MiB, with masked 2 KiB snippets, byte/line links,
  decoder/prefix/missing/budget coverage. Typed clock/unit time windows require one
  result scope and retain untimed counts; no cross-capture relative alignment is
  inferred. Cache hits verify source/result bytes;
  corrupt/missing caches rebuild. Existing million-line source prefix is honored.
  Structural pprof/Speedscope refs never become fabricated text line numbers.
- Durable profile-baseline 1.0.0 conditions pin immutable result/source/model/
  options/selection/policy/capture/clock refs. Backend replay of both originals
  runs on each reuse, compares entire exact reconstructed models and pinned
  binding, then returns verified_by_reimport separately from identity_basis=declared.
  Old or forged claims, changed/missing originals, edited model/options, stale
  encoding/result/revision, policy revisions and unsupported legacy source
  snapshots refuse rather than silently upgrade a historical baseline.
- Entity mapping is complete, current, one-to-one and declared; capture-context
  mapping pins both digests and a reason. Clock hashes and semantic measurement/
  capture/quality gates remain strict. Unknown/no-policy options cannot be relaxed.
  Qualified canonical consumers disclose original options/incidental differences;
  older generic scalar comparison remains strict. Explicit clock alignment and
  unparsed log-field/time extraction are outside these qualified capabilities.
- Workspace exposes case/source/result intake, search/coordinates, original replay,
  pin/compare and exact selected snapshots through opaque JSON/cancelable scopes.
  CLI profile-query and case-query provide the same operations. CLI service errors
  are propagated (regression catches shadowed-error false success). Source-linked
  saved snapshots retain backend ownership leases; late created selections release
  their lease when canceled or the workspace changes.

Validation: full uncached race after R11 reports 102 packages ok / 5 without tests,
1493 tests passed / 6 explicit skips / 0 failures. Additional CLI refusal race
regression and final actual CLI end-to-end check pass after its isolated error-
propagation fix. Host vet/build, frontend state/TypeScript/production build,
generated bindings, signed Mac package, Windows amd64/386 cross-vet/production
build, embedded offline-resource/desktop-only guards and disconnected four-importer
Linux CLI observation pass. CLI checks also cover exact selected denominator,
restarts, stale cursors, cache rebuild, immutable pins, changed-source refusal,
old 0.3.0 reader/comparison gate and actual Go source-type mismatch refusal.
Native Mac UI was unavailable because the screen remained locked; native Windows
execution and independent T-635/final R11 review are unexecuted here.

Evidence: `docs/review/evidence/2026-11-12-t635/local.json`. Reused producer corpus
is unchanged; no new reusable diagnostic rule or sibling runtime dependency was
introduced. Pull was run before edits and refreshed before commit. The independent
review commit 1847b77 arrived during execution and is preserved. Unrelated HTTP
documents/script and the original tracker block remain outside the task commit.
User requested commit for this task; no push is part of this completion.

Current totals: 217 DONE / 1 TODO / 10 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(15 open). Next ready implementation is neutral T-638. T-636 remains PENDING until
T-635's review gate passes; T-634's registered prerequisite is now satisfied.

### T-638 metric history implementation activated (2026-11-12)

User requested implementation and processing of reviews arriving during the work.
Pull before edits at befd5f3 is already current. No unprocessed project review is
present at activation. Preserve unrelated HTTP files/script and original tracker
block. T-638 is IN_PROGRESS; T-635 remains REVIEW. Implement a neutral exact
metrics_timeseries contract, bounded original-file import, timestamped history /
range provenance / OTLP JSON scalar and histogram structures, trustworthy derived
intervals with explicit gaps/reset/overlap/unknown status, immutable saved readers,
CLI/Wails/generic consumers and fixtures. Preserve metrics_snapshot semantics.
Q04 and instrument/version-specific GenAI aliases remain separate gates; no live
receiver, automatic cross-export merge, percentile averaging or client/server
population merging is implied. Recheck review intake during execution.
Current totals: 217 DONE / 0 TODO / 10 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-635 independent R12 review intake during T-638 (2026-11-12)

Independent review 2e0e16c arrived while T-638 was underway. Recorded before
archival. Original verdict CONDITIONAL (P3 only) explicitly permits T-635 DONE
once the nonblocking findings are queued; all four are being handled here before
large metric/visual consumers. Ordered actionable queue:

| Priority | Finding | Planned disposition |
|---|---|---|
| 1 / P3 | R12-01 | Bounded short-lived in-process baseline comparison cache, bound to full case/query/policy/source/result identities; recheck bytes per page; CLI stateless executions remain separately disclosed. |
| 2 / P3 | R12-03 | New isolated index-mask policy/version for common key/value secrets, JWT/AWS patterns and idempotent auth placeholders; invalidate old derived indexes without rewriting snapshots. Reusable patterns belong to shared assets. |
| 3 / P3 | R12-02 | Explicit self/inclusive sort in canonical query/hash; UI defaults Top self, old omitted sort preserves inclusive behavior. |
| 4 / P3 | R12-04 | Numeric saved-result size/limit and available sample count, actionable guidance, paired docs; no hidden aggregation or raised storage ceiling. |

Review does not block T-638. Native UI/Windows and final independent review are
not inferred from local remediation. Existing HTTP edits remain separate.

### T-638 implementation and T-635 R12 completion (2026-11-12)

T-638 is REVIEW after local implementation and verification, not self-awarded
DONE. New MetricHistory 1.0.0 / metrics_timeseries 0.1.0 keeps exact values,
resource/scope/schema/attributes, temporality and start/end times; scalar special
values, reset/gap/overlap/order/unknown statuses and separate explicit/exponential
buckets are preserved. No percentile averaging, automatic cross-series sum or
client/server/OTLP-Prometheus population merging is introduced. Original gzip
hashes, coordinate references and provenance-qualified duplicate counts are bound.
Neutral snapshots/range query provenance and structurally supported OTLP JSON
feed a bounded canonical transport, immutable selected views, CLI/Wails query and
generic detail/timeline/export/case readers. Existing metrics_snapshot is unchanged.
The actual synthetic SDK Python 1.38.0 fixture qualifies only its encoded slice;
Q04 actual user export/instrument/version/GenAI aliases remain DECISION_PENDING.

R12-01–R12-04 are locally fixed and verified. Cache authority exists only after
original replay within the current process, with 2 entries/128 MiB total/2-minute
expiry; each page rehashes original/result bytes and full case/query binding. CLI
processes remain stateless. Sort self/inclusive binds cursors; UI Top defaults
to self while omitted API sort keeps inclusive behavior. Separate index-mask
0.2.0 policy/hash covers common key/value secrets, JWT/AWS patterns and stable
auth placeholders; old snapshots/capture mask 0.1.0 stay unchanged. Result limit
errors expose actual bytes/64 MiB and actionable selection/capture guidance,
with sample count explicitly unavailable before bounded decoding. No cap was raised.

The unchanged review was archived only after the queue was recorded. T-635 is
DONE under R12's explicit nonblocking-queue closure permission; original verdict
remains CONDITIONAL and final local fixes have no independent PASS. T-636,
T-637 and T-641 are now TODO in dependency order; T-639/T-640/T-642/T-643/T-644
remain PENDING on T-638 and their input-specific gates. Next work is T-636.

Validation and immutable hashes are recorded in
`docs/review/evidence/2026-11-12-t638/local.json`. The Mac was locked, so actual
WebView interaction was not observed; native Windows execution remains with the
Windows PC acceptance stream. Cross-builds are not native acceptance. Shared
reusable fixtures/schema and index masking rules have byte-identical application
copies and no runtime sibling dependency. Unrelated HTTP documents/script and
the pre-existing tracker block remain outside this task's commit. No push is
authorized for this task.

Current totals: 218 DONE / 3 TODO / 7 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(14 open). T-638 awaits independent review.

Final validation detail: the full uncached race run passed 1,513 tests with six
explicit skips across 104 tested packages (six packages without tests). Final
metric boundary/provenance/error and opaque-identity display changes then passed
focused race runs, including CLI/Wails and original-byte cache invalidation.
8k/16k timeline/detail preservation and cache reuse pass. Host and Windows
amd64/386 vet, generated bindings, frontend state/production build, signed Mac
package and both GUI cross-builds pass. Browser actual-component checks with
synthetic Go outputs cover initial loading, exact large values, 25-row pagination
and explicit source selection; this is renderer evidence, not native WebView IPC.
A network-none clean Linux bundle passed all five registered fixture importers
and metric inspect/normalize/import/query under syscall observation, with zero
INET attempts and a successful failed-connect positive control.

### T-636 visual artifact implementation activated (2026-11-12)

User requested implementation and local commit. Pull at 2454987 is already
current. T-632/T-634/T-635 are DONE; T-636 is IN_PROGRESS. No unprocessed review
was present at activation. Preserve unrelated HTTP documents, verifier script
and the original tracker history block. Implement pinned AnalysisView/specs,
a fixed separate desktop WebView render session with bundled font readiness,
PNG/SVG/table output, real native clipboard completion, immutable artifact
snapshots and explicit regeneration/version gates. Existing SaveChartArtifact
remains an already-rendered-byte saver. CLI validation/spec/static exports are
separate from unsupported CLI raster/vector rendering and clipboard. Office
paste/native Windows acceptance must be observed, not inferred from cross-builds.
Current totals: 218 DONE / 2 TODO / 7 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-638 R13 independent review intake during T-636 (2026-11-12)

Read independent review 4542e89 in full before archival. Verdict CONDITIONAL
(P2 1 / P3 6); explicit permission allows T-638 DONE after R13-01 remediation
with remaining P3 findings queued. Preserve existing 0.1.0 readers and original
review bytes. T-636 proceeds; resolve the payload P2 before more consumers.

| Order | Finding | Disposition / dependency |
|---|---|---|
| 1 / P2 | R13-01 | Locally fixed: selected-series compact payload 0.2.0, minimal projections, compact CLI result output and measured actionable limits; 0.1.0 saved bytes/readers retained. Actual 100k-point CLI import/readback and one-series storage reduction verified; permits T-638 closure under original review condition. |
| 2 / P3 | R13-02 | LOCALLY_FIXED: verified result-byte rehash on every request, owned model/interval index and binary-search window paging;2 entries/192MiB structural admission/fixed2-minute lazy expiry/LRU, oversized valid entries uncached. Caller isolation, stale cursors, cancellation, cleanup,0.1/0.2 exact readers and stateless CLI verified.100k local cold4.160s/warm3 pages73.320ms, no RSS/general latency claim. Evidence: docs/review/evidence/2026-11-12-metric-cache/local.json. |
| 3 / P3 | R13-04 | LOCALLY_FIXED: value-free SourceError codes/decoded JSON paths/rules/decoder offsets; exact-range failures still refuse whole source without partial model/publication. Existing saved semantics and resource/cancellation contracts preserved. |
| 4 / P3 | R13-03 | LOCALLY_FIXED: normalizer 1.1.0 range gauges expose declared-cadence gaps; no interpolation/rates; saved 1.0.0 semantics preserved. |
| 5 / P3 | R13-05 | LOCALLY_FIXED: normalizer 1.2.0 marks identical equal-time cohorts duplicate_observation, differing cohorts conflicting_timestamp; retains every ref and suppresses ambiguous rates. |
| 6 / P3 | R13-06 | LOCALLY_FIXED: dedicated exact interval CSV 1.0.0 via CLI and desktop; whole selection/all pages, values/rates/buckets/refs, bounded output and no-overwrite publication. Generic CSV remains summary-only. |
| 7 / P3 | R13-07 | LOCALLY_FIXED: shared source/result picker searches exact identities, 50 rows/page, filtered all-page selection preserves hidden IDs, UTF-8 selection budget guidance. |

Original review permits closure after P2 plus this queue; it is not a new PASS,
and native WebView/Windows acceptance is separate. T-635 remains DONE; R12-02,
R12-03 and R12-04 were independently confirmed, R12-01 has no new timing proof.

### T-636 implementation/local verification and T-638 R13 P2 closure (2026-11-12)

T-636 is REVIEW. Implemented immutable bounded AnalysisView/VisualArtifactSpec,
four current templates (provider table, profile Top/drilldown, selected flamegraph,
strict profile diff with both sources/quality frozen), deep-page flamegraph vertical origin, fixed separate hidden desktop WebView, bundled font hash/
readiness, exact selected strings with disclosed layout/label limits, split
images, PNG/SVG/table save, true native clipboard completion, historical library,
ZIP verification/import and 1–4 visual Report Pack with optional backend appendix.
Existing SaveChartArtifact/AnalysisResult/exporter contracts remain intact.
Pinned selection/value/layout inputs are separate from cross-platform pixel-byte
identity. Old images remain byte-preserved; dependency/selection changes require
an explicit new revision. Headless CLI generates/validates specs and exports saved
bytes/static tables; new PNG/SVG rendering and clipboard are explicitly refused.

R13-01 is locally resolved: metrics_timeseries 0.2.0 retains selected series only
and all interval context points, stores minimal point/status and time/clock/ref
projections, writes compact CLI JSON and reports measured size/actionable limits.
Legacy 0.1.0 saved reader/validation remains. Synthetic 100k timed points/10 series:
27,670,574-byte model and 58,573,608-byte result; actual original-file CLI import
produces 62,074,016 bytes and reopens/queries all 100,000 points. The cold CLI query
is ~7.06s on this host, not a warm-cache improvement. Selecting one of four series
reduces a 40k-point result from 23,424,243 to 5,857,892 bytes. Bucket/attribute sizes
can lower capacity; 64 MiB result/case ceiling stays unchanged. Generic consumers
recognize only the declared metric projection v2 shapes. R13-03 range step and
R13-06 summary-only CSV limitations are corrected in paired documentation.

T-638 is DONE under the original R13 explicit after-P2-remediation permission,
with R13-02/04/03/05/06/07 retained in the ordered P3 queue above. The original
review SHA-256 is `9962f608e4d555d08aede6503a494bdde9b7e779a44b039cf955953666f60ede`;
its unchanged bytes were archived only after intake. Original CONDITIONAL stays;
no independent final PASS or native acceptance is inferred.

Verification: full uncached Go race suite (107 tested packages, 5 no-test packages),
host/Windows amd64+386 vet, frontend state/build, generated TS bindings, macOS
package/signature and Windows amd64+386 production cross-builds. Actual Chromium
renderer produces identical 1600×1000 PNG SHA-256 under 640×480/1280×960 DPR1 and
1920×1080 DPR2; Korean, long integers/fractions, 3-image row ranges, transparent
flamegraph and positive/negative diff were inspected. Font mismatch and abort
publish no PNG. UI mock-transport checks verify native-copy pending/error/success
states and stale preview removal; this is not an OS clipboard write. Clipboard
wire tests cover CF_HTML UTF-8 byte offsets and DIBV5 alpha/orientation. Actual CLI
saved PNG export preserves bytes; all saved formats work and new rendering is
refused. Network-none Linux clean-install CLI process trees have zero INET syscalls,
including new visual spec/validate; failed-connect positive control is detected.

Mac native UI access was blocked by the locked desktop. Native WebView render,
Windows/Office editable-table and PNG/SVG paste, and desktop outbound observation
remain open; the paired VISUAL_ARTIFACTS guides contain concrete Windows steps.
Source/receipts/hashes: `docs/review/evidence/2026-11-12-t636/local.json`.
Reusable schema/style assets are locally committed in projects-assets at `bc86a7c`
and embedded as offline resources; no sibling checkout is required at runtime.

Final git pull at 4542e89 was already current. Commit locally only as requested;
no push. Preserve the unrelated HTTP docs, verifier and original tracker block.
T-639/T-640/T-642 are now TODO on T-638 completion; T-643 stays PENDING on Q02/
selected Apple input, T-644 on T-639 and real fixtures, T-645 on demand/inputs.
Next ready implementation is T-637. Current totals: 219 DONE / 5 TODO / 4 PENDING /
4 REVIEW / 0 IN_PROGRESS = 232 (13 open).

### T-637 perf/constrained BPF implementation activated (2026-11-12)

User requested implementation, commit and push. Initial `git pull --ff-only` at
4864ddd is already current; docs/review has no unprocessed review. Preserve the
unrelated HTTP documents/verifier/tracker block. T-637 is IN_PROGRESS. Implement
bounded producer-qualified perf text and explicit BPF/event/histogram contracts,
retaining exact timestamps/periods, source positions, loss/clock/entity context
and incomplete scheduler intervals. Sampled costs, off/on-CPU endpoints and
aggregate histograms stay distinct. No automatic perf/BPF attachment, privilege
change, tool download, generic binary decoder or lock-contention inference. Reuse
AnalysisResult providers, ProfileSet projection and existing source/report paths.
Original producer/Windows/native acceptance must remain evidence-scoped.
Counts: 219 DONE / 4 TODO / 4 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-636 R14 independent review intake during T-637 (2026-11-12)

Read review 6033d5b in full before archival. Verdict CONDITIONAL (P2 1/P3 6);
original explicitly permits T-636 DONE after R14-01 remediation with P3 queued.
Record all findings before moving original bytes (SHA-256: `8d4fc9a82b842592feb1649ca8795f146908c0c0a66948ee0c9434840fb3349c`).

| Order | Finding | Disposition / dependency |
|---|---|---|
| 1 / P2 | R14-01 | Narrow spreadsheet protection to actual formula/numeric/date risks; distinguish HTML/TSV/header behavior and disclose changed-cell counts. Locally fixed and verified: narrow 0.2.0 policy, per-format counts in artifact/bridge/UI, formula-only header treatment and current table reconstruction for old ZIPs. T-636 closure condition satisfied; independent final PASS not claimed. |
| 2 / P3 | R14-02 | LOCALLY_FIXED; before new diff template expansion: pin delta/name ordering and zero-delta policy; label inclusive columns explicitly. Existing strict comparison values remain exact. |
| 3 / P3 | R14-03 | LOCALLY_FIXED: refuse nonzero offset for profile views or bind a validated cursor; currently only cursor controls profile paging. |
| 4 / P3 | R14-04 | LOCALLY_FIXED: validate required image count ≤5 during spec creation/validation, not only rendering. |
| 5 / P3 | R14-05 | LOCALLY_FIXED: stronger generator/pixel provenance or deterministic SVG comparison. Correct docs now: checksums detect corruption/partial changes, not authenticated authorship or PNG/view equivalence. |
| 6 / P3 | R14-06 | LOCALLY_FIXED: font subsetting/dedup tradeoff. Document practical 5-page ~33 MB bundles / ~16 artifacts / at most 2 such Report Packs under current byte ceilings. |
| 7 / P3 | R14-07 | LOCALLY_FIXED; with R13-06: richer metric value projection for generic views/exports. Correct docs now: 0.2 generic detail/HTML/Report Pack contain point IDs/status and time/clock/ref; exact values remain dedicated metric query output. |

T-637 continues. Original CONDITIONAL is preserved; native WebView/clipboard/Office/
Windows acceptance remains separate. R13-01 was independently reconfirmed at
90k points/10 series and the old 0.1.0 reader; T-638 stays DONE with its P3 queue.

### T-637 implementation/local verification and T-636 R14 P2 closure (2026-11-12)

T-637 is REVIEW. SystemEvidence 1.0.0 / result 0.1.0 separates sampled stacks,
explicit scheduler/event records and aggregate histogram buckets. The file-only
normalizer accepts qualified perf script fields, constrained events JSONL and
pinned bpftrace 0.22.0 histogram JSON shape. Original compressed-byte SHA, exact
integers, line/byte positions, capture/script/version, clock, PID/TID lifetime,
period kind/unit, loss/unwind and histogram unit/population/window/reset are kept.
Unsupported fields/messages/binary input refuse explicitly. Native perf 6.1.190
actually exported the CC0 synthetic perf.data fixture in an isolated network-none
container; this is converter-layout evidence, not host collection or live BPF.

Scheduler projections preserve unknown/open/ambiguous intervals as null/status;
derived complete numeric spans require declared zero loss, known clock/boot/
namespace/lifetime. Off-CPU is not lock contention. Explicit sample-count/period/
on-CPU/off-CPU/runqueue/event-duration selections create owned ProfileSet results
for existing tree/drilldown/report-image paths without relaxed comparison identity.
Histogram-only captures advertise no profile/timeline metric or inferred quantile.
Bounded pages retain inclusive/open bounds, exact counts and original semantics.
Case original navigation binds case revision/result/source hashes and reparses the
source before trusting its event coordinates. Gzip direct text navigation refuses.

CLI inspect/normalize/import/project, opaque Wails bridge, Workspace controls,
offline EN/KO guides and six-fixture standalone bundle are implemented. Late
projection completion after cancellation releases its owned lease. Reusable
fixtures/schema/manifest are committed in projects-assets at 2ca24f8; project
contract copies match byte-for-byte. No automatic tool download, collection,
privilege change or symbol fetch is added.

R14-01 is locally fixed and verified: ordinary paths/URLs/headers are unchanged;
TSV protects only formula/coercion risks, HTML protects formula prefixes while
requesting text format, and receipts/descriptor/bridge/UI disclose changed-cell
counts. Old artifacts without optional receipts still read, with current table
protection rebuilt. T-636 is DONE under the review's explicit P2-remediation
closure permission. R14-02–R14-07 remain ordered P3 follow-ups above. Documentation
now states checksum-only authenticity scope, practical large-font artifact limits
and minimal generic metric projections. Original R14 bytes/verdict are preserved;
no independent verification of the final P2 correction or new PASS is claimed.

Local verification: full uncached Go race suite 109 tested packages + 6 without
tests (115 total), host and Windows amd64/386 vet, frontend state/build, bindings,
macOS package/ad-hoc strict signature, both Windows GUI cross-builds and desktop
guard passed. 10k/20k sample results are 10,536,821/21,146,821 bytes with exact
10/20-billion-nanosecond denominators. Actual CLI/bridge tests cover all source
formats, exports, source forgery/staleness, refusal and cancellation. Isolated
Chromium verified actual React components with real CLI DTOs and synthetic Wails
IPC: qualified capture/errors, explicit metric units, aggregate-only histogram
bounds/counts and canceled late lease cleanup. Linux CLI syscall observer: 17
successful operations + one expected converter denial, zero INET syscalls; failed
network positive control emitted two. Native Wails runtime/clipboard/Office/
Windows and Linux sampling/scheduler/BPF attachment remain unexecuted.

Evidence: docs/review/evidence/2026-11-12-t637/local.json. Existing HTTP documents,
verifier and original tracker history block remain outside this task's commit.
Current totals: 220 DONE / 4 TODO / 4 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(12 open). Next ready implementation is T-639, then T-640/T-641 and T-642 by
priority/dependency; Q01–Q04 and actual producer fixtures still gate their slices.

### T-639 neutral device/serving implementation activated (2026-11-12)

User requested implementation, commit and push while independent review is ongoing.
Initial git pull --ff-only at 662b56d is already current; no unprocessed review
exists at intake. Preserve unrelated HTTP docs/verifier/tracker history. T-639
is IN_PROGRESS. Implement file-only device/serving semantics referencing original
MetricHistory once, explicit host/device/MIG/process/rank mapping and bounded
NVIDIA query CSV normalization, configuration and sanitized runtime/error/request
observations. Preserve missing values, source type/unit, reset/gap and client/
server boundaries; do not invent GPU attribution, causal diagnosis or histogram
quantiles. Q03 still gates version-specific vLLM aliases; neutral bindings do not
activate Q01/Q02/Q04. New reviews must be recorded before archival.
Counts: 220 DONE / 3 TODO / 4 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-637 R15 independent review intake during T-639 (2026-11-12)

Read 1a97ff8 review in full before archival. CONDITIONAL: P2 2/P3 4. Original
explicitly permits T-637 DONE after R15-01 and R15-02 with P3 queued. Original
SHA-256 f631d0d79e510fd81fbedc1f3a1ce1c7d1ae168c37c3fe9dc77778a60bd0169b.
T-637 remains REVIEW during remediation; T-639 may continue after the storage P2.

| Order | Finding | Disposition / dependency |
|---|---|---|
| 1 / P2 | R15-01 | Locally fixed/tested: fresh keyed map, stable bucket-shape code, synthetic keyed fixture checked against upstream 0.22.0 formatter. No native BPF qualification. |
| 2 / P2 | R15-02 | Locally fixed/tested before completing T-639: shared frame/stack pools in 1.1.0 model / 0.2.0 payload, old model/result readers and byte preservation, bounded pool expansion, actionable bytes/limit/processed-record/perf --time errors; 20k/24k 25-frame CLI and saved-result readback pass. |
| 3 / P3 | R15-03 | LOCALLY_FIXED: separate converted binary original_sha256 provenance from the actual imported text hash, then enforce an explicit imported-source hash assertion. Do not compare binary hash to text hash. |
| 4 / P3 | R15-04 | LOCALLY_FIXED: explicit idle/ignored-TID or CPU-scoped idle policy; document waker/idle declaration and unqualified idle noise. |
| 5 / P3 | R15-05 | LOCALLY_FIXED: sleeping/runnable off-CPU selection. Document stackless scheduler projection as unattributed endpoint cost and overlap with runqueue. |
| 6 / P3 | R15-06 | Locally fixed/tested: spreadsheet-text-0.2.1 leaves ordinary decimals unchanged; full-year dotted dates remain protected. Old 0.2.0 receipt counts retain versioned validation; current descriptors/exports use 0.2.1. |

Keep original CONDITIONAL, independent scope and actual Linux/BPF/native Windows/
Office acceptance boundaries. No final independent PASS is claimed.

### T-639 implementation/local verification and T-637 R15 closure (2026-11-12)

T-639 is REVIEW. Implemented neutral InferenceEvidence 1.0.0 and common
inference_serving 0.1.0 with one retained MetricHistory model. Signals reference
series IDs; observation rows reference exact original point IDs. Unbound metrics
remain original-only, with no inferred aliases or synthetic healthy zeros.
Device/MIG/process-lifetime/worker/replica, host/boot/container/pod/rank, explicit
source-bound mappings, model/config/workload/warmup and client/server/gateway
populations remain distinct. Device/MIG/worker data is never replicated or summed.

Implemented limited fixed nvidia-smi.query-csv.v1 source normalization with exact
query fields, declared version/host/boot and optional per-row capture times.
N/A/not-supported/permission-denied are distinct; MiB/percent conversions are
exact and raw values remain. Invalid physical ranges yield null normalized values.
Input is synthetic layout-qualified only: no native GPU/driver/tool execution.
Metric bindings attach a manifest to canonical T-638 metrics without changing
original identities. Q03 specific vLLM aliases remain unselected/unqualified.
Pinned 0.8.5 metric names are synthetic explicit examples, not an automatic adapter.

Scoped gauges/counters/histograms/summary/precomputed rates retain original forms
and boundaries. Counter reset/gap/lifetime checks reuse T-638; histogram interval
counts/rates remain observations and are never multiplied by bucket-unit scales.
Runtime observations are structured records, not arbitrary vendor log parsing;
CUDA/framework/container/host OOM, Xid/ECC/reset and lifecycle stay separate.
Explicit request TTFT/TPOT uses observed token boundaries; <2 output tokens and
missing/reversed boundaries remain null/status. No request reconstruction from
buckets, phase-percentile sums, causal diagnosis, prompt bodies or trace-engine
expansion. Original metric resource attributes still need sanitized input.

CLI inspect/normalize/import/query/open-metrics, opaque Wails methods, Workspace
source assistance, entity/signal filtering, exact original pages and owned metric
history results are implemented. Canceled late creations release their leases.
Common generic providers/HTML/Report Pack/selected visual tables are reused;
CSV retains its existing summary/availability scope. Case persistence and original
source viewer remain existing paths; no new raw vendor-log reparse proof is claimed.
Paired guides and seven-fixture standalone bundle are updated. Shared fixtures and
frame-pool/keyed-hist contracts are committed in projects-assets at cc7fd5c.

R15-01 and R15-02 are locally resolved. SystemEvidence 1.1.0 shares frame/stack
tables and system_evidence 0.2.0 stores compact snapshots; old 1.0.0/0.1.0 readers
remain and an actual CLI fixture generated from archived 662b56d validates without
rewriting old bytes. Pool expansion and references are bounded. Actual synthetic
perf-text CLI at depth 25: 20k samples (35,790,000 source bytes) → 5,942,538-byte
model / 18,016,161-byte result / 20,000,000,000 ns projected denominator; 24k
(42,950,000 source bytes) → 7,134,538 / 21,628,161 bytes / 24,000,000,000 ns.
Unit-generated scale variants have slightly different labels/coordinates and
measured 17,933,051/21,533,051-byte results; evidence distinguishes both fixtures.
R15-06 is locally resolved with decimal regression and version-aware old table
receipt validation. R15-03–R15-05 remain ordered P3 follow-ups, with hash provenance,
idle declaration/noise and stackless/state-overlap limitations documented. T-637
is DONE under original R15 explicit after-P2-remediation closure permission.
Original review bytes/verdict remain CONDITIONAL; no new independent PASS.

Validation: full uncached host race suite 111 tested packages + 7 without tests
(118 total; 1562 passed/7 skipped test completion events), host/Windows amd64/386
vet, frontend state/build, Wails bindings, macOS package/ad-hoc strict signature,
both Windows GUI cross-builds and desktop-only guard pass. T-639 10k/20k-point
saved-model/readback results are 7,685,924/15,405,924 bytes. Actual bridge/CLI
exercises verify all input routes, stale hash/cursor, output refusal, ownership,
exports and visual spec. Isolated Chromium exercises real React with CLI DTOs
and synthetic Wails IPC: capture errors, explicit entity/MIG/absent capability,
large integer, reset/gap and late canceled lease release. Linux network-none
clean-install observer: 24 successful CLI operations + one expected converter
denial, zero INET syscalls, two positive-control syscalls. Native GPU/vLLM, new
Wails/Windows runtime, Office/clipboard and hosted CI remain unexecuted. macOS
linker warnings about newer object deployment targets remain disclosed.

Evidence: docs/review/evidence/2026-11-12-t639/local.json. User's unrelated HTTP
docs/verifier and original 10-line tracker history remain outside this commit.
Current totals: 221 DONE / 3 TODO / 4 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(11 open). Next ready work: T-640, then T-641 and T-642 by priority/dependency.
T-643/Q02, T-644/T-639 completion plus actual fixtures and T-645 demand/input
trigger remain PENDING. Q01–Q04 still gate their corresponding product slices.

### T-640 LLM trace/gateway/lifecycle implementation activated (2026-11-12)

User requested implementation, commit and push; independent T-639 review is
ongoing. Initial git pull --ff-only at 4cd6003 is already current. No unprocessed
review at intake. Preserve unrelated HTTP docs/verifier/tracker history. T-640
is IN_PROGRESS: extend trace_import with a bounded body-free LLM semantic model,
OTLP parent/link/scope provenance, exact nullable timing/token observations,
logical request versus attempt identity and lifecycle populations. Keep nested
work, client/server, retry/fallback and cold/warm/recovery distinct. Q01 product
JSON exports remain gated; no API/authentication/online lookup or quality verdict.
Read and record any arriving review before archival.
Counts: 221 DONE / 2 TODO / 4 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-639 R16 independent review intake during T-640 (2026-11-12)

Read a740ed0 review in full before archival. CONDITIONAL: P2 2/P3 4. Original
explicitly permits T-639 DONE after R16-01/02 with P3 queued. Original SHA-256:
04e67855e472aeb0ba10d51e45077d22750ed6a41367e16e8003518456a2041c. T-639 remains REVIEW during remediation.

| Order | Finding | Disposition / dependency |
|---|---|---|
| 1 / P2 | R16-02 | Fill missing event IDs during explicit input normalization, validate supplied IDs and document a user-writable event example before T-640 reuse. Saved readers remain strict. |
| 2 / P2 | R16-01 | Add explicit declared metadata for untyped Prometheus range series without mutating original identity; derive only with qualified lifetime and disclose signal/name/unit expectations in errors. Keep old readers. |
| 3 / P3 | R16-03 | Accept [N/A] and trailing blank rows; preserve physical source line and timestamp arity; improve bounded column/value error. |
| 4 / P3 | R16-04 | Add explicit retained-series selection to normalization and actionable original-splitting guidance; preserve interval context of retained series. |
| 5 / P3 | R16-05 | Document measured unique-frame worst case versus repeated stacks; no higher admission promise from sample count. |
| 6 / P3 | R16-06 | New request projection reports equal first/last multi-token boundaries as null/status; preserve old payload reader semantics. |

R15-01/02/06 and old 0.1.0 reader were independently reconfirmed; T-637 stays
DONE with its existing P3 queue. No native GPU/vLLM/Wails/Windows or new PASS.

### T-640 implementation/local verification and T-639 R16 closure (2026-11-12)

T-640 is REVIEW. Implemented LLMTrace 1.0.0 and existing trace_import payload
0.3.0 / llm_core 1.0.0, limited trace.llm-json (llm-trace) importer. Legacy trace
0.2.0 remains on its existing reader/screen; raw supported LLM carriers are refused
there with normalization guidance. Semantic profiles are pinned explicit subsets,
not a claim of every GenAI/OpenInference version. Actual OTel Python SDK 1.38.0
carrier plus synthetic edge/convention fixtures qualify the implemented slice.
Q01 Langfuse/LangSmith product JSON and online APIs remain gated.

- Immutable bounded source acquisition, exact uint64 strings/nulls, resource/scope
  and source hash/position, parent and link kept separate; no raw names, arbitrary
  attributes, prompt/completion/tool arguments/status messages in canonical model.
  Supported raw carriers cannot enter new case copies or decoded text indexing.
  This is an allowlisted-contract guard, not arbitrary vendor-content detection
  or retroactive raw-archive sanitization; technical IDs require sanitized input.
- Iterative missing/duplicate/cycle audit and union-subtracted observed child
  elapsed time, with unqualified intervals explicit. No CPU self/causal critical
  path or proximity matching. Explicit logical/attempt namespace, CLIENT→SERVER
  pair qualification, retry/fallback predecessor cycles and token authority avoid
  token sums across boundary or attempts. Model/revision/tokenizer/method and all
  cold/warm/warmup/recovery/unknown populations stay separate.
- Explicit token boundaries/events yield TTFT and exact rational TPOT only for
  >=2 tokens with distinct ordered boundaries. Conflicting/equal/missing times
  are null/status; SSE chunks are not tokens. Recognized lifecycle events retain
  untimed observations. Bounded digest/selector cursors, opaque owned Wails query
  and actual React source/drilldown components reuse generic providers/exports.
  Context inventory is capped at 20; contexts query pages the full matching set.
  Separate context-load and detail-query cancellation scopes preserve fresh
  inventory when a parent handle changes while old filters reset.
- Shared contracts and profile mapping are pinned at projects-assets commit
  5c543abb24ea0edacc8db3e027a676848f576272 with byte-identical embedded mirrors.
  Offline distribution has eight canonical importer fixtures and paired guides.

R16 remediation outcomes (recorded original before unchanged archive movement):

| Finding | Local result |
|---|---|
| R16-02 P2 | Missing event IDs generated at explicit bindings normalization only. Supplied wrong IDs rejected; canonical readers remain strict. Complete writable fixture/example shipped. |
| R16-01 P2 | Only untyped Prometheus range originals accept explicit unit/kind/temporality/monotonic/start declarations. Original core IDs/hash unchanged; semantic/Workspace views disclose declaration. Missing lifetime yields no rate; errors identify signal/name/unit. |
| R16-03 P3 | [N/A]/trailing blanks accepted, actual-row timestamp arity and physical references retained; bounded field/value errors. |
| R16-04 P3 | CLI retained-series-id and bindings retained_series_ids sidecar keep all selected predecessor/reset points. Size guidance explains input splitting before normalization. |
| R16-05 P3 | Paired system guide distinguishes ~4,012 unique 25-frame samples reaching 16 MiB from 20k/24k repeated-stack tests. No sample-count capacity claim. |
| R16-06 P3 | New inference payload 0.2.0 has null/status for equal first/last >=2-token boundaries; old 0.1.0 projection semantics and exact legacy fixture bytes retained. |

R16 original SHA-256 remains
04e67855e472aeb0ba10d51e45077d22750ed6a41367e16e8003518456a2041c.
Its explicit after-P2-remediation closure permission is satisfied: T-639 is DONE.
Original verdict stays CONDITIONAL, with no independent PASS for final fixes.
Native GPU/vLLM, Wails/Windows and clipboard/Office acceptance remain separate.

Local validation receipts, exact command/check scope and source hashes are in
`docs/review/evidence/2026-11-12-t640/local.json`. The legacy raw facade also refuses >64 MiB before partial inspection and rechecks
decoded semantic keys before result creation; existing saved readers are unchanged.
Host full race: 111 packages /
1,575 test cases passed before final bounded-context addition; final task-specific
race and vet cover the last changes. Deep 10k/20k chains: model 9,109,710 / 18,229,710
bytes and result 17,622,749 / 35,252,749 bytes, including saved readback. These are
synthetic layouts, not a general maximum capacity promise. Actual CLI DTO + actual
React components with mocked Wails transport verify namespace gate, separate
parent/link, one attempt, lifecycle, cleared selections and ignored late canceled
responses. Mac package/signature and both Windows GUI cross-builds are local build
evidence; known host C-object 27.2 vs deployment 11 warnings are retained. No native
Windows/WebView/GPU/Office claim. Linux network-none observer covers 28 successful
CLI operations plus expected JFR conversion refusal, with zero AF_INET/AF_INET6
attempts; positive-control failed connection was detected.

Current totals: 222 DONE / 2 TODO / 4 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232
(10 open). Next ready implementation: T-641, then T-642. T-643/Q02, T-644 actual
producer fixtures and T-645 demand/input gates remain PENDING. T-640 awaits its
independent review. Unrelated HTTP docs/verifier/tracker history remain uncommitted.

### T-641 runtime/thread/TA/DB depth implementation activated (2026-11-12)

User requested implementation, commit and push; T-640 independent review remains
in progress. Root/shared git pull --ff-only are current at intake (eadc040 /
5c543ab). No unprocessed root review exists at intake. Preserve unrelated HTTP
docs/verifier and the ten-line tracker history block.

Scope: explicit Python task/worker/allocation and Node report/event-loop inputs,
qualified CPU-TID/nid correlation with process/thread lifetime and clock/window
checks, evidence-based Rust symbols/classification preserving raw identity,
separate observed/inferred thread state and robust multi-dump identities. Deepen
GC pause union and PostgreSQL plan observations using existing common consumer
paths. New bounded runtime_evidence payload uses the common AnalysisResult; its
cross-runtime observations and identity/window queries cannot be represented by
legacy fixed thread DTOs, so old thread/runtime readers stay separate. No live
collector, unsafe pickle/marshal, heap object graph, online DB query or automatic
causal/thread/task attribution. Producer-specific support requires pinned fixtures.

Counts: 222 DONE / 1 TODO / 4 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232.

### T-640 R17 independent review intake during T-641 (2026-11-12)

Read the complete independent review before archiving. Original SHA-256:
8100ae2a998a703d209e242562b55101e82be0eb0e2b6ac22149538c53dd8913.
Verdict CONDITIONAL (P2 1 / P3 4, no P1). It explicitly permits T-640 DONE after
R17-01 remediation with P3 queued. T-640 remains REVIEW during remediation;
T-639 DONE is not reopened. T-640 body exclusion, graph/token/attempt semantics,
fixtures and 115 committed evidence hashes were independently confirmed.

| Priority / dependency | Finding | Required action |
|---|---|---|
| 1 / P2 | R17-01 | Preserve exact rational scalar counter rates through inference unit conversion, including non-integer and scaled rates; never silently drop failed conversions. Preserve old 0.1.0/0.2.0 stored readers and projections via a new payload version. |
| 2 / P3 | R17-02 | Expand best-effort raw-carrier markers for Traceloop/Langfuse/OpenInference body fields and show intake caveat. No product-adapter qualification or complete arbitrary-body detection claim. |
| 3 / P3 | R17-03 | Page only matched output rows; reuse verified graph/model projection by result digest with bounded lifetime/size cache. No cross-owner or stale source reuse. |
| 4 / P3 | R17-04 | Document measured ~35k-span 32 MiB limit for the reviewed shape and original trace splitting; remove nonexistent selection guidance. |
| 5 / P3 | R17-05 | Consistently null TTFT/TPOT when supplied boundaries contradict span/each other; document per-profile timing capability and custom event requirement.  LOCALLY_FIXED. |

Raw review content is preserved unchanged after this queue is recorded. Native
Windows/WebView and actual Langfuse/LangSmith exports remain outside its verdict.


### T-641 local implementation and T-640 R17 remediation (2026-11-12)

T-641 is REVIEW. Added limited runtime.evidence-json importer and bounded canonical
runtime_core 1.0.0 inside the existing AnalysisResult (runtime_evidence 0.1.0).
Python task/worker/tracemalloc/GC, Node reporting/worker/event-loop/handle, Rust panic
and symbol, explicit CPU CSV/thread-dump and PostgreSQL plan sources normalize
before common consumers. Fixtures pin actual CPython 3.14.4, Node v25.9.0 and
rustc 1.94.1 synthetic API executions; JVM/CPU/PostgreSQL inputs are structural
synthetic fixtures, not real OS/DB captures. No live collector, arbitrary Python
execution, heap object graph, SQL execution or invented task CPU/GIL duration.

CPU checks require one explicit pair and full host/boot/namespace/PID/start,
encoded native TID, thread lifetime, aligned clock and containing interval. Missing
context remains candidate; contradictions are incompatible. Source declarations
are not authenticated provenance or causality. Legacy multi-dump 0.2.0 isolates
unqualified observations instead of joining names; old readers are preserved.
Thread 0.2.0 separates observed state from inferred categories. Rust raw symbols
remain identity; bounded pinned demangling only changes display/evidence basis.

Allocation profile projection preserves exact live bytes or block counts for one
entity/source/capture and uses existing tree/query/visual/export consumers; it
refuses cross-capture sums. GC pauses use scoped interval union with concurrent
phases separate. Plan estimates stay separate from actual per-loop rows/time;
null and observed zero differ. Timeline uses only unix clocks. Bounded CLI/Wails/
Workspace queries, immutable ownership, stale cursor/source refusal, common tables,
CSV/JSON/HTML/report pack and durable case round-trips are reused. Offline guides,
fixtures and the demangler's BSD license are bundled, with no runtime download.

| Finding | Local outcome and remaining boundary |
|---|---|
| R17-01 / P2 | Inference payload 0.3.0 preserves exact 20/3 and scaled 20971520/3 rates, refuses failed conversion, and preserves original 0.1.0/0.2.0 saved projections. Actual old-binary rational-rate fixture remains accepted unchanged. |
| R17-02 / P3 | Added Traceloop/Langfuse/OpenInference/input/output markers, escaped/gzip regressions and intake caveat. Best-effort with possible false positives; no complete arbitrary-body detection/vendor adapter claim. |
| R17-03 / P3 | Serialize requested page only; verified model/projection cache is limited to two entries/two minutes/96 MiB conservative JSON charge. Every call checks ownership/file integrity; warm 20k-span query reused the same decoded graph (~6.7 ms in local non-race run). Corruption with restored mtime is refused. No RSS guarantee or cross-process CLI cache. |
| R17-04 / P3 | Documented reviewed ~950-byte/span shape reaching 32 MiB near 35k, original export splitting and no nonexistent in-app oversized-model trace selection. |
| R17-05 / P3 | QUEUED for next versioned LLM duration change: consistently null contradictory first/last/span boundaries, preserve saved 0.3.0 projections. Per-profile custom timing capability documented now. Does not block T-642 under original review's closure permission. |

T-640 is DONE under R17's explicit permission after P2 remediation with P3 queued.
Original CONDITIONAL and its body-exclusion/graph/token/attempt findings remain
unchanged; no independent PASS of this remediation is claimed. Original review
SHA-256: 8100ae2a998a703d209e242562b55101e82be0eb0e2b6ac22149538c53dd8913.
Full verification commands, fixture/source hashes and exclusions are recorded in
`docs/review/evidence/2026-11-12-t641/local.json`. Browser QA uses actual React and
actual CLI DTOs with mock IPC; it does not establish native WebView/Windows runtime.
Host linker emits known SDK 27.2 vs deployment 11.0 warnings; package verification
is not minimum-macOS runtime acceptance. Unrelated HTTP docs/verifier/tracker history
remain excluded from these commits.

Counts: 223 DONE / 1 TODO / 4 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232 (9 open).
Next ready implementation is T-642. T-643/Q02, T-644 actual producer fixtures and
T-645 demand/input remain PENDING. T-641 awaits independent review; no preparation
or redundant re-review of DONE tasks is opened by this status update.

Shared runtime fixture contract committed in projects-assets at `e717b27` (28 pinned file hashes verified); the root embeds a byte-identical prefixed mirror and includes its manifest/schema in the offline fixture bundle.


### T-642 workload/crash evidence implementation activated (2026-11-12)

User requested implementation, verification, commit and push. Root/shared
`git pull --ff-only` are current at 5484b3b/e717b27. T-641 remains REVIEW while
its independent review proceeds; no unprocessed root review exists at intake.
Preserve unrelated HTTP documents/verifier and the original ten-line tracker block.

Scope: bounded normalized load-test and LLM-benchmark exports, explicit arrival/
concurrency/token/warmup/client conditions, fail-closed baseline compatibility,
and body-free hs_err_pid crash context. Keep parser/analyzer/exporter/UI separate
and reuse common AnalysisResult, ownership, providers, exports, durable cases and
offline fixture distribution. No load generator, core/heap engine, online endpoint
integration or automatic crash root-cause/causal correlation. Producer support is
limited to pinned fixtures; other listed candidates remain demand/fixture gated.

Counts: 223 DONE / 0 TODO / 4 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232 (9 open).


### T-641 R18 independent review intake during T-642 (2026-11-12)

Read the entire 105-line review before archive. Original SHA-256: 57e5ebfa7137ab6eacc38df0e4406af32382c1b78d85687d2de1f39b0a8c6829.
Verdict CONDITIONAL (P2 2/P3 2). Explicitly permits T-641 DONE after R18-01/02
remediation with R18-03/04 queued. T-641 remains REVIEW during remediation.
Independent actual Python/Node/Rust/JDK inputs confirmed allocation/GC/event-loop/
CPU context and Rust symbols; all 141 committed T-641 hashes match. T-640's
R17-01/02 were independently confirmed and its DONE status is not reopened.

| Priority/dependency | Finding | Required action |
|---|---|---|
| 1/P2 | R18-01 | Accept optional [name] in actual CPython 3.14 faulthandler headers; retain name and reject thread markers with zero recognized snapshots. Add actual producer fixture. |
| 2/P2 | R18-02 | Add bounded explicit correlation-context input to CLI and desktop multi-dump paths, including process/thread lifetimes; report diagnostics when missing context isolates dumps and disables persistence findings. Keep safe identity matching. |
| 3/P3 | R18-03 | Include GC event path in interval/phase errors, without raw secret-bearing body dumps. |
| 4/P3 | R18-04 | Distinguish decimal JVM nid from 0x-prefixed hex and test actual JDK/macOS header forms. |

Only after recording this ordered queue, archive the unchanged review. Native
Windows/WebView and actual PostgreSQL/py-spy/Memray remain outside its verdict.


### T-642 local implementation and T-641 R18 remediation (2026-11-12)

T-642 is REVIEW. Limited execution.evidence-json / execution_core 1.0.0 /
execution_evidence 0.1.0 uses the common AnalysisResult. Existing fixed thread,
profile, metric and trace DTOs cannot represent benchmark collection conditions,
statistic populations and standalone crash context; this is a domain extension,
not another envelope. Parsers, model validation, analyzers, query/compare consumers
and exporters remain separate. No load generator, benchmark client, core/heap
engine, endpoint call, automated root cause or causal diagnosis is added.

Pinned sources: actual k6 1.3.0 loopback handleSummary; actual LLMPerf statistics/
JSON functions at f1d6bed47e4501b0e371082b41601b59ab55269f with floating synthetic
metrics, not its complete runner/model/GPU execution; synthetic JMeter 5.6.3 JTL
layout; actual Temurin 21.0.10+7 synthetic crash with core creation disabled,
sanitized into a partial hs_err fragment. Unknown producer versions and GenAI-Perf/
nGrinder/Gatling remain fixture gated. Numeric values/units, request/token/error
populations, warmup/arrival/configured versus observed duration, client/target and
model/token conditions are retained. Body-bearing fields, URL/messages, command
lines, environment and memory dump sections are excluded from normalized output;
this does not authenticate raw input or scrub arbitrary archives.

JTL label SHA scopes samples; nearest-rank statistics never merge labels or
percentiles. LLMPerf ITL, overall token throughput and per-request throughput stay
separate. Null/zero and exact decimal/rational values remain distinct. Baseline
comparison checks equal known declared conditions and metric definitions. Unknown,
mixed-warmup, cross-policy or crash comparisons return incompatible without numeric
rows. A zero baseline has no percent change. Verification scope is explicitly
validated_normalized_snapshots_not_original_reimport, not the profile-specific
original-reimport case baseline path. Saved cases reopen exact snapshots; ordered
result/source digests and context/version-bound cursors support reproducible pages.
No statistical significance or automatic regression threshold is claimed.

Custom bounded CLI/Wails/Workspace queries/condition comparison and source digest
pinning reuse common providers, unix sample timeline, visual tables, JSON/CSV/HTML/
report pack and case storage. UI cancellation/parent replacement/target removal
ignore stale responses. Comparison JSON export is explicitly a page. Canonical16
MiB/result64 MiB, 50k samples/8192 metrics and separate row/line/frame/group budgets
fail closed; byte ceilings can be reached first. The offline corpus now has 139
files; its entry ceiling is 256 with original per-file8 MiB/total32 MiB limits.

| Finding | Local result |
|---|---|
| R18-01/P2 | Actual Python3.14.4 named Thread/Current thread headers preserve worker names, 3 snapshots; unsupported/empty thread input is refused. Actual new producer fixture and CLI regression pass. |
| R18-02/P2 | Bounded explicit correlation-context JSON/file in CLI, same optional Wails request/desktop input. Process/format/runtime-ID/thread-start validation; atomic map/slice copies preserve caller data. Unmapped snapshots stay isolated with diagnostics and visible desktop warnings. Actual same-JVM JDK21 dumps warn without context; declared synthetic lifetime context restores persistent BLOCKED1. Declarations are not authenticated starts. |
| R18-03/P3 | Python GC phase/interval errors include /gc_events/N; inverted canonical uint times are shown, malformed secret-bearing values are not echoed. |
| R18-04/P3 | 0x nid uses hex; unprefixed decimal nid keeps os_tid_decimal. Actual macOS JDK21 and earlier hex fixtures pass. Python hex runtime IDs/JVM tid pointers are not native TIDs. |

T-641 is DONE under the original review's explicit after-R18-01/02-remediation
permission. All four R18 findings are locally fixed/verified. Original CONDITIONAL
is preserved; no independent PASS of the final fixes is claimed. Original review
SHA-256: 57e5ebfa7137ab6eacc38df0e4406af32382c1b78d85687d2de1f39b0a8c6829.

Verification: full host race tests, host/Windows amd64/386 vet, frontend build/state,
Wails bindings, Mac package/signature, both production GUI cross-builds, 43 actual
CLI operations (including explicit refusals), common consumers/case bytes/ownership/
source pinning, 20k-sample scale/record/byte bounds, and 44 clean-install Linux
network-none CLI operations (43 successes + expected JFR denial; zero inet calls,
positive-control failed connect detected). Browser QA is actual React with actual
CLI DTOs and mock IPC, not native WebView/Windows acceptance. Local evidence and
staged source hashes: docs/review/evidence/2026-11-12-t642/local.json. Known host SDK
27.2/deployment11.0 linker warnings do not establish minimum OS runtime support.
Unrelated HTTP documents/verifier and the original ten-line tracker block remain
excluded from task commits. Q01–Q04 and actual producer/native gates stay separate.

Counts: 224 DONE / 0 TODO / 3 PENDING / 4 REVIEW / 1 IN_PROGRESS = 232 (8 open).
No implementation is currently TODO. T-643 awaits Q02/actual Apple input; T-644
requires actual producer fixtures; T-645 awaits demand/permitted input/version.
T-642 awaits independent review; existing Windows acceptance items stay unchanged.

Shared execution corpus and R18 runtime fixtures committed in projects-assets at `a3ac18a`; 18 execution and 33 runtime manifest file hashes are verified and embedded as byte-identical prefixed mirrors.

### T-643 implementation started (2026-11-12)

Both repositories pulled with --ff-only; no incoming changes. Root review intake
contains only README.md. Q02 first qualification target intentionally selected as
Ollama 0.40.16 / macOS 27.2.1 / Apple M5 Max, manually obtained read-only API and
macOS snapshots. This does not identify the user production engine. Baseline API
reported no loaded models; an existing small model may be used for a bounded
synthetic local check without downloads. Shared-memory observations will use the
existing metric history model and common AnalysisResult consumers, with explicit
host/boot/local-device identity and unavailable telemetry; NVIDIA identity and
dedicated VRAM assumptions are excluded. Other Apple engines remain unqualified.

### R19 / T-642 independent review intake (2026-11-12)

Original verdict CONDITIONAL, P2 x1 / P3 x3. Independent review confirms JTL
statistics, comparisons, privacy and all R18 fixes. Review explicitly permits
T-642 DONE after R19-01 remediation with R19-02..04 queued. Priority order:

| Finding | Priority | Status | Action |
|---|---|---|---|
| R19-01 | P2 | LOCALLY_FIXED | Replace per-sample JTL repetition with bounded exact per-label distributions; retain exact nearest-rank statistics at ordinary 60k+ loads. Report observed/limit and non-composable percentile limits. Preserve old saved results. |
| R19-02 | P3 | LOCALLY_FIXED | Handle inline hs_err elapsed, native symbols containing spaces, and problematic/native/java section identity or deduplication; qualify actual producer fixture. |
| R19-03 | P3 | LOCALLY_FIXED | Disclose suspect epoch-ms timestamps and inconsistent first-byte/connect durations without silently changing units. |
| R19-04 | P3 | LOCALLY_FIXED | Explicit unverified JMeter-version policy; never require false version declarations. |

R19-01 will be addressed with T-643; P3 follow-ups retain their own queue.
Original review SHA-256: 634e6a0441c3707daa44f5fe0e7c7e476326911c5cb5be7e9a0fac06da1f3978. Recorded before archival; no final-fix independent PASS claimed.

### T-643 implementation verified / R19-01 closure (2026-11-12)

T-643 is REVIEW. The scoped Q02 fixture is actual Ollama 0.40.16 on macOS 27.2.1
(26B131), Mac17,6 / Apple M5 Max after a bounded synthetic 8-token gemma:2b run.
The existing model was not downloaded; the test-loaded model was unloaded, and
/api/ps returned to the empty baseline. This is no declaration of the user
production engine. Three sanitized snapshots, closed schema/manifest and exact
metric model are shared assets and embedded byte-identical mirrors.

The Apple adapter preserves host/boot/local-device, unified/shared memory,
OS/hardware/engine/collector provenance and source windows. System bytes/pressure/
swap, per-model residency estimates and explicit unavailable GPU activity/power/
thermal stay separate. No GPU utilization/dedicated VRAM/temperature/OOM inference,
NVIDIA identity or automatic privileged collection. Existing metrics_timeseries
0.2.0 / MetricHistory 1.0.0, common consumers, exact cases and saved readers remain.

R19-01 is locally fixed: stream exact per-label sorted latency counts/error counts,
1M-row/100k-total-bin/128-label bounds, explicit count/limit/non-composable percentile
errors; ≤1000 rows retains old detail. Large results use execution_core1.1.0 /
execution_evidence0.2.0 with paged individual distribution buckets and no repeated
sample timeline. Newly imported canonical models use the current projection;
saved 0.1.0 projections remain exact. Derived metrics are checked against counts.
60k rows / 3 labels match independent expected counts/errors/min/max/mean/p50/p90/
p95/p99; result is 501965 bytes rather than per-sample repetition. Both limits and
distribution tampering are tested. T-642 DONE is permitted by original R19 after
this remediation; R19-02–04 remain queued, original CONDITIONAL is preserved and no
independent PASS of final changes is asserted.

Verification: full 116-package host race suite; host/Windows amd64/386 vet;
frontend production/state, Wails bindings, Mac package/signature, both production
Windows GUI cross-builds. 29 actual CLI operations cover Apple/large JTL common
imports/queries/JSON/CSV/HTML/report-pack and exact case reopen after cache cleanup.
49 clean-install Linux network-none operations (48 successes + expected JFR denial)
show zero inet syscalls; failed-connect positive control is detected. Browser QA
uses Playwright on the actual React component and actual CLI inventory shaped as
the bridge DTO, with mock IPC: 14 series/42 points, 3 selected series, original SHA
passed to normalization and callback selection checked. It is not native WebView/
Windows acceptance. Initial test attempt found a registry conformance specimen/
payload mismatch; fixed, then the complete race suite passed. Known host SDK27.2/
deployment11.0 linker warnings do not establish minimum-OS runtime qualification.

Counts: 225 DONE / 0 TODO / 3 PENDING / 4 REVIEW / 0 IN_PROGRESS = 232 (7 open).
Next numbered implementation is T-644 after actual PyTorch/Nsight/distributed
fixtures; T-645 remains demand/input-gated. T-643 awaits independent review;
existing native Windows acceptance items and nonblocking review queues remain.
Local evidence and source/evidence hashes: docs/review/evidence/2026-11-12-t643/local.json.
Unrelated HTTP docs/verifier and original tracker block remain outside this commit.

Shared Apple/JTL corpus committed in projects-assets at `460920f`; 25 manifest file hashes verified against byte-identical embedded mirrors.

### T-644 activated / actual producer inputs (2026-11-12)

User explicitly requested implementation, actual input acquisition, validation,
commit and push. Both repositories git pull --ff-only returned current. Existing
HTTP docs/verifier and original tracker block remain out of scope. Review intake
contains README only; T-643 review is ongoing. PyTorch 2.15.0 / NumPy 2.5.4 is
installed in an isolated temporary Python3.13 environment for actual CPU/Gloo
fixture generation. Public NVIDIA training Nsight report is being acquired from
pinned commit b17c40eef23929d08ec42e66667b29ae336ea8ac (LFS original hash available),
then exported with the official tool. No local NVIDIA/CUDA execution claim.
T-644 is IN_PROGRESS; bounds, scoped clock/rank identity and producer qualification
must be verified before any implementation completion or independent approval.

### R20 / T-643 independent review intake (2026-11-12)

Original CONDITIONAL: P2 x1, P3 x3, no P1. Independent actual captures confirm
Apple semantics/source pinning/exact uint64 and R19-01 exactness/scale/tamper
refusals. Original permits T-643 DONE after R20-01 with R20-02..04 queued.

| Finding | Priority | Status | Action |
|---|---|---|---|
| R20-01 | P2 | LOCALLY_FIXED | Version capture selection coverage: excluded model count and selected model match/load state; reveal missing selector without retaining excluded names/digests; retain saved 1.0 normalization. |
| R20-02 | P3 | LOCALLY_FIXED | Qualify single-category physical bounds, tag hygiene and lowercase digest handling without silently rewriting saved identities. |
| R20-03 | P3 | LOCALLY_FIXED | Explain byte-before-row JTL bounds with observed default-column example and actionable acquisition byte errors; split percentiles cannot be combined. |
| R20-04 | P3 | EVIDENCE_PARTIAL | Add actual multi-load and later nonzero swap/pressure / other producer-platform fixtures without forced memory stress. |

R20-01 is prioritized with T-644. Existing P3 queues precede R20-02, R20-03,
R20-04 by dependency; multi-load observations here may cover only part of R20-04.
Original review SHA-256: def911af8d3b5793e153d9959e0d58a3c0f3cc6eebbc52a43a4f4992014e9d4d. Recorded before archival; no final-fix independent PASS.

### 2026-11-12 — native Windows follow-up after pull

Tested code: `3b968ea97cfb81b3ebdb0454549d685dd7e649c0`, based on `d9e62b1d`.
Four newly added CLI tests now reuse the Windows-aware binary builder. Manifest-
pinned original profile fixtures and bundled documentation sources keep LF bytes.
The converter cancellation test waits for an actual child before starting its
five-second cancellation/termination assertion. No product source changed.

Both native amd64 and 386 full suites passed: 116 packages and 1,672 passing
test-completion events each, zero failures, 15 skips each. Both full vets, frontend
state/production build, offline resources and ten registered importer fixtures,
desktop-only/server refusal, registry rename, production builds and 15-second
startup/controller intervals passed. Optional original preset filters and xlrd
1,001-row independent XLS reads passed on both architectures. Cancellation passed
ten additional repetitions per architecture. Local race remains unexecuted because
the host has no C compiler. Original registration and dump policy were preserved.

The actual amd64 production window reached all 21 empty/form routes through native
menus, synchronized Korean/English menu/page labels, and reopened this run's
synthetic saved session through supported `--open`. Selected POST Headers, truthful
Body-off disclosure and native menu reentry restored two rows. The HAR picker
opened, but Computer Use could not address its input field; selection/Analyze is
unverified. MSA Timeline mixed-language labels and raw cancellation error text
remain recorded UI follow-ups. Exact CSS/area, alternate OS DPI, small-window and
full populated/conditional/error acceptance are not inferred from this smoke.

Real Windows service/proxy soak passed 603.731s, 172 cycles and 44,720 requests,
retaining at most 64 rows. This is not multi-hour continuously operated GUI proof.
Both controller reports retain 4/16 PASS and aggregate UNVERIFIED. T-601 driver/
peer/admin, T-611 original-client/cookie, T-613 full parity and T-627
layout/DPI conditions remain open. Counts stay 225 DONE / 3 PENDING / 4 REVIEW =
232 (7 open). Evidence: docs/review/evidence/2026-11-12-windows/README.md.

### T-644 implementation verified / R20-01 closure (2026-11-12)

T-644 is REVIEW. Actual PyTorch 2.15.0 CPU/Gloo two-rank synthetic execution
produced 22 supported events per rank (four tracks each). Exact decimal trace
microseconds become integer nanoseconds; original traces also pass the parser
without retaining raw host/path metadata. Rank/source clocks remain isolated.
No native PyTorch CUDA/NCCL or distributed LLM inference qualification is claimed.

The pinned NVIDIA public training report (original SHA-256
9210e3e3919b3221e191aa61676843bce962804309013123dc59a42678d7efb2)
was exported with official Nsight Systems 2024.2.3.38/schema3.11.1 in a network-none
Linux container. Actual full SQLite has 140917 kernel, 167348 runtime API and 1926
memcpy rows. An explicit 80–80.5ms window selects 20 whole intersecting events;
the reusable sanitized 32KiB subset contains 19 events/three tracks, seven kernels
and independently checked 38944ns kernel sum/union. The official full-run kernel
summary has 50 grouped rows/400 reported statistics; it stays summary-only and
separate from the subset timeline. This is public Tesla T4 report evidence, not
local NVIDIA/GPU execution. Three timeline sources merge to 63 events/11 tracks.

The adapter adds bounded immutable source inspection/pinned normalization, exact
source/rank/process/device/stream/flow identity, operator inclusive/self, separate
per-device kernel sums and interval unions, ambiguous/unresolved correlation,
paged exact query, selected-source SVG with provenance, CLI/Wails/React controls
and common AnalysisResult JSON/CSV/HTML/report-pack/case recovery. Explicit clock
mappings are audit metadata only. No cross-clock alignment, CPU+GPU total,
hardware utilization, instruction tuning or causal critical path is inferred.
Existing trace0.2/LLM0.3 readers remain. Raw inputs are bounded at64MiB, canonical
models32MiB/50k events/4096 tracks/16 sources, query100 rows/256KiB, SVG2000 events/
64 tracks; over-limit operations refuse explicitly. SQLite is read-only with
fixed supported physical tables and bounded cache, never arbitrary SQL.

R20-01 is locally fixed and verified: collector1.1.0 records selected selector
hash and per-snapshot selected-match/excluded model counts. Actual Ollama states
are 1 matched/1 excluded, 0 matched/2 excluded, and 0/0 empty after cleanup. No
excluded model identity or embedding/request body is stored; no model download.
Both test-loaded models were unloaded and the original empty baseline restored.
Legacy1.0 normalized/saved bytes remain unchanged, with read-time unknown-coverage
notice. The desktop action guard now also accepts execution_evidence0.2 so the
R19 large-JTL detail panel mounts. T-643 is DONE under original R20 explicit
P2-remediation closure permission; original CONDITIONAL is preserved. R20-02/03
remain queued; multi-load covers only part of R20-04, while other environment and
nonzero swap/pressure fixtures remain unqualified. No final-fix independent PASS.

Verification: full 118-package host race suite passed; final parser/model/analyzer/
catalog/distribution race checks cover later exact-number bounds and clock-warning
wording. Host and Windows amd64/386 vet, frontend state/production build, generated
bindings, Mac package/signature and both Windows GUI cross-builds pass. Forty-three
actual CLI operations cover common consumers, expected refusals, source hashes,
queries, SVG/XML and byte-identical case recovery after cache cleanup. Clean-install
Linux network-none observation covers57 operations (56 successes + expected JFR
refusal), zero inet syscalls and a detected failed-connect positive control.
Browser QA uses actual React and actual CLI DTOs with mock IPC/save dialogs:
19 Nsight events, exact38944ns busy values, 22 rank1 events, stale-page clearing,
original-SHA normalization handoff and SVG action; no alerts or page overflow.
It does not establish native WebView or Windows runtime acceptance. Known host
SDK27.2/deployment11.0 linker warnings do not establish minimum-OS support.

Final git pull detected five new Windows/CI commits while the local independent
review commit existed. Integrated origin/main by merge without rewriting review
history; SSOT Windows updates and unrelated HTTP work were preserved. Affected
CLI/distribution/converter race checks and the changed Windows386 build runner
were rechecked after integration. Prior Windows evidence is retained separately.

Shared GPU and Apple selection corpora committed in projects-assets at ab1429a;
18 GPU and11 Apple manifest entries match byte-identical embedded mirrors and Git
index blobs. Cross-platform byte attributes are in shared follow-up 431b02b; a
core.autocrlf=true checkout preserves33 root and31 shared corpus/document files
including the binary SQLite, without rewriting upstream notice bytes. The
offline bundle contains11 importer fixtures and85 dependency notices. Local
evidence and staged source/evidence hashes: docs/review/evidence/2026-11-12-t644/local.json.
Counts:226 DONE /0 TODO /2 PENDING /4 REVIEW /0 IN_PROGRESS =232 (6 open).
Next numbered work T-645 remains demand/permitted-input/producer gated; independent
T-644 review, existing P3 queues and native Windows acceptance remain open.
Original HTTP documents/verifier and the ten-line tracker block are excluded.

### T-645 activated / bounded DB and OS snapshot batch (2026-11-12)

User requested implementation, actual inputs, verification, commit and push.
Both repositories pulled --ff-only; current at09413bf /431b02b. No root review
awaits intake; T-644 review is ongoing. This request supplies demand for a bounded
first batch: PostgreSQL16.14 pg_stat_database exports and Linux iproute2 ss -s
summary exports from isolated synthetic network-none producer containers. Existing
MetricHistory/AnalysisResult consumers are reused; import never contacts a DB,
executes SQL, starts tools or grants collection privileges. Counter lifetime/reset,
clock/read skew and network namespace identity must stay explicit. This does not
activate AWR/ASH/Tibero, arbitrary DBs/APMs/profiles/serving engines, Q01 or Q04.
Unrelated HTTP docs/verifier and original tracker block remain excluded.
Counts:226 DONE /0 TODO /1 PENDING /4 REVIEW /1 IN_PROGRESS =232 (6 open).

### R21 / T-644 independent review intake (2026-11-12)

Original CONDITIONAL, P2x1/P3x3, noP1. Review independently confirmed actual
PyTorch CPU/Gloo precision/self/rank/privacy, synthetic Nsight overlap union,
source/cursor/SVG refusals and real Ollama R20-01. Explicitly permits T-644 DONE
after R21-01 with R21-02..04 queued. Findings recorded before archival:

| Finding | Priority | Status | Action |
|---|---|---|---|
| R21-01 | P2 | LOCALLY_FIXED | Correct the raw64MiB pre-window limit claim; provide and actually verify bounded offline SQLite subsetting and profiler schedule guidance. Count retained supported PyTorch events, not ignored metadata, against the event budget. |
| R21-02 | P3 | LOCALLY_FIXED | Disclose missing supported SQLite tables and reject selected-table views consistently. |
| R21-03 | P3 | LOCALLY_FIXED | Separate schema/record limits and report at_least for the first limit breach; raw count alone does not prove retained event count. |
| R21-04 | P3 | LOCALLY_FIXED | Add actual/structural overlapping-stream SQLite and ambiguous correlation/flow fixture coverage without promoting structural CUDA shapes. |

R21-01 is addressed with T-645; existing P3 queue remains dependency ordered.
Original review SHA-256: 64ddac161130ed528acdd8341de149a4e2507f934ef73bf082966f49797994af. No independent PASS of final fixes is asserted.

### T-645 implementation verified / R21-01 closure (2026-11-12)

T-645 is REVIEW for the requested bounded first batch, not all optional adapters.
Actual PostgreSQL16.14/server_version_num160014 runs in an owned disposable
network-none server; five read-only pg_stat_database snapshots cover unknown
stats_reset, reset A, load, reset B, load. Fourteen cumulative counters and one
instantaneous connected-backend gauge give15 series/75 points. Synthetic writes
and resets were confined to this disposable DB, then the container was stopped/
removed. Original DB names/SQL/user/connection strings are excluded from captures.
The reusable read-only SQL recipe is separate from import, which never executes it.

Actual iproute2-6.1.0 ss -s on Linux7.0.14-linuxkit was run before/during/after four
owned loopback pairs; established endpoint counts0/8/0 and21 reported gauges/63
points are retained. All endpoints close and no address/port/process text is
retained. TCP total/closed/orphan counts can include shared-kernel accounting
outside the namespace, so scope does not prove locality. FRAG units are fragment
queues. No rates/packet loss/throughput/RTT/bandwidth/ownership is inferred.

The closed manual contracts require sanitized host/boot/instance/database or
namespace context, exact uint64/ns strings, versions, ordered nonoverlapping read
bounds and4MiB/1..256 snapshots (ss16KiB per summary). PostgreSQL uses actual
stats_reset, including unknown, as counter start; postmaster start only separates
process lifetime and never substitutes for missing reset. Delayed updates/read
skew, collector backend/transaction overhead and reset-only epoch coverage are
explicit. Existing MetricHistory1.0.0 / metrics_timeseries0.2.0 consumers remain:
original-SHA inspection/save, paged query, common JSON/CSV/HTML/report-pack, visual
templates and byte-identical durable cases. No new generic importer/result type.
Other D17 candidates and Q01/Q04 still require their own actual producer fixtures.

R21-01 is locally fixed: raw/expanded64MiB is explicitly checked before selection;
large files need separate preprocessing/shorter capture. The shipped manual
Python/sqlite3 subset helper bounds inputs2GiB/300s, cache4MiB, retained whole
intervals50k and output64MiB, preserves source rowids/required names and emits
original/subset SHA/window/table-count receipt. It never executes source DDL and
refuses selected-table views/virtual/generated columns, sidecars, changed input,
unknown versions and existing outputs. It is never automatically invoked by import.
A114905088-byte padded actual NVIDIA export reduced to24576 bytes with20 exact
whole intersecting intervals, including an API starting before the window; all
rowids/start/end values match an independent sqlite3 query. Nine acquisition/
refusal checks passed. Actual PyTorch schedule guidance is documented; ignored
metadata/python_function no longer consumes the retained supported-event budget.
A100001-ignored-record regression retains exactly22 supported events. R21-03 is
also locally fixed via distinct schema error and at_least overflow wording.
R21-02 optional-table presence/view handling and R21-04 overlap/actual-flow fixture
follow-ups remain queued. Original CONDITIONAL and explicit after-P2 DONE permission
are preserved: T-644 DONE, no final-fix independent PASS claimed.

Verification: full118-package host race suite (1690 passing test events/zero
failures), then affected race reruns for the later R21 changes and remote Windows
URI/test-deadline integration. Host/Windows amd64/386 vet, frontend production/state,
generated bindings, Mac package/signature and both GUI cross-builds pass. Actual
CLI39 operations include exact independently computed rational PostgreSQL rates,
unknown/reset states, socket0/8/0 gauges, stale cursor/output/identity refusals,
common exports,36-series/138-point merge and byte-identical case recovery after
cache cleanup. Clean-install Linux network-none observation covers70 operations
(including one explicit manual helper and expected JFR refusal), zero inet calls
and detected failed-connect positive control. Actual React with CLI DTOs/mock
IPC/save dialogs verifies both format selections, hidden identity overrides,
inspection SHA handoff/selection callback, exact5/3 rows and no alert/overflow.
This is not native WebView/Windows or minimum-OS runtime acceptance; known host
SDK27.2/deployment11.0 linker warnings remain separately scoped.

Both repositories were pulled at start. Later local review commit ce7925a and
remote7a1c1df/97501f8 were merged without rewriting review history; final affected
checks include the Windows SQLite file-URI fix. Shared corpus commit1d07649 has
11 manifest entries mirrored byte-identically with LF checkout policy. Local
source/evidence index: docs/review/evidence/2026-11-12-t645/local.json.
Counts:227 DONE /0 TODO /1 PENDING /4 REVIEW /0 IN_PROGRESS =232 (5 open).
No numbered implementation is TODO; T-645 awaits independent review. Existing
P3 queues, native Windows acceptance and individually gated future adapters remain.
Unrelated HTTP documents/verifier and original ten-line tracker block are preserved
and excluded from task staging/commit.

### 2026-11-12 — merged native Windows verification (97501f86)

Remote T-643/T-644 changes through `09413bfe` arrived before the Windows follow-up
push. Only the unpublished context-fix commit was rebased onto them (`7a1c1dfe`);
published history and unrelated changes were preserved. T-644's CLI regression
had another extensionless executable, now built through `buildBinary(t)`. Actual
Windows Nsight normalization then exposed a file URI drive/authority error.
`97501f86` uses `file:///C:/...` for the owned immutable read-only copy while
retaining query_only, trusted_schema and fixed schema/column checks.

Both complete native Windows suites passed after integration: 118 packages,
1,689 passing test-completion events, zero failures and 15 skips each. Both full
vets, updated frontend state/production builds, 11 offline importer fixtures,
both production builds and 15-second startup/controller runs passed. Each native
controller report still has 4/16 PASS with aggregate UNVERIFIED. The earlier
21-route/Headers/Body screenshots retain their actual pre-T-644 build hashes;
they do not prove new GPU visual or CUDA/NCCL producer acceptance.

Race follow-ups preserve the full 54,001-node model and exact saved bytes, with
a five-minute import wait and independent three-minute load/refusal contexts.
Render sessions each retain their own 120-second limit; catalog setup/selection/
deactivation/import/reopen each get a fresh 20-second context. CI limits package
concurrency to two and permits 20 minutes per cumulative race package. These are
test/CI settings, not changed product limits or reduced assertions.

Clean-host COM registration/replacement/cleanup passed on both hosted
architectures; the installed local vendor registration was preserved. Driver/
admin/peer reach, actual vendor early-bound/session-cookie interoperability,
complete populated/conditional/DPI UI and multi-hour desktop parity remain open.
At the tested 97501f86 snapshot, totals follow the separately integrated T-643
closure: 226 DONE / 2 PENDING / 4 REVIEW = 232 (6 open). This Windows work closes
none of those gates.
Evidence: docs/review/evidence/2026-11-12-windows/merged-validation.json.

Hosted source validation completed successfully at the same `97501f86` commit:
Engine Native run `43217091988` passed all eight jobs; General CI run
`43217092334` passed both jobs. Windows amd64 full race, native 386 full tests,
live/body E2E, forced-crash dump exclusion and original dump-policy restoration
all passed. Linux/macOS full race, both GUI cross-builds, frontend and Mac package
checks passed as well. Earlier superseded failures/cancellations remain recorded;
the final source verdict is PASS. Native layout/driver/vendor/parity boundaries
above remain. Exact steps: docs/review/evidence/2026-11-12-windows/ci.json.
The later independent T-644 review and T-645 implementation arrived while these
receipts were being recorded. Their current task totals/statuses are preserved;
the successful Windows source scope here remains 97501f86, not the later T-645
implementation.

### R13-02 next ready follow-up activated (2026-11-12)

User requested the next task while T-645 independent review is ongoing, then
explicitly requested git pull. No numbered implementation is TODO; the first
ready dependency-ordered P3 is R13-02 (T-638 metric history warm query cache).
Pulled remote Windows receipts c83740a, preserving/restoring the exact original
tracker block and unrelated HTTP docs/verifier. No unprocessed root review exists.
Task counts stay227 DONE/4 REVIEW/1 PENDING; T-638 remains DONE under its original
conditional closure and T-645 remains REVIEW. This follow-up does not reopen a
completed gate or activate another vendor/native acceptance task.

### R22 / T-645 independent review intake (2026-11-12)

Original CONDITIONAL, P3x3, noP1/P2 or completion prerequisite. Review explicitly
permits T-645 DONE with nonblocking findings queued. Independent owned actual
PostgreSQL capture14 derived counters and ss capture84 values matched independent
calculations; actual Nsight subset and PyTorch R21-01/03 remediation confirmed.
Findings recorded before archival, respecting older dependency-ordered P3 work:

| Order | Finding | Priority | Status | Action |
|---|---|---|---|---|
| 1 | R22-03 | P3 | LOCALLY_FIXED | Explain NULL reset/crash-recovery conservatism and permitted maintenance-window options; provide a separate read-only wrapper collector without automatic resets or DB access during import. |
| 2 | R22-01 | P3 | LOCALLY_FIXED | Explicit unknown-to-known epoch transition status/rate policy; preserve prior unknown epoch and saved revisions. |
| 3 | R22-02 | P3 | LOCALLY_FIXED | Qualify numbackends>=1 and nondecreasing reset timestamps or disclose/refuse invariant violations explicitly. |

Original review SHA-256: 1b9ac84b86142063407a4002470e255f4dfbc58e63d2f9145ef350a49596a1a7. Original verdict remains CONDITIONAL, not PASS. No new native Windows/WebView or other producer qualification follows.
R13-02 remains the active next ready follow-up; R22 items follow older queues.

### R13-02 implementation verified / R22 queue closure (2026-11-12)

Completed the next ready nonblocking follow-up. Desktop metric inventory/query/
materialization pins the result owner and re-reads/hashes all managed snapshot
bytes on every request. Only an exactly verified immutable digest can hit the
private model/interval index. Full registered payload validation and interval
preparation happen on a cold miss, while warm queries binary-search selected
series end-time ranges and copy only bounded outgoing rows. No original producer
file is automatically reimported. Exact values, reset/gap/overlap/predecessor
context, histograms, untimed selection, cursor identity and canonical order match
the stateless path. Both0.1/0.2 saved results remain exact, and CLI stays stateless.

Cache retention is limited to2 entries/192MiB conservative structural admission
(core JSON twice plus private index accounting, not measured RSS), fixed2-minute
eligibility with lazy expired-entry cleanup and LRU byte/count eviction. Larger
valid preparations run uncached. Cold admission is cancellable/serialized; failed
or canceled preparations do not cache. Shutdown clears entries. Neither caller-
mutated inventory/page/refs/buckets nor same-size/restored-mtime byte tampering
can alter/bypass cached evidence. Lease release retains ordinary revisions per
existing policy; explicit cleanup removes bytes and warm caches cannot resurrect
them. Shared Read now uses the same bounded context-aware byte verification.

Actual local100k-point regression:61469739 result bytes,162991130 structural cache
charge, cold inventory/preparation4.160083375s and three25-row warm pages73.319542ms
total, each including a full byte hash. No universal latency, RSS or native UI/
Windows claim. Prepared/stateless pages match for neutral, actual PostgreSQL/
Linux and SDK explicit/exponential histogram cases. Ownership/cleanup, concurrent
caller isolation, byte/count eviction, fixed expiry, stale cursor/digest,
cancellation, legacy saved bytes and100k reuse pass. Full host119-package race,
host/Windows amd64/386 vet, frontend production/state, bindings, Mac package/
signature and both GUI cross-builds pass. Actual CLI39 operations retain exact
exports/case recovery, and clean-install network-none70 operations (including
manual helper and expected JFR refusal) record zero inet calls; failed-connect
positive control is detected. Offline resources match their source docs/tools.

R22 independent review arrived during this work: noP1/P2, explicit DONE permission
with three nonblocking P3 findings queued. Recorded before archiving unchanged;
T-645 is DONE, R22-03/01/02 follow older queues, original CONDITIONAL preserved.
No new PASS of this cache change or other queued findings is asserted. Counts
stay228 DONE/3 REVIEW/1 PENDING=232 (4 open native Windows acceptance tasks).
R13-04 is next ready, followed by R13-03/05/06/07 and the existing ordered review
queues. T-638 remains DONE under original closure permission; it is not reopened.

Git pull initially integrated Windows receipts c83740a while preserving the exact
original tracker block. Later6317b80 changed only LF attributes for the shipped
manual subset helper and was merged without changing the tested Go source bytes.
Local evidence and committed-source index:
docs/review/evidence/2026-11-12-metric-cache/local.json.
Unrelated HTTP docs/verifier and original ten-line tracker block remain excluded.

### 2026-11-12 — T-645 native Windows follow-up

Imported the qualified PostgreSQL16.14 and Linux iproute2-6.1.0 captures on native
Windows 11 amd64 and 386. Both architectures pass all seven affected Go packages
(261 passing test-completion events including subtests, zero failures, four skips
each), vet, 61 actual CLI commands each, exact query paging/rational rates/reset
semantics, Unicode/space paths, exports, 36-series/138-point merge and byte-exact
snapshot-only case save/export. Frontend state/build, both GUI builds and eleven
offline fixture importers pass. Actual amd64 Wails app verifies both source
formats, saved result loading, gauge query and next-page rendering. Native 386
startup survives 15 seconds; the smoke subsequently forces its owned process
after the graceful-close timeout, so graceful shutdown is not claimed.

Fixed a Windows checkout issue in `6317b804`: the manual Nsight subset script
was converted to CRLF while its embedded offline copy remains LF. Pinning the
source script to LF restores resource byte parity without changing script logic
or bundle hashes. Fresh committed `core.autocrlf=true` checkout and resource
checks pass. The tool also passes Unicode-path subsetting and seven negative
checks with unchanged original bytes.

Tested binaries: `c83740a1`; T-645 source: `b36d83e0`. Their entire engine tree is
identical to `6317b804`. Upstream source CI passed all eight Engine Native jobs
and both general CI jobs, including hosted Windows full race/live/body/crash and
policy-restoration checks. The checkout fix has separate CI runs and is not
silently assigned the upstream verdict. No new local full-race/live/body/crash,
complete DPI/layout or multi-hour parity claim. T-645 remains REVIEW pending
independent review; other optional adapters remain gated. Evidence and exact
scope: `docs/review/evidence/2026-11-12-t645-windows/README.md`.

The Windows receipt above records its tested pre-review snapshot. Current T-645
status is DONE under R22 explicit permission; it does not qualify the later R13-02
cache on native Windows. Final documentation-only integration:8060406; tested Go
source bytes and all119-package race/CLI/build results are unchanged.


### R13 user-directed follow-up batch — 2026-11-12

Requested order R13-03 → R13-05 → R13-06 → R13-07 overrides the ordinary P3 queue for this batch; R13-04 remains next afterwards. Each item is committed separately, with push only after all four. Initial git pull --ff-only: already up to date at ae79e4b. Unrelated HTTP work remains excluded.

R13-03 LOCALLY_FIXED: range parser emits normalizer 1.1.0; adjacent known end timestamps separated by more than declared positive step set gap independently of value/conflict status. No interpolation, inferred cadence or gauge rates. Saved normalizer 1.0.0 retains its original projections. Explicit merge preserves the newest input revision. Targeted model/parser/analyzer/service tests verify gaps, reordered/duplicate/nonfinite observations, optional cadence, window predecessor context, prepared/stateless equality and saved-result validation. Original independent verdict and native Windows acceptance remain unchanged.

R13-05 LOCALLY_FIXED: newly parsed sources record normalizer 1.2.0. Entire equal-time cohorts compare exact observation payloads excluding only IDs/refs; identical exports remain separate duplicate_observation rows, mixed cohorts are all conflicting_timestamp. No derived delta/rate/buckets or arbitrary predecessor from either cohort; next cumulative point is ambiguous. Delta overlap suppression preserves duplicate/conflict classification. Legacy normalizer 1.0.0/1.1.0 remains readable with unchanged semantics; shared schema explicitly enumerates revisions. Race tests cover gauge/cumulative/delta/histogram duplicate cohorts, third-writer conflicts, exact integers, provenance retention, legacy and existing parser/analyzer/catalog regressions. No push yet.

R13-06 LOCALLY_FIXED: dedicated metriccsv exporter (CSV 1.0.0) selects whole intervals, emits exact scalar/rational/bucket strings plus statuses/gaps/refs/series/source identities and digest/revision/selection. CLI history-csv defaults to saved selection; Wails ExportMetricIntervalCSV uses owned verified cache reads and local no-overwrite publication. Desktop action uses a save dialog, cancellable request scope and explicit spreadsheet text-import guidance. Free-text formula prefixes are escaped; validated numeric strings retain exact bytes. Output >64MiB fails as a whole. Unit/integration/actual CLI tests check SDK histograms, exact values/rates, source coordinates, deterministic CSV, windows, cancellation, legacy read, stale digest and overwrite refusal. Frontend production/state and regenerated Wails bindings validate surface wiring. Generic CSV remains summary-only; native Windows runtime acceptance is separate.

R13-07 LOCALLY_FIXED: shared source/result picker searches exact identity strings, renders at most50 series/page, filtered select-all/clear spans every matching page and retains hidden selections with explicit counts/clear-all. UTF-8 selection JSON above60KiB disables save/query/materialize/CSV with guidance, reserving query envelope/cursor space. Inspection resets old selection; canceled/late inspections cannot restore it. State tests cover10k-series search, pagination bounds, hidden-ID preservation/idempotence and exact UTF-8/large-integer identity search. Playwright observed actual React with actual CLI10k inventory and mocked IPC:5000 matches selected across100 pages, hidden selection retained on filter change, page2, source-save ID handoff, all10k result selection disables actions, exact9007199254740993 display, digest-bound CSV/query requests and ignored late canceled inspection. This is browser/mock IPC evidence; real Wails methods are separately integration-tested and native Windows acceptance remains separate.

Batch verification complete: full host race 120 packages/1709 passing test events, zero failures; host/Windows amd64/386 vet; frontend production/state; regenerated bindings; final Mac package/signature and both Windows GUI cross-builds; final embedded resources/distribution checks. Actual CLI39 snapshot/case operations +11 semantic operations and 10k-series import/selected CSV pass. Network-none71 operations including interval CSV report zero inet calls with failed-connect positive control detected. Receipt: docs/review/evidence/2026-11-12-r13-followups/local.json. All four requested items are locally fixed; no independent PASS or new native Windows runtime claim. Push follows the fourth commit only. Counts remain228 DONE/3 REVIEW/1 PENDING=232; T-638 stays DONE with original CONDITIONAL verdict preserved. R13-04 is the next ready task after this requested batch.


### R13-04 source diagnostics — 2026-11-13

Started with git pull --ff-only: already up to date at91e1f03. Existing HTTP docs/verifier and the exact original ten-line tracker block are excluded from this task. No unprocessed independent review is present; parent CONDITIONAL and T-638 DONE stay unchanged.

R13-04 LOCALLY_FIXED: original OTLP/snapshot/range numeric and JSON type errors now expose bounded SourceError Code/Path/Rule plus decoder-supplied decoded byte offset, with no input value, filename, raw strconv or Go type in the message. Paths include actual OTLP data-kind containers and nested attribute/bucket fields; gzip uses decoded JSON coordinates. Snake_case resource_metrics explicitly requests lowerCamelCase resourceMetrics. Error-only streaming location and one-point model localization preserve valid IDs/refs/normalization and warm-query costs. Existing source/model resource refusal guidance and cancellation remain their own contracts.

Exact-range policy remains whole-file refusal, not rounding/dropping/new point status: errors return no partial model and publish no normalized/result file. Saved model/normalizer and payload0.1.0/0.2.0/AnalysisResult contracts are unchanged. Capture semantic diagnostics stay unchanged; their decoder type errors now share the bounded boundary.

Targeted race/parser/model/analyzer/catalog and actual CLI/Wails tests pass:1000 points with index777 invalid, exponent limits/signed overflow/precision/shape/snake_case, nested attributes, deterministic exponential sides, deferred snapshot validation, gzip, cancellation, long private labels/values and no publication. Actual CLI39 operations compare six valid models byte-for-byte with the prior binary and keep saved0.1.0 bytes unchanged. Final verification complete: full race 120 packages/1721 passing test events, zero failures; host/Windows amd64/386 vet, Mac production/signature, both Windows GUI cross-builds, unchanged frontend state tests, embedded resources/notices and desktop-only guards. Network-none76 operations record zero inet calls, including5 diagnostic/no-publication refusals; failed-connect positive control detected. Receipt: docs/review/evidence/2026-11-13-r13-04/local.json. No independent PASS or new native Windows runtime acceptance is asserted. Counts stay228 DONE/3 REVIEW/1 PENDING=232. Next ready task: R14-02.


### Review follow-up batch — 2026-11-13

User authorized the21 remaining review follow-ups in listed order, one commit/item and push only after the complete batch. Start git pull --ff-only: already up to date at4b0743d. Existing HTTP docs/verifier and original ten-line tracker block are excluded; native Windows acceptance4 tasks and Q01–Q04 remain separate. Order: R14-02 → R14-03 → R14-04 → R14-05 → R14-06 → R14-07 → R15-03 → R15-04 → R15-05 → R17-05 → R19-02 → R19-03 → R19-04 → R20-02 → R20-03 → R20-04 → R21-02 → R21-04 → R22-03 → R22-01 → R22-02.

R14-02 LOCALLY_FIXED: Exact delta_abs/delta/name ordering, zero-delta policy pinned before paging, target-only frame names and inclusive columns; comparison percentages stay in the exact API. Targeted visual/profile integration tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R14-03 LOCALLY_FIXED: Nonzero profile_top/profile_flamegraph offsets fail explicitly before result acquisition; profile.cursor remains the paging contract; diff/provider offsets and historical views unchanged. Targeted integration tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R14-04 LOCALLY_FIXED: NewSpec/Spec.Validate reject more than5 images before rendering; required_images included in CLI validation and refusal guidance. Exact5/over5 boundary tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R14-05 LOCALLY_FIXED: Supported SVGs are deterministically regenerated from frozen view/spec/pinned font and byte-compared on build/read; consistently rehashed modified SVG refused. Historical unsupported renderers remain hash/structure-only. PNG equivalence/authorship explicitly not certified. Artifact/tamper regressions passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R14-06 LOCALLY_FIXED: Resolved reviewed font tradeoff by retaining full self-contained Unicode font; no lossy subsetting or external dependency. Actual compressed/expanded byte sizes exposed in descriptors/UI; concrete Report Pack byte refusals and practical five-image16-library/2-pack capacity documented. Save/list byte consistency tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R14-07 LOCALLY_FIXED: Added compiled paged metrics:intervals detail via verified cache, disposable rich HTML interval/bucket projection and hash-linked Report Pack presentations. Original saved JSON remains byte-identical; presentation-budget fallback explicitly disclosed in packs. Provider/paging/HTML/pack/immutability tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R15-03 LOCALLY_FIXED: Separated upstream original_sha256 provenance from optional imported_sha256 assertion; exact input bytes verified before parsing and saved-model validation, gzip hashes compressed bytes. Old captures unchanged. Parser/legacy/catalog and hash-mismatch tests passed; shared optional schema/manifest mirrored. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R15-04 LOCALLY_FIXED: Explicit ignored_tids excludes declared lifetimes from interval costs/profiles while preserving raw observations/coordinates; idle policy documented. Scheduler header waker no longer requires an irrelevant observed-entity declaration; payload/sample targets remain strict. Scheduler/legacy/ignored-policy tests passed; shared schema mirrored. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R15-05 LOCALLY_FIXED: Added explicit off_cpu/sleeping and off_cpu/runnable projections with distinct identities/capture context; aggregate off_cpu unchanged. Stackless endpoint attribution and runqueue overlap disclosed; exact partition/legacy/projection tests passed. No inferred lock or blocked call tree. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R17-05 LOCALLY_FIXED: New explicit token-boundaries.1.1.0 policy nulls both TTFT/TPOT for supplied contradictory span/token boundaries; old models without policy keep saved projections. Profile capability/custom event table added. Five contradiction cases, canonical revision/privacy and service integration passed. Shared optional schema mirrored; repaired prior system README/offline mirrors in their own unpublished commits. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R19-02 LOCALLY_FIXED: HotSpot inline elapsed/full bounded native symbols and optional problematic/native/java section identities implemented; old frames remain readable. Actual isolated Temurin21.0.10+7 synthetic crash captured with core dumps disabled; only allowlisted sanitized excerpt/producer hashes shared. Parser/canonical/privacy/actual CLI and full catalog tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R19-03 LOCALLY_FIXED: Pinned reusable epoch-ms plausibility policy; suspect units warned without guessing, contradictory first-byte/connect values nulled with sample warnings and aggregate affected-row counts even in distribution mode. Elapsed distributions and legacy saved semantics preserved. Parser/distribution/catalog tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R19-04 LOCALLY_FIXED: Explicit JMeter unverified-version opt-in preserves true/unknown producer version and source qualification; UI guidance and closed schema mirrored. Full execution/parser/catalog tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R20-02 LOCALLY_FIXED: New Apple capture input enforces individual VM/capacity and residency/size bounds, sanitized local tags, control-free text and lowercase digests. No saved identity rewriting. Full parser/catalog tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R20-03 LOCALLY_FIXED: JTL byte-before-row capacity guidance added with reviewed default-column observation; acquisition keeps typed exact budget details and adds population/split-percentile guidance. Original/decompressed byte refusals state exact/lower-bound counts. Parser/catalog tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R20-04 EVIDENCE_PARTIAL: Actual 12-sample/two-second multi-load capture added; matched/excluded1/1 throughout, owned models unloaded and empty state restored. swap_used0/raw pressure1 is not elevated-pressure evidence. Explicit qualification matrix retains missing natural nonzero-swap/elevated-pressure input, other platforms and long-duration qualification. Parser/catalog tests passed. User input requested; full batch push remains held until required input is resolved. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R21-02 LOCALLY_FIXED: Nsight now discloses present/absent coverage for all four supported optional tables; absent never proves zero activity. Selected-name views/virtual tables refused before querying; generated columns remain refused. All optional-table missing/view regressions and catalog passed; legacy saved warnings preserved. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R21-04 LOCALLY_FIXED: Added shared deterministic overlapping-stream SQLite mutation with independent exact sum77888/union51968 ns, duplicate launch candidates and ambiguous/orphan/valid Chrome flow endpoints. Structural qualification explicitly distinguished from actual CUDA/NCCL. Parser/analyzer/catalog passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R22-03 LOCALLY_FIXED: Documented NULL-reset/crash-recovery conservatism and owner-approved maintenance alternatives. Shipped separate explicit fixed-SQL BEGIN READ ONLY psql wrapper in offline tools; no automatic reset/import-time DB access. Actual disposable network-none PostgreSQL16.14 three-snapshot capture retained unknown epochs and no rates; server removed. Parser/catalog/distribution tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

R22-01 LOCALLY_FIXED: Pinned metric normalizer1.3.0 for new PostgreSQL imports: known current start after unknown predecessor gets epoch_transition_from_unknown, no inferred earlier continuity/delta/rate; next same-epoch derivation and known-reset behavior retained. Saved1.0/1.1/1.2 semantics preserved. Revision, parser, analyzer and catalog tests passed. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

Final batch verification (2026-11-13): host race120 passing packages/1759 passing test events,0 failures; host/Windows386/amd64 vet, frontend state/production, TypeScript bindings, final Mac production/signature and both Windows GUI cross-builds pass. Final source/resource checks include111 shared manifest entries,85 dependency notices and catalog/distribution checks after guide-only finalization. Actual CLI26 operations (including nonzero rational diff ordering and node ties), clean-install network-none86 operations/0 inet syscalls with failed-connect positive control detected, mock-IPC React checks and all4 unrelated user changes preserved. Receipt: docs/review/evidence/2026-11-13-review-followups/local.json. Twenty follow-ups locally remediated; R20-04 remains EVIDENCE_PARTIAL for missing authorized natural nonzero-swap/elevated-pressure input.21 per-item commits include its partial-evidence commit. No batch push until that input condition is resolved; no independent PASS/native Windows acceptance claimed. Numbered task counts remain228 DONE/3 REVIEW/1 PENDING=232.

R22-02 LOCALLY_FIXED: New PostgreSQL captures require numbackends>=1 and nondecreasing known stats_reset even across unknown observations; explicit refusal codes, no correction. Parser/model/catalog tests and full120-package/1759-event race suite passed. Final batch receipt/paired guide status finalized; R20-04 still partial and push held. No new independent PASS; original parent verdicts and existing saved revisions are preserved.

### Comprehensive review disposition (2026-11-13)

Read the complete unprocessed review at `1cbc77b` before disposition. Its target
is `f69daa2`, verdict CONDITIONAL (P2 3 / P3 7, no P1). All RS-01–RS-10 actions
are recorded in the ordered backlog and Task Registry before moving the original
to `docs/review/done/2026-11-13_claude-code_T-628-T-645_comprehensive-review.ko.md`.
Original SHA-256: `e2b0af0ee53e235ca5ec23ec401e1ee88a2afba93901e4982d89fafe634da8aa`.

RS-01 is addressed by registering nine separately gated residual-scope tasks
T-654–T-662 and limiting the T-644/T-645 titles to their bounded first batches.
RS-03/RS-10 share T-646; RS-02 is T-647; RS-04–RS-09 map to T-648–T-653
as detailed above. The immediate RS-09 stale queued statements were removed from
both execution guides and their bundled mirrors. Broader guide/history cleanup
and the missing English MSA guide remain explicitly open in T-651.

The user's current request authorizes commit and push, superseding the earlier
batch-publication hold. R20-04 remains EVIDENCE_PARTIAL and T-662 PENDING; no
missing evidence, producer policy or independent PASS is inferred from publication.
Existing T-628–T-645 DONE statuses stand under the original closure permissions.
Counts reconcile to 249 tasks: 228 DONE / 7 TODO / 0 IN_PROGRESS / 3 REVIEW /
11 PENDING (21 open). No new reusable diagnostic asset was authored; shared
assets remain referenced. Existing HTTP guide/verifier/history edits are preserved
outside this commit.

Disposition validation: 249 unique six-column Task Registry rows and status counts
reconcile; all seventeen new tasks have registered dependencies, a cycle-free
new-task graph, backlog entries and explicit gates. Changed guide/index local
links, review SHA-256, EN/KO bundled guide byte identity and preservation of all
four unrelated working-tree changes pass. `python3 scripts/build-offline-assets.py --check`, `go test ./internal/distribution -count=1` and `git diff --check` pass.
These are disposition/resource checks; no new runtime feature, full-suite rerun,
hosted CI or independent re-review is claimed.

### T-646 architecture/offline gates activated (2026-11-13)

User requested all T-646–T-662 work, one commit after each completed task and a
single push after the complete batch. Existing HTTP docs/verifier/history remain
outside task commits. No new fixture/environment was supplied; build isolated
actual-producer environments where feasible. CUDA/NCCL still requires suitable
hardware; no synthetic fixture is an actual GPU execution claim. T-646 is
IN_PROGRESS. Architecture source checks and Linux disconnected CI are being
implemented; no hosted-run success is inferred before publication.

### T-646 locally completed (2026-11-13)

Added AST-based production import boundaries across OS-specific source files:
parsers cannot depend on analyzers/exporters; exporters cannot depend on parsers
or analyzers except two exact legacy report files; models/statistics/contracts
remain independent. Domain packages cannot import networking or process execution
APIs; pure net/url and net/netip are allowed. The exact JFR discovery exception
allows LookPath only, not Command or execution. New canonical analyzers cannot
import source parsers. Stale exceptions fail. Table probes cover denied new paths.

Added a Linux CI job that builds the standalone CLI, prepares the observer, then
runs the shipped observer in network-none with a failed-connect positive control
and archives observations. Script/Dockerfile changes trigger the workflow. Direct
and facade localconvert offline refusals are tested before missing-tool/input/temp
allocation. Actual local Docker run passed 70 operations (including expected
converter denial), zero inet syscalls for observed operations and two positive-
control inet syscall lines. This is Linux CLI/manual-helper evidence, not desktop
or Windows traffic coverage. Architecture/distribution tests and diff check pass.
T-646 DONE for gate implementation/local execution; hosted CI and independent
re-review have not run. No push until the complete requested batch.

### T-647 locally completed (2026-11-13)

Common producerpolicy.Select owns strict-default/explicit-opt-in handling across
nine pinned paths: k6, LLMPerf, JMeter, PyTorch, Nsight SQLite/summary, Ollama,
PostgreSQL and iproute2. Actual or unknown version and optional unverified_version
qualification survive source normalization, canonical storage and reports.
Source-owned versions cannot be overridden; PostgreSQL string/numeric versions
must agree. Collector/schema requirements, shape/value/budget checks and canonical
saved-input option refusal remain. No version range is implicitly qualified.
Unverified benchmark baseline comparison fails closed with an explicit reason;
metric/GPU comparison stays unavailable under the existing contract.

Generic CSV previously omitted canonical source tables. ExportPresentation now
adds a bounded, disposable source qualification appendix for CSV and HTML; old
qualified results and canonical saved bytes remain unchanged. Tests exercise all
nine changed-version paths, unknown version, default refusal, invalid structures,
conflicting declarations, canonical readback, CSV/HTML and forged qualification.
All focused normal/race suites, registry/catalog contract fixtures, architecture,
distribution/input-assistance checks and offline mirror/manifest checks pass.
Mutated producer versions are structural tests, not actual new-version runtime
qualification. Original saved fixtures still match their projections. Shared
schemas/docs/manifests and ASSET_INDEX updated in projects-assets b836c27;
digest-pinned embedded mirrors and offline fixtures refreshed. T-647 DONE locally;
no new independent PASS, hosted CI or native-platform qualification. Push held for
completion of the complete user-requested batch.

### T-648 locally completed (2026-11-13)

All nine Strict wrappers now delegate directly to wirejson.StrictWithLimits with
the caller-owned byte limit. Source allowances no longer pass through the metric
model's hidden 32 MiB ceiling; saved-model Decode retains each model's default.
Shared SHA/JSON hash, canonical uint64, lowercase digest and control-free UTF-8
text helpers preserve established identity encoding and domain validation rules.
Structural depth/string/key/array bounds remain independently enforced. Domain
byte refusals retain their actual caller limit and error domain.

The new strict decoder preserves number precision, rejects unknown struct fields,
duplicate keys/trailing data/invalid UTF-8, and publishes output atomically. It
clones ordinary caller defaults before overlay so a failure cannot modify maps;
custom versioned UnmarshalJSON readers retain their own replacement semantics.
Full regression exposed presentation-default and zero custom-reader compatibility
cases during development; both are fixed and covered by direct/CLI regressions.
A 42 MiB k6 summary with fourteen ignored custom metrics now imports under its
64 MiB source allowance; a 32 MiB caller limit still returns EXECUTION_BYTES_BUDGET,
and oversized strings still refuse. Exact byte and integer boundaries, unknown
fields, precision, default preservation, map rollback and fixture hashes pass.
Final whole-module Go race suite and go vet ./... pass. EN/KO parser design is
updated. T-648 DONE locally; no independent re-review/hosted/native acceptance
claim. No push until the complete requested batch is finished.

### T-649 locally completed (2026-11-13)

Compiled result descriptors now publish versioned panel IDs and supported action
tokens. The backend owns payload eligibility; all 15 canonical payload variants
map to eight renderer protocols. The React panel dispatches on validated panel
capabilities instead of hardcoded type/payload lists. Unknown result/panel/protocol,
missing action and denied compatibility remain generic, with a bounded disclosure.
Adding a panel requires exhaustive TypeScript component mapping; a Go contract test
compares the complete compiled registry against the frontend renderer inventory.
Source/distribution/legacy descriptors inherit the same panel declaration.

Imported metadata cannot grant capabilities. Descriptor registration validates
panel/action tokens and protocol; descriptor inventory, capability DTO and cached
projection return independent copies. Adversarial mutation, metadata spoofing,
unknown payload and invalid panel tests pass. Existing provider operations and
AnalysisResult storage/versions remain unchanged. Generated Wails bindings,
frontend state regressions/production build, focused analyzer/registry/provider
Go suites and targeted race checks pass. Initial test inventory minimum was
corrected to the observed 15 payload variants; the final contract check passes.
EN/KO data model documents the bounded DTO contract. Native WebView/Windows,
hosted CI and independent re-review remain separate. T-649 DONE locally. T-652
moves from PENDING to TODO on completed T-648/T-649 prerequisites. No batch push.

### T-650 locally completed (2026-11-13)

Added direct boundary suites to six previously indirectly tested model packages:
execution/GPU/inference/LLM/runtime/system. Valid pinned old fixtures, fixed 16/32MiB
model byte boundaries, unsupported versions/fields, missing references/contexts,
invalid timing/privacy/conversion and duplicate identifiers are exercised directly.
A reusable test-only runner and 12 retained malformed models avoid relying only
on parser coverage. SVG tests independently parse XML, retain escaped malicious
text without script/handler nodes, enforce source/query scope and refuse 2001
events instead of truncating the 2000-event selection ceiling.

Added bounded property fuzz targets for hs_err/JTL/k6 JSON, PyTorch JSON/Nsight
SQLite, ss summary text, and jstack/faulthandler. Each target accepts only bounded
input, publishes no invalid models and verifies canonical readback on success.
Four five-second/two-worker runs pass; these are short smoke runs, not exhaustive
security or long-running fuzz evidence. Interesting inputs and full raw replay
cases are retained in diagnostics.parser-adversarial.v1 (29 files plus manifest)
and a test-only hermetic mirror. Explicit LF/whitespace attributes preserve the
actual perf exporter bytes without changing the sample to satisfy diff checks.
Shared corpus/index committed as projects-assets 6f3a7c7.

Retained parser replay verifies signed-int64 overflow refusal, independently
expected known-epoch deltas 7 and 1, imported-text hash mismatch and null TTFT/TPOT
for a first token before its span. Model/SVG/parser suites and targeted race,
R13/R14/R15/R17 local regressions, interval CSV, corpus hashes/mirror, architecture
and focused vet checks pass. English/Korean independent revalidation requests
include fresh-checkout/SHA instructions, inputs, probes and evidence boundaries:
docs/{en,ko}/LOCAL_DIAGNOSTICS_REVALIDATION.md. Independent R13–R17 verification
is explicitly requested, not performed or renamed PASS. T-650 DONE for the local
test/corpus/request slice; original CONDITIONAL and native/runtime gates stand.
No push until the complete requested batch.

### T-651 current-behavior guides — 2026-11-13

Reconciled EN/KO user guides and bundled mirrors with current behavior, retained old section anchors, and added the English MSA servlet-dispatch guide. Both servlet defaults and self-time accounting were verified against implementation. Task history and independent verdicts remain in tracker/reviews. Markdown local links/fences, deterministic bundle checks and distribution/input-assistance regressions passed. Receipt: `docs/review/evidence/2026-11-13-t651/local.json`. Native and independent acceptance are separate.

### T-652 bounded acquisition and query reuse — 2026-11-13

Extracted ingestion snapshot/cleanup/verification and bounded cancellable gzip helpers plus digest-bound cursor/page components. MetricHistory migrated with unchanged normalized bytes and established cursor wire format; domain validation, admission and owner/hash verification stay in their original layers. Added EN/KO existing-result reuse assessment gate. Affected race suites and vet passed, including compressed SHA parity, overflow and cancellation. Receipt: `docs/review/evidence/2026-11-13-t652/local.json`.

### T-653 legacy percentile disclosure — 2026-11-13

Inventoried linear interpolation, floor index, nearest rank and bounded reservoir policies in shared versioned asset `measurement.legacy-percentile-policies.v1` (`50abed5`) and compiled mirror. Six new legacy result families disclose additive percentile_method metadata; saved readers and numeric algorithms are unchanged. Independent fixed p95 examples are 85.45 / 3 / 100 for [1,2,3,100]. HTTP diff retains source summaries without assigning missing legacy policies. Normal/race analyzer and statistics regressions and vet passed. Receipt: `docs/review/evidence/2026-11-13-t653/local.json`.

### T-655 actual cProfile flat adapter — 2026-11-13

Activated from the user instruction to create missing environments. Selected CPython cProfile 3.14.4 after importer-matrix review; owner-generated actual synthetic function export and shared schema/collector now qualify cprofile-flat.v1. The parser reuses ProfileSet/profile_evidence and source query/export consumers; preserves exact self seconds with nonadditive cumulative/call labels, no inferred stacks/timeline/CPU and no pickle input. Capture options remain strict. Race/model/parser/architecture, saved readback, CSV/HTML and frontend build passed. Receipt: `docs/review/evidence/2026-11-13-t655/local.json`.

### T-657 actual SQLite snapshot adapter — 2026-11-13

Selected SQLite 3.53.0 under the user instruction to create inputs/environments. Manual read-only fixed-PRAGMA collector and strict sqlite-pragma-snapshots.v1 adapter reuse MetricHistory/common consumers. Actual synthetic DB capture retains 4 gauges × 3 samples without transaction/rate inference; data_version is connection-local and sequential reads are not atomic. Shared asset `5ec81d7`; exact raw-value/common CSV/HTML, version/kind refusals, affected race suites and vet passed. Receipt: `docs/review/evidence/2026-11-13-t657/local.json`.

### T-658 actual Docker Linux CPU tick adapter — 2026-11-13

Selected Docker Desktop Linux7.0.14-linuxkit / USER_HZ100 under user environment-creation authorization. Closed linux-proc-stat.v1 adapter and manual collector reuse existing MetricHistory. Actual 3×10 gauge capture plus network-none collector execution preserve shared-kernel scope, guest overlap and nonmonotonic iowait; no percentage/rate/process attribution inferred. Shared asset `5e7412b`; raw equality, common exports, scope/version refusals, affected race and vet passed. Receipt: `docs/review/evidence/2026-11-13-t658/local.json`.
