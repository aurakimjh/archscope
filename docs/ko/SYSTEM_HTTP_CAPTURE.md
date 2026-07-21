# 시스템 전체 HTTP 통신 캡처 및 분석 (설계 노트)

## 0. 이 문서의 위치와 전제

본 문서는 HTTPAnalyzer 계열 도구가 제공하던 **시스템 전역 HTTP/HTTPS 트래픽
캡처 및 프로세스별 분석** 기능을 ArchScope에 도입하기 위한 설계 노트다.

> **개정 이력**
>
> - 2026-07-18: 경쟁 솔루션 조사 반영, R9~R12 추가, 라이브 뷰 필수 확정
> - 2026-07-19: Windows-first 운영 전제 확정
> - 2026-07-19: Codex 설계 검토(P0 1~5, P1 6~9, P2 10) 반영. 주요 철회
>   항목 — 프로세스별 `bytesSystem` 커버리지 전제(10.1.1), 원본 wire 헤더 보존
>   요구(6.2.1), 단일 `Timings` 모델(6.3), 크기 임계치 기반 `AnalysisResult`
>   계약(6.6), TLS 실패 시 자동 패스스루(5.3), JWT payload·쿠키 해시 보존
>   기본값(11.3.1), "네 개의 방언" 요약(6.5)
> - 2026-07-20 (본 개정): Phase 0 게이트 산출물 작성(T-566~571). 추가 항목 —
>   검증 가능한 출처 원장(부록 A 전면 재작성), Windows 모드별 기능·정확도 행렬과
>   런타임/증거 호환성 축 분리(9.3, 9.4), 트랜잭션 경계·시간 기준점·상태 기계·
>   H1/H2 골든 불변식(6.3.1~6.3.4), 유계 세션 저장소 계약(7.6), CA 수명주기와
>   권한 계약·적대적 테스트 행렬(11.2.1, 11.6, 11.7), Windows
>   proof-of-capability 수용 기준(10.4)
> - 2026-07-20 (2차): Phase 1 fixture 코퍼스 구축(T-572). 합성 우선 HAR 코퍼스
>   20종(방언 10·형식 이상 8·적대적 2)과 대형 입력 생성기를 공유 자산 저장소
>   `../projects-assets/test-data/har-fixtures/`에 추가 — manifest에 provenance·
>   근거 claim·기대 diagnostic·리댁션 assertion 기록, 실기 export 수집 계획 포함
> - 2026-07-20 (3차): HTTP 전용 세션 Diff 계약 확정(T-575, 12.4.1). URL
>   템플릿화 규칙과 버전, 비교 차원(endpoint/host/process)과 활성 조건, 명시적
>   분모·독립 백분위수, 시간 정렬 등급(aligned/duration_only/none), 상한 있는
>   `http_capture_diff` 결과 타입, HTTP_DIFF_* finding, 현재 코드 기준 Wails
>   진입점(HttpCapturePage 액션 + Workspace 비교 라우팅, 신규 NavKey 없음)
> - 2026-07-21: 영문 짝 문서 추가(T-576). `docs/en/SYSTEM_HTTP_CAPTURE.md` —
>   importer/방언 지원 매트릭스, 데이터 모델 레퍼런스, 보안 가이드를 절로 갖춘
>   축약 레퍼런스. 본 한국어 노트가 규범 원본이며 부록 A 원장이 감사 가능한
>   주장 등록부로 유지된다. T-571 잔여 실측(real-NIC) 완료 시 영문 8절도 갱신.
>   아울러 2026-07-20 병합에서 유실된 T-572/T-575 개정(게이트 6·11, 12.4.1)을
>   복구하고 게이트 6·7 상태를 구현 현황에 맞게 갱신

전제를 먼저 명시한다.

- **제품 실행 환경은 Windows-first다.** ArchScope 데스크톱 UI와 실시간 HTTP 캡처의
  1차 지원·검증 플랫폼은 Windows다. Linux/macOS에서 생성된 HAR·프로파일·로그를
  Windows UI로 가져와 분석하는 오프라인 호환성은 유지하지만, Linux/macOS용
  실시간 캡처 백엔드와 데스크톱 UI 동등성은 1차 출시 조건이 아니다. 따라서 이
  문서의 "크로스 플랫폼"은 **입력 데이터·정규화·분석 모델의 이식성**과
  **실시간 캡처 런타임의 지원 범위**를 구분해 읽어야 한다.
- HTTPAnalyzer(IEInspector)는 **"단종"이 아니라 "휴면"이다.** 2026-07-18 조사
  결과 공식 EOL 공지가 존재하지 않고 벤더 사이트와 구매 페이지가 여전히 살아
  있다. 정확한 표현은 **약 2018~2019년 이후 미갱신**이다. 근거는 1.4절에 있다.
  기획 입력의 "단종됐다"는 표현은 이 문서에서 **채택하지 않는다.**
- 본 문서는 그 제품을 *기능 레퍼런스*로만 참조하며, 특정 회사·제품의 현재 상태를
  주장하지 않는다. "HTTPAnalyzer가 했던 것"이라는 표현은 전부 **기능 요구사항의
  축약**으로 읽어야 한다. 구현 세부를 모방하자는 뜻이 아니다.
- 1.4·1.5절의 경쟁 도구 서술에는 **검증 표시**를 붙였다. `[V]`는 1차 출처로
  확인한 사실, `[I]`는 추론, `[?]`는 확인 실패다. 설계 결정을 `[I]`·`[?]`에
  걸지 않는다.

그리고 이 문서에서 가장 중요한 문장을 앞에 둔다.

> **이 기능은 ArchScope의 기존 아키텍처 전제를 깨는 첫 번째 기능이다.**

근거는 3.0절에 있다. 이 파장을 받아들일지 결정하는 것이 설계 검토의 첫 번째
안건이며, 나머지 절은 "받아들인다면 어떻게"에 대한 답이다.

## 1. 배경 및 동기

### 1.1 남는 공백

HTTPAnalyzer 계열 도구가 채우던 자리는 다음 세 가지 조건을 **동시에** 만족하는
지점이었다.

1. 브라우저뿐 아니라 **데스크톱 앱·CLI·백그라운드 서비스**까지 포함한 전역 캡처
2. **어느 프로세스가 보낸 요청인지** 명확히 구분
3. 요청/응답 헤더·바디·타이밍·상태 코드까지 **한 화면에서** 확인

개별 조건을 만족하는 도구는 지금도 많지만, 세 조건을 동시에 만족하는 도구는
드물다.

| 도구 | 전역 캡처 | 프로세스 식별 | HTTP 시맨틱 | 한계 |
|---|---|---|---|---|
| Chrome DevTools Network | ✗ (해당 탭만) | ✗ (자명하므로 불필요) | ◎ | 브라우저 밖을 못 본다 |
| Wireshark | ◎ | ✗ | △ (해석은 되나 UI가 패킷 중심) | TLS 불투명, PID 없음 |
| Fiddler | △ (프록시 인지 앱만) | ◎ (이름+PID) | ◎ | 프록시를 무시하는 앱은 안 보임. Chrome이 프로세스 하나로 뭉침 |
| Charles | △ | △ (이름만·기본 꺼짐) | ◎ | 〃 |
| Proxyman | △ | △ (macOS 전용) | ◎ | 〃 |
| mitmproxy | △ / ○ (OS 레벨 모드 있음) | △ (캡처 필터만, 신원 미보존) | ◎ | CLI 중심 |
| macOS `nettop` / Windows 리소스 모니터 | ◎ | ◎ | ✗ (바이트 수만) | 내용이 없다 |

**전역 캡처와 프로세스 식별과 HTTP 시맨틱의 교집합**이 비어 있다. 그리고 그
교집합이 필요한 순간은 실무에서 반복된다.

> 이 표는 2026-07-18 조사로 개정되었다. 개정 전에는 프록시 도구들을 한 줄로 묶고
> 프로세스 식별을 "일부 추정"으로 적었으나 **부정확했다.** Fiddler는 이름과 PID를
> 기본 컬럼으로 제공하고, Charles도 이름 수준의 귀속을 한다. 근거와 각 도구의
> 실제 제약은 1.5절에 있다. **그리고 조사는 이 표가 놓친 두 번째 공백을
> 드러냈다 — 시간축 집계다(1.5.4).**

- "사내 배포 툴이 어디로 뭘 보내는지 모르겠다"
- "Electron 앱이 시작할 때 8초 걸리는데 네트워크 때문인지 아닌지"
- "JVM 배치가 외부 API를 몇 번 호출하는지 로그에 안 남는다"
- "설치한 에이전트가 예상 밖의 엔드포인트로 통신하는지 확인해야 한다"

### 1.2 왜 하필 ArchScope인가

ArchScope는 응답시간의 **근거**를 좁혀 온 도구다. access log → APM(Jennifer) →
서버 프로파일 → 브라우저 CPU 프로파일 순으로 계층을 내려왔고, 각 단계의 산출물이
`AnalysisResult`라는 하나의 봉투에 담긴다(`apps/engine-native/internal/models/analysis_result.go:73`).

여기에 빠진 계층이 **애플리케이션과 네트워크 사이의 경계**다. 현재 ArchScope가
네트워크에 대해 아는 것은 전부 **간접 증거**다.

- access log는 **서버가 받은 뒤**의 기록이다. 클라이언트가 언제 보냈고 연결에
  얼마나 썼는지는 없다.
- Jennifer MSA의 `applyNetworkGap`(`apps/engine-native/internal/analyzers/jenniferprofile/msa.go:195`)은
  `external_call_elapsed - callee_response`로 네트워크 시간을 **빼서 추정한다.**
  이것이 DNS인지 TCP 핸드셰이크인지 TLS인지 큐잉인지 구분하지 못한다.
- OTLP/Zipkin trace import는 **계측된 코드만** 보여준다. 계측되지 않은 프로세스는
  없는 것과 같다.

HTTP 캡처는 이 세 가지 추정을 **직접 관측으로 대체한다.** `applyNetworkGap`이
숫자 하나로 뭉뚱그린 구간이 DNS 240ms + TLS 90ms + TTFB 30ms로 분해된다. 이는
새 기능이 아니라 **기존 분석의 근거 강화**이며, ArchScope의 제품 논리와 정확히
같은 방향이다.

여기에 세 가지가 더 있다.

1. **교차 분석 자산이 이미 있다.** HTTP 트랜잭션의 시간 구간과 CPU 프로파일의
   샘플 구간을 겹치면 "이 fetch가 기다리는 400ms 동안 CPU는 무엇을 했는가"에
   답할 수 있다. 이 교차는 12절에서 다루며, 캡처 단독보다 훨씬 큰 가치다.
2. **리포트 파이프라인 재사용.** HTML/PPTX/CSV/JSON export와 Diff가
   `AnalysisResult`만 있으면 그대로 동작한다(`cmd_report.go:51`).
3. **로컬 완결성.** ArchScope는 SaaS 연동 없이 로컬에서 끝나는 도구이고, 캡처
   데이터처럼 민감한 자료를 다루기에 이 성질이 오히려 강점이다. AI 해석조차
   `validateLocalURL`로 localhost를 강제한다(`apps/engine-native/internal/aiinterpretation/runtime.go:33`).

### 1.3 비목표

명시적으로 하지 않는 것들이다. 이 목록은 범위 방어선이므로 임의로 넘지 않는다.

- **DevTools/Wireshark 대체를 노리지 않는다.** 실시간 인터랙티브 디버깅은 계속
  전용 도구의 영역이다. ArchScope의 역할은 `.cpuprofile` 설계 노트와 동일하게
  **보관·비교·근거화·리포트**다.
- **인증서 피닝(certificate pinning) 우회를 구현하지 않는다.** 피닝된 앱은 MITM
  대상에서 제외하고 메타데이터만 기록한다. 우회는 기술적으로 가능하더라도
  제품 기능으로 제공하지 않는다(11.4).
- **트래픽 변조·리플레이·목킹을 제공하지 않는다.** 캡처는 **읽기 전용**이다.
  프록시 모드가 경로상 개입하더라도 페이로드를 수정하지 않는다.
- **HTTP 외 프로토콜을 1차 범위에 넣지 않는다.** gRPC(HTTP/2 위이므로 부분
  가능), WebSocket, MQTT는 향후 과제다(13절).
- **원격 호스트 캡처를 하지 않는다.** 로컬 머신 한 대가 대상이다.

### 1.4 HTTPAnalyzer 기능 레퍼런스 (조사 결과)

#### 1.4.1 제품 상태 — "휴면"이지 "단종"이 아니다

| 항목 | 확인 내용 | 표시 |
|---|---|---|
| 최종 버전 | `7.6.4.508` | `[V]` |
| 최종 릴리스 시점 | **2018-03 ~ 2019-07 사이**. 벤더가 날짜를 공개하지 않아 삼각측량함 | `[V]` |
| 삼각측량 근거 | Wayback 2018-03 스냅샷은 `7.6.1.481`, 2019-07·2023-06 스냅샷과 현재 사이트가 모두 `7.6.4.508`로 동일. 동봉 OpenSSL `1.0.2n`은 2017-12 릴리스 | `[V]` |
| 사이트 저작권 | `© 2004-2016` | `[V]` |
| OS 지원 상한 | **Windows 10.** Windows 11 지원 표기가 없다 | `[V]` |
| 브라우저 지원 상한 | IE 11 | `[V]` |
| EOL 공지 | **존재하지 않는다.** 어떤 형태의 종료 발표도 찾지 못함 | `[V, 부정]` |
| 판매 | 주문 페이지 활성, BlueSnap 결제, $149~$169 | `[V]` |
| 실제 결제·지원 응답 여부 | 확인하지 않음 (거래를 시도하지 않았다) | `[?]` |

**커널 드라이버에 의존하는 제품이 Windows 11 지원을 표기한 적이 없다**는 점이
실질적 함의다. 최신 Windows에서 동작 여부는 미검증이다 `[?]`.

#### 1.4.2 캡처 메커니즘 — 2계층 구조

이 항목이 ArchScope 설계에 가장 중요하다. 조사 초기에는 Winsock LSP 추정이
있었으나, **벤더 changelog가 더 강한 증거**를 준다 `[V]`.

- `v7.5.2.454`: "high performance **proxy-less netfilter solution**, compatible
  with antiviruses/firewalls/other network filters"
- `v7.6.1.481`: "memory leaks after unloading **the driver**"
- `v7.6.4.508`: "Changed the **driver loading order** for compatibility with
  360 Total Security"

즉 캡처는 **커널 모드 네트워크 필터 드라이버**다. 프록시가 아니다.

반면 HTTPS 평문은 다른 계층에서 얻는다 `[V]`:

> "HTTPS is available if the application uses the Microsoft WININET API,
> Mozilla NSS API or OpenSSL API."

**정리하면 HTTPAnalyzer는 두 메커니즘을 서로 다른 계층에서 병용했다.**

| 목적 | 계층 | 수단 |
|---|---|---|
| 캡처 + 프로세스 귀속 | 커널 | 네트워크 필터 드라이버 |
| TLS 평문 | 유저랜드 | 라이브러리 API 훅 (WinINET / NSS / OpenSSL) |

이는 3.7절이 채택한 **원칙 B(다층 백엔드 + 단일 데이터 모델)의 실제 선례**다.
새로운 도박이 아니라 이미 상용 제품이 택했던 구조라는 뜻이다.

동시에 **한계도 같은 모양으로 물려받는다.** 훅 대상 라이브러리를 안 쓰면
평문을 못 본다 — Schannel·.NET `HttpClient`·Java·Go는 목록에 없다 `[I]`. 이는
3.2절이 프록시에 대해 이미 기술한 커버리지 구멍과 **구조적으로 동일한 문제**다.
캡처 계층을 아무리 낮춰도 TLS 평문 계층은 여전히 라이브러리에 종속된다.

#### 1.4.3 UI — 채택할 것

벤더 V7 도움말 파일 기준이다 `[V]`. 아래는 실제로 ArchScope에 반영할 항목만
추렸다.

| HTTPAnalyzer 요소 | 내용 | ArchScope 반영 |
|---|---|---|
| **그리드 내 `Timeline` 막대 컬럼** | "같은 그룹 내 다른 요청과의 상대 시간을 표현하는 차트 막대" | **채택.** 8.3절 목록 컬럼 |
| **2단 그룹 그리드** | 그룹 헤더 행에 집계(Count, 송수신 바이트) + 자식 요청 행 | **채택.** 4.5절 2단 트리와 동형 |
| 7구간 타이밍 | `Blocked`/`DNS`/`Connect`(SSL 포함)/`Send`/`Wait`/`Receive`/`CacheRead` | 6.3절과 비교 → 아래 주석 |
| `TimeToFirstByte` 별도 필드 | `Wait`와 **별개로** 노출 | 채택 검토 |
| `ConnectId` 컬럼 | "요청이 사용한 TCP 연결" | **채택.** 6.1절 `Connection` 계층과 일치 |
| `Stage` 컬럼 | `REQUEST_OPEN`→`SEND`→`SENT`→`COMPLETE`/`REDIRECT`/`CLOSE` | **실시간 모드에서 채택.** 7.4절 |
| **Summary Panel** | 페이지/프로세스/전체 3중 스코프 실시간 집계 | **채택.** 8.7절 |
| Summary의 `TotalHTTPSOverhead`, `DNS Lookups`, `TCP Connects` | TLS 오버헤드·DNS·연결 수를 1급 지표로 | **채택.** 10.1절 지표에 추가 |
| **Hints** | Performance / Security / Functional 3분류 룰 엔진. 그리드 필터 연동, 개별 억제 가능 | **채택.** 6.7절 `AddFinding`과 거의 동형 |
| Excel식 컬럼 드롭다운 필터 | 비파괴 — "display filter는 로깅을 막지 않는다" | **채택.** 8.5절, Wireshark 모델과 동일 |
| 9종 content 뷰어 | Text/Hex/Image/Preview/JSON/AMF/ViewState/URLDecode/Soap | 8.4절과 대체로 일치 |
| 압축률 인라인 표기 | "4064 bytes, gzip compressed to 2035 bytes (49.93% saving)" | **채택.** R4 |

**타이밍 구간 차이 주의.** HTTPAnalyzer는 `Connect`에 SSL을 포함하고 별도 `TLS`
구간이 없다. ArchScope 6.3절은 `ConnectMs`와 `TLSMs`를 분리한다. **분리를
유지한다** — TLS 핸드셰이크 비용은 오늘날 독립 진단 대상이고, HAR도 `ssl`을
별도 필드로 둔다. 다만 HAR 왕복 시 6.5.4절의 규칙을 따른다.

#### 1.4.4 채택하지 않을 것

| 항목 | 이유 |
|---|---|
| Tamper (요청 변조) | 1.3절 비목표 |
| Request Builder / 리플레이 | 1.3절 비목표 |
| 커널 드라이버 캡처 | 배포 비용. 3.3절과 같은 판단. 게다가 아래 AV 문제 |
| 정규식 없는 검색 | HTTPAnalyzer는 정규식 미지원 `[V, 부정]`. ArchScope는 지원한다 |
| WebSocket 미지원 | HTTPAnalyzer에 없다 `[V, 부정]`. 13절 향후 과제로 유지 |

#### 1.4.5 드라이버·훅 방식의 숨은 비용 — 안티바이러스

벤더 changelog가 스스로 기록한 문제다 `[V]`. 이것이 조사에서 얻은 **가장 실무적인
경고**다.

- "Added minor changes to **avoid antivirus false positives**"
- "Added a workaround for compatibility with **Windows Defender**"
- "Fixed: a compatibility issue with **F-Secure**"
- "Changed the driver loading order for compatibility with **360 Total Security**"

명시적으로 이름이 나온 보안 제품만 3종, 여기에 오탐 회피 작업이 별도로 있다.
BHO DLL(`IEInspectorBHO.dll`)은 악성코드가 같은 파일명으로 위장하는 사례까지
보고되어 있다 `[V]`.

**함의: 훅/드라이버 기반 캡처는 안티바이러스와 상시 전투를 벌인다.** 이 비용은
9.1절 플랫폼 행렬과 13절 Phase 5(pcap/ETW/eBPF)에 반영해야 하며, 프록시 방식을
1순위로 둔 판단(3.7절)을 **추가로 뒷받침한다.** 프록시는 일반 사용자 권한의
평범한 로컬 소켓 서버라 AV 충돌면이 훨씬 좁다.

#### 1.4.6 조사의 한계 — 사용자 평판은 확인 불가

기획 입력은 "사용자 리뷰 조사"를 요청했으나, **실질적인 리뷰 코퍼스가
존재하지 않는다** `[?]`. 두 조사자가 독립적으로 같은 결론에 도달했다.

- StackOverflow 언급 4건, 전부 한 줄짜리 추천, 점수 0~2점
- AlternativeTo 등재는 **2014-11 이후 갱신 없음**, 좋아요 4개, 유일한 리뷰는
  스팸으로 분류된 링크 스팸
- Reddit·기술 블로그·포럼 스레드: 검색 결과 없음
- Softpedia·Software Informer 평점: 봇 차단(403/Cloudflare)으로 **확인 실패**

> **불만이 없다는 것을 호평으로 읽으면 안 된다.** 더 방어 가능한 해석은 이
> 도구가 불만 스레드를 만들 만한 사용자 규모에 도달한 적이 없다는 것이다.

따라서 **본 설계는 "사용자가 극찬한 UI"라는 근거를 사용하지 않는다.** 1.4.3절의
채택 항목은 전부 벤더 도움말 문서에서 확인한 *기능 사실*에 근거하며, 그 기능이
좋다는 판단은 ArchScope 팀의 것이다. 없는 사회적 증거를 지어내지 않는다.

### 1.5 경쟁 도구 비교 — 빈 자리는 어디인가

2026-07-18 조사. 버전과 날짜를 명기한 이유는 이 영역이 빠르게 변하기 때문이다.

#### 1.5.1 Fiddler — 대표주자의 실제 약점

**제품이 둘로 갈라졌고, 방향이 다르다** `[V]`.

| | Fiddler Classic | Fiddler Everywhere |
|---|---|---|
| 최신 | `5.0.20262` (2026-06-18) | `7.8.0` (2026-05-28) |
| 플랫폼 | Windows 전용 | Win/mac/Linux |
| 가격 | 무료 | **구독 전용** Lite $7 / Pro $13 / Enterprise $37 (월, 연납) |
| 상태 | "active development 아님" 공표. 그러나 **여전히 릴리스 중** (2026-05 TLS 1.3 추가) | 활성 |

흔히 인용되는 "Classic은 TLS 1.3 미지원"은 **이제 사실이 아니다** `[V]`. 반면
**Classic의 HTTP/2 미지원은 유효**하며, Telerik 포럼 답변은 "장기 계획, 일정
없음"이다 `[V]`.

**핵심 발견 — 전체 타임라인 분석의 부재.** 기획 입력의 가설이 조사로
확인되었고, 예상보다 강한 형태다.

> Classic의 Timeline 탭은 **선택한 세션에 한정된** 워터폴이다. CTRL로 행을 골라야
> 채워진다. 벤더 문서가 직접 인정한다 `[V]`: *"the granular timings aren't shown
> on the Timeline tab today; the intent of this tab is to show the relative
> timings between multiple individual requests."*
>
> Everywhere의 Overview/Insights도 마찬가지로 **선택 스코프**다. p90/p95/p99를
> 제공하지만 이는 *선택한 행들*에 대한 백분위수이지 시간 창에 대한 것이 아니다.

**두 제품 어디에도 캡처 전체에 대한 시계열 뷰가 없다** `[V, 소거법 확인]` —
RPS 차트 없음, 처리량 그래프 없음, 상태코드 시간대별 누적 영역 없음, 지연
백분위수 추이 없음, 브러시로 시간 구간을 잘라내는 상시 오버뷰 스트립 없음.

이 구조적 결과가 중요하다. **"오류율이 언제 튀었나"에 답하려면 이미 어디를 봐야
할지 알고 있어야 한다.** 선택이 선행 조건이므로 탐색적 분석이 성립하지 않는다.

**메모리·성능** `[V]`:

| 제품 | 문서화된 문제 |
|---|---|
| Classic | 요청/응답 바디를 **연속 메모리 블록**으로 보관. 벤더 문서가 "수천 세션"에서 실패를 기술하고, 대응책으로 **"최근 200 세션만 유지"**를 제시 — 이 숫자가 설계 지점의 정직한 척도다 |
| Everywhere | 2021년 포럼 신고: **캡처 50 MB에 메모리 32 GB 사용.** 벤더는 가동시간을 지목했으나 신고자는 "100 MB 미만 이미지/영상 캡처만으로 재현"이라 반박 |
| Everywhere | 2025-06 피드백: *"there's an incredible amount of lag in the Fiddler Everywhere interface"*, "네이티브 같지 않고 브라우저 안에서 도는 앱 같다" |

**라이선스 반발** `[V]`: 구독 전용·영구 라이선스 없음·무료 티어 없음.
브레이크포인트와 룰 편집이 Pro 게이트. 그리고 가장 문제적인 항목 —
**오프라인 모드가 Enterprise($37/월) 전용**이다. 폐쇄망 디버깅이라는 핵심
전문 용례가 최상위 티어에 있다 `[I: 문제적이라는 판단은 본 문서의 것]`.
Classic 유료화 요청은 **0표로 Declined** 처리되었다 `[V]`.

주의: Reddit은 크롤러 차단이라 **검색하지 못했다** `[?]`. "Reddit 반발"류
주장은 2차 인용이므로 채택하지 않는다. 위 항목은 전부 Telerik 자사 포럼·피드백
·문서에서 확인한 것이다.

**Fiddler의 진짜 강점은 프로세스 귀속이다** — 이 점은 정직하게 인정해야 한다.
Classic은 **프로세스명 + PID**를 기본 컬럼으로 제공하고(`X-PROCESSINFO`),
Everywhere의 network capturing 모드는 **캡처 시점에 PID/프로세스명으로 필터링**
한다 `[V]`. 조사 대상 중 가장 강력하다.

단, 결정적 한계가 있다 `[V, Eric Lawrence]`: *"Fiddler is showing you the
process that issued the network request, which (in Chrome) is the central
network process."* **Chrome 전체가 프로세스 하나로 뭉친다.** 렌더러별 구분이
불가능하다. 4.5절의 2단 트리가 이 문제를 정확히 겨냥한다.

#### 1.5.2 전체 비교 행렬

범례: ● 완전 · ◐ 부분/선택 스코프 · ○ 없음 · ? 미검증

| 항목 | Fiddler Classic | Fiddler Everywhere | Charles | mitmproxy | Proxyman | Wireshark | Chrome DevTools | Burp |
|---|---|---|---|---|---|---|---|---|
| 버전 (2026-07) | 5.0.20262 | 7.8.0 | 5.2 | 12.2.3 | 6.12+ | 현행 | 현행 | 현행 |
| 플랫폼 | Win | Win/mac/Lin | Win/mac/Lin | Win/mac/Lin | mac 중심 | Win/mac/Lin | Chrome | Win/mac/Lin |
| 가격 | 무료 | 구독 $7~37/월 | $50 영구 | 무료(MIT) | $89 영구 | 무료(GPL) | 무료 | 무료 / ~$475·년 |
| 시스템 프록시 캡처 | ● | ● | ● | ● | ● | — | — | ● |
| **OS 레벨 캡처** | ○ | ● WFP 드라이버(Win)/NE(mac), beta, Linux 없음 | ○ | ● NE(mac)/WinDivert(Win)/eBPF k6.8+(Lin) | ○ | ● libpcap (링크 레이어) | ○ | ○ |
| 요청별 타이밍 분해 | ● | ● | ● | ◐ | ● | ● | ● 10구간 | ◐ 타이머 2개 |
| 다중 요청 워터폴 | ◐ 선택 스코프 | ◐ 선택 스코프 | ◐ 선택 스코프 | ○ | ? 6.6.0 Chart View, 문서 없음 | ○ | ● Waterfall 컬럼 | ○ |
| **상시 전체 오버뷰 스트립** | ○ | ○ | ○ | ○ | ○ | ◐ Capture Info | ● **브러시 → 표 필터** | ○ |
| **처리량/RPS/오류율 시계열** | ○ | ○ | ○ | ○ | ○ | ● **I/O Graphs** | ○ | ○ |
| 집계 통계 | ◐ Statistics | ◐ 선택 스코프, p90/95/99 | ◐ 호스트별 | ○ | ○ | ● Conversations 등 (시간축 없음) | ◐ 요약 바 | ○ |
| **프로세스 귀속** | ● 이름+PID | ● + 캡처시 필터 | ◐ 이름만·로컬만·기본 꺼짐 | ◐ 캡처 필터만, 신원 폐기 | ◐ mac 전용·문서 없음 | ○ | ○ | ○ |
| HAR 내보내기 | ● | ● v1.1/v1.2 | ◐ **비규격** | ● (10.1+) | ● **1.2 미준수** | ○ | ● 130+ 기본 소독 | ○ |
| HAR 가져오기 | ● | ● (+`.cap`) | ? | ● **확장자 무관 자동 판별** | ● (+Charles `.chls`) | ○ | ● 드래그앤드롭 | ○ |
| **메모리 상한/축출** | ○ 수동 "최근 200" | ○ | ○ **기본 `-Xmx256m`** | ○ **상한도 축출도 없음** | ○ | ○ 전체 파일 인메모리 | ◐ 바디 버퍼만 | ○ |
| 스크립팅 | ● FiddlerScript+.NET | ○ **없음** | ○ **없음** | ● Python 애드온, 핫리로드 | ● JS+npm | ● Lua+tshark | ◐ CDP | ● Montoya+Bambda |

#### 1.5.3 조사가 뒤집은 통념 두 가지

설계 전제로 쓰기 전에 바로잡는다.

1. **Charles는 프로세스 귀속을 한다** `[V]`. 1.1절 표의 "일부 추정"은 부정확했다.
   Client Process 도구가 *"the name of the local client process"*를 보여준다.
   단 제약이 크다 — **이름만(PID 없음), 로컬 전용, 연결 수락 전 지연이 생겨
   기본 꺼짐.** 이 지연은 4.4절이 기술한 소켓 테이블 조회의 비용과 정확히
   일치하며 `[I]`, ArchScope가 같은 경로를 택할 때 감수할 비용의 실측 사례다.

2. **mitmproxy는 OS 레벨 캡처를 하면서 프로세스 신원을 버린다** `[V]`.
   `mode local:curl`처럼 **캡처 시점 필터**는 되지만, Python `Client` 클래스에
   `pid`·`process_name` 필드가 **아예 없다**(코드 검색 결과 0건). 필터링은 Rust
   레이어에서 끝나고 UI까지 오지 않는다. 즉 "어느 프로세스인가"를 사후에 물을 수
   없고, 관심 프로세스마다 캡처 세션을 따로 떠야 한다.

#### 1.5.4 결론 — ArchScope가 차지할 자리

조사의 종합이다 `[I: 전략적 판단]`.

> **HTTP 도구 전 범주에서 "시간축 집계"가 비어 있다.**
>
> 2026-07-18 기준, 조사한 공식 문서와 공개 UI 스크린샷 범위에서
> Fiddler(2종)·Charles·mitmproxy·Proxyman·DevTools·Burp 중 **처리량·RPS·동시성·
> 오류율의 시계열 차트를 확인하지 못했다.** 확인한 최대치가 선택 스코프
> 워터폴이다.
>
> 제대로 된 시계열을 가진 유일한 도구는 **Wireshark(I/O Graphs)** 인데,
> **Wireshark의 HTTP 통계에는 시간축이 없다.** HTTP 지표를 시간축에 올리려면
> display filter로 직접 조립해야 한다.
>
> 공백이 대칭이다 — **Wireshark는 집계는 있으나 HTTP를 모르고, HTTP 도구들은
> HTTP는 알되 집계가 없다.** 이 교집합이 ArchScope의 자리다.

**이 두 명제는 소거법 기반이므로 검증 범위를 함께 읽어야 한다.** 공식 문서에
기능 설명이 없다는 사실과 제품에 기능이 없다는 사실은 같지 않다. 부록 A의 출처
원장에 각 주장의 확인 경로를 남기고, 반증이 나오면 이 결론을 갱신한다.

이는 1.1절이 지목한 공백(전역성 × 프로세스 × HTTP 시맨틱)과 **다른 축의 공백**
이며, 조사 범위에서는 둘 다 비어 있었다. 그리고 두 번째 공백이 ArchScope의 기존
강점과 더 잘 맞는다 — 시간축 위에 이종 증거를 올리는 것은 `IncidentTimelinePage`가 이미 하는
일이고, 12절의 교차 분석이 서 있는 토대다.

부수적으로 확인된 세 번째 공백:

> **조사 범위 내에서 메모리 상한과 축출을 문서화한 도구를 확인하지 못했다.**
> Charles는 JVM 기본
> `-Xmx256m`에 "Info.plist를 직접 편집하라"가 벤더 공식 해법이다. Fiddler
> Classic 문서는 안전 작업 집합을 200 세션으로 제시한다. Everywhere는 50 MB
> 캡처에 32 GB를 썼다. mitmproxy는 상한도 축출도 없다. Proxyman은 바디를 가진
> 요청 약 1,000건에서 메모리가 튄다 `[V]`.
>
> **디스크 기반 · 유계 메모리 스트리밍 캡처는 그 자체로 방어 가능한 포지션이다.**
> 원칙 A(캡처와 분석을 파일 경계로 분리)가 이미 이 방향을 가리키고 있었다.

#### 1.5.5 직접 차용할 UX 패턴

조사에서 건진 구체적 구현 아이디어다. 출처를 밝히고 채택 위치를 명시한다.

| 패턴 | 출처 | 채택 위치 |
|---|---|---|
| **오버뷰 스트립을 브러시로** — 드래그한 시간 창으로 표가 필터링 | Chrome DevTools `[V]` | 8.7절. **"전체 타임라인"의 핵심 상호작용** |
| **Conversations 행에 인라인 기간 간트 막대** | Wireshark `[V]` | 8.7절 호스트/프로세스 집계표 |
| **집계표에서 "이 선택으로 I/O Graphs 열기"** 교차 링크 | Wireshark `[V]` | 8.7절 |
| **자동 스크롤이 위로 스크롤하면 멈추고, 맨 아래로 돌아오면 재개** | Wireshark, Fiddler SmartScroll `[V]` | 8.6절. Wireshark 버그 #11034(첫 수동 스크롤 후 영구 해제)를 반면교사로 |
| **캡처 필터와 표시 필터의 명확한 분리** | Wireshark `[V]` | 8.5절. 파괴적/비파괴적 구분을 UI에서 오해 불가능하게 |
| 응답 헤더로 **임의 커스텀 컬럼** 생성 | Chrome DevTools `[V]` | 8.3절. 저비용 고효용 |
| **스택 기반 initiator 체인** (Shift-hover, 유발자 녹색·의존 적색) | Chrome DevTools `[V]` | 13절 향후. Referer 휴리스틱보다 정확 |
| MIME 색상·연결 재사용 점·TTFB 선·**버퍼링된 응답은 빗금** | Fiddler Classic Timeline `[V]` | 8.3절. 뷰의 스코프는 좁아도 **시각 어휘 자체는 우수하다** |
| **사전 신뢰된 브라우저를 동봉** — CA 설치 절벽 제거 | Burp 내장 Chromium `[V]` | 14절 열린 질문에 추가 |
| 형식 자동 판별(확장자 무관) 관대한 임포터 | mitmproxy `[V]` | 6.5절 HAR 파서의 참조 구현 |

## 2. 요구사항 정리

기획 입력을 검증 가능한 형태로 재작성한다.

| ID | 요구사항 | 수용 기준 |
|---|---|---|
| R1 | **capability tier로 정의된** HTTP/HTTPS 캡처 범위 | 각 tier의 수용 기준을 개별로 만족한다 (2.1). "시스템 모든 프로세스"를 단일 요구로 두지 않는다 |
| R2 | 프로세스별 트리 루트 분리 | 트리 최상위가 프로세스이고, 미식별 트랜잭션은 별도 `(미식별)` 루트로 격리된다 |
| R3 | 요청/응답 헤더 표시 | fidelity 등급별 보장을 만족한다 — `semantic`은 이름·값·다중값, `raw_wire`만 원본 순서·casing (6.2.1) |
| R4 | 요청/응답 바디 표시 | JSON/폼/텍스트는 포맷팅, 바이너리는 hex, 압축은 자동 해제 |
| R5 | 타이밍 분해 | DNS/연결/TLS/요청전송/TTFB/응답수신 6구간이 구분된다 |
| R6 | 상태 코드·메서드·URL·크기 | 목록에서 정렬·필터 가능 |
| R7 | 기존 아키텍처 정합 | 산출물이 `AnalysisResult` 계약을 따르고 Wails 서비스로 노출된다 |
| R8 | **Windows-first 실행 + 이종 OS 증거 분석** | Windows 데스크톱에서 HAR 가져오기와 1개 이상의 실시간 캡처 모드가 동작하고, Linux/macOS에서 생성된 지원 HAR·프로파일·로그를 같은 Windows UI에서 분석할 수 있다. Linux/macOS 실시간 캡처는 별도 capability로 표시하며 1차 출시를 막지 않는다 |
| R9 | **HAR 가져오기** | Chrome/Firefox/Safari/Charles/Fiddler/Proxyman/Insomnia가 만든 HAR을 읽고, **생성 도구별 방언 차이를 정규화**한다 (6.5) |
| R10 | **실시간 캡처 표시** | 캡처 중 트랜잭션이 화면에 나타난다. 목표 지연 1초 이내, 초당 500건에서 UI가 응답성을 유지한다 (7.4, 8.6) |
| R11 | **전체 타임라인 분석** | 캡처 전체에 대한 요청률·오류율·처리량 시계열을 제공하고, **시간 구간 브러시 선택이 모든 뷰를 필터링**한다 (8.7) |
| R12 | **두 모드 단일 파이프라인** | 실시간 세션과 가져온 HAR이 **같은 데이터 모델·같은 분석기·같은 화면**을 쓴다 (7.5) |

