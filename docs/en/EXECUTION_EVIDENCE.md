# Offline execution and comparison evidence

T-642 normalizes benchmark exports and bounded JVM crash text before import.
The limited `execution.evidence-json` importer (`execution-evidence`) uses the
existing `AnalysisResult` with `execution_evidence` 0.1.0/0.2.0 and `execution_core` 1.0.0/1.1.0. New generic model imports publish 0.2.0, including legacy 1.0.0 cores; saved 0.1.0 projections remain unchanged.
Its run conditions, statistic populations and crash fields require a domain model;
it does not introduce another envelope or change saved profile/runtime readers.
Parsers extract facts; the analyzer supplies common providers; UI/exporters consume them.

## Qualified source formats

| Format | Fixed evidence | Interpretation |
|---|---|---|
| k6-handle-summary-json | Actual k6 1.3.0 handleSummary JSON from a 2-VU, 1-second loopback workload | Selected HTTP counters/rate/trends, bytes, observed run duration. Time values are milliseconds normalized exactly to seconds. The failed-request Rate is a fraction, not requests/second. Whole-export aggregate; arbitrary tagged submetrics/scenario bodies are ignored and counted. |
| llmperf-summary-json | Upstream revision f1d6bed47e4501b0e371082b41601b59ab55269f, RESULTS_VERSION 2023-08-31 | Flattened LLMPerfResults JSON. Actual upstream metrics_summary/JSON functions executed with synthetic numeric input. No LLM, GPU, Ray client, tokenizer download or endpoint execution was used. |
| jmeter-jtl-csv | Synthetic JMeter 5.6.3 CSV layout | Required timeStamp/elapsed/label/success headers, explicit epoch milliseconds. Latency/Connect/allThreads/responseCode are optional. Label SHA separates request populations; no raw URL/message/thread-name retention. This is structural qualification, not actual JMeter execution. |
| hs-err-text | Sanitized fragment from an actual Temurin 21.0.10+7 JVM crash | Fatal signal, PID/native TID, JRE version, elapsed time when present, bounded problematic/native/Java frame symbols/modules. Not a core/heap dump engine or root-cause verdict. |
| execution-evidence-json | Bundled canonical model/schema | Strict offline transport; only `{}` importer options. One benchmark or one crash per model, never a sum of multiple captures. |

GenAI-Perf, nGrinder, Gatling and other producer versions remain input/fixture-gated.
LLMPerf qualification covers its pinned statistics/serialization path with floating
synthetic metrics, not the complete runner (whose numpy integer JSON behavior is
outside this qualification). Source revision, hashes and development dependency
versions are in the bundled producer provenance file. Tools are development-only;
the application never starts a load generator, benchmark client or crashed JVM.

