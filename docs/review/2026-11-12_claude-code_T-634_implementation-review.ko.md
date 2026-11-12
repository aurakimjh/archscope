# T-634 py-spy·Speedscope·pprof 원본 측정 의미 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-634 구현 커밋 `eaa8c54`(Implement T-634 exact original profile source semantics). 직전 `e650b53`(T-633 후속 수정) 위에 놓인다.
- 계약 기준: `work_status.md`의 T-634 행과 수용 조건(R5-02 포함), `docs/ko/PROFILE_SOURCE_IMPORT.md`, `docs/ko/DATA_MODEL.md`·`IMPORTER_SUPPORT_MATRIX.md`의 T-634 절
- 증거: `docs/review/evidence/2026-11-12-t634/local.json`(`source_sha256` 42개와 `evidence_sha256` 25개가 `eaa8c54` 커밋 내용과 모두 일치), `cli-report.json`, `offline-summary.json`
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(`cases/source_pages.go`, HTTP 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`parsers/profile/source*.go`, `profilemodel/source_contract.go`·`comparison.go`·`operations.go`, `profilesourceservice.go`, `cmd_profile_source.go`, `ProfileSourceAssistance.tsx`), 저장소 밖 scratch worktree(`eaa8c54`)에서 빌드한 실제 CLI로 `profile inspect-source/import-source`, `profile-set diff/pprof` 실행(격리 `ARCHSCOPE_STATE_ROOT`). **독립 검증용 실제 Go 1.26 `runtime/pprof` 프로파일**(heap·allocs·mutex·block·goroutine·cpu)을 직접 수집해 `go tool pprof`의 합계와 대조했다. py-spy는 설치돼 있지 않아 저장소의 digest 고정 py-spy 0.4.1 fixture 구조와 합성 Speedscope로 대신했다.
- 범위: 원본 py-spy raw/Speedscope(sampled·evented)/pprof 정규화, 명시 선택, 수집 옵션 정책(R5-02), 비교·내보내기 보존. 일반 scalar/baseline 정책 채택은 T-635이므로 평가하지 않는다.

## 1 결론

**판정: 조건부.** 가중치·단위의 정확성은 매우 좋다. 실제 Go 프로파일 6종의 합계가 `go tool pprof`와 바이트·나노초 단위로 정확히 일치하고(`alloc_space` 3,529,624 B, `inuse_space` 1,090,120 B, mutex delay 896,615,059 ns 등), Speedscope의 소수·0 가중치와 evented 구간 합계, pprof 내보내기→재가져오기 왕복도 정확하다. 수집 옵션 정책도 `pid` 변경은 엔티티 매핑을 요구하고 위조 whitelist/ignore는 효과가 없다. 그러나 **pprof metric을 위치 번호(`pprof-0`…)로만 식별하고 원본 sample type(`alloc_space`/`inuse_space` 등)을 inventory·화면·결과에 드러내지 않으며, 컴파일된 정책이 없는 모든 producer/version 조합에서는 위치가 같은 서로 다른 type을 비교 가능으로 취급한다**(R11-01). 그 결과 `alloc_space`와 `inuse_space`를 대조하는 diff가 경고 없이 성공한다. 이것은 이 과업이 막으려던 종류의 의미 혼동이다. P1은 없다. P2 1건, P3 3건이다.

