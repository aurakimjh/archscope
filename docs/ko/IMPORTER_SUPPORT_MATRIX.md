# Importer Support Matrix

이 문서는 `v0.3.5` 이후 아직 릴리스되지 않은 browser profile, Lighthouse,
HAR 추가분을 포함해 현재 지원하는 local evidence importer를 정리한다.

| Family | Result type | CLI | 대표 입력 |
|---|---|---|---|
| Access 및 edge log | `access_log` | `access-log analyze` | nginx/common/combined, Apache/OHS, Tomcat/Jetty, HAProxy, Envoy/Istio, cloud load balancer, API Gateway |
| Server log | `server_log` | `server-log analyze` | Tomcat, Jetty, JBoss/WildFly, WebLogic, WebSphere, GlassFish/Payara, nginx/Apache error log |
| OpenTelemetry log | `otel_logs` | `otel analyze` | JSONL/NDJSON, OTLP Logs JSON |
| Metrics snapshot | `metrics_snapshot` | `metrics import` | Prometheus/OpenMetrics text |
| Observability export | `observability_evidence` | `observability import` | Loki query JSON, Tempo trace JSON, Grafana dashboard JSON |
| Trace import | `trace_import` | `trace import` | OTLP JSON/JSONL trace, Zipkin v2 JSON, Elastic APM `_search` JSON, Elastic source NDJSON, Jaeger QueryService/local trace JSON, guarded SkyWalking GraphQL `queryTrace.spans` JSON |
| Database evidence | `database_slow_query` | `database-log analyze` | PostgreSQL log/csvlog, MySQL slow log, MongoDB profiler JSON, Redis slowlog, SQL Server xevent JSON, PostgreSQL/MySQL EXPLAIN JSON |
| Broker evidence | `broker_log` | `broker-log analyze` | Kafka, RabbitMQ log/diagnostics JSON, Pulsar, NATS, ActiveMQ |
| Platform evidence | `kubernetes_evidence` | `platform import` | Kubernetes event/pod JSON, kubelet/runtime log, CloudTrail, GCP audit, Azure Activity |
| Runtime profile | `profile_evidence` | `profile import` | pprof `.pb.gz`, async-profiler collapsed/HTML, py-spy, rbspy, speedscope/dotnet-trace, perf collapsed, JFR JSON stack, StackProf, PHP profiler JSON, Xdebug, Swift/async stack, Pyroscope/Parca snapshot, Chrome Performance trace `.json`/`.json.gz`(`chrome-trace-json`), V8 `.cpuprofile` — Node `--cpu-prof`·CDP `Profiler.stop` 포함(`v8-cpuprofile`) |
| Browser audit | `browser_audit_evidence` | `browser import` | Lighthouse report JSON(`lighthouse-json`). 원본 report score 보존, Core Web Vitals/audit/resource projection, URL 리댁션, bounded table 적용 |
| HTTP capture evidence | `http_capture` | `http-capture analyze` | 방언 판별이 있는 HAR 1.2 (Chrome, Firefox, Safari, Charles, Fiddler, Proxyman, Insomnia, generic); 가져오기 시점 리댁션; entry 상한 |
| Evidence stitching | `stitched_evidence` | `stitch analyze` | access, trace, runtime profile, database, broker, platform importer가 만든 기존 `AnalysisResult` JSON. exact key와 timestamp/service-alias stitching 지원 |
| API/event contract | `api_contract_analysis` | `api-contract analyze` | OpenAPI JSON/YAML + access-log result JSON, AsyncAPI JSON/YAML + broker result JSON |
| Architecture documentation | `architecture_docs` | `architecture-docs draft` | service, contract, runtime, deployment, finding, risk evidence를 담은 기존 `AnalysisResult` JSON |

모든 importer는 parser diagnostics를 `metadata.diagnostics` 아래 보존하고,
Evidence Board와 report-pack capture에 적합한 bounded table을 생성한다.

