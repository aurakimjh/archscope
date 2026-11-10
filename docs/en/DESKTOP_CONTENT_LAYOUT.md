# Desktop layout for reading analysis results at the default window size

**Proposed design, pending implementation.** At 1280×800, opening input or starting capture should immediately expose readable, selectable data. Allocate the width recovered from navigation and the height recovered from instructions to tables, graphs and the inspector.

## Current layout

`main.go` creates a 1280×800 window, registers a native HTTP Capture menu and sets a 50px invisible macOS title bar. Preserve window controls and dragging. `App.tsx` opens Profiler by default and places three theme buttons and two locale buttons above the page. The sidebar occupies 224px when expanded (17.5% of nominal width) and 56px when collapsed in `public/style.css`. Profiler and GC place file and option controls before results. HTTP already collapses its source disclosure after load and also exposes the request builder and live panel. T-620 measures the native baseline. The dimensions below are design targets.

## Navigation and commands

Use one native menu row on Windows and the system menu on macOS. Verify Linux menu exposure and use one equivalent in-window menu only where needed. Remove the default sidebar and theme/locale header. Keep the tool name, current input and frequent actions on one contextual row. Move secondary actions into overflow on narrow windows.

| Menu | Commands |
|---|---|
| File | Open in current view, recent files, save/export |
| Analyze | Profiling and comparison, logs and runtimes, traces and web |
| Capture | Open HTTP view, start, pause, stop, find, group controls |
| Workspace | Collection and evidence, visualization and correlation, reports and export |
| View | Settings, theme, language, panel visibility |
| Help | Current tool help and About (application menu on macOS) |

Route navigation through existing NavKeys and HTTP actions through the capture command router. Share command availability between menus and the toolbar. One accelerator press must dispatch once. In HTTP the reading order is menu, compact capture controls, process-grouped request grid, draggable splitter, selected headers/body/timing, then recording status and counts.

## Help and states

Reuse the existing `HelpTip` question mark next to the relevant label. Show definitions, formulas and supported formats on hover or keyboard focus. Click or Enter/Space pins the overlay. Escape or outside click dismisses it. Allow the pointer to enter the overlay and position it inside the viewport and outside clipped scroll containers. Supply a specific accessible name and description. Put multi-step or interactive guidance in a help panel that returns focus on close.

An empty view presents one next action such as opening a file or starting capture. Collapse the input dock to the file name and a replace action on load. Keep errors, recording state, omitted counts, partial evidence, units, permissions and consent visible beside the affected action or result. Expand explanations on demand.

## First visible content by feature

The table describes populated views. A selected result tab uses the same primary space. Empty workspaces expose the action to add a first item. Settings opens directly on editable values.

| View | Keep visible | Open on demand |
|---|---|---|
| Profiler (`profiler`) | Flamegraph or top frames, selected stack path | Source, format and analysis options |
| Profile diff (`diff`) | Regressed and improved frames with deltas | Replace inputs and matching options |
| GC log (`gc_log`) | Pause trend and GC event rows | Source, JVM environment and metric definitions |
| JFR (`jfr`) | Top methods and threads for the selected range | Import options and event type definitions |
| Access log (`access_log`) | Request, error and latency trends or URL ranking | Parser options and aggregation definitions |
| Exceptions (`exception`) | Events and the selected stack | Signature rules |
| Thread dump (`thread_dump`) | Deadlock status, contending threads and locks | Runtime-specific options |
| Jennifer MSA (`msa_profile`) | Transaction timing breakdown and selected call | CSV mapping and parser report |
| Trace import (`trace_import`) | Traces, spans and selected path | Format detection and aggregation notes |
| HTTP capture (`http_capture`) | Process-grouped requests and lower headers/body inspector | Request builder, import and compatibility probe |
| Browser CPU profile (`browser_cpu`) | Flamegraph and top frames | Profile import instructions |
| Lighthouse (`browser_audit`) | Core metrics and audit findings | Acquisition instructions and report metadata |
| Analysis workspace (`analysis_workspace`) | Saved results and current selection | Collection instructions and bulk actions |
| Export center (`export_center`) | Selected results, format, destination file/folder and export action | Format notes and secondary options |
| Report diff (`report_diff`) | Changed rows across the two reports | Replace inputs and comparison semantics |
| Chart studio (`chart_studio`) | Chart canvas for the selected data | Template and renderer selection, PNG/SVG/CSV export |
| Incident timeline (`incident_timeline`) | Ordered events and omitted count | Grouping notes and advanced filters |
| SLO and golden signals (`slo_golden_signals`) | Measured values, verdicts and unassessed targets | Target editing and formulas |
| Service flow (`service_flow`) | Service connections and selected edge metrics | Aliases and aggregation notes |
| Evidence board (`evidence_board`) | Pinned evidence and its source | Board instructions and export options |
| Settings (`settings`) | Values and controls in the selected group | Setting descriptions, recent file management and About |

