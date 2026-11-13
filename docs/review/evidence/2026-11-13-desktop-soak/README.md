# Native Windows desktop soak and recovery

T-613 remains **PENDING** for the full original/vendor/driver parity contract.
This selected run uses real production Wails processes and their authenticated
loopback controller, with synthetic traffic to an owned Docker origin. Bodies
are disabled under the current Windows policy. No CA, driver or system-policy
changes were made.

The two-hour runs use product649aa15f (same product as8c66290b documentation
baseline), separately from the latest frontend fixes. The actual executable hash
and stamped commit are recorded per architecture. `soak-runner-used.mjs` is the
exact earlier runner used for those runs; the reusable script in scripts now
seeds4 final requests and keeps the owned child referenced for final UI review.
A short latest-amd64 smoke validates that updated runner; it does not replace
the actual two-hour observations.

Each run keeps128 backend rows and the default native live display can grow to
500rows. At49–50minutes, native menus navigated away and back while capture
remained running; amd64 rehydrated128 rows, and386 reentry coincided with the
scheduled50-minute clear and showed48 fresh rows. A selected row aged out or was
cleared, so selection preservation is not claimed. Headers and explicit disabled
request/response-body messages were inspected. Late native screenshots around
113minutes still show running capture, bounded500 display and128 retention.

The separate latest-product390c47ca crash tests force-terminate only the owned
process after12 synthetic requests. Restarting the same isolated store produces
a native recovery summary:1session,12records,0B discarded tail. Both architectures
then save/open12 rows and check selected-ID equality. `recovery-runner-used.mjs`
waits for the tester's observed flag before save/open; it is an archived test
receipt, not an automatic UI verdict. Preliminary readback screenshots are a
separate earlier12-record run of the same executables. The final repeated runs'
exact binary/result hashes are in recovery-amd64.json and recovery-386.json.
No power-loss or corrupted-tail fault is claimed.

Memory checkpoints cover only the root Go/Wails process, not the full WebView
process tree or a formal leak proof. Controller identity/origin checks sample up
to100 retained rows per checkpoint. Snapshot counters reset at scheduled clears;
generatedRequests is cumulative. No full vendor-equivalence or driver/backend
acceptance follows from these synthetic explicit-proxy cases. CGO/race tests are
unavailable on this host and are not claimed.

The public reports contain no controller authentication tokens or private session
IDs. Capture stores and saved .httpsession files stay in the private .tmp output.
The fixture container is loopback-published, using existing observer image
sha256:6f277eab1780718568b81482da41897a9444d912c69d82371166083c17f68e39.
Only owned test processes/container are cleaned up; existing Kubernetes/user
containers remain untouched.


## Final timed observations

| Architecture | Timed seconds | Generated requests | Dropped | Retained maximum | Pause/resume | Clears | Terminal save |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| amd64 |7200.294|25388|0|128|119|11|128 rows, save/open passes |
|386|7201.643|25344|0|128|120|12|Empty after final clear; original harness fails on null.length |

All241/242 checkpoints pass the configured root-process responding/window,
retention and private-memory guards. Root private memory start/end/peak is
98.64/78.54/98.64MiB on amd64 and89.31/63.27/89.31MiB on386. The old runners released
their children after writing results; no native windows were exposed afterwards.
The final screenshot observations around113minutes are not post-two-hour UI
observations.

The386 failure is retained verbatim in [soak-386.json](soak-386.json), not relabeled
as a successful end-to-end run. Its save/open commands returned an empty session,
then the verifier dereferenced transactions:null. The reusable runner now seeds4
requests after the timer, before stop/save, and keeps the child referenced.
An actual bounded386 recheck saves/opens40rows. A deterministic final-clear
fixture then clears immediately before that same seed and saves/opens exactly4
rows, with the selected request visible in the native UI. These are10-second
rechecks, not another two-hour run; see
[terminal-boundary-rechecks.json](terminal-boundary-rechecks.json).

Latest-product crash/restart recovery passes12records on both architectures with
native recovery summaries and selected-ID save/open equality. The completed
owned test desktops, QA app, Windows Settings/Control Panel windows and loopback
origin were stopped/closed. Existing user/Kubernetes containers were untouched.
Korean/System and Windows display1 scale100% were restored. Overall T-613 remains
PENDING for the disclosed full parity/backend acceptance scope.
