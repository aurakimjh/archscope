# 오프라인 GPU·추론 서빙 근거

T-639는 중립 InferenceEvidence 1.0.0 모델과 `inference_serving` 0.1.0 결과를
추가한다. 공통 AnalysisResult, 기존 MetricHistory reader·케이스 저장·provider·
시각 자료 내보내기를 재사용한다. 원본 메트릭은 `inference_core.metrics`에 한 번만
저장하고 signal은 series ID, 관측 행은 point ID를 참조한다. 연결되지 않은 series도
원본 탐색에 남기며 자동 alias나 가상의 정상 0을 만들지 않는다.

## 구현·검증 범위

| 입력 | 계약과 검증 범위 |
|---|---|
| NVIDIA query CSV | `nvidia-smi.query-csv.v1`, `csv,noheader,nounits`, 아래 고정 field 순서·선언 version·host/boot. 번들 535.104.05 레이아웃 자료는 합성이다. 실제 GPU·도구·driver 실행은 하지 않았으며 해당 driver의 모든 출력을 지원한다는 뜻이 아니다. XML·pmon/dmon·vendor 오류 로그 문법은 등록하지 않았다. |
| 정규 MetricHistory + bindings | 기존 T-638로 원본을 먼저 정규화한다. `metrics` 멤버가 없는 entity/signal/context/link/runtime manifest를 붙인다. 원본 값·identity는 유지한다. DCGM/서빙 metric도 실제 정의를 확인하여 명시적으로 연결할 수 있으며 이름으로 자동 활성화하지 않는다. |
| 정규 InferenceEvidence | 제한 지원 importer `inference.evidence-json`, alias `inference-evidence`, 옵션 `{}`. 엄격한 크기·의미 검증을 적용한다. |
| vLLM 예시 | 합성 queue/running/cache/token/latency metric에 scope·unit·통계 형식·model/config·runtime 관측을 명시한다. 이름은 고정 0.8.5 문서의 예시다. **Q03은 유지**하며 특정 vLLM 버전 parser/자동 alias·실시간 endpoint 검증으로 올리지 않는다. |

NVIDIA query field는 다음 순서로 고정한다.

```text
uuid,pci.bus_id,name,driver_version,utilization.gpu,utilization.memory,memory.used,memory.total,temperature.gpu,power.draw,clocks.sm
```

Capture JSON에 `contract`, `host_id`, `boot_id`, `producer_version`, `query_fields`,
`timestamps_ns`를 선언한다. 시각은 정확한 Unix ns 문자열·null이고 전체가 모르면
빈 배열을 쓴다. 배열을 제공하면 모든 CSV 행 수와 일치해야 한다. 파일 mtime을
시계로 쓰지 않는다. 반복 UUID/PCI 매핑은 일치해야 하며 local GPU index만으로
join하지 않는다. `N/A`·`[Not Supported]`·`[Insufficient Permissions]`를 서로 다른
품질로 보존하고 수치를 만들지 않는다. MiB→bytes·percent→ratio 변환은 명시적이고
정확하다. `utilization.memory`는 메모리 I/O 사용률이며 DCGM DRAM activity와
분리한다. 물리 범위를 벗어나면 raw 값은 유지하고 정규화 값만 null/status로 두며
값 보정이나 장애 심각도를 추론하지 않는다.

## 식별과 의미 연결

Entity는 device·MIG·process 수명·worker·replica를 구분하고 안정적인 선언 host/boot
ID를 요구한다. Device의 opaque UUID는 NVIDIA 고유 형식을 보편 요구하지 않는다.
MIG는 instance ID와 명시적 parent, process는 PID namespace·PID·시작 시각을
요구한다. Container/pod UID·rank를 보존한다. Link는 source ref와 동일 host/boot를
요구하지만 실제 사용량 귀속을 검증했다는 뜻은 아니다. 공유 device 값을 worker
수만큼 복제하거나 MIG 하위 값과 합산하지 않는다. 한 series에는 의미 연결 하나만
허용한다.

Signal은 semantic·entity·context·boundary·population·통계 형식·변환을 고정한다.
Device used/capacity, framework allocated/reserved, 엔진 KV cache는 별도 scope다.
자원·사용률·queue의 순간 관측은 gauge여야 한다. Counter·histogram·summary quantile·
사전 계산 rate를 분리한다. Summary/rate는 원본 계산 설명을 선언해야 하며 그 설명은
원격 계산을 인증하지 않는다. 변환은 명시한 물리 단위 쌍만 받는다.

Context는 engine/version·model/revision·tokenizer·dtype/quantization·TP/PP/DP·
context/batch/sequence 한도·cache/speculation/adapter·replica·workload·warmup 선언을
보존한다. 누락 설정은 지어내지 않는다. 시작·재시작·교체·warmup·recovery 사건을
표시하고 요청을 묵시적으로 제외하거나 비교 호환을 승인하지 않는다.

