# ArchScope 제품 확장 TO-DO

Last updated: 2026-05-11

이 문서는 `PRODUCT_EXPANSION_NOTES.md`의 후보 기능을 실제 실행 순서로
정리한 작업 보드다. 우선순위는 로컬 evidence 재분석, 고객 보고서 가치,
현재 Go/Wails 구조와의 결합 비용을 기준으로 잡는다.

## 상태 표기

- `[x]` 완료 또는 현재 작업트리에 구현됨
- `[ ]` 미착수
- `[~]` 설계/조사 진행 중

## P0: 제품 확장 기준 정리

- [x] APM별 export/API 가능성 공식 문서 리서치
  - 산출물: `docs/ko/APM_IMPORT_MATRIX.md`
- [x] 1차 import 포맷 우선순위 결정
  - 1순위: OpenTelemetry OTLP JSON file
  - 2순위: Zipkin v2 JSON
  - 3순위: Elastic APM Elasticsearch documents / `_search` export
  - 4순위: Jaeger Query JSON / QueryService
- [x] SaaS API connector를 파일 import 이후 단계로 분리
  - 대상: Datadog, Dynatrace, New Relic, AppDynamics
  - 이유: 권한, 보존 기간, 샘플링, rate limit, token 보안 정책 필요
- [ ] 제품 확장 메뉴 구조 최종안 확정
  - Workspace / Diagnostics / Service Flow / Architecture / Operations / Security
  - 기존 `MSA Timeline`, `Compare`, `Parser Report` 이름 변경 여부 결정

## P1: Trace Import MVP

- [x] Trace import canonical span model 초안
  - trace/span/parent/service/remote/duration/status/error/attributes/source format
- [x] OTLP JSON file parser
  - 위치: `apps/engine-native/internal/parsers/traceimport`
  - 샘플: `examples/traces/sample-otlp-traces.jsonl`
- [x] Zipkin v2 JSON parser
  - 위치: `apps/engine-native/internal/parsers/traceimport`
  - 샘플: `examples/traces/sample-zipkin-v2.json`
- [x] Trace import analyzer
  - 위치: `apps/engine-native/internal/analyzers/traceimport`
  - 결과 type: `trace_import`
  - 출력: summary, service distribution, kind distribution, top traces,
    slow spans, spans table, traces table, service dependencies, service summary
- [x] CLI 연결
  - 명령: `archscope-engine trace import --in <file> --format auto|otlp-json|zipkin-v2-json`
- [x] Go 테스트
  - `go test ./internal/parsers/traceimport ./internal/analyzers/traceimport ./cmd/archscope-engine`
  - `go test ./...`

## P1: Trace Import 다음 구현

- [ ] Elastic APM `_search` response importer
  - 입력: Elasticsearch search response JSON
  - 핵심 경로: `hits.hits[*]._source`
  - 매핑: `trace.id`, `span.id`, `transaction.id`, `parent.id`,
    `service.name`, `@timestamp`, `event.duration`, `event.outcome`
- [ ] Elastic APM source-only NDJSON importer
  - `_source`만 줄 단위로 export한 파일 허용
  - `_search` wrapper 없이도 동일 canonical model로 변환
- [ ] Trace import critical path 계산
  - trace duration 내 가장 긴 parent-child path
  - single dominant span, external wait, service boundary latency 분리
- [ ] Trace import finding 확장
  - `SLOW_TRACE_P95`
  - `CLOCK_SKEW_SUSPECTED`
  - `UNBALANCED_SERVICE_LATENCY`
  - `HIGH_ERROR_SERVICE_EDGE`
- [ ] Jaeger compatibility importer
  - 1차: HTTP JSON best-effort
  - 2차: QueryService gRPC 또는 OTLP 기반 안정 경로 검토

## P1: Wails UI 연결

