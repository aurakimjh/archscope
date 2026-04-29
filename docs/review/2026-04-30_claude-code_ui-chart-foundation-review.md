# UI Extensibility & Chart Foundation 검토 의견서

- **작성일**: 2026-04-30
- **작성자**: Claude Code (Opus 4.6)
- **대상 프로젝트**: ArchScope — Application Architecture Diagnostic & Reporting Toolkit
- **검토 범위**: `5cf047c..654f871` (2 commits, 29 files changed, +1589/-74)
- **이전 검토 문서**: `/docs/review/done/2026-04-29_claude-code_phase1-review.md`, `/docs/review/done/2026-04-29_claude-code_phase2-readiness-followup.md`

---

## 0. Executive Summary

- **종합 평가**: Phase 2 첫 작업 4건은 Chart Studio를 위한 토대를 체계적으로 구축했다. 차트 템플릿/팩토리 분리, i18n 키 체계, ECharts 6 마이그레이션, 그리고 placeholder 분석기의 상태 모델링까지 — 각각이 독립적으로 동작하면서 횡단적으로도 잘 결합되어 있다. 특히 `chartTemplates.ts`의 메타데이터 구조는 Chart Studio의 차트 검색/필터링에 바로 사용할 수 있는 수준이다. 다만 상태 모델이 boolean 플래그 기반이 아닌 string union으로 올바르게 설계되었음에도, 불가능 상태 차단이 타입 레벨에서 완전하지 않고, 접근성 속성이 부재하며, 일부 UI 문자열이 i18n 바깥에 남아 있다.
- **Chart Studio 진입 가능 여부**: ⚠️ 조건부 가능 — 현재 토대로 시작 가능하나, 동적 차트 등록과 옵션 직렬화 계층이 부재하여 Chart Studio 중반에 보강이 필요하다.
- **작업별 종합 등급**:
  - T-019: **B** — 상태 모델이 올바르나 접근성과 race condition 방어 미흡
  - T-020: **A** — 차트 레이블의 구조적 i18n 분리 달성, 키 네이밍 일관
  - T-021: **B+** — 템플릿 메타데이터 설계 우수, 단 동적 등록/직렬화 계층 미비
  - T-033: **A** — ECharts 6 마이그레이션 안정적, 다크모드 테마 사전 등록 완료
- **즉시 조치 권고 Top 3**:
  1. `PlaceholderPage`의 `analyzePlaceholder()`에서 unmount 이후 `setState` 호출 방어 추가
  2. file dialog filter 레이블(`"Log files"`, `"All files"` 등)을 i18n 키로 전환
  3. `ChartPanel`에 `aria-busy` 속성 추가 (데이터 로딩 중 접근성 개선)

---

## 1. 변경 사항 개요

### 1.1 검토 대상 커밋 범위

| Commit | 주요 변경 |
|---|---|
| `fe20cdf` | Phase 2 follow-up 리뷰 처리, review_decisions.md 갱신 |
| `654f871` | T-019/T-020/T-021/T-033 통합 구현: ECharts 6, 차트 팩토리/템플릿, i18n 확장, Placeholder 상태 스켈레톤, ESLint 도입, CI lint/coverage 추가 |

### 1.2 변경 규모 및 영향 범위

| 영역 | 파일 수 | 주요 변경 |
|---|---|---|
| 차트 시스템 | 4 | `chartFactory.ts` (신규), `chartTemplates.ts` (신규), `chartOptions.ts` (리팩토링), `echartsTheme.ts` (다크모드) |
| UI 페이지 | 7 | `PlaceholderPage.tsx` (대폭 확장), `DashboardPage.tsx` (팩토리 전환), 3 placeholder 분석기 (GC/Thread/Exception), `AccessLogAnalyzerPage.tsx`, `ProfilerAnalyzerPage.tsx` |
| 컴포넌트 | 2 | `ChartPanel.tsx` (renderer/theme props), `AnalyzerFeedback.tsx` (EngineMessagesPanel) |
| i18n | 1 | `messages.ts` (+12 keys: 차트 축/범례 레이블) |
| 인프라 | 3 | `eslint.config.js` (신규), `ci.yml` (lint/coverage 추가), `package.json` (ECharts 6, ESLint) |
| 문서 | 4 | `CHART_DESIGN.md` en/ko (팩토리/ECharts 6), `PARSER_DESIGN.md` en/ko (bounded percentile) |

### 1.3 의존성 변동

