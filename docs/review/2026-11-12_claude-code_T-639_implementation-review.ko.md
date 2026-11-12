# T-639 GPU·추론 서빙 근거(InferenceEvidence) 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-639 구현 커밋 `4cd6003`(feat(diagnostics): implement T-639 device and serving evidence). 직전 `1a97ff8`(T-637 리뷰) 위에 놓이며, 이 커밋은 T-637 리뷰 R15-01·R15-02·R15-06의 로컬 수정(SystemEvidence 1.1.0 / `system_evidence` 0.2.0)도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-639 행과 완료 기록, `docs/ko/INFERENCE_EVIDENCE.md`, `docs/ko/SYSTEM_EVIDENCE.md`의 R15 수정 절, `docs/ko/DATA_MODEL.md`·`IMPORTER_SUPPORT_MATRIX.md`의 T-639 절
- 증거: `docs/review/evidence/2026-11-12-t639/local.json`(`source_index_files` 98개와 `evidence_files` 16개의 SHA-256이 `4cd6003` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`inferencemodel/model.go`, `parsers/inference/source.go`, `analyzers/inference/analyzer.go`, `analysis/inference_evidence.go`, `cmd_inference.go`, `catalog/inference_evidence.go` 개요, `parsers/systemtrace/source.go`의 keyed hist 수정), 저장소 밖 scratch worktree(`4cd6003`)에서 빌드한 실제 CLI로 격리 상태 루트에서 번들 합성 fixture(NVIDIA CSV, 바인딩, 정규 set)와 직접 만든 CSV 변형 9종, Prometheus range·OTLP 입력 + 직접 만든 바인딩, 규모 입력, perf script 규모 입력을 `inference inspect/normalize/import-source`, `metrics history-normalize`, `system import-source/project-profile`로 끝까지 실행했다. throwaway Go 테스트 1개를 쓰고 삭제했다.
- 범위: InferenceEvidence 1.0.0 / `inference_serving` 0.1.0(NVIDIA query CSV, MetricHistory + bindings, 정규 set), 의미 정의·단위 변환·scope 검증, 관측 행, 구조화 event와 요청 TTFT/TPOT, CLI·Wails 브리지, 그리고 T-637 R15 수정 확인. 실제 NVIDIA/vLLM 수집, 네이티브 WebView·Windows, hosted CI는 범위 밖(작성 측도 미관찰)이다.

## 1 결론

**판정: 조건부(P2 2건, P3 4건).** 계산·의미 계약은 실제 실행에서 정확했다. NVIDIA CSV의 단위 변환(75 %→`0.75`, 8192 MiB→`8589934592` bytes)이 정확하고, `N/A`·`[Not Supported]`·`[Insufficient Permissions]`가 서로 다른 상태로 남으며, 물리 범위 밖 값(120 %, 음수 전력)은 원본을 유지한 채 정규화 값만 null이 되고, 요청 TPOT가 `(last−first)/(tokens−1)`로 정확히 계산되고 token 1개 요청은 null이다. 모델은 장치/worker 값을 복제하거나 합산하지 않고 scope 위반을 명시적으로 거부한다. T-637 리뷰 R15-01(keyed-map histogram)·R15-02(stack 깊이)·R15-06(소수 표기) 수정도 실제로 동작한다. 다만 **실사용 경로 두 곳이 막혀 있다.** Prometheus range로 받은 vLLM/DCGM 메트릭은 어떤 의미로도 연결할 수 없고(R16-01), 요청 TTFT/TPOT와 runtime(OOM·Xid) 사건을 담는 구조화 event는 사용자가 만들 수 없는 자기 해시 ID가 필요해 fixture 밖에서 쓸 수 없다(R16-02).

