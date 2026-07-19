# 시스템 전체 HTTP 캡처 설계 검토

- 작성일: 2026-07-19
- 검토자: Codex
- 대상: `docs/ko/SYSTEM_HTTP_CAPTURE.md` (commit `cc45896` 기준)
- 관련 조사: 별도 조사 파일은 없으며, 같은 설계서의 1.4~1.5절과 HAR·보안·UX
  관련 절에 2026-07-18 경쟁 솔루션 조사 결과가 통합되어 있음
- 범위: 제품 범위, HAR 정규화, MITM 프록시, 프로세스 귀속, 타이밍·wire
  fidelity, 장시간 스트리밍, `AnalysisResult`/Wails 경계, 보안·프라이버시,
  기존 코드와의 정합성
- 운영 전제 보완(2026-07-19 사용자 확인): ArchScope 데스크톱 UI와 실시간 캡처는
  Windows-first이며, Linux/macOS에서 생성된 프로파일·로그·HAR은 Windows UI에서
  오프라인 분석한다. Linux/macOS live capture parity는 1차 출시 조건이 아님.
- 검토 방식: 설계서 전체와 현재 Go/Wails 코드 정적 대조, Linux·Windows·Go·
  Chromium·Wails 및 경쟁 도구 공식 문서 교차 확인. 구현 변경 없음.

## 판정

**Phase 1(HAR 가져오기·사후 분석)은 P0 계약을 보완한 뒤 조건부 승인하고,
Phase 2 이상의 실시간 MITM 캡처는 설계를 수정하기 전까지 보류한다.**

Windows-first라는 운영 전제로 범위를 좁히면 세 플랫폼의 실시간 캡처를 동시에
완성해야 하는 부담은 사라진다. 따라서 1차 구현 순서는 **Windows UI에서 이종 OS
증거 가져오기 → Windows MITM proxy → Windows ETW/WFP capability 검증**으로
조정한다. 다만 Windows의 일반 network performance counter도 adapter 단위이고,
`GetExtendedTcpTable`은 PID가 붙은 endpoint table이지 byte counter가 아니므로
coverage·wire fidelity·streaming 블로커는 그대로 남는다.

설계의 큰 방향은 좋다. 캡처와 분석을 파일 경계로 분리하고, HAR 방언을 정규화
단계로 격리하며, 데이터 충실도와 누락을 사용자에게 노출하고, 장시간 세션을
디스크 기반으로 처리하려는 판단은 ArchScope의 parser/analyzer/exporter/UI 분리
원칙과 잘 맞는다. 경쟁 솔루션 조사도 단순 기능표를 넘어 채택할 UX와 피해야 할
안티바이러스·메모리·CA 비용을 설계 결정에 연결했다.

그러나 현재 문서에는 구현 불가능하거나 서로 양립할 수 없는 계약이 남아 있다.
가장 큰 문제는 다음 다섯 가지다.

1. Windows의 adapter counter나 TCP endpoint table만으로 프로세스별 전체
   네트워크 바이트를 계산할 수 없으므로 커버리지 판정 전제가 성립하지 않는다.
2. 원본 헤더 순서·전송 압축 바디·클라이언트 관점 타이밍을 보존한다는 요구가
   제안된 Go `net/http` 기반 모델과 맞지 않는다.
3. 유계 메모리, 비차단 수집, 기록 무손실을 동시에 약속하면서 디스크 지연·가득 참
   시의 정책을 정의하지 않았다.
4. 전체 세션을 `AnalysisResult`에 넣다가 5,000건부터 지연 로딩으로 바꾸는 계약은
   동일한 result type의 의미를 데이터 크기에 따라 바꾸고 Wails IPC를 다시
   비대하게 만든다.
5. JWT payload 유지, 무염 쿠키 해시, content hash blob, 자동 TLS 패스스루 등은
   “기본 안전”이라는 보안 목표를 충족하지 못한다.

## 관련 솔루션 조사 검토

### 확인 결과

