# T-628 구현 재재리뷰 (R3) — 2026-11-11

- 대상 커밋: archscope `6c3717d` (fix(ingestion): address T-628 R2 comparison and resource findings). 문서 후속 `e209778`도 포함해 확인했다.
- 이전 리뷰: `docs/review/done/2026-11-11_claude-code_T-628_implementation-re-review-R2.ko.md` (N-01~N-04, 판정 CONDITIONAL)
- 처리 기록: `work_status.md`의 "T-628 R2 remediation completed locally", 증거 `docs/review/evidence/2026-11-11-t628-r2/local.json` (`source_sha256` 21개 항목이 `6c3717d` 커밋 내용과 모두 일치)
- 작성: claude-code가 재재리뷰 작성 역할만 맡았다. R2의 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 수행했다. 구현 코드와 기존 미커밋 HTTP 변경은 수정하지 않았다.
- 범위: R2 지적 4건의 수정과 그 부작용. 같은 기간 병합된 T-629(ProfileSet)는 별도 작업이므로 평가하지 않는다. 다만 `catalog.Builtin()`에 profile importer가 함께 등록되어 전체 테스트에는 포함됐다.
- 방법: 수정 diff(`63d3b6a..6c3717d`) 정독, 저장소 밖 scratch worktree에서 재실행·재현 테스트(미커밋). 이 문서는 `done/`이나 `work_status.md` 본문을 수정하지 않는다.

## 1 결론

**판정: 승인에 가까운 조건부(P3만 남음).** R2의 P2(N-01)와 P3 3건은 코드와 재현으로 해소를 확인했다. 신규 P1·P2 finding은 없다. P3 3건이 남았고 어느 것도 T-628 완료를 막지 않는다.

- **T-628 `DONE` 복귀: 가능.** 단 P3 R3-01은 T-631 compare provider 전에 닫기를 권한다.
- **T-629·T-630·T-631:** 이 리뷰 범위에서 막는 요인 없음.
- 실제 WebView IPC/UI, Windows 네이티브 실행, hosted CI는 여전히 미검증이다(4절).

## 2 R2 finding 반영 확인

| ID | 등급 | 판정 | 확인 근거 |
|---|---|---|---|
| N-01 | P2 | **해소** | 비교 가능성(선택 해시·커버리지 종류·semantic revision)과 관측 대상 식별(source hash)을 분리했다(`resultcontract/comparison.go`). 서로 다른 로그끼리 비교가 성공한다: before/after 로그에서 `/summary/total_events` delta 1, percent 100/3(33.3%), `mode:"baseline_target"`. 순서를 바꾸면 -1/-25로 부호가 따라간다. 응답에 baseline·target handle, 저장 결과 sha256, 원본 sha256이 들어 있다. |
| N-02 | P3 | 해소(안전장치) | `ReleaseEvidenceResult`를 Wails와 생성 바인딩에서 제거했고 반사 테스트가 없음을 단정한다(`evidenceservice_test.go:261`). `Service.Release`는 내부 관리 연산으로 남는다. 참조 소유권 자체는 T-630. |
| N-03 | P3 | 해소(잔여 소결함) | 1바이트 lookahead로 정확히 EOF인 경우를 구분한다. 3줄 파일에 `max_lines:3`을 주면 줄바꿈 유무와 관계없이 coverage 표지가 없다. 빈 줄만 남은 경우는 R3-03. |
| N-04 | P3 | 해소 | 항목 100,000개·부록 16 MiB 상한. 사전 계산 단계에서 슬라이스 길이로 거부하고 `EXPORT_PROJECTION_BUDGET_EXCEEDED`를 반환한다. 원본 결과를 변경하지 않는다. |

N-01 동작 재현 결과(같은 옵션 기준, `TestR3Compare`):

| 사례 | 결과 |
|---|---|
| 서로 다른 로그, 같은 옵션 | 성공, delta 1 |
| 같은 handle | `same_input`, delta 0 |
| 선택 옵션이 다른 두 결과 | 거부 |
| 완료 결과 vs `max_lines` 결과 | 거부 |
| `max_lines:2` 두 로그 | 성공(앞 2줄 비교), `coverage_kind:"selected_records:max_lines"` |
| 부분(partial) 결과, 다른 원본 | 거부(fail closed) |
| 부분 결과, 같은 원본 | 성공(`same_input`) |
| 기본값을 명시한 옵션 vs 생략 | 거부(옵션 해시가 다름. 문서화된 보수적 동작) |

## 3 신규 finding

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R3-01 | P3 | 비교 거부 사유가 하나의 일반 오류로 뭉뚱그려져 원인을 구분할 수 없음 | `resultcontract/comparison.go:72` |
| R3-02 | P3 | 비교 가능성 정보가 저장 파일의 자기 선언이며 원본·옵션과의 결속을 검증하지 않음 | `resultcontract/contract.go` `populationIdentity`, `analysis/results.go` Load |
| R3-03 | P3 | `max_lines`의 선택 표지가 빈 줄만 남은 경우에도 붙음 | `parsers/serverlog/importer.go:83-92` |

### R3-01 거부 사유가 구분되지 않음 (P3)

**근거**: 서로 다른 선택 옵션, 커버리지 종류 불일치, partial 결과, semantic revision 불일치, 정책 버전 미지원이 모두 `population comparison policy mismatch; original reanalysis may be required` 한 문장으로 반환된다(`comparison.go:72`).

**재현**(`TestR3Compare`): 서로 다른 옵션, 완료 vs `max_lines`, 기본값 명시 vs 생략, partial 교차 비교의 오류 문자열이 동일하다. `Delta`에 사유 필드도 없다.

