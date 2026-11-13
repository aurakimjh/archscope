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
