# Local Windows validation — 2026-11-09

Product baseline: `a9896c392d7117ca1dcfb28753d176fc9f917f78` plus the
Windows test/build fixes committed with this report. This is local regression
evidence, not a new release acceptance or complete Stand-alone parity verdict.

## Repository synchronization

- Active repository: ArchScope; branch `main`, upstream `origin/main` at
  `https://github.com/aurakimjh/archscope.git`.
- Initial HEAD/upstream cache: `d348bcb`. Fetch found 12 newer commits.
- `git pull --ff-only origin main` fast-forwarded to `a9896c3`.
- No tracked local edits, unmerged paths, or merge/rebase/cherry-pick state
  were present. The pre-existing untracked `.tmp/` tree was preserved and is
  excluded from the commit.

## Local execution results

Host: Windows amd64, version 26H2, build 26300.9711; Go 1.26.3;
Node 22.15.1. Commands below ran in `apps/engine-native` unless noted.

| Check | Result |
|---|---|
| `go test ./... -count=1` | PASS: 77 test packages; 5 additional packages have no tests |
| `go vet ./...` | PASS |
| `go build ./cmd/archscope-engine ./cmd/archscope-app` | PASS |
| `TestWindowsLiveCaptureE2E` | PASS: actual loopback HTTP proxy, confirmed native process attribution, persisted/finalized exchange |
| `TestWindowsCaptureProcessSelectionE2E` | PASS: two real child processes from the native process snapshot; selecting one stores only its row, selecting both stores both rows; exact PID/start-time attribution and observed/persisted/dropped totals verified |
| Existing capture service controls and active-detail tests | PASS in the full suite and targeted native rerun |
| `npm run test:state` (frontend directory) | PASS |
| `npm run build` (frontend directory) | PASS |
| `task windows:build ARCH=amd64` (app directory) | PASS using the project's Wails CLI `v3.0.0-alpha2.117`; generated bindings match the baseline |
| `scripts/windows-gui-smoke.ps1 -ExePath apps/engine-native/cmd/archscope-app/bin/archscope.exe -StartupSeconds 15` (repository root) | PASS: 19,059,200-byte executable stayed alive for 15 seconds; the script requested graceful shutdown, then used its forced-shutdown fallback |
| Native WebView2 inspection | App opened and rendered the HTTP Capture idle/source screen; actual screenshot archived below |
| Native resolver-cost measurement | PASS: 50/50 confirmed samples, mean 3.314 ms, p95 6.116 ms, maximum 7.411 ms; measurement only, no predeclared latency SLO |
| Local `go test ./... -race -count=1` | NOT RUN: Go reports that race requires CGO; this host has `CGO_ENABLED=0` and no discovered GCC/Clang toolchain |

The resolver measurement used `ARCHSCOPE_RUN_RESOLVER_PERF=1` and
`ARCHSCOPE_RESOLVER_PERF_OUT`, running
`go test -count=1 -v ./internal/capture/procmap -run '^TestWindowsResolverCostMeasurement$'`.
Its path-free output is [resolver-cost.json](resolver-cost.json).

The first sandboxed Go run failed because the sandbox denied replacement of
existing manifest files and loopback connections. The same targeted tests and
then the entire suite passed outside that sandbox on this Windows host. These
failures do not establish an application defect; no storage or socket security
checks were weakened to make them pass.

## Fixes and build hygiene

- Windows `.syso` cleanup now uses `powershell -NoProfile` and removes only
  the generated architecture-specific file. The original command loaded the
  user's PowerShell profile and emitted an `AuthorizationManager` error.
  The rebuilt task completed successfully with the corrected command.
- GUI startup smoke launches its temporary test process with a hidden window.
  Native interactive inspection separately used a visible repository build.
- Added a Windows-only two-client selection integration regression. It uses
  real process enumeration and TCP-owner attribution without resolver mocks,
  system proxy changes, or certificate installation.
- The preinstalled Wails CLI was `v3.0.0-alpha.87`. The pinned CLI was installed
  into the task's `.tmp/` tools directory and used for the successful rebuild.
  Incidental generated-binding and npm lockfile changes were restored; neither
  dependency versions nor generated API contents were changed.

## T-603 disposition: incomplete

T-603 remains `TODO`. The native app's source panel correctly reports that
the existing Windows crash-dump policy prevents body capture. No CA was
installed, no system proxy was changed, and the machine dump policy was not
relaxed. The safety acknowledgment was left for the user to operate directly.

The native running-session screenshots, selecting a process/request in the
live UI, Headers/Body inspection while a new row arrives, native recording
commands, and stop/reopen detail sequence are **not certified** by this run.
The automated two-client test proves the native service/process boundary; it
does not substitute for those WebView2 interactions. In particular, an actual
captured Body value cannot be certified while this machine's body gate is
unavailable.

The app requests a 1280×800 window. Computer Use returned a 1266×793 bounded
native-window screenshot; it has not been resized or described as an exact
1280×800 acceptance image:

![Native Windows capture preflight](native-capture-preflight.jpg)

T-601/T-602 and the dependent T-613 parity work remain open according to the
existing registry. This run does not implement their missing capture modes,
exchange contracts, or remaining desktop features.
