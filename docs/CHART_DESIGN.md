# Chart Design

ArchScope charts are report-ready views over normalized analysis results.

## Base Chart Types

- Line Chart
- Bar Chart
- Stacked Bar Chart
- Horizontal Bar Chart
- Area Chart
- Heatmap
- Histogram
- Timeline Marker Chart
- Pie/Donut Chart, limited use

## Report-ready Chart Features

- title editable
- subtitle editable
- axis label editable
- legend show/hide
- unit conversion
- time bucket changes
- Top N changes
- chart size preset
- PowerPoint 16:9 preset
- light/dark theme
- Korean/English label toggle
- PNG export
- SVG export
- HTML interactive export

## Default Chart Templates

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

## Current Skeleton

The initial desktop app renders ECharts from sample JSON-style data in TypeScript. The chart option builders are isolated from React components so later export logic can reuse chart definitions.
