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


## Mac native verification and fixes (2026-11-10 evening)

Used the actual macOS menus and file picker of an isolated Wails 3.1.0 audit
app. No browser or Electron was launched. All 21 routes were reached through
native menus and filled the available width. Populated profiler, GC, access-log,
JFR, exception, thread-dump, Jennifer, Trace, browser CPU, Lighthouse and HAR
views were exercised. Profile/report comparison, workspace handoff, evidence
addition, HTML export, Settings save, locale/theme and About were also checked.

Native testing exposed and fixed the following issues:

- The shared flex child used intrinsic content width, leaving the profiler in
  half of the window. It now fills the window.
- HTTP session explanations initially expanded and pushed the request grid out
  of view. File tools, session tools and field filters now start collapsed, and
  recording actions share one row.
- Pinned help ignored Escape when macOS mouse clicks left focus elsewhere.
  Closed help also swallowed Escape. Help now dismisses independently of focus,
  and a detail drawer initially focuses its close button.
- The file drop group nested multiple buttons inside a button role. Browse,
  help and clear are now separate accessible controls.
- Access logs open URLs and Trace opens traces; Lighthouse metadata/categories
  and profiler progress logs start collapsed.
- Desktop progress logs wrote inside the signed app bundle. Their default is
  now the user cache `ArchScope/logs`, preserving explicit overrides.
- The measurement script assumed `window.wails`, which the bundled npm SDK does
  not expose. The earlier missing report was affected by this defect as well
  as the locked desktop; that earlier run remains incomplete evidence.

| State | Observed CSS viewport | First data y | Fully visible data rows | Inspector height |
|---|---|---:|---:|---:|
| HTTP 30-request HAR, default window | 1280×800 | 253 | 12 requests | 160 |
| HTTP, small window | 1025×769 | 253 | 11 requests | 160 |
| HTTP, small window with 125% WebView zoom | 820×615 | 251 | 5 requests | 160 |
| GC, sufficient-event fixture | 1280×800 | 210 | 10 events | — |
| Profiler, deep-stack fixture | 1280×800 | 254 | canvas | — |

The native size menu requested 1024×768; observed CSS dimensions differed by one
pixel after resize. WebView zoom is not Windows OS scaling. Headers/Body,
160→184→160 keyboard splitter adjustment, resized frame zoom/reset, and an
`OrderRepository` drilldown were checked. HAR host groups are pseudo-processes;
this is not actual PID attribution or Windows live-capture evidence.

See [final geometry](../review/evidence/2026-11-10-layout/mac-native-geometry.json)
and [verification cases](../review/evidence/2026-11-10-layout/mac-native-validation.json).
The observer records only geometry and counts while native controls are operated.
The visible row/canvas/detail union was 60.7% for HTTP, 64.5% for the profiler and
49.3% for GC. This conservative metric excludes findings and metric cards and
does not establish the overall 70% content target.

T-621 Mac native menu acceptance is complete. Remaining acceptance includes the
same-condition original `d98a958` baseline, exhaustive conditional-tab/error/help
edge cases, exact result-area adjudication, chart-template outputs, and Windows
live/menu/scaling tests. Computer Use did not approve the separate original-UI
comparison app, so that baseline could not be launched. T-620 remains PENDING;
T-622 through T-627 remain REVIEW with their unverified conditions explicit.


### Original baseline follow-up

The separate original-UI app became available on the detailed follow-up run.
T-620 is complete: all 21 routes were measured at native 1280×800, Korean,
100% WebView scale, with the same deep-profile, GC-pressure and 30-request HAR
fixtures. See [original geometry](../review/evidence/2026-11-10-layout/mac-original-baseline.json).
Original profiler first-data y=484, union 29.6%; GC Events shows six full rows
at y=410; HTTP shows no complete request rows. Original HTTP source picking
scrolls the page, so its observations are marked after-load/scroll states.
The earlier denied-app note above is historical and no longer blocks T-620.

### Detailed Mac acceptance completed (2026-11-11)

This follow-up supersedes the earlier incomplete-Mac and denied-baseline notes.
T-620, T-621, T-622, T-624, T-625 and T-626 are DONE. The Mac portion of
T-623 and T-627 is complete; those tasks remain REVIEW for Windows live
recording/process/reentry and native menus/OS scaling respectively.

