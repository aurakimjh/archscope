# Chart Studio Design

Chart Studio will build on the Phase 2 chart template registry rather than bypassing it.

## Factory Data Contract

The chart factory now accepts dashboard fixture data and real analyzer result types. Chart Studio should keep this model and add analyzer-specific type narrowing instead of passing arbitrary JSON directly into option builders.

Near-term data sources:

- dashboard sample fixtures
- `access_log` results
- `profiler_collapsed` results
- future GC/thread/exception results after their contracts are defined

## Option Persistence

Persisted chart settings should store user intent, not a full generated ECharts option blob.

Recommended shape:

```text
{
  template_id: string,
  title_override: string | null,
  renderer: "canvas" | "svg",
  theme: "archscope" | "archscope-dark",
  option_overrides: object
}
```

At render time:

1. Load the base option from the factory.
2. Apply small validated overrides with a deep merge.
3. Keep source `AnalysisResult` data separate from presentation settings.

## Tree-shaking Decision

ECharts `echarts/core` manual imports can reduce bundle size, but it also requires explicit registration for every chart, component, and renderer. The current catalog is still small, so full tree-shaking is deferred until Chart Studio increases the chart catalog or bundle size becomes a release blocker.