## Acceptance

- Close menus and help, then measure after load and before scrolling at the default window size and 100% scale. Clip rows, plots and selected detail to the actual WebView and divide their union by its viewport area excluding OS decorations. Exclude group headers and virtual overscan rows and count intersections only once. Exclude navigation, controls, instructions and blank space. Target at least 70% for populated result views with sufficient data. Record the reason for excluding forms (Settings and Export), empty states and sparse results from the ratio gate. Check that the first action, required inputs and save, completion or error state fit in the default window instead.
- For tables with sufficient input, show ten full data rows. In HTTP show ten request rows in an expanded group plus four inspector lines simultaneously. In GC show the pause trend and events together. In Profiler show frames and the selected path together. Target at least 300 CSS pixels of plot height on chart-focused tabs. Preserve readable font sizes.
- Check 1024×768 at 125% scale and both Korean and English. Primary actions and help must remain keyboard-accessible. Allow interior table scrolling and keep commands and dismiss controls within the window.
- Pair before/after native screenshots and measurements using the same input, window, scale and tab. Exercise empty and populated/editable states on every route, and error, partial, selection and help states where applicable. On Windows verify arrival of new requests, stable selection and page re-entry during capture. Migrate the sidebar DOM selectors in `verify-windows-live-capture.ps1` to the new navigation path.

## Interaction and measurement details from the review

**From input to result (T-620, T-622).** On first launch, Profiler offers file open and drop on the same target. After selection show the filename and Analyze, while running show progress and supported cancellation, and on success show results and Replace. On cancellation or failure keep the selected file and a retry action. A collapsed profile comparison identifies both inputs. A multi-file thread dump exposes count and order. Open recent files on demand and allow reselection when a path no longer exists. When a filter hides all rows, show the active condition with a clear action.

**Space above the first result (T-622, T-625, T-626).** Fold sticky file headers and the vertical `AnalyzerOptionsDock` trigger into the same action row. Condense summaries such as the five Service Flow metric cards into short values with units above the table. Remove repeated headings and instructions and reserve question marks for terms that need interpretation. Keep critical findings and missing data beside the value. Check the first data offset with long Korean filenames and conditions. Shrink and enlarge the window and confirm table scrolling and chart resizing use the available space while selection and panel dismissal stay reachable.

**Menu dispatch (T-621).** Profiler currently passes empty open and settings callbacks to `useShortcuts`. Wire Ctrl/Cmd+O to the active file picker and Ctrl/Cmd+, to Settings. Disable unsupported commands. Carry forward HTTP Clear and Auto Scroll. Update native labels and checkmarks when the locale changes. Opening and closing a menu should keep the current page mounted with its selection, scroll position and draft inputs. Handle keyboard input in the open overlay before the active tool. Escape while viewing help during analysis should dismiss only help. On macOS replace the existing drag region and keep window controls and action targets distinct. [Menus and window](../../apps/engine-native/cmd/archscope-app/main.go) · [Profiler shortcuts](../../apps/engine-native/cmd/archscope-app/frontend/src/hooks/useShortcuts.ts)

**Help density (T-622).** Consolidate repeated help within a group under one question mark at its heading. Current `HelpTip` hover, focus and click handlers share one open flag. Exercise hover → click → move pointer → Escape and Tab → Enter → Escape. Pinned text must stay visible until dismissed and only one overlay may close per Escape. Start measurements with options and recent files closed. Show changed values or a change indicator in the compact row. [HelpTip](../../apps/engine-native/cmd/archscope-app/frontend/src/components/HelpTip.tsx) · [Options drawer](../../apps/engine-native/cmd/archscope-app/frontend/src/components/AnalyzerOptionsDock.tsx)

