# T-635 정규 프로파일 조회·케이스 검색·재분석 검증 baseline 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-635 구현 커밋 `befd5f3`(feat(evidence): implement T-635 queries and verified baselines). 직전 `1847b77`(T-634 리뷰) 위에 놓이며, 이 커밋은 T-634 R11 후속 수정(source normalizer 1.1.0, payload 0.4.0)도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-635 행과 수용 조건(R3-02, R5-02 포함), `docs/ko/CASE_EVIDENCE_QUERIES.md`, `docs/ko/DATA_MODEL.md`·`PARSER_DESIGN.md`·`PROFILE_SOURCE_IMPORT.md`의 T-635 절
- 증거: `docs/review/evidence/2026-11-12-t635/local.json`(`source_sha256` 54개와 `evidence_sha256` 26개가 `befd5f3` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`analysis/case_baselines.go`, `analysis/profile_query.go`, `profilemodel/query.go`·`mapping.go`, `cases/search.go`, `cmd_case_query.go`), 저장소 밖 scratch worktree(`befd5f3`)에서 빌드한 실제 CLI로 격리 case store를 만들어 `case create/edit/save-result`, `profile-query`, `case-query search/pin-baseline/compare-baseline`을 끝까지 실행. 직접 수집한 실제 Go 1.26 `runtime/pprof` 프로파일과 합성 Speedscope 원본, 텍스트 로그를 사용했고 변조·위조 시나리오를 재현했다.
- 범위: 정규 profile query 1.0.0(selection/result/query 고정 cursor, 분모, 시간창), 케이스 검색 색인·적용 범위 공개, 원본 재분석 검증 baseline pin과 비교, 명시적 entity/capture 매핑. 새 producer 파서와 시계 정렬은 범위 밖이다. T-634 R11 수정은 확인 항목으로만 다룬다.

## 1 결론

**판정: 조건부(P3만 남음).** 핵심 계약이 실제 실행에서 모두 성립했다. 위조한 baseline pin 3종(모델 해시·selection·정책 변조)이 전부 `BASELINE_BINDING_CHANGED`로 거부되고, 케이스 저장소의 원본 개체를 한 바이트 바꾸면 `CASE_HASH_MISMATCH`로 거부되며, 원본을 교체하면 결과가 stale 처리되어 pin과 비교가 모두 막힌다. 검증 표지 `verified_by_reimport`는 저장된 입력의 선언이 아니라 원본 재분석에서만 나오고 비교의 `identity_basis`는 `declared`로 유지된다. 쿼리 분모는 필터와 무관하게 정확히 유지되고 cursor는 결과·질의·case 변경에 모두 무효화된다. 앞선 T-634 리뷰의 R11-01~R11-03 수정도 실제로 동작한다. 남은 것은 성능·표시·한도에 관한 P3 4건이며 P1·P2는 없다.

