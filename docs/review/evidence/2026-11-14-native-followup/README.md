# Native follow-up (2026-11-14)

T-627 remains REVIEW. These are selected populated-state observations, not the
complete original-reference / conditional / error / OS-DPI matrix.

## MSA summary and average scope

The native production bridge imported `msa-two-guids.txt`, derived from the
existing Jennifer `sample_two_tx.txt`: two GUID groups with root times 84/168ms,
four profiles, one signature. Analyze showed four detected/declared profiles,
two matched edges, zero incomplete profiles and zero excluded average groups.

The populated Summary cards and table labels, Details/validation headings,
parser status labels and scope controls now use the EN/KO message catalog (150
keys). Actual native Korean and English Summary screenshots are attached. This
is scoped localization: several explanatory paragraphs and child-chart labels
still need conversion, so full MSA locale acceptance remains open.

Native Average transaction mode showed two Go drilldown samples, root average
126ms and network gap average 47ms. The native Save dialogs produced the attached
JSON and HTML through Go projection. JSON readback independently asserts
`sample_count=2`, `root_response_time_ms=126`, `network_call_ms=47`. The fixture
retains the original sample's categorised-time-over-root warning; this is not a
realistic critical-path benchmark.

Frontend `test:state` and production build passed. Production Windows amd64 and
386 builds and app tests passed while these changes were present. Window was
1280x1000, Windows scale100%, WebView zoom100% for these observations. Exact file
hashes are in `artifact-sha256.json`.

## JFR heatmap and memory result semantics

Native Browse/Analyze loaded a synthetic48-event JFR-print JSON with eight event
types. All-mode showed48 events,18 stack samples and one unique stack. The populated
48-bucket heatmap selected00:00:24 into From time; Analyze produced24/48 events and
nine stack samples. Reset then Analyze restored48/48 and18 samples. The stack
flamegraph and populated method/package/thread tables rendered.

The native-memory fixture pairs all six allocations with frees. Default leak-only
correctly reports zero bytes, but previously the empty canvas incorrectly asked to
run analysis. Disabling leak-only previously labelled all six selected allocations
as unfreed, and searching Worker displayed12,288 samples despite byte weights.
Screenshots preserve these failures. The fix keeps computed values unchanged,
labels the all-allocation selection accurately, passes Bytes as flamegraph weight
unit, and shows analysis-complete text for an empty selection. The same real native
sequence now displays12,288 Bytes /100% /1 frame, selected events6, selected bytes
12.0KB, and an allocation-stack title. State regressions and production frontend
build passed; production+layoutaudit Windows build passed. Binary JFR conversion
and full event-mode parity are outside these JSON/UI observations.

## JVM metadata integration defect

The actual multi-dump/JVM screen showed carrier pinning1 and native method1, but
SMR0 and histogram0 despite populated input. The native Go parser emits
`[]map[string]any`; the analyzer accepted only JSON-decoded `[]any`. Normalizing
both supported row representations restores tagged/unmatched-address SMR rows
and class histogram entries without a JSON round trip.

A new real-jstack parser-to-analyzer integration test covers native and JSON
bundles, both SMR forms with deduplication, and histogram bytes/order. Both
architectures passed multithread and javajstack package tests (amd641.088s/0.629s,
3861.100s/0.644s). The production+layoutaudit native build passed. In the native
fixed app, `jvm-signals-jstack.txt` displays pinning1, native1, SMR2 and histogram2.
The four JVM subtabs were observed: populated carrier/native before the fix,
SMR and histogram before/after. Histogram is100 String instances/2400 bytes and
10 Order instances/1600 bytes, descending by bytes. The fixed fixture adds one
untagged SMR address to the original before fixture to cover both diagnostic forms.
No Java process or driver was required for this synthetic file analysis.

## Report Pack and visual artifact lifecycle

A native JVM Workspace result was added as an evidence card. Board Report Pack
HTML and ZIP downloaded through the real WebView2 UI. The attachedZIP has eight
CRC-valid entries, one card and one source result, retaining histogram count2.

A saved unknown access_log result correctly refused visual rendering and refused
inclusion as a backend JSON appendix without producing a report. No fallback
placeholder was exported. A supported built-in `profile-set.json` then analyzed
with explicit worker-a/worker-b and duration selection. The result contains eight
summary rows, wall-time0.35seconds and unavailable sample count. Added to Workspace,
its summary rendered in the actual desktop render window at2400x1600, five rows
per image: two pages. Page2 preserves unavailable total_samples and0.35seconds.

Native Save dialogs exported the complete visual bundle. PNG dimensions, ZIP CRC,
all member hashes, spec/query/source SHA, full eight-row TSV/HTML and availability
metadata were independently parsed. Original bundle13,702,038bytes; all outputs
are exact-byte hashed. Two PNGs and compact provenance are attached; full bundles
with per-SVG embedded fonts remain in the ignored local acceptance directory.
`visual-integrity.json` records every archive/member SHA, including full archives.

Board opened the historical image. Regenerate pinned revision created a separate
revision with both PNGs, complete tables and view byte-identical to the original.
Explicit upgrade created a third revision while retaining the first two (the
render dependencies were unchanged; version-mismatch upgrade is not claimed).
After app force-stop/restart, all three library entries and both evidence cards
survived; the original image opened with no backend handle. Regeneration correctly
requested reopening the pinned source. Reimporting the original ZIP deduplicated
it. The exact saved result was reopened and added to the fresh Workspace; its
SHA equals the visual source SHA. A visual Report Pack with that backend appendix
saved successfully, and the appendix is byte-identical to that result JSON.
A separate images-only visual Report Pack also passed CRC/member checks.

