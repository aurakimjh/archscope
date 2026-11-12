# T-643 Apple Silicon 증거 어댑터 구현 독립 리뷰

- 작성: claude-code (독립 리뷰어; 구현·처리와 다른 AI)
- 대상: `d9e62b1` (T-643 구현 + T-642 R19-01 처리). 이 리뷰는 구현 코드를 수정하지 않았다.
- 방법: 대상 커밋의 scratch worktree에서 CLI를 새로 빌드하고 격리 `ARCHSCOPE_STATE_ROOT`로 실행했다. 이 Mac은 Apple Silicon(arm64, macOS 27.2.1)이며 Ollama 0.40.16이 실제로 동작 중이라 번들 수집기를 직접 실행해 독립 캡처를 만들었다. JTL은 Python으로 별도 생성·별도 계산했다.

## 1. 결론

**CONDITIONAL — P2 1건, P3 3건, P1 없음.**

- T-643의 계약(공유 메모리 의미 보존, GPU 활용률·전력·온도 추정 거부, 정확한 uint64, 소스 고정)은 독립 캡처에서 모두 지켜졌다.
- T-642 R19-01 수정(JTL 정확 분포)은 독립 입력으로 완전히 확인되었다.
- 다만 **수집기가 `--model`로 고르지 않은 다른 로드 모델을 표시 없이 버려서**, 결과가 "이 모델만 상주"로 읽힌다(R20-01, P2). 이를 처리한 뒤 T-643 DONE 복귀를 허용한다. R20-02~R20-04는 후속 큐로 충분하다.

## 2. 확인한 계약 충족 항목

| 항목 | 확인 내용 |
|---|---|
| 실제 수집기 | 이 Mac에서 `capture-apple-ollama.py`를 직접 실행(빈 상태 2샘플, gemma:2b 로드 후 2샘플). 둘 다 `history-import` 성공, `metrics_timeseries 0.2.0`, 시리즈 12개/포인트 24개(빈 상태). 로드 캡처의 모델 값은 `size = size_vram = 1799817788`, context 512, Q4_0으로 `ollama ps`와 같다. |
| 상태 복원 | 로드한 모델은 `keep_alive: 0`으로 내렸고 `/api/ps`가 `{"models":[]}`로 돌아왔다. 수집기는 모델을 로드·생성·다운로드하지 않는다. |
| 번들 fixture | `capture.json` SHA-256 `a7f438c5…9487`이 manifest와 `history-inspect`의 `source_sha256`에서 일치한다. |
| 의미 보존 | GPU 활용률·전력은 `no_recorded_value`+사유, 숫자를 넣으면 `APPLE_CAPTURE_UNSUPPORTED_MEASUREMENT`로 거부. pressure는 정수 코드 그대로(`critical` 같은 문자열 거부), thermal 미기록은 0이 아니라 사유 있는 null. |
| 엄격 입력 | 알 수 없는 필드(`prompt`, 모델 `details`), 중복 JSON 키, 비정규 정수(`1799817788.0`, `1.79e9`), 2^64 오버플로, 겹치는 스냅샷, 중복 모델 digest, `memory_capacity=0`, `swap_used>total`, `context_length=0`, 비-Apple chip, 다른 Ollama 버전, `models:null`이 모두 코드와 함께 거부된다. 2^53+1 값은 정확히 보존된다. |
| 식별자 | host/boot/device/hardware/OS/build/engine/collector가 모든 시리즈 resource에 들어가고 호스트 경로·사용자명은 결과에 나타나지 않는다(수집기가 읽는 필드는 고정 허용목록). |
| R19-01 정확도 | 6만 행·3 label JTL의 27개 통계(count/errors/min/max/mean/p50/p90/p95/p99)가 Python nearest-rank 독립 계산과 전부 일치(`diff 0`). 결과 크기 602,558바이트(이전 설계는 6만 행에서 거부). |
| R19-01 규모 | 20만 행·30 label 0.4초/64 MB RSS, 20만 행·128 label 0.7초/117 MB, 100만 행·5 label 1.2초/86 MB, 30만 행·30 label 균일분포 13.7 MB 모델까지 통과. |
| R19-01 경계 | 1,000행은 payload 0.1.0/core 1.0.0 개별 표본 1,000개, 1,001행은 payload 0.2.0/core 1.1.0 분포 2개·표본 0개로 전환. 1.0.0과 1.1.0 결과 사이 `execution compare`가 정상 동작(양쪽 모두 `compatible_declarations`, 한쪽에만 있는 지표는 `missing_or_unavailable`). |
| R19-01 변조 | 분포 count·errors·bucket 순서·bucket count·지표값·payload 버전·core 버전·표본 주입·비정규 seconds·분포 표·summary 변조 11종이 모두 `EXECUTION_DISTRIBUTION_*`/`EXECUTION_RESULT_CORE_*`/`EXECUTION_SAMPLE_INVALID`로 거부된다. `execution query`의 `distributions` 종류도 페이지 단위로 동작한다. |

