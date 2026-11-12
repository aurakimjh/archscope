# Exact metric history

T-638 adds `metrics_timeseries` payload 0.2.0 (saved 0.1.0 remains readable) and MetricHistory model
1.0.0 with explicit normalizer revisions 1.0.0–1.2.0. Legacy `metrics_snapshot` import and saved readers retain their semantics.
The common AnalysisResult envelope, parser/analyzer/exporter separation and
backend-owned exact values remain unchanged.

## Inputs and qualification

- Timestamped neutral snapshot sequences explicitly declare type, unit,
  monotonicity, temporality, attributes and optional start time.
- Prometheus HTTP API range **matrix** JSON requires supplied query provenance
  (`{"query":"...","step_ns":"1000000000"}`). Query results are gauges with
  unknown physical unit; names do not prove raw counter type. Query/step enter
  identity. Missing query metadata is refused. Native histogram matrix values
  are refused, not interpreted as OTLP exponential histograms.
- OTLP lowerCamelCase JSON preserves gauge/sum, explicit histogram and exponential
  histogram structures. Integer enum values, exact integer strings/numbers,
  resource/scope attributes, scope version and both schema URLs are retained.
  Unknown message fields are ignored; ambiguous oneofs, duplicate attribute
  keys, malformed buckets and summary points are refused. Binary OTLP is outside
  this slice. Finite histogram sum/min/max are supported; non-finite values there
  are explicitly refused. Scalar NaN/±Inf and no-recorded-value flags remain
  unavailable observations, not zero values.

The pinned synthetic fixture was actually collected with OpenTelemetry Python
SDK 1.38.0 and serialized with its official OTLP encoder and protobuf JSON
mapping. It covers cumulative counter, up/down counter, explicit and exponential
histograms, exact large integers and resource/scope identity. This qualifies that
fixture slice only. Q04 still gates the user's export route, instrumentation
version and GenAI aliases. No source publisher authentication is asserted.

