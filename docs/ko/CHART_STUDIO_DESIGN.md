# Chart Studio 설계

Chart Studio는 Phase 2 chart template registry를 우회하지 않고 그 위에 구현한다.

## Factory Data Contract

Chart factory는 dashboard fixture data와 실제 analyzer result type을 받을 수 있다. Chart Studio도 이 모델을 유지하고, arbitrary JSON을 option builder에 직접 전달하기보다 analyzer별 type narrowing을 추가해야 한다.

가까운 data source:

- dashboard sample fixture
- `access_log` result
- `profiler_collapsed` result
- contract가 정의된 뒤의 GC/thread/exception result

## 구현된 MVP

현재 Chart Studio 화면은 shared chart template registry와 factory를 사용해 실제 template preview workflow를 제공한다.

- catalog에서 chart template 선택
- panel title 편집
- template이 지원하는 경우 Canvas/SVG renderer 전환
- ArchScope light/dark theme 전환
- 생성된 ECharts option JSON 확인
- template에 정의된 export preset metadata 확인

이 방식은 source analyzer data와 presentation setting을 분리하고 arbitrary JSON을 chart builder에 직접 전달하지 않는다.

## Option Persistence

Persisted chart setting은 생성된 ECharts option 전체가 아니라 사용자의 의도를 저장해야 한다.

권장 shape:

```text
{
  template_id: string,
  title_override: string | null,
  renderer: "canvas" | "svg",
  theme: "archscope" | "archscope-dark",
  option_overrides: object
}
```

Render 시점:

1. Factory에서 base option을 만든다.
2. 검증된 작은 override를 deep merge로 적용한다.
3. 원본 `AnalysisResult` data와 presentation setting을 분리한다.

## Tree-shaking 결정

ECharts `echarts/core` manual import는 bundle size를 줄일 수 있지만, 모든 chart/component/renderer를 명시적으로 등록해야 한다. 현재 catalog는 아직 작으므로, Chart Studio로 chart catalog가 늘어나거나 bundle size가 release blocker가 될 때까지 full tree-shaking은 보류한다.
