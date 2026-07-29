# H-RG4 — Windows Live-Capture UI and E2E Second Re-Review (T-581)

- **Review group:** H-RG4 (live UI and Windows E2E)
- **Target task:** T-581 Windows live-capture UI, progress/finalization events,
  acceptance fixture, Windows E2E, and client harness
- **Reviewer:** claude-code (independent second re-review)
- **Date:** 2026-07-29
- **Verdict:** `CONDITIONAL` — ten of the twelve previous findings are verified
  resolved and two are resolved in mechanism, but the previous blocking finding
  R2 is still open and one new blocking finding (S1) sits on the same finalized
  handoff R1 was raised against
- **Predecessors:**
  - `docs/review/done/2026-07-28_claude-code_H-RG4_windows-live-capture-ui-e2e-review.md`
    (`CONDITIONAL`, L1–L14)
  - `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-re-review.md`
    (`CONDITIONAL`, R1–R12)
- **Evidence base:** full read of the remediation commits `151f65c`
  (`fix(t581): remediate H-RG4 backend re-review`) and `dedda0a`
  (`fix(t581): complete Claude H-RG4 UI handoff (R8/R9/R11)`) against the working
  tree at `dedda0a`, plus the surrounding sources:
  `internal/analyzers/httpcapture/analyzer.go`,
  `internal/capture/proxy/server.go`, `internal/capture/stream/pipeline.go`,
  `internal/capture/session/manager.go`, `internal/capture/store/store.go`,
  `internal/capture/redact/redact.go`, `internal/capture/procmap/resolver*.go`,
  `internal/capture/acceptance/evidence.go`, `internal/capture/types.go`,
  `cmd/archscope-app/captureservice.go`,
  `cmd/archscope-app/captureservice_test.go`,
  `cmd/archscope-app/capture_windows_e2e_test.go`,
  `cmd/archscope-app/testdata/t581_live_capture_acceptance.json`,
  `frontend/src/state/liveHttpCapture.ts`,
  `frontend/src/components/LiveHttpCapturePanel.tsx`,
  `frontend/src/pages/HttpCapturePage.tsx`, `frontend/src/i18n/messages.ts`,
  `scripts/verify-windows-live-capture.ps1`, and the H-RG4 sections of
  `docs/en|ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` and
  `docs/en|ko/USER_GUIDE.md`.

## Scope

This gate re-validates the live-capture renderer contract and the Windows
acceptance package for T-581 against the H-RG4 checklist and the `PASS`
criterion in `docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`
§H-RG4:

> **PASS:** Windows supported-tier scenarios for browser/curl/JVM/Electron, page
> re-entry, long sessions, and recovery pass E2E; H2/QUIC/pinning limitations
> never look like successful semantic capture.

plus the two conditions H-SEC2 bound to this tier (`SEC-10` body-capture
preflight, `SEC-17` explicit unknown-attribution opt-in). Every R-finding from
the predecessor re-review is re-verified against the current code. New findings
are raised only where they sit inside the H-RG4 acceptance path.

No disposition is recorded here; that belongs to the implementing agent. No
repository file outside this document was modified during this review.

## Verification performed

Run from `apps/engine-native` on darwin/arm64 (Go 1.26.5, Node/npm from the
frontend workspace):

