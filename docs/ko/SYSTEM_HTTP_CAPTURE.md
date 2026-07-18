# 시스템 전체 HTTP 통신 캡처 및 분석 (설계 노트)

## 0. 이 문서의 위치와 전제

본 문서는 HTTPAnalyzer 계열 도구가 제공하던 **시스템 전역 HTTP/HTTPS 트래픽
캡처 및 프로세스별 분석** 기능을 ArchScope에 도입하기 위한 설계 노트다.

전제를 먼저 명시한다.

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
`AnalysisResult`라는 하나의 봉투에 담긴다(`internal/models/analysis_result.go:73`).

여기에 빠진 계층이 **애플리케이션과 네트워크 사이의 경계**다. 현재 ArchScope가
네트워크에 대해 아는 것은 전부 **간접 증거**다.

- access log는 **서버가 받은 뒤**의 기록이다. 클라이언트가 언제 보냈고 연결에
  얼마나 썼는지는 없다.
- Jennifer MSA의 `applyNetworkGap`(`internal/analyzers/jenniferprofile/msa.go:195`)은
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
   `validateLocalURL`로 localhost를 강제한다(`internal/aiinterpretation/runtime.go:33`).

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
> Fiddler(2종)·Charles·mitmproxy·Proxyman·DevTools·Burp 중 **처리량·RPS·동시성·
> 오류율의 시계열 차트를 가진 도구가 하나도 없다.** 최대치가 선택 스코프
> 워터폴이다.
>
> 제대로 된 시계열을 가진 유일한 도구는 **Wireshark(I/O Graphs)** 인데,
> **Wireshark의 HTTP 통계에는 시간축이 없다.** HTTP 지표를 시간축에 올리려면
> display filter로 직접 조립해야 한다.
>
> 공백이 대칭이다 — **Wireshark는 집계는 있으나 HTTP를 모르고, HTTP 도구들은
> HTTP는 알되 집계가 없다.** 이 교집합이 ArchScope의 자리다.

이는 1.1절이 지목한 공백(전역성 × 프로세스 × HTTP 시맨틱)과 **다른 축의 공백**
이며, 둘 다 비어 있다. 그리고 두 번째 공백이 ArchScope의 기존 강점과 더 잘
맞는다 — 시간축 위에 이종 증거를 올리는 것은 `IncidentTimelinePage`가 이미 하는
일이고, 12절의 교차 분석이 서 있는 토대다.

부수적으로 확인된 세 번째 공백:

> **메모리 상한과 축출이 있는 도구가 하나도 없다.** Charles는 JVM 기본
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
| R1 | 시스템 모든 프로세스의 HTTP/HTTPS 캡처 | 브라우저·CLI(`curl`)·JVM·Electron 각 1개 이상에서 트랜잭션이 잡힌다 |
| R2 | 프로세스별 트리 루트 분리 | 트리 최상위가 프로세스이고, 미식별 트랜잭션은 별도 `(미식별)` 루트로 격리된다 |
| R3 | 요청/응답 헤더 표시 | 원본 헤더 순서와 중복 헤더가 보존된다 |
| R4 | 요청/응답 바디 표시 | JSON/폼/텍스트는 포맷팅, 바이너리는 hex, 압축은 자동 해제 |
| R5 | 타이밍 분해 | DNS/연결/TLS/요청전송/TTFB/응답수신 6구간이 구분된다 |
| R6 | 상태 코드·메서드·URL·크기 | 목록에서 정렬·필터 가능 |
| R7 | 기존 아키텍처 정합 | 산출물이 `AnalysisResult` 계약을 따르고 Wails 서비스로 노출된다 |
| R8 | 크로스 플랫폼 | macOS/Linux/Windows에서 최소 한 가지 캡처 모드가 동작한다 |
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
| **권한 상승이 없다** | 런타임 코드에 `sudo`/setuid/launchd/드라이버 없음. 유일한 외부 프로세스는 JFR 도구(`internal/parsers/jfr/recording.go:119`) | **깨진다** — 모드에 따라 root/관리자 권한이 필요하다 |
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
가장 큰 정직한 한계이며, 12.1의 커버리지 리포트가 필요한 이유다.

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
   동시에 준다. 커버리지가 부분적이라는 약점은 12.1의 커버리지 리포트로
   *가시화*하여 관리한다. 감추지 않는 것이 핵심이다.
