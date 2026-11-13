# 오프라인 실행·비교 근거

실행 근거는 부하·LLM 벤치마크 export와 제한된 JVM 크래시 텍스트를 정규화합니다.
제한적 `execution.evidence-json` importer(alias execution-evidence)는 기존
AnalysisResult의 execution_evidence 0.1.0/0.2.0 및 execution_core 1.0.0/1.1.0을 사용합니다. 새 generic 모델 가져오기는 기존 1.0.0 core도 0.2.0 projection으로 만들며 저장된 0.1.0 결과는 변경하지 않습니다.
수집 조건·통계 population·crash 필드가 필요해 도메인 모델을 두지만 별도 envelope를
만들거나 기존 profile/runtime 저장 reader를 바꾸지 않습니다. Parser는 사실 추출,
analyzer는 공통 provider 제공, UI/exporter는 소비를 맡습니다.

## 고정한 입력 근거

| format | 근거 | 해석 |
|---|---|---|
| k6-handle-summary-json | 실제 k6 1.3.0, localhost 2 VU/1초 합성 작업의 handleSummary | 선택한 HTTP count/rate/trend·bytes·실측 실행 시간. ms는 정확히 초로 환산합니다. 실패 요청 Rate는 비율이며 requests/second가 아닙니다. 전체 export 집계로서 임의 tag submetric/scenario 본문은 제외하고 개수를 남깁니다. |
| llmperf-summary-json | revision f1d6bed47e4501b0e371082b41601b59ab55269f, RESULTS_VERSION 2023-08-31 | 평탄화한 LLMPerfResults JSON. 실제 upstream 통계/JSON 함수를 합성 수치에 실행했습니다. LLM·GPU·Ray client·tokenizer download·endpoint 실행은 아닙니다. |
| jmeter-jtl-csv | 합성 JMeter 5.6.3 CSV 형식 | timeStamp/elapsed/label/success 필수, epoch ms 명시. Latency/Connect/allThreads/responseCode는 선택입니다. label SHA로 population을 분리하며 URL·메시지·thread name은 제외합니다. 실제 JMeter 실행 수용은 아닙니다. |
| hs-err-text | 실제 Temurin 21.0.10+7 합성 JVM crash의 정제한 발췌 | fatal signal·PID/native TID·JRE version·있을 때 elapsed time·제한된 문제/native/Java frame symbol/module. core/heap engine이나 원인 판정이 아닙니다. |
| execution-evidence-json | 번들 정규 모델·schema | 엄격한 오프라인 전송, importer options는 {}만 허용. 모델 하나는 benchmark 또는 crash 하나이며 여러 capture를 합산하지 않습니다. |

GenAI-Perf·nGrinder·Gatling·다른 producer 버전은 입력/fixture 대기입니다. LLMPerf는
float 합성 수치로 고정한 통계/직렬화 경로를 검증했으며 numpy integer 직렬화 등을 포함한
전체 runner 수용은 아닙니다. 원본 revision/hash·개발 의존 버전은 번들 provenance에
있습니다. 도구는 예제 생성 개발에만 사용하고 제품에서 부하 실행기·benchmark client·
crash JVM을 시작하지 않습니다.

