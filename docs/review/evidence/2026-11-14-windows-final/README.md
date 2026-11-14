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
