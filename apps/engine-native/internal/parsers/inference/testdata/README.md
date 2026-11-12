# 장치·추론 서빙 의미 연결 계약

CC0 합성 fixture입니다. NVIDIA 드라이버나 vLLM을 실제 실행한 출력은 아닙니다. `nvidia-query.csv`는 명시한 query field와 csv,noheader,nounits 레이아웃만 검증하며 실제 GPU·driver 지원 범위를 증명하지 않습니다. `serving-snapshots.json`은 중립 시계열 계약이며 vLLM 0.8.5 문서의 이름을 명시적 바인딩 예시로 사용합니다. 특정 버전 자동 alias 지원은 별도 fixture와 대상 버전 결정이 필요합니다.

- 원본 metric model은 한 번만 저장하고 `signals`는 series ID를 참조합니다. 미연결 이름도 원본 탐색에 남습니다.
- 장치·MIG·process 수명·worker/rank 매핑은 선언 자료이며 관측 사용량을 workload에 배분하지 않습니다. 메모리·KV cache scope와 client/server 경계를 분리합니다.
- N/A·미지원·권한 부족, 정확한 큰 정수, counter reset/gap, histogram 원본을 보존합니다. Histogram에서 요청·분위수·단계 합계를 만들지 않습니다.
- runtime 이벤트는 별도 JSONL의 명시적 관측입니다. 원본 vendor 로그의 임의 문자열 분류를 검증하지 않습니다. OOM 종류·Xid·수명 및 성공/취소/미완료 token 경계 사례를 포함합니다. 프롬프트·응답 본문은 없습니다.
- `bindings.json`은 canonical MetricHistory 파일에 붙이는 명시적 계약입니다. `metric-model.json`과 `inference-evidence-set.json`은 정규화 기대값입니다.

공식 참고: [NVIDIA query 문서](https://docs.nvidia.com/deploy/nvidia-smi/index.html), [vLLM 0.8.5 metrics](https://docs.vllm.ai/en/v0.8.5/design/v1/metrics.html). 단위·통계 형식은 이름 접미사로 추정하지 않습니다.

## 명시적 range 메타데이터와 사용자 event 입력 (1.1.0)

`r16-*` 파일은 Prometheus range 원본이 gauge/빈 unit인 상태를 보존하고 signal.declared_metadata로만 unit/kind/temporality/monotonic 및 선택적 start_ns를 선언한다. 원본 series/point ID와 hash를 바꾸지 않고 계산 뷰에서만 적용한다. start_ns가 없으면 cumulative rate는 unavailable이다. 원격 TYPE 확인이나 lifetime 진위를 인증하지 않는다.

`r16-bindings-user-input.json`의 event ID는 빈 문자열이다. 입력 정규화에서만 생성하며 잘못 제공한 ID는 거부한다. 저장 reader는 빈 ID를 고치지 않는다. retained_series_ids sidecar/옵션은 선택한 series의 모든 point를 남기고 signals/quality만 함께 제한한다. 원본 byte 예산을 넘는 파일은 정규화 전에 분할해야 한다. 기존 1.0.0 fixture는 그대로 보존한다. 새 payload는 0.2.0, 기존 0.1.0 reader는 유지한다.
