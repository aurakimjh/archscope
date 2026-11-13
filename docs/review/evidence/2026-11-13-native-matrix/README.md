# Windows native selected matrix, 2026-11-13

T-627 remains **REVIEW**. [native-cases.json](native-cases.json) preserves52 selected
observations, including historical failures and their subsequent correcting
commits. A selected PASS describes only that case. The actual native UI was
operated through Windows Computer Use; layout JSON is a passive observer.

Five product fixes were committed separately: profiler percentage, GC table
containment, browser CPU percentage, primary control/HTTP client area, and
handle-backed Export Center integrity. The corresponding before/after receipts
are in this directory. Unknown saved access_log HTML refuses without output;
its original JSON saves byte-for-byte. Fresh profiler HTML and PPTX preserve
10000 samples. The PPTX implementation emits metadata and summary slides only.

All8 Chart Studio templates rendered across the session (comparison uses an
identical-result zero-delta case). Native PNG2428x1400, valid SVG1214x700 and
CSV request-count6 exports were independently parsed. This does not establish
every renderer/format combination for every template. Lighthouse imported
score42 and provenance were checked in native JSON/HTML saves.

Actual Windows Display settings changed display1 scaling100% to125%, then back
to100%. Wails GetScreen reported scale1.25,3840x2160 physical and3072x1728 logical;
WebView DPR was1.25 with QA zoom100%. Selected1024x768 and1280x800 chart/help and
HTTP Header cases are in the os125 screenshots and phase JSON. Tool screenshots
are normalized; use raw geometry for dimensions. A phase contains earlier views
at other scales too; each view carries DPR and the receipt records the latest
OS screen metadata. WebView125% tests are separately identified.

Remaining acceptance includes original reference equivalence, all21 routes at
all conditional/error/DPI combinations, populated JFR heatmap and remaining JVM
subtabs, complete visual-library/report-pack lifecycle and remaining exports.
MSA populated labels still mix English in Korean; average/export modes were not
observed. UIA did not establish drawer focus restoration. Sparse fixtures do
not establish a70% result-area verdict. HTTP0.702 is only the selected standard
client/Headers case in its specific receipt.

Latest production amd64/386 app tests pass50.468s/53.294s, both GUI builds pass,
and production+layoutaudit compile/vet passes with no tests run. Frontend state
and production build, profile/parser and8 analysis/exporter Go packages passed
with the separately committed fixes. No CGO/race claim. QA-only helpers remain
excluded from production. Korean/System,100ms/TopN20/wall and OS100% restored.

[artifact-hashes.json](artifact-hashes.json) inventories the sanitized fixtures,
outputs, geometry and screenshots. T-613 live/reentry/crash/soak evidence is
recorded separately, with its actual executable source and hash.
