# Remaining acceptance and native follow-up, 2026-11-14

T-627 remains REVIEW and T-613 remains PENDING. The concrete21-route and16-case
remaining inventories are in remaining-acceptance.json and the dated sections of
../../../ko/DESKTOP_CONTENT_LAYOUT.md and ../../../ko/SYSTEM_HTTP_CAPTURE.md.
Selected successful slices do not close all conditions on a route or contract.

## Recovery report defect and native correction

The current production crash/restart harness captures12 synthetic loopback HTTP
requests with bodies disabled, force-terminates only its owned desktop, then adds
an uncommitted27B rolling staging file and35B unreferenced blob to its verified
isolated stopped store. This does not corrupt committed records or simulate
power loss. On restart the product removes exactly62B and preserves all12record
bytes and IDs. The old service discarded the opening recovery report and showed
0B; it also substituted cumulative persisted count for retained rows.

d91376ea adds OpenWithRecovery and forwards the actual report. Both architectures
now show1session/12records/62B and pass exact committed-byte/ID preservation and
save/open selection. Unit coverage includes incomplete NDJSON,2retained of3saved
rows, orphan cleanup and a second scan with0new discarded bytes. Both complete
capture suites pass; production app tests pass42.123s/44.388s. recovery-validation
pins hashes and scope. The later Builder change is renderer-only.

The initial concurrent same-title amd64 screenshots are INCONCLUSIVE: foreground
images did not establish architecture ownership. They remain in history. A
single-owned-window amd64 rerun and matching386file timestamps provide the scoped
visual proof. Baseline386's0B FAIL remains unchanged. Fixed native summaries are
inside the expanded session-summary disclosure. The archived runner is the actual
fixed-run source; baseline had an extra unused duration option and its initial
runner bytes were not pinned. Earlier two-hour runs remain separate and predate
this recovery-report fix. No new two-hour or leak-free verdict is claimed.

## Native HTTP notes, Builder and legacy editor

The12-row saved-file fixture receives a star and Unicode note on sequence1/query0.
The actual nested request-order button sorts12..1 while preserving selection and
notes. CtrlS saves to a new path and CtrlO reopens with equal IDs and selection;
Notes restores the star/text. Sort and detail tab reset on open. Down selects the
next request without borrowing its annotation; Up restores the original note.
Independent parsing compares every transaction field except star/comment and
pins both file hashes. Original and annotated files are still unchanged after
Builder sending and manual legacy inspection. This is a saved-file test, not the
complete live-annotation/columns/groups/search matrix.

Copy to Builder previously opened only its nested details while the outer file
tools stayed closed. e7a4debe reveals the outer tools; native amd64/386 both expose
the GET/query0 draft immediately. On amd64 an edited URL is explicitly sent to
the owned Docker loopback fixture, producing a separate1-row200 response while
the original12-row inspector remains on query0. No body/CA/trust/driver/dump policy
was changed. Two drag gestures did not establish draft replacement; drag stays
INCONCLUSIVE, with cancellation/error/multipart/reference combinations still open.

The inert manual base64 editor displays AMF0 available tree/text (12B, x=hi),
truncated AMF0 invalid (3B consumed/1B unverified tail), unsupported ViewState,
AMF3 limited (byte array exceeds64KiB), and ViewState pair (8B consumed/2B explicitly
unverified tail). It does not execute classes, decrypt or validate a MAC. These
are manual fixtures, not retained live bodies. legacy-fixtures.json records exact
bytes and observed states. Two AMF0 tree/text observations use accessibility
readback rather than an archived screenshot. Inspector internal scrolling exposes
inputs/actions, and focused splitter+Home restores the default height.

## Evidence and remaining gates

native-cases.json contains 25 observations: 19 selected PASS,
2 historic FAIL and 4 INCONCLUSIVE. No automatic
UI verdict is inferred from controller success. No fresh CSS geometry or OS-DPI
measurement was made; this run did not change scaling. The21-route reference,
area/rows/plot, conditional/error/help/keyboard, locale and actual-DPI matrix stays
open. Live body policy, transparent/proxy-ignoring/service reach, original COM
foreign-client/session-cookie semantics and clean-host registration are distinct
gates. Genuine675-entry HALOG codec and prior fixed2h capture slices remain done
in their documented scope. Longer/idle/resource-pressure and handle diagnosis
are not replaced by root memory guards. No Windows race run with CGO=0 is claimed.

Cleanup verifies descriptor-owned executables and the labeled --rm Docker fixture;
zero owned test windows remain. Other containers, user apps, COM registrations
and normal desktop stores are untouched by the harness. Private controller tokens,
PIDs, raw sessions and failure logs remain untracked. Public artifact hashes cover
exact exported bytes, with historical failures and ambiguities preserved.