Windows 실시간 HTTP 캡처는 별도의 file importer가 아니다. 종료된 로컬 세션을
같은 bounded `http_capture` 분석 계약으로 전달하지만 릴리스 범위는 더 좁다.
Windows 전용 loopback proxy로 수집하며 endpoint 귀속은 개별 connection 근거이다. 지원하는 body 단계와 프로토콜 제한은 [HTTP 캡처](SYSTEM_HTTP_CAPTURE.md)를 따른다.

<a id="내부-profileset-교환-경로-t-629"></a>

### 내부 ProfileSet 교환 경로

`profile.set-json` / `profile-set`은 제한된 내부 ProfileSet 1.0.0 JSON 교환 경로다.
profile/metric을 명시하여 `profile_evidence` 0.2.0을 만든다. 번들 fixture는 합성 자료이며
공통 코어를 검증할 뿐 실제 producer/버전의 지원 근거가 아니다. 기존 `profile import`는
0.1.x 정규화 의미를 유지한다. `profile-set normalize-legacy`도 그 제한을 명시하며
py-spy/Speedscope/pprof 입력의 수치 정정을 입증하지 않는다. 별도 원본 가져오기 경로는 원본 의미를 보존하며 결과 provider는 공통 화면을 제공한다. 내부 adapter 1.0.1은 32 MiB 이하 모델을 raw 256 KiB 이하 chunk record로
나눠 공통 직렬화 record 1 MiB 정책을 유지한다. normalized/결과/temp ceiling은 함께
적용한다. 실효 상한과 3,000 stack/54,001 node 검증 사례는 DATA_MODEL을 따른다
(생산 부하 보증이 아님). verified 자동 감지는 64 KiB probe 안의 완전한 문서에 한정하고
더 큰 문서는 profile-set 명시 선택과 전체 검증을 요구한다.


<a id="정밀-baselinetarget-비교-r2-n-01"></a>

### 정밀 baseline/target 비교

CompareEvidenceScalar는 버전이 있는 비교 정책이 일치하는 완료/명시 선택 import의
baseline/target handle을 순서대로 받는다. 서로 다른 원본 hash를 응답에 보존하면서
0이 아닌 차이를 계산한다. ProfileSet 비교는 원본에 독립적인 모델 조건을 확인하고
양쪽 모델/원본/결과 참조를 반환한다. 정책 누락·미지 버전인 과거 scalar 결과는 기존
identity 규칙을 유지한다. 선택·측정·의미/수집·coverage 조건 차이는 거절하고 partial
import에는 다른 원본의 scalar 비교 권한을 부여하지 않는다. 영속 baseline 관리와
entity/time matching은 케이스·정규 조회 서비스가 담당하며 legacy profiler diff는 기존 호환 동작을
유지한다.

비교 identity는 원본 검증이 아닌 `declared`임을 응답에 명시한다. scalar 거절 사유는
Wails 오류 `cause.code`로 구분한다([데이터 모델](DATA_MODEL.md)). max_lines 뒤에 공백만
남으면 선택된 coverage로 표시하지 않는다. 과거 저장 coverage는 유지하므로 다른
원본 비교에 필요한 과거 coverage 정정은 재import로 한다. 검증된 영속 baseline
결속은 정규 조회 범위다.

Profile 비교는 측정된 quality 수/비율 차이를 허용하고 양쪽 값과 변화 경고를
보존한다. clock/심볼/conversion 범주는 일치해야 하며 process/thread/worker 차이는
정규 조회 서비스에서 명시 매핑을 제공해야 하며 미매핑 요청은 entity_mapping_required로 거절한다. 과거 저장 정책 hash는
보존하며 새로운 scalar 정책으로 자동 승격하지 않는다.

실효 wire 상한에는 합계/문서 한도 외에 디코딩된 UTF-8 문자열/key별 4 MiB도 있다.
초과 문자열은 json_string_bytes 예산 상세로 거절한다. 불투명 capture 옵션은 전체
일치를 유지하며 의미/부수 key 정책은 원본 가져오기의 producer별 검증 및 정규 조회 baseline 연결이
필요하다([데이터 모델](DATA_MODEL.md)). 현재 whitelist나 새 producer 지원을 주장하지 않는다.

<a id="선언형-스냅샷-팩-t-632"></a>

### 선언형 스냅샷 팩

