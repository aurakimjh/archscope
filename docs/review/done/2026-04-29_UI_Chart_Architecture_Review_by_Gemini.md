# [Report] UI Extensibility & Chart Rendering Architecture Review

**Date:** 2026-04-29  
**Reviewer:** Gemini (Principal Front-end Architect)  
**Subject:** Frontend Infrastructure & Chart Rendering Optimization Audit

---

## 1. Executive Summary

Phase 2의 대시보드 고도화와 Chart Studio 확장성을 대비한 프론트엔드 기반 작업 내역을 검토하였습니다. **ECharts 6 업그레이드**, **Chart Factory 패턴 도입**, 그리고 **i18n 통합 구조**는 전반적으로 매우 우수한 설계 완성도를 보여주고 있습니다. 

특히 `ChartTemplate`을 통한 메타데이터 정의와 `ChartFactory`의 결합은 신규 분석 결과에 따른 시각화 컴포넌트 추가를 O(1)의 복잡도로 해결할 수 있는 강력한 토대를 마련했습니다.

---

## 2. 아키텍처 진단 결과 (Architectural Diagnostics)

### A. UI State Skeleton (T-019)
- **진단:** `AccessLogAnalyzerPage` 등의 페이지에서 `AnalyzerState` (idle, ready, running, success, error) 유한 상태 머신(FSM) 개념을 도입하여 비동기 작업의 생명주기를 안정적으로 관리하고 있음.
- **강점:** 
    - `canAnalyze` 플래그를 통한 버튼 비활성화 로직이 단순하고 명확함.
    - `ErrorPanel`과 `EngineMessagesPanel`을 통한 사용자 피드백 경로가 통합됨.
- **보완 권고:** 대용량 파일 분석 시 UI가 장시간 'running' 상태에 머물 수 있으므로, 향후에는 단순 스피너 외에 진행률(Progress Bar) 연동이 필수적임.

### B. Locale-aware Chart Labels (T-020)
- **진단:** `ChartLabels` 타입을 통해 차트 내부 렌더링에 필요한 모든 문자열을 외부(i18n)에서 주입받는 구조를 채택함.
- **강점:** 
    - 차트 옵션 생성 로직(`chartOptions.ts`)이 `useI18n` 훅에 직접 의존하지 않아 순수 함수로서의 테스트 용이성을 유지함.
    - `ChartTemplate`의 `titleKey`, `axisLabelKeys` 등을 통해 메타데이터 수준에서 다국어 처리를 지원함.

### C. Chart Factory & Templates (T-021)
- **진단:** `chartTemplates.ts` (메타데이터) + `chartOptions.ts` (구현체) + `chartFactory.ts` (결합부)의 3층 구조로 설계됨.
- **강점 (OCP 준수):** 
    - 신규 차트 추가 시 기존 로직 수정 없이 `chartTemplates` 배열에 정의를 추가하고 `chartFactories`에 매핑만 하면 됨.
    - `ChartTemplateId`를 문자열 리터럴 유니언으로 관리하여 타입 안정성(Type Safety)을 확보함.

### D. ECharts 6 Upgrade (T-033)
- **진단:** `package.json`의 `echarts: ^6.0.0` 반영 및 `ChartPanel`의 렌더러/테마 파라미터화 완료.
- **강점:** 
    - `renderer="svg"` 지원을 통해 저사양 환경 및 고해상도 내보내기 최적화 기반 마련.
    - `echarts.init` 시점에 테마와 렌더러를 동적으로 주입하여 유연한 UI 대응 가능.

---

## 3. 코드 스멜 및 리팩토링 제안 (Code Smells & Refactoring)

### 1) `AccessLogAnalyzerPage` 내 차트 옵션 빌더 중복 [High]
- **현상:** `AccessLogAnalyzerPage.tsx` 내부에 `buildRequestChartOption` 함수가 여전히 존재하며, 이는 `chartOptions.ts`의 로직과 중복되거나 파편화되어 있음.
- **리팩토링 제안:** 모든 페이지에서 `createChartOption` (Factory)을 사용하도록 통일하고, 페이지 내 로컬 빌더를 제거하십시오.

```typescript
// AS-IS (AccessLogAnalyzerPage.tsx)
const chartOption = useMemo(() => buildRequestChartOption(result, t("requestsAxis")), [result, t]);

// TO-BE
const chartOption = useMemo(() => {
  if (!result) return null;
  return createChartOption("AccessLog.RequestCountTrend", result, {
    requestsAxis: t("requestsAxis"),
    // ... other labels
  });
}, [result, t]);
```

### 2) `ChartPanel` 내 리사이즈 이벤트 누수 방지 [Medium]
- **현상:** `window.addEventListener("resize", resize)` 사용 중.
- **리팩토링 제안:** 현대적인 브라우저 환경에서는 `ResizeObserver`를 사용하여 특정 `div`의 크기 변화를 감지하는 것이 더 정밀하고 효율적입니다.

### 3) Tree-shaking 최적화 미흡 [Low]
- **현상:** `ChartPanel.tsx`에서 `import * as echarts from "echarts"` 사용.
- **리팩토링 제안:** 번들 사이즈 최적화를 위해 필요한 컴포넌트(LineChart, BarChart, CanvasRenderer 등)만 `echarts/core`를 통해 명시적으로 임포트하는 'Manual Import' 방식으로의 전환을 권장합니다.

---

## 4. 종합 평가 및 결론

ArchScope의 UI 및 차트 아키텍처는 **"구성 중심의 확장성(Configuration-driven Extensibility)"**을 훌륭하게 달성했습니다. 특히 `ChartTemplate` 기반의 아키텍처는 향후 사용자가 직접 차트를 커스텀하는 'Chart Studio' 기능을 구현할 때 매우 유리한 고지를 점하게 해줍니다.

제안된 **차트 빌더 통합**과 **ResizeObserver 도입**을 Phase 2 초기 스프린트에서 반영한다면, 더욱 견고하고 완성도 높은 진단 도구로 거듭날 것입니다.
