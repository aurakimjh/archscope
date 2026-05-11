# 로드맵

Last updated: 2026-05-11

이 문서는 현재 Go/Wails 기반 ArchScope 라인의 통합 제품 로드맵입니다. 기존
phase 로드맵, 제품 확장 메모, APM import matrix, AI 보조 해석 설계를 한곳에
모아 제품 방향과 실행 우선순위를 볼 수 있게 정리합니다.

제품 방향은 이 문서를 기준으로 봅니다. 현재 실행 상태와 검증 기록은 루트의
`work_status.md`를 기준으로 봅니다. 이 로드맵은 기존 단일 언어 제품 확장
메모, 제품 확장 TODO, APM import matrix 문서를 대체합니다.

## 로드맵 입력 문서

- `work_status.md` - 현재 실행 상태와 활성 TO-DO ID.
- `docs/en/AI_INTERPRETATION.md`, `docs/ko/AI_INTERPRETATION.md` -
  evidence-bound AI 해석 정책과 runtime guardrail.
- `docs/en/REPORT_EXPORT_DESIGN.md`, `docs/ko/REPORT_EXPORT_DESIGN.md` -
  report export와 before/after 설계 맥락.

## 제품 방향

ArchScope는 로컬 진단 분석기 모음에서 아키텍처, 운영, 고객 보고서를 위한
Evidence Studio로 확장한다. 핵심 가치는 local-first evidence 재분석이다.
사용자는 실제 시스템에서 내려받은 파일이나 export를 불러와 deterministic
finding을 확인하고, 근거를 수집하고, 고객 데이터 외부 전송 없이 보고서를
만들 수 있어야 한다.

원칙:

- SaaS별 API connector보다 로컬 파일과 표준/open 포맷을 먼저 지원한다.
- Parser, analyzer, exporter, UI 책임을 분리한다.
- 공통 `AnalysisResult` contract를 deterministic source of truth로 유지한다.
- AI interpretation은 optional, evidence-bound, deterministic analyzer output과
  분리된 결과로 취급한다.
- canonical roadmap에 들어온 제품 문서는 한글과 영문을 함께 유지한다.

## 현재 기준선

- 활성 제품: `apps/engine-native` 아래 Go/Wails desktop app.
- 활성 UI: `apps/engine-native/cmd/archscope-app/frontend` 아래 Wails v3 React
  frontend.
- 활성 엔진: `apps/engine-native/internal` 아래 Go parser, analyzer, exporter,
  profiler, AI interpretation module.
- Release baseline: stable `v0.3.1`.
- Archive: Python/FastAPI/browser 소스는 `archive/` 아래 historical reference로만
  유지한다.

## 완료된 기반

### 기반 및 차트

- Repository, desktop UI, parser, exporter, JSON result 기반.
- Access log와 collapsed profiler MVP.
- Parser diagnostics, encoding fallback, type-specific `AnalysisResult`
  contract.
- Chart Studio, chart template, PNG/SVG/CSV export, report label language
  toggle, report-ready chart 개선.

### JVM 및 Runtime 진단

- GC log, Java thread dump, Java exception, JFR parser/analyzer 경로.
- Access log, GC, profiler, thread, JFR, OTel, runtime evidence를 잇는
  timeline correlation 설계.
- Java, Go, Python, Node.js, .NET source를 위한 multi-language stack/dump
  parsing.
- Carrier pinning, SMR/zombie thread, lock contention, deadlock finding을
  포함한 thread dump 강화.

### Go/Wails 통합

- 활성 제품 라인을 Go/Wails desktop 구현으로 통합.
- Python/browser 구현은 archive하고 신규 제품 기능 대상에서 제외.
- Wails service binding을 활성 engine/UI boundary로 확정.
- Access log, OTel JSONL, GC log, JFR JSON, thread dump, Jennifer profile,
  profiler HTML/SVG 경로에 large-file guardrail 추가.

### Profiler, Access Log, GC, MSA 개선

- JFR-first profiler workflow, differential flame, heatmap selection,
  per-thread isolation, pprof export, tree view, native-memory leak finding,
  recent-files workflow.
- URL 통계, static/API 분류, percentile timeline, throughput, status
  distribution, `SLOW_URL_P95`, `ERROR_BURST_DETECTED`를 포함한 access log
  개편.
- JVM info card, worker/CPU mismatch warning, toggleable heap series, pause
  overlay, rectangle zoom, point decimation을 포함한 GC log 심화.
- Jennifer MSA service-call network-time summary와 topology placement 추가.
  내부 single-digit ms 호출과 gateway/external double-digit ms 호출이 시각적으로
  분리된다.

### Trace Import MVP

