# MSA 응답시간 — Servlet Dispatch 카테고리 추가 (설계 노트)

## 1. 배경

MSA 타임라인의 **응답시간 구성**(`MsaResponseTimeBreakdown`)은 root 응답시간을
카테고리로 분해하고, 분류되지 않은 나머지를 단일 잔여값(`method_time_ms`,
화면상 "Method / 미분류")으로 둔다.

```
root = SQL + CheckQuery + 2PC + Fetch + Network(gap) + UnprofiledExternalCall
       + NetworkPrep + ConnAcquire + (custom slices) + MethodTime
MethodTime(미분류) = root − covered(위 합계)
```

기존 **Network Prep**은 *caller* 측 개념이다. `sendToService` 같은 HTTP 클라이언트
wrapper 메소드를 `NETWORK_PREP_METHOD`로 분류한 뒤,
`준비시간 = wrapper elapsed − 감싼 EXTERNAL_CALL elapsed`로 계산한다(external 그룹).

이번에 추가하는 것은 그 **callee(수신측) 대칭** 개념이다. 제니퍼 프로파일에서
`jakarta.servlet.http.HttpServlet.service`가 컨트롤러 업무 메소드를 감싸고,
그 `service()` 프레임의 self-time(컨트롤러를 제외한 서블릿/프레임워크 dispatch
구간)이 현재 미분류에 섞여 있다. 이를 **Servlet Dispatch**라는 독립 카테고리로
분리하고 미분류에서 제거한다.

## 2. 정의와 계산식 (확정)

- **명칭**: Servlet Dispatch / 서블릿 디스패치
- **응답시간 그룹**: `internal`
- **계산 범위**: `service()` 프레임의 **self-time** —
  `service elapsed − union(모든 직접 자식 구간)`. 즉 컨트롤러 업무 메소드 + SQL +
  외부호출 등 **모든 자식**을 차감한다. (`method_hotspots.go`의 self-time 로직과 동일.)

> Network Prep와의 차이: Network Prep는 자식 중 EXTERNAL_CALL **한 종류만** 차감하지만,
> Servlet Dispatch는 **모든 자식**을 차감한다. 이렇게 해야 컨트롤러 업무 시간은
> 미분류(업무)로 남고 dispatch 오버헤드만 분리된다. 커스텀룰 방식
> (`customRuleMethodElapsed`)은 plain 메소드 자식을 차감하지 않으므로 이 용도에
> 부적합하다(컨트롤러 업무시간까지 흡수됨).

### 더블카운트 검증

`Servlet Dispatch = service self-time`은 SQL/외부호출 자식을 이미 제외하므로
그 카테고리들과 겹치지 않는다. 컨트롤러 업무 self-time은 Servlet Dispatch에도,
covered에도 포함되지 않으므로 그대로 미분류에 남는다. 따라서
`covered + 미분류 = root` 항등식이 유지된다.

## 3. 식별자 (네이밍)

| 영역 | 식별자 |
|---|---|
| EventType 상수 | `JenniferEventServletDispatch` |
| EventType 값(JSON) | `SERVLET_DISPATCH_METHOD` |
| breakdown 필드 | `ServletDispatchMs` / `servlet_dispatch_ms` |
| BodyMetrics 누적 | `ServletDispatchCumMs` / `servlet_dispatch_cum_ms` (+ count, 선택적 상세리스트) |
| 기본 패턴 변수 | `defaultServletDispatchPatterns = ["jakarta.servlet.http.httpservlet.service"]` |
| 헬퍼 | `ServletDispatchPatternsWithDefaults([]string) []string` |
| Options 필드 | `ServletDispatchPatterns []string` |
| 옵션 JSON(프론트) | `servletDispatchPatterns` |
| 프론트 슬라이스 key | `servlet_dispatch_ms` (group: `internal`) |
| 표시 라벨 | 서블릿 디스패치 / Servlet Dispatch |

## 4. 변경 지점

### 백엔드 (Go)

1. **`internal/models/jennifer_profile.go`**
   - `JenniferEventServletDispatch JenniferEventType = "SERVLET_DISPATCH_METHOD"` 추가.
   - `JenniferResponseTimeBreakdown`에 `ServletDispatchMs int json:"servlet_dispatch_ms"` 추가.
   - `JenniferBodyMetrics`에 `ServletDispatchCumMs`, `ServletDispatchCount`
     (+ 선택: `ServletDispatchMethods []JenniferServletDispatchMethod` 상세리스트) 추가.

