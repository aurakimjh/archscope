# H-COV1 독립 리뷰 — T-571 Windows coverage proof

- 리뷰일: 2026-07-27
- 리뷰어: Codex
- 기준 브랜치/커밋: `main` / `e6157056e26c4c1453c696c04019be46a12d3cf4`
- 증거 커밋: `a40ea90` (real-NIC 측정), `e498ff3` (direct TCP-owner 및 CAP-5 보완),
  `e615705` (병합)
- 리뷰 범위:
  - `docs/ko|en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`
  - `docs/ko|en/SYSTEM_HTTP_CAPTURE.md`
  - `spikes/t571-windows-coverage`
  - `spikes/t571-windows-coverage/results-real-nic-20260727`
- 게이트: `H-RG2`의 개별 리뷰 `H-COV1`

## 1. 최종 판정

**판정: `CONDITIONAL`**

현재 제출물은 다음 사실을 충분히 뒷받침한다.

1. ETW의 현재 `System.Execution.ProcessID` 기반 프로세스 귀속은 CAP-2
   zero-tolerance를 위반하므로 폐기해야 한다.
2. ALE connection audit가 꺼진 measured configuration의 WFP netevents는 허용
   연결과 bypass를 관측하지 못하므로 제품 coverage source로 사용할 수 없다.
3. direct `iphlpapi!GetExtendedTcpTable` 구현은 PowerShell subprocess 경로를
   제거했고, 보존된 persistent control connection 5개에서 5/5 PID 일치, 잘못된
   PID 귀속 0, bypass connection 탐지, 보고서 기준 CPU delta 4.8%p를 재현한다.

그러나 이 사실들로 **절대 coverage ratio**까지 승인할 수는 없다. TCP-owner는
CAP-3가 `N/A`이고 probe 자체가 1초보다 짧은 연결을 관측하지 못한다고 명시한다.
그런데 judge는 `CAP-3.Applied == false`를 성공과 같게 처리하여
`coverage ratio 노출 가능, Confidence: high`와 `counter_fallback: false`를
생성한다. 이는 `SYSTEM_HTTP_CAPTURE.md` §10.4.2의 CAP-3 조건과 §10.4.3의
“CAP-1~CAP-4 통과” 조건, 그리고 검증되지 않은 분모를 UI에 내보내지 않는다는
상위 원칙을 위반한다.

따라서 현 시점의 승인 범위는 **프록시가 스스로 관측한 5개 정수 카운터와
개별 endpoint lookup 성공 시의 process attribution**까지다. 시스템 전체 또는
프로세스 전체에 대한 coverage percentage/ratio는 금지한다. `H-COV1 PASS` 전까지
`H-RG3` 진입도 금지한다.

WFP에 대해서는 별도 결론을 낸다. **WFP를 제품 후보에서 보수적으로 제거하고 아무
capability도 주장하지 않는다면 ALE audit-enabled 재실행은 이 게이트의 필수
blocker가 아니다.** 반대로 WFP를 지원·미지원으로 일반화하거나 WFP source를
coverage에 다시 사용하려면 audit policy 상태를 기록한 재실행이 먼저다.

## 2. 게이트 기준에 대한 독립 판정

### 2.1 CAP-1~CAP-6

`report.json`의 결론을 그대로 채택하지 않고 후보별 ground truth와 observation을
다시 대조했다.

