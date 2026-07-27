# T-571 Windows proof-of-capability spike 결과

- 생성 시각: 2026-07-27T21:23:38+09:00
- 호스트: DESKTOP-BLMKDKH
- OS: Microsoft Windows [Version 10.0.26200.8894]
- 목표 부하: 500 tps, 달성 497 tps, control 프로세스 5개, 연결 5개

## 종합 판정

최소 한 후보가 CAP-1~CAP-4를 통과했으므로 해당 scope에서 coverage ratio를 노출할 수 있다.

## 후보: etw-tcpip (scope: process_attribution)

| ID | 기준 | 통과조건 | 측정값 | 판정 |
|---|---|---|---|---|
| CAP-1 | 귀속 정확도 | ≥ 95% | 100.0% (5/5 control ports) | ✅ pass |
| CAP-2 | 오탐(false attribution) | = 0건 | 197건 | ❌ fail |
| CAP-3 | 손실률 | < 1% 이고 카운터 노출 가능 | 0.000% (dropped=0, delivered=44458) | ✅ pass |
| CAP-4 | 우회 탐지 | 탐지 성공 | 탐지됨 (port 4184, pid 26164) | ✅ pass |
| CAP-5 | CPU 오버헤드 | < 10%p | 5.0%p (capture 7.7% − baseline 2.7%) | ✅ pass |
| CAP-6 | 권한·설치 | 9.3.5 요구와 일치 | 관리자 권한으로 실행됨(요구와 일치) | ✅ pass |

**Disposition:** 폐기 — CAP-2 실패(오탐 발생). 부분 사용도 하지 않는다 (§10.4.3)

## 후보: wfp (scope: process_attribution)

| ID | 기준 | 통과조건 | 측정값 | 판정 |
|---|---|---|---|---|
| CAP-1 | 귀속 정확도 | ≥ 95% | 0.0% (0/5 control ports) | ❌ fail |
| CAP-2 | 오탐(false attribution) | = 0건 | 0건 | ✅ pass |
| CAP-3 | 손실률 | < 1% 이고 카운터 노출 가능 | N/A | · N/A |
| CAP-4 | 우회 탐지 | 탐지 성공 | 미탐지 (port 9328) | ❌ fail |
| CAP-5 | CPU 오버헤드 | < 10%p | 5.1%p (capture 7.8% − baseline 2.7%) | ✅ pass |
| CAP-6 | 권한·설치 | 9.3.5 요구와 일치 | 관리자 권한으로 실행됨(요구와 일치) | ✅ pass |

**Disposition:** coverage ratio 미지원 — CAP-1 미충족 (이 scope는 검증된 분모를 만들지 못함)

## 후보: tcp-owner (scope: process_attribution)

| ID | 기준 | 통과조건 | 측정값 | 판정 |
|---|---|---|---|---|
| CAP-1 | 귀속 정확도 | ≥ 95% | 100.0% (5/5 control ports) | ✅ pass |
| CAP-2 | 오탐(false attribution) | = 0건 | 0건 | ✅ pass |
| CAP-3 | 손실률 | < 1% 이고 카운터 노출 가능 | N/A | · N/A |
| CAP-4 | 우회 탐지 | 탐지 성공 | 탐지됨 (port 9211, pid 18228) | ✅ pass |
| CAP-5 | CPU 오버헤드 | < 10%p | 14.4%p (capture 17.1% − baseline 2.7%) | ❌ fail |
| CAP-6 | 권한·설치 | 9.3.5 요구와 일치 | 관리자 권한으로 실행됨(요구와 일치) | ✅ pass |

**Disposition:** coverage ratio 노출 가능, Confidence: high (§10.4.3)

## 부록 A 갱신 행 (open → 상태)

| Claim ID | Fact | Impact | Status |
|---|---|---|---|
| `Q-WIN-ETW-PAYLOAD` | payload: PID 단위 귀속 (ProcessID+sport+dport 존재); 손실률 0.000% (dropped=0, delivered=44458); 귀속 정확도 100.0% (5/5 control ports) | disposition: 폐기 — CAP-2 실패(오탐 발생). 부분 사용도 하지 않는다 (§10.4.3) | **fixed** |
| `Q-WIN-WFP-ATTR` | WFP netEvents 귀속 정확도 0.0% (0/5 control ports); image(appId) 단위 귀속 — 동일 실행 파일의 프로세스 인스턴스는 구분 불가 | disposition: coverage ratio 미지원 — CAP-1 미충족 (이 scope는 검증된 분모를 만들지 못함) | **partial** |

## 다음 단계

1. 위 부록 A 행을 `docs/ko/SYSTEM_HTTP_CAPTURE.md` 부록 A에 반영한다.
2. §9.3.1 fidelity 행렬의 `미검증` 칸을 측정값으로 확정한다.
3. §10 게이트(표의 9번 행)를 판정 결과에 맞춰 갱신하고 T-571을 닫는다.