2. **`internal/parsers/jenniferprofile/event_classifier.go`**
   - `defaultServletDispatchPatterns = ["jakarta.servlet.http.httpservlet.service"]`.
   - `ServletDispatchPatternsWithDefaults` 헬퍼 (NetworkPrep 헬퍼와 동형).
   - `classifyEventsWithOptions`에서 `METHOD`/`UNKNOWN` 대상으로 Network Prep
     매칭 다음 우선순위로 Servlet Dispatch 매칭 추가.

3. **`internal/parsers/jenniferprofile/parser.go`**
   - `Options`에 `ServletDispatchPatterns []string` 추가.

4. **`internal/analyzers/jenniferprofile/aggregator.go`**
   - `SERVLET_DISPATCH_METHOD` 프레임의 self-time 누적 → `ServletDispatchCumMs`.
     `containmentParents` / self-time 계산을 재사용 (모든 자식 union 차감).

5. **`internal/analyzers/jenniferprofile/msa.go`** (`computeGroupMetrics`)
   - 프로파일별 `ServletDispatchCumMs` 합산.
   - `bd.ServletDispatchMs` 설정 + `covered` 합계에 포함 → `method_time_ms` 자동 감소.
   - `applyCustomBreakdownRules` 이후에도 `recomputeBreakdownRatios`의 covered 식에 포함.

6. **`internal/analyzers/jenniferprofile/method_hotspots.go`**
   - `isMethodFrameEvent`는 `METHOD`만 인정하므로 새 타입은 **자동으로 hotspots 랭킹 제외**.
     단, containment 트리에는 계속 참여(자식 차감용)하도록 `isStructuralEvent`에는 넣지 않음(현 상태 유지).

7. **`internal/analyzers/jenniferprofile/custom_rules.go`**
   - `breakdownBucketForEvent`에 `SERVLET_DISPATCH_METHOD → "servlet_dispatch_ms"` 추가.
   - `customRuleDeductsChildEvent`에 새 타입 추가(커스텀룰이 이 구간을 자식으로 차감).
   - `subtractBreakdownBucket` / `recomputeBreakdownRatios` covered 식에 `servlet_dispatch_ms` 반영.

8. **테스트**: `analyzer_test.go`, `method_hotspots_test.go`,
   event_classifier 관련 테스트 — 분류·self-time·미분류 감소·hotspots 제외 케이스 추가.

### 프론트엔드 (TS/React)

9. **`cmd/archscope-app/engineservice.go`** + **`frontend/src/bridge/engine.ts`**
   - 요청 구조체/인터페이스에 `servletDispatchPatterns?: string[]` 추가 후 Options로 전달
     (NetworkPrepPatterns와 동일 배선).

10. **`frontend/src/components/MsaResponseTimeBreakdown.tsx`**
    - `SLICE_DEFS`에 `{ key: "servlet_dispatch_ms", label: "Servlet Dispatch", group: "internal", color: "#f97316", hint: "service() 등 프레임워크 dispatch self-time" }` 추가.
    - `GroupMetrics.response_time_breakdown` 타입에 `servlet_dispatch_ms?` 추가.

11. **`frontend/src/pages/JenniferProfilePage.tsx`**
    - `MSA_EVENT_SEGMENTS`에 `{ id: "SERVLET_DISPATCH_METHOD", label: "Servlet dispatch (service 등)" }` 추가.
    - `MSA_EVENT_PRESETS`에 기본 프리셋 추가
      (`SERVLET_DISPATCH_METHOD: ["jakarta.servlet.http.HttpServlet.service"]`).
    - NetworkPrep처럼 `SERVLET_DISPATCH_METHOD`를 에디터에서 분리해 전용 옵션
      (`servletDispatchPatterns`)으로 split 전송. 기본값 안내 문구 추가.

12. **i18n / help**: `frontend/src/i18n/messages.ts`, `frontend/src/help/helpCatalog.ts`
    라벨·도움말 추가.

## 5. 불변식 / 주의

- 내장 분류(EXTERNAL_CALL/FETCH/SQL 등)가 항상 우선. 새 패턴은 `METHOD`/`UNKNOWN`에만 적용.
- 기본 패턴은 항상 활성(사용자 패턴은 추가로 확장, NetworkPrep와 동일 정책).
- `service self-time` 계산은 모든 자식을 union 차감 → 컨트롤러 업무시간은 미분류 유지.
- covered가 root를 초과하면 기존처럼 `NegativeMethodTime`으로 0 클램프.
