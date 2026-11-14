# T-613 live columns, grouping, stored search and annotations

Windows production builds at `eab03d07` include two separately committed fixes:

- `d542b85a`: annotation replacement tolerates transient Windows readers using
  the existing bounded one-second retry. A permanent lock returns an error,
  preserves committed bytes and removes staging files.
- `eab03d07`: Offset uses each PID/start process origin independently of the
  selected display grouping. The same request previously changed from 0.000s
  under process grouping to 0.015s without grouping.

The earlier product baseline is `8d54d2e6`. Its three FAIL observations remain
in native-cases.json: one annotation-lock failure and two views of the Offset
defect. There are 40 native observations, 37 selected PASS and 3 historic FAIL.
These are scoped observations, not an overall contract parity verdict.

## Fixture and independent data checks

Each production desktop owns two real Node producer processes, attached by PID
and start time. They send 600 requests through the explicit proxy to one labeled
Docker loopback origin. Every exchange is confirmed to one producer. GET/POST
and both hosts each have 300 rows; 200/404/503 and the three MIME types each have
200. Bodies are disabled. The native reducer holds 500 rows, the backend live
window 64 and physical retention 640. The first 60 archive requests are outside
the initial native live tail.

Ten independently isolated controller searches pass in all three runs: URL 60,
response header 40, request header 20, query 10, cookie name 30, omitted body 0,
empty POST data 0, reconstructed stream 60, uppercase URL 60 and no-match 0.
The JSON receipts include synthetic sequences and expected/actual counts.
These do not establish positive retained-body or POST-body search.

Another 80 requests produce 680 captured/persisted, 640 retained, 40 evicted and
zero drops/skips. Native fresh search returns the 20 remaining archive rows.
On amd64, Next with the old cursor refuses continuation; Search clears the error
and starts a fresh result. Its error is still generic English, and recovery is
manual. Native pagination is 25/25/10 on baseline amd64 and fixed386. Virtual
rows are only claimed visible when mounted; page totals do not prove all rows
were on screen at once.

Both fixed runs annotate sequence50 while capture is running. The selected
request survives the first40 evictions. Star and exact Korean comment persist
through new traffic, searching/refinding, stopping, saving and native reopening.
Each 640-row export has exactly the durable store's unique ID set (41..680),
selected ID50 and one annotation. Independent Python readback checks every
other field, accounting explicitly for portable-session normalization: host
includes the port and query is derived from the retained redacted URL. Those
two fields change on all640 rows; only star/comment additionally change on one.
This is not a false claim of raw byte equality with live store records.

amd64 uses the product export controller for its640-row save and CtrlO for native
open; fixed386 uses actual CtrlS and CtrlO. Baseline amd64 also used native CtrlS
for600 rows. A mistaken null sessionId in a later baseline harness command caused
the runner to stop; it is documented in validation.json and is not counted as a
product export failure or as a completed680-row baseline export.

## Native UI scope

On amd64, the ten alpha-query rows verify all six group modes: process5+5,
host6+4, method5+5, status4+3+3, MIME4+3+3 and no grouping. Received-byte totals
are independently consistent with42B per row. Both process origins stay0.000s,
including without grouping despite their separated start times. Offsets are
relative to each process within the displayed result set, not a new claim of
an absolute session-wide zero across arbitrary filters/pages.

Both architectures move sequence after Started and change its width56→96.
amd64 additionally hides Method, then uses a fresh run to hide Host/Path and
show optional Offset. Preferences survive a new analyzer and a fully stopped
and restarted desktop using the same isolated WebView profile. Fresh processes
opening their portable files restore selection/star/comment. Group and detail
tab reset on open. The full six-group native matrix is amd64;386 additionally
covers process groups, pagination, column order/width and the note lifecycle.

For the lock test, an owned filesystem reader denies delete sharing, waits for
annotation staging creation, then releases250ms later. Baseline native save
fails immediately with Access is denied. Both fixed native builds succeed with
exact disk readback. Windows unit regressions additionally hold the lock for
the whole bounded retry and verify unchanged old bytes and no staged files.

## Verification and remaining conditions

Both architectures pass all24 capture packages and production app tests
(36.431s amd64,47.402s386); state regression and production frontend/builds pass.
validation.json pins source commit, binary hashes, commands and limitations.
The used fixture/reader/restart harness sources are archived as receipts, with
their private workstation paths; they are not a general reusable runner.
The reusable export verifier and Docker origin live under scripts/acceptance/layout.

T-613 remains PENDING. This run does not close every column value, drag resize,
filter/group/keyboard combination, native annotation length/redaction/failure
boundary, positive body search, reference interoperability or locale/DPI case.
The full vendor COM/foreign session-cookie, driver/service/proxy-ignoring reach,
body-policy, committed-corruption/power-loss and longer memory/handle gates
remain separate. Earlier two-hour receipts are unchanged; no fresh soak or
Windows race result is claimed with CGO=0. T-627 remains REVIEW.

Cleanup leaves zero owned desktop windows or producer processes; the labeled
Docker fixture is stopped/removed and all seven preexisting containers remain.
No system security/privacy, trust, COM registration or body policy is changed.
Raw descriptors, credentials, captures and process inventories remain private.
artifact-hashes.json covers exact public bytes, excluding itself.