| Command | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./internal/capture/... ./cmd/archscope-app/... ./internal/analyzers/httpcapture/...` | clean |
| `go test ./...` | pass |
| `go test -race -count=2 ./internal/capture/...` | pass |
| `GOOS=windows go vet ./internal/capture/... ./cmd/archscope-app/... ./cmd/archscope-engine/...` | clean |
| `GOOS=windows go build ./cmd/archscope-engine` | pass |
| `npm run test:state` (frontend) | pass |
| `npm run build` (frontend) | pass |
| i18n key-parity scan of `messages.ts` | 997 distinct keys, every key paired, 84 `liveCapture*` keys |
| `go test -overlay` probe — mixed live session provenance | **R1/R3/R5 confirmed resolved** |
| `go test -overlay` probe — failed passthrough tunnel | **R4 confirmed resolved** |
| `go test -overlay` probe — aborted in-flight row and session grade | **reproduced S2** |
| `go test -overlay` probe — crash-recovered session readback | **reproduced S3** |

All probes were supplied through `go test -overlay` from the session scratch
directory; no repository file was added or modified for them.

## Verdict summary

The Codex backend follow-up and the Claude UI handoff are, on the code paths
they touch, correct and well tested. Finalized live sessions now derive their
provenance and aggregate fidelity from the stored rows; pinning failures and
failed tunnels keep their honest mode, fidelity, attribution, and coverage; the
capture-time redaction summary is persisted and read back; attribution resolves
once per accepted connection and confirms both PID and process start time; the
renderer consumes a versioned engine contract instead of local literals and
discloses a contract it cannot honour; the fidelity emphasis is derived from
`isDecodedLiveFidelity`; every engine state token now renders through paired
EN/KO label maps; aborted rows are persisted; and the paired plan matches the
work.

Two things block a `PASS`.

1. **The finalized fidelity card still tells the user, in prose, that the
   session is imported foreign-tool evidence.** R1 removed
   `har_import`/`foreign_tool`/`semantic` from the metadata block, and the three
   machine-readable fields on the card are now honest. The explanatory sentence
   printed directly beneath them is not: `httpCaptureFidelityHint` is a hardcoded
   HAR string — "This is imported foreign-tool evidence, not a live proxy
   capture. Timing phases come from the HAR exporter." — rendered unconditionally
   for every result including the live-finalized one (S1). The condition the
   previous re-review set for R1 is met in the data and violated in the sentence
   that interprets it.
2. **R2 is unchanged.** No Windows evidence artifact exists in or is referenced
   from the repository; `work_status.md` and the plan both record the artifact as
   still required. The harness itself is now genuinely broad — h2-only, pinning,
   a 5,000-request long session, a WebView2 CDP page-re-entry check, and a
   separate recovery session readback, all fail-closed — but the criterion asks
   for evidence, and there is none to inspect.

Below those, one high-severity finding concerns the same redaction disclosure R5
fixed, surviving on the recovery path that the R2 artifact is required to
include (S3), and three medium findings concern the honesty of the finalized
aggregate grade (S2), the untranslated finalized card (S4), and the missing
privacy boundary for the artifact R2 mandates (S5).

## Disposition of the previous findings

| Prev. | Severity | Status | Evidence |
|---|---|---|---|
| R1 | B | **Resolved** | `BuildLive` replaces the hardcoded block with `liveProvenance` (`analyzer.go:83-95,261-283`): mode is the single observed mode or `mixed`, `observation_point` is `proxy`, fidelity is `weakestFidelity` (`:285-315`), and per-token `capture_mode_counts`/`fidelity_counts`/`coverage_counts` are emitted (`:197-202`). `AnalyzeCaptureSession` calls it (`captureservice.go:212`). Probe on a live session containing one decoded MITM row and one failed-interception row returned `capture_mode=mixed`, `fidelity=unsupported`, `observation_point=proxy`. Regression: `TestBuildLiveDerivesWeakestFidelityAndArchScopeProvenance`, `TestCaptureServiceSessionCanBeAnalyzedAfterStop`. The prose half of the same card is S1 |
| R2 | B | **Not resolved** — harness extended, evidence still absent | `verify-windows-live-capture.ps1` now adds the pinning probe (`:337-354`), the h2-only JVM probe (`:356-408`), a ≥1,000-request long session (`:410-434`), a CDP page-re-entry check (`:438-465`), and a separate recovery-session readback (`:564-567`), and writes a `schemaVersion: 3` artifact with a SHA-256 sidecar (`:583-604`). No artifact, path, or checksum exists anywhere in the repository; `work_status.md:74` and the plan (`:298`) both still list it as outstanding. `capture_windows_e2e_test.go` is unchanged and still one plain-HTTP `GET` under a comment claiming it "exercises the same explicit-proxy path used by browser/curl/JVM/Electron clients" (`:17-20`) |
| R3 | H | **Resolved** | The handshake-failure path passes the resolved `process` and a mode that does not assert interception (`server.go:335-343`, `CaptureModeNotCaptured = "proxy_not_captured"` at `:36`); `failureTransaction` now takes mode, fidelity, and process and derives `Coverage` from the process (`:499-517`). Probe row: `mode=proxy_not_captured fidelity=unsupported state=failed` retained and persisted. Regression: `TestTLSHandshakeFailureRetainsAttributionWithoutClaimingMITM` |
| R4 | H | **Resolved** | The tunnel dial failure emits `CaptureModePassthrough`/`FidelityUnsupported` (`server.go:301-306`). Probe: progress and finalized rows both `proxy_passthrough` / `unsupported`, `state=failed`, coverage derived from the process. Regression: `TestFailedPassthroughTunnelPreservesTerminalModeAndCoverage` |
| R5 | H | **Resolved on the stop path**; see S3 for the recovery path | `Manager.Stop` writes `pipeline.RedactionSummary()` into the manifest (`manager.go:280`, `store.go:294-300`); `AnalyzeCaptureSession` reads it back (`captureservice.go:206-212`) and `acceptance.Build` carries it into the evidence (`evidence.go:92-97`). Probe: a session with one redacted query reported `redaction.applied=true`, `counts={"query_value":2}`, `summary.redaction_applied=true` |
| R6 | M | **Resolved in mechanism**; measurement still unrecorded | Attribution now runs once per accepted connection in `ConnContext` (`server.go:134-138`, `processForRequest` at `:395-400`), so a keep-alive page load resolves once rather than once per request. Regression: `TestPlainProxyResolvesProcessOncePerClientConnection`. No new latency/CPU figure is recorded; the only measurement on file (H-COV1, 4.8%p) predates both the second table read and the connection-scoped cache |
| R7 | M | **Resolved** | `Resolve` re-reads the owner table and requires the same PID **and** the same process start time before labelling `confirmed` (`resolver.go:63-71`, `processStartTime` at `resolver_windows.go:116-123`). Regressions: `TestResolverConfirmsStablePIDAndStartTime`, `TestResolverRejectsPIDReuseAcrossStartTimes`. `docs/en/USER_GUIDE.md:166-174` restates the guarantee to match |
| R8 | M | **Resolved for the renderer block**; the pattern reappears in the harness block (S6) | `capture.LiveCaptureContract` is versioned and served to the renderer (`types.go:16-36`), the renderer resolves it, falls back on an unknown schema, and discloses the mismatch (`liveHttpCapture.ts:190-271`, `LiveHttpCapturePanel.tsx:199-211,656`), the row cap comes from the contract (`:880`), and the fixture's `renderer` block is compared field-by-field against `capture.DefaultLiveCaptureContract()` (`captureservice_test.go:211-219`) |
| R9 | L | **Resolved** | `liveFidelityTone` is derived from `isDecodedLiveFidelity` (`liveHttpCapture.ts:443-457`) and is what the table's fidelity cell styles itself from (`LiveHttpCapturePanel.tsx:103-107,862-868`), so the helper now gates a real product presentation |
| R10 | L | **Resolved** | The paired plans record the Codex and Claude follow-up as complete and carry no stale handoff sentence (`docs/en|ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §H-RG4) |
| R11 | L | **Resolved for the live panel**; see S4 for the finalized card | Transaction state, session state, CA state, and attribution all resolve through closed token sets and paired EN/KO label maps (`liveHttpCapture.ts:465-560`, `LiveHttpCapturePanel.tsx:113-150,852-855`); unknown tokens say so instead of leaking the wire value |
| R12 | L | **Resolved**, with a side effect (S2) | `abortInflight` persists every aborted row and counts it in `observed`/`captured`/`persisted` (`pipeline.go:414-452`), so `aborted_transactions` and the evidence `state:aborted` counter are populated. Regression: `TestCloseAbortsEveryInflightTransactionState`. SEC-17 is preserved because unattributed progress rows never enter the live ring (`manager.go:190-195,474-477`) |