2. **pcap (A)** — 프록시가 놓친 트래픽의 존재를 증명하는 대조군. 평문 HTTP는
   완전 분석, HTTPS는 메타데이터(SNI·크기·타이밍)만.
3. **플랫폼 네이티브 (E)** — Windows ETW 우선. PID·URL 보강.
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

동작 규칙:

1. TLS 핸드셰이크 실패를 감지하면 해당 `(process, host)` 쌍을 **자동
   패스스루 목록**에 올린다.
2. 이후 그 조합은 복호화 없이 통과시키고 메타데이터(SNI, 바이트 수, 타이밍)만
   기록한다.
3. UI는 해당 트랜잭션을 `fidelity: "metadata_only"`로 표시하고 **이유를
   명시한다** — "이 앱은 인증서 피닝을 사용하여 내용을 볼 수 없습니다".

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
    Reused      bool   `json:"reused"`
}

type HTTPMessage struct {
    // 원본 순서와 중복을 보존해야 하므로 map이 아니라 슬라이스 (R3)
    Headers      []HeaderField `json:"headers"`
    BodySize     int64  `json:"bodySize"`      // 실제 전송(압축 상태)
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
    Timings  Timings     `json:"timings"`

    StartedAt string  `json:"startedAt"`
    TotalMs   float64 `json:"totalMs"`

    // "proxy_mitm" | "proxy_passthrough" | "pcap" | "pcap_keylog" | "ebpf" | "etw"
    CaptureMode string `json:"captureMode"`
    // "full" | "headers_only" | "metadata_only"
    Fidelity    string `json:"fidelity"`
    Error       string `json:"error,omitempty"`
}
```

`Fidelity`와 `CaptureMode`가 모든 트랜잭션에 붙는 것이 원칙 B의 구현이다. 서로
다른 백엔드가 만든 트랜잭션이 한 뷰에 섞이므로, 각 행이 **무엇을 얼마나
관측했는지**를 자기 자신이 말해야 한다.

### 6.3 타이밍 모델

W3C Resource Timing과 HAR `timings`를 따른다. 재발명하지 않는 이유는 HAR
상호운용(6.5)과 브라우저 데이터와의 비교 가능성 때문이다.

```go
type Timings struct {
    BlockedMs float64 `json:"blockedMs"` // 큐 대기 (커넥션 풀 등)
    DNSMs     float64 `json:"dnsMs"`     // 이름 해석
    ConnectMs float64 `json:"connectMs"` // TCP 핸드셰이크 (TLS 제외)
    TLSMs     float64 `json:"tlsMs"`     // TLS 핸드셰이크
    SendMs    float64 `json:"sendMs"`    // 요청 전송
    WaitMs    float64 `json:"waitMs"`    // TTFB — 서버 처리
    ReceiveMs float64 `json:"receiveMs"` // 응답 본문 수신
}
```

주의점 네 가지를 명시한다. 4항은 2026-07-18 조사에서 추가되었다.

1. **재사용 연결에서는 DNS/Connect/TLS가 0이다.** `Connection.Reused`가 참일 때
   0을 "측정 실패"로 오해하지 않도록 UI가 구분해야 한다.
2. **프록시 모드에서 이 값들은 프록시↔서버 구간이다.** 클라이언트↔프록시 구간은
   별도로 관측되며, 클라이언트가 체감한 총 시간은 두 구간의 합에 프록시 처리
   시간이 더해진 값이다. 두 관점을 모두 저장하고 UI에서 전환 가능하게 한다.
   합쳐서 하나의 숫자로 뭉개면 3.2의 침습성 문제가 데이터에 숨는다.
3. **HTTP/2 다중 스트림에서 `BlockedMs`의 의미가 달라진다.** 동일 연결의 다른
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
정규화 후:  TotalMs ≈ BlockedMs + DNSMs + ConnectMs + TLSMs + SendMs + WaitMs + ReceiveMs
```