| 후보 | CAP-1 귀속 | CAP-2 오탐 | CAP-3 손실 | CAP-4 bypass | CAP-5 비용 | CAP-6 권한·설치 | 독립 disposition |
|---|---|---|---|---|---|---|---|
| ETW Kernel-Network | 명목상 5/5 port에서 정답 PID도 한 번 이상 존재 | **FAIL — 잘못된 PID/port pair 197개** | 잠정 PASS — normalized obs에 44,458 delivered, 0 dropped | 명목상 탐지 | PASS — report 4.5%p | 부분 증명 — elevated만 확인 | **폐기. 어떤 production coverage/귀속에도 부분 사용 금지** |
| WFP netevents | **N/A — audit 미활성 구성에서 ALLOW가 관측되지 않음** | N/A — 이벤트 부재를 0건 성공으로 셀 수 없음 | N/A | N/A — 같은 이유 | PASS — report 4.6%p | 부분 증명 — elevated만 확인 | **measured configuration 미지원. 제품 후보에서 제거 가능** |
| TCP endpoint ownership | **PASS(제한적) — persistent socket 5/5** | **PASS — control port의 잘못된 PID pair 0** | **N/A — loss/drop counter 없음** | **PASS — persistent bypass socket 탐지** | **PASS — 4.8%p** | 미증명 — 9.3.5에 TCP-owner 행이 없고 non-admin 경로 미실측 | **개별 lookup attribution만 승인. absolute ratio는 미승인** |

#### CAP-1

- ETW observation은 control port 5개 모두에 ground-truth PID record를 포함하므로
  judge 정의의 `5/5`는 기계적으로 재현된다. 그러나 같은 port에 수많은 다른 PID도
  동시에 붙어 있어 CAP-2에서 즉시 폐기된다. `CAP-1 100%`만 독립적으로 표시하면
  오해를 만든다.
- TCP-owner의 5/5는 직접 API가 반환한 owner PID와 ground truth가 일치한다는
  증거로는 신뢰할 수 있다. 다만 표본은 실행당 5개이고 모두 약 28초간 유지된
  persistent keep-alive connection이다. 임의 수명의 시스템 연결에 대한 recall
  또는 coverage denominator를 증명하지는 않는다.
- WFP는 ALLOW audit가 없는 실행이므로 CAP-1을 `0% fail`보다 `N/A /
  configuration unsupported`로 써야 정확하다.

#### CAP-2

- ETW의 197은 잘못 귀속된 **이벤트 197건**이 아니라 control local port에 붙은
  서로 다른 잘못된 PID flow record 197개다. zero-tolerance 판정에는 어느 쪽이든
  충분하지만 보고서가 단위를 명시해야 한다.
- TCP-owner control port에는 ground truth와 다른 PID record가 없었다. 제출된
  control scope에서는 CAP-2를 통과한다.
- WFP는 relevant ALLOW event가 0이므로 “false attribution 0건 PASS”가 아니라
  측정 불가다. 아무것도 보지 못한 후보가 오탐 기준을 통과했다고 표시해서는 안 된다.

#### CAP-3

- ETW normalized observation은 `delivered=44458`, `dropped=0`을 기록한다. 다만
  이 수치의 원본인 ETL/tracerpt summary가 제출 디렉터리에 없어 독립 source-level
  검증은 할 수 없었다.
- TCP-owner와 WFP는 `kernel_reported_dropped=-1`이고 judge도 CAP-3를 `N/A`로
  기록한다. 특히 TCP-owner는 “구조적 gap이므로 lost event가 아니다”라는 이름
  바꾸기로 denominator 검증을 대신할 수 없다. 1초 polling 사이에 생성·종료된
  connection은 numerator와 denominator 양쪽에서 조용히 사라진다.
- §10.4.3의 high-confidence 조건은 CAP-1~CAP-4 통과다. CAP-3 `N/A`는 CAP-3
  pass가 아니므로 TCP-owner는 이 조건을 충족하지 않는다.

#### CAP-4

- TCP-owner는 `goes_via_proxy=false`인 port 7802 / PID 18516을 28 polls에서
  관측했다. persistent bypass connection 탐지 증거로는 유효하다.
- 단명 bypass app 탐지를 일반화할 수 없다. 같은 harness가 polling 가능성을
  보장하려고 persistent keep-alive를 의도적으로 사용한다.
