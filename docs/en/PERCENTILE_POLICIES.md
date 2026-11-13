# Percentile policies in legacy results

Newly computed legacy results disclose `metadata.percentile_method` (serialized
from metadata extras) with a versioned policy ID, algorithm and population.
This additive metadata does not change the payload or any calculated number.
Saved results without the field keep their original bytes and undisclosed policy.
Reanalyze originals to obtain explicit disclosure; reading an old result does not
assign it a new calculation method.

| Family | Calculation | Population and limits |
|---|---|---|
| Access log | Reservoir then linear interpolation | Known response times per aggregate; capacity 10,000, seed 12,345, Go PCG; above capacity an estimate |
| GC log | Linear interpolation | Parsed pause durations; half-even rounding to 3 decimals |
| HTTP capture | Linear interpolation | Retained valid entry durations; diff carries precomputed source summaries; old sources remain undisclosed |
| Trace import | Linear interpolation | Observed per-trace elapsed durations |
| Database slow query | Floor index | Parsed query durations under the input filter |
| Jennifer profile | Nearest rank | GUID-group and signature response-time observations |

Linear interpolation uses rank `(n-1)*p/100`. Floor index selects the value at
`floor((n-1)*p/100)`. Nearest rank selects `ceil(n*p/100)-1`, clamped to the sample.
For `[1,2,3,100]`, p95 is respectively **85.45**, **3**, and **100**. These are
intentional legacy differences. Reservoir interpolation uses the retained sample
and is exact for that sample, not necessarily for the full input above capacity.
Its PRNG sequence is not the Python implementation's sequence.

Do not average or add percentiles across populations, split files or time windows.
A matching statistic label does not establish matching measurement semantics.
Canonical execution evidence retains its own per-measurement statistical policy;
MetricHistory histograms retain buckets without an inferred quantile.
