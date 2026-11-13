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