- WFP의 미탐지는 audit-disabled measured configuration의 disposition을 정하는
  데는 충분하지만 WFP 기술 자체의 CAP-4 fail을 증명하지 않는다.

#### CAP-5

- direct TCP-owner observation의 total CPU는 8.007%, 생성 report에 보존된 baseline은
  약 3.184%이므로 delta 4.823%p가 재계산된다. 10% 기준 아래다.
- baseline은 5초 idle sample이고 capture 값은 loadgen/network work를 포함한 30초
  system-wide `_Total` sample이다. probe만의 인과적 overhead 측정은 아니지만,
  total 자체도 10% 아래라 이번 표본의 CAP-5 pass 결론은 보수적으로 수용한다.
- 설계 문서의 ETW/WFP 5.0%p/5.1%p는 생성 report의 4.5%p/4.6%p와 다르다.

#### CAP-6

judge의 `Pass: obs.Elevated`는 CAP-6의 충분조건이 아니다. §9.3.5와 §11.6은 권한
유무뿐 아니라 별도 설치물, session-only helper, 비상주, 캡처·파일 쓰기만 허용,
IPC peer credential 검증을 계약으로 둔다.

- ETW/WFP가 elevated에서 실행됐다는 점은 실제 실행 권한 요구와 일치한다.
- spike는 production helper/IPC/install lifecycle을 구현하거나 시험하지 않는다.
- TCP-owner는 §9.3.5 표에 독립 행이 없고 orchestrator 전체가 ETW/WFP 때문에
  elevated이므로, 이 증거로 일반 사용자 권한의 production lookup을 증명하지 못한다.

따라서 세 후보 모두 CAP-6 전체 PASS가 아니라 “실행 권한 일부 확인, production
privilege contract는 H-RG3/H-SEC2에서 검증”으로 제한해야 한다.

## 3. 심각도순 findings

### COV-1. [P0] CAP-3 N/A를 성공으로 간주해 검증되지 않은 absolute ratio를 승인한다

- 위치:
  - `cmd/judge/main.go:352-356`
  - `cmd/judge/main.go:87-97`
  - `results-real-nic-20260727/report.json`
  - `SYSTEM_HTTP_CAPTURE.md` §10.4.2~10.4.3
- 코드:
  - `cap14Pass := ... && (!c3.Applied || c3.Pass)`
  - disposition 문자열에 `"ratio 노출"`이 포함되면 전체
    `counter_fallback=false`
- 증거:
  - TCP-owner는 `kernel_reported_dropped=-1`, CAP-3 `applied=false`.
  - probe는 1초 미만 connection을 볼 수 없다고 명시한다.
  - 부하는 약 500 HTTP transactions/s였지만 persistent connection은 5개뿐이다.
    transaction 부하가 connection-observation recall을 검증한 것이 아니다.
- 영향:
  - UI가 “시스템/프로세스 트래픽의 N%를 보았다”는 검증되지 않은 분모를 표시할 수
    있다.
  - 설계가 P0로 규정한 “조용한 누락”을 coverage 기능 자체가 재도입한다.
- 필수 조치:
  1. 현 증거 기준 judge를 `counter_fallback=true`로 고친다.
  2. CAP-3 `N/A`는 ratio-bearing disposition을 얻지 못하도록 한다.
  3. 단명 connection을 포함한 ground-truth connection set과 miss/loss accounting을
     새로 측정해 CAP-3를 통과하기 전에는 absolute ratio를 금지한다.
  4. judge의 disposition/counter fallback을 table-driven unit test로 고정한다.

### COV-2. [P1] “raw evidence”가 아니라 normalized observation만 보존되어 acquisition을 독립 검증할 수 없다

- 위치: `results-real-nic-20260727`
- 보존됨: candidate별 ground truth, aggregated `obs_*.json`, 생성 report.
- 누락됨:
  - ETW `kernelnet.etl`, `kernelnet.xml`, `kernelnet.summary.txt`
  - WFP `netevents.xml`, audit policy 상태 출력
  - TCP-owner poll별 raw table 또는 최소 control tuple sample
  - `typeperf` baseline/capture 원시 sample
  - 실행 명령 transcript, binary/commit hash, receiver log