## Acceptance-criteria assessment

| H-RG4 item | Assessment | Evidence |
|---|---|---|
| Start/stop, session state, CA install/remove, first-use warning | Implemented | `LiveHttpCapturePanel.tsx`; CA controls gated on `firstUseAccepted` and an inactive session |
| Process tree, stable live list, terminal in-progress reconciliation | Implemented | `boundedDistinct`, `failLive`, `abortInflight`, state column with paired labels |
| Scroll-respecting follow mode, batched updates, row cap | Implemented | Row cap now supplied by `LiveCaptureContract` |
| Persistence/drop/backpressure/disk/recovery status with explicit drop warning | Implemented | Nine stat tiles, drop-share line, distinct drop/kernel-loss strings |
| Honest fidelity, coverage, passthrough, unattributed warnings | Met for the live table | Pinning and failed tunnels now reach the table with honest mode and fidelity (R3/R4) |
| Same-page finalized-session lazy loading after stop | **Not met** | The metadata is honest (R1) but the card's explanatory sentence still describes the session as an import (S1); the aggregate grade can read `pending` for a finished session (S2); the values themselves are untranslated raw tokens (S4) |
| `SEC-17` explicit unknown-attribution opt-in | Enforced and correctly disclosed | `pipeline.go:125-131`, `manager.go:190-195,474-477`, `activeUnattributedPolicy` |
| `SEC-10` bodies remain omitted | Met | `BodyStorage: "omitted"` on every emitted and progress transaction; `metadataOnly` strips previews and refs |
| Windows E2E for browser/curl/JVM/Electron, re-entry, long sessions, recovery | **Not met** | R2 — the harness now covers the scenarios, but no run has been archived; QUIC remains unexercised (S7) |