- **T-635 `DONE` 판정:** 이 리뷰는 차단 항목이 없으므로 R12-01~R12-04를 후속 큐에 넣고 `DONE` 복귀를 허용한다. 실제 WebView UI와 Windows 네이티브 검증은 별도 수용 항목이다.
- **후속 작업:** T-636·T-638·T-639는 R12-01(반복 재분석 비용)이 규모 있는 비교 화면에 영향을 주므로 그 전에 처리하는 편이 좋다. 막지는 않는다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 정확한 분모, 모든 보기에서 같은 선택 | 충족 | 실제 Go heap `alloc_space` 선택에서 `denominator = 3529624`(= `go tool pprof` 합계). `top`을 7행씩 23쪽 끝까지 넘기면 161행·중복 없음, self 합이 분모와 같다. `evidence` 보기는 109건, `tree`는 238행. |
| 시간창은 시작 timestamp 반개구간, 분모 불변 | 충족 | 합성 evented Speedscope(전체 100.5 ms, 7건)에서 `[10,30)`→2건(`10`, `20.25`), `[20.25,30)`→1건, `[60,100.5)`→1건, `[0,0)`→0건이며 모든 질의의 `denominator`는 `100.5`. `from>to`는 `PROFILE_TIME_WINDOW_INVALID`. 시각 없는 pprof에 창을 주면 `matched 0`과 `untimed_observations 109`를 공개한다. |
| cursor는 결과·질의 고정 | 충족 | 같은 질의의 cursor는 유효하고, limit·view·다른 결과를 바꾼 cursor는 `PROFILE_CURSOR_STALE`. 검색 cursor는 text 변경·원본 교체 뒤 `CASE_CURSOR_STALE`. |
| 케이스 검색: 범위·누락 공개 | 충족 | 응답의 `coverage`가 원본·결과별 `indexed/total`을 담고 바이너리 pprof 원본은 `text_decoder_unavailable`(indexed 0)로 정직하게 표시한다. limit 0/201, 알 수 없는 필드, 오래된 revision은 모두 거부. 5,000표본 두 결과에서 `fn7` 검색 최초 1.4 s, 캐시 후 0.11 s. |
| baseline pin은 원본 재분석으로만 검증 | 충족 | `pin-baseline`이 원본과 복구한 옵션으로 정규화를 다시 실행해 저장 모델과 전체 대조한다. 결과에 묶이지 않은 원본(`BASELINE_ORIGINAL_NOT_BOUND`, 텍스트 로그 포함)은 거부. 편집한 pin(모델 해시·selection·정책 null)은 비교 시 `BASELINE_BINDING_CHANGED`. |
| 원본·결과 무결성 | 충족 | 관리 개체 파일의 중간 바이트 1비트 변조 → `CASE_HASH_MISMATCH: source changed; reimport explicitly`, 원복하면 `verified_by_reimport`. `replace-source` 후 결과 stale → pin·비교 모두 `BASELINE_RESULT_MISSING_OR_STALE`. |
| 명시적 entity 매핑이 strict 조건을 우회하지 못함 | 충족 | worker 이름이 다른 두 결과: 매핑 없음 `entity_mapping_required`, 완전한 1:1 매핑+사유는 성공(`warnings: ["capture_policy_uses_declared_producer","entity_mapping_declared"]`, 응답에 매핑 기록), 틀린 tuple·빈 사유는 `entity_mapping_invalid`, 매핑을 줘도 `rate` 차이는 `capture_options_mismatch`. |
| 비교 페이지·크기 상한 | 충족(코드·실행) | 기본 50행·최대 200행·1 MiB, cursor는 질의 해시에 묶임. |
| T-634 R11 수정 확인 | 충족 | `inspect-source`의 metric에 `source_type`(`alloc_objects`, `alloc_space` …)와 `source_unit`이 추가됨. 정책 없는 `alloc_space`↔`inuse_space` diff가 이제 `measurement_mismatch`로 거부됨. 15만표본 거부 메시지가 `observations=100001 (limit 100000) … reduce collection duration/rate/workers or explicitly split the source`로 원인을 설명. 선언 producer 기반 정책은 `capture_policy_uses_declared_producer` 경고로 드러남. |
| 증거 무결성 | 충족 | `source_sha256` 54개, `evidence_sha256` 26개가 커밋 내용과 모두 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R12-01 | P3 | 비교 페이지마다 기준·대상 원본을 전부 재분석함(5,000표본에서 쪽당 약 3.4 s) | `analysis/case_baselines.go` `CompareCaseBaseline`, `reimportCaseProfile` |
| R12-02 | P3 | `top` 보기가 포괄(inclusive) 순이라 hot self frame이 아래로 밀림 | `profilemodel/query.go` |
| R12-03 | P3 | 알려진 패턴 밖 비밀(`password=…`)이 파생 색인에 그대로 저장되고 반환됨(문서에 한계는 명시) | `cases/search.go:84-` `cleanSearch` |
| R12-04 | P3 | 저장 결과 64 MiB 상한 때문에 stack이 다양한 프로파일은 case에 저장·pin할 수 없고 오류가 원인을 말하지 않음 | `cases/model.go` `MaxObjectBytes`, `analysis` 결과 크기 |

## 4 Finding 상세

### R12-01 비교 쪽마다 전체 재분석 (P3)

**근거**: `CompareCaseBaseline`은 호출마다 기준 원본과 대상 원본을 `reimportCaseProfile`로 정규화 재실행하고 `CompareMapped`로 전체 차이를 계산한 뒤 `offset`으로 잘라 반환한다. 다음 쪽 cursor도 같은 경로를 처음부터 다시 거친다. 문서의 "재사용할 때마다 원본을 다시 분석"은 의도이지만 쪽마다 반복할 필요는 없다.

**재현**(합성 Speedscope 5,000표본 두 개, 결과 약 24 MiB, diff 행 52,933): `pin-baseline` 1.6 s, `compare-baseline` 1쪽 3.35 s, **2쪽 3.41 s**. 50행씩 넘기면 1,000쪽을 3,400초에 걸쳐 같은 계산을 반복한다. (T-631 R8-02와 같은 부류다.)

**수정 제안**: 검증 결과(재분석 모델, binding)를 `(case revision, result/source digest, 정책 해시)` 키로 수명이 짧은 캐시에 두고 같은 `query_sha256`의 다음 쪽은 재사용한다. 검증 의미는 첫 쪽 실행 시점에 한 번 확정하고 cursor가 그 revision에 묶여 있음을 유지한다.

### R12-02 `top` 정렬 기준 (P3)

**근거/재현**: 실제 Go heap의 `top` 첫 행들이 `runtime.main`(inclusive 2,263,560, self 0), `runtime/pprof.profileWriter`(1,209,080, self 0) 등 통과 frame이고, `go tool pprof -top`이 1위로 보이는 `runtime/pprof.StartCPUProfile`(flat 1,152.30 kB, 33.43%)은 아래에 있다. 보기가 포괄 가중치 내림차순이다. self 정렬을 선택할 수단이 질의에 없다(`Query`에 sort 필드 없음).

