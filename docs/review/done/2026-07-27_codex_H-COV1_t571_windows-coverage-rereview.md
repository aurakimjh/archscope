# H-COV1 독립 재검토 — T-571 Windows coverage proof

- 재검토일: 2026-07-27
- 리뷰어: Codex
- 기준 코드: `main` / `e6157056e26c4c1453c696c04019be46a12d3cf4` 위 최종
  remediation working tree
- 선행 리뷰:
  `docs/review/done/2026-07-27_codex_H-COV1_t571_windows-coverage-review.md`
- 재검토 범위:
  - `spikes/t571-windows-coverage/cmd/judge`
  - `spikes/t571-windows-coverage/results-real-nic-20260727`
  - `spikes/t571-windows-coverage/README.md`
  - `docs/ko|en/SYSTEM_HTTP_CAPTURE.md`
  - `docs/ko|en/BROWSER_PROFILE_HTTP_CAPTURE_IMPLEMENTATION_PLAN.md`
  - `work_status.md`

## 1. 최종 판정

**판정: `PASS`**

첫 리뷰의 `COV-1`~`COV-6` 차단 사항은 모두 닫혔다.

- CAP-3 `N/A`는 ratio-bearing이 아니며 regenerated report는
  `counter_fallback: true`다.
- CAP-2는 `Applied && Pass`여야 ratio 후보가 된다. CAP-2 `N/A`도 별도
  disposition과 회귀 테스트로 ratio가 억제된다.
- ETW는 197 distinct false PID/port pairs로 폐기되며 부분 사용하지 않는다.
- WFP는 relevant ALLOW event를 보지 못한 audit-disabled measured configuration을
  CAP-1/2/4 `N/A`로 표시하고 제품 coverage 후보에서 보수적으로 제거한다.
  audit-enabled capability는 주장하지 않는다.
- TCP-owner는 persistent endpoint의 개별 attribution만 허용하고 absolute
  system/process coverage ratio에는 사용하지 않는다.
- CAP-6은 자동 PASS에서 `N/A / 부분 확인`으로 낮아졌고 production helper,
  privilege, IPC, install 검증은 H-SEC2/H-RG3로 이관됐다.
- ETW PID source는 최종 judge appendix와 paired design docs에서 정확히
  `Event/System/Execution@ProcessID` header PID로 표현된다.
- 커밋된 evidence는 normalized observation tier이며 source-level raw package가
  아니라는 한계가 README, KO/EN 설계, plan, work status에 명시된다.
- judge의 P0 안전 판정과 candidate별 ground-truth 선택에 회귀 테스트가 추가됐다.

따라서 `H-RG2 / H-COV1`을 닫고 다음 그룹 진입 조건으로 사용할 수 있다. 이 PASS는
absolute coverage ratio를 새로 승인하는 판정이 아니다. 승인 결과는
**counter-only fallback + 성공한 개별 endpoint attribution**이다.

최종 working tree에서 gate 관련 잔여 finding은 없다.

## 2. 원 finding별 재검토

| 원 finding | 재검토 결과 | 근거 |
|---|---|---|
| COV-1 — CAP-3 N/A가 absolute ratio를 허용 | **CLOSED** | `ratioBearing`이 CAP-3 `Applied`를 필수로 요구하고 report가 `counter_fallback=true` |
| COV-2 — normalized observation을 raw evidence로 과장 | **CLOSED** | source ETL/WFP XML/poll table/typeperf/transcript/hash 미보존을 명시하고 evidence tier를 하향 |
| COV-3 — CAP-6을 elevated boolean으로 자동 PASS | **CLOSED** | CAP-6 `Applied=false`, production 계약 H-SEC2/H-RG3 이관, TCP-owner 권한 행 추가 |
| COV-4 — ETW event header PID를 payload PID로 표현 | **CLOSED (핵심 계약)** | judge appendix와 KO/EN §8/§9.3.1/§10.4.6/ledger가 header PID + payload ports로 분리 |
| COV-5 — WFP/CAP-5/report/docs 불일치 | **CLOSED** | WFP N/A/제거, ETW 4.5%p·WFP 4.6%p·TCP 4.8%p, gate 상태가 일치 |
| COV-6 — judge 안전 판정 회귀 테스트 없음 | **CLOSED** | `cmd/judge/main_test.go`가 N/A, false attribution, fallback, WFP, ground-truth pairing을 검증 |

