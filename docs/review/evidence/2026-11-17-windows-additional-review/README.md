# Additional Windows evidence recheck — 2026-11-17

Task 1 of the user-authorized three-task batch. Reviewed 635512cf..acdd1d49,
eight commits/128 files. Existing uncommitted November16 review is included in
this review commit; unrelated HTTP preparation is excluded. No push until all
three tasks finish. No new confirmed product P1/P2; ordinary body-rich failures
remain a release-acceptance investigation, not a proven allocation defect.

The read-only audit pins acdd1d49 and passes ordinary/optimized Python equally:
111 exact artifact hashes and complete manifest, 28 native saved images, XML
axes/value/bar, PNG CRC/decompression/geometry and seven cross-architecture PNG
pairs. Four diagnostic source bindings retain original and Git hashes. The
initial x64 icon-bounds measurement and cancelled x86 attempt are not promoted.

All four ordinary memory screens reproduce original budgets and classifications.
Unique x64/x86 PASS; body-rich x64/x86 FAIL at 4.801/8.764MiB per cycle against
the unchanged 4MiB limit. The 216 projection records match all three hosts.
Instrumented last-eight process/JS matches have maximum lag5.328s and bounded
80cards/80arrays/10240rows. Small JS-heap slope and increasing renderer-private
bytes narrow investigation only; they do not identify a native allocator.

No received artifact is rewritten. No Windows private binary/raw workload or
fresh native run is represented as locally verified. The original ordinary
FAILs remain distinct from the instrumented run's passing tree screen.

Reproduce from repository root:

```sh
python3 docs/review/evidence/2026-11-17-windows-additional-review/audit_received.py /tmp/additional-audit.json
python3 -O docs/review/evidence/2026-11-17-windows-additional-review/audit_received.py /tmp/additional-audit-optimized.json
node --test scripts/test-import-memory-fixtures.mjs scripts/test-desktop-memory-checks.mjs
```

Task2 memory investigation and Task3 WREV3-01 remediation are committed
separately. Existing review documents remain pending a different reviewer;
no independent final PASS or numbered-task closure is asserted.