| 패키지 | 이전 | 이후 | 비고 |
|---|---|---|---|
| `echarts` | `^5.5.1` | `^6.0.0` | **메이저 업그레이드** |
| `eslint` | 미사용 | `^10.2.1` | 신규 도입 |
| `@eslint/js` | 미사용 | `^10.0.1` | 신규 도입 |
| `typescript-eslint` | 미사용 | `^8.59.1` | 신규 도입 |
| `ruff` (Python) | 미사용 | CI에서 사용 | `pyproject.toml`에 설정 추가 |

---

## 2. 작업별 검증 결과

### 2.1 T-019: UI 상태 스켈레톤 [등급: B]

- **변경 위치**: `PlaceholderPage.tsx` (1→135 lines), `GcLogAnalyzerPage.tsx`, `ThreadDumpAnalyzerPage.tsx`, `ExceptionAnalyzerPage.tsx`, `AnalyzerFeedback.tsx` (+`EngineMessagesPanel`)

#### 상태 모델링 평가

**올바른 설계 결정**: `AnalyzerState = "idle" | "ready" | "running" | "error"` string union type 사용 (`PlaceholderPage.tsx:8`). boolean 플래그 조합(`isLoading && isError`)이 아닌 상태 머신 접근으로, 불가능 상태 조합을 구조적으로 줄였다.

AccessLog/Profiler 분석기 페이지는 `"success"` 상태를 추가로 포함하여 5-state 모델(`"idle" | "ready" | "running" | "success" | "error"`)을 사용한다 (`AccessLogAnalyzerPage.tsx:21`). Placeholder는 실제 결과가 없으므로 4-state가 적절하다.

**상태 전이 매핑**:
```
idle → ready (파일 선택)
ready → running (Analyze 클릭)
running → error (분석 실패 / not implemented)
error → ready (파일 재선택)
ready → idle (없음 — 전이 누락이지만 현 UX에서 불필요)
```

**잘 처리된 점**:
- `canAnalyze = Boolean(filePath) && state !== "running"` — running 중 중복 클릭 방지 (`PlaceholderPage.tsx:33`)
- 파일 선택 시 `setError(null)` 호출로 이전 에러 상태 초기화 (`PlaceholderPage.tsx:83`)
- `ErrorPanel`이 `error === null`이면 `null` 반환 — 조건부 렌더링 깔끔

**미흡한 점**:

1. **Unmount 이후 setState 호출 가능** [Severity: Medium]
   - `PlaceholderPage.tsx:101-114`의 `analyzePlaceholder`는 `await Promise.resolve()` 이후 `setError`와 `setState`를 호출한다. 현재는 동기적 `Promise.resolve()`이므로 문제가 없지만, 이 패턴이 실제 async IPC 호출로 교체될 때 컴포넌트 unmount 후 setState가 호출될 수 있다.
   - **권고**: AbortController 패턴이나 ref-based mount 체크를 실제 분석기 연동 시 적용. 현재 placeholder에서는 주석으로 의도를 명시하면 충분.

2. **disabled 사유 미전달** [Severity: Low]
   - Analyze 버튼이 `disabled={!canAnalyze}`로 비활성화되지만, **왜** 비활성화인지(파일 미선택 vs 분석 진행 중) 사용자에게 전달되지 않는다.
   - **권고**: Phase 3 이후 tooltip이나 helper text 추가 고려. 현 단계에서는 버튼 텍스트가 `"Analyzing..."` / `"Analyze"`로 구분되므로 최소한의 피드백은 제공.

3. **접근성 속성 부재** [Severity: Medium]
   - `aria-busy`, `aria-disabled`, `role="alert"` 등이 사용되지 않았다. 에러 메시지가 `ErrorPanel`에 나타나지만 스크린 리더에게 자동으로 알려지지 않는다.
   - **권고**: `ErrorPanel`의 `<section>`에 `role="alert"`를 추가하면 에러 발생 시 스크린 리더가 즉시 안내한다.

```tsx
// AnalyzerFeedback.tsx — ErrorPanel 개선
<section className="message-panel error-panel" role="alert">
```

4. **PlaceholderPage 재사용성** [Severity: 긍정적 평가]
   - `PlaceholderAnalyzerProps` 인터페이스로 파일 선택 UI를 추상화하고, `analyzer` prop이 없으면 순수 placeholder로 동작하는 분기가 깔끔하다. `GcLogAnalyzerPage`, `ThreadDumpAnalyzerPage`, `ExceptionAnalyzerPage`가 각각 5줄 미만으로 구현됨 — 새 분석기 추가 시 동일 패턴 즉시 적용 가능.

---

### 2.2 T-020: 차트 i18n [등급: A]

- **변경 위치**: `messages.ts` (+12 keys), `chartOptions.ts` (`ChartLabels` type), `chartFactory.ts`, `DashboardPage.tsx`

#### 하드코딩 잔존 여부