원본 계약: [k6 custom summary](https://grafana.com/docs/k6/latest/results-output/end-of-test/custom-summary/),
[고정 LLMPerf](https://github.com/ray-project/llmperf/tree/f1d6bed47e4501b0e371082b41601b59ab55269f),
[JMeter 저장 필드](https://jmeter.apache.org/usermanual/listeners.html).

## 수집 조건

오프라인 fixture 압축을 풀어 catalog/execution-evidence-k6-options.json,
execution-evidence-llmperf-options.json 또는 execution-evidence-jmeter-options.json을
복사하세요. 원본 지원 UI에는 같은 객체에서 format만 제외해 입력합니다. Crash options는
{"format":"hs-err-text"}입니다. Benchmark는 context 객체가 필요하고, 미지 조건도
표시용으로 가져올 수 있지만 비교는 제한됩니다.

Context는 run/scenario ID, workload·client/target 환경 hash, open/closed/unknown 도착 모델,
도착률·동시성·설정 시간·워밍업 시간, cold/steady/mixed/unknown 단계, client/server/gateway
관측 경계, protocol·streaming 조건을 보존합니다. configured_duration_ns와 k6의 실제
실행 시간은 다릅니다. 실측 실행 시간은 별도 scalar입니다. 선언한 phase로 표본을 자동
필터링하지 않습니다. 워밍업·실패 요청·scenario를 제외하려면 이미 범위를 분리한 원본을
만드세요. 워밍업 시간을 자동 차감하지 않습니다.

LLM은 model/revision/tokenizer, 선언한 입출력 token 분포 hash, 요청 token mean/stddev도
보존합니다. LLMPerf가 export한 요청 분포 수치·동시성을 복사하고 sidecar와 모순되면
거부합니다. 고정 thread loop의 도착 모델은 closed이며 open schedule로 바꿔 해석하지
않습니다. Sidecar hash는 정제한 조건 선언이며 실험 인증이 아닙니다. 알려진 model을
선언했으면 원본 model 이름과 일치해야 합니다.

## 통계·개인정보

각 측정은 의미 이름·통계량·환산/원본 값과 단위·population·group SHA·producer 정책·
원본 위치를 유지합니다. 큰 수·정확한 소수는 Go/JS 사이 문자열이고 JTL 파생 mean은
유리수 문자열일 수 있습니다. count와 rate를 구분하며 누락은 null, 관측 0은 0입니다.
supporting_records는 JTL 통계 산출에 조사한 원본 행 수이며 실패/성공 population의
크기가 아닙니다. 원본 summary가 주지 않은 표본 개수는 unavailable입니다.

1,000행 이하 JTL은 개별 표본을 유지하고 label별 전체 행의 count/error count와 nearest-rank latency
min/max/mean/p50/p90/p95/p99를 만듭니다. 이미 집계한 percentile을 평균내거나 label을
합치지 않고, 첫/마지막 timestamp로 throughput을 추론하지 않습니다. first-byte·connect는
전체 요청 latency와 구분합니다. 중복 header·열 수 불일치는 거부하고 미사용 열은 제외합니다.

LLMPerf 분포는 성공 요청만 사용하고 started/completed/error count는 population을
분리합니다. 전체 실행 output-token throughput과 요청별 throughput 분포는 다릅니다.
ITL은 고정 producer 정의를 보존하며 TPOT로 이름을 바꾸지 않습니다. token 분포 통계는
시간이 아닌 token 수입니다. 오류 메시지·code-frequency 문자열·임의 metadata는 제외합니다.

원본에는 비밀값이 있을 수 있으므로 case copy/index 전에 정규화하고 sidecar 기술 식별자도
정제하세요. 알려진 본문 필드 제외는 임의 민감 문자열 완전 탐지나 과거 archive scrub이
아닙니다. label SHA는 identity fingerprint이며 익명화 보장이 아닙니다. Crash memory/
register dump·명령행·환경·임의 메시지·전체 원문 구역은 제외합니다. END.가 없으면
partial_or_unknown이며 marker 하나로 완전성을 인증하지 않습니다. 문제 frame은 원인이 아닙니다.

## Baseline 호환성과 사용

Workspace의 현재 결과가 순서가 있는 baseline이고 다른 execution 결과가 target입니다.
저장한 case snapshot을 먼저 다시 열어 같은 바이트로 사용할 수 있습니다. 검증 범위는
validated_normalized_snapshots_not_original_reimport, identity는 declared입니다.
Profile 전용 원본 재수입 case baseline API를 재사용하거나 raw export를 독립 인증하지 않습니다.

Workload/scenario/client/target, producer format/version, 도착/설정 시간/동시성/워밍업/
단계/관측 경계/protocol/streaming이 알려져 있고 같아야 비교합니다. LLM model/token
조건도 알려진 동일 값이어야 합니다. Mixed 워밍업·미지 조건·서로 다른 도구 정책·crash
baseline은 사유와 incompatible을 반환하고 수치 차이는 내지 않습니다. Run ID는 달라도
되며 표본 wall-clock을 같은 시각이라고 보지 않습니다. 암묵적 환경 mapping은 없습니다.

단위·population·label SHA·통계 정책을 포함한 같은 지표 정의만 차감합니다. 누락 행·값은
unavailable, target-base·변화율은 정확한 문자열이며 baseline 0의 변화율은 없습니다.
Percentile 차이는 관측이지 분포 합성·통계적 유의성·regression 임계값·인과·자동 최적화
권고가 아닙니다. 페이지 JSON에는 두 결과/원본 digest·context fingerprint·정책·다음 cursor를
보존합니다. 전체 차이 export가 아닌 현재 페이지임을 표시합니다. 기존 summary/table/
timeline·JSON/CSV/HTML/report pack·시각화 표·durable case를 사용합니다.

```sh
archscope-engine execution inspect-source --in summary.json --options-file options.json
archscope-engine execution normalize-source --in summary.json --options-file options.json --out model.json
archscope-engine evidence import --importer execution.evidence-json --in model.json --options '{}' --out result.json
archscope-engine execution import-source --in model.json --out result.json
archscope-engine execution query --in result.json --query '{"kind":"metrics","limit":25}'
archscope-engine execution compare --in baseline-result.json --target target-result.json --query '{"limit":25}' --out comparison-page.json
archscope-engine execution import-source --in hs_err_pid.txt --options '{"format":"hs-err-text"}' --out crash-result.json
```

두 import 명령 중 하나를 새 출력 경로에 사용하세요. CLI는 기존 파일·관리 case 저장소를
덮어쓰지 않습니다. UI inspect의 raw source digest를 normalize 때 다시 확인합니다.
취소·오래된 응답은 현재 결과를 교체하지 않습니다. Query kind는 metrics/samples/distributions/contexts/
sources/crash/crash_frames이며 metric/group filter는 관련 kind에만 적용합니다.

## 한도·검증 경계

원본/해제 입력 64 MiB, 정규 모델 16 MiB, 결과 64 MiB, options 64 KiB입니다. 과거 개별 표본 배열은50k, 새 JTL은 개별표본1000 또는 집계행100만/고유bin10만,
지표8192, JTL label group128, crash frame128, 텍스트 줄1 MiB, crash줄500k 한도입니다.
JSON depth/record 한도도 적용하며 byte 상한에 먼저 도달할 수 있습니다. 페이지100/UI25행,
수집 슬롯2개·5분 예산·취소 확인을 적용합니다. 조용히 절삭하지 말고 원래 실행/입력 범위를 줄이세요. 분할 파일 백분위는 합성할 수 없습니다.
저장 파일 fingerprint·소유권을 확인한 뒤 기존 semantic cache를 재사용하며 2개/2분/
96 MiB JSON charge 한도는 RSS 보장이 아닙니다. 로컬 테스트·build·Windows cross-build는
독립 리뷰·native Windows/WebView·실제 운영 producer 수용을 대체하지 않습니다.
Q01–Q04·Apple/PyTorch/Nsight·추가 adapter는 각 gate를 유지합니다.

JTL 행 수·오류·elapsed 통계는 samples/sample_errors/sample_latency와 all_samples/failed_samples population으로 보존합니다. JMeter 행은 transaction이나 비HTTP sampler일 수 있어 HTTP 요청으로 자동 재명명하지 않습니다. Options 예제의 합성 ID/hash/값은 실제 정제한 수집 조건으로 모두 바꾸세요.

<a id="r19-01-큰-jtl의-정확한-분포-보관"></a>

## 큰 JTL의 정확한 분포 보관

1,000행 이하 JTL은 기존 1.0.0 / 0.1.0 개별 표본 경로를 유지합니다. 큰 JTL은
label별 정확히 정렬된 latency→count 분포와 오류 수를 스트리밍 집계하여
execution_core 1.1.0 / execution_evidence 0.2.0으로 저장합니다. 기존 저장 결과의
1.0.0 / 0.1.0 projection/descriptor는 변경하지 않습니다. count/min/max/mean과
nearest-rank p50/p90/p95/p99를 분포에서 정확하게 계산하며 백분위를 합성하지 않습니다.
큰 결과의 `distributions` 조회는 bucket 한 개가 한 행입니다. 개별 표본의 timeline·
first-byte/connect·thread count·response code는 보관하지 않는다는 제한을 표시합니다.
`aggregated_samples`는 실제 집계 요청 수, `sample_records`는 남긴 개별 레코드 수(큰 결과는 0)입니다.

한도는 JTL 1,000,000행, 전체 (label, latency) 고유 bin 100,000개, label 128개,
원본/확장 입력 64 MiB, 모델 16 MiB, 결과 64 MiB 중 먼저 닿는 값입니다.
행/bin 오류는 관측 수·한도·분할 파일 백분위의 합성 불가를 명시합니다. 한도에 닿으면
원래 실행 범위를 줄여야 하며 임의 규모 집계 보장은 아닙니다. 60,000행/label 3개를
독립 기대 count/error/7개 지연 통계와 대조합니다. 분포·통계 변조 및 버전 바꾸기를
거부합니다. 50,000개 한도는 과거 개별 표본 배열의 읽기 검증에만 남습니다.

HotSpot은 독립 elapsed 줄과 Time 헤더에 붙은 elapsed를 읽고 공백을 포함한 제한된 native 심볼을 보존하며 명령 offset·절대 경로 주석을 제거합니다. frame에 선택적 section=problematic/native/java를 기록해 다른 구역의 같은 심볼을 좌표와 함께 별도 관측으로 유지합니다. section 없는 과거 frame도 읽습니다. core dump를 끈 별도 Temurin21.0.10+7 실제 합성 crash에서 inline elapsed·세 구역만 정제한 표본을 추가했으며 환경·인자·사용자 경로를 저장하지 않습니다.

공유 계약의 epoch-ms-plausibility.1.0.0 정책을 포함합니다. 2000년 이전/2100년 이후 epoch-ms는 단위를 추정하지 않고 경고하며 원래 선언된 ms→ns 시각을 보존합니다. first-byte>elapsed 또는 Connect>first-byte/elapsed는 해당 수치를 null로 하고 timing_warnings와 전체 affected_rows 수에 기록합니다. 큰 입력의 분포 전용 모드에서도 경고 수를 유지하며 정상 elapsed 분포·백분위와 과거 저장 표본은 재해석하지 않습니다.

JMeter 5.6.3 외 버전은 기본 거절합니다. 필요한 경우 source options에 `allow_unverified_version:true`와 실제 `producer_version`을 지정하세요. 미선언 버전은 `unknown`으로 보존하며 `qualification:unverified_version` 경고가 남습니다. 필수 열/값 구조만 확인하며 해당 생산자 버전을 검증했다는 뜻이 아닙니다.

행 한도보다 바이트 한도가 먼저 적용됩니다. 원본과 압축 해제 입력 각각 64 MiB 이하가 필요하며 100만 JTL 행은 별도 최대치입니다. 리뷰에서 기본 열 약 150바이트/행 입력은 약 44만 행이 상한이었고 35만 행/52.8 MiB 통과, 45만 행/67.8 MiB 거절이 관측되었습니다. 실제 용량은 열 텍스트에 따라 다릅니다. 전체 모집단을 유지하면서 불필요한 열을 줄이세요. 시간/모집단을 다시 선택하면 근거 범위가 달라집니다. 분할 파일의 백분위를 합산/평균하여 전체 실행 백분위를 복원할 수 없습니다. 바이트 오류에는 실제 획득 바이트 또는 명시적 하한과 조치가 포함됩니다.

검증된 버전이 기본값입니다. 다른 버전의 명시적 구조 검증 선택, 실제/미상 버전 보존과 제한은 [미검증 버전 정책](INPUT_ASSISTANCE.md#미검증-생산자-버전의-명시적-선택)을 참조하세요.