- 영향:
  - judge의 정규화 이후 계산은 재현할 수 있지만 ETW PID field 선택, dropped count,
    WFP event 종류, CPU baseline을 원천 자료에서 다시 확인할 수 없다.
  - 구현 계획의 H-COV1 PASS 조건인 “raw evidence와 재현 절차”를 아직 충족하지 못한다.
- 필수 조치:
  - 다음 acquisition은 source artifacts, audit state, stdout/stderr, 명령 인자,
    commit/binary hashes 및 CPU samples를 함께 보존한다.
  - absolute ratio를 포기하고 counter-only로 닫더라도, 제출물을 “raw”라고 부르지
    말고 evidence tier와 재현 가능한 범위를 정확히 적는다.

### COV-3. [P1] CAP-6 judge가 권한 계약을 단일 elevated boolean으로 축소한다

- 위치: `cmd/judge/main.go:291-303`
- 규범: `SYSTEM_HTTP_CAPTURE.md` §9.3.5, §11.6.
- 문제:
  - elevated이면 무조건 CAP-6 PASS다.
  - helper 수명·권한 범위·IPC·설치물을 전혀 확인하지 않는다.
  - TCP-owner는 규범 표에 행도 없으며 non-admin 실행이 측정되지 않았다.
- 영향:
  - production privilege contract가 증명된 것처럼 report와 문서에 남는다.
- 필수 조치:
  - CAP-6를 자동 PASS하지 말고 candidate별 요구 체크리스트 또는 “spike에서
    부분 확인 / production gate로 이관” 상태로 표현한다.
  - TCP-owner production 경로의 권한·helper 필요 여부를 §9.3.5에 명시한다.

### COV-4. [P2] ETW 문서가 event header PID를 payload PID로 표현한다

- 위치:
  - `cmd/etwprobe/main.go:163-170, 208`
  - `SYSTEM_HTTP_CAPTURE.md` §9.3.1, §10.4.6, Appendix A
- 문제:
  - parser가 읽는 PID는 `Event/System/Execution@ProcessID`다.
  - 문서와 ledger는 “payload의 ProcessID+sport+dport”처럼 표현한다.
  - 같은 문서의 `V-WIN-ETW-TCPIP`는 event header PID를 발신 process로 그대로
    사용하지 말라는 공식 주의를 이미 기록한다.
- 영향:
  - 197 false pair의 원인을 제대로 설명하지 못하고, 향후 다른 provider/parser가
    같은 실수를 반복할 수 있다.
- 조치:
  - ETW candidate는 계속 폐기하되, source를 `System.Execution header PID`로
    정확히 수정하고 `CAP-1 100%`를 독립적인 성공 주장으로 노출하지 않는다.

### COV-5. [P2] WFP 및 CAP-5 문구가 원시 disposition과 생성 report에 일치하지 않는다

- WFP:
  - probe 자체 note는 audit-disabled 구성에서 `미확정`이라고 한다.
  - report는 CAP-1 0% fail, CAP-2 0건 pass, CAP-4 fail로 확정한다.
  - 올바른 표현은 “measured configuration에서 unavailable; WFP 일반 capability
    N/A”다.
- CAP-5:
  - generated report: ETW 4.5%p, WFP 4.6%p, TCP-owner 4.8%p.
  - KO/EN 설계 표: ETW 5.0%p, WFP 5.1%p, TCP-owner 4.8%p.
- gate ledger:
  - KO 문서의 gate 9 상태는 아직 “ETW/WFP 실 NIC 재실측 잔여”라고 적혀 있어
    최신 H-RG2 상태와 충돌한다.