**차트 옵션 내부**: 한국어/영어 문자열이 차트 옵션에 직접 박혀있지 않음을 grep으로 확인 완료. 모든 차트 레이블이 `ChartLabels` 객체를 통해 전달된다.

**차트 외부 잔존** [Severity: Low]:
- file dialog filter 레이블: `"Log files"`, `"All files"`, `"Collapsed stack files"`, `"Thread dump files"` — 총 5개 페이지에서 영문 하드코딩 (`AccessLogAnalyzerPage.tsx:53-54`, `ProfilerAnalyzerPage.tsx:46-47`, `GcLogAnalyzerPage.tsx:13-14`, `ThreadDumpAnalyzerPage.tsx:14`, `ExceptionAnalyzerPage.tsx:13-14`)
- select option 레이블: `"NGINX"`, `"Apache"`, `"OHS"`, `"WebLogic"`, `"Tomcat"`, `"Custom Regex"` (`AccessLogAnalyzerPage.tsx:152-157`)
- 이들은 OS 네이티브 dialog와 기술 용어이므로 i18n 대상에서 제외할 수 있으나, 정책적 결정이 필요하다.

#### i18n 구조 평가

**강점**:
- `MessageKey = keyof typeof messages.en` 타입으로 번역 키가 컴파일 타임에 검증된다 (`messages.ts:211`). 존재하지 않는 키를 `t()`에 전달하면 타입 에러.
- `ChartLabels` 타입이 `chartOptions.ts:5-11`에 명시적으로 정의되어, 차트 옵션 빌더가 요구하는 레이블 셋이 컴파일 타임에 보장된다.
- 차트 관련 12개 키 (`requestsAxis`, `millisecondsAxis`, `statusSeries`, `samplesAxis`, `p95Series`, `requestCountTrend`, `responseTimeP95Trend`, `statusCodeDistribution`, `profilerComponentBreakdown` + 축/범례 전용 키)가 `en`/`ko` 양쪽에 동일한 구조로 정의.

**번역 키 체계 일관성**:
- `xxxAxis` (축 레이블): `requestsAxis`, `millisecondsAxis`, `samplesAxis` — 일관
- `xxxSeries` (범례 레이블): `statusSeries`, `p95Series` — 일관
- `xxxTrend` / `xxxDistribution` / `xxxBreakdown` (차트 제목): 일관

**언어 전환 동작**:
- `useMemo([data, t])` 의존성으로 locale 변경 시 `chartOptions`가 재계산된다 (`DashboardPage.tsx:20-39`). `t` 함수 참조가 locale 변경 시 새로 생성되므로 (`I18nProvider.tsx:32-42`), React 재렌더 + ECharts `setOption`이 자동으로 트리거된다.
- 현재 `ChartPanel`은 `useEffect([option, renderer, theme])`에서 매번 `chart.dispose()` + `echarts.init()`을 수행한다. 이는 locale 전환 시 차트가 완전히 재생성됨을 의미한다. 성능 최적화(ECharts `setOption`으로 부분 갱신)는 Chart Studio 단계에서 검토하면 된다.

**잘 처리된 점**:
- `chartTemplates.ts`의 `titleKey`, `axisLabelKeys`, `legendLabelKeys`가 모두 `MessageKey` 타입이다 (`chartTemplates.ts:13-15`). 템플릿 정의 시점에 번역 키 존재가 보장.
- 단위 문자열(`"ms"`)이 한국어에서도 `"ms"`로 유지됨 — 기술 단위는 번역하지 않는다는 암묵적 정책이 올바르다.

**미흡한 점**:
- `Intl.NumberFormat`, `Intl.DateTimeFormat`이 사용되지 않았다. 현재 `toLocaleString()`은 브라우저 locale을 따르므로 앱 locale과 불일치할 수 있다. Phase 2 중기에 명시적 포맷터 도입 권고.

---

### 2.3 T-021: 차트 템플릿 & 옵션 팩토리 [등급: B+]

- **변경 위치**: `chartTemplates.ts` (신규 81 lines), `chartFactory.ts` (신규 31 lines), `chartOptions.ts` (리팩토링), `DashboardPage.tsx` (팩토리 소비)

#### 추상화 계층 평가

3-tier 분리가 명확하다:

| 계층 | 역할 | 파일 |
|---|---|---|
| **Template** (메타데이터) | 차트 ID, 결과 타입, 종류, i18n 키, 렌더러 지원, export 포맷 | `chartTemplates.ts` |
| **Factory** (매핑) | template ID → option builder 함수 디스패치 | `chartFactory.ts` |
| **Option Builder** (순수 함수) | 데이터 + 레이블 → EChartsOption 생성 | `chartOptions.ts` |

