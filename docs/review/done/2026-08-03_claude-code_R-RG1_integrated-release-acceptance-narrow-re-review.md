# R-RG1 — Narrow Re-Review (T-584)

- **Review group:** R-RG1 (integrated release acceptance — Codex backend/CI,
  Claude UI from the closed groups, independent reviewer)
- **Target task:** T-584
- **Scope:** narrow. R1–R3 of the 2026-07-31 `CONDITIONAL` review only: the
  release notes describing the gate instead of the release (**R1**), the
  silently dropped Windows GUI half of the Windows acceptance item (**R2**),
  and the reused Windows evidence artifact whose recorded SHA-256 no longer
  matched the repository bytes (**R3**). The first review's platform
  verification (local Go/frontend gates, CI runs `30601745253` and
  `30601745284`, the resolver-cost artifact) is not repeated here and does not
  need to be: the remediation commit changes no buildable surface, which this
  review verifies below rather than assumes.
- **Reviewer:** claude-code, acting as the independent reviewer for this group.
  Disclosure: as in the first R-RG1 review, the Claude-owned UI halves of the
  closed groups are the same agent family's work; those groups are not
  re-opened here. Every claim below is backed by a command executed against
  the repository, a digest recomputed from the archived bytes, or GitHub run
  metadata fetched independently — not by the remediation commit message or
  the `work_status.md` narrative. No code, design document, `work_status.md`
  entry, CI configuration, or finding disposition is modified by this review;
  the commit that carries it adds only this document.
- **Date:** 2026-08-03
- **Verdict:** `PASS` for the narrow R1–R3 scope. All three findings are
  closed the way the first review's closure instructions asked, including the
  two failure modes those instructions warned against: the scope was not
  narrowed again (the GUI item is restored *and* was actually run), and the
  checksum was not silently swapped (the original CRLF digest is retained as
  provenance everywhere the new pin appears, and the closed H-RG4 review was
  annotated, not rewritten). No blocking finding remains open in the R-RG1
  review chain. With the first review's platform verification carrying over
  unchanged, R-RG1 is closed with `PASS`; the "no tag or GitHub release
  before `R-RG1 PASS`" constraint is satisfied and tagging becomes an
  unblocked release decision, not an obligation of this review.
