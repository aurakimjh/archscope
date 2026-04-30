# Report Export Design

ArchScope should help architects move from diagnostic evidence to report artifacts.

## Export Targets

Initial design targets:

- JSON analysis results
- CSV tables
- PNG charts
- SVG charts
- HTML interactive chart bundles

Later phases may add:

- PowerPoint export
- Before/after comparison reports
- Executive summary documents
- Packaged evidence bundles

## Export Contract

Exporters consume normalized `AnalysisResult` structures. They should not parse raw logs directly.

```text
AnalysisResult -> Exporter -> Report Artifact
```

## JSON Export

JSON is the primary interchange format between the Python engine and desktop UI.

## CSV Export

CSV export should operate on `tables` entries or selected `series` entries.

## HTML Export

The first HTML report MVP is intentionally portable and static. It renders either:

- a normalized `AnalysisResult` JSON file, including summary, findings, diagnostics, series, tables, and chart data previews
- a parser debug JSON file, including redacted context, verdict, parser error groups, exception metadata, and hints

CLI:

```text
python -m archscope_engine.cli report html --input result.json --out report.html
```

Interactive chart bundles and full HTML flamegraph export remain separate follow-up work.

## Multilingual Export Direction

Report-facing text should be selected from locale resources for English and Korean. Exporters should not translate raw evidence or measured values.

## PowerPoint Direction

PowerPoint export is intentionally out of scope for Phase 1. Chart templates should still maintain 16:9 presets and title/subtitle metadata so future slide generation is straightforward.
