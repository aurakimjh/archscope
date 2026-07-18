# 시스템 전체 HTTP 통신 캡처 및 분석 (설계 노트)

## 0. 이 문서의 위치와 전제

본 문서는 HTTPAnalyzer 계열 도구가 제공하던 **시스템 전역 HTTP/HTTPS 트래픽
캡처 및 프로세스별 분석** 기능을 ArchScope에 도입하기 위한 설계 노트다.

전제를 먼저 명시한다.

- HTTPAnalyzer(IEInspector)의 단종 여부와 시점은 본 문서가 검증한 사실이 아니라
  **기획 입력으로 주어진 전제**다. 본 문서는 그 제품을 *기능 레퍼런스*로만
  참조하며, 특정 회사·제품의 현재 상태를 주장하지 않는다.
- 따라서 "HTTPAnalyzer가 했던 것"이라는 표현은 전부 **기능 요구사항의 축약**으로
  읽어야 한다. 구현 세부를 모방하자는 뜻이 아니다.

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
| Charles / Fiddler / Proxyman | △ (프록시 인지 앱만) | △ (일부 추정) | ◎ | 프록시를 무시하는 앱은 안 보임 |
| mitmproxy | △ | ✗ | ◎ | CLI 중심, 프로세스 개념 없음 |
| macOS `nettop` / Windows 리소스 모니터 | ◎ | ◎ | ✗ (바이트 수만) | 내용이 없다 |

**전역 캡처와 프로세스 식별과 HTTP 시맨틱의 교집합**이 비어 있다. 그리고 그
교집합이 필요한 순간은 실무에서 반복된다.

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

주의점 세 가지를 명시한다.

1. **재사용 연결에서는 DNS/Connect/TLS가 0이다.** `Connection.Reused`가 참일 때
   0을 "측정 실패"로 오해하지 않도록 UI가 구분해야 한다.
2. **프록시 모드에서 이 값들은 프록시↔서버 구간이다.** 클라이언트↔프록시 구간은
   별도로 관측되며, 클라이언트가 체감한 총 시간은 두 구간의 합에 프록시 처리
   시간이 더해진 값이다. 두 관점을 모두 저장하고 UI에서 전환 가능하게 한다.
   합쳐서 하나의 숫자로 뭉개면 3.2의 침습성 문제가 데이터에 숨는다.
3. **HTTP/2 다중 스트림에서 `BlockedMs`의 의미가 달라진다.** 동일 연결의 다른
   스트림과 대역폭을 나눠 쓰므로 `ReceiveMs`가 서버 성능과 무관하게 늘어난다.

불변식: `TotalMs ≈ 각 구간의 합`. 어긋나면 파서 진단에 finding을 남긴다
(`AddFinding`, `analysis_result.go:135`).

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

### 6.5 HAR 상호운용

HAR 1.2는 이 영역의 사실상 표준이며 DevTools·Charles·Proxyman·Fiddler가 모두
지원한다. 원칙 A에 따라 **HAR을 1급 입출력 포맷으로 삼는다.**

- **가져오기**: 표준 HAR을 그대로 읽는다. 프로세스 정보가 없으므로 전체를 단일
  `(HAR 가져오기)` 루트에 넣는다. **캡처 기능 없이도 이 경로만으로 제품 가치가
  성립한다** — Phase 1의 근거다.
- **내보내기**: 표준 HAR로 내보내되 ArchScope 확장은 `_archscope` 접두 필드에
  담는다(HAR 규격이 `_` 접두 커스텀 필드를 허용한다). 프로세스 정보, `fidelity`,
  `captureMode`가 여기 들어간다. 다른 도구에서 열면 확장 필드는 무시되고 표준
  부분만 읽힌다.

