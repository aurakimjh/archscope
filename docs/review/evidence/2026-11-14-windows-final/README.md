# Windows remaining-test follow-up

This is an incremental scoped receipt. T-613 remains PENDING and T-627 REVIEW;
the original independent CONDITIONAL review is unchanged. Production import
memory and passive layout-audit native UI are separately identified below.

## 2. Unicode notes and editing guards

Native Windows amd64 layout-audit build at adfc9544, with the ordinary production
frontend and passive observer, opens a synthetic32-row session. A selected star
and exact33UTF-8-byte Hangul/emoji/combining-accent note survive native Save,
CtrlS to a new file and controller-driven native reopen. Full-field comparator
policy1.0.0 verifies all32rows and selection; absent false/empty annotations are
the declared Go omitempty defaults. The original source is unchanged.

CtrlS while the textarea is focused does not open a dialog; after clicking the
grid it opens exactly one Save dialog. Native file-name keyboard entry/Enter
completes the save. The modal accessibility indexes were not actionable through
the helper, so observed Alt+N/keyboard entry was used instead.

Two column-editor native drag attempts did not establish reordering; they are
INCONCLUSIVE, not PASS or an established product defect. The helper rejects the
Hangul composition key, so actual IME composition is still unverified. Literal
type_text duplicated a Unicode delivery; it is excluded as an input-fidelity
claim. Exact set_value plus native Save and independent file comparison establish
the final value, not ordinary IME/paste fidelity. Existing4096-byte boundary,
redaction-growth and persistence-failure receipts remain unchanged.

Files: notes-unicode-*.jpg/txt, columns-drag-attempt.*, unicode-readback.json,
native-cases.json. Private controllers, tokens and session files stay ignored.

## 1. Import memory and reentry

The initial10-cycle32/512/4096-row import screen exceeded tree growth budgets on
both production architectures. It is retained as GROWTH_REQUIRES_DIAGNOSIS, not
retrospectively relabeled PASS. Native Workspace shows30result cards after30
imports. The workspace intentionally retains up to80results; a separately
declared36-cycle run warms up28cycles before8measured endpoints to cross that
limit. No GC/cache trim is forced.

The36-cycle x86 run passes with tree delta+6.203MiB and median pair slope
+0.653MiB/cycle. The36-cycle x64 run still fails the unchanged slope budget
(delta+32.008MiB, slope+5.943MiB/cycle). A separately declared16-cycle continuation
in that same owned x64 app adds8warmup and8measured cycles; its last window passes
(+7.090MiB,+2.296MiB/cycle, root handles664→664). Original early FAIL receipts are
preserved. This is a finite late-window result, not universal leak freedom or a
claim that all initial growth was established to be a cache.

Five result receipts are independently recalculated. Every measured window has
the same7process identities. The80-result registry bound and eviction of old
payload references are covered by a101-import state regression; native accessible
text truncates at19999characters and exposes only35titles, so it does not prove
the full native card count. The initial30-card observation was below that limit.
Full app tests after the two fixes pass43.890s/46.409s on Windows amd64/386.

Native amd64 Workspace→Capture reentry restores32rows, selected first request and
Raw200. The route transition is separate from the automatic matched samples.

## Imported annotation growth defect

Baseline native Open and a new Go regression accept3300input bytes that expand
to6000redacted comment bytes. Sanitize now checks the4096-byte limit after masking
as well as before. Windows amd64/386 sessionfile suites pass1.124s/1.008s; exact
ASCII/Hangul/emoji4096-byte boundaries remain valid and source bytes never change.
Both fixed desktop controller matrices reject the expanding input, truncated JSON
and duplicate IDs, and subsequently open the valid32-row file. Native CtrlO also
shows the refusal after expanding file tools. The initially hidden error is a
separate UI defect still being corrected; no silent rejection UI PASS is claimed.

## Hidden import-error correction

Native CtrlO from collapsed file tools reproduced an invisible failure despite
the typed refusal. HttpCapturePage now expands file tools when an import error
arrives. The same native CtrlO flow automatically shows the error and Open retry
controls with no reveal click. Frontend state tests and production build pass;
both guarded desktop builds succeed. A valid32-row retry succeeds. This changes
error visibility, not file contents or the validation refusal.

## 3. Builder and file combinations

