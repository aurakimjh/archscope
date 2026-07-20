# T-574 build-vs-integrate spike — H1 MITM PoC (option 3)

`docs/ko/SYSTEM_HTTP_CAPTURE.md` 의 "프록시 build-vs-integrate 결정"에서 정의한
네 선택지 중 **추천안(옵션 3: 자체 H1 시맨틱 MITM + H2 passthrough)** 을 실증하는
최소 프로토타입이다. 결정이 종이 비교가 아니라 **동작하는 프로토타입**에 근거하도록
하는 것이 목적이다.

## 무엇을 증명하나

- **의존성 0** — 서드파티 프록시 라이브러리도, 번들 런타임도 없다. Go 표준
  라이브러리만으로 CONNECT + TLS 종단 + on-the-fly 인증서 + H1 파싱/포워딩/캡처가
  된다. 이것이 옵션 3을 옵션 1(라이브러리)·옵션 2(subprocess)보다 우선하는 핵심
  근거다: **단일 서명 바이너리**로 Windows 패키징·서명·업데이트가 단순해진다.
- **CA/인증서 수명주기** — 머신 로컬 CA를 만들고 호스트별 leaf를 연결 시점에
  발급한다(§11.2 CA lifecycle의 실측 뼈대).
- **H2 passthrough 결정** — ClientHello의 ALPN을 피킹해, h2 전용이면 가로채지 않고
  raw 터널로 통과시킨다(fidelity `unsupported`로 정직하게 기록). 반쪽짜리 H2 MITM을
  하지 않는다.
- **업스트림 검증 유지** — 업스트림 TLS 검증을 절대 끄지 않는다(§11의 요구).
  self-test가 이를 실제로 확인한다.

## 실행

```bash
# 오프라인 end-to-end 검증 (네트워크 불필요):
go run . -selftest
```

self-test는 인프로세스 HTTPS origin을 띄우고, **MITM CA만 신뢰하는** 클라이언트를
프록시로 통과시킨다. 클라이언트가 200 + origin 본문을 받으면 → 프록시가 유효한
leaf를 발급해 실제로 가로챘다는 뜻이고, 프록시는 origin을 origin 자체 CA로
검증한다(검증이 켜진 채로 동작함을 증명).

```bash
# 실제 프록시로 사용:
go run . -listen 127.0.0.1:8080
# 출력된 CA를 신뢰시키고 HTTPS 프록시를 127.0.0.1:8080 으로 지정하면
# 가로챈 트랜잭션이 JSON 라인으로 stdout에 찍힌다.
```

## 파일

| 파일 | 역할 |
|---|---|
| `ca.go` | 머신 로컬 CA 생성 + 호스트별 leaf 발급(캐시, IP/DNS SAN 처리) |
| `clienthello.go` | TLS ClientHello 피킹 → ALPN/SNI 파싱 → h2-only passthrough 판정 |
| `proxy.go` | CONNECT 처리, intercept(H1 종단·포워딩·캡처) / passthrough(raw 터널) |
| `main.go` | CLI + 오프라인 self-test |
| `util.go` | 바이트 카운팅 리더/커넥션 |

## 스파이크의 경계 (제품이 아님)

이 PoC가 **하지 않는** 것 = 옵션 3 구현 시 남는 실제 작업:

- 바디를 §7.6 bounded session store로 스트리밍하지 않는다(전량 통과만).
- 세션 커서 API, 트랜잭션 상태 기계(§6.3.3), INV 불변식(§6.3.4) 미구현.
- **PID 귀속은 미구현** — 운영 경로는 T-571에서 실측 검증한 `GetExtendedTcpTable`
  역조회(연결 로컬 포트 → owner PID)를 재사용한다. 여기서는 캡처에 넣지 않는다.
- keep-alive/parallel 연결, WebSocket, 1xx, trailer, 취소, upstream proxy, PAC 등
  §의 엣지 케이스 미처리.
- H2 **가로채기**(옵션 4)는 범위 밖 — 여기서는 passthrough만.

이 목록이 곧 옵션 3의 잔여 공수 신호이며, 리스크 레지스터의 근거다.