Native UI deletion was not exercised: the installed computer-use confirmation
policy requires action-time confirmation for app-interface deletion ("Always
Confirm at Action-Time (Even If Pre-Approved)" / "Delete data (cloud and local)").
Policy source: C:/Users/aurak/.codex/plugins/cache/openai-bundled/computer-use/26.1106.51428/docs/confirmations.md,
referenced by its skills/computer-use/SKILL.md. Existing cards and artifacts were
left intact. Office paste, dependency-mismatch upgrade and every export-format /
template combination remain separate conditions. This is not a full lifecycle PASS.

Final production app tests passed amd6442.960s and38650.208s. The386 supported
`build/windows/test.mjs` runner applies the pinned Wails3.1.0 compatibility patch.
A direct unpatched386 `go test` attempt panicked during Wails callback startup;
that unsupported invocation is not counted as a product test pass. No SDK cache
was edited. T-627 remains REVIEW.

## MSA child charts, options and direct-leaf treemap

Follow-up converts the response composition, topology, timeline, treemap,
method hotspots, explanatory text, pattern groups and built-in preset names
to the same live EN/KO catalog. Recent-file actions and relative times now
follow the selected language. Rule IDs, user names and calculations are unchanged.
The API table now correctly labels callee response, caller-measured duration
and their difference with millisecond units; the difference is not wire latency.

A new coherent synthetic four-profile/two-GUID fixture independently expects
average root1100ms = SQL200ms + call-duration gap50ms + method850ms. Average
method self times are400/250/200ms, per-call maxima400/300/250ms. Native analysis
and saved average JSON match these values. Caller mean250ms, callee mean200ms,
caller p95295ms/max300ms and one call per transaction display under the corrected
headers. This small deterministic example is not a performance benchmark.

The populated fixture exposed a real treemap defect: a direct root-to-service
leaf has no grandchildren, so the old leaf counter incorrectly showed an empty
state. Gate rendering on the actual positive elapsed call input instead. The
before screenshot and FAIL observation remain; the fixed native view shows
/stock100.0% x2 with a250ms timeline call. English/Korean switching preserves
the selected average and values.

Native1024x768 at WebView125%/OS100% covers stacked composition legends, chart
and table modes, internal horizontal scrolling to maximum/share columns and
the English options drawer/preset list. Main and outer horizontal overflow
are false in the attached passive layout report. Windows Settings and Control
Panel launch attempts did not expose a targetable window in the current UI
tool, so no additional actual OS125% case is claimed in this section. Earlier
actual OS125% evidence stays separate. Frontend state tests and the final
production frontend/layoutaudit build pass. Remaining complete reference,
conditional/locale/DPI combinations keep T-627 REVIEW.

## Actual PowerPoint clipboard acceptance and transport fix

PowerPoint16.0.10417.20228 received native Copy image as a picture object in a
new local presentation. Its embedded2400x1600 PNG has RGBA pixels identical to
the original rendered image, including all eight rows, unavailable sample count,
0.35seconds and provenance. Copy complete selected table also retained all data,
but the first paste exposed a formatting failure: the trailing HTML provenance
paragraph became an independent text shape overlapping the table caption. The
saved PPTX confirms that the caption itself was already a table row.

Clipboard HTML now puts title/selection and provenance in merged rows inside
one editable table, with explicit data/metadata font sizes. Formula-like metadata
is protected as text. The saved artifact HTML and TSV remain unchanged. A real
new native build copied the same result again; PowerPoint slide3 now displays
one11-row table (metadata,header,eight data rows,provenance), without overlapping
independent text. Slide1 preserves the successful image and slide2 preserves the
original formatting failure. Native saved PPTX, before/after screenshots and
independent XML/PNG checks are attached. All six saved image/SVG/table/view/spec
members remain byte-identical across the clipboard-only change.

The new transport test parses the clipboard table, checks whole multi-page
selection, Unicode/exact values, escaping, formula-like titles and provenance
within one root. Visuals/clipboard package tests pass amd64 and386; final production
app tests pass42.469s/50.348s using the supported Windows runner. Additional
1024x768/WebView125% SQL threshold switching shows the two200ms synthetic rows.
Word/Excel, other Office versions, SVG insertion and complete clipboard-format
combinations are not inferred from this selected PowerPoint result. T-627 REVIEW.

## Recovered actual OS125% route and dark MSA contrast

The Control Panel launch later exposed Windows Tools. Its Control Panel >
Appearance > Taskbar link opened a targetable Windows Settings window. System >
Display confirmed display1 at100%,3840x2160; selecting125% was then possible.
This supersedes the earlier inability to extend the OS-DPI observations.
The passive app receipt confirms OS scale1.25, physical3840x2160 and logical
3072x1728; WebView zoom remains100%. Settings screenshots containing account
identity were not archived.

Selected saved-visual actions, populated average MSA composition and1024x768
timeline/treemap were observed at actual OS125%. Switching EN to KO retains
root1100ms/call250ms and stock100%x2. A subsequent dark-theme observation exposed
low-contrast timeline axes,service names and outside call labels. Timeline now
uses theme-aware text/grid colors while retaining dark root text on its light
bar. Topology node/edge labels also follow the resolved theme. Native before
FAIL and fixed dark screenshots show readable axes,service and250ms call labels;
the50ms topology label and nodes are visible. Switching back to System/light
recolors the same chart without changing selection/values. Frontend state and
production/native layoutaudit builds pass. OS100% restoration will be recorded
after the remaining selected cases. Full DPI/theme/reference matrix remains open.
