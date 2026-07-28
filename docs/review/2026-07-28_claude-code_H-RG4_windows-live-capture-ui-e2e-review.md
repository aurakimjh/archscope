# H-RG4 — Windows Live-Capture UI and E2E Review (T-581)

- **Review group:** H-RG4 (live UI and Windows E2E)
- **Target task:** T-581 Windows live-capture UI, progress/finalization events,
  acceptance fixture, Windows E2E, and client harness
- **Reviewer:** claude-code (independent)
- **Date:** 2026-07-28
- **Verdict:** `CONDITIONAL` — three blocking findings (L1, L2, L3) plus four
  high-severity findings must be resolved before an H-RG4 `PASS`
- **Evidence base:** full read of commit `927383f` (`feat(t581): add Windows
  live capture UI and E2E`) and the surrounding sources:
  `internal/capture/proxy/server.go`, `internal/capture/stream/pipeline.go`,
  `internal/capture/session/manager.go`, `internal/capture/procmap/*`,
  `internal/capture/certstore/*`, `internal/capture/redact/redact.go`,
  `cmd/archscope-app/captureservice.go`,
  `cmd/archscope-app/capture_windows_e2e_test.go`,
  `cmd/archscope-app/testdata/t581_live_capture_acceptance.json`,
  `frontend/src/components/LiveHttpCapturePanel.tsx`,
  `frontend/src/state/liveHttpCapture.ts`,
  `frontend/src/state/regression.test.ts`,
  `scripts/verify-windows-live-capture.ps1`, and the H-RG4 sections of
  `docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` and
  `docs/en/USER_GUIDE.md`.

## Scope

This gate validates the live-capture renderer contract and the Windows E2E
acceptance package for T-581 against the H-RG4 checklist and `PASS` criterion in
`docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §H-RG4, plus the
two conditions H-SEC2 bound to this tier (`SEC-10` body-capture preflight,
`SEC-17` explicit unknown-attribution opt-in). The H-RG3 engine foundation
itself was accepted under H-SEC2 and is only revisited where T-581 changes its
behavior or where a pre-existing defect sits directly in the path of the H-RG4
acceptance scenarios.

## Verification performed

Run from `apps/engine-native` on darwin/arm64 (Go 1.26.5):

| Command | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./internal/capture/... ./cmd/archscope-app/...` | clean |
| `go test ./internal/capture/... ./cmd/archscope-app/...` | pass |
| `go test -race ./internal/capture/...` | pass (existing tests are single-connection; see L2) |
| `npm run test:state` (frontend) | pass |
| `go test -race` with an overlay-only concurrency probe | **DATA RACE** — see L2 |

The concurrency probe was supplied through `go test -overlay` from a scratch
directory; no repository file was added or modified for it.

## Verdict summary

The renderer contract is well built. Session/CA/first-use controls, the
500-row bounded window with scroll-respecting follow mode, ID-keyed replacement
of in-progress rows, event-skip resync against an authoritative live window,
page-reentry hydration, and the same-page finalized `AnalysisResult` handoff are
all present and covered by state tests. The `SEC-17` condition is genuinely
enforced *below* the renderer: `stream.Pipeline.Submit` drops non-`confirmed`
attribution before redaction and persistence, and
`session.retainLiveMetadata` gates the progress path with the identical
predicate, so the opt-in cannot be bypassed from the UI. Bodies remain
unconditionally omitted with an explicit `SEC-10` disclosure in the panel.

Three things block a `PASS`:

1. The new in-flight progress row hardcodes `fidelity: "semantic"` for every
   capture mode, including passthrough tunnels — the precise presentation the
   H-RG4 `PASS` criterion forbids (L1).
2. The live path redacts concurrently through a `redact.Policy` that is not
   goroutine-safe; a confirmed data race on an unsynchronized map can abort the
   desktop process with `fatal error: concurrent map writes` during exactly the
   multi-connection browser scenarios this gate must run (L2).
3. No Windows acceptance evidence exists, and none of the three delivered
   artifacts (E2E test, PowerShell harness, acceptance fixture) can produce it
   in its current form (L3).

## Acceptance-criteria assessment

