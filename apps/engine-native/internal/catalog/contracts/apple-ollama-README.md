# Apple Silicon / Ollama 수동 캡처 계약

의미 기반 ID: `diagnostics.apple-ollama-capture.v1`.

`capture.json`은 macOS 27.2.1 (26B131), Mac17,6 / Apple M5 Max,
Ollama 0.40.16에서 기존 gemma:2b Q4_0 / context 512로 8토큰을 생성한
직후 실제로 얻은 3개 스냅샷이다. host/boot 태그만 예제 값으로 치환했다.
프롬프트·응답 본문·호스트명·사용자명·경로·환경변수는 포함하지 않는다.
모델 digest는 공개 모델 식별자다. 운영 부하·성능 비교·보안 검증을 뜻하지 않는다.

수동 읽기 순서: `hw.memsize`, `vm_stat`의 페이지 크기×각 항목 페이지 수,
Darwin `sysctlbyname(vm.swapusage)`의 xsw_usage 정확한 byte 필드,
`kern.memorystatus_vm_pressure_level`, `pmset -g therm`, 로컬 `/api/ps`.
모든 API/명령은 사용자가 별도로 실행하며 가져오기 프로그램은 실행하지 않는다.
API ps는 선택한 모델의 name/digest/size/size_vram/context_length와
quantization_level만 허용한다. 임의 로그·본문을 이 파일에 붙이지 않는다.

시작/종료 Unix ns는 순차 읽기의 관측 범위다. 각 값이 같은 순간에 측정된 것은
아니며 적분·평균 구간도 아니다. host/boot/device_local_id는 선언한 로컬 식별자다.
공유 메모리 항목은 겹치므로 합하거나 모델 추정치를 빼서 GPU free를 만들지 않는다.
`size_vram`은 모델별 producer 추정치이며 dedicated VRAM·GPU utilization·전력이 아니다.
pressure는 private sysctl 정수 코드로만 보존하며 enum/백분율을 추정하지 않는다.
thermal 미기록은 정상 0이나 온도 0이 아니다. 누락 값은 null+명시된 이유다.
GPU utilization 및 power의 숫자 입력은 이 계약에서 지원하지 않는다.

한도: 4 MiB, 스냅샷 1~256개, 스냅샷당 모델 0~64개, canonical uint64 문자열,
증가하고 겹치지 않는 관측 범위. 모델 추정 byte는 JSON 정수/정수 문자열을 받되
float64로 바꾸지 않는다. 버전 위조는 금지한다. 다른 Ollama 버전·MLX·MPS·
llama.cpp·powermetrics 자료는 별도 실제 입력 검증 후 계약을 확장한다.
OS 버전은 필수 provenance이며 현재 실제 검증은 위 한 환경으로 제한된다.

`capture.schema.json`은 닫힌 구조 계약이다. 시간 순서·실측 지원 범위·중복 모델·
정수 한도·swap used<=total 등 의미 조건도 검증해야 한다.
`metric-model.json`은 원본 SHA-256, 수집 조건, 출처 좌표와 정확한 값을 보존한다.

참고: https://docs.ollama.com/api/ps 및
https://github.com/ollama/ollama/blob/v0.40.16/server/routes.go,
https://github.com/ollama/ollama/blob/v0.40.16/server/sched.go,
https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_memorystatus_notify.c,
https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/sysctl.h.
