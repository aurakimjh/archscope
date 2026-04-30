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
- `Profiler.FlamegraphDrilldown`
- `Profiler.ExecutionBreakdownDonut`
- `Profiler.ExecutionBreakdownBars`
- `ThreadDump.ThreadStateDistribution`
- `Exception.ExceptionTrend`

## Chart Template Factory

Phase 2 uses a template registry and chart factory entrypoint so Chart Studio can discover chart definitions before it owns editing and export workflows.

- `chartTemplates.ts` stores stable template IDs, title message keys, renderer support, dark-mode support, and export format support.
- `chartFactory.ts` maps template IDs to reusable ECharts option builders.
- React pages should request chart options through the factory rather than importing every option builder directly.

## Current Skeleton

The initial desktop app renders ECharts from sample JSON-style data in TypeScript. The chart option builders are isolated from React components so later export logic can reuse chart definitions.

## ECharts 6 Direction

Phase 2 adopted Apache ECharts 6 for the desktop chart surface after the Engine-UI Bridge PoC stabilized.

Relevant capabilities:

- dynamic theme switching and dark mode for analyst workflows,
- broken axis for highly skewed latency or GC pause distributions,
- violin, beeswarm, and range charts for response-time and profiler distributions,
- SVG rendering/export improvements for report generation,
- v5 compatibility theme if visual churn needs to be minimized.

Migration notes:

- Existing line, bar, horizontal bar, and donut options build successfully on ECharts 6.
- `ChartPanel` accepts `canvas` or `svg` renderer mode so SVG export work can reuse the same component path.
- `archscope` and `archscope-dark` themes are registered up front; dark-mode UI controls can select the theme later without changing chart option builders.
- Broken axis and custom distribution charts remain template-level capabilities to evaluate when GC pause and latency distribution analyzers produce the required series.
- Vite production build currently emits a large bundle warning; code splitting should be handled during Chart Studio/export expansion.

## i18n Direction

Chart titles, axis labels, and legend labels should be derived from locale resources. Raw metric values and evidence strings are not translated.

## Profiler Drill-down Charts

Profiler Analyzer now renders multi-stage flamegraph drill-down from the common `FlameNode` contract. Each stage keeps its own flamegraph, metrics, top stacks, top child frames, and execution breakdown.

The drill-down UI supports:

- breadcrumb navigation from `All` through applied filters;
- include/exclude text and regex filters;
- anywhere, ordered, and subtree match modes;
- preserve-full-path and re-root-at-matched-frame view modes;
- stage metrics for matched samples, estimated seconds, total ratio, parent-stage ratio, and elapsed ratio.

Execution breakdown is shown with a donut chart, a horizontal bar chart, and a category top stack table. It is recalculated from the selected drill-down stage.
