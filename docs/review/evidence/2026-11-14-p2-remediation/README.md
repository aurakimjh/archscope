# WR-01 / WR-02 local remediation evidence

The user requested both P2 fixes, regressions and tests available on the MacBook
and Docker Desktop. Evidence was captured from uncommitted fixes based on
`ab42eeb8`; subsequent publication is tracked in Git history.
The original review remains CONDITIONAL. This is implementation and local
validation evidence, without an independent PASS or new Windows runtime verdict.

## WR-01: preserve retained query spelling

`requestQuery` now slices after the first question mark and before the fragment.
Subsequent question marks, duplicate keys and encoded/malformed percent sequences
remain unchanged; explicit query evidence still takes precedence. A question mark
inside a fragment does not create a query.

The added frontend regression failed on the old code before the fix. The fixed
state suite checks nested query extraction, Path display/filtering/ascending and
descending sort, Query parameter decoding and agreement with Raw reconstruction.
The native Mac production app imports a synthetic two-row HAR, selects request2,
filters Path by `id=7&keep=2` to exactly1/2 rows, and displays `/detail?id=7` plus
`keep=2` in Query and the complete request target in Raw. HAR/native integration
is distinct from the retained-URL fallback exercised directly by state tests;
native live capture is unavailable on this Mac.

The standard package is ad-hoc signed and verified. For attributed UI testing,
the same executable code is packaged with the private test bundle identifier
`com.archscope.p2qa.20261114` and name `ArchScope P2 QA`, then re-signed. Removing
signatures from comparison copies yields byte-identical executables, and the
observed executable contains the exact current production HTTP frontend asset.
Only test-bundle metadata/signature differ. The initial ordinary-ID observations
are excluded from public acceptance evidence because attribution was ambiguous.
The isolated app's native AX readback and screenshots are the published proof;
its process is confirmed stopped after testing.

## WR-02: refuse incomplete soak runs

The Windows runner uses schema4 and imports the shared predicate module. Each
checkpoint requires running state, valid nonnegative integer counters and zero
drops. Captured/persisted mismatch may drain for up to5seconds between bounded
controller calls. A state change or persistent mismatch fails the run.

Finalization requires an explicit-session stop returning finalized and a fresh
authoritative stats response with finalized state, zero drops and equal counts.
There is no post-stop in-flight allowance. Save, readback and open must then
succeed with a nonempty matching retained count and selected ID before the runner
sets passed. Public finalStats excludes the private session ID.

The28 Node tests cover healthy capture, transient drain, deadline expiry,
recoverable/failed/paused/unexpected states, invalid/lost counters, bad final
counters, stop/stats/save/open failures and saved count/selection disagreement.
Both macOS Node and a network-none Linux Node22 container pass. The workflow now
runs these regressions and triggers on runner/helper/test changes. Future native
receipts must pin the shared module as well as the runner. Existing two-hour
runner copies, reports and artifact hashes were not changed.

## Additional host and container checks

- Mac frontend state suite and production build; whole Go race suite
  (131passing packages,4without tests,1,898passing test events,7skipped,0failures)
  and vet;
  native production build, ad-hoc package/signature checks and HAR UI observations.
- Mac real loopback common-engine soak with race:121.264seconds,56cycles,
  14,560requests, maximum64retained rows. This is separate from native UI testing.
- Native CLI access-log import produces6sample rows; Lighthouse import produces
  4audit rows.
- Docker Desktop Linux/arm64 disconnected clean-install observer:70operations,
  zero observed inet syscalls; failed-connect positive control detected2calls.
  Offline JFR conversion is refused before invocation.
- Four Linux/arm64 capture test binaries (store/session/proxy/stream), including
  75passing test/subtest events, pass in a
  read-only network-none container with a1GiB temporary filesystem. An initial
  256MiB tmpfs run failed the existing512MiB disk-reserve guard; that environmental
  failure is retained in local.json. Increasing only the test mount capacity
  resolves it; production reserve policy and tests were not weakened.
- Windows amd64/386 GUI production cross-builds have verified PE architectures.
  They are compilation evidence, without Windows execution.
- Deterministic offline/input-assistance assets, desktop-only guard, HTTP report
  and synthetic-fixture contracts, workflow YAML and diff checks pass.

The exact check summaries, executable/source hashes and limitations are in
[local.json](local.json); the public file digest list is
[artifact-sha256.json](artifact-sha256.json). Raw full test output and binaries
remain in the task-specific locally excluded directory. The preexisting Docker
container remains running, task containers are removed, and no system proxy,
trust, body-capture policy or driver setting was changed.

T-613 remains PENDING and T-627 remains REVIEW. The revised Windows runner still
needs a fresh native Windows run; these Mac/Linux checks do not replace that
execution, full reference/locale/DPI acceptance or longer memory qualification.
