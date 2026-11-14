# Native layout measurement

Build the frontend, then build the Wails application with
`-tags production,layoutaudit`. Use an isolated audit app bundle and set
`ARCHSCOPE_LAYOUT_OUT` to a private JSON output path. The production build
excludes the observer, QA menu and QA application name.

With `ARCHSCOPE_LAYOUT_SCRIPT` unset, the embedded passive observer records
geometry as a tester operates the native menus, file picker and controls. It
never navigates, clicks, focuses or enters input. The QA menu offers 1280 × 800
and 1024 × 768 window sizes, a 1280 × 1000 file-dialog size, plus 100%
and 125% WebView zoom. WebView zoom is
not an OS DPI test. The observer clips rows/canvases/details to scroll ancestors,
counts HTTP request rows separately from header tables, and records no cell
contents or file paths. Its area ratio is a conservative union of visible data
rows, canvases (including axes), and the HTTP detail panel; it excludes findings
and metric cards and is not an automatic 70% acceptance verdict. Small fixture
results must be identified as sparse. Native resize can produce a one-CSS-pixel
rounding difference; record observed dimensions rather than relabeling them.

With `ARCHSCOPE_LAYOUT_SCRIPT` set to the absolute path of `measure.js`, the
legacy initial-route sweep runs and exits after its report or three minutes.
That script dispatches route events or sidebar clicks inside the native WebView;
it does not establish actual OS menu interaction. The passive mode stays open
for 30 minutes by default; an explicit `ARCHSCOPE_LAYOUT_MAX_SECONDS` from 30
through 21600 permits a longer owned audit session. Both modes use the pinned Wails runtime transport because
the npm-bundled SDK does not expose `window.wails`.

Copy the JSON before quitting or restarting the audit app: a new process starts
a fresh observation set and reuses the configured output file. An absent report
is an incomplete run. Build the original reference separately with the same
fixtures to establish before/after evidence. A current UI sweep cannot substitute
for the original baseline. Exercise populated states, conditional tabs and output
files before claiming full feature acceptance.

## Detailed follow-up inputs

Run `python3 scripts/acceptance/layout/create_fixtures.py --out /private/tmp/archscope-native-fixtures`
from the repository root. It references sanitized repository/shared examples and
creates project-specific long Korean paths, 60 GC events, no-clock GC, tiny
profile, paired thread dumps, native-memory JFR JSON and a 30-request HAR. It
does not change shared assets or create production evidence.

The observer also records horizontal overflow, selected-tab visibility, help
popup containment and SVG chart surfaces. Original and intermediate sessions
remain separate from final geometry. Use a selected populated inspector when
adjudicating HTTP detail area; a blank detail box is not result evidence. The
HTTP default is 184px, restored by Home or double-click; saved user heights are
preserved within the available viewport.

The receipt records the actual Wails screen scale factor and logical/physical
screen bounds separately from WebView DPR. Change OS scaling only through the
normal Windows Display settings, record the selected display, and restore its
original scale afterwards. A receipt can contain earlier observations from the
same process at different scales; use each view DPR and the separately saved
phase receipt, not only the latest screen metadata.

## Production Windows desktop soak

Use the production executable (without layoutaudit), an isolated loopback-only
synthetic HTTP origin and a new output directory. `soak-origin.py` serves the
fixture on container port8000; publish it only on127.0.0.1. The script never
changes Windows body-capture or network policy.

```powershell
node scripts/verify-windows-desktop-soak.mjs --exe .tmp/archscope.exe --out .tmp/native-soak-new --origin http://127.0.0.1:12017/ --duration 7200
```

The runner attaches only its own PID/start identity, disables bodies, uses128
retained rows, pauses/resumes each minute and clears every10minutes. It records
controller counters and root-process memory/liveness every30seconds. That memory
is not total WebView memory. Confirm UI correctness separately through the native
app. Private controller/session files contain authentication/session identity;
never commit them. `progress.json` and `result.json` omit these private IDs.

After the duration, the current runner seeds4 final requests to guarantee a
nonempty save/open check even if the last clear occurs at the boundary. The
completed result is written while the owned app remains alive for observation;
the terminal command exits after that app is stopped. Verify its executable path
and private descriptor PID, then stop only that owned process and the owned
fixture container. Never clean up unrelated desktop apps or Docker containers.

Schema4 validates running state and zero drops at each checkpoint, allowing
captured/persisted counters up to5seconds to converge between bounded controller
calls. Stop must return finalized; the subsequent authoritative stats must also
be finalized with equal counters and zero drops before any save/open can pass.
The runner imports `scripts/lib/desktop-soak-checks.mjs`; preserve that module's
exact source/hash with future archived runner receipts. Host-side failure
regressions run with `node --test scripts/test-desktop-soak-checks.mjs`.
Earlier archived runners and their native evidence remain unchanged.

The archived2026-11-13 two-hour runner predates the4-request final seed and
referenced-child change. Its actual source is preserved with the evidence. Empty
boundary saves, if any, must be disclosed and supplemented by a separate
nonempty round-trip; never present empty readback as populated coverage.

## P2 private store/export readback

Use the maintained verifier for the four-request nested-query run or the
130-second schema4 smoke. Run on the host owning the isolated private store:

```powershell
python scripts/acceptance/layout/verify_p2_windows_readback.py --run-dir .tmp/p2-windows-20261114/query-amd64 --kind query --arch amd64 --report .tmp/query-amd64-readback-v2.json
python scripts/acceptance/layout/verify_p2_windows_readback.py --run-dir .tmp/p2-windows-20261114/soak-386 --kind soak --arch 386 --report .tmp/soak-386-readback-v2.json
```

This reads existing private files without running the app or altering its store.
The schema2 report uses `allTransactionFieldsMatchDeclaredPolicy` and
`annotationsMatch`, with a versioned comparison policy and no excluded fields.
Expected host/query derive from the source retained URL; the exported values
must match, rather than being silently overwritten during comparison.
Live-session `annotations.json` is authoritative by transaction ID, replacing
inline starred/comment even when the sidecar entry is absent. Missing annotation
fields mean false/empty, matching Go zero values/omitempty. Invalid types, changed
star/comment and notes transferred to another ID fail. Unknown transaction fields
are compared too. Export-request annotation overrides and arbitrary additional
sanitization transformations are outside this fixture's declared scope.

The report contains counts, hashes and policy, without private IDs or comment
values. Failures publish no success report; an existing report is never
overwritten. Archive this maintained script's exact bytes with new native
receipts. Older `readback-verifier-used.py` files are immutable historical
snapshots with their original exclusions; their booleans do not gain annotation
coverage from this fix. Re-running the new checker against those private stores
is separate evidence, not a retrospective change to the old receipt.

Portable synthetic regressions (no Windows UI execution):

```sh
python3 -B -m unittest discover -s scripts/acceptance/layout -p test_p2_windows_readback.py -v
```
