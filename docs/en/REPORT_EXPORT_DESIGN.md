# Report Export Design

ArchScope should help architects move from diagnostic evidence to report artifacts.

## Export Targets

Current engine/UI targets:

- JSON analysis results
- CSV tables
- PNG charts
- SVG charts
- HTML static reports
- Before/after comparison reports
- Packaged evidence bundles
- PowerPoint export
- Report-pack HTML/ZIP bundles with customer summaries and provenance

The Wails Evidence Board and Report Pack flows provide a local report-oriented
export path: saved evidence cards can be exported as static HTML, JSON evidence
packs, or report-pack ZIP bundles. These exports consume persisted Analysis
Workspace results and derived `AnalysisResult` artifacts; they do not re-parse
source files during export.

## Export Contract

Exporters consume normalized `AnalysisResult` structures. They should not parse raw logs directly.

```text
AnalysisResult -> Exporter -> Report Artifact
```

## JSON Export

JSON is the primary interchange format between the Go engine CLI, Wails desktop
services, Analysis Workspace, and report/export workflows.

## CSV Export

CSV export should operate on `tables` entries or selected `series` entries.

## HTML Export

The first HTML report MVP is intentionally portable and static. It renders either:

- a normalized `AnalysisResult` JSON file, including summary, findings, diagnostics, series, tables, and chart data previews
- a parser debug JSON file, including redacted context, verdict, parser error groups, exception metadata, and hints

CLI:

```text
archscope-engine report html --in result.json --out report.html
```

Profiler result JSON now receives a static HTML flamegraph section when `charts.flamegraph`
is present. It is intended for report review and evidence sharing, not as a full
interactive flamegraph profiler UI.

## Evidence Board Export

Evidence Board export is a UI-level MVP around saved evidence cards. It does not
re-parse source files and does not require a server:

- HTML report: a static single-file summary of the collected evidence cards.
- JSON evidence pack: `type = archscope_evidence_pack`, schema version, export
  timestamp, card count, and the card payloads.
- Report pack HTML: a static report bundle view that combines Evidence Board
  cards with session-derived Incident Timeline, SLO, and Service Flow artifacts.
- Report pack ZIP: a browser-generated package containing `index.html`,
  `report-pack.json`, `customer-summary.json`, `ai-interpretation.json`,
  `evidence-cards.json`, `incident-timeline.json`, `slo-analysis.json`, and
  `service-flow.json`.
- Report pack provenance: `report-pack.json` preserves source result metadata,
  analyzer options, captured evidence references, deterministic findings,
  derived artifact references, and optional AI interpretation provenance when
  present.
- Customer summary: `report-pack.json` and `customer-summary.json` include an
  evidence-linked customer-facing overview and key observations. Every summary
  item lists evidence card IDs or source evidence references, and the raw
  evidence appendix remains in the HTML and JSON pack.
- AI interpretation: report packs include AI interpretation provenance and only
  include AI findings in customer-facing/report sections when the whole
  interpretation passes the evidence gate.

This is the short-term bridge between ad-hoc analyzer findings and the planned
engine-level report pack workflow.

## Before/After Diff

The first comparison report is a deterministic `comparison_report` `AnalysisResult`.
It compares numeric `summary` fields and finding counts:

```text
archscope-engine report diff --before before.json --after after.json --out diff.json
```

The diff command writes a `comparison_report` JSON file. Render it with the same
HTML exporter when a portable report is needed:

```text
archscope-engine report html --in diff.json --out diff.html
```

## PowerPoint Export

The minimal PowerPoint exporter writes a `.pptx` package directly from an
`AnalysisResult` JSON file:

```text
archscope-engine report pptx --in result.json --out report.pptx
```

The MVP includes title, source metadata, summary metrics, and findings slides.
Chart image embedding and slide-level theme editing remain later work.

## Multilingual Export Direction

Report-facing text should be selected from locale resources for English and Korean. Exporters should not translate raw evidence or measured values.

## CLI Summary

```text
archscope-engine report json --in result.json --out result.pretty.json
archscope-engine report csv --in result.json --out report.csv
archscope-engine report html --in result.json --out report.html
archscope-engine report pptx --in result.json --out report.pptx
archscope-engine report diff --before before.json --after after.json --out diff.json
```
