# H-RG4 — Windows Live-Capture UI and E2E Re-Review (T-581)

- **Review group:** H-RG4 (live UI and Windows E2E)
- **Target task:** T-581 Windows live-capture UI, progress/finalization events,
  acceptance fixture, Windows E2E, and client harness
- **Reviewer:** claude-code (independent re-review)
- **Date:** 2026-07-29
- **Verdict:** `CONDITIONAL` — the previous blocking findings L1 and L2 are
  resolved and L3 is substantially remediated in mechanism, but two new blocking
  findings (R1, R2) and three high-severity findings (R3, R4, R5) must be
  resolved before an H-RG4 `PASS`
- **Predecessor:** `docs/review/done/2026-07-28_claude-code_H-RG4_windows-live-capture-ui-e2e-review.md`
  (`CONDITIONAL`, findings L1–L14)
- **Evidence base:** full read of the remediation commits `b66dc76`
  (`fix(t581): remediate H-RG4 backend findings`), `1e909fd`
  (`fix(t581): remediate H-RG4 UI, state, and i18n findings`), and `46f2c16`
  (`fix(t581): complete Windows live capture acceptance`) against the working
  tree at `46f2c16`, plus the surrounding sources:
  `internal/capture/proxy/server.go`, `internal/capture/stream/pipeline.go`,
  `internal/capture/session/manager.go`, `internal/capture/redact/redact.go`,
  `internal/capture/procmap/*`, `internal/capture/acceptance/evidence.go`,
  `internal/capture/types.go`,
  `internal/analyzers/httpcapture/analyzer.go`,
  `cmd/archscope-app/captureservice.go`,
  `cmd/archscope-app/captureservice_test.go`,
  `cmd/archscope-app/capture_windows_e2e_test.go`,
  `cmd/archscope-app/testdata/t581_live_capture_acceptance.json`,
  `cmd/archscope-engine/cmd_http_capture.go`,
  `frontend/src/components/LiveHttpCapturePanel.tsx`,
  `frontend/src/pages/HttpCapturePage.tsx`,
  `frontend/src/state/liveHttpCapture.ts`,
  `frontend/src/state/regression.test.ts`,
  `frontend/src/i18n/messages.ts`,
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
preflight, `SEC-17` explicit unknown-attribution opt-in). Every L-finding from
the predecessor review is re-verified against the current code. New findings are
raised only where they sit inside the H-RG4 acceptance path.

No disposition is recorded here; that belongs to the implementing agent. No
repository file outside this document was modified during this review.

## Verification performed

Run from `apps/engine-native` on darwin/arm64 (Go 1.26.5, Node/npm from the
frontend workspace):

