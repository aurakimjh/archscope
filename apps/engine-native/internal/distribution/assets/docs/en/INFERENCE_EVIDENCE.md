# Offline GPU and inference serving evidence

T-639 adds `inference_serving` 0.1.0 over the neutral InferenceEvidence 1.0.0
contract. The common AnalysisResult envelope and existing MetricHistory readers,
case persistence, generic providers and visual exports are reused. Source metrics
are retained **once** in `inference_core.metrics`; signals refer to series IDs,
and observation rows refer to point IDs. Unbound series remain in original
metric history without an inferred alias or a synthetic zero.

## Qualified scope

| Input | Implemented contract / evidence |
|---|---|
| NVIDIA query CSV | Explicit `nvidia-smi.query-csv.v1`, `csv,noheader,nounits`, exact fields below, declared producer version and host/boot. The bundled 535.104.05-layout data is synthetic. No NVIDIA tool, GPU, driver or native collection was executed; this validates this layout, not all outputs of that driver. XML, pmon/dmon and vendor error-log grammars are not registered. |
| Canonical MetricHistory + bindings | Existing T-638 source normalization first; supply an explicit manifest with entity/signal/context/link/runtime observations and no `metrics` member. Original metrics and raw identities remain unchanged. DCGM/serving metrics can be bound explicitly when their physical definitions are known; names never activate automatic aliases. |
| Canonical InferenceEvidence | Registered limited importer `inference.evidence-json`, alias `inference-evidence`, `{}` options. Strict bounded model validation. |
| vLLM example | Synthetic queue/running/cache/token/latency metrics with explicit scope, unit, statistical form, model/config and runtime observations. Names illustrate the pinned 0.8.5 documentation. **Q03 remains open:** this is no version-specific vLLM parser/alias qualification or live endpoint integration. |

NVIDIA query fields, in order:

```text
uuid,pci.bus_id,name,driver_version,utilization.gpu,utilization.memory,memory.used,memory.total,temperature.gpu,power.draw,clocks.sm
```

A capture JSON declares `contract`, `host_id`, `boot_id`, `producer_version`,
`query_fields` and `timestamps_ns`. Times are exact Unix-nanosecond strings,
null, or an empty list for entirely unknown time. A supplied list must match every
CSV row. File mtime is never a clock. Repeated device UUID/PCI mappings must agree.
The UUID is identity; local GPU index is display-only. `N/A`, `[Not Supported]`
and `[Insufficient Permissions]` remain distinct quality with no numeric value.
Raw decimal observations are retained. MiB→bytes and percent→ratio conversions
are exact and explicit; `utilization.memory` is memory-I/O utilization and is
separate from DCGM DRAM activity. Invalid physical ranges retain raw values and
show unavailable normalized values, without clamping or an inferred severity.

## Identity, signals and populations

Entities distinguish device, MIG, process lifetime, worker and replica. All need
stable declared host/boot IDs. Devices have an opaque stable device UUID (not a
universal NVIDIA-only format); MIG has its own instance ID and an explicit parent.
Process entities require PID namespace, PID and start time. Container/pod UID and
rank are retained. Links have source references and matching host/boot, and remain
**declared**, not verified process attribution. A worker can reference a shared
device, but device measurements are never copied into each worker or summed with
MIG children. One source series has at most one explicit signal binding.

A signal fixes semantic role, entity, context, boundary, population, statistical
form and conversion. Device used/capacity, framework allocated/reserved and engine
KV cache are separate. Instantaneous utilization/resource/queue signals require
gauge form. Counter, histogram, summary-quantile and precomputed-rate forms remain
distinct. Summary/precomputed forms require a declared source-calculation note;
that note does not authenticate a remote calculation. Unit conversions accept
only the documented physical unit pairs, never a name suffix.

A context preserves engine/version, model/revision, tokenizer, dtype/quantization,
TP/PP/DP, context/batch/sequence limits, cache/speculation/adapter configuration,
replica count, workload ID and warmup declaration. Missing parameters are not
invented. Warmup/startup/restart/replacement/recovery events remain visible; no
implicit request filtering or comparison-compatibility grant is made.

Runtime observations are **typed structured records**, not a free-form vendor log
parser. CUDA, framework, container and host OOM, Xid, ECC and device reset are
separate kinds. Codes do not certify a root cause or hardware failure. References
pin declared source hashes/positions; they do not prove that a raw vendor log was
interpreted correctly. Use the existing case source viewer to inspect original
files; no new verified-original reparse path is claimed for these declarations.
Prompt/completion/tool bodies have no field in the new event contract. Existing
MetricHistory resource/attribute provenance is preserved, so sanitize those source
files before import as well.