R9~R12가 이번 개정에서 추가된 요구사항이다. R11은 1.5.4절의 조사 결론에서
직접 유도되었다 — 경쟁 도구 전부가 비워둔 축이다. R12는 그 둘을 별도 기능으로
만들지 않겠다는 선언이며, 7.5절이 이를 구조로 보장한다.

R10의 "초당 500건"은 근거 있는 수치가 아니라 **초기 목표치**다. 1.5.2절이
보여주듯 경쟁 도구들이 이 지점에서 무너지므로(Fiddler Classic 문서상 안전
작업 집합 200 세션, Proxyman은 약 1,000건), 실측 후 조정한다.

### 2.1 R1의 capability tier

개정 전 R1은 "시스템 모든 프로세스"라고 쓰면서 수용 기준은 4종 각 1개 캡처였고,
Phase 2는 다시 R1의 "부분"만 만족한다고 적었다. **하나의 requirement가 동시에
필수이자 부분이자 향후 목표이면 release gate로 쓸 수 없다.** Windows-first 전제를
반영해 분리한다.

| Tier | 범위 | 수용 기준 |
|---|---|---|
| A | Windows UI에서 HAR·프로파일·로그 가져오기. **증거를 생성한 OS는 무관** | 6.5의 방언 코퍼스가 정규화·진단을 통과 |
| B | Windows explicit/system proxy를 준수하는 로컬 애플리케이션 | 브라우저·`curl`·JVM·Electron 각 1개 이상 |
| C | Windows ETW/WFP/Npcap 기반 **메타데이터** 커버리지 | 10.1.2의 카운터가 산출됨 |
| D | Windows **복호화된 시맨틱** 커버리지. 피닝·QUIC 제외 | Tier B 대상에서 `semantic` fidelity 달성 |
| E | Linux/macOS 실시간 캡처 | 후속 capability. **Windows 출시 gate가 아니다** |

각 플랫폼·프로토콜·앱 유형 조합에 `supported | partial | unsupported | not_tested`와
fidelity 등급을 명시하고, Phase별로 어느 tier를 승격할지 정한다. `not_tested`를
`unsupported`와 뭉개지 않는 것이 요점이다.

R2의 "미식별 루트 격리"가 중요하다. 프로세스 식별은 뒤에서 보듯 **원리적으로
100%가 불가능한 경우**가 있고, 이를 숨기고 임의 귀속시키면 분석 결론이 틀린다.
모르는 것은 모른다고 표시하는 것이 요구사항이다.

## 3. 캡처 방식 비교

### 3.0 먼저: 아키텍처 전제 파괴

방식을 고르기 전에, 어떤 방식을 고르든 공통으로 깨지는 전제를 정리한다. 현재
ArchScope 코드베이스의 사실은 다음과 같다.

| 현재 전제 | 근거 | 캡처 도입 시 |
|---|---|---|
| 모든 입력은 **파일**이다 | `ingestion` 전체가 경로 기반. `DetectFile(path)`(`registry.go:111`), `accesslog.Analyze(path, …)`(`analyzer.go:311`). 파일 워처·리스너·stdin 리더가 없다 | **깨진다** — 시간축을 따라 흐르는 스트림이 입력이 된다 |
| 분석은 **1회성 배치**다 | Wails 서비스가 요청-응답형. 비동기도 `taskID` 기반 유한 작업(`engineservice.go:657`) | **깨진다** — 시작/정지가 있는 장시간 세션이 생긴다 |
| **권한 상승이 없다** | 런타임 코드에 `sudo`/setuid/launchd/드라이버 없음. 유일한 외부 프로세스는 JFR 도구(`apps/engine-native/internal/parsers/jfr/recording.go:119`) | **깨진다** — 모드에 따라 root/관리자 권한이 필요하다 |
| **아웃바운드 통신이 없다** | AI 해석의 localhost Ollama가 유일(`runtime.go:31`) | 유지 가능 — 캡처는 인바운드 관측이다 |
| 데이터가 **사용자가 가져온 파일**이다 | import 모델 | **깨진다** — 도구가 데이터를 *생성*하고, 그 데이터는 타인의 자격증명을 포함할 수 있다 |

즉 이 기능은 "importer 하나 추가"가 아니라 **제품 성격의 확장**이다. 네 개의
전제가 동시에 깨진다.

이 파장을 관리하기 위한 설계 원칙을 세운다.

> **원칙 A — 캡처와 분석을 분리한다.** 캡처기는 `.har`(또는 확장 포맷) 파일을
> 만드는 것으로 책임을 끝낸다. 분석기는 기존과 동일하게 **파일을 읽는다.**

이 원칙은 세 가지를 동시에 해결한다.

- 기존 파일 기반 ingestion 계약이 유지된다. 분석기는 캡처의 존재를 모른다.
- 캡처 없이도 **남이 만든 `.har`를 열 수 있다**(DevTools·Charles·Proxyman
  export). 이것만으로도 독립적인 가치가 있고, Phase 1의 실질 산출물이 된다.
- 권한이 필요한 코드를 **분리된 실행 단위**에 가둘 수 있다(9.2).

라이브 뷰(캡처 중 실시간 갱신)는 이 파일 계약 **위에** 얹는 부가 기능이지,
분석의 전제가 아니다.

### 3.1 방식 A — 패킷 캡처 (libpcap / Npcap)

BPF 장치나 Npcap 드라이버로 링크 레이어 프레임을 뜬 뒤 TCP 스트림을 재조립하고
HTTP를 파싱한다. Go에서는 `gopacket` + `tcpassembly`가 표준 경로다.

| 항목 | 평가 |
|---|---|
| 커버리지 | ◎ 프로세스가 협조하지 않아도 전부 보인다. 유일하게 R1을 원리적으로 만족 |
| 프로세스 식별 | ✗ **패킷에 PID가 없다.** 4-tuple 역조회 필요(4절), 짧은 연결은 놓친다 |
| HTTPS | ✗ 암호문. 키 없이는 SNI/인증서/크기/타이밍만 |
| 권한 | ✗ root 또는 `cap_net_raw`. Windows는 Npcap **별도 설치** |
| 타이밍 | ◎ 패킷 단위 타임스탬프라 핸드셰이크 분해가 가장 정확 |
| 침습성 | ◎ 완전 수동(passive). 앱 동작을 바꾸지 않는다 |
| 구현 난도 | 상 — 재조립, 순서 어긋남, 재전송, keep-alive 파이프라이닝, chunked, HTTP/2 HPACK |

**HTTP/2와 HTTP/3가 결정적 약점이다.** HTTP/2는 HPACK 동적 테이블이 연결 시작
시점부터 이어져 있어 **캡처를 중간에 시작하면 헤더를 영구히 복원할 수 없다.**
HTTP/3는 QUIC이라 TLS가 트랜스포트에 내장되어 재조립 자체가 별개 구현이다.

즉 pcap 단독은 **평문 HTTP/1.1에서만 완전**하고, 그 비중은 현대 환경에서 낮다.
그럼에도 가치가 있는 이유는 **커버리지 검증** 때문이다 — 프록시 모드가 놓친
트래픽이 있는지 확인하는 대조군으로 쓸 수 있다(10.3).

### 3.2 방식 B — MITM 프록시

로컬 프록시를 띄우고 시스템 프록시 설정으로 트래픽을 유도한다. CONNECT를 가로채
자체 CA로 서명한 리프 인증서를 즉석 발급해 TLS를 종단한다. Charles/Fiddler/
mitmproxy의 방식이며 Go 표준 라이브러리(`net/http`, `crypto/tls`)만으로 구현
가능하다.

| 항목 | 평가 |
|---|---|
| 커버리지 | △ **프록시 설정을 존중하는 앱만.** 아래 상술 |
| 프로세스 식별 | ◎ **의외의 강점.** 4.4절 참조 — 이 방식의 핵심 |
| HTTPS | ◎ 완전 평문. 피닝 제외 |
| 권한 | ◎ **일반 사용자 권한.** 시스템 프록시 설정과 CA 신뢰 등록만 승격 필요 |
| 타이밍 | △ 프록시 관점 기준. 클라이언트↔프록시와 프록시↔서버가 분리 관측되어 오히려 **분해가 쉽다**. 다만 프록시 자신의 오버헤드가 섞인다 |
| 침습성 | ✗ **경로에 개입한다.** 커넥션 재사용, ALPN 협상, TLS 버전이 실제와 달라질 수 있다 |
| 구현 난도 | 중 — 잘 알려진 영역 |

커버리지의 현실을 정직하게 적는다.

| 부류 | 시스템 프록시 준수 | 비고 |
|---|---|---|
| 브라우저(Chrome/Edge/Safari) | ◎ | Firefox는 자체 설정이 기본 |
| macOS `URLSession` 기반 앱 | ◎ | 시스템 설정을 자동 반영 |
| Windows WinINet/WinHTTP 앱 | ◎ / △ | WinHTTP는 별도 프록시 설정을 쓰는 경우 있음 |
| `curl`, `wget` | △ | `http_proxy`/`https_proxy` **환경변수** 필요 |
| Go 표준 `net/http` | △ | `ProxyFromEnvironment`가 기본이나 커스텀 Transport면 무시 |
| JVM | ✗ | `-Dhttps.proxyHost` 등 **JVM 인자 필요.** 시스템 설정 무시 |
| Node.js | ✗ | 기본적으로 프록시를 보지 않는다. `undici` 전역 dispatcher 등 별도 설정 |
| Electron | △ | Chromium 부분은 준수, Node 부분은 무시 — **한 앱 안에서 갈린다** |
| Go/Rust 정적 바이너리 | ✗ | 자체 TLS 스택 + 자체 신뢰 저장소라 CA 등록도 안 먹는 경우 |

**"모든 프로세스"라는 R1을 프록시 단독으로는 만족하지 못한다.** 이것이 이 설계의
가장 큰 정직한 한계이며, 10.1의 커버리지 리포트가 필요한 이유다.

### 3.3 방식 B' — 투명 프록시 (transparent proxy)

방식 B의 커버리지 한계를 OS 레이어에서 해결한다. 앱 설정과 무관하게 커널이
트래픽을 로컬 프록시로 리다이렉트한다.

| 플랫폼 | 수단 | 비용 |
|---|---|---|
| macOS | `pf` 앵커 + `rdr-to`, 또는 **Network Extension(transparent proxy provider)** | pf는 root. NE는 **Apple Developer Program 엔티틀먼트 + 공증 + 시스템 확장 설치** |
| Linux | `iptables`/`nftables` REDIRECT + `SO_ORIGINAL_DST`, 또는 TPROXY | root / `CAP_NET_ADMIN` |
| Windows | WFP 콜아웃 드라이버, WinDivert | **커널 드라이버 서명** 필요 |

커버리지는 크게 오르지만(◎에 근접) 배포 비용이 급증한다. 특히 macOS Network
Extension은 `com.apple.developer.networking.networkextension` 엔티틀먼트가
필요하고 이는 **Apple의 개별 승인 대상**이다. Windows 드라이버 서명도 마찬가지
성격의 장벽이다.

투명 프록시는 매력적이지만 **초기 범위에서 제외한다.** 배포 파이프라인 자체를
바꿔야 하는 요구이며, 그 비용을 치르기 전에 Phase 1~2로 가치를 검증해야 한다.

### 3.4 방식 C — eBPF (Linux)

TLS 라이브러리의 `SSL_write`/`SSL_read`에 uprobe를 걸어 **암호화 직전/복호화
직후의 평문**을 가로챈다. Pixie, Coroot, `ecapture` 등이 쓰는 방식이다.

| 항목 | 평가 |
|---|---|
| 커버리지 | ○ 대상 라이브러리를 쓰는 프로세스 전부. 앱 설정 불필요 |
| 프로세스 식별 | ◎ **완벽.** 훅 시점에 PID/TID/comm/cgroup을 커널에서 직접 얻는다. 경합 없음 |
| HTTPS | ◎ **MITM 없이 평문.** CA 설치 불필요, **피닝도 무력화되지 않고 그냥 보인다** |
| 권한 | ✗ root 또는 `CAP_BPF`+`CAP_PERFMON` |
| 이식성 | ✗ **Linux 전용.** 커널 5.8+ 권장, CO-RE/BTF 필요 |
| 구현 난도 | 상 — 아래 |

난도의 실체는 **라이브러리별 심볼 오프셋**이다.

| 런타임 | 훅 대상 | 문제 |
|---|---|---|
| OpenSSL | `SSL_write`/`SSL_read` | 버전마다 `SSL` 구조체 오프셋이 달라 fd 추출이 깨진다 |
| BoringSSL (Chrome, gRPC) | 동형 API | 정적 링크 + 심볼 스트립이 흔하다 |
| GnuTLS / NSS | `gnutls_record_send`, `PR_Write` | 각각 별도 구현 |
| **Go `crypto/tls`** | OpenSSL을 **안 쓴다** | Go 런타임 전용 uprobe 필요. goroutine 스택 이동 때문에 uretprobe가 불안정 |
| **JVM (JSSE)** | 순수 Java 구현 | uprobe 대상이 없다. JVMTI 에이전트가 별도로 필요 |

즉 eBPF도 "모든 프로세스"는 아니다. **Go와 JVM이라는 ArchScope의 주 사용자층이
빠진다**는 점이 뼈아프다. 그럼에도 Linux 서버 환경에서는 가장 강력하다.

macOS에는 대응물이 없다(DTrace는 SIP로 제한, Endpoint Security는 네트워크
페이로드를 주지 않는다). Windows의 eBPF-for-Windows는 아직 이 용도로 성숙하지
않았다.

### 3.5 방식 D — TLS 키 로그 (SSLKEYLOGFILE)

`SSLKEYLOGFILE` 환경변수를 설정한 채 앱을 실행하면 TLS 세션 시크릿이 파일에
기록되고, 이를 pcap과 결합해 오프라인 복호화한다.

| 항목 | 평가 |
|---|---|
| 커버리지 | △ **환경변수를 존중하는 런타임만**. 게다가 **실행 전에** 설정되어야 한다 |
| 프로세스 식별 | ✗ pcap과 동일한 한계(4절) |
| HTTPS | ○ 복호화 가능. **실제 TLS 협상이 그대로 일어난다** — 침습 0 |
| 권한 | pcap 몫의 권한만 |
| 침습성 | ◎ **가장 낮다.** 피닝 앱도 그대로 동작하고 그대로 복호화된다 |

지원 현황: Chrome/Firefox/`curl`/OpenSSL 기반 앱은 대체로 지원. Go는 `tls.Config.
KeyLogWriter`를 **코드에서 명시**해야 한다(환경변수만으로는 안 된다). JVM은
표준 지원이 없어 `-Djavax.net.debug` 계열이나 별도 에이전트가 필요하다.

가치는 **회귀 검증용 정밀 모드**에 있다. "프록시를 태웠더니 타이밍이 달라졌다"는
의심이 들 때, 침습 없이 실제 트래픽을 확인하는 대조군으로 쓴다.

### 3.6 방식 E — 플랫폼 네이티브 이벤트

페이로드 없이 **연결 메타데이터**만 OS가 알려주는 경로다.

- **Windows ETW** — `Microsoft-Windows-TCPIP`, `Microsoft-Windows-WinINet`,
  `Microsoft-Windows-WinHttp` 공급자. WinINet 공급자는 요청 URL과 상태 코드까지
  **PID와 함께** 준다. 권한도 관리자면 충분하고 드라이버가 필요 없다. Windows
  한정으로는 비용 대비 효과가 가장 좋다.
- **macOS Network Extension(`NEFilterDataProvider`)** — 플로우 이벤트에
  `sourceAppAuditToken`이 실려 **PID 매핑이 정확하다.** 다만 3.3의 엔티틀먼트
  장벽이 동일하게 적용된다.
- **Linux `sock_diag`(netlink)** — `/proc` 스캔보다 빠른 소켓 테이블 조회.
  이벤트가 아니라 폴링이라 경합은 남는다.

이들은 단독 캡처 수단이 아니라 **프로세스 식별 보조**로 쓴다(4절).

### 3.7 종합 및 선택

| | 커버리지 | PID | HTTPS | 권한 | 이식성 | 난도 |
|---|---|---|---|---|---|---|
| A. pcap | ◎ | ✗ | ✗ | 상 | ◎ | 상 |
| B. MITM 프록시 | △ | ◎ | ◎ | **하** | ◎ | 중 |
| B'. 투명 프록시 | ◎ | ◎ | ◎ | 상 | △ | 상 |
| C. eBPF | ○ | ◎ | ◎ | 상 | ✗ (Linux) | 상 |
| D. 키 로그 | △ | ✗ | ○ | 중 | ○ | 중 |
| E. 네이티브 이벤트 | ◎ (메타만) | ◎ | ✗ | 중 | ✗ (개별) | 중 |

**단일 방식으로 R1을 만족하는 선택지가 없다.** 따라서 다층 구조를 택한다.

> **원칙 B — 모드는 여러 개, 데이터 모델은 하나.** 캡처 백엔드를 `Capturer`
> 인터페이스 뒤에 두고, 어떤 백엔드든 동일한 `Transaction`을 산출한다. 각
> 트랜잭션은 `capture_mode`와 `fidelity`를 자기 기술(self-describing)한다.

도입 순서는 **가치/비용 비율** 순서다.

1. **MITM 프록시 (B)** — 유일하게 일반 사용자 권한으로 HTTPS 평문 + 정확한 PID를
   동시에 준다. 커버리지가 부분적이라는 약점은 10.1의 커버리지 리포트로
   *가시화*하여 관리한다. 감추지 않는 것이 핵심이다.
2. **플랫폼 네이티브 (E)** — Windows ETW 우선. PID·URL과 coverage evidence를
   보강한다.
3. **pcap (A)** — 프록시가 놓친 트래픽의 존재를 증명하는 대조군. 평문 HTTP는
   완전 분석, HTTPS는 메타데이터(SNI·크기·타이밍)만.