| Command | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./internal/capture/... ./cmd/archscope-app/...` | clean |
| `go test ./...` | pass |
| `go test -race -count=2 ./internal/capture/{stream,redact,proxy,session}/...` | pass |
| `GOOS=windows go vet ./internal/capture/... ./cmd/archscope-app/... ./cmd/archscope-engine/...` | clean |
| `GOOS=windows go build ./cmd/archscope-engine` | pass |
| `npm run test:state` (frontend) | pass |
| `npm run build` (frontend) | pass |
| `go test -overlay` probe — finalized-handoff metadata | **reproduced R1** |
| `go test -overlay` probe — pinning / failed-tunnel transactions | **reproduced R3, R4** |

All probes were supplied through `go test -overlay` from the session scratch
directory; no repository file was added or modified for them.

## Verdict summary

The remediation is substantial and, on the live table itself, correct. The
previously confirmed data race is gone and locked down by a real concurrency
regression; progress fidelity is honest end to end for both the engine and the
renderer; in-flight rows are now terminal, positionally stable, batched,
counted, and separated from measured values; the SEC-17 policy in force is read
back from the session instead of from a local checkbox; `observed` gives the
drop counters an honest denominator; and the Windows harness has been rebuilt
from an operator checklist into a fail-closed product-readback script.

Two things block a `PASS`.

1. **The same-page finalized handoff still declares `semantic`.** The live table
   was fixed; the analysis result the same page loads after stop was not.
   `AnalyzeCaptureSession` routes live transactions through
   `httpcapture.Build`, whose metadata block hardcodes `fidelity: "semantic"`,
   `capture_mode: "har_import"`, and `observation_point: "foreign_tool"`. A
   Windows session consisting entirely of h2/QUIC/pinned passthrough is
   presented, one click after stopping, as semantic capture — the precise
   presentation the H-RG4 `PASS` criterion forbids (R1). The same result object
   is pushed into the Workspace, so the claim propagates downstream.
2. **No Windows acceptance evidence exists in the repository.** The harness can
   now produce it and would fail closed, which closes the mechanical half of L3.
   But the acceptance run itself is recorded only as prose in `work_status.md`;
   no evidence artifact is committed or referenced, and the harness does not
   cover page re-entry, long sessions, recovery, or the unsupported tier, which
   are four of the seven scenarios the `PASS` criterion names (R2).

Three high-severity findings concern the honesty of what the engine records for
the failure paths this gate is specifically about: certificate-pinning failures
lose their attribution and are silently discarded (R3), failed passthrough
tunnels finalize as `proxy_mitm` (R4), and the finalized view tells the user no
sensitive fields were redacted for a session in which every record was redacted
(R5).

## Disposition of the previous findings

| Prev. | Severity | Status | Evidence |
|---|---|---|---|
| L1 | B | **Resolved (live path)**; see R1 for the finalized path | `proxy/server.go:402-405` emits `pending` for MITM and `unsupported` for passthrough; `liveHttpCapture.ts:305-325` maps a closed token set with `unknown` → "not yet determined"; `server_test.go:199-221` asserts the passthrough progress row and the stop-mid-tunnel finalized row; `regression.test.ts:982-1002` |
| L2 | B | **Resolved** | `redact.Policy` now carries `sync.RWMutex`; `bumpN` (`redact.go:416-422`), `addWarning` (`:354-358`), and the `custom` rule read/disable path (`:319-339`) are locked; `TestPipelineRedactionIsSafeAcrossConcurrentProgressAndPersistence` (32×20 concurrent transactions) passes under `-race -count=2`. The predecessor's probe no longer reproduces |
| L3 | B | **Partially resolved** — mechanism yes, evidence no (R2); fixture row cap still unbound (R8) | Harness reads back product rows via `http-capture acceptance-evidence` and fails on missing clients, missing rows, or contradicted contracts (`verify-windows-live-capture.ps1:251-286,318-320`); fixture now binds to `proxy.CaptureModeMITM`, `proxy.FidelityDecodedWire`, `proxy.FidelityUnsupported`, `acceptance.SchemaVersion`, `acceptance.MaxEvidenceRows`, `store.MaxFetchLimit` (`captureservice_test.go:112-166`) |
| L4 | H | **Resolved** | Progress is batched by the publisher's `BatchInterval` alongside transactions (`pipeline.go:261-304`), emitted as `CaptureProgressEvent{Items}` (`captureservice.go:75-79`), and consumed in one dispatch (`liveHttpCapture.ts:175-183`); `TestProgressIsBatchedStableAndAbortedOnClose` asserts one call for two rows |
| L5 | H | **Resolved** | `failLive` terminates rows rejected by the pipeline (`pipeline.go:400-410`); `abortInflight` marks both `request_sent` and `receiving` rows `aborted` at close (`:412-434`); `TrackProgress` upserts into the live ring so hydrate/resync no longer deletes in-flight rows (`:340-357`); renderer shows a state column, in-flight badge and `—` placeholders (`LiveHttpCapturePanel.tsx:722-758`) |
| L6 | H | **Resolved** | `Session.RetainUnattributedMetadata` (`types.go:91`) is set at `Start` (`manager.go:155-158`) and displayed through `activeUnattributedPolicy` (`liveHttpCapture.ts:391-399`, `LiveHttpCapturePanel.tsx:363-366,488`), with an explicit locked-policy line while active |
| L7 | H | **Resolved** | `observed` counts every transaction before the privacy drop (`pipeline.go:125`, `types.go:66`); the panel adds an `Observed` tile, a drop-share line, and localized drop / unattributed / kernel-loss warnings (`LiveHttpCapturePanel.tsx:580-634`); `buildLiveCoverageDisclosure` documents why `unattributed > captured` is normal |
| L8 | M | **Resolved** | `boundedDistinct` replaces a row at its first-seen position (`liveHttpCapture.ts:223-240`); `regression.test.ts:965-980` |
| L9 | M | **Resolved** | `Manager.Start` returns `ErrModeUnavailable` off Windows (`manager.go:129-131`); `TestManagerAdvertisesOnlyWindowsAsSupportedLivePlatform` |
| L10 | M | **Resolved (documented)** | `docs/en/USER_GUIDE.md:155-156` and `docs/ko/USER_GUIDE.md:152-154` state the CurrentUser-root boundary and the JVM/NSS exception; the harness requires `-JavaTrustStore` for JVM HTTPS and records the gap otherwise (`verify-windows-live-capture.ps1:172-180`) |
| L11 | L | **Resolved** | `progressTransaction` leaves `Path` empty when the URL has no path (`server.go:397-399`); asserted at `server_test.go:204` |
| L12 | L | **Resolved** | A repeat `started` for the same session is idempotent and preserves `follow` (`liveHttpCapture.ts:143-156`); `regression.test.ts:868-899` |
| L13 | L | **Partially resolved** — see R7 | `processInstance` now labels `inferred` by default (`resolver_windows.go:98`) and `Resolve` upgrades to `confirmed` only after a second TCP-table read returns the same PID (`resolver.go:43-58`), but the start time is a precondition and is never compared |
| L14 | L | **Resolved** | Distinct labels and warnings for "Discarded by policy" and "Lost before capture" (`messages.ts` `liveCaptureDropped`/`liveCaptureKernelDropped` families, `LiveHttpCapturePanel.tsx:584-586,622-628`) |

## Acceptance-criteria assessment

| H-RG4 item | Assessment | Evidence |
|---|---|---|
| Start/stop, session state, CA install/remove, first-use warning | Implemented | `LiveHttpCapturePanel.tsx:399-557`; CA controls gated on `firstUseAccepted` and an inactive session; `StopCapture` removes trust and releases the authority (`captureservice.go:140-147`) |
| Process tree, stable live list, terminal in-progress reconciliation | Implemented | `buildLiveProcessGroups`, `boundedDistinct` first-seen replacement, `failLive` + `abortInflight`, in-flight badge and count |
| Scroll-respecting follow mode, batched updates, row cap | Implemented | Follow mode `:243-247,370-377`; batched progress; 500-row cap |
| Persistence/drop/backpressure/disk/recovery status with explicit drop warning | Implemented | Nine stat tiles including `Observed`, drop-share line, six distinct warning strings |
| Honest fidelity, coverage, passthrough, unattributed warnings | **Not met** | Live table is honest; the finalized handoff on the same page claims `semantic` (R1); pinning failures never reach the table at all (R3); failed tunnels record `proxy_mitm` (R4) |
| Same-page finalized-session lazy loading after stop | Implemented, dishonest metadata | Handoff works (`HttpCapturePage.tsx:128-143`); its metadata block is wrong (R1) and its redaction disclosure is inverted (R5) |
| `SEC-17` explicit unknown-attribution opt-in | Enforced and now correctly disclosed | `pipeline.go:126-132`, `manager.go:190-195,450-453`, `activeUnattributedPolicy` |
| `SEC-10` bodies remain omitted | Met | `BodyStorage: "omitted"` on every emitted and progress transaction; panel discloses the pending preflight |
| Windows E2E for browser/curl/JVM/Electron, re-entry, long sessions, recovery | **Not met** | R2 |

## Findings

Severity: **B**locking, **H**igh, **M**edium, **L**ow. No disposition is
recorded here; that belongs to the implementing agent.

### R1 (B) — The finalized live session is presented as `semantic` HAR import

`CaptureService.AnalyzeCaptureSession` builds the finalized `AnalysisResult`
with `httpanalyzer.Build(transactions, "capture://<id>", "archscope-live",
"canonical-v1", …)` (`cmd/archscope-app/captureservice.go:199`). `Build`
delegates to `BuildParsed`, whose capture-metadata block is a literal
(`internal/analyzers/httpcapture/analyzer.go:161-177`):

```go
result.Metadata.Extra["http_capture"] = map[string]any{
    …
    "capture_mode":      "har_import",
    "observation_point": "foreign_tool",
    "fidelity":          "semantic",
    …
}
```

Reproduced with an overlay probe that feeds `Build` a single
`proxy_passthrough` / `unsupported` transaction — that is, a session in which
ArchScope decoded nothing at all:

```
http_capture metadata for a 100% passthrough live session:
  "capture_mode": "har_import",
  "fidelity": "semantic",
  "observation_point": "foreign_tool",