The production application uses the macOS menu bar: ArchScope, File, Edit,
Analyze, Workspace, Capture, View and Help (localized in Korean). The old
sidebar screenshot belongs to original reference d98a958. `Layout QA` and its
size/zoom commands exist only in the audit build; they are absent from the
production app. Native production HAR import and inspector were checked after
packaging. No browser or Electron UI was used.

| Populated final state | Observed CSS viewport | First data y | Visible data union | Additional result |
|---|---:|---:|---:|---|
| Deep profiler | 1280×800 | 172 | 71.2% | Frame zoom/reset and matching/empty drilldown |
| GC 60-event stress input | 1280×800 | 147 | 72.5% | 26 full preview rows; full Events retained |
| HTTP selected Headers | 1281×801 | 175 | 70.7% | 15 full requests plus six header rows; 184px inspector |
| Chart Studio | 1281×801 | 184 | 72.3% | Both renderers and all eight templates |

The original profiler measured y=484 and 29.6% with the same deep profile; the
same 30-request HAR showed no full request rows in the recorded original
after-load/scroll state. Original GC had six full Events rows with the shared
GC-pressure input. The final GC density row above uses a separate 60-event
stress input, so its row count is not a controlled before/after comparison.
Ratios count the clipped union once, include plot axes, and exclude controls,
metric cards and blank space; HTTP acceptance uses selected populated detail.
Sparse analyzer/workspace fixtures do not establish a universal 70% verdict.
Settings and Export are forms: required inputs, primary actions, cancellation,
completion/error and recovery were checked instead. WebView zoom at 125% is
recorded as WebView zoom, not OS DPI evidence.

Detailed fixes include the completion-before-task-ID race, long-path flex and
drawer-focus bugs, global HAR sequence numbers, unavailable byte counts,
native chart saves across PNG/SVG/CSV, and editable Settings drafts validated
on Save. GC preview counts expose 32/60 and the full Events tab exposes the
retained 50/60; the preview has five compact columns, while Events keeps all
15 columns. Diagnostic thresholds and the common AnalysisResult contract are
unchanged. Native file outputs were decoded/parsed, not merely named.

See [final validation](../review/evidence/2026-11-10-layout/mac-detail-validation.json),
[final geometry](../review/evidence/2026-11-10-layout/mac-detail-geometry.json),
[historical sessions](../review/evidence/2026-11-10-layout/mac-detail-history.json),
and [output manifest](../review/evidence/2026-11-10-layout/mac-detail-exports.json).
Per-task T622–T626 JSON records contain conditional-tab and recovery cases.
The fixture generator and measurement instructions are in
[scripts/acceptance/layout](../../scripts/acceptance/layout/README.md).

Go test/vet, tagged audit tests, frontend state/build, native macOS package and
strict signature checks, desktop-only/server-build refusal and Windows x86/x64
GUI cross-builds pass. Cross-builds do not prove Windows runtime behavior.
Windows capture reach/automation/parity and genuine legacy `.halog` reference
compatibility remain external gates in work_status.md.

### Windows runtime follow-up (2026-11-11)

T-623 is now DONE. The production Wails/WebView2 application showed two real
PID/start-time groups and ten persisted synthetic exchanges. Process filtering,
Headers/Body tab and request selection during arrivals, disabled follow, native
Analyze/Capture menu navigation, same-session reentry, F2 stop and finalized
Headers inspection passed. Page-local selection/filter/follow reset on remount;
the running session and its rows restore. Bodies were not retained in this run.

Both x86/x64 builds now survive native startup and pass authenticated automation.
Native x86 execution found a POINT callback ABI error and Win32 Ptr API imports
in Wails 3.1.0; the Windows build wrapper applies source-hash/version-guarded
fixes in a temporary SDK copy. The COM smoke uses explicit late-bound property
reads because PowerShell cannot infer properties without ITypeInfo. Unicode
shortcut execution and both architecture COM clients pass.

