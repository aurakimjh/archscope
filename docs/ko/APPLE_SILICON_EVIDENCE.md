# Apple Silicon 오프라인 메모리 근거

T-643은 메트릭 이력 원본 지원 및 `metrics history-inspect/history-normalize/history-import`에
`apple-ollama-capture-json`을 추가합니다. auto는 명시된 `apple-ollama-capture.v1` 계약을
인식합니다. 기존 MetricHistory 1.0.0 / `metrics_timeseries` 0.2.0 AnalysisResult의
조회·차트·JSON/CSV/HTML·report pack·정확한 case snapshot을 재사용합니다.
NVIDIA UUID/MIG/Xid/PCI/dedicated VRAM은 요구하지 않으며 가져오기는 수집기를 실행하지 않습니다.

## 첫 실제 검증 대상

Q02의 첫 **제한된 fixture 대상**으로 Ollama 0.40.16, macOS 27.2.1 (26B131),
Mac17,6 / Apple M5 Max를 선정했습니다. 사용자의 운영 엔진이라고 가정하지 않습니다.
기존 gemma:2b Q4_0 / context 512로 합성 8토큰을 생성한 직후 실제 스냅샷 3개를
확보했습니다. 테스트로 올린 모델은 해제했으며 다운로드는 없었습니다.
다른 macOS는 선언한 버전만 보존하며 실제 검증을 주장하지 않습니다. 다른 Ollama 버전,
MLX·PyTorch MPS·llama.cpp/Metal은 실제 입력으로 별도 매핑 검증이 필요합니다.

## 수동 수집과 가져오기

의도한 모델을 이미 로드한 Apple Silicon에서 사용자가 직접 실행합니다.

```sh
python3 scripts/capture-apple-ollama.py --out apple-capture.json --host-id local-host-tag --boot-id unique-os-boot-tag --model gemma:2b --samples 3
archscope-engine metrics history-inspect --in apple-capture.json
archscope-engine metrics history-normalize --in apple-capture.json --out apple-model.json
archscope-engine metrics history-import --in apple-capture.json --out apple-result.json
```

도우미는 별도 수동/개발 유틸리티이며 고정 loopback `/api/version`, `/api/ps`,
sysctl, vm_stat, `pmset -g therm`만 읽습니다. 모델 로드·추론·다운로드·sudo·
powermetrics·memory_pressure를 실행하지 않습니다. host/boot 태그는 비밀정보를 제거한
선언 값으로 호스트/OS boot마다 구분해야 합니다. 하드웨어 serial이나 인증된 identity가 아닙니다.
선택한 정확한 모델의 허용 메타데이터만 보존합니다. 공유 전 기술 식별자를 확인해야 하며
임의 텍스트의 자동 비식별화를 보장하지 않습니다. 기존 출력은 덮어쓰지 않습니다.
macOS/Python/Ollama는 수동 수집에만 필요하고 오프라인 가져오기에는 필요하지 않습니다.
화면에서는 메트릭 이력 → `apple-ollama-capture-json` 또는 auto → 원본 확인 →
정확한 시계열 선택 → 새 모델 저장 → 분석 순서입니다. 저장한 모델은 raw capture가 아니며
`metrics.history-json` importer로 엽니다.

## 값의 의미와 관측 불가

- `hw.memsize`는 시스템 전체 공유 메모리 용량입니다. VM 항목은 관측한 페이지 크기×개수이며
  항목이 겹치므로 합산하거나 GPU free/메모리 비율/OOM 임계값을 만들지 않습니다.
- Swap은 Darwin `sysctlbyname(vm.swapusage)`의 정확한 byte 필드를 사용합니다.
  반올림된 sysctl 표시 suffix를 역산하지 않으며 used가 total보다 크면 거부합니다.
- `size`/`size_vram`은 모델별 producer가 보고한 **상주 메모리 추정치**입니다.
  전용 VRAM pool, GPU 사용률·효율·워크로드 귀속 근거가 아닙니다. 모델 간 합산하지 않습니다.
- Pressure는 private sysctl의 정수 코드를 enum/비율 추정 없이 보존합니다. pmset의 thermal
  warning은 온도가 아닙니다. 미기록 메시지는 정상 0이 아니라 unavailable입니다.
- 이 계약은 GPU utilization/전력 수치를 지원하지 않습니다. null에는 `not_recorded`,
  `not_collected`, `permission_required`, `unsupported`를 붙이며 `no_recorded_value`로
  정규화합니다. 0으로 만들지 않습니다. 별도 producer 검증 전 숫자 GPU/전력 필드는 거부합니다.
- capture start/end Unix ns는 순차 읽기의 범위와 시차이며 동시 측정이나 집계 구간이 아닙니다.
  시스템 압력·swap은 모델 추정치 옆의 문맥이며 인과 진단을 내리지 않습니다.

host/boot/local-device·하드웨어·OS/build·수집기/엔진 버전·공유 메모리 모델·관측 scope와
모델 digest/quantization/context length를 identity에 보존합니다. 값/시간은 2^53 초과도
정확한 uint64 문자열입니다. 원본 SHA-256(gzip은 압축 원본)·좌표·수집 provenance를 보존합니다.
관측 불가 이유가 달라지면 별도 시계열이며 보간·mtime 시간 추정은 없습니다.

한도는 4 MiB, 증가하고 겹치지 않는 관측 범위 1~256개, 범위당 모델 0~64개입니다.
중복 모델 digest/미지 필드/중복 JSON 키·정수 overflow·이유 없는 null·identity/producer
덮어쓰기는 거부합니다. 기존 source lease·5분/2-slot·취소·원본 SHA pin·덮어쓰기 방지·
백엔드 소유 정확한 값 조회 경로를 유지합니다.

참고: [Ollama ps](https://docs.ollama.com/api/ps),
[고정 API 매핑](https://github.com/ollama/ollama/blob/v0.40.16/server/routes.go),
[고정 메모리 보고](https://github.com/ollama/ollama/blob/v0.40.16/server/sched.go),
[Darwin sysctl 구조](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/sysctl.h),
[private pressure 조회](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_memorystatus_notify.c).
