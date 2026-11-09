# T-603 native Windows body capture — PASS

Date: 2026-11-09. Product source: `413d6cad882770c729bf20b6e61349f8dd85cf1d`.
The user explicitly authorized temporary dump-policy changes for the remaining
body test and required restoration afterward. T-603 is complete; this does not
close the other Stand-alone feature/parity tasks.

## Actual desktop acceptance

The production Windows Wails/WebView2 executable was rebuilt with the pinned
Wails CLI `v3.0.0-alpha2.117`. It requested a 1280×800 window; the native capture
tool returned the 1266×793 bounded image, without resizing it. The idle screen,
live requests and lower inspector were exercised on this physical Windows PC.

`scripts/windows-body-capture-fixture.mjs` ran a loopback origin and two real
Node child processes. Their PID/start-time groups remained distinct. Each
client sent three POSTs (`first`, `while-body`, `while-headers`), for six total
`200 OK` exchanges. All requests and responses were synthetic JSON; no external
site, system proxy change or CA trust installation was involved.

| Acceptance action | Observed result |
|---|---|
| Start with F9 and explicit body opt-in | Both real SEC-10 preflights allowed capture under the temporarily isolated policy |
| Select process A | The grid showed only A's rows, with a scoped count of 2/4 during Body inspection and 3/6 during Headers inspection |
| Inspect Body while another A/B request arrives | Original `/t603/body/a-first` stayed selected; request `T603-REQUEST-A`, response `T603-RESPONSE-A` and Korean `본문 캡처 검증` remained visible |
| Inspect Headers while another A/B request arrives | Original request remained selected; JSON content types, request/response client marker A and fixture marker `T603-body` matched |
| Check measured first exchange | 87 request bytes, 140 response bytes, `200 OK`; process A totals reached 274 request / 433 response bytes across its three rows |
| Stop with F2, then Load finalized analysis | All six rows reopened from the store; selecting the original A request showed the same request/response markers and Korean payload in Body |
| Finish | Capture stopped; fixture servers/clients and the test app exited before policy restoration |

Evidence: [idle](native-idle.jpg), [live captured bodies](native-live-body.jpg),
[live headers](native-live-headers.jpg), [reopened bodies](native-reopened-body.jpg).

## Automated regression and CI gate

With the real Windows policy temporarily isolated:

```powershell
$env:ARCHSCOPE_RUN_BODY_E2E = '1'
go test -count=1 -v ./cmd/archscope-app -run '^TestWindowsBodyCaptureE2E$'
```

The local test passed using the real manager/proxy SEC-10 checks, native TCP
owner attribution, request/response body retention and store-backed detail.
The finalized-detail assertion is also included in the committed test. Ordinary
runs skip this opt-in case. Engine Native CI now runs it after policy preparation
and before the existing forced-crash evidence and always-on policy restoration.

## Policy restoration

The local elevated worker saved recovery state outside Git, held the temporary
policy for at most 30 minutes, and restored on an explicit release signal in
`finally`. It preserved the machine LocalDumps subtree and tracked only these
test executable exclusions: `archscope.exe`, `archscope-app.test.exe`, and
`proxy.test.exe`.

The first restoration check detected that PowerShell registry `Rename-Item`
had recreated primary-group security metadata on three keys. The original
values/subkeys were present. Recovery reconstructed a descriptor set that
**exactly matched the pre-change SHA-256**, then applied it with the registry
ACL API and restored the user exclusions. Both complete trees subsequently
matched their original hashes, including value types/data, subkeys and full
owner/group/DACL descriptors. No temporary backup key remains. The clean
result is [policy-restoration.json](policy-restoration.json); raw recovery
state and local machine configuration are intentionally not published.

The committed local worker and CI policy helper now use native `RegRenameKey`
instead of provider copy/recreation. The local worker persists complete original
trees as well as their hashes and attempts user cleanup even if a machine
restore conflict occurs. `scripts/test-windows-policy-registry.ps1` passed a
round trip with a protected child ACL and an unexpanded `%TEMP%` value, and is
included in Windows CI. These changes make future temporary policy tests
repeatable without the metadata loss found in the initial approach.

Restoration applies to policy and user WER exclusions. Per-process reporting
flags ended with the test processes. Existing capture stores and unrelated
user files were preserved.