**영향**: T-631 UI가 사용자에게 "다른 옵션으로 다시 가져오라"와 "partial은 아직 비교 불가"를 구분해 안내하지 못한다. 특히 기본값 명시 vs 생략은 사용자가 이유를 알 수 없다.

**수정 제안**: 안정적인 사유 코드를 둔다. 예: `selection_mismatch`, `coverage_mismatch`, `partial_not_comparable`, `semantic_revision_mismatch`, `policy_unsupported`, `policy_missing`. 오류 본문은 바꾸지 않고 코드만 추가해도 된다. 각 사유에 대한 테스트를 추가한다.

### R3-02 비교 가능성이 자기 선언임 (P3)

**근거**: `populationIdentity`는 `metadata.populations`의 문법(해시 형식, 빈 값 여부)만 검증한다. `analysis.Service.Load`는 저장된 결과 파일을 envelope·schema로만 검증하고 받아들인다. 그래서 편집된 저장 파일이 임의의 `source_sha256`, 유효한 형식의 `selection_hash`, 지원되는 `comparison` 정책을 선언하면 비교 가능한 모집단으로 취급된다. 응답이 결과 sha256을 기록하더라도 그 값은 선언의 진위를 보증하지 않는다. 코드 정독으로 확인했고 편집 파일 재현은 하지 않았다.

**영향**: 개인 로컬 도구에서 사용자가 자신의 파일을 편집하는 위험이라 현재 위협은 낮다. 다만 T-635가 baseline을 durable하게 저장하고 보고서에 "비교 가능"을 인용하게 되면 이 비교가 검증된 사실처럼 보일 수 있다.

**수정 제안**: 비교 증거에 "선언된 정체성이며 원본 재계산으로 검증되지 않음"을 명시하는 필드를 둔다(예: `identity_basis: "declared"` vs 원본이 있을 때 `"verified_by_reimport"`). 또는 T-635에서 baseline 저장 시 import provenance와 결과 해시를 결속한다. 어느 쪽이든 문서에 한계를 적는다.

### R3-03 빈 줄만 남은 경우의 선택 표지 (P3)

**근거**: lookahead가 "다음 바이트가 존재하는가"만 보므로 `max_lines` 뒤에 빈 줄만 남아도 `selection:"max_lines"`가 붙는다.

**재현**(`TestR3MaxLinesEdges`, 3줄 + `max_lines:3`): 줄바꿈 유무에 관계없이 정확히 3줄이면 표지가 없다(해소). 3줄 뒤에 `\n` 또는 `\r\n` 한 줄이 더 있으면 `coverage={"complete_records":3,"partial":false,"selection":"max_lines"}`가 기록되고, 비교 정책도 `selected_records:max_lines`로 바뀐다.

**영향**: 실제로 제외된 레코드가 없는데 정책이 달라져, 같은 내용의 파일끼리 `complete_records`와 `selected_records:max_lines`로 갈려 비교가 거부될 수 있다. legacy는 빈 줄을 레코드로 세지 않으므로 수치 parity에는 영향이 없다.

**수정 제안**: lookahead를 공백만 있는 줄을 건너뛰고 다음 비어 있지 않은 줄을 확인하도록 바꾸거나, 표지를 "옵션이 적용됨"으로 정의해 문서화한다. 후자가 단순하다면 정책 이름을 `option_applied:max_lines`처럼 바꾸는 것도 방법이다.

## 4 검증 범위

### 실행한 것 (Mac arm64)

| 항목 | 결과 |
|---|---|
| `go vet ./...` | 통과 |
| `go test -race -count=1 ./...` | 실패·race 0 |
| Windows amd64 `go vet ./internal/analysis ./internal/resultcontract` | 통과(교차 vet. 실행 아님) |
| 프런트엔드 `tsc --noEmit`, `npm run test:state` | 통과 |
| 재현 테스트 2건(scratch worktree, 미커밋) | R3-01·R3-03 재현, N-01 동작 8개 사례 확인 |
| 증거 파일 `source_sha256` 21개 | 일치 |

### 코드·테스트 정독으로만 확인한 것

N-02의 반사 테스트와 N-04의 상한 로직은 diff와 작성 측 회귀 테스트로 확인했고 직접 재현하지는 않았다. 100,000개 항목에 가까운 투영의 실제 크기·지연은 측정하지 않았다. R3-02는 코드 정독이다. ProfileSet(T-629) 비교(작성 측이 보고한 다른 원본 delta 0.05)는 이 리뷰의 범위가 아니라 확인하지 않았다.

### 실행하지 않은 것 (기존 미검증 항목과 구분)

- 실제 WebView IPC/UI: 새 import·비교 API를 호출하는 화면이 없다(T-631).
- Windows 네이티브 실행: 저장소 잠금·rename·삭제·cache 경로는 미확인. 교차 vet까지만.
- hosted CI, 실제 producer 로그, 장시간 부하.
- 저장소 상한(1,024개·512 MiB) 도달과 재시작 시 해시 계산 비용.

## 5 처리 권장 순서

1. T-628을 `DONE`으로 복귀하는 판단. 현재 P1·P2는 없다.
2. **R3-01**: T-631 compare provider 전에 사유 코드를 추가한다.
3. R3-02: T-635 baseline 설계에 "선언 vs 검증" 구분을 넣는다. 그때까지 문서에 한계를 적는다.
4. R3-03: 해당 영역을 손댈 때 처리한다.
