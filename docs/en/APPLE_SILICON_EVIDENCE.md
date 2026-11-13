# Apple Silicon offline memory evidence

Apple Silicon evidence adds `apple-ollama-capture-json` to Metric History source assistance and
`metrics history-inspect/history-normalize/history-import`. Auto recognizes the
explicit `apple-ollama-capture.v1` contract. The existing MetricHistory 1.0.0 and
`metrics_timeseries` 0.2.0 AnalysisResult provide queries, charts, JSON/CSV/HTML,
report packs and exact case snapshots. No NVIDIA UUID, MIG, Xid, PCI address or
dedicated VRAM is required. No parser/analyzer starts a collector or API call.

## First qualified environment

Q02 is selected **for this bounded fixture** as Ollama 0.40.16, macOS 27.2.1
(26B131), Mac17,6 / Apple M5 Max. This is an implementation qualification target,
not an assumption about the user's production engine. An existing gemma:2b Q4_0
model, context 512, generated 8 synthetic tokens; three actual snapshots followed.
The test-loaded model was unloaded afterwards. No model download was needed.
Other macOS versions are recorded as declared metadata; they have no native
qualification claim. Other Ollama versions, MLX, PyTorch MPS and llama.cpp/Metal
need versioned actual fixtures before a supported mapping is added.

## Manual collection and import

On Apple Silicon, with the intended model already loaded, explicitly run:

```sh
python3 scripts/capture-apple-ollama.py --out apple-capture.json --host-id local-host-tag --boot-id unique-os-boot-tag --model gemma:2b --samples 3
archscope-engine metrics history-inspect --in apple-capture.json
archscope-engine metrics history-normalize --in apple-capture.json --out apple-model.json
archscope-engine metrics history-import --in apple-capture.json --out apple-result.json
```

The helper is a separate, user-run development/manual utility. It only reads
fixed loopback `/api/version` and `/api/ps`, sysctl, vm_stat and `pmset -g therm`.
It never loads a model, generates content, downloads, invokes sudo/powermetrics
or runs memory_pressure. Host/boot tags are sanitized declarations, unique per
host/OS boot; they are not hardware serials or authenticated identities.
Only the exact selected model's allowed metadata survives API filtering. Review
technical model identifiers before sharing; no arbitrary text sanitization is
promised. Existing output is refused. macOS/Python/Ollama are manual collection
prerequisites, never offline importer dependencies. In the desktop, select
Metric History → `apple-ollama-capture-json` (or auto), inspect, select exact
series, save the new model and Analyze. Saved model files cannot be auto-parsed
as raw capture files; use the `metrics.history-json` importer for models.

## Meaning and availability

- `hw.memsize` is system-wide shared memory capacity. VM categories are page
  counts multiplied by the observed page size. They overlap: no category sum,
  inferred free GPU bytes, device-memory ratio or OOM threshold is calculated.
- Swap uses Darwin `sysctlbyname(vm.swapusage)` byte fields, avoiding precision
  loss from rounded `sysctl` display suffixes. Used bytes cannot exceed total.
- `size` / `size_vram` remain producer-reported **per-model residency estimates**.
  They are not a dedicated VRAM pool, GPU utilization, efficiency or proof of
  workload attribution. Multiple models remain separate; no cross-model sum.
- Pressure retains the private sysctl integer code without enum/ratio mapping.
  Available pmset thermal warning codes are warning levels, not temperatures.
  “No thermal warning level has been recorded” stays unavailable, never normal 0.
- GPU utilization and power are unavailable in this contract. Null values carry
  `not_recorded`, `not_collected`, `permission_required` or `unsupported` and
  normalize to `no_recorded_value`, not measured zeros. Numeric GPU/power fields
  are refused until a separate producer-qualified mapping exists.
- Capture start/end Unix nanoseconds describe sequential reads and skew, not
  simultaneous measurements or an aggregation interval. System pressure and
  swap are context alongside model estimates, never causal diagnosis.

Identity includes host, boot, local device, hardware, OS/build, collector and
engine versions, memory model, metric scope and per-model digest/quantization/
context length. Values/times remain exact uint64 strings, including values above
2^53. Original file SHA-256 (compressed bytes for gzip), source positions and
collection provenance survive normalization. Availability reason changes create
separate series. No synthetic interpolation or mtime clock is introduced.

Bounds: 4 MiB capture, 1–256 increasing nonoverlapping snapshot windows, 0–64
models per snapshot; duplicate model digest/unknown fields/duplicate JSON keys,
noncanonical or overflowing integers, missing availability reasons and identity
or producer overrides are refused. Existing source leases, 5-minute jobs,
2-slot admission, cancellation, immutable source pinning, no-overwrite model
publication and backend-owned exact query values remain in force.

Sources: [Ollama API ps](https://docs.ollama.com/api/ps),
[pinned API response mapping](https://github.com/ollama/ollama/blob/v0.40.16/server/routes.go),
[pinned scheduler memory reporting](https://github.com/ollama/ollama/blob/v0.40.16/server/sched.go),
[Darwin sysctl layout](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/sysctl.h),
[private pressure query](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_memorystatus_notify.c).

collector1.1.0 adds model_selection_sha256 and required per-snapshot excluded_model_count / selected_model_match_count (0 or 1). Excluded model names/digests stay absent. Selected zero is distinct from an empty host. Actual multi-load/missing-selector/empty captures cover 1/1, 0/2 and 0/0. Older1.0 captures retain canonical normalization and saved projections; inspection adds a read-time unknown-coverage notice. Nonzero swap/elevated pressure and other environments remain unqualified.

New captures require each VM category ≤ physical capacity, size_vram ≤ size and swap_used ≤ total; categories are never summed. Host/boot/device use sanitized tags, text excludes control characters and model digests must be lowercase hex. Invalid new inputs are refused; saved identities are never rewritten.

An additional actual multi-load capture has 12 samples at two-second spacing, with one selected and one excluded model; both owned models were unloaded and the empty state verified afterwards. swap_used=0 and raw pressure code=1 do not establish elevated pressure. Actual nonzero swap/elevated pressure, other producer/platform combinations and long-running workloads remain unqualified; obtain authorized natural observations without forced memory stress.

Qualified versions remain the defaults. For explicit structural-only opt-in, actual/unknown version preservation and qualification limits, see [Unverified producer versions](INPUT_ASSISTANCE.md#explicit-unverified-producer-versions).