- **T-634 `DONE` 판정:** R11-01 처리 후 가능하다.
- **T-635 착수:** R11-01은 T-635 일반 scalar 비교의 metric 식별에 직접 영향하므로 T-635 전에 닫는 것을 권한다. R11-02~R11-04는 막지 않는다.
- 실제 WebView UI, Windows 네이티브 실행, 실제 py-spy 실행, hosted CI는 미검증이다(5절).

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| pprof 값·단위가 원본과 정확히 일치 | 충족 | 직접 수집한 Go 1.26 프로파일 대조(`go tool pprof -top`): heap `alloc_space` 3,529,624 B = 3.37 MB, `inuse_space` 1,090,120 B = 1064.57 kB, block 1,336,812,803 ns, mutex 896,615,059 ns, cpu 60,000,000 ns, goroutine 1. 모든 metric 종류·단위(count/bytes/duration/cpu_time)가 맞게 분류된다. |
| 모든 value column 보존 | 충족 | heap은 4개 column(`alloc_objects`, `alloc_space`, `inuse_objects`, `inuse_space`), mutex/block/cpu는 2개, goroutine은 1개 metric으로 모두 노출된다. |
| 0·소수 가중치 보존(Speedscope sampled) | 충족 | `0.010 + 0.0000001 + 0` = `0.0100001`(0 가중치 표본 `sample_records=3`으로 유지), 두 번째 profile `0.001`. 두 profile 합산 `0.0110001`. 모든 profile이 별도로 선택·합산된다. |
| evented: 정확한 exclusive 구간과 `[unattributed]` | 충족 | 중첩·같은 시각 닫기/열기·앞뒤 공백이 있는 이벤트에서 합계가 선언된 시간폭 100.5 ms와 같다(10+10.25+9.75+0+20+10+40.5). 기록은 7건. |
| py-spy raw: 정확한 소수·지수 가중치 | 충족 | `12`, `0.5`, `3e0`을 합쳐 `15.5` count, 3건. |
| pprof 내보내기는 정확한 projection, 왕복 | 충족 | `profile-set pprof --tick-scale 1`로 내보낸 파일이 `go tool pprof`에서 3.37 MB로 열리고 원본 reader로 다시 가져오면 `3529624 bytes`다. 틱 배율 없이는 `explicit pprof tick scale required`로 거부. |
| 수집 옵션 정책은 컴파일된 binding만 권한을 가짐 | 충족 | 실제 CLI로 py-spy 0.4.1 Speedscope 가져오기 후 diff: `pid` 변경은 `entity_mapping_required`, `rate` 변경은 `capture_options_mismatch`, 입력에 넣은 `{"whitelist":["rate"],"ignore":["rate"]}`는 효과 없음(`capture_options_mismatch`). `output_path`만 다르면 diff가 성공하고 `warnings:["incidental_capture_options_differ"]`, `incidental_differences:["output_path"]`로 공개된다. |
| 정책 binding이 type까지 구분(컴파일된 binding 안) | 충족 | go1.26.10을 선언한 `alloc_space` 결과와 `inuse_space` 결과의 diff는 `capture_policy_mismatch`로 거부된다. |
| 한도와 거부 | 충족(대부분) | 원본·압축 해제 각각 64 MiB, 잘린 성공 없음. 100,000 관측 초과는 전체 거부(R11-02에서 별도 평가). |
| 증거 무결성 | 충족 | `source_sha256` 42개, `evidence_sha256` 25개가 커밋 내용과 모두 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R11-01 | **P2** | pprof metric이 위치 번호로만 식별되고 원본 sample type이 보이지 않으며, 정책이 없는 조합에서는 다른 type끼리 경고 없이 비교됨 | `parsers/profile/source_pprof.go:50`, `parsers/profile/source.go:65`, `profilemodel/comparison.go:97` |
| R11-02 | P3 | 100,000 관측 상한이 실제 py-spy 수집 길이에서 걸리고 오류가 원인을 설명하지 않음 | `parsers/profile/source.go:258`, `profilemodel/model.go:23` |
| R11-03 | P3 | 선언만으로 정책이 적용됨: 사용자가 말한 producer/version이 곧 정책 권한 | `profilemodel/source_contract.go:88-` |
| R11-04 | P3 | 합성 Speedscope sampled 가중치 `none`을 sample_count로 단정 | `parsers/profile/source_speedscope.go` |

## 4 Finding 상세

### R11-01 pprof metric 식별과 비교 안전성 (P2)