ArchScope 내부 모델은 `ConnectMs`와 `TLSMs`를 **분리 보관**하므로 합에 둘 다
들어간다. 따라서 HAR 입력은 6.5.4절의 정규화를 **통과한 뒤에야** 이 불변식을
적용한다.

**정규화 전에 불변식을 검사하면 `CAPTURE_TIMING_INCONSISTENT`가 모든 Firefox
HAR에서 오탐한다.** 진단 finding(`AddFinding`, `analysis_result.go:135`)은
정규화 이후 단계에만 건다. 정규화 자체가 실패한 경우는 별도 코드
`HAR_TIMING_DIALECT_UNKNOWN`으로 구분한다 — 둘은 원인도 대응도 다르다.

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
├─ bodies/             blob (내용 해시 기준 파일명 — 중복 제거)
└─ manifest.json       무결성 해시, 스키마 버전, 리댁션 정책 기록
```

`manifest.json`에 리댁션 정책을 기록하는 것이 중요하다. 나중에 이 캡처를 여는
사람이 **무엇이 지워졌는지** 알아야 결론을 신뢰할 수 있다.

### 6.5 HAR 상호운용 — 사후 분석 경로

R9의 설계다. HAR은 이 영역의 사실상 표준이며 원칙 A에 따라 **1급 입출력
포맷으로 삼는다.** 캡처 기능 없이도 이 경로만으로 제품 가치가 성립한다 —
Phase 1의 근거다.

그러나 조사 결과 **HAR은 하나의 포맷이 아니라 네 개의 방언**이며, 규격 자체가
관리되지 않는다. 이 절은 그 현실을 설계에 반영한다.

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

**분석 가치를 보존하는 리댁션 기법** — 11.3절 규칙에 추가한다.

| 기법 | 효과 | 출처 |
|---|---|---|
| **헤더 이름은 남기고 값만 가린다** | 헤더 집합이 보존되어 진단 신호 대부분이 살아남는다 | 공통 |
| **JWT 서명만 제거** — `header.payload.redacted` | 토큰은 무효화되나 **클레임은 계속 읽힌다** | Cloudflare·Okta가 독립적으로 수렴 `[V]` |
| **쿠키 값을 해시 접두로 치환** | 엔트리 간 상관관계 보존, 재사용 불가 | Okta `[V]` |
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

### 6.6 `AnalysisResult` 매핑

R7 준수. 기존 봉투를 그대로 쓴다.

| 봉투 필드 | 내용 |
|---|---|
| `Type` | `"http_capture"` |
| `SourceFiles` | 세션 디렉터리 또는 `.har` 경로 |
| `Summary` | 총 트랜잭션/프로세스/호스트 수, 총 전송량, 오류율, 상태 코드 분포, 소요시간 백분위수, **커버리지 지표**(12.1) |
| `Tables` | `transactions`, `processes`, `hosts`, `slowest`, `errors` |
| `Series` | 시간축 요청률/전송량/오류율, 프로세스별 스택 |
| `Charts` | 상태 코드 분포, 호스트별 시간 기여, 타이밍 구간 분해 |
| `Metadata.Findings` | 진단 결과(6.7) |
| `Metadata.Extra` | `captureSession` 원본 트리 — `MarshalJSON`(L166)이 평탄화 |

`Metadata.Extra`에 트리 전체를 넣는 선택은 검토가 필요하다. 대용량 세션에서
`AnalysisResult` JSON이 거대해진다. **트랜잭션 수가 임계치(예: 5,000)를 넘으면
`Extra`에는 요약 트리만 넣고 상세는 세션 디렉터리에서 지연 로딩**하도록 한다.
전용 Wails 메서드(`FetchCaptureTransactions(sessionID, filter, page)`)를 둔다.

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
`internal/parsers`, 분석기는 `internal/analyzers` 아래일 것을 강제하므로 이를
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

`internal/capture`를 파서/분석기 밖에 두는 것이 의도적이다. 캡처는 **증거를
생성하는 활동**이지 증거를 읽는 활동이 아니다. 원칙 A의 코드 상 표현이다.

### 7.2 `Capturer` 인터페이스

```go
type Capturer interface {
    Name() string
    Available() (bool, string)              // 사용 가능 여부와 불가 사유
    RequiredPrivilege() Privilege           // none | admin | root
    Start(ctx context.Context, cfg Config, sink Sink) error
    Stop() error
    Stats() Stats                           // 커버리지 지표 (12.1)
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

### 7.3 Wails 서비스 노출

기존 `EngineService`/`ProfilerService` 관례(`cmd/archscope-app/main.go:57-58`)를
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
    ByProcess map[ProcessKey]int
}

