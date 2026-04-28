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

HTML export는 chart option과 normalized data를 포함하여 desktop app 없이도 interactive review가 가능해야 한다.

## 다국어 Export 방향

Report-facing text는 locale resource를 기준으로 English/Korean label을 선택해야 한다. Exported data value와 raw evidence는 변경하지 않는다.

## PowerPoint 방향

PowerPoint export는 Phase 1 범위가 아니다. 다만 chart template은 16:9 preset과 title/subtitle metadata를 유지하여 향후 slide generation을 쉽게 만든다.