## 3. Finding 요약

| ID | 심각도 | 요약 |
|---|---|---|
| R20-01 | P2 | 수집기가 선택하지 않은 로드 모델을 표시 없이 제외; 오타·미로드와 "모델 없음"을 구분할 수 없음 |
| R20-02 | P3 | 가져오기가 형식만 검증하고 불변식·식별자 위생은 검증하지 않음 |
| R20-03 | P3 | 문서의 "100만 행"보다 64 MiB 입력 한도가 먼저 걸림(기본 열 약 44만 행); 오류에 추정치·조치 없음 |
| R20-04 | P3 | 수집기·계약이 단일 버전·단일 장비·합성 8토큰 한 건에만 검증됨; 다중 로드와 실제 부하 형태는 미검증 |

## 4. Finding 상세

### R20-01 (P2) 선택하지 않은 로드 모델이 흔적 없이 사라진다

- 근거: `scripts/capture-apple-ollama.py:88` `for model in models if model['name'] == model_name`; 스키마·`apple.go`에는 제외 사실을 기록할 필드가 없다.
- 재현: 이 Mac에서 `gemma:2b`와 `nomic-embed-text`를 둘 다 로드(`/api/ps`가 `335733063`·`1799817788` 두 모델을 반환)하고 `--model gemma:2b`로 캡처하면 `models`에 gemma만 남는다. 임베딩 모델 335 MB는 어디에도 없다. `--model gemma2b`(오타)로 캡처하면 `models: []`가 되며, 모델이 정말 로드되지 않은 빈 캡처(`empty.json`)와 구분되지 않는다. 가져온 결과에도 경고나 개수 필드가 없다.
- 영향: Apple 통합 메모리 시나리오의 핵심은 "시스템 메모리가 이렇게 쓰였고 모델 추정치는 이렇다"를 함께 읽는 것이다. 채팅+임베딩 동시 로드는 흔한 구성인데, 결과가 단일 모델만 상주한 것처럼 보여 메모리 압박의 원인 해석이 한쪽으로 기운다. 문서는 "정확히 선택한 모델 메타데이터만 남는다"고 적지만, 결과 자체가 생략 사실을 말하지 않아 사후 복원도 불가능하다.
- 권고: 수집기가 이름 없이 **제외된 모델 수**(`excluded_model_count`)와 선택 모델의 로드 여부를 스냅샷에 기록하고, 가져오기가 그 값을 시리즈·경고로 노출한다(모델 이름·digest는 계속 저장하지 않는다). 선택 모델이 0개 일치하면 수집 시점에 경고하거나 `--model`이 `/api/ps`에 없을 때 명시적으로 알린다.

### R20-02 (P3) 형식은 검증하지만 불변식과 식별자 위생은 검증하지 않는다

- 근거: `apple.go:57` `appleText`는 길이와 `\x00\r\n`만 거른다. 스키마에도 의미 제약이 없다.
- 재현: 다음이 모두 `rc 0`으로 수용된다 — `size_vram > size`(99999999999), `vm_free_bytes`가 `memory_capacity`보다 큼, `host_id = /Users/aura/secret`(경로가 모든 시리즈 resource·내보내기에 들어감), 모델 이름의 탭 문자, 대문자 digest(소문자와 다른 시리즈 identity가 됨).
- 영향: 문서가 "임의 텍스트 정제는 약속하지 않는다"고 밝혔으므로 사양 위반은 아니나, 수집기를 거치지 않은 수동 편집 입력이 물리적으로 불가능한 값을 정상 증거로 남긴다. 합계·잔여 계산을 하지 않는 설계라 직접 오판 경로는 없다.
- 권고: 단일 항목 불변식(`size_vram ≤ size`, 각 VM 범주 ≤ capacity), digest 소문자 정규화 또는 거부, 식별자 태그 문자 집합 제한(예: `[A-Za-z0-9._-]`)을 추가한다.

