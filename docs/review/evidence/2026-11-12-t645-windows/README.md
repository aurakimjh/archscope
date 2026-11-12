# T-645 native Windows follow-up — 2026-11-12

Local Windows verdict: **PASS within the scope below**. T-645 remains **REVIEW**
pending independent review. This verifies importing the qualified PostgreSQL16.14
and Linux iproute2-6.1.0 captures on Windows; it does not qualify native Windows
capture producers or the other optional DB/OS adapters.

The tested binaries were built at `c83740a1ae4c2a0d921768f7d78804dc06cc8cfa`.
The complete `apps/engine-native` Git tree is identical at upstream T-645
`b36d83e0`, the tested build, and fix `6317b804`:
`cce8f67f4ba69d35a0a84a27e2f59b5ce2f91cfb`. Binary hashes, PE machine types,
commands, environment and limits are recorded in [validation.json](validation.json).

## Fixed Windows checkout failure

`python scripts/build-offline-assets.py --check` failed because Git checked out
`scripts/subset-nsight-sqlite.py` with CRLF (6632 bytes), while the embedded copy
retains LF (6553 bytes). The original Git blob and embedded copy already matched.
Commit `6317b804` adds `text eol=lf` for this source script. It changes no Python
logic, bundle bytes or source hashes. A fresh `core.autocrlf=true` checkout from
the committed index exactly matches the embedded file, and the resource check
passes. The before/after output is retained in `offline-assets*.log`.

## Verification

- Native amd64 and native 386: all seven affected Go packages pass, with **261
  passing test-completion events including subtests**, zero failures and four
  skips on each architecture. Both `go vet` runs pass. [go-tests.json](go-tests.json)
  lists every passing test, package timing and skips. The Windows production SDK
  wrapper is used for 386. Local runs are ordinary tests, without `-race`.
- Both CLI executables pass **61 commands each**, including auto/explicit source
  detection, normalization, no-overwrite, Unicode/space paths, direct and catalog
  import, exact query paging, stale cursor/limit refusal, JSON/CSV/HTML/report-pack
  export and merge. PostgreSQL retains 15 series/75 points and the sequence
  `first_cumulative_point → start_time_unavailable → derived → reset → derived`.
  Derived rates are independently checked as rational numbers against capture
  timestamps. The reset rate uses only its own post-reset epoch. Linux retains
  21 series/63 gauges, no deltas/rates, and established values `0,8,0`. Merge
  yields 36 series/138 points. [cli-validation.json](cli-validation.json)
- Both architectures save and export both results through a private case store
  with exact byte equality. These are explicitly snapshot-only cases, without
  original source intake or a portable case claim.
  [saved-case-validation.json](saved-case-validation.json)
- The manual subset tool successfully writes a Unicode-path SQLite subset and
  receipt: 32768-byte fixture to 24576-byte output, 19 retained events. Original
  bytes remain unchanged. Seven negative checks refuse source/output aliases,
  existing output/receipt, reversed/noncanonical times, SQLite sidecars and an
  unqualified producer. This uses the existing small fixture, not the separate
  padded large-input Linux exercise. [subset-validation.json](subset-validation.json)
- Frontend state tests and production build pass. Both production Windows GUI
  executables build. All eleven offline fixture importers pass.
- Actual amd64 Wails/WebView2 app: both explicit source formats inspect correctly
  (15/75 and 21/63); producer/version/query overrides are hidden for these formats.
  A saved result opens from a Unicode/space path. Metric query renders 63 gauge
  rows with exact values and source positions; Next advances to the second page.
  Accessibility captures and [the screenshot](ui-linux-ss-query.jpg) are retained.
  Native file-save dialog handling was not exercised here; Wails service tests
  cover source pinning, stale source/result refusal, normalization/no-overwrite
  and exact saved-result readback.
- Native 386 GUI remains alive for 15 seconds. The existing smoke script then
  requests close and forces its owned process after the eight-second timeout;
  graceful shutdown is **not** claimed. See `gui-386.log`.

Local skips are the opt-in body E2E, capture client helper, transparent direct
client E2E and POSIX owner-only permission assertion. Full race/live/body/crash
coverage is supplied by the source-equivalent upstream CI below, not claimed as
new local runs. Native UI coverage is one 1266×793 window, without a complete
conditional/DPI/long-duration audit.

## Hosted source validation

Upstream T-645 `b36d83e038dd42abbae0e488dc9083083763ad4c` passed all eight
[Engine Native jobs](https://github.com/aurakimjh/archscope/actions/runs/43218865978)
and both [general CI jobs](https://github.com/aurakimjh/archscope/actions/runs/43218865561).
The Windows amd64 job passed full race tests, live/body E2E, native registry
rename regression, forced-crash dump exclusion and dump-policy restoration.
Native 386 full tests, Linux/macOS full tests, both Windows GUI cross-builds,
frontend and macOS package smoke also passed. Exact job/step verdicts are in
[ci-source-baseline.json](ci-source-baseline.json).

Fix `6317b804` separately triggered
[Engine Native](https://github.com/aurakimjh/archscope/actions/runs/43224081144)
and [general CI](https://github.com/aurakimjh/archscope/actions/runs/43224081375).
At receipt time, general CI passed both jobs. Engine Native frontend, both GUI
cross-builds and macOS offline/package smoke passed; full platform test jobs
were still running. Those run states are tracked separately; the upstream
successes above must not be read as a new hosted verdict for the checkout-only
fix.
