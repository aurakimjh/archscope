# 리뷰 문서 인덱스

처리 전 문서는 이 폴더에, 처리 완료 문서는 `done/`에 둔다. 작성자와 처리자는 서로 다른 AI가 맡는다.

## 미처리

없음.

## 처리 기록

| 날짜 | 문서 | 대상 | 판정·처리 |
|---|---|---|---|
| 2026-11-11 | [T-630 구현 독립 리뷰](done/2026-11-11_claude-code_T-630_implementation-review.ko.md) | `82bc6a8` | CONDITIONAL(P2 2/P3 5). R6-01~R6-07을 우선순위·의존성 순으로 work_status.md에 반영한 뒤 원문 보관. 수정 미착수, T-630 REVIEW 유지. T-631 비차단; R6-02는 T-635 전에 해소. |
| 2026-11-11 | [T-629 구현 재리뷰 R2](done/2026-11-11_claude-code_T-629_implementation-re-review-R2.ko.md) | `27fcd3d` | CONDITIONAL(P3 2), R4-01~R4-05 해소 및 DONE 복귀 가능 확인. R5-01 수정·검증 완료, R5-02 T-634/T-635 계약·수용 조건 반영(현재 엄격 비교 유지). R2의 명시적 허용에 따라 T-629 DONE. 원문 보존; PASS로 변경하지 않음. |
| 2026-11-11 | [T-629 ProfileSet 구현 독립 리뷰](done/2026-11-11_claude-code_T-629_implementation-review.ko.md) | `f06673a` / 확인 `a320881` | CONDITIONAL(P2 2/P3 3). R4-01~R4-05를 작업표에 우선순위·의존성 순으로 반영한 뒤 원문 보관. 5건 수정·회귀 검증 완료. 3,000-stack/54,001-node 규모, 품질 비교·매핑 거절, 캐시·quality 변조·감지 경계 검증. R2에서 5건 해소 확인 및 DONE 복귀 허용; 최종 R5 처리와 검증은 별도 기록. PASS 선언 없음. |
| 2026-11-11 | [T-628 구현 재재리뷰 R3](done/2026-11-11_claude-code_T-628_implementation-re-review-R3.ko.md) | `6c3717d` | CONDITIONAL(P3만 잔여), N-01~N-04 해소 및 T-628 DONE 복귀 가능 확인. R3-01~R3-03을 작업표에 반영한 뒤 원문 보관; 3건 후속 수정·로컬 검증 완료. R3의 명시적 완료 허용에 따라 T-628 DONE; 최종 P3 수정분의 독립 검증이나 PASS를 주장하지 않음. |
| 2026-11-11 | [T-628 구현 재리뷰 R2](done/2026-11-11_claude-code_T-628_implementation-re-review-R2.ko.md) | `63d3b6a` | CONDITIONAL 유지. N-01~N-04를 작업표에 반영한 뒤 원문 보관. R2 후속 수정·회귀 검증 완료(비교 출처/호환 분리, 공개 해제 제거, EOF 경계, 투영 예산); R3에서 R2 수정분 해소 확인. T-629 비차단; 독립 PASS 선언 없음. |
| 2026-11-11 | [T-628 구현 독립 리뷰](done/2026-11-11_claude-code_T-628_implementation-review.ko.md) | `a698d26` | 원 판정 CONDITIONAL 유지. R-01~R-14를 work_status.md에 반영한 뒤 원문 보존·이동; R-01~R-14 후속 수정·로컬 검증 완료. 독립 재리뷰 전 T-628 REVIEW 유지; 원 리뷰의 R-01 조건을 충족하여 T-629 착수 가능. |
