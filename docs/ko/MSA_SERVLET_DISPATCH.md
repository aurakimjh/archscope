# MSA 응답시간의 서블릿 디스패치

MSA 응답시간 구성은 서블릿 디스패치 오버헤드를 일반 메소드 시간과 분리합니다.
Servlet Dispatch는 `internal` 그룹입니다. 수신측 `HttpServlet.service()` 프레임이
컨트롤러 업무를 감싸며, 이 프레임의 self-time만 이 카테고리에 포함됩니다.

## 분류

다음 기본 패턴을 대소문자 구분 없는 부분 문자열로 찾습니다.

- `jakarta.servlet.http.HttpServlet.service`
- `javax.servlet.http.HttpServlet.service`

`servletDispatchPatterns`는 기본 패턴에 사용자 패턴을 추가합니다. 빈 값과
대소문자만 다른 중복은 제거합니다. 내장 SQL·fetch·외부호출 분류가 우선하며,
Network Prep 매칭 후 남은 `METHOD`/`UNKNOWN` 이벤트에만 dispatch 패턴을 적용합니다.

## 계산

```text
Servlet Dispatch = service elapsed − union(모든 직접 자식 구간)
Method Time = root 응답시간 − 이미 분류한 카테고리 합계
```

일반 컨트롤러 메소드·SQL·외부호출을 포함한 모든 직접 자식을 차감하고, 겹친 구간은
한 번만 셉니다. 컨트롤러 업무는 일반 메소드 시간에 남고 SQL·외부호출은 각자의
카테고리를 유지합니다. Network Prep는 호출측 wrapper에서 외부호출 자식만 빼므로
다른 계산입니다. 일반 커스텀 분류 룰은 plain 메소드 자식을 차감하지 않아 dispatch
self-time을 대신할 수 없습니다.

예를 들어 service 프레임이 100 ms이고 컨트롤러 자식의 합집합이 80 ms이면 Servlet
Dispatch는 20 ms입니다. 유효한 입력에서 분류한 합계와 남은 메소드 시간은 root
응답시간과 같습니다. 분류한 시간이 root를 초과하면 기존 `NegativeMethodTime`
처리가 남은 메소드 시간을 0으로 제한하고 불일치를 표시합니다. 잘못된 입력에 대해
상호 배타적인 시간 계산이 보장된다는 뜻은 아닙니다.

## 저장 필드와 표시

| 영역 | 식별자 |
|---|---|
| 파싱 이벤트 | `JenniferEventServletDispatch` / `SERVLET_DISPATCH_METHOD` |
| 응답시간 구성 | `ServletDispatchMs` / `servlet_dispatch_ms` |
| Body 누적 | `ServletDispatchCumMs`, `ServletDispatchCount` |
| 파서 옵션 | `ServletDispatchPatterns` |
| 데스크톱 옵션 | `servletDispatchPatterns` |
| 차트 그룹 | `internal` |

MSA 차트는 Servlet Dispatch를 별도 조각으로 표시합니다. 이 이벤트는 일반 메소드
hotspot 순위에서 제외하되 자식 시간 차감을 위한 containment 트리에는 유지합니다.
커스텀 응답시간 계산도 전용 bucket을 인식합니다. 저장 결과의 기존 값은 유지하며,
패턴을 바꾸려면 원본 프로파일을 다시 분석해야 합니다.

파서 classifier와 Jennifer profile aggregator가 이 의미를 소유합니다.
[영문 가이드](../en/MSA_SERVLET_DISPATCH.md)도 같은 내용을 설명합니다.
