# 오프라인 LLM·gateway·lifecycle trace 근거

LLM trace 근거는 기존 `trace_import` 결과에 payload 0.3.0 / `llm_core` 1.0.0을
추가한다. 공통 AnalysisResult, 케이스 저장, provider 표·timeline 및 시각/보고서
export를 재사용한다. 기존 trace 0.2.0 reader와 전용 화면은 유지한다.

## 검증한 입력

OTLP JSON/JSONL(선택적 gzip)을 명시적 trace namespace·시계·의미 프로파일로
먼저 정규화한다. 제한 지원 importer `trace.llm-json`(alias `llm-trace`)는
본문 제외 LLMTrace 1.0.0과 `{}` 옵션을 받는다. 온라인 API·SDK 주입·자동
수집·제품별 JSON reader는 추가하지 않는다.

| 프로파일 | 검증 범위 |
|---|---|
| `neutral.v1` | 임베드 매핑의 명시적 `diag.llm.*`, `diag.gateway.*`, `diag.lifecycle.*` 속성 |
| `otel-genai.v1.37.0` | 고정 규약의 operation/provider/request model/input·output token 선택 필드. 전체 필드/버전 지원 아님 |
| `openinference.core.v1` | span kind/model/token 부분집합 자체 계약. 모든 OpenInference 라이브러리 버전 검증 아님 |

실제 OTel Python SDK 1.38.0 span 객체와 OTLP protobuf encoder로 고정 합성
fixture를 생성했다. 네트워크 exporter는 실행하지 않았으며 protobuf JSON의
base64 ID를 OTLP JSON hex ID로 변환했다. 의미 규약/경계 fixture는 별도 합성
자료다. Langfuse/LangSmith 제품 export를 검증한 것은 아니며 Langfuse 4.79.0 observations API v2는 별도 제한 adapter로 검증하고 그 밖의 **Q01 경로는 보류**한다.
공유 manifest가 원본·스키마·기대 모델 hash를 고정하고 앱이 필요한 자산을
임베드하므로 실행 시 형제 checkout이나 인터넷에 의존하지 않는다.

## 본문 제외와 원본 참조

기술 resource/scope identity, trace/span/parent ID, link, 원본 hash·위치, 정확한
시각, 선택한 role/model/token, 명시적 request/attempt 및 구조적 lifecycle만
유지한다. 원본 span 이름·임의 속성·prompt/completion·tool 인자·status message·
알 수 없는 event 본문은 결과 생성과 케이스 첨부 전에 제외한다. 본문 opt-in은
없다. allowlist의 기술 식별자와 model/provider ID도 사전에 민감 정보를 제거해야
한다. 제외 항목 수는 미선택 속성·이름/메시지·알 수 없는 event 수이며 모든 raw
JSON 필드 수를 의미하지 않는다.

지원 의미 prefix를 가진 raw LLM carrier는 기존 trace 분석과 새 케이스 원본
접수에서 거부한다. 케이스 텍스트 색인도 escaped key와 gzip을 포함해 해당 raw
자료를 거부한다. 먼저 정규화하고 그 모델과 결과만 첨부한다. 이 보호는 선언된
지원 계약을 식별하며 임의의 미지원 제품 본문까지 검출하지 않는다. 기존 raw
archive를 소급 수정하거나 삭제하지 않는다. hash는 바이트 동일성만 확인하며
생성 경위의 진위나 민감 정보 제거를 인증하지 않는다.

## 시간·그래프·합산 의미

- Parent와 link는 별도 관계다. 중복 span은 관측을 모두 남기고 우승자를 고르지
  않는다. 누락·모호한 parent, cycle, 역전 시간·unknown clock은 상태로 표시한다.
- Duration은 관측 end-start다. `observed_self_ns`는 포함 관계가 확인된 직접
  자식 구간의 합집합을 한 번 빼서 중첩 이중 계산을 방지한다. 불완전·순환·모호한
  그래프는 exclusive time을 보장하지 않는다. CPU self time이나 인과 critical
  path가 아닌 관측 경과 시간이다.
- 논리 요청은 request namespace와 logical ID, 시도는 추가 attempt ID가 필요하다.
  initial/retry/fallback/previous 관계는 선언이며 누락·순환을 표시한다. 시간 근접·
  동일 model 이름·span link로 identity를 복원하지 않는다.