Native amd64 Builder uses the owned loopback Docker origin: GET302does not follow
Location, cancellation stops a15second delayed request at8400ms, malformed headers
are refused before any request, and edited PATCH with18UTF8bytes succeeds200after
correction. The original32-row file/selection/note are separate and unchanged.
Native Builder/source panes and origin request hashes are observed independently.

Both fixed architectures pass23controller cases each: truncated/duplicate-ID/
post-masking-overflow input refusal with unchanged bytes;10export formats × all/
filtered scopes; stale filtered-ID refusal and valid retry. Session/HAR preserve
all values and selection; XML additionally declares empty cookie slices→null
(64fields for32rows,6for3rows). Compact XML explicitly omits bodies. Native HALOG
refuses the emoji comment under codepage949without output, then a separately
declared compatible-comment input exports/reopens successfully. No silent
transcoding or exact native-format field parity is claimed. Text/URL/HTML/XLS and
response folders are output checks, not universal native dialog/Office acceptance.

Initial harness assumptions that native949could encode emoji and that XML empty
cookie arrays remain[]were corrected before the successful v3runs; original setup
failures stay private. They are not new product defects. Native Go Builder/proxy/
sessionfile tests pass on both arches (functional-validation.json), covering raw/
form/multipart, timeout/cancel, TLS refusal and response limits. GUI form/multipart
and Builder drag remain separate from those backend tests.

## Retained body redaction provenance defect

Full-field comparison of the actual body-qualified x86 session exposes two request
redacted=true flags disappearing in export while the payloads remain masked.
Sanitize now ORs prior provenance with newly applied masking. A three-cycle
export/open regression fails before the fix and passes afterward in both full
Windows sessionfile suites (1.252s/1.255s). The existing disclosure must survive
idempotent masking; this is not a plaintext-body leak. Native store reexport is
verified separately below when complete.

## 4. Actual body gate and retained-content verification

Fresh fixed x86 reports actual sec10_verified and captures only the attached
owned Node producer's synthetic loopback traffic with bodies enabled for that
session. The first four exchanges verify JSON/form masking, duplicate request/
response cookie names and two Set-Cookie headers,262144retained of300020response
bytes, positive stored-body/POST/cookie search, selected export and native reopen.
The first harness attempt assumed completion order equaled request sequence;
the large body completed after HTML. The corrected probe binds checks by URL.
No product ordering defect or initial harness PASS is claimed.

Full-field comparison then finds the redacted-flag defect above. Fixed40188233
reexports that same retained store and passes policy1.0.0 with all4transactions,
selection and flags. The original pre-fix export remains unchanged. Native/echo
Hints now shows REDACTED after reopen. A new6-exchange capture additionally
verifies a sanitized1x1PNG, exact gzip-decoded JSON (34decoded/54wire bytes), no
drops and full-field policy1.0.0 readback of all6transactions. Native Body/Preview,
Cookies and truncation hints match the dedicated receipts. HTML's script text
stays literal in formatting-only preview; accessible document remains ArchScope.
Raster metadata removal/pixel-not-masked disclosure stays visible.

Fresh x64 actual preflight reports sec10_unavailable/local_dumps_collecting.
The real start request with captureBodies=true is refused; no bypass or positive
x64body claim is made. Normal per-executable startup preflight runs; no manual
security/privacy/trust or body-policy UI changes are made. Enabling the synthetic
x86 session through the product controller does not change the default-off policy.

Docker24capture packages pass race before the redacted-flag correction; a final
Docker sessionfile race run including both new regressions passes1.206s. Windows
CGO0test results are not Windows race claims. Body-enabled long-memory, additional
formats/limits/error conditions and every native combination remain separate.

## Unsupported Preview explanation

Native gzip JSON is retained and readable in Raw, but its unsupported graphical
Preview incorrectly said the payload was not retained. The fallback now explains
HTML/raster support and points to Raw/JSON whenever retained text is present.
Absent text keeps the existing unavailable message. EN/KO labels compile;
frontend state/build pass and native KO before FAIL/after PASS uses the same file.

## 5. T-627 profiler native conditions

Malformed input produces0records/1skipped line with INVALID_SAMPLE_COUNT and
NO_VALID_RECORDS in Parser Report. A valid200path/20level retry renders normally;
its clipped plot ratio0.598is disclosed, not relabeled70percent. A separately
prepared200path/32level fixture fills0.727of the default1264x741CSSviewport at
1280x800native size/actualOS100percent/WebView100percent before scrolling. The
colored data rows visibly occupy that area; no blank canvas inflation is used.

