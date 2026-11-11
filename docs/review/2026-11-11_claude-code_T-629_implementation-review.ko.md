# T-629 ProfileSet 구현 독립 코드 리뷰 — 2026-11-11

- 대상: T-629 구현 커밋 `f06673a`(feat(profile): implement T-629 exact ProfileSet core and legacy facades)와 이후 반영분. 확인 기준 HEAD는 `a320881`이다.
- 계약 기준: `docs/ko/DATA_MODEL.md` "T-629 ProfileSet 공통 코어 계약", `docs/ko/PARSER_DESIGN.md` "T-629 구현과 이행 경계", `docs/ko/LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md` §16.7, `docs/ko/IMPORTER_SUPPORT_MATRIX.md`
- 증거: `docs/review/evidence/2026-11-11-t629/local.json` (`source_sha256` 36개 항목이 `f06673a`의 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 HTTP 변경은 수정하지 않았다.
- 방법: 소스 정독, 저장소 밖 scratch worktree에서 재실행·재현(미커밋), 기준 커밋 `63d3b6a`를 별도로 빌드한 CLI와의 출력 비교.
- 범위: T-629(ProfileSet 코어, legacy facade, catalog·CLI·Wails 저장 결과 경로). py-spy/Speedscope/pprof 수치 정정은 T-634, generic 화면은 T-631이므로 평가하지 않는다.

## 1 결론

**판정: 조건부.** 정밀 수치 모델, 값 보존, 재계산 기반 결과 검증, legacy 호환은 계약대로 구현되어 있다. 다만 **새 정규 경로가 실제 규모의 프로파일을 받지 못하고**(R4-01), **서로 다른 수집끼리의 비교 조건이 실제 자료에서는 거의 충족되지 않는**(R4-02) 구조라, T-634 이후의 producer 정정이 이 경로에 올라가기 전에 고쳐야 한다. P1은 없다. P2 2건, P3 3건이다.

- **T-629 `DONE` 판정:** R4-01·R4-02 처리 또는 "합성·소규모 전용 경로"로 계약과 지원표를 정정한 뒤 가능하다. 현재 문서가 일부 한계(record 바이트 상한)를 밝혔지만 문서의 명목 상한과 실제 상한이 크게 다르다.
- **T-634·T-635 착수:** R4-01은 T-634 전에, R4-02는 T-635 전에 닫기를 권한다. T-630·T-631은 이 리뷰로 막히지 않는다.
- 실제 WebView IPC/UI, Windows 네이티브 실행, hosted CI는 여전히 미검증이다(5절).

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| legacy 호환: 기존 출력 유지 | 충족 | `63d3b6a`로 따로 빌드한 CLI와 `profiler analyze-collapsed`(wall·cpu 각 4개 입력)·`analyze-jennifer-csv` 출력을 생성 시각·progress log 경로만 제외해 비교했고 전부 동일. 종료 코드도 동일. |
| 정밀 수치(소수 문자열, scale, 0, 부호) | 충족 | `profilemodel.Number`, `Text`(유한 소수만 렌더), 트리·diff는 유리수 연산, float·최소 1 보정 없음. |
| 0·null·부재 구분 | 충족 | `sample_json.go`가 명시 null과 부재를 구분해 왕복한다. |
| 정의되지 않은 필드 거절, 미래 정보는 `extensions` | 충족 | `DisallowUnknownFields`, `Extensions` map. |
| 버전 번호만 바꿔 새 계산을 활성화하지 못함 | 충족 | `ValidateModelResult`가 보존된 모델에서 summary·series·tables·charts·measurement·population을 재계산해 대조한다. 한 type에 겹치지 않는 payload 범위별 reader를 등록하고 겹치면 거절한다. |
| 재귀·동명 frame 구분 | 충족 | frame은 symbol/module/build/file/line 해시 ID. 표시 이름은 identity에서 제외. 반복 출현은 별도 경로 위치로 유지. |
| legacy 정수 projection 거절 조건 | 충족 | 비정수 tick, 부호, int 범위 초과를 거절하고 이름 충돌 시 stable key를 요구한다. 386 빌드를 고려해 플랫폼 int 한계를 사용한다. |
| 비교 증거에 출처 보존 | 충족 | 응답에 baseline/target 모델 digest·source hash·handle·저장 결과 digest, `identity_basis:"declared"`를 담는다(R3-02 반영). |
| 취소·수명 공유 | 충족(코드 확인) | 질의가 `beginLoad`의 slot·shutdown·deadline을 공유하고 `each`가 샘플마다 ctx를 확인한다. |
| 자원 초과는 거절, 절단·반올림 없음 | 충족 | 문자열·프레임 참조·트리·직렬화 예산 초과 시 오류. 단 실효 상한은 R4-01. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R4-01 | **P2** | 문서의 명목 상한과 달리 실효 처리 규모가 매우 작아 실제 프로파일을 정규 경로로 가져올 수 없음 | `catalog/profile_set.go`(단일 record), `ingestion/lease.go` `Within`, `profilemodel/operations.go` BuildTree |
| R4-02 | **P2** | 비교 identity에 품질 수치와 process/thread 식별자가 들어 있어 서로 다른 수집은 거의 비교되지 않음 | `profilemodel/model.go` `selectionIdentity`, `operations.go` `Compare` |
| R4-03 | P3 | 모든 `Check`가 전체 모델에서 결과를 재계산하고 결과가 모델·트리·frame 표를 중복 저장함 | `analyzers/profile/model.go` `ValidateModelResult`, `BuildSet` |
| R4-04 | P3 | `metadata.quality` 사본은 재계산 대조 대상이 아니라 변조되어도 통과함 | `analyzers/profile/model.go` |
| R4-05 | P3 | 64 KiB를 넘는 ProfileSet은 자동 감지되지 않음 | `catalog/profile_set.go` Detector |

## 4 Finding 상세

### R4-01 실효 처리 규모가 명목 상한보다 훨씬 작음 (P2)

**근거**: 문서(DATA_MODEL "운영 상한")는 입력 64 MiB, frame/stack/sample 각 100,000개, tree node 200,000개, 트리 직렬화 8 MiB를 적는다. 그러나 새 결과(`profile_evidence` 0.2.0)를 만드는 경로는 `profile.set-json` importer 하나이고, 이 importer는 모델 전체를 **하나의 record**로 sink에 쓴다(`profile_set.go`의 `ParserFactory`). `BoundedSink`의 record 상한은 서비스 정책 1 MiB이며 `Budgets.Within`은 요청이 정책을 올리지 못하게 한다. 그래서 어떤 옵션으로도 1 MiB를 넘는 모델은 가져올 수 없다. 문서는 이 record 상한을 한 문장으로 언급하지만 64 MiB/100,000/200,000 수치가 같은 문단에 명목 값으로 남아 있다. 또 트리 직렬화 8 MiB 계산이 노드당 256바이트 이상을 가정해 실효 노드 수를 약 2만~3만으로 낮춘다.

**재현**(Mac arm64):
- 3,000개 stack × 깊이 14의 collapsed 입력을 `profile-set normalize-legacy`로 변환하면 모델이 4.9 MB다. 이를 `evidence import --importer profile-set`으로 가져오면 `IMPORT_BUDGET_EXCEEDED: Import stopped; no result was published.`로 실패한다. 어느 상한인지, 값이 얼마인지 알 수 없다. 같은 방식으로 50개 stack × 깊이 6 입력(91 KB)은 성공한다. 대략 600개 stack 규모가 한계다.
- Go 단위 probe: 30,000 샘플 × 깊이 8 모델은 `BuildSet`이 `tree serialization budget exceeded`로 실패한다(명목 노드 상한 200,000 이전). 50,000 샘플 × 깊이 10은 문자열 예산에서 실패한다. 8,000 샘플 × 깊이 6은 성공하지만 결과 JSON이 11.5 MB다(R4-03).

**영향**: 일반 Java·Python 서비스 프로파일은 수만 노드를 넘는다. T-634가 py-spy/Speedscope/pprof 원본 decoder를 이 모델 위에 올리려면 이 경로가 실제 규모를 받아야 한다. 현재는 정규 경로가 합성·소규모 입력 전용이다. 거절 방식(절단 없이 거부)은 설계와 일치하지만 사용자가 원인과 한계를 알 수 없다.

**수정 제안**:
1. importer가 선언하는 record 상한(정책의 절대 상한 이내)을 도입하거나, 모델을 frames/stacks/profile 단위 여러 record로 나눠 streaming으로 쓴다. 정책 상한을 올리지 않고도 큰 모델이 들어갈 수 있다.
2. 트리 직렬화 8 MiB 규칙과 `MaxNodes`를 일치시키거나, 문서에 **실효** 상한(예: 노드 약 2만)을 적고 명목 값을 제거한다.
3. 예산 거절 오류에 초과한 항목과 한계 값을 담는다(`IMPORT_BUDGET_EXCEEDED` 아래의 세부 사유 코드).
4. 큰 모델 import 회귀 테스트(≥1 MiB, 노드 ≥50,000)를 추가한다.

**완료 기준**: 3,000 stack 규모 입력이 가져와지거나, 지원표가 소규모 전용임과 실효 상한을 명시한다. 한계 초과 시 사유가 구분되어 반환된다.

### R4-02 비교 identity가 수집마다 달라지는 값을 포함함 (P2)

**근거**: `Compare`는 두 모델의 `ComparisonIdentity`가 같아야 한다. 이 identity(`selectionIdentity`)는 `s.Quality` **전체**(lost·unknown·truncated 수치, symbol_resolution, clock_confidence, conversion_loss), 선택된 profile의 process/thread/worker/clock/ID, capture options, producer 버전을 해시한다. 설계는 "품질·entity 차원 일치"를 요구하지만, 측정된 수치(손실 샘플 수 등)나 프로세스 ID는 수집마다 다르다. 구현은 구분 없이 모두 일치를 요구한다.

**재현**(실제 CLI): 같은 구조의 두 legacy-normalized 모델은 비교가 성공한다(`comparison_mode:"baseline_target"`, 291행, `identity_basis:"declared"`). 이는 legacy 모델의 quality가 모두 null이고 process가 비어 있기 때문이다. 두 번째 모델의 `quality.lost`를 `"3"`으로 바꾸면 `population/source/selection/capture conditions mismatch`로 거부된다. 프로필의 `process`만 `pid-200`으로 바꿔도 동일하게 거부된다. 기존 테스트는 품질·entity가 같은 입력만 비교한다.

**영향**: R2의 N-01(원본 hash 때문에 비교 불가)과 같은 종류의 문제가 품질·entity 차원에서 반복된다. T-634 이후 실제 producer가 lost 수와 PID를 채우면 서로 다른 배포 시점의 프로파일은 비교되지 않는다. 사용자는 "왜 거부됐는지"도 알 수 없다(오류가 하나로 뭉뚱그려짐, R3-01과 같은 현상).

**수정 제안**:
1. 비교 가능성 조건을 "범주"로 한정한다: producer·format·adapter 버전, metric(kind/unit/scale/sign), 선택 규칙, clock 신뢰도 범주, symbol 해석 가능 여부.
2. 측정된 품질 수치(lost/unknown/truncated/resolution 값)는 조건이 아니라 양쪽 참조에 기록해 보여 준다. 임계 차이는 경고로 낸다.
3. entity는 정확 일치 대신 역할 키(예: 서비스·worker 역할)나 명시적 매핑을 받는다. 영속 매핑은 T-635 범위이므로 그때까지 "process/thread 불일치 → entity 매핑 필요" 사유 코드로 거절한다.
4. 거절 사유 코드(`quality_class_mismatch`, `entity_mapping_required`, `selection_mismatch` 등)를 추가하고 각 경로에 테스트를 둔다.

**완료 기준**: lost 수치만 다른 두 모델이 비교되고 차이가 응답에 보인다. process가 다른 모델은 명확한 사유 코드로 거절된다.

### R4-03 결과 크기와 검증 비용 (P3)

**근거**: `BuildSet`은 결과에 보존 모델(`metadata.profile_core`), 전체 트리(`charts.flamegraph`), frame 표(`tables.frames`)를 모두 담는다. 서비스는 `Check`마다 `ValidateModelResult`를 실행하며, 이 함수가 모델 검증과 `BuildSet` 전체 재계산·비교를 수행한다. `View`·`Scalar`·`Export`·`Compare`가 모두 `Check`를 부른다.

**재현**(`TestR4Perf`, 8,000 샘플×깊이 6): 결과 JSON 11.5 MB, `ValidateModelResult` 한 번에 약 0.53초, 64개 필터 drilldown 약 0.83초. 결과 크기에 거의 선형이다.

**영향**: 현재 가능한 규모에서는 허용 범위지만 R4-01을 풀어 규모를 키우면 handle 읽기마다 초 단위 지연이 생긴다.

**수정 제안**: 저장 결과 해시를 키로 검증 결과를 캐시한다. 저장 시점에 한 번 검증하고 읽을 때는 해시 일치만 확인한다. 결과의 중복 저장(모델·트리·frame 표)은 문서에 크기 증폭 계수를 적는다.

### R4-04 `metadata.quality` 사본 미검증 (P3)

**근거**: `BuildSet`은 `Metadata.Extra["quality"] = set.Quality`로 사본을 넣지만 `ValidateModelResult`의 재계산 대조 목록(summary·series·tables·charts·measurement·populations)에 포함되지 않는다.

**재현**: 저장 결과의 `metadata.quality.lost`를 `"999"`, `symbol_resolution`을 `"1"`로 편집한 파일을 `evidence export`(json·html)에 넣으면 거부되지 않고 그대로 통과한다. 권위 있는 모델의 quality 값과 달라져도 감지되지 않는다.

**영향**: quality를 `metadata.quality`에서 읽는 소비자가 편집된 값을 신뢰할 수 있다. 위협은 낮지만 "재계산으로 변조를 막는다"는 계약의 구멍이다.

**수정 제안**: 대조 목록에 `metadata.quality`를 포함하거나 사본을 제거하고 `profile_core.model.quality`만 사용하도록 문서화한다.

### R4-05 큰 ProfileSet의 자동 감지 (P3)

**근거**: Detector는 `probe.Head`(최대 64 KiB)를 모델로 디코드한다. 그보다 큰 문서는 잘려 디코드에 실패하고 `.json` 확장자 hint(0.1)만 반환된다.

**재현**: 91 KB 모델을 `--importer` 없이 가져오면 `needs_selection`(hint만 있음)과 `IMPORT_SELECTION_REQUIRED`가 반환된다. 명시 선택하면 성공한다.

**영향**: `profile_ids`·`metric_id` 옵션이 어차피 필수라 자동 선택의 실익이 작아 심각하지 않다. 다만 "검증된 감지"가 64 KiB 이하에서만 동작함을 지원표에 적을 필요가 있다.

## 5 검증 범위

### 실행한 것 (Mac arm64)

| 항목 | 결과 |
|---|---|
| `go vet ./...` | 통과 |
| `go test -race` (profilemodel·analysis·analyzers/profile·catalog·profiler·parsers/profile·cmd) | 통과 |
| legacy CLI 비교(`63d3b6a` 대비, 8개 collapsed 조합 + Jennifer CSV) | 동일 |
| 실제 CLI: normalize-legacy → evidence import → profile-set diff | 소규모 성공, 큰 입력 R4-01 재현, 품질·process 변형 R4-02 재현 |
| 재현 probe(scratch worktree, 미커밋) | R4-01·R4-03 규모/시간 측정 |
| 증거 `source_sha256` 36개 | 일치 |

### 코드·테스트 정독으로만 확인한 것

pprof export의 tick scale·정수 범위·레이블 충돌 처리, drilldown 응답 예산, 질의의 취소 전파, Wails 메서드 시그니처는 정독과 작성 측 회귀 테스트(전체 통과)로 확인했고 별도 재현은 하지 않았다. 필터 64개·깊이 1,024·frame 참조 1,000,000 같은 상한의 정확한 경계도 직접 재현하지 않았다.

### 실행하지 않은 것 (기존 미검증 항목과 구분)

- 실제 WebView IPC/UI: 새 ProfileSet 메서드를 호출하는 화면이 없다(T-631). Go 메서드 직접 호출까지만.
- Windows 네이티브 실행: 교차 빌드·vet만 있다.
- hosted CI, 실제 producer 입력·버전 적합성(Q01~Q04 미결정, T-634 범위).
- 큰 모델에서의 메모리 사용량과 Wails 직렬화 지연.

## 6 처리 권장 순서

1. **R4-01**: 큰 모델 import 경로와 실효 상한 문서화. T-634 전에 닫는다.
2. **R4-02**: 비교 identity를 범주와 수치로 분리하고 사유 코드를 추가한다. T-635 전에 닫는다.
3. R4-03: R4-01 해결과 함께 검증 캐시를 설계한다.
4. R4-04·R4-05: 해당 영역을 손댈 때 처리한다.