### 2.1 COV-1 — ratio와 counter fallback

현재 `ratioBearing`은 다음 조건을 모두 요구한다.

1. candidate observation이 measured여야 한다.
2. CAP-1이 applied/pass여야 한다.
3. CAP-2가 applied/pass여야 한다.
4. CAP-3가 applied여야 한다.
5. CAP-4가 applied/pass여야 한다.

CAP-2가 applied 상태에서 fail이면 disposition이 먼저 `폐기`가 된다. CAP-2가
N/A이고 CAP-1만 pass인 synthetic case는
`CAP-2 미측정; ratio 노출 보류`로 분리된다. CAP-3가 N/A이면
`개별 endpoint 귀속만 승인 — CAP-3 미측정; absolute coverage ratio 금지`가
된다.

CAP-3이 **측정됐지만 threshold를 실패**한 경우만 기존 normative table에 따라
loss rate를 함께 표시하는 medium ratio가 될 수 있다. CAP-3 N/A와 CAP-3 fail이
더 이상 섞이지 않는다.

실제 regenerated `report.json`:

```text
counter_fallback=true
overall_outcome=어떤 후보도 CAP-1~CAP-4를 통과하지 못했다.
                절대 coverage ratio를 제거하고 5개 카운터만 유지한다.
```

candidate별 안전 경로:

| 후보 | ratio 차단 이유 | 최종 disposition |
|---|---|---|
| ETW | CAP-2 applied/fail, 197 false pairs | 폐기, 부분 사용 금지 |
| WFP | CAP-1/2/3/4 N/A | measured configuration 미지원, 제품 후보 제거 |
| TCP-owner | CAP-3 N/A | 개별 endpoint attribution만 승인 |

`summarizeCoverage`는 세 후보 중 ratio-bearing candidate가 없으므로 five-counter
fallback을 선택한다. 원 COV-1은 재발하지 않는다.

### 2.2 COV-2 — evidence tier와 재현성

문서와 README는 다음을 구분한다.

- 보존됨: candidate별 ground truth, normalized `obs_*.json`, generated
  `report.json`/`report.md`.
- 보존되지 않음: ETL/tracerpt source summary, WFP XML/audit state, poll별 TCP
  table, typeperf samples, command transcript, binary hashes.

KO/EN `SYSTEM_HTTP_CAPTURE.md`의 real-NIC 절, source ledger와 plan, README,
`work_status.md` 모두 현재 패키지를 “normalized evidence”라고 부른다. H-COV1 PASS
기준도 존재하지 않는 raw artifact를 요구하는 문구에서 “재현 절차와 evidence
tier/한계의 정직한 기록”으로 정렬됐다.

이는 원본이 없다는 사실을 보완한 것이 아니라 **증거가 실제로 말할 수 있는 범위를
정확히 낮춘 것**이다. 이번 gate가 absolute ratio를 포기하는 결론이므로 안전하며,
향후 ratio를 다시 주장하려면 README에 열거된 source-level package를 새로
보존해야 한다.

### 2.3 COV-3 — CAP-6

judge는 모든 candidate에서 CAP-6을 다음과 같이 생성한다.

```text
Applied=false
Pass=false
Value=부분 확인 — 관리자 권한 실행; production 계약은 H-SEC2 이관
Detail=helper 수명·권한 범위·IPC peer 검증·설치 계약은 이 spike에서 미측정
```