Selecting worker.4 shows4100/20100weight(20.40percent), matching fixture arithmetic.
At1024x768the selected path and reset control survive. Help fits, Escape dismisses
and Return reopens from the help trigger. Options settle fully inside the small
window with internal scrolling, reachable Close/footer Analyze, Escape/Return
trigger roundtrip and unchanged selected path. Intermediate animation frames are
not used for fit verdicts. File-picker Escape preserves the prior input/result;
this is not in-flight analysis cancellation. Full original-reference, actual
OS125percent/locale combinations and remaining profiler conditions stay open.

## T-627 diff input provenance

Raw-weight forward20100→22600shows12.4percent regression; reverse22600→20100shows
-11.1percent improvement, matching fixture arithmetic. Replacing a path initially
left the old chart/summary under the new input. DiffPage now clears result/error
when either path or normalization changes. Native before FAIL/after PASS verifies
immediate invalidation, actual missing-target error and successful valid retry.
The initial forward screen is1024x768; the fixed reverse/retry is1280x800. This
does not close every mismatch/cancellation/locale/DPI/reference condition.

## Diff weights and Windows integer bounds

A selected native raw Diff frame incorrectly displayed4,600,000,000samples for
4,600canonical weight. The million-scaled legacy projection also overflowed to
-2147483648on Windows386. The UI now uses canonical metadata.a/b for geometry,
search and tooltips; normalized results label fractional shares explicitly.
Legacy fallback removes the scale, and fractional search/tooltips retain up to
six decimals. Go caps only the legacy integer projection at native max-int,
with explicit samples_scale/samples_clamped metadata; exact a/b/delta survives.

The new386regression fails before the fix, then both complete Windows profiler
suites pass(0.953s/1.013s). Frontend state/build pass. Native x64and386show
4,600weight/20.35percent for worker.4 with the same raw reverse fixture. These
UI receipts precede the final six-decimal formatting-only rebuild; normalized
and final-build smoke are separate observations. Original input parity and all
Diff cancellation/locale/DPI combinations remain open.

## GC native conditions and dense event rows

Two no-clock events show ordinal #1/#2 and an ordinal chart axis. Missing rate/
throughput evidence stays unavailable; this sparse fixture is not area-qualified.
The60-event fixture shows1..60seconds,10..21ms repeating pauses,930ms total,
15.5ms average, and200/100/50MB heap series. Summary shows plot plus more than10
complete rows. JVM tab explicitly reports missing source information. Keyboard
Left moves from JVM to Events.1024x768options fit, Escape returns focus and
Return reopens without dropping the selected tab.

Events initially wraps its long cause/type cells and shows only7complete rows.
The row now keeps text on one line inside its existing horizontal scroller.
Same native input shows16complete rows at1280x800and at least14at1024x768.
Frontend build passes; this is a CSS-only change, verified in real native views.
No blanket70percent/actualOS125percent/original-reference verdict is claimed.

## JFR source replacement, binary input and event rows (continued November15)

Malformed JSON reports line1col4/ENGINE_FAILED and valid sparse3-event JSON retry
removes it, preserving42.5ms GC/12ms park and sparse-stack warning. Stack→Right
activates Wall-clock tab; sparse all-mode input has no heatmap cells. Selected
previous heatmap/native-memory receipts remain separate.

Replacing the file left the old result/findings under a new filename, including
when the new analysis failed. File select/clear now invalidate both inflight
identities, projections and errors; each new analysis starts empty. Native fixed
binary3010-event→invalidJSON replacement immediately empties old metrics and
findings, and subsequent actual parse failure leaves them empty.

An owned4second Java workload records a real .jfr with MicrosoftJDK25.0.1.8.
Without ARCHSCOPE_JFR_CLI the existing explicit-config boundary correctly refuses
binary conversion. A separate owned app launched with the absolute installed
jfr.exe path converts it and reports3010total/filtered events,153stack samples,
29unique stacks. Independent jfr print JSON confirms3010events/55types. Aggregate
oracle/hash and generator source are archived; raw JFR/JSON stay private because
JDK startup metadata contains host details. No foreign JFR/full-parser claim.

Long timestamps and row spacing initially permit fewer than10notable rows. Rows
now preserve timestamps on one line with compact24px evidence targets and existing
internal horizontal scrolling; same binary shows11complete rows at1280x800.
Frontend state/build pass. No global area/actualOS125/locale/reference/cancel claim.
Initial build overlapped the final frontend build; that candidate was discarded
and rebuilt after frontend completion before any launch or verdict.