서명된 기본 팩은 `data.synthetic-metrics` / `synthetic-metrics`를 설치한다.
명시적 `evidence --pack-store` 또는 데스크톱 카탈로그에서 metrics_snapshot 0.1.1을
생성한다. synthetic.metrics 1.0.0 JSONL 합성 fixture 적합성만 검증했다.
리터럴 조건/별칭/정확한 배율은 고정 해석기 기능이며 제품/시계열/임의 스크립트
지원을 추가하지 않는다. [오프라인 팩](OFFLINE_PACKS.md)을 참조한다.

### 입력 안내와 JFR 변환

입력 탐색은 현재 일반 카탈로그와 기존 개별 진입점을 구분하며 새로운 생성 도구
지원을 주장하지 않는다. 바이너리 JFR은 절대 경로로 명시한 설치 JDK를 제한적으로
실행하고 provenance를 기록한다. JSON 분석에는 변환기가 필요하지 않으며 오프라인
세션에는 미리 변환한 JSON을 사용한다. [입력 안내](INPUT_ASSISTANCE.md)를 참조한다.

<a id="정확한-원본-프로파일-경로-t-634"></a>

### 정확한 원본 프로파일 경로

`profile inspect-source`, `normalize-source`, `import-source`는 정확한 가중치와
단위를 보존하고 profile/metric을 명시적으로 선택합니다. Desktop Workspace는
원본을 새 `profile.source-set-json` / `profile-source-set` 모델로 정규화해
`profile_evidence` 0.4.0을 생성합니다. `profile import`는 기존 의미를 유지합니다.
Evented Speedscope는 이벤트 개수 대신 exclusive 경과 구간과 unattributed gap을
사용하며 sampled 시간 가중치를 CPU 시간으로 추정하지 않습니다. pprof는 모든
column, 음수, label과 period를 보존하고 period를 이중 적용하지 않습니다.

| 범위 | 검증 | 수집 옵션 정책 |
|---|---|---|
| py-spy 0.4.1 raw / sampled Speedscope seconds | 실제 합성 subprocess 수집 fixture와 고정 digest | 해당 metric/형식 binding의 컴파일된 1.0.0 정책 |
| Go runtime/pprof go1.26.10 heap | 실제 합성 4-column 수집 fixture와 고정 digest | alloc/inuse objects·space의 컴파일된 1.0.0 정책 |
| 그 밖에 구조가 지원되는 sampled/evented Speedscope / 지원 단위 pprof | 합성 형식 회귀, producer/platform/option 전면 검증 없음 | unknown/no-policy 옵션은 strict |

기존 모델과 일반 scalar 비교는 엄격한 전체 capture 비교를 유지하며 영속 baseline
연결은 정규 조회입니다. fixture로 도구·플랫폼 전체 지원, dump text 지원이나 원본 정체성
인증을 주장하지 않습니다. Windows 네이티브/UI 수용과 독립 리뷰는 별도입니다.
한도·CLI 예시·projection 제약·정책 key·근거 범위는
[정확한 원본 프로파일 가져오기](PROFILE_SOURCE_IMPORT.md)를 참고하세요.

<a id="t-635-공통-조회검증-baseline과-r11-의미-버전"></a>

### 공통 조회와 검증된 baseline

[케이스 근거 조회](CASE_EVIDENCE_QUERIES.md)는 제한된 cursor 기반 profile 보기·근거,
색인 범위를 공개하는 원본/summary 검색, 원본 좌표, 선택한 불변 결과와 baseline
재분석·매핑 gate를 정의합니다. 현재 원본 정규화는 normalizer 1.1.0 / 결과 0.4.0을 생성하고
원래 type/unit을 producer 정책 없이 metric identity에 포함합니다. Speedscope none은
일반 count 가중치이며 표본 수를 단정하지 않습니다. 기존 0.3.0은 바이트·hash·읽기·저장·
조회·내보내기를 유지하되 비교에는 새 원본 재분석이 필요합니다. 새 정책은 선언/
file-exporter basis와 경고를 공개합니다. 영속 baseline 검증은 매번 양쪽 원본을 다시
분석하고 identity_basis=declared를 유지합니다. 일반 scalar 비교는 계속 strict이며
새 canonical/baseline 화면이 검증 정책을 소비합니다. 시계 정렬·미검증 producer 지원을
추정하지 않습니다. Windows 네이티브/UI 및 독립 리뷰는 별도입니다.