- 조치:
  - paired KO/EN 문서, report, ledger가 같은 수치·상태·단위를 사용하도록 고친다.

### COV-6. [P2] judge의 핵심 안전 판정에 자동화된 회귀 테스트가 없다

- `go test ./...`은 통과하지만 `cmd/judge`에는 test file이 없다.
- 직접 TCP table decoder test는 정상·truncated/oversized row를 검증한다.
- 반면 다음 P0 조건은 무검증이다:
  - CAP-2 하나라도 발생하면 candidate 폐기
  - CAP-3 N/A/fail에서 ratio 억제
  - 모든 candidate 실패 시 five-counter fallback
  - WFP relevant observation 0일 때 CAP-2를 성공으로 세지 않음
  - per-candidate ground truth pairing
- 조치: synthetic ground truth/observation fixture를 이용한 judge unit test를
  추가한다.

## 4. direct GetExtendedTcpTable 증거 신뢰성

### 승인하는 부분

- `table_windows.go`는 `iphlpapi.dll!GetExtendedTcpTable`을 IPv4(AF_INET)와
  IPv6(AF_INET6)에 직접 호출한다.
- 필요한 buffer 크기를 먼저 받고 `ERROR_INSUFFICIENT_BUFFER` race를 최대 3회
  처리한다.
- `table_decode.go`의 IPv4 24-byte / IPv6 56-byte owner-PID row decoding,
  network-order port decoding은 unit test와 Windows cross-build를 통과했다.
- observation의 control tuple은 다음과 같이 ground truth와 일치했다.

| local port | expected PID | observed PID | polls |
|---:|---:|---:|---:|
| 7798 | 11680 | 11680 | 28 |
| 7799 | 33316 | 33316 | 28 |
| 7800 | 2368 | 2368 | 28 |
| 7801 | 34968 | 34968 | 28 |
| 7802 (bypass) | 18516 | 18516 | 28 |

따라서 “PowerShell이 아니라 direct API를 호출한다”와 “존재 중인 endpoint의 owner
PID lookup이 정확했다”는 주장은 신뢰할 수 있다.

### 승인하지 않는 부분

- 이 결과를 arbitrary connection의 관측률로 일반화하지 않는다.
- 5 persistent sockets를 28회 반복 관측한 것은 독립 표본 140개가 아니다.
- 13,992 successful HTTP transactions는 5 sockets 위에서 재사용됐으므로
  connection coverage CAP-3의 분모가 아니다.
- polling interval보다 짧은 connection의 missed count를 계산하지 못한다.
- production proxy가 연결 수립 시 동기 lookup하는 실제 timing/race는 이 probe와
  동일하지 않다.

## 5. UI 공개 승인/금지 행렬

이 행렬은 **현재 제출물과 remediation 전 상태**에 적용한다.