**타입 안전성**:
- `ChartTemplateId` union type이 `chartFactories` Record의 key와 1:1 매핑 (`chartFactory.ts:18-23`). 새 ID를 추가하면 factory에도 추가하지 않으면 컴파일 에러.
- `ChartFactory = (data: DashboardSampleResult, labels: ChartLabels) => EChartsOption` — 순수 함수 서명, 사이드 이펙트 없음.
- **주의**: `ChartFactory`의 `data` 파라미터가 `DashboardSampleResult` 고정이다. 실제 분석기 결과(`AccessLogAnalysisResult`, `ProfilerCollapsedAnalysisResult`)를 Chart Studio에서 사용하려면 data 파라미터를 제네릭화하거나 union으로 확장해야 한다.

**잘 처리된 점**:

1. `ChartTemplate` 메타데이터가 Chart Studio에 필요한 핵심 정보를 이미 포함한다:
   - `resultType`: 어떤 분석 결과에 적용되는지
   - `chartKind`: 시각적 분류 (line, bar, donut, horizontal_bar)
   - `supportedRenderers`: Canvas/SVG 지원 여부
   - `supportsDarkMode`: 다크모드 지원 여부
   - `exportFormats`: 내보내기 포맷 목록

2. `DashboardPage`가 팩토리를 통해 차트를 생성한다 (`DashboardPage.tsx:31-37`):
   ```typescript
   dashboardChartTemplateIds.map((templateId) => {
     const template = getChartTemplate(templateId);
     return {
       id: template.id,
       title: t(template.titleKey),
       option: createChartOption(template.id, data, labels),
     };
   });
   ```
   — 페이지가 개별 option builder를 직접 import하지 않고 factory를 통해 요청하는 패턴이 올바르다.

3. `getChartTemplate`이 `find`로 검색하고 `throw`로 방어 (`chartTemplates.ts:76-80`). 이 함수는 정적 ID로만 호출되므로 런타임에 throw될 가능성은 낮지만, 동적 로딩 시나리오(Chart Studio)에서 잘못된 ID를 방어한다.

**미흡한 점**:

1. **동적 차트 등록 메커니즘 부재** [Severity: Medium]
   - `chartFactories`가 파일 레벨 상수 (`chartFactory.ts:18-23`)로 정의되어, 런타임에 새 차트 타입을 등록할 수 없다.
   - Chart Studio가 사용자 정의 차트를 지원하려면 등록 API가 필요하다.
   - **권고**: Chart Studio 진입 시 `registerChartFactory(id, factory)` 함수를 추가하되, 현재 정적 등록은 그대로 유지.

2. **옵션 직렬화/역직렬화 부재** [Severity: Medium]
   - Chart Studio에서 사용자가 차트 옵션을 편집하고 저장/불러오기하려면 EChartsOption을 JSON으로 직렬화 가능해야 한다.
   - 현재 option builder가 순수 함수이므로 출력값은 직렬화 가능하지만, 이를 위한 명시적 스키마나 저장 API가 없다.
   - **권고**: Chart Studio 작업 시 함께 설계. 현 단계에서는 선행 작업 불필요.

3. **`data` 타입이 `DashboardSampleResult` 고정** [Severity: Medium]
   - `chartFactory.ts:13-16`의 `ChartFactory` 타입이 `DashboardSampleResult`만 받는다. 실제 분석기 결과(`AccessLogAnalysisResult` 등)를 Chart Studio에서 렌더링하려면 data 파라미터를 확장해야 한다.
   - 현재 Dashboard 전용이므로 동작에 문제는 없지만, Chart Studio 진입 시 첫 번째 수정 대상이 된다.
   - **권고**: `ChartFactory<T>` 제네릭 또는 `AnalysisResult` union으로 확장. 이 작업은 Chart Studio 시작과 동시에 수행 가능.

---

### 2.4 T-033: ECharts 6 업그레이드 [등급: A]

- **변경 위치**: `package.json` (`echarts: ^6.0.0`), `echartsTheme.ts` (다크모드 테마), `ChartPanel.tsx` (renderer/theme props), `CHART_DESIGN.md` en/ko

#### 마이그레이션 완전성

ECharts 5 → 6 전환이 안정적으로 수행되었다:

| 점검 항목 | 결과 |
|---|---|
| `package.json` 버전 | `^6.0.0` 확인 |
| TypeScript 컴파일 | `tsc --noEmit` 정상 통과 (에러 0건) |
| ESLint | 정상 통과 (`--max-warnings=0`) |
| ECharts 자체 타입 | v6는 자체 타입 정의를 포함하므로 `@types/echarts` 불필요 — 올바르게 미사용 |
| Deprecated API | 현재 사용 중인 API(line, bar, pie, tooltip, legend, grid, xAxis, yAxis)가 모두 v6에서 유지됨 |
| `import * as echarts from "echarts"` | v6에서도 동일하게 동작 — 호환 |

