# Review of received Windows P2 execution evidence

Fast-forward pull imported `6a3f0485` and `e113eadc` from `db8dc59b`, preserving
the four preexisting local HTTP changes. The received commits add evidence and
status updates; they change no product implementation. The tested product is
`324a8d97`. This review checks received evidence and repeats portable regressions
on macOS. It is not a new Windows execution or an independent re-review of this
same author's P2 fixes. The original CONDITIONAL review remains pending for a
different AI; T-613 PENDING, T-627 REVIEW and T-662 TODO remain unchanged.

## Verified scope

- Both artifact inventories match every declared SHA-256 and byte length:
  24 WR-01 entries and 42 combined entries, with no unlisted combined files.
  Overlapping entries are counted once by the combined inventory.
- All seven distinct tested source paths match the product Git blobs. The three
  frontend worktree digests match the exact CRLF variant of their LF blobs;
  runner/module/test/origin snapshots are exact LF copies. This validates the
  declared source byte relationship; executable bytes are private and were not
  independently re-hashed here.
- WR-01 seed and readback receipts agree on four requests per architecture,
  empty separate live query, exact nested/encoded URLs, selected sequence2,
  finalized stop and zero drops. Fifteen native query observations are recorded.
- WR-02 JSONs independently recompute to 130.3817031/130.5664087seconds,
  468/464captured=persisted, zero drops, six running checkpoints and two pauses
  per architecture. Both finalStats are finalized,128retained,340/336evicted;
  saved-file digests and selected341/337 receipts agree. Clear count is0.
- The combined native inventory contains the same17case records as the split
  inventories (order differs), with16JPEG references. The screenshotless386
  ascending observation is explicitly reported as accessibility-only. Six
  representative JPEGs were inspected here: amd64 nested Raw, ascending/descending
  sort,386 repeated-query inspector and both final round-trip screens. Their
  visible values agree with the recorded limited scope. Other case verdicts
  remain producer observations, without a fresh live UI re-execution.
- The received TAP records28pass/0fail/0skip. Current host execution repeats all
  28predicate tests successfully; frontend state regressions and production build
  also pass. Windows app/capture package pass counts are received summaries;
  Windows package tests were not rerun here.
- Private store/export/executable files are deliberately unpublished. Full
  store/export comparisons cannot be repeated from public receipts alone.
  Cleanup is a received ownership-checked receipt, not a new remote inventory.

## WREV-01 (P3): qualify the readback comparison claim

The received `readback-verifier-used.py:9–12` drops `starred` and `comment` before
comparing store and export rows, in addition to normalizing host/query. Its
`allOtherFieldsEqualAfterPortableURLNormalization` flag and "all-field" wording
therefore describe a broader comparison than the executed predicate establishes.

The attached [annotation-probe.py](annotation-probe.py) executes that received
function on synthetic rows with different star/comment values. They still
compare equal. This demonstrates a test/claim boundary, not an observed product
annotation-loss defect. The current nested-query and short-finalization evidence
is usable within its disclosed scope; it does not establish annotation equality.

Follow-up under T-613: explicitly list both omitted annotation fields and URL
normalization in future comparison flags/text, or compare the annotation values
with a declared missing/default policy and positive changed-value rejection.
Historical received bytes stay unchanged. The work-status summary now uses the
narrower wording and initially tracked WREV-01 as P3 TODO. No additional product P1/P2 was
established by this evidence review.

## User-requested WREV-01 remediation

WREV-01 is now LOCALLY_VERIFIED. The maintained
`scripts/acceptance/layout/verify_p2_windows_readback.py` replaces the broad flag
with schema2 `allTransactionFieldsMatchDeclaredPolicy`, `annotationsMatch` and a
versioned policy listing no excluded transaction fields. Source retained URL
normalizes expected host/query; exported host/query are actually checked.
Live-session annotations.json supplies authoritative starred/comment by ID,
replacing inline fields as ExportCaptureFile does. Missing annotation values
mean false/empty; wrong types, changed values, missing nondefault values or notes
attached to a different request fail. Other transaction fields remain compared.
This fixture scope excludes request-level annotation overrides and additional
sanitization transformations. Reports omit private IDs and comment values.

The immutable received comparator still accepts changed synthetic star/comment;
the maintained comparator rejects each change. Mac and Linux/Docker each pass
23 stdlib unittest cases, including four-row query and128-row soak file fixtures,
default/cleared/Unicode notes, missing/corrupt sidecars, identity/extra-field
changes and CLI failure without success publication (also under Python -O).
Existing reports are never overwritten. The two targeted Go annotation boundary
and real export/save/reopen tests pass. The existing SDK deployment-target linker
warnings remain; no Go product implementation changed. CI now executes the new
Python regressions when either maintained script changes.

These are synthetic/host regressions, not new native Windows annotation readback.
Old Windows raw files are private, and their old flags are not upgraded by this
fix. The new verifier can be run on that owning Windows host to publish a separate
schema2 result. The archived used scripts/reports and all received digests remain
unchanged. See [regression-results.json](regression-results.json) and
[remediation-probe.json](remediation-probe.json).

## Remaining acceptance

The130-second run does not reach the10-minute Clear branch, establish new
two-hour behavior or memory/handle plateau, or exercise native disk-fault
injection. The28controlled predicate cases do not replace those native scenarios.
Both runs have seven tree processes; private bytes and root handles increase
during the short window, which is insufficient for a leak verdict. Existing
long-run receipts remain separate. Full original-reference, locale/DPI,
body/vendor/driver/service acceptance and independent re-review remain open.

See [local.json](local.json) and [artifact-sha256.json](artifact-sha256.json).
The user authorized committing and pushing this review, WREV-01 remediation,
regression evidence and tracker updates. Git history records publication; the
original independent review and native acceptance gates remain separate.