| 값/표현 | 현재 UI 공개 | 조건과 정확한 문구 |
|---|---|---|
| `captured` | **승인** | 프록시가 직접 관측한 정수 count. “시스템 전체”가 아님을 명시 |
| `passthrough` | **승인** | 프록시가 직접 관측한 정수 count |
| `unattributed` | **승인** | 관측했으나 process를 확정하지 못한 정수 count |
| `dropped` | **승인** | 해당 capture pipeline이 스스로 계상한 정수 count와 source를 함께 표시 |
| `unsupported` | **승인** | 프록시가 분류 가능한 H2 passthrough/QUIC 등 정수 count; 보이지 않은 모든 system traffic을 뜻하지 않음 |
| 개별 TCP endpoint의 owner PID/process | **조건부 승인** | tuple lookup이 그 시점에 성공한 transaction/connection만 `confirmed`; 실패·race는 `inferred` 또는 `unknown` |
| TCP-owner sampling interval/단명 연결 한계 | **승인·필수 고지** | coverage diagnostic에 `1s sample; sub-interval connections may be missed` |
| absolute/system/process coverage ratio 또는 percent | **금지** | CAP-3가 N/A이고 denominator가 검증되지 않음 |
| `CoverageEvidence.Confidence: high` for TCP-owner ratio | **금지** | CAP-1~CAP-4 전체 통과가 아님 |
| `counter_fallback: false` | **금지** | 현 evidence에서는 `true`가 안전한 결론 |
| five counters를 합쳐 만든 “전체 대비 비율” | **금지** | proxy-observed composition을 system coverage로 오인할 수 있음 |
| ETW PID attribution 또는 ETW 기반 coverage denominator | **금지** | CAP-2 fail candidate는 부분 사용도 금지 |
| ETW `CAP-1 100%` 단독 성공 표시 | **금지** | CAP-2 197 false pairs와 분리하면 오해 |
| WFP attribution/coverage ratio | **금지** | audit-disabled measured configuration에서 relevant ALLOW flow 0 |
| “WFP는 기술적으로 지원 불가/실패”라는 일반화 | **금지** | 증거는 measured configuration의 미지원만 입증 |
| “트래픽 0 = 시스템에 트래픽 없음” | **금지** | “이 capture source에서 관측 0”으로만 표현 |

제품에서 비율을 꼭 제공하려면 `CoverageEvidence`의 `metric`, `scope`, `source`,
`samplingInterval`, `confidence`가 모두 채워져야 하고 numerator/denominator의
정의와 loss/miss counter가 같은 scope에서 검증돼야 한다.

## 6. WFP audit-enabled 재실행 결정

**결정: WFP 제거를 선택한다면 audit-enabled 재실행은 H-COV1 PASS의 필수 조건이
아니다.**

이 판단의 이유는 다음과 같다.

1. 현재 실행은 WFP의 positive capability를 측정하지 못했다. 이를 CAP-1 0%라는
   기술 일반 실패로 쓰는 것은 부정확하다.
2. 그러나 product 후보를 제거하는 결정에는 “현재 기본/측정 구성에서 ALLOW
   connection이 보이지 않는다”는 증거로 충분하다. 기능을 덜 주장하는 보수적
   disposition은 false claim을 만들지 않는다.
3. H-RG3의 1차 product path는 MITM proxy + direct endpoint attribution이며 WFP
   capability가 필수 구현 전제가 아니다.
4. WFP를 다시 후보로 넣거나 WFP source의 counter/ratio를 UI에 쓰려는 순간에는
   audit subcategory 설정 전후 상태, 재부팅/정책 적용 여부, ALLOW event 원본,
   loss semantics를 보존한 재실행이 필수다.

따라서 remediation은 WFP를 `failed`가 아니라
`not measured under required audit configuration / not used by product`로 기록하고,
source ledger를 `partial`로 유지해야 한다.

## 7. 재현 및 검증 명령

### 7.1 실행한 검사

```powershell
git rev-parse HEAD
# e6157056e26c4c1453c696c04019be46a12d3cf4

git log --oneline --decorate -12
git show --stat --oneline a40ea90
git show --stat --oneline e498ff3
git show --stat --oneline e615705

$env:GOWORK='off'
go test ./...
$env:GOOS='windows'
$env:GOARCH='amd64'
go test ./...
go vet ./...
go build ./cmd/...
```

결과:

- native `go test ./...`: PASS
- Windows amd64 `go test ./...`: PASS
- Windows amd64 `go vet ./...`: PASS
- Windows amd64 `go build ./cmd/...`: PASS
- `cmd/tcpownerprobe` decoder tests: PASS
- `cmd/loadgen` tests: PASS
- `cmd/judge`: test file 없음

처음 sandbox 내부 실행은 Go build cache 접근 권한 때문에 실패했고, 같은 명령을
승인된 외부 실행으로 다시 수행해 모두 통과했다. 코드/테스트 실패가 아니다.

### 7.2 observation 독립 재계산

