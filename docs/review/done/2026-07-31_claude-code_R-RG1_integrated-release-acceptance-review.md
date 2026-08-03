# R-RG1 — Integrated Release Acceptance Review (T-584)

- **Review group:** R-RG1 (integrated release acceptance — Codex backend/CI,
  Claude UI from the closed groups, independent reviewer)
- **Target task:** T-584
- **Scope:** full. The whole release candidate as it stands at `main`, measured
  against the R-RG1 gate as written in `work_status.md` (Task Registry T-584)
  and in §7 `R-RG1` of the paired implementation plans: full Go test/vet/build,
  frontend test/build, Windows live E2E, macOS offline import/package smoke,
  paired documentation, support/security/performance matrices, and honest
  release notes.
- **Reviewer:** claude-code, acting as the independent reviewer for this group.
  Disclosure: the Claude-owned UI halves of H-RG1/H-RG4/H-RG5/X-RG1 are the
  same agent family's work; those groups are closed and are not re-opened here.
  Every claim below is backed by a command I executed, a CI log or artifact I
  downloaded, or a file I read — not by the commit messages, the plan's
  completion checkboxes, or the `work_status.md` narrative. No code, design
  document, `work_status.md` entry, CI configuration, or finding disposition
  was modified by this review.
- **Date:** 2026-07-31
- **Verdict:** `CONDITIONAL`. The platform gates that T-584 actually built are
  real and they pass — I re-ran them locally and re-verified the pushed CI
  evidence down to the archived artifact bytes, and the resolver-cost numbers
  in the documentation match the artifact exactly. But three declared R-RG1
  scope items are not met: the release notes describe only this gate's CI work
  and omit the entire feature wave the release would ship (**R1**); the
  "Windows GUI" half of the plan's own Windows acceptance item was dropped from
  both the plan checklist and the task registry without being run or deferred
  with rationale, and the packaged Windows GUI has not been launched since
  before any of this release's UI existed (**R2**); and the Windows
  live-capture evidence artifact that R-RG1 explicitly *reuses* no longer
  matches the SHA-256 recorded for it in six places, including the H-RG4 fourth
  re-review that granted the `PASS` (**R3** — the content is intact and I
  identified the exact cause, but the pin is currently unverifiable). None of
  the three requires new product code. Do not create a tag or GitHub release
  until they close and a narrow re-review confirms them.
