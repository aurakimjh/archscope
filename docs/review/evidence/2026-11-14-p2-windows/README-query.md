# WR-01 native Windows verification

Fast-forward pull brought `324a8d97`, which already fixes both P2 findings from
the original CONDITIONAL review. This commit adds Windows execution evidence;
no additional WR-01 product correction was needed.

The actual production Windows amd64 and386 executables use source commit324a8d97.
Both PE machine types and executable hashes are checked. Each app runs alone
with a new isolated capture/WebView profile, one real Node producer attached by
PID/start identity, and four requests to the owned Docker loopback origin.
Bodies stay disabled. Every transaction is200/complete/confirmed, and the live
transaction's separate query field is empty: these observations exercise the
retained-URL fallback that the earlier Mac HAR test did not exercise natively.

Both native apps show the complete Path
`/lookup?next=/detail?id=7&keep=2`. Path contains `id=7&keep=2` selects exactly
request2 of4. Query shows `next=/detail?id=7` and `keep=2`; Raw contains the complete
GET target, agreeing with Path. Filtering `/lookup` gives two requests: ascending
Path order is request1(id3),request2(id7), and descending reverses them while
preserving request2's selected Query detail.

386 additionally verifies Path filtering by literal `%3F%26` and the target
`/repeat?value=??tail?&raw=%3F%26&raw=%3f`. Query preserves `??tail?`, two `raw`
parameters decoded as `?&` and `?`, and their encoded source spelling/case. Raw
keeps all question marks, duplicate keys and percent spelling.

There are15 selected native PASS observations. One386 ascending-sort screenshot
was occluded by an unrelated user window and excluded; native accessibility order
establishes that observation. The later descending screenshot is unobstructed.
Only the owned ArchScope app was controlled. This is not a full layout or DPI
verdict. Fragment boundaries, malformed percent escapes and explicit-query
precedence are covered by the Windows state regressions, without claiming that
HTTP transmitted a fragment.

Both captures stop finalized with4captured=4persisted and zero drops. Independent
readback compares all four IDs/URLs and every other field in each portable export,
accounting explicitly for portable host/query normalization. Selected request2 is
retained. Raw controller credentials, IDs, sessions and process inventories remain
private; public receipts contain synthetic URLs, counts and hashes. Screenshots
can show the owned synthetic producer's PID/start label.

Windows frontend state/build pass. Full production app tests pass in38.091s
amd64/42.259s386, and all24 capture packages pass on each architecture. The used
query generator and independent readback script are archived with their private
workstation paths as receipts. `query-source-hashes.json` separates executed
worktree bytes from Git blob bytes to make Windows line endings explicit.

The original review remains CONDITIONAL and in its original location for
independent handling. T-613 stays PENDING/T-627 REVIEW; full reference/locale/DPI,
body/vendor/driver/service and long-memory gates remain. WR-02's fresh Windows
runner results are documented separately in this folder after completion. Earlier
Mac, Linux and two-hour receipts are unchanged.