candidate별 `ground_truth_*.json`을 해당 `obs_*.json`과 pair한 뒤 control local
port에서 정답 PID/image 존재 수와 잘못된 PID/image pair 수를 다시 계산했다.

```text
etw:      correct=5/5, falsePairs=197, delivered=44458, dropped=0,
          totalTx=14005, captureCPU=7.727
wfp:      correct=0/5, falsePairs=0, delivered=1, dropped=-1,
          totalTx=14005, captureCPU=7.776
tcpowner: correct=5/5, falsePairs=0, delivered=30, dropped=-1,
          totalTx=13992, captureCPU=8.007
```

이 값은 생성 report의 기계 계산과 일치한다. 단 WFP의 `falsePairs=0`은 relevant
ALLOW observation 0에서 나온 공집합이지 CAP-2 positive proof가 아니며,
TCP-owner의 `delivered=30`은 poll 횟수이지 connection/event delivered count가
아니다.

### 7.3 evidence SHA-256

```text
ground_truth_etw.json       05782177CE0A05E5CB04685B9A417F284439AC527C1DFF9BAAB37B6F690F891A
ground_truth_tcpowner.json  92B3CEC1A834788B8DA88EB233134870223165DD956E24FB3A36235A6E6833D9
ground_truth_wfp.json       06511DA2006E16432E2FF982EFDCC042C6E3F577ADB98C2A0895BBB4314483FD
ground_truth.json           92B3CEC1A834788B8DA88EB233134870223165DD956E24FB3A36235A6E6833D9
obs_etw.json                8403352FB71E28790018B7E0513744DC2E66EF37E11E5932A2429B0875A0E839
obs_tcpowner.json           B7F5BC445507F3D558DF0E946592C7EC09FF02BB623579B153FD49884BA010BE
obs_wfp.json                7FF09C471D99E5C16BD626C1BFA684140A5C8CD43281BDD37CE8A464A25983F0
report.json                 2410890E34751A32CFF044E75E9D42AC23A4F7711937310B26C7F5196CD60A
```

## 8. 재리뷰 조건과 gate 결론

`H-COV1 PASS`를 위해 다음을 완료해야 한다.

1. **COV-1:** CAP-3 N/A를 ratio pass로 취급하지 않도록 judge와 report를 고치고
   현 evidence의 `counter_fallback=true`를 확정한다.
2. **COV-1:** UI/API 계약에서 absolute/system/process coverage ratio와 TCP-owner
   `Confidence: high`를 제거한다. ratio를 유지하려면 단명 connection을 포함한
   별도 CAP-3 재측정을 제출한다.
3. **COV-2:** source artifact가 없는 현재 evidence의 한계를 명시한다. 새 ratio
   측정을 택하면 ETL/WFP XML/TCP poll samples/typeperf/transcript/hash를 보존한다.
4. **COV-3:** CAP-6를 elevated boolean 자동 PASS에서 분리하고 TCP-owner의
   production 권한 행을 규범 문서에 추가한다.
5. **COV-4~5:** ETW header PID, WFP N/A disposition, CAP-5 숫자와 gate/source
   ledger 상태를 KO/EN/report에서 일치시킨다.
6. **COV-6:** judge의 zero-tolerance, N/A, fallback, per-candidate ground-truth
   pairing 회귀 테스트를 추가한다.
7. 독립 재리뷰에서 위 exposure matrix가 구현 계약에 반영됐는지 확인한다.

**Gate 결론:** `H-RG2 / H-COV1`은 아직 통과하지 않았다. 현재 안전한 경로는
counter-only fallback이며, 이 교정과 재리뷰가 끝나기 전에는 `H-RG3` 실시간 캡처
엔진 그룹을 시작할 수 없다. WFP audit-enabled rerun은 WFP를 제거하는 경로에서는
요구하지 않지만, WFP capability를 다시 주장하는 경로에서는 필수다.