- 외부 trace import용 canonical trace/span model.
- OTLP JSON-file parser와 Zipkin v2 JSON parser.
- Summary, services, traces, spans, dependencies, service summary를 포함한
  `trace_import` analyzer result.
- CLI command:
  `archscope-engine trace import --in <file> --format auto|otlp-json|zipkin-v2-json`.
- `examples/traces` 아래 sample trace fixture.

### 증거 기반 AI 보조 해석

- `apps/engine-native/internal/aiinterpretation` 아래 Go 구현.
- Evidence registry, evidence selector, prompt builder, Ollama client, AI
  finding validator.
- `InterpretationResult`를 `AnalysisResult`와 분리하는 contract.
- Local-only 기본 provider 정책, prompt-injection defense, validation,
  low-confidence filtering, evaluation requirement.

## 단기 로드맵: 0.3.x 안정화

이 항목들은 `work_status.md`의 현재 실행 큐와 계속 맞춰야 한다.

1. `trace_import`를 Wails UI에 연결한다.
   - Summary card, service dependency table/chart, trace table, span table,
     findings panel을 추가한다.
   - UI 위치를 `Service Flow > Trace / OTel`로 둘지, 기존 OTel analyzer 화면
     확장으로 둘지 결정한다.

2. Elastic APM file import를 구현한다.
   - Elasticsearch `_search` response JSON을 지원한다.
   - `hits.hits[*]._source` 기반 source-only NDJSON export를 지원한다.
   - Elastic transaction과 span을 canonical trace model로 정규화한다.

3. Trace critical path와 richer finding을 추가한다.
   - 가장 긴 parent-child span chain 기준 critical path.
   - External wait와 service-boundary latency 분리.
   - Finding: `SLOW_TRACE_P95`, `SLOW_SPAN_DOMINATES_TRACE`,
     `ERROR_SPAN_IN_TRACE`, `MISSING_PARENT_SPAN`,
     `CLOCK_SKEW_SUSPECTED`, `UNBALANCED_SERVICE_LATENCY`,
     `HIGH_ERROR_SERVICE_EDGE`.

4. Evidence Board skeleton을 만든다.
   - Analyzer finding, chart selection, table row, parser diagnostic, source
     metadata, comment, hypothesis, impact, recommendation을 담는 reusable
     evidence card model을 정의한다.
   - 분석 화면 공통 "Add to Evidence" action을 추가한다.
   - Evidence card 기반 HTML/ZIP export를 시작한다.

5. Release hardening을 마무리한다.
   - Windows host 또는 VM에서 직접 GUI launch smoke test를 수행한다.
   - Signing/notarization 작업을 계속한다.
   - 필요한 frontend bundle splitting을 진행한다.
   - 실제 로그가 현재 bounded-memory envelope를 넘으면 GC event streaming을
     더 깊게 검토한다.

## 중기 로드맵: Evidence Studio

### 장애 타임라인

- Timestamp/range, source analyzer, severity, label, evidence reference를 갖는
  공통 timeline event model을 정의한다.
- Access-log error burst, slow URL p95, throughput spike, GC/JFR event,
  thread-dump contention/deadlock signal, exception, profiler hotspot,
  trace-import event를 하나의 시간축에 배치한다.
- 장애 중 무엇이 어떤 순서로 일어났는지를 설명하는 보고서 축으로 사용한다.

### SLO 및 Golden Signals

- Access log, trace import, Jennifer MSA, exception, GC, JFR, thread dump, JVM
  signal을 대상으로 golden signals inventory를 만든다.
- Latency, traffic, errors, saturation에 대한 SLI metric을 정의한다.
- SLO target config, violating-window detection, error-budget burn table,
  affected service/endpoint breakdown을 추가한다.

### Service Flow 및 MSA Topology

- Jennifer MSA topology와 trace-import dependency model을 통합한다.
- Caller, callee, call count, average/max/total latency, error count,
  network gap을 담는 common service-edge schema를 정의한다.
- Unmatched call, missing parent, network gap을 service-edge finding으로
  정규화한다.
- Service-flow evidence를 C4 dynamic view 또는 sequence-like export로 내보낸다.

### 보고서 및 Evidence Pack

- Evidence Board 내용을 기반으로 report-ready HTML, ZIP, 이후 PPTX/PDF export를
  만든다.
- Source metadata, analyzer option, captured evidence, deterministic finding,
  optional AI interpretation provenance를 보존한다.
- 결론 뒤에 raw evidence를 숨기지 않는 고객-facing summary를 지원한다.

### AI 보조 해석 제품화

