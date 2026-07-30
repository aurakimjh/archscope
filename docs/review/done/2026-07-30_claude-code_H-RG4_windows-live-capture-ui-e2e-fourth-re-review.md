# H-RG4 — Windows Live-Capture UI and E2E Fourth Re-Review (T-581, narrow V1–V3)

- **Review group:** H-RG4 (live UI and Windows E2E)
- **Target task:** T-581 — narrow re-review limited to the third re-review's
  conditions V1 (artifact privacy-scope enforcement and replacement), V2
  (paired-plan drift), and V3 (R6 measurement deferral record)
- **Reviewer:** claude-code (independent fourth re-review)
- **Date:** 2026-07-30
- **Verdict:** `PASS` — the replacement artifact satisfies its own declared
  privacy scope, the harness now derives that declaration from its published
  output instead of asserting it, the client isolation and fixture-row filter
  are both present and bound by the Go acceptance fixture, and V2/V3 are
  recorded in both languages. H-RG4 is closed; T-581 transitions to `DONE`
  and T-582 / H-RG5 is unblocked.
- **Predecessors:**
  - `docs/review/done/2026-07-28_claude-code_H-RG4_windows-live-capture-ui-e2e-review.md`
    (`CONDITIONAL`, L1–L14)
  - `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-re-review.md`
    (`CONDITIONAL`, R1–R12)
  - `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-second-re-review.md`
    (`CONDITIONAL`, S1–S9)
  - `docs/review/done/2026-07-29_claude-code_H-RG4_windows-live-capture-ui-e2e-third-re-review.md`
    (`CONDITIONAL`, V1–V3)
- **Evidence base:** the diff since the third re-review's baseline
  (`f9b0721..4995586`: `scripts/verify-windows-live-capture.ps1`,
  `cmd/archscope-app/captureservice_test.go`, the paired EN/KO implementation
  plans, `work_status.md`, and the replaced artifact + sidecar), a full
  independent structural and privacy audit of
  `docs/review/evidence/2026-07-29_t581_windows-live-capture-schema-v4.json`
  (all 1,012 archived rows, the recovery block, every privacy field, and
  whole-file scans for URLs, secrets, identifiers, and local paths), the
  harness contract `scripts/t581-live-capture-harness-contract.json`, and the
  V2/V3 sections of both implementation plans.

## Scope

The third re-review's `PASS` conditions were:

1. **V1** — an archived Windows schema-v4 artifact whose contents satisfy its
   own declared privacy scope, produced by a harness that derives or verifies
   that declaration, with updated checksum and records; the offending artifact
   replaced, not annotated.
2. **V2, V3** — plan/status drift corrected in both languages, and the R6
   measurement either taken or its deferral recorded.
3. No S1–S9 re-verification unless their code paths changed; a fresh artifact
   under the corrected harness plus the V1 diff is sufficient evidence.

Per condition 3 this review is narrow. I verified that the diff since the
third re-review contains **no product source change** — it touches only the
harness script, string assertions in the Go acceptance fixture test, the
artifact and its sidecar, and the paired records — so the third re-review's
verification of S1–S9, the full suites, and the race suite remains valid for
the product code as shipped.

## Verification performed

Run from `apps/engine-native` on **windows/amd64** (Windows 11 Pro
10.0.26200, Go native) — the first review in this series executed on the
artifact's own platform.

| Command / check | Result |
|---|---|
| `go build ./...` | pass |
| `go vet ./internal/capture/... ./cmd/archscope-app/... ./internal/analyzers/httpcapture/...` | clean |
| `go test ./...` | pass (all packages, including `cmd/archscope-app` and the acceptance fixture that now asserts the V1 harness mechanisms) |
| `go test -count=2 ./internal/capture/... ./cmd/archscope-app/...` | pass, uncached |
| `-race` suite | not executable in this environment (no C toolchain for Windows cgo); the third re-review's `-race -count=2` pass covers byte-identical Go source — the V1 diff changes no product code |
| `sha256sum` of the replaced artifact | `69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111` — matches the committed sidecar, `work_status.md`, and both plans |
| Artifact host audit (Python, all 1,012 `capture.rows` + recovery rows) | every row's URL host is `127.0.0.1`; zero non-loopback rows |
| Whole-file URL scan (1,013 URLs incl. recovery) | zero non-loopback URLs anywhere in the artifact |
| Whole-file identifier/secret/path scan | raw `t581-secret` absent; 1,001 `[REDACTED]` tokens; zero `client_id` parameters; zero drive-letter paths; none of the third re-review's background hosts (`edge.microsoft.com`, `config.edge.skype.com`, `bing.com`, `office.com`, `gvt1.com`, `clients2.google*`) appear |
| Privacy-block consistency | `fixtureTrafficOnly: true`, `trafficScope: loopback_fixture_only`, `sourceRows: 1023 = archivedRows: 1012 + omittedNonFixtureRows: 11`; `localPathsOmitted: true` independently confirmed; `maxArtifactRows: 2000 ≥ 1012` |
| Session-counter reconciliation | store totals (1,023) minus archived (1,012) decompose consistently across every counter family: decoded 1,016→1,010, unsupported 7→2, mitm 1,016→1,010, not_captured 4→1, passthrough 3→1, failed 4→1 — the 11 omitted rows are disclosed in the counts, not hidden |
| Scenario matrix | each of the 8 client markers (curl/browser/JVM/Electron × HTTP/HTTPS) has exactly one decoded/confirmed/omitted-bodies row; `quic-udp` marker absent from the store; h2-only `proxy_passthrough`/`unsupported` row present; pinning row present (below) |
| Pinning evidence | one fixture-origin CONNECT row `https://127.0.0.1:18443`, `state: failed`, `captureMode: proxy_not_captured`, `fidelity: unsupported`, `coverage/processAttribution: confirmed`, `processName: java.exe` — the fresh fixture-origin proof the V1 disposition required |
| Long session / re-entry / recovery | 1,000/1,000 requests; `query_value` redactions 2,000 ≥ 1,000 and `process_path` 2,042; re-entry 500 restored ≤ 1,023 product rows with product readback; recovery block `recoverable`, stats 1/1/1, checkpointed `known: true` redaction beside one loopback, redacted row |
| `contradictions` | `[]` |

