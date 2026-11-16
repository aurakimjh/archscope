# Received Windows follow-up review — 2026-11-16

Reviewed `98eeaf3e..635512cf`: eight commits, 90 changed files. No new confirmed
product P1/P2 was found. One P3 verification-runner race is source-reproduced;
it does not invalidate the received completed native passes. This is a local
code/evidence recheck, not a new native Windows run or independent closure of
older Codex-authored CONDITIONAL reviews.

[audit_received.py](audit_received.py) reads the pinned Git commit without
rewriting received artifacts. [received-audit.json](received-audit.json) records:

- All 69 received artifacts match exact Git bytes and the manifest is complete.
- All 17 source records resolve: six exact Git-byte hashes and eleven exact
  Windows CRLF hashes. Both original and canonical Git digests are retained.
- Fourteen native saved CSVs preserve exact values/units/template/type/source;
  all seven x64/x86 file pairs are byte-identical. Native SVG-axis/Save records
  bind to the readback hashes; Canvas creation does not qualify all renderers.
- Six ribbon runs cover 180 locale/tab geometry checks and 18 theme selections.
  All 66 recorded selected-text contrast ratios independently recompute;
  minimum 4.9041878729. Actual OS scale is 100%; WebView125 is not OS125.
- Ordinary x86 36-cycle/108-open memory results reproduce original fixed budgets
  using eight measured points and seven stable process identity ordinals.
  Tree delta -192512B / slope 746934.8571428572B per cycle; root delta -8896512B /
  slope 59904B per cycle; handles/threads delta and slope zero.
- The 13 native case entries preserve two historical failures, ten passes and
  one partial native history-count observation. No exact80 native count is
  inferred from the partial view or substituted from headless ownership tests.

Windows binaries, source models and private test logs are unavailable here;
producer-side hashes/claims remain producer-side evidence. The arithmetic and
public saved outputs above are recomputed locally. Three headless workflow
receipts (3 cases/9 exports each), 219 projection-parity records, two preparatory
pack matrices and four Docker diagnostic-memory receipts retain their limited
scope. Ephemeral pack trust is not official signing or installer qualification.

## WREV3-01 — P3 — result file can be read before JSON publication completes

The new runner parses the receipt immediately and retries ENOENT only at
`scripts/verify-windows-ribbon.mjs:32`. Its actual producer uses non-atomic
`os.WriteFile` at `layout_audit.go:83`. A reader can observe the created/truncated
file before its complete JSON write. A valid later receipt then cannot prevent
the immediate SyntaxError. This is a possible false FAIL/flakiness path, not a
false PASS or a new application defect.

[reproduce_receipt_read.mjs](reproduce_receipt_read.mjs) executes the actual
runner source with controlled Windows process/file I/O in a VM. Complete and
ENOENT-then-complete controls pass; partial-then-complete fails on read1 before
the complete read is offered. [reproduction.json](reproduction.json) preserves
the result and source digest. No actual app/process/window is launched.

Follow-up under T-663: publish the receipt atomically, or distinguish an
in-progress receipt from terminal invalid JSON with bounded retry. Verify
complete, initially missing, partial-to-complete, permanently malformed,
process-exit and timeout paths. Keep genuine failed reports as failures.
The product and runner are not modified in this review.

## Local validation and release scope

[local-validation.json](local-validation.json) records full affected stream/app
race, 100 targeted virtual-timer race repeats, affected vet, frontend state/build,
normal/optimized UTF-8 coverage (2197 phrases per added locale), resource checks
and normal/optimized receipt-audit parity. Existing SDK link warnings do not
establish minimum-macOS runtime acceptance. Two representative received images
were inspected for selected dark-theme readability and partial history scope.

Current open registry: 251 tasks = 243DONE/5REVIEW/1PENDING/2TODO. T-601, T-627,
T-663, T-664 and T-611 remain REVIEW; T-613 PENDING; T-654/T-662 TODO. WREV3-01 is a
follow-up within T-663, not a new numbered task. The tracker additionally lists
mandatory official pack compatibility/signing/migration, supported-scope
acceptance dispositions and final version/packaging/notes/CI gates.

Reproduce from repository root:

```sh
python3 docs/review/evidence/2026-11-16-windows-review/audit_received.py /tmp/windows-recheck.json
python3 -O docs/review/evidence/2026-11-16-windows-review/audit_received.py /tmp/windows-recheck-optimized.json
node docs/review/evidence/2026-11-16-windows-review/reproduce_receipt_read.mjs /tmp/windows-receipt-race.json
```
