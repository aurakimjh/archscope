# T-613 column/filter/keyboard and annotation boundaries

Two product fixes are committed separately:

- `4db0294c`: Type filtering now uses the resource classification shown by the
  grid. Live Path values/filtering/sorting include the query from the retained
  redacted URL when the separate query field is empty. Unknown negative byte
  sizes no longer satisfy numeric upper/lower bounds.
- `49c31b3b`: the comment editor counts UTF-8 bytes against4096, shows a KO/EN
  overflow instruction and disables Save without truncating the draft. The
  service rejects both oversized input and output that grows past4096 after
  redaction, before changing committed annotations.

The native binaries use product commit49c31b3b. The later144-combination state
regression is test-only. Baseline uses eab03d07. validation.json pins executable
hashes, commands, scope and limitations. There are40 native observations:
35 selected PASS,1 baseline FAIL and4 explicitly INCONCLUSIVE attempts.

## Data and native filter/keyboard observations

Each isolated Windows desktop owns two real Node producers attached by PID/start.
They send120 synthetic requests through the real explicit proxy to the labeled
Docker loopback origin. Bodies stay disabled, backend live window64, physical
retention128. Both producer identities are confirmed. Seed receipts pin120 rows
and zero drops; all traffic and capture profiles are test-owned.

amd64 Type contains `json` finds40/120. The mounted second producer header shows
20; the complementary20 is fixture-derived, not a claim both virtual headers
were visible. Path contains `slot=004` finds exactly request5. ProcessB70/140
AND Path for producerA gives0; ProcessB AND `slot=129` gives request130. A no-match
Type value gives an empty grid; clearing it restores rows. The old selected detail
may remain while filtered out; process changes clear selection.

Under Type=json, PageUp112→52 and PageDown52→112 traverse ten filtered request
rows. Down112→118 then another Down keeps the final selection118. Live descending
sorting preserves request5's star/comment; Down selects3, Up restores5 and its
note. An initial cross-group expectation was wrong because descending sorting
changes group order. The actual out-of-range PageDown is a no-op, and PageUp5→25
is recorded separately. The mistaken expectation is retained as INCONCLUSIVE.

On386, Ctrl+Shift+Left collapses both process groups of the first25/60 stored
search snapshot into headers13+12; Ctrl+Shift+Right restores request rows. With
explicit focus in the comment textarea, Ctrl+Shift+Left selects the word `draft`
and leaves the process group expanded. The first attempt without proven focus
is INCONCLUSIVE, followed by the successful focused observation.

amd64 column width1 normalizes to48 and900 to800. The grid scrolls horizontally
inside its pane. Restore defaults returns the compact default column layout
without changing sort/selection. One column-editor drag did not establish
reordering; arrow-button ordering was already verified in the earlier receipt.
No header drag-resize implementation or full optional-column display matrix is
claimed by these tests.

The independent state fixture has48 rows, two process identities, three MIME/
status values and one ID-bound annotation. Three method scopes × four column
filters × six groups × two sort directions give144 combinations. Expected
filtered IDs come from fixture-index predicates. Each combination checks group/
sort ID preservation, deltas−10/−1/+1/+10 over visible request order, collapsed
rows and unchanged annotation identity. This is state coverage, not144 native
UI verdicts.

## Annotation boundary and failure observations

Ordinary baseline Ctrl+A/type_text input requested4096 ASCII bytes but kept1024:
the old maxlength limited UTF-16 characters. UI Automation set_value bypassed that
limit and is separately INCONCLUSIVE as ordinary-input evidence. On the fixed
app one type_text delivery produced8192 rather than4096; the overflow guard
correctly blocked it. Exact fixed boundaries below use set_value plus native
Save and independent disk readback, without a paste-delivery fidelity claim.

- amd64:4096 ASCII bytes,1365 Hangul plus `a` (4096 bytes), and1024 emoji
  (4096 bytes) save exactly.4097 ASCII bytes retain the draft, show the overflow
  counter and disable Save.
- 386: the4096-byte Hangul and emoji cases save exactly;1366 Hangul characters
  (4098 bytes) are refused without changing the saved emoji note. Empty0-byte
  comment saves and clears the prior saved comment.
- Both:300 repetitions of synthetic `password=x ` are3300 input bytes, but
  masking grows the output beyond4096. Native Save refuses the update, retains
  the draft and preserves the previous readable annotation. A short synthetic
  secret then saves as `password=[REDACTED]` and clears the error.
- amd64: an owned reader denies delete sharing for1600ms after staging creation,
  exceeding the existing one-second retry. Save returns an error with the prior
  annotation and draft intact. Releasing the reader and saving the same draft
  succeeds. No staging files remain after the runs. The full error screenshot
  with the private capture-directory identifier is excluded; the sanitized
  native observation and disk readback remain.

The Go boundary test first failed because3300 input bytes were accepted even
though the redacted result exceeded4096. It now covers valid ASCII/Hangul/emoji
boundaries, overflow, redaction expansion, and unchanged prior annotations on
rejected service updates. Full production app tests pass on amd64(38.418s) and
386(44.482s). Frontend state and production build pass. No new Windows race run
is claimed with CGO=0.

## Eviction, exports and remaining conditions

amd64 adds20 requests:140 persisted,128 retained,12 evicted, zero drops. Annotated
request5 itself is evicted. Its detail selection clears after authoritative
fetch; an old live-tail row can still remain until the renderer cap. The old
annotation entry remains on disk, but the128-row export contains only13..140,
has no invalid selected ID and does not transfer the note/star to any retained
request. Automatic orphan compaction is not established.

Independent readback checks three portable exports: amd64120 rows before target
eviction, amd64128 afterward, and386120 with a redacted note before its later
empty-comment edit. Unique IDs, selection, note and every other field agree with
the source after the declared portable host/query normalization. Export hashes
pin these snapshots; no equality with later edited annotations is implied.

T-613 remains PENDING. Remaining work includes every optional column/native
filter/operator/process/group/keyboard cross-product, conclusive drag behavior,
IME/input-method and actual locale/DPI/reference matrices. Existing oversized
annotation files still report errors; they are not silently migrated/truncated.
Orphan cleanup, complete persistence-failure/concurrency combinations and memory
acceptance remain separate. Positive live-body/POST search, vendor COM/foreign
cookies, transparent/service reach and longer memory/handle diagnosis retain
their existing gates. Earlier two-hour evidence is unchanged; T-627 stays REVIEW.

The runner/producer/reader and readback sources are archived exactly as used,
with private workstation paths. They are receipts, not a general harness. An
initial unsupported list limit returned25 and stopped the first baseline runner;
the archived corrected runner paginates limit100. Baseline2 and both fixed runs
have valid120-row seed receipts. Raw credentials, stores and sessions stay private.
Cleanup verifies zero owned desktop/producers and removes only the labeled Docker
fixture; all seven preexisting containers remain. No security/privacy/trust/COM
or body policy changes were made. artifact-hashes.json pins exact public bytes.