- 동일 명시적 attempt의 유일한 직접 CLIENT→SERVER 쌍은 한 시도다. 중복·모호한
  관측은 표시하고 쌍의 token은 한쪽 authority를 직접 선택해야 노출한다. 단일
  관측은 자체 token을 표시할 수 있다. 시도 간/client-server token 합산·비용/
  청구 추정은 하지 않는다.
- Model/revision/tokenizer/method·provider/route·boundary·outcome·population은
  관측마다 보존한다. 논리/시도 표는 모든 선언 model·population을 보여 주며
  비교 호환성을 승인하지 않는다.
- Cold/warm/warmup/recovery/unknown은 명시적 population이며 주변 event로 추론하지
  않는다. startup/model load/compile/cache/restart/replacement/recovery는 시각이
  없어도 남기고 요청을 자동 제외하지 않는다.
- TTFT는 명시적 request start와 first token의 차이다. TPOT는 token≥2이고 서로
  다르며 순서가 맞는 경계에서만 `(last-first)/(output_tokens-1)`이다. 무한 소수는
  정확한 유리수 문자열이다. 누락·충돌·동일 경계는 null/status다. Network/SSE chunk는
  token으로 해석하지 않고 고정 event 매핑만 사용한다.

## 화면·CLI

Workspace에서 민감 정보를 제거한 OTLP를 선택하고 **LLM / gateway trace 근거**를
연다. 프로파일과 namespace를 지정하고 공통 Unix 시계가 확인된 경우만 `unix`를
선택한다. 확인 → 새 모델 저장 → `trace.llm-json`과 `{}`로 분석한다. Raw carrier를
케이스에 첨부하지 않는다. **LLM span·시도·lifecycle 상세**에서 span·정규 record·
parent/link·논리 요청·시도·event를 페이지 조회한다. Trace ID·record ID·명시적 request
필터는 정확히 일치한다. 맥락에서 service/instance·계측 scope/version/schema를
확인한다. 부모 결과 hash와 selector가 cursor를 묶고 선택 변경 시 UI 이전 요청을
취소한다. 기존 provider/timeline·HTML/Report Pack·선택 표 시각 export를 재사용한다.
전용 trace raster·native clipboard/Office·새 trace engine 검증은 포함하지 않는다.

```sh
archscope-engine trace llm-inspect --in source.json --options-file options.json
archscope-engine trace llm-normalize --in source.json --options-file options.json --out llm-trace-set.json
archscope-engine evidence import --in llm-trace-set.json --importer trace.llm-json --options '{}' --out result.json
archscope-engine trace llm-query --in result.json --query '{"kind":"attempts","limit":25}'
archscope-engine evidence export --in result.json --export-format report-pack --out report.zip
```

옵션 예:

```json
{"semantic_profile":"neutral.v1","trace_namespace":"sanitized-capture-a","producer":"declared collector","producer_version":"1","clock":"unix"}
```

`trace llm-import`는 정규화와 결과 생성을 결합한다. 새 출력은 덮어쓰기·관리 저장소를
거부한다. 원본/확장 각 64 MiB, 모델 32 MiB, 결과/스냅샷 64 MiB, span 5만,
context 4096, link/인식 event 각 10만, envelope 128 제한이다. 정규화 동시 2개,
5분 제한, 조회 최대 100행(UI 25), cursor 크기 제한을 적용한다. 개수 전에 byte
제한에 도달할 수 있으므로 원본을 trace 단위로 분할하며 조용히 자르지 않는다.
1만/2만 깊은 parent 합성 그래프와 저장 결과 reader는 로컬 검증했다. Native
Wails/Windows 및 실제 제품 export 실행 증거는 별도다.

