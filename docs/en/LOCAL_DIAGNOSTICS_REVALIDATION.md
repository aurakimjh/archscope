# Independent local diagnostics revalidation request

This is a request and reproducible input inventory, not an independent verdict.
The original comprehensive review remains CONDITIONAL. Use a fresh checkout of
the commit being reviewed and record its full SHA. Implementation-side passes
and synthetic corpus metadata do not establish PASS, native Windows/WebView,
CUDA/NCCL, actual new producer versions or deployment acceptance.

## Inputs and scope

The canonical reusable corpus is `diagnostics.parser-adversarial.v1` in the shared
asset index. A digest-pinned test-only mirror is under
`apps/engine-native/internal/modeltest/testdata/adversarial`. Verify every manifest
hash before replaying. Twelve models must refuse unsupported versions or broken
references. Fuzz corpus files use Go `go test fuzz v1` encoding and require bounded
refusal or a valid canonical saved-model readback; they are not all negative cases.
The four five-second fuzz runs are a short smoke, not an exhaustive campaign.

| Area | Independently check | Retained inputs / executable probes |
|---|---|---|
| R13 metric history | Exact integer overflow coordinates, known-epoch deltas 7 and 1, predecessor/reset semantics, 100k warm pages, owner/cache isolation, interval CSV and paged selection | `metric-signed-int64-overflow.otlp.json`, `metric-known-epochs.snapshots.json`; modeltest replay; analysis metric cache/history tests; metric CSV tests; frontend state suite |
| R14 report artifacts | Pinned diff sort/offset before pagination; more than five images refused; regenerate SVG against frozen view/spec/font even after an attacker recomputes hashes; actual ZIP/expanded byte limits; report presentations do not change canonical JSON | Analysis visual-view tests, visuals artifact/protection/version tests; generate fresh artifacts using the shipped pinned font rather than trusting a stored SVG hash |
| R15 perf/BPF | Upstream binary provenance distinct from exact imported text/gzip hash; declared ignored lifetimes retain raw records; waker headers do not invent observed entities; sleeping/runnable metrics stay separate | `system-perf-source.txt` and `system-imported-hash-mismatch.options.json`; systemtrace parser and analyzer R15 regressions |
| R17 token timing | First token before span requires null TTFT and TPOT under current policy; last-before-first, last-after-span and missing-boundary cases; legacy policy snapshots retain their previous projection | `llm-token-before-span.otlp.json` with existing neutral options; modeltest replay and `TestR1705ContradictingSuppliedTokenBoundsInvalidateBoth` |
| Direct model/SVG gaps | Six models' exact byte limits, unsupported fields/references/IDs, SVG XML escaping and whole-selection budget refusal | Each model's `TestDirectModelBoundaries`, modeltest corpus tests, exporters/gputrace SVG tests |

## Commands

Run from `apps/engine-native` in the reviewed checkout:

```sh
go test ./internal/modeltest ./internal/executionmodel ./internal/gputracemodel ./internal/inferencemodel ./internal/llmmodel ./internal/runtimemodel ./internal/systemmodel ./internal/exporters/gputrace -count=1
go test ./internal/analysis ./internal/visuals ./internal/parsers/metrichistory ./internal/parsers/systemtrace ./internal/analyzers/systemtrace ./internal/analyzers/traceimport -run 'Test(R13|R14|R15|R17)' -count=1 -timeout 10m
go test ./internal/exporters/metriccsv -count=1
```

Run each fuzz target independently with `-run '^$' -fuzz '^TARGET$' -fuzztime=30s
-parallel=2` in its parser package. Targets: execution/FuzzExecutionTextCSVJSON,
gputrace/FuzzGPUJSONSQLite, metrichistory/FuzzSocketSummaryText and
runtimeevidence/FuzzRuntimeThreadText. Retain any newly found failing input.
Frontend: run `npm run test:state` and `npm run build` from the frontend folder.

The reviewer should also calculate expected values without calling the same
normalizer/analyzer and attempt a new tamper variant. Separate source facts,
local tests, independent confirmation and unexecuted runtime scopes. Write the
review under `docs/review/`; a different AI processes it into work_status.md before
archiving. A request, test run or hash match must not be recorded as approval.