## Findings

Severity: **B**locking, **H**igh, **M**edium, **L**ow. No disposition is
recorded here; that belongs to the implementing agent.

### S1 (B) — The finalized live session is still described as imported foreign-tool evidence

`HttpCapturePage` renders `FidelityBanner` for every analysis result it holds,
including the one `onLiveFinalized` dispatches after a live session stops
(`pages/HttpCapturePage.tsx:128-143`, `:260`). The banner prints the three
metadata fields R1 fixed and then, immediately below them, a fixed hint string
(`:435`):

```ts
httpCaptureFidelityHint:
  "This is imported foreign-tool evidence, not a live proxy capture. Timing phases come from the HAR exporter.",
```

(`i18n/messages.ts:1013-1014`; the Korean table carries the same claim at
`:2072-2073`.) The string is not conditional on provenance, so one click after
stopping a Windows live capture the user is told that what they are looking at
is not a live proxy capture and that its timing phases came from a HAR exporter.

Consequences:

- The previous re-review's condition 1 — "no finalized live-capture session may
  present `fidelity: "semantic"`, `capture_mode: "har_import"`, or
  `observation_point: "foreign_tool"`" — is satisfied in the metadata and
  violated in the sentence that explains the metadata. A user reads the prose,
  not the enum.
- It directly contradicts the field printed 30 pixels above it
  (`observation_point: proxy`) and the `capture://<id>` source label, so the card
  is now internally inconsistent in a way it was not before R1 was fixed.
- The second sentence is also factually wrong for a live session: live timings
  are measured by the proxy, not imported, and `timings.importedHar` is absent —
  which is exactly why `CAPTURE_TIMING_INCONSISTENT` can never fire on this path.

Suggested direction: the banner already receives `meta`; select the hint from
`meta.observation_point` (or from a dedicated provenance token) so the HAR
sentence stays on the HAR path and the live path gets its own — decoded-wire
proxy capture, ArchScope-measured timings, bodies omitted. A state test that
feeds `extractCaptureMeta` a live result and asserts the live hint key would
close it.