| H-RG4 item | Assessment | Evidence |
|---|---|---|
| Start/stop, session state, CA install/remove, first-use warning | Implemented | `LiveHttpCapturePanel.tsx:228-305,364-430`; CA install/remove gated on `firstUseAccepted` and inactive session; `CaptureService.StopCapture` removes trust and releases the authority (`captureservice.go:134-141`) |
| Process tree, stable live list, in-progress transaction state | Implemented with defects | `buildLiveProcessGroups` (`liveHttpCapture.ts:215-245`); in-progress rows via `capture:progress`; row position is not stable across finalization (L8) |
| Scroll-respecting follow mode, batched updates, row cap | Partial | Follow mode (`LiveHttpCapturePanel.tsx:222-226,335-342`) and the 500-row cap are correct; the new progress path is **not** batched (L4) |
| Persistence/drop/backpressure/disk/recovery status | Partial | Stat tiles and backpressure/event-loss banners exist (`LiveHttpCapturePanel.tsx:535-565`); deliberate `dropped` records get no warning text (L7) |
| Honest fidelity, coverage, passthrough, unattributed warnings | **Not met** | Progress rows claim `semantic` for passthrough (L1); no unattributed warning (L7); JVM/NSS trust-store boundary undisclosed (L10) |
| Same-page finalized-session lazy loading after stop | Implemented | `loadFinalized` → `AnalyzeCaptureSession` → `HttpCapturePage.onLiveFinalized` registers to Workspace under `capture://<id>` |
| `SEC-17` explicit unknown-attribution opt-in | Enforced, disclosure defect | `pipeline.go:117-124`, `manager.go:172-178,423-426`, `manager_test.go` `TestLiveMetadataRequiresConfirmedAttributionOrExplicitOptIn`; opt-in state is not restored on page re-entry (L6) |
| `SEC-10` bodies remain omitted | Met | `BodyStorage: "omitted"` on every emitted and progress transaction; panel discloses the pending preflight (`liveCaptureBodyDisabled`) |
| Windows E2E for browser/curl/JVM/Electron, re-entry, long sessions, recovery | **Not met** | L3 |

## Findings

Severity: **B**locking, **H**igh, **M**edium, **L**ow. No disposition is
recorded here; that belongs to the implementing agent.

### L1 (B) — In-flight rows present passthrough traffic as semantic capture

`progressTransaction` hardcodes `Fidelity: "semantic"` regardless of mode
(`internal/capture/proxy/server.go:388-397`), and `tunnel()` calls it with
`"proxy_passthrough"` (`server.go:266-273`). The live table renders
`transaction.fidelity` verbatim (`LiveHttpCapturePanel.tsx:665-667`).

Consequences:

- An h2-only ALPN, pinned, or explicitly allow-listed passthrough connection is
  displayed as `semantic` for the **entire lifetime of the tunnel**. The row
  only flips to `unsupported` when the tunnel closes (`server.go:288-292`), which
  for an h2 or WebSocket connection can be the whole session.
- If the session is stopped while the tunnel is open, the finalized row is never
  delivered at all: `Manager.Stop` cancels the run context and closes the
  pipeline before the tunnel's `emit` lands (`manager.go:248-257`), so `Submit`
  returns "capture stream is closed" and the `semantic` row is the final state
  the user ever sees.
- `"semantic"` is not a neutral placeholder. Per
  `docs/en/SYSTEM_HTTP_CAPTURE.md:84-88` it is a canonical **positive** fidelity
  grade meaning "canonical header names, values, duplicate multi-values, decoded
  body, semantic timings" — none of which ArchScope possesses for an opaque
  tunnel. The progress row therefore makes an affirmative capture-quality claim
  about traffic it cannot read at all.
- The same transaction also shows two different fidelity grades during its life,
  since finalized MITM rows use `decoded_wire` (`server.go:431`) — and the grade
  shown first is the one that is wrong for passthrough.

This is a direct violation of the H-RG4 `PASS` criterion "H2/QUIC/pinning
limitations never look like successful semantic capture", and of the checklist
item "Honest fidelity, coverage, passthrough, and unattributed warnings". Note
that `regression.test.ts:846` bakes `fidelity: "semantic"` into the pending-row
fixture, so the state suite ratifies the behavior rather than catching it.

Suggested direction: give in-flight rows a fidelity that cannot be mistaken for
a completed decode (e.g. `pending`/`in_flight`), derive the tunnel progress row's
fidelity from the passthrough decision that is already known at CONNECT time,
and render fidelity through a label map so an unrecognized value degrades to
"not yet determined" rather than to a reassuring string.

### L2 (B) — Confirmed data race in the live redaction path

`Pipeline.redact` (`stream/pipeline.go:326-335`) is invoked from every proxy
connection goroutine through `Submit`, and T-581 adds a second per-transaction
invocation through `LiveMetadata` on the progress path
(`pipeline.go:284-286`, `manager.go:172-178`). `redact.Policy` is not
goroutine-safe: `bumpN` writes the plain map `Policy.counts`
(`redact/redact.go:398-402`), `applyCustom` mutates `rule.disabled` and appends
to `Policy.warnings` (`redact.go:302-336`), and `addWarning` appends to the same
slice (`redact.go:342-349`). There is no mutex anywhere in the type.

