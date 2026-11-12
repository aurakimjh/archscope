# T-640 LLM·gateway·lifecycle trace 근거(LLMTrace) 구현 독립 코드 리뷰 — 2026-11-12

- 대상: T-640 구현 커밋 `eadc040`(feat(trace): implement T-640 LLM evidence and address T-639 review). 직전 `a740ed0`(T-639 리뷰) 위에 놓이며, 이 커밋은 T-639 리뷰 R16-01~R16-06의 로컬 수정(inference payload 0.2.0, 선언된 range 메타데이터, event ID 생성 등)도 함께 포함한다.
- 계약 기준: `work_status.md`의 T-640 행과 완료 기록, `docs/ko/LLM_TRACE_EVIDENCE.md`, `docs/ko/INFERENCE_EVIDENCE.md`의 R16 수정 절, `docs/ko/DATA_MODEL.md`·`PARSER_DESIGN.md`의 T-640 절
- 증거: `docs/review/evidence/2026-11-12-t640/local.json`(`source_index_files` 101개와 `evidence_files` 14개의 SHA-256이 `eadc040` 커밋 내용과 모두 일치)
- 작성: claude-code가 독립 리뷰 작성 역할만 맡았다. 처리(수정·`work_status.md` 반영·`done/` 이동)는 다른 AI가 한다. 구현 코드와 기존 미커밋 변경(HTTP 문서 변경 등)은 수정하지 않았다.
- 방법: 소스 정독(`llmmodel/model.go`, `parsers/traceimport/llm.go`, `analyzers/traceimport/llm.go`·`llm_query.go`, `analysis/llm_trace.go`, `cmd_llm_trace.go`, `analyzers/inference/analyzer.go`의 관측 변환), 저장소 밖 scratch worktree(`eadc040`)에서 빌드한 실제 CLI로 격리 상태 루트에서 번들 fixture 4종(실제 OTel Python SDK 1.38.0, GenAI, OpenInference, edge)과 직접 만든 OTLP 입력(개인정보 포함 1건, 그래프·token·attempt 사례 30 span, 규모 2종)을 `trace llm-inspect/normalize/import/query`, 기존 `trace import`, `case edit source`로 끝까지 실행했다. T-639 R16 수정은 `inference import-source`로 확인했다. throwaway Go 테스트 1개를 쓰고 삭제했다.
- 범위: LLMTrace 1.0.0 / `trace_import` 0.3.0 / `llm_core`(본문 제외 정규화, parent·link, 구간 합집합 self time, 논리 요청·시도·token authority, TTFT/TPOT, lifecycle), 원본 carrier 거부 규칙, 질의·cursor, CLI·Wails 브리지, 그리고 T-639 R16 수정 확인. 실제 Langfuse/LangSmith export(Q01), 네이티브 WebView·Windows는 범위 밖이다.

## 1 결론

**판정: 조건부(P2 1건, P3 4건).** T-640의 핵심 계약은 실제 실행에서 성립했다. 본문(prompt·completion·시스템 지시·span 이름·status message·event/link/scope/resource 속성)을 담은 OTLP에서 결과와 정규화 모델에 비밀 문자열이 **0건**이었고 제외 필드 수 13건이 기록됐다. parent/link가 분리되고, cycle·중복·누락 parent가 명시되며, 구간 합집합을 뺀 self time이 정확하고(100−(10~60 합집합 50+10)=40 ms), 시도 체인·cycle·client/server 쌍·token authority·TPOT 계산이 정확하다. 지원 의미 prefix를 가진 raw carrier는 기존 trace 분석과 케이스 원본 접수(평문·gzip·유니코드 escape)에서 거부된다. 다만 **같은 커밋에서 확인한 T-639 observation 변환에 정확성 결함이 있다.** 비정수 rate(예: 15초 간격 +100 → `20/3`)가 조용히 null이 된다(R17-01, 내가 T-639 리뷰에서 놓친 항목이다). 나머지는 용량·성능·분류에 관한 P3이다. T-639 리뷰 R16-01~R16-03 수정은 실제로 동작한다.

- **T-640 `DONE` 판정:** R17-01(P2)을 처리한 뒤 `DONE` 복귀를 허용한다. R17-02~R17-05는 후속 큐에 넣으면 된다. 실제 제품 export·네이티브 Windows/WebView는 작성 측이 밝힌 대로 별도 항목이다.
- **T-639와의 관계:** R17-01은 T-639가 도입한 `Observations` 변환(`analyzers/inference/analyzer.go`)의 결함이며 T-639의 `DONE` 근거(R16 수정)와는 별개로 후속 수정이 필요하다.

## 2 확인한 계약 충족 항목