### S2 (M) — One aborted in-flight row downgrades the whole finalized session to `pending`

R12 persists in-flight rows at close, but `abortInflight` changes only `State`,
`EndedAt`, and `Error` (`stream/pipeline.go:416-429`); the row keeps the
progress-time grade `pending` — which the project's own remediation for R4
identified as a grade that must not survive on a terminal row. `weakestFidelity`
then returns the first present grade in the order `unknown`, `pending`,
`decoded_wire`, `semantic` (`analyzers/httpcapture/analyzer.go:297-315`), so a
single aborted row decides the session.

Reproduced with an overlay probe feeding `BuildLive` 200 decoded rows and one
aborted in-flight row:

```
{
  "capture_mode": "proxy_mitm",
  "fidelity": "pending",
  "fidelity_counts": { "decoded_wire": 200, "pending": 1 },
  "aborted": 1,
  "total": 201
}
```

Their own regression fixes the input: `TestCloseAbortsEveryInflightTransactionState`
persists rows built with `Fidelity: "pending"` and asserts only the state.

Consequences: a finished session of otherwise fully decoded traffic reports
`fidelity: "pending"` — "not yet determined" — on the fidelity card, in the
Workspace copy of the result, and in the acceptance evidence
(`fidelity:pending` counter). Any session stopped while a browser keeps a
connection open hits this, which is the normal case for the long-session
scenario. The direction of the error is conservative (it understates capture
quality rather than overstating it), which is why this is not blocking, but it
makes the aggregate grade nearly useless for real sessions and it puts a
non-terminal grade on a terminal artifact.

Suggested direction: give aborted rows a terminal grade at abort time (the same
`unsupported`, or a dedicated `not_captured`), or exclude `aborted` rows from the
aggregate and report them through the existing `aborted_transactions` summary
and the per-token counts.

### S3 (H) — A crash-recovered session reports that nothing was redacted and that nothing was captured

The redaction summary is written to the manifest in exactly one place, inside
`Manager.Stop` (`session/manager.go:280`), and so are the capture stats (`:300`).
A session whose process dies before `Stop` therefore has neither. `acceptance.Build`
substitutes an empty summary and a zeroed `Stats` in that case
(`acceptance/evidence.go:79-83,92-97`), and `AnalyzeCaptureSession` substitutes
the same empty summary (`captureservice.go:206-210`).

Reproduced with an overlay probe that captures three transactions, lets the
store flush, and then reads the session back from a fresh service without ever
calling `Stop`:

```
manifest after simulated crash: state=running redaction=<nil>
RecoverCaptureSessions -> reports=[{Recovered:true DiscardedBytes:0 Records:3}]
acceptance.Build -> rows=3 redaction={Applied:false Rules:[] Counts:map[]}
```

Every one of those three rows was redacted before it reached the disk
(`stream/pipeline.go:463-470` runs on every record), and their URLs on disk
contain `token=%5BREDACTED%5D`. The evidence nevertheless states `applied:false`,
and any UI reading it renders "No sensitive fields matched the redaction policy."
The stats block of the same evidence reports `observed: 0`, `captured: 0`,
`persisted: 0` next to three stored rows.

This matters more than an ordinary edge case because the R2 artifact is
*required* to contain a recovery block: the harness reads
`-RecoverySessionPath` through the same `acceptance-evidence` command
(`verify-windows-live-capture.ps1:486`) and only checks that the state is
`recoverable` and that at least one row exists (`:564-567`). The artifact that is
supposed to close R2 will therefore carry a recovery section that asserts no
redaction was applied and that zero transactions were observed, alongside the
rows that disprove both.

Suggested direction: persist the redaction summary and the capture stats
incrementally (the store already flushes the manifest on state, stats, and
finding writes, so a periodic write alongside the existing `StatsInterval` tick
is cheap), and distinguish "no rule matched" from "the record did not survive" —
an absent summary should read as unknown, not as clean.

### S4 (M) — The finalized fidelity card prints raw engine tokens and hides the distribution R1 added

