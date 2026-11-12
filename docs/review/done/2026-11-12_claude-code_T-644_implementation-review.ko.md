# T-644 CPU/GPU 실행 구간 근거(PyTorch·Nsight) 구현 독립 리뷰

- 작성: claude-code (독립 리뷰어; 구현·처리와 다른 AI)
- 대상: `09413bf` (T-644 구현 + T-643 R20-01 처리). 이 리뷰는 구현 코드를 수정하지 않았다.
- 방법: 대상 커밋의 scratch worktree에서 CLI를 새로 빌드하고 격리 `ARCHSCOPE_STATE_ROOT`로 실행했다. PyTorch 2.15.0(CPU/Gloo)을 **별도 venv에 직접 설치**해 작성자와 다른 워크로드(record_shapes, 다른 연산·반복 수, 1·2 rank, 스택 유무)로 실제 트레이스를 만들었고, CPU self 시간·이벤트 수는 Python으로 독립 계산했다. Nsight는 이 환경에 `nsys`가 없어 번들 SQLite fixture와 `sqlite3`로 만든 변형 입력을 사용했다.

## 1. 결론

**CONDITIONAL — P2 1건, P3 3건, P1 없음.**

- 번들 범위(PyTorch 2.15.0 CPU/Gloo, Nsight 2024.2.3.38 / schema 3.11.1)의 정확도·개인정보·rank/clock 격리·변조 거부는 독립 입력에서도 맞다. T-643 R20-01 수정도 실제 Ollama로 확인했다.
- 다만 **원본 크기 한도가 시간 창 선택보다 먼저 적용되어**, 문서가 안내하는 "큰 SQLite는 창으로 좁힌다"가 동작하지 않는다(R21-01, P2). 실제 Nsight 캡처는 보통 64 MiB를 넘으므로 이 어댑터의 대상 입력이 사실상 가져와지지 않는다. R21-01 처리 후 T-644 DONE 복귀를 허용하고 R21-02~R21-04는 후속 큐로 충분하다.

## 2. 확인한 계약 충족 항목

| 항목 | 확인 내용 |
|---|---|
| 실제 PyTorch 트레이스 | 이 Mac에 PyTorch 2.15.0/Python 3.13을 새로 설치, 2-rank CPU/Gloo(matmul·relu·all_reduce·`record_function`)로 rank별 트레이스 생성. 두 rank 모두 가져오기 성공. rank 0 결과 228 이벤트 = 원본 `cpu_op` 216 + `user_annotation` 12와 정확히 일치. |
| 정확한 시간 | `ts=3628280780637.014`, `dur=504.592` µs가 `from_ns 3628280780637014`, `duration_ns 504592`로 정확히 변환된다. |
| CPU self 독립 대조 | 216개 CPU 연산의 `observed_self_ns`를 Python으로 독립 계산(포함 구간 − 같은 track 직접 자식 구간 합집합)해 **216/216 일치**. 상태 문구가 "source parent proof 없음"을 명시한다. |
| 개인정보 | 사용자 어노테이션 이름(`secret-step-N`)과 `host_name`, 사용자 경로가 결과에 0건. 어노테이션은 해시, `gloo:all_reduce`는 communication으로 분리. |
| rank/clock 격리 | 두 rank 병합 시 456 이벤트·8 track·source 2개. 같은 source 중복 병합은 `GPU_TRACE_DUPLICATE_SOURCE`, source 지정 없는 시간 질의는 `GPU_TRACE_SOURCE_CLOCK_REQUIRED`, 지정 없는 SVG는 `GPU_TRACE_SVG_SOURCE_REQUIRED`로 거부. `rank:"1"` 필터 동작. |
| 시간 창 | 6만 이벤트 트레이스(5만 한도 초과)에 절반 구간 창을 주면 30,049개로 가져오며 독립 기대값과 같다. 5만 초과 시 `GPU_TRACE_EVENT_BUDGET: observed=50001 limit=50000; … intervals are never silently truncated`로 거부된다. |
| Nsight 합/합집합 | 번들 SQLite에서 kernel 7개 합 38,944 ns, 합집합 38,944 ns가 `sqlite3` 독립 계산과 같다. 겹치는 스트림 kernel을 직접 추가한 변형(14개)에서 합 77,888 ns·합집합 **57,696 ns**가 독립 계산과 정확히 일치. |
| Nsight 거부 | `end < start` 행(`…TIME_OR_NAME_INVALID at …/rowid/1`), 다른 export 버전(`GPU_TRACE_NSYS_VERSION`), 빈 `device_map`(`GPU_TRACE_DEVICE_MAPPING_REQUIRED`)이 모두 코드와 함께 거부된다. |
| SVG | 단일 source 선택 SVG가 well-formed XML이고 외부 리소스·script 없음, 병합 결과 SVG는 선택한 source 한 곳의 provenance만 포함한다. |
| T-643 R20-01 | 이 Mac의 실제 Ollama 0.40.16에서 gemma:2b와 nomic-embed-text를 모두 로드 후 collector 1.1.0으로 캡처: `--model gemma:2b`는 `selected_model_match_count 1 / excluded_model_count 1`, 오타(`gemma2b`)는 `0 / 2`로 기록되고 가져온 결과에 `ollama.selection.excluded_model_count` 시리즈가 생긴다. 제외 모델 이름은 저장되지 않는다. 두 모델은 내렸고 `/api/ps`가 `{"models":[]}`로 복원됐다. |