type Aggregator struct {
    resolution time.Duration    // 적응적 — 아래
    buckets    []Bucket
}
```

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
| `capture:transactions` | 100ms 배치 | 신규 트랜잭션 메타데이터 배열 | **드롭 가능** — 카운터로 대체 |
| `capture:aggregate` | 1s | 최근 버킷 델타 | **드롭 불가** — 작고 상수 크기 |
| `capture:stats` | 1s | 총계·프로세스 수·커버리지 지표 | 드롭 불가 |

**`capture:transactions`만 드롭 대상이라는 점이 중요하다.** 목록은 밀려도
사용자가 스크롤하면 파일에서 읽어 복구된다. 반면 집계와 통계가 끊기면 화면이
멈춘 것처럼 보인다 — 크기가 작으니 드롭할 이유도 없다.

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

**요건: `Add`는 순서 무관(order-independent)해야 한다.** 실시간에서는 트랜잭션이
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

## 8. UI 설계

### 8.1 메뉴 위치

`.cpuprofile` 설계 노트(7.3절)가 "브라우저 성능 분석" 그룹을 신설했다. 같은
논리로 **"네트워크 분석"** 그룹을 신설한다.

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
| **요청** | 헤더 원본 순서 그대로(중복 포함), 쿼리 파라미터 분해, 바디 뷰어 |
| **응답** | 헤더, 바디 뷰어, 압축 해제 전/후 크기 병기 |
| **타이밍** | 6구간 수평 막대 + 수치. 재사용 연결은 "연결 재사용"으로 명시 |
| **연결/TLS** | 5-tuple, ALPN, TLS 버전/암호군, **서버 인증서 체인**(5.2) |
| **원본** | 재구성된 원본 바이트. 복사/저장 가능 |

바디 뷰어는 콘텐츠 타입에 따라: JSON(접이식 트리 + 검색), XML/HTML(구문 강조),
폼(키-값 표), 이미지(미리보기), 그 외(hex + ASCII). 압축은 자동 해제하되
**원본 인코딩을 표시**한다 — `Content-Encoding` 문제 자체가 디버깅 대상인 경우가
많다.

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
| `fidelity` | full / headers_only / metadata_only |
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

## 9. 크로스 플랫폼

### 9.1 모드 가용성 행렬

| 모드 | macOS | Linux | Windows |
|---|---|---|---|
| MITM 프록시 | ◎ 사용자 권한. 시스템 프록시 설정 시 관리자 인증 | ◎ 환경변수/GNOME 설정. 배포판별 편차 | ◎ WinINet 프록시 설정 |
| CA 신뢰 등록 | 키체인 (관리자 인증) | 배포판마다 다름 — `update-ca-certificates` / `trust anchor` / NSS DB 별도 | 인증서 저장소 (관리자) |
| pcap | ◎ BPF 장치 권한 필요 | ◎ `cap_net_raw` | △ **Npcap 별도 설치** |
| eBPF | ✗ | ◎ 커널 5.8+ / BTF | ✗ |
| ETW | ✗ | ✗ | ◎ 관리자 |
| Network Extension | △ 엔티틀먼트 필요 | ✗ | ✗ |
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

`internal/parsers/jfr/recording.go:119`의 `exec.LookPath` + 외부 도구 호출이
외부 실행 파일을 다루는 기존 선례이나, **권한 승격 선례는 없다.** 헬퍼 서명·배포·
검증은 신규 과제이며 `docs/ko/PACKAGING_PLAN.md`의 개정이 필요하다.

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
| `bytesObserved` vs `bytesSystem` | 관측 바이트 대 시스템 총 네트워크 바이트 |

마지막 지표가 핵심 아이디어다. OS는 프로세스별 네트워크 바이트 카운터를 값싸게
제공한다(macOS `nettop` 계열 API, Linux `/proc/<pid>/net/dev`·cgroup, Windows
성능 카운터). 이를 캡처 관측량과 비교하면:

> "이 프로세스는 4.2 MB를 주고받았는데 캡처에는 0 B가 잡혔다 —
> **프록시를 우회하고 있다.**"

라는 결론을 **캡처 없이도** 낼 수 있다. 이는 pcap 대조군(3.7의 2순위)보다 훨씬
싸고 권한도 낮다. 사용자에게는 "이 프로세스를 보려면 이렇게 설정하세요"라는
실행 가능한 안내로 이어진다.

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

## 11. 보안 및 프라이버시

이 기능은 ArchScope에서 **가장 위험한 기능**이다. 설계 단계에서 방어선을
확정한다.

### 11.1 이 도구가 무엇인지 정직하게

시스템 전역 HTTP 캡처는 **도청 도구**다. 기술적으로 중립이라는 말로 넘어갈 수
없다. 방어선:

- **로컬 머신 한정.** 원격 캡처, 원격 배포, 원격 조회 기능을 만들지 않는다.
- **은닉 불가.** 캡처 중에는 항상 화면에 표시가 있고, 백그라운드 무표시 캡처
  모드를 제공하지 않는다. 트레이 아이콘 상태 변경을 포함한다.
- **자동 시작 없음.** 앱 실행만으로 캡처가 시작되지 않는다. 명시적 시작만.
- **최초 사용 시 고지.** 법적·윤리적 고지를 1회 표시하고 확인을 받는다. 타인의
  통신을 동의 없이 캡처하는 것은 다수 관할권에서 위법일 수 있다는 내용.
- **CLI에도 동일 적용.** `archscope-engine`에 캡처 시작 명령을 추가한다면 동일한
  고지와 명시적 플래그를 요구한다. GUI만 보호하고 CLI를 열어두면 방어선이 없는
  것과 같다.

### 11.2 CA 개인키

5.1에서 언급한 최대 위험이다.

| 항목 | 처리 |
|---|---|
| 저장 | macOS 키체인 / Windows DPAPI / Linux Secret Service. 폴백 시 0600 파일 |
| 메모리 | 사용 후 즉시 폐기. 로그·크래시 덤프에 절대 포함 금지 |
| 내보내기 | **제공하지 않는다.** 백업 기능도 없다 (재생성이 정답) |
| 머신 고유성 | 머신마다 새로 생성. 빌드에 CA를 동봉하지 않는다 |
| 만료 | 1년. 만료 시 자동 재생성 + 재등록 안내 |
| 제거 | 설정 화면에서 1클릭. 앱 제거 시 안내 |

**빌드에 CA를 동봉하지 않는다**는 항목이 특히 중요하다. 공용 CA 개인키가 배포되면
그 개인키로 전 세계 사용자의 TLS를 위조할 수 있다. 과거 여러 제품이 실제로 저지른
사고다.

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
- **분석 가치를 보존하는 기법을 우선한다** — 헤더 이름 보존, JWT 서명만 제거,
  쿠키 값 해시 치환. 6.5.6절 표 참조. 무차별 `****`는 데이터를 쓸모없게 만든다.

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
- 세션 삭제는 blob 포함 완전 삭제.
- 디스크 사용량을 설정 화면에 표시한다.

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
있어(`internal/analyzers/accesslog/analyzer.go:159,193,266`) 경로 단위 조인이
가능하다. 매칭 키는 `(경로 템플릿, 시각 근사, 상태 코드)`이며, 서버가
`X-Request-Id`를 에코하면 정확 매칭이 된다.

### 12.4 리포트

`AnalysisResult`를 따르므로 기존 export가 그대로 동작한다(`cmd_report.go:51`).
`DiffPage`로 두 세션 비교도 가능하다 — "배포 전후 프론트엔드 요청 수가 12개에서
31개로 늘었다" 같은 회귀를 잡을 수 있다.

Diff 시 주의: 트랜잭션은 프로파일 샘플과 달리 **1:1 대응이 없다.** URL 정규화
(경로 파라미터 → 템플릿)를 거친 뒤 집계 단위로 비교해야 한다. 원시 URL로 비교하면
`/user/1`과 `/user/2`가 별개로 잡혀 diff가 무의미해진다.

## 13. 구현 Phase

각 Phase는 **독립적으로 출시 가능한 가치**를 갖도록 나눈다.

### Phase 0 — 설계 확정 (구현 없음)

3.0의 아키텍처 전제 파괴를 수용할지 결정한다. 특히:

- 권한 승격 헬퍼를 제품에 넣을 것인가 (9.2)
- CA 신뢰 등록이라는 위험을 감수할 것인가 (11.2)
- 커버리지 부분성을 제품 특성으로 인정할 것인가 (10절)

**세 질문 중 하나라도 부정이면 Phase 1까지만 하고 멈춘다.** Phase 1은 이 세
가지 중 어느 것도 요구하지 않으므로 안전하게 착수 가능하다.

### Phase 1 — HAR 가져오기와 분석 (캡처 없음)

캡처를 만들지 않고 **남이 만든 HAR을 분석**한다. DevTools·Charles·Proxyman
사용자가 즉시 쓸 수 있다.

1. `internal/models/http_capture.go` — 6.2 데이터 모델
2. `internal/parsers/httpcapture` — HAR 파서. **7단 파이프라인(6.5.4)**
3. **`parsers/httpcapture/dialect.go` — 방언 정규화기.** Chrome/Firefox/Safari/
   Charles/Fiddler/Proxyman/Insomnia/generic. **이것을 파서와 분리된 1급 단계로
   만드는 것이 Phase 1의 핵심 설계 부채 방지책이다** (6.5.4)
4. `capture/redact` — 11.3 리댁션. **가져오기 경로에 먼저 적용**한다 (6.5.6)
5. `ingestion` 등록 — `http_capture` 소스 종류, **내용 기준** `.har` 탐지기 (6.5.9)
6. `internal/analyzers/httpcapture` — 집계, 6.7 + 6.5.5 진단, `AnalysisResult`
7. `capture/aggregate` — 시간 버킷 집계기. **일괄 경로만 먼저** (7.4.3)
8. `archscope-engine http-capture analyze --in x.har` CLI
9. `HttpCapturePage` + "네트워크 분석" 메뉴 그룹 (8.1)
10. 프로세스 트리(단일 `(HAR 가져오기)` 루트), 목록(8.3), 상세(8.4), 필터(8.5)
11. **전체 타임라인 + 브러시 선택 (8.7)** — 사후 모드에서 먼저 완성

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

R1(부분)·R2·R3·R4·R5·R6를 실제로 만족시킨다.

12. `internal/capture` 골격 — `Capturer` 인터페이스, 세션 수명주기
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

23. 프로세스별 시스템 바이트 카운터 수집 (10.1)
24. 커버리지 지표 산출 + `Summary` 반영
25. 미관측 프로세스 감지 및 **설정 안내 UI** (10.2). **실시간 경고 포함**(8.6.3)
26. 피닝 자동 감지 및 패스스루 (5.3)
27. `fidelity` 배지의 UI 전면 반영

### Phase 4 — 교차 분석

28. 캡처 세션 ↔ CPU 프로파일 시각 정렬 및 겹침 뷰 (12.1)
29. 캡처 ↔ Jennifer MSA `applyNetworkGap` 검증 뷰 (12.2)
30. 캡처 ↔ access log 클라이언트/서버 대조 (12.3)
31. 세션 Diff — URL 정규화 포함 (12.4)

**12절이 이 기능의 존재 이유이므로 Phase 4가 실질적 완성이다.** Phase 2~3만
하면 기존 프록시 도구의 열등한 복제품에 그친다.

### Phase 5 — 심화 캡처 (선택)

여기서 처음 권한 승격이 필요하다. Phase 3의 커버리지 지표가 "프록시로 부족하다"를
**데이터로 증명한 뒤에만** 착수한다.

32. 권한 헬퍼 골격 (9.2)
33. pcap 백엔드 + 대조 모드 (10.3)
34. Windows ETW (WinINet/WinHTTP) — 비용 대비 효과 최상
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
7. **CLI 캡처 명령을 만들 것인가.** 11.1의 방어선 유지가 CLI에서 더 어렵다.
   자동화 가치와 오남용 위험을 견줘야 한다.
8. **`.har` 외 포맷.** `.saz`(Fiddler), `.chls`(Charles)는 독자 포맷이고
   `.chlsj`(Charles JSON)는 읽을 만하다. Phase 1 범위에 넣을지.
   조사 참고: Proxyman이 `.chls`/`.chlsj` 가져오기를 **경쟁 도구 이탈 경로로
   의도적으로 제공**한다 `[V]`. 같은 전략이 가능하다. 또한 Fiddler Everywhere는
   **`.cap`(패킷 캡처) 가져오기**를 지원하는 유일한 도구인데 `[V]`, 이는 3.1절의
   pcap 백엔드와 별개로 "남이 뜬 pcap을 읽는" 저비용 경로가 될 수 있다.
9. **HAR 방언 정규화의 검증 데이터를 어떻게 확보할 것인가.** 6.5.2절의 차이는
   조사로 확인했으나, ArchScope 정규화기가 실제로 올바른지 검증하려면 **7종
   도구의 실제 export 샘플**이 필요하다. `projects-assets`의 sanitized 샘플
   체계에 HAR 코퍼스를 추가하는 것이 자연스럽다. 이것 없이는 6.5.4절이
   추측 기반이 된다.
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

2026-07-18 개정에서 추가된 결론:

- **경쟁 도구 전 범주에서 "HTTP 시맨틱을 아는 시간축 집계"가 비어 있다**(1.5.4).
  Fiddler·Charles·mitmproxy·Proxyman·DevTools·Burp 어느 것도 캡처 전체에 대한
  요청률·오류율·처리량 시계열을 갖지 않는다. Wireshark만 시계열을 갖는데 그
  HTTP 통계에는 시간축이 없다. **이 대칭적 공백이 ArchScope의 자리이며**, 1.1절이
  지목한 공백(전역성 × 프로세스 × HTTP)과 별개의 축이다.
- 따라서 **전체 타임라인(R11)을 Phase 1로 앞당긴다.** HAR만으로 성립하므로
  캡처를 기다릴 이유가 없고, Phase 0의 세 질문이 부정으로 결론나도 이 가치는
  남는다.
- **라이브 뷰는 필수로 확정**했다(R10, 7.4.1). 개정 전의 유보를 철회한다.
  성능 부담은 3단 버퍼(7.4.2)로 해결하며, **유계 메모리는 그 자체로 방어 가능한
  포지션**이다 — 조사 대상 도구 중 메모리 상한과 축출을 가진 것이 하나도 없다.
- **실시간과 사후는 별개 기능이 아니다**(R12, 7.5). 원칙 A가 이미 답을 갖고
  있었다 — 캡처기는 파일을 만들고 분석기는 파일을 읽으며, 실시간은 그 파일이
  자라는 중일 뿐이다. 두 경로가 같은 집계 결과를 내는지는 테스트로 강제한다.
- **HAR은 하나의 포맷이 아니라 네 개의 방언**이며 규격은 폐기 상태다(6.5).
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