## Disposition of the third re-review's findings

| Prev. | Severity | Status | Evidence |
|---|---|---|---|
| V1 | B | **Resolved** | Enforcement now exists at three layers. (1) *Client isolation:* Edge runs headless with an ephemeral `--user-data-dir` plus `--disable-background-networking`, `--disable-component-update`, `--disable-sync`, `--disable-default-apps`, `--no-first-run` (`verify-windows-live-capture.ps1:217-229`); Electron appends the equivalent switches (`:370-372`). (2) *Scope filter:* `Test-FixtureArtifactRow` (`:76-83`) restricts published `capture.rows` to loopback hosts (`:635-638`), with unparsable URLs excluded fail-closed; the contract-bound contradiction `archived evidence contains a non-fixture traffic row` (`:644-646`) and the derived `localPathsOmitted` check over the fully serialized JSON (`:793-798`) both throw after archiving. (3) *Derived declaration:* `trafficScope`, `fixtureTrafficOnly`, `sourceRows`/`archivedRows`/`omittedNonFixtureRows`, and `localPathsOmitted` are computed from the actual output (`:770-777`, `:793-795`), no longer literals. The Go acceptance fixture asserts the script retains `Test-FixtureArtifactRow`, `omittedNonFixtureRows`, `$fixtureTrafficOnly`, the `ArchScopeT581Pinning` probe, its rejection string, and its HTTP/1.1 ALPN restriction (`captureservice_test.go:256-266`), so the mechanisms cannot silently drift out. The replacement run (2026-07-30) archived 1,012 loopback-only rows, disclosed 11 omitted background rows, and my independent audit found zero non-loopback URLs, zero background hosts, zero stable identifiers, and zero local paths in the entire file. The unsafe original was replaced, and the interim filtered copy (`0b2bfc99…4975`) intentionally carried a contradiction so it could not pass. Checksum and records updated everywhere they are quoted. |
| V2 | L | **Resolved** | Both plans record the artifact lifecycle truthfully: EN §1/§8/§H-RG4 and KO §1/§8/§H-RG4 describe the first artifact as rejected by V1 and the replacement as archived with the `69565684…0111` checksum; the "no inspectable Windows artifact exists" text is gone from both languages. `work_status.md` and the plans agree. |
| V3 | L | **Resolved** | The R6 resolver-cost measurement deferral is now a recorded decision with rationale in three places: both plans (EN "R6 resolver-cost measurement is deferred from the H-RG4 correctness/privacy gate to the `R-RG1` Windows integrated performance check…", KO equivalent) and the `work_status.md` intake row "2026-07-30 H-RG4 T-581 V1–V3 disposition". The rationale — resolution bounded to once per accepted connection, long-session acceptance passed, isolated profiling belongs in the integrated performance check — is recorded, so the condition is discharged rather than dropped. |

## Non-blocking observations

Recorded for the implementing agents; none blocks the gate.

- **O1 (L):** `$fixtureTrafficOnly` is derived from the already-filtered row
  set (`:639-641`), so it is true by construction and the `:644` contradiction
  is unreachable in practice; the effective enforcement point is the filter
  itself. This matches the third re-review's suggested direction (b)
  ("filter … and derive from the archived rows"), and the independent audit
  confirms the archived contents, but a future hardening could additionally
  record the source-row derivation or unit-test the filter predicate.
- **O2 (L):** the fixture filter and scope contradiction apply to
  `capture.rows` but not to `recovery.rows`. The recovery block is clean in
  this artifact (verified: one loopback, redacted row), but a future crash
  store could carry non-fixture rows into the artifact unchecked. Extend the
  filter or add a recovery-scope contradiction on the next harness change.
- **O3 (info):** `reviewBeforeArchive: true` remains a literal — it attests an
  operator process step and is not machine-derivable; acceptable as labeled.
- **O4 (info):** the recovery store is the 2026-07-29 crash store re-read
  through the corrected harness's fail-closed recovery checks
  (`RecoverySessionPath` is an operator-supplied input by design, `:95-99`).
  Its contents are privacy-clean and S3-consistent; freshness was not a
  condition.

## Verdict and consequences

All three conditions of the third re-review are independently verified
resolved, and the defect class this review series repeatedly surfaced —
machine-readable claims asserted rather than derived — is now closed at the
evidence layer as well as in the product: the artifact's privacy declaration
is computed from what was actually published, checked by contradictions that
throw, and pinned by a checksum that matches everywhere it is quoted.

- **H-RG4: `PASS`.** The group gate is closed.
- **T-581 → `DONE`.** SEC-17 remains enforced below the UI with the opt-in
  disclosure verified in earlier cycles; SEC-10 (dump-exclusion preflight)
  continues to bind any future tier that enables body capture — this slice
  stores no bodies (`omitted` on every archived row).
- **T-582 / H-RG5 is unblocked.** Its `T-581 PASS` dependency is satisfied.
- The V3 deferral transfers one obligation forward: the isolated
  resolver-cost measurement is owed at `R-RG1`, and O1/O2 are available as
  optional hardening for the next harness change.
