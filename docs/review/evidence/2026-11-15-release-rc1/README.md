# v0.4.0-rc.1 release receipt

Published and verified on 2026-11-15:
[GitHub prerelease](https://github.com/aurakimjh/archscope/releases/tag/v0.4.0-rc.1).
The annotated tag points to `9f0e5116eaef64fd5f87abb381adf90c9d99b2e5`;
tag object `2400ab73d0e722fc263f03f4e3c19ba30d86840c`.
This receipt was committed after publication; the verified tag was not moved.
The latest stable release remains `v0.3.5-24`.

## Test evidence

- [Local validation](local-validation.json): full Go/race, 131 packages each;
  vet; frontend state/build; signed-bootstrap RC compatibility; actual arm64
  app metadata and ad-hoc signature. Local log byte counts/digests identify
  the test output; local checks are not independent review.
- [Hosted validation](hosted-validation.json): all runs passed on the exact
  candidate. Engine Native has nine passing jobs; CI, Windows GUI x64/x86,
  Windows COM x64/x86, three-platform packaging preflight and tag-triggered
  publication also passed. Earlier failed/cancelled preparation attempts are
  preserved in this receipt; UTF-8 decoding and archive modes were fixed
  before the immutable tag was created.
- [Hosted race log receipts](hosted-race-logs.json): each macOS/Windows/Linux
  full race step passed 131 packages, with the command and complete job-log
  digests. Additional Windows acceptance tests are separate from that count.
- [Offline summary](offline-summary.json): 70 network-none CLI operations,
  zero target Internet syscalls, two failed-connect positive-control syscalls.
  Owner-only raw models are not included or made public.

## Published artifact verification

[Preflight assets](preflight-assets.json) were checked before tagging.
[Published assets](published-assets.json) were downloaded after the successful
[publication run](https://github.com/aurakimjh/archscope/actions/runs/43714519800).
[Release metadata](published-release.json) preserves the prerelease/draft flags,
URLs, sizes and GitHub digests. All 11 assets were downloaded; the ten entries in
[SHA256SUMS](SHA256SUMS) match, and all 11 GitHub asset digests match.

ZIP CRC checks passed. macOS app ZIP and DMG contain the same arm64 binary,
short version `0.4.0`, build `4001`; deep/strict ad-hoc signature and DMG
checksum checks passed. This is not Developer ID signing or notarization.
Windows executable and NSIS installer resources carry numeric `0.4.0.1` and
public `0.4.0-rc.1`; the 32-bit NSIS stub packages the x64 app. This workflow
does not Authenticode-sign Windows files. Linux TAR keeps executable mode;
DEB contains the same x64 binary, and DEB/RPM versions are `0.4.0~rc.1-1`.
Manuals, release notes and conformance corpus match the tagged source exactly.
These artifact checks do not imply clean-host installation acceptance.

## Remaining scope

T-601 REVIEW, T-611 REVIEW, T-613 PENDING, T-627 REVIEW, T-654 TODO and
T-662 TODO keep their existing states. Full original-tool/native interaction,
finite memory boundaries, actual two-GPU CUDA/NCCL and natural swap/pressure
qualification remain as documented. A stable 0.4.0 also needs a newly qualified
compatible signed bootstrap pack. See the
[English scope](../../../en/releases/v0.4.0-rc.1.md) and
[Korean scope](../../../ko/releases/v0.4.0-rc.1.md).
