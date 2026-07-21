# H-RG1 — T-579 오프라인 HAR 분석 UI 개선 재리뷰

- 리뷰 ID: **H-RG1-UI-R1**
- 리뷰일: 2026-07-21
- 리뷰어: Codex (UI 독립 재리뷰, findings-only)
- 리뷰 기준 HEAD: `4c814bb` (`fix(http-capture-ui): remediate H-RG1 UI review (U1-U6)`)
- 비교 기준: `7e02142..4c814bb`
- 원 리뷰: `docs/review/done/2026-07-21_codex_H-RG1_http-capture-ui-review.md`
- 범위 제외: Go HAR parser/redaction 자체의 재판정. 단, UI PASS에 필요한 bounded
  query/aggregate 인터페이스는 H-RG1 수직 슬라이스 경계로 확인했다.
- **UI 재리뷰 판정: `CONDITIONAL`**

## 결론

U2의 MIME/duration/fidelity control, U3의 stale-result provenance reset, U4의
request/response/timing/process tabs와 cookie/body 상태, U5의 keyboard timeline 및
dialog focus 처리는 구현되었다. `npm run test:state`와 production build도 통과하며,
EN/KO key parity도 TypeScript build에서 확인됐다.

그러나 U1의 핵심 수용 기준인 **전체 선택 구간 재집계**는 아직 닫히지 않았다. 현재
timeline은 전체 세션 집계지만 filter/list/tree/selection summary는 엔진이 내려준
bounded `tables.transactions`에만 적용된다. 화면이 이를 “표시 행 기준 하한”으로
고지한 것은 이전보다 정직하지만, H-RG1 계약의 selected-window recomputation이나
전체 세션 filter denominator를 제공한 것은 아니다. 또한 U6에서 요구한 fixture 기반
populated page/Wails/Workspace 통합 회귀 대신 순수 함수와 Workspace 저장 함수만
검증한다. 따라서 이번 재리뷰도 `CONDITIONAL`이다.

| 심각도 | 건수 | 판정 영향 |
|---|---:|---|
| P0 | 0 | 없음 |
| P1 | 1 | UI PASS 전 필수 수정 |
| P2 | 1 | fixture 기반 통합 증거 필요 |
| P3 | 0 | 없음 |

## 독립 검증 증거

- `npm run test:state` → **exit 0**
- `npm run build` → TypeScript와 Vite production build **성공**
- 로컬 Vite/in-app browser smoke:
  - 한국어 HTTP capture 빈 상태와 disabled 분석 action 확인
  - 닫힌 transaction dialog가 `aria-hidden="true"`, `inert`, `tabIndex=-1`인 상태 확인
  - 일반 브라우저에는 Wails native binding이 없어서 실제 HAR populated state는 실행하지
    못함. 이 제한은 아래 R2가 요구하는 component/Wails fixture 회귀로 해소해야 한다.
- `HttpCapturePage.tsx`, `SlideOverPanel.tsx`, `state/httpCapture.ts`,
  `state/regression.test.ts`, analyzer의 bounded result 경계를 원 리뷰 U1-U6과 대조

## 닫힌 원 리뷰 항목

- **U2 기능 구현:** MIME base type, min/max duration, fidelity filter가 추가되고 brush
  window와 조합된다. 단, 선택지와 적용 대상이 bounded rows에 한정되는 문제는 R1에
  통합했다.
- **U3:** file select/clear, analyze start, analyze error가 이전 result/source/filter/detail을
  일관되게 제거한다. A → B 선택/실패 순수 reducer 회귀가 있다.
- **U4:** request/response/timing/process tabs와 cookie, content type, body storage,
  redacted 상태가 상세에 노출된다.
- **U5 코드 경로:** start/end range input, Enter/Space transaction activation,
  `SlideOverPanel` initial focus/trap/restore/closed `inert`가 구현됐다. 실제 populated
  interaction 자동 증거는 R2에 포함한다.

## 잔여 Findings

### R1. [P1] 선택 구간·필터·요약이 전체 세션이 아니라 bounded inline rows만 사용한다

- 위치:
  - `HttpCapturePage.tsx:125-148` — `tables.transactions`에서 method/MIME/fidelity
    option, filter, summary, tree를 모두 만든다.
  - `HttpCapturePage.tsx:319-353` — bounded selection 지표를 “표시 행 기준 하한”으로
    고지한다.
  - `state/httpCapture.ts:222-327` — filter와 projection은 전달받은 row 배열만 처리한다.
  - `internal/analyzers/httpcapture/analyzer.go:153-172` — transaction table은 `topN`으로
    잘리지만 timeline과 session summary는 전체 entry에서 계산된다.
- 문제: 1,000건 세션에서 `topN=50`이면 brush와 filter는 첫 50개 상세에만 적용된다.
  51번째 이후에만 존재하는 MIME/fidelity는 dropdown에 나타나지 않고, 선택 구간에
  실제 200건이 있어도 카드/list/tree는 그중 bounded rows만 센다. timeline selection과
  filter가 동일한 **전체 선택 집합**을 공유하지 않는다.
- 평가: “하한” 고지는 잘못된 전체 수치 주장을 막는 유효한 완화다. 하지만 구현 계획의
  “전체 타임라인, brush 선택, 선택 구간 재집계”와 PASS 기준인 “timeline selection과
  filters가 같은 denominator”를 충족시키지는 않는다.
- 필수 조치: 엔진에 bounded/paged filter+aggregate query를 추가하거나, body/detail을
  제외한 전체 lightweight transaction projection을 별도 bounded contract로 제공한다.
  UI는 동일 query/snapshot에서 summary/list/tree/filter option을 받아야 한다. 첫 page
  밖에만 존재하는 MIME/error/window fixture로 denominator를 고정한다.

### R2. [P2] fixture-driven populated page와 Workspace 통합 회귀가 여전히 없다

- 위치: `state/regression.test.ts:686-795`.
- 문제: 추가된 테스트는 synthetic row의 순수 filter/projection/reducer와
  `addWorkspaceResult` 저장·조회만 검증한다. `HttpCapturePage`를 렌더하지 않고,
  `engine.analyzeHttpCapture` 성공/실패, 실제 Workspace 등록 호출, tabs/cookies,
  keyboard dialog, bounded/diagnostic/degenerate 화면을 연결해서 검증하지 않는다.
- 원 재리뷰 조건: 정상 HAR, degenerate timestamp, truncated detail,
  diagnostic/error HAR를 이용한 populated component 또는 Wails 통합 회귀.
- 필수 조치: 최소한 위 네 상태를 fixture result로 렌더하는 component integration
  harness를 추가한다. 분석 성공 → Workspace entry, A 성공 → B 실패 provenance,
  filter/brush/card denominator, detail tabs/cookies와 focus restore를 실제 DOM에서
  검증한다.

## 재리뷰 조건

1. R1의 전체 세션 filter/selection aggregate 계약을 엔진과 UI가 함께 구현한다.
2. 첫 bounded page 밖의 transaction을 포함하는 denominator 회귀를 추가한다.
3. R2의 fixture-driven populated page/Workspace/a11y 통합 회귀를 추가한다.
4. `npm run test:state`, component integration test, production build를 다시 통과한다.

이 재리뷰는 H-RG1 UI만 다룬다. H-RG1 전체 PASS에는 H-SEC1 재리뷰와 엔진 개선
재리뷰, 최종 그룹 리뷰가 별도로 필요하다.
