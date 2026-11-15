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
budgets and no forced GC/cache trim. Baseline and two fixed x64 runs completed
sequentially and passed. Windows process metrics, source hashes and native functional checks
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
absent from the normal production bundle. Diagnostic-only108rapid native opens
verified actual WebView adoption separately from the primary ordinary runs.


## Completed ordinary Windows screens

All three predeclared runs completed with the original36/28/8 protocol and
unchanged budgets. Independent verifier checks every fixture hash, open count,
selected ID, process identity, metric sum, median delta and Theil–Sen slope.
Both fixed profiles use the identical ordinary production d25581d4 binary.

| Run | Seconds | Final-window tree median MiB | Tree delta MiB | Tree slope MiB/cycle | Verdict |
| --- | ---: | ---: | ---: | ---: | --- |
| baseline-amd64 | 927.6 | 838.277 | -32.645 | 0.622 | PASS |
| fixed-amd64-1 | 932.9 | 572.941 | 16.820 | 2.216 | PASS |
| fixed-amd64-2 | 931.9 | 558.758 | 6.344 | 0.791 | PASS |

Tree limits remain64MiB delta/4MiB per cycle; root16/1MiB, handle24/2 and
thread4/0.5 budgets also pass. Each fixed screen performs108opens in ~15.5minutes.
No forced GC, pressure injection or cache/working-set trimming is used.
The current baseline also PASSES. Earlier10/36-cycle failures remain unchanged;
this comparison does not demonstrate that every historical late-growth cause
has been reproduced or eliminated. The interactive host is shared with other
apps/user activity. No x86 native memory screen is newly qualified.

After measurement, actual native UI still shows32/32 transactions/first row,
checked star and the exact Hangul note in both fixed profiles (five paired
screenshot/accessibility receipts). These are read-only bookends, not IME/edit
or new DPI/layout qualification. Docker semantic comparison additionally checks
all full retained result values and metadata across108imports, not counts alone.

## Actual renderer ownership and remaining attribution

Diagnostic-only WebView counters independently observe80cards,121184logical
rows,3frozen arrays/4640unique rows after108opens. A later passive idle observation
reports JS heap29,604,516B versus renderer OS private416,047,104B. Chromium heap
metrics may be rounded; OS private includes other allocations. The difference
has not been conclusively attributed to cache, fragmentation or a leak. This
diagnostic binary/timing is excluded from ordinary memory-budget verdicts.

Full guarded app tests pass x64/x86 and final ordinary79fa117c builds pass both
architectures with the diagnostic getter absent. All owned apps and labeled
containers are stopped; seven preexisting containers and private profiles remain.
T-613 stays PENDING, T-627 REVIEW and independent CONDITIONAL. See
remaining-acceptance.json for the concrete remaining acceptance gates, including
unique/body-rich/longer memory workloads and original client/IME/driver/host limits.