4. **eBPF (C)** — Linux 서버 환경 심화.
5. **투명 프록시 (B')** — 엔티틀먼트/드라이버 비용을 감수할 근거가 생기면.

## 4. 프로세스 식별

R2의 "프로세스별 트리 루트"가 이 절에 달려 있다. 방식별로 난도가 완전히 다르다.

### 4.1 문제의 구조

패킷과 소켓에는 PID가 없다. PID는 **커널의 소켓 소유권 정보**에 있고, 이를
`(protocol, localAddr, localPort, remoteAddr, remotePort)` 5-tuple로 역조회해야
한다.

```
[패킷/플로우]  5-tuple ──조회──> [OS 소켓 테이블] ──> PID ──> 실행 경로, 명령줄, 부모
```

### 4.2 플랫폼별 조회 수단

| 플랫폼 | API | 특성 |
|---|---|---|
| macOS | `proc_pidinfo(PROC_PIDLISTFDS)` → `proc_pidfdinfo(PROC_PIDFDSOCKETINFO)` | 전 PID 순회. `lsof`가 쓰는 경로. **비용이 크다** |
| macOS | `NEFilterFlow.sourceAppAuditToken` | 정확하고 이벤트 기반. 엔티틀먼트 필요 |
| Linux | `/proc/net/tcp{,6}` → inode → `/proc/*/fd/` 심볼릭 링크 역스캔 | O(프로세스 × fd). 느리다 |
| Linux | netlink `sock_diag`(INET_DIAG) | inode/uid를 빠르게. 여전히 폴링 |
| Linux | eBPF `tcp_connect` / `inet_sock_set_state` tracepoint | **이벤트 기반, 경합 없음** |
| Windows | `GetExtendedTcpTable(TCP_TABLE_OWNER_PID_ALL)` | **PID를 직접 반환.** 가장 단순 |
| Windows | ETW TCPIP 공급자 | 이벤트 기반 |

### 4.3 폴링의 근본 한계

폴링 기반 조회에는 **경합(race)이 있다.**

```
t0  프로세스가 connect()
t1  요청 전송, 응답 수신
t2  close()          ← 소켓이 테이블에서 사라짐
t3  캡처기가 폴링     ← 없다. PID 미상.
```

수명이 폴링 주기보다 짧은 연결은 원리적으로 놓친다. 짧은 REST 호출이 정확히
그런 모양이라 이 손실은 실무적으로 크다.

완화 수단과 그 한계:

| 수단 | 효과 | 대가 |
|---|---|---|
| 폴링 주기 단축(50~100ms) | 손실률 감소 | CPU 상승. macOS 전체 fd 순회는 특히 비싸다 |
| **연결 시작 시점 조회** | 큰 개선 — 소켓이 확실히 살아 있다 | 연결 시작을 알아야 한다(프록시/eBPF는 안다, pcap은 SYN으로 추정) |
| 소켓 테이블 캐시 + LRU | 종료 직후 조회 성공 | 포트 재사용 시 **오귀속 위험** |
| 이벤트 기반 전환 | 완전 해결 | eBPF/ETW/NE 필요 |

**포트 재사용 오귀속이 캐시의 진짜 위험이다.** 캐시 항목에는 반드시 유효기간과
관측 시각을 붙이고, 만료된 항목으로 귀속한 트랜잭션은 `attribution: "inferred"`로
표시해 확정 정보와 구분한다. 확신 없는 귀속을 확신 있게 표시하면 분석이 틀린다.

### 4.4 프록시 모드의 구조적 이점

3.2에서 "의외의 강점"이라고 적은 이유가 여기 있다.

프록시는 **자기 자신이 연결의 종단**이다. 클라이언트가 프록시에 붙으면 그 TCP
연결은 프록시가 요청을 처리하는 **내내 살아 있다.** 따라서:

```go
// 프록시가 클라이언트 연결을 수락한 직후 — 이 시점에 소켓은 반드시 살아 있다
raddr := conn.RemoteAddr().(*net.TCPAddr)   // 클라이언트의 (IP, 포트)
pid   := lookupPIDByLocalEndpoint(raddr)     // 이 5-tuple의 소유자 = 요청 프로세스
```

폴링 방식이면서도 **경합이 없다.** 조회 시점에 연결이 살아 있음이 구조적으로
보장되기 때문이다. 게다가 조회는 연결당 1회이고(요청당이 아니라), keep-alive
덕분에 다수 요청이 한 번의 조회를 공유한다.

이것이 1순위로 프록시를 택한 두 번째 이유다. HTTPS 평문뿐 아니라 **프로세스
식별의 정확성**까지 일반 사용자 권한으로 얻는다.

### 4.5 프로세스 메타데이터와 트리 구성

PID를 얻은 뒤 수집하는 정보다.

| 필드 | macOS | Linux | Windows |
|---|---|---|---|
| 실행 경로 | `proc_pidpath` | `/proc/<pid>/exe` | `QueryFullProcessImageName` |
| 명령줄 | `sysctl KERN_PROCARGS2` | `/proc/<pid>/cmdline` | PEB / WMI |
| 부모 PID | `KERN_PROC_PID` | `/proc/<pid>/stat` | `CreateToolhelp32Snapshot` |
| 시작 시각 | 〃 | `/proc/<pid>/stat` (btime 기준 환산) | `GetProcessTimes` |
| 사용자 | 〃 | `/proc/<pid>/status` | 토큰 |

**PID 재사용 문제**: PID는 재사용되므로 PID 단독은 프로세스 식별자가 될 수 없다.
`(pid, start_time)` 쌍을 **`ProcessKey`**로 삼는다. 시작 시각이 다르면 다른
프로세스다.

트리 루트 결정에는 판단이 들어간다. 순수 PID 기준이면 Chrome 하나가 렌더러
수십 개로 흩어지고, 실행 경로 기준이면 서로 다른 JVM 배치가 한 덩어리가 된다.
그래서 **2단 트리**를 채택한다.

```
Google Chrome                    (실행 경로 기준 그룹)     412 요청
├─ chrome (helper) pid 4821      (ProcessKey)             318
└─ chrome (gpu)    pid 4830                                94
java                                                       57
├─ pid 9102  -jar batch.jar --job=sync                     57
└─ …
(미식별)                                                    12
```

상위 = 실행 경로 그룹, 하위 = `ProcessKey`. 명령줄을 하위 라벨에 붙여야 같은
바이너리의 여러 인스턴스를 사람이 구분할 수 있다. `(미식별)` 루트는 **항상
존재하며 0건이어도 표시한다** — 미식별이 0이라는 사실 자체가 커버리지 정보다.

## 5. HTTPS 처리 전략

### 5.1 CA 생성과 신뢰

MITM은 자체 루트 CA를 만들고, 방문 호스트마다 리프 인증서를 즉석 서명한다.

| 항목 | 결정 | 근거 |
|---|---|---|
| CA 키 | P-256 ECDSA | RSA-2048 대비 리프 발급이 빠르다 |
| CA 유효기간 | 1년, 만료 시 재생성 | 영구 CA는 위험이 누적된다 |
| CA 저장 | OS 키체인/자격증명 저장소, 파일 폴백 시 0600 | 11.2 |
| 리프 캐시 | 호스트별 메모리 캐시, 유효기간 2일 | 재발급 비용 절감 |
| SAN | 원 인증서의 SAN 복제 + SNI | 와일드카드 인증서 대응 |
| 신뢰 등록 | **사용자 명시 동작으로만.** 자동 등록 금지 | 11.2 |

CA 신뢰 등록은 이 제품에서 **가장 위험한 단일 동작**이다. 등록된 CA의 개인키가
유출되면 그 머신의 모든 TLS가 위조 가능해진다. 따라서:

- 등록은 별도 화면에서 **위험을 설명한 뒤** 사용자 확인을 받아 수행한다.
- 등록 시점에 관리자 인증을 요구한다(OS가 요구하는 대로).
- **제거 경로를 같은 화면에 둔다.** 등록만 쉽고 제거가 어려우면 안 된다.
- 앱 제거 시 CA 제거를 안내한다(자동 제거는 권한 문제로 보장 못 함 — 이를 숨기지
  않고 명시).

### 5.2 업스트림 검증은 유지한다

MITM 프록시가 흔히 저지르는 실수는 업스트림 TLS 검증을 끄는 것이다. 그러면
도구 자체가 중간자 공격의 통로가 된다.

**프록시는 업스트림 인증서를 정상 검증한다.** 검증 실패 시 트랜잭션을 실패로
기록하고 클라이언트에게 오류를 돌려준다. 사용자가 특정 호스트에 대해 예외를
지정할 수 있으나 **호스트 단위 명시 예외**만 허용하고 전역 무효화는 제공하지
않는다.

원 서버 인증서의 정보(발급자, 만료, SAN, 체인, TLS 버전, 협상된 암호군, ALPN)는
그대로 캡처해 트랜잭션 상세에 표시한다. 실무에서 자주 필요한 정보다.

### 5.3 피닝 대응 — 우회하지 않는다

인증서 피닝을 쓰는 앱은 MITM 시 연결이 실패한다. 이는 **버그가 아니라 보안이
의도대로 동작한 것**이다.

동작 규칙 (2026-07-19 개정 — 자동 패스스루를 철회한다):

1. TLS 핸드셰이크 실패를 감지하면 **원인을 먼저 진단한다.** 핸드셰이크 실패는
   피닝만의 증거가 아니다. CA 미신뢰, 프로토콜/cipher 불일치, 만료된 인증서,
   클라이언트 인증서 요구 실패도 같은 증상을 낸다. **실패를 곧바로 피닝으로
   간주해 자동 우회하면 이 모든 경우를 함께 우회하게 되고**, 사용자는 자기 환경의
   실제 문제를 보지 못한다.
2. 패스스루는 **자동으로 켜지지 않는다.** 다음 둘 중 하나여야 한다.
   - 진단 결과를 보여준 뒤 사용자가 명시적으로 승인
   - 사전에 등록된 allowlist에 해당
3. 패스스루 항목은 범위를 명시한다 — `(process identity, host, port, expiry)`.
   기본 만료는 현재 세션이다. 무기한 전역 예외를 만들지 않는다.
4. 통과시킨 조합은 복호화 없이 메타데이터(SNI, 바이트 수, 타이밍)만 기록한다.
5. UI는 해당 트랜잭션을 `coverage: "metadata_only"`로 표시하고 **진단된 이유를
   명시한다** — "이 앱은 인증서 피닝을 사용하는 것으로 보입니다"와 "ArchScope CA가
   이 앱의 신뢰 저장소에 없습니다"는 사용자가 취할 행동이 전혀 다르다.

자동 패스스루가 "투명한 동작"도 아니라는 점을 덧붙인다. 그 시점에 **첫 연결은
이미 실패한 뒤**이므로, 사용자는 어차피 실패를 한 번 겪는다. 그렇다면 실패
원인을 알려주는 편이 낫다.

피닝 우회 기능은 제공하지 않는다(1.3). 사용자가 자기 앱을 디버깅하려면 그 앱의
디버그 빌드를 쓰거나 방식 D(키 로그)를 쓰면 된다 — 그것이 올바른 경로다.

### 5.4 키 로그 경로

방식 D는 별도 조합으로 지원한다.

```
[SSLKEYLOGFILE=keys.log 로 앱 실행] + [pcap 파일]
                    └──────┬──────┘
                     오프라인 결합 → 복호화 → HTTP 파싱
```

트랜잭션에 `capture_mode: "pcap_keylog"`가 붙는다. 침습이 0이라 프록시 모드
결과를 검증하는 기준선(baseline)으로 쓸 수 있다.

## 6. 데이터 모델

### 6.1 계층

```
CaptureSession            캡처 1회 (시작~종료, 모드, 호스트 환경)
└─ ProcessGroup           실행 경로 기준 묶음          ← 트리 1단
   └─ ProcessInstance     ProcessKey(pid, startTime)  ← 트리 2단
      └─ Connection       TCP/TLS 연결 (5-tuple, ALPN, 인증서)
         └─ Transaction   요청/응답 1쌍               ← 분석 최소 단위
            ├─ Request    메서드, URL, 헤더, 바디
            ├─ Response   상태, 헤더, 바디
            └─ Timings    6구간
```

`Connection` 계층을 별도로 두는 이유는 **keep-alive와 HTTP/2 멀티플렉싱** 때문
이다. 한 연결이 다수 트랜잭션을 나르고, 연결 수립 비용(DNS/TCP/TLS)은 그 중 첫
트랜잭션에만 귀속된다. 이를 구분하지 않으면 "왜 이 요청만 300ms 느린가"에 답할 수
없다.

### 6.2 Go 타입 (초안)

```go
// internal/models/http_capture.go
package models

type ProcessKey struct {
    PID       int32  `json:"pid"`
    StartTime string `json:"startTime"` // RFC3339Nano — PID 재사용 구분자
}

type ProcessInstance struct {
    Key         ProcessKey `json:"key"`
    Name        string     `json:"name"`
    ExecPath    string     `json:"execPath"`
    CommandLine string     `json:"commandLine,omitempty"`
    User        string     `json:"user,omitempty"`
    ParentPID   int32      `json:"parentPid,omitempty"`
    // "confirmed" | "inferred" | "unknown" — 4.3의 신뢰도
    Attribution string     `json:"attribution"`
}

type Connection struct {
    ID          string `json:"id"`
    Process     *ProcessKey `json:"process,omitempty"` // nil이면 미식별
    Protocol    string `json:"protocol"`   // "tcp"
    LocalAddr   string `json:"localAddr"`
    LocalPort   int    `json:"localPort"`
    RemoteAddr  string `json:"remoteAddr"`
    RemotePort  int    `json:"remotePort"`
    ServerName  string `json:"serverName,omitempty"` // SNI
    ALPN        string `json:"alpn,omitempty"`       // "h2" | "http/1.1" | "h3"
    TLSVersion  string `json:"tlsVersion,omitempty"`
    CipherSuite string `json:"cipherSuite,omitempty"`
    PeerCert    *CertInfo `json:"peerCert,omitempty"`
    OpenedAt    string `json:"openedAt"`
    ClosedAt    string `json:"closedAt,omitempty"`
}

type HTTPMessage struct {
    // 관측된 순서와 중복을 보존하기 위해 map이 아니라 슬라이스 (R3).
    // 단 "원본 wire 순서"는 fidelity 등급이 raw_wire일 때만 보장된다 — 6.2.1
    Headers      []HeaderField `json:"headers"`
    // 관측 지점에서 실제로 읽은 전송 바이트. 자동 해제가 적용된 경로에서는
    // 측정 불가이며 -1(unknown)로 둔다. 0과 unknown을 섞지 않는다 — 6.2.1
    BodySize     int64  `json:"bodySize"`
    BodyDecoded  int64  `json:"bodyDecoded"`   // 해제 후
    BodyEncoding string `json:"bodyEncoding,omitempty"` // gzip, br, zstd
    ContentType  string `json:"contentType,omitempty"`
    // "inline" | "blob" | "truncated" | "omitted" | "redacted"
    BodyStorage  string `json:"bodyStorage"`
    BodyRef      string `json:"bodyRef,omitempty"` // blob 저장소 키
    BodyPreview  string `json:"bodyPreview,omitempty"`
}

type HeaderField struct {
    Name     string `json:"name"`
    Value    string `json:"value"`
    Redacted bool   `json:"redacted,omitempty"`
}

type Transaction struct {
    ID           string `json:"id"`
    ConnectionID string `json:"connectionId"`
    Sequence     int    `json:"sequence"`         // 연결 내 순번
    StreamID     int    `json:"streamId,omitempty"` // HTTP/2

    Method   string `json:"method"`
    URL      string `json:"url"`
    Scheme   string `json:"scheme"`
    Host     string `json:"host"`
    Path     string `json:"path"`
    Query    string `json:"query,omitempty"`
    HTTPVersion string `json:"httpVersion"`

    StatusCode int    `json:"statusCode"`
    StatusText string `json:"statusText,omitempty"`

    Request  HTTPMessage `json:"request"`
    Response HTTPMessage `json:"response"`
    // 관점별로 분리한다 — 6.3
    Timings  TimingSet   `json:"timings"`

    // 연결 내에서 이 트랜잭션이 기존 연결을 재사용했는지.
    // Connection에 두면 연결 수명 전체를 하나의 bool로 뭉갠다 — 6.3
    UsedExistingConnection bool `json:"usedExistingConnection"`

    // 모든 시각은 측정 지점(관측자)을 함께 기록한다 — 6.2.2
    StartedAt string  `json:"startedAt"`
    EndedAt   string  `json:"endedAt,omitempty"`
    State     TxState `json:"state"` // 7.4.5 — 진행 중/완료/실패/중단
    TotalMs   float64 `json:"totalMs"`

    // "proxy_mitm" | "proxy_passthrough" | "pcap" | "pcap_keylog" | "ebpf" | "etw" | "har_import"
    CaptureMode string `json:"captureMode"`
    // 관측 범위 — "full" | "headers_only" | "metadata_only"
    Coverage    string `json:"coverage"`
    // 충실도 등급 — "semantic" | "decoded_wire" | "raw_wire" (6.2.1)
    Fidelity    string `json:"fidelity"`
    Error       string `json:"error,omitempty"`
}
```

`Fidelity`와 `CaptureMode`가 모든 트랜잭션에 붙는 것이 원칙 B의 구현이다. 서로
다른 백엔드가 만든 트랜잭션이 한 뷰에 섞이므로, 각 행이 **무엇을 얼마나
관측했는지**를 자기 자신이 말해야 한다.

#### 6.2.1 fidelity 등급 — R3의 실제 보장 범위

2026-07-19 검토가 지적한 모순을 여기서 해소한다. **"원본 헤더 순서·casing·중복
보존"과 "Go `net/http` 기반 프록시"는 그대로 양립하지 않는다.** `http.Header`는
`map[string][]string`이고 field name을 canonical form으로 바꾼다. map을 거친
뒤에는 서로 다른 헤더 이름 사이의 원래 순서와 원래 casing을 복구할 수 없고, 일부
중복 응답 헤더는 결합될 수 있다. HTTP/2는 pseudo-header·HPACK decode·trailer가
있어 "원본 순서"라는 말의 의미부터 HTTP/1.1과 다르다.

따라서 R3을 하나의 절대 요구로 두지 않고 **세 등급으로 분리**한다.

| 등급 | 보장 | 얻는 방법 |
|---|---|---|
| `semantic` | 헤더 이름(canonical)·값·중복 다중값, 해제된 바디, 시맨틱 타이밍 | Go `net/http` 기반 프록시로 달성 가능 |
| `decoded_wire` | 위 + 관측 지점의 전송 바이트 수, 압축 인코딩 식별, chunked/H2 프레이밍 바이트 구분 | 자동 해제를 끄고 wire byte를 별도 계측 |
| `raw_wire` | 위 + 원본 field 순서·casing·중복, 원본 프레이밍 | 전용 raw parser/spool 경로가 있을 때만 |

**Phase 2의 기본 보장은 `semantic`이다.** `decoded_wire`는 `Transport`의 자동
gzip 요청·투명 해제를 끄고(`Response.Uncompressed` 경로를 피하고) 계측 계층을
따로 둔 뒤에만 주장한다. `raw_wire`는 Phase 2 범위 밖이며, 별도 경로가 생기기
전까지 **문서·UI 어디에서도 약속하지 않는다.**

바이트 수는 세 가지를 구분해 저장한다 — decoded entity byte, 전송(압축) byte,
프레이밍 포함 wire byte. 셋을 하나의 `bodySize`로 뭉개면 "압축이 실제로 먹히고
있는가"라는 질문에 답할 수 없다.

golden test는 최소 다음을 덮는다: HTTP/1.1과 HTTP/2, 1xx interim response,
CONNECT, 스트리밍(SSE/chunked), trailer, WebSocket upgrade, 클라이언트 취소,
자동/수동 decompression.

#### 6.2.2 측정 지점

`StartedAt`·`EndedAt`·`State`·바이트 카운터는 **누가 어디서 관측했는지**를
같이 적어야 의미가 확정된다. 프록시 모드에서 `StartedAt`은 "클라이언트가 프록시에
요청을 보내기 시작한 시각"이며, 업스트림 서버가 요청을 받은 시각이 아니다. 가져온
HAR에서는 생성 도구의 관측 지점이 그대로 승계된다. 세션 메타데이터에
`observationPoint`(`client_proxy` | `proxy_upstream` | `foreign_tool`)를 기록하고
UI는 이를 숨기지 않는다.

### 6.3 타이밍 모델

W3C Resource Timing과 HAR `timings`를 따른다. 재발명하지 않는 이유는 HAR
상호운용(6.5)과 브라우저 데이터와의 비교 가능성 때문이다.

**하나의 `Timings`로 두 관점을 모두 담을 수는 없다.** 개정 전 본문은 값 하나에
"프록시↔서버 구간"과 "클라이언트 관점"을 동시에 저장한다고 적어 자기모순이었다.
MITM 프록시는 원래 클라이언트의 DNS 해석·서버 TCP 핸드셰이크·서버 TLS 핸드셰이크를
**직접 관측하지 못한다.** 관점을 타입으로 분리한다.

```go
type TimingSet struct {
    ClientProxy   *TimingPhases `json:"clientProxy,omitempty"`   // 클라이언트 ↔ 프록시
    ProxyInternal *TimingPhases `json:"proxyInternal,omitempty"` // 프록시 큐·처리
    ProxyUpstream *TimingPhases `json:"proxyUpstream,omitempty"` // 프록시 ↔ 서버
    ImportedHar   *TimingPhases `json:"importedHar,omitempty"`   // 외부 도구 관측값
}

// 값 상태 — 0으로 세 상태를 표현하지 않는다
type TimingState string // "known" | "not_applicable" | "unknown"

type Duration struct {
    Ms    float64     `json:"ms"`
    State TimingState `json:"state"`
}

type TimingPhases struct {
    Blocked Duration `json:"blocked"` // 큐 대기 (커넥션 풀 등)
    DNS     Duration `json:"dns"`     // 이름 해석
    Connect Duration `json:"connect"` // TCP 핸드셰이크 (TLS 제외)
    TLS     Duration `json:"tls"`     // TLS 핸드셰이크
    Send    Duration `json:"send"`    // 요청 전송
    Wait    Duration `json:"wait"`    // TTFB — 아래 2항 참조
    Receive Duration `json:"receive"` // 응답 본문 수신
}
```

`float64` 하나로는 "0 ms였다", "이 관점에는 해당 없다", "측정하지 못했다"를
구분할 수 없다. 이 구분이 없으면 재사용 연결의 DNS 0 ms와 프록시가 볼 수 없는
클라이언트 DNS가 같은 값으로 보인다. `State`를 필수로 두는 이유다.

주의점 다섯 가지를 명시한다. 4항은 2026-07-18 조사에서, 1·2항 개정은 2026-07-19
검토에서 왔다.

1. **재사용 연결에서는 DNS/Connect/TLS가 `not_applicable`이다.** 0 ms가 아니다.
   연결 재사용 여부는 `Connection.Reused`가 아니라 **트랜잭션의
   `UsedExistingConnection`** 또는 연결 내 `Sequence > 0`으로 판정한다. 연결
   하나에 bool 하나를 두면 첫 요청과 이후 요청이 구분되지 않는다.
2. **`Wait`는 "서버 처리 시간"이 아니다.** TTFB에는 프록시 큐 대기, 네트워크 왕복,
   서버 큐, 서버 처리가 함께 들어간다. UI 레이블도 "서버 처리"가 아니라 "첫 바이트
   대기(TTFB)"로 쓴다. 서버 처리 시간을 알고 싶으면 12절의 교차 분석이 필요하다.
   프록시 모드에서 클라이언트 관점 값은 `ProxyUpstream`의 합에 `ProxyInternal`을
   더해 **추정**할 뿐이며, 추정값에는 `state: "unknown"`이 아니라 별도
   `derived: true` 표시를 붙인다.
3. **HTTP/2 다중 스트림에서 `Blocked`의 의미가 달라진다.** 동일 연결의 다른
   스트림과 대역폭을 나눠 쓰므로 `ReceiveMs`가 서버 성능과 무관하게 늘어난다.
4. **`ssl` 구간의 귀속이 도구마다 다르다.** HAR 규격은 `ssl`이 `connect`에
   **포함**된다고 규정하는데(하위 호환 목적), 실제 구현이 갈린다.

   | 생성 도구 | `entry.time` 계산 | 결과 |
   |---|---|---|
   | Chrome | `blocked+dns+connect+send+wait+receive` — **`ssl` 제외** | 규격 준수 |
   | Firefox | `blocked+dns+connect+ssl+send+wait+receive` — **`ssl` 포함** | **TLS 이중 계상** |

   두 코드베이스에서 각각 확인된 사실이다 `[V]`. **도구가 다른 HAR의
   `entry.time`을 정규화 없이 비교하면 안 된다.**

불변식은 조건부로 기술해야 한다.

```
정규화 후, 단일 관점 내에서, state=known인 구간만 합산:
  TotalMs ≈ Blocked + DNS + Connect + TLS + Send + Wait + Receive
```

**관점을 섞어서 합산하지 않는다.** `ClientProxy`와 `ProxyUpstream`을 더하면
프록시 처리 구간이 이중 계상된다. ArchScope 내부 모델은 `Connect`와 `TLS`를
**분리 보관**하므로 합에 둘 다 들어간다. 따라서 HAR 입력은 6.5.4절의 정규화를 **통과한 뒤에야** 이 불변식을
적용한다.

**정규화 전에 불변식을 검사하면 `CAPTURE_TIMING_INCONSISTENT`가 모든 Firefox
HAR에서 오탐한다.** 진단 finding(`AddFinding`, `analysis_result.go:135`)은
정규화 이후 단계에만 건다. 정규화 자체가 실패한 경우는 별도 코드
`HAR_TIMING_DIALECT_UNKNOWN`으로 구분한다 — 둘은 원인도 대응도 다르다.

#### 6.3.1 트랜잭션의 경계 — 무엇이 한 건인가

타이밍 구간을 나누기 전에 **구간의 바깥 경계**가 정의되어야 한다. 개정 전 문서는
`StartedAt`/`EndedAt`/`TotalMs`를 필드로만 두고 그 시각이 무슨 사건인지 적지
않았다. 카운터·백분위수·`inFlight`가 전부 이 정의 위에 서므로 P0다.

**단위 규칙: 트랜잭션 하나 = 요청 메시지 하나 + 그에 대응하는 최종 응답 하나.**

| 경계 사건 | 정의 |
|---|---|
| **시작** (`StartedAt`) | 관측 지점에서 **요청 라인(H1) 또는 HEADERS 프레임(H2)의 첫 바이트**를 읽은 시각 |
| **끝** (`EndedAt`) | 응답 본문의 마지막 바이트 — trailer가 있으면 trailer까지 — 를 읽은 시각. 또는 종단 상태(`failed`/`aborted`)가 확정된 시각 |

`StartedAt`을 "연결 수립"으로 잡지 않는 이유는 **연결 재사용 시 두 트랜잭션이
같은 시작 시각을 갖게 되기 때문**이다. `Blocked`(커넥션 풀 대기)는 시작 이전이
아니라 시작 이후 구간으로 모델링한다.

경계가 헷갈리는 경우를 전부 고정한다.

| 상황 | 트랜잭션 수 | 근거 |
|---|---|---|
| 리다이렉트 체인 (301 → 200) | **각각 1건.** 총 2건 | 각각 별개의 요청·응답 쌍이다. UI가 체인으로 묶어 보여주는 것은 표시의 문제 |
| 1xx interim (100 Continue, 103 Early Hints) | **최종 응답과 합쳐 1건** | interim은 최종 응답이 아니다. 도착 시각은 `interimAt[]`에 별도 기록 |
| 401 → 재요청 (인증 챌린지) | **각각 1건.** 총 2건 | 위 리다이렉트와 동일 |
| 클라이언트 재시도 | **각각 1건** | 재시도임을 알 수 없는 경우가 많다. 추론으로 묶지 않는다 |
| `CONNECT` 터널 | **터널 수립이 1건**, 터널 내부 트랜잭션이 각각 1건 | 수립은 `Coverage: metadata_only`. 복호화 시 내부 건이 별도로 생긴다 |
| WebSocket upgrade | **upgrade 핸드셰이크가 1건** | 이후 메시지 스트림은 트랜잭션이 아니다. 메시지 파싱은 Phase 5 |
| H2 서버 푸시 | **1건** (요청 없이) | `StartedAt`은 PUSH_PROMISE 수신 시각. `Method`/`URL`은 promise에서 온다 |
| 응답 없이 연결 끊김 | **1건**, `aborted` | `EndedAt`은 끊김 감지 시각. `TotalMs`는 `known` |

#### 6.3.2 시간 기준점과 단위

세 가지를 분리한다. 하나로 뭉치면 NTP 조정이나 절전 복귀 때 음수 구간이 나온다.

| 용도 | 원천 | 필드 | 단위 |
|---|---|---|---|
| **구간 길이** 측정 | monotonic clock (`time.Since`) | `TimingPhases.*`, `TotalMs` | ms, `float64` |
| **절대 시각** 표시·정렬 | wall clock | `StartedAt`, `EndedAt` | RFC3339Nano 문자열 |
| **버킷 인덱스** 계산 | 세션 epoch 기준 monotonic 경과 | `Bucket.StartMs` | ms, `int64` |

계약을 명시한다.

- **모든 구간 길이는 monotonic clock에서 나온다.** 두 wall clock 시각을 빼서
  구간을 만들지 않는다. Go의 `time.Time`은 monotonic reading을 함께 갖지만
  **직렬화·역직렬화하면 소실**되므로, NDJSON에 쓴 시각을 다시 읽어 빼는 경로를
  만들지 않는다. 구간은 계산된 뒤에 저장한다.
- **단위는 밀리초 `float64`로 통일한다.** HAR `timings`가 ms이고(6.5), 브라우저
  데이터와 직접 비교하는 것이 이 모델의 목적이기 때문이다. 내부 측정 분해능은
  ns이지만 **저장·비교·표시는 전부 ms**이며, 소수점 이하를 버리지 않는다 —
  1 ms 미만 구간을 0으로 만들면 재사용 연결의 `Send`가 사라진다.
- **음수 구간은 허용하지 않는다.** monotonic 원천에서는 발생할 수 없고, 가져온
  HAR에서 발생하면 그 필드를 `state: "unknown"`으로 낮추고
  `HAR_TIMING_NEGATIVE` finding을 남긴다. 0으로 잘라내지 않는다 — 그러면 잘못된
  입력이 정상값으로 위장한다.
- **`TotalMs`는 구간 합이 아니라 독립 측정값이다.** 시작과 끝 사이의 monotonic
  경과다. 합과 일치하는지는 6.3.4의 불변식이 검사하며, **불일치는 오류가 아니라
  관측 가능한 사실**이다(측정되지 않은 구간이 존재한다는 뜻).

#### 6.3.3 상태 기계

7.4.5가 `TxState` 값을 정의하고, 여기서 **전이**를 고정한다. 값만 있고 전이가
없으면 구현마다 다른 상태 순서가 나온다.

```
                 ┌──────────────→ failed
                 │                  ▲
  request_sent ──┼──→ receiving ────┤
                 │        │         │
                 └────────┴──→ aborted
                          │
                          └──→ complete
```

| 전이 | 조건 | 부수 효과 |
|---|---|---|
| → `request_sent` | 요청 첫 바이트 관측 | `StartedAt` 확정. `inFlight` 증가 |
| `request_sent` → `receiving` | 응답 상태 라인/HEADERS 관측 | `StatusCode` 확정. `Wait` 구간 종료 |
| `receiving` → `complete` | 본문(+trailer) 완료 | `EndedAt`·`TotalMs` 확정. **집계 계층에 반영**. `inFlight` 감소 |
| `request_sent`/`receiving` → `failed` | 업스트림 오류, TLS 검증 실패, 프로토콜 오류 | `Error` 기록. `EndedAt` 확정. 집계에 오류로 반영 |
| `request_sent`/`receiving` → `aborted` | 클라이언트 취소, 연결 끊김, **캡처 정지 시 미완료** | `EndedAt` 확정. 집계의 백분위수에는 **넣지 않는다** |

규칙 셋을 명시한다.

- **종단 상태는 `complete`/`failed`/`aborted` 셋뿐이며 전이는 단방향이다.** 종단에
  도달한 트랜잭션은 갱신되지 않는다. UI의 행 갱신(7.4.5)도 종단 도달까지만이다.
- **집계에는 `complete`와 `failed`만 반영한다.** `aborted`는 지속시간이 사용자
  행동에 의해 결정되므로 백분위수를 오염시킨다. 단 `aborted` 카운터는 별도로
  노출한다 — **중단율 자체가 진단 신호**다(타임아웃 설정 오류 등).
- **`request_sent`에서 응답 없이 캡처가 끝나면 `aborted`이지 `failed`가 아니다.**
  서버가 응답하지 못한 것과 우리가 관측을 멈춘 것은 다르다.

#### 6.3.4 계약 버전과 골든 불변식

이 모델은 저장 포맷이므로 **버전이 붙는다.** `manifest.json`의
`captureSchemaVersion`이 6.3~6.4 전체의 버전이며(7.6), 아래 불변식은 그 버전의
수용 기준이다.

**전 버전 공통 불변식** — 위반 시 구현 버그다.

| ID | 불변식 |
|---|---|
| `INV-1` | `EndedAt ≥ StartedAt`, `TotalMs ≥ 0` |
| `INV-2` | 종단 상태의 트랜잭션은 `EndedAt`과 `TotalMs`가 `known` |
| `INV-3` | `state != known`인 구간은 합산에 참여하지 않는다 |
| `INV-4` | 한 관점(`TimingSet`의 한 필드) 내에서만 합산한다 |
| `INV-5` | `UsedExistingConnection == true`이면 `DNS`/`Connect`/`TLS`는 `not_applicable` |
| `INV-6` | 연결 내 `Sequence == 0`인 트랜잭션은 `UsedExistingConnection == false` |
| `INV-7` | `sum(known 구간) ≤ TotalMs + ε` (ε = 1 ms). **초과하면 이중 계상이다** |

`INV-7`이 부등식인 것이 요점이다. 6.3.2가 말했듯 `TotalMs`는 독립 측정값이므로
**측정되지 않은 구간만큼 합이 작을 수 있다.** 작은 것은 정상이고, 큰 것은 버그다.
`CAPTURE_TIMING_INCONSISTENT`는 초과일 때만 warn으로 올린다.

**HTTP/1.1 golden 불변식**

| ID | 불변식 |
|---|---|
| `INV-H1-1` | 같은 연결의 트랜잭션은 `Sequence`가 0부터 연속이며, 시간 구간이 겹치지 않는다 (파이프라이닝 미지원 전제) |
| `INV-H1-2` | `Sequence > 0`이면 `UsedExistingConnection == true` |
| `INV-H1-3` | `StreamID`는 비어 있다 |

**HTTP/2 golden 불변식**

| ID | 불변식 |
|---|---|
| `INV-H2-1` | 같은 연결의 트랜잭션 시간 구간은 **겹칠 수 있다.** 겹침을 오류로 판정하지 않는다 |
| `INV-H2-2` | `StreamID`가 연결 내에서 유일하다 |
| `INV-H2-3` | 첫 스트림을 제외한 모든 트랜잭션이 `UsedExistingConnection == true`, `DNS`/`Connect`/`TLS`는 `not_applicable` |
| `INV-H2-4` | `Blocked`는 커넥션 풀 대기가 아니라 **스트림 다중화 대기**를 뜻한다. 두 의미를 같은 필드에 담되 `TimingPhases`의 해석은 `HTTPVersion`에 따른다 (6.3-3) |
| `INV-H2-5` | 서버 푸시 트랜잭션은 요청 헤더가 비어 있어도 유효하다 |

`INV-H1-1`과 `INV-H2-1`이 정반대라는 점이 이 표를 나눈 이유다. **버전 무관 겹침
검사를 하나 두면 H2에서 전량 오탐한다.**

이 불변식들은 golden fixture로 강제한다. 6.2.1의 golden test 목록(H1/H2, 1xx,
CONNECT, 스트리밍, trailer, WebSocket upgrade, 취소, decompression)에 **각
fixture가 위 ID 중 무엇을 검증하는지 매핑**하는 것이 Phase 0 게이트 4의 완료
조건이다.

### 6.4 바디 저장 정책

바디는 데이터 크기와 민감도가 동시에 폭발하는 지점이라 정책이 필요하다.

| 조건 | 처리 |
|---|---|
| ≤ 64 KiB, 텍스트류 | `inline` — 세션 파일에 직접 |
| > 64 KiB | `blob` — 별도 파일, `BodyRef`로 참조 |
| > 설정 상한(기본 10 MiB) | `truncated` — 앞부분만 |
| 미디어/스트림(video, event-stream) | `omitted` — 크기만 |
| 민감 패턴 감지 | `redacted` — 11.3 |
| 사용자가 바디 수집 끔 | 전부 `omitted` |

세션 산출물은 **디렉터리**다.

```
capture-20260718-142211/
├─ session.json        메타 + 프로세스/연결/트랜잭션 (바디 제외)
├─ bodies/             blob (랜덤 opaque ID 파일명 — 11.3.1)
└─ manifest.json       손상 탐지 해시, 스키마 버전, 리댁션 정책 기록
```

blob 파일명을 **내용 해시로 두지 않는다.** 중복 제거에는 유리하지만 세션 간 동일
내용 누출과 알려진 작은 값의 사전대입을 허용한다(11.3.1). 중복 제거가 필요하면
manifest 내부의 keyed 값으로 처리하고 파일명에는 노출하지 않는다.

`manifest.json`에 리댁션 정책을 기록하는 것이 중요하다. 나중에 이 캡처를 여는
사람이 **무엇이 지워졌는지** 알아야 결론을 신뢰할 수 있다. 단 이 해시는 손상
탐지용이며 변조 방지가 아니다(11.3.2).

### 6.5 HAR 상호운용 — 사후 분석 경로

R9의 설계다. HAR은 이 영역의 사실상 표준이며 원칙 A에 따라 **1급 입출력
포맷으로 삼는다.** 캡처 기능 없이도 이 경로만으로 제품 가치가 성립한다 —
Phase 1의 근거다.

그러나 조사 결과 **HAR은 하나의 포맷이 아니라 여러 방언의 집합**이며, 규격 자체가
관리되지 않는다. 이 절은 그 현실을 설계에 반영한다.

**"네 개의 방언"이라는 요약은 철회한다 (2026-07-19).** 7개 생성 도구를 열거하면서
방언 수를 4로 못 박은 것은 내부적으로 맞지 않았다. 방언 수는 제품 수와 같지도,
고정되지도 않는다. 대신 `DialectID`와 **feature matrix**로 정의한다.

```go
type DialectID string // "chrome" | "firefox" | "safari" | "charles" | ...

type DialectFeatures struct {
    ID              DialectID
    CreatorMatch    []string // log.creator.name 매칭 규칙
    SSLInConnect    bool     // entry.time에 ssl이 포함되는가 (6.3의 4항)
    BodySizeOnH2    bool     // HTTP/2 응답에서 bodySize를 주는가
    EncodesBase64   bool
    ProvidesProcess bool     // client process 정보를 담는가
    // 새 차이가 발견되면 여기에 필드를 추가한다 — 방언 수를 세지 않는다
}
```

미확인 creator는 `unknown` 방언으로 두고 **보수적 기본값**(정규화 없음 + 경고
finding)을 적용한다. 방언을 잘못 추정해 조용히 값을 바꾸는 것이 최악이다.

#### 6.5.1 규격의 실제 상태

| 위치 | 상태 |
|---|---|
| W3C 편집 초안 | **폐기됨.** 문서 상단에 *"never published… has been abandoned"* 와 **DO NOT USE** 고지. 2012-08-14 이후 갱신 없음 `[V]` |
| Odvarko 원본 블로그 규격 | 살아 있으나 **TLS 인증서 만료** — 정상 fetch 불가 `[V]` |
| `har-schema` (JSON Schema draft-06) | npm 2.0.0, 2017년 마지막 배포. **사실상 유일한 규범적 산출물** `[V]` |
| 제안된 1.3 | 미비준, 구현체 없음 `[V]` |

> **결정: `har-schema`의 `required` 배열을 계약으로 삼는다.** 산문 규격은 참조만
> 하고, 인용 시 폐기 상태를 명시한다. 스키마가 산문보다 엄밀하고 기계 검증이
> 가능하다.

단 **스키마 검증만으로는 부족하다.** 실제 Firefox export는 엄격한 스키마 검증을
**통과하지 못하며** `[V]`, Insomnia의 조작된 타이밍이나 Safari의 헤더 병합은
스키마가 잡아내지 못한다. 검증은 필요조건이지 충분조건이 아니다.

#### 6.5.2 방언 차이 — 파서가 알아야 할 것

| | Chrome | Firefox | Safari | Insomnia |
|---|---|---|---|---|
| `startedDateTime` | UTC `Z` | **로컬 시간 `±HH:MM`** | UTC `Z` 또는 **빈 문자열** | **전 항목이 동일 타임스탬프** |
| `timings` | 완전, `ssl` 제외 | **`{}` 가능** (required 위반), `ssl` 포함 | 리다이렉트는 전부 `-1`/`0` | **조작됨** |
| 중복 헤더 | 보존 | 보존 | **병합됨** (객체 순회) | 불완전 |
| `cache` | 항상 `{}` | **실제로 채워짐** | `{}` | `{}` |
| `pages` | 있음 | 있음 | 항상 `page_0` 하나 | **아예 없음** |
| WebSocket | `_webSocketMessages` | 없음 | 없음 | 없음 |

**R3(중복 헤더 보존)에 대한 정직한 단서:** Safari HAR은 원본에서 이미 중복 헤더가
병합되어 있다 `[V]`. ArchScope가 복원할 수 없다. 이 경우 `fidelity`를 낮추고
UI에 사유를 표시한다 — 6.2절의 자기 기술 원칙이 여기서 다시 쓰인다.

#### 6.5.3 파서 함정 — 우선순위 순

1. **`headersSize + bodySize`를 계산하지 않는다.** `-1`이 예외가 아니라 일반적
   이다 — Insomnia는 항상, Chrome은 **모든 HTTP/2 이상 응답에서**, Safari는 모든
   리다이렉트에서 `-1`이다 `[V]`. Chrome의 경우 원인이 구조적이다: DevTools에
   원시 헤더 텍스트가 없어 `bodySize`를 계산할 수 없다.
2. **크기 필드 세 개는 서로 다른 양이며 도구 간 호환되지 않는다.** Chrome은
   `content.size`=디코딩 후, `bodySize`=전송된 바디, `_transferSize`=전체 전송량.
   Firefox의 `bodySize`는 **전송량**이라 Chrome의 `_transferSize`에 가깝다 `[V]`.
   `log.creator.name`별로 정규화한다.
3. **`content.encoding` 부재가 평문을 뜻하지 않는다.** Firefox는 base64 본문을
   `encoding` 없이 내보낸 이력이 있다 `[V]`. MIME + base64 형태 휴리스틱으로
   보완한다.
4. **`content.text` 부재는 정상이다.** Chrome은 인스펙터 캐시에서 축출된 바디를
   `size`만 남기고 뺀다. 오류로 처리하면 안 된다 `[V]`.
5. **`entry.time`을 신뢰하지 말고 재계산한다.** 6.3절 4항의 `ssl` 문제.
6. **`timings`에 required 멤버가 통째로 없을 수 있다** (Firefox `{}`). 거부하지
   말고 기본값으로 채운 뒤 `fidelity`를 낮춘다 `[V]`.
7. **`startedDateTime`이 UTC라고 가정하지 않는다.** `±HH:MM`과 빈 문자열을
   처리한다. **Insomnia HAR은 이 필드로 정렬하면 안 된다** (전부 같은 값) `[V]`.
8. **리다이렉트 체인에 명시적 링크가 없다.** `response.redirectURL` → 다음
   엔트리의 `request.url`로 재구성한다. Insomnia는 `redirectURL`이 항상 빈
   문자열이라 재구성 불가 `[V]`.
9. `content.compression`은 **음수일 수 있고**(Safari) 아예 없을 수 있다(Chrome의
   캐시/304/206) `[V]`.
10. **`cookies: []`가 쿠키 없음을 뜻하지 않는다** — Chrome 130+는 기본 소독한다.
11. `JSON.parse` 전에 **BOM을 제거**한다 `[V]`.
12. HTTP/2 유사 헤더(`:method` 등) 노출 여부는 **미확인** `[?]`. 방어적으로
    `name.startsWith(":")`를 건너뛰되, **관측된 동작으로 문서에 단정하지 않는다.**

부수 확인: `chrome://net-export`는 **HAR이 아니라 NetLog**다 `[V]`.
`log.version`+`log.entries` 시그니처로 판별해 명확한 오류 메시지와 함께 거부한다.

#### 6.5.4 파이프라인 — 정규화를 1급 단계로

핵심 설계 판단이다. **방언 정규화를 파서에 흩어 놓지 않고 독립 단계로 분리한다.**

```
.har 파일
   │
   ├─(1) 형식 판별      BOM 제거 → log.version/log.entries 확인 → NetLog 등 거부
   │                    확장자에 의존하지 않는다 (mitmproxy 참조 구현)
   ├─(2) 스키마 검증    har-schema required 배열. 실패는 거부가 아니라 finding
   ├─(3) 방언 식별      log.creator.name/version → Dialect 결정
   │                    미상이면 Dialect="generic" + 보수적 규칙
   ├─(4) 정규화         ★ 여기가 이 설계의 핵심
   │                    · ssl 귀속 통일 (6.3절 4항)
   │                    · 크기 필드 3종 의미 통일
   │                    · 시각을 UTC 절대시각으로
   │                    · -1 / 필드 부재 → Unknown 표현으로 (0이 아니다)
   ├─(5) 모델 매핑      → 6.2절 Transaction / Connection
   │                    connection 필드로 Connection 계층 재구성
   ├─(6) 리댁션         11.3절. 가져오기 시점에도 적용한다 (6.5.6)
   └─(7) 분석           기존 analyzers/httpcapture — 캡처 세션과 완전히 동일
```

```go
// internal/parsers/httpcapture/dialect.go
type Dialect string

const (
    DialectChrome   Dialect = "chrome"
    DialectFirefox  Dialect = "firefox"
    DialectSafari   Dialect = "safari"
    DialectCharles  Dialect = "charles"
    DialectFiddler  Dialect = "fiddler"
    DialectProxyman Dialect = "proxyman"
    DialectInsomnia Dialect = "insomnia"
    DialectGeneric  Dialect = "generic" // 미상 — 보수적 처리
)

type Normalizer interface {
    Dialect() Dialect
    // SSLInConnect가 참이면 ssl이 connect에 포함되어 있다고 해석한다
    SSLInConnect() bool
    // TimeIncludesSSL이 참이면 entry.time이 ssl을 이중 계상하고 있다
    TimeIncludesSSL() bool
    NormalizeTimings(har HarTimings) (Timings, []Warning)
    NormalizeSizes(e HarEntry) (SizeSet, []Warning)
    NormalizeTime(raw string) (time.Time, error)
}
```

`Warning`을 반환값에 두는 것이 의도적이다. **정규화 과정에서 잃은 정보는
사용자에게 보고되어야 한다.** 조용히 보정하면 6.7절 진단의 신뢰가 무너진다.

`-1`과 필드 부재를 **`0`으로 만들지 않는 것**이 특히 중요하다. HAR 규격은 두
경우의 의미가 다르다고 규정한다 — 필드 부재는 "이 도구가 이 구간을 아예 측정할
수 없음", `-1`은 "측정 가능하나 이 요청에는 해당 없음"(예: 재사용 연결의
`connect`)이다 `[V]`. 6.3절 1항이 지적한 "재사용 연결의 0을 측정 실패로 오해"
문제와 같은 뿌리다.

#### 6.5.5 진단 finding 추가

6.7절 표에 HAR 전용 코드를 추가한다.

| 코드 | 심각도 | 조건 |
|---|---|---|
| `HAR_DIALECT_UNKNOWN` | info | `creator`로 방언 판별 실패 — generic 규칙 적용 |
| `HAR_SCHEMA_VIOLATION` | warn | required 필드 누락. 거부하지 않고 계속 |
| `HAR_TIMING_DIALECT_UNKNOWN` | warn | `ssl` 귀속을 판정할 수 없음 |
| `HAR_TIMINGS_MISSING` | warn | `timings`가 비었거나 required 멤버 부재 |
| `HAR_SIZES_UNAVAILABLE` | info | `bodySize`/`headersSize`가 대부분 `-1` |
| `HAR_BODIES_ABSENT` | info | `content.text`가 없음 — 크기만 분석 가능 |
| `HAR_HEADERS_COLLAPSED` | warn | Safari 등 중복 헤더 병합 — R3 미충족 |
| `HAR_TIMESTAMPS_DEGENERATE` | warn | 전 항목 동일 시각(Insomnia) — 시계열 분석 불가 |
| `HAR_REDIRECTS_UNLINKABLE` | info | `redirectURL` 부재로 체인 재구성 불가 |
| `HAR_PRESANITIZED` | info | 이미 소독된 것으로 보임 — 6.5.6 |

