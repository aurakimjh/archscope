# T-662 passive natural memory observation

The user authorized a read-only observation while Windows testing proceeds,
explicitly excluding forced memory load. T-662 remains **TODO / EVIDENCE_PARTIAL**.
This window does not supply the missing natural nonzero-swap/elevated-pressure
qualification or an independent review verdict.

## Observed window and values

On 2026-11-14 from 14:10:35.896802 to 14:15:36.281871 Asia/Seoul, collector 1.1.0
recorded 11 samples at approximately 30-second intervals (300.385 seconds).
The host is Apple Silicon/macOS 27.2.1 build 26B131, Ollama 0.40.16, 128 GiB capacity.
The intentional observation-only model selector neither loads nor unloads a
model. Selected and excluded model counts were both 0 at every sample.

| Measurement | Observed values |
|---|---|
| Swap total / used | Both exactly 0 bytes in all 11 collector samples |
| Pressure | Raw private sysctl code 1 in all 11 samples; no stable enum/ratio mapping |
| Independent OS reads | 13 readings: before, 11 during and after; all 0 swap bytes/code 1 |
| `memory_pressure -Q` | Literal reported free percentage 95% in all 13 queries |
| `vm_stat` compressor pages | 0 in all 13 reads |
| `vm_stat` swapins/swapouts/pageouts | All cumulative counters 0 in all 13 reads |
| Collector VM free bytes | 47,150,546,944–47,295,610,880; a separate VM category |

The pressure query percentage is preserved as reported. It is not treated as
the vm_stat free-RAM fraction or evidence of a stable pressure enum. Thermal,
GPU-utilization and power unavailability reasons remain unavailable, not zero.

## Independent decoding and readback

The existing collector uses ctypes `sysctlbyname` for exact xsw_usage bytes.
The independent reader instead invokes `sysctl -b vm.swapusage` and decodes
`=QQQII` with Python struct.unpack, plus separate pressure sysctl, vm_stat and
query-only `memory_pressure -Q` calls. The 11 paired swap/pressure observations
agree exactly. These readers share the kernel source and are sequential,
not atomic simultaneous snapshots; maximum paired start skew is 416.123 ms.

Existing CLI inspect, normalize, import and query all pass on the actual capture.
The normalized model has 14 series/154 points; imported type is metrics_timeseries.
All 33 selected swap-total/swap-used/pressure points retain the exact source
values 0/0/1. Normalized source SHA-256 matches the original capture bytes.
The existing CLI from the P2 validation was used; relevant CLI/parser Go sources
are unchanged between its ab42eeb8-based worktree and current 324a8d97.

## Scope and preservation

No stress test, benchmark, model load/generation, swap/pressure policy change,
sudo, powermetrics, process inventory or user-content collection was performed.
Ollama access was limited to fixed-loopback GET version/ps. The observation
process exited after this bounded window; there is no continuing background
monitor. Transient conditions between the 30-second probes may be missed, and
no causal attribution to a workload is claimed.

The prior 2026-11-13 capture remains unchanged. This new zero-swap window is real
evidence of the sampled natural state, while T-662 still requires qualifying
natural nonzero-swap/elevated-pressure values and their independent validation.
No force-load action is authorized or needed to manufacture such a sample.

See [local.json](local.json), [actual capture](actual-natural-capture.json),
[independent series](independent-series.json), [normalized model](normalized-model.json),
[analysis result](analysis-result.json) and [selected query](query.json).
[artifact-sha256.json](artifact-sha256.json) pins public file bytes. Evidence was
captured before publication. The user subsequently authorized committing and
pushing this observation and tracker update while retaining T-662 as TODO.
