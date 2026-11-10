# Native Windows follow-up — 2026-11-11

T-623 is complete. T-601, T-608, T-611, T-613 and T-627 retain the specific
conditions below. This is native local evidence, not complete Stand-alone parity.

## Changes and reproducible checks

Product/build/harness commit: `7f698067afa899d2a95b5d6b79a0da2fd30e15c5`.
The earlier visual run used `ecf42163e0a1d9d6a64544335b55311d85515469`;
the fix changes build/harness/CI files, with no renderer or Go product edits.
See [validation.json](validation.json) for exact binary hashes and check scopes.

| Check | Result |
|---|---|
| Native amd64 `go test ./...`, `go vet ./...` | PASS |
| Native 386 `go test -count=1 ./internal/capture/...` | PASS |
| Real TCP-owner E2E and single/two-process selection | PASS |
| Suspended executable and Unicode/space/quote/empty-argument/cwd shortcut launch | PASS |
| Frontend `npm ci`, state regressions, production build | PASS; zero npm audit vulnerabilities |
| Desktop-only policy and actual server compilation refusal | PASS |
| Both architecture production GUI builds and 15-second process liveness | PASS; smoke used forced shutdown after graceful timeout |
| Both architecture authenticated desktop fixture harness | Four of sixteen parity cases PASS; aggregate UNVERIFIED |
| Both architecture COM activation/lifecycle/log enumeration/Item/PID/save | PASS for the ArchScope late-bound contract |
| Native-host service/proxy soak | PASS: 120.998s, 98 cycles, 25,480 synthetic requests |
| Local race detector | NOT RUN: no C compiler; `CGO_ENABLED=0` |

Build from `apps/engine-native/cmd/archscope-app`, setting `GOARCH` to `amd64`
or `386`, through `node build/windows/build.mjs -tags production -trimpath
-ldflags "-H windowsgui -X main.desktopBuildCommit=COMMIT" -o ABS_EXE .`.
Use the checked-in `windows-gui-smoke.ps1`, `verify-http-standalone.mjs` and
`verify-http-com.ps1` against the outputs. COM registration was per-user,
ArchScope-owned and removed after each run. The private fixture/controller stores
and diagnostic stderr remain in the untracked local `.tmp/windows-20261111` tree.

### Runtime defects reproduced and corrected

The first 386 executable built successfully but panicked at package initialization:
`compileCallback: argument size is larger than uintptr`. Wails 3.1.0 registers
an eight-byte `POINT` by value as one argument. The x86 callback wrappers now
receive its two 32-bit stack slots and forward the original point unchanged.