`HAR_TIMESTAMPS_DEGENERATE`가 중요하다. **8.7절의 전체 타임라인은 이 경우 무의미
하므로 뷰를 비활성화하고 사유를 표시한다.** 12절 원칙("정렬 신뢰도가 낮으면
교차 뷰를 제공하지 않는다")의 HAR판이다.

#### 6.5.6 가져온 HAR의 민감 데이터 — 소독되었다고 믿지 않는다

**이 항목은 11.3절의 전제를 바꾼다.**

Chrome 130+(2024-10)는 HAR을 기본 소독해 내보낸다 `[V]`. 그러나 소독의 실제
범위를 소스(`Log.ts`)에서 확인한 결과 매우 좁다 `[V]`:

```
제거되는 것:  request.headers  의 authorization, cookie
              response.headers 의 set-cookie
              request.cookies / response.cookies 배열
              _eventSourceMessages[].data
```

**건드리지 않는 것: 쿼리스트링 토큰, POST 바디, 응답 바디, `_webSocketMessages`.**

> 즉 **"소독된" Chrome HAR에도 URL의 `access_token=`과 폼 전송의 자격증명이
> 그대로 남아 있는 것이 통상적이다.** 가져온 HAR을 안전하다고 가정하면 안 된다.

`_webSocketMessages`가 소독되지 않는다는 점은 특히 주의를 요한다 —
`_eventSourceMessages`에는 소독 분기가 있는데 WebSocket에는 없다 `[V]`.

**설계 결정: 리댁션을 캡처뿐 아니라 가져오기 경로에도 적용한다**(파이프라인 6단계).
11.3절은 원래 "우리가 생성한 데이터"를 대상으로 했으나, 남이 만든 HAR이 오히려
더 위험하다. 사용자가 그 내용을 모르기 때문이다.

이 결정의 근거가 되는 실제 사고 `[V]`: Okta 지원 시스템 침해(2023-09~10, 고객
134곳)에서 업로드된 HAR의 세션 토큰이 사용되었고, BeyondTrust 보고에 따르면
공격자는 **업로드 30분 이내**에 해당 쿠키를 사용했다. 이 지연 시간이 "가져오기
시점 기본 소독"의 근거다 — 사후 조치로는 늦다.

*(정정: Cloudflare의 동일 사건 블로그는 "지원 티켓에서" 토큰이 탈취되었다고만
쓰고 HAR을 명시하지 않는다. HAR 인과관계는 Okta와 BeyondTrust만 인용한다.)*

**분석 가치를 보존하는 리댁션 기법** — 아래 표의 기본값은 11.3.1의 안전성
재검토를 반영한다. 상관관계를 보존하는 약한 형태는 명시적 옵트인일 때만 허용한다.

| 기법 | 효과 | 출처 |
|---|---|---|
| **헤더 이름은 남기고 값만 가린다** | 헤더 집합이 보존되어 진단 신호 대부분이 살아남는다 | 공통 |
| **JWT payload 전체 삭제** | 기본값. `alg`/`typ`과 만료 시각처럼 비식별 최소 메타데이터만 선택적으로 남긴다 | 11.3.1 |
| **쿠키 값 완전 삭제** | 기본값. 상관관계가 꼭 필요할 때만 세션별 랜덤 키 HMAC을 명시 선택한다 | 11.3.1 |
| MIME 기준 `content.text` 폐기 (JS/CSS/이미지/폰트) | 대량 노이즈 제거 | 공통 |
| **탐지 후 확인** — `key=` 패턴(64자 이하)을 훑어 후보를 제시하고 사용자에게 확인 | 정적 차단 목록의 한계를 넘는다 | Cloudflare `extractInlineKvKeys` `[V]` |
| 불리언이 아닌 **수집 등급** (`full`/`minimal`, 바디 `omit`/`embed`/`attach`) | 6.4절 바디 정책과 결합 | Playwright `recordHar` `[V]` |

생태계 관찰 하나 `[V]`: **Google과 Cloudflare의 HAR 새니타이저가 둘 다 아카이브
(보관) 상태인데, Salesforce와 Zendesk는 여전히 고객에게 그것들을 안내한다.**
이는 리댁션을 외부 도구에 위임하지 말고 **제품에 내장해야 한다는 근거**다.

구현 주의: Cloudflare 구현은 **파싱된 트리가 아니라 원시 JSON 텍스트에 정규식을
적용**한다 `[V]`. 취약한 방식이다. ArchScope는 이미 모델을 파싱하므로
**필드 경로 인지 방식**으로 구현한다 — 구조적으로 우월하고 기존 아키텍처에 맞는다.

#### 6.5.7 내보내기

표준 HAR 1.2로 내보내되 ArchScope 확장은 `_archscope` 접두 필드에 담는다(규격이
`_` 접두 커스텀 필드를 허용하고, 파서는 타 도구 필드를 무시해야 한다고 규정한다).
프로세스 정보, `fidelity`, `captureMode`가 여기 들어간다.

```json
{
  "_archscope": {
    "schemaVersion": "1",
    "process": { "pid": 4821, "startTime": "…", "execPath": "…",
                 "attribution": "confirmed" },
    "captureMode": "proxy_mitm",
    "fidelity": "full",
    "redaction": { "applied": true, "rules": ["auth-headers", "jwt-signature"] }
  }
}
```

`ssl` 귀속은 **규격을 따라 Chrome 관례로 내보낸다**(`entry.time`에서 `ssl` 제외).
ArchScope 내부의 분리된 `TLSMs`는 `timings.ssl`에 넣고 `connect`에도 합산해
규격 문구를 만족시킨다. 왕복 손실이 없도록 `_archscope`에 원본 분리값을 함께
기록한다.

**주의: 우리가 내보낸 HAR도 남에게 주는 순간 위험물이다.** 11.3절의 내보내기
재확인이 여기 적용되며, 6.5.6절의 Okta 사례가 그 이유다.

#### 6.5.8 참조 구현과 라이브러리

| 대상 | 평가 |
|---|---|
| `@types/har-format` (TS) | **필드 수준 타입 참조로는 전 생태계 최고** `[V]` |
| `har-schema` (JSON Schema) | required/optional의 권위 있는 산출물. 2017년 이후 정지이나 규격도 정지 `[V]` |
| `har-validator` (npm) | **폐기됨(deprecated).** 사용하지 않는다 `[V]` |
| `chrome-har` (JS) | **CDP 이벤트 → HAR 엔트리 매핑의 최고 참조** `[V]` |
| Playwright `recordHar` | **수집 등급 API 설계의 참조** `[V]` |
| `github.com/google/martian/v3/har` (Go) | 전체 구조체 + **content-type별 바디 로깅 술어**. Go 구현의 1차 참조 `[V]` |
| `har` crate (Rust) | serde 직렬화만. **Rust에는 HAR 검증기도 새니타이저도 존재하지 않는다** `[V]` |

Go 구현이므로 `martian/v3/har`이 실질적 출발점이다. 다만 **구조체 정의는
`@types/har-format`·`har-schema`·`martian` 셋을 교차 확인**한다 — 어느 하나도
단독으로는 신뢰하기 어렵다.

#### 6.5.9 ingestion 등록

기존 관례를 따른다: `registry.go`의 `DetectorFunc`(L37)로 `.har` JSON의
`log.version`/`log.entries` 시그니처를 탐지하고, 새 소스 종류 `http_capture`를
`boundary.go:11`의 상수군에 추가한다.

탐지는 **확장자가 아니라 내용 기준**으로 한다(mitmproxy 참조 구현). `.json`으로
저장된 HAR, `.har`로 저장된 NetLog가 모두 실재한다.

Phase 1의 명시적 계약으로 다음을 함께 등록한다. **현재 코드에 `http_capture`
계열이 존재하지 않으므로, 이 항목들은 "이미 있는 것"이 아니라 Phase 1에서
새로 만들어야 하는 산출물이다.**

| 산출물 | 위치 |
|---|---|
| `SourceKind` 상수 | `boundary.go`의 상수군 |
| family spec | ingestion core family 등록 (현재 미등록) |
| detector | `registry.go`의 `DetectorFunc` |
| CLI leaf | `archscope-engine` 하위 명령 |
| Wails binding | `CaptureService` 등록 + `bridge/engine.ts` 수기 바인딩 |

### 6.6 `AnalysisResult` 매핑

R7 준수. 기존 봉투를 그대로 쓴다.

| 봉투 필드 | 내용 |
|---|---|
| `Type` | `"http_capture"` |
| `SourceFiles` | 세션 디렉터리 또는 `.har` 경로 |
| `Summary` | 총 트랜잭션/프로세스/호스트 수, 총 전송량, 오류율, 상태 코드 분포, 소요시간 백분위수, **커버리지 지표**(10.1) |
| `Tables` | `transactions`, `processes`, `hosts`, `slowest`, `errors` |
| `Series` | 시간축 요청률/전송량/오류율, 프로세스별 스택 |
| `Charts` | 상태 코드 분포, 호스트별 시간 기여, 타이밍 구간 분해 |
| `Metadata.Findings` | 진단 결과(6.7) |
| `Metadata.Extra` | **세션 참조와 상한 있는 요약만** — `captureSessionRef`(ID, 스키마 버전, 스토어 경로, 카운터). 원본 트리를 넣지 않는다 |

**개정 (2026-07-19): 크기에 따라 계약을 바꾸지 않는다.** 개정 전 설계는 트랜잭션
5,000건을 임계치로 `Extra`의 내용을 바꾸겠다고 했다. 이는 같은 `http_capture`
타입이 4,999건일 때와 5,001건일 때 서로 다른 self-contained 계약을 갖는다는
뜻이고, 임계치 숫자 자체도 임의였다. 게다가 현재 `Metadata.Extra`는 marshal 시
metadata에 그대로 평탄화되므로(`apps/engine-native/internal/models/analysis_result.go:147-181`)
트리 전체가 Wails JSON IPC와 report JSON에 실린다. `Tables.transactions`와도
중복된다.

새 경계는 하나다.

- **`AnalysisResult`는 세션 크기와 무관하게 항상 동일한 상한 있는 봉투다.**
  summary, 상한 있는 series/charts, findings, 세션 참조만 담는다. `Tables`에는
  상위 N개(느린 요청·오류·호스트·프로세스)만 넣고 N을 봉투에 명시한다.
- 상세 트랜잭션·프로세스·연결·바디는 **버전이 붙은 `CaptureSessionStore`**에서
  cursor pagination으로 읽는다. cursor에는 snapshot/version을 포함해 캡처가
  자라는 중에도 페이지가 어긋나지 않게 한다.
- report/export는 봉투를 부풀리지 않고, **스토어를 읽어 상한 있는 집계를 만드는
  전용 projection**을 거친다.

```go
type CaptureSessionRef struct {
    SessionID     string `json:"sessionId"`
    SchemaVersion int    `json:"schemaVersion"`
    StorePath     string `json:"storePath"`
    Transactions  int64  `json:"transactions"`
    Truncated     bool   `json:"truncated"` // 봉투의 Tables가 잘렸는지
}
```

Wails 메서드는 `FetchCaptureTransactions(sessionID, filter, cursor)`로 두고,
cursor는 불투명 문자열이되 내부에 `(snapshotVersion, lastKey)`를 담는다.

**세션 Diff는 generic report diff로 대체할 수 없다.** 현재 generic diff는 summary의
숫자 필드와 finding 수만 비교한다. HTTP 세션 비교는 URL 정규화(경로 템플릿화),
비교 차원(host/process/endpoint), 분모 정의, 시간창 정렬을 명시한 **HTTP 전용
analyzer**가 필요하다. 그 계약은 **12.4.1로 확정되었다**(T-575) — 결과 타입은
`http_capture_diff`이고 구현은 Phase 4 항목 31이다.

### 6.7 진단 finding

`AddFinding(severity, code, message, evidence)` 관례를 따른다.

| 코드 | 심각도 | 조건 |
|---|---|---|
| `CAPTURE_EMPTY` | warn | 트랜잭션 0건 — 프록시 미적용 가능성 안내 |
| `CAPTURE_LOW_ATTRIBUTION` | warn | 미식별 비율 > 20% |
| `CAPTURE_PINNED_HOSTS` | info | 피닝으로 패스스루된 호스트 존재 |
| `CAPTURE_TIMING_INCONSISTENT` | warn | `TotalMs`와 구간 합이 불일치 |
| `CAPTURE_TRUNCATED_BODIES` | info | 상한 초과로 잘린 바디 존재 |
| `CAPTURE_REDACTED` | info | 리댁션이 적용됨 — 결론 해석 시 주의 |
| `CAPTURE_H2_HEADERS_LOST` | warn | pcap 중간 시작으로 HPACK 복원 실패(3.1) |
| `CAPTURE_SLOW_DNS` | warn | DNS > 200ms인 트랜잭션 존재 |
| `CAPTURE_CONNECTION_CHURN` | warn | 동일 호스트 연결 수/요청 수 비율이 높음 — keep-alive 미작동 의심 |

마지막 두 개는 단순 관측이 아니라 **해석**이다. ArchScope의 성격상 이런 종류의
finding이 실제 사용자 가치를 만든다.

## 7. 엔진 구조

### 7.1 패키지 배치

기존 경계 규칙(`ValidateEvidenceFamilySpecs`, `boundary.go:147`)이 파서는
`apps/engine-native/internal/parsers`, 분석기는 `apps/engine-native/internal/analyzers` 아래일 것을 강제하므로 이를
따른다.

```
internal/
├─ capture/                    ← 신규 최상위. 파서도 분석기도 아니다
│  ├─ capturer.go              Capturer 인터페이스, 세션 수명주기
│  ├─ proxy/                   방식 B — MITM 프록시
│  │  ├─ server.go
│  │  ├─ ca.go                 CA 생성/리프 서명/캐시
│  │  └─ passthrough.go        피닝 감지 및 패스스루 목록
│  ├─ pcap/                    방식 A (빌드 태그로 선택적)
│  ├─ procmap/                 4절 — 플랫폼별 PID 매핑
│  │  ├─ procmap_darwin.go
│  │  ├─ procmap_linux.go
│  │  └─ procmap_windows.go
│  ├─ store/                   세션 디렉터리 쓰기, blob, manifest
│  └─ redact/                  11.3 리댁션 규칙
├─ parsers/httpcapture/        세션 디렉터리 / HAR 읽기
└─ analyzers/httpcapture/      집계·진단·AnalysisResult 생성
```

`apps/engine-native/internal/capture`를 파서/분석기 밖에 두는 것이 의도적이다. 캡처는 **증거를
생성하는 활동**이지 증거를 읽는 활동이 아니다. 원칙 A의 코드 상 표현이다.

### 7.2 `Capturer` 인터페이스

```go
type Capturer interface {
    Name() string
    Available() (bool, string)              // 사용 가능 여부와 불가 사유
    RequiredPrivilege() Privilege           // none | admin | root
    // 비블로킹. 백그라운드 고루틴을 띄우고 즉시 세션 ID를 반환한다.
    Start(ctx context.Context, cfg Config, sink Sink) (SessionID, error)
    // 멱등. 이미 정지·미존재 세션이면 오류 없이 현재 상태를 반환한다.
    Stop(ctx context.Context, id SessionID) (SessionState, error)
    Stats(id SessionID) Stats               // 커버리지 지표 (10.1)
}

type Sink interface {
    OnConnection(Connection)
    OnTransaction(Transaction)
    OnProcess(ProcessInstance)
}
```

`Available()`이 **사유 문자열을 반환**하는 것이 중요하다. "pcap 모드 사용 불가"만
띄우면 사용자가 할 수 있는 게 없다. "Npcap이 설치되어 있지 않습니다"라고 말해야
한다.

#### 7.2.1 세션 수명주기

개정 전 인터페이스는 `Start`가 error만 반환하고 `Stop()`에 세션 ID가 없는데
Wails `StartCapture`는 즉시 세션 ID를 반환한다고 적어 서로 맞지 않았다. **모든
연산을 세션 ID 기준으로 통일한다.**

```
created → starting → running → stopping → finalized
                        │                     ▲
                        └──→ failed           │
                        └──→ recoverable ─────┘  (복구 후 finalize)
```

| 항목 | 계약 |
|---|---|
| 동시 세션 | Phase 2에서는 **1개만** 허용. 두 번째 `Start`는 `ErrSessionActive` |
| `Start` | 비블로킹. 백엔드 초기화 실패는 `starting`에서 `failed`로 전이 |
| `Stop` | 멱등. 기본 타임아웃 10초. 초과 시 강제 종료 후 `recoverable` |
| 미완료 트랜잭션 | 정지 시 `StateAborted`로 확정하고 finding을 남긴다(7.4.5) |
| flush | NDJSON은 레코드 단위 append, 기본 1초 또는 4 MiB마다 fsync |
| 크래시 복구 | 다음 실행 시 `finalized`가 아닌 세션을 발견하면 NDJSON을 **마지막 유효 레코드까지** 읽고, 잘린 꼬리는 버린 뒤 manifest를 재생성한다. 복구 사실과 버린 바이트 수를 finding으로 남긴다 |

`recoverable`은 "데이터는 쓸 만하지만 정상 종료되지 않았다"는 상태다. 이를
`failed`와 뭉개면 크래시 후 세션이 통째로 버려진다.

### 7.3 Wails 서비스 노출

기존 `EngineService`/`ProfilerService` 관례(`apps/engine-native/cmd/archscope-app/main.go:57-58`)를
따라 `CaptureService`를 추가 등록한다. 프론트 바인딩은 손으로 쓴
`bridge/engine.ts`의 `Call.ByName` 방식을 그대로 따른다.

| 메서드 | 성격 | 비고 |
|---|---|---|
| `ListCaptureModes()` | 동기 | 각 모드의 `Available()` 결과와 사유 |
| `StartCapture(cfg)` | 비동기 | `{sessionId}` 반환 |
| `StopCapture(sessionId)` | 동기 | 세션 디렉터리 경로 반환 |
| `GetCaptureStats(sessionId)` | 동기 | 라이브 카운터 |
| `AnalyzeCaptureSession(path)` | 동기 | `AnalysisResult` — **기존 분석기와 동형** |
| `ImportHar(path)` | 동기 | 〃 |
| `FetchCaptureTransactions(...)` | 동기 | 페이지네이션 (6.6) |
| `ExportHar(sessionId, opts)` | 동기 | 리댁션 옵션 포함 |
| `InstallCaCertificate()` / `RemoveCaCertificate()` | 동기 | 권한 승격 유발 |

이벤트는 `init()`의 기존 등록 패턴(`main.go:37-46`)에 맞춰 `capture:started`,
`capture:transaction`, `capture:stats`, `capture:stopped`, `capture:error`를
추가한다.

**`capture:transaction`을 트랜잭션마다 보내면 안 된다.** 초당 수백 건이 나오면
프론트가 죽는다. 100ms 배치 + 백프레셔(큐 상한 도달 시 요약만 전송)를 넣는다.
UI가 못 따라가도 **세션 파일에는 전부 기록된다** — 원칙 A의 또 다른 이점이다.

### 7.4 실시간 스트리밍 파이프라인

R10의 설계다. **이 절은 14절 열린 질문 2번에 대한 답이며, 원래 문서의 잠정
결론을 뒤집는다.**

#### 7.4.1 결정 변경 — 라이브 뷰는 필수다

개정 전 14절 열린 질문 2번은 *"캡처 중에는 카운터만, 정지 후 분석"* 안이 더
안전할 수 있다고 적었다. **이 판단을 철회한다.**

근거:

1. **캡처는 대화형 행위다.** 사용자는 "지금 이 버튼을 누르면 무슨 요청이
   나가는가"를 확인하려고 캡처를 켠다. 정지해야 결과를 볼 수 있으면 시도-확인
   주기가 매번 세션 하나가 되어 도구의 실용성이 무너진다.
2. **커버리지 문제를 즉시 드러내야 한다.** 10절의 요지는 "안 잡힌 것"과 "통신이
   없는 것"을 구분해 주는 것이다. 프록시를 안 타는 앱은 **캡처 중에 0건으로
   보여야** 사용자가 그 자리에서 10.2절의 설정 안내를 적용할 수 있다. 정지 후에야
   알면 캡처를 처음부터 다시 해야 한다.
3. **경쟁 도구 전부가 실시간 표시를 한다** `[V, 1.5.2절]`. 없으면 비교 열위다.

성능 부담은 실재하지만 **설계로 해결할 문제이지 기능을 포기할 이유가 아니다.**
그리고 1.5.4절이 확인했듯 **경쟁 도구들이 실패하는 지점이 정확히 여기다** —
유계 메모리 스트리밍이 차별점이 될 수 있는 이유다.

#### 7.4.2 3단 버퍼 — 유계 메모리의 구조

```
[Capturer]  트랜잭션 완료
    │  (고루틴, 논블로킹 채널 송신. 가득 차면 드롭 대신 아래 3계층으로)
    ▼
┌── (1) 기록 계층 ──────────────────────────────────────────┐
│  세션 파일에 append. 유실 없음. 이것이 진실의 원천이다.      │
│  NDJSON append + 바디는 blob으로 분리 (6.4)                │
│  ★ UI가 아무리 밀려도 이 계층은 절대 드롭하지 않는다        │
└──────────────────────────────────────────────────────────┘
    │
┌── (2) 라이브 창 (메모리) ─────────────────────────────────┐
│  최근 N건 (기본 5,000) 링 버퍼 — 바디 제외 메타데이터만     │
│  축출된 항목은 파일에 있으므로 스크롤 시 지연 로딩          │
│  ★ 메모리 상한이 여기서 확정된다                           │
└──────────────────────────────────────────────────────────┘
    │
┌── (3) 집계 계층 (메모리, 상수 크기) ──────────────────────┐
│  시간 버킷 히스토그램 — 8.7절 전체 타임라인의 자료          │
│  트랜잭션 수와 무관하게 크기가 일정하다                     │
└──────────────────────────────────────────────────────────┘
```

##### 세 약속은 동시에 성립하지 않는다 — 정책을 정한다

2026-07-19 검토가 지적한 대로, 디스크 처리량이 유입량보다 낮거나 디스크가 가득
차면 다음 셋을 동시에 만족할 수 없다.

- producer(Capturer)를 블록하지 않는다
- 메모리를 유계로 유지한다
- 모든 트랜잭션을 손실 없이 기록한다

**우선순위를 명시한다: 유계 메모리 > 무손실 기록 > 비차단.** 즉 기록 큐가 밀리면
Capturer를 **블록시키고**, 그래도 해소되지 않으면 캡처를 중단한다.

| 상황 | 동작 |
|---|---|
| 기록 큐 > high-water mark (기본 64 MiB) | producer 블로킹 시작. UI에 명시 표시 |
| 기록 큐 > hard limit, 또는 디스크 예약 공간(기본 512 MiB) 소진 | **명시적 오류를 내고 캡처를 중지한다** |
| 사용자가 degradation을 명시 선택한 경우 | 바디만 `omitted`로 낮춰 계속. 메타데이터는 유지 |

**어떤 경우에도 조용히 downgrade하지 않는다.** 바디 생략은 기본값이 아니라 사용자가
사전에 선택한 정책일 때만 발동하고, 발동 사실은 세션 manifest와 finding에 남는다.

flush/fsync 정책과 크래시 복구는 7.2절의 세션 수명주기에서 정의한다.

##### 손실 회계 — 하나의 "dropped"로 뭉개지 않는다

서로 원인이 다른 사건을 같은 카운터에 넣으면 진단이 불가능하다. 각각 monotonic
카운터로 분리해 `Stats()`와 `capture:stats`에 노출한다.

| 카운터 | 의미 |
|---|---|
| `captured` | Capturer가 완성한 트랜잭션 수 |
| `persisted` | 기록 계층이 디스크에 확정한 수 |
| `bodyOmitted` | 정책에 따라 바디를 버린 수 |
| `eventSkipped` | UI 이벤트 배치를 건너뛴 수 (기록과 무관) |
| `kernelDropped` | pcap/ETW 등 커널 계층이 알려준 유실 |
| `parseFailed` | 관측했으나 HTTP로 해석하지 못한 수 |

`captured != persisted`인 채로 세션이 끝나면 finding을 남긴다.

**설계의 핵심은 (3)이 (2)와 독립이라는 점이다.** 라이브 창에서 축출된 오래된
트랜잭션도 집계에는 이미 반영되어 있다. 따라서 **전체 타임라인은 캡처 시작
시점부터 전 구간을 항상 보여줄 수 있으며, 메모리는 상수다.**

이것이 1.5절의 경쟁 도구들이 못 한 일이다. 그들은 전체를 메모리에 들고
있으려다 실패했고(Charles 256 MB 기본 힙, Fiddler 32 GB 사례), 그래서 전체
타임라인도 못 만들었다. **유계 메모리와 전체 타임라인은 상충하지 않는다 —
집계를 원본과 분리하면 된다.**

#### 7.4.3 시간 버킷 집계

```go
// internal/capture/aggregate.go
type Bucket struct {
    StartMs   int64            // 버킷 시작 (세션 시작 기준 상대)
    Count     int              // 트랜잭션 수
    Errors    int              // 4xx+5xx
    BytesIn   int64
    BytesOut  int64
    // 백분위수 산출용 — 전량 보관하지 않는다
    DurationSketch *tdigest.TDigest
    ByStatus  [6]int           // 1xx..5xx, 기타
    // 상한 없는 map을 두지 않는다 — 아래 "상수 크기의 조건" 참조
    ByProcess BoundedCounter   // top-K + other
}

type Aggregator struct {
    epoch      time.Time        // 세션 시작 — 버킷 경계의 기준점
    resolution time.Duration    // 적응적 — 아래
    buckets    []Bucket
}
```

##### 상수 크기의 조건

"집계 계층은 상수 크기"라는 말은 **조건부로만 참이다.** `ByProcess`를 열린 map으로
두면 프로세스 cardinality만큼 자라고, 버킷마다 t-digest를 두면 버킷 상한
2,000개만으로는 메모리 상한이 정해지지 않는다. 두 가지를 명시한다.

- `ByProcess`는 **top-K + `other` 합계**(기본 K=32)로 제한하거나, 별도의 상한 있는
  dimension store로 분리한다. 축출된 프로세스는 `other`에 합산되며 UI에 그렇게
  표시한다.
- t-digest는 압축 파라미터를 고정해 sketch 하나의 상한 바이트를 정하고,
  `버킷 수 × sketch 상한`을 문서에 실측값으로 적는다.

##### 결정성 — 버킷 경계는 도착 순서에 의존하지 않는다

7.5.2절이 `Add`의 순서 무관성을 요구하는데, 적응형 인접 병합은 그 자체로는
이를 보장하지 않는다. 병합이 언제 일어나는지가 실시간 완료 순서와 장기 요청의
도착 시점에 따라 달라지기 때문이다. 따라서:

- 버킷 경계는 **세션 epoch와 현재 해상도만으로** 결정한다. 즉 버킷 인덱스는
  `floor((startedAt - epoch) / resolution)`이며 도착 순서와 무관하다.
- 해상도 상승(compaction)은 **결정적**이어야 한다. 2:1 병합은 항상 짝수 인덱스
  경계에서만 일어나고, 최종 해상도는 `상한 도달 횟수`가 아니라
  `세션 길이 / 버킷 상한`으로부터 유도한다. 같은 입력 집합이면 병합이 몇 번
  일어났든 같은 결과가 나온다.
- 이를 property test로 강제한다: 같은 트랜잭션 집합을 (a) 실시간 완료 순,
  (b) 시작 시각 순, (c) 무작위 순으로 먹여 세 결과가 동일한지 검증한다.

**해상도는 적응적이어야 한다.** 고정 100ms 버킷이면 8시간 캡처에서 288,000개가
된다. Wireshark I/O Graphs가 1 ms~10 min 범위를 제공하는 것과 같은 문제다 `[V]`.

> **규칙: 버킷 수가 상한(예: 2,000)에 도달하면 인접 버킷을 2:1로 병합하고
> 해상도를 2배로 늘린다.** 메모리는 상한에 묶이고, 해상도는 캡처 길이에 따라
> 자연스럽게 낮아진다.

병합이 가능하려면 집계량이 **결합법칙을 만족**해야 한다. `Count`·`Bytes`·`Errors`는
단순 합이라 문제없다. 백분위수는 그렇지 않으므로 **t-digest 같은 병합 가능한
스케치**를 쓴다. 평균만 저장했다가 나중에 백분위수를 요구받는 실수를 피한다.

#### 7.4.4 이벤트 프로토콜

7.3절의 이벤트를 구체화한다. **한 종류가 아니라 성격이 다른 세 종류다.**

| 이벤트 | 주기 | 내용 | 백프레셔 |
|---|---|---|---|
| `capture:transactions` | 100ms 배치 | 신규 트랜잭션 메타데이터 배열 | **생략 가능** — 카운터로 대체 |
| `capture:aggregate` | 1s | 최근 버킷 델타 | 생략하지 않음 — 작고 상수 크기 |
| `capture:stats` | 1s | 총계·프로세스 수·커버리지 지표 | 생략하지 않음 |

**`capture:transactions`만 생략 대상이라는 점이 중요하다.** 목록은 밀려도
사용자가 스크롤하면 파일에서 읽어 복구된다. 반면 집계와 통계가 끊기면 화면이
멈춘 것처럼 보인다 — 크기가 작으니 생략할 이유도 없다.

##### "드롭 불가"는 전달 보장이 아니다 — 복구로 설계한다

2026-07-19 검토의 지적을 반영한다. **Wails events는 emit/listen 메커니즘이지
전달 보장·재연결·ack 프로토콜이 아니다.** 따라서 "이 이벤트는 드롭 불가"라고
선언하는 것으로는 아무것도 보장되지 않는다. 보내는 쪽이 생략하지 않는다는 뜻일
뿐이며, 이벤트 유실·프론트 재마운트·페이지 재진입은 여전히 발생한다.

설계를 **push + 복구 가능한 pull**로 둔다.

- 모든 캡처 이벤트에 `sessionId`, monotonic `sequence`, `snapshotVersion`을 넣는다.
- 프론트는 `sequence`의 공백을 감지하면 `GetCaptureSnapshot(sessionId, sinceVersion)`
  으로 상태를 다시 맞춘다. 마운트 직후에도 같은 경로로 초기 상태를 받는다.
- 즉 이벤트는 **지연을 줄이는 최적화**이고, 정확성의 원천은 스냅샷 조회다.

이 구조가 없으면 "집계가 끊기지 않는다"는 약속을 지킬 수단이 없다.

공식 근거: [Wails Events API](https://wails.io/docs/reference/runtime/events/)

백프레셔 발동 시 UI에 **명시적으로 표시**한다:

```
⚡ 유입 속도가 높아 목록 갱신을 건너뛰는 중 (초당 1,240건)
   — 모든 트랜잭션은 세션에 기록되고 있습니다
```

조용히 누락시키면 사용자는 캡처가 놓쳤다고 오해한다. 10절의 "커버리지 정직성"
원칙이 UI 성능 영역에도 그대로 적용된다.

#### 7.4.5 부분 트랜잭션

실시간에서만 생기는 문제다. 응답이 아직 안 온 요청을 어떻게 보여줄 것인가.

HTTPAnalyzer의 `Stage` 컬럼이 이 문제를 이미 풀었다(1.4.3절) `[V]`. 같은 모델을
쓴다.

```go
type TxState string

const (
    StateRequestSent TxState = "request_sent" // 요청 전송, 응답 대기
    StateReceiving   TxState = "receiving"    // 응답 헤더 수신, 바디 진행 중
    StateComplete    TxState = "complete"
    StateFailed      TxState = "failed"
    StateAborted     TxState = "aborted"      // 클라이언트가 끊음
)
```

- 목록에 **진행 중 행을 즉시 표시**한다. 스피너와 경과 시간을 함께 보여준다.
  느린 요청은 완료되기 전에 보여야 가치가 있다 — 그것이 조사 대상이므로.
- 완료 시 같은 행을 갱신한다. 새 행을 추가하지 않는다.
- 집계 계층은 **완료 시점에만** 반영한다. 진행 중 항목이 백분위수를 오염시키면
  안 된다. 단 `inFlight` 카운터를 별도로 노출한다 — **동시 요청 수는 그 자체로
  진단 가치가 있다**(커넥션 풀 고갈 등).
- 캡처 정지 시 미완료 트랜잭션은 `StateAborted`로 확정하고 finding을 남긴다.

### 7.5 두 모드의 통합 — R12

기획 입력이 요구한 "사후 분석과 실시간 분석 모두 지원"의 구조적 답이다.

#### 7.5.1 통합의 원리

**두 모드를 별개 기능으로 만들지 않는다.** 원칙 A가 이미 답을 갖고 있다 —
캡처기는 파일을 만들고, 분석기는 파일을 읽는다. 실시간 모드는 그 파일이
*자라는 중*일 뿐이다.

```
                        ┌──────────────────────────┐
[실시간 캡처] ──write──▶ │  세션 (NDJSON + blob)    │
                        │                          │ ──read──▶ [분석기]
[HAR 가져오기] ─convert─▶ │  동일 데이터 모델 (6.2)  │            (동일)
                        └──────────────────────────┘
                                    │
                              [동일 UI (8절)]
```

| | 실시간 모드 | 사후 모드 |
|---|---|---|
| 입력 | Capturer → Sink | `.har` 또는 세션 디렉터리 |
| 데이터 모델 | `Transaction` (6.2) | **동일** |
| 집계 | 증분(7.4.3) | **동일 코드를 일괄 실행** |
| 분석기 | `analyzers/httpcapture` | **동일** |
| UI | 8절 화면 | **동일 화면** |
| 차이 | 스트리밍 갱신, 부분 트랜잭션, 정지 버튼 | 전체 즉시 로드, 상태 100% 완료 |

**차이가 UI 상단 도구 모음과 갱신 여부에만 있다.** 목록·상세·타임라인·필터·
집계는 코드가 하나다.

#### 7.5.2 증분 집계와 일괄 집계의 동일성 보장

통합의 유일한 실질적 위험이다. **같은 데이터에 대해 두 경로가 다른 숫자를 내면
안 된다.**

`Aggregator`를 두 경로가 공유하되, 사후 모드는 단순히 시간순으로 전부 먹인다.

```go
// 실시간: 도착 순
agg.Add(tx)

// 사후: 동일 메서드를 시간순으로
for _, tx := range sortByStart(all) { agg.Add(tx) }
```

**요건: `Add`는 순서 무관(order-independent)해야 한다.** 이 요건은 자동으로
성립하지 않는다 — 적응형 버킷 병합이 도착 순서에 의존하면 깨진다. 7.4.3절의
"결정성" 규칙(epoch 기준 버킷 인덱스 + 결정적 compaction)이 이를 받친다. 실시간에서는 트랜잭션이
완료 순으로 도착하므로 시작 시각 순서와 다르다. 버킷 귀속을 **완료 시각이 아니라
시작 시각 기준**으로 하면 두 경로가 같은 결과를 낸다.

늦게 도착한 트랜잭션이 이미 UI로 전송된 과거 버킷에 속하는 경우가 생긴다.
**해당 버킷을 재전송한다** — 버킷은 작으므로 비용이 없다. "지나간 버킷은 확정"
으로 처리하면 장기 요청이 통계에서 사라진다.

이 동일성은 **테스트로 강제한다**: 같은 세션을 (a) 실시간 재생, (b) 파일 일괄
로드 두 경로로 처리해 집계 결과가 일치하는지 검증한다. 회귀 위험이 높은
지점이라 자동 검증이 필요하다.

#### 7.5.3 실시간 → 사후 전환

캡처 정지는 **모드 전환이지 데이터 전환이 아니다.**

1. Capturer 정지, 미완료 트랜잭션 확정(7.4.5)
2. 세션 파일 flush, `manifest.json` 작성(6.4)
3. **화면을 다시 그리지 않는다.** 같은 뷰가 그대로 있고 상단 표시만 "녹화 중"
   → "세션 (2026-07-18 14:22, 3분 12초)"로 바뀐다
4. 라이브 창에서 축출되었던 항목이 이제 파일에서 지연 로딩 가능해진다

사용자 관점에서 **정지는 화면을 잃는 사건이 아니다.** 이것이 통합 설계의
체감 가치다.

### 7.6 `CaptureSessionStore` — 유계 세션 저장소

6.6이 "상세는 스토어에서 cursor로 읽는다"고 선언했고, 7.4.2가 3단 버퍼의 메모리
상한을 정했다. **이 절은 그 스토어의 실제 계약이다** — 스키마, 메모리 상한,
축출 정책, 디스크 스필. Phase 0 게이트 5의 산출물이다.

원칙 하나로 시작한다.

> **디스크가 진실의 원천이고, 메모리는 캐시다.** 메모리에만 존재하는 트랜잭션은
> 없다. 따라서 축출은 데이터 손실이 아니라 캐시 미스이며, 축출 정책을 공격적으로
> 잡아도 정확성이 무너지지 않는다.

#### 7.6.1 온디스크 레이아웃

6.4의 세션 디렉터리를 스토어 관점에서 확장한다.

```
capture-20260718-142211/
├─ manifest.json          스키마 버전, 리댁션 정책, 상태, 손상 탐지 해시
├─ transactions.ndjson    트랜잭션 레코드 append-only. 진실의 원천
├─ connections.ndjson     연결 레코드 append-only
├─ processes.ndjson       프로세스 인스턴스 append-only
├─ index/
│  ├─ offsets.bin         레코드 순번 → 파일 오프셋 (고정 폭 int64)
│  └─ buckets.ndjson      집계 스냅샷 (7.4.3) — 재시작 시 복원용
└─ bodies/                blob (랜덤 opaque ID — 11.3.1)
```

설계 판단 세 가지.

- **NDJSON append-only인 이유는 크래시 복구다**(7.2.1). 마지막 레코드가 잘려도
  그 앞까지는 유효하다. 임베디드 KV/SQLite는 쓰기 중 크래시 시 복구가 엔진에
  달리고, 의존성·바이너리 크기·파일 잠금 문제가 따라온다. **캡처의 접근 패턴은
  append + 순차 스캔 + 오프셋 랜덤 읽기뿐이라 DB가 필요 없다.**
- **`index/offsets.bin`이 cursor pagination을 O(1)로 만든다.** 레코드 순번 `n`의
  오프셋이 `8n` 위치에 있다. 인덱스가 손상되면 NDJSON 전체 스캔으로 재생성
  가능하다 — **인덱스는 파생물이지 원천이 아니다.**
- **`buckets.ndjson`은 최적화이지 원천이 아니다.** 없으면 NDJSON을 다시 읽어
  재계산한다. 7.4.3의 결정성 보장 덕분에 재계산 결과가 원본과 동일하다.

#### 7.6.2 레코드 스키마와 버전

```go
type StoredTransaction struct {
    Seq int64 `json:"seq"` // 스토어 내 단조 증가. cursor의 기준
    Transaction
}
```

`Seq`는 **완료 순서**다(6.3.3의 종단 전이 시점). 시작 순서가 아니다. 진행 중
트랜잭션은 스토어에 없으며, 라이브 창에만 존재한다 — 이것이 7.4.5의 "집계는
완료 시점에만 반영"과 같은 경계다.

`manifest.json`에 버전이 셋 들어간다. **하나로 뭉치면 부분 호환을 표현할 수 없다.**

| 필드 | 의미 | 증가 조건 |
|---|---|---|
| `captureSchemaVersion` | 6.2~6.4 데이터 모델 | 필드 의미 변경·삭제 |
| `storeLayoutVersion` | 7.6.1 디렉터리·파일 구조 | 파일 추가·이름 변경 |
| `redactionPolicyVersion` | 11.3 규칙 집합 | 기본 규칙 변경 |

읽기 정책: **알 수 없는 상위 `captureSchemaVersion`은 거부**하고 명확한 오류를
낸다. 하위 버전은 마이그레이션 없이 읽되(필드 누락은 `unknown` 상태로 처리),
`CAPTURE_SCHEMA_OLD` finding을 남긴다. 알 수 없는 **필드**는 무시한다 — 이쪽은
전방 호환이 유용하다.

#### 7.6.3 메모리 상한

7.4.2의 3계층에 실제 숫자를 배정한다. **각 계층이 독립적으로 유계여야 전체가
유계다.**

| 계층 | 상한 | 근거 |
|---|---|---|
| 라이브 창 (2) | 5,000건 × 메타데이터 | 바디 제외. 트랜잭션당 상한을 두고(아래) 실측 후 조정 |
| 집계 (3) | 2,000 버킷 × (고정 필드 + t-digest 상한 + top-K 32) | 7.4.3 |
| 기록 큐 (1) | high-water 64 MiB / hard limit | 7.4.2의 백프레셔 |
| blob 캐시 | 32 MiB LRU | 상세 탭에서 최근 본 바디만 |
| 오프셋 인덱스 | 8 bytes × 레코드 수 — **전량 상주하지 않는다** | 아래 |

두 항목에 명시적 상한이 더 필요하다.

- **트랜잭션 메타데이터 자체가 무제한이다.** URL·헤더 이름·에러 문자열이 길면
  "5,000건"이 메모리 상한이 되지 못한다. 라이브 창에 넣는 레코드는 **표시에
  필요한 부분만 잘라낸 축약형**으로 둔다 — URL은 앞 2 KiB, 헤더는 개수만,
  본문 미리보기 없음. 전체는 상세 탭 열 때 스토어에서 읽는다.
- **오프셋 인덱스를 전량 메모리에 올리지 않는다.** 1억 건이면 800 MB다. 파일
  그대로 두고 필요한 위치만 `ReadAt`한다. OS 페이지 캐시가 실질적인 캐싱을 한다.

#### 7.6.4 축출 정책

| 계층 | 정책 | 축출된 것을 다시 얻는 방법 |
|---|---|---|
| 라이브 창 | **FIFO 링 버퍼.** 오래된 것부터 | 스토어에서 `Seq` 범위 읽기 |
| blob 캐시 | LRU | `BodyRef`로 blob 파일 재읽기 |
| 집계 버킷 | 축출 없음 — **2:1 병합으로 해상도만 낮춘다** (7.4.3) | 원본 NDJSON 재계산 |
| 프로세스/연결 | 축출 없음 | cardinality가 트랜잭션 수에 비례하지 않는다 |

**집계만 축출이 아니라 병합인 것이 핵심 설계다.** 오래된 트랜잭션이 라이브 창에서
사라져도 집계에는 이미 반영되어 있으므로 **전체 타임라인은 항상 세션 전 구간을
보여준다**(7.4.2). 축출 정책이 UI 기능을 깎지 않는다.

스크롤 시 지연 로딩의 접근 패턴을 명시한다. 사용자가 목록을 위로 스크롤하면
축출된 구간이 필요해진다.

- 요청 단위는 **`Seq` 범위 청크**(기본 500건)다. 개별 건을 하나씩 읽지 않는다.
- 읽은 청크는 라이브 창과 **별도의 스크롤 캐시**(상한 3청크)에 둔다. 링 버퍼에
  되돌려 넣으면 라이브 창의 FIFO 의미가 깨진다.
- 필터가 걸린 상태의 스크롤은 **스토어 측 필터링**으로 처리한다. 전량을 읽어
  프론트에서 거르면 유계 메모리 설계가 무의미해진다.

#### 7.6.5 디스크 스필과 상한

메모리 상한은 디스크로 밀어내는 것으로 지켜진다. 그렇다면 **디스크 자체의 상한**이
다음 질문이다. 7.4.2가 "예약 공간 소진 시 캡처 중지"를 정했고, 여기서 세션 크기
정책을 정한다.

| 설정 | 기본값 | 초과 시 |
|---|---|---|
| 세션 최대 크기 | 2 GiB | 아래 정책에 따름 |
| 디스크 예약 공간 | 512 MiB | **캡처 중지** (7.4.2) |
| 세션 최대 시간 | 없음 | — |
| 바디 총량 상한 | 세션 크기의 70% | 초과 시 신규 바디만 `omitted` |

세션 크기 초과 시의 정책은 **사용자 선택이며 기본값은 중지**다.

| 정책 | 동작 | 기본 |
|---|---|---|
| `stop` | 캡처를 중지하고 세션을 정상 finalize | **기본값** |
| `body_only` | 바디를 `omitted`로 낮추고 메타데이터는 계속 기록 | 옵트인 |
| `rotate` | 메타데이터를 오래된 것부터 삭제하며 계속 (링 세션) | 옵트인 |

**`rotate`가 기본이 아닌 이유**는 7.4.2의 원칙과 같다. 사용자 데이터를 말없이
지우지 않는다. `rotate`를 선택하면 그 사실과 삭제된 건수를 manifest와 finding에
남기고, 전체 타임라인에는 **"이 구간의 상세는 삭제됨"** 표시를 넣는다 — 집계는
남아 있으므로 시계열 자체는 온전하다.

바디 상한을 세션 크기의 비율로 두는 이유는, 바디 하나가 세션 전체를 채워
메타데이터 기록을 굶기는 상황을 막기 위해서다. **메타데이터가 바디보다
중요하다** — 바디 없는 트랜잭션 목록은 여전히 분석 가능하지만 그 반대는 아니다.

#### 7.6.6 스토어 인터페이스

```go
type CaptureSessionStore interface {
    // 쓰기 — Capturer 측
    Append(tx StoredTransaction) error
    Flush() error                    // 7.2.1의 fsync 정책
    Finalize(state SessionState) error

    // 읽기 — Wails/분석기 측
    Meta() (SessionMeta, error)
    // cursor는 불투명. 내부에 (snapshotVersion, lastSeq) — 6.6
    Fetch(filter Filter, cursor string, limit int) (Page, error)
    Body(ref string) (io.ReadCloser, error)
    Buckets(from, to int64) ([]Bucket, error)

    // 복구 — 7.2.1
    Recover() (RecoveryReport, error)
}

type Page struct {
    Items      []StoredTransaction `json:"items"`
    NextCursor string              `json:"nextCursor,omitempty"`
    // 캡처가 자라는 중이면 total은 이 스냅샷 기준값이다
    SnapshotVersion int64          `json:"snapshotVersion"`
    Total           int64          `json:"total"`
}
```

계약 규칙.

- **`Fetch`는 캡처 중에도 안전하다.** cursor에 `snapshotVersion`이 들어 있어,
  페이지를 넘기는 도중 새 트랜잭션이 append되어도 이미 본 항목이 밀려나거나
  중복되지 않는다. 새 항목은 다음 스냅샷에서 보인다.
- **`limit`에 상한을 강제한다**(기본 500, 최대 2,000). 프론트가 큰 값을 보내
  봉투 크기 제한(6.6)을 우회하는 경로를 막는다.
- **`Recover()`는 읽기 전 필수다.** `finalized`가 아닌 세션을 열 때 잘린 꼬리를
  버리고 인덱스를 재생성한다(7.2.1). 복구 보고서는 finding으로 승격된다.
- **`Append`는 단일 writer 전제다.** 동시 세션이 1개뿐이므로(7.2.1) 파일 잠금
  경합을 설계하지 않는다. 다만 **다른 프로세스가 같은 세션을 읽는 것은 허용**되며,
  append-only이므로 읽기 측이 부분 레코드를 볼 수 있다 — 읽기 측은 마지막 개행
  이후를 버린다.

## 8. UI 설계

### 8.1 메뉴 위치

`.cpuprofile` 설계 노트(7.3절)가 "브라우저 성능 분석" 그룹을 **제안**했다. 같은
논리로 **"네트워크 분석"** 그룹을 신설한다.

**전제를 정확히 적는다 (2026-07-19).** 현재 `Sidebar.tsx`/`App.tsx`에는 "브라우저
성능 분석"도 "네트워크 분석"도 **구현되어 있지 않다.** 다른 설계 노트에 예정되어
있다는 사실을 "이미 있다"처럼 쓰면 Phase 의존성이 흐려진다. 따라서 이 절의 작업은
`.cpuprofile` 구현을 전제하지 않는 **현재 코드 기준의 독립적인 navigation
migration**으로 취급한다. 두 그룹이 같은 시기에 들어오면 통합하고, 아니면 각자
독립적으로 추가 가능해야 한다.

`Sidebar.tsx`에는 라우트 테이블이 없고 손으로 쓴 배열이 전부라는 제약이 그대로
적용된다 — `NavKey` union(L41-59), `NAV_ICONS`(L63-82), 그룹별 localStorage 키
(L37-39).

```
사이드바
├─ 분석 도구              (기존 9개, 변경 없음)
├─ 브라우저 성능 분석      (.cpuprofile 노트에서 신설)
├─ 네트워크 분석          ← 신규
│   ├─ HTTP 캡처          (http_capture)
│   └─ HAR 가져오기       (http_har)      ※ 캡처 페이지에 통합할 수도 있음
└─ 워크스페이스           (기존 8개, 변경 없음)
```

"브라우저 성능 분석" **다음, 워크스페이스 앞**에 둔다. 앞의 세 그룹이 관측
계층을 위에서 아래로(서버 → 브라우저 → 네트워크) 훑고, 워크스페이스는 성격이
달라 뒤에 남는다.

식별자 규칙:

| 대상 | 값 |
|---|---|
| `NavKey` 추가 | `"http_capture"`, (선택) `"http_har"` |
| 그룹 헤더 i18n | `navNetworkTools` — ko "네트워크 분석" / en "Network analysis" |
| 항목 i18n | `navHttpCapture` — ko "HTTP 캡처" / en "HTTP capture" |
| 페이지 | `pages/HttpCapturePage.tsx` |
| localStorage | `archscope.profiler.sidebar.network.expanded` |
| 아이콘 | `Network` 또는 `Radio` (`lucide-react`) |
| 도움말 키 | `pageHttpCapture`, `sidebarNetwork` |

`messages.ts`는 `MessageKey = keyof typeof messages.en`이라 ko 누락 시 타입
에러가 난다. en/ko를 함께 추가한다.

**초기에는 항목 하나(`http_capture`)만 만든다.** HAR 가져오기는 같은 페이지의
드래그 앤 드롭으로 흡수하는 편이 낫다 — 사용자에게는 "파일을 여는 것"과 "캡처를
여는 것"이 같은 행위이기 때문이다.

### 8.2 화면 구성

```
┌────────────────────────────────────────────────────────────────────────┐
│ [모드 ▾ 프록시] [● 캡처 시작] [세션 열기] [HAR 내보내기]   ● 녹화 중 02:14 │
│ 412 트랜잭션 · 18 프로세스 · 34 호스트 · 미식별 3% · 진행 중 4            │
├──────────────────────┬─────────────────────────────────────────────────┤
│ 프로세스 트리         │ ┌─ 전체 타임라인 (상시 표시·브러시 선택) ───┐   │
│                      │ │ ▁▃█▅▂▁▃▇█▄▂▁▁▂▅█▆▃▁  요청률              │   │
│ ▾ Google Chrome  412 │ │ ▁▁▂▁▁▁▁▁▁█▇▃▁▁▁▁▁▁▁  오류율 (적색)       │   │
│   ├ renderer  4821   │ │        └──선택 구간──┘  ← 드래그하면 아래  │   │
│   └ gpu       4830   │ │                          전체가 필터링됨   │   │
│ ▾ java         57    │ └───────────────────────────────────────────┘   │
│   └ 9102 batch.jar   │ ┌─ 트랜잭션 목록 ───────────────────────────┐   │
│ ▸ curl          9    │ │ # 상태 호스트   경로  크기 시간 워터폴     │   │
│ ▸ (미식별)      12 ⚠ │ │ 1 200 api.x.com /v1/… 4KB 128ms  ▁▃▅     │   │
│                      │ │ 2 500 api.x.com /v1/… 1KB 2.4s ⚠ ▁▁███▂  │   │
│ [필터]               │ │ 3 ⋯   api.x.com /v1/… —   1.2s   ▁▃▃▃⋯   │   │
│                      │ └───────────────────────────────────────────┘   │
│                      │ ┌─ 상세 ─────────────────────────────────────┐  │
│                      │ │ [요약][요청][응답][타이밍][연결/TLS][원본]  │  │
│                      │ └────────────────────────────────────────────┘  │
└──────────────────────┴─────────────────────────────────────────────────┘
```

개정 전 배치와의 차이는 **타임라인의 위상**이다. 원래는 목록 위의 장식적
스파크라인이었으나, 이제 **상시 표시되는 브러시 가능한 1급 뷰**다(8.7절).
1.5.4절의 조사 결론 — 경쟁 도구 전부가 비워둔 축 — 이 화면에서 이 위치를
차지한다.

좌측 트리가 R2의 구현이다. 노드 선택이 우측 전체(타임라인·목록·통계)를 필터링
한다. 트리 노드에는 요청 수와 함께 **총 소요시간 기여도**를 막대로 표시한다 —
"요청이 많은 프로세스"보다 "시간을 많이 쓴 프로세스"가 대개 더 유용하다.

`(미식별)` 노드는 0건이어도 표시하고, 건수가 있으면 경고 아이콘과 함께 4.3의
원인 설명 링크를 붙인다.

목록 3행의 `⋯`는 진행 중 트랜잭션이다(7.4.5절). 실시간 모드에서만 나타난다.

#### 8.2.1 조작 계층 — 툴바와 상세 패널

위 배치도가 **데이터 흐름**(타임라인 → 목록 → 상세)을 보여준다면, 아래는 같은
화면을 **조작 계층** 관점에서 다시 그린 것이다. 상단 툴바의 구성과 하단 상세
패널의 탭 배열이 초점이다. HTTPAnalyzer의 화면 문법을 참고했다 `[V, 1.4.3절]`.

```
┌──────────────────────────────────────────────────────────┐
│ 툴바: [▶캡처] [■중지] [🗑클리어] [필터바...] [검색] [HAR↓] │
├────────────┬─────────────────────────────────────────────┤
│ 프로세스   │  # │ Status │ Method │ URL       │ Size │ Time │
│ 트리       │ ───┼────────┼────────┼───────────┼──────┼──────│
│            │  1 │  200   │  GET   │ /api/...  │ 2.1K │ 45ms │
│ ▼ Chrome   │  2 │  301   │  GET   │ /old/...  │  0   │ 12ms │
│   ├ api.x  │  3 │  500   │  POST  │ /submit   │ 892  │ 2.1s │
│ ▼ Node     │                                              │
│   ├ db.y   │                                              │
├────────────┴─────────────────────────────────────────────┤
│ [Headers] [Body] [Cookies] [Timing] [Query] [Preview]    │
│ ─── Request Headers ───                                   │
│ GET /api/users HTTP/1.1                                   │
│ Host: api.example.com                                     │
│ Authorization: Bearer ****                                │
└──────────────────────────────────────────────────────────┘
```

두 배치도는 **같은 화면의 서로 다른 단면**이다. 상세 패널이 하단 전폭이냐 우측
하단이냐는 구현 시 결정할 여지로 남긴다 — 전폭은 헤더·바디 가독성이 좋고, 우측
하단은 트리 선택과 상세를 동시에 본다.

툴바 항목:

| 항목 | 동작 | 비고 |
|---|---|---|
| `▶ 캡처` | 캡처 시작 | 모드(프록시/eBPF 등) 선택은 인접 드롭다운 |
| `■ 중지` | 캡처 중지 | 중지 시 사후 분석 모드로 전환(8.6.4절) |
| `🗑 클리어` | 현재 세션 버퍼 비우기 | **아래 확인 규칙 참조** |
| 필터바 | 표시 필터 입력 (8.5절) | 캡처 필터는 별도 색상 |
| 검색 | URL·헤더·바디 전체 검색 (8.5절) | 바디 검색은 엔진 위임(8.8절) |
| `HAR ↓` | HAR 내보내기 | 가져오기는 드래그 앤 드롭(8.1절) |

> **클리어는 파괴적 동작이다.** 저장되지 않은 트랜잭션이 있으면 건수를 명시한
> 확인 대화상자를 띄운다 — "412건이 삭제됩니다. 내보내지 않은 세션입니다."
> 이미 파일로 저장된 세션이면 확인 없이 진행한다. 실시간 캡처 중 클리어는
> 캡처를 중단하지 않고 버퍼만 비우며, 이 경우 집계(7.4.3절)도 함께 초기화되어
> 타임라인이 리셋된다는 점을 대화상자에 밝힌다.

### 8.3 트랜잭션 목록

HTTPAnalyzer의 36컬럼 그리드(1.4.3절)와 Fiddler/DevTools의 컬럼 모델을 참고하되,
**기본 컬럼은 좁게 두고 사용자가 넓힌다.**

| 기본 컬럼 | 비고 |
|---|---|
| `#` | 세션 내 순번 |
| 상태 | 코드 + 색상. 진행 중은 `⋯` |
| 메서드 | |
| 호스트 / 경로 | 경로는 말줄임, 호버 시 전체 |
| 크기 | 압축 상태 기준, 호버 시 해제 후 병기 |
| 시간 | 총 소요 |
| **워터폴** | **그리드 내 막대.** 아래 상술 |

**상태 코드 색상 규칙:**

| 계열 | 색상 | 의미 | 목록 표현 |
|---|---|---|---|
| 2xx | 초록 | 성공 | 코드만. 강조 없음 |
| 3xx | 파랑 | 리다이렉트 | 코드 + 대상 호스트가 다르면 `↗` |
| 4xx | 노랑 | 클라이언트 오류 | 코드 + 행 배경 옅은 노랑 |
| 5xx | 빨강 | 서버 오류 | 코드 + 행 배경 옅은 빨강 + `!` |
| `⋯` | 회색 | 진행 중 (7.4.5절) | 흐린 행 |
| `✕` | 회색+빨강 | 연결 실패·중단 — 응답 없음 | 오류 사유 툴팁 |

`✕`를 별도로 두는 이유는 **"5xx"와 "응답이 아예 없었다"가 다른 진단**이기
때문이다. 전자는 서버가 답한 것이고 후자는 연결·DNS·TLS 단계의 실패다(6.3절의
어느 구간에서 끊겼는지가 툴팁에 들어간다).

색상은 **단독 신호가 되어서는 안 된다.** 4xx/5xx는 배경색과 함께 `!` 기호를,
3xx는 `↗`를 병기한다. 색각 이상 사용자에게 초록/빨강 구분은 신뢰할 수 없고,
이 도구의 핵심 판단이 대부분 "오류인가"이기 때문이다. 8.7.2절이 오류율 지표를
적색 고정으로 둔 것과 같은 이유로, 적색은 **오류 외의 용도로 쓰지 않는다.**

선택 가능 컬럼: 프로세스, PID, 연결 ID, 프로토콜(h1/h2/h3), TLS 버전, ALPN,
콘텐츠 타입, 원격 IP, `fidelity`, `captureMode`, 상태(7.4.5), 시작 시각, TTFB,
DNS, 연결, 캐시 여부, 리다이렉트 대상.

**그리드 내 워터폴 막대**가 HTTPAnalyzer에서 가져오는 핵심 아이디어다 `[V]`.
각 행에 그 트랜잭션의 시간 구간을 **선택된 시간 창 기준 상대 위치**로 그린다.
목록을 시작 시각순으로 정렬하면 그대로 워터폴 차트가 된다 — 별도 뷰가 필요
없다.

시각 어휘는 Fiddler Classic Timeline에서 가져온다 `[V, 1.5.5절]`. 그 뷰는
스코프가 좁아 쓸모가 제한적이었지만 **인코딩 자체는 우수하다.**

| 인코딩 | 의미 |
|---|---|
| 막대 색상 | 콘텐츠 타입 (이미지/JS/CSS/문서/기타) |
| 막대 분절 | 6구간 타이밍(6.3절)을 명암으로 |
| 세로선 | TTFB 지점 |
| 좌측 점 | 연결 재사용(녹색) / 신규 연결(적색) |
| 빗금 | **프록시가 버퍼링한 응답** — 이 막대의 타이밍은 실제와 다르다 |
| 적색 `!` | 4xx/5xx |
| 흐린 막대 | 진행 중 (미완료) |

빗금 규칙은 Fiddler가 정직하게 처리한 지점이라 그대로 가져온다. **3.2절이
지적한 프록시의 침습성이 데이터가 아니라 화면에서 드러나야 한다.**

**커스텀 컬럼**: 임의 응답 헤더로 컬럼을 만드는 DevTools 기능을 채택한다
`[V, 1.5.5절]`. 구현 비용이 낮고, 사내 고유 헤더(`X-Request-Id`, `X-Upstream`,
`X-Cache`)를 축으로 삼는 실무 요구가 잦다.

### 8.4 트랜잭션 상세 탭

R3~R6를 만족하는 지점이다.

| 탭 | 내용 |
|---|---|
| **요약** | 메서드·URL·상태·시간·크기, 프로세스, 커넥션 재사용 여부, `fidelity` 배지 |
| **요청** | 헤더 원본 순서 그대로(중복 포함), 바디 뷰어 |
| **응답** | 헤더, 바디 뷰어, 압축 해제 전/후 크기 병기 |
| **쿠키** | 요청/응답 쿠키 표. 아래 상술 |
| **쿼리** | URL 파라미터 분해 표 |
| **타이밍** | 6구간 수평 막대 + 수치. 재사용 연결은 "연결 재사용"으로 명시 |
| **연결/TLS** | 5-tuple, ALPN, TLS 버전/암호군, **서버 인증서 체인**(5.2) |
| **원본** | 재구성된 원본 바이트. 복사/저장 가능 |

**쿠키와 쿼리를 요청 탭 안이 아니라 독립 탭으로 승격한다.** 두 가지 모두 요청
탭에 접어 넣을 수 있지만, 실무에서 조회 빈도가 높고 표 형태가 필요해 헤더 원본
나열과 같은 화면에 두면 양쪽 다 읽기 나빠진다. HTTPAnalyzer와 DevTools가 모두
독립 탭으로 둔 배치이기도 하다 `[V]`.

**쿠키 탭** 컬럼:

| 컬럼 | 비고 |
|---|---|
| 이름 | |
| 값 | **기본 마스킹.** 세션 토큰이 대부분이다 — 11절 리댁션 규칙 적용 |
| 도메인 | |
| 경로 | |
| 만료 | 절대 시각 + 상대 표기("3일 후"). 세션 쿠키는 `Session` |
| 방향 | 요청(`Cookie`) / 응답(`Set-Cookie`) |
| 플래그 | `HttpOnly`·`Secure`·`SameSite` — **누락 시 경고 아이콘** |

플래그 누락 경고가 이 탭을 단순 표 이상으로 만든다. `Secure` 없는 인증 쿠키나
`SameSite` 미지정은 캡처를 보다가 발견되는 전형적 결함이고, 6.7절 진단 규칙과
연결할 여지가 있다.

**쿼리 탭**은 URL 파라미터를 키-값 표로 분해하고 URL 인코딩을 해제해 보여준다.
원본 인코딩 문자열도 병기한다 — 이중 인코딩이 디버깅 대상인 경우가 있다. 배열
표기(`a[]=1&a[]=2`)와 중복 키는 **합치지 않고 순서대로 나열**한다. 서버마다
해석이 다르므로 도구가 임의로 정규화하면 안 된다.

바디 뷰어는 **Raw / Parsed / Hex / Preview** 네 가지 보기를 제공한다.

| 보기 | 내용 |
|---|---|
| Raw | 디코딩된 텍스트 그대로. 가공 없음 |
| Parsed | JSON 정렬(pretty) + 접이식 트리 + 검색, XML/HTML 구문 강조, 폼 키-값 표 |
| Hex | 헥사 + ASCII 병렬. 바이너리·인코딩 문제 진단용 |
| Preview | JSON 트리, HTML 렌더, 이미지 미리보기 |

Preview의 **HTML 렌더는 샌드박스에서만** 수행한다. 캡처된 응답 본문은 신뢰할 수
없는 입력이며, 스크립트 실행과 외부 리소스 요청을 차단하지 않으면 분석 도구가
캡처 대상에게 요청을 보내는 꼴이 된다. 기본값은 렌더 끔이고 사용자가 명시적으로
켠다.

**압축은 자동 해제한다 — `gzip`, `deflate`, `br`(Brotli), `zstd`.** 해제 후
크기와 원본 크기를 함께 표시하고 **`Content-Encoding` 원본 값을 항상 명시**한다.
`Content-Encoding` 문제 자체가 디버깅 대상인 경우가 많기 때문이다. 해제에
실패하면 조용히 원본을 보여주지 말고 **실패 사실과 사유를 표시**한다 — 손상된
응답과 미지원 코덱은 다른 문제다. 알 수 없는 코덱은 Hex 보기로 떨어뜨린다.

이 자동 디코딩이 HTTPAnalyzer에서 가져오는 실용적 이점이다 `[V, 1.4.3절]`.
프록시 도구 다수가 사용자에게 수동 디코딩을 요구하거나 압축된 바이트를 그대로
보여주는데, 분석 중 가장 자주 마주치는 마찰 지점이다.

리댁션된 헤더/바디는 값을 가리되 **가려졌다는 사실과 규칙 이름**을 보여준다.
`****`만 있으면 사용자가 원본이 없는 건지 가려진 건지 모른다.

### 8.5 필터

**캡처 필터와 표시 필터를 명확히 분리한다.** Wireshark의 핵심 구조이며, 사용자가
가장 자주 혼동하는 지점이기도 하다 `[V, 1.5.5절]`.

| | 캡처 필터 | 표시 필터 |
|---|---|---|
| 적용 시점 | 캡처 중 (기록 전) | 기록된 데이터에 대해 |
| 데이터 손실 | **파괴적 — 되돌릴 수 없다** | 비파괴적 |
| 용도 | 볼륨 제어, 민감 호스트 제외 | 분석 |
| UI 표시 | **적색 계열 + 경고 아이콘** | 통상 |

HTTPAnalyzer도 같은 구분을 두고 문서에 명시했다 — *"display filter는 로깅을 막지
않는다"* `[V]`. 두 도구가 독립적으로 같은 결론에 도달했다는 것은 이 구분이
본질적임을 뜻한다.

> **UI 요건: 두 필터가 시각적으로 혼동 불가능해야 한다.** 캡처 필터가 활성이면
> 상단에 상시 배너를 띄운다 — "캡처 필터 적용 중: 제외된 트래픽은 기록되지
> 않습니다." 10절의 커버리지 정직성이 여기에도 적용된다. 사용자가 스스로 만든
> 사각지대도 사각지대다.

표시 필터 목록:

| 필터 | 형태 |
|---|---|
| 프로세스 | 트리 선택 |
| 호스트/도메인 | 다중 선택 + 검색 |
| 메서드 | 토글 칩 |
| 상태 코드 | 계열 토글(2xx/3xx/4xx/5xx) + 개별 |
| 소요시간 | 하한 슬라이더 |
| 크기 | 하한 |
| 콘텐츠 타입 | 다중 선택 |
| 텍스트 검색 | URL·헤더·바디 대상 (바디는 별도 토글 — 비용이 크다) |
| `coverage` | full / headers_only / metadata_only |
| `fidelity` | semantic / decoded_wire / raw_wire (6.2.1) |
| 시간 구간 | 타임라인 드래그 |

필터 상태는 URL이 아니라 페이지 상태에 두되(라우터가 없으므로),
**리포트 export 시 적용 중인 필터를 함께 기록**한다. 필터된 결과로 만든 리포트가
전체인 것처럼 읽히면 안 된다.

### 8.6 실시간 UI 동작

R10의 화면 설계다. 7.4절의 파이프라인이 만든 데이터를 어떻게 보여줄 것인가.

#### 8.6.1 자동 스크롤 — 정확한 규칙

Wireshark와 Fiddler가 수렴한 패턴을 채택하되, **Wireshark의 알려진 버그를
반면교사로 삼는다** `[V, 1.5.5절]`.

```
기본 상태:  맨 아래 고정. 새 트랜잭션이 아래에 쌓이며 따라간다
사용자가 위로 스크롤:  ▶ 자동 스크롤 일시 중지
                       ▶ "▼ 새 트랜잭션 137건" 플로팅 배지 표시
배지 클릭 또는 맨 아래 도달:  ▶ 자동 스크롤 재개
```

**핵심: 맨 아래로 돌아오면 자동으로 재개되어야 한다.** Wireshark 이슈 #11034는
첫 수동 스크롤 이후 자동 스크롤이 **영구히 꺼지는** 버그다 `[V]`. 사용자가
매번 수동으로 다시 켜야 한다. 이 함정을 피하는 것이 요건이다.

행 선택도 같은 원리다. **선택된 행이 있으면 자동 스크롤을 중지한다** — 사용자가
무언가를 보고 있는데 화면이 움직이면 안 된다.

#### 8.6.2 갱신의 시각적 안정성

실시간 목록에서 가장 흔한 실패는 **행이 튀는 것**이다.

- 신규 행은 짧은 하이라이트 후 정상 배경으로 (300ms). 애니메이션은 넣지 않는다 —
  초당 수백 건에서 애니메이션은 소음이다.
- **정렬을 실시간으로 재적용하지 않는다.** 사용자가 "시간" 내림차순으로 정렬해
  둔 상태에서 새 트랜잭션이 중간에 끼어들면 읽을 수 없다. 정렬이 시작 시각순이
  아니면 신규 행을 **보류 영역에 모으고** 사용자가 갱신 버튼을 누를 때 병합한다.
- 진행 중 행이 완료로 바뀔 때 **위치를 유지**한다(7.4.5절).

#### 8.6.3 실시간 전용 표시

| 요소 | 내용 |
|---|---|
| 녹화 인디케이터 | 적색 점 + 경과 시간. **11.1절의 은닉 불가 원칙과 연결** |
| 진행 중 카운터 | `진행 중 4` — 동시 요청 수는 그 자체로 진단 정보 |
| 유입률 | `초당 128건` |
| 백프레셔 경고 | 7.4.4절의 문구 |
| 커버리지 실시간 경고 | 미식별 비율이 임계 초과 시 즉시 표시 (10.1절) |

**마지막 항목이 실시간 모드의 고유 가치다.** "java 프로세스가 통신 중인데 캡처에
0건"이라는 사실을 **캡처 중에** 알려주면 사용자가 그 자리에서 10.2절의 JVM 프록시
인자를 적용하고 다시 시도할 수 있다. 정지 후에 알면 처음부터 다시 해야 한다.
7.4.1절이 라이브 뷰를 필수로 판단한 두 번째 근거가 이것이다.

#### 8.6.4 실시간 모드 vs 사후 분석 모드

같은 페이지가 두 모드를 오간다. 캡처를 시작하면 실시간, 중지하거나 저장된
세션·HAR을 열면 사후 분석이다. **모드는 화면 어디서든 한눈에 구분되어야
한다** — 어느 모드인지 모르면 "왜 새 요청이 안 들어오지"와 "왜 화면이 계속
움직이지"가 둘 다 발생한다.

| 요소 | 실시간 모드 | 사후 분석 모드 |
|---|---|---|
| 모드 표시 | 적색 점 + 경과 시간 (`● 녹화 중 02:14`) | 세션명 + 캡처 시각·구간 |
| 툴바 | `■ 중지` 활성, `▶ 캡처` 비활성 | `▶ 캡처` 활성, `■ 중지` 비활성 |
| 자동 스크롤 | 있음 (8.6.1절 규칙) | 없음 |
| 신규 행 하이라이트 | 있음 (300ms) | 없음 |
| 정렬 | 시작 시각순 외에는 보류 영역 병합(8.6.2절) | 자유. 즉시 재정렬 |
| 진행 중 트랜잭션 | `⋯`로 표시 | 없음. 있다면 `✕`(중단)으로 확정 |
| 타임라인 | 우측으로 자람. 기본 전체 맞춤(8.7.5절) | 고정 구간 |
| 브러시 선택 | 가능. 선택 시 자동 맞춤 중지 | 가능. 제약 없음 |
| 유입률·동시 요청 수 | 표시 | 숨김 (의미 없음) |
| 백프레셔 경고 | 표시 (7.4.4절) | 숨김 |
| 커버리지 경고 | **즉시 표시 — 이 모드의 고유 가치**(8.6.3절) | 요약에 정적으로 표시 |
| 캡처 필터 | 변경 가능. 변경 시 배너 갱신 | 비활성 — 이미 기록된 데이터다 |
| 표시 필터 | 가능 | 가능 |
| 바디 로딩 | 라이브 창 내는 메모리, 밖은 파일(7.4.2절) | 전부 파일 지연 로딩 |
| 클리어 | 버퍼+집계 초기화, 캡처는 계속 | 세션 닫기와 동등 |
| HAR 내보내기 | 가능 (현재까지의 스냅샷임을 명시) | 가능 |

**설계 원칙: 사후 분석 모드는 실시간 모드의 기능 축소판이 아니다.** 실시간에서
빠지는 것은 정렬·재배치 자유도이고, 사후에서 빠지는 것은 캡처 제어와 라이브
지표다. 분석 기능 자체 — 필터·검색·타임라인 브러시·집계·상세 탭 — 는 **양쪽이
완전히 동일하다.** 8.8.1절의 유계 메모리 요건 덕분에 사후 모드에서도 파일 크기가
분석 가능 범위를 제한하지 않는다.

실시간 → 사후 전환 시 **화면 상태를 보존한다.** 중지 버튼을 눌렀을 때 선택된
행·적용된 필터·타임라인 브러시 구간이 그대로 남아야 한다. 중지는 캡처를 끝내는
동작이지 분석을 리셋하는 동작이 아니다.

### 8.7 전체 타임라인 — 경쟁 도구가 비워둔 자리

R11의 설계다. **1.5.4절의 조사 결론이 직접 구현되는 지점이며, 이 기능이 ArchScope
HTTP 캡처의 차별점이다.**

#### 8.7.1 왜 이것이 차별점인가

Fiddler·Charles·mitmproxy·Proxyman·DevTools·Burp 중 **캡처 전체에 대한 시계열
차트를 가진 도구가 없다** `[V]`. 최대치가 선택 스코프 워터폴이다. Wireshark만
제대로 된 시계열(I/O Graphs)을 갖는데 **HTTP 통계에는 시간축이 없다** `[V]`.

이 공백의 실질적 결과:

> Fiddler에서 "오류율이 언제 튀었나"에 답하려면 **이미 어디를 봐야 할지 알고
> 있어야 한다.** 집계가 선택에 종속되므로 탐색이 성립하지 않는다.

ArchScope는 반대로 간다. **먼저 전체를 보고, 이상한 구간을 찾아, 그 구간으로
좁힌다.** 이것이 ArchScope가 access log·APM·프로파일에서 이미 하던 분석 흐름과
같다 — 새로운 UX를 발명하는 것이 아니라 기존 제품 논리를 HTTP에 적용하는 것이다.

#### 8.7.2 오버뷰 스트립 — 브러시가 핵심

화면 상단에 **상시 표시**되는 시계열 스트립이다. 접을 수 있으나 기본은 펼침.

```
┌─ 전체 타임라인 ─────────────────────────────────── [지표 ▾] [1s ▾] ┐
│ 요청률   ▁▃█▅▂▁▃▇█▄▂▁▁▂▅█▆▃▁▁▂▃▁▁▁▂▄▆█▅▂▁▁▁▂▃▂▁                  │
│ 오류율   ▁▁▂▁▁▁▁▁▁█▇▃▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁                  │
│ 처리량   ▂▃▅▄▃▂▃▅▆▄▃▂▂▃▄▆▅▃▂▂▂▃▂▂▂▃▄▅▆▄▃▂▂▂▃▃▂▁                  │
│          ▲                    └────선택────┘                      │
│      캡처 시작                  14:23:10 ~ 14:23:52 (42초)         │
└──────────────────────────────────────────────────────────────────┘
```

**상호작용은 Chrome DevTools의 오버뷰 스트립에서 가져온다** `[V, 1.5.5절]` —
가로로 드래그해 시간 창을 선택하면 **아래 모든 뷰(목록·트리·상세·집계)가 그
구간으로 필터링된다.** 이것이 "가장 저렴한 전체 타임라인 구현 경로"이며 어떤
프록시 도구도 갖고 있지 않다.

선택 가능 지표(다중 표시):

| 지표 | 산출 |
|---|---|
| 요청률 | 버킷당 완료 트랜잭션 수 |
| 오류율 | 4xx/5xx 비율 — **적색 고정** |
| 처리량 | 송수신 바이트 |
| 지연 백분위수 | p50/p90/p99 (t-digest, 7.4.3절) |
| 동시 요청 수 | 진행 중 트랜잭션 수 |
| **연결 수립 수** | DNS·TCP·TLS 발생 건수 — keep-alive 진단 |
| 프로세스별 누적 | 프로세스를 색으로 구분한 누적 영역 |

**연결 수립 수**를 1급 지표로 두는 것이 HTTPAnalyzer의 Summary Panel에서 가져온
판단이다 `[V]`. `DNS Lookups`·`TCP Connects`·`TotalHTTPSOverhead`를 별도 지표로
노출했고, 6.7절의 `CAPTURE_CONNECTION_CHURN` 진단과 직결된다.

Wireshark I/O Graphs에서 가져올 세부 `[V]`:

- Y축 **로그 스케일** 토글 — 오류율처럼 값 범위가 넓은 지표에 필수
- **이동 평균** N버킷 — 잡음 제거
- 버킷 해상도 수동 지정 (자동은 7.4.3절의 적응 규칙)
- 지표별 표시 필터 적용 (예: 특정 호스트만의 오류율)

한 가지 미묘한 규칙도 함께 가져온다: **값이 0인 버킷은 선/막대 차트에서는
그리되 산점도에서는 생략한다.** 0을 점으로 찍으면 "측정했고 0이었다"와 "데이터가
없다"가 구분되지 않는다.

#### 8.7.3 집계 표 — 호스트·프로세스·엔드포인트

Wireshark의 Conversations를 HTTP 시맨틱으로 옮긴다 `[V, 1.5.5절]`.

| 컬럼 | 비고 |
|---|---|
| 대상 | 호스트 / 프로세스 / 엔드포인트(정규화된 URL 템플릿) |
| 요청 수 | |
| 오류 수 / 오류율 | |
| 총 시간 / 평균 / p90 / p99 | |
| 송수신 바이트 | |
| 연결 수 | 요청 수 대비 비율이 높으면 keep-alive 미작동 |
| **기간 간트 막대** | **인라인.** 아래 |

**행 안에 인라인 간트 막대**를 그려 그 대상이 세션의 어느 구간에 활동했는지
보여준다. Wireshark Conversations가 Start time과 Duration 컬럼에 걸쳐 그리는
방식이다 `[V]`. "이 호스트는 캡처 초반에만 호출됐다"가 표를 읽지 않고 보인다.

**교차 링크**: 각 행에서 "이 대상으로 타임라인 보기"를 제공해 8.7.2의 스트립을
해당 대상으로 필터링한다. Wireshark가 Conversations에서 I/O Graphs를 여는 버튼과
같은 발상이며 `[V]`, 집계표와 시계열을 오가는 것이 실제 분석 동선이다.

엔드포인트 집계는 **URL 정규화**를 거친다(12.4절과 동일 로직). `/user/1`과
`/user/2`가 별개 행이면 집계가 무의미하다.

#### 8.7.4 상태 코드·타이밍 분해 차트

`AnalysisResult.Charts`로 나가는 항목이며(6.6절) 리포트에도 그대로 실린다.

- 상태 코드 분포 (2xx/3xx/4xx/5xx 누적)
- **타이밍 구간 분해 누적 막대** — 호스트별로 DNS/연결/TLS/대기/수신이 총
  시간에 기여한 비율. "이 호스트는 대기가 아니라 DNS가 문제"가 한눈에 보인다
- 호스트별 시간 기여도 (상위 N)
- 콘텐츠 타입별 전송량

타이밍 구간 분해가 특히 ArchScope답다. 1.2절이 지적한 `applyNetworkGap`의
추정을 **분해된 실측으로 대체**하는 것이 이 기능의 존재 이유이고, 이 차트가 그
결과물의 표준 표현이다.

#### 8.7.5 실시간에서의 타임라인

**전체 타임라인은 실시간 모드에서도 전 구간을 보여준다.** 7.4.2절의 3계층
버퍼에서 집계 계층이 라이브 창과 독립이기 때문에 가능하다 — 라이브 창에서
축출된 오래된 트랜잭션도 집계에는 남아 있다.

- 스트립은 우측으로 자라며, 기본은 **전체 구간에 맞춤(fit)**
- 사용자가 브러시로 구간을 선택하면 **자동 맞춤을 중지**한다. 보고 있는 구간이
  움직이면 안 된다(8.6.1절과 같은 원리)
- 해상도가 7.4.3절 규칙으로 낮아질 때 **화면이 튀지 않게** 부드럽게 재바인딩

### 8.8 성능 제약

트랜잭션 수만 건은 일상적이다. 기존 최적화 선례(MSA 타임라인 막대 상한, 캔버스
플레임그래프 행 버킷 히트 테스트)와 같은 방향으로:

- 목록은 가상 스크롤. DOM에 보이는 행만.
- 타임라인은 캔버스 + 시간 버킷 집계. SVG 요소를 트랜잭션마다 만들지 않는다.
- 라이브 갱신은 100ms 배치(7.3).
- 바디는 **상세 탭을 열 때** 지연 로딩. 목록 렌더에 바디가 개입하면 안 된다.
- 텍스트 검색 중 바디 검색은 엔진 측에서 수행하고 결과 ID만 받는다.

#### 8.8.1 유계 메모리를 제품 요건으로

1.5.4절이 확인한 세 번째 공백이다. **경쟁 도구 중 메모리 상한과 축출을 가진
것이 하나도 없다** `[V]`.

| 도구 | 실측/문서화된 한계 |
|---|---|
| Charles | JVM 기본 `-Xmx256m`. 벤더 해법이 "Info.plist를 직접 편집하라" |
| Fiddler Classic | 문서가 안전 작업 집합으로 **200 세션** 제시. 바디를 연속 메모리 블록으로 보관 |
| Fiddler Everywhere | **50 MB 캡처에 32 GB 사용** 신고 |
| mitmproxy | **상한도 축출도 없다.** 유지보수자가 가상화를 "가장 중요한 과제"로 인정 |
| Proxyman | 바디 있는 요청 **약 1,000건**에서 메모리 급증. 장기 구동 시 호스트 네트워크 저하 |
| Wireshark | 전체 파일 인메모리. 100 MB 이상에서 저하 |

따라서 다음을 **명시적 제품 요건**으로 둔다.

> **UI 프로세스의 메모리 사용량은 캡처된 트랜잭션 수에 비례해서는 안 된다.**
> 라이브 창(7.4.2절)이 상한이고, 그 밖은 파일에서 지연 로딩한다. 집계는 상수
> 크기다(7.4.3절).

이 요건이 지켜지면 "8시간 캡처를 켜 두었다"가 정상 사용 사례가 된다. 경쟁 도구
대부분에서 이는 사고다. **원칙 A(캡처와 분석의 파일 경계 분리)가 처음부터 이
방향을 가리키고 있었고**, 이 절은 그것을 측정 가능한 요건으로 고정한다.

검증 방법: 10만 건 합성 세션으로 UI 메모리가 상한 내에 머무는지 회귀 테스트한다.
`Metadata.Extra` 임계치(6.6절)와 함께 실측이 필요한 항목이다.

## 9. 플랫폼 전략 — Windows-first 실행, 이종 OS 증거 분석

이 절의 우선순위는 **Windows 실행을 먼저 완성하고, 입력 데이터의 이식성을
유지한 뒤, 다른 OS의 실시간 캡처를 확장하는 것**이다.

1. Windows 데스크톱 UI + HAR import + MITM proxy를 1차 제품 경로로 검증한다.
2. Linux/macOS에서 생성된 HAR·프로파일·로그는 Windows에서 오프라인 분석한다.
3. Linux/macOS용 실시간 캡처 백엔드는 capability가 검증될 때 추가하며, Windows
   출시의 동등성 gate로 사용하지 않는다.

### 9.1 모드 가용성 행렬

| 모드 | Windows (1차) | macOS (후속 live) | Linux (후속 live) |
|---|---|---|---|
| MITM 프록시 | ◎ WinINet 프록시 설정 | ◎ 사용자 권한. 시스템 프록시 설정 시 관리자 인증 | ◎ 환경변수/GNOME 설정. 배포판별 편차 |
| CA 신뢰 등록 | 인증서 저장소 (관리자) | 키체인 (관리자 인증) | 배포판마다 다름 — `update-ca-certificates` / `trust anchor` / NSS DB 별도 |
| pcap | △ **Npcap 별도 설치** | ◎ BPF 장치 권한 필요 | ◎ `cap_net_raw` |
| eBPF | ✗ | ✗ | ◎ 커널 5.8+ / BTF |
| ETW | ◎ 관리자 | ✗ | ✗ |
| Network Extension | ✗ | △ 엔티틀먼트 필요 | ✗ |
| 키 로그 + pcap | ○ | ○ | ○ |

Linux의 CA 신뢰가 특히 지저분하다. 시스템 저장소(`/usr/local/share/ca-certificates`)와
**NSS DB(Chrome/Firefox가 쓰는 별도 저장소)가 따로 논다.** 두 곳 모두 처리해야
브라우저에서 동작하고, 이는 배포판/브라우저 조합마다 다르다. 이 부분은 자동화를
시도하되 **실패 시 수동 절차를 화면에 출력**하는 것이 현실적이다.

### 9.2 권한 분리

3.0에서 "권한 상승이 없다"는 전제가 깨진다고 했다. 파장을 최소화하는 구조:

```
[ArchScope 앱 — 일반 사용자 권한]
   │ 로컬 IPC (Unix 소켓 / 명명 파이프, 피어 자격 검증)
   ▼
[archscope-capture-helper — 필요한 모드에서만 승격]
```

- **프록시 모드는 헬퍼가 필요 없다.** 프록시 자체는 사용자 권한으로 뜬다.
  시스템 프록시 설정 변경과 CA 등록만 OS 인증 대화상자를 유발하는 **일회성
  동작**이다. 이것이 프록시를 1순위로 둔 세 번째 이유다.
- pcap/eBPF/ETW는 헬퍼를 통해야 한다. 헬퍼는 **캡처만** 하고 파일을 쓰며,
  분석·UI·네트워크 송신은 하지 않는다.
- 헬퍼는 상주 데몬으로 설치하지 않는다. **세션 동안만 살아 있다.** 상주
  권한 프로세스는 공격면이 크고, 이 제품의 사용 패턴상 필요하지도 않다.

`apps/engine-native/internal/parsers/jfr/recording.go:119`의 `exec.LookPath` + 외부 도구 호출이
외부 실행 파일을 다루는 기존 선례이나, **권한 승격 선례는 없다.** 헬퍼 서명·배포·
검증은 신규 과제이며 `docs/ko/PACKAGING_PLAN.md`의 개정이 필요하다.

### 9.3 Windows 모드별 기능·정확도 행렬

9.1은 "그 모드가 Windows에서 뜨는가"만 답한다. **release gate로 쓰려면 "뜬 다음
무엇을 어느 정확도로 주는가"가 필요하다.** 2.1의 capability tier와 6.2.1의
fidelity 등급을 모드 축으로 교차시킨 것이 이 절이며, Phase 0 게이트 2의 산출물이다.

읽는 규칙 두 가지를 먼저 고정한다.

- **`not_tested`를 `unsupported`로 쓰지 않는다.** 미검증은 미검증이다. 아래 표의
  `미검증`은 전부 "실측 전"이라는 뜻이며, T-571 spike와 Phase 2 E2E가 이 칸을
  채운다.
- **행렬의 각 칸은 트랜잭션 필드로 표현 가능해야 한다.** UI가 모드별 각주를 다는
  대신 `Fidelity`·`Coverage`·`Attribution`이 행 단위로 답한다(6.2). 행렬은 그
  필드가 어떤 값을 갖게 되는지의 사전이다.

#### 9.3.1 프로세스 귀속

| 모드 | 귀속 수단 | 기대 `Attribution` | 실패 조건 |
|---|---|---|---|
| MITM 프록시 | 프록시 소켓의 peer endpoint → `GetExtendedTcpTable` 역조회 (`V-WIN-TCPTABLE`) | `confirmed` — 연결 수립 시점에 조회 성공 시 | 단명 연결에서 조회 전 endpoint 소멸 → `inferred` 또는 `unknown` (4.3) |
| HAR 가져오기 | 없음. 생성 도구가 남긴 값이 있으면 승계 | `unknown` 기본 | 대부분의 방언이 PID를 남기지 않는다 |
| ETW | 이벤트 payload의 `Execution` PID + `sport`/`dport` (`Q-WIN-ETW-PAYLOAD`) | payload에 PID·포트 존재 **확인 (2026-07-20)**. 단 실 NIC 트래픽 한정 — **loopback 미관측** | loopback(127.x) 트래픽은 이벤트로 나오지 않음(10.4.5); 실 NIC 귀속 정확도는 미측정, 실 NIC 재실행 필요 |
| WFP | `netsh wfp` netevents 의 appId (`Q-WIN-WFP-ATTR`) | 기본 구성에서 **미확정 (2026-07-20)** — 허용(ALLOW) 연결이 netevents에 기록되지 않음 | ALE connection audit 미설정 시 ALLOW 미기록(드롭만 관측, 10.4.5) |
| TCP endpoint ownership | `Get-NetTCPConnection`/`GetExtendedTcpTable` owner PID (`V-WIN-TCPTABLE`) | `confirmed` — loopback 포함 PID 정확도 **100% 실측 (2026-07-20)** | 폴링 간격보다 짧은 연결은 미관측(구조적, 4.3) |
| Npcap | 없음 — 패킷에 PID가 없다 | `unknown`. endpoint table 대조 시 `inferred` | 폴링 한계 그대로 (4.3) |
| pcap + 키 로그 | 키 로그를 남긴 프로세스로 한정 | `confirmed` (대상 프로세스에 한해) | 그 외 프로세스는 관측 대상 아님 |

프록시 모드가 유일하게 `confirmed`를 **구조적으로** 얻는 경로다(4.4). 이것이
3.7절 선택의 근거 중 하나이며, 나머지 모드는 전부 실측이 선행되어야 한다.

#### 9.3.2 HTTP 버전

| 모드 | HTTP/1.1 | HTTP/2 | HTTP/3 (QUIC) | WebSocket |
|---|---|---|---|---|
| MITM 프록시 | 지원 | Phase 2 결정 대상 — build-vs-integrate 선택지 3은 **H2 passthrough** (13절) | **unsupported.** UDP 기반이라 이 프록시 경로를 타지 않는다 | upgrade 감지 후 메타데이터. 메시지 파싱은 Phase 5 |
| HAR 가져오기 | 지원 | 지원 (생성 도구가 기록한 만큼) | 생성 도구가 기록했다면 승계 | 방언별 편차 — 6.5 |
| Npcap | 평문만. TLS 구간은 메타데이터 | HPACK 상태 때문에 **중간 시작 시 헤더 복원 실패** (`CAPTURE_H2_HEADERS_LOST`) | 메타데이터만 | 평문만 |
| pcap + 키 로그 | 지원 | 지원 (연결 시작부터 캡처한 경우) | **미검증** | 지원 |

**QUIC은 모든 Windows 라이브 모드에서 `unsupported`다.** 이를 커버리지 카운터의
`unsupported`로 명시 계상하고(10.1.2), 조용히 0건으로 두지 않는다. 브라우저가
HTTP/3로 붙으면 프록시 모드에서는 요청이 아예 보이지 않으므로, **사용자에게
"QUIC을 끄면 관측된다"는 안내를 띄우는 것이 실질적 대응**이다(10.2와 같은 성격).

#### 9.3.3 헤더·바디 fidelity

6.2.1의 세 등급을 모드에 배정한다. **이 표가 R3의 실제 계약이다.**

| 모드 | 기본 등급 | `decoded_wire` 승격 조건 | `raw_wire` |
|---|---|---|---|
| MITM 프록시 | `semantic` | 자동 해제를 끄고 wire byte 계측 계층을 별도로 둘 때 | 범위 밖 — 약속하지 않는다 |
| HAR 가져오기 | `semantic` | 생성 도구가 wire byte를 남긴 방언에 한해 | 불가 — HAR에 원본 프레이밍이 없다 |
| Npcap | `decoded_wire` (평문 구간) | 기본 제공 — 패킷이 원본이다 | 가능하나 Phase 5 |
| pcap + 키 로그 | `decoded_wire` | 기본 제공 | 가능하나 Phase 5 |
| ETW / WFP | `metadata_only` (`Coverage`) | 해당 없음 — 페이로드를 주지 않는다 | 불가 |

패킷 기반 모드가 fidelity에서 프록시보다 **우월하다**는 점을 숨기지 않는다.
프록시를 1순위로 둔 이유는 fidelity가 아니라 귀속·권한·AV 비용이다(3.7, 1.4.5).
두 축은 서로 다른 이유로 갈리며, 이 표는 그 트레이드오프를 명시하는 자리다.

#### 9.3.4 타이밍 관점

6.3의 `TimingSet` 중 어떤 필드가 채워지는지를 모드가 결정한다.

| 모드 | `ClientProxy` | `ProxyInternal` | `ProxyUpstream` | `ImportedHar` |
|---|---|---|---|---|
| MITM 프록시 | known | known | known | — |
| HAR 가져오기 | — | — | — | known (도구 관점 승계, 6.5.4 정규화 후) |
| Npcap / 키 로그 | — | — | — | — (관측 지점이 다르므로 별도 관점으로 채운다) |

**프록시 모드는 원 클라이언트의 DNS·서버 TCP/TLS 핸드셰이크를 직접 관측하지
못한다**(6.3). 따라서 `ClientProxy.DNS`는 `not_applicable`이고, 클라이언트 관점
총합은 `derived: true` 추정값으로만 제공한다. 패킷 기반 모드는 관측 지점이
클라이언트 NIC이라 관점 자체가 다르므로, `TimingSet`에 관점을 하나 더 두거나
`observationPoint`로 구분한다(6.2.2) — **어느 쪽이든 프록시 값과 직접 합산하지
않는다.**

#### 9.3.5 권한·설치 요구

| 모드 | 실행 권한 | 일회성 권한 상승 | 별도 설치물 | 헬퍼 필요 |
|---|---|---|---|---|
| HAR 가져오기 | 일반 사용자 | 없음 | 없음 | 아니오 |
| MITM 프록시 | 일반 사용자 | CA 신뢰 등록, 시스템 프록시 설정 시 (11.2, 5.1) | 없음 | **아니오** |
| Npcap | 일반 사용자 (드라이버가 처리) | 설치 시 관리자 | **Npcap** (`V-WIN-NPCAP-INSTALL`) | 예 |
| ETW | 관리자 | 세션마다 | 없음 | 예 |
| WFP | 관리자 | 세션마다 | 없음 | 예 |
| pcap + 키 로그 | 일반 사용자 + Npcap | 위와 동일 | Npcap | 예 |

**프록시만 상주 권한 승격이 없다.** 9.2의 헬퍼 구조가 필요한 모드와 그렇지 않은
모드가 여기서 갈리며, Phase 2가 프록시로 시작하는 세 번째 근거다.

#### 9.3.6 명시적 unsupported

"안 되는 것"을 표로 고정한다. 조용한 0건이 가장 위험하다는 10절 원칙의 사전 적용이다.

| 대상 | 상태 | 사용자에게 보이는 것 |
|---|---|---|
| HTTP/3 · QUIC | 전 모드 `unsupported` | `unsupported` 카운터 + QUIC 비활성화 안내 |
| 인증서 피닝 앱 | 프록시에서 `metadata_only` | 진단된 원인 표시 (5.3). **우회하지 않는다** |
| Schannel/.NET/Java/Go 등 프록시 미준수 앱 | 프록시에서 미관측 | 프로세스 트리에 미관측 표시 + 설정 안내 (10.2) |
| 로컬호스트 간 통신 | 프록시 설정 우회가 흔함 | 안내. Npcap 대조 모드로 확인 (10.3) |
| 비 HTTP 트래픽 | 범위 밖 (1.3) | 계상하지 않음 |
| 원격 머신 트래픽 | **영구 범위 밖** (11.1) | 기능 자체가 없음 |

### 9.4 Windows 런타임 지원과 이종 OS 증거 호환성은 다른 축이다

R8이 두 가지를 요구하는데 하나의 "크로스 플랫폼"으로 읽으면 릴리스 판정이
불가능해진다. **분리해서 각각의 수용 기준을 둔다.**

| 축 | 의미 | 1차 출시 조건 | 검증 방법 |
|---|---|---|---|
| **런타임 지원** | 그 OS에서 ArchScope가 실시간 캡처를 수행하는가 | **Windows만.** 9.3의 행렬이 Windows 기준으로 채워지면 충족 | Windows E2E (Phase 0 게이트 10) |
| **증거 호환성** | 그 OS에서 *생성된* 증거를 Windows UI에서 분석할 수 있는가 | **Linux/macOS/Windows 전부.** Tier A | 이종 OS 생성 fixture 코퍼스 (T-572) |

증거 호환성 쪽에서 실제로 OS 차이가 새는 지점만 따로 다룬다. 나머지는 HAR 스키마
수준에서 동일하다.

| 새는 지점 | 증상 | 처리 |
|---|---|---|
| 경로 표기 | `execPath`·`SourceFiles`에 POSIX 경로가 들어온다 | 정규화하지 않고 **원본 보존**. 표시만 축약 (11.3.3) |
| 시간대 | 생성 도구가 로컬 오프셋으로 기록 | RFC3339 오프셋을 보존하고 UTC로 정렬. 오프셋 누락 시 finding |
| 줄바꿈·인코딩 | CRLF/LF, BOM | 파서에서 흡수 |
| 방언 차이 | Firefox의 `ssl` 이중 계상 등 | 6.5.4 정규화. **OS가 아니라 생성 도구의 함수다** |

마지막 행이 요점이다. **HAR 방언 차이는 생성 OS가 아니라 생성 도구에서 온다.**
따라서 "Linux에서 만든 HAR"이라는 분류는 진단에 쓸모가 없고, `creator.name`과
`creator.version`이 분류 키다. 이종 OS fixture가 필요한 이유는 방언 때문이 아니라
**경로·시간대·인코딩 때문**이며, T-572의 코퍼스 설계도 그 축으로 나눈다.

## 10. 커버리지 정직성

3.2에서 프록시가 R1을 완전히 만족하지 못한다고 했다. 이 한계를 **제품이 스스로
말하게** 만드는 것이 이 절이다. 조용한 누락이 가장 위험하다 — 사용자는 "안 잡힌
것"과 "통신이 없는 것"을 구분할 수 없다.

### 10.1 커버리지 지표

각 세션의 `Summary`에 포함한다.

| 지표 | 의미 |
|---|---|
| `attributedRatio` | PID가 확정된 트랜잭션 비율 |
| `decryptedRatio` | 평문 확보 비율 (전체 대비) |
| `passthroughHosts` | 피닝 등으로 통과시킨 호스트 목록 |
| `proxyUnawareProcesses` | 프록시를 거치지 않은 것으로 **의심되는** 프로세스 |

#### 10.1.1 철회 — "프로세스별 시스템 바이트" 전제는 성립하지 않는다

개정 전 이 절은 `bytesObserved` 대 `bytesSystem`을 핵심 지표로 두고, OS가
프로세스별 네트워크 바이트 카운터를 값싸게 제공한다고 적었다. **2026-07-19 검토
결과 이 전제는 세 플랫폼 어디에서도 그대로 성립하지 않는다.**

| 근거로 삼았던 것 | 실제 |
|---|---|
| Windows 성능 카운터 | byte 지표가 **Network Interface/Adapter 단위**다. 프로세스 단위가 아니다 |
| `GetExtendedTcpTable` | owner PID가 붙은 **endpoint table**이지 연결별 누적 byte counter가 아니다 |
| Linux `/proc/<pid>/net` | 해당 프로세스가 속한 **network namespace**의 스택 정보다. 프로세스 단독 카운터가 아니다 |
| macOS `nettop` | 이를 받치는 **안정적인 공개 per-process API가 확인되지 않았다** |

Windows ETW의 TCP/IP event는 send/receive를 process/flow로 귀속시킬 후보이지만,
**event header의 PID를 그대로 발신 프로세스로 쓰지 말라는 공식 주의**가 있다.
따라서 실제 event payload와 손실률을 먼저 실측해야 한다.

이것은 Phase 3 한 항목의 오류가 아니다. 설계가 Phase 2의 부분 커버리지를 정직하게
보완하는 **핵심 장치**로 10절을 쓰고 있으므로 제품 신뢰성의 P0다.

#### 10.1.2 대체 설계 — 증거에 scope를 붙인다

모든 커버리지 신호는 자기 scope를 밝힌다. **scope가 같은 값끼리만 비율로
비교하고, 그 외에는 "참고 신호"로만 표시한다.**

```go
type CoverageEvidence struct {
    Metric           string  `json:"metric"`
    Value            float64 `json:"value"`
    // "process" | "cgroup" | "network_namespace" | "adapter" | "host"
    Scope            string  `json:"scope"`
    Source           string  `json:"source"`   // etw | wfp | npcap | perfcounter | proxy
    Privilege        string  `json:"privilege"`
    SamplingInterval string  `json:"samplingInterval,omitempty"`
    Confidence       string  `json:"confidence"` // high | medium | low
}
```

**Windows Phase 2의 완료 조건은 프로세스별 byte 비교가 아니다.** 관측 가능한
카운터로 정의한다.

| 카운터 | 의미 |
|---|---|
| `captured` | 복호화·파싱까지 성공한 트랜잭션 |
| `passthrough` | 통과시켰고 메타데이터만 있는 것 |
| `unattributed` | 관측했으나 프로세스를 확정하지 못한 것 |
| `dropped` | 유실(7.4.2의 손실 회계와 동일 정의) |
| `unsupported` | QUIC 등 이 모드가 다룰 수 없는 것 |

프록시 우회 탐지는 **Windows ETW/WFP 또는 Npcap 대조가 실제로 가능해진 뒤**
승격한다. Linux eBPF는 후속 capability다. **Windows proof-of-capability spike가
실패하면 절대적 coverage ratio 표시 자체를 제거한다** — 검증 못 한 분모로 만든
비율은 정직성 장치가 아니라 새로운 거짓말이다.

공식 근거:

- [Microsoft network-related performance counters](https://learn.microsoft.com/en-us/windows-server/networking/technologies/network-subsystem/net-sub-performance-counters)
- [Microsoft GetExtendedTcpTable](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable)
- [Microsoft TCP/IP ETW events](https://learn.microsoft.com/en-us/windows/win32/etw/tcpip)
- [Linux proc_pid_net(5)](https://man7.org/linux/man-pages/man5/proc_pid_net.5.html)

### 10.2 프로세스별 설정 안내

3.2의 표를 UI 안내로 전환한다. 예: 트리에서 `java` 프로세스가 미관측으로
표시되면 `-Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=<port>` 를 복사 가능한
형태로 제시한다. Node면 `NODE_EXTRA_CA_CERTS`와 프록시 dispatcher 설정을 안내
한다.

**이 안내가 프록시 방식의 커버리지 약점을 실질적으로 상쇄하는 유일한 수단이다.**
기술로 못 메우는 부분을 사용자 행동으로 메운다.

### 10.3 pcap 대조 모드

프록시와 pcap을 **동시에** 돌리는 모드를 둔다. pcap은 프록시가 못 본 5-tuple을
찾아 "관측되지 않은 연결" 목록을 만든다. HTTPS면 SNI만이라도 얻는다.

비용(root 권한, CPU)이 있으므로 기본값이 아니라 **커버리지 진단용 옵션**이다.

### 10.4 Windows proof-of-capability spike (T-571)

10.1.1이 프로세스별 byte 전제를 철회했고, 10.1.2가 "실측 뒤에 승격한다"고 정했다.
**무엇을 측정하면 승격인지**를 여기서 고정한다. 이 기준이 없으면 spike가 "해봤다"
로 끝나고 결론이 남지 않는다.

#### 10.4.1 측정 대상

Windows에서 프록시 우회 탐지에 쓸 수 있는 후보는 셋이며, **각각 다른 scope의
증거를 준다**(10.1.2의 `CoverageEvidence.Scope`).

| 후보 | 주장하려는 것 | 미확인 항목 |
|---|---|---|
| ETW TCP/IP | 연결 수립·송수신을 프로세스로 귀속 | payload 필드 구성, PID 신뢰성, 고부하 손실률 (`Q-WIN-ETW-PAYLOAD`) |
| WFP | 필터 계층에서 연결 관측 | 귀속 정확도, 설치·권한 비용 (`Q-WIN-WFP-ATTR`) |
| Npcap | 패킷 수준 5-tuple 관측 | 별도 설치 수용성. **PID는 원리적으로 없다** |

#### 10.4.2 수용 기준

각 후보에 대해 아래를 **수치로** 기록한다. 판정은 후보별로 독립이다.

| ID | 기준 | 통과 조건 |
|---|---|---|
| `CAP-1` | 귀속 정확도 | 알려진 대조군(프록시를 타는 프로세스 N개)에 대해 귀속이 일치하는 비율. **95% 이상** |
| `CAP-2` | 오탐 | 통신하지 않은 프로세스에 트래픽이 귀속되는 사례. **0건** |
| `CAP-3` | 손실률 | 초당 500 트랜잭션 부하에서 커널 계층이 보고한 유실. **1% 미만**이고 카운터로 노출 가능 |
| `CAP-4` | 우회 탐지 | 프록시를 의도적으로 무시하는 앱(예: 프록시 미준수 HTTP 클라이언트)의 연결을 탐지. **탐지 성공** |
| `CAP-5` | 비용 | 캡처 중 CPU 오버헤드. **10% 미만** |
| `CAP-6` | 권한·설치 | 9.3.5의 요구가 실제와 일치 |

`CAP-2`가 0건인 이유는 **오탐이 이 기능의 목적을 정면으로 파괴하기 때문**이다.
10절의 존재 이유가 "조용한 누락을 드러내는 것"인데, 잘못된 귀속은 새로운 조용한
거짓말이다. 정확도가 낮은 것보다 **틀린 것이 나쁘다.**

#### 10.4.3 판정과 그 결과

| 결과 | 산출물 |
|---|---|
| 어느 후보든 `CAP-1`~`CAP-4` 통과 | 해당 scope에서 coverage ratio를 UI에 노출. `CoverageEvidence.Confidence: high` |
| `CAP-1`은 통과하나 `CAP-3` 실패 | ratio를 노출하되 **손실률을 함께 표시**. `Confidence: medium` |
| `CAP-2` 실패 | 그 후보를 **폐기한다.** 부분 사용도 하지 않는다 |
| 전 후보 실패 | **절대적 coverage ratio 표시를 제거한다.** 10.1.2의 5개 카운터(`captured`/`passthrough`/`unattributed`/`dropped`/`unsupported`)만 남긴다 |

마지막 행이 T-571의 실질적 안전장치다. **비율을 못 만들어도 제품은 성립한다** —
카운터는 프록시 자신의 관측만으로 산출되며 검증된 분모를 요구하지 않는다. 비율은
있으면 좋은 것이지 없으면 안 되는 것이 아니고, **검증 못 한 분모로 만든 비율은
정직성 장치가 아니라 새로운 거짓말이다.**

spike 결과는 부록 A의 `Q-WIN-ETW-PAYLOAD`·`Q-WIN-WFP-ATTR` 행을 채우며, 9.3.1
행렬의 `미검증` 칸도 함께 확정된다.

#### 10.4.4 Linux/macOS는 막지 않는다

eBPF(Linux)와 Network Extension(macOS)이 더 나은 귀속을 줄 가능성은 있으나,
**Windows 출시의 gate가 아니다**(9절, 2.1의 Tier E). 이 spike는 Windows 범위로
한정하고, 다른 OS의 capability 조사는 후속 항목으로 둔다. 여기서 Linux 증명을
기다리면 Windows-first 전제가 무의미해진다.

#### 10.4.5 실측 결과 (2026-07-20, loopback smoke)

첫 실측을 `spikes/t571-windows-coverage` 하네스로 수행했다. 환경은 단일 머신
(Windows 10.0.26200.8737), 대조군 5개 프로세스가 loopback 리스너로 약 500 tps
(달성 498 tps)를 구동했다. **단일 머신 loopback 구성**이라는 점이 결과 해석의
핵심 전제다.

| 후보 | scope | CAP-1 | CAP-2 | CAP-3 | CAP-4 | CAP-5 | 요약 |
|---|---|---|---|---|---|---|---|
| ETW Kernel-Network | 프로세스 귀속 | N/A¹ | N/A¹ | 손실 0%² | N/A¹ | pass | payload·손실 확인; 귀속은 실 NIC 필요 |
| TCP endpoint ownership | 프로세스 귀속 | **100% (5/5)** | **0건** | N/A | **탐지** | 경계³ | **CAP-1~4 통과 (Confidence: high)** |
| WFP netevents | 프로세스 귀속 | N/A⁴ | N/A⁴ | N/A | N/A⁴ | pass | 기본 구성 미확정 |

- ¹ **ETW는 loopback(127.x) 트래픽을 관측하지 않는다.** 대조군 포트가 151 MB
  이벤트 XML에 한 번도 나오지 않았다. 반면 실제 NIC 트래픽 140개 flow는 PID·포트와
  함께 정상 관측됐다. 따라서 ETW 귀속 정확도·우회 탐지는 실 NIC 트래픽으로만 측정
  가능하며, 이번 loopback 실행에서는 N/A로 남는다(실패가 아님).
- ² 30초 캡처에서 100,541 이벤트, **손실 0건 / 버퍼 손실 0건**. payload에는
  `Execution` PID와 `sport`/`dport`가 존재한다 → `Q-WIN-ETW-PAYLOAD`의 payload·손실
  질문은 확정.
- ³ CAP-5 CPU 오버헤드가 표본에 따라 9~13%p로 10%p 경계를 오갔다. 원인은 probe가
  `Get-NetTCPConnection`을 1초 간격 PowerShell로 폴링하기 때문이다. 운영 경로는
  `GetExtendedTcpTable`을 직접 호출해 이 비용을 제거해야 한다(구현 시 개선 항목).
- ⁴ WFP `netsh wfp show netevents`는 기본 구성에서 **허용(ALLOW) 연결을 기록하지
  않았다** — 드롭 이벤트 1건만 관측됐다. WFP 귀속을 쓰려면 ALE connection audit
  활성화가 선행되어야 하며, 그 전까지 WFP 행은 미확정이다.

**결론.** 단일 머신 loopback 검증에서 **TCP endpoint ownership scope가 CAP-1~4를
통과**했다 — `V-WIN-TCPTABLE`의 owner-PID 역조회를 실측으로 재확인한 것이며, 프록시
모드의 귀속 근거(9.3.1의 `confirmed`)가 실측으로 뒷받침된다. 다만 이 scope는 폴링
기반이라 폴링 간격보다 짧은 연결을 놓치므로, 우회 탐지의 완전한 커널 관측을
대체하지는 못한다. ETW는 payload·손실이 확인되어 ratio 노출의 payload 전제는
충족하나, **귀속 정확도와 우회 탐지(CAP-1/4)는 실 NIC 재실측이 남았다.** WFP는
audit 설정 없이는 후보에서 제외된다.

**잔여 작업 (T-571).** (a) ETW/WFP를 실 NIC 타깃(`-Target <다른 호스트:포트>`)으로
재실행해 ETW의 CAP-1/CAP-4를 확정한다. (b) `GetExtendedTcpTable` 직접 호출로
TCP-owner CAP-5를 재측정한다. (c) WFP는 ALE audit 활성 여부에 따라 재평가한다. 이
재실측 전까지 §10.1.2의 절대 coverage ratio는 노출하지 않고
`captured/passthrough/unattributed/dropped/unsupported` 5개 카운터만 유지한다.

## 11. 보안 및 프라이버시

이 기능은 ArchScope에서 **가장 위험한 기능**이다. 설계 단계에서 방어선을
확정한다.

### 11.0 위협 모델

방어선을 나열하기 전에 **누구로부터 무엇을 지키는지**를 먼저 고정한다. 이것이
없으면 개별 대책의 충분성을 판정할 수 없다.

| 행위자 | 능력 | 주요 위험 | 1차 방어 |
|---|---|---|---|
| 로컬 사용자 (본인) | 앱과 세션 파일 전권 | 자기 자격증명을 무심코 파일로 남김 | 저장 시점 리댁션 기본 켜짐 (11.3) |
| 공용 워크스테이션의 다른 사용자 | 같은 머신의 다른 계정 | 세션 디렉터리·CA 키 열람 | OS 키체인 저장, 플랫폼별 소유자 전용 ACL 폴백, 앱 데이터 경로 권한 (11.2) |
| 내보낸 파일 수신자 | HAR/리포트만 보유 | 리댁션이 부족해 토큰·PII 유출 | 내보내기 시 재확인 + export 제외 정책 (11.3) |
| 악의적 HAR 제공자 | 입력 파일을 제어 | 파서 자원 고갈, 압축 폭탄, 과도한 중첩 | 가져오기 자원 상한 (6.5.6, 아래 11.3) |
| CA 개인키 탈취자 | 키 파일 획득 | 해당 머신 TLS 위조 | 머신 고유 생성, 내보내기 없음, 1년 만료 (11.2) |
| 크래시 덤프·로그 수집 | OS가 생성한 크래시 산출물·로그(활성 디버거의 전권 메모리 읽기는 범위 밖) | 평문 바디·CA 키 유출 | 키 로드 전 덤프 비활성화, 키스토어 서명 handle 우선, 민감 버퍼 수명·크기 최소화 (11.2) |

**"기본 안전"의 정의**: 사용자가 아무 설정도 바꾸지 않은 상태에서 만들어진 세션
파일을 제3자에게 그대로 넘겨도 **재사용 가능한 자격증명이 들어 있지 않아야
한다.** 아래 11.3의 개정은 전부 이 기준에서 나온다.

### 11.1 이 도구가 무엇인지 정직하게

시스템 전역 HTTP 캡처는 **도청 도구**다. 기술적으로 중립이라는 말로 넘어갈 수
없다. 방어선:

- **로컬 머신 한정.** 원격 캡처, 원격 배포, 원격 조회 기능을 만들지 않는다.
- **은닉 불가.** 캡처 중에는 항상 화면에 표시가 있고, 백그라운드 무표시 캡처
  모드를 제공하지 않는다. 트레이 아이콘 상태 변경을 포함한다.
- **자동 시작 없음.** 앱 실행만으로 캡처가 시작되지 않는다. 명시적 시작만.
- **최초 사용 시 고지.** 법적·윤리적 고지를 1회 표시하고 확인을 받는다. 타인의
  통신을 동의 없이 캡처하는 것은 다수 관할권에서 위법일 수 있다는 내용.
- **라이브 캡처 CLI를 제공하지 않는다.** `archscope-engine`은 파일 가져오기와
  분석만 제공한다. 캡처 시작은 지속 인디케이터와 중지 제어가 있는 Wails UI에서만
  가능하며 detached/headless 시작 경로는 등록하지 않는다. 이 계약을 바꾸려면 별도
  보안 결정과 `SEC-16` 재검증이 선행되어야 한다.

#### 11.1.1 캡처 범위 최소화

라이브 캡처를 시작할 때 사용자는 host 패턴 allowlist, 확인된 process identity
allowlist, 또는 **"모든 로컬 프로세스"**를 명시적으로 선택한다. 마지막 선택은
별도 경고·세션별 확인 없이는 활성화하지 않는다.

- host/process scope는 세션 manifest에 기록하고 캡처 중 UI에 계속 표시한다.
- scope 밖 트랜잭션은 **헤더·바디를 세션 저장소에 쓰기 전에 드롭**한다.
- process 범위를 선택했는데 귀속이 `unknown`이면 기본은 저장하지 않는 fail-closed다.
  사용자가 별도로 `metadata_only` 보존을 선택한 경우에만 최소 연결 메타데이터를
  남기며 헤더·바디는 수집하지 않는다.
- scope 변경은 새 설정 버전과 감사 이벤트를 남기고 이후 트랜잭션에만 적용한다.

### 11.2 CA 개인키

5.1에서 언급한 최대 위험이다.

| 항목 | 처리 |
|---|---|
| 저장 | macOS 키체인 / Windows DPAPI / Linux Secret Service. 파일 폴백은 아래 플랫폼별 소유자 전용 ACL 계약을 강제 |
| 메모리 | Go GC 메모리의 완전 zeroize를 주장하지 않는다. 키 load 전 crash dump를 비활성화하고, 비수출 키스토어 서명 handle을 우선하며, 불가피한 평문 버퍼는 유계·최단 수명으로 유지하고 로그에 넣지 않는다 |
| 내보내기 | **제공하지 않는다.** 백업 기능도 없다 (재생성이 정답) |
| 머신 고유성 | 머신마다 새로 생성. 빌드에 CA를 동봉하지 않는다 |
| 만료 | 1년. 만료 시 자동 재생성 + 재등록 안내 |
| 제거 | 설정 화면에서 1클릭. 앱 제거 시 안내 |

파일 폴백은 권한을 강제하지 못하면 생성 자체를 실패시킨다.

- **Windows:** ACL 상속을 끄고 현재 사용자와 `SYSTEM`만 읽기/쓰기를 허용한다.
- **macOS/Linux:** 키 파일 `0600`과 상위 앱 데이터 디렉터리 `0700`을 함께 검증한다.
- 어떤 플랫폼에서도 단순 기본 상속 권한이나 world/group-readable 폴백을 허용하지
  않는다. 권한 실측 결과는 CA 상태 진단에 남긴다.

Go 런타임에서는 GC가 복사한 키·바디 잔본을 신뢰성 있게 zeroize할 수 없으므로
"사용 후 즉시 폐기"를 물리적 삭제 보장으로 표현하지 않는다. Phase 2 구현은 민감
자료를 읽기 **전에** Linux의 `RLIMIT_CORE=0`/지원 시 `PR_SET_DUMPABLE=0`, Windows의
WER/local-dump 제외, macOS의 core-dump 비활성 정책을 적용하고 실패 시 HTTPS 평문
캡처를 시작하지 않는다. 가능한 플랫폼은 CA 서명을 OS 키스토어의 non-exportable
handle로 수행한다. 평문 바디는 bounded buffer에서만 처리하고 참조를 즉시 해제하되,
동일 사용자 권한의 활성 디버거로부터 힙을 보호한다고 주장하지 않는다.

호스트별 **리프 개인키는 메모리 캐시에만 존재하고 디스크에 기록하지 않는다.**
세션 종료 시 캐시 참조를 폐기하며, 위와 같은 Go 런타임 한계를 동일하게 적용한다.

**빌드에 CA를 동봉하지 않는다**는 항목이 특히 중요하다. 공용 CA 개인키가 배포되면
그 개인키로 전 세계 사용자의 TLS를 위조할 수 있다. 과거 여러 제품이 실제로 저지른
사고다.

#### 11.2.1 CA 수명주기

5.1이 CA의 속성을, 11.2가 키 보관을 정했다. **남은 것은 상태 전이다** — 언제
생성되고, 언제 신뢰 저장소에 들어가고, 언제 나오는가. 이 경로가 불명확하면
"제거했다고 생각했지만 남아 있는 CA"가 만들어진다. 그것이 이 기능의 최악
시나리오다.

```
  none ──generate──→ generated ──install──→ trusted
                         ▲                     │
                         │                  uninstall
                         │                     ▼
                         └──── generated ←─────┘
                         │
                      expired ──regenerate──→ generated
```

| 상태 | 의미 | 캡처 가능 여부 |
|---|---|---|
| `none` | CA 없음 | 프록시 모드에서 HTTPS 불가. HTTP만 |
| `generated` | 키·인증서 생성됨. 신뢰 저장소에 없음 | HTTPS 연결이 클라이언트에서 거부됨 |
| `trusted` | OS/브라우저 신뢰 저장소에 등록됨 | 정상 |
| `expired` | 유효기간 경과 | 신규 리프 발급 중단. 재생성 필요 |

각 전이의 계약.

| 전이 | 트리거 | 권한 | 실패 시 |
|---|---|---|---|
| `generate` | 프록시 모드 최초 시작 시 자동 | 없음 | 캡처 시작 실패. 명확한 오류 |
| `install` | **사용자 명시 동작만**(5.1) | 관리자 인증 | `generated` 유지. 부분 등록 상태를 남기지 않는다 |
| `uninstall` | 설정 화면 1클릭 | 관리자 인증 | **아래 부분 실패 규칙** |
| `regenerate` | 만료 감지 또는 사용자 요청 | 없음 (등록은 별도) | — |

**부분 실패가 이 절의 핵심 문제다.** Windows 인증서 저장소와 브라우저별 저장소가
하나가 아니다(9.1의 Linux NSS DB 논의와 같은 구조). 등록·제거가 여러 저장소에
걸쳐 일어나므로 **일부만 성공할 수 있다.** 규칙:

- **제거는 각 저장소별 결과를 개별 보고한다.** "제거 완료"라는 단일 메시지를
  띄우지 않는다. 실패한 저장소는 이름과 **수동 제거 절차**를 함께 보여준다.
- **부분 제거 상태를 `none`으로 표시하지 않는다.** 하나라도 남아 있으면 상태는
  `trusted`이며 UI가 경고를 유지한다.
- 등록이 부분 성공하면 **성공한 것을 되돌린다.** 절반만 신뢰되는 CA는 "어떤 앱은
  되고 어떤 앱은 안 되는" 진단 불가능한 상태를 만든다.

"모든 저장소"는 플랫폼별 고정 집합이 아니라 **발견된 전체 집합**이다.

- Windows: 대상 Windows 인증서 저장소와 발견된 모든 Firefox NSS 프로필.
- macOS: 대상 Keychain trust domain과 발견된 모든 Firefox NSS 프로필.
- Linux: 시스템 trust anchor, `~/.pki/nssdb`, 그리고 발견된 Firefox/Chromium의
  모든 사용자·프로필별 NSS DB.
- 설치 시 저장소 identity와 CA fingerprint를 manifest에 기록한다. 제거 시 기록된
  집합과 현재 재열거한 집합의 합집합을 처리한 뒤 다시 전수 확인한다.
- 열거 불가·잠김·권한 실패 저장소는 성공으로 간주하지 않는다. 이름과 수동 제거
  절차를 보고하고 상태를 `trusted`로 유지한다.

**앱 제거 시 CA는 자동으로 사라지지 않는다.** 언인스톨러가 관리자 권한을 갖는다는
보장이 없기 때문이다(5.1). 따라서:

- 설정 화면과 최초 등록 화면 양쪽에 **"앱을 지워도 CA는 남는다"**를 명시한다.
- 언인스톨 시 CA 제거를 안내하고, 제거 절차 문서를 링크한다.
- **CA가 남아 있는 상태를 탐지할 수 있게** 한다. 재설치 시 기존 CA를 발견하면
  재사용하지 않고 **이전 CA를 제거한 뒤 새로 만든다** — 이전 설치의 개인키가
  어떻게 보관되었는지 알 수 없기 때문이다.

만료 처리는 자동 재생성이되 **재등록은 자동이 아니다**(5.1의 "자동 등록 금지"와
일관). 만료 후 첫 캡처 시도에서 `generated` 상태로 떨어지고, 사용자에게 재등록을
요청한다.

### 11.3 민감 데이터 리댁션

캡처 데이터에는 자격증명이 그대로 들어 있다. **기본값이 안전해야 한다.**

기본 리댁션 대상:

| 대상 | 규칙 |
|---|---|
| 요청 헤더 | `Authorization`, `Proxy-Authorization`, `Cookie`, `X-Api-Key`, `X-Auth-Token` 등 |
| 응답 헤더 | `Set-Cookie` |
| URL 쿼리 | `token`, `access_token`, `api_key`, `password`, `secret`, `signature`, `code` |
| 바디 | JSON 키 `password`, `token`, `secret`, `credit_card`, `ssn` 등 (설정 가능) |
| 패턴 | JWT(`eyJ…`), AWS 키(`AKIA…`), 카드번호(Luhn 검증 포함) |

평문 바디·URL fragment·비표준 헤더 값의 `key=value`도 구조화 JSON/폼/쿼리와
동일한 민감 키 집합(`token`, `*token`, `session`, `sessionId`, `code`, `auth`,
`*secret`, `*credential` 등)을 사용한다. 채널별로 더 좁은 denylist를 두지 않는다.

정책:

- **기본 켜짐.** 끄려면 명시적 설정 변경 + 경고 확인.
- 리댁션은 **저장 시점에** 적용한다. 메모리에만 원본이 존재했다가 사라진다.
  화면에만 가리고 파일에는 원본을 쓰면 무의미하다.
- 리댁션 여부와 규칙 목록을 `manifest.json`에 기록(6.4).
- **내보내기 시 재확인.** HAR을 남에게 주는 것이 가장 흔한 유출 경로다. 내보내기
  대화상자에서 리댁션 수준을 다시 묻는다.
- 사용자 정의 규칙(정규식)을 허용한다. 사내 고유 토큰 형식이 있기 때문이다.
- **가져오기 경로에도 적용한다.** 6.5.6절 참조. 이 항목이 개정에서 추가되었다 —
  원래 이 절은 "우리가 생성한 데이터"만 대상으로 했으나, **남이 만든 HAR이 오히려
  더 위험하다.** 사용자가 그 내용을 모르고, Chrome의 기본 소독은 쿼리스트링
  토큰·POST 바디·응답 바디·WebSocket 메시지를 **건드리지 않기** 때문이다 `[V]`.
- 리댁션은 **파싱된 필드 경로 기준**으로 구현한다. 원시 JSON 텍스트에 정규식을
  거는 방식(Cloudflare 새니타이저의 접근 `[V]`)은 취약하다. ArchScope는 이미
  모델을 파싱하므로 구조적 방식이 자연스럽고 우월하다.
- 분석 가치를 보존하는 기법을 쓰되 **기본값은 안전 쪽에 둔다.** 헤더 이름은
  보존한다. 값 보존 기법은 아래 11.3.1의 제약을 따른다.

#### 11.3.1 개정 — "분석 가치 보존" 기법의 안전성 재검토

2026-07-19 검토가 지적한 대로, 개정 전의 세 기법은 11.0의 기본 안전 기준을
충족하지 못한다.

| 개정 전 | 문제 | 개정 후 기본값 |
|---|---|---|
| JWT는 **서명만 제거** | 토큰 재사용은 막지만 payload의 이메일·subject·tenant·role·내부 식별자가 그대로 남는다. PII 유출 경로로는 그대로 열려 있다 | **payload 전체 삭제.** 헤더의 `alg`/`typ`과 만료 시각 정도만 남긴다 |
| 쿠키 값을 **일반 해시 접두**로 치환 | 무염 해시는 세션 간 동일성 누출과 사전대입에 취약하다. 쿠키 값 공간은 좁은 경우가 많다 | **완전 삭제.** 상관관계가 필요하면 **세션별 랜덤 키 HMAC**을 사용자가 명시 선택했을 때만 |
| 바디 blob을 **내용 해시로 파일명** | 세션 간 동일 내용 누출, 알려진 작은 값의 사전대입이 가능하다 | blob 식별자는 **랜덤 opaque ID**. 내용 해시가 필요하면 manifest 내부의 keyed integrity 값으로만 |

**상관관계 보존은 옵트인이다.** 기본 정책은 삭제이며, session-scoped HMAC은
사용자가 그 트레이드오프를 이해하고 선택했을 때만 활성화한다.

#### 11.3.2 manifest의 "무결성"이 뜻하는 것

`manifest.json`의 해시는 **우발적 손상 탐지**다. 서명이나 keyed MAC이 없으면
변조 방지가 아니다. 파일을 고칠 수 있는 사람은 해시도 다시 계산할 수 있다.
문서·UI 어디에서도 "무결성 보장"이라고 쓰지 않고 **"손상 탐지"**로 한정한다.

#### 11.3.3 프로세스 메타데이터도 민감하다

`commandLine`, `execPath`, `user`는 HTTP 밖의 민감정보를 담는다 — 명령줄 인자로
넘긴 토큰·비밀번호, 홈 디렉터리 경로에 박힌 실명, 프로젝트 경로에 박힌 고객사명이
실제로 흔하다. 필드별 등급을 둔다.

| 필드 | 기본 수집 | 기본 export |
|---|---|---|
| `pid`, `name` | 수집 | 포함 |
| `execPath` | 수집 | **경로 축약**(basename + 깊이 표시) |
| `commandLine` | 수집하되 **인자 값 리댁션 적용** | **제외** |
| `user` | 수집 | **제외** |

#### 11.3.4 사용자 정의 정규식의 자원 상한

사용자 규칙은 catastrophic backtracking과 과도한 스캔 비용을 만들 수 있다.
다음을 강제한다.

- 백트래킹이 없는 엔진(Go `regexp`, RE2 계열)만 사용한다
- 패턴 길이 상한, 규칙 수 상한
- 규칙당 실행 시간 상한과 초과 시 해당 규칙 비활성화 + finding
- 스캔 대상 크기 상한 (예: 바디 앞 1 MiB)

#### 11.3.5 가져온 HAR의 자원 상한

파싱을 시작하기 **전에** 구조를 제한한다 — 전체 파일 크기, entry 수, 필드 수,
개별 문자열/바디 크기, JSON 중첩 깊이, 압축 해제 후 크기 비율(압축 폭탄 방지).

또한 스키마 위반을 두 종류로 나눈다. 6.5절이 "strict failure를 warning으로
계속 진행"이라고만 적은 부분을 여기서 구체화한다.

- **fatal structural error** — `log.entries`가 배열이 아닌 등 구조 자체가 깨진
  경우. 중단한다.
- **recoverable dialect warning** — 벤더별 시맨틱 차이, 선택 필드 누락. 정규화하고
  finding을 남기며 계속한다.

### 11.4 하지 않는 것

- 인증서 피닝 우회 (5.3)
- 트래픽 변조·주입·리플레이 (1.3)
- 자격증명 추출·수집을 목적으로 하는 전용 기능
- 캡처 데이터의 외부 전송 — AI 해석에 넘길 때도 **리댁션 후 요약만**, 그리고
  기존 localhost 강제(`validateLocalURL`)를 유지
- 은닉·무표시 동작 (11.1)

### 11.5 보존

- 세션 디렉터리 기본 위치는 앱 데이터 경로. 사용자가 변경 가능.
- 기본 보존 기간을 두고(예: 7일) 초과 세션은 삭제를 **안내**한다. 자동 삭제는
  사용자 데이터를 말없이 지우는 것이라 기본값으로 하지 않는다.
- 세션 삭제는 인덱스·manifest·blob 참조와 파일을 모두 unlink하는 **논리적 삭제**다.
  SSD/파일시스템 매체 수준 secure erase를 보장하지 않으며 UI와 문서에서 "완전
  소거"라고 표현하지 않는다.
- 향후 리댁션 off를 지원한다면 해당 세션은 기본 자동 보존 대상에서 제외하고,
  명시 저장·짧은 만료·매체 잔존 경고를 요구한다. Phase 1 HAR import에는 redaction
  off 경로가 없다.
- 디스크 사용량을 설정 화면에 표시한다.

### 11.6 권한 요구 — 한자리에 모은 계약

9.3.5가 모드별 권한을 정리했고, 여기서는 **보안 관점의 계약**을 고정한다. 요점은
"무엇이 권한을 요구하는가"가 아니라 **"권한을 얻은 코드가 무엇을 할 수 있는가"**다.

| 동작 | 권한 | 지속성 | 승격된 코드의 범위 |
|---|---|---|---|
| HAR 가져오기·분석 | 없음 | — | — |
| 프록시 캡처 | 없음 | — | — |
| CA 등록/제거 | 관리자 인증 | **일회성** | 인증서 저장소 조작만 |
| 시스템 프록시 설정 | 관리자 인증(OS가 요구 시) | 일회성 | 프록시 설정 키만 |
| ETW/WFP/pcap 세션 | 관리자 | **세션 동안만** | 헬퍼 — 캡처 후 파일 쓰기만 |

세 가지 불변 규칙을 둔다.

- **상주 권한 프로세스를 설치하지 않는다.** 헬퍼는 세션 동안만 살아 있고(9.2),
  서비스/데몬으로 등록하지 않는다. 상주 승격 프로세스는 이 제품이 감당할 이유가
  없는 공격면이다.
- **헬퍼는 분석하지 않고 네트워크로 송신하지 않는다.** 캡처해서 파일에 쓰는 것이
  전부다. 파싱·리댁션·UI·AI 연동은 전부 비승격 프로세스에서 일어난다. 승격된
  코드의 표면적을 최소화하는 것이 목적이다.
- **IPC는 피어 자격을 검증한다**(9.2). 로컬 소켓/명명 파이프에 아무나 붙어서
  헬퍼에게 캡처를 시키면 권한 승격 취약점이 된다.

프록시 모드가 **권한을 전혀 요구하지 않는다**는 사실을 다시 강조한다. 11.0의
위협 모델에서 가장 위험한 행위자는 "CA 개인키 탈취자"인데, **프록시 모드의 위험은
CA 등록이라는 일회성 동작에 집중되어 있고 캡처 자체는 평범한 사용자 권한
소켓 서버**다. 이 구조가 3.7절 선택을 보안 관점에서도 지지한다.

### 11.7 보안 수용 기준 — 적대적 테스트 행렬

11.0의 위협 모델은 각 행위자에 대해 **깨졌는지 확인할 수단**이 있어야 완성된다.
Phase 0 게이트 3의 완료 조건은 이 행렬이 통과되는 것이다.

| ID | 행위자 | 테스트 | 통과 기준 |
|---|---|---|---|
| `SEC-1` | 로컬 사용자 | 기본 설정으로 인증 헤더·JWT·쿠키·카드번호를 포함한 트래픽 캡처 후 세션 파일 grep | **재사용 가능한 자격증명이 0건.** JWT payload 미포함 (11.3.1) |
| `SEC-2` | 로컬 사용자 | 쿼리스트링 토큰, POST 바디 비밀번호 | 리댁션됨. 원본이 blob에도 없음 |
| `SEC-3` | 내보낸 파일 수신자 | HAR export 후 동일 grep | `SEC-1`과 동일. `commandLine`·`user` 미포함 (11.3.3) |
| `SEC-4` | 악의적 HAR 제공자 | 압축 폭탄(1:1000 이상), 10만 entry, 깊이 1000 중첩, 100 MiB 단일 문자열 | 파싱 전 거부. 메모리 상한 내 종료 (11.3.5) |
| `SEC-5` | 악의적 HAR 제공자 | 구조 손상(`log.entries`가 객체) | fatal로 중단. 부분 결과를 정상처럼 표시하지 않음 |
| `SEC-6` | 악의적 HAR 제공자 | 비밀이 든 HAR 가져오기 | 가져오기 경로에도 리댁션 적용 (11.3, 6.5.6) |
| `SEC-7` | 사용자 정의 규칙 | catastrophic backtracking 유발 패턴 | RE2라 발생 불가. 시간 상한 초과 시 규칙 비활성화 + finding (11.3.4) |
| `SEC-8` | CA 개인키 탈취자 | 키 파일 위치·권한 확인, export 기능 탐색 | 키체인/DPAPI/Secret Service 저장. 폴백은 Windows owner-only DACL 또는 POSIX 파일 0600+상위 디렉터리 0700. **export 경로 없음** |
| `SEC-9` | CA 개인키 탈취자 | 두 머신에서 생성한 CA 비교 | 서로 다름. 빌드에 동봉된 CA 없음 |
| `SEC-10` | 크래시 덤프 수집 | 각 지원 OS에서 키 load 전 dump 정책 확인 후 캡처 중 강제 크래시·OS crash artifact 검색 | dump 비활성화/제외가 실패하면 HTTPS 평문 캡처 시작 거부. 생성된 crash artifact·로그에 CA 키·평문 바디 없음. 활성 디버거 메모리 읽기 방어는 비보장으로 명시 |
| `SEC-11` | 공용 워크스테이션 타 사용자 | 다른 계정으로 세션 디렉터리 접근 | OS 권한으로 차단 |
| `SEC-12` | — | CA 제거 후 기록된 저장소+현재 발견된 모든 NSS/OS trust store 전수 확인 | 모든 발견 저장소에서 제거. 열거/제거 부분 실패 시 명시 보고하고 `trusted` 유지 (11.2.1) |
| `SEC-13` | — | 피닝 앱 대상 캡처 | 자동 우회 없음. 진단된 원인 표시 (5.3) |
| `SEC-14` | — | 업스트림 인증서 검증 실패 호스트 | 트랜잭션 실패로 기록. 전역 무효화 경로 없음 (5.2) |
| `SEC-15` | — | 헬퍼 IPC에 비인가 프로세스 연결 시도 | 피어 자격 검증으로 거부 (11.6) |
| `SEC-16` | 로컬 사용자 | CLI/비대화형/detached 라이브 캡처 시작 시도 | 캡처 시작 명령·daemon 경로가 없어 거부. 라이브 캡처는 지속 인디케이터가 있는 Wails UI에서만 가능 (11.1) |
| `SEC-17` | 로컬 사용자 | host/process scope 밖 트래픽과 process 귀속 `unknown` 트래픽 발생 | 범위 밖 헤더·바디가 저장소 어디에도 없음. `unknown`은 명시 선택 없이는 저장하지 않음 (11.1.1) |

`SEC-1`과 `SEC-3`이 **11.0의 "기본 안전" 정의를 그대로 테스트로 옮긴 것**이며,
나머지는 그 정의를 지탱하는 개별 방어선이다. 이 행렬은 Phase 2 착수 조건이 아니라
**Phase 1(HAR 가져오기)부터 해당 항목이 적용된다** — `SEC-4`~`SEC-7`은 캡처 없이도
전부 유효하다.

## 12. 기존 분석과의 시너지

이 절이 "왜 별도 도구가 아니라 ArchScope인가"에 대한 최종 답이다.

### 12.1 HTTP 캡처 × CPU 프로파일

가장 가치가 큰 교차다. 두 데이터가 답하는 질문이 상보적이다.

| 데이터 | 답하는 것 |
|---|---|
| HTTP 캡처 | **기다린 시간** — 언제 무엇을 요청했고 얼마나 걸렸나 |
| CPU 프로파일 | **일한 시간** — 그때 CPU가 무엇을 하고 있었나 |

이를 겹치면:

```
시각 →   0ms        500ms       1000ms      1500ms
HTTP     ├─GET /api/config──┤
                       ├─GET /api/user─────────┤
CPU      ██ 부트스트랩      ░░░ 유휴    ████ JSON 파싱 + 리렌더
              ↑                  ↑              ↑
         병렬 없음          네트워크 대기    응답 후 처리 폭주
```

결론이 명확해진다. "느린 것은 서버가 아니라 **요청을 순차로 보낸 것**이고,
마지막 400ms는 네트워크가 아니라 **응답 처리 비용**이다."

구현 요건은 **시계 정렬**이다.

- 캡처와 프로파일 모두 벽시계(monotonic이 아닌) 기준 절대 시각을 기록한다.
- `.cpuprofile`의 `startTime`은 마이크로초이고 V8 시계 기준이라 벽시계와
  오프셋이 있다. 보정이 필요하다.
- 오프셋 추정이 불가능하면 **교차 뷰를 제공하지 않는다.** 어긋난 시각으로 그린
  상관관계는 틀린 결론을 만든다. 정렬 신뢰도가 낮으면 그렇다고 표시한다.

배치 위치는 기존 `IncidentTimelinePage`가 유력하다. 시간축 위에 이종 증거를
겹치는 것이 이미 그 페이지의 역할이기 때문이다.

### 12.2 HTTP 캡처 × Jennifer MSA

1.2에서 지적한 `applyNetworkGap`(`msa.go:195`)의 추정을 **실측으로 검증**한다.

```
Jennifer:  external_call_elapsed 340ms - callee_response 95ms = network gap 245ms  (추정)
캡처:      DNS 180ms + Connect 22ms + TLS 38ms + Send 1ms + Wait 96ms + Receive 3ms
                                                              ↑ callee_response와 일치
결론:      245ms의 정체는 네트워크 지연이 아니라 DNS 해석이다.
```

이 수준의 결론은 두 데이터 중 하나만으로는 나오지 않는다. 실측이 추정을 검증하고
분해까지 해 준다.

연결 키는 `(호스트, 시간 구간)`이다. 트레이스 ID 헤더(`traceparent`,
`X-B3-TraceId`)가 요청 헤더에 있으면 **정확 매칭**이 가능하다. 캡처가 헤더를
보므로 이는 자연스럽게 얻어진다.

### 12.3 HTTP 캡처 × access log

클라이언트 관점과 서버 관점을 같은 요청에 대해 나란히 놓는다.

| 관점 | 측정 |
|---|---|
| 캡처(클라이언트) | 요청 시작 ~ 응답 완료 = 1,240ms |
| access log(서버) | 처리 시간 = 85ms |
| 차이 | 1,155ms — 서버 밖에서 소요 |

access log 분석기가 이미 `responseTimeStats`, `urlStats`, `routeStats`를 만들고
있어(`apps/engine-native/internal/analyzers/accesslog/analyzer.go:159,193,266`) 경로 단위 조인이
가능하다. 매칭 키는 `(경로 템플릿, 시각 근사, 상태 코드)`이며, 서버가
`X-Request-Id`를 에코하면 정확 매칭이 된다.

### 12.4 리포트

`AnalysisResult`를 따르므로 기존 export가 그대로 동작한다(`cmd_report.go:51`).
"배포 전후 프론트엔드 요청 수가 12개에서 31개로 늘었다" 같은 회귀 비교는
가능하지만, **legacy `DiffPage`나 generic report diff가 이를 제공한다고 주장하지
않는다** — 그 경로는 summary 숫자와 finding 수만 비교한다(6.6). HTTP 세션
비교의 확정 계약은 12.4.1이다.

Diff 시 주의: 트랜잭션은 프로파일 샘플과 달리 **1:1 대응이 없다.** URL 정규화
(경로 파라미터 → 템플릿)를 거친 뒤 집계 단위로 비교해야 한다. 원시 URL로 비교하면
`/user/1`과 `/user/2`가 별개로 잡혀 diff가 무의미해진다.

#### 12.4.1 HTTP 전용 세션 Diff 계약 (T-575) — 확정 (2026-07-20)

**URL 템플릿화.** 결정적 규칙을 순서대로 적용한다: 숫자만 세그먼트 → `{id}`,
UUID 형식 → `{uuid}`, 16진 16자 이상 → `{hash}`, base64url 22자 이상 →
`{token}`, 이메일 형태 → `{email}`. 쿼리스트링은 **키 집합만**(정렬) 보존하고
값은 버린다 — 값은 비교 차원이 아니다. 비교 키는
`method + host + 템플릿화된 path`다. 규칙에는 버전을 붙인다
(`urlTemplateVersion`) — 두 세션의 버전이 다르면 비교 전에 낮은 쪽을 재계산한다.
세션당 템플릿 수는 트래픽 상위 K(기본 1,000)로 상한하고 초과분은 `{other}`
버킷으로 접은 뒤 diagnostic을 남긴다 — 6.6의 봉투 크기 안정성이 diff에도
그대로 적용된다.

**비교 차원.** `endpoint`(기본) / `host` / `process` 세 차원이다. `process`
차원은 **두 세션 모두** 프로세스 귀속이 있을 때만 활성화한다 — HAR 가져오기
세션은 `(HAR 가져오기)` 단일 의사 프로세스이므로(8.2) 이 차원을 비활성하고
사유를 표시한다. 차원별 합계는 교차 검증에 쓴다: Σ endpoint = Σ host = 총
트랜잭션 수.

**분모 정의.** 모든 비율 지표는 분자·분모를 결과에 명시적으로 담는다.
오류율 = 오류 트랜잭션 수 / 전체 트랜잭션 수(차원 단위별). 두 세션의 기록
길이가 다를 수 있으므로 절대 건수 델타에는 **분당 정규화율**
(건수 / 캡처 시간)을 병기하되, 시각을 신뢰할 수 없는 세션
(`HAR_TIMESTAMPS_DEGENERATE`)에서는 정규화율을 생략한다. 백분위수는 각 세션에서
독립 계산한 값을 나란히 놓는다 — 섞거나 평균 내지 않는다.

**시간창 정렬.** 정렬 등급을 결과에 기록한다: `aligned`(양쪽 절대 시각 신뢰
가능) / `duration_only`(상대 길이만 비교 가능) / `none`(degenerate 타임스탬프).
`none`이면 시계열 겹침 뷰를 제공하지 않는다 — "정렬 신뢰도가 낮으면 교차 뷰를
제공하지 않는다"는 12절 원칙의 diff판이다.

**결과 계약.** 파생 분석 계약 패턴(T-458/T-459)을 따르는 새 `AnalysisResult`
Type **`http_capture_diff`**를 정의한다.

| 봉투 필드 | 내용 | 상한 |
|---|---|---|
| `Summary` | 두 `CaptureSessionRef`, 총량 델타, `time_alignment`, `urlTemplateVersion` | 고정 크기 |
| `Tables` | `endpoints_changed`(&#124;델타&#124; 상위), `endpoints_added`, `endpoints_removed`, `hosts_changed`, `processes_changed`(가능 시) | 각 top-K (기본 50), K를 봉투에 명시 |
| `Metadata.Findings` | `HTTP_DIFF_TRAFFIC_SHIFT`, `HTTP_DIFF_ERROR_RATE_UP`, `HTTP_DIFF_LATENCY_REGRESSION`(p95 기준), `HTTP_DIFF_NEW_ERROR_ENDPOINT`, `HTTP_DIFF_ALIGNMENT_LOW`(info) | — |

상세 트랜잭션은 diff 봉투에 넣지 않는다. 드릴다운은 세션별
`FetchCaptureTransactions` cursor API(6.6)를 그대로 쓴다. report/export는
`http_capture_diff` 봉투만 소비하며 스토어를 재주사하지 않는다 — 파생 결과를
export 시점에 재계산하지 않는다는 T-460 원칙과 같다.

**Wails 내비게이션 (현재 코드 기준, 독립 통합).** 8.1의 "네트워크 분석" 그룹
계획을 그대로 따르되, **diff 진입점은 신규 NavKey가 아니다.** 진입은 두 곳이다:
(a) `HttpCapturePage`의 "세션 비교" 액션, (b) Analysis Workspace 비교
워크플로(T-426)에서 `http_capture` 결과 2건을 선택하면 generic diff가 아니라
HTTP 전용 diff로 라우팅. legacy `DiffPage`(profiler 4종)는 이 경로와 무관하며
`http_capture`를 받지 않는다. "브라우저 성능 분석" 메뉴의 존재를 전제하지
않는다는 8.1의 전제도 유지된다 — 이 계약은 현재 코드의 `Sidebar.tsx`/`App.tsx`
구조만 가정한다.

## 13. 구현 Phase

Codex 엔진·Claude UI 담당 범위와 그룹 단위 승인 순서는
[브라우저 프로파일·HTTP 캡처 구현 및 리뷰 게이트 계획](./BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md)에서
관리한다. 설계 Phase 번호는 유지하되 실제 착수는 그 문서의 `C-RG1 → H-RG1 →
H-RG2 → H-RG3 → H-RG4 → H-RG5 → X-RG1` 승인 순서를 따른다.

각 Phase는 **독립적으로 출시 가능한 가치**를 갖도록 나눈다.

### Phase 0 — 설계 확정 (구현 없음)

3.0의 아키텍처 전제 파괴를 수용할지 결정한다. 특히:

- 권한 승격 헬퍼를 제품에 넣을 것인가 (9.2)
- CA 신뢰 등록이라는 위험을 감수할 것인가 (11.2)
- 커버리지 부분성을 제품 특성으로 인정할 것인가 (10절)
- ~~Windows를 실시간 캡처의 1차 지원 플랫폼으로 둘 것인가.~~ **해결됨
  (2026-07-19).** Windows-first로 확정하고 Linux/macOS는 오프라인 입력 호환성과
  후속 live-capture capability로 분리한다 (9절).

**앞의 세 미해결 질문 중 하나라도 부정이면 Phase 1까지만 하고 멈춘다.** Phase 1은
그 세 가지 중 어느 것도 요구하지 않으므로 안전하게 착수 가능하다. 플랫폼 우선순위는
이미 Windows-first로 확정되었다.

#### Phase 0 게이트 (2026-07-19 검토 반영)

아래 게이트를 통과하기 전에는 해당 Phase의 구현을 시작하지 않는다.

| 순서 | 게이트 | 완료 조건 | 상태 |
|---|---|---|---|
| 1 | 조사 근거 고정 | 원장 형식·상태 값이 정의되고, **`open`인 행에 기대는 설계 결정이 없음** (부록 A) | **설계 확정 (2026-07-20).** `open` 행 해소는 잔여 작업 |
| 2 | Windows capability·fidelity 계약 | 2.1의 tier와 6.2.1의 fidelity가 모드별 행렬로 고정되고, 런타임 지원과 증거 호환성이 분리됨 (9.3, 9.4) | **설계 확정 (2026-07-20).** `미검증` 칸은 게이트 9가 채움 |
| 3 | 보안 위협 모델 | 11.0의 위협 모델, 11.2.1 CA 수명주기, 11.6 권한 계약, 11.7 적대적 테스트 행렬이 승인됨 | **설계 확정 (2026-07-20).** 테스트 실행은 Phase 1~2 |
| 4 | 트랜잭션·시간 모델 | 6.3.1~6.3.4의 경계·단위·상태 기계·H1/H2 골든 불변식이 승인됨 | **설계 확정 (2026-07-20)** |
| 5 | bounded result/store 계약 | 6.6의 봉투 경계와 7.6의 스토어 스키마·메모리 상한·축출·스필이 승인됨 | **설계 확정 (2026-07-20)** |
| 6 | Phase 1 HAR importer | 방언·malformed·security fixture 코퍼스로 정규화·진단 검증 | **코퍼스 구축됨 (2026-07-20, T-572) + importer MVP 구현됨 (2026-07-20).** `../projects-assets/test-data/har-fixtures/` 합성 우선 20종, manifest에 기대 diagnostic 고정. **잔여: importer 골든 테스트를 코퍼스 manifest에 연결**하고 실기 export 보강 |
| 7 | deterministic aggregate | 7.4.3의 순서 무관 property test 통과 | **완료 (2026-07-20, T-573).** 오프라인 start-order와 라이브 completion-order 집계 패리티 테스트 통과 |
| 8 | 프록시 build-vs-integrate spike | 아래 참조 | **결정 완료 (2026-07-20, T-574).** 옵션 3(자체 H1 MVP+H2 passthrough) 채택; PoC(`spikes/t574-mitm-proxy-poc`) self-test 통과; 리스크 레지스터 작성 |
| 9 | Windows 커버리지 proof | 10.4의 `CAP-1`~`CAP-6` 판정. 실패 시 절대 ratio 제거 | **1차 실측 완료 (2026-07-20, loopback smoke, 10.4.5).** TCP-owner CAP-1 100%(5/5)·CAP-2 0·CAP-4 탐지 → CAP-1~4 통과; ETW payload·손실 확인(귀속은 실 NIC 필요); WFP 허용경로 미확정. **잔여: ETW/WFP 실 NIC 재실측** |
| 10 | Windows Phase 2 캡처 | disk full/crash/event loss/CA failure/pinning/streaming E2E 통과 | 미착수 |
| 11 | HTTP 전용 Diff | URL 템플릿화와 시간 정렬 신뢰도를 포함한 비교가 검증됨 (6.6) | **설계 확정 (2026-07-20, T-575).** 12.4.1 — `http_capture_diff` 계약, 템플릿화 규칙 버전, 정렬 등급, top-K 상한, Workspace 라우팅. 구현·검증은 Phase 4 항목 31 |

게이트 1~5는 **설계 산출물**이고 6~11은 **실측·구현 산출물**이다. 2026-07-20
개정으로 앞의 다섯이 문서로 확정되었으며, 이로써 **Phase 1 착수를 막는 설계
게이트는 남아 있지 않다.** 게이트 1의 `open` 행과 게이트 2의 `미검증` 칸은 Phase 1
범위에서 어떤 결정도 지지하지 않으므로 착수를 막지 않는다 — 다만 Phase 2 진입
전에는 게이트 8~10이 선행되어야 한다.

##### 프록시 build-vs-integrate 결정

Go 표준 라이브러리로 CONNECT와 TLS 종단을 **시작**할 수 있다는 사실과, 장시간
안정적인 HTTP/1.1+HTTP/2 디버깅 프록시를 **완성**하는 일은 다르다. hop-by-hop
header, proxy authentication, ALPN, H2 flow control, trailer, 스트리밍, 취소,
1xx, WebSocket, upstream proxy, IPv6, PAC/`no_proxy`를 모두 다루면 Phase 2는
사실상 독립 제품이다. 1.5절의 경쟁 도구 조사는 이 영역의 성숙도를 보여주지만
정작 이 결정에는 쓰이지 않았다.

Phase 0에서 다음 네 선택지를 spike하고 결과를 기록한다.

1. 검증된 Go proxy 라이브러리를 in-process로 사용
2. mitmproxy 등 local subprocess를 adapter 뒤에 두고 사용
3. H1 시맨틱 프록시를 제한된 MVP로 구현하고 H2는 passthrough
4. 처음부터 자체 H1/H2 프록시를 구현

비교 축은 라이선스, 바이너리 크기, 보안 업데이트 경로, 프로세스 귀속 hook 가능
여부, fidelity 등급 도달 범위, 크래시 격리, 그리고 **Windows 설치·서명·업데이트**를
우선한다. 크로스 플랫폼 패키징은 후속 항목이다.

###### spike 결과와 결정 (T-574, 2026-07-20)

**후보 라이브러리 실사.** 옵션 1(Go 라이브러리)·옵션 2(subprocess)의 현실적 후보를
라이선스·유지·H2 기준으로 조사했다.

| 후보 | 라이선스 | H2 | 유지 상태 | 판정 |
|---|---|---|---|---|
| elazarl/goproxy | BSD-3-Clause | `h2.go` 존재, 성숙도 제한 | 활발 (v1.8.4, 2026-05) | 옵션 1의 유일한 실사용 후보 |
| google/martian | Apache-2.0 | h2 폴더 존재 | **2026-02 아카이브(종료)** | 탈락 — 유지 종료 |
| AdguardTeam/gomitmproxy | **GPL-3.0** | 없음 | 2021 이후 정체 | 탈락 — 카피레프트 + H2 없음 + 정체 |
| mitmproxy | MIT | H1/H2/H3 성숙 ◎ | 활발 | 옵션 2 후보 (Python 런타임 번들) |

GPL-3.0(gomitmproxy)는 배포되는 상용 데스크톱 앱에 부적합하고, martian은 아카이브되어
보안 업데이트 경로가 없다. 즉 **옵션 1은 사실상 goproxy 하나**로 좁혀지며, 그마저 H2
가로채기 충실도가 제한적이다.

**비교 매트릭스** (Windows 우선; ◎ 우수 / ○ 양호 / △ 미흡 / ✗ 결격)

| 기준 | 1. goproxy in-proc | 2. mitmproxy subprocess | 3. 자체 H1 MVP + H2 passthrough | 4. 자체 H1/H2 |
|---|---|---|---|---|
| 라이선스 | ○ BSD-3 | ○ MIT (번들 표기 부담) | ◎ 자체 | ◎ 자체 |
| 바이너리/배포 크기 | ◎ 단일 Go 바이너리 | ✗ Python+deps 수십~수백 MB | ◎ 단일 바이너리 | ◎ 단일 바이너리 |
| Windows 서명·패키징 | ◎ | ✗ 인터프리터+네이티브 deps 서명 부담 | ◎ | ◎ |
| 보안 업데이트 경로 | ○ 상류 의존 | △ Python+deps CVE 표면 큼 | ◎ 자체 릴리스 | ◎ 자체 릴리스 |
| PID 귀속 hook | ○ accept 래핑 | △ 별도 프로세스, IPC 상관 필요 | ◎ 자체 accept (T-571 검증) | ◎ 자체 accept |
| fidelity 도달 | ○ H1, H2 제한 | ◎ H1/H2/H3 | ○ H1 `decoded_wire`, H2 passthrough | ◎ H1/H2 |
| 크래시 격리 | △ in-proc panic 위험 | ◎ 별도 프로세스 | ○ in-proc(복구+bounded store) | ○ |
| 계약 정합성(§6.3/§7.6/§11) | △ 콜백에 계약을 얹어야 함 | △ IPC 경계로 재정의 | ◎ 계약 위에 직접 구현 | ◎ |
| 최초 출시 공수 | ○ 중 | ○ 중(번들 파이프라인) | ◎ 소~중 | ✗ 대 |

**결정: 옵션 3 (자체 H1 시맨틱 MVP + H2 passthrough)** 를 Phase 2 구현 경로로
채택한다. `Proxy`/`Interceptor` 인터페이스 뒤에 두어, 이후 옵션 4(자체 H2 가로채기)
계층을 재작성 없이 끼워 넣을 수 있게 설계한다.

근거:
- **단일 서명 Go 바이너리** — 진행 중인 서명·번들 게이트 작업과 정합. 옵션 2의 Python
  번들 서명·업데이트·공격표면 부담을 피한다.
- **계약 정합성** — §6.3 트랜잭션 상태 기계, §6.3.4 INV 불변식, §7.6 bounded store,
  §11 보안을 프록시 루프 위에 **직접** 구현한다. 라이브러리(옵션 1)나 IPC(옵션 2)에
  계약을 역으로 맞추지 않는다.
- **PID 귀속** — accept 루프를 소유하므로 T-571에서 100% 실측한 `GetExtendedTcpTable`
  역조회를 그대로 붙인다.
- **정직한 H2 열화** — H2는 반쪽 MITM 대신 passthrough(`unsupported`)로 통과시키고
  §9.3.2 fidelity 행렬에 기록한다. 깨진 가로채기보다 낫다.

기각:
- **옵션 1(goproxy)**: 유일한 실사용 라이선스 후보이나 H2 충실도 제한 + 우리 계약과
  임피던스 불일치. 참조 구현으로만 남긴다.
- **옵션 2(mitmproxy subprocess)**: fidelity·크래시 격리는 최고이나 Windows 서명·
  패키징·업데이트·공격표면에서 결격. **문서화된 폴백**으로 보류 — 자체 H2(옵션 4)
  전에 H2 가로채기가 하드 요구가 되면 재검토한다.
- **옵션 4(자체 H1/H2)**: 장기 목표. Phase 2 최초 출시엔 과대. 옵션 3의 인터페이스
  뒤에서 후속 승격한다.

**PoC 실증.** `spikes/t574-mitm-proxy-poc`에 옵션 3의 최소 프로토타입을 순수 stdlib로
구현하고 오프라인 self-test로 검증했다: MITM CA만 신뢰하는 클라이언트가 인프로세스
HTTPS origin에 대해 200 + 본문을 수신(= 실제 가로채기 성공), 프록시는 origin을 그 CA로
검증(= 업스트림 검증 유지), 캡처에 디코딩된 트랜잭션을 기록. ALPN 피킹 기반 h2-only
passthrough 판정도 테스트로 확인. `GOOS=windows` 크로스컴파일 통과. 서드파티 의존성
0 — 옵션 3의 "단일 바이너리" 전제를 실물로 확인했다.

**리스크 레지스터.**

| ID | 리스크 | 심각도 | 가능성 | 완화 |
|---|---|---|---|---|
| R-574-1 | H2/H3 사이트 급증으로 passthrough 비중이 커져 캡처 가치 하락 | 중 | 중 | 옵션 4를 인터페이스 뒤 후속 승격; passthrough 비율을 지표로 노출 |
| R-574-2 | in-proc 프록시 panic이 앱 전체 중단 | 중 | 저 | 연결별 recover + 향후 child-process 격리 옵션(옵션 2 폴백 경로 재사용) |
| R-574-3 | leaf 발급이 연결 임계경로에 있어 지연/CPU | 저 | 중 | 호스트별 캐시(PoC 구현), 사전 발급, ECDSA 사용 |
| R-574-4 | 자체 H1 파서의 엣지(1xx, trailer, 취소, keep-alive, WebSocket) 누락 | 중 | 중 | net/http 재사용 최대화 + fixture 코퍼스(T-572) 회귀 |
| R-574-5 | CA 개인키 노출 시 해당 머신 TLS 위조 | 고 | 저 | §11.2 머신 고유·비수출·1년 만료, OS 키체인/0600 |
| R-574-6 | upstream 검증을 실수로 끄면 보안 붕괴 | 고 | 저 | 검증 상시 ON을 계약으로 고정(PoC 준수), 코드리뷰/테스트 게이트 |
| R-574-7 | 옵션 2 폴백 필요 시 subprocess 번들 파이프라인 부재 | 저 | 중 | 인터페이스 경계 유지로 후속 도입 비용 최소화 |

### Phase 1 — HAR 가져오기와 분석 (캡처 없음)

캡처를 만들지 않고 **남이 만든 HAR을 분석**한다. DevTools·Charles·Proxyman
사용자가 즉시 쓸 수 있다. 1차 UI 검증 환경은 Windows이며, HAR을 생성한 OS는
제한하지 않는다. 즉 Linux/macOS에서 생성한 증거를 Windows ArchScope로 가져오는
제품 사용 패턴을 이 Phase에서 먼저 고정한다.

1. `apps/engine-native/internal/models/http_capture.go` — 6.2 데이터 모델
2. `apps/engine-native/internal/parsers/httpcapture` — HAR 파서. **7단 파이프라인(6.5.4)**
3. **`parsers/httpcapture/dialect.go` — 방언 정규화기.** Chrome/Firefox/Safari/
   Charles/Fiddler/Proxyman/Insomnia/generic. **이것을 파서와 분리된 1급 단계로
   만드는 것이 Phase 1의 핵심 설계 부채 방지책이다** (6.5.4)
4. `capture/redact` — 11.3 리댁션. **가져오기 경로에 먼저 적용**한다 (6.5.6)
5. `ingestion` 등록 — `http_capture` 소스 종류, **내용 기준** `.har` 탐지기 (6.5.9)
6. `apps/engine-native/internal/analyzers/httpcapture` — 집계, 6.7 + 6.5.5 진단, `AnalysisResult`
7. `capture/aggregate` — 시간 버킷 집계기. **일괄 경로만 먼저** (7.4.3)
8. `archscope-engine http-capture analyze --in x.har` CLI
9. `HttpCapturePage` + "네트워크 분석" 메뉴 그룹 (8.1)
10. 프로세스 트리(단일 `(HAR 가져오기)` 루트), 목록(8.3), 상세(8.4), 필터(8.5)
11. **전체 타임라인 + 브러시 선택 (8.7)** — 사후 모드에서 먼저 완성
12. **fixture 코퍼스 구축 (구현 착수 전).** 아래 참조

##### Phase 1 fixture 코퍼스

> **구축됨 (2026-07-20, T-572).**
> `../projects-assets/test-data/har-fixtures/`에 합성 우선 코퍼스 20종
> (방언 10 · 형식 이상 8 · 적대적 2)과 대형/과중첩 입력 생성기를 추가하고
> `ASSET_INDEX.md`에 등록했다. 디렉터리별 `manifest.json`이 provenance,
> 근거 claim(6.5.2/6.5.3의 `[V]` 항목), **기대 방언 판정·기대 diagnostic·
> 리댁션 assertion**을 골든 조건으로 고정한다. 한계도 기록되어 있다 —
> 전부 합성이므로 CreatorMatch 문자열은 실기 export 수집 전까지 잠정값이며,
> 실기 수집 우선순위(Chrome/Firefox 1순위, Fiddler는 T-574 스파이크와 병행)는
> 코퍼스 README의 교체 계획을 따른다. 아래 원문 표는 코퍼스 요구 사양으로
> 유지한다.

6.5.4의 정규화가 추측이 아니라 검증된 것이 되려면 실제 export 샘플이 필요하다.
구현 **전에** sanitized fixture를 `../projects-assets`에 추가하고
`ASSET_INDEX.md`에 등록한다.

| 구분 | 항목 |
|---|---|
| 방언 | Chrome(sanitized/민감정보 포함, H1·H2), Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic |
| 형식 이상 | BOM, content 누락, encoding 없는 base64, `bodySize=-1`, 동일 timestamp |
| 적대적 입력 | 초대형·과중첩·malformed JSON, 압축 폭탄, 쿼리/바디/WebSocket에 비밀 포함 |
| 생성 OS | Linux/macOS에서 생성해 Windows UI로 가져오는 경로를 포함한다 (Tier A) |

각 fixture에 **생성 제품·버전·명령, 원본 보관 여부, sanitization 방법, 기대
fidelity와 기대 diagnostic**을 기록한다. 이 기록이 없으면 fixture는 회귀 테스트가
아니라 그냥 파일이다.

**아키텍처 전제를 하나도 깨지 않는다.** 파일 입력, 배치 분석, 권한 없음. 순수
importer 추가다. 리스크가 낮고 UI 자산이 전부 만들어지므로 이후 Phase가 가벼워
진다.

**개정에서 Phase 1이 커졌다.** 방언 정규화(3)와 전체 타임라인(11)이 추가되었기
때문이다. 그럼에도 이 배치가 옳다고 보는 이유:

- 방언 정규화를 나중에 끼워 넣으면 파서 전체를 다시 짜야 한다. **정규화는 설계
  단계에서만 싸다.**
- 전체 타임라인(R11)은 1.5.4절 기준 **이 제품의 차별점**이다. 캡처 없이 HAR만
  으로도 성립하므로 Phase 1에서 검증하는 편이 낫다. 캡처를 기다릴 이유가 없다.
- 결과적으로 **Phase 1만으로도 "Fiddler보다 나은 타임라인 분석"이라는 주장이
  성립한다.** Phase 0의 세 질문이 부정으로 결론나도 이 가치는 남는다.

### Phase 2 — MITM 프록시 캡처

**Tier B·D**(2.1)와 R2·R3(`semantic` 등급)·R4·R5·R6를 만족시킨다. "R1 부분 만족"
같은 표현은 쓰지 않는다 — tier로 말한다.

Phase 2의 release gate는 **Windows 데스크톱**이다. Windows의 system proxy,
certificate store, process attribution, packaging을 먼저 end-to-end로 검증한다.
macOS/Linux proxy 실행은 같은 canonical session을 만들 수 있는지 확인하는
compatibility spike로 취급하되 1차 출시를 막지 않는다.

12. `apps/engine-native/internal/capture` 골격 — `Capturer` 인터페이스, 세션 수명주기
13. `capture/proxy` — HTTP/1.1 + HTTP/2, CONNECT, MITM, CA 관리
14. `capture/procmap` — 3플랫폼 PID 매핑 (4.4 프록시 최적화 경로)
15. `capture/store` — 세션 디렉터리(NDJSON append), blob, manifest
16. **`capture/stream` — 3단 버퍼 (7.4.2).** 기록/라이브 창/집계
17. **`aggregate`의 증분 경로 + 순서 무관성 보장 (7.5.2)**
18. `CaptureService` Wails 노출 + 3종 이벤트 배치·백프레셔 (7.4.4)
19. UI — 캡처 시작/정지, **프로세스별 트리 루트**
20. **실시간 UI — 자동 스크롤 규칙(8.6.1), 시각적 안정성(8.6.2), 진행 중
    트랜잭션(7.4.5), 백프레셔 표시(7.4.4)**
21. CA 설치/제거 화면 + 위험 고지 (11.2)
22. 최초 사용 고지 (11.1)

여기서 전제 파괴가 시작된다(스트림 입력, 장시간 세션, 데이터 생성). 다만
**권한 승격은 아직 없다.**

**Phase 1의 자산이 여기서 회수된다.** 타임라인·목록·상세·필터·집계기·분석기가
이미 있으므로 Phase 2가 만드는 것은 **캡처기와 스트리밍 계층뿐**이다. 7.5절의
통합 설계(R12)가 의도한 결과다 — 실시간 모드는 새 화면이 아니라 기존 화면에
데이터가 흘러드는 것이다.

**필수 검증 항목**: 7.5.2절의 동일성 테스트(같은 세션을 실시간 재생과 파일
일괄 로드 두 경로로 처리해 집계가 일치하는지)를 Phase 2 완료 조건에 넣는다.
두 모드가 다른 숫자를 내면 통합 설계가 무너진다.

### Phase 3 — 커버리지 정직성

프록시의 부분 커버리지를 관리 가능하게 만든다. **Phase 2와 짝이며 미루면 안
된다.** 이것 없는 Phase 2는 사용자를 오도한다.

23. **Windows 우선** 커버리지 evidence spike (10.1.2) — 성능 카운터가 제공하는 adapter
    scope와 ETW TCP/IP event의 process/flow 귀속 가능 범위를 구분해 실측한다.
    같은 scope의 분모를 증명하지 못하면 프로세스별 `bytesSystem` 비율은 제공하지
    않는다 (10.1)
24. 커버리지 지표 산출 + `Summary` 반영
25. 미관측 프로세스 감지 및 **설정 안내 UI** (10.2). **실시간 경고 포함**(8.6.3)
26. TLS 핸드셰이크 실패 **원인 진단** + 사용자 승인 기반 패스스루 (5.3). 자동
    패스스루는 철회되었다
27. `fidelity` 배지의 UI 전면 반영

### Phase 4 — 교차 분석

28. 캡처 세션 ↔ CPU 프로파일 시각 정렬 및 겹침 뷰 (12.1)
29. 캡처 ↔ Jennifer MSA `applyNetworkGap` 검증 뷰 (12.2)
30. 캡처 ↔ access log 클라이언트/서버 대조 (12.3)
31. 세션 Diff — 12.4.1 확정 계약(`http_capture_diff`, URL 템플릿화, 정렬 등급,
    top-K 상한) 구현. Workspace 비교 워크플로 라우팅 포함

**12절이 이 기능의 존재 이유이므로 Phase 4가 실질적 완성이다.** Phase 2~3만
하면 기존 프록시 도구의 열등한 복제품에 그친다.

### Phase 5 — 심화 캡처 (선택)

여기서 처음 권한 승격이 필요하다. Phase 3의 커버리지 지표가 "프록시로 부족하다"를
**데이터로 증명한 뒤에만** 착수한다.

32. 권한 헬퍼 골격 (9.2)
33. Windows ETW (WinINet/WinHTTP) — Windows-first 전략상 선택 백엔드 중 최우선
34. pcap 백엔드 + 대조 모드 (10.3)
35. Linux eBPF (uprobe SSL) — 서버 환경
36. 키 로그 + pcap 오프라인 복호화 (5.4)
37. WebSocket, gRPC (13절)

**Phase 5 착수 전 반드시 반영할 조사 결과**: 1.4.5절이 보여주듯 훅/드라이버
방식은 **안티바이러스와 상시 충돌한다.** HTTPAnalyzer는 Windows Defender·
F-Secure·360 Total Security 각각에 대한 우회 코드를 changelog에 남겼고, 오탐
회피 작업을 별도로 했다 `[V]`. 33~35번 항목의 일정 산정에 이 비용을 포함해야
하며, 이는 프록시를 1순위로 둔 3.7절의 판단을 추가로 뒷받침한다.

투명 프록시(3.3)와 macOS Network Extension은 **로드맵에 올리지 않는다.** 배포
파이프라인 자체를 바꾸는 요구이므로 별도 의사결정 문서가 필요하다.

## 14. 열린 질문

설계 검토에서 답이 필요한 항목들이다.

1. **Phase 0의 세 질문.** 특히 CA 신뢰 등록. 이것 없이는 HTTPS 평문이 불가능하고,
   HTTPS 평문 없이는 이 기능의 가치가 절반 이하다. 그러나 제품 위험은 이 기능
   전체에서 가장 크다.
2. ~~**라이브 뷰가 필수인가.**~~ **해결됨 (2026-07-18).** 필수로 확정했다.
   근거와 설계는 7.4.1절. 원래의 "캡처 중에는 카운터만" 안은 철회한다 — 캡처가
   대화형 행위라는 점, 커버리지 문제를 캡처 중에 드러내야 한다는 점(8.6.3),
   경쟁 도구 전부가 실시간 표시를 한다는 점 때문이다. 성능 부담은 7.4.2절의
   3단 버퍼로 해결하며, 이는 오히려 1.5.4절이 확인한 **유계 메모리라는 차별점**
   으로 전환된다.
3. **`.cpuprofile` 노트의 "브라우저 성능 분석"과 합칠 것인가.** 브라우저 CPU
   프로파일과 HTTP 캡처는 프론트엔드 담당자에게 사실상 한 세트다. 별도 그룹이
   맞는지, 사이드바가 그룹 4개로 비대해지지 않는지 검토가 필요하다.
4. **HTTP/2 지원 범위.** 프록시가 h2를 종단하려면 상당한 구현이 든다. 초기에
   ALPN에서 h2를 제외하고 **h1으로 다운그레이드**를 강요하는 안이 있으나, 이는
   3.2의 침습성을 크게 키우고 성능 특성을 왜곡한다. h2를 처음부터 지원하는 편이
   장기적으로 낫다고 보나 Phase 2 범위가 커진다.
5. **HTTP/3(QUIC).** 프록시에서 다루기 어렵고 pcap에서는 더 어렵다. 초기에는
   **QUIC 차단으로 h2 폴백을 유도**하는 방식이 실용적이나, 이는 명백한 트래픽
   개입이라 1.3의 비목표와 충돌한다. 차단 대신 "관측 불가"로 기록하는 안이
   원칙에 맞으나 커버리지가 줄어든다.
6. **`Metadata.Extra` 임계치.** 6.6의 5,000건은 임의 수치다. 실측 필요.
7. ~~**CLI 캡처 명령을 만들 것인가.**~~ **해결됨 (2026-07-21).** 라이브 캡처
   시작 명령은 제공하지 않는다. `archscope-engine`은 파일 가져오기·분석만 맡고,
   캡처 시작은 지속 인디케이터와 중지 제어가 있는 Wails UI로 제한한다(11.1,
   `SEC-16`).
8. **`.har` 외 포맷.** `.saz`(Fiddler), `.chls`(Charles)는 독자 포맷이고
   `.chlsj`(Charles JSON)는 읽을 만하다. Phase 1 범위에 넣을지.
   조사 참고: Proxyman이 `.chls`/`.chlsj` 가져오기를 **경쟁 도구 이탈 경로로
   의도적으로 제공**한다 `[V]`. 같은 전략이 가능하다. 또한 Fiddler Everywhere는
   **`.cap`(패킷 캡처) 가져오기**를 지원하는 유일한 도구인데 `[V]`, 이는 3.1절의
   pcap 백엔드와 별개로 "남이 뜬 pcap을 읽는" 저비용 경로가 될 수 있다.
9. ~~**HAR 방언 정규화의 검증 데이터를 어떻게 확보할 것인가.**~~ **해결됨
   (2026-07-19).** `projects-assets`에 sanitized HAR 코퍼스를 추가하고
   `ASSET_INDEX.md`에 등록하는 것으로 확정했다. 범위와 provenance 요건은 13절
   "Phase 1 fixture 코퍼스" 참조. 구현보다 **먼저** 만든다.
10. **CA 설치 절벽을 우회할 것인가.** Burp는 **사전 신뢰된 Chromium을 동봉**하고,
    Proxyman은 `simctl`로 iOS 시뮬레이터를 **원클릭 구성**한다 `[V, 1.5.5절]`.
    둘 다 11.2절의 가장 위험한 동작(시스템 CA 신뢰 등록)을 **회피**하는 접근이다.
    ArchScope가 브라우저를 동봉하는 것은 배포 크기 면에서 과하나, **"이 세션에서만
    신뢰하는 격리 브라우저 프로필"** 정도는 검토할 만하다. 11.2절의 위험을
    실질적으로 낮추는 유일한 아이디어다.
11. **R10의 "초당 500건"이 맞는 목표치인가.** 2절 주석대로 근거 없는 초기값이다.
    경쟁 도구가 무너지는 지점(Fiddler 문서상 200 세션, Proxyman 약 1,000건)을
    고려하면 낮게 잡아도 우위이나, 실측 후 확정해야 한다.

## 15. 요약

- 시스템 전역 HTTP 캡처는 전역성·프로세스 식별·HTTP 시맨틱의 교집합이 비어 있는
  자리를 채우며, ArchScope의 "추정을 관측으로 대체" 노선과 정확히 맞는다.
- 단일 캡처 방식으로 "모든 프로세스"를 만족하는 선택지는 **없다.** 다층 백엔드와
  통일된 데이터 모델(원칙 B)이 답이며, 커버리지 부분성은 감추지 말고
  **지표로 노출**해야 한다(10절).
- MITM 프록시를 1순위로 택하는 이유는 셋이다 — 일반 사용자 권한, HTTPS 평문,
  그리고 **경합 없는 정확한 PID 식별**(4.4).
- 캡처와 분석을 파일 경계로 분리하면(원칙 A) 기존 파일 기반 ingestion 계약이
  유지되고, HAR 가져오기만으로 Phase 1이 독립적 가치를 갖는다.
- 이 기능은 ArchScope의 아키텍처 전제 네 가지를 동시에 깬다(3.0). Phase 0에서
  이를 수용할지 결정하는 것이 선행 조건이다.
- 최종 가치는 캡처 자체가 아니라 **교차 분석**(12절)에 있다. Phase 4까지 가지
  않으면 기존 프록시 도구의 열등한 복제품에 그친다.
- 제품 실행은 **Windows-first**다. Linux/macOS는 우선 그 환경에서 생성된 HAR·
  프로파일·로그를 Windows UI에서 분석하는 입력 호환성을 보장하고, 해당 OS의
  실시간 캡처는 후속 capability로 확장한다.

2026-07-18 개정에서 추가된 결론:

- **조사 범위(2026-07-18 공식 문서·공개 UI)에서 "HTTP 시맨틱을 아는 시간축
  집계"를 가진 경쟁 도구를 확인하지 못했다**(1.5.4).
  Fiddler·Charles·mitmproxy·Proxyman·DevTools·Burp 어디에서도 캡처 전체에 대한
  요청률·오류율·처리량 시계열을 찾지 못했다. Wireshark만 시계열을 갖는데 그
  HTTP 통계에는 시간축이 없다. **이 대칭적 공백이 ArchScope의 자리이며**, 1.1절이
  지목한 공백(전역성 × 프로세스 × HTTP)과 별개의 축이다.
- 따라서 **전체 타임라인(R11)을 Phase 1로 앞당긴다.** HAR만으로 성립하므로
  캡처를 기다릴 이유가 없고, Phase 0의 세 질문이 부정으로 결론나도 이 가치는
  남는다.
- **라이브 뷰는 필수로 확정**했다(R10, 7.4.1). 개정 전의 유보를 철회한다.
  성능 부담은 3단 버퍼(7.4.2)로 해결하며, **유계 메모리는 그 자체로 방어 가능한
  포지션**이다 — 조사 범위에서 메모리 상한과 축출을 문서화한 도구를 확인하지
  못했다.
- **실시간과 사후는 별개 기능이 아니다**(R12, 7.5). 원칙 A가 이미 답을 갖고
  있었다 — 캡처기는 파일을 만들고 분석기는 파일을 읽으며, 실시간은 그 파일이
  자라는 중일 뿐이다. 두 경로가 같은 집계 결과를 내는지는 테스트로 강제한다.
- **HAR은 하나의 포맷이 아니라 여러 방언의 집합**이며 규격은 폐기 상태다(6.5).
  방언 수를 세지 않고 `DialectID` + feature matrix로 다룬다.
  Firefox는 `entry.time`에 TLS를 이중 계상하고, Chrome은 HTTP/2 응답에서
  `bodySize`를 못 준다. **방언 정규화를 파서와 분리된 1급 단계로 두는 것**이
  이 영역의 핵심 설계 결정이다.
- **가져온 HAR을 안전하다고 가정하지 않는다**(6.5.6). Chrome의 기본 소독은
  헤더와 쿠키만 건드리고 쿼리스트링 토큰·POST 바디·응답 바디·WebSocket 메시지를
  남긴다. 리댁션을 가져오기 경로에도 적용한다.
- HTTPAnalyzer는 **단종이 아니라 휴면**이며(1.4.1), 그 구조는 **커널 드라이버
  캡처 + 유저랜드 TLS 훅의 2계층**이었다(1.4.2). 이는 원칙 B(다층 백엔드)의
  실제 선례다. 동시에 그 방식이 치른 **안티바이러스 비용**(1.4.5)은 프록시를
  1순위로 둔 판단을 뒷받침한다.
- 기획 입력이 요청한 "사용자 리뷰 조사"는 **수행 불가로 결론**났다(1.4.6).
  실질적 리뷰 코퍼스가 존재하지 않는다. 본 설계는 없는 사회적 증거를 근거로
  삼지 않으며, UI 채택 항목은 전부 벤더 문서에서 확인한 기능 사실에 근거한다.

---

## 부록 A. 출처 원장 (source ledger)

2026-07-19 검토 지적 사항이다. **본문의 `[V]` 표시만으로는 다음 검토자가 주장을
검증하거나 갱신할 수 없다.** 가격·제품 버전·기능은 변하는 정보이므로 확인 경로가
문서에 남아야 한다.

### A.1 원장 형식

모든 `[V]` 주장은 아래 열을 갖는다. Phase 0 게이트 1의 완료 조건이 이 표를 빠짐
없이 채우는 것이다.

| 열 | 내용 |
|---|---|
| 주장 ID | 본문에서 참조할 수 있는 안정적 식별자 (예: `V-CHARLES-MEM`) |
| 출처 URL | 공식 문서 또는 소스 저장소 경로 |
| 문서/commit 버전 | 문서 버전 또는 소스 commit 해시 |
| 조회일 | 확인한 날짜 |
| 인용한 사실 | 그 출처에서 실제로 확인한 내용 |
| 설계 영향 | 이 사실이 바꾼 설계 결정과 해당 절 |

가격표와 제품 버전표에는 **"as of <날짜>"**를 항상 유지한다.

#### A.1.1 주장 ID 규칙

식별자는 `<표시>-<영역>-<주제>` 형태다.

| 부분 | 값 |
|---|---|
| 표시 | `V` 확인, `I` 추론, `N` 확인된 부정(존재하지 않음), `Q` 확인 실패 |
| 영역 | `HA`(HTTPAnalyzer), `COMP`(경쟁 도구), `WIN`, `LNX`, `MAC`, `SPEC`(규격), `PLAT`(ArchScope 의존 플랫폼) |
| 주제 | 짧은 대문자 슬러그 |

본문에서 `[V]`만 쓰던 자리는 재검토 시 `[V-HA-DRIVER]` 형태로 승격한다. 승격은
해당 절을 손댈 때 함께 처리하며, **원장에 행이 없는 ID는 본문에서 쓰지 않는다.**

#### A.1.2 상태 값

| 상태 | 의미 | 설계에서의 사용 |
|---|---|---|
| `fixed` | URL·조회일·인용 사실이 모두 채워짐 | 설계 결정의 근거로 사용 가능 |
| `partial` | 출처는 있으나 버전 고정 또는 인용 범위가 미확정 | 보조 근거로만. 단독으로 결정을 지지하지 못함 |
| `open` | 확인 경로 미확정 | **근거로 쓰지 않는다.** 해당 주장에 기대는 서술은 조사 범위 한정 표현으로 낮춘다 |

`partial`과 `open`은 Phase 0 게이트 1의 잔여 작업이며, 게이트 통과 조건은
"모든 행이 `fixed`"가 아니라 **"`open`인 행에 기대는 설계 결정이 하나도 없음"**
이다. 확인 불가능한 사실이 존재한다는 것 자체는 결함이 아니다 — 그것에 결정을
매다는 것이 결함이다.

### A.2 원장 — ArchScope 의존 플랫폼·규격

설계가 직접 기대는 1차 출처다. 이 영역의 `open`은 Phase 1 착수를 막는다.

| ID | 출처 | 버전 고정 | 조회일 | 인용한 사실 | 설계 영향 | 상태 |
|---|---|---|---|---|---|---|
| `V-SPEC-HAR-SSL` | [HAR 1.2 spec](http://www.softwareishard.com/blog/har-12-spec/) | 1.2 (버전 고정) | 2026-07-18 | `timings.ssl`은 `connect`에 **포함**된다(하위 호환 목적) | 6.3-4의 `ssl` 이중 계상 문제와 6.5.4 정규화의 근거 | fixed |
| `V-SPEC-HAR-CHROME` | [NetworkPanel.ts](https://chromium.googlesource.com/devtools/devtools-frontend/+/HEAD/front_end/panels/network/NetworkPanel.ts) | **commit 미고정 (HEAD 참조)** | 2026-07-18 | DevTools HAR export의 `entry.time` 산식이 `ssl`을 제외한다 | 6.3-4 방언 표의 Chrome 행 | partial |
| `V-SPEC-HAR-FIREFOX` | Firefox devtools netmonitor HAR builder | **경로·commit 미고정** | 2026-07-18 | `entry.time` 산식에 `ssl`이 포함되어 TLS가 이중 계상된다 | 6.3-4 방언 표의 Firefox 행. **이 행이 `open`으로 내려가면 Firefox 정규화 규칙은 "미검증 방언"으로 재분류한다** | partial |
| `V-SPEC-HAR-SANITIZE` | Chrome DevTools HAR export 소독 동작 | **commit 미고정** | 2026-07-18 | 기본 소독이 헤더·쿠키에 한정되고 쿼리스트링 토큰·POST 바디·응답 바디·WebSocket 메시지는 남는다 | 11.3의 가져오기 경로 리댁션, 6.5.6 | partial |
| `V-PLAT-GOHTTP` | [pkg.go.dev/net/http](https://pkg.go.dev/net/http) | Go 1.x 문서 (마이너 버전 미고정) | 2026-07-19 | 헤더 정규화·자동 해제(decompression) 동작이 원본 wire 바이트를 보존하지 않는다 | 6.2.1의 raw wire 헤더 보장 **철회** | partial |
| `V-PLAT-WAILS-EVT` | [Wails Events API](https://wails.io/docs/reference/runtime/events/) | Wails v2 문서 | 2026-07-19 | emit/listen 메커니즘이며 전달 보장·재연결·ack 프로토콜이 아니다 | 7.4.4의 push + 복구 가능한 pull 구조 | fixed |
| `V-PLAT-TDIGEST` | t-digest 알고리즘 원논문 | **판본 미고정** | 2026-07-18 | 병합 가능(mergeable) 분위수 스케치 | 7.4.3 버킷 2:1 병합의 전제 | partial |

### A.3 원장 — Windows 캡처·귀속

10절과 9절의 근거다. 이 영역은 **T-571의 proof-of-capability spike 이전까지 전부
`partial` 이하로 취급한다** — 문서가 존재한다는 사실과 실측이 일치한다는 사실은
다르다.

| ID | 출처 | 버전 고정 | 조회일 | 인용한 사실 | 설계 영향 | 상태 |
|---|---|---|---|---|---|---|
| `N-WIN-PERFCTR-PROC` | [net-sub-performance-counters](https://learn.microsoft.com/en-us/windows-server/networking/technologies/network-subsystem/net-sub-performance-counters) | Learn 문서 (개정일 미고정) | 2026-07-19 | byte 지표가 Network Interface/Adapter 단위다. **프로세스 단위 byte 카운터가 아니다** | 10.1.1 `bytesSystem` 전제 **철회** | fixed |
| `V-WIN-TCPTABLE` | [GetExtendedTcpTable](https://learn.microsoft.com/en-us/windows/win32/api/iphlpapi/nf-iphlpapi-getextendedtcptable) | Win32 API 문서 | 2026-07-19 | owner PID가 붙은 **endpoint table**이며 연결별 누적 byte counter가 아니다 | 4.2 귀속 수단, 10.1.1 | fixed |
| `V-WIN-ETW-TCPIP` | [etw/tcpip](https://learn.microsoft.com/en-us/windows/win32/etw/tcpip) | Win32 ETW 문서 | 2026-07-19 | TCP/IP 이벤트가 존재하며, **event header의 PID를 그대로 발신 프로세스로 쓰지 말라는 공식 주의**가 있다 | 10.1.1의 실측 선행 요구, T-571 spike 범위 | fixed |
| `Q-WIN-ETW-PAYLOAD` | Microsoft-Windows-Kernel-Network ETW (logman/tracerpt 실측, 10.4.5) | 빌드 26200.8737 실측 | 2026-07-20 | payload에 `Execution` PID·`sport`·`dport` 존재; 30초/약 500 tps에서 100,541 이벤트 중 **손실 0건**. 단 **loopback(127.x) 트래픽은 관측되지 않음** | payload·손실 확정으로 ratio 노출의 payload 전제는 충족; 귀속 정확도(CAP-1)는 실 NIC 트래픽으로 재측정 필요 | partial |
| `Q-WIN-WFP-ATTR` | `netsh wfp show netevents` (실측, 10.4.5) | 빌드 26200.8737 실측 | 2026-07-20 | 기본 구성 netevents 덤프에 **허용(ALLOW) 연결이 기록되지 않음** — 드롭 1건만 관측; 허용 경로 귀속 미확정 | WFP 귀속은 ALE connection audit 활성화가 선행되어야 함; 미설정 시 9.3 WFP 행은 계속 미확정 | partial |
| `V-WIN-NPCAP-INSTALL` | Npcap 배포 조건 | **라이선스 판본 미고정** | 2026-07-18 | 별도 설치가 필요하다 | 9.1의 Windows pcap `△` 등급, 배포 계획 | partial |

### A.4 원장 — HTTPAnalyzer 기능 레퍼런스

1.4절의 근거다. 이 영역은 **기능 요구사항의 출처**로만 쓰이며 어떤 런타임 보장도
지지하지 않는다.

| ID | 출처 | 버전 고정 | 조회일 | 인용한 사실 | 설계 영향 | 상태 |
|---|---|---|---|---|---|---|
| `V-HA-VERSION` | 벤더 사이트 + Wayback 스냅샷 | 스냅샷 3점 (2018-03 / 2019-07 / 2023-06) | 2026-07-18 | 최종 버전 `7.6.4.508`, 2018-03 스냅샷은 `7.6.1.481` | 1.4.1의 "2018-03~2019-07 사이 미갱신" 삼각측량 | fixed |
| `N-HA-EOL` | 벤더 사이트 전수 확인 | 2026-07-18 시점 사이트 | 2026-07-18 | **EOL 공지가 존재하지 않는다** | 0절의 "단종이 아니라 휴면" 표현 확정 | fixed |
| `V-HA-DRIVER` | 벤더 changelog | `v7.5.2.454` / `v7.6.1.481` / `v7.6.4.508` 항목 | 2026-07-18 | "proxy-less netfilter solution", "unloading the driver", "driver loading order" | 1.4.2의 커널 필터 드라이버 판정, 3.7 원칙 B의 선례 | fixed |
| `V-HA-TLSHOOK` | 벤더 기능 설명 | 2026-07-18 시점 페이지 | 2026-07-18 | "HTTPS is available if the application uses the Microsoft WININET API, Mozilla NSS API or OpenSSL API" | 1.4.2의 2계층 구조, TLS 평문의 라이브러리 종속성 | fixed |
| `V-HA-AV` | 벤더 changelog | 4개 항목 | 2026-07-18 | Windows Defender / F-Secure / 360 Total Security 호환 작업, 오탐 회피 변경 | 1.4.5의 AV 비용, 프록시 1순위 판단 보강 | fixed |
| `V-HA-UI` | 벤더 V7 도움말 파일 | V7 help | 2026-07-18 | `Timeline` 컬럼, 2단 그룹 그리드, `Stage` 컬럼, Summary Panel, Hints 3분류 | 1.4.3의 채택 항목 전체 | fixed |
| `N-HA-REGEX` | 벤더 V7 도움말 파일 | V7 help | 2026-07-18 | 검색에 정규식 지원이 없다 | 1.4.4 | fixed |
| `Q-HA-REVIEWS` | — | — | 2026-07-18 | 실질적 사용자 리뷰 코퍼스가 존재하지 않음(SO 4건, AlternativeTo 2014 이후 미갱신, 평점 사이트 봇 차단) | 1.4.6 — **사회적 증거를 근거로 쓰지 않는다는 결론 자체가 이 행의 산출물이다** | fixed(부정 확인) |
| `Q-HA-WIN11` | — | — | — | 최신 Windows에서의 실제 동작 여부 | **미확인.** 어떤 설계 결정도 여기에 기대지 않는다 | open |

### A.5 원장 — 경쟁 도구

1.5절의 근거다. **가격·버전은 변동 정보이므로 전 행에 `as of 2026-07-18`이
적용된다.**

| ID | 출처 | 버전 고정 | 조회일 | 인용한 사실 | 설계 영향 | 상태 |
|---|---|---|---|---|---|---|
| `V-COMP-MITM-LOCAL` | [Proxy Modes — Local Capture](https://docs.mitmproxy.org/stable/concepts/modes/) | stable 문서 | 2026-07-19 | local capture가 전체 장치 또는 **process name/PID를 대상으로 지정**할 수 있다 | 4.4 프록시 모드의 귀속 이점, `V-COMP-MITM-PID` 반박 | fixed |
| `V-COMP-CHARLES-PROC` | [Client Process Tool](https://www.charlesproxy.com/documentation/tools/client-process/) | 제품 문서 | 2026-07-19 | client process 조회 도구가 존재한다 | 1.5 비교표, 4.2 | fixed |
| `V-COMP-FIDDLER-CORE` | [FiddlerCoreStartupSettings](https://docs.telerik.com/fiddlercore/api/fiddler.fiddlercorestartupsettings) | API 문서 | 2026-07-19 | 시작 설정 옵션 구성 | 1.5.1 | fixed |
| `I-COMP-CHARLES-MEM` | 커뮤니티 보고 | — | 2026-07-18 | 기본 힙 256 MB 제약으로 장시간 세션이 실패한다 | 7.4.2가 유계 메모리를 차별점으로 삼은 동기. **추론이므로 설계를 지지하지 않고 동기만 설명한다** | partial |
| `I-COMP-FIDDLER-MEM` | 커뮤니티 보고 | — | 2026-07-18 | 장시간 세션에서 32 GB 사용 사례 | 위와 동일 | partial |
| `V-COMP-WIRESHARK-IO` | Wireshark 문서 | 판본 미고정 | 2026-07-18 | I/O Graphs가 1 ms~10 min 해상도를 제공한다 | 7.4.3 적응적 해상도 설계의 선례 | partial |
| `V-COMP-CF-SANITIZE` | Cloudflare HAR 새니타이저 | 판본 미고정 | 2026-07-18 | 원시 JSON 텍스트 정규식 방식 | 11.3의 "구조적 리댁션이 우월하다" 판단의 대조군 | partial |
| `Q-COMP-MITM-PID` | — | — | — | "mitmproxy는 수집 이후 PID 필드를 보존하지 않는다" | **철회 대상.** `V-COMP-MITM-LOCAL`이 대상 지정은 가능함을 보인다. 보존 여부는 코드 수준 주장이므로 **source commit과 검색 경로가 원장에 오르기 전까지 근거로 쓰지 않는다** | open |
| `Q-COMP-ABSOLUTE` | — | — | — | 1.5.4절의 두 절대 명제 | 조사 범위 한정 표현으로 하향 조정 완료(2026-07-19). 확인 경로는 미기재 | open |

### A.6 갱신 절차

원장은 한 번 채우고 끝나는 표가 아니다.

- **주기**: 가격·제품 버전 행(`A.5` 전체, `V-HA-VERSION`)은 6개월마다 재확인한다.
  재확인 시 조회일을 갱신하고, 값이 바뀌면 본문 표와 함께 고친다.
- **HEAD 참조 승격**: `V-SPEC-HAR-CHROME`처럼 `HEAD`를 가리키는 행은 Phase 1
  fixture 작업(T-572) 때 **당시 commit 해시로 고정**한다. fixture의 기대값이
  그 시점 동작에 묶이기 때문이다.
- **`open` 행 처리**: `open`인 채로 6개월이 지나면 해당 주장을 본문에서 삭제하거나
  추론 표시로 낮춘다. 무기한 "확인 예정"으로 남겨두지 않는다.
- **역방향 검사**: 본문 수정 시 새로 생긴 `[V]`에 원장 행이 없으면 리뷰에서
  반려한다. 이 규칙이 없으면 원장은 즉시 낡는다.