- **T-639 `DONE` 판정:** R16-01(P2)과 R16-02(P2)를 처리한 뒤 `DONE` 복귀를 허용한다. R16-03~R16-06은 후속 큐에 넣으면 된다. 실제 NVIDIA/vLLM 수집, 네이티브 Wails/Windows/Office 수용은 작성 측이 밝힌 대로 별도 항목이다.
- **후속 작업과의 관계:** T-640(LLM trace)·T-644(분산 추론)가 이 모델의 event/context를 재사용한다. R16-02에서 event 입력 경로를 정하는 방식이 이들의 입력 계약이 된다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| NVIDIA query CSV 정규화 | 충족 | 번들 3행 CSV: entity 2개(UUID/PCI), signal 14개, 관측 21개. `75`→`0.75 ratio`, `8192` MiB→`8589934592 bytes`, `123.50`→`123.5 watts`, 시각은 declared ns. 장치 b의 `N/A`→`not_available: N/A`, `[Not Supported]`→`not_supported`, `[Insufficient Permissions]`→`permission_denied`(값은 null). 같은 UUID의 PCI 충돌은 `INFERENCE_DEVICE_MAPPING_CONFLICT`. |
| 물리 범위 | 충족 | `utilization.gpu 120` → 원본 `120` 유지, 정규화 null, `physical_value_out_of_range`. 음수 전력 `-3.5`도 같다(온도는 음수 허용). 값 보정이나 장애 판정은 없다. |
| 엄격한 CSV 계약 | 충족 | 단위가 붙은 값(`37 %`), 다른 driver 버전, MIG UUID 접두(`MIG-…`), BOM은 거부되고 `quoted` 장치 이름(`"NVIDIA A100, 80GB"`)과 CRLF는 처리된다. |
| 요청 TTFT/TPOT | 충족 | 번들 요청 3건: 성공·3 tokens → TTFT 200 ms, TPOT `(1.8−1.2)/(3−1)` = `300000000`; 1 token → TTFT만 있고 TPOT `at_least_two_output_tokens_required`; 취소·0 token → `boundary_unavailable`. 1-token에 0 TPOT를 만들지 않는다. |
| scope·단위 검증(코드) | 충족 | 장치 telemetry를 worker에 붙이면 `INFERENCE_SCOPE_INVALID`, 한 series에 의미 하나, counter는 `sum`+monotonic, 순간 자원 gauge를 counter로 해석하면 거부, ratio는 0~1, 변환은 명시한 단위 쌍(`percent_to_ratio`, `mib_to_bytes`, `milliseconds_to_seconds`)만. 이름으로 자동 alias를 만들지 않으며 미연결 series는 원본에 남는다(`unmapped_series_count 1`). |
| 정규 set과 T-638 재사용 | 충족 | 번들 정규 set: 관측 51건 중 counter `first_cumulative_point` 4·`derived` 7·`reset` 1, 장치 N/A 3건. 원본 MetricHistory는 한 번만 저장된다. |
| T-637 R15-01 수정 확인 | 충족 | keyed-map bpftrace hist(`@latency:{nginx:[…], worker:[{"min":2,"count":9007199254740993}]}`)가 key별 행으로 보존된다(`nginx` 2행, `worker` 열린 tail + 큰 정수). 같은 파일의 flat 메시지와 한 파일에 섞어도 된다. |
| T-637 R15-02 수정 확인 | 충족 | 같은 stack을 반복 사용하는 현실적 perf script(400종 stack, 깊이 25): 20,000 sample(원본 51.2 MB) → 결과 18.6 MB, 24,000 sample(61.5 MB) → 22.2 MB, 투영 분모 `24000000000` ns. 한도에 닿으면 오류가 이제 바이트·한도·처리 record 수와 `perf script --time` 안내를 담는다(아래 R16-05 참고). |
| T-637 R15-06 수정 확인 | 충족 | `0.1`·`1.5`·`12.5`·`3.14`가 TSV에서도 변경되지 않는다. 분수·날짜·수식형은 계속 보호. |
| 이전 payload reader | 충족 | `system_evidence` 0.1.0 저장 fixture를 `system project-profile`로 그대로 열 수 있다(sample 3, weight 3). |
| 증거 무결성 | 충족 | 해시 114개(소스 98·증거 16) 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R16-01 | P2 | Prometheus range 기반 메트릭은 어떤 signal로도 연결할 수 없고 오류가 원인을 말하지 않음 | `inferencemodel/model.go` `Definition`, `parsers/metrichistory/json.go:151` |
| R16-02 | P2 | 구조화 event(runtime·요청)는 사용자가 만들 수 없는 자기 해시 ID가 필요해 fixture 밖에서 쓸 수 없음 | `inferencemodel/model.go:131,284` |
| R16-03 | P3 | NVIDIA CSV의 `[N/A]`와 끝 빈 줄이 파일 전체를 중단함 | `parsers/inference/source.go` `parseCSV` |
| R16-04 | P3 | 결과 용량이 약 8만 point이고 안내하는 "series 줄이기"가 이 경로에 없음 | `analyzers/inference/analyzer.go:197-202` |
| R16-05 | P3 | 모든 frame이 서로 다른 perf 입력은 여전히 약 4천 sample에서 한도에 닿음(문서화 필요) | `systemmodel` frame/stack 풀 |
| R16-06 | P3 | 요청 TPOT가 first==last인 다중 token 요청에서 `0`을 관측값으로 보고 | `analyzers/inference/analyzer.go:51-60` |

## 4 Finding 상세

### R16-01 Prometheus range 메트릭 연결 불가 (P2)