- [ ] `trace_import` 결과를 렌더링할 페이지/탭 위치 결정
  - 후보 1: Service Flow > Trace / OTel
  - 후보 2: 기존 OTel analyzer 화면 확장
- [ ] Trace summary 카드
  - total spans, unique traces, services, dependencies, errors, missing parents
- [ ] Service dependency table/chart
  - caller, callee, call count, avg/total duration, error count
- [ ] Trace table
  - trace id, span count, service count, duration, error count, slowest span
- [ ] Span table
  - trace id, span id, parent, service, kind, duration, status, error
- [ ] Trace import findings panel
  - 기존 `AnalyzerFeedback` 또는 findings UI와 재사용 여부 확인

## P1: Evidence Board / Evidence Pack

- [ ] Evidence card model 설계
  - source analyzer, source file, chart/table/finding id, captured data,
    comment, hypothesis, impact, recommendation
- [ ] 분석 화면 공통 "Add to Evidence" action 설계
  - chart, table row, finding, parser diagnostics에서 카드 생성
- [ ] Evidence Board UI skeleton
  - 카드 목록, 필터, 태그, severity, source analyzer
- [ ] HTML report export skeleton
  - Evidence card 기반 정적 HTML
- [ ] ZIP export skeleton
  - report JSON, source metadata, generated HTML/assets 묶음

## P2: Incident Timeline

- [ ] 공통 timeline event model 설계
  - timestamp/range, source analyzer, severity, label, evidence ref
- [ ] Access Log event mapping
  - error burst, slow URL p95, throughput spike
- [ ] GC/JFR event mapping
  - GC pause, allocation/promotion spike, notable JFR event
- [ ] Thread Dump event mapping
  - BLOCKED burst, deadlock, lock contention, JVM signal
- [ ] Trace import event mapping
  - slow trace, error span, missing parent, external service latency

## P2: SLO / Golden Signals

- [ ] Golden signals input inventory
  - latency: Access Log, Trace import, Jennifer MSA
  - traffic: Access Log, Trace import counts
  - errors: Access Log, Trace import, Exception analyzer
  - saturation: GC/JFR/Thread Dump/JVM signals
- [ ] SLI metric model
  - p50/p95/p99, error rate, throughput, saturation indicators
- [ ] SLO target config
  - latency threshold, availability target, time window
- [ ] error budget burn table
  - burn rate, violating windows, affected services/endpoints

## P2: Service Flow / MSA Topology 강화

- [ ] Jennifer MSA topology와 trace import dependency model 통합 가능성 검토
- [ ] service edge 공통 schema
  - caller, callee, calls, avg/max/total latency, errors, network gap
- [ ] C4 dynamic view 또는 sequence-like export 설계
- [ ] unmatched call / missing parent / network gap을 같은 service edge finding으로 정규화

## P3: Architecture Documentation Pack

- [ ] arc42 섹션 초안 생성 입력 정의
  - Context, Runtime View, Deployment View, Quality Requirements, Risks
- [ ] ADR 초안 생성 입력 정의
  - decision, context, alternatives, tradeoffs, consequences, evidence refs
- [ ] Evidence Board와 문서 pack export 연결

## P3: Security / Compliance Evidence

- [ ] 로그 기반 민감정보 노출 가능성 finding 후보 정의
- [ ] OWASP Top 10 관점의 access/error/log 패턴 inventory
- [ ] SBOM/CycloneDX import feasibility 조사
- [ ] vulnerability/license/service impact map 설계

## 보류 항목

- [ ] Datadog SaaS API connector
- [ ] Dynatrace DQL/API connector
- [ ] New Relic NerdGraph connector
- [ ] AppDynamics snapshot API connector
- [ ] Pinpoint import

보류 사유:

- 계정/권한/token 저장 정책 필요
- 보존 기간/샘플링 때문에 evidence 완전성 보장이 어려움
- 로컬 파일 import UX가 먼저 안정화되어야 connector의 입력 contract가 흔들리지 않음
