# Manual database and socket snapshots

T-645 qualifies a bounded first batch: PostgreSQL16.14 `pg_stat_database` and
iproute2-6.1.0 `ss -s` on Linux7.0.14-linuxkit. Both normalize through the existing
MetricHistory1.0.0 / `metrics.history-json` / `metrics_timeseries`0.2.0 path.
This does not qualify other DBs, report formats, kernels, APMs, profiles or serving
engines. No importer connects to a database, runs SQL/tools or starts collection.

## Actual inputs and source scope

| Format | Actual qualification | Retained observations |
|---|---|---|
| postgres-stat-database-json | PostgreSQL16.14/server_version_num160014, collector1.0.0; disposable network-none server and synthetic database | Five snapshots: unknown reset, reset A, load, reset B, load. Fourteen cumulative counters and one connected-backend gauge:15 series/75 points. |
| linux-ss-summary-json | iproute2-6.1.0, collector1.0.0, Linux7.0.14-linuxkit; isolated network-none namespace | Baseline, four owned loopback pairs, then owned endpoint cleanup. Established endpoints0/8/0;21 gauges/63 points. |

The bundled `system-db-*` corpus includes exact source/model/schema hashes,
producer provenance, read-only collection SQL and a loopback fixture generator.
The temporary PostgreSQL server was stopped/removed after acquisition. No model,
production data, DB query text, connection string, endpoint address/port or process
name is retained. Host/boot/instance/database/namespace tags are sanitized user
declarations, not authenticated identities. Existing saved models/results are
unchanged; no new result envelope or new generic importer is added.

## Collection and normalization

Use the separate `postgres-snapshot.sql` example with a manually operated psql
session: `BEGIN; SET LOCAL stats_fetch_consistency=snapshot;`, the read-only query,
then `COMMIT;`. Record the actual server version, `track_counts=true`, database OID
and sanitized context as the bundled `postgres-stat-database.v1` capture wrapper.
The capture declares exactly one database, excludes the shared OID0 row, and must
not be interpreted as the complete cluster. Sampling itself adds backend/transaction
activity. Test resets and writes were performed only on the disposable fixture DB;
normal collection does not reset statistics or change application data.

For Linux, manually run `LC_ALL=C ss -s` and record `time.time_ns()` before/after
each invocation, actual `ss -V`, kernel release and sanitized host/boot/network
namespace context in the `linux-ss-summary.v1` wrapper. Import accepts only the
qualified eight-line nonempty summary layout; arbitrary `ss`/netstat listings and
extra columns are refused. Collection requires neither packet capture nor a
privileged in-app collector. Other kernels/environments need additional fixtures.

```sh
archscope-engine metrics history-inspect --in capture.json
archscope-engine metrics history-normalize --in capture.json --out model.json
archscope-engine metrics history-import --in capture.json --out result.json
archscope-engine metrics history-query --in result.json --query '{"selection":{"series_ids":["metric-..."]},"limit":25}'
```

Auto detection uses the closed contract. Explicit formats are listed above.
Capture-owned producer/version/context cannot be overridden through query,
namespace or export-ID options. Existing destinations are refused. In Workspace,
open Metric history, choose the format (or auto), inspect, select exact series,
save a new model and Analyze. The source SHA from inspection must still match.
Existing exact history queries, common JSON/CSV/HTML/report packs, visual templates
and byte-exact saved cases remain available. Generic CSV retains summary/availability tables; HTML adds bounded exact interval
values/rates and bucket detail. Use history-query JSON or interval CSV for larger
selections. Merging sources does not establish shared clocks or comparable scopes.

## Counter and gauge meanings

PostgreSQL counter starts are the actual `stats_reset` timestamp, including
null/omitted unknown. Postmaster start is a process-lifetime identity, never a
substitute counter epoch: a clean restart may retain statistics with an older
reset. Unknown starts suppress derived rates; unannounced decreases remain
explicit refusals to derive. Known resets preserve the new epoch and report only
post-reset value/rate, not the missing earlier part. Read bounds are sequential
observation skew and counters update asynchronously; they are not query execution
intervals. `numbackends` is a current connected-backend gauge including the
collector, not active SQL or a connection-pool queue. Buffer hits/reads do not prove
OS-cache misses or physical disk I/O; no query percentiles/cache percentage are inferred.

Socket values are reported gauges, never cumulative packet/connection counters.
TCP total/closed/orphan accounting can include shared-kernel values outside the
declared namespace. The capture context does not prove all values are local.
Overlapping Total/INET/protocol/IPv4/IPv6 rows are not additive. FRAG entries use
fragment-queue units, not socket units; signed producer `closed` calculations are
retained. No connection throughput, drops, RTT, bandwidth, process attribution or
causality is derived. Sequential reads need not obey simultaneous accounting identities.

## Bounds and acceptance

Each raw capture is at most4MiB with1..256 strictly ordered nonoverlapping read
windows. Individual `ss` summaries are at most16KiB. Exact canonical uint64 values
and nanoseconds stay strings; no JavaScript float conversion. Existing model32MiB,
10k series/100k points, compact result32MiB and query1..200 rows/256KiB limits apply.
Malformed/missing values, unknown versions/fields, conflicting options and overlapping
windows refuse explicitly; no silent record truncation or zero substitution.

Tests include actual source/model/schema conformance, reset/unknown lifetime,
structural large-integer/negative-closed/lifetime separation, source pinning,
common result/saved readers and cancellation. Browser renderer verification uses
actual React/CLI DTOs with mock IPC and does not prove native WebView or Windows
runtime acceptance. Independent review remains a separate completion gate.

References: [PostgreSQL16 statistics](https://www.postgresql.org/docs/16/monitoring-stats.html),
[iproute2-6.1.0 summary implementation](https://github.com/iproute2/iproute2/blob/v6.1.0/misc/ss.c).

A never-reset database with `stats_reset=NULL` retains cumulative observations without delta/rate. The same postmaster lifetime does not prove statistics continuity across backend crash recovery, so no start is inferred. Keep cumulative-only evidence, or have the database owner separately approve a maintenance window and assess reset effects on other monitoring before collecting a new known epoch. There is no automatic reset. The separate `capture-postgres.py --out capture.json --host-id host-a --boot-id boot-a --instance-id db-a --database-tag sample --samples 3` uses libpq environment/pgpass and fixed SQL in `BEGIN READ ONLY`, assembling the capture wrapper. It excludes SQL/connection strings/database names from output; importing never connects to the DB. Collection itself affects observed backends/transactions.

New PostgreSQL imports use normalizer1.3.0. The first unknown→known transition retains its known current start and reports `epoch_transition_from_unknown` with null delta/rate. It assumes no continuity for the earlier unknown epoch; the next observation in the same known epoch can derive. Known→known resets keep post-reset derivation. Saved1.0/1.1/1.2 models retain their original status/calculation; explicit merge selects the newest input normalizer.

Captures require `numbackends≥1` because the collector is connected, and nondecreasing known `stats_reset` timestamps even across intervening unknown observations. Backwards resets refuse with `POSTGRES_CAPTURE_RESET_BACKWARDS`; zero backends refuse with `POSTGRES_CAPTURE_BACKENDS`. No timestamp/value correction is performed.
