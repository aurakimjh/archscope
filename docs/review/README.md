# 리뷰 문서 인덱스

처리 전 문서는 이 폴더에, 처리 완료 문서는 `done/`에 둔다. 작성자와 처리자는 서로 다른 AI가 맡는다.

## 미처리

| 날짜 | 문서 | 대상 | 판정 |
|---|---|---|---|
| 2026-11-11 | [T-629 ProfileSet 구현 독립 리뷰](2026-11-11_claude-code_T-629_implementation-review.ko.md) | `f06673a` (확인 HEAD `a320881`) | 조건부 (P2 2, P3 3). legacy 호환·정밀 수치 충족; 실효 규모·비교 identity 보완 필요. |

## 처리 기록

| 날짜 | 문서 | 대상 | 판정·처리 |
|---|---|---|---|
| 2026-11-11 | [T-628 구현 재재리뷰 R3](done/2026-11-11_claude-code_T-628_implementation-re-review-R3.ko.md) | `6c3717d` | CONDITIONAL(P3만 잔여), N-01~N-04 해소 및 T-628 DONE 복귀 가능 확인. R3-01~R3-03을 작업표에 반영한 뒤 원문 보관; 3건 후속 수정·로컬 검증 완료. R3의 명시적 완료 허용에 따라 T-628 DONE; 최종 P3 수정분의 독립 검증이나 PASS를 주장하지 않음. |
| 2026-11-11 | [T-628 구현 재리뷰 R2](done/2026-11-11_claude-code_T-628_implementation-re-review-R2.ko.md) | `63d3b6a` | CONDITIONAL 유지. N-01~N-04를 작업표에 반영한 뒤 원문 보관. R2 후속 수정·회귀 검증 완료(비교 출처/호환 분리, 공개 해제 제거, EOF 경계, 투영 예산); R3에서 R2 수정분 해소 확인. T-629 비차단; 독립 PASS 선언 없음. |
| 2026-11-11 | [T-628 구현 독립 리뷰](done/2026-11-11_claude-code_T-628_implementation-review.ko.md) | `a698d26` | 원 판정 CONDITIONAL 유지. R-01~R-14를 work_status.md에 반영한 뒤 원문 보존·이동; R-01~R-14 후속 수정·로컬 검증 완료. 독립 재리뷰 전 T-628 REVIEW 유지; 원 리뷰의 R-01 조건을 충족하여 T-629 착수 가능. |
