# T-629 ProfileSet 구현 재리뷰 (R2) — 2026-11-11

- 대상 커밋: archscope `27fcd3d` (fix(profile): address T-629 scale and comparison review findings). 이전 리뷰 대상은 `f06673a`(확인 HEAD `a320881`).
- 이전 리뷰: `docs/review/done/2026-11-11_claude-code_T-629_implementation-review.ko.md` (R4-01~R4-05, 판정 CONDITIONAL)
- 처리 기록: `work_status.md`의 "T-629 independent-review remediation completed locally", 증거 `docs/review/evidence/2026-11-11-t629-review/local.json` (`source_sha256` 23개 항목이 `27fcd3d` 커밋 내용과 모두 일치)
- 작성: claude-code가 재리뷰 작성 역할만 맡았다. 이전 리뷰의 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 수행했다. 구현 코드와 기존 미커밋 HTTP 변경은 수정하지 않았다.
- 방법: 수정 diff(`c7a1570..27fcd3d`) 정독, 저장소 밖 scratch worktree에서 실제 CLI·Go probe 재실행(미커밋). 번호는 이전 리뷰의 R4-xx를 이어 R5-xx로 둔다.

## 1 결론

**판정: 승인에 가까운 조건부(P1·P2 없음, P3 2건).** 이전 P2 2건(R4-01, R4-02)과 P3 3건(R4-03~R4-05)은 모두 재현으로 해소를 확인했다. 신규 finding은 P3 2건이며 T-629 종료를 막지 않는다.

- **T-629 `DONE` 복귀: 가능.** 신규 P3는 해당 영역을 손댈 때 처리하면 된다.
- **T-634 착수:** R4-01이 해소되어 막는 요인 없음. 단 R5-02(capture 옵션 비교)는 producer 정정 때 함께 정하기를 권한다.
- **T-635 착수:** R4-02가 해소되어 막는 요인 없음. 엔티티 매핑 자체는 계약대로 T-635 범위다.
- 실제 WebView IPC/UI, Windows 네이티브 실행, hosted CI는 여전히 미검증이다(4절).

## 2 이전 finding 반영 확인

| ID | 등급 | 판정 | 확인 근거 |
|---|---|---|---|
| R4-01 | P2 | **해소** | 모델을 256 KiB 이하 chunk로 나눠 기존 동기 sink에 쓴다(`catalog/profile_transport.go`, `profile_set.go`). 정책 상한은 올리지 않았다. 실제 CLI: 4.9 MB 모델(3,000 stack)이 이전에는 `IMPORT_BUDGET_EXCEEDED`였으나 이제 성공(1.9초, 결과 7.05 MB). 32.1 MB 모델(20,000 stack)도 성공(12.1초, 결과 44.7 MB). 32 MiB를 넘는 입력은 `IMPORT_BUDGET_EXCEEDED: profile_transport_bytes: profile_transport_bytes 34698552 exceeds 33554432`로 어느 한도인지 숫자와 함께 거절한다. 트리 직렬화는 추정(노드당 256바이트)에서 노드별 실측 합산, 상한 64 MiB로 바뀌었다. |
| R4-02 | P2 | **해소** | 측정 품질 수치는 비교 조건에서 빠지고 양쪽 참조에 보존된다. `lost`만 다른 두 모델이 비교되어 291행을 반환하고 `quality_differences:["lost"]`, `warnings:["quality_measurements_differ"]`가 붙는다. process가 다르면 `entity_mapping_required`, clock 신뢰도가 다르면 `quality_class_mismatch`로 사유 코드와 함께 거절한다. 엔티티 매핑을 임의로 추정하지 않는다. |
| R4-03 | P3 | 해소 | 검증 성공을 저장 결과 내용 해시로 캐시한다(레지스트리당 64개, 변조·다른 내용은 miss). 7 MB 결과의 `Check`가 첫 호출 387 ms에서 이후 22~25 ms로 줄었다(해시 비용 선형). 결과 크기 증폭(모델 4.9 MB → 결과 7.05 MB)은 작성 측 기록에 있고, 문서에는 임시 디스크의 base64 spool 증폭이 반영됐다(결과 중복 저장 계수 자체의 문서화는 확인하지 못했다). |
| R4-04 | P3 | **해소** | `metadata.quality`가 재계산 대조 목록에 들어갔다. 편집된 결과(`lost:"999"`)는 `evidence export`와 `profile-set tree` 모두 `invalid_result: profile result/model projection mismatch`로 거절된다. |
| R4-05 | P3 | 해소(문서화) | 64 KiB 이하만 검증된 감지, 큰 문서는 hint와 명시 선택이라는 한계가 지원표에 기록됐다. |

## 3 신규 finding

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R5-01 | P3 | 모델 안의 단일 문자열이 4 MiB를 넘으면 예산 상세 없이 `IMPORT_FORMAT_MISMATCH`로 거절됨 | `profilemodel/model.go` `Decode`(wirejson 기본 문자열 상한), `catalog/profile_set.go` ValidateInput |
| R5-02 | P3 | capture 옵션을 통째로 비교해 pid·duration 같은 부수 값만 달라도 비교가 거부됨 | `profilemodel/comparison.go` `comparable` |