Reproduced with a probe run through `go test -race -overlay` (32 goroutines ×
20 transactions carrying `Cookie`/`Authorization`/`Set-Cookie` headers and a
query token):

```
WARNING: DATA RACE
  Read at 0x00c0000b7e60 by goroutine 32:
    redact.(*Policy).bumpN()  redact.go:400
    redact.(*Policy).RedactURL()  redact.go:141
    stream.(*Pipeline).redact()  pipeline.go:327
    stream.(*Pipeline).LiveMetadata()  pipeline.go:285
  Previous write at 0x00c0000b7e60 by goroutine 55:
    redact.(*Policy).bumpN()  redact.go:400
    redact.(*Policy).RedactHeaders()  redact.go:163
    stream.(*Pipeline).redact()  pipeline.go:327
```

Attribution, stated honestly: the race is **not introduced by T-581**. Disabling
the progress call in the probe still reproduces it through `Submit` alone
(`pipeline.go:126`), so it dates to the T-580/H-RG3 engine and was not caught by
H-SEC2. What T-581 changes is that it doubles the per-transaction redaction rate
and puts the live UI in front of real Windows browser traffic for the first
time. A concurrent map write in Go is not a recoverable error — the runtime
aborts the process with `fatal error: concurrent map writes`, which would kill
the desktop app mid-capture and leave the session `recoverable` at best.

The existing suites stay green because every capture test drives a single
connection; `go test -race ./...` provides no protection here.

Suggested direction: make `redact.Policy` safe for concurrent use (a mutex
around `counts`/`warnings`/`custom`, or per-goroutine counters merged at
finalize), and add a concurrency regression to the stream package so the
guarantee is tested rather than assumed.

### L3 (B) — The Windows acceptance package cannot produce acceptance evidence

The gate's `PASS` criterion requires Windows scenarios for browser, curl, JVM,
and Electron, plus page re-entry, long sessions, and recovery. None of the three
artifacts can substantiate that:

**`capture_windows_e2e_test.go`** performs exactly one plain-HTTP `GET` from a
Go `http.Client` through the explicit proxy. It never exercises CONNECT/TLS
interception, passthrough, h2, page re-entry, long sessions, backpressure,
recovery, or the progress→finalize replacement that is the centerpiece of this
slice. Its header comment claims it "exercises the same explicit-proxy path used
by browser/curl/JVM/Electron clients"; that is an overstatement — none of those
clients share Go's TLS stack, ALPN behavior, or trust-store resolution.

**`scripts/verify-windows-live-capture.ps1`** only checks that client processes
exit `0`. It never queries ArchScope for captured transactions, so it cannot
verify `captureMode`, `fidelity`, or attribution; those are emitted as a prose
`operatorChecks` list (lines 162-169) for a human to confirm by eye. It also
forces the supported tier — `--http1.1` for curl (line 68) and
`HttpClient.Version.HTTP_1_1` for the JVM client (line 100) — so it never
exercises the h2/QUIC/pinning tier that operator check #4 asks about. Finally,
when no client is installed every row is `available:false` and the script still
exits `0` (lines 174-177): a zero-coverage run is indistinguishable from a
successful one.

**`testdata/t581_live_capture_acceptance.json` + `TestT581LiveCaptureAcceptanceFixture`**
are self-referential. The test asserts the fixture's literal JSON values against
hardcoded constants in the test body; nothing binds either side to product
behavior. `rowCap: 500` is not compared with `LIVE_TRANSACTION_ROW_CAP`,
`maxFetchLimit: 2000` is not compared with `store.MaxFetchLimit`, and
`fidelity: "decoded_wire"` is not compared with what the interceptor emits. The
test cannot fail unless someone edits the fixture.

Suggested direction: assert the fixture against the real constants and against a
transaction produced by the running proxy; extend the Windows E2E to cover
CONNECT/TLS interception, a passthrough tunnel, and the progress→finalized
replacement; make the PowerShell harness read back session stats/transactions
from the app and fail when an expected client is missing or when a captured row
contradicts the expected tier.

### L4 (H) — Progress events are emitted one IPC message per request, unbatched