별도 `research` 문서는 추가되지 않았다. commit `cc45896`은
`SYSTEM_HTTP_CAPTURE.md` 한 파일만 변경했고, 조사 결과는 다음 위치에 통합됐다.

- 1.4: HTTPAnalyzer 제품 상태, 캡처 구조, UI, 드라이버·후킹 비용
- 1.5: Fiddler, Charles, mitmproxy, Proxyman, Wireshark, DevTools, Burp 비교
- 6.5: HAR 방언, 검증, 민감정보 소독, 참조 구현
- 7.4·8.6·8.7: 경쟁 도구에서 차용한 실시간·타임라인 UX
- 8.8: 장시간 캡처의 메모리 한계

이 통합 방식은 “조사 결과가 어느 설계 결정에 쓰였는가”를 읽기 쉽다는 장점이 있다.
특히 다음 결론은 설계에 유용하게 반영됐다.

- Charles도 로컬 client process 이름을 제공한다는 정정
- HAR을 단일 규격이 아니라 생성 도구별 방언으로 다루는 결정
- 가져온 HAR도 안전하다고 가정하지 않고 저장 전에 리댁션하는 결정
- 실시간 목록과 전체 집계를 분리하고, 표시 필터를 비파괴적으로 두는 UX
- 커널 드라이버·TLS 후킹보다 HAR import와 명시적 프록시를 먼저 검토하는 순서

### 보완이 필요한 조사 품질

`[V]`, `[I]`, `[?]` 표시는 유용하지만 **각 `[V]`를 재현할 URL, 문서 버전 또는
commit, 조회일이 문서에 없다.** 가격·제품 버전·기능은 변하는 정보이므로 현재
표만으로는 다음 검토자가 검증하거나 갱신할 수 없다. 최소한 문서 말미에
`주장 ID / 출처 URL / 문서 또는 commit 버전 / 조회일 / 인용한 사실 / 설계 영향`
형태의 출처 원장이 필요하다.

또한 “HTTP 도구 전 범주에 시계열이 없다”, “메모리 상한과 축출이 있는 도구가
하나도 없다”는 소거법 기반 절대 명제는 검증 범위를 함께 적어야 한다. 공식 문서에
기능이 없다는 사실과 제품에 기능이 없다는 사실은 같지 않다. “2026-07-18 기준,
조사한 공식 문서와 공개 UI에서 확인하지 못했다” 정도로 낮추는 것이 방어 가능하다.

mitmproxy의 경우 공식 문서는 local capture가 전체 장치 또는 process name/PID를
대상으로 삼을 수 있다고 명시한다. 설계서의 “수집 이후 PID 필드가 보존되지 않는다”는
더 구체적인 코드 수준 주장일 수 있지만, 해당 source commit과 검색 경로를 출처
원장에 남겨야 한다. 반대로 Charles의 client process 이름·로컬 전용·연결 수락 지연
설명은 공식 문서와 일치한다.

공식 근거:

