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