- **Evidence base:** `main` at `b48511d` ("docs: resolve R-RG1 conditional
  findings") on top of `f5efea8` ("review: R-RG1 integrated release acceptance
  CONDITIONAL (T-584)"); `CHANGELOG.md`; `git log v0.3.5..HEAD`; the paired
  `BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` and
  `IMPORTER_SUPPORT_MATRIX.md`; `.github/workflows/windows-gui-smoke.yml` and
  `scripts/windows-gui-smoke.ps1`; GitHub Actions run `30809473903` (metadata
  fetched from GitHub, not from the repository's own records);
  `docs/review/evidence/2026-07-29_t581_windows-live-capture-schema-v4.json`
  and its `.sha256` sidecar, with digests recomputed from the archived bytes;
  `docs/review/done/2026-07-30_claude-code_H-RG4_windows-live-capture-ui-e2e-fourth-re-review.md`;
  `work_status.md`; `.gitattributes`.

## What the remediation actually changed

`b48511d` is the only commit after the first review, and `git show --stat`
confirms it touches exactly eight files, all of them documentation or review
records: `CHANGELOG.md`, both implementation plans, both importer matrices,
the archived H-RG4 fourth re-review, the evidence `.sha256` sidecar, and
`work_status.md`. No file under `internal/`, `cmd/`, `frontend/`, or
`.github/` changes; `git tag --points-at HEAD` is empty; the pushed `main` is
`b48511d` itself. The handoff's "no product source, tag, or release changes"
claim is verified, and it is what lets the first review's platform results
carry over: the binaries, tests, and CI configuration those results describe
are byte-identical at this commit.

## R1 — closed: the release notes now describe the release

`[Unreleased]` in `CHANGELOG.md` now carries six `Added` entries covering the
post-`v0.3.5` feature wave, and I cross-checked them against the history
rather than against the finding's list: `git log v0.3.5..HEAD --no-merges`
(138 commits) contains user-facing `feat` work in exactly six families, and
each maps to an entry — Chrome Performance trace / V8 `.cpuprofile` analysis
(C-RG1 / T-578), Lighthouse report import and the browser-audit page
(T-585 / T-586), bounded HAR 1.2 import (H-RG1 / T-579), the opt-in Windows
live HTTP capture tier (H-RG3/H-RG4 / T-580, T-581), HTTP session Diff
(H-RG5 / T-582), and HTTP evidence correlation (X-RG1 / T-583). No feature
family in the log is missing from the notes.

The first review's secondary requirements hold as well: the Security
paragraph is retained verbatim and now qualifies features that are actually
announced above it; the live-capture entry itself carries the narrow-tier
qualifiers ("opt-in", "loopback-only", "connection-specific") rather than
reading like a general capture feature; and the two release-gate bullets that
previously masqueraded as the whole release moved under `Changed`, which is
what they are. **O5** closed with it: both importer matrices now describe
"the currently supported local evidence importers, including the unreleased
browser-profile, Lighthouse, and HAR additions after `v0.3.5`" instead of
claiming a `v0.3.5`-only scope.

## R2 — closed: the Windows GUI smoke was restored and actually run

The finding offered two acceptable closures — run the gate, or restore the
item with an explicit deferral. The remediation did the stronger one, and the
record and the run both check out:

- **The record:** §7 `R-RG1` of both plans again carries the Windows GUI item,
  as a checked entry naming workflow run `30809473903`, and the T-584 registry
  row again reads "Windows GUI/live E2E". The scope that was silently narrowed
  is restored in both places the first review named.
- **The run, verified from GitHub rather than from the repository's own
  claim:** run `30809473903` is a "Windows GUI Smoke" workflow run on `main`
  at commit `f5efea8`, manually dispatched on 2026-08-03, conclusion
  `success`, with its single "Build and launch Wails exe" job succeeding in
  about three minutes.
- **What that success asserts,** read from the workflow and
  `scripts/windows-gui-smoke.ps1` as pinned at that commit: the job builds
  `archscope.exe` via `task windows:build ARCH=amd64`, and the script throws
  if the binary is smaller than 1,048,576 bytes, launches it, polls every
  second, and throws if the process exits at any point inside the 15-second
  startup window before attempting graceful shutdown. "Stayed alive for the
  15-second smoke window" is therefore script-enforced, not narrative.
- **Freshness, which was the substance of the finding:** the gap R2 named was
  a last GUI launch on 2026-07-20 at `1a07346`, before any of this release's
  UI existed. The new run's commit `f5efea8` contains every UI surface of the
  release, and the only commit after it (`b48511d`) is documentation — so the
  smoked binary was built from exactly the product source this release would
  ship.

One caveat is recorded for honesty rather than as a finding: the specific
figure of an 18,909,184-byte executable quoted in the plans and
`work_status.md` comes from the run's log output, which this environment
could not re-read; what this review independently confirms is the run's
`success` conclusion, its commit, its date, and the assertions the pinned
script enforces (size floor, launch, 15-second liveness). That is sufficient
for the R2 closure, whose requirement was that the GUI gate be run against
the release candidate and recorded.

## R3 — closed: the evidence pin verifies again, with provenance intact

- **The pin verifies.** `sha256sum` recomputed over the repository's archived
  artifact yields
  `2737133818ee1881cfb6ee73f622e1ca6137b63b1818ee699a7eae5fc7218c7e`, which
  now matches the `.sha256` sidecar and every active record: `work_status.md`
  (three occurrences), both implementation plans, and the H-RG4 fourth
  re-review. Anyone following the documented verification step gets a match
  again.
- **The provenance survived, as the closure instruction demanded.** Beside
  each new pin, the records state that this is the repository's LF-normalized
  digest under `.gitattributes` and that the original Windows CRLF artifact
  digest was `69565684d57b20d763ed477f731a9eb836bcc8fbde657cdff10bce0085030111`.
  The closed H-RG4 fourth re-review was annotated with a dated correction
  note ("corrected during R-RG1 on 2026-08-03") rather than silently
  rewritten, so its original finding remains explicable instead of looking
  wrong.
- **The equivalence claim was re-proven, not inherited.** Re-expanding the
  repository bytes to CRLF (`s/\n/\r\n/`) and hashing recomputes exactly the
  original `69565684…30111`, confirming the two digests describe the same
  content in two line-ending forms and that no content change occurred. The
  artifact's substance was also spot-rechecked: `1,023 = 1,012 + 11` still
  reconciles and the contradiction set is still empty.
- **The pin is stable going forward.** `.gitattributes` still forces
  `docs/review/evidence/*.json` and `*.sha256` to LF, so the recorded digest
  and the repository bytes can no longer drift apart by normalization.

## What remains open

Nothing blocking, anywhere in the R-RG1 chain. Non-blocking observations
**O1–O4**, **O6**, and **O7** from the first review remain open as recorded
there (**O5** closed with R1 above), alongside the X-RG1 O1/O2/O4/O6–O11,
H-RG4 O1/O2, and H-RG5 O1–O4 hardening items — none is a gate condition.
Per the standing rule, the actual version tag and GitHub release remain a
separate decision taken after this gate, not an action of it; `v0.4.0`
remains reserved for the broader Evidence Studio roll-up.

## Scope limits

- No Windows host was run for this re-review. The R2 verification rests on
  GitHub's run metadata for `30809473903` and on reading the workflow and
  smoke script as pinned at `f5efea8`; the run's log bytes (and the exact
  executable size they report) were not independently re-read.
- The first review's platform gates (local Go/frontend suites, CI runs, the
  resolver-cost artifact bytes) were not re-executed. The justification is
  stated in "What the remediation actually changed": `b48511d` alters no
  buildable file, so re-running them would re-verify an identical surface.
- Findings are dispositioned by the project, not by this review; this
  document records the verdict only.