The card renders `meta.fidelity`, `meta.capture_mode`, and
`meta.observation_point` verbatim (`pages/HttpCapturePage.tsx:429-431`). Before
R1 those were three fixed English words; now they are engine tokens the user has
never seen — `mixed`, `proxy_mitm`, `proxy_not_captured`, `proxy_passthrough`,
`unsupported`, `pending`, `proxy` — printed untranslated in both locales, on the
same screen where R11 has just replaced every raw token in the live table with a
paired EN/KO label.

The metadata block now also carries `capture_mode_counts`, `fidelity_counts`, and
`coverage_counts` (`analyzers/httpcapture/analyzer.go:197-202`), which is what
makes `mixed` interpretable. Nothing renders them, so a session reported as
`mixed` / `unsupported` gives the user no way to see that 200 of 201 rows were
decoded.

Given that this card is the finalized honesty surface the `PASS` criterion is
about, it should get the same closed label map and a per-mode breakdown as the
live table.

### S5 (M) — The artifact R2 requires has no stated privacy boundary

The harness embeds the complete product evidence in the artifact it writes
(`verify-windows-live-capture.ps1:596-598`), which is up to
`store.MaxFetchLimit` = 2,000 rows carrying URLs, hosts, paths, status codes,
process names, and process attribution, plus `sessionPath` resolved to an
absolute local path (`:590`) and the operator's OS version string (`:588`). The
artifact is written with default ACLs, unlike the `0600` the engine uses for its
own export.

R2 asks for this file to be committed to — or referenced from — a public
repository. Nothing in the plan, the user guide, or the harness states what may
be captured during an acceptance run, that the artifact must be produced against
loopback fixtures only, or that `sessionPath` and the row URLs must be reviewed
before archiving. The condition that closes R2 should not be the step that
publishes an operator's browsing metadata.

Suggested direction: state the acceptance-run scope in the plan (loopback
fixture origins only), strip or relativize `sessionPath`, and either bound the
archived row set or state explicitly that the archived rows are fixture traffic.

### S6 (L) — The fixture's `harness` block reproduces the unbound-contract pattern R8 removed from the `renderer` block

`fixture.Renderer` is now compared field-by-field against
`capture.DefaultLiveCaptureContract()`, which is the right shape. The block added
in its place is not: `requiresArchivedArtifact`, `unsupportedH2`,
`unsupportedPinning`, `pageReentry`, and `recovery` are asserted `true` against
nothing (`captureservice_test.go:231-239`), and
`fixture.Harness.SchemaVersion` is compared to `acceptance.HarnessSchemaVersion`
while the PowerShell script writes its own literal `schemaVersion = 3`
(`verify-windows-live-capture.ps1:585`) that no test reads. The Go constant and
the script can drift apart, and the five booleans describe the script rather than
constrain it.

### S7 (L) — QUIC is still asserted in the fixture and exercised nowhere

`t581_live_capture_acceptance.json:39-43` declares a `quic` unsupported-tier
scenario. There is no QUIC probe in the harness, no QUIC test in the Go tree
(`grep -ri quic` over `internal/capture` and `cmd/archscope-app` returns
nothing), and the harness raises no contradiction for its absence. QUIC is named
in the `PASS` criterion; the honest position — that a UDP transport never reaches
an explicit HTTP proxy at all, so the limitation is invisibility rather than a
false grade — is stated in neither the harness nor the coverage disclosure the
panel builds.

### S8 (L) — The page-re-entry check is locale-bound and does not read back from the product

The CDP expression finds the navigation entry by the literal English string
`"HTTP capture"` (`verify-windows-live-capture.ps1:441`) and then counts
`document.querySelectorAll("tbody tr")` across the whole document (`:456`). On a
Korean UI the button lookup fails and the harness throws for the wrong reason; if
any other table is mounted on the page, the row count is inflated. It is also the
only scenario in the harness whose outcome is observed in the DOM rather than
read back from the product store, which is the property that made the rest of the
harness trustworthy.

