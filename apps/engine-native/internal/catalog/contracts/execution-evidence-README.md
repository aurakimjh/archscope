# 실행 결과·크래시 텍스트 근거 v1

공식 자산 ID: `diagnostics.execution-evidence.v1`.
부하·LLM 벤치마크와 JVM 크래시 문맥을 오프라인 진단 도구에서 재사용하기 위한
민감정보가 제거된 계약 예제입니다. 실행기·수집 agent·운영 진단 정책이 아닙니다.

- k6 1.3.0: 실제 localhost 합성 HTTP/2 VU/1초 실행의 handleSummary JSON.
  HTTP 원본·본문 대신 수치만 정규화하며 측정 duration과 선언한 configured duration은 다릅니다.
- LLMPerf: `f1d6bed47e4501b0e371082b41601b59ab55269f`의 실제 metrics_summary와
  LLMPerfResults/flatten_dict를 합성 수치에 실행했습니다. 모델·GPU·API·Ray 실행은 아닙니다.
  성공 요청 3개와 오류 요청 1개로 token/TTFT/ITL·집계/요청별 throughput·client error를 구분합니다.
  통계 입력은 float입니다. 고정 JSON formatter의 numpy int64 직렬화 제약을 우회하며
  실제 benchmark runner의 전체 수용을 주장하지 않습니다. 원본 함수 SHA와 pandas 버전은
  producer-provenance.json에 있습니다. upstream 코드/의존성은 이 자산에 포함하지 않습니다.
- JMeter 5.6.3 JTL: 합성 epoch-ms CSV 구조 예제입니다. 실제 JMeter 실행 수용이 아닙니다.
  오류 본문·URL 표지는 정규 결과에서 제외되어야 하며 label은 SHA로 population을 분리합니다.
- HotSpot Temurin 21.0.10+7: 별도 합성 JVM에서 Unsafe.putAddress(0)를 실행하고 core 생성을
  끈 상태의 실제 hs_err_pid 발췌입니다. pc/PID/TID는 치환하고 명령행·환경의 나머지는 제거했습니다.
  END.를 보존하지 않은 부분 자료이며 root-cause·실제 운영 장애를 증명하지 않습니다.

`*-options.json`은 선언한 수집 조건입니다. workload/client/target/token 분포 hash는
합성 조건을 가리키며 인증·실험 통제 증명이 아닙니다. 누락 조건은 자동 비교를 막아야 합니다.
단위·population·통계 정책을 유지하고 percentile을 평균·합산하지 않습니다. 서로 다른 도구의
동일 이름 지표도 정책·오류 population·denominator가 다르면 자동으로 비교하지 않습니다.

`generate_fixture.py OUTPUT TOOLS JDK_HOME`은 개발 전용입니다. 격리한 임시 도구 디렉토리에
고정 k6 바이너리, upstream revision의 llmperf-token_benchmark_ray.py,
llmperf-utils.py, llmperf-common_metrics.py, llmperf-revision.txt가 필요합니다.
별도 Python 환경에서 pandas를 사용합니다. 새로운 표본은 시점·실측값이 달라지므로 원본 함수·
도구 버전을 확인하고 정규 모델과 manifest hash를 함께 갱신해야 합니다. 임의 운영 JVM이나
대상 endpoint를 인자로 받지 않습니다. 원본 hs_err는 임시 디렉토리에서 삭제합니다.

참조 원본:
- https://grafana.com/docs/k6/latest/results-output/end-of-test/custom-summary/
- https://github.com/grafana/k6/releases/tag/v1.3.0
- https://github.com/ray-project/llmperf/tree/f1d6bed47e4501b0e371082b41601b59ab55269f
- https://jmeter.apache.org/usermanual/listeners.html

JTL 행은 transaction/비HTTP sampler일 수도 있으므로 samples/sample_errors/sample_latency와 all_samples/failed_samples로 유지합니다. HTTP 요청 수로 자동 재명명하지 않습니다.

정확한 큰 JTL 분포: `jmeter-1001.csv`는 최소 압축 경계를 넘는 합성 원본이며
`jmeter-1001-set.json`은 1.1.0 모델이다. 1,000행 이하의 기존 모델은 유지한다.
큰 파일은 label별 latency→count와 오류 수를 보존하고 개별 timestamp/first-byte/
connect/응답 코드/thread/timeline은 보관하지 않는다. 전체 count/nearest-rank 통계는
분포에서 정확히 다시 계산한다. 1,000,000행·100,000개 전체 고유 latency bin·128개
label·16 MiB 모델/64 MiB 입력 한도가 먼저 적용된다. 한도를 넘으면 원래 실행 범위를
줄여야 하며 나눈 파일의 백분위는 합성할 수 없다. 기존 1.0.0 schema/model은 읽기 호환된다.

`hs-err-21.0.10-full-sections.txt`는 core dump를 끈 별도 JVM의 실제 합성 crash에서 header·inline elapsed·problematic/native/java frame만 허용 목록으로 남긴 정제 발췌입니다. wallclock/PID를 치환하고 환경·인자·사용자 경로·원시 메모리를 제외했습니다. `producer-provenance.json`에 원본·발췌 SHA를 기록합니다. 구역이 다른 동일 심볼은 별도 관측으로 보존합니다.

JTL timing 정책은 `jtl-timing-policy.json`에 고정합니다. 선언된 epoch ms가 2000~2100 범위 밖이면 단위 추정 없이 경고하고 원래 ms→ns 값을 보존합니다. first-byte>elapsed 또는 Connect>first-byte/elapsed는 해당 모순 수치를 null로 하고 sample.timing_warnings와 전체 affected_rows에 기록합니다. 큰 입력의 분포 압축 뒤에도 경고 수를 유지하며 elapsed 통계는 바꾸지 않습니다.

JMeter 5.6.3 외 버전은 기본 거절한다. `allow_unverified_version:true`를 명시하면 필수 열/값 구조만 확인한다. 실제 `producer_version`을 유지하며 미선언은 `unknown`, source `qualification:unverified_version`과 제한사항을 기록한다. 생산자 버전 검증을 통과했다는 뜻이 아니다.

원본/해제 후 64 MiB 한도가 100만 JTL 행보다 먼저 적용된다. 기본 열 구성 리뷰 예는 약 44만 행이며 열 길이에 따라 달라진다. 필요 없는 열을 제외해도 모집단은 유지하고, 분할 백분위는 합산/평균하지 않는다.