**수정 제안**: `sort: "self"|"inclusive"`를 질의 계약에 추가하고(cursor·query hash에 포함), UI에서 "Top self"를 기본으로 노출하거나 현재 보기를 `top_inclusive`로 이름을 바꾼다.

### R12-03 파생 색인의 비밀 저장 (P3)

**근거**: `cleanSearch`는 `common.RedactText`(text-mask 0.1.0)를 적용한 뒤 최대 2 KiB snippet을 저장한다. 이 마스킹은 Authorization/Cookie/URL/이메일/IP/긴 숫자 등 알려진 패턴만 다룬다(문서 한계와 작성 측 증거 `limitations`도 명시). **재현**: 텍스트 로그에 `password=hunter2 user=bob`, `Authorization: Bearer abcdef1234567890abcdef`, 이메일, 카드번호 형태 숫자를 넣고 외부 원본으로 case에 추가해 검색했다. 이메일·카드·Bearer 토큰은 검색되지 않고 마스킹됐지만(`Authorization: Bearer <TOKEN len=6> len=22>`로 표시되어 마스크 문구 자체가 어색하다) **`password=hunter2`는 snippet에 그대로 반환되고 색인 파일(`cases/indexes/search-case-*.json`, 권한 0600)에도 평문으로 남는다**.

**영향**: 색인은 폐기 가능한 파생물이지만 디스크에 지속된다. `password=`/`api_key=`/JWT/AWS 키 같은 가장 흔한 형태가 빠져 있다.

**수정 제안**: key=value 형태의 흔한 비밀 키 이름(`password|passwd|secret|token|api[_-]?key|access[_-]?key`)과 JWT·AWS 액세스 키 접두사를 text-mask에 추가해 새 마스킹 버전으로 올리고(기존 `0.1.0` 결과는 유지), Bearer 마스크 문구의 중복 접미를 고친다. 한계는 UI의 케이스 검색 안내에도 노출한다.

### R12-04 저장 결과 상한과 프로파일 규모 (P3)

**근거**: 정규화 결과는 모델 전체를 AnalysisResult에 담고 case 저장은 `MaxObjectBytes = 64 MiB`(`cases/model.go:20`)로 제한한다. baseline pin은 저장된 결과가 있어야 한다. **재현**(stack이 모두 다른 합성 Speedscope, 프레임 3,000개, 깊이 4~8): 2,000표본 → 10.8 MiB, 5,000 → 24.3 MiB, 10,000 → 46.5 MiB로 표본당 약 4.7 KiB이며 약 2만 표본(원본 1 MB 미만)은 94 MB 결과가 되어 `save-result`가 `CASE_BUDGET_EXCEEDED: result byte limit`으로 거부된다(원인 설명 없음). stack이 반복되는 일반 프로파일은 훨씬 작으므로 이는 최악에 가깝지만, 이 경우 baseline 기능을 쓸 수 없다.

**수정 제안**: 거부 메시지에 결과 크기·상한·표본 수를 담고, 정규화 단계에서 같은 stack·같은 weight 표본의 정확한 집계(가중치 합으로 의미 보존) 여부를 검토한다. 상한 안내를 `CASE_EVIDENCE_QUERIES.md`에 추가한다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `befd5f3`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 102개 패키지 모두 `ok`
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 80개 대조
- 실제 CLI 시나리오: case 생성·원본 추가(관리 복사/외부)·결과 저장, `profile-query` 6개 보기·cursor 4종·시간창 5종, `case-query search`(마스킹 7종 검색, 범위 공개, 인자 거부 3종, cursor 변조), `pin-baseline`·`compare-baseline`(정상, 잘못된 원본, 텍스트 원본, 오래된 revision, 위조 pin 3종, 원본 개체 변조·복구, `replace-source` stale, 매핑 5종), 5,000표본 시간 측정, 결과 크기 측정, 색인 파일 내용 확인
- throwaway 스크립트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 WebView UI(`ProfileEvidenceExplorer`, `CaseEvidencePanel`; 작성 측은 화면 잠금으로 새 네이티브 UI를 관찰하지 못했다고 기록), Windows 네이티브 실행
- 실제 py-spy 수집으로 만든 원본의 baseline 왕복(설치 안 됨)
- 오프라인 syscall 관측은 재실행하지 않았다(증거 정독). hosted CI.
- `OpenProfileEvidence`의 pyspy-raw 줄 좌표 열기는 실행하지 않고 코드만 정독했다.
- 휴대용 restore가 pin을 보존하되 검증 권한을 주지 않는 동작은 실행하지 않았다.

## 6 권고 처리 순서

1. R12-01 — 비교 쪽 사이 검증 결과 캐시. 규모 있는 비교 화면 전.
2. R12-03 — 마스킹 규칙 확장과 마스크 문구 정리(새 마스킹 버전).
3. R12-02 — `top` 정렬 옵션.
4. R12-04 — 저장 한도 오류 문구와 표본 집계 검토.
