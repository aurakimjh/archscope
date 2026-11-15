# v0.4.0-rc.1 release receipt

Local preparation on 2026-11-15 (Darwin arm64) is recorded in
[local-validation.json](local-validation.json). It includes full Go/race/vet,
frontend state/build, signed-bootstrap compatibility and actual ad-hoc-signed
macOS bundle checks. Test log byte counts and SHA-256 digests identify the
local logs; they do not turn local checks into independent review.

[offline-summary.json](offline-summary.json) is the bounded public summary of
70 network-none CLI operations. Target Internet syscalls were zero; the failed
connect positive control produced two. Owner-only raw models are deliberately
not included or made public.

Hosted CI, three-platform packaging preflight, tag publication and downloaded
release assets are pending on the preparation commit. Publication results will
be appended in a separate receipt after observation. The tag must point to the
candidate that passed those hosted gates.

T-601/T-611/T-613/T-627/T-654/T-662 keep their existing states. See the
[English release scope](../../../en/releases/v0.4.0-rc.1.md) and
[Korean release scope](../../../ko/releases/v0.4.0-rc.1.md).
