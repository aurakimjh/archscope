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