T-627 still requires the complete Windows view/menu/OS DPI matrix. Computer Use
denied access to the separate audit application, so no new Windows result-area
ratio, exact CSS viewport or alternate OS-scale verdict is claimed. Production
menu locale and Analyze/Capture routing were observed. T-601 transparent/service
reach, the remaining vendor automation contract and full parity remain open.
T-608 native `.halog` codec/reference interoperability is complete for the
observed V7.6.4 format. See the [codec evidence](../review/evidence/2026-11-11-windows/HALOG_CODEC.md)
for retained-text, encoding and timezone limits.

### Native Windows menu follow-up (2026-11-12)

The actual production Wails/WebView2 window reached all 21 routes through native
menus in empty/form states and synchronized Korean/English page/menu labels.
The current controller fixture was reopened through supported `--open`; selected
POST Headers, Body-off disclosure and two saved rows after native menu reentry
passed. This is sparse two-row evidence, not a 70% density verdict. The HAR file
picker opened, but Computer Use could not address its input field; file selection
and Analyze were not completed. Mixed Korean MSA Timeline labels in English mode
and raw dialog-cancel error text remain follow-ups.

T-627 remains REVIEW for populated/conditional/error coverage, exact CSS viewport
and area, small-window and alternate OS DPI acceptance. Both native Windows full
Go suites/vets, production builds, frontend checks and a 603.731-second service/
proxy soak passed; this does not establish multi-hour native GUI parity.
See [Windows evidence](../review/evidence/2026-11-12-windows/README.md).


### Populated native follow-up (2026-11-14)

Selected Windows native MSA summary/composition/topology/timeline/treemap/hotspot
and options views now follow live English/Korean switching. A coherent two-GUID
fixture independently matches average root1100=SQL200+call-duration gap50+method850ms.
The gap is caller duration minus callee response, not wire latency. API table
headings distinguish those measurements and units. A direct root-to-service
call now renders in the treemap instead of incorrectly reporting no calls. Small
1024x768 at WebView125% retains values and internal table scrolling; this is
OS100%, not a new actual OS125% case.

Native JFR heatmap/stack/memory states and JVM carrier/native/SMR/histogram views
have selected populated evidence. Saved visuals and Report Packs were created,
reopened, regenerated, upgraded, restarted and reimported with pinned integrity
checks. Actual PowerPoint image paste retains exact2400x1600 pixels. Native table
paste exposed provenance overlap; clipboard transport now keeps title,data and
source in one editable table, verified by before/after PPTX and screenshots.
Historical saved visual bytes are unchanged. Supported amd64/386 app tests pass.

These are selected cases. The complete original-reference, conditional/error,
locale and OS-DPI matrix remains open; T-627 stays REVIEW. Detailed inputs,
failures, fixes, exports and exact hashes are in the
[native follow-up receipt](../review/evidence/2026-11-14-native-followup/README.md).


Actual OS125% follow-up subsequently became available through Control Panel >
Appearance > Taskbar > Windows Settings. Passive Wails1.25 and physical3840x2160/
logical3072x1728 distinguish it from WebView zoom. Selected MSA/JFR/JVM and visual
actions were observed; dark timeline/topology labels and long HTTP persistence
errors were corrected with native before/after evidence. JFR heatmap selection
retains25/48events; native-memory all-allocation labels use12KB, and JVM SMR2/
histogram2 remain populated. Windows100%,WebView100%,Korean/System were restored,
confirmed by scale1.0/equal physical/logical app receipt. T-627 remains REVIEW.


Native Word/Excel/PowerPoint16.0.10417.20228 table paste was additionally checked.
Word's fixed900px table clipped the default portrait text area; clipboard-only
relative100% width now fits with exact rows/units/provenance. PowerPoint retains
one11-row table inside the slide. Excel preserves strings and zero formulas,
but keeps narrow default column widths; manual widening is recorded separately.
Saved visual PNG/SVG/HTML/TSV/view/spec bytes remain unchanged. Other Office
versions/SVG insertion and the complete T-627 matrix remain open.


Installed Word/Excel/PowerPoint16.0.10417.20228 also accept one selected saved SVG
through native Insert>Picture. Office normalizes the14MB source to4120B,retains
2400x1600 anddata/provenance text,removes the embedded font,and trims one trailing
space on empty Annotation. SVG-byte/font equivalence is not claimed; PNG remains
the primary paste format. Word fit is contained but sparse text is small at100%.
Other versions/full typography and format combinations remain open.