**Result tabs and split panes (T-623 through T-625).**
- The normal HTTP view uses `100dvh−230px`, a 420px minimum and a 320px default detail height. Divide the actual space below the new toolbar between the grid and inspector. Ten 26px requests alone require 260px, plus group and column headers. Measure default geometry on a fresh launch. Clamp a previously saved tall inspector on restart or resize so both panes remain usable. Preserve the user's splitter position within those limits, selected request and detail tab, and disabled auto-scroll. Check scope and body consent before capture, Stop during recording, and inspection after stop. [Request grid and splitter](../../apps/engine-native/cmd/archscope-app/frontend/src/components/HttpAnalyzerView.tsx)
- Profiler currently selects Summary when analysis completes. Show the flamegraph and selection path first when a frame tree exists, otherwise use top frames or the available summary. For shallow trees assess readability at their natural depth. In GC Summary expose the pause trend and an event preview, retaining the full Events, Heap and JVM tabs. For logs without a time axis show available events and the reason. Verify frame hit testing and row selection after resizing.
- Inventory conditional views as well as landing tabs. Examples include JFR recording, profile and native memory, single and multiple thread dumps, Jennifer MSA detail and the exception stack drawer. At narrow widths keep the selected tab visible and every tab reachable by keyboard or overflow. Summarize long URLs, stacks and filenames on one line and make the full value readable or copyable in detail.

**Reproduction and evidence (T-620, T-626, T-627).** For each view record the input or creation sequence, state and tab, first data offset in CSS pixels, full rows or plot height, then result area ratio or form verdict. Start Profiler with the [collapsed sample](../../examples/profiler/sample-wall.collapsed) and GC with the [GC sample](../../examples/gc-logs/sample-hotspot-gc.log). For row budgets use a separate sufficiently populated fixture held constant before and after. Follow a workspace result from registration through selection into charts, evidence and export. Keep destination file/folder selection, Settings save, format, units and provenance accessible. In Chart Studio change templates, resize and check both canvas and exported output.

Record OS, outer window size, measured WebView width and height in CSS pixels, OS scale, locale, fixture and fresh/saved layout settings. The Windows harness clicks `nav button.nav-item` in the WebView. Exercise the replacement native menu at the OS level and read back the active route and restored capture in the WebView. Record actual Windows recording evidence separately from Mac measurements. A view with a higher data ratio still fails if open, supported cancellation or selected detail is unreachable.

## Implementation sequence

- **T-620** Measure all 21 views at the default window size (depends on —).
- **T-621** Move global navigation to native menus (depends on T-620).
- **T-622** Compact shared input controls and help (depends on T-620).
- **T-623** Prioritize the HTTP grid and inspector (depends on T-621, T-622).
- **T-624** Prioritize profiler and GC results (depends on T-621, T-622).
- **T-625** Convert nine remaining analysis views (depends on T-621, T-622).
- **T-626** Convert eight workspace views and Settings (depends on T-621, T-622).
- **T-627** Verify all views in native desktop windows (depends on T-623–T-626).

## Implementation and evening verification, 2026-11-10

The T-621 through T-626 implementation is recorded in individual commits. Native
menus, shortcuts, compact inputs/options/help, HTTP split panes, GC Summary,
Profiler landing tabs, the remaining analyzers and workspace layouts are updated.
Drop files on the tool's file picker. Tools remount on navigation as before and
HTTP recording restores from the service. Opening a menu preserves the current page.

Native collection produced no ready report while the Mac was locked. The user
deferred interactive checks until the evening of 2026-11-10. T-620 baseline and
T-627 native acceptance remain pending, and T-621 through T-626 remain REVIEW.
The 70% result area and ten-row/four-line HTTP budgets are not yet measured.

1. Open `apps/engine-native/cmd/archscope-app/bin/archscope.app` and reach every
   menu route. Exercise Open, Recent Files, Settings, theme, locale and About.
2. Populate the viewport/scale/locale matrix with real results. Compare baseline
   commit `d98a958` in a separate checkout with current HEAD using identical
   fixtures. The default `measure.js` collects initial view geometry and does
   not populate results or automatically exercise every conditional tab.
3. Verify hover, pin, pointer transfer, Escape, drawer dismissal, viewport edges,
   long paths, empty/error states and clearing filters.
4. Exercise GC trends/events and profiler drilldown, then resize and check hit
   testing and selection against the displayed content.
5. Inspect a saved HTTP file and the grid/detail splitter. On Windows check live
   arrivals, process selection, disabled auto-scroll and return after navigation.

The live-capture PowerShell harness now exercises re-entry through the shell route
listener. Physical OS menu interaction needs separate verification. The retained
untracked local `verify-http-desktop.mjs` expects the old sidebar and must be
updated before it can validate the new UI.
