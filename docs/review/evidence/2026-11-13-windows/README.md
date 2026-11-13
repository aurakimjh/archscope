# Native Windows follow-up - 2026-11-13

Pulled `f25ab80a` with `git pull --ff-only`. Two Windows failures were fixed
and committed separately: `d169bb62` pins bundle source/embedded bytes across
CRLF checkouts; `1bd8b850` makes Python JSON I/O explicitly UTF-8 on a Korean
Windows host. The Python notice builder now accepts an isolated `--out` path.
Shipped license resources were preserved.

## Verification

- Native amd64 and 386 full uncached Go suites each pass **131 packages and
  1,912 test-completion events**, zero failures, 15 skipped test events and four
  packages without tests. Exact commands, package timings and every skip are in
  [validation.json](validation.json). Both full vet runs pass.
- Frontend `npm ci`, state regressions and production build pass. Both CLI and
  production GUI architectures build; PE types and binary hashes are recorded.
- Eleven mandatory offline importer fixtures pass on each architecture. The
  desktop-only gate rejects server builds.
- Korean Windows Python3.10 with `PYTHONUTF8=0`/cp949 passes both resource checks.
  Isolated notice generation under a Korean/space path verifies 84 dependencies
  and every notice hash. [Encoding receipt](python-encoding.json).
- A fresh `core.autocrlf=true` index checkout preserves all six copied collectors
  and four input-assistance assets exactly. [Checkout receipt](checkout-parity.json).
- Both stamped production GUI executables survive the controller startup window
  and pass CONTROL-01, PROCESS-01, EXCHANGE-01 and SESSION-01, including real
  loopback HTTP103/302/201, PID/start-time and saved selection. This is **4/16**;
  the aggregate remains **UNVERIFIED**, with no assessment contradictions:
  [amd64](control-amd64.json), [386](control-386.json).
- The native-host common-engine soak passes 122.824seconds, 38 cycles and 9,880
  synthetic loopback requests with at most64 retained rows. [Soak](soak.json).
- Actual amd64 Wails UI reaches Analysis Workspace through the native menu,
  inspects the SQLite3.53.0 portable capture from a Korean/space path (4 series,
  12 points, exact CLI digest), and selects all four gauges. Open/Save dialogs
  appear, but the Computer Use helper cannot target their filename elements;
  **completed native file saving is not verified**. [UI receipt](native-ui.json).

## Scope and remaining work

The tested engine/frontend Git tree is unchanged between the pulled baseline
and script-fix commit `1bd8b850`; this is local native validation, not an
independent review or full native parity verdict. The full Go runs use CGO=0,
without local race instrumentation. Body capture and the transparent driver
remain opt-in; no dump policy, CA trust, system proxy or driver was changed.
Private controller tokens/capture paths and large logs remain under `.tmp/` and
are excluded from Git.

T-601/611/613/627 remain open: actual driver/service reach, vendor compatibility,
full feature/populated/conditional/DPI coverage and multi-hour native soak still
need their specific evidence. T-654 remains user-deferred TODO (two-GPU Linux
CUDA/NCCL environment required; this host has one GPU). T-662 remains TODO for
natural nonzero-swap/elevated-pressure Apple Silicon evidence. Nothing in this
follow-up closes those six tasks or upgrades an independent CONDITIONAL verdict.