### R5-01 단일 문자열 4 MiB 상한이 미문서·비상세 (P3)

**근거**: `profilemodel.Decode`는 `wirejson.Decode`의 기본 상한(문자열 하나 4 MiB)을 거친다. 문서의 모델 문자열 상한은 "참조 문자열 합 32 MiB"만 적고 개별 문자열 4 MiB는 없다. `ValidateInput`은 예산 오류가 아닌 모든 디코드 실패를 `IMPORT_FORMAT_MISMATCH`로 바꾼다.

**재현**(실제 CLI, `a.model.json`의 `extensions.x`에 `a`를 채움): 3 MiB는 성공, 5 MiB와 30 MiB는 `IMPORT_FORMAT_MISMATCH: Import stopped; no result was published.`. 같은 모델이 32 MiB 합계 한도 안인데도 형식 오류로 보인다. 원인이 4 MiB 문자열 상한임은 코드 정독과 3 MiB/5 MiB 경계 재현으로 판단했다.

**영향**: 실제 producer가 큰 확장 문자열(예: 큰 원본 메타데이터)을 싣는 경우 형식이 잘못된 것으로 오해한다. 지금은 합성·legacy 입력만 있어 영향이 작다.

**수정 제안**: 문서에 개별 문자열 상한을 적고, 디코드 단계의 `wirejson` 예산 오류를 `BudgetProblem`(resource `json_string_bytes` 등)으로 변환한다. 4 MiB 경계 회귀 테스트를 추가한다.

### R5-02 capture 옵션 전체 비교 (P3)

**근거**: `comparable`은 `capture_options` 전체를 정규화해 동일성만 확인한다(`capture_options_mismatch`). 이 값은 producer가 기록한 불투명 객체라 interval·event 같은 의미 있는 항목과 pid·출력 경로·실행 시간 같은 부수 항목을 구분하지 못한다.

**재현**: 같은 모델의 `capture_options`만 `{"duration_sec":30,"pid":1234}`와 `{"duration_sec":60,"pid":4321}`로 다르게 하면 `capture_options_mismatch: declared capture options differ`로 거부된다. 품질 수치와 process는 R4-02에서 풀렸지만 같은 종류의 문제가 capture 옵션에 남아 있다.

**영향**: 서로 다른 배포 시점의 capture는 pid가 거의 항상 다르므로, producer가 pid를 옵션에 넣는 순간 그 producer의 프로파일은 비교되지 않는다. 사유 코드는 구분되어 있어 사용자는 원인을 알 수 있다.

**수정 제안**: producer adapter(T-634)가 "비교에 의미 있는 옵션 키"를 선언하도록 하고, 비교는 그 키만 대조한다. 나머지 옵션은 양쪽 참조에 보존해 차이를 경고로 표시한다. 선언이 없는 불투명 옵션은 현재처럼 거부한다.

## 4 검증 범위

### 실행한 것 (Mac arm64)

| 항목 | 결과 |
|---|---|
| `go vet ./...` | 통과 |
| `go test -race -count=1 ./...` | 실패·race 0 |
| Windows amd64 `go vet ./internal/analysis ./internal/profilemodel ./internal/catalog` | 통과(교차 vet. 실행 아님) |
| 프런트엔드 `tsc --noEmit`, `npm run test:state` | 통과 |
| 실제 CLI: 4.9 MB·32.1 MB 모델 import, 32 MiB 초과 거절 | 위 2절 |
| 실제 CLI: lost/process/clock/capture 옵션 변형 비교, `metadata.quality` 변조 | 위 2·3절 |
| 검증 캐시 효과(7 MB 결과) | 387 ms → 22~25 ms |
| 증거 `source_sha256` 23개 | 일치 |

### 코드·테스트 정독으로만 확인한 것

chunk 순서·오프셋 검증, 호출자가 낮춘 stack/record 상한 존중, 취소 시 staging 정리, 예산 상세의 JavaScript 안전 정수 초과 값(문자열 전송)은 diff와 작성 측 회귀 테스트(전체 통과)로 확인했고 직접 재현하지 않았다. 32 MiB 한계 근처에서의 메모리 최대치(입력 본문, chunk 재조립, 모델, 트리가 동시에 존재)는 측정하지 않았다.

### 실행하지 않은 것 (기존 미검증 항목과 구분)

- 실제 WebView IPC/UI: 새 ProfileSet 메서드를 호출하는 화면이 없다(T-631).
- Windows 네이티브 실행: 교차 빌드·vet만 있다.
- hosted CI, 실제 producer 입력과 버전 적합성(Q01~Q04 미결정, T-634 범위).
- 32 MiB급 입력의 장시간 처리(12초)에 대한 UI 취소·진행 표시 동작.

## 5 처리 권장 순서

1. T-629의 `DONE` 복귀 판단. P1·P2는 없다.
2. **R5-02**: T-634의 producer adapter 설계에서 의미 있는 capture 옵션 키 선언을 포함한다.
3. R5-01: 해당 영역을 손댈 때 처리한다(문서 한 줄과 오류 변환).
