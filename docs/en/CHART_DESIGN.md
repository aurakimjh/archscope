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

## ECharts 6 Direction

Phase 2 should evaluate and adopt Apache ECharts 6 once the Engine-UI Bridge PoC is stable.

Relevant capabilities:

- dynamic theme switching and dark mode for analyst workflows,
- broken axis for highly skewed latency or GC pause distributions,
- violin, beeswarm, and range charts for response-time and profiler distributions,
- SVG rendering/export improvements for report generation,
- v5 compatibility theme if visual churn needs to be minimized.

The upgrade should be treated as a chart-system task, not a one-line dependency bump. Chart snapshots and report export behavior should be checked after the migration.

## i18n Direction

Chart titles, axis labels, and legend labels should be derived from locale resources. Raw metric values and evidence strings are not translated.
