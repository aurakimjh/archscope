# Native layout measurement

Build the frontend, then build the Wails application with
`-tags production,layoutaudit`. Use an isolated audit app bundle and set
`ARCHSCOPE_LAYOUT_OUT` to a private JSON output path. The production build
excludes the observer, QA menu and QA application name.

With `ARCHSCOPE_LAYOUT_SCRIPT` unset, the embedded passive observer records
geometry as a tester operates the native menus, file picker and controls. It
never navigates, clicks, focuses or enters input. The QA menu offers 1280 × 800
and 1024 × 768 window sizes plus 100% and 125% WebView zoom. WebView zoom is
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
for at most 30 minutes. Both modes use the pinned Wails runtime transport because
the npm-bundled SDK does not expose `window.wails`.

Copy the JSON before quitting or restarting the audit app: a new process starts
a fresh observation set and reuses the configured output file. An absent report
is an incomplete run. Build the original reference separately with the same
fixtures to establish before/after evidence. A current UI sweep cannot substitute
for the original baseline. Exercise populated states, conditional tabs and output
files before claiming full feature acceptance.
