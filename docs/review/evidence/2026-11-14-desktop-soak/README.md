# Windows desktop soak follow-up (2026-11-14)

T-613 remains PENDING for the full original/vendor/driver parity contract.

## Isolated renderer ownership

Automation descriptor launches now keep WebView2 data beside their descriptor in
`.archscope-webview2`. Normal launches retain their existing default profile.
This prevents simultaneous test apps from sharing a browser process and makes
per-app process-tree memory observations interpretable. It does not change body
capture or user privacy settings.

The soak runner checks every retained row through pagination, requires zero drops,
and records root plus alive descendant private bytes and working set from a
single process snapshot. Descendant creation dates are checked against parents.
These samples are liveness/resource observations, not proof of absence of leaks.

Validation before the long run:

- Production app package tests passed on Windows amd64 (36.802s) and 386 (41.538s).
- Simultaneous 10-second production smokes passed on both architectures: 36 saved
  rows each, seven live processes each, 360,603,648 / 394,100,736 tree private bytes.
- Both apps remained alive for subsequent native observation.
- Earlier preliminary runs were deliberately stopped after discovering shared
  WebView ownership; they are not counted as two-hour passes.

Long-run and post-run native results will be appended after observation.

## Real 386 failure and Windows manifest-sharing fix

The isolated386 run failed after the5640.540s checkpoint (19514 completed request
attempts counted by the runner) with socket hang up; the desktop remained alive.
This is a FAIL, not a two-hour pass. The controller journal independently records
the preceding fatal error: atomic replacement of manifest.json returned Windows
Access is denied. Capture then stopped in recoverable state. Inspection retained
128 complete200 rows, sequence19385–19512; current generation captured886 versus
persisted885. Zero dropped counter does not erase that persistence failure.
The Docker fixture was still running, with zero restarts and no OOM indication.
The specific process holding the destination was not identified.

A real Windows reader handle denying delete sharing reproduces the same error
in386 before the fix. Capture-store atomic rename now retries only Windows
access/sharing/lock errors for at most one second, preserving the committed old
destination throughout. Other errors return immediately; permanent lock still
fails without replacing old bytes and leaves no pending staging file. No file
is deleted to make replacement succeed. The native reader-release, permanent
lock and store/session/proxy suites pass amd64/386. One pre-fix diagnostic run
was deliberately stopped after the failure journal established the cause; it
is not counted as a crash or a two-hour test. A fresh fixed-build long run is
required and will be recorded separately. The runner now also keeps bounded
private desktop logs and private failure journal/status, without retrying failed
traffic or converting a failed run into a pass.

## Original isolated amd64 two-hour PASS (before lock fix)

The original amd64 production build7a0e1b1e completes7200.717s and24864 requests,
119pause/resume cycles and11clears. Every retained snapshot is paginated and
validated, max128rows and0drops; final generation captured/persisted2040/2040.
Saved128rows reopen with exact selected-ID equality; file SHA is in the report.
The desktop remains alive at completion. The unchanged schema2 runner used is
attached. This successful run predates the Windows-sharing-lock fix and does
not substitute for the separately running fixed-build verification.

Native final readback shows128/128 saved rows and selected sequence24736. Raw
and Headers show200GET,Content-Type application/json,Cache-Control no-store and
Content-Length45. Body explicitly explains that both bodies were not recorded.
A Profiler menu roundtrip restores128rows and the same selected request after
asynchronous hydration; the detail tab resets to Raw. These final observations
use OS125%/WebView100%; the run began at OS100%. Early Headers/Body and mid-run
reentry observations are also attached. The mid amd64 reentry coincided with a
clear and displayed36fresh rows; no preservation of a pre-clear selection is
claimed. The mid386 reentry showed128rehydrated plus incoming rows.

The failed386 and deliberate pre-fix diagnostic attempts remain separate. Both
architectures now require completion of the fixed-build two-hour runs.