References: [OTel metric model](https://opentelemetry.io/docs/specs/otel/metrics/data-model/),
[OTLP JSON](https://opentelemetry.io/docs/specs/otlp/#json-protobuf-encoding),
[Prometheus range API](https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries).

## Interval rules

All values, bucket counts, nanosecond times and deltas stay strings. Rates use
exact rational strings per second. Cumulative sums/histograms are differenced
only within the same start-time lifetime. The first cumulative point has no
inferred zero predecessor. An explicit new lifetime after the previous end can
produce a reset interval; an unexplained monotonic decrease, lifetime overlap,
conflicting end time, unknown start or missing predecessor produces no rate.
Non-monotonic sums retain signed changes. Delta points use their supplied value
once over their own interval. Every overlapping delta interval is unavailable.

Points are ordered by end time for calculation, with original positions and
reordering flags retained. Declared cadence gaps in sum/histogram intervals expose
an average across the whole interval without interpolation. Missing delta
intervals are disclosed as gaps. Range-query models at normalizer 1.1.0 and later use: `gap=true` when consecutive observed
end times differ by more than the declared positive `step_ns`. Values and statuses
(including unavailable/conflicting observations) are retained; no missing samples,
interpolation or gauge rates are invented. Selection windows retain predecessor
context. Saved normalizer 1.0.0 keeps its original gap interpretation; an explicit
merge retains the newest input normalizer. No cadence is invented for arbitrary cumulative exports. OTLP zero/absent
time means unavailable; neutral snapshots/range exports can retain Unix epoch
zero. Untimed points remain in detail and are excluded from timeline projection.

Explicit histogram boundaries/counts and exponential scale, signed offsets,
positive/negative counts, zero count/threshold remain distinct. Cumulative bucket
differences require matching schemas and nondecreasing count in every bucket;
changed exponential scale/threshold is refused for derivation, not silently
rebucketed. Interval min/max are never subtracted. Original histograms remain
available separately from derived buckets. No percentile averaging or quantile
estimation is performed.

A selection filters point **end times** in `[from_ns,to_ns)`. It retains entire
intervals, including an interval start outside the selection. It never clips or
rescales values. Series are never summed: units, resource/scope, attributes,
type/temporality, origin and query distinguish populations. Client/server and
OTLP/Prometheus copies remain separate. Exemplars preserve explicit IDs as
optional evidence, without inferred trace correlation.

## Provenance and bounds

Original SHA-256 covers compressed bytes when gzip is used. Normalization keeps
format coordinates and exact point references. Merge deduplicates only the same
source hash/position/payload, or a declared export ID bound to format,
producer/version/position and identical payload. Conflicting payloads remain
visible; different exports are not deduplicated by matching timestamps alone.
Original/source/result identity is declared, not a reimport-verified baseline.

Original and gzip-decoded inputs are bounded to 64 MiB (neutral snapshot JSON
validation currently also applies the 32 MiB model JSON ceiling). Models are
32 MiB, 128 source references, 10,000 series, 100,000 points, 4,096 buckets per
side and 500,000 total buckets. Explicit histograms allow 4,096 boundaries and
4,097 counts. Merges also cap expanded provenance references at 100,000. Result
and case objects remain 64 MiB. Original imports allow two concurrent operations
and a five-minute deadline. Common jobs retain their independent lower configured
budgets and use ≤256 KiB chunks, not a single oversized record. Queries allow
1–200 rows and 1 MiB; generic providers retain 100 rows / 256 KiB. Requests and
options are bounded; very large selections may require a smaller series set.
Payload 0.2.0 stores only explicitly selected series and all their points needed
for predecessor/overlap context, including points outside the selected window.
The result's series inventory is therefore limited to that retained subset;
reopen the original/model to select excluded series. Minimal point/status and
time/clock/reference tables avoid duplicating detailed interval/bucket rows.
Detailed queries calculate from the pinned model. CLI normalization/merge and
`history-import` write compact JSON, so accepted output can be read within the
same byte ceiling. Oversized results report measured bytes, point/series counts
and the action to reduce series or split the export; time filtering alone need
not shrink retained interval context. Limits fail explicitly without dropping
observations to fit. Synthetic scale evidence: 100,000 timed points/10 series,
model 27,670,574 bytes, result 58,573,608 bytes; 40,000 points/4 series result
23,424,243 bytes versus 5,857,892 bytes for one retained series. Bucket/attribute
size still matters; these measurements do not guarantee all 100k-point inputs fit.

## CLI and desktop

```
archscope-engine metrics history-inspect --in export.json --producer opentelemetry-python --producer-version 1.38.0
archscope-engine metrics history-normalize --in export.json --out model.json
archscope-engine evidence import --in model.json --importer metrics.history-json --options '{"series_ids":["metric-..."]}' --out result.json
archscope-engine metrics history-query --in result.json --query '{"selection":{"series_ids":["metric-..."]},"limit":25}'
archscope-engine metrics history-merge --inputs model-a.json,model-b.json --out merged.json
```

`metrics history-import` directly creates a result from an original, selecting
all retained series unless `--selection` is supplied. `--query` supplies range
provenance on original imports. Normalization/merge publish a new file and refuse
existing destinations and managed-store paths. No receiver or network fetch runs.

In Workspace, open **Metric history**, inspect the original, explicitly select
series, save a new model, then Analyze. The generic importer is
`metrics.history-json` (alias `metric-history`). **Exact metric intervals and
provenance** exposes identity, exact values/rates, original and derived buckets,
status, coordinates, windows and paged queries. **Add selection as new result**
creates a new owned result; late canceled results release their leases. Query
cursors bind result digest and selection/window/page identity. Saved case bytes
and JSON exports preserve their saved revision. Generic detail/timeline and
HTML/report-pack work without adding a product-specific result envelope.
Generic CSV currently exports summary/availability only, **not interval rows**;
use **Export interval CSV** or `metrics history-csv --in saved-result.json --out intervals.csv`
for the dedicated interval projection (CSV 1.0.0). `--selection` overrides the saved
selection explicitly; otherwise it exports the saved selection, across all pages.
Exact numeric strings, rational rates, observed/derived histogram JSON, statuses,
gaps, original coordinates, series identity and referenced source metadata are
retained. End-time windows keep whole intervals and their predecessor context.
Missing scalar fields are empty; absent histograms are JSON `null`. Free-text
name/unit formula prefixes are apostrophe-escaped; import numeric columns as text
in spreadsheets to preserve precision. Output is bounded to 64 MiB with an explicit
whole-export refusal; no truncation or overwrite of existing/managed files. Metric comparison/SLI
policy, automatic source alignment and native UI/Windows acceptance remain separate.

Exact decimal literals are limited to 1,024 characters, exponent ±128 and
128 fractional places; values outside these numeric bounds are explicitly refused.

Desktop series identities travel as opaque `identity_json` strings, keeping large
numeric attributes exact during JavaScript display. Selection uses backend IDs.

## Recorded follow-ups (R13)

The independent CONDITIONAL review permits closure after its P2 payload fix with
P3 work queued. R13-02 now supplies a process-local validated model/interval index
cache for repeated desktop queries, as described below. Numeric/shape
errors can lack point coordinates, and out-of-range exact values refuse the
whole file. Identical observations from different exports remain separate and
can receive `conflicting_timestamp`, even if values match. The original series
picker lists the bounded full inventory without search/pagination. These are
R13-04/05/07 follow-ups, not completed features. R13-03 and R13-06 limitations
are stated above. Native WebView/Windows and real user export acceptance remain
separate from local unit/CLI and synthetic SDK qualification.

R14-07 clarification: 0.2.0 generic detail/HTML/Report Pack tables retain point
IDs/status and time/clock/reference projections; they do not include metric
name/unit/value/delta/rate rows. Exact values and buckets are available through
the dedicated history query/explorer. A richer generic export projection remains
a follow-up with R13-06.

Apple Silicon / Ollama manual capture adapter: [APPLE_SILICON_EVIDENCE.md](APPLE_SILICON_EVIDENCE.md).


T-645 bounded actual DB/OS snapshot batch: [Manual database and socket snapshots](SYSTEM_DB_SNAPSHOTS.md). Existing MetricHistory transport, exact epochs/gauges and common consumers; other adapters remain fixture-gated.


## R13-02 verified warm query index

Desktop metric inventory/detail requests pin their result owner and re-read/hash
all stored bytes on **every request**, including warm hits. Identical file size or
restored mtime cannot bypass that check. A supplied digest and each cursor still
bind the immutable result/selection/window/page-size identity. Removed revisions
cannot be resurrected by cached data. Ordinary unowned results remain readable
until explicit cache cleanup, preserving the existing ownership contract.

A cold request validates the registered0.1.0/0.2.0 result and prepares an owned
interval index once. A warm page reuses that index, binary-searches selected series'
end-time ranges and copies only the bounded outgoing rows. Derived reset/gap/
previous-point context, exact fractions, histogram buckets, untimed filtering and
series/time ordering match the stateless query. Caller mutation of inventory,
selection, pointers, references or buckets cannot change the cached snapshot.
Selecting a new result retains the existing immutable materialization path.

The cache admits at most2 entries and192MiB of conservative structural charge
(twice the core JSON size plus owned interval/index accounting; **not measured RSS**).
Larger valid preparations run uncached. Expiry is fixed at2 minutes after admission,
not extended by hits; expired entries are purged on the next access, and shutdown
clears the cache. Byte/count limits evict least-recently-used entries. Cold preparation
is serialized with cancellable admission; errors/canceled preparations never enter
the cache. No cache is persisted or imported from a receipt. CLI invocations remain
stateless and keep their existing result bytes and query behavior.

Local regression evidence includes stateless/prepared byte-equivalent pages for
neutral/actual PostgreSQL/Linux/SDK histogram cases,100k-point warm reuse, byte
tampering with restored mtime, owner/cleanup, lazy expiry/byte eviction, concurrent
caller isolation, cancellation and unchanged legacy saved bytes. These are backend
checks, not native WebView/Windows or general latency/RSS acceptance. Remaining
R13-03/04/05/06/07 follow-ups retain their separate queues.

New source imports use normalizer 1.2.0. Equal-time cohorts whose complete
observation payloads agree (including start, flags, histogram and exemplars) are
`duplicate_observation`; any differing payload makes the entire cohort
`conflicting_timestamp`. Every export/ref is retained. Neither cohort nor its
next cumulative point produces a derived rate; no writer is selected. Overlapping
delta cohorts also retain the duplicate/conflict status with derivation suppressed.
Saved 1.0.0/1.1.0 results keep the original equal-time classification.