정의 근거: [OTel GenAI 1.37.0](https://github.com/open-telemetry/semantic-conventions/blob/v1.37.0/docs/gen-ai/gen-ai-spans.md),
[OTLP trace protocol 1.7.0](https://github.com/open-telemetry/opentelemetry-proto/blob/v1.7.0/opentelemetry/proto/trace/v1/trace.proto),
[OpenInference conventions](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md).

Context 개요는 20개로 제한한다. contexts 조회로 일치하는 계측 맥락 전체를 제한 페이지로 확인한다.

기존 raw trace 분석 facade도 원본 64 MiB 초과 시 분할 안내와 함께 거부하여 일부만 검사한 tail을 접수하지 않는다. 기존 저장 reader는 유지하며 legacy 결과 발행 전에 해독된 의미 key를 다시 검사한다.


<a id="r17-입력조회-제약"></a>

## 입력·조회 제약

원본 carrier 거부는 GenAI/OpenInference/diag/Traceloop/Langfuse와 input.value/output.value
등 알려진 marker를 검사하며 escape/gzip도 포함합니다. 일반 속성명의 오탐 가능성이 있는
best-effort 검사이며 임의 본문 완전 탐지나 Langfuse/LangSmith adapter가 아닙니다.
미지원 export는 case copy 전에 정제하세요. 조회는 요청 페이지만 직렬화하고 서비스는
검증한 graph/model projection을 2개·2분·보수적 JSON charge 96 MiB cache에서 재사용합니다.
매 요청 소유권·파일 무결성을 다시 확인하며 CLI 프로세스 간 cache는 없습니다.

리뷰한 span당 약 950 byte 형식은 약 35k span에서 32 MiB에 도달할 수 있어 명목 50k보다
먼저 제한됩니다. 정규화 전에 원본 export를 trace별로 분할하세요. 초과 모델을 앱 안에서
trace 선택해 줄이는 기능은 없습니다.

| Profile | first/last-token 시간 지원 |
|---|---|
| neutral.v1 | 명시적 diag.llm first/last 경계 또는 인식하는 token event가 필요합니다. |
| otel-genai.v1.37.0 / openinference.core.v1 | 해당 필드를 보존하되 first/last 측정은 고정 custom 경계/event가 필요합니다. 전체 span 시간은 TTFT/TPOT가 아닙니다. |

`diag.llm.first_token_unix_nano`, `diag.llm.last_token_unix_nano`, `diag.llm.first_token`, `diag.llm.last_token`.

새 정규화는 token_timing_policy=token-boundaries.1.1.0을 기록합니다. 제공된 경계가 span 밖이거나 first>last이면 output token 수와 무관하게 TTFT/TPOT를 모두 null로 하고 token_boundary_order_invalid로 통일합니다. 정책 필드가 없는 저장 투영은 과거 계산을 유지하며 first=last는 구분된 TPOT 경계를 제공하지 않습니다.

| 의미 프로파일 | TTFT/TPOT 가능 조건 |
|---|---|
| neutral.v1 | 명시적 diag.llm first/last 속성 또는 event와 적격 span/clock/token 수 필요 |
| otel-genai.v1.37.0 | 표준 GenAI usage로 timing을 추정하지 않으며 명시적 diag.llm 확장 필요 |
| openinference.core.v1 | 표준 OpenInference span으로 timing을 추정하지 않으며 명시적 diag.llm 확장 필요 |

## Langfuse 4.79 observations API 선택(Q01)

의미 프로파일 `langfuse-observations.v4.79`, producer `langfuse`, 버전 `4.79.0`, 명시 namespace와 clock `unix`를 선택합니다. 본문 없는 observations API v2 JSON 선택 페이지를 받으며 이전 trace ingestion event나 LangSmith export는 다루지 않습니다. 고정 Docker 이미지로 로컬 self-hosted4.79.0을 실제 구동하고 합성 OTLP observation 세 건을 저장한 뒤 `GET /api/public/v2/observations?fields=core,basic,usage,model`로 읽었습니다. 외부 모델/서비스·cloud 계정은 사용하지 않았습니다.

`export-langfuse-observations.py --trace-id HEX --out new-export.json`은 LANGFUSE_HOST / LANGFUSE_PUBLIC_KEY / LANGFUSE_SECRET_KEY 환경에서 loopback 주소와 자격 증명을 읽어 body-free 필드를 선택하고 disk 전에 allowlist로 정제합니다. Key는 증거에 저장하지 마세요. 비어 있지 않은 pagination metadata는 거절하며 선택 페이지를 명시적으로 내보냅니다. 결과는 trace/전체 수집 coverage를 주장하지 않습니다.

명시적 usageDetails.input/output만 측정값입니다. API inputUsage/outputUsage의 기본 0은 무시하므로 누락은 관측 불가, 명시적 0은 0으로 유지합니다. GENERATION/RETRIEVER/EMBEDDING/SPAN을 role에 연결하고 이름·입출력 본문·임의 metadata·project/user/session·비용은 제외합니다. 관측되지 않은 boundary/outcome/token authority는 unknown이며 level ERROR만 failed로 연결합니다. 파생 latency/TTFT 필드에서 token 시각을 만들지 않습니다. 기존 LLMTrace1.0.0/trace_import0.3.0 조회·원본 좌표·공통 export를 재사용하고 기존 OTLP reader·저장 바이트를 유지합니다. LangSmith·다른 Langfuse 버전·client/server token 중복 제거를 검증한 자료는 아닙니다.
