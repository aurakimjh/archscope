# T-571 Windows proof-of-capability spike

`docs/ko/SYSTEM_HTTP_CAPTURE.md` §10.4 에서 정의한 Windows 커버리지 실측
스파이크의 실행 하네스다. **설계(§10.4의 계약)는 2026-07-20에 이미 닫혔고,
남은 것은 "실측 그 자체"** 다 — 이 하네스가 그 측정을 자동화한다.

이 스파이크가 채우는 것:

- 부록 A 의 `Q-WIN-ETW-PAYLOAD`, `Q-WIN-WFP-ATTR` 행 (`open` → `fixed`/`partial`)
- §9.3.1 fidelity 행렬의 `미검증` 칸
- §10 게이트(9번 행)의 판정

## 무엇을 측정하나

§10.4.1 의 세 후보를 각각 **독립적으로** 측정하고, §10.4.2 의 `CAP-1`~`CAP-6`
을 수치로 기록한다. loadgen 이 만드는 **알려진 대조군**(PID·local port 를 다 아는
프로세스들)을 정답지 삼아 각 probe 의 귀속을 채점한다.

| 후보 | scope | probe | 귀속 근거 |
|---|---|---|---|
| ETW TCP/IP | process attribution | `etwprobe` | Kernel-Network provider 이벤트의 ProcessID (`V-WIN-ETW-TCPIP`) |
| WFP | process attribution | `wfpprobe` | `netsh wfp` netEvents 의 appId(이미지 경로) |
| TCP endpoint ownership | process attribution | `tcpownerprobe` | `Get-NetTCPConnection` 의 OwningProcess (교차검증 기준선) |
| Npcap | flow 5-tuple | (선택) | 5-tuple 만; **PID는 원리적으로 없음** — 별도 설치 필요 |

Npcap 은 별도 설치가 필요해 기본 실행에는 빠져 있다. 실행하려면 packet 관측
결과를 `results/obs_npcap.json` (scope `flow_5tuple`) 로 만들어 두면 judge 가
자동으로 포함한다.

## 요구사항

- Windows 10 2004+ / Windows Server 2022+ (logman, tracerpt, netsh wfp,
  expand, typeperf, Get-NetTCPConnection 는 모두 OS 기본 포함)
- Go 1.26.x (리포 toolchain)
- **관리자 권한 PowerShell** — 커널 ETW 세션과 WFP 캡처는 elevated 토큰 필수

추가로 설치할 것은 없다. probe 는 전부 OS 내장 도구를 감싼다.

## 실행

관리자 권한 PowerShell 에서:

```powershell
cd spikes\t571-windows-coverage
.\run-spike.ps1 -Window 30 -Tps 500 -Workers 5
```

`run-spike.ps1` 이 하는 일:

1. elevated 확인
2. 6개 바이너리 빌드 (`bin\`)
3. idle baseline CPU 샘플 (CAP-5 delta 용)
4. ETW → WFP → TCP-owner 순으로, 각 패스마다 probe 를 띄우고 loadgen 으로
   500 tps 부하를 준 뒤 `results\obs_*.json` 수집
5. judge 실행 → `results\report.md`, `results\report.json`

## 결과물

- `results\report.md` — `CAP-1`~`CAP-6` 판정표, 후보별 disposition(§10.4.3),
  종합 판정(counter fallback 여부), 부록 A 갱신 행
- `results\report.json` — 같은 내용의 기계 판독용

## CAP 기준 (§10.4.2)

| ID | 기준 | 통과 |
|---|---|---|
| CAP-1 | 귀속 정확도 | ≥ 95% |
| CAP-2 | 오탐 | 0건 (실패 시 후보 **폐기**) |
| CAP-3 | 손실률 | < 1%, 카운터 노출 가능 |
| CAP-4 | 우회 탐지 | proxy-bypass 프로세스 탐지 성공 |
| CAP-5 | CPU 오버헤드 | < 10%p (capture − baseline) |
| CAP-6 | 권한·설치 | 9.3.5 요구와 일치 |

disposition 은 §10.4.3 을 그대로 따른다: **CAP-2 실패 = 폐기**,
CAP-1~4 통과 = ratio 노출(high), CAP-1 통과·CAP-3 실패 = ratio+손실률(medium),
**전 후보 실패 = 절대 ratio 제거, 5개 카운터만 유지**. 마지막 경우가 T-571의
실질적 안전장치이며, judge 가 `counter_fallback: true` 로 표시한다.

## 개별 도구 (수동 실행)

```powershell
# 부하만
bin\loadgen.exe -role parent -tps 500 -workers 5 -duration 30s -out results\ground_truth.json

# ETW probe만 (다른 창에서 loadgen 과 동시에)
bin\etwprobe.exe -window 30s -out results\obs_etw.json

# 판정만 다시
bin\judge.exe -dir results -baseline-cpu 1.5 -out results\report.json -md results\report.md

# CAP-4 수동 확인 (시스템 프록시가 설정된 환경에서 프록시를 무시하는 클라이언트)
bin\bypassclient.exe -target some-host:80 -count 20
```

## 설계상의 정직성 장치

이 하네스는 §10 의 목적("조용한 누락을 드러낸다")을 스스로도 지킨다:

- **probe 가 실행되지 못하면 pass 도 fail 도 아니다** — judge 는 `미측정`으로
  기록하고 해당 부록 A 행을 `open` 으로 남긴다. 못 잰 것을 통과로 위장하지 않는다.
- **WFP 는 appId(이미지) 단위 귀속**이라 동일 실행 파일의 프로세스 인스턴스를
  구분하지 못한다. judge 는 이 한계를 CAP-1 detail 에 명시한다.
- **TCP-owner 는 폴링**이라 poll 간격보다 짧은 연결을 못 본다. loadgen 워커가
  persistent keep-alive 연결을 유지하는 이유이며, 이 구조적 누락은 note 로 남긴다.
- **검증 못 한 분모로 ratio 를 만들지 않는다** — 어떤 후보도 통과 못 하면
  judge 가 절대 ratio 를 제거하고 카운터만 남긴다.

### Loopback caveat

`-Target` 없이 실행하면 loadgen 은 loopback(127.0.0.1) listener 를 쓴다.
loopback 트래픽의 귀속·관측은 실제 NIC 경로와 다를 수 있다(특히 Npcap 은 별도
loopback 어댑터 필요). **출시 판정용 최종 측정은 `-Target <다른 호스트:포트>`
로 실제 NIC 를 경유해 다시 확인**하는 것을 권장한다. loopback 결과는 1차
스모크로만 취급한다.

## 판정 후

`report.md` 의 부록 A 행을 `docs/ko/SYSTEM_HTTP_CAPTURE.md` 부록 A 에 반영하고,
§9.3.1 의 `미검증` 칸과 §10 게이트(9번 행)를 갱신한 뒤 T-571 을 닫는다.
영문 문서 parity(T-576)는 그 다음이다.
