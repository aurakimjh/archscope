# 오프라인 CPU/GPU 실행 구간

T-644는 기존 trace_import AnalysisResult에 payload0.4.0 /
gpu_execution_core1.0.0과 `trace.gpu-execution-json`(alias gpu-execution)을 추가합니다.
기존 trace0.2.0·LLM trace0.3.0 저장 reader는 유지합니다. Parser·구간 분석·SVG exporter·
React 표현을 분리하며 장치 telemetry는 기존 MetricHistory에 남깁니다.

## 실제 검증 범위

| 형식 | 실제 입력 | 의미 |
|---|---|---|
| pytorch-trace-json | PyTorch2.15.0 / Python3.13.13, CPU/Gloo 2-rank의 matrix multiply/all_reduce/relu 합성 실행 | CPU operator, 이름을 hash 처리한 annotation, 명시적 Gloo 통신 관측, rank/world/backend, flow |
| nsight-sqlite | NVIDIA training 보고서 고정 commit b17c40eef23929d08ec42e66667b29ae336ea8ac의 LFS SHA 확인 후 공식 Nsight2024.2.3.38로 내보낸 schema3.11.1 | CUDA runtime API·kernel·memcpy·닫힌 NVTX 구간, packed process ID, 명시한 장치 매핑 |
| nsight-summary-csv | 위 export에 대한 공식 cuda_gpu_kern_sum reporter의 실제 CSV | 보고된 count/duration/share만 보존. 구간·stream overlap·launch 링크·상세 타임라인을 만들지 않음 |

로컬 2-rank CPU 작업은 실제 분산 LLM·CUDA/NCCL·운영 부하·호스트 간 동기화 검증이
아닙니다. NVIDIA 공개 보고서는 원격 GPU의 실제 자료이며 이 Mac에서 GPU를 실행한
증거가 아닙니다. PyTorch CUDA 형태는 구조 테스트만 있으며 GPU pid/device 필드를
CPU process로 바꾸어 추정하지 않습니다. 다른 버전은 실제 fixture가 필요합니다.

개발 전용 network-none 컨테이너에서 공식 내보내기를 실행했습니다. 32KiB SQLite
fixture는 실제 table DDL/rowid, [80,80.5)ms에 시작하는 실제 행, 필요한 이름과
export 버전만 보존합니다. 환경·세션·명령줄·GPU UUID 테이블은 제외했습니다.
41,431,040-byte 전체 export도 명시한 창으로 파싱하여 창 이전에 시작한 API까지
확인했습니다. 원본/export hash와 정제 범위는 번들 producer-provenance에 있습니다.
집계 CSV는 부분집합이 아닌 원래 전체 실행에 대한 값입니다.

## 수동 수집과 원본 범위

공식 도구는 별도로 실행하며 가져오기는 도구를 실행하지 않습니다.

```sh
nsys export --type sqlite --output trace.sqlite trace.nsys-rep
nsys stats --report cuda_gpu_kern_sum --format csv --output summary trace.sqlite
archscope-engine gpu-trace inspect-source --in trace.sqlite --options-file capture.json
archscope-engine gpu-trace normalize-source --in trace.sqlite --options-file capture.json --out model.json
archscope-engine evidence import --in model.json --importer trace.gpu-execution-json --options '{}' --out result.json
```

검증된 SQLite용 capture.json 예시입니다. 태그는 정제한 실제 수집 맥락으로 바꿉니다.

```json
{"format":"nsight-sqlite","producer_version":"2024.2.3.38","capture_id":"local-capture","host_id":"local-host-tag","boot_id":"unique-boot-tag","process_namespace":"nsys-packed","run_id":"local-run","clock_id":"local-export-clock","device_map":{"0":"local-gpu"},"from_ns":"80000000","to_ns":"80500000"}
```

PyTorch는 format=pytorch-trace-json / producer_version=2.15.0과 같은 범위 필드,
선택적 시간창을 사용합니다. rank/world/backend는 이름 추정 없이 distributedInfo에서
읽습니다. `torch.profiler.profile(activities=[ProfilerActivity.CPU])` 및
`prof.export_chrome_trace(...)`로 CPU 자료를 내보낼 수 있습니다. 원본 host/path/임의
args를 정규 모델에 복사하지 않고 임의 annotation 이름은 hash 처리합니다. 기술 operator/
kernel 이름과 사용자가 선언한 식별자도 민감할 수 있어 공유 전에 정제해야 합니다.
tensor 값·모델 가중치·GPU 메모리 dump는 요구하거나 보존하지 않습니다.