### R20-03 (P3) 문서의 행 한도보다 입력 바이트 한도가 먼저 걸린다

- 근거: `parsers/execution/source.go:59,71` 64 MiB 입력 한도, `executionmodel/distribution.go` `MaxJTLRows = 1_000_000`.
- 재현: JMeter 기본 열(responseMessage·threadName·URL 포함, 행당 약 150바이트) 합성 JTL은 35만 행(52.8 MiB)까지 통과, 45만 행(67.8 MiB)은 `IMPORT_BUDGET_EXCEEDED: input_bytes 71126023 exceeds 67108864`로 거부된다. 즉 이 열 구성에서 실제 상한은 약 44만 행이며 문서의 100만 행에 닿지 않는다. 오류에는 행 수·예상 한도·조치가 없다. 고유 latency bin 한도(10만)도 128 label×균일 2000ms 분포에서 12.7만 행에 `EXECUTION_DISTRIBUTION_BUDGET`(rows/distinct/limit 포함)로 걸린다. 로그노멀 분포로는 128 label·20만 행도 통과해 현실 입력에서는 드물다.
- 권고: 문서에 "열 구성에 따라 64 MiB가 먼저 적용되며 기본 열에서 약 44만 행"을 적고, 바이트 한도 오류에 한도와 분할 불가(백분위 합성 불가) 안내를 붙인다.

### R20-04 (P3) 검증 범위가 한 장비·한 버전·한 합성 요청이다

- 근거: `apple.go`의 `c.Collector != "1.0.0" || c.Engine != "0.40.16"` 고정, 수집기 `version != '0.40.16'` 거부, manifest `qualification: "…단일 환경 검증"`.
- 설명: 의도된 보수적 범위이고 문서도 같은 말을 한다. 다만 이 리뷰의 독립 캡처도 같은 장비·같은 버전이라 **환경 독립성은 없다**. 다중 로드·장시간·swap 사용·pressure 상승 상태의 실제 캡처, 다른 Ollama 버전·macOS 버전·칩은 미검증이다. 수집기가 지원 범위 밖 버전을 거부하므로 위험은 낮다.
- 권고: 후속으로 swap/pressure가 0이 아닌 상태와 다중 모델 상태의 실제 캡처를 fixture로 추가한다(R20-01 수정 후).

## 5. 실행한 검증과 미검증 항목

**실행함**
- 대상 커밋 scratch worktree 빌드, `go vet ./...`(host), `GOOS=windows GOARCH=386 go vet ./...` 통과.
- 관련 패키지 `go test -race -count=1`: `parsers/metrichistory`, `parsers/execution`, `analyzers/execution`, `catalog`, `cmd/archscope-engine` 모두 `ok`.
- 실제 수집기·실제 Ollama 0.40.16으로 빈 상태·단일 로드·이중 로드·오타 4종 캡처, 변조 입력 22종, JTL 12종 입력 크기/경계/비교/변조 시나리오, JTL 27개 통계 Python 대조.

**실행하지 않음**
- 전체 116개 패키지 race 스위트(관련 패키지만 실행), Wails WebView·Playwright 화면, 네이티브 Windows, hosted CI.
- Linux network-none 오프라인 syscall 검증 49건(작성자 증거는 열람만 했고 재실행하지 않음).
- Mac 패키징·서명, 다른 macOS/Ollama/칩 조합, 장시간·swap·pressure 상승·다중 모델 상태의 실제 부하.
- T-642 R19-02~R19-04(실제 hs_err 파서, JTL 타당성, JMeter 버전 정책)는 작업표에 후속 큐로 남아 있으며 이번에 다시 판정하지 않았다.

## 6. 권고 처리 순서

1. R20-01 — 제외 모델 수·선택 모델 로드 여부 기록과 가져오기 노출(이후 T-643 DONE 복귀 허용).
2. R20-02 — 불변식·식별자·digest 정규화.
3. R20-03 — 한도 문서·오류 안내.
4. R20-04 — 다중 모델·swap/pressure 실제 fixture(R20-01 이후).

참고: 대상 커밋이 `done/`으로 옮긴 T-642 리뷰의 `docs/review/README.md` 미처리 행은 아직 남아 있다(처리자 소관).