Producer contracts: [k6 custom summary](https://grafana.com/docs/k6/latest/results-output/end-of-test/custom-summary/),
[pinned LLMPerf](https://github.com/ray-project/llmperf/tree/f1d6bed47e4501b0e371082b41601b59ab55269f),
[JMeter saved fields](https://jmeter.apache.org/usermanual/listeners.html).

## Collection conditions

Unzip the offline fixtures and start with `catalog/execution-evidence-k6-options.json`,
`execution-evidence-llmperf-options.json` or `execution-evidence-jmeter-options.json`.
The source UI accepts the same object without its `format` field. Crash options
are simply `{"format":"hs-err-text"}`. Normalization requires a context object for
benchmarks; unknown conditions can be imported for display but disable comparison.

Context preserves run/scenario IDs, workload and client/target environment hashes,
open/closed/unknown arrival model, arrival rate, concurrency, configured duration,
warmup duration, cold/steady/mixed/unknown phase, client/server/gateway boundary,
protocol and streaming condition. Configured duration differs from observed k6 run
duration, which remains a separate measured scalar. Export sample times/metrics are
not filtered by the declared phase: prepare a separately scoped source if excluding
warmup, failed requests or scenarios is required. No warmup subtraction is invented.

LLM context additionally preserves model/revision/tokenizer, declared input/output
token-distribution hashes and requested token mean/stddev. LLMPerf's numeric request
parameters/concurrency are copied from its export; contradictory sidecar values
are refused. Its pinned thread-loop arrival model is closed, not an open arrival
schedule. Sidecar hashes describe sanitized conditions; they do not authenticate
an experiment. The raw model name must agree when a known model is declared.

## Statistics and privacy

Each measurement retains semantic name, statistic, normalized/original value and
unit, population, group SHA, producer policy and source position. Exact values and
large counts stay strings across Go/JS; derived JTL means can be rational strings.
Counts differ from rates. Missing values remain null and observed zero remains zero.
`supporting_records` is the number of source rows inspected for a JTL statistic,
not the size of its failed/successful output population. It is unavailable for
source summaries that do not supply that information.

Small JTL (≤1,000 rows) preserves individual samples and computes count/error count plus nearest-rank
latency min/max/mean/p50/p90/p95/p99 per label over all rows. It does not average
exported percentiles, merge labels or infer throughput from first/last timestamps.
JTL first-byte and connect fields remain separate from request latency. Headers
must be unique and rows must have matching arity; unknown columns are ignored.

LLMPerf distributions use successful requests; started/completed/error counts keep
their separate populations. Whole-run output-token throughput differs from the
distribution of per-request output throughput. ITL preserves the pinned producer
formula and is not relabeled TPOT. Token distribution percentiles are token counts,
not time. Error messages/code-frequency strings and arbitrary metadata are excluded.

Raw benchmark/crash files may contain secrets. Normalize before case source copy or
indexing, and sanitize technical identities in the sidecar. This source path excludes
known body-bearing fields, not arbitrary sensitive strings or retrospective archives.
SHA label IDs are identity fingerprints, not a guarantee of anonymization. Crash
memory/register dumps, command lines, environment, arbitrary messages and full raw
report sections are excluded; missing END. remains partial_or_unknown. A marker
alone does not authenticate completeness, and a problematic frame is not a cause.

## Baseline compatibility and workflow

The active Workspace result is the ordered baseline; select another execution result
as target. Saved case snapshots can be reopened first, preserving exact bytes. This
uses `validated_normalized_snapshots_not_original_reimport` verification scope and
`declared` identity. It does not reuse the profile-specific, original-reimport case
baseline API or certify that the raw export was independently authenticated.

Comparison requires known equal workload/scenario/client/target, producer format and
version, arrival/configured duration/concurrency/warmup/phase/boundary/protocol/streaming
conditions. LLM model/token conditions must also be known and equal. Mixed warmup,
unknown fields, cross-tool policies and crash baselines produce `incompatible` with
reasons and no numeric difference rows. Context run ID can differ between captures;
wall-clock sample timestamps are not equated. There is no implicit environment mapping.

Only matching metric definitions (including unit, population, label SHA and statistic
policy) are differenced. Missing rows/values stay unavailable. Exact target-minus-base
and percent change are reported; a zero baseline has no percent change. Percentile
differences are observations, not merged distributions, statistical significance,
regression thresholds, causal attribution or automatic optimization advice.

Pages are bounded; cursors bind ordered result digests and selectors. Save a comparison
page JSON with both result/source digests, context fingerprint, policy and next cursor;
it is explicitly a page, not an export of all differences. Common summary/table/timeline,
JSON/CSV/HTML/report pack, visual tables and durable cases reuse existing consumers.

```sh
archscope-engine execution inspect-source --in summary.json --options-file options.json
archscope-engine execution normalize-source --in summary.json --options-file options.json --out model.json
archscope-engine evidence import --importer execution.evidence-json --in model.json --options '{}' --out result.json
archscope-engine execution import-source --in model.json --out result.json
archscope-engine execution query --in result.json --query '{"kind":"metrics","limit":25}'
archscope-engine execution compare --in baseline-result.json --target target-result.json --query '{"limit":25}' --out comparison-page.json
archscope-engine execution import-source --in hs_err_pid.txt --options '{"format":"hs-err-text"}' --out crash-result.json
```

Use either import command with a fresh output path. CLI destination files never
overwrite existing or managed case files. UI inspection pins the raw source digest
and rechecks it during normalization. Cancelled/stale operations cannot replace the
active result or retain late ownership. Query kinds are metrics/samples/distributions/contexts/
sources/crash/crash_frames; metric/group filters apply only to their relevant kinds.

## Bounds and acceptance

Raw/expanded input ≤64 MiB, canonical model ≤16 MiB, result ≤64 MiB, options ≤64 KiB.
Legacy saved sample arrays allow at most 50k entries; new JTL retains ≤1000 individual samples or ≤1M aggregated rows / 100k distinct bins. Other limits: 8192 metrics, 128 JTL label groups, 128 crash frames, 1 MiB text
lines and 500k crash lines. JSON decoder depth/record limits also apply. Byte ceilings
can precede record ceilings. Pages ≤100 rows (UI25); reduce original run/export scope (split-file percentiles cannot be combined) rather
than silently truncating. Two source slots, five-minute timeout and cancellation
checks apply. Saved file fingerprints/ownership are verified before semantic cache
reuse. The existing two-entry/two-minute/96 MiB JSON-charge cache is not an RSS bound.

Local tests, production builds and Windows cross-builds do not replace independent
review, native Windows/WebView or actual production producer captures. Q01–Q04,
Apple/PyTorch/Nsight and additional adapters retain their separate gates.

JTL row counts/errors/elapsed statistics are named samples/sample_errors/sample_latency with all_samples/failed_samples populations. JMeter rows may be transactions or non-HTTP samplers; they are never automatically relabeled HTTP requests. Replace every synthetic option ID/hash/value with actual sanitized collection conditions.

## R19-01: exact large JTL distributions

JTL up to 1,000 rows retains the existing 1.0.0 / 0.1.0 sample projection.
Larger JTL streams into exact per-label sorted latency→count distributions and
error counts in execution_core 1.1.0 / execution_evidence 0.2.0. Old saved 1.0.0 /
0.1.0 snapshots retain their exact projection and descriptor. Counts/min/max/mean
and nearest-rank p50/p90/p95/p99 are recomputed from distribution counts; no
sample duplication or percentile averaging. Large imports expose paged
`distributions` detail (one bucket per row); per-request timeline, first-byte,
connect, thread count and response code are intentionally absent with a limitation.
`aggregated_samples` counts underlying requests; `sample_records` counts retained
individual records (zero in compressed results).

Bounds: 1,000,000 JTL rows, 100,000 distinct (label, latency) bins total, 128
labels, original/expanded input 64 MiB, model 16 MiB and result 64 MiB. Whichever
bound is reached first applies. Row/bin errors include observed count, limit and
that split-file percentiles cannot be combined. Reduce the original run scope
when limits apply; this is not a promise of arbitrary-size aggregation. A 60,000
row / 3-label test checks exact independent expected counts/errors and all seven
latency statistics. Distribution/metric tampering and version relabeling are
refused. The 50,000 limit remains only for legacy saved sample-array validation.

R19-02 (actual hs_err elapsed/symbol/section), R19-03 (JTL consistency diagnostics)
and R19-04 (unverified JMeter version policy) remain explicitly queued; current
pinned producer restrictions continue. No new native Windows/WebView claim.