Workspace의 **CPU/GPU 실행** 원본 지원에서 수집 JSON을 입력 → 원본 확인 → 새 모델
저장 → 분석 순서입니다. 확인한 원본 SHA가 정규화 때 일치해야 하며 기존 파일은
덮어쓰지 않습니다. 정규 모델의 generic import 옵션은 {}입니다. CLI의 직접 결과 생성은
`gpu-trace import-source`, 여러 원본 결합은 `gpu-trace merge-models --inputs a.json,b.json`을
사용합니다. 같은 원본 identity의 중복은 거부하며 clock을 암묵적으로 합치지 않습니다.

## 정확한 구간과 연결 규칙

- PyTorch ts/dur의 microsecond를 정확한 integer ns로 바꾸고 displayTimeUnit은 표시만
  위한 값으로 취급합니다. sub-ns/overflow는 거부합니다. Nsight start/end는 ns입니다.
  2^53을 넘는 값도 측정값을 JavaScript number로 바꾸지 않습니다. 화면 좌표만 표시 비율입니다.
- 원본 hash/producer/schema/capture/host/boot/process namespace/run/rank/clock,
  device_map 및 track의 process/thread/device/context/stream, 출처 좌표를 보존합니다.
  Nsight globalPid/globalTid는 packed ID이며 process correlation에만 하위 thread 성분을
  제거합니다. 인증한 OS PID나 process 수명으로 표시하지 않습니다.
- CUDA API↔GPU는 같은 원본·packed process·correlation ID의 유일한 API가 근거입니다.
  다중 후보는 ambiguous, 누락은 unresolved입니다. launch 하나에 GPU 여러 개가 연결될 수
  있습니다. kernel 이름으로 NCCL/rank/요청/prefill/decode/workload를 확정하지 않습니다.
- flow는 원본 안의 명시한 ID를 보존합니다. id2 global은 해당 원본 안의 process를 넘을 수
  있고 legacy/local은 process 범위입니다. 누락·반복·역전 endpoint를 표시하며 이름으로
  CPU/kernel이나 인과 critical path를 만들지 않습니다.
- CPU self는 같은 track의 관측된 직접 자식 구간 합집합을 inclusive에서 뺀 값입니다.
  시간 포함 관계이며 원본 call graph라는 뜻은 아닙니다. crossing/equal 양수 구간이 있으면
  해당 track의 self는 unavailable입니다. CPU+GPU는 합하지 않으며 annotation/통신은 분리합니다.
- 원본/장치별 kernel 시간 합계, stream 사이의 합집합, 관측 kernel event extent는 별개입니다.
  extent는 전체 수집 window나 hardware 사용률의 분모가 아니며 memcpy도 분리합니다.
- 시간 선택은 [from,to)에 겹치는 전체 interval을 보존하며 0-duration은 point입니다.
  자르기·보간·재비례는 하지 않습니다. 시간 조회는 원본 clock 하나가 필요합니다.
  clock_group/clock_offset_ns(부호 있는 정확한 정수)/clock_uncertainty_ns는 모두 있어야
  하고 감사용으로 보존합니다. 현재 버전은 명시한 mapping이 있어도 여러 원본의 타임라인
  정렬을 수행하지 않습니다. NTP 보정·rank straggler·critical path를 추정하지 않습니다.

## 조회·내보내기·저장 케이스

```sh
archscope-engine gpu-trace query --in result.json --query '{"kind":"events","source_id":"copied-source-id","limit":25}'
archscope-engine gpu-trace query --in result.json --query '{"kind":"busy","limit":25}'
archscope-engine gpu-trace export-svg --in result.json --query '{"kind":"events","source_id":"copied-source-id","limit":25}' --out selected.svg
```

events/tracks/sources/busy/relations/summaries를 조회합니다. source/rank filter와 결과
SHA·선택 조건에 묶인 cursor를 사용합니다. 화면 미리보기는 현재 페이지 행만 표시합니다.
SVG는 선택 전체를 최대2000 event/64 track까지 내보내고 넘으면 거부합니다. 정확한 시각,
query, 선택한 원본의 provenance와 결과 SHA를 담고 label을 escape하며 외부 자원을 쓰지
않습니다. 집계 전용 파일에는 상세 타임라인이 없습니다. JSON/CSV/HTML/report-pack,
generic 정확한 표와 durable case snapshot을 재사용합니다. 저장 snapshot은 원본 재반입
검증의 증거가 아닙니다.