## Access log partial parsing, filters and result provenance

The six-line repository sample repeated20times plus one malformed line produces
120parsed/1skipped(line121NO_FORMAT_MATCH). Native status80x2xx/20x4xx/20x5xx,
40/120=33.33percent errors, sole-minute request120and P95=1500ms match fixture
arithmetic. Parser→Left activates Status with keyboard focus. Static-only URL
filter produces0rows and Home restores all5URI aggregates. A direct helper
click on the HTML dropdown was refused for popup ownership; keyboard End/Return
works. This is not a product dropdown defect.

URI aggregates intentionally span methods; /api/orders/1001combines GET+DELETE
into40requests/20errors/811.5ms mean. The column mislabeled its representative
GET as Method; it now says Sample method in EN/KO without changing backend
aggregation/schema. Clear file also left old aggregates visible with no input;
select/clear/new-run now invalidate inflight identity and prior result/error.
Native before FAIL/after PASS and frontend state/build pass. Sparse5URI/one-minute
views do not establish the full70percent/dense/OS-DPI/reference/output matrix.

## Exception groups, long stacks and input lifecycle

The owned generator creates35exceptions/35signatures/1type with51frames each.
Native totals match. Case0 detail drawer scroll reaches ProcessingStage49;
Escape returns focus to Details and Return reopens the same event. A nonmatching
search gives explicit0items while source totals stay35; clearing restores rows.
Case1signature group shows its exact signature/count1in an inline card below the
35-row table, reachable with two scrolls. This is not an immediate-viewport or
signature-drawer PASS.

Initial event rows wrap the frame-count label and expose fewer than10complete
rows. One-line cells and compact padding now show10plus paging before scrolling.
Clear file also left all old results visible. Select/clear/new-run now invalidate
inflight identity, result, filter, page and active event/signature. An intermediate
candidate cleared result but retained the selected signature card; that FAIL is
preserved. Final native Case1select→Clear removes both result and selected card.
Frontend state/build pass. Small-window/actualOS125/locale/reference/partial-file
matrix remains separate; no universal70percent verdict.

## Thread single/multiple/lock modes and live option identity

Owned text fixtures contain2BLOCKED threads holding/waiting for opposite locks
at two declared timestamps. Native single reports2threads/2BLOCKED/2holders and
correct acquire frames/lock IDs. Multi-picker preserves explicit reverse02→01
order in result source rows. Without process/thread lifetime context,4snapshots
stay isolated and a warning disables cross-dump persistence;0is not healthy proof.
Lock mode detects2contended/2unique locks and1canonical worker1→worker2→worker1
cycle with exact0x00000002/01edges. Hold/wait4counts are across2dump observations.
No real process was deadlocked.

The analysis callback omitted correlationContextJSON from dependencies: editing
only that field reran with the prior empty context. Added the dependency and clear
old result at new-run start. Native fixed incomplete context now refuses with
THREAD_CORRELATION_PROCESS_CONTEXT_REQUIRED; changing only JSON then retries
successfully, joins4snapshots into2unique threads and removes isolation warning.
Threshold3does not qualify2snapshots; changing it to2correctly yields exactly2
persistent BLOCKED threads with first index0and matching full signatures. Context
is an explicit synthetic caller declaration, not producer authentication.

Native file-picker Escape preserves two paths/result/selected tab. Clear All
removes inputs/results and disables Analyze. Frontend state/build pass. Sparse
fixtures and file-picker cancellation do not close dense/full-format/actualOS-DPI/
locale/reference/inflight-cancellation combinations.

## MSA partial inputs, capacity exclusions and file-set provenance

Native repository sample_count_mismatch.txt retains1profile/84ms with explicit
TRANSACTION_COUNT_MISMATCH(declared4/found1TXID). A separately generated missing
END/TOTAL profile reports MISSING_PROFILE_END/MISSING_TOTAL and complete1of2in
Parser diagnostics. Its initial incomplete-count observation is INCONCLUSIVE,
not a defect/PASS: that summary counter specifically counts producer capacity
markers. Labels now explicitly say Capacity-limited profiles in EN/KO, including
the file table, so parser errors and capacity flags are distinguishable. No new
backend rule about averaging every kind of failed profile is claimed.

