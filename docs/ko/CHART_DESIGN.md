# 차트 설계

ArchScope chart는 표준화된 analysis result를 기반으로 하는 report-ready view이다.

## 기본 차트 유형

- Line Chart
- Bar Chart
- Stacked Bar Chart
- Horizontal Bar Chart
- Area Chart
- Heatmap
- Histogram
- Timeline Marker Chart
- Pie/Donut Chart, limited use

## 보고서용 차트 기능

- title editable
- subtitle editable
- axis label editable
- legend show/hide
- unit conversion
- time bucket 변경
- Top N 변경
- chart size preset
- PowerPoint 16:9 preset
- light/dark theme
- Korean/English label toggle
- PNG export
- SVG export
- HTML interactive export

## 기본 차트 템플릿

- `AccessLog.RequestCountTrend`
- `AccessLog.ResponseTimeTrend`
- `AccessLog.StatusCodeDistribution`
- `AccessLog.SlowUrlTopN`
- `GC.PauseTimeline`
- `GC.HeapUsageTrend`
- `Profiler.CpuWallBreakdown`
- `Profiler.TopStacks`
- `ThreadDump.ThreadStateDistribution`
- `Exception.ExceptionTrend`

## 현재 Skeleton

초기 desktop app은 TypeScript sample data에서 ECharts를 렌더링한다. Chart option builder는 React component와 분리되어 있으며, 향후 export logic에서도 같은 chart definition을 재사용할 수 있다.

## i18n 방향

차트 title, axis label, legend label은 UI locale과 report export locale을 기준으로 변환 가능해야 한다. Raw data value는 번역하지 않고, report-facing label만 locale resource에서 가져온다.