- [mitmproxy Proxy Modes — Local Capture](https://docs.mitmproxy.org/stable/concepts/modes/)
- [Charles Client Process Tool](https://www.charlesproxy.com/documentation/tools/client-process/)
- [FiddlerCore startup settings](https://docs.telerik.com/fiddlercore/api/fiddler.fiddlercorestartupsettings)
- [Chromium DevTools HAR export UI/source](https://chromium.googlesource.com/devtools/devtools-frontend/+/HEAD/front_end/panels/network/NetworkPanel.ts)

## P0 — 구현 전에 해소해야 할 블로커

### 1. Windows에서도 프로세스별 커버리지 바이트 전제가 자동으로 성립하지 않는다

설계는 `bytesObserved`와 `bytesSystem`을 비교해 프록시 우회 프로세스를 찾는다
(`SYSTEM_HTTP_CAPTURE.md:2024-2045`). Windows-first로 좁혀도 이 전제는 바로
성립하지 않는다. Windows 공식 network performance counter의 byte 지표는
Network Interface/Adapter 단위다. `GetExtendedTcpTable`은 owner PID가 붙은 TCP
endpoint를 제공하지만 연결별 누적 byte counter는 제공하지 않는다. ETW TCP/IP
event는 send/receive와 process/flow 분석 후보지만, event header의 PID를 그대로
발신 process로 쓰지 말라는 공식 주의가 있어 실제 event payload와 손실률을 먼저
검증해야 한다.

Linux `/proc/<pid>/net`은 **해당 프로세스가 속한 network namespace의 네트워크
스택 정보**이며 프로세스 단독 카운터가 아니다. 이 문제는 Linux live capture가
후순위가 되면서 1차 Windows release blocker에서는 내려가지만, 향후 Linux
capability를 표시할 때 같은 잘못을 반복하지 않도록 남겨 둔다. macOS의 `nettop`을
받치는 안정적인 공개 per-process API도 설계에 제시되지 않았다.

이 문제는 Phase 3 한 항목의 오류가 아니다. 설계가 Phase 2의 부분 커버리지를
정직하게 보완하는 핵심 장치로 10절을 사용하므로 제품 신뢰성의 P0다.

권장 수정:

- `CoverageEvidence`에 `scope`(`process|cgroup|network_namespace|adapter|host`),
  `source`, `privilege`, `samplingInterval`, `confidence`를 명시한다.
- 서로 같은 scope인 값만 비율로 비교하고, 그 외에는 “참고 신호”로만 표시한다.
- Windows Phase 2 완료 조건은 per-process byte 비교가 아니라 `captured / passthrough /
  unattributed / dropped / unsupported`의 관측 가능한 카운터로 정의한다.
- proxy bypass 탐지는 Windows ETW/WFP 또는 Npcap 대조가 실제로 가능해진 뒤
  승격한다. Linux eBPF는 후속 capability다.
- Windows proof-of-capability spike가 실패하면 10.1의 절대적 coverage ratio를
  제거한다.

공식 근거:

- [Linux proc_pid_net(5)](https://man7.org/linux/man-pages/man5/proc_pid_net.5.html)
- [Microsoft network-related performance counters](https://learn.microsoft.com/en-us/windows-server/networking/technologies/network-subsystem/net-sub-performance-counters)
- [Microsoft GetExtendedTcpTable](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable)
- [Microsoft TCP/IP ETW events](https://learn.microsoft.com/en-us/windows/win32/etw/tcpip)

### 2. wire fidelity 요구와 `net/http` 구현안이 충돌한다

R3은 원본 헤더 순서와 중복 보존을 요구하고(`:400`), `HeaderField` slice만 쓰면
가능하다고 본다(`:861-878`). 하지만 Go `http.Header`는
`map[string][]string`이며 field name을 canonicalize한다. map을 거친 뒤에는 서로
다른 헤더 이름의 원래 순서와 원래 casing을 복구할 수 없다. 일부 중복 응답 헤더는
결합될 수도 있다. HTTP/2에서는 pseudo-header, HPACK decode, trailer를 포함해
“원본 순서”라는 요구의 의미부터 HTTP/1.1과 다르다.

`BodySize`를 “실제 전송(압축 상태)”으로 정의한 부분(`:864`)도 기본
`http.Transport`와 맞지 않는다. Transport가 gzip을 자동 요청한 경우 응답 body를
투명하게 해제하고 `Content-Length`와 `Content-Encoding`을 지운다. 원본 전송
바이트가 필요하면 automatic decompression을 끄고 wire byte 계측을 별도로 해야
한다. chunked framing, HTTP/2 frame byte, decoded entity byte도 분리해야 한다.

타이밍 모델은 하나의 `Timings`만 정의하면서 프록시의 client↔proxy와
proxy↔upstream 두 관점을 모두 저장한다고 적어 내부 모순이다(`:922-940`). MITM
프록시는 원래 클라이언트의 DNS·서버 TCP·서버 TLS 수행을 직접 보지 못한다.
`WaitMs // TTFB — 서버 처리`도 부정확하다. TTFB에는 프록시 큐, 네트워크 왕복,
서버 처리 등이 함께 들어가며 서버 처리 시간만 의미하지 않는다.

권장 수정:

- fidelity level을 `semantic`, `decoded_wire`, `raw_wire`처럼 명시하고 캡처 모드별
  보장 표를 만든다.
- Phase 2의 기본 보장은 semantic headers와 decoded body로 낮춘다. exact HTTP/1.1
  wire 보존은 별도 raw parser/spool 경로가 있을 때만 주장한다.
- `Timings`를 `clientProxy`, `proxyInternal`, `proxyUpstream`, `importedHar` 관점으로
  분리하고 각 값에 `known|not_applicable|unknown` 상태를 둔다. plain `float64`의
  0으로 세 상태를 표현하지 않는다.
- connection reuse는 `Connection.Reused`가 아니라 transaction의
  `UsedExistingConnection` 또는 connection 내 sequence로 표현한다.
- `StartedAt`, `EndedAt`, `State`, byte counters의 측정 지점을 주 모델에 포함한다.
- HTTP/1.1/H2, 1xx, CONNECT, streaming, trailers, WebSocket upgrade, cancellation,
  decompression의 fidelity golden test를 정의한다.

공식 근거:

- [Go net/http Header와 Response.Uncompressed](https://pkg.go.dev/net/http)

### 3. 무손실 기록·비차단 수집·유계 메모리가 동시에 보장되지 않는다

7.4는 Capturer가 논블로킹 채널로 보내면서 기록 계층은 절대 drop하지 않는다고
한다(`:1396-1406`). 그러나 디스크 처리량이 유입량보다 낮거나 디스크가 가득 차면
다음 셋을 동시에 만족할 수 없다.

- producer를 block하지 않음
- 메모리를 유계로 유지
- 모든 transaction을 손실 없이 기록

현재 설계에는 이 상황의 backpressure, capture 중단, body downgrade, 손실 카운터,
disk reserve, fsync/flush, crash recovery 정책이 없다. UI 이벤트의
`capture:aggregate`와 `capture:stats`도 “drop 불가”라고만 했는데, Wails events는
emit/listen 메커니즘이지 전달 보장·재연결·ack 프로토콜이 아니다.

시간 버킷 역시 엄밀히 상수 크기가 아니다. 각 bucket의
`ByProcess map[ProcessKey]int`는 프로세스 cardinality만큼 자라고, bucket별
t-digest까지 합치면 2,000개 상한만으로 메모리 상한을 보장할 수 없다. 적응형 인접
병합은 완료 순서와 장기 요청 도착 시점에 따라 결과가 달라질 수 있어 `Add`의
order-independent 요구(`:1545-1570`)도 자동으로 성립하지 않는다.

권장 수정:

- 기록 queue의 byte 상한과 high-water mark를 정의한다.
- disk full/slow 시 기본 동작을 “명시적 오류 후 capture 중지”로 정하고, 선택적으로
  body를 omit하는 degradation을 별도 정책으로 둔다. 어떤 경우에도 조용히
  downgrade하지 않는다.
- `captured`, `persisted`, `bodyOmitted`, `eventSkipped`, `kernelDropped`,
  `parseFailed`를 서로 다른 monotonic counters로 둔다.
- Wails event에는 `sessionId`, monotonic `sequence`, `snapshotVersion`을 넣고,
  이벤트 유실·페이지 재진입 시 `GetCaptureSnapshot(sinceVersion)`으로 복구한다.
- bucket 경계를 session epoch와 resolution으로 결정하고 deterministic compaction을
  정의한다. 실시간 완료 순서와 batch 시작 순서를 섞은 property test를 둔다.
- process breakdown은 top-K + other 또는 별도 bounded dimension store로 분리한다.

공식 근거:

- [Wails Events API](https://wails.io/docs/reference/runtime/events/)

### 4. 세션 저장소와 `AnalysisResult`의 역할 경계가 불안정하다

설계는 `Tables.transactions`와 `Metadata.Extra.captureSession`에 상세 트리를 넣고,
5,000건부터 요약 트리와 Wails 지연 로딩으로 바꾼다(`:1253-1271`). 현재
`Metadata.Extra`는 marshal 시 metadata에 그대로 flatten되므로
(`apps/engine-native/internal/models/analysis_result.go:147-181`) 전체 트리가 Wails
JSON IPC와 report JSON에 실린다. transaction table과 원본 트리도 중복된다.

같은 `http_capture` 타입이 4,999건일 때와 5,001건일 때 다른 self-contained 계약을
갖게 되며, 임계치가 임의 수치라는 점도 문서가 인정한다. export CLI는 generic
JSON을 렌더할 수 있지만, 그것이 전용 HTTP report와 의미 있는 session diff가
“변경 없이” 동작한다는 뜻은 아니다. 현재 generic report diff는 summary의 숫자
필드와 finding 수만 비교한다. URL template/host/process/time-window 단위 비교는
별도 analyzer가 필요하다.

또한 ingestion core family에 `http_capture`가 없고, 현재 Sidebar/App에도
“브라우저 성능 분석” 또는 “네트워크 분석” 그룹이 구현되어 있지 않다. 다른 설계
노트에 예정됐다는 이유로 현재 코드에 존재한다고 표현하면 phase 의존성이 흐려진다.

권장 수정:

- `AnalysisResult`는 항상 bounded summary/series/findings와 세션 참조만 담는다.
  세션 크기에 따라 계약을 바꾸지 않는다.
- 상세 transaction/process/connection/body는 versioned `CaptureSessionStore`에서
  cursor pagination으로 읽는다. cursor에 snapshot/version을 포함한다.
- report/export용으로는 store를 읽어 bounded 집계를 만드는 전용 projection을 둔다.
- session diff는 generic report diff가 아니라 URL normalization, dimension,
  denominator, time-window를 명시한 HTTP 전용 analyzer로 둔다.
- ingestion `SourceKind`, family spec, detector, CLI leaf, Wails binding을 Phase 1의
  명시적 계약에 추가한다.
- 사이드바 그룹은 `.cpuprofile` 구현을 전제로 하지 말고 현재 코드 기준의 독립
  navigation migration으로 적는다.

### 5. 보안 기본값을 더 강하게 잡아야 한다

가져오기와 저장 시점 리댁션, CA 개인키 미동봉, 원격 캡처 금지, 캡처 중 명시 표시
원칙은 적절하다. 다만 다음 세부는 기본 안전 목표와 충돌한다.

- JWT에서 signature만 제거하면 토큰 재사용은 막아도 payload claim의 이메일,
  subject, tenant, role, 내부 식별자는 그대로 노출된다.
- cookie 값을 일반 hash prefix로 바꾸면 동일성·사전대입 공격에 취약하다. session별
  random key를 사용한 HMAC 또는 기본 완전 삭제가 필요하다.
- body blob을 content hash로 이름 붙이면 세션 간 동일성 누출과 알려진 작은 값의
  사전대입이 가능하다.
- `manifest`의 hash는 우발적 손상 탐지는 하지만 서명/HMAC 없이는 변조 방지가
  아니다. “무결성”의 의미를 corruption detection으로 한정해야 한다.
- command line, exec path, user는 토큰·홈 경로·고객명 같은 HTTP 밖의 민감정보를
  포함할 수 있으나 process metadata 리댁션 정책이 없다.
- 사용자 regex는 catastrophic backtracking 또는 과도한 scanning 비용을 만들 수
  있으므로 엔진, 길이, 시간, 대상 크기를 제한해야 한다.
- TLS handshake 실패를 곧바로 pinning으로 간주해 자동 passthrough하면 CA 미신뢰,
  protocol mismatch, expired cert, client auth 실패까지 우회한다. 첫 연결은 이미
  실패했으므로 동작도 투명하지 않다.

권장 수정:

- 위협 모델을 local user, shared workstation user, exported-file recipient,
  malicious HAR, compromised CA key, crash dump로 나눠 작성한다.
- 기본 정책은 JWT payload와 cookie value 완전 삭제로 하고, 상관관계 보존은
  session-scoped HMAC을 명시적으로 선택한 경우만 허용한다.
- blob identifier는 random opaque ID로 두고 content hash는 필요하면 manifest
  내부의 keyed integrity 값으로 제한한다.
- process metadata도 field별 수집 등급·리댁션·export 제외 정책을 갖게 한다.
- passthrough는 실패 원인을 진단한 뒤 사용자 확인 또는 사전 allowlist로만
  활성화하고 `(process identity, host, port, expiry)` 범위를 명시한다.
- imported HAR는 구조·field count·string/body size·nesting depth를 먼저 제한하고,
  schema violation을 fatal structural error와 recoverable dialect warning으로 나눈다.

## P1 — 설계 완성도를 위한 보완

### 6. 프록시 구현은 build-vs-integrate 결정을 먼저 해야 한다

Go 표준 라이브러리로 CONNECT와 TLS 종단을 시작할 수 있다는 사실과, 장시간 안정적인
HTTP/1.1+HTTP/2 debugging proxy를 만드는 일은 다르다. hop-by-hop header, proxy
authentication, ALPN, H2 flow control, trailers, streaming, cancellation, 1xx,
WebSocket, upstream proxy, IPv6, PAC/no_proxy를 모두 다루면 Phase 2는 독립 제품에
가깝다. 경쟁 솔루션 조사는 이 영역의 성숙도를 보여주지만 정작 build-vs-integrate
결정에는 사용되지 않았다.

Phase 0에서 최소한 다음 선택지를 spike하고 기록해야 한다.

- 검증된 Go proxy library를 in-process로 사용
- mitmproxy 등의 local subprocess를 adapter 뒤에 사용
- H1 semantic proxy를 제한된 MVP로 구현하고 H2는 passthrough
- 처음부터 자체 H1/H2 proxy를 구현

각 선택지는 라이선스, binary size, 보안 업데이트, process attribution hook,
raw fidelity, crash isolation, **Windows 설치·서명·업데이트**를 우선 비교하고,
cross-platform packaging은 후속 항목으로 둔다.

### 7. 캡처 수명주기와 복구 계약이 부족하다

`Capturer.Start`는 error만 반환하고 `Stop()`에는 session ID가 없는데, Wails
`StartCapture`는 즉시 session ID를 반환한다고 한다. Start가 blocking인지,
background goroutine을 만드는지, 동시 세션을 허용하는지, stop의 idempotency와
timeout이 무엇인지 정해지지 않았다.

`CaptureSession` state machine을
`created → starting → running → stopping → finalized|failed|recoverable`로 두고,
모든 operation을 session ID 기준으로 만들어야 한다. 비정상 종료 후 incomplete
NDJSON의 마지막 유효 record까지 복구하고 manifest를 재생성하는 규칙도 필요하다.

### 8. HAR importer의 호환성과 검증 코퍼스를 분리해야 한다

설계가 7개 creator를 열거하면서 “네 개의 방언”이라고 요약하는 부분은 내부적으로
맞지 않는다. dialect는 제품 개수와 같지 않을 수 있으므로 `DialectID`와 feature
matrix로 정의해야 한다. strict schema failure를 warning으로 계속 진행한다는 정책도
구조 필수 필드 누락과 vendor semantic 차이를 구분해야 한다.

Phase 1은 구현 전에 다음 sanitized fixture를 `../projects-assets`에 추가하고
`ASSET_INDEX.md`에 등록해야 한다.

- Chrome sanitized/with-sensitive-data, H1/H2
- Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic HAR
- BOM, missing content, base64 without encoding, `bodySize=-1`, identical timestamp
- oversized/deep/malformed JSON, decompression bomb, secret-bearing query/body/WebSocket

각 fixture에는 생성 제품·버전·명령, 원본 보관 여부, sanitization 방법, 기대
fidelity와 diagnostic을 기록해야 한다.

### 9. Windows 실행과 이종 OS 증거 분석을 capability tier로 분리해야 한다

R1은 “시스템 모든 프로세스”라고 쓰면서 수용 기준은 browser/curl/JVM/Electron
각 1개 캡처이고, Phase 2는 다시 R1 “부분”만 만족한다고 한다. 하나의 requirement가
동시에 필수·부분·향후 목표가 되면 release gate로 쓸 수 없다.

Windows-first 운영 전제를 반영해 다음처럼 분리하는 편이 검증 가능하다.

- Tier A: Windows UI에서 HAR·프로파일·로그 import. 증거 생성 OS는 무관
- Tier B: Windows explicit/system proxy를 준수하는 local applications
- Tier C: Windows ETW/WFP/Npcap 기반 metadata coverage
- Tier D: Windows decrypted semantic coverage, pinning/QUIC 제외
- Tier E: Linux/macOS live capture. 후속 capability이며 Windows 출시 gate 아님

각 플랫폼·프로토콜·앱 유형에 `supported|partial|unsupported|not_tested`와 fidelity를
명시하고 Phase별로 승격할 tier를 정한다.

## P2 — 문서 품질과 운영 준비

### 10. 용어와 링크를 현재 저장소 기준으로 정리해야 한다

- 설계서의 `internal/...`, `cmd/...` 경로는 현재 실제 루트인
  `apps/engine-native/...`를 반영해야 한다.
- `12.1 커버리지 리포트`처럼 실제 절 번호가 10.1인 교차 참조를 정리해야 한다.
- 연구 결과와 설계 결정을 한 파일에 유지한다면 source ledger를 별도 부록으로 두고,
  변경이 잦은 가격·버전 표에는 “as of”를 유지한다.
- 영어 문서, importer support matrix, data model, security guide는 Phase 1 계약이
  확정된 뒤 한국어 문서와 짝을 맞춘다.

## 권장 진행 순서

| 순서 | 게이트 | 완료 조건 |
|---|---|---|
| 1 | 조사 근거 고정 | 모든 `[V]`에 URL/commit/조회일이 있고 절대 주장의 조사 범위가 명시됨 |
| 2 | Windows capability·fidelity 계약 | Windows 모드별 coverage, process attribution, header/body/timing 보장이 표로 고정되고 Linux/macOS live parity는 후속으로 분리됨 |
| 3 | 보안 위협 모델 | CA, imported HAR, redaction, process metadata, export 기본값이 승인됨 |
| 4 | bounded result/store 계약 | `AnalysisResult`는 크기와 무관하게 동일하고 상세는 versioned store에서 paging됨 |
| 5 | Phase 1 HAR importer | 7종 sanitized fixture와 malformed/security fixture로 normalize·diagnostic 검증 |
| 6 | deterministic aggregate | 실시간 완료 순서와 batch 시작 순서가 같은 bounded 결과를 냄 |
| 7 | Windows proxy build-vs-integrate spike | H1/H2·Windows 패키징/서명·보안 업데이트·PID hook 비용을 실측해 구현 경로 결정 |
| 8 | Windows Phase 2 capture | disk full/crash/event loss/CA failure/pinning/streaming E2E를 통과 |
| 9 | Windows coverage 심화 | ETW/WFP/Npcap proof가 있는 scope에서만 coverage ratio를 노출 |
| 10 | HTTP 전용 Diff/교차 분석 | URL template과 시간 정렬 신뢰도를 포함한 의미 있는 비교가 검증됨 |

## 한 문장 요약

**Windows-first로 실행 범위를 좁히고 Linux/macOS 증거는 Windows UI의 오프라인
입력으로 지원하는 방향은 타당하지만, Windows의 프로세스별 커버리지 전제·wire
fidelity·무손실 스트리밍·대용량 result·보안 기본값은 여전히 충돌하므로 Phase 1
계약부터 작게 고정한 뒤 Windows MITM 캡처로 확장해야 한다.**
