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
- `Profiler.FlamegraphDrilldown`
- `Profiler.ExecutionBreakdownDonut`
- `Profiler.ExecutionBreakdownBars`
- `ThreadDump.ThreadStateDistribution`
- `Exception.ExceptionTrend`

## Chart Template Factory

Phase 2에서는 Chart Studio가 편집과 export workflow를 소유하기 전에 chart definition을 발견할 수 있도록 template registry와 chart factory entrypoint를 사용한다.

- `chartTemplates.ts`는 stable template ID, title message key, renderer 지원, dark-mode 지원, export format 지원 여부를 저장한다.
- `chartFactory.ts`는 template ID를 재사용 가능한 ECharts option builder에 매핑한다.
- React page는 개별 option builder를 직접 import하기보다 factory를 통해 chart option을 요청해야 한다.

## Chart Factory 사용 예시

### Template Registry (chartTemplates.ts)

```typescript
export const chartTemplates: ChartTemplate[] = [
  {
    id: "AccessLog.RequestCountTrend",
    titleKey: "chart.accessLog.requestCountTrend",
    supportedRenderers: ["canvas", "svg"],
    supportsDarkMode: true,
    exportFormats: ["png", "svg", "html"],
    preset: { width: 800, height: 400, aspect: "16:9" },
  },
  // ...
];
```

### Option Builder (chartFactory.ts)

```typescript
import type { AnalysisResult } from "../types";
import type { EChartsOption } from "echarts";

export function buildChartOption(
  templateId: string,
  result: AnalysisResult,
  overrides?: Partial<ChartOverrides>,
): EChartsOption {
  const builder = optionBuilders[templateId];
  if (!builder) throw new Error(`Unknown template: ${templateId}`);
  return builder(result, overrides);
}

// 개별 option builder 예시
function buildRequestCountTrend(
  result: AnalysisResult,
  overrides?: Partial<ChartOverrides>,
): EChartsOption {
  const series = result.series.requests_per_minute ?? [];
  return {
    title: { text: overrides?.title ?? "Request Count Trend" },
    xAxis: { type: "category", data: series.map((r) => r.time) },
    yAxis: { type: "value", name: "Requests" },
    series: [{ type: "line", data: series.map((r) => r.value), smooth: true }],
  };
}
```

### React Page에서 Chart Factory 사용

```typescript
import { buildChartOption } from "../charts/chartFactory";

function AccessLogAnalyzerPage() {
  const [result, setResult] = useState<AnalysisResult | null>(null);

  const chartOption = result
    ? buildChartOption("AccessLog.RequestCountTrend", result)
    : null;

  return <ChartPanel title="Request Trend" option={chartOption} />;
}
```

## 성능 가이드라인

| Data Points | 권장 Renderer | 비고 |
|-------------|---------------|------|
| < 1,000 | Canvas 또는 SVG | 차이 미미 |
| 1,000 - 10,000 | Canvas | SVG는 DOM 부하 증가 |
| 10,000 - 50,000 | Canvas + sampling | chart에서 sampling 적용 권장 |
| 50,000+ | Canvas + aggregation | 분 단위로 사전 집계 후 렌더링 |

대량 데이터 처리 시 참고:
- ECharts `large: true` 옵션으로 대량 scatter/line 최적화 가능
- Flamegraph는 visible viewport만 렌더링 (virtual scrolling 불필요)
- SVG renderer는 인쇄/PDF용으로 선명하지만 대량 데이터에서 느림

## 현재 Skeleton

초기 desktop app은 TypeScript sample data에서 ECharts를 렌더링한다. Chart option builder는 React component와 분리되어 있으며, 향후 export logic에서도 같은 chart definition을 재사용할 수 있다.

## ECharts 6 방향

Phase 2에서는 Engine-UI Bridge PoC 안정화 이후 desktop chart surface에 Apache ECharts 6을 적용했다.

관련 기능:

- Analyst workflow를 위한 dynamic theme switching 및 dark mode
- Latency 또는 GC pause처럼 편차가 큰 분포를 표현하기 위한 broken axis
- Response-time 및 profiler distribution을 표현하기 위한 violin, beeswarm, range chart
- Report generation을 위한 SVG rendering/export 개선
- Visual churn을 최소화해야 할 경우 v5 compatibility theme 검토

Migration note:

- 기존 line, bar, horizontal bar, donut option은 ECharts 6에서 정상 build된다.
- `ChartPanel`은 `canvas` 또는 `svg` renderer mode를 받을 수 있으므로 SVG export 작업이 같은 component path를 재사용할 수 있다.
- `archscope`와 `archscope-dark` theme을 사전에 등록하므로, 이후 dark-mode UI control은 chart option builder를 바꾸지 않고 theme만 선택하면 된다.
- Broken axis와 custom distribution chart는 GC pause 및 latency distribution analyzer가 필요한 series를 제공할 때 template-level capability로 평가한다.
- Vite production build는 현재 큰 bundle 경고를 출력한다. Code splitting은 Chart Studio/export 확장 시점에 처리한다.

## i18n 방향

차트 title, axis label, legend label은 UI locale과 report export locale을 기준으로 변환 가능해야 한다. Raw data value는 번역하지 않고, report-facing label만 locale resource에서 가져온다.

## Profiler Drill-down Charts

Profiler Analyzer는 공통 `FlameNode` contract를 기반으로 multi-stage flamegraph drill-down을 렌더링한다. 각 stage는 자체 flamegraph, metrics, top stacks, top child frames, execution breakdown을 가진다.

Drill-down UI는 다음을 지원한다.

- `All`에서 적용된 filter까지 이어지는 breadcrumb navigation
- include/exclude text 및 regex filter
- anywhere, ordered, subtree match mode
- preserve-full-path 및 re-root-at-matched-frame view mode
- matched samples, estimated seconds, total ratio, parent-stage ratio, elapsed ratio stage metric

Execution breakdown은 donut chart, horizontal bar chart, category top stack table로 표시한다. Breakdown은 선택된 drill-down stage 기준으로 다시 계산한다.