`ingestion` 등록도 기존 관례를 따른다: `registry.go`의 `DetectorFunc`(L37)로
`.har` JSON의 `log.version`/`log.entries` 시그니처를 탐지하고, 새 소스 종류
`http_capture`를 `boundary.go:11`의 상수군에 추가한다.

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
│ 412 트랜잭션 · 18 프로세스 · 34 호스트 · 미식별 3%                        │
├──────────────────────┬─────────────────────────────────────────────────┤
│ 프로세스 트리         │ ┌─ 타임라인 ────────────────────────────────┐   │
│                      │ │  ▁▃█▅▂▁ 요청률 / 오류(적색) / 전송량       │   │
│ ▾ Google Chrome  412 │ └───────────────────────────────────────────┘   │
│   ├ renderer  4821   │ ┌─ 트랜잭션 목록 ───────────────────────────┐   │
│   └ gpu       4830   │ │ # 메서드 상태 호스트   경로   크기  시간   │   │
│ ▾ java         57    │ │ 1 GET   200  api.x.com /v1/…  4KB  128ms  │   │
│   └ 9102 batch.jar   │ │ 2 POST  500  api.x.com /v1/…  1KB  2.4s ⚠ │   │
│ ▸ curl          9    │ └───────────────────────────────────────────┘   │
│ ▸ (미식별)      12 ⚠ │ ┌─ 상세 ─────────────────────────────────────┐  │
│                      │ │ [요약][요청][응답][타이밍][연결/TLS][원본]  │  │
│ [필터]               │ └────────────────────────────────────────────┘  │
└──────────────────────┴─────────────────────────────────────────────────┘
```

좌측 트리가 R2의 구현이다. 노드 선택이 우측 전체(타임라인·목록·통계)를 필터링
한다. 트리 노드에는 요청 수와 함께 **총 소요시간 기여도**를 막대로 표시한다 —
"요청이 많은 프로세스"보다 "시간을 많이 쓴 프로세스"가 대개 더 유용하다.

`(미식별)` 노드는 0건이어도 표시하고, 건수가 있으면 경고 아이콘과 함께 4.3의
원인 설명 링크를 붙인다.

### 8.3 트랜잭션 상세 탭

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

### 8.4 필터

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

### 8.5 성능 제약

트랜잭션 수만 건은 일상적이다. 기존 최적화 선례(MSA 타임라인 막대 상한, 캔버스
플레임그래프 행 버킷 히트 테스트)와 같은 방향으로:

- 목록은 가상 스크롤. DOM에 보이는 행만.
- 타임라인은 캔버스 + 시간 버킷 집계. SVG 요소를 트랜잭션마다 만들지 않는다.
- 라이브 갱신은 100ms 배치(7.3).
- 바디는 **상세 탭을 열 때** 지연 로딩. 목록 렌더에 바디가 개입하면 안 된다.
- 텍스트 검색 중 바디 검색은 엔진 측에서 수행하고 결과 ID만 받는다.

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
2. `internal/parsers/httpcapture` — HAR 1.2 파서
3. `ingestion` 등록 — `http_capture` 소스 종류, `.har` 탐지기
4. `internal/analyzers/httpcapture` — 집계, 6.7 진단, `AnalysisResult` 생성
5. `archscope-engine http-capture analyze --in x.har` CLI
6. `HttpCapturePage` + "네트워크 분석" 메뉴 그룹 (8.1)
7. 프로세스 트리(단일 `(HAR 가져오기)` 루트), 목록, 상세, 타이밍, 필터

**아키텍처 전제를 하나도 깨지 않는다.** 파일 입력, 배치 분석, 권한 없음. 순수
importer 추가다. 리스크가 낮고 UI 자산이 전부 만들어지므로 이후 Phase가 가벼워
진다.

### Phase 2 — MITM 프록시 캡처

R1(부분)·R2·R3·R4·R5·R6를 실제로 만족시킨다.

8. `internal/capture` 골격 — `Capturer` 인터페이스, 세션 수명주기
9. `capture/proxy` — HTTP/1.1 + HTTP/2, CONNECT, MITM, CA 관리
10. `capture/procmap` — 3플랫폼 PID 매핑 (4.4 프록시 최적화 경로)
11. `capture/redact` — 11.3 리댁션
12. `capture/store` — 세션 디렉터리, blob, manifest
13. `CaptureService` Wails 노출 + 이벤트 배치 (7.3)
14. UI — 캡처 시작/정지, 라이브 갱신, **프로세스별 트리 루트**
15. CA 설치/제거 화면 + 위험 고지 (11.2)
16. 최초 사용 고지 (11.1)

여기서 전제 파괴가 시작된다(스트림 입력, 장시간 세션, 데이터 생성). 다만
**권한 승격은 아직 없다.**

### Phase 3 — 커버리지 정직성

프록시의 부분 커버리지를 관리 가능하게 만든다. **Phase 2와 짝이며 미루면 안
된다.** 이것 없는 Phase 2는 사용자를 오도한다.

17. 프로세스별 시스템 바이트 카운터 수집 (10.1)
18. 커버리지 지표 산출 + `Summary` 반영
19. 미관측 프로세스 감지 및 **설정 안내 UI** (10.2)
20. 피닝 자동 감지 및 패스스루 (5.3)
21. `fidelity` 배지의 UI 전면 반영

### Phase 4 — 교차 분석

22. 캡처 세션 ↔ CPU 프로파일 시각 정렬 및 겹침 뷰 (12.1)
23. 캡처 ↔ Jennifer MSA `applyNetworkGap` 검증 뷰 (12.2)
24. 캡처 ↔ access log 클라이언트/서버 대조 (12.3)
25. 세션 Diff — URL 정규화 포함 (12.4)

**12절이 이 기능의 존재 이유이므로 Phase 4가 실질적 완성이다.** Phase 2~3만
하면 기존 프록시 도구의 열등한 복제품에 그친다.

### Phase 5 — 심화 캡처 (선택)

여기서 처음 권한 승격이 필요하다. Phase 3의 커버리지 지표가 "프록시로 부족하다"를
**데이터로 증명한 뒤에만** 착수한다.

26. 권한 헬퍼 골격 (9.2)
27. pcap 백엔드 + 대조 모드 (10.3)
28. Windows ETW (WinINet/WinHTTP) — 비용 대비 효과 최상
29. Linux eBPF (uprobe SSL) — 서버 환경
30. 키 로그 + pcap 오프라인 복호화 (5.4)
31. WebSocket, gRPC (13절)

투명 프록시(3.3)와 macOS Network Extension은 **로드맵에 올리지 않는다.** 배포
파이프라인 자체를 바꾸는 요구이므로 별도 의사결정 문서가 필요하다.

## 14. 열린 질문

설계 검토에서 답이 필요한 항목들이다.

1. **Phase 0의 세 질문.** 특히 CA 신뢰 등록. 이것 없이는 HTTPS 평문이 불가능하고,
   HTTPS 평문 없이는 이 기능의 가치가 절반 이하다. 그러나 제품 위험은 이 기능
   전체에서 가장 크다.
2. **라이브 뷰가 필수인가.** 원칙 A는 캡처 후 분석을 기본으로 삼는다. 라이브
   갱신은 UX상 매력적이지만 7.3의 백프레셔·성능 부담을 낳는다. "캡처 중에는
   카운터만, 정지 후 분석"으로 시작하는 안이 더 안전할 수 있다.
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