`Server.progress` runs synchronously on the proxy handler goroutine
(`server.go:367-371`) → `captureEventSink.Progress` → `emitEvent` →
`app.Event.Emit` (`profilerservice.go:243-249`). Finalized rows are batched by
the publisher's `BatchInterval` (`pipeline.go:230-264`); progress rows are not.
A single browser page load produces hundreds of individual IPC messages, each
triggering a React dispatch and an O(500) `boundedDistinct` rebuild
(`liveHttpCapture.ts:191-205`). The checklist item is "batched updates, and row
cap" — the cap is implemented, the batching is not applied to the new hot path.
`Stats.backpressured` only reflects store queueing (`pipeline.go:167-208`), so
renderer flooding on this path is invisible in the UI.

### L5 (H) — In-flight rows have no terminal state and no reconciliation

A progress row is replaced only when a finalized transaction with the same ID
arrives. It never arrives when the pipeline rejects the record (hard-limit or
closed-stream error paths), when the session stops while a tunnel or keep-alive
request is open, or when the client disconnects mid-request. Those rows remain
in the table as `request_sent` with `0 ms` indefinitely — no aging, no
"unresolved" marker. Compounding this, `GetCaptureLiveWindow` returns only the
persisted ring (`pipeline.go:266-282`), so the hydrate/resync path silently
deletes in-flight rows rather than reconciling them: from the user's side, rows
appear and then vanish with no explanation.

### L6 (H) — The SEC-17 opt-in state is not restored on page re-entry

`retainUnattributed` is renderer-local `useState`
(`LiveHttpCapturePanel.tsx:87`) and `capture.Session` carries no corresponding
field (`internal/capture/types.go:81-89`), so `GetCurrentCaptureSession` cannot
report it. Re-entering the page while a session is running always shows the
checkbox unchecked — including for a session that was started **with**
unattributed retention enabled. The panel then states the opposite of the
active privacy policy.

Enforcement itself is correct (the flag is captured at `Start` and cannot be
changed mid-session), so this is a disclosure defect — but this checkbox *is*
the explicit opt-in that H-SEC2 condition C2 requires, and page re-entry is an
explicit H-RG4 acceptance scenario, so its displayed state must be authoritative
for the running session.

### L7 (H) — Silent mass drops are never surfaced as a warning

`Submit` drops every non-`confirmed` transaction before counting it as
`captured` (`pipeline.go:117-125`). On Windows this fires whenever the TCP-owner
row is gone by lookup time — `Resolve` returns "TCP owner row disappeared before
attribution" (`procmap/resolver.go:52`) — which is the expected outcome for
short-lived connections and is consistent with the H-RG2/H-COV1 finding that
direct TCP-owner attribution supports persistent endpoints only.

The warning banner triggers only on `backpressured`, `eventSkipped`,
`unsupported`, or `passthrough` (`LiveHttpCapturePanel.tsx:547-563`). Neither
`dropped` nor `unattributed` produces any explanatory text — only a stat tile
with a one-word label (`liveCaptureDropped: "Dropped"`), and no i18n string in
either language explains that ArchScope deliberately discarded traffic. A user
whose browser session is 90% dropped sees an apparently healthy running capture
with a number they cannot interpret. The checklist explicitly requires an
"unattributed" warning.

Secondary accounting effect: `captured` now excludes dropped records while
`unattributed` includes them, so `unattributed > captured` is a normal state and
the tiles read as mutually inconsistent. There is also no counter that answers
"how much traffic did the proxy see", which is the honest denominator for this
disclosure.

### L8 (M) — Row position is not stable across finalization

`boundedDistinct` keeps the **last** occurrence in array order
(`liveHttpCapture.ts:191-205`). When a finalized row replaces its progress row,
it is appended at the tail, so the row jumps from its original position to the
bottom of the list. With concurrent requests, completing rows leapfrog
still-pending ones and the visible order churns. The panel is titled "Stable
live rows"; the regression test asserts replacement
(`regression.test.ts:860-874`) but never position.

### L9 (M) — Platform gating is renderer-only

`Modes()` now reports `Available:false` off Windows (`manager.go:72-84`), but
`Manager.Start` never consults it — only the disabled Start button prevents a
start (`LiveHttpCapturePanel.tsx:470-475`), and that check reads `modes[0]`
only. A start on a non-Windows host therefore produces a fully "running"
session in which the non-Windows resolver marks every process `unknown`
(`procmap/resolver_other.go:15-20`), so with the default policy 100% of traffic
is dropped. Combined with L7, the result is an outwardly healthy session that
captures nothing and says nothing about why.

### L10 (M) — Trust-store scope is undisclosed for two of the four acceptance clients

