# H-RG4 — Windows Live-Capture UI and E2E Third Re-Review (T-581)

- **Review group:** H-RG4 (live UI and Windows E2E)
- **Target task:** T-581 Windows live-capture UI, progress/finalization events,
  acceptance fixture, Windows E2E, client harness, and the archived Windows
  schema-v4 evidence artifact
- **Reviewer:** claude-code (independent third re-review)
- **Date:** 2026-07-29
- **Verdict:** `CONDITIONAL` — every code-side condition from the second
  re-review (S1, S3, and the S2/S4/S5–S9 dispositions) is independently
  verified resolved, and the R2 artifact exists, is checksum-pinned, and covers
  every required scenario with an empty contradictions array. What blocks the
  `PASS` is the artifact itself: it violates its own machine-readable privacy
  declaration. 57 of its 1,067 archived rows are the operator's Edge/Electron
  background traffic — including decoded MITM rows with full query strings and
  a stable browser sync `client_id` — inside a file that asserts
  `fixtureTrafficOnly: true`, because the harness hardcodes that assertion
  instead of deriving or checking it.
- **Predecessors:**
  - `docs/review/done/2026-07-28_claude-code_H-RG4_windows-live-capture-ui-e2e-review.md`
    (`CONDITIONAL`, L1–L14)
  - `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-re-review.md`
    (`CONDITIONAL`, R1–R12)
  - `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-second-re-review.md`
    (`CONDITIONAL`, S1–S9)
- **Evidence base:** full read of the S-remediation state of the working tree:
  `internal/analyzers/httpcapture/analyzer.go`,
  `internal/capture/stream/pipeline.go`, `internal/capture/store/store.go`,
  `internal/capture/session/manager.go`, `internal/capture/redact/redact.go`,
  `internal/capture/acceptance/evidence.go`,
  `cmd/archscope-app/captureservice_test.go`,
  `cmd/archscope-app/capture_windows_e2e_test.go`,
  `frontend/src/state/httpCapture.ts`, `frontend/src/state/liveHttpCapture.ts`,
  `frontend/src/pages/HttpCapturePage.tsx`, `frontend/src/i18n/messages.ts`,
  `frontend/src/state/regression.test.ts`,
  `scripts/verify-windows-live-capture.ps1`,
  `scripts/t581-live-capture-harness-contract.json`, the H-RG4 sections of
  `docs/en|ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` and
  `docs/en|ko/USER_GUIDE.md`, and a full structural and privacy inspection of
  `docs/review/evidence/2026-07-29_t581_windows-live-capture-schema-v4.json`
  (all 1,067 capture rows, the recovery block, and every declared field).

## Scope

This gate re-validates the outstanding conditions the second re-review set for
an H-RG4 `PASS`:

1. S1 resolved — no finalized live session described as imported foreign-tool
   evidence in any user-visible string;
2. R2 resolved — an inspectable Windows evidence artifact with an empty
   `contradictions` array, committed by path and checksum, **subject to S5**;
3. S3 resolved — a session that did not reach `Stop` must not claim a clean
   redaction sheet or zero counters next to stored rows;
4. S2, S4, S5 resolved or explicitly accepted with recorded rationale;
5. R6's missing measurement and S6–S9 deferrable with a recorded decision.

Every S-finding disposition is re-verified against the current code rather than
accepted from the disposition records. The archived Windows artifact is treated
as evidence under review, not as trusted input. No product source file was
modified during this review; this document and the `work_status.md` review
record are the only writes.

## Verification performed

Run from `apps/engine-native` on linux/amd64 (Go 1.26.3, Node 22.22.2, npm from
the frontend workspace; GTK4/WebKitGTK dev packages installed so the Wails app
package compiles on Linux). The Windows harness was **not** re-executed; the
archived artifact was inspected offline instead.