Synthetic Profile capacity exceeded marker yields1capacity-limited profile,
1excluded group and0signatures with the existing explicit5MB notice. This is
parser/UI marker qualification, not actual5MB producer pressure. Adding a complete
profile and rerunning yields average Go drilldown1/1and84ms; excluded capacity
group is not counted. With only excluded groups, MSA average initially showed an
empty selector/disabled exports without guidance. It now explains that no eligible
signature exists and points to Summary/Parser diagnostics.

Adding/removing/restoring input selections previously left prior result/exports
under a different file vector. Changes now invalidate the result and inflight
identity, reset drilldown keys, and ignore late replies. New-run starts empty;
unmount retires the request. Native second-file addition immediately removes old
result/export controls, and a valid rerun restores the mixed-input result.
Frontend state/build pass; native before/after cases are separate build pins.
Current native MSA accepts Jennifer text exports(.txt/.log/.profile); no CSV column
mapping control is exposed. CSV mapping/reference equivalence, every locale/DPI/
partial combination and in-flight cancellation are not declared complete.

## Trace input identity, units and missing parents

Owned Jaeger fixtures produce12traces/36spans, each240ms with90/70ms children.
Invalid JSON refuses with ENGINE_FAILED and valid retry restores correct values.
Changing input immediately clears old rows; select/clear/new-run retire inflight
identity and result/error. An orphan fixture keeps3spans, reports missingParent1,
1dependency and explicit warning without inventing a missing parent.

Trace/Span/average/estimated-path duration headers now explicitly state ms in
EN/KO. Native ef9c9dd8-plus-trace-units verifies final headers and values. Prior
unit-omission PARTIAL is preserved. Pre-fix span layout had20complete rows and
0.714clipped union at1280x800(OS100/Web100); Trace had12complete/0.618. This does
not establish every tab/locale/DPI/reference combination. Trace route exposes
tabs/options/evidence; it has no per-trace selection/filter control, so those
checklist interactions are not asserted complete. Frontend state/build pass.

## Browser CPU count, weight units, search and input lifecycle

Owned V8 fixture has12sample records, twelve1000us deltas and12000us recording.
By the existing observation-interval convention, eleven1ms weighted runs remain
(the final observation has zero tail):11000us/11ms, renderList6000us/54.55percent.
Native initially mislabeled11000as sample count and kept all results after Clear.
The count now uses profile_sample_count(12), while table/flame tooltip weights
explicitly useµs when value_unit=microseconds. No engine values changed. A missing
count is shown as unavailable instead of inventing0. Nonmatching search says no
match; selecting renderList zooms and shows exact6000µs/54.55percent.

Select/clear/new-run now clear prior result/error and retire request identity;
unmount also ignores late replies. Native fixed Clear removes all prior content.
Frontend state/build pass. Native77dc2bb4-plus-cpu precedes the final EN/KO
screen-reader caption correction(sample share→recorded weight share); that text
is build-verified. Sparse2stack fixture does not qualify dense70percent/all-format/
actualOS-DPI/reference matrix or native inflight cancellation.

## Lighthouse partial values and result lifecycle

Owned partial fixture replaces performance score with null, removes LCP numeric
measurement and declares runtime interruption. Native shows unavailable—not0—
performance/LCP score, retains CLS0.08/96 and4audits/2requests/48.8KB; explicit
LIGHTHOUSE_RUNTIME_ERROR warns incomplete data. Network73/66ms match input.
Malformed JSON refuses BROWSER_AUDIT_FAILED and valid partial retry clears error.
The route has no audit filter control; no0filter interaction is asserted.

Clear initially retained all result/export content without an input. Select/clear/
new-run now reset result/error/notice and retire inflight identity, including on
unmount; late replies cannot repopulate a changed input. Native5f28e08d-plus-audit
fixed Clear removes all prior content/actions. Frontend state/build pass. Per-audit
failure detail, every format/locale/DPI/reference/export combination remain
separate from the tested null-score/global runtime warning and prior42score
JSON/HTML export evidence.

## JFR evidence must preserve missing durations

Source review found the evidence callback substituted0ms for a null duration
when message was empty. It now uses the shared unavailable-aware milliseconds
formatter. Owned1event SafepointBegin JSON has no duration/message. Native source
table shows—; generated Evidence Board card shows- and exact duration_ms:null
payload/source/jfr:event:1. No backend mutation or0ms assertion. Card is below
7preexisting visual artifacts and requires scroll; those artifacts were preserved.
Frontend state/build pass. Native5f28e08d-plus-audit-jfr-evidence includes this
change plus the prior CPU caption and Lighthouse fixes. Source review is the
before evidence; no native before screenshot is claimed for the callback bug.

