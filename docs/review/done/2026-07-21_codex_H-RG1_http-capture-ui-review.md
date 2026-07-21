# H-RG1 — T-579 오프라인 HAR 분석 UI 독립 리뷰

- 리뷰 ID: **H-RG1-UI** (그룹 **H-RG1**, 작업 **T-579**의 UI 슬라이스)
- 리뷰일: 2026-07-21
- 리뷰어: Codex (UI 독립 구현 리뷰, findings-only)
- 리뷰 기준 HEAD: `fed84f8` (`8b4d64a`의 T-579 UI 구현 포함)
- 기준 계약:
  - `docs/ko/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §6 `H-RG1`
  - `docs/en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md` §6 `H-RG1`
  - `docs/ko/SYSTEM_HTTP_CAPTURE.md`의 HAR import·충실도·리댁션·UI 계약
- 리뷰 대상:
  - `frontend/src/pages/HttpCapturePage.tsx`
  - `frontend/src/state/httpCapture.ts`
  - `frontend/src/bridge/types.ts`
  - `frontend/src/i18n/messages.ts`
  - `frontend/src/state/regression.test.ts`
- 범위 제외: Go HAR parser/analyzer/redaction/exporter 및 CLI/Wails parity의 엔진
  판정. 엔진 슬라이스는 Claude가 별도로 리뷰한다.
- **UI 판정: `CONDITIONAL`** — H-RG1 전체를 `PASS`로 전환하지 않는다.

> 이 문서는 UI findings만 기록한다. 소스 구현은 수정하지 않았다. 아래 P1/P2를
> 반영한 뒤 UI 재리뷰가 필요하며, H-RG1 전체 PASS에는 별도의 엔진 리뷰와 H-SEC1
> 재리뷰 PASS도 필요하다.

## 요약

T-579 UI는 기존의 단순 표 페이지에서 크게 발전했다. typed `http_capture` 와이어
형식, HAR origin 기반 유사 프로세스 트리, 타임라인 brush, 트랜잭션 목록·상세
slide-over, fidelity/redaction/diagnostic 노출, EN/KO 문구와 순수 상태 회귀 테스트가
구현되었고 현재 frontend state test와 production build는 통과한다. import-only
문구도 라이브 캡처 기능으로 오인시키지 않는다.

그러나 계약의 핵심 수용 기준 두 가지가 아직 충족되지 않는다. 첫째, brush와
필터는 목록·트리만 바꾸고 상단 요약 카드는 전체 결과를 그대로 표시하므로 선택
구간과 필터 결과가 같은 분모를 사용하지 않는다. 둘째, 계약에 명시된 MIME,
duration, fidelity 필터가 없다. 여기에 새 파일 선택이나 분석 실패 뒤에도 이전
결과가 남아 새 입력의 결과로 오인될 수 있는 provenance 문제가 있다. 따라서 UI
슬라이스 판정은 `CONDITIONAL`이다.

| 심각도 | 건수 | 판정 영향 |
|---|---:|---|
| P0 | 0 | 없음 |
| P1 | 3 | UI PASS 전 필수 수정 |
| P2 | 3 | UI PASS 전 수정·회귀 증거 필요 |
| P3 | 0 | 없음 |

## 검증 증거

2026-07-21 현재 checkout에서 다음을 독립 실행했다.

- `npm run test:state` → **exit 0**
- `npm run build` → TypeScript와 Vite production build **성공**
- 로컬 Vite 화면을 in-app browser로 확인:
  - 한국어 HTTP capture 진입, import-only 설명, 빈 상태, 분석 버튼 disabled 상태 확인
  - 라이브 캡처 시작·daemon·headless 동작을 암시하는 버튼이나 문구 없음
- `HttpCapturePage.tsx`, `state/httpCapture.ts`, bridge type, i18n, regression harness를
  수용 기준과 줄 단위로 대조

일반 브라우저 개발 서버에는 Wails native binding이 없으므로, 실제 HAR 분석 결과가
채워진 화면은 이번 browser smoke에서 실행하지 못했다. 채워진 상태는 소스와 순수
상태 테스트로 검토했으며, 이 한계 자체가 U6의 fixture-driven Wails UI 회귀 증거
요구로 남는다.

## 확인된 통과 항목

- **타입 계약:** `HttpCaptureAnalysisResult`, transaction/message/timing/process,
  summary/series/table/metadata가 명시적 TypeScript 형식으로 정의되어 있다.
- **유사 프로세스 트리:** HAR에 OS process가 없을 때 host origin을 합성 root로
  사용하고 `process → connection → transaction` 계층과 오류·duration·byte 집계를
  만든다. 실제 process가 있으면 process root를 우선한다.
- **기본 brush 동작:** 좌우 방향을 정규화하고 선택한 timeline bucket을 시간 창으로
  변환해 transaction 목록과 tree에 적용한다. 순수 상태 테스트가 기본·역방향·빈
  timeline을 확인한다.
- **정직한 evidence copy:** dialect, capture mode, observation point, fidelity,
  bounded detail, redaction policy/count, truncation, parser diagnostics와 findings를
  화면에 노출한다. HAR import를 live proxy capture라고 표현하지 않는다.
- **기본 상태:** 분석 전 빈 상태, 분석 실패 ErrorPanel, timeline 불가 상태,
  필터 결과 없음, bounded detail 경고가 각각 존재한다.
- **언어 정합성:** 이번 UI에 추가된 문구는 EN/KO 양쪽에 존재하고 production
  TypeScript build가 key 정합성을 통과한다.

## Findings

심각도는 P0 blocker, P1 major, P2 minor/acceptance gap, P3 nit 순이다. 모든 finding의
구현 담당은 Claude UI 세션이며, U1은 bounded aggregate 계약 때문에 엔진 리뷰어와
인터페이스를 합의해야 할 수 있다.

### U1. [P1] 선택 구간·필터와 요약 카드가 같은 분모를 사용하지 않는다

- 위치:
  - `HttpCapturePage.tsx:104-118` — engine의 전체 summary와 bounded transaction
    table을 따로 읽는다.
  - `HttpCapturePage.tsx:173-187` — 여섯 summary card는 항상 engine 전체 summary다.
  - `HttpCapturePage.tsx:192-214` — brush/filter는 목록과 tree에만 적용된다.
  - `state/httpCapture.ts:172-184` — 필터 결과만 반환하고 선택 구간 재집계가 없다.
- 계약: “전체 타임라인, brush 선택, 선택 구간 재집계” 및 “timeline selection과
  filters가 같은 denominator를 사용”해야 한다.
- 문제: 예를 들어 전체 1,000건 중 한 구간의 10건을 선택해도 목록 표시는 10건으로
  줄지만 Transactions, Error rate, Hosts, Endpoints, p95, Response bytes는 1,000건
  전체 값을 계속 보여준다. 더구나 `tables.transactions`는 기본 50건의 bounded
  detail이므로 브라우저에서 그 배열만 다시 집계해도 전체 선택 구간 분모를 복원할
  수 없다.
- 영향: 사용자는 선택 구간의 카드라고 자연스럽게 해석하지만 실제로는 전체 세션
  값이므로 분석 결론이 잘못될 수 있다. H-RG1 PASS 기준의 직접 위반이다.
- 권고: 선택 window와 모든 활성 필터를 하나의 projection 계약으로 묶고, 카드·tree·
  list의 numerator/denominator와 “상세 N/M건” 한계를 명시한다. bounded detail만으로
  계산할 수 없는 집계는 전체 집계처럼 보이지 않게 하고, 필요하면 엔진의 bounded
  aggregate/query 계약과 조정한다. 동일 분모를 고정하는 회귀 테스트를 추가한다.

### U2. [P1] 계약에 명시된 필터 중 MIME·duration·fidelity가 구현되지 않았다

- 위치:
  - `state/httpCapture.ts:133-151` — filter 형식은 query, method, status class,
    errors-only, window뿐이다.
  - `state/httpCapture.ts:153-183` — query는 method/url/host/path/status만 검색한다.
  - `HttpCapturePage.tsx:701-742` — 화면 control도 query/method/status/error만 있다.
  - `state/regression.test.ts:608-614` — 현재 제공되는 축만 검증한다.
- 계약: method/status/host/path/**MIME/duration/error/fidelity** filter.
- 문제: host/path는 통합 substring 검색으로 제한적으로 동작하지만 MIME,
  duration range, fidelity는 모델에 값이 있어도 선택할 수 없다.
- 영향: 대형 HAR에서 느린 요청, 특정 content type, fidelity가 낮은 transaction을
  분리할 수 없어 H-RG1의 분석 UI 완성 조건을 충족하지 못한다.
- 권고: host/path의 명시적 필터 또는 명확한 query semantics와 함께 MIME,
  min/max duration, fidelity 필터를 추가하고, brush와 조합된 교차 회귀 테스트를
  작성한다.

### U3. [P1] 새 입력 선택·분석 실패 시 이전 결과가 남아 provenance를 오인시킨다

- 위치:
  - `HttpCapturePage.tsx:80-102` — 분석 시작과 catch에서 `result`를 비우지 않는다.
  - `HttpCapturePage.tsx:138-139` — file select/clear가 file 상태만 바꾼다.
- 재현 흐름:
  1. HAR A를 성공적으로 분석한다.
  2. HAR B를 선택하거나 A를 clear한다.
  3. 아직 분석하지 않았거나 B 분석이 실패한다.
  4. A의 summary/timeline/tree/list가 B 선택 또는 B 오류 아래 계속 남는다.
- 영향: 증거 분석 도구에서 이전 결과가 현재 파일 결과처럼 보이는 것은 source
  provenance 오류다. 실패 상태와 정상 결과가 동시에 렌더링되어 오류 처리 계약도
  불명확하다.
- 권고: 파일 변경·clear·새 분석 시작 시 result/filter/detail을 일관되게 reset하고,
  실패 시 이전 결과를 유지하려면 입력 이름과 “이전 성공 결과” 상태를 명시적으로
  분리한다. A 성공 → B 선택 → B 실패 회귀 테스트를 추가한다.

### U4. [P2] 트랜잭션 상세가 request/response/timing/process 탭 계약과 쿠키 표시를 충족하지 않는다

- 위치: `HttpCapturePage.tsx:802-947`.
- 문제:
  - 계약은 request/response/timing/process **detail tabs**를 요구하지만 현재는 한
    세로 문서에 timing, request headers/body, response headers/body, process를 연속
    표시한다.
  - `CaptureHTTPMessage.cookies`와 `httpCaptureDetailCookies` i18n key가 존재하지만
    상세 화면은 cookies를 렌더링하지 않는다.
  - request/response content type과 body storage/redacted 상태도 상세에서 충분히
    설명되지 않아 MIME/fidelity 판단과 연결되지 않는다.
- 영향: 데이터는 일부 존재하지만 상세 탐색 계약과 evidence completeness가
  맞지 않는다. 특히 리댁션된 cookie의 존재 여부를 검증할 UI가 없다.
- 권고: tabs를 구현하고 cookies, content type, body storage/redacted state를 각
  메시지 탭에 포함한다. 각 탭의 empty/redacted 상태를 회귀 테스트한다.

### U5. [P2] timeline, transaction row, slide-over의 키보드 접근성이 불완전하다

- 위치:
  - `HttpCapturePage.tsx:455-492` — timeline은 pointer drag만 처리하고 keyboard
    selection control이나 slider semantics가 없다.
  - `HttpCapturePage.tsx:773-792` — transaction `<tr>`에 `onClick`만 있고 focus,
    button/link semantics, Enter/Space handler가 없다.
  - `SlideOverPanel.tsx:45-111` — Escape는 지원하지만 open 시 focus 이동·trap,
    close 후 trigger 복원, 닫힌 panel의 focus 차단(`inert`)이 없다.
- 영향: 키보드와 보조기술 사용자는 timeline window를 선택하거나 표에서 상세를
  열기 어렵고, dialog focus가 배경으로 빠질 수 있다.
- 권고: timeline에 keyboard 가능한 range/slider 또는 동등한 시작·끝 control을
  제공하고, transaction은 실제 button/link semantics를 사용한다. dialog는 초기
  focus, trap, restore, 닫힌 상태의 focus 차단을 보장한다.

### U6. [P2] Workspace 등록과 채워진 UI 상태의 통합 회귀 증거가 없다

- 위치:
  - `HttpCapturePage.tsx:87-93` — 성공 시 `addWorkspaceResult`를 호출한다.
  - `state/regression.test.ts:523-666` — 순수 tree/filter/window/redaction/timing만
    검증하고 page rendering, Workspace entry, error/partial interaction은 검증하지
    않는다.
- 문제: 호출 코드는 존재하지만 `http_capture` result type/source label/summary가
  Workspace에 보존되는지, timeline과 filter가 실제 렌더에서 같은 상태를 쓰는지,
  truncation/diagnostics/detail이 fixture 결과로 보이는지 자동 증거가 없다.
- 영향: `npm run test:state`와 build가 통과해도 H-RG1의 “Workspace regressions” 및
  채워진 화면 acceptance를 증명하지 못한다.
- 권고: 최소한 하나의 정상 HAR, degenerate timestamp, truncated detail,
  diagnostic/error HAR를 이용한 Wails 또는 component integration test를 추가한다.
  성공 결과의 Workspace 등록과 A→B 실패 provenance도 함께 고정한다.

## UI 판정과 재리뷰 조건

**판정: `CONDITIONAL`.** 현재 구현은 typed bridge, import-only 정직성, pseudo-process
tree, 기본 brush/filter, fidelity/redaction/diagnostic 노출과 EN/KO build 기반을
갖췄다. 그러나 U1의 denominator 불일치와 U2의 필터 누락은 H-RG1 PASS 기준을 직접
위반하고, U3는 현재 파일과 이전 결과를 혼동시킬 수 있다.

UI 재리뷰 전 필수 조건:

1. U1: selected window와 모든 필터의 summary/list/tree denominator를 하나의 계약으로
   일치시키고 회귀 테스트로 고정한다.
2. U2: MIME/duration/fidelity를 포함한 계약상 필터를 완성하고 조합 테스트를 추가한다.
3. U3: 파일 변경·clear·실패 시 stale result provenance를 제거한다.
4. U4/U5: 상세 tabs/cookies와 keyboard/dialog 접근성을 보완한다.
5. U6: fixture-driven 채워진 화면과 Workspace 등록 통합 회귀를 추가한다.
6. `npm run test:state`, production build, EN/KO key parity를 다시 통과한다.

이 UI 재리뷰 PASS는 H-RG1 전체 PASS의 한 구성요소일 뿐이다. 별도 Claude 엔진 리뷰와
H-SEC1 재리뷰가 모두 PASS된 뒤에만 H-RG1을 닫을 수 있다.