| Command | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./internal/capture/... ./cmd/archscope-app/... ./internal/analyzers/httpcapture/...` | clean |
| `go test ./...` | pass (all packages, including `cmd/archscope-app`) |
| `go test -race -count=2 ./internal/capture/...` | pass |
| `GOOS=windows go vet ./internal/capture/... ./cmd/archscope-app/... ./cmd/archscope-engine/...` | clean |
| `GOOS=windows go build ./cmd/archscope-engine` | pass |
| `npm run test:state` (frontend) | pass — includes the new S1/S3/S4 hint, token-map, distribution, and redaction-state regressions and both-locale key coverage |
| `npm run build` (frontend, tsc + vite) | pass |
| `sha256sum` of the archived artifact | matches the committed sidecar and the recorded checksum `e1bdfafe…f8a4` |
| Artifact structural audit (Python, all 1,067 rows + recovery block) | schema 4, contradictions `[]`, all eight client probes and three unsupported probes present — **and reproduced V1** (57 non-fixture rows vs. `fixtureTrafficOnly: true`) |
| `go test -overlay` probe — `BuildLive` on 200 decoded + 1 aborted `unsupported` row | aggregate `fidelity=unsupported`, `fidelity_counts={decoded_wire:200, unsupported:1}`, `observation_point=proxy` — S2's terminal grade decides conservatively and S4's distribution makes it interpretable |
| `go test -overlay` probe — `BuildLive` on a legacy store still carrying a `pending` row | aggregate `pending` with `redaction.known=false` passed through — only reachable from pre-fix stores; the current pipeline can no longer persist `pending` (see S2) |

## Verdict summary

The code is done. Every one of the twelve second-re-review findings has a
verified, tested resolution or a recorded disposition, the harness now
enforces nearly everything it previously asserted, and the Windows run it
produced covers the full required matrix with product readback and an empty
contradictions array. Three earlier review cycles' worth of honesty
requirements — provenance-derived prose, closed label maps, terminal grades,
crash-safe checkpoints, contract-bound fixtures — now hold end to end, and I
could not break them with independent probes.

One thing blocks the gate, and it is the same class of defect this review
series has been about, relocated one level up: **the evidence artifact makes a
machine-readable claim about itself that its own contents disprove.** The
harness validates that the *target* URLs are loopback fixtures, but the proxy
necessarily captures everything the launched clients emit — and headless Edge
and Electron emitted their usual background traffic. 57 archived rows carry
`edge.microsoft.com`, `config.edge.skype.com`, `substrate.office.com`,
`www.bing.com`, `clients2.google.com`, and peers; 26 of them are decoded MITM
rows with full query strings, one of which is an Edge sync request carrying a
stable `client_id`, and two hit an MSA identity endpoint. The artifact's
privacy block nevertheless states `trafficScope: "loopback_fixture_only"` and
`fixtureTrafficOnly: true`, because `verify-windows-live-capture.ps1` writes
those values as literals (`:687-688`) and no contradiction check examines the
archived rows' hosts. S5 warned that "the condition that closes R2 should not
be the step that publishes an operator's browsing metadata"; this artifact does
exactly that, in a public repository, under a declaration that it doesn't.

The remediation is narrow: constrain or filter the archived rows to the
declared scope, derive the privacy booleans from the data, re-run, re-checksum.
Nothing in the product needs to change.

## Disposition of the previous findings

| Prev. | Severity | Status | Evidence |
|---|---|---|---|
| S1 | B | **Resolved** | `CAPTURE_PROVENANCE_HINT_KEYS` selects the card's prose from `resolveCaptureProvenance(meta.observation_point)` (`state/httpCapture.ts:237-246`, rendered at `HttpCapturePage.tsx:483`): `proxy` → live-proxy sentence, `foreign_tool` → HAR sentence, anything else → claims neither origin. The unconditional `httpCaptureFidelityHint` key is deleted from both locale tables, and regressions assert the live/HAR/unknown selection, the key's absence, and that the live sentence contains no HAR/foreign-tool text (`regression.test.ts:757-775`) |
| R2 | B | **Resolved in mechanism and execution; artifact blocked by V1** | The artifact exists at `docs/review/evidence/2026-07-29_t581_windows-live-capture-schema-v4.json`, its SHA-256 matches the sidecar and the recorded value, `contradictions` is `[]`, and every required scenario has product readback: 8/8 client probes (curl/browser/JVM/Electron × HTTP/HTTPS), h2-only passthrough, attributed pinning failure, QUIC invisibility (no marker row), 1,000/1,000 long-session rows with the raw `t581-secret` absent and `query_value` redaction ≥ 2,000, 500 restored re-entry rows ≤ 1,067 product rows, and a separately recovered crash store. The artifact's *content* violates its declared privacy scope — V1 |
| S3 | H | **Resolved** | Backend: `AppendWithCheckpoint` persists capture stats and the redaction summary in the same flush lifecycle as rows (`store.go:319-360`, wired at `pipeline.go:225`); `Recover` reconciles counters with stored rows (`persisted=records`, `observed/captured ≥ persisted`) and marks redaction `Known=false` whenever rows exist beyond the last checkpoint (`store.go:728-738`) — absent is now unknown, never clean. `acceptance.Build` substitutes `Known:false` for legacy manifests (`evidence.go:135`). Renderer: `known=false` renders a caution-styled "not recorded" badge and sentence stating redaction still ran before every write; rule chips are suppressed; legacy payloads without the flag stay known (`httpCapture.ts:296-310`, `HttpCapturePage.tsx:522-557`). The artifact's real crash-recovery block shows `stats 1/1/1` and a checkpointed `known:true` summary beside the stored row — the exact case S3 was raised on now reads back honestly |
| S2 | M | **Resolved** | `abortInflight` assigns `Fidelity: "unsupported"` at abort time (`pipeline.go:426`), and `TestCloseAbortsEveryInflightTransactionState` now asserts the persisted grade, not only the state. My probe confirms one aborted row grades a 201-row session `unsupported` — conservative — while `fidelity_counts` (200/1) keeps it interpretable. `pending` remains reachable only from pre-fix legacy stores |
| S4 | M | **Resolved** | Fidelity, capture mode, observation point, detail storage, and coverage resolve through closed token sets and paired EN/KO label maps with the raw engine token preserved in the hover `title` (`httpCapture.ts:108-283`, `HttpCapturePage.tsx:454-483`); `capture_mode_counts`/`fidelity_counts`/`coverage_counts` render as distribution rows beneath the aggregate with unknown-bucket merging and deterministic ordering; the per-transaction detail panel uses the same fidelity map (`:1261`). The maps are exhaustive `Record`s over their unions, so a new engine enum fails `tsc` |
| S5 | M | **Resolved in code; defeated in the shipped artifact — V1** | Owner-only ACL (`Protect-Artifact`), `sessionRef` reduced to the directory leaf, no local paths (0 drive-letter references in the artifact), rows bounded by the contracted 2,000 cap, loopback-only *target* validation (`:71-73`), and a privacy metadata block all exist. But `fixtureTrafficOnly`/`trafficScope`/`localPathsOmitted`/`reviewBeforeArchive` are hardcoded literals (`:686-692`), and no check compares the archived rows against the declared scope — which is how 57 non-fixture rows shipped under `fixtureTrafficOnly: true` |
| S6 | L | **Resolved** | `scripts/t581-live-capture-harness-contract.json` is the single contract: the PowerShell script loads it and fails closed on missing fields (`:42-58`), takes `schemaVersion`, `productEvidenceSchemaVersion`, `minLongSessionRequests`, and `maxArtifactRows` from it, and the Go suite reads the same JSON, `DeepEqual`s it against `acceptance.DefaultHarnessContract()`, and asserts the script actually references the contract file and its fields (`captureservice_test.go:236-259`). The five booleans are now load-bearing preconditions rather than descriptions |
| S7 | L | **Resolved** | The harness sends a real UDP datagram at the proxy endpoint and records the honest position — "product readback must remain empty" — then raises a contradiction if any QUIC-marker row appears in the product store (`:414-424`, `:642-647`). The artifact contains the probe and no such row. Probe unavailability is itself a contradiction (fail-closed) |
| S8 | L | **Resolved** | The re-entry CDP expression finds the navigation button by `aria-current` state instead of an English label, and the DOM row count is reconciled against product readback — `session.totalRows ≥ restoredRows` is a contradiction check (`:506-540`, `:585-587`). The artifact shows 500 restored rows (exactly the contract row cap) against 1,067 product rows |
| S9 | L | **Resolved** | The generic `httpcapture.Build` is gone; only `BuildLive` and `BuildParsed` exist, and no non-test caller can re-enter the HAR provenance path with live rows |
| R6 (measurement) | M | **Still unrecorded** — V3 | Connection-scoped resolution is implemented and tested (verified in the second re-review); the allowed deferral of the cost measurement was conditioned on a recorded decision, and no record exists in the plan, `work_status.md`, or the review-intake table |

## Acceptance-criteria assessment

| H-RG4 item | Assessment | Evidence |
|---|---|---|
| Start/stop, session state, CA install/remove, first-use warning | Implemented | Unchanged since the second re-review; app-package tests pass |
| Process tree, stable live list, terminal in-progress reconciliation | Implemented | Aborted rows now terminal in grade as well as state (S2) |
| Scroll-respecting follow mode, batched updates, row cap | Implemented | Row cap contract-driven (R8); re-entry restored exactly the 500-row cap in the real run |
| Persistence/drop/backpressure/disk/recovery status with explicit drop warning | Implemented | Crash-recovered sessions now read back reconciled stats and an honest unknown-redaction state (S3) |
| Honest fidelity, coverage, passthrough, unattributed warnings | Met | Live table (R3/R4/R9/R11) and finalized card (S1/S4) both hold; artifact rows show honest `proxy_not_captured`/`proxy_passthrough`/`unsupported` grades for real pinning and tunnel failures |
| Same-page finalized-session lazy loading after stop | **Met** | The card's metadata, prose, labels, distributions, and redaction disclosure are all provenance-honest and localized; probes could not surface a dishonest string |
| `SEC-17` explicit unknown-attribution opt-in | Enforced | Unattributed rows never enter the live ring before the opt-in; `unattributed`/`dropped` counters remain the denominator; race suite passes |
| `SEC-10` bodies remain omitted | Met | Every artifact row shows `requestBodyStorage`/`responseBodyStorage: "omitted"`; `bodyOmitted` counter present |
| Windows E2E for browser/curl/JVM/Electron, re-entry, long sessions, recovery | **Met technically; artifact blocked by V1** | All scenarios have product-readback evidence in the archived run; the artifact's privacy declaration is false for its own contents |

## Findings

Severity: **B**locking, **H**igh, **M**edium, **L**ow. No disposition is
recorded here; that belongs to the implementing agent.

### V1 (B) — The archived artifact contradicts its own privacy declaration and publishes operator background traffic

The artifact that closes R2 declares:

```json
"privacy": {
  "trafficScope": "loopback_fixture_only",
  "fixtureTrafficOnly": true,
  ...
}
```

Its `capture.rows` contain 57 transactions that are not fixture traffic: Edge
and Electron background requests to `edge.microsoft.com`,
`config.edge.skype.com`, `clients2.google.com`, `clients2.googleusercontent.com`,
`redirector.gvt1.com`, `www.bing.com`, `substrate.office.com`, `mss.office.com`,
`prod.rewardsplatform.microsoft.com`, and
`access-point.cloudmessaging.edge.microsoft.com`, attributed to the operator's
`msedge.exe` and `electron.exe`. 26 of them are decoded (`proxy_mitm` /
`decoded_wire`) rows whose full URLs and query strings are archived, including:

- `edge.microsoft.com/sync/v1/feeds/me/syncEntities/command/?client=Chromium&client_id=[REDACTED_STABLE_ID]`
  — a stable browser sync client identifier;
- two `edge.microsoft.com/identity/api/v3/msa` requests;
- component-updater and experimentation URLs disclosing exact browser version,
  channel, architecture, and `lang=ko` locale.

The cause is structural, not operational. The harness validates that the
*target* fixture URLs are loopback (`verify-windows-live-capture.ps1:71-73`)
but never examines what the proxy actually captured: the privacy block is
written as four hardcoded literals (`:686-692`), and none of the fourteen
contradiction checks compares archived row hosts against the declared scope. A
headless browser launched with `--proxy-server` routes its background traffic
through the proxy by design, so the violation will recur on every future run
until the scope is enforced rather than asserted.

Consequences:

- The second re-review's condition 2 made R2 "subject to S5", and S5's
  resolution was precisely "state explicitly that the archived rows are
  fixture traffic". The statement exists and is false — the same
  asserted-not-derived defect class as L1, R1, and S1, now in the evidence
  package itself.
- Operator machine metadata (a stable sync `client_id`, identity-endpoint
  access, browser build/locale fingerprint) is committed to a public
  repository inside a file whose own header says it contains none. Whether the
  headless profile was ephemeral cannot be determined from the artifact —
  which is itself the problem the declaration was supposed to solve.
- `contradictions: []` — the array the entire acceptance rests on — is
  reported clean for a run that violated the harness's declared privacy
  contract, so the artifact's strongest honesty signal is wrong about the one
  property that was hand-added for publication safety.

Suggested direction: enforce the declared scope instead of asserting it.
Either (a) isolate the clients from background traffic — ephemeral
`--user-data-dir`, `--disable-background-networking`,
`--disable-component-update`, `--disable-sync` (and the Electron partition
equivalents) — or (b) have the harness filter archived rows to
fixture-origin hosts plus the explicitly expected probe rows, and in both
cases derive `fixtureTrafficOnly` from the archived rows and add a
contradiction when the contract demands fixture-only and a non-loopback host
appears. Re-run, re-archive, update the checksum and the records. No product
code change is required.

### V2 (L) — The paired implementation plans still record the artifact as nonexistent

`docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §H-RG4 ("no
inspectable Windows artifact exists … Only the Windows artifact and an
independent re-review remain") and §8 ("Next, a Windows operator generates and
archives a fresh schema-v4 artifact"), and the Korean pair at the same
sections, predate the 2026-07-29 execution record in `work_status.md`. The
authoritative paired plan and the status source of truth now disagree about
whether R2's artifact exists — the same drift class as R10. (The plan text
will need updating for the V1 re-run in any case.)