## 3. Finding 요약

| ID | 심각도 | 요약 |
|---|---|---|
| R21-01 | P2 | 원본 크기·원본 이벤트 수 한도가 시간 창보다 먼저 적용되어 큰 Nsight/PyTorch 캡처는 창을 줘도 가져올 수 없음; 문서는 반대로 안내 |
| R21-02 | P3 | 선택 테이블이 없거나 VIEW로 바뀌어도 조용히 건너뜀(문서는 view 거부라고 적음); 결과에 누락 고지 없음 |
| R21-03 | P3 | 한도 오류 문구가 모호함(`TORCH_SCHEMA_OR_RECORD_BUDGET`에 수치 없음, `observed=50001`은 정확한 개수가 아님) |
| R21-04 | P3 | 번들 Nsight fixture가 합=합집합이라 겹침 계산을 검증하지 못함; 모호 상관·flow 경로의 실제 입력 부재 |

## 4. Finding 상세

### R21-01 (P2) 시간 창이 원본 크기 한도를 줄여 주지 못한다

- 근거: `parsers/gputrace/source.go:64,76` 입력 64 MiB 한도, `parsers/gputrace/pytorch.go:67` `len(input.Events) > 100000`(창 적용 전, 무시되는 metadata·`python_function` 포함). 문서 `docs/en/GPU_EXECUTION_EVIDENCE.md:118` "Large SQLite exports need a smaller explicit window; no silent first-N truncation."
- 재현(Nsight): 번들 SQLite에 70 MiB 패딩 테이블을 추가해 73.5 MB 파일을 만들고 번들 옵션(+`from_ns/to_ns`)으로 `inspect-source`: `IMPORT_BUDGET_EXCEEDED: input_bytes 73535488 exceeds 67108864`. 창 선택 SQL은 실행되기 전에 거부된다. 작성자가 해시로 검증한 전체 export(41.4 MB)는 한도 안이라 통과했을 뿐이다.
- 재현(PyTorch): 같은 소형 연산을 반복한 실제 트레이스를 만들어 비교 — 5.98만 이벤트는 창으로 가져옴, **10.35만 이벤트**는 창과 무관하게 `GPU_TRACE_TORCH_SCHEMA_OR_RECORD_BUDGET`(수치·조치 없음)으로 거부, 20.7만 연산(80.6 MiB)은 64 MiB 한도로 거부. `with_stack=True` 트레이스는 `python_function`·metadata가 원본 수에 포함된다.
- 영향: 실제 Nsight 캡처(수 초 분량도 수백 MB)는 거의 모두 64 MiB를 넘고, `nsys export`에는 시간 창 옵션이 없다. 사용자가 따를 수 있는 공식 경로가 없어 어댑터의 주 입력이 사실상 막힌다. 문서의 안내가 동작하지 않는 방향이라 사용자가 원인을 오해한다.
- 권고: (a) SQLite는 임시 복사 대신 경로에서 읽기 전용으로 열어 크기 한도를 창 선택 이후 행 수·이벤트 수 한도로 바꾸거나, (b) 그렇지 않다면 문서에서 "원본 64 MiB 이하만 지원"으로 바로잡고 사용자가 `sqlite3`로 범위를 잘라내는 방법과 PyTorch 프로파일러 `schedule`로 구간을 줄이는 방법을 안내한다. PyTorch 원본 수 한도는 지원 이벤트 수만 센다.