- **Evidence base:** `main` at `7a528dd` ("docs: record T-584 platform
  acceptance evidence") on top of `b666309` ("build: prepare T-584 backend
  release gate"), which is the only code-bearing commit in this gate;
  `.github/workflows/engine-native.yml`;
  `internal/capture/procmap/resolver_windows_performance_test.go`;
  `cmd/archscope-app/capture_windows_e2e_test.go`; `CHANGELOG.md`; the paired
  `BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`, `PERFORMANCE.md`,
  `USER_GUIDE.md`, and `IMPORTER_SUPPORT_MATRIX.md`; GitHub Actions runs
  `30601745253` (Engine Native) and `30601745284` (CI) with their logs and the
  downloaded `r-rg1-windows-resolver-cost` artifact;
  `docs/review/evidence/2026-07-29_t581_windows-live-capture-schema-v4.json`
  and its `.sha256` sidecar.

## What T-584 actually changed

`b666309` is the only commit that touches anything executable, and what it
touches is test and CI surface, not product source:

- `.github/workflows/engine-native.yml` — three Windows-only steps in the
  existing `go` matrix job (live-capture E2E, resolver-cost measurement,
  artifact upload), a `npm run test:state` step in the frontend job, and a new
  `macos-release-smoke` job (offline access-log + Lighthouse import, `task
  package`, `test -x` on the bundle binary, `codesign --verify --deep
  --strict`).
- `internal/capture/procmap/resolver_windows_performance_test.go` — new,
  `//go:build windows`, opt-in behind `ARCHSCOPE_RUN_RESOLVER_PERF=1`.
- Documentation: `CHANGELOG.md`, paired `PERFORMANCE.md`, `USER_GUIDE.md`,
  `IMPORTER_SUPPORT_MATRIX.md`, paired implementation plans, `work_status.md`.

`7a528dd` is documentation only (plans, `PERFORMANCE.md`, `work_status.md`).

I verified the "no product source, no tag, no release" claim rather than
accepting it: `git show --stat` on both commits shows no file under
`internal/` other than the new `_test.go`, no file under `frontend/src`, and
no `frontend/bindings` regeneration; `git tag --points-at HEAD` is empty and
`gh release list` still tops out at the pre-existing `v0.3.5-24`. That part of
the handoff is honest.

## Verification performed

Run from `apps/engine-native` and
`apps/engine-native/cmd/archscope-app/frontend` on darwin/arm64 (macOS 26.5.2,
Go 1.26.5, Node v25.9.0, npm 11.17.0). This is not a Windows host, so every
Windows claim below is verified from the pushed CI run's logs and its uploaded
artifact, not from a local execution — that limitation is stated again in
"Scope limits" at the end.

| Gate | Command / source | Result |
|---|---|---|
| Go build | `go build ./...` | pass |
| Go vet | `go vet ./...` | pass |
| Go tests (uncached) | `go test ./... -count=1` | pass, all packages `ok`, no `FAIL`, no skipped package |
| Frontend state regressions | `npm run test:state` | pass |
| Frontend production build | `npm run build` | pass (`tsc` + `vite`, built in 227 ms) |
| Offline access-log import | `archscope-engine access-log analyze --in examples/access-logs/sample-nginx-access.log` | exit 0, 20,214-byte populated result |
| Offline Lighthouse import | `archscope-engine browser import --format lighthouse-json` | exit 0, 8,605-byte populated result |
| Engine Native CI | `gh run view 30601745253` | `success` on `b666309`; all five jobs `success` |
| Parallel CI workflow | `gh run view 30601745284` | `success` on `b666309`; both jobs `success` |

The two offline imports are the exact commands the new `macos-release-smoke`
job runs; I reproduced them locally against the same inputs and confirmed the
emitted results are populated `AnalysisResult` JSON, which the CI job itself
does not assert (see **O4**).

### The Windows evidence holds up on inspection

This is the part of the gate most worth checking independently, because it is
the part I cannot run. It survives:

- The live-capture E2E step really executed: the Windows job log shows `go test
  -count=1 ./cmd/archscope-app -run '^TestWindowsLiveCaptureE2E$'` returning
  `ok ... 0.075s`, not a skip. Reading the test, it is a real end-to-end path —
  it starts a capture service, proxies a request through it, and asserts
  `Persisted == 1` with `Observed/Captured == 1` and `Unattributed/Dropped ==
  0`, `Attribution == "confirmed"`, `Fidelity == "decoded_wire"`, `CaptureMode
  == "proxy_mitm"`, and `BodyStorage == "omitted"` on both request and
  response. The `confirmed` assertion means the CI run genuinely exercised
  `GetExtendedTcpTable` owner attribution, not a stub.
- The resolver measurement really executed and really measured: `--- PASS:
  TestWindowsResolverCostMeasurement (0.03s)` with
  `ARCHSCOPE_RUN_RESOLVER_PERF=1` present in the step env. The 0.03 s wall time
  reconciles with 50 samples at a 0.539 ms mean plus one warm-up call.
- I downloaded the uploaded artifact (`r-rg1-windows-resolver-cost`, artifact
  ID 8782187263, 393 bytes) rather than trusting the log excerpt. Its contents:
  `samples: 50`, `confirmed_samples: 50`, `mean_ms: 0.539`, `p50_ms: 0.547`,
  `p95_ms: 0.731`, `max_ms: 0.857`, `os: windows`, `architecture: amd64`,
  `owner_table_reads_per_sample: 2`,
  `connection_scope: accepted_loopback_tcp_connection`,
  `acceptance_policy: measurement_only_no_predeclared_product_slo`. Every
  number quoted in `docs/en|ko/PERFORMANCE.md`, in the plan checklists, and in
  `work_status.md` matches the artifact exactly. There are no process paths, no
  session identifiers, and no host names in it.
- The measurement is not measuring a cache. I checked this specifically,
  because H-RG4 R6 introduced *connection-scoped resolver caching* and a test
  that resolves the same connection 50 times would otherwise be timing a map
  lookup. `procmap.Resolver` holds no cache — `Resolve` calls `ownerRows()`,
  matches, builds the process instance, then calls `ownerRows()` a second time
  and re-reads the start time to set `Attribution = "confirmed"`
  (`internal/capture/procmap/resolver.go:34-76`). The caching R6 added lives
  above this type. So each of the 50 samples pays two real
  `GetExtendedTcpTable` round trips (IPv4 + IPv6 per read) plus
  `OpenProcess`/`QueryFullProcessImageName`/`GetProcessTimes`. The stated
  measurement contract is accurate.
- The "R-RG1 reuses the full external T-581 matrix" claim in the user guides is
  defensible on drift grounds, which I checked rather than assumed: since the
  harness artifact was produced (`4995586`, 2026-07-30), the only non-test
  change under `internal/capture/**` and `cmd/archscope-app/captureservice.go`
  is a single line in `66d346a` adding
  `httpanalyzer.SetDiffCaptureSessionRef(...)` to the finalized-analysis path.
  The live capture, redaction, attribution, recovery, and persistence paths the
  harness exercised are byte-identical to what ships.
- I spot-verified the reused artifact's own content: `privacy.trafficScope =
  loopback_fixture_only`, `sourceRows 1023 = archivedRows 1012 +
  omittedNonFixtureRows 11`, `contradictions: []`, and a whole-file URL scan
  finds exactly two distinct hosts, `127.0.0.1:18080` and `127.0.0.1:18443`.
  The content is what H-RG4 accepted. Its *checksum* is a separate problem —
  see **R3**.

### Documentation and matrices

Paired EN/KO parity holds for everything T-584 touched: the new "Windows
Resolver Cost" / "Windows 리졸버 비용" `PERFORMANCE.md` sections carry the same
command, the same schema-v1 field list, the same explicit
"no predeclared SLO, so no retrofitted threshold" statement, and the same
run-`30601745253` result paragraph. `USER_GUIDE.md` and
`IMPORTER_SUPPORT_MATRIX.md` moved in both languages together. The importer
matrix addition is careful in the way this project's earlier gates demanded —
it states that live capture is not a new importer and enumerates the narrower
scope (Windows-only, loopback, HTTP/1.x metadata, no bodies, H2/QUIC
passthrough/unsupported, connection-specific attribution rather than
whole-machine coverage). `docs/en/SYSTEM_HTTP_CAPTURE.md` needed no R-RG1 edit;
its SEC-10/SEC-17 rows still describe the binding conditions accurately and
contain no stale release-tier status.

The user guide's status wording was also corrected honestly: the HTTP evidence
row and the Windows live-capture section now say the tier passed H-RG4 and
remains a release candidate until R-RG1 passes, replacing the stale "third
`CONDITIONAL` dated 2026-07-29" text.

## Blocking findings

### R1 — The release notes describe the release gate, not the release

`CHANGELOG.md` is the only release-notes surface in the repository (I searched:
no `RELEASE_NOTES`, no other file carrying an `Unreleased` section). Its
`[Unreleased]` section, as written by `b666309`, contains exactly three
bullets: the Windows release-gate coverage, the macOS smoke job, and the
frontend gate now running state tests — plus a Security paragraph. That is a
truthful description of T-584. It is not a description of the release.

Between the `v0.3.5` tag (`9b2ece0`, 2026-05-17) and `HEAD` there are 136
non-merge commits, of which roughly 25 are user-facing `feat` commits, and
`git log --since=2026-05-17 -- CHANGELOG.md` returns exactly one commit:
`b666309`. So none of the following has a changelog entry:

- Chrome Performance trace and V8 `.cpuprofile` analysis with the browser CPU
  profile page (C-RG1 / T-578).
- Offline HAR 1.2 `http_capture` import with dialect detection, import-time
  redaction, and the HAR analysis page (H-RG1 / T-579).
- The Windows live HTTP capture engine and its live-capture UI (H-RG3/H-RG4 /
  T-580, T-581) — the flagship of this wave, and the subject of the one
  Security paragraph that *was* written.
- HTTP session Diff with the grade-aware comparison UI (H-RG5 / T-582).
- HTTP × profile/server-evidence correlation with the drilldown/overlay UI
  (X-RG1 / T-583).
- Lighthouse report JSON import and the dedicated browser-audit page (T-585,
  T-586).

A reader of these notes at release time would conclude that the release added
CI gates and changed nothing else, while the `Added` heading sits directly
above two CI bullets. The plan's own R-RG1 requirement is stronger than
"mention the limits": "Release notes distinguish offline HAR, Windows live
tiers, H2/QUIC/pinning, and coverage limitations" — the Security paragraph does
the limits half well, but there is nothing for it to qualify, because the
features it constrains are never announced. `IMPORTER_SUPPORT_MATRIX.md` still
opens with "as of `v0.3.5`" while listing importers that shipped after it
(**O5**), which is the same gap seen from the other side.

**To close:** write the `[Unreleased]` `Added` / `Changed` entries for the
post-0.3.5 feature wave, keeping the existing Security paragraph, and keep the
Windows live tier explicitly marked as the narrower tier it is. No code change.

### R2 — The Windows GUI half of the acceptance item was dropped, not run and not deferred

§7 `R-RG1` of both implementation plans, before `b666309` rewrote it, read:
"Run full Go test/vet/build, frontend state tests/build, **Windows GUI/live
E2E**, and macOS offline-import/package smoke." The rewritten checklist
replaces that with a `[x]` item covering only "the native Windows in-process
live-capture E2E", and the T-584 Task Registry row in `work_status.md` likewise
now reads "Windows live E2E". The word GUI disappears from both without a
deferral note.

This is not a wording nit, because the thing it named is genuinely unverified.
`.github/workflows/windows-gui-smoke.yml` — which builds
`archscope.exe` via `task windows:build` and launches it through
`scripts/windows-gui-smoke.ps1` — triggers only on `workflow_dispatch` or on
pushes that touch the workflow file or that script:

```yaml
    paths:
      - ".github/workflows/windows-gui-smoke.yml"
      - "scripts/windows-gui-smoke.ps1"
```

`gh run list --workflow=windows-gui-smoke.yml` returns three runs ever, the
most recent on **2026-07-20** at `1a07346` — before the HTTP Capture page, the
live-capture panel, the session-diff comparison UI, the correlation panel, and
the Lighthouse page existed. The packaged Windows binary has therefore never
been launched with any of the UI this release ships, while the same gate does
smoke the macOS bundle end to end. Windows is also the only platform on which
the flagship live-capture feature runs at all. `work_status.md` item 8 of the
Next Execution Queue still instructs "Keep release verification healthy before
each release cut by repeating Windows GUI smoke", so the project's own standing
rule agrees.

The project precedent here is clear and cheap to follow: H-RG4 V3 was accepted
precisely *because* the deferral of the resolver measurement was recorded with
a rationale rather than quietly dropped.

**To close:** either dispatch `windows-gui-smoke.yml` against the release
candidate and record the run ID, or restore the GUI item in both plans and the
registry row with an explicit deferral and rationale. Either is acceptable;
silently narrowing the scope is not.

### R3 — The reused Windows evidence artifact no longer matches its recorded checksum

R-RG1 does not re-run the external Windows matrix; `USER_GUIDE.md` (both
languages, as edited by this gate) states that R-RG1 "reuses that full external
matrix". That makes the pinned artifact part of *this* gate's evidence, so I
verified the pin:

```
recorded (sidecar + 5 other records):
  69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111
actual  (shasum -a 256 of the repository copy):
  2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e
```

The digest is recorded in `docs/review/evidence/…json.sha256`, in
`work_status.md` three times (lines 35, 616, 668), in the H-RG4 fourth
re-review at `docs/review/done/2026-07-30_…fourth-re-review.md:68`, and in both
implementation plans at line 279. All six are currently unverifiable from the
repository.

The content is not corrupt, and I established that rather than leaving it as a
suspicion. The repository copy is 598,903 bytes with 17,476 LF and **zero** CR
bytes, and:

```
perl -pe 's/\n/\r\n/' <artifact> | shasum -a 256
  → 69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111
```

The recorded digest is the digest of the **CRLF** form. `.gitattributes`
contains `docs/review/evidence/*.json text eol=lf`, so the checksum was
computed on the Windows-side original and the file was then normalized to LF on
commit. Every blob ever committed for this path hashes to the LF value; the
CRLF bytes never existed in the repository. Content integrity is intact — the
JSON still shows `1023 = 1012 + 11`, empty `contradictions`, and loopback-only
hosts, as noted above.

What is broken is the pin, and the pin is the mechanism the whole H-RG4 →
R-RG1 evidence chain rests on. Anyone who follows the documented step of
checksumming the archived artifact gets a mismatch and has no way, short of
the analysis above, to distinguish "line-ending normalization" from "the
evidence was altered". The fourth re-review's finding that "the checksum
matches the sidecar and every record" is not reproducible today.

**To close:** record the LF-normalized digest
(`2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e`) in the
sidecar and the five other records, with a one-line note that the artifact is
stored LF-normalized under `.gitattributes` and that the original Windows CRLF
digest was `69565684…30111`; or mark the path `binary`/`-text` and re-commit
the byte-exact original. Do not silently swap the number without the note — the
old digest appears in a closed review document, and a bare replacement would
make that document look wrong instead of explaining it. No code change.

## Non-blocking observations

- **O1 — The new Windows E2E step adds visibility, not coverage.**
  `TestWindowsLiveCaptureE2E` carries no environment gate, so the matrix job's
  `go test ./... -race -count=1` already runs it on `windows-latest`; the
  dedicated step re-runs the same test a second time. Useful as a named,
  fail-loud gate, but the plan checkbox reads as if new coverage was added.
- **O2 — `owner_table_reads_per_sample: 2` is a literal, not a measurement.**
  It is only *indirectly* true, via the test's `Attribution == "confirmed"`
  assertion — `Resolve` performs the second `ownerRows()` read solely on the
  path that can set `confirmed`. If a future change lets `confirmed` be reached
  without the verification read, or makes confirmation optional, the archived
  evidence would keep asserting 2 reads while measuring 1. Same class as H-RG4
  O3 (literal `reviewBeforeArchive`). Deriving the count from an instrumented
  counter would make the field self-verifying.
- **O3 — The measured cost is dominated by a variable the docs do not name.**
  `ownerPIDRows()` reads the entire IPv4 *and* IPv6 owner tables and
  `matchingOwnerPID` linearly scans them, so per-sample cost scales with the
  machine's total TCP connection count. The measurement ran on a near-idle
  hosted runner. `PERFORMANCE.md` does say "runner-specific baseline, not a
  product SLO", which is honest as far as it goes, but a reader on a busy
  workstation has no way to know which direction the number moves or why. One
  sentence naming table size as the scaling variable would close it.
- **O4 — The macOS smoke asserts exit status, not output.** The offline import
  step writes two result files and never inspects them; a regression that
  emitted an empty-but-valid `AnalysisResult` with exit 0 would pass the gate.
  I reproduced both commands locally and the outputs are populated (20,214 and
  8,605 bytes), so nothing is wrong today. The inputs are also small — 636
  bytes of nginx log and a 2,920-byte Lighthouse report — which makes
  "representative offline evidence" in the changelog generous. A `jq`-level
  assertion on result type and a non-empty table would make this a real gate.
- **O5 — `IMPORTER_SUPPORT_MATRIX.md` still says "as of `v0.3.5`"** in both
  languages while its table now lists Chrome trace, V8 `.cpuprofile`,
  Lighthouse, and HAR importers that shipped after that tag. Related to **R1**
  and worth fixing in the same pass.
- **O6 — Only the macOS bundle is smoke-packaged before the tag.**
  `release.yml` packages darwin/arm64, windows/amd64, and linux/amd64 into
  `.dmg` / NSIS `.exe` / `.deb`+`.rpm`, and it runs *only* on `v*` tag push. If
  the Windows or Linux packaging path has drifted, the first signal arrives
  after the tag exists. Overlaps with **R2** but is broader than it.
- **O7 — "Windows native Go/race/live-E2E passed" reads fuller than it is.**
  The in-process E2E is a plain-HTTP proxy smoke over one transaction; the
  browser/curl/JVM/Electron, HTTPS, pinning, H2 passthrough, QUIC-absence,
  long-session, re-entry, and recovery matrix rests entirely on the reused
  T-581 harness artifact. The test's own doc comment and the user guides say
  this plainly; the `work_status.md` summary sentence does not, and it is the
  sentence most likely to be read at release time.

## Scope limits

- I ran no Windows host. Every Windows claim in this review is verified from
  the `30601745253` job logs, the downloaded artifact bytes, and source
  reading — not from local execution. Whether `GetExtendedTcpTable` behaves the
  same on a loaded production machine as on the hosted runner is exactly what
  **O3** is about, and this review does not settle it.
- I did not re-open C-RG1, H-RG1 through H-RG5, or X-RG1. Their verdicts stand.
  **R3** concerns the *pinning* of H-RG4's artifact, not the artifact's content
  and not H-RG4's verdict.
- I did not execute the T-581 PowerShell harness. The drift check in
  "Verification performed" establishes that the capture engine it exercised is
  unchanged; it does not re-prove the matrix.
- Findings are not dispositioned here, per the reviewer boundary. **R1**,
  **R2**, and **R3** are all documentation, CI-invocation, or record-keeping
  work; none of them requires a product code change, and all three should be
  closable in a single follow-up commit followed by a narrow re-review.