#### 다크 모드

`echartsTheme.ts:21-28`에서 `archscope-dark` 테마를 별도 등록:

```typescript
echarts.registerTheme("archscope-dark", {
  ...baseTheme,
  color: ["#60a5fa", "#34d399", "#fbbf24", "#f87171", "#a78bfa", "#22d3ee"],
  textStyle: { ...baseTheme.textStyle, color: "#f8fafc" },
});
```

- 밝은 테마 대비 명도가 높은 색상 팔레트로 전환 — 시각적으로 적절
- `ChartPanel`이 `theme` prop을 받으므로 (`ChartPanel.tsx:9`), 앱 레벨 다크모드 토글이 `theme="archscope-dark"` 전달만으로 동작
- 단, **현재 앱에 다크모드 토글 UI가 없다**. 테마는 등록되었지만 활성화 경로가 없음. Phase 3 이후 UI 토글 추가 시 즉시 사용 가능.

#### SVG Renderer

`ChartPanel`이 `renderer` prop을 받아 `echarts.init(el, theme, { renderer })` 에 전달 (`ChartPanel.tsx:25`). 기본값은 `"canvas"`, `"svg"` 선택 가능.

- SVG export 준비 완료 — Chart Studio에서 SVG 내보내기 시 `renderer="svg"`로 인스턴스를 생성하면 된다.
- 현재 모든 차트가 canvas로 렌더링되므로, SVG와 canvas 간 시각적 차이는 Chart Studio 작업 시 검증 필요.

#### Broken Axis / Custom Chart

`CHART_DESIGN.md:76`에서 명시적으로 언급:
> "Broken axis and custom distribution charts remain template-level capabilities to evaluate when GC pause and latency distribution analyzers produce the required series."

현재 구현된 차트(line, bar, donut, horizontal_bar)에서는 broken axis가 불필요하며, GC pause 분석기가 구현될 Phase 3에서 평가하는 것이 적절하다.

#### 번들 크기

`CHART_DESIGN.md:77`에서 인지:
> "Vite production build currently emits a large bundle warning; code splitting should be handled during Chart Studio/export expansion."

ECharts 6 전체 import(`import * as echarts from "echarts"`)는 번들 크기에 불리하다. 그러나 현재 chart 타입이 4개뿐이므로 tree-shaking 도입은 Chart Studio 단계에서 실제 차트 타입이 늘어난 후 효과적이다.

**잘 처리된 점**:
- 마이그레이션이 기존 차트 옵션 구조를 변경하지 않고 완료됨 — 무변경 마이그레이션은 가장 안전한 경로
- 다크모드 테마 사전 등록으로 향후 토글 UI 추가 시 차트 코드 변경 불필요
- `ChartPanel`의 `aria-label={title}` — 접근성 기본 준수 (`ChartPanel.tsx:38`)
- resize 이벤트 핸들러 등록/해제가 cleanup 함수에서 올바르게 처리됨 (`ChartPanel.tsx:28-34`)

**미흡한 점**:

1. **ChartPanel의 완전 재생성** [Severity: Low]
   - `useEffect` 의존성이 `[option, renderer, theme]`이므로, option이 바뀔 때마다 `chart.dispose()` + `echarts.init()`이 호출된다. `setOption`으로 부분 갱신하면 더 효율적이지만, 현재 차트 4개, 데이터 크기 미미하므로 성능 영향 없음.
   - **권고**: Chart Studio에서 차트 옵션 실시간 미리보기를 구현할 때 `setOption` 경로로 전환.

2. **시각적 회귀 테스트 부재** [Severity: Low]
   - ECharts 5 → 6에서 폰트 렌더링, 범례 위치, 툴팁 스타일의 미묘한 차이가 있을 수 있다. 현재 자동화된 시각적 회귀 테스트가 없다.
   - **권고**: Chromatic/Percy 같은 시각적 회귀 도구 도입은 차트 타입이 10개 이상일 때 가치가 있다. 현 단계에서는 수동 확인으로 충분.

---

## 3. 횡단 검증 결과

### 3.1 i18n × 차트 팩토리 통합

**통합 상태: 양호**

`DashboardPage.tsx:24-29`에서 `t()` 호출로 `ChartLabels` 객체를 생성하고, 이를 `createChartOption(templateId, data, labels)`에 전달한다. 팩토리 → 옵션 빌더 → EChartsOption 경로에서 모든 텍스트가 i18n 키를 거친다.

