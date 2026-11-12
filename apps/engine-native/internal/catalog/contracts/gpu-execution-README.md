# CPU/GPU·rank 실행 구간 계약

의미 기반 ID: `diagnostics.gpu-execution.v1`.

PyTorch 2.15.0 / Python3.13.13의 실제 CPU profiler와 Gloo 2-rank 실행에서
rank별 Chrome trace를 확보했다. CPU matrix multiply/all_reduce/relu 합성 작업이다.
호스트명·경로·trace 이름과 임의 args는 허용 목록에서 제거했으며 원본 decimal token을
보존했다. 실제 CUDA PyTorch·분산 LLM/NCCL의 검증으로 확장하지 않는다.

NVIDIA 공개 training 보고서의 고정 commit/LFS SHA-256을 확인하고 공식 Nsight
Systems 2024.2.3.38 도구로 network-none 컨테이너에서 SQLite(schema3.11.1)로
내보냈다. `nsight-sanitized.sqlite`는 실제 [80,80.5) ms 구간에서 시작하는 행과
필요한 StringIds/버전 메타데이터만 뽑고 실제 table DDL/rowid를 보존한 32 KiB
fixture이다. 전체 실행을 대표하거나 이 Mac에서 NVIDIA GPU를 실행했다는 뜻은 아니다.
원본/내보내기 hash, 출처 URL, 정제 범위는 producer-provenance.json에 있다.
`nsight-summary.csv`는 공식 cuda_gpu_kern_sum reporter의 실제 집계 값이다.
이 집계 자료에서 stream/시간 구간을 역으로 만들면 안 된다.

정규화는 source/host/boot/process namespace/run/rank와 개별 source clock을 보존한다.
Nsight globalPid/globalTid는 packed ID 그대로이며 이름으로 rank나 host를 연결하지 않는다.
GPU ordinal은 선언한 device_map으로만 장치에 연결한다. CUDA API↔GPU는 같은 원본·
packed process·correlation ID의 유일한 근거만 사용한다. Flow는 원본 범위에서 보존한다.
CPU 연산/self, GPU kernel 합계, stream 간 kernel 구간 합집합, 관측 event extent를
구별하고 CPU+GPU를 더해 wall time이나 hardware 사용률로 만들지 않는다.
독립 rank clock, 불명확한 correlation/겹친 CPU 구간은 임의 연결·self 비용을 만들지 않는다.

입력은 최대64 MiB, 모델32 MiB, 결과64 MiB, event50k/track4096/source16이다.
Nsight 큰 파일은 명시적 시간창으로 전체 interval을 선택하고 자르거나 재비례하지 않는다.
Parser는 collector나 nsys를 실행하지 않는다. 이미 집계된 CSV는 summary-only다.
기술 이름은 여전히 민감할 수 있으므로 공유 전 정제해야 한다.

공식 근거: https://docs.pytorch.org/docs/2.15/profiler.html 및
https://docs.nvidia.com/nsight-systems/AnalysisGuide/index.html#sqlite-schema-reference.