`certstore` installs the temporary CA only into `CurrentUserRoot`, and the
Windows backend rejects any other store name
(`certstore/backend_windows.go:56-59`, `lifecycle.go:44`). Chromium/Edge,
Electron, and curl (schannel) consume that store; the JVM uses its own
`cacerts` truststore and Firefox uses NSS, so HTTPS interception for those
clients cannot work without a manual trust step. The harness avoids the issue by
testing plain HTTP, and neither the new USER_GUIDE section nor the first-use
notice states the boundary — so a JVM HTTPS failure during acceptance will
present as a product defect rather than a documented limitation.

### L11 (L) — CONNECT progress rows fabricate a path

`progressTransaction` defaults `Path` to `"/"` (`server.go:376,390`), so a
tunnel row renders as `host/` in the URL column, visually identical to a real
`GET /`. Combined with L1 this makes an opaque tunnel look like an ordinary
decoded request.

### L12 (L) — Duplicate `started` dispatch can discard early rows

`dispatch({type:"started"})` fires both from the `StartCapture` promise
(`LiveHttpCapturePanel.tsx:239-242`) and from the `capture:started` event
(`:151-154`), and the reducer resets to `initialLiveCaptureState` on each
(`liveHttpCapture.ts:127-131`). Progress events landing between the two are
discarded. The same reset also clears the user's `follow` preference on every
start.

### L13 (L) — "confirmed" attribution is asserted, not validated

Windows `processInstance` labels every PID returned by the TCP-owner table
`Attribution: "confirmed"` unconditionally
(`procmap/resolver_windows.go:93-98`), with no start-time or PID-reuse check at
attribution time. That label is now the load-bearing key for the `SEC-17`
retention decision. This predates T-581 and matches what H-COV1 accepted, but
its new role warrants an explicit statement of what "confirmed" does and does
not warrant.

### L14 (L) — `Dropped` and `KernelDropped` are indistinguishable in the UI

`Stats.Dropped` (a deliberate privacy drop) sits alongside
`Stats.KernelDropped` (lost data) with no naming or presentation distinction
(`types.go:66-74`, `LiveHttpCapturePanel.tsx:540`). "Dropped" in a capture tool
conventionally means data loss.

## What was verified as sound

- `SEC-17` is enforced below the renderer and cannot be bypassed from the UI.
  The persistence predicate (`stream.unattributed`) and the progress predicate
  (`session.retainLiveMetadata`) are logically identical, so no unattributed
  transaction reaches either disk or the live view without the opt-in, and the
  drop happens *before* redaction and marshaling.
- Bodies are unconditionally omitted on both the finalized and the new progress
  path (`BodyStorage: "omitted"`, empty `BodyPreview`), and the panel discloses
  that `SEC-10` gates any future body tier.
- Progress and finalized rows share a transaction ID by construction
  (`server.go:177,199,269,323,335,345`), and the proxy test asserts the
  correspondence — the replacement contract itself is sound.
- The store root stays application-owned: `StartCapture` clears any
  renderer-supplied `StoreRoot` (`captureservice.go:127-132`).
- The CA is removed and the authority released on stop
  (`captureservice.go:134-141`), consistent with `SEC-12`.
- Event-skip detection and authoritative resync are correctly wired
  (`liveHttpCapture.ts:140-149`, `LiveHttpCapturePanel.tsx:191-220`) and covered
  by state tests.
- `Manager.Stop`'s `captured != persisted` finalization check is unaffected by
  the new drop accounting, since dropped records never increment `captured`.
- Build, vet, Go tests, and the frontend state suite all pass as claimed in
  `work_status.md`.

## Conditions for an H-RG4 `PASS`

1. **L1** resolved: no in-flight or finalized row can display a semantic-looking
   fidelity for passthrough, h2-only, QUIC, or pinned traffic, at any point in
   its lifetime, including when the session is stopped mid-tunnel. Covered by a
   test.
2. **L2** resolved: redaction is safe under concurrent proxy connections, with a
   concurrency regression test in the stream package.
3. **L3** resolved: real Windows evidence for browser, curl, JVM, and Electron
   over both HTTP and HTTPS, plus page re-entry, a long session, event-skip
   recovery, and a failure-recovery case — produced by a harness that reads back
   what ArchScope actually captured and fails on absent clients or contradicted
   expectations, not by an operator checklist.
4. **L4–L7** resolved or explicitly accepted with a recorded rationale, since
   each concerns either renderer stability under real traffic or the honesty of
   what the live view claims.
5. L8–L14 may be deferred with a recorded decision.

Until conditions 1–3 are met, T-581 should remain in `REVIEW`; `work_status.md`
should not record an H-RG4 `PASS` and H-RG5 should stay `PENDING`.