### R21-02 (P3) 선택 테이블 누락·VIEW가 조용히 건너뛰어진다

- 근거: `parsers/gputrace/sqlite.go`에서 `tables[name]`은 `kind == "table"`일 때만 채워지고, 필수 검사는 `META_DATA_EXPORT`·`StringIds`뿐이다. 나머지 4개 테이블은 `if !tables[def.table] { continue }`로 건너뛴다. 문서(`…:121`)는 "views … refused"라고 적는다.
- 재현: `CUPTI_ACTIVITY_KIND_KERNEL`을 VIEW로 바꾼 파일과 이 테이블을 DROP한 파일 모두 `rc 0`으로 가져와진다. 이벤트 19→12, `gpu_device_count` 1→0, `busy` 질의는 빈 결과이며 경고·limitation 어디에도 "kernel 테이블 없음"이 없다.
- 영향: 결과가 "GPU kernel 활동이 없었다"로 읽힌다. 실제로 kernel이 없던 캡처와 VIEW/삭제로 가려진 캡처를 구분할 수 없다.
- 권고: 지원 테이블 4종 각각에 대해 `present/absent/view_refused`를 결과(경고 또는 source 메타)에 기록하고, VIEW·가상·생성 열은 문서대로 거부한다.

### R21-03 (P3) 한도 오류 문구

- `GPU_TRACE_TORCH_SCHEMA_OR_RECORD_BUDGET`은 schema 불일치와 원본 수 초과를 한 코드로 묶고 관측 수·한도가 없다. `GPU_TRACE_EVENT_BUDGET: observed=50001`은 "한도를 넘은 첫 값"이지 전체 개수가 아닌데 `observed`로 표기된다.
- 권고: schema와 record budget을 분리하고 원본 수·지원 이벤트 수·한도를 표기하며, 정확한 개수를 모르면 `at_least`로 쓴다.

### R21-04 (P3) fixture가 핵심 계산을 구분하지 못한다

- 번들 Nsight fixture는 kernel 7개가 겹치지 않아 합과 합집합이 같은 38,944 ns다. 합집합 구현이 합을 그대로 반환해도 통과한다(제 겹침 변형 입력에서는 올바랐다). 또한 모호 상관(여러 runtime 후보)·flow endpoint·CUDA PyTorch 형태는 구조 테스트만 있고 실제 입력이 없다. 문서는 CUDA/NCCL을 미검증으로 밝히므로 사양 위반은 아니다.
- 권고: 스트림이 겹치는 kernel을 가진 합성 SQLite fixture(또는 제 변형 입력과 같은 방식의 테스트)를 회귀로 추가한다.

## 5. 실행한 검증과 미검증 항목

**실행함**
- 대상 커밋 scratch worktree 빌드, `go vet ./...`(host), `GOOS=windows GOARCH=386 go vet ./...` 통과.
- `go test -race -count=1`: `parsers/gputrace`, `analyzers/gputrace`, `parsers/metrichistory`, `catalog`, `cmd/archscope-engine`, `cmd/archscope-app` 모두 `ok`(`exporters/gputrace`는 테스트 파일 없음).
- 실제 PyTorch 2.15.0 트레이스 2-rank·대용량 변형 4종, Nsight 번들 fixture와 변형 6종, 실제 Ollama collector 1.1.0 캡처 2종, SVG 2종.

**실행하지 않음**
- 전체 118개 패키지 race 스위트, Wails WebView·Playwright 화면, 네이티브 Windows, hosted CI, 오프라인 syscall 57건 재실행, Mac 패키징·서명.
- 실제 `nsys` 실행과 export(도구 없음; 번들 fixture·변형만 사용). PyTorch CUDA/NCCL, 다른 PyTorch/Nsight 버전, 다중 호스트.
- T-643 R20-02~R20-04, T-642 R19-02~R19-04는 후속 큐 상태 그대로 두고 다시 판정하지 않았다.

## 6. 권고 처리 순서

1. R21-01 — 크기·원본 수 한도를 창 이후로 옮기거나 문서를 바로잡고 줄이는 방법 안내(이후 T-644 DONE 복귀 허용).
2. R21-02 — 테이블별 존재/누락 고지, VIEW 거부.
3. R21-03 — 한도 오류 분리·수치 표기.
4. R21-04 — 겹침·모호 상관 회귀 fixture.