언어 전환 시 `useMemo([data, t])` 의존성에 의해 `chartOptions`가 재계산되고, React 재렌더로 `ChartPanel`이 새 `option`을 받아 ECharts를 재생성한다.

### 3.2 ECharts 6 × 팩토리 추상화

**통합 상태: 양호**

팩토리가 `EChartsOption` 타입을 반환하고, `ChartPanel`이 `echarts.init()`에 전달한다. ECharts 6의 신규 기능(SVG renderer, custom theme)은 `ChartPanel`의 `renderer`/`theme` props로 접근 가능하며, 팩토리 내부에는 ECharts 5 전용 옵션이 없다.

`chartTemplates.ts`의 `supportedRenderers`와 `supportsDarkMode` 필드가 ECharts 6 기능을 메타데이터로 선언하고 있어, Chart Studio가 이 정보를 참조하여 렌더러/테마 선택 UI를 제공할 수 있다.

### 3.3 상태 스켈레톤 × 차트 렌더링

**통합 상태: 부분적**

- `DashboardPage`는 `data === null`일 때 로딩 메시지를 표시하고, 데이터 로드 후 차트를 렌더링한다 — 기본 케이스 처리 완료.
- `AccessLogAnalyzerPage`는 분석 결과 없을 때 빈 차트(빈 데이터로 EChartsOption 생성)를 표시한다 — 빈 차트 영역이 시각적으로 자연스럽지 않을 수 있다.
- **권고**: 분석 결과 없을 때 차트 영역에 "분석 결과를 기다리는 중" placeholder를 표시하면 UX가 개선된다. 이는 Phase 2 후반 UI 개선 시 처리 가능.

### 3.4 다크 모드 일관성

**통합 상태: 준비 완료, 미활성화**

- `archscope-dark` ECharts 테마가 등록되었지만, 앱 전체 다크모드 토글이 없다.
- CSS 변수 기반 디자인 토큰이 아직 도입되지 않았으므로, 다크모드 활성화 시 CSS 전체 테마 시스템이 필요하다.
- **권고**: 다크모드는 Phase 3 이후의 사용자 대면 기능으로 적절. 현재 ECharts 테마 사전 등록은 올바른 사전 작업.

### 3.5 테스트 인프라 통합

- CI에 ESLint(`npm run lint`)와 Python ruff(`ruff check .`) + coverage(`--cov`)가 추가됨 — T-051 보너스 작업.
- Python 테스트 25개 전수 통과 확인, TypeScript `tsc --noEmit` + ESLint `--max-warnings=0` 정상 통과 확인.
- 차트 옵션 빌더에 대한 단위 테스트는 아직 없다. option builder가 순수 함수이므로, "입력 데이터 → 예상 EChartsOption 구조" 테스트를 작성하면 Chart Studio 확장 시 회귀 방지에 유용하다.

---

## 4. Chart Studio 진입 적합성 평가

### 4.1 즉시 가능한 영역

| 영역 | 근거 |
|---|---|
| 차트 메타데이터 검색/필터링 | `ChartTemplate`이 `resultType`, `chartKind`, `supportedRenderers`, `exportFormats`를 포함하므로 Chart Studio의 차트 카탈로그 UI에 바로 사용 가능 |
| 차트 미리보기 | `createChartOption` + `ChartPanel`로 선택한 차트를 즉시 미리보기 가능 |
| i18n 적용 차트 제목/레이블 | `titleKey`, `axisLabelKeys`가 `MessageKey` 타입이므로 Chart Studio에서 locale-aware 편집 가능 |
| SVG/Canvas 렌더러 전환 | `ChartPanel`의 `renderer` prop으로 즉시 지원 |
| 다크모드 테마 미리보기 | `theme` prop 전환으로 즉시 지원 |

### 4.2 보강이 필요한 영역

| 영역 | 현재 상태 | 필요 보강 | 예상 공수 |
|---|---|---|---|
| **데이터 소스 확장** | `ChartFactory`가 `DashboardSampleResult` 고정 | 제네릭 data 파라미터 또는 분석기별 union 타입 | 0.5일 |
| **동적 차트 등록** | 정적 `Record` 상수 | `registerChartFactory()` API | 0.5일 |
| **옵션 편집 UI** | 없음 | 사용자 옵션 입력 폼 + deep merge 전략 | 2~3일 |
| **옵션 직렬화/저장** | 없음 | JSON 직렬화 + localStorage/파일 저장 | 1일 |
| **차트 타입 아이콘/설명** | 없음 | `ChartTemplate`에 `icon`, `description` 필드 추가 | 0.5일 |

### 4.3 추가 사전 작업 권고

**Chart Studio 진입 판정: ⚠️ 시작 가능하지만 도중에 추상화 보강이 필요하다.**