## Time and numeric rules

Original point values, timestamps, bucket bounds/counts and identities keep their
exact string representation. Counter intervals/rates reuse T-638's lifetime,
reset, overlap, missing/flag and gap semantics. A precomputed-rate gauge is not
rate-differenced again. A histogram interval's delta and rate are **observation
count** and **observations/second**; physical bucket-unit conversions never scale
those counts. Original/interval bucket arrays are available in the exact metric
query. No phase-percentile addition, derived histogram quantiles, cross-export
sum, device-to-request allocation, interpolation or causal bottleneck verdict.

Explicit request records keep client/server/gateway boundary, context, population
and outcome (success/failed/cancelled/incomplete). Observed token boundaries yield
`TTFT = first_token - request_start`; request TPOT is
`(last_token - first_token)/(output_tokens - 1)` only for at least two output tokens.
Missing or reversed boundaries produce null/status. Nonterminating TPOT is an
exact rational nanosecond string. One-token requests never receive a synthetic
zero TPOT. Network/SSE chunks have no automatic token interpretation. Aggregate
source metrics retain their producer definition even if it differs from these
explicit-request formulas. No request distribution is reconstructed from them.

## Desktop and exports

1. In Workspace choose a source and open **GPU / inference serving evidence**.
   Select NVIDIA CSV or a canonical MetricHistory file plus bindings JSON. Inspect,
   save a new model, and Analyze with `inference.evidence-json` and `{}`.
2. Provider tables show entities, signals, observations, configuration, mappings,
   runtime events and exact request boundaries/derived values. The event timeline
   uses only explicitly timed runtime records; metric samples stay metric history.
3. Open **Device / serving scope and metric drilldown**, choose an entity and one
   explicit signal, then inspect its exact original metric. Paging is bound to
   the parent result hash, signal selection and query cursor.
4. **Open metric history in Workspace** creates an owned immutable selected
   `metrics_timeseries` 0.2.0 result, preserving every point of that series for
   predecessor/reset context. Existing T-638 chart, histogram and selection paths
   apply. A canceled late creation releases its lease. Shared scopes are not added.
5. Generic JSON/CSV/HTML/Report Pack and selected provider-table visual artifacts
   reuse existing consumers. CSV remains summary/availability export; exact detailed
   values are available in saved JSON, retained HTML tables and original metric
   queries. No new specialized GPU raster renderer or native Office acceptance is
   claimed. Use explicit columns to include value/unit/status in visual tables.

## CLI

```sh
archscope-engine inference inspect-source --in gpu.csv --format nvidia-smi-csv --capture-file capture.json
archscope-engine inference normalize-source --in gpu.csv --format nvidia-smi-csv --capture-file capture.json --out model.json
archscope-engine inference normalize-source --in metric-model.json --format metric-bindings-json --bindings-file bindings.json --out serving-model.json
archscope-engine evidence import --in serving-model.json --importer inference.evidence-json --options '{}' --out result.json
archscope-engine inference query-metric --in result.json --signal-id signal-output_tokens --limit 25
archscope-engine inference open-metrics --in result.json --signal-id signal-output_tokens --out token-history.json
archscope-engine evidence export --in result.json --export-format report-pack --out report.zip
```

`import-source` combines normalization and result construction. New destinations
refuse overwrite/managed paths. Input/expanded source ≤64 MiB, model ≤32 MiB,
entities ≤1,024, links ≤4,096, signals ≤2,048, contexts ≤1,024, structured events
≤20,000; inherited metrics limits remain 100k points/10k series/500k total buckets.
Capture/bindings options ≤128 KiB (source provenance also inherits its 64 KiB
limit), source normalization admits two calls with five-minute limits, metric
pages ≤200. Result/case snapshots remain ≤64 MiB and refuse excess explicitly;
point count alone does not guarantee admission. Native GPU/vLLM, Wails/Windows,
clipboard/Office and hosted CI evidence remain separate. Evidence receipts:
`docs/review/evidence/2026-11-12-t639/`.

Primary definitions: [NVIDIA nvidia-smi](https://docs.nvidia.com/deploy/nvidia-smi/index.html),
[vLLM 0.8.5 metrics](https://docs.vllm.ai/en/v0.8.5/design/v1/metrics.html).