Runtime 사건은 **명시적 구조화 관측**이며 임의 vendor 로그 문자열 parser가 아니다.
CUDA/framework/container/host OOM, Xid, ECC, device reset을 다른 kind로 보존한다.
코드 하나로 근본 원인이나 물리 고장을 확정하지 않는다. Source hash/position은 선언
근거이며 raw vendor 로그 해석을 입증하지 않는다. 기존 케이스 원본 viewer로 실제
파일을 확인한다. 이번 선언에 대해 새로운 원본 재parse 검증 경로를 주장하지 않는다.
새 event 계약에 prompt/completion/tool 본문 필드는 없다. 기존 MetricHistory의
resource/attribute provenance는 유지하므로 원본 파일도 사전에 비식별 처리해야 한다.

## 시간·수치 계약

원래 값·시각·bucket 경계/count·identity를 정확한 문자열로 유지한다. Counter는
T-638 수명/reset/중복/누락/flag/gap 처리를 재사용하고 이미 계산한 rate gauge를 다시
차분하지 않는다. Histogram의 interval delta/rate는 **관측 count / observations/sec**다.
물리 bucket 단위 변환으로 count를 배율 변환하지 않는다. 원본/구간 bucket은 정확한
메트릭 조회에서 확인한다. 단계 분위수 합산·histogram 분위수 추정·이중 export 합산·
device→request 비용 배분·보간·인과 판정은 수행하지 않는다.

명시적 request는 client/server/gateway 경계·context·population·성공/실패/취소/미완료를
구분한다. TTFT는 관측 first token−request start, 요청 TPOT는 output token 2개 이상이고
first/last 경계가 있을 때 `(last−first)/(tokens−1)`로 계산한다. 누락·역순 경계는
null/status다. 끝나지 않는 소수는 정확한 유리수 ns 문자열로 유지한다. 1-token 요청에
0 TPOT를 만들지 않으며 SSE/network chunk를 token으로 추정하지 않는다. 집계 원본
metric의 정의는 이 요청 공식과 달라도 원래 의미로 남고 요청 분포를 복원하지 않는다.

## Desktop과 내보내기

1. Workspace에서 원본을 선택하고 **GPU / 추론 서빙 근거**를 연다. NVIDIA CSV 또는
   정규 MetricHistory+bindings를 선택해 검사하고 새 모델로 저장한 후
   `inference.evidence-json`, `{}`로 분석한다.
2. Provider 표에서 entity·signal·관측·설정·매핑·runtime 사건·정확한 요청 값을 본다.
   Timeline에는 명시적으로 시각이 있는 runtime 사건만 넣고 metric은 이력으로 유지한다.
3. **장치 / 서빙 범위와 메트릭 상세**에서 entity와 signal 하나를 직접 선택해 원본을
   조회한다. 페이지는 parent result hash·signal 선택·cursor에 묶인다.
4. **Workspace에서 메트릭 이력 열기**는 선택 series의 모든 point를 유지하는 소유
   `metrics_timeseries` 0.2.0 결과를 만든다. T-638 chart·histogram·selection을 사용하고
   predecessor/reset 맥락을 보존한다. 취소 후 늦게 생성된 결과 lease는 해제한다.
5. JSON/CSV/HTML/Report Pack·선택 provider 표 이미지는 기존 consumer를 재사용한다.
   CSV는 기존 요약/가용성 출력이며 상세 정밀 값은 JSON·보존 HTML 표·원본 메트릭
   조회에 남는다. 새 전용 GPU 이미지 renderer나 native Office 수용 검증은 포함하지
   않는다. 표 이미지는 value/unit/status 열을 직접 선택한다.

## CLI

```sh
archscope-engine inference inspect-source --in gpu.csv --format nvidia-smi-csv --capture-file capture.json
archscope-engine inference normalize-source --in gpu.csv --format nvidia-smi-csv --capture-file capture.json --out model.json
archscope-engine inference normalize-source --in metric-model.json --format metric-bindings-json --bindings-file bindings.json --out serving-model.json
archscope-engine evidence import --in serving-model.json --importer inference.evidence-json --options '{}' --out result.json
archscope-engine inference query-metric --in result.json --signal-id signal-output_tokens --limit 25
archscope-engine inference open-metrics --in result.json --signal-id signal-output_tokens --out token-history.json
archscope-engine evidence export --in result.json --export-format report-pack --out report.zip
```

`import-source`는 정규화와 결과 생성을 합친다. 새 파일은 기존 파일/관리 경로를
덮어쓰지 않는다. 원본/확장 파일 64 MiB, 모델 32 MiB, entity 1,024, link 4,096,
signal 2,048, context 1,024, 구조화 event 20,000 한도다. 기존 metric 한도는 100k
point/10k series/500k 전체 bucket이다. Capture/bindings 옵션 128 KiB이며 source
provenance는 별도 64 KiB 한도를 상속한다. 정규화는 2개 동시 호출·5분, 메트릭 페이지는
200행까지다. 결과/케이스는 64 MiB를 넘으면 조치 안내와 함께 거부한다. Point 수만으로
반입을 보장하지 않는다. 실제 GPU/vLLM·Wails/Windows·clipboard/Office·hosted CI는
별도 증거다. 로컬 근거는 `docs/review/evidence/2026-11-12-t639/`에 둔다.

공식 정의: [NVIDIA nvidia-smi](https://docs.nvidia.com/deploy/nvidia-smi/index.html),
[vLLM 0.8.5 metrics](https://docs.vllm.ai/en/v0.8.5/design/v1/metrics.html).