| 계약 | 결과 | 근거 |
|---|---|---|
| 본문 제외 | 충족 | `gen_ai.input.messages`·`gen_ai.output.messages`·`gen_ai.system_instructions`·`user.email`·`http.url?token=`·span 이름·status message·이벤트 이름/속성·link 속성·scope/resource 속성 등 12개 위치에 `SECRET_*` 표지를 넣고 GenAI 프로파일로 정규화·가져오기: 정규화 모델과 결과에서 `SECRET` 0건, `omitted_field_count` 13. 허용 목록 필드(`model` 등)만 남는다. |
| 구간 합집합 self time | 충족 | 부모 0~100 ms, 자식 10~40·30~60·80~90 → self `40000000` ns(겹치는 자식을 한 번만 뺌). 자식이 부모 구간을 벗어나면 `child_interval_unqualified`로 self를 만들지 않는다. |
| 그래프 감사 | 충족 | 상호 parent 두 span과 자기 parent 하나가 모두 `cycle`(cyclic 3), 같은 span ID 두 개가 `duplicate_span_id`(우승자 선택 없음), 없는 parent가 `missing_parent`. 해당 span의 self는 `graph_ambiguous`/null. |
| token 경계 | 충족 | output 5 tokens, first 1.2 s·last 2.2 s → TTFT `200000000`, TPOT `250000000`(=1000 ms/4); first==last → `token_boundaries_not_distinct`; last<first → `token_boundary_order_invalid`; token 1개 → `tpot_requires_at_least_two_tokens`; 경계 하나만 있으면 `observed_first_token`. |
| 논리 요청·시도 | 충족 | a1(initial)→a2(retry)→a3(fallback) 체인이 `explicit_previous_attempt`, a4↔a5 순환이 `previous_attempt_cycle`, CLIENT→SERVER 직접 쌍이 `explicit_client_server_pair`, token authority를 `server`로 선언하면 서버 값(90)만 노출하고 선언하지 않으면 `not_aggregated`, 같은 시도의 관측 3개는 `ambiguous_attempt_observations`. 시도 간·client/server token 합산은 없다. |
| raw carrier 거부 | 충족 | 기존 `trace import`가 `LLMTRACE_SEMANTIC_REQUIRED`로 거부, `case edit source`가 평문 `CASE_LLM_SANITIZED_SOURCE_REQUIRED`·gzip 모두 거부, 유니코드 escape된 키(`gen_ai.input.messages`)도 거부. |
| 번들 fixture | 충족 | 실제 OTel Python SDK 1.38.0 표본 6 span·1 link·3 event·1 시도, GenAI 1 span, OpenInference 1 span, edge 10 span(cyclic 2·duplicate 2·missing parent 1·attempt 2). 모두 payload `0.3.0`. |
| T-639 R16-01 수정 | 충족 | Prometheus range(단위 빈 gauge)를 `declared_metadata`로 연결: 카운터(`tokens`/`sum`/cumulative/monotonic/start 0), gauge(`requests`), 비율(`1`) 모두 성공. 선언이 맞지 않으면 `INFERENCE_SIGNAL_DEFINITION: signal=… series=… original_kind=gauge original_unit="": INFERENCE_DECLARATION_INVALID…`처럼 원인을 말한다. `start_ns`를 선언하지 않으면 `start_time_unavailable`로 rate를 만들지 않는다. |
| T-639 R16-02·R16-03 수정 | 충족 | 사용자 바인딩의 빈 event ID가 정규화에서 생성되어 payload `0.2.0` 결과와 요청 TTFT 200 ms·TPOT 300 ms가 나오고, 틀린 ID는 `INFERENCE_EVENT_INVALID`. `[N/A]`는 `not_available: [N/A]`로, 끝 빈 줄은 무시되며, 잘못된 값은 `line=1 column=5 field=utilization.gpu value="abc"`로 안내된다. |
| 증거 무결성 | 충족 | 해시 115개(소스 101·증거 14) 일치. |

## 3 Finding 요약

| ID | 등급 | 제목 | 주 위치 |
|---|---|---|---|
| R17-01 | P2 | inference 관측 표에서 비정수 rate가 조용히 null이 됨(rate 의미 표시는 남음) | `analyzers/inference/analyzer.go` `Observations`의 `convert(v.Rate)` |
| R17-02 | P3 | 원본 carrier 거부가 `gen_ai.`/`openinference.`/`diag.*` 표지에 한정되어 흔한 계측의 본문이 케이스에 평문 저장됨 | `parsers/traceimport/llm.go` `RequiresLLMNormalization` |
| R17-03 | P3 | 질의 쪽마다 전체 그래프 재투영과 전체 행 직렬화(2만 span에서 쪽당 약 3.9 s) | `analyzers/traceimport/llm_query.go:48-125` |
| R17-04 | P3 | 문서의 5만 span 한도는 모델 32 MiB 때문에 약 3만 5천 span에서 도달 | `llmmodel/model.go:15-17,Validate` |
| R17-05 | P3 | token 경계 분류가 일부 모순 사례를 섞어 표시하고, 실제 GenAI 계측 span에는 경계 속성이 없음 | `analyzers/traceimport/llm.go` token 경계 구간 |

