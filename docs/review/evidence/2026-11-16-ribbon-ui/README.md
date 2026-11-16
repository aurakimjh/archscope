# Native ribbon UI verification and scoped publication

Date: 2026-11-16. The user requested UI testing followed by commit and push.
These are local native WebView tests, not an independent implementation or
linguistic review. T-663/T-664 remain REVIEW.

The maintained `scripts/verify-ribbon-native.js` exercises actual React controls
inside the Wails WebView. The `layoutaudit` build configures native window size
and actual WebView zoom through `ARCHSCOPE_LAYOUT_VIEWPORT`. The ordinary product
build excludes these test settings and progress events.

## Matrix

| Native window request | WebView zoom | Purpose |
| --- | --- | --- |
| 1280 × 800 | 100% | Expanded command groups |
| 1024 × 768 | 125% | Compact groups and popups |
| 900 × 768 | 125% | Secondary tab selector and compact popups |

Each scenario covers English, Korean, Japanese, Simplified Chinese and Taiwan
Traditional Chinese, with all six ribbon command tabs per locale. This yields
90 locale/tab layout combinations. Tests require one tab row, a compact ribbon
height, no horizontal panel overflow, and all visible commands or opened group
popup controls within the WebView bounds. All 21 route commands must be present;
the Settings navigation command is executed. This does not claim analysis-file
import or execution of every route's domain workflow.

The smoke also checks saved locale values; light/dark/system theme resolution;
page identity across tabs, language changes, File and Back; ArrowRight/Home focus;
disabled capture commands outside HTTP; tabs-only mode, temporary commands,
Escape and outside-pointer closing; hide/restore and restore-button focus.
No capture is started, no private input is imported and test preferences are
restored. Each app uses an owned temporary state root and private controller.
Controller credentials are excluded from evidence.

## Finding and correction

`before-popup-1024.json` and `before-popup-900.json` preserve failures with the
English Chart Studio command outside the group popup bounds. The last group
used unconditional right alignment even when that group was near the left edge
of the window. Removing that special alignment keeps the group popup anchored
to its own left edge. The final matrix rechecks every group's popup in every
locale. Earlier receipts also show zoom remained at 1.0: setting it before the
native window existed did not apply it. The audit runner now applies and checks
the requested zoom after window initialization. Those earlier runs are not
reported as 125% tests.

All three final scenarios PASS:90 locale/tab layouts and9 theme selections.
The final WebView zoom readings are1.0/1.25/1.25, with CSS viewport widths
1280/819/720 (WebView integer rounding). See `local.json` and native receipts.

The first extended wide run produced no receipt before being stopped for the
progress-enabled harness; it is not a PASS. Test progress is now recorded to a
private `.progress.json` file. Two subsequent background runs reached the
visibility phase but had no final receipt by300seconds; they are retained as
INCONCLUSIVE in `incomplete-background-runs.json`. The final runner also waits
for temporary commands to appear before the outside-pointer action and saves
the renderer result before native geometry. Its three reruns completed. Final native receipts and `local.json` contain the
completed run results. `source-hashes.json` binds the final product, test and
guide files. It excludes `work_status.md`, whose unrelated pre-existing history
block is deliberately left unstaged.

## Validation limits

Computer-use access again returned `cgWindowNotFound`. No screenshot or manual
readability approval is claimed. DOM geometry, computed theme colors and native
renderer interactions are recorded separately from that limitation. WebView
125% zoom is not OS DPI scaling. Actual Windows WebView2/font/IME/native shortcut
and OS DPI checks, and independent language review remain open.

Frontend state/build, translation coverage, desktop-only guard, smoke syntax,
native menu tests, audit-tag vet and Windows amd64/386 production cross-builds
are checked alongside the matrix. The earlier app race run remains documented
in [the initial implementation receipt](../2026-11-16-ribbon-languages/README.md).
No parser/analyzer/exporter or common AnalysisResult contract changed.

Reproduce by building from `apps/engine-native`:

```sh
go build -tags 'production,layoutaudit' -o /tmp/archscope-ribbon-qa ./cmd/archscope-app
```

Run the native binary in an isolated temporary profile with
`ARCHSCOPE_LAYOUT_SCRIPT` pointing to the maintained smoke,
`ARCHSCOPE_LAYOUT_OUT` pointing to a private result,
`ARCHSCOPE_LAYOUT_MAX_SECONDS=300`, and, for example,
`ARCHSCOPE_LAYOUT_VIEWPORT='{"width":900,"height":768,"zoom":1.25}'`.
Compare the receipt's `webviewZoom` and renderer viewport with the requested
configuration before accepting the run. Package the Mac binary in a test app
bundle as appropriate for the host; use a different profile for each scenario.
