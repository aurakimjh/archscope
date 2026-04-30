# 보고서 Export 설계

ArchScope는 진단 근거를 보고서 artifact로 전환하는 과정을 지원해야 한다.

## Export 대상

초기 설계 대상:

- JSON analysis results
- CSV tables
- PNG charts
- SVG charts
- HTML interactive chart bundles

향후 추가 대상:

- PowerPoint export
- Before/after comparison reports
- Executive summary documents
- Packaged evidence bundles

## Export Contract

Exporter는 표준화된 `AnalysisResult`를 입력으로 사용한다. Raw log를 직접 parse하지 않는다.

```text
AnalysisResult -> Exporter -> Report Artifact
```

## JSON Export

JSON은 Python engine과 desktop UI 사이의 기본 interchange format이다.

## CSV Export

CSV export는 `tables` entry 또는 선택된 `series` entry를 대상으로 한다.

## HTML Export

첫 HTML report MVP는 portable static HTML을 생성한다. 입력은 다음 두 가지를 지원한다.

- summary, findings, diagnostics, series, tables, chart data preview를 포함한 normalized `AnalysisResult` JSON
- redacted context, verdict, parser error group, exception metadata, hint를 포함한 parser debug JSON

CLI:

```text
python -m archscope_engine.cli report html --input result.json --out report.html
```

`charts.flamegraph`가 있는 profiler result JSON은 static HTML flamegraph section으로 렌더링한다. 이는 report review와 evidence sharing을 위한 기능이며, profiler UI 수준의 full interactive flamegraph는 아니다.

## Before/After Diff

첫 comparison report는 deterministic `comparison_report` `AnalysisResult`다. Numeric `summary` field와 finding count를 비교한다.

```text
python -m archscope_engine.cli report diff --before before.json --after after.json --out diff.json
```

`--html-out`을 사용하면 같은 static HTML report exporter로 comparison HTML도 생성한다.

## PowerPoint Export

Minimal PowerPoint exporter는 `AnalysisResult` JSON에서 `.pptx` package를 직접 생성한다.

```text
python -m archscope_engine.cli report pptx --input result.json --out report.pptx
```

MVP는 title, source metadata, summary metrics, findings slide를 포함한다. Chart image embedding과 slide-level theme editing은 후속 작업으로 남긴다.

## 다국어 Export 방향

Report-facing text는 locale resource를 기준으로 English/Korean label을 선택해야 한다. Exported data value와 raw evidence는 변경하지 않는다.

## PowerPoint 방향

PowerPoint export는 Phase 1 범위가 아니다. 다만 chart template은 16:9 preset과 title/subtitle metadata를 유지하여 향후 slide generation을 쉽게 만든다.
