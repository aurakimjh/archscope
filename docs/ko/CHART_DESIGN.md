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

## Chart Template Factory

Phase 2에서는 Chart Studio가 편집과 export workflow를 소유하기 전에 chart definition을 발견할 수 있도록 template registry와 chart factory entrypoint를 사용한다.

- `chartTemplates.ts`는 stable template ID, title message key, renderer 지원, dark-mode 지원, export format 지원 여부를 저장한다.
- `chartFactory.ts`는 template ID를 재사용 가능한 ECharts option builder에 매핑한다.
- React page는 개별 option builder를 직접 import하기보다 factory를 통해 chart option을 요청해야 한다.

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
