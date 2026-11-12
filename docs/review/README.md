# 리뷰 문서 인덱스

처리 전 문서는 이 폴더에, 처리 완료 문서는 `done/`에 둔다. 작성자와 처리자는 서로 다른 AI가 맡는다.

## 미처리

| 날짜 | 문서 | 대상 | 판정 |
|---|---|---|---|
| 2026-11-12 | [T-635 정규 프로파일 조회·케이스 검색·재분석 검증 baseline 구현 독립 리뷰](2026-11-12_claude-code_T-635_implementation-review.ko.md) | `befd5f3` | 조건부 (P1·P2 없음, P3 4). 위조 pin·원본 변조·stale·매핑 우회 시나리오가 모두 거부되고 T-634 R11 수정도 확인. R12-01 비교 쪽마다 전체 재분석(쪽당 약 3.4 s), R12-02 top 정렬, R12-03 `password=` 색인 저장, R12-04 결과 64 MiB 상한. T-635 DONE 복귀 허용. |

## 처리 기록

| 날짜 | 문서 | 대상 | 판정·처리 |
|---|---|---|---|
| 2026-11-12 | [T-634 원본 측정 의미 독립 리뷰](done/2026-11-12_claude-code_T-634_implementation-review.ko.md) | `eaa8c54` | CONDITIONAL(P2 1/P3 3). R11-01→R11-02→R11-03/R11-04를 작업표에 기록 후 원문 보관. T-635 baseline 선행 조건으로 normalizer 1.1.0 / payload 0.4.0의 원본 type·unit 및 정책 독립 비교 identity를 추가하고 기존 0.3.0 hash/저장 reader는 보존, 비교는 재분석 필요로 제한. 수치 한도·정책 basis/경고·none 단위도 로컬 수정. 원 R11-01 처리 후 종결 허용에 따라 T-634 DONE; 원 판정은 CONDITIONAL, 최종 독립 PASS는 주장하지 않음. 근거: evidence/2026-11-12-t635/local.json. |
| 2026-11-12 | [T-633 입력 안내·수동 수집·로컬 JFR 변환 구현 독립 리뷰](done/2026-11-12_claude-code_T-633_implementation-review.ko.md) | `3d443d3` | CONDITIONAL(P2 1/P3 4). R10-01→R10-04/R10-03→R10-02→R10-05를 작업표에 반영 후 원문 보관. R10-01–R10-05 로컬 수정·검증 완료. 원 리뷰의 R10-01 수정 후 종결 허용에 따라 T-633 DONE. 원 CONDITIONAL 유지, 최종 수정의 독립 재리뷰·Windows 네이티브 관측은 별도. 근거: evidence/2026-11-12-t633-r10/local.json. |
| 2026-11-12 | [T-632 구현 독립 리뷰](done/2026-11-12_claude-code_T-632_implementation-review.ko.md) | `34dbd8f` | CONDITIONAL(P2 1/P3 3). R9-01→R9-02→R9-03→R9-04를 작업표에 반영 후 원문 보관. R9-01–R9-04 로컬 수정/검증 완료. 원 리뷰의 R9-01 수정 후 종결 허용에 따라 T-632 DONE. 원 CONDITIONAL 유지; 최종 수정의 독립 재리뷰 및 Windows 네이티브 관측은 별도. 근거: evidence/2026-11-12-t632-r9/local.json. |
| 2026-11-11 | [T-631 구현 독립 리뷰](done/2026-11-11_claude-code_T-631_implementation-review.ko.md) | `ff06c54` | CONDITIONAL(P2 1/P3 3). R8-01→R8-02→R8-04→R8-03을 work_status.md 후속 큐에 반영 후 원문 보존. 2026-11-12 R8-01~R8-04 수정·로컬 검증 완료(선형 참조, 제한 캐시, 구조화 거절, 표시 제어문자). 리뷰의 R8-01 처리 후 허용 조건에 따라 T-631 DONE. 원 CONDITIONAL 유지; 최종 수정의 독립 재리뷰/PASS는 주장하지 않음. |
| 2026-11-11 | [T-630 구현 재리뷰 R2](done/2026-11-11_claude-code_T-630_implementation-re-review-R2.ko.md) | `e2c1c7d` | CONDITIONAL(P3 1), R6-01~R6-07 해소 및 DONE 복귀 허용. R7-01을 작업표에 반영한 뒤 원문 보존·이동; 잠금 오류 분리·원인 안내 수정. T-630 DONE, 최종 P3 수정의 독립 재검증·PASS 주장 없음. |
| 2026-11-11 | [T-630 구현 독립 리뷰](done/2026-11-11_claude-code_T-630_implementation-review.ko.md) | `82bc6a8` | CONDITIONAL(P2 2/P3 5). R6-01~R6-07을 우선순위·의존성 순으로 work_status.md에 반영한 뒤 원문 보관. R6-01~R6-07 수정·로컬 검증 완료(경로 식별자, 소유권 cache 정리/재열기, 저장 멱등, 손상 삭제 계획, 이관 안내, 원본 색인, 표시 이름). T-630은 독립 재리뷰 전 REVIEW 유지, 원 판정 CONDITIONAL 보존. T-631 비차단. |
| 2026-11-11 | [T-629 구현 재리뷰 R2](done/2026-11-11_claude-code_T-629_implementation-re-review-R2.ko.md) | `27fcd3d` | CONDITIONAL(P3 2), R4-01~R4-05 해소 및 DONE 복귀 가능 확인. R5-01 수정·검증 완료, R5-02 T-634/T-635 계약·수용 조건 반영(현재 엄격 비교 유지). R2의 명시적 허용에 따라 T-629 DONE. 원문 보존; PASS로 변경하지 않음. |
| 2026-11-11 | [T-629 ProfileSet 구현 독립 리뷰](done/2026-11-11_claude-code_T-629_implementation-review.ko.md) | `f06673a` / 확인 `a320881` | CONDITIONAL(P2 2/P3 3). R4-01~R4-05를 작업표에 우선순위·의존성 순으로 반영한 뒤 원문 보관. 5건 수정·회귀 검증 완료. 3,000-stack/54,001-node 규모, 품질 비교·매핑 거절, 캐시·quality 변조·감지 경계 검증. R2에서 5건 해소 확인 및 DONE 복귀 허용; 최종 R5 처리와 검증은 별도 기록. PASS 선언 없음. |
| 2026-11-11 | [T-628 구현 재재리뷰 R3](done/2026-11-11_claude-code_T-628_implementation-re-review-R3.ko.md) | `6c3717d` | CONDITIONAL(P3만 잔여), N-01~N-04 해소 및 T-628 DONE 복귀 가능 확인. R3-01~R3-03을 작업표에 반영한 뒤 원문 보관; 3건 후속 수정·로컬 검증 완료. R3의 명시적 완료 허용에 따라 T-628 DONE; 최종 P3 수정분의 독립 검증이나 PASS를 주장하지 않음. |
| 2026-11-11 | [T-628 구현 재리뷰 R2](done/2026-11-11_claude-code_T-628_implementation-re-review-R2.ko.md) | `63d3b6a` | CONDITIONAL 유지. N-01~N-04를 작업표에 반영한 뒤 원문 보관. R2 후속 수정·회귀 검증 완료(비교 출처/호환 분리, 공개 해제 제거, EOF 경계, 투영 예산); R3에서 R2 수정분 해소 확인. T-629 비차단; 독립 PASS 선언 없음. |
| 2026-11-11 | [T-628 구현 독립 리뷰](done/2026-11-11_claude-code_T-628_implementation-review.ko.md) | `a698d26` | 원 판정 CONDITIONAL 유지. R-01~R-14를 work_status.md에 반영한 뒤 원문 보존·이동; R-01~R-14 후속 수정·로컬 검증 완료. 독립 재리뷰 전 T-628 REVIEW 유지; 원 리뷰의 R-01 조건을 충족하여 T-629 착수 가능. |
