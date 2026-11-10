# Native layout measurement

Build the Wails application with `-tags production,layoutaudit` after building
frontend assets. Set `ARCHSCOPE_LAYOUT_SCRIPT` to the absolute path of `measure.js`
and `ARCHSCOPE_LAYOUT_OUT` to a private output file, then launch that native binary.
The application runs the script in its own WebView and exits after the report or
three minutes. The production build excludes this hook. Use sanitized input and
an isolated audit app bundle. An absent report is an incomplete run.

The initial sweep records empty or previously loaded state on all routes. It
records viewport geometry and fully visible rows. Populate the per-feature
fixtures and inspect each result tab before treating the full acceptance matrix
as complete. The design specifies the required result and form measurements.