<a id="t-638-메트릭-이력"></a>

### 메트릭 이력

[정확한 메트릭 이력](METRIC_HISTORY.md)은 모델/normalizer 1.0.0,
`metrics.history-json` transport와 `metrics_timeseries` payload 0.2.0(기존 저장 0.1.0 reader 유지)을 정의합니다.
중립 snapshot sequence·명시적 query 출처의 Prometheus range matrix·OTLP JSON에서
정확한 값, resource/scope/schema/속성 식별자와 temporality를 유지합니다.
Delta/cumulative/reset/gap/overlap과 explicit/exponential bucket은 별도 의미입니다.
공통 envelope로 상세/timeline·저장 조회·export를 연결하고 기존 metrics_snapshot 의미는
유지합니다. SDK Python 1.38.0 합성 표본 범위만 검증하며 Q04/GenAI alias와 네이티브
수용 검증은 별도입니다. 자동 합산/quantile 계산은 없습니다. baseline 캐시는 새 query
재분석 결과만 프로세스 내 제한 보관하고 페이지마다 원본/결과 hash와 case binding을
재검사합니다. 기존의 매번 전체 재분석 설명은 이 범위에서 대체합니다.

<a id="t-637-파일-기반-시스템-근거"></a>

## 파일 기반 시스템 근거

`system.evidence-json` / `system_evidence` 0.2.0은 제한 지원입니다. 실제 perf 6.1.190의
합성 data→sample text 출력, 제한 scheduler text 문법, 명시적 event JSONL과 bpftrace 0.22
histogram JSON shape를 검증했습니다. Native scheduler/BPF 수집, 다른 layout/버전과 임의
script는 별도입니다. Sample count·period·scheduler duration·집계 bucket을 구분하고
perf binary decoder/live attach는 추가하지 않습니다. [흐름·조건·한도](SYSTEM_EVIDENCE.md).


<a id="t-639-장치서빙-근거"></a>

## 장치·서빙 근거

제한 지원 `inference.evidence-json`은 InferenceEvidence 1.0.0을 공통
`inference_serving` 0.1.0으로 처리한다. 원본 MetricHistory를 한 번만 저장하고
signal은 series/entity/context/unit/통계 형식/모집단을 명시적으로 연결한다.
누락 값은 unavailable이다. NVIDIA CSV는 고정 query 계약을 요구하고 fixture는
합성이며 Q03의 특정 vLLM 버전 alias gate는 유지한다. Parser·의미 analyzer·
generic/export 책임은 분리한다. 한도·정확한 요청 공식·원본 메트릭 상세와 미실행
native 수용 범위는 [오프라인 장치·서빙 계약](INFERENCE_EVIDENCE.md)을 참고한다.

SystemEvidence 1.0.0 / system_evidence 0.1.0 저장 reader는 유지하며 새 1.1.0은 frame/stack을 중복 제거하고 0.2.0 결과와 과거 snapshot을 분리한다.

<a id="t-640-구현-경계-2026-11-12"></a>

## LLM trace 근거

제한 importer `trace.llm-json`은 LLMTrace 1.0.0을 기존 trace_import 0.3.0 / llm_core 1.0.0으로 분석한다. 기존 trace 0.2.0 reader는 유지한다. Parent/link·자식 합집합 경과 시간·명시적 논리/시도 accounting·token 경계·lifecycle 집단을 구분하고 본문 제외 정규화를 케이스 복사/색인보다 먼저 적용한다. 정확한 페이지 query/UI와 기존 provider/export를 재사용한다. Q01 제품 JSON adapter는 실제 입력 검증 전까지 보류한다. [LLM trace 근거](LLM_TRACE_EVIDENCE.md) 참조. 추론 근거는 untyped range의 명시적 메타데이터와 입력 event ID 생성을 추가하며 1.0/1.1 모델·0.1/0.2 결과 reader를 유지한다. [추론 서빙 근거](INFERENCE_EVIDENCE.md) 참조.


