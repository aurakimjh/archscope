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
