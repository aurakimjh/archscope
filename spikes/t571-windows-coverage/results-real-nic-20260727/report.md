# T-571 Windows proof-of-capability spike 결과

- 생성 시각: 2026-07-27T22:25:11+09:00
- 호스트: DESKTOP-BLMKDKH
- OS: Microsoft Windows [Version 10.0.26200.8894]
- 목표 부하: 500 tps, 달성 497 tps, control 프로세스 5개, 연결 5개

## 종합 판정

어떤 후보도 CAP-1~CAP-4를 통과하지 못했다. 절대 coverage ratio를 제거하고 captured/passthrough/unattributed/dropped/unsupported 5개 카운터만 유지한다 (§10.1.2).

> **counter fallback = true** — 절대 coverage ratio를 제거하고 5개 카운터만 노출한다.

## 후보: etw-tcpip (scope: process_attribution)

| ID | 기준 | 통과조건 | 측정값 | 판정 |
|---|---|---|---|---|
| CAP-1 | 귀속 정확도 | ≥ 95% | 100.0% (5/5 control ports) | ✅ pass |
| CAP-2 | 오탐(false attribution) | = 0건 | 197건 | ❌ fail |
| CAP-3 | 손실률 | < 1% 이고 카운터 노출 가능 | 0.000% (dropped=0, delivered=44458) | ✅ pass |
| CAP-4 | 우회 탐지 | 탐지 성공 | 탐지됨 (port 4184, pid 26164) | ✅ pass |
| CAP-5 | CPU 오버헤드 | < 10%p | 4.5%p (capture 7.7% − baseline 3.2%) | ✅ pass |
| CAP-6 | 권한·설치 | 9.3.5 요구와 일치 | 부분 확인 — 관리자 권한 실행; production 계약은 H-SEC2 이관 | · N/A |

**Disposition:** 폐기 — CAP-2 실패(오탐 발생). 부분 사용도 하지 않는다 (§10.4.3)

## 후보: wfp (scope: process_attribution)

| ID | 기준 | 통과조건 | 측정값 | 판정 |
|---|---|---|---|---|
| CAP-1 | 귀속 정확도 | ≥ 95% | N/A — measured configuration에서 relevant ALLOW 이벤트 미관측 | · N/A |
| CAP-2 | 오탐(false attribution) | = 0건 | N/A — measured configuration에서 relevant ALLOW 이벤트 미관측 | · N/A |
| CAP-3 | 손실률 | < 1% 이고 카운터 노출 가능 | N/A | · N/A |
| CAP-4 | 우회 탐지 | 탐지 성공 | N/A — measured configuration에서 relevant ALLOW 이벤트 미관측 | · N/A |
| CAP-5 | CPU 오버헤드 | < 10%p | 4.6%p (capture 7.8% − baseline 3.2%) | ✅ pass |
| CAP-6 | 권한·설치 | 9.3.5 요구와 일치 | 부분 확인 — 관리자 권한 실행; production 계약은 H-SEC2 이관 | · N/A |

**Disposition:** 측정 구성 미지원 — WFP를 제품 coverage 후보에서 제거; audit-enabled capability는 주장하지 않음

## 후보: tcp-owner (scope: process_attribution)

| ID | 기준 | 통과조건 | 측정값 | 판정 |
|---|---|---|---|---|
| CAP-1 | 귀속 정확도 | ≥ 95% | 100.0% (5/5 control ports) | ✅ pass |
| CAP-2 | 오탐(false attribution) | = 0건 | 0건 | ✅ pass |
| CAP-3 | 손실률 | < 1% 이고 카운터 노출 가능 | N/A | · N/A |
| CAP-4 | 우회 탐지 | 탐지 성공 | 탐지됨 (port 7802, pid 18516) | ✅ pass |
| CAP-5 | CPU 오버헤드 | < 10%p | 4.8%p (capture 8.0% − baseline 3.2%) | ✅ pass |
| CAP-6 | 권한·설치 | 9.3.5 요구와 일치 | 부분 확인 — 관리자 권한 실행; production 계약은 H-SEC2 이관 | · N/A |

**Disposition:** 개별 endpoint 귀속만 승인 — CAP-3 미측정; absolute coverage ratio 금지

## 부록 A 갱신 행 (open → 상태)

| Claim ID | Fact | Impact | Status |
|---|---|---|---|
| `Q-WIN-ETW-PAYLOAD` | Event/System/Execution@ProcessID header PID + event payload sport/dport 관측; 손실률 0.000% (dropped=0, delivered=44458); 귀속 정확도 100.0% (5/5 control ports) | disposition: 폐기 — CAP-2 실패(오탐 발생). 부분 사용도 하지 않는다 (§10.4.3) | **fixed** |
| `Q-WIN-WFP-ATTR` | measured configuration에서 relevant ALLOW 이벤트 미관측; audit-enabled capability는 주장하지 않음 | disposition: 측정 구성 미지원 — WFP를 제품 coverage 후보에서 제거; audit-enabled capability는 주장하지 않음 | **fixed** |

## 다음 단계

1. 위 부록 A 행을 `docs/ko/SYSTEM_HTTP_CAPTURE.md` 부록 A에 반영한다.
2. §9.3.1 fidelity 행렬의 `미검증` 칸을 측정값으로 확정한다.
3. 정규화 evidence, source-level 원본 미보존 한계, scope별 disposition을 독립 H-COV1 검토에 제출한다.
4. H-COV1 `PASS` 후에만 §10 게이트와 T-571을 닫는다.