## 4 Finding 상세

### R17-01 비정수 rate 소실 (P2)

**근거**: `Observations`는 스칼라 signal의 delta·rate를 `convert`로 변환하는데 이 함수가 `m.Number`로 값을 파싱한 뒤 단위 배율을 곱한다. 구간 rate는 T-638에서 정확한 분수 문자열(`RatString`, 예 `20/3`)로 나오지만 `m.Number("20/3")`은 `invalid exact decimal` 오류라 `convert`가 `nil`을 반환한다. 반면 `rate_meaning`과 `delta_unit`은 그대로 채워지고 `rate_unit`만 비어, 행이 "increment_per_second인데 값 없음"으로 보인다. 히스토그램 경로는 `convert`를 거치지 않고 문자열을 그대로 쓰므로 영향이 없다.

**재현**: vLLM 이름의 Prometheus range(15초 간격, 누적 0·100·200·300·400)를 선언 메타데이터(`tokens`, `sum`, cumulative, monotonic, `start_ns 0`)로 연결한 결과의 관측: delta `100`, `status derived`, `rate_meaning increment_per_second`, **`interval_rate null`, `rate_unit ""`**. 번들 fixture에서는 rate가 `10`·`2` 같은 정수라 이 결함이 드러나지 않는다. T-638 구간 계산은 rate를 `RatString`(분수 문자열)으로 만들고, throwaway 테스트로 `Number("10")`는 성공하지만 `Number("20/3")`·`Number("1/2")`는 `invalid exact decimal`임을 확인했다.

**영향**: 간격이 나누어떨어지지 않는 모든 counter·precomputed rate signal(대부분의 실제 간격)의 rate가 inference 결과에서 사라진다. 보고된 값이 없어 오해 소지는 작지만 핵심 파생 값이 누락된다. T-639 리뷰에서 번들 fixture만 보고 놓쳤다.

**수정 제안**: 분수 문자열을 `big.Rat.SetString`으로 파싱하고, 파싱 실패 시 조용히 null로 두지 말고 상태(`rate_unconvertible`)를 남긴다. 비정수 rate와 단위 배율(`percent_to_ratio` 등) 조합의 회귀 테스트를 추가한다.

### R17-02 raw carrier 판별 범위 (P3)

**근거**: `RequiresLLMNormalization`은 `gen_ai.`·`openinference.`·`diag.llm.`·`diag.gateway.`·`diag.lifecycle.` 부분 문자열이 있는 파일만 raw LLM carrier로 거부한다. 문서도 "임의의 미지원 제품 본문은 검출하지 않는다"고 밝힌다.

**재현**: Traceloop 형식 `llm.prompts.0.content`, Langfuse 형식 `langfuse.observation.input` 속성에 표지 문자열을 넣은 OTLP(위 표지 prefix 없음)는 `trace import`가 그대로 분석하고 `case edit source`가 케이스 객체 저장소에 복사한다. 저장된 객체 파일에서 `SECRET_LF_INPUT`이 평문으로 확인되었다.

**영향**: 위험은 문서화되어 있으나 가장 흔한 두 계측 계열의 본문이 케이스(공유·내보내기 대상)에 평문으로 들어간다. 표지 목록 확장은 작은 비용이고, 케이스 추가 시 LLM 본문 가능성에 대한 경고 문구도 없다.

**수정 제안**: 표지에 `llm.prompts`·`llm.completions`·`traceloop.`·`langfuse.`·`ai.prompt`·`input.value`/`output.value`(OpenInference 본문) 등을 추가하고 문서에 목록과 한계를 적는다. 이 보호는 선언된 표지의 최선 노력임을 UI 접수 단계에도 표시한다.

### R17-03 질의 쪽마다 전체 재처리 (P3)

**근거**: `QueryLLM`은 쪽마다 `ProjectLLM`(전체 span 그래프·self time·attempt)을 다시 계산하고, 일치하는 모든 행을 JSON으로 직렬화한 뒤 `offset`~`limit` 구간만 잘라 반환한다(`llm_query.go:48-125`). Wails 경로(`withLLM`)는 여기에 결과 파일 읽기·디코드·`ReadLLMCore` 검증이 더해진다. T-638 R13-02·T-635 R12-01과 같은 부류다.

**재현**: 2만 span 한 개 체인(결과 36.1 MB)에서 CLI `trace llm-query --query '{"kind":"spans","limit":25}'`가 쪽당 3.9 s. 4만 span 입력은 모델 한도로 가져오기 단계에서 거부되어 측정하지 못했다.

