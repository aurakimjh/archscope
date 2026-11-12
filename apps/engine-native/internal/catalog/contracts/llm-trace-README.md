# 본문 제외 LLM trace 계약 1.0.0

CC0 합성 데이터만 포함한다. 제품별 실제 고객 export가 아니다.

- `generate_sdk.py`: OTel Python SDK 1.38.0의 실제 span 객체를 생성하고 OTLP protobuf exporter encoder를 사용한다. 고정 ID/시간과 합성 body marker를 사용한다. protobuf MessageToDict 출력의 base64 ID를 OTLP JSON용 hex ID로 변환한다. 통신하지 않는다. SDK 및 protobuf 버전은 스크립트에 고정된다.
- `sdk-python-1.38.0.otlp.json`: 위 SDK carrier 검증. 라이브러리의 GenAI 계측 전체를 검증하지 않는다.
- `neutral.v1`: 명시적 diag.llm / diag.gateway / diag.lifecycle 계약이다. 프로파일은 semantic-profiles.json에 고정된다.
- `otel-genai.v1.37.0`: 선택한 operation/provider/model/token 속성만. 다른 버전 및 전체 GenAI 규약 호환성은 주장하지 않는다.
- `openinference.core.v1`: 명시적 span kind/model/token 부분집합의 자체 고정 계약이다. 모든 OpenInference SDK 버전 지원이 아니다.
- edge-cases: client/server pair, retry/fallback, orphan, cycle, duplicate, reversed/equal/missing time, link, untimed recovery를 보존한다.
- `*-set.json`: 정규화 결과. 원본 이름, 임의 속성, prompt/completion, tool argument, status message 및 알 수 없는 event 본문은 제외한다. 기술 식별자는 입력 단계에서 이미 민감 정보를 제거해야 한다. omitted_field_count는 미선택 속성/원본 이름/메시지/알 수 없는 event 항목 수이며 모든 raw JSON 필드의 수가 아니다.

parent와 link, 논리 요청과 시도, client/server 및 lifecycle population은 구별한다. 시간 근접 연결, client/server token 합산, 시도 간 token 합산, 인과 critical path 추론은 하지 않는다. Q01 Langfuse/LangSmith JSON은 검증 대상이 아니다.

근거 규약: https://github.com/open-telemetry/semantic-conventions/blob/v1.37.0/docs/gen-ai/gen-ai-spans.md , https://github.com/open-telemetry/opentelemetry-proto/blob/v1.7.0/opentelemetry/proto/trace/v1/trace.proto , https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md .

재생성: SDK generator를 고정 의존성 환경에서 실행한 후 workspace app의 scripts/generate-llm-fixtures.go에 이 디렉터리를 지정한다. manifest와 프로젝트의 컴파일 임베드 복사본을 함께 갱신한다. 모델 32 MiB / 결과 64 MiB / 원본 및 확장 각각 64 MiB가 개수 제한보다 먼저 적용된다. 제한 초과 시 원본을 trace 단위로 분할하며 조용히 자르지 않는다.

새 정규화는 source.token_timing_policy=`token-boundaries.1.1.0`을 기록합니다. 제공된 token 경계가 span 또는 서로 모순되면 TTFT·TPOT를 모두 null로 하고 token_boundary_order_invalid로 통일합니다. 필드가 없는 기존 모델·저장 결과는 기존 계산을 유지합니다. GenAI/OpenInference 표준 span에는 token 경계가 자동으로 존재하지 않으며 명시적인 diag.llm 속성·event 확장이 필요합니다.