- UI에 AI interpretation provenance를 표시한다.
- AI finding을 deterministic finding과 시각적으로 분리한다.
- Golden diagnostics, evidence-reference integrity, quote-to-source matching,
  low-confidence filtering, hallucination review를 이용한 evaluation gate를
  추가한다.
- 생성된 모든 claim이 유효한 evidence reference를 가질 때만 Evidence Board와
  report generation에 연결한다.

## 장기 로드맵: 아키텍처 및 운영 확장

### API 및 Event Contract 분석

- OpenAPI specification을 import하고 access-log evidence와 대조한다.
- 문서에 없는 API, 사용되지 않는 API, 느린 API, 오류율 높은 API, contract
  drift를 탐지한다.
- AsyncAPI definition을 import하고, evidence가 있을 때 Kafka, RabbitMQ,
  WebSocket 등 producer/consumer 흐름을 분석한다.

### 아키텍처 문서 패키지

- arc42의 Context, Runtime View, Deployment View, Quality Requirements, Risks
  초안 입력을 생성한다.
- Decision, context, alternatives, tradeoffs, consequences, evidence reference를
  포함한 ADR 초안을 생성한다.
- C4 diagram, service-flow evidence, incident timeline, Evidence Board card를
  architecture review 산출물로 연결한다.

### Security/Compliance Evidence

- 로그 내 민감정보 노출 가능성을 탐지한다.
- OWASP Top 10 관점의 access/error/log pattern inventory를 만든다.
- SBOM/CycloneDX import feasibility를 조사한다.
- Vulnerability, license, affected-service impact map을 설계한다.
- 관련 source evidence가 있을 때만 threat model, security logging,
  redaction evidence를 추가한다.

### Before/After Multi-Signal 비교

- 비교 기능을 profiler output 밖으로 확장한다.
- 튜닝 또는 배포 전후 access-log latency/error/traffic, GC pause/heap,
  JFR signal, MSA external call latency, thread blocking, profiler hotspot을
  비교한다.

### 제품 Navigation

- Workflow 중심 navigation으로 이동한다:
  Workspace, Diagnostics, Service Flow, Architecture, Operations, Security,
  Settings.
- `MSA Timeline`은 `MSA / Service Flow`, `Compare`는 `Before / After`로 바꾸는
  방안을 검토한다.
- Parser report는 각 analyzer 가까이에 두되, 중요한 parser evidence는
  Evidence Board로 승격할 수 있게 한다.

## 외부 APM Import 로드맵

### File-First 우선순위

1. OpenTelemetry OTLP JSON file - trace import MVP 완료.
2. Zipkin v2 JSON - trace import MVP 완료.
3. Elastic APM Elasticsearch `_search` response와 source-only NDJSON - 다음
   구현 대상.
4. Jaeger compatibility import - P1/P2. 가능하면 UI-internal HTTP JSON보다
   stable QueryService 또는 OTLP 경로를 우선한다.
5. Apache SkyWalking GraphQL response import - schema/version validation 이후
   P1/P2 feasibility 항목.

### SaaS 및 제품별 Connector

아래 항목은 file import contract와 token/security policy가 안정화된 뒤로 둔다.

- New Relic NerdGraph trace detail 및 Historical Data Export JSON.
- Datadog Spans API와 Continuous Profiler/profile export feasibility.
- Dynatrace distributed trace CSV/table download 및 DQL/API export.
- Splunk/Cisco AppDynamics transaction snapshot import.
- Pinpoint는 안정적인 공식 export/API 경로가 확인된 뒤에만 검토한다.

보류 사유:

- 제품마다 authentication/authorization model이 다르다.
- Retention window, indexed-span sampling, rate limit 때문에 evidence 완전성이
  떨어질 수 있다.
- SaaS API token은 명시적인 local storage, redaction, compliance policy가
  필요하다.
- Connector 입력을 public contract로 만들기 전에 local file-import UX가 먼저
  안정화되어야 한다.

## 문서 로드맵

- 이 로드맵은 한글과 영문을 mirror로 유지한다.
- 제품 확장, 외부 APM import, Evidence Studio 계획은 별도 단일 언어 메모 대신
  이 로드맵에 유지한다.
- 상세 설계 문서는 `docs/en`과 `docs/ko` 아래에 둔다.
- `work_status.md`는 전체 제품 로드맵이 아니라 활성 실행 상태에 집중한다.

## 보류 또는 범위 밖

- Heap dump / `.hprof` 분석은 명시적으로 범위 밖이다.
- Direct SaaS connector는 local file import, evidence contract, token policy가
  안정화될 때까지 보류한다.
- Archive된 Python/FastAPI/browser 소스에는 신규 제품 기능을 추가하지 않는다.
