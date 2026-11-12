# 메트릭 시계열 합성 계약 표본

- `sdk-python-1.38.0.otlp.json`: OpenTelemetry Python SDK 1.38.0 및 공식 OTLP encoder로 합성 counter, up/down counter, explicit/exponential histogram을 2회 수집한 실제 SDK 출력. 큰 정수, resource/scope/schema, point 속성을 포함한다. 운영 자료는 없다.
- `generate_sdk.py`: 격리 venv에서 `opentelemetry-sdk==1.38.0 opentelemetry-exporter-otlp-proto-http==1.38.0 protobuf==6.33.6` 설치 후 실행한다. 실제 수집 시각이 달라지므로 재생성 시 manifest도 갱신한다.
- `snapshots.json`: exact cumulative 합계와 reset/gap 조건의 수작업 합성 표본.
- `prometheus-range.json` + `prometheus-query.json`: range matrix 및 명시적 query/step 출처. matrix 값의 원래 instrument type/단위를 추정하지 않는다.
- `metric-history-set.json`: 위 SDK 원본에서 정규화한 bounded canonical transport. 정확한 구간 계산은 앱의 compiled validator와 테스트에서 검증한다.

이 표본은 SDK 1.38.0의 해당 형식만 검증한다. 특정 운영 exporter 경로, GenAI metric alias/version 또는 일반적인 producer 호환성을 승인하지 않는다. OTLP와 Prometheus 복사본을 자동 합산하지 않는다.

출처: https://opentelemetry.io/docs/specs/otel/metrics/data-model/ 및 https://opentelemetry.io/docs/specs/otlp/ . 합성 자료와 generator는 CC0-1.0. SDK 의존성은 fixture 생성에만 사용하며 앱 런타임에 포함하지 않는다.