<a id="t-641-런타임-근거"></a>

## 런타임 근거

런타임 근거는 AnalysisResult 안의 별도 정규 모델을 사용합니다. 고정 producer 형식·identity/window 확인·live allocation 프로파일 재사용·GC 합집합·실행계획 loop당 의미·저장 reader 경계는 [RUNTIME_EVIDENCE.md](RUNTIME_EVIDENCE.md)를 참고하세요.


<a id="t-642-실행-근거"></a>

## 실행 근거

제한된 benchmark/crash 문맥은 공통 AnalysisResult의 execution_core 1.0.0 / execution_evidence 0.1.0을 사용합니다. 고정 k6/LLMPerf·합성 JTL/정제한 실제 HotSpot 입력을 정규화한 뒤 기존 provider/case/export를 사용합니다. 비교는 선언 조건 snapshot 검증이며 profile 원본 재수입 baseline과 구분합니다. [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md)를 참고하세요.

Apple Silicon 근거: `apple-ollama-capture-json`은 Apple Silicon/Ollama 0.40.16 수동 스냅샷을 기존 metrics.history-json으로 정규화합니다. 공유 메모리 byte·모델별 상주 추정치·관측 불가를 분리하며 NVIDIA identity/전용 VRAM/자동 수집/GPU 사용률 추정은 없습니다. 실제 fixture 범위와 한도: [APPLE_SILICON_EVIDENCE.md](APPLE_SILICON_EVIDENCE.md).

큰 JTL 지원은 큰 JTL의 정확한 분포용 execution_core 1.1.0 / execution_evidence 0.2.0을 추가하며 저장된 1.0.0/0.1.0은 유지합니다. [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md).

GPU 실행 근거: trace.gpu-execution-json / trace_import0.4.0 / gpu_execution_core1.0.0은 정확한 CPU/GPU/rank 구간·명시적 연결·kernel 합계/합집합·집계 전용 근거를 분리합니다. CLI/Wails/Workspace·제한된 선택 SVG와 기존 소비자를 사용하며 저장된 trace/LLM reader는 유지합니다. [GPU_EXECUTION_EVIDENCE.md](GPU_EXECUTION_EVIDENCE.md).


DB/OS 스냅샷의 실제 DB·OS 첫 검증 배치: [수동 DB·소켓 스냅샷](SYSTEM_DB_SNAPSHOTS.md). 기존 MetricHistory·정확한 epoch/gauge·공통 소비자를 재사용하며 다른 어댑터는 실제 입력 검증 전까지 보류합니다.

추가 제한 profile adapter: CPython cProfile 3.14.4 `cprofile-flat-json`(실제 합성 workload, 평면 self-time 전용). [원본 프로파일](PROFILE_SOURCE_IMPORT.md#cpython-cprofile-평면-소유자-export)을 참조하세요.

## Sentry transaction export

명시 형식 `sentry-transaction-json`은 Sentry Python SDK2.69.2 transaction JSON을 제한 검증합니다. `capture-sentry-synthetic.py --out new-transaction.json`은 메모리 SDK transport로 합성 transaction과 span 두 개를 실제 생성하며 Sentry로 전송하지 않습니다. `trace import --format sentry-transaction-json`으로 기존 trace_import0.2.0·그래프/상세·케이스·export를 재사용합니다. 새 결과 envelope는 없습니다.

파서는 trace/span/parent ID·정확한 원본 시각·매핑한 operation 분류·status만 보존합니다. 이름·description·SQL/URL·사용자/request/breadcrumb·임의 속성은 제외합니다. Service는 unknown이며 parent/child 시계 불일치를 잘라내거나 인과적 coverage로 승격하지 않습니다. RFC3339와 숫자 epoch seconds는 정수 ns로 정확히 표현되어야 합니다. 미지원 버전·중복 ID·역전 시각·sub-ns는 전체 원본을 거절합니다. 파일/해제64MiB·depth128·50k span 한도를 적용합니다. 실제 fixture는 합성 SDK 설정 하나의 근거이며 cloud 조회·다른 SDK·운영 자동 계측 검증이 아닙니다.