**근거**: `source_pprof.go:50`은 value column의 metric ID를 `fmt.Sprintf("pprof-%d", i)`로 만든다. 원본의 type 이름(`alloc_space`, `inuse_objects`, `delay`, `contentions` …)은 확장 속성(`SourceMetric.Type`)에만 들어가고, 사용자가 metric을 고르는 `Inventory()`(`source.go:65`)와 UI(`ProfileSourceAssistance.tsx`의 `${m.id} · ${m.kind} · ${m.unit} × ${m.scale}`)와 결과의 `measurement`·`capture_policy.metric`에는 나타나지 않는다. 비교는 `Metric` 구조체(id, kind, unit, scale, signed)가 같으면 같은 측정으로 본다. 원본 type을 구분하는 것은 컴파일된 `CapturePolicy` 해시뿐인데(`comparison.go:97`), 정책은 py-spy 0.4.1과 go1.26.10의 지정 type에만 존재하고 그 밖에는 `nil`을 돌려준다. 양쪽이 모두 `nil`이면 `capture_policy_mismatch` 검사를 통과한다.

**재현**(직접 수집한 Go heap 프로파일, 이 커밋에서 빌드한 CLI):
1. `profile inspect-source --in heap.pb.gz --format pprof --producer go.runtime/pprof --producer-version go1.26.10`: metrics가 `pprof-0 sample_count count`, `pprof-1 bytes bytes`, `pprof-2 sample_count count`, `pprof-3 bytes bytes`. `alloc_objects`와 `inuse_objects`(`pprof-0`/`pprof-2`), `alloc_space`와 `inuse_space`(`pprof-1`/`pprof-3`)를 inventory만으로 구분할 수 없다. 사용자는 pprof의 column 순서를 외우고 있어야 한다.
2. 같은 프로파일에서 `inuse_*` 두 column만 남긴 변형 파일(`[inuse_objects, inuse_space]`)을 만들면 이 파일의 `pprof-1`은 `inuse_space`다. 원본 heap의 `pprof-1`(`alloc_space`)을 baseline, 변형 파일의 `pprof-1`을 target으로 `--producer`를 **선언하지 않고**(정책 없음) 각각 `import-source`한 뒤 `profile-set diff`를 실행하면 성공한다: `comparison_mode: baseline_target`, `warnings: []`, 첫 diff 행 `before 2263560 → after 1051376, delta -1212184`. 경고도 거부도 없다. 같은 입력에 `--producer go.runtime/pprof --producer-version go1.26.10`을 선언하면 `capture_policy_mismatch`로 거부된다. 즉 안전은 선언한 producer 문자열에 달려 있다.

**영향**: 이 과업의 목표는 "pprof의 값/단위 선택이 가져오기·비교·내보내기를 통해 보존"되는 것이다. 위치 번호는 한 프로파일 안에서만 의미가 있고, 서로 다른 프로파일·Go 버전·언어 런타임(예: Java async-profiler pprof)은 같은 번호가 다른 type을 가리킨다. 사용자가 "alloc을 inuse와 비교한" 거짓 회귀를 보게 된다. T-635가 일반 scalar 비교에서 이 metric 식별을 그대로 쓸 것이므로 영향이 커진다.

**수정 제안**:
1. metric ID를 type과 unit에서 만든 안정적 문자열(`alloc_space.bytes` 등, 중복 시 서수 접미)로 바꾸거나 최소한 `SourceMetric.Type`을 inventory·UI·결과 `measurement`에 노출한다. 새 의미 revision(normalizer/payload)으로 만들고 기존 결과를 재해석하지 않는다.
2. 비교는 정책 유무와 무관하게 원본 sample type·단위를 필수 일치 조건으로 삼는다(`SourceMetric.Type/Unit`을 `comparisonConditions`에 포함).
3. 회귀 테스트: `alloc_space` vs `inuse_space`(정책 없음)가 `semantic_mismatch`류로 거부되고, inventory가 type을 노출한다. 실제 Go heap의 4 column을 fixture로 쓴다.

### R11-02 관측 100,000 상한과 실제 py-spy (P3)

**근거**: `source.go:258`은 프로파일 전체의 관측 수가 `MaxSamples = 100,000`을 넘으면 전체 가져오기를 거부한다. 저장소의 실제 py-spy 0.4.1 fixture(`pyspy-speedscope.json`)는 표본 하나당 `samples` 항목 하나로 기록한다(가중치 `0.02`초, profile당 약 99표본/2초). 기본 100 Hz 수집은 단일 스레드에서 약 16.7분, 4개 스레드에서 약 4분이면 100,000에 도달한다. **재현**: 150,000표본(파일 2.1 MB)의 합성 Speedscope는 `IMPORT_BUDGET_EXCEEDED: exact profile observation/frame/depth budget exceeded`로 거부되고(9만 표본은 정상), 메시지는 관측 상한이 원인임을 알려주지 않는다.

