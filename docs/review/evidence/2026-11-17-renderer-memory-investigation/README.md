# Renderer memory investigation — 2026-11-17

Status: LOCAL_ANALYSIS_COMPLETE_WINDOWS_REQUIRED. Batch task2 is incomplete.
This directory records new Mac/Docker executions and reproducible analysis of
received Windows evidence. It contains no new native Windows run.

[Analysis](analysis.json) recomputes role-private deltas and median pairwise
slopes from the original eight post-warmup samples at pinned acdd1d49. It checks
sample counts, the original budget, role sums and received metrics; normal and
optimized Python produce identical results. Leave-one-out sensitivity retains
the original cycle coordinates and is diagnostic only. The original 8-point
verdict and all failed receipts are preserved.

| Ordinary workload | Tree slope MiB/cycle | Renderer slope MiB/cycle | Original verdict |
|---|---:|---:|---|
| Unique amd64 | 0.052 | 0.172 | PASS |
| Body-rich amd64 | 4.801 | 5.213 | FAIL |
| Unique 386 | -0.393 | 0.022 | PASS |
| Body-rich 386 | 8.764 | 8.567 | FAIL |

The tree slope limit remains 4 MiB/cycle. Even omitting any one measured point
leaves each body-rich slope above that limit; this does not justify changing
the window or discarding samples. Growth is concentrated in the renderer.
The received instrumented run has bounded 80-card/80-array/10,240-row ownership,
a small isolate used-heap slope (0.004 MiB/cycle), and renderer-private slope
4.458 MiB/cycle. Process-private commitment and one isolate's used heap are
different accounting domains. Their difference cannot establish an allocator,
a native leak, a DOM leak, GPU allocation, or a specific retained reference.

## New local executions

All 216 exact synthetic ordinary input files were regenerated using the existing
`importMemoryDocuments` and published seed. Mac ran the actual backend
`TestQualifiedImportWorkloadProjection`; every input/projection/fingerprint
matches the received Windows amd64/386 and Linux records. No raw model payloads
or local absolute paths are included. [Projection receipt](mac-projections.json).

The current compiled `analysisWorkspace` ran 36 cycles for each scenario on Mac
and disconnected Docker. Results retain exactly 80 immutable arrays with
40,960 unique rows or 10,240 body-rich rows. Clear empties the registry and all
old IDs disappear. The post-Clear heap increments are:

| Host | Unique MiB | Body-rich MiB |
|---|---:|---:|
| Mac Node25.9.0 | 0.285 | 0.286 |
| Docker Linux Node22.23.2 | 0.219 | 0.236 |

The four full reports are alongside this README. These are explicit-GC object
ownership diagnostics, not ordinary renderer-private acceptance or T-662 natural
swap/pressure evidence. No production capacity, representation or budget was
changed because these measurements do not identify a cause-specific fix.
[Commands and source bindings](local.json).

To reproduce local ownership tests, create a fresh owned fixture directory with
`JSON.stringify(doc)` from each generated document in unique-then-body-rich
order and a manifest with 216 absolute paths. Run the qualification command in
local.json. Add a `manifest.json` in the generated `-models` directory with
cycles36, warmupCycles28, historyCapacity80 and scenario arrays of the 108 model
basenames (`.httpsession.json`) per workload. Compile current frontend state via
`npm run test:state`, then run:

```sh
node --expose-gc scripts/diagnose-workspace-retention.mjs \
  apps/engine-native/cmd/archscope-app/frontend/.tmp/state-tests/state/analysisWorkspace.js \
  <owned-model-directory> body-rich <new-receipt.json>
# Repeat for unique; run the same code/models in disconnected Node22 Docker.
```

## Windows collection prepared, execution pending

`scripts/collect-renderer-memory.mjs` reads only numeric CDP aggregates:
`Runtime.getHeapUsage`, `Memory.getDOMCounters`, `Performance.getMetrics`.
The client refuses all other methods, including forced GC, leak-preparation,
snapshots and script evaluation. The existing PowerShell process sampler is
reused to verify executable/root start identity, the owned loopback debug
listener's exact start identity, and exactly one app and renderer. The operator
must select one embedded Wails page target explicitly. Ambiguous ownership or
measurement failures fail; unsupported domains and optional fields stay unknown.

Each output record is bounded, excludes URLs/payloads/credentials and replaces
private PID/start identities with run-local ordinals. Output must be new. Counts,
sample spans, and process-private bytes can be correlated without retaining HTTP
bodies in the diagnostic output. Local mock-transport/ownership regressions pass
11 cases on both Mac and isolated Docker. PowerShell and the complete Windows
collector have not executed here; no native validity is claimed from those tests.

On an explicitly owned diagnostic app with an already established, verified
loopback debugging endpoint and private controller descriptor, run from the
repository root (Node22 or newer):

```powershell
node scripts/collect-renderer-memory.mjs --descriptor <private-controller.json> --exe <owned-app.exe> --port <owned-debug-port> --target-id <embedded-page-id> --commit <exact-build-commit> --out <new-private-counters.jsonl> --samples 240 --interval-ms 5000
```

Run the original body-rich protocol separately against that owned app and
correlate timestamps, reporting any unavailable CDP fields. Instrumentation may
change allocation behavior, so this diagnostic never replaces ordinary memory
acceptance. No registry, driver, dump/security policy change is part of this tool.
It does not collect allocation stacks. If numeric domains remain flat while
private bytes rise, appropriate Windows native allocator/graphics attribution
is still required before claiming a cause or choosing a production patch.

Then apply a cause-specific correction if established, and rerun ordinary
production x64/x86 unique/body-rich workloads with the original protocol and
budgets, without forced GC/trim/debug instrumentation. Preserve all historical
FAIL/PARTIAL receipts. Actual Windows execution access is required for this next
step; task2 and T-613 acceptance remain pending. The user subsequently authorized
publication now to enable Windows testing, superseding the earlier batch push
hold. Publishing these tools does not qualify a new native result.

Protocol field semantics follow the official
[V8 Runtime schema](https://raw.githubusercontent.com/ChromeDevTools/devtools-protocol/master/json/js_protocol.json)
and [browser Memory/Performance schema](https://raw.githubusercontent.com/ChromeDevTools/devtools-protocol/master/json/browser_protocol.json).
Runtime heap values describe the corresponding isolate. `getDOMCounters` is used;
`getDOMCountersForLeakDetection`, which prepares the renderer for leak detection,
is deliberately excluded.