```

`HttpCapturePage` renders exactly these three fields in the "Capture fidelity"
card (`pages/HttpCapturePage.tsx:429-431`, via `extractCaptureMeta`), and
`onLiveFinalized` also pushes the result into the Workspace under
`capture://<sessionId>` (`:128-143`), so the claim survives into every
downstream view and report of that session.

Consequences:

- The H-RG4 `PASS` criterion — "H2/QUIC/pinning limitations never look like
  successful semantic capture" — is violated one click after the live table
  correctly refuses to make that claim. The user's last impression of the
  session is the wrong one.
- `"semantic"` is a canonical **positive** grade per
  `docs/en/SYSTEM_HTTP_CAPTURE.md:87` ("canonical header names, values, duplicate
  multi-values, decoded body, semantic timings"). ArchScope holds none of that
  for an opaque tunnel, and for its own MITM rows the correct grade is
  `decoded_wire` — which is what the per-row `fidelity` column of the same
  screen shows. The card and the table it sits above contradict each other.
- `capture_mode: "har_import"` and `observation_point: "foreign_tool"` also
  destroy provenance: an ArchScope-owned proxy capture is labelled as an import
  from a third-party tool. For a product whose value is trustworthy evidence,
  this is the more damaging half of the finding.

The redaction half of the same metadata block is R5.

Suggested direction: give `Build`/`BuildParsed` the capture provenance instead
of hardcoding it — derive `capture_mode`, `observation_point`, and `fidelity`
from the entries themselves (for a mixed session, the honest aggregate is the
weakest grade present, plus per-mode counts), and let the HAR path pass its
`har_import`/`foreign_tool`/`semantic` values in explicitly. The acceptance
evidence builder already computes exactly these distributions
(`acceptance/evidence.go:99-108`, `mode:`/`fidelity:`/`coverage:` counters), so
the shape is available.

### R2 (B) — The Windows acceptance evidence is asserted but not archived, and four of the seven `PASS` scenarios are outside the harness

L3 asked for "real Windows evidence … produced by a harness that reads back what
ArchScope actually captured and fails on absent clients or contradicted
expectations". The harness half is now genuinely done:
`verify-windows-live-capture.ps1` waits for a terminal manifest state, invokes
`archscope-engine http-capture acceptance-evidence`, matches every client probe
to a captured row by an injected query marker, and throws when a client is
missing, a client fails, no row matches, or a row contradicts the supported-tier
contract (`:251-286`, `:318-320`). `acceptance.Build` is read-only, requires a
stopped session, and is bounded by `store.MaxFetchLimit`, so SEC-16 is
preserved.

What is missing is the evidence itself and the rest of the criterion:

- **No artifact.** The harness writes
  `t581-windows-live-capture-evidence.json` to a caller-chosen path. Nothing in
  the repository contains, references, or links a produced evidence file. The
  only record of the acceptance run is prose in `work_status.md` ("eight-client
  HTTP/HTTPS product-readback matrix, a 6,653-row long session, H2-only
  unsupported/passthrough, off-page event resynchronization, crash recovery, and
  CA cleanup"). An independent reviewer cannot verify a claim whose sole proof
  is the claim. Every prior gate in this project (H-COV1 in particular) was
  closed against inspectable numbers.
- **Scenario coverage.** The harness covers exactly one of the criterion's
  scenarios — the four supported-tier clients over HTTP and HTTPS. Page
  re-entry, long sessions, and recovery are not exercised at all, and the
  unsupported tier is actively excluded: curl is pinned to `--http1.1`
  (`:139`) and the JVM client to `HttpClient.Version.HTTP_1_1` (`:159`), so no
  h2/QUIC/pinned connection is ever produced. The fixture asserts an
  `unsupportedTier` (`t581_live_capture_acceptance.json:34-50`) that nothing in
  the acceptance path can generate; only the Go unit test
  `TestH2OnlyALPNIsExplicitPassthrough` covers it, on darwin, against a
  synthetic origin.
- **`capture_windows_e2e_test.go`** is improved — it now asserts
  `observed`/`captured`/`unattributed`/`dropped`, `processAttribution ==
  "confirmed"`, `decoded_wire`, `proxy_mitm`, and body omission — but it is
  still a single plain-HTTP `GET` from a Go `http.Client`. It never exercises
  CONNECT/TLS interception, passthrough, page re-entry, long sessions,
  backpressure, or recovery. Its header comment still claims it "exercises the
  same explicit-proxy path used by browser/curl/JVM/Electron clients"; that
  remains an overstatement.

Suggested direction: commit (or attach and reference by path and checksum) the
`schemaVersion: 2` evidence JSON from a real Windows run, with the
`contradictions` array empty; extend the harness with an unsupported-tier probe
(one h2-forced client and one pinned client) asserting `fidelity == unsupported`
rather than absence; and add explicit re-entry, long-session, and recovery steps
whose outcome is read back from the product rather than observed by an operator.

### R3 (H) — Certificate-pinning failures lose attribution and are silently discarded

When TLS interception fails at the client handshake — the canonical pinning
case — `intercept` builds the diagnostic transaction but never attaches the
already-resolved process (`internal/capture/proxy/server.go:320-325`):

```go
if err := tlsConn.Handshake(); err != nil {
    tx := failureTransaction(&http.Request{Method: http.MethodConnect, Host: hostPort}, …)
    tx.Fidelity = FidelityUnsupported
    tx.Error = "TLS interception failed; diagnose certificate pinning or trust and require explicit scoped passthrough"
    s.emit(tx)
    return
}
```

`process` is a parameter of `intercept` and is in scope; both sibling failure
paths do assign it (`:202-204`, `:290-294`). Because it is omitted here,
`stream.unattributed(tx)` is true (`pipeline.go:455-457`) and the record is
dropped before persistence under the default `SEC-17` policy.

Reproduced with an overlay probe driving a real CONNECT + client handshake
against the proxy with a resolver that returns a `confirmed` process:

```
pinning-failure transaction: mode="proxy_mitm" fidelity="unsupported"
  state="failed" coverage="unknown" process=<nil>
  error="TLS interception failed; diagnose certificate pinning or trust…"
```

Consequences:

- With the default (recommended, privacy-preserving) policy, the single most
  actionable diagnostic this engine produces never reaches the live table, the
  store, the finalized analysis, or the acceptance evidence. The user sees a
  browser error page and an ArchScope session that says nothing about it.
  `pipeline.MarkUnsupported()` still fires (`manager.go:205-207`), so the
  `Unsupported` tile and the fidelity warning banner do move — an aggregate
  hint, with the row that explains it deleted.
- The drop is invisible in the coverage disclosure as a *cause*: it lands in
  `dropped`/`unattributed` next to ordinary short-lived-connection misses.
- Even with the opt-in enabled, the row claims `capture_mode: "proxy_mitm"` for
  a connection that was never intercepted, because `failureTransaction`
  hardcodes that mode (`server.go:483`). See R4.

Suggested direction: assign `tx.Process = process` on this path (and set
`Coverage` from it), and give pinning/handshake failures a mode that does not
assert interception. A regression test for "a handshake failure produces a
retained, attributed `unsupported` row under the default policy" belongs in the
proxy or stream package.

### R4 (H) — A failed passthrough tunnel finalizes as `proxy_mitm` with `pending` fidelity

`tunnel` emits a correct progress row (`proxy_passthrough` / `unsupported`) and
then, if the upstream dial fails, replaces it by ID with a transaction from
`failureTransaction`, which hardcodes `CaptureMode: CaptureModeMITM` and
`Fidelity: FidelityPending` (`server.go:288-295`, `:479-486`).

Reproduced with an overlay probe against a refused upstream:

```
progress row:  mode="proxy_passthrough" fidelity="unsupported" state="request_sent"
finalized row: mode="proxy_mitm"        fidelity="pending"     state="failed"
```

Consequences:

- The row's mode changes from the truth to a false positive claim over its
  lifetime — the mirror image of the L1 defect, on the finalized side.
- `fidelity: "pending"` on a **terminal** (`failed`) row renders permanently as
  "not yet determined" (`isLiveTransactionInFlight` treats `failed` as terminal,
  so the row is no longer marked in flight, yet its grade says the grade has not
  been decided). The same value is written to the store and counted by
  `acceptance/evidence.go` as `mode:proxy_mitm` / `fidelity:pending`, so the
  acceptance evidence understates passthrough and overstates interception.
- `failureTransaction` also fixes `Coverage: "unknown"` while the caller then
  attaches a `confirmed` process, so the row's coverage field and its process
  attribution disagree.

Suggested direction: pass the mode (and the coverage derived from the process)
into `failureTransaction` instead of hardcoding MITM, and give terminal failures
a fidelity that reads as terminal (`unsupported` for a tunnel, or a dedicated
`not_captured`) rather than `pending`.

### R5 (H) — The finalized live session states that no sensitive fields were redacted

`Build` synthesizes an empty redaction summary for callers that already hold
normalized transactions (`analyzers/httpcapture/analyzer.go:59-66`):

```go
Redaction: redact.Summary{Version: redact.PolicyVersion, Rules: []string{}, Counts: map[string]int{}},
```

so `redaction.applied` is `false` and `summary["redaction_applied"]` is `false`
for every finalized live session. The UI renders that as the badge
"No sensitive fields matched the redaction policy."
(`pages/HttpCapturePage.tsx:444-461`, `messages.ts` `httpCaptureRedactionNone`),
and the `CAPTURE_REDACTED` info finding is never added.

This is false for the live path by construction: `Pipeline.redact` runs on
**every** record before persistence and again for every live-metadata row
(`stream/pipeline.go:136,337,444-453`), rewriting URLs, both header sets, and
the process command line, and `tx.Query` is unconditionally cleared. A user
comparing a captured URL against the real request has been told, in the product,
that nothing was substituted.

The redaction counts do exist at capture time (`redact.Policy.Summary()`), but
they are not carried into the store manifest, so `AnalyzeCaptureSession` has
nothing to read back. Fixing the disclosure requires persisting the summary with
the session — the same plumbing R1 needs for provenance.

### R6 (M) — Attribution now enumerates the whole system TCP table twice per request

`procmap.Resolver.Resolve` reads the full owner-PID table, resolves the process,
then reads the full table a second time to confirm the PID
(`internal/capture/procmap/resolver.go:39-59`). Each `ownerPIDRows()` performs
two `GetExtendedTcpTable` calls (IPv4 + IPv6) plus decode
(`resolver_windows.go:28-46`), and `processInstance` allocates a 64 KB UTF-16
buffer per call (`:106`). One plain-HTTP request therefore costs four full TCP
table enumerations on the proxy handler goroutine, synchronously, before the
request is forwarded; `handlePlain` resolves per request, so a page load
multiplies this by its request count.

The H-COV1 measurement recorded for T-571 was taken against the single-read
resolver. No new measurement was recorded after the L13 remediation doubled the
cost, and the long-session claim in `work_status.md` (6,653 rows) carries no
latency or CPU figure. There is no per-connection memoization even though the
`(clientPort, proxyPort)` tuple is stable for the life of a connection.

### R7 (M) — "Confirmed" attribution still does not compare start times

The plan and `work_status.md` describe the L13 remediation as "two-read
PID/start-time confirmation". The code requires `process.Key.StartTime != ""` as
a **precondition** and then confirms only that a second table read returns the
same PID (`resolver.go:46-58`); the start time is never re-read or compared.
The guarantee that is actually delivered is "the connection→PID row survived two
consecutive enumerations", which narrows but does not close the PID-reuse
window, and does not bind the PID to a process identity.

This label is the load-bearing key for the `SEC-17` retention decision
(`stream.unattributed`, `session.retainLiveMetadata`), so the gap between the
documented guarantee and the implemented one should be closed in one direction
or the other — either compare the start time on the second read, or restate the
guarantee in the plan and the user guides.

### R8 (M) — The acceptance fixture's renderer contract is still unbound

L3 called out three self-referential fixture assertions. Two are fixed
(`maxFetchLimit` → `store.MaxFetchLimit`, fidelity/mode → `proxy.*` constants).
The third is not: `fixture.Renderer.RowCap != 500` is still a bare literal
compared against a bare literal (`captureservice_test.go:145`), and nothing on
the TypeScript side reads the fixture, so `LIVE_TRANSACTION_ROW_CAP`
(`state/liveHttpCapture.ts:1`) and the fixture can drift apart without any test
failing. The same applies to `resyncOnEventSkip`,
`restoreCurrentSessionOnPageReentry`, and `finalizedSessionUsesAnalysisResult`,
which are asserted `true` against nothing.

### R9 (L) — `isDecodedLiveFidelity` is exported and tested but never used by the product

The disposition recorded in `work_status.md` states that
"`isDecodedLiveFidelity` gates any positive capture claim". It does not: the
function is referenced only by `state/regression.test.ts`
(`:998-1002`); `LiveHttpCapturePanel.tsx` does not import it. The renderer's
actual protection is the closed label map, which is sufficient — but the claimed
gate does not exist, and the regression test protects a function no product code
calls.

### R10 (L) — The implementation plan is stale for the completed UI remediation

`docs/en|ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §H-RG4 still
carries four unchecked Live UI boxes and the sentence "React/UI/state/i18n
remediation is handed to Claude" / "this does not constitute `PASS` without the
Claude UI handoff", which `1e909fd` completed. `work_status.md` and the plan now
disagree about the state of the same work. The plan is the review-gate contract,
so it should not lag behind the gate it governs.

### R11 (L) — The live table's state column is not localized

Every other cell and label in the panel goes through `t(...)`, but the new state
column prints the raw engine token (`LiveHttpCapturePanel.tsx:748-751`) —
`request_sent`, `receiving`, `complete`, `failed`, `aborted` — with only the
suffix "· in flight" localized. `caStatus.state` and `session.state` have the
same issue elsewhere in the panel. Given the project's English/Korean parity
guardrail and the fact that this column was added specifically to explain
unresolved rows to the user, it should have a label map like the fidelity
column.

### R12 (L) — Aborted rows are shown but never persisted

`abortInflight` publishes `aborted` rows to the renderer at close
(`pipeline.go:412-434`) but does not write them to the store, and the store is
what `AnalyzeCaptureSession` and `acceptance.Build` read. After stopping a
session, the live table shows N aborted rows while the finalized analysis
reports `aborted_transactions: 0` and the acceptance evidence has no
`state:aborted` counter. The reconciliation is correct for the live view and
invisible to every artifact derived from it.

## What was verified as sound

- **The L2 race is genuinely gone.** `redact.Policy` is mutex-protected on every
  mutating path, the concurrency regression in the stream package exercises 640
  concurrent redactions through both `Submit` and `LiveMetadata`, and
  `-race -count=2` across `stream`, `redact`, `proxy`, and `session` is clean.
  The predecessor's overlay probe no longer reproduces.
- **`SEC-17` remains enforced below the renderer and is now correctly
  disclosed.** The persistence predicate (`stream.unattributed`) and the progress
  predicate (`session.retainLiveMetadata`) are still logically identical, the
  drop still happens before redaction and marshaling, and the displayed policy
  now comes from `Session.RetainUnattributedMetadata` rather than a local
  checkbox.
- **`SEC-10` holds.** Bodies are unconditionally omitted on the finalized and
  progress paths, `metadataOnly` strips previews and refs before any renderer
  exposure, and the panel still discloses the pending preflight.
- **Shutdown ordering is correct.** `Manager.Stop` now drains the capture source
  before cancelling the submission context (`manager.go:271-277`), and
  `Pipeline.Close` holds `sendMu` across `abortInflight` and `close(p.progress)`
  while `writer` has already exited via `<-p.done`, so no `Submit`, `failLive`,
  or `TrackProgress` can send on the closed progress channel. I looked for a
  send-on-closed-channel panic on this path and could not construct one.
- **The read-only evidence command respects SEC-16.** `acceptance.Build` opens
  the session read-only, refuses non-terminal sessions, bounds rows by
  `store.MaxFetchLimit`, and the CLI writes `0600`
  (`cmd_http_capture.go`, `helpers.go` `writePrivateJSONAny`).
- **Progress/finalized ID correspondence** is still sound by construction, and
  the renderer's in-place replacement preserves position.
- **i18n parity holds.** All 57 `liveCapture*` keys exist in both the English
  and Korean tables (114 occurrences, none unpaired), and
  `regression.test.ts:1150+` asserts the parity for the new disclosure keys.
- **Build, vet, full Go test suite, Windows cross-compilation vet/build, the
  frontend state suite, and the frontend production build all pass** as claimed
  in `work_status.md`.

## Conditions for an H-RG4 `PASS`

1. **R1** resolved: no finalized live-capture session may present
   `fidelity: "semantic"`, `capture_mode: "har_import"`, or
   `observation_point: "foreign_tool"`. The finalized metadata must be derived
   from the session's actual transactions, and a session containing passthrough,
   h2, QUIC, or pinned traffic must say so. Covered by a test that feeds
   `Build`/`AnalyzeCaptureSession` a non-semantic session and asserts the
   metadata block.
2. **R2** resolved: an inspectable Windows evidence artifact with an empty
   `contradictions` array, produced by the harness, committed or referenced from
   the repository; plus harness coverage for the unsupported tier and for page
   re-entry, long sessions, and recovery — the four `PASS` scenarios currently
   outside it.
3. **R3, R4, R5** resolved: pinning/handshake failures retained and attributed,
   failed tunnels recorded with their real mode and a terminal fidelity, and the
   finalized redaction disclosure telling the truth about a redacted session.
4. **R6–R8** resolved or explicitly accepted with a recorded rationale, since
   each concerns either the cost of the new attribution path or the strength of
   the acceptance contract.
5. R9–R12 may be deferred with a recorded decision.

Until conditions 1–3 are met, T-581 should remain in `REVIEW`;
`work_status.md` should not record an H-RG4 `PASS` and H-RG5 should stay
`PENDING`.