After that fix, native window initialization exposed a second fatal failure:
`Failed to find GetWindowLongPtrW procedure in user32.dll`. The 386 SDK copy now
imports `GetWindowLongW`, `SetWindowLongW` and `SetClassLongW`. These Ptr names
are macros on Win32, as described by
[Microsoft's Win32 API documentation](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowlongptrw).
Both SDK source hashes and version 3.1.0 are checked before patching a temporary
copy/modfile; neither module cache nor dependency pins change. Native GUI CI
now builds and launches both amd64 and 386.

The original control harness could finish before the native window failed.
It now checks process survival for fifteen seconds after descriptor readiness
and writes bounded stderr/failure diagnostics only to its private run directory.
The public report retains `nativeWindowVerified=false`: process liveness is
not a visual verdict.

PowerShell returned null on direct COM property reads without ITypeInfo, even
though method calls ran. Explicit `InvokeMember(GetProperty)` fixes this and
preserves COM collection objects instead of pipeline-enumerating them on return.
The corrected script successfully checks BuildCommit, SessionID, logging/paused
states, ProxyAddress, log Count and entry URL/PID on x86 and x64 servers.

## T-623 actual production window

The production Wails/WebView2 app, with no QA menu, recorded ten loopback-only
synthetic POSTs from two real Node child processes. Every final stored row was
`complete`, status 200 and `confirmed` against its PID/start-time pair: five per
process. No CA installation, global proxy change or dump-policy change was used.
Body capture was off; the Body view correctly disclosed non-recording.

- [Live Headers](live-headers.png): the first A request remains selected while
  another A/B pair arrives; 4/4 total rows and matching synthetic A headers.
- [Filtered Body](live-filter-body.png): process B shows 4/8 rows, follow is off,
  its original request remains selected, and Body truthfully says not recorded.
- [Native menu reentry](live-reentry.png): Analyze → Profiler and Capture → HTTP
  Capture preserve the same running session, proxy and all ten requests, including
  the pair sent while away. Page-local selection/filter/follow reset on remount.
- F2 finalized recording. Load finalized analysis reopened all ten exchanges;
  selecting the original B request exposed the same 200/POST URL, request marker
  B, response marker B, fixture header and 87/140 byte sizes through native UIA.
  No final screenshot is archived because an unrelated foreground window
  overlapped that capture.

Native production locale switching also changed both page text and menu labels
from Korean to English. This is not a new 21-route or OS scaling matrix. Computer
Use rejected the separate audit app with “Computer Use was not approved to use
archscope-audit”; no further audit UI input was attempted. T-627 remains REVIEW.
No Windows area ratio, alternate OS DPI or exact CSS viewport pass is claimed.

## Legacy sample and remaining gates

The user supplied a genuine local `.halog`; it remains outside Git. Reference
executable version 7.6.4.508 loaded 675 entries through its installed COM API.
The native input's hash stayed unchanged after loading/probing. A Full XML export
was also saved privately. It fails XML parsing at line 16532, column 96; it is
not a validated field-mapping oracle. Only hashes, sizes, parser location and
entry count are archived in [halog-reference.json](halog-reference.json).
The generic probe still correctly returns UNVERIFIED; writer/version evidence
is supplied separately here. T-608 moves from PENDING to REVIEW because sample
absence is resolved. A native codec, field comparison and ArchScope-generated
file reopening in the reference tool remain unimplemented/unverified.

T-601 still needs verified WinDivert binaries/privilege plus a peer IPv4 port-80
fixture to prove proxy-ignoring/service reach. This host reported the transparent
backend unavailable; no fallback was presented as transparent capture.
T-611's supported native adapter/shortcut tests pass, but vendor dual-IID,
ITypeInfo/events/filter/retention and cross-process session-cookie semantics
remain open. No destructive WinINet cache operation was run. T-613 retains the
full feature/reference matrix and multi-hour desktop soak. The two-minute soak
uses the real service/proxy on Windows, not a continuously operated GUI.

Public evidence contains synthetic traffic and metadata only. Raw reference
logs/XML, user paths, controller tokens and private diagnostics are excluded.

## Post-push CI follow-up

CI exposed and resolved two portability issues after the initial local run:

- `cd01dff9`: copied Go module directories retain read-only modes on Unix.
  Cleanup now makes only directories in the owned temporary tree writable,
  without following symlinks. The Linux-hosted 386 cross-build then passed.
- `8ded194e`: the Windows runner's shortcut retained an 8.3 `RUNNER~1` alias.
  The test now compares file/directory identity with `os.SameFile`; Unicode,
  raw arguments, quoting and the child's working-directory checks stay strict.
  Both local architectures and Windows CI passed.

[Engine Native run 42848607855](https://github.com/aurakimjh/archscope/actions/runs/42848607855)
at `8ded194e` passed Windows Go vet/build/full race tests, live-capture E2E,
50/50 native resolver samples, protected registry rename, body-capture E2E,
forced-crash dump exclusion and policy restoration. Linux Go/race, frontend
and both GUI cross-build jobs also passed. The overall workflow is **cancelled**
because its two macOS jobs never started: GitHub reports
`The configuration 'macos-14-arm64-us-default' is not supported`.
This pre-existing runner configuration is outside the Windows test changes.

[Windows GUI Smoke 42848207160](https://github.com/aurakimjh/archscope/actions/runs/42848207160)
at `cd01dff9` passed native production startup on both amd64 and 386.
[General CI 42848607891](https://github.com/aurakimjh/archscope/actions/runs/42848607891)
at `8ded194e` passed. The later test change has no buildable product changes.
Exact step verdicts and resolver measurement (mean 3.119 ms, p95 3.879 ms;
measurement only, no product SLO) are in [ci-validation.json](ci-validation.json).