**수정 제안**: 검증된 모델과 투영을 `(결과 digest)` 키로 수명이 짧은 캐시에 두고, 일치 행 전체를 직렬화하지 말고 쪽 범위만 직렬화한다.

### R17-04 문서 용량과 실질 용량 (P3)

**근거/재현**: span당 정규화 모델이 약 950 B라 4만 span(원본 18.2 MB) 합성 입력이 `LLMTRACE_MODEL_BUDGET: bytes=38011489 limit=33554432 spans=40000; split or select original traces`로 거부된다. 문서의 span 5만 한도는 모델 32 MiB 한도가 먼저 닿는다(문서도 "개수 전에 byte 제한에 도달할 수 있다"고 적음). 오류에는 바이트·한도·span 수가 있어 이전 리뷰들보다 낫다. 2만 span 체인은 결과 36.1 MB로 성공한다.

**수정 제안**: 문서에 span 길이 기준 실질 상한(약 3만 5천)을 측정값으로 적고, 가능하면 context·반복 문자열을 공유 표로 옮겨 span당 크기를 줄인다. "또는 trace 선택" 안내가 가능한 경로인지(trace 선택 옵션이 있는지)도 정리한다.

### R17-05 token 경계 분류 (P3)

**근거/재현**: 첫 token이 span 시작 전(900 ms < 1000 ms)이면 TTFT 없이 `token_boundary_order_invalid`, first(1500 ms) > last(1200 ms)이면 TTFT `500000000`을 보고하면서 TPOT는 `token_boundary_order_invalid`로 둔다. 즉 같은 "모순된 경계"에서 한쪽은 TTFT를 내고 다른 쪽은 내지 않는다. 또 실제 OTel GenAI 계측 span에는 `diag.llm.first_token_unix_nano`류 속성이나 `diag.llm.first_token` event가 없어(프로파일은 이 이름만 매핑) GenAI·OpenInference 프로파일에서는 TTFT/TPOT가 항상 null이고, 계측을 직접 추가한 neutral 프로파일에서만 계산된다.

**수정 제안**: 경계가 모순이면 TTFT·TPOT를 같은 규칙으로 null 처리하고 상태를 하나로 통일한다. 문서에 프로파일별 TTFT/TPOT 가능 여부를 표로 적는다.

## 5 실행한 검증과 미검증 항목

**실행함**(scratch worktree `eadc040`, Mac arm64):
- `go vet ./...` 통과, `go test -race -count=1 ./...` 111개 패키지 모두 `ok`(테스트 파일 없는 8개 제외)
- Windows amd64·386 교차 `go vet ./...` 통과
- 프런트엔드 `tsc --noEmit`, `npm run test:state` 통과(node_modules는 작업 트리 심볼릭 링크)
- 증거 JSON 해시 115개 대조
- 실제 CLI 시나리오: 번들 fixture 4종 가져오기, 개인정보 포함 OTLP(12개 위치) 정규화/가져오기와 비밀 문자열 검색, 그래프·token·attempt 30 span 사례, raw carrier 거부(기존 trace import·case source 평문/gzip/유니코드 escape·미지원 벤더 형식), 규모(2만 체인·4만 광폭), 질의 쪽 시간, T-639 R16 수정 확인(range 선언 5종·event ID·`[N/A]`·값 오류)
- throwaway 스크립트·생성 파일은 scratchpad에만 두었고 저장소에는 아무것도 넣지 않았다.

**미검증**:
- 실제 Langfuse/LangSmith export(Q01 유지), 실제 계측 라이브러리(OpenInference 전체·GenAI 다른 버전)로 만든 trace, 실제 gateway/vLLM 로그
- Wails 브리지·Workspace 화면(`LLMTraceEvidence.tsx`)의 앱 내 실행, 네이티브 Windows, 오프라인 syscall 관측 재실행, hosted CI
- T-639 R16-04(`retained_series_ids`)·R16-06(payload 0.2.0 first==last TPOT)은 코드와 단위 테스트 통과만 확인했고 직접 입력으로 재현하지 않았다.
- 케이스 텍스트 색인 거부는 `case edit source`로만 확인했고 색인 경로 자체를 별도로 실행하지 않았다.

## 6 권고 처리 순서

1. R17-01 — 분수 rate 파싱과 회귀 테스트. T-639 결과에도 소급 적용(0.2.0 reader 호환 유지).
2. R17-02 — raw carrier 표지 확장과 접수 경고. 케이스에 본문이 들어가기 전.
3. R17-03 — 투영·검증 캐시와 쪽 범위 직렬화.
4. R17-04·R17-05 — 용량 문서화, 모순 경계 분류 통일.
