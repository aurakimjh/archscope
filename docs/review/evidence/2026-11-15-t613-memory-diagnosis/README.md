# T-613 renderer retention diagnosis — 2026-11-15

Baseline30e5e078. T-613 remains PENDING and original independent CONDITIONAL stays
unchanged. Original failed import receipts are not rewritten or relabeled.

Historical role attribution places initial growth in renderer (+155.6/+154.8MiB)
while root Go memory falls. The historical saturated x64 renderer also grows
~31.15MiB, so initial history retention alone is not proof of every late fluctuation.

Matched isolated Docker/Node36-cycle probes run the actual compiled workspace
module with actual backend projections and explicit diagnostic GC. Both retain
80cards/121184logical rows. Before:80distinct transaction arrays/~516MiB heap.
Correction:3distinct arrays/4640unique rows/~54MiB heap. This is retained-object
attribution only; it is never substituted for an ordinary native memory screen.
Other per-result tables, metadata and separate history cards remain present.

The primary protocol was declared before baseline launch: ordinary production,
36cycles,28warmup,8measured,4096→512→32 rows,3s/open+15ssmall-file idle; unchanged
budgets and no forced GC/cache trim. Baseline and two fixed x64 runs are planned
sequentially. Windows process metrics, source hashes and native functional checks
are recorded after completion. Controller/profile/raw fixtures stay private.

## Exact shared transaction snapshots

OpenCaptureFile stamps SHA256 over the exact sanitized transaction projection
(count then each canonical JSON row), not filename/mtime or a second source read.
Only this explicit opt-in proof permits workspace sharing. History uses an
immutable clone; caller rows remain independent. Matching retained entries own
shared rows; WeakMap metadata has no separate strong result cache. Other results
and legacy unversioned calls retain their previous ownership. All80history cards,
per-analysis metadata/provenance and differing comments/order/payload stay distinct.
Go regression fails without fingerprints; state regression fails without sharing.
Same-size/same-mtime annotation changes and row count/order invalidate identity.
Full guarded app tests pass Windowsamd6440.224s/38643.409s; frontend state/build pass.
Original lifecycle memory tests are preserved; new import tests live separately.
Native repeat qualification remains a separate receipt, not implied by unit tests.

## Value equivalence and diagnostic-only native counters

Isolated Docker checker compares all result fields for108imports and all80retained
cards (including separate creation metadata/provenance/title). SHA256 over the
retained full-value results agrees before/after. Immutable rows remain3arrays;
clearing the registry releases every entry. This is semantic/ownership proof,
not an ordinary Windows memory screening result.

A VITE_MEMORY_DIAGNOSTICS=1 bundle plus existing layoutaudit build can expose
read-only entry/row/array/frozen counts to the passive observer. No payload/path,
second owner/cache, action API or forced GC is added. The getter is verified
absent from the normal production bundle. Diagnostic-only rapid native opens
will verify actual WebView adoption separately from the primary ordinary runs.
