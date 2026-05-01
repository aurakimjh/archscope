# Demo-Site Data Workflow

ArchScope can run the shared demo-site manifests stored in
`../projects-assets/test-data/demo-site`. The data remains outside this repository;
ArchScope reads the manifests and writes generated report bundles locally.

## CLI

Run all demo-site scenarios:

```bash
./scripts/run-demo-site-data.sh
```

Run a selected scenario:

```bash
python -m archscope_engine.cli demo-site run \
  --manifest-root ../projects-assets/test-data/demo-site \
  --data-source synthetic \
  --scenario gc-pressure \
  --out /tmp/archscope-demo-bundles
```

Output layout:

```text
<out>/
  index.html
  synthetic/<scenario>/
    index.html
    run-summary.json
    *-<analyzer_type>.json
    *-<analyzer_type>.html
    *-<analyzer_type>.pptx
    normal-baseline-vs-<analyzer_type>.json
    normal-baseline-vs-<analyzer_type>.html
  real/<scenario>/
    ...
```

`run-summary.json` records analyzer outputs, failed analyzers, skipped line
counts, reference-only correlation files, key metrics, findings, and generated
comparison reports. Scenario `index.html` renders the same information as a
portable report.

## Analyzer Type Mapping

The canonical demo-site manifest mapping lives in:

```text
../projects-assets/test-data/demo-site/analyzer_type_mapping.json
```

ArchScope reads this JSON when running demo-site manifests; command rendering is
not duplicated in the engine. To inspect the active mapping:

```bash
python -m archscope_engine.cli demo-site mapping \
  --manifest-root ../projects-assets/test-data/demo-site
```

Examples:

| manifest `analyzer_type` | CLI command |
|---|---|
| `access_log` | `access-log analyze` |
| `profiler_collapsed` | `profiler analyze-collapsed` |
| `profiler_collapsed` + `format: jennifer_csv` | `profiler analyze-jennifer-csv` |
| `jfr_recording` | `jfr analyze-json` |
| `otel_logs` | `otel analyze` |

## Desktop UI

Demo Data Center supports:

- selecting the demo manifest root
- filtering by `real`, `synthetic`, or all data
- running one scenario or all visible scenarios
- opening generated JSON/HTML/PPTX/index files
- sending JSON outputs to Export Center
- showing failed analyzer, skipped line, finding, and reference-only context summaries

Demo-site runs use a longer Electron engine timeout than single-file analyzer
actions so full scenario runs have room to finish. The UI remains in a running
state until the engine returns; streaming progress events are still deferred.

The desktop package includes a Playwright/Electron smoke test for the Demo Data
Center. It runs with an `ARCHSCOPE_E2E_DEMO_STUB=1` Electron main-process fixture
so CI can verify navigation, run-result rendering, and Export Center handoff
without depending on external demo-site files.

## OpenTelemetry Scenario Checks

OTel analysis now prefers parent-span relationships when `parent_span_id` or
compatible aliases are present. When parent span data is missing, it falls back
to timestamp ordering. Demo manifest expectations for services and trace counts
are copied into result metadata and mismatches are emitted as findings.