## Workspace registration, active selection and visible results

Native registered JFR1event and removal gives explicit empty state; removing that
owned result leaves independent evidence/artifacts intact. Initial expanded
import assistance pushed result cards below the default viewport. Import controls
now start collapsed when existing entries are present, and start open for an
empty workspace; explicit user toggle remains stable. Job/status/error/result
provider content stays outside the collapsed import form.

Fresh4677d721-plus-workspace-fold analyzes ownedAccess120requests andTrace12/36.
Both cards, exact source paths and summaries now fit before scroll at1280x800.
Use onAccess switches active selection without altering values. Import assistance
reopens all controls; Spacecollapses it with visible focus and preserves selection.
Frontend state/build pass. Scoped improvement, not every result kind/70percent/
locale/DPI/reference/provider-combination acceptance.

## Export Center formats, failure retry and canceled output draft

Native Workspace-selectedAccess result is correctly selected in ExportCenter.
HTML writing to an existingdirectory refuses visibly; a distinctfile retry
succeeds. Native5mode exports(HTML/JSON/singleCSV/CSVdirectory/PPTX) succeed.
Independent readback confirms120requests/315.5msmean/P951500,18CSVfiles and3PPTX
slides,validZIP/XML,and every displayed PPTX metric matchesJSON. PPTX intentionally
contains only10summary fields; initial verifier wrongly expected average in that
subset and was corrected to verify actualdisplayedkeys. FullOffice rendering is
not asserted. No source was changed. Output hashes:workspace-export-readback.json.

Native Browse→Escape erased a previouslytypedpath. browseOutput now only updates
on nonemptyselection and surfaces realdialog errors. Fixed0b4fcd83-plus-export-cancel
preserves exactdraft; emptyWorkspace disablesExport. Frontendstate/buildpass.
The5outputmodes use4677d721-plus-workspace-fold; cancellationfixedbuild is separate.
HTMLselect popup refresh twice returned helper no-monitor errors after successful
selection; relist/activate/reobserve recovered. No product dropdown failure claim.

PPTX source also revealed Go map iteration selecting random10summary fields with
no omissionnotice; stable boundedselection correction is a separate follow-up,
not alreadycovered by the oldoutput receipt. No multiselection control exists in
ExportCenter; full kinds/format/reference/locale/DPI/cancelmatrix remains separate.

## Stable bounded PowerPoint summaries

Native priorPPTX contained correctvalues for a random10fieldsubset; the initial
readback assumption that315.5must appear was wrong for the documented10fieldcap.
Source review separately establishes Go map iteration makes subset/order unstable;
callers cannot pre-sort a Go map as the oldcomment suggested. Export now sorts
keys, retains10metrics and explicitly states exactomittedcount/JSON-CSValternative.
Meaningful32key/20projection regression failsbefore(noomissionnotice) and passes
after,fullPPTXexporter suiteWindowsamd640.698s/3860.625s. Sourceoriginalmeasurement
semantics unchanged. Nativeafter is pending finalintegratedbuild; oldnativePPTX
receipt is not relabeled. This does not assert independentPythonorderingparity.

## Report Diff nonzero changes, input identity and retry

Owned legacyJSON reports100→125requests give+25/+25percent,250→200ms mean gives
-50/-20percent; changedmetrics2/findingdelta0. Added-onlynew_metric is excluded by
the existing common-numeric-key contract, not synthesized as0. No UI filter or
per-row selection control exists. ChangingAfterpath initially kept oldcomparison;
input/labelchanges now retire request identity, clear result/error/busy, ignore
late replies and retire on unmount. Native519051ef-plus-report-diff verifies
immediateclear,invalidJSONdecode refusal and validnonzero retry. BeforepickerEscape
preservesbothpaths/result. 1024x768 OS100/Web100 retainsallcontrols/twoexactrows;
passive1008x709dpr1 reports nohorizontal/outeroverflow. No70percent/denseclaim.
Frontendstate/build and fullreportdiff suitesWindowsamd640.636s/3860.647s pass.
Full type/precision/refusal/export/reference/locale/DPI matrix remains separate.