현재 토대로 Chart Studio의 카탈로그/미리보기/테마 전환은 즉시 구현할 수 있다. 그러나 "사용자 정의 차트 옵션 편집 → 저장 → 불러오기"라는 핵심 기능을 구현하려면 `ChartFactory` 타입의 제네릭화, deep merge 전략, 직렬화 계층이 필요하며, 이는 Chart Studio 작업 첫 스프린트에서 함께 설계하면 된다.

선행 보강 없이 시작하되, 첫 2일 내에 데이터 소스 확장과 동적 등록 API를 구현하는 것을 권고한다.

---

## 5. 신규 발견 이슈

#### NEW-Issue #1: `AccessLogAnalyzerPage`의 `buildRequestChartOption` 하드코딩 [Severity: Low]

- **위치**: `AccessLogAnalyzerPage.tsx:271-289`
- **현상**: `AccessLogAnalyzerPage`에 독자적인 `buildRequestChartOption` 함수가 남아있다. 이 함수는 dashboard의 `requestCountTrendOption`과 유사하지만 `type: "bar"`로 다르다.
- **문제점**: 팩토리 경로를 우회하여 직접 EChartsOption을 생성하므로, 향후 차트 옵션 변경 시 팩토리와 이 함수를 모두 수정해야 한다.
- **권고**: Chart Studio 작업 시 이 함수도 팩토리로 통합하거나, 분석기 페이지 전용 차트 옵션을 별도 template으로 등록.
- **Chart Studio와의 관계**: 병행

#### NEW-Issue #2: `EngineMessagesPanel` 스타일 불일치 [Severity: Low]

- **위치**: `AnalyzerFeedback.tsx:43-57`
- **현상**: `EngineMessagesPanel`이 `className="message-panel"` (기본 패널)을 사용하지만, `ErrorPanel`은 `"message-panel error-panel"`이다. engine messages는 에러가 아닌 정보성 메시지이므로 별도 스타일(배경색, 아이콘)이 적절하다.
- **권고**: `info-panel` CSS 클래스를 추가하여 정보성 메시지를 시각적으로 구분.
- **Chart Studio와의 관계**: 후속

#### NEW-Issue #3: `formatNumber`/`formatMilliseconds`/`formatPercent` 중복 [Severity: Low]

- **위치**: `AccessLogAnalyzerPage.tsx:295-305`, `ProfilerAnalyzerPage.tsx:252-266`
- **현상**: 동일한 포맷 헬퍼 함수가 두 분석기 페이지에 각각 정의되어 있다.
- **권고**: `src/utils/formatters.ts`로 추출. MetricCard 통합(T-044)과 동일한 패턴.
- **Chart Studio와의 관계**: 선행 — Chart Studio에서도 동일 포맷터 필요.

#### NEW-Issue #4: `ChartPanel` 접근성 보강 [Severity: Medium]

- **위치**: `ChartPanel.tsx:38`
- **현상**: `aria-label={title}`은 있으나, 차트 로딩 중 `aria-busy`, 차트 에러 시 fallback 텍스트가 없다.
- **권고**: ECharts 인스턴스가 초기화되기 전까지 `aria-busy="true"` 설정.
- **Chart Studio와의 관계**: 병행

#### NEW-Issue #5: `registerArchScopeTheme()`이 `DashboardPage`에서만 호출 [Severity: Low]

- **위치**: `DashboardPage.tsx:16`
- **현상**: `registerArchScopeTheme()`이 `DashboardPage`의 `useEffect`에서 호출된다. 다른 페이지에서 `ChartPanel`을 사용하면서 테마 등록이 누락되면 기본 테마로 fallback된다.
- **권고**: `App.tsx` 또는 `main.tsx`에서 앱 초기화 시 1회 호출로 이동.
- **Chart Studio와의 관계**: 선행 — Chart Studio가 차트를 렌더링하려면 테마가 반드시 등록되어 있어야 함.

---

## 6. 이전 검토 의견서의 관련 항목 추적

| RD / RS 항목 | 원래 지적 | 해소 상태 | 비고 |
|---|---|---|---|
| **RD-012** | "Convert chart settings to JSON templates" | **해소** | `chartTemplates.ts`로 구현. JSON 템플릿 메타데이터 분리 완료. |
| **RD-014** | "Add Analyze button placeholder states" | **해소** | `PlaceholderPage`에 idle/ready/running/error 상태 구현. |
| **RD-015** | "Keep chart title, legend, and axis labels i18n-ready" | **해소** | `ChartLabels` 타입 + `MessageKey` 기반 i18n 분리. |
| **RD-030** | "Decouple Chart Builders into Factory Pattern" | **해소** | `chartFactory.ts` + `chartOptions.ts` 분리. 순수 함수 option builder. |
| **RS-012** | "ECharts 6 upgrade evaluation" | **해소** | `echarts: ^6.0.0` 적용, dark theme 등록, renderer 분리, 마이그레이션 노트 문서화. |
| **RD-045** | "Surface captured engine messages in analyzer UI" | **해소** | `EngineMessagesPanel` 컴포넌트 추가, placeholder 페이지에 engine-message 피드백 포함. |
| **RD-047** | "Add CI lint and coverage reporting" | **해소** (T-051) | ESLint + ruff + pytest-cov가 CI에 추가됨. |