**영향**: 파일 크기 한도(64 MiB) 안에 있는 일반적인 장시간 py-spy 수집이 거부된다. 문서에 상한이 적혀 있어(`관측 100,000개`) 정직한 거부이긴 하지만 실제 수집 길이 환산은 없다. **수정 제안**: 오류에 "관측 N개 > 100,000, 약 X초(rate) 분량"처럼 원인을 담고, 동일 stack·동일 weight 연속 표본을 정확한 가중치 합으로 접는 것이 허용되는지(관측별 시각을 만들지 않는 sampled이므로 의미 보존 가능)를 검토한다.

### R11-03 선언된 producer가 정책 권한을 부여 (P3)

**근거**: `CapturePolicy`는 `s.Producer`/`s.ProducerVersion`/format이 컴파일된 binding과 같을 때만 완화 정책을 준다(`source_contract.go:88-`). 그 값은 사용자가 `--producer`/`--producer-version`으로 선언하거나 Speedscope 파일의 `exporter` 문자열에서 온다. 문서는 `ProducerBasis`(declared/file_exporter)를 기록하고 "원본 인증이 아님"을 밝히지만, 이 선언이 곧 `output_path`/`pid`·(pprof의 경우) `duration`을 incidental로 바꾸는 권한이다. 다른 도구의 파일에 `exporter: "py-spy@0.4.1"`을 적으면 정책이 적용된다. 영향은 완화 폭이 `output_path` 한 키로 제한적이라 작다. **수정 제안**: 정책이 적용된 결과에 `capture_policy.basis`(declared/file_exporter)를 노출하고 비교 결과에서 선언 기반 정책임을 경고로 남긴다.

### R11-04 Speedscope `none` 단위 (P3)

**근거**: `source_speedscope.go`는 `unit: "none"`을 `count`/`sample_count`로 매핑한다. Speedscope 명세에서 `none`은 단위 없는 임의 가중치이며 표본 수라는 보장이 없다. 문서는 이 매핑을 의도로 밝히고 있다(`none`/bytes는 count/bytes). 가중치가 표본 수가 아닌 경우의 표시가 "sample count"로 단정되는 점만 지적한다. **수정 제안**: kind를 `weight`(일반 개수)로 두거나 provenance에 원래 unit 문자열 `none`을 노출한다(원본 값은 이미 보존됨).

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `eaa8c54`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 102개 패키지 모두 `ok`
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON의 소스·증거 해시 67개 대조
- 실제 CLI: 직접 수집한 Go 1.26 프로파일 6종 inspect/import 및 `go tool pprof` 합계 대조, heap 변형 파일 diff(정책 유·무), 합성 Speedscope(sampled 소수/0, 다중 profile, evented 중첩·공백), py-spy raw 합성, 9만/15만 표본 상한, 수집 옵션 정책 시나리오 6종(`pid`·`rate`·`output_path`·위조 whitelist·null/0), pprof 내보내기와 재가져오기
- throwaway 프로그램과 생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 WebView UI(작성 측은 화면 잠금으로 새 네이티브 UI를 관찰하지 못했다고 기록), Windows 네이티브 실행
- 실제 py-spy 실행(설치 안 됨). 저장소의 digest 고정 py-spy 0.4.1 fixture 구조만 확인했고 다른 py-spy 버전·옵션은 보지 않았다.
- 다른 producer/버전의 pprof(Java, Rust 등)와 label이 있는 goroutine profile 그룹핑은 직접 대조하지 않았다.
- 오프라인 syscall 관측은 재실행하지 않았다(증거 정독). hosted CI.

## 6 권고 처리 순서

1. R11-01 — metric ID·원본 type 노출, 비교 조건에 sample type 포함, 회귀 테스트. T-634 닫기 전, T-635 착수 전.
2. R11-02 — 상한 오류 문구와 장시간 수집 처리.
3. R11-03, R11-04 — 정책 basis 노출, `none` 단위 표기.