따라서 elevated observation 하나로 CAP-6이 통과하지 않는다. KO/EN 권한 표에는
TCP endpoint ownership을 “일반 사용자/unprivileged 예상, H-RG3 production path에서
재검증”으로 추가했다. helper lifetime, session-only privilege, IPC peer
verification, install contract는 H-SEC2/H-RG3의 실제 구현 gate에 남아 있다.

H-COV1은 이 partial/deferred 상태를 승인한다. T-571 spike가 구현하지 않은
production privilege boundary를 증명했다고 주장하지 않기 때문이다.

### 2.4 COV-4 — ETW source semantics

최종 표현은 다음과 같다.

```text
Event/System/Execution@ProcessID header PID
+ event payload sport/dport
```

이 표현이 judge appendix, regenerated report, KO/EN real-NIC 절, KO process
attribution matrix와 source ledger에 일치한다. `System.Execution` PID와 event
payload port를 하나의 “payload PID”로 부르던 핵심 오류는 닫혔다.

ETW CAP-1 5/5는 report에 남지만 CAP-2 197 distinct false pairs와 바로 붙어 있고,
candidate disposition은 폐기다. 따라서 PID field 존재가 정확한 process
attribution으로 승격되지 않는다.

### 2.5 COV-5 — WFP와 수치/상태 parity

WFP observation은 audit-enabled 측정이 아니다. remediation은 이를 다음과 같이
처리한다.

- control relevant ALLOW set이 비어 있으면 CAP-1/2/4를 모두 N/A로 만든다.
- “아무것도 보지 않았으므로 false attribution 0 pass”로 계산하지 않는다.
- WFP 기술 일반의 실패라고 주장하지 않는다.
- measured configuration을 제품 후보에서 제거하고 audit-enabled capability를
  주장하지 않는다.
- 이 보수적 제거를 위해 audit-enabled rerun을 요구하지 않는다.
- WFP를 다시 product source로 쓰려면 별도 audit-enabled evidence가 필요하다.

현재 KO/EN/report 수치는 ETW 4.5%p, WFP 4.6%p, TCP-owner 4.8%p로 일치한다.
ETW/WFP CAP-5는 폐기/미지원 disposition을 바꾸지 않는 composite 참고값이라고
문서가 제한한다. plan과 work status도 “첫 리뷰 CONDITIONAL, remediation 완료,
re-review 대기” 상태로 일치한다.

### 2.6 COV-6 — 회귀 테스트

추가된 `cmd/judge/main_test.go`는 다음 안전 불변식을 검증한다.

- CAP-3 N/A이면 ratio 금지와 individual-attribution-only disposition.
- measured CAP-3 pass이면 high, measured CAP-3 fail이면 medium.
- CAP-2 fail 하나라도 candidate 폐기.
- CAP-2 N/A이면 ratio 금지와 explicit unmeasured disposition.
- CAP-3 N/A candidate만 있으면 `counter_fallback=true`.
- WFP relevant observation set이 비면 CAP-1/2/4 N/A와 제품 후보 제거.
- CAP-6은 spike에서 N/A.
- candidate별 ground-truth file이 shared fallback보다 우선.

이 테스트는 원 리뷰가 요구한 P0 safety branches와 stale shared ground-truth
회귀를 직접 고정한다.

## 3. 실제 report 및 노출 계약 확인

### 3.1 regenerated report

`report.json`과 `report.md`는 다음 값으로 일치한다.

| 후보 | CAP-1 | CAP-2 | CAP-3 | CAP-4 | CAP-6 | disposition |
|---|---|---|---|---|---|---|
| ETW | 5/5 pass | 197 fail | 0% pass | detected | N/A/partial | discard |
| WFP | N/A | N/A | N/A | N/A | N/A/partial | measured config unsupported; removed |
| TCP-owner | 5/5 pass | 0 pass | N/A | detected | N/A/partial | individual endpoint only |