### V3 (L) — R6's deferred measurement still has no recorded decision

The second re-review allowed R6's missing resolver-cost measurement to be
deferred "with a recorded decision". The connection-scoped fix is implemented
and tested, but no deferral decision is recorded in the plan, the review
history, or the intake table. Record either the measurement or the deferral
rationale so the condition is discharged rather than dropped.

## What was verified as sound

- **The S1 fix is complete, not cosmetic.** The hint key selection lives in
  state (`httpCapture.ts`), is asserted by Node regressions for live, HAR, and
  unknown results, the removed unconditional key is asserted absent from both
  locale tables, and the live sentence is asserted free of HAR/foreign-tool
  language. The unknown-provenance path claims neither origin.
- **S3 holds on the exact path that produced it.** The artifact's recovery
  block — a real crash-recovered Windows session that never reached `Stop` —
  reads back `observed:1, captured:1, persisted:1` beside its one stored row
  and a checkpointed `known:true` redaction summary with per-rule counts,
  where the second re-review's probe found `applied:false` and zeroed stats.
  The store-level reconciliation (`persisted=records`,
  `observed/captured ≥ persisted`, `Known=false` past the checkpoint) is
  covered by store, pipeline, and acceptance regressions.
- **The redaction proof in the artifact is real.** The raw long-session secret
  (`t581-secret`) appears nowhere in the file; 1,001 URLs carry
  `token=%5BREDACTED%5D`; `query_value` redaction counts (2,004) exceed the
  1,000-request long session; the recovery row is redacted on disk.