**근거**: Prometheus range 가져오기는 모든 series를 `gauge`, 단위 `""`로 만든다(`parsers/metrichistory/json.go:151`). 그런데 `Definition`은 counter 형식에 `sum`+monotonic, count 의미에 단위 `1/count/tokens/requests/…`, ratio 의미에 `1/ratio`를 요구한다. 따라서 range에서 온 series는 어떤 의미에도 맞지 않는다. 문서는 "DCGM/서빙 metric도 실제 정의를 확인하여 명시적으로 연결할 수 있다"고만 말한다.

**재현**: 실제 vLLM 이름의 range matrix(`vllm:num_requests_running`, `vllm:generation_tokens_total`, `vllm:gpu_cache_usage_perc`)를 `metrics history-normalize --format prometheus-range-json`으로 정규화하고, 서로 다른 의미 4가지로 바인딩했다.

| 바인딩 | 결과 |
|---|---|
| `running_requests`, gauge | `INFERENCE_COUNT_UNIT_MISMATCH` |
| `output_tokens`, counter | `INFERENCE_COUNTER_MISMATCH` |
| `output_tokens`, gauge | `INFERENCE_COUNT_UNIT_MISMATCH` |
| `kv_cache_usage`, gauge | `INFERENCE_UNIT_MISMATCH: ratio/` |

오류는 어느 signal·series·단위인지 말하지 않는다. vLLM·DCGM exporter 값은 대부분 Prometheus에 있으므로 가장 흔한 입력 경로가 차단된다. OTLP JSON이나 snapshot sequence(단위·종류를 선언)로 먼저 변환하면 동작하지만 문서에 그 우회가 없다.

**수정 제안**: 바인딩이 range series의 단위와 원래 종류를 **명시적으로 선언**하게 허용하거나(예: `declared_unit`, `declared_kind` — 원본 identity는 유지하고 선언임을 표시), 문서에 range는 연결할 수 없고 필요한 변환 절차를 적는다. 오류에는 signal ID·series 이름·원본 단위·기대 단위를 담는다.

### R16-02 구조화 event 입력 경로 부재 (P2)

**근거**: event는 `id == EventID(event)`(필드 전체의 SHA-256)를 요구한다(`inferencemodel/model.go:131,284`). ID를 계산하는 코드는 번들 fixture 생성 스크립트(`scripts/generate-inference-fixtures.go:135`)뿐이고 CLI·UI·문서 어디에도 사용자가 ID를 만들거나 채우는 방법이 없다. 번들 `runtime-observations.jsonl`은 importer 형식이 아니며 fixture가 바인딩 JSON에 이미 해시 ID를 포함해 둔다.

**재현**: 번들 `bindings.json`의 event 하나의 `id`를 빈 문자열 또는 임의의 `inference-event-000…`로 바꿔 `inference import-source --format metric-bindings-json`을 실행하면 둘 다 `INFERENCE_EVENT_INVALID`(원인 없음).

**영향**: 문서가 약속한 runtime 사건(CUDA/framework/container/host OOM, Xid, ECC, reset, 수명)과 **요청 TTFT/TPOT**가 fixture 밖에서는 입력할 수 없다. 현재 경로에서 TPOT 계산 기능을 쓰려면 사용자가 Go의 JSON 직렬화 순서에 맞춰 SHA-256을 손으로 계산해야 한다.

**수정 제안**: 가져오기에서 ID를 비워 두면 도구가 계산해 채우도록 하고(비어 있지 않은 ID는 계속 검증), 문서에 event JSONL/JSON 작성 예와 필드 규칙을 싣는다. 또는 서버 로그/요청 기록용 중립 event JSONL importer를 둔다.

### R16-03 CSV 견고성 (P3)

**근거/재현**: `parseCSV`는 `N/A`만 not-available로 인식한다. nvidia-smi가 지원되지 않는 항목에 `[N/A]`를 출력하는 것은 알려진 형태이지만(이 환경에서 실제 도구를 실행하지는 않았다) `[N/A]`가 들어간 행은 `INFERENCE_CSV_VALUE_INVALID line=1 column=5`로 **파일 전체**가 중단된다. 또 파일 끝에 빈 줄이 하나 더 있으면 `INFERENCE_CSV_INVALID line=2: EOF`로 중단된다. 오류는 값이나 열 이름을 보여주지 않는다. 문서는 `N/A`·`[Not Supported]`·`[Insufficient Permissions]`만 말한다.

**수정 제안**: `[N/A]`를 `N/A`와 같은 상태로 받아들이고 후행 빈 줄을 무시하며, 오류에 열 이름과 값을 담는다.

### R16-04 결과 용량과 안내 (P3)