### S9 (L) — `httpcapture.Build` is now unreachable from product code

With `AnalyzeCaptureSession` on `BuildLive` and the HAR path on `BuildParsed`,
the exported `Build` (`analyzers/httpcapture/analyzer.go:69-81`) has no non-test
caller. It is the function that hardcodes `har_import`/`foreign_tool`/`semantic`
for a caller-supplied transaction list, which is precisely the shape R1 was
about. Leaving it exported keeps the R1 defect one call away.

## What was verified as sound

- **R1's remediation is correct end to end, not just at the unit level.** A live
  session driven through the real proxy — one decoded MITM request and one failed
  TLS interception — produced `mode:proxy_mitm=1`, `mode:proxy_not_captured=1`,
  `fidelity:decoded_wire=1`, `fidelity:unsupported=1` in the acceptance evidence
  and `capture_mode=mixed`, `fidelity=unsupported`, `observation_point=proxy` in
  the finalized analysis. The empty-session case is covered too and cannot claim
  imported semantics.
- **R3 and R4 are covered by real proxy-level regressions, not fixtures.** Both
  new tests drive an actual CONNECT and assert the finalized row; my independent
  probes reproduced the same rows through a live service.
- **SEC-17 survives the R12 change.** Aborted rows are persisted from the live
  ring, and the live ring is only ever populated through `retainLiveMetadata`, so
  the default policy still drops unattributed records before persistence and
  before renderer exposure. The `unattributed`/`dropped` counters remain the
  honest denominator alongside `observed`.
- **SEC-10 and SEC-16 hold.** Bodies are unconditionally omitted on every path;
  `acceptance.Build` opens read-only, refuses a non-terminal session, bounds rows
  by `store.MaxFetchLimit`, and the CLI writes `0600`.
- **Shutdown ordering remains correct.** `Server.Stop` waits on both the
  connection and handler wait groups before `Manager.Stop` cancels and closes the
  pipeline, and `Pipeline.Close` holds `sendMu` across `abortInflight` and
  `close(p.progress)` after the writer has exited, so the new abort-persistence
  path cannot send on a closed channel.
- **The harness is genuinely fail-closed where it does run.** Missing clients,
  failed clients, unmatched markers, contradicted supported-tier contracts,
  missing h2/pinning rows, a short long-session, an unproven redaction summary, a
  non-`recoverable` recovery session, and any `semantic` grade on an
  unsupported-tier row all append a contradiction, and the script throws after
  writing the artifact and its SHA-256 sidecar.
- **i18n parity holds.** 997 distinct message keys, every one present in both
  tables, 84 `liveCapture*` keys including the new state, CA, attribution, and
  contract-mismatch families.
- **Build, vet, the full Go suite, `-race -count=2` across the capture packages,
  Windows cross-compilation vet and build, the frontend state suite, and the
  frontend production build all pass** as claimed in `work_status.md`.

## Conditions for an H-RG4 `PASS`

1. **S1** resolved: no finalized live-capture session may be described as
   imported foreign-tool evidence in any user-visible string, not only in the
   metadata. Covered by a renderer test that asserts the live hint for a live
   result and the HAR hint for a HAR result.
2. **R2** resolved: an inspectable Windows evidence artifact with an empty
   `contradictions` array, produced by the current harness, committed or
   referenced from the repository by path and checksum — subject to S5.
3. **S3** resolved: a session that did not reach `Stop` must not claim that no
   redaction was applied, and must not report zero counters next to stored rows.
4. **S2, S4, S5** resolved or explicitly accepted with a recorded rationale.
   Each concerns what the finalized artifact or screen asserts about a session,
   which is the subject of the `PASS` criterion.
5. **R6**'s missing measurement and **S6–S9** may be deferred with a recorded
   decision.

Until conditions 1–3 are met, T-581 should remain in `REVIEW`;
`work_status.md` should not record an H-RG4 `PASS` and H-RG5 should stay
`PENDING`.