- **The harness's contradiction set is genuinely fail-closed where it looks.**
  Missing clients, failed probes, unmatched markers, contract violations,
  missing h2/pinning rows, short long-sessions, unproven redaction, QUIC
  visibility, `semantic` grades on unsupported rows, and re-entry/product
  divergence all append contradictions and throw after archiving. V1 is a gap
  in *what* it looks at, not in how it fails.
- **The S6 contract binding is bidirectional.** The Go suite reads the same
  JSON the script loads, compares it field-by-field to the Go constant set,
  and asserts the script's textual dependence on the contract fields — the
  drift channel the finding described is closed.
- **`weakestFidelity` decides conservatively under S2.** 200 decoded rows plus
  one aborted row grade the session `unsupported`, never the reverse; the
  distributions R1 added and S4 now renders keep that honest without making it
  useless.
- **SEC-17 and SEC-10 hold.** Aborted-row persistence still flows only through
  the live ring populated under `retainLiveMetadata`; every artifact row shows
  omitted bodies; the race suite passes twice over.
- **Build, vet, the full Go suite including the app package, `-race -count=2`
  across the capture packages, Windows cross-vet and engine cross-build, the
  frontend state suite, and the production frontend build all pass** in this
  environment.

## Conditions for an H-RG4 `PASS`

1. **V1** resolved: an archived Windows schema-v4 artifact whose contents
   satisfy its own declared privacy scope, produced by a harness that derives
   or verifies that declaration (fixture-scope contradiction check or
   client-isolation flags), with an updated checksum and updated
   `work_status.md`/plan records. The existing artifact must be replaced or
   filtered, not annotated.
2. **V2, V3** recorded: plan/status drift corrected in both languages, and the
   R6 measurement either taken or its deferral recorded.
3. No re-verification of S1–S9 is required unless their code paths change; a
   fresh artifact regenerated under the corrected harness plus the V1 diff is
   sufficient evidence for the next review, which can be narrow.

Until condition 1 is met, T-581 should remain in `REVIEW`; `work_status.md`
should not record an H-RG4 `PASS` and H-RG5 should stay `PENDING`.