원본/확장 입력64MiB, 정규 JSON 모델32MiB, event50k/track4096/source16/summary4096,
결과64MiB 한도 중 먼저 닿는 값이 적용됩니다. 원본/확장64MiB는 시간 창 선택 전에
검사합니다. 창은 보존할 event/track만 줄이고 큰 원본 자체를 허용하지 않습니다.
조용한 first-N 절삭은 없습니다. 2-slot/5분·취소를 적용합니다. SQLite는 private
불변 읽기 전용 복사본, query_only/trusted_schema 제한, 고정 테이블 query와 행 예산을
사용합니다. 필수 metadata/name 테이블은 실제 테이블이어야 하며 선택된 실제 테이블의
generated column·미검증 schema를 거부합니다. 선택 테이블의 누락/view를 건너뛰는
R21-02는 후속 큐이므로 빈 kernel 결과를 GPU 활동이 없었다는 증거로 해석하지 않습니다. 일반 SQLite나
.nsys-rep binary, 임의 SQL, 설치 도구를 실행하는 입력 경로가 아닙니다.

검증 근거는 실제 producer fixture, 독립 정수 구간 계산, source/cursor/소유권/변조 테스트,
전체 race, 공통 소비자·배포 자원·오프라인 CLI입니다. 네이티브 Windows/WebView,
PyTorch CUDA/NCCL 실제 실행·다른 export 버전·hardware 튜닝은 별도 수용 조건입니다.

공식 참조: [PyTorch profiler](https://docs.pytorch.org/docs/2.15/profiler.html),
[분산 패키지](https://docs.pytorch.org/docs/2.15/distributed.html),
[Nsight export/schema](https://docs.nvidia.com/nsight-systems/AnalysisGuide/index.html#sqlite-schema-reference),
[NVIDIA training 보고서](https://github.com/NVIDIA/nsight-training/tree/b17c40eef23929d08ec42e66667b29ae336ea8ac/cuda/2021_gtc/x-ac-03-v1/task1/task/nsys/precollected_reports).

Nsight는 runtime API/kernel/memcpy/닫힌 NVTX 테이블만 선택합니다. OS runtime·driver API·memset·marker·hardware counter 테이블은 이번 범위 밖이며 memcpy도 byte/방향 기반 대역폭이 아닌 구간을 제공합니다.


## 큰 원본을 줄이는 경로 (R21-01)

64MiB를 넘는 검증 버전의 standalone Nsight SQLite는 먼저 Python 표준 sqlite3를
사용하는 별도 수동 helper로 줄인 뒤 가져옵니다.

```sh
python3 scripts/subset-nsight-sqlite.py --in full.sqlite --from-ns 80000000 --to-ns 80500000 --out selected.sqlite
archscope-engine gpu-trace inspect-source --in selected.sqlite --options-file capture.json
```

helper는 원본2GiB/300초, SQLite cache4MiB, 교차하는 전체 구간50k·출력64MiB를
제한합니다. 원본 rowid, 지원 정수 열, 버전 metadata3개, 필요한 기술 이름만 보존합니다.
입력 DDL·환경·session·command line·NVTX text는 실행/복사하지 않습니다. 선택 테이블의
view·virtual/generated column, sidecar, 미지원 버전, 변경된 원본, 기존 출력/receipt를
거절합니다. 원본/subset hash·선택 창·테이블별 행 수 receipt를 함께 출력합니다.
기술 이름은 공유 전에 별도로 확인하세요. 가져온 source SHA는 subset 바이트의 것이며
큰 원본과의 관계는 receipt로 보존합니다. 앱이 helper를 실행하거나 불변64MiB 제한을
우회하지 않습니다. 2GiB가 넘으면 producer에서 더 짧게 수집하고, nsys export에 창
옵션이 있다고 가정하지 않습니다. Python·도구 설치는 별도 수동 선행 조건입니다.

PyTorch는 profiler schedule/activities로 짧은 active 구간을 수집하고 불필요한
stack/record_shapes를 줄일 수 있습니다. 예를 들어 schedule(wait=1,warmup=1,active=2,
repeat=1), prof.step(), on_trace_ready로 active 구간마다 export합니다. 실제 파일 크기를
확인해야 하며 schedule이 바이트 상한을 보장하지는 않습니다. 64MiB 내에서는 무시되는
metadata/python_function을50k 지원 event 예산에 포함하지 않습니다. 창을 먼저 적용하고
보존할 지원 event가 넘을 때 at_least로 첫 초과를 명시합니다. 나머지 R21 P3의 테이블
존재 고지·실제 flow 입력 등은 별도 후속 큐에 유지합니다.

분리 배포 bundle에도 같은 수동 helper를 `resources/tools/subset-nsight-sqlite.py`로 포함합니다. Python에서 명시적으로 실행하며 앱이 자동 실행하지 않습니다.
