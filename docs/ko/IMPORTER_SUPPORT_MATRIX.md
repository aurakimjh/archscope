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
Windows 전용 loopback proxy의 HTTP/1.x metadata만 다루고 body를 저장하지 않으며,
HTTP/2와 QUIC는 passthrough/unsupported다. Endpoint 귀속은 개별 connection
증거이며 시스템 전체 트래픽 coverage를 의미하지 않는다.

### 내부 ProfileSet 교환 경로 (T-629)

`profile.set-json` / `profile-set`은 제한된 내부 ProfileSet 1.0.0 JSON 교환 경로다.
profile/metric을 명시하여 `profile_evidence` 0.2.0을 만든다. 번들 fixture는 합성 자료이며
공통 코어를 검증할 뿐 실제 producer/버전의 지원 근거가 아니다. 기존 `profile import`는
0.1.x 정규화 의미를 유지한다. `profile-set normalize-legacy`도 그 제한을 명시하며
py-spy/Speedscope/pprof 입력의 수치 정정을 입증하지 않는다(T-634). generic 화면은
T-631이다. 내부 adapter 1.0.1은 32 MiB 이하 모델을 raw 256 KiB 이하 chunk record로
나눠 공통 직렬화 record 1 MiB 정책을 유지한다. normalized/결과/temp ceiling은 함께
적용한다. 실효 상한과 3,000 stack/54,001 node 검증 사례는 DATA_MODEL을 따른다
(생산 부하 보증이 아님). verified 자동 감지는 64 KiB probe 안의 완전한 문서에 한정하고
더 큰 문서는 profile-set 명시 선택과 전체 검증을 요구한다.


### 정밀 baseline/target 비교 (R2 N-01)

CompareEvidenceScalar는 버전이 있는 비교 정책이 일치하는 완료/명시 선택 import의
baseline/target handle을 순서대로 받는다. 서로 다른 원본 hash를 응답에 보존하면서
0이 아닌 차이를 계산한다. ProfileSet 비교는 원본에 독립적인 모델 조건을 확인하고
양쪽 모델/원본/결과 참조를 반환한다. 정책 누락·미지 버전인 과거 scalar 결과는 기존
identity 규칙을 유지한다. 선택·측정·의미/수집·coverage 조건 차이는 거절하고 partial
import에는 다른 원본의 scalar 비교 권한을 부여하지 않는다. 영속 baseline 관리와
entity/time matching은 T-630/T-635에 남기며 legacy profiler diff는 기존 호환 동작을
유지한다.

비교 identity는 원본 검증이 아닌 `declared`임을 응답에 명시한다. scalar 거절 사유는
Wails 오류 `cause.code`로 구분한다(DATA_MODEL R3-01/R3-02). max_lines 뒤에 공백만
남으면 선택된 coverage로 표시하지 않는다. 과거 저장 coverage는 유지하므로 다른
원본 비교에 필요한 과거 coverage 정정은 재import로 한다. 검증된 영속 baseline
결속은 T-635 범위다.

R4 profile 비교는 측정된 quality 수/비율 차이를 허용하고 양쪽 값과 변화 경고를
보존한다. clock/심볼/conversion 범주는 일치해야 하며 process/thread/worker 차이는
T-635의 명시 매핑 전까지 entity_mapping_required로 거절한다. 과거 저장 정책 hash는
보존하며 새로운 scalar 정책으로 자동 승격하지 않는다.

실효 wire 상한에는 합계/문서 한도 외에 디코딩된 UTF-8 문자열/key별 4 MiB도 있다.
초과 문자열은 json_string_bytes 예산 상세로 거절한다. 불투명 capture 옵션은 전체
일치를 유지하며 의미/부수 key 정책은 T-634의 producer별 검증 및 T-635 baseline 연결이
필요하다(DATA_MODEL R5). 현재 whitelist나 새 producer 지원을 주장하지 않는다.

### 선언형 스냅샷 팩 (T-632)

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