보고서의 `ground_truth` top-level 값은 마지막 TCP-owner pass지만 judge는 scoring 때
다음 candidate별 파일을 우선한다.

- `ground_truth_etw.json`
- `ground_truth_wfp.json`
- `ground_truth_tcpowner.json`

`TestCandidateGroundTruthPrefersPerCandidateFile`이 이 동작을 검증한다.

### 3.2 승인되는 UI/API 값

| 값 | 재검토 승인 |
|---|---|
| proxy self-observed `captured` 정수 count | 승인 |
| proxy self-observed `passthrough` 정수 count | 승인 |
| proxy self-observed `unattributed` 정수 count | 승인 |
| capture pipeline이 계상한 `dropped` 정수 count | 승인 |
| proxy가 분류한 `unsupported` 정수 count | 승인 |
| lookup 시점에 성공한 개별 endpoint의 PID/process attribution | 조건부 승인 (`confirmed`; 실패는 `inferred`/`unknown`) |
| 1초 polling 및 sub-interval miss 한계 표시 | 필수 |

### 3.3 금지되는 UI/API 값

| 값 | 재검토 판정 |
|---|---|
| absolute system/process coverage ratio 또는 percentage | 금지 |
| TCP-owner ratio에 `Confidence: high` | 금지 |
| `counter_fallback=false` | 금지 |
| five counters의 합을 시스템 전체 분모로 부르는 비율 | 금지 |
| ETW PID attribution/coverage denominator의 부분 사용 | 금지 |
| WFP attribution/ratio 또는 audit-enabled capability | 금지 |
| traffic 0을 system traffic 없음으로 해석 | 금지 |

이 행렬은 paired docs와 regenerated report의 disposition에 부합한다.

## 4. 검증 명령과 결과

최종 CAP-2 robustness 변경 이후 다음 검사를 다시 실행했다.

```powershell
cd spikes\t571-windows-coverage
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
- `cmd/judge`: PASS
- `cmd/loadgen`: PASS
- `cmd/tcpownerprobe`: PASS

추가 read-only 확인:

```powershell
git diff --check
Get-Content results-real-nic-20260727\report.json -Raw | ConvertFrom-Json
Get-FileHash results-real-nic-20260727\ground_truth_*.json -Algorithm SHA256
```

- `git diff --check`: PASS. 출력된 LF→CRLF 문구는 checkout 경고이며 whitespace
  error가 아니다.
- `report.json`: `counter_fallback=True`.
- candidate ground-truth SHA-256:

```text
ground_truth_etw.json       05782177CE0A05E5CB04685B9A417F284439AC527C1DFF9BAAB37B6F690F891A
ground_truth_tcpowner.json  92B3CEC1A834788B8DA88EB233134870223165DD956E24FB3A36235A6E6833D9
ground_truth_wfp.json       06511DA2006E16432E2FF982EFDCC042C6E3F577ADB98C2A0895BBB4314483FD
```

Go cache 접근은 sandbox 외부 승인 실행을 사용했으며 repository source를 변경하지
않았다.

## 5. 잔여 finding

**없음.**

## 6. Gate 결론

**`H-COV1 PASS`.**

이 PASS가 승인하는 것은 다음뿐이다.

1. ETW process attribution 폐기.
2. audit-disabled WFP measured configuration의 보수적 product 후보 제거.
3. direct TCP-owner의 성공한 개별 persistent-endpoint attribution.
4. absolute ratio 없이 five self-observed integer counters를 사용하는
   `counter_fallback=true`.
5. CAP-6 production 검증을 H-SEC2/H-RG3에 남기는 명시적 partial disposition.

따라서 `H-RG2`를 완료 처리할 수 있고, 절차상 `work_status.md`와 paired plans에 이
재검토 PASS를 반영한 뒤 `H-RG3` 진입 조건으로 사용할 수 있다. 향후 absolute
coverage ratio 또는 WFP capability를 다시 도입하려면 새 source-level evidence와
별도 gate가 필요하다.