**근거/재현**: 결과는 연결된 모든 signal의 관측 행(point당 약 800 B)과 원본 모델 전체를 담는다. 합성 OTLP gauge(10 series, `requests` 단위)로 측정: 40,000 point → 결과 32.9 MB, 90,000 point → `INFERENCE_RESULT_BUDGET: 74129481 bytes exceeds 67108864; select fewer original series/events`. 실질 상한은 약 8만 point다(문서 한도 100,000 point). 그런데 이 경로에는 series를 고르는 옵션이 없고(바인딩은 manifest, 정규화는 항상 전체 series 포함) 연결하지 않은 series도 모델에 남으므로 안내가 실행 가능하지 않다. T-638 R13-01과 같은 설계 패턴이다.

**수정 제안**: 모델에 포함할 series를 바인딩에서 고르게 하거나 연결된 signal의 series만 모델에 두고, 오류에 "정규화 전 원본을 나누세요" 같은 실제 가능한 조치를 적는다.

### R16-05 서로 다른 frame만 있는 perf 입력 (P3)

**근거/재현**: R15-02 수정은 같은 frame/stack을 풀로 공유해 중복을 없앤다. 반복 stack 입력은 위처럼 2만 sample 이상 처리된다. 반면 frame이 sample마다 모두 다른 입력(깊이 25, 주소·심볼이 sample별로 다름)은 4,012 sample에서 `SYSTEM_MODEL_BUDGET: bytes=16779234 limit=16777216 processed_records=4012; split capture with perf script --time …`로 중단된다. 오류 문구는 개선되었다. 실제 perf 출력은 호출 위치 주소가 반복되는 경우가 많아 위 최악은 드물지만, 문서는 용량이 stack 구성에 달려 있다는 점을 숫자와 함께 설명하지 않는다.

**수정 제안**: 문서에 "고유 frame 수"가 한도를 결정하며 반복 stack은 수만 sample, 고유 frame 위주는 약 4천 sample이라는 측정을 적는다.

### R16-06 first==last 다중 token 요청의 TPOT (P3)

**근거**: `request()`는 token이 2개 이상이고 first/last가 있으면 `(last−first)/(tokens−1)`를 계산한다. first와 last가 같은 값(예: 비스트리밍 응답의 도착 시각 하나만 기록)이면 TPOT가 `0`이고 상태는 `observed_token_boundaries`다. 문서는 SSE chunk를 token으로 추정하지 않는다고 하지만 이 경우의 0 값은 "관측된 token 간격이 0"처럼 읽힌다.

**수정 제안**: first==last이면 TPOT를 null과 `token_boundaries_not_distinct` 같은 상태로 두거나 문서에 이 경우를 명시한다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `4cd6003`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 111개 패키지 모두 `ok`(테스트 파일 없는 7개 제외)
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 114개 대조
- 실제 CLI 시나리오: 번들 NVIDIA CSV·바인딩·정규 set, CSV 변형 9종(`[N/A]`·단위·범위·빈 줄·CRLF·따옴표·MIG·다른 driver·BOM), Prometheus range 바인딩 4종, 이벤트 ID 변형 2종, 규모(4만·9만 point), R15 수정 확인(keyed hist·perf 2만/2.4만 sample·소수 표기·0.1.0 reader)
- throwaway 스크립트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 NVIDIA GPU·driver·`nvidia-smi` 출력, DCGM exporter, vLLM 서버와 `/metrics` 수집. 번들 자료는 모두 합성이고 이 환경에서도 실행하지 않았다. 따라서 `[N/A]` 출력 형태(R16-03)는 알려진 동작에 대한 기대이며 실제 도구 출력과 대조하지 않았다.
- 케이스 저장·원본 viewer 연동, Wails 브리지와 Workspace 화면(`InferenceEvidence.tsx`), 소유 `metrics_timeseries` 결과 생성의 앱 내 실행, Windows 실행, 오프라인 syscall 관측 재실행, hosted CI.
- event 요청 필드의 모든 경계 사례는 번들 3건과 코드 정독으로만 확인했다(사용자가 만들 수 없는 ID 때문에 임의 사례를 만들지 못했다).
- T-637 R15-03~R15-05(후속 큐)는 이 리뷰에서 다시 평가하지 않았다.

## 6 권고 처리 순서

1. R16-02 — event ID 자동 채움과 입력 형식·작성 예. T-640이 같은 입력 계약을 쓰기 전.
2. R16-01 — range series의 선언된 단위/종류 또는 명시적 문서와 상세 오류.
3. R16-03 — `[N/A]`·후행 빈 줄 허용, 오류에 열/값.
4. R16-04 — 모델 series 선택과 실제 가능한 조치 안내.
5. R16-05·R16-06 — 용량 문서화, first==last TPOT 상태.