---

## 7. 종합 평가 및 검토자 코멘트

이번 Phase 2 첫 작업 4건은 Chart Studio를 위한 구조적 토대를 올바르게 설치했다. 가장 인상적인 것은 세 가지다:

**첫째, `ChartTemplate` 메타데이터 설계**. 단순히 option builder를 분리한 것이 아니라, `resultType`, `chartKind`, `supportedRenderers`, `exportFormats`, i18n `titleKey`까지 포함한 선언적 메타데이터를 정의했다. 이 구조는 Chart Studio의 "차트 검색 → 미리보기 → 설정 변경 → 내보내기" 워크플로에 직접 매핑된다. 메타데이터를 먼저 설계하고 구현을 따라오게 한 것은 올바른 순서다.

**둘째, i18n의 타입 안전성**. `MessageKey = keyof typeof messages.en` 타입이 `ChartTemplate.titleKey`와 `ChartLabels`까지 관통하므로, 번역 키 누락이 컴파일 타임에 잡힌다. 많은 프로젝트가 i18n을 문자열 기반으로 처리하여 런타임에서야 누락을 발견하는 것과 대조된다.

**셋째, ECharts 6 마이그레이션의 보수적 접근**. "동작하는 것을 유지하면서 새 기능의 문을 열어놓는" 전략 — 기존 line/bar/donut 옵션을 그대로 유지하면서 다크모드 테마와 SVG renderer를 사전 등록한 것 — 이 메이저 버전 업그레이드의 교과서적 접근이다.

개선이 필요한 부분은 세 가지다:

**첫째, 접근성**. `ErrorPanel`에 `role="alert"`가 없고, `ChartPanel`에 `aria-busy`가 없으며, disabled 버튼에 사유 설명이 없다. 현재 내부 도구이므로 당장의 리스크는 낮지만, 접근성은 나중에 "한꺼번에 추가"하기 어려운 영역이므로 점진적으로 적용하는 것이 좋다.

**둘째, `ChartFactory`의 data 타입 경직성**. `DashboardSampleResult` 고정은 Chart Studio 진입 시 첫 번째 마찰 지점이 될 것이다. 이 변경은 어렵지 않으므로(0.5일 수준), Chart Studio 첫 스프린트에서 즉시 처리할 수 있다.

**셋째, 분산된 format 헬퍼 함수**. `formatNumber`, `formatMilliseconds`, `formatPercent`가 두 페이지에 중복 정의되어 있다. MetricCard 통합(T-044)과 동일한 패턴이므로, 차트 작업이 더 진행되기 전에 공유 유틸리티로 추출하는 것을 권한다.

---

## 8. 참고: 외부 리서치 및 레퍼런스

| 항목 | 자료 |
|---|---|
| ECharts 6 릴리스 | Apache ECharts Changelog — v6.0.0은 TypeScript 자체 타입 포함, SVG 렌더러 성능 개선, custom series API 안정화 |
| ECharts 5→6 마이그레이션 | 공식 마이그레이션 가이드 — 대부분의 5.x 옵션이 6.x에서 호환. `echarts.use()`의 모듈 등록 방식 변경이 주요 breaking change |
| React i18n 타입 안전성 패턴 | `keyof typeof messages` 패턴은 외부 라이브러리(react-i18next) 없이 가장 가벼운 타입 안전 i18n 구현. 단, pluralization, interpolation이 필요하면 라이브러리 도입 필요 |
| ECharts Tree Shaking | `echarts/core` + `echarts/charts` + `echarts/components` 방식으로 필요한 차트/컴포넌트만 import 가능. 번들 크기 50-70% 절감 사례 |
| Web Accessibility — ARIA 라이브 리전 | `role="alert"` + `aria-live="assertive"`로 동적 에러 메시지를 스크린 리더에 즉시 전달. `aria-busy`로 로딩 상태 표시 |
| Reservoir Pattern for Chart Factory | 정적 Record 대신 Map + register 패턴을 사용하면 런타임 차트 등록이 가능하면서 타입 안전성 유지 가능 |
