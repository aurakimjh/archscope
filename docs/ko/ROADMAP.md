# 로드맵

Last updated: 2026-05-16

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
- Release baseline: stable `v0.3.2`.
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
- JFR analyzer contract 정리. JDK `jfr` CLI는 `.jfr`를 `jfr print --json`으로
  변환하는 boundary로 두고, ArchScope는 보고서용 이벤트 요약,
  async-profiler 스택 evidence, UX hint를 만든다.

### Go/Wails 통합

- 활성 제품 라인을 Go/Wails desktop 구현으로 통합.
- Python/browser 구현은 archive하고 신규 제품 기능 대상에서 제외.
- Wails service binding을 활성 engine/UI boundary로 확정.
- Access log, OTel JSONL, GC log, JFR JSON, thread dump, Jennifer profile,
  profiler HTML/SVG 경로에 large-file guardrail 추가.
- Go/Wails desktop app용 Windows GUI smoke workflow 추가. GitHub-hosted
  Windows runner에서 Windows 실행 파일을 빌드하고 `archscope.exe`를 실행한 뒤
  GUI process가 유지되는지 확인하고 정상 종료한다.
- Release workflow hardening 추가. macOS signing/notarization secret preflight,
  build 이후 codesign/stapler validation, route-level frontend lazy loading,
  modular ECharts bundling, startup/chart-runtime chunk budget을 포함한다.

### Profiler, Access Log, GC, MSA 개선

- JFR-first profiler workflow, differential flame, heatmap selection,
  per-thread isolation, pprof export, tree view, native-memory leak finding,
  recent-files workflow.
- URL 통계, static/API 분류, percentile timeline, throughput, status
  distribution, `SLOW_URL_P95`, `ERROR_BURST_DETECTED`를 포함한 access log
  개편.
- JVM info card, worker/CPU mismatch warning, toggleable heap series, pause
  overlay, rectangle zoom, point decimation을 포함한 GC log 심화. 현재 Go
  analyzer는 young/old/metaspace series, OOM alert, long-pause event finding을
  함께 낸다.
- `stackTrace.frames`를 top methods, packages, threads, sample stacks,
  flamegraph-ready call tree로 집계하는 async-profiler 지향 JFR stack 분석.
- Jennifer MSA service-call network-time summary와 topology placement 추가.
  내부 single-digit ms 호출과 gateway/external double-digit ms 호출이 시각적으로
  분리된다.

### Trace Import MVP

- 외부 trace import용 canonical trace/span model.
- OTLP JSON-file parser, Zipkin v2 JSON parser, Elastic APM Elasticsearch
  `_search` response parser, Elastic APM source-only NDJSON parser, Jaeger
  QueryService/local trace JSON parser, schema-guarded SkyWalking GraphQL
  `queryTrace.spans` parser diagnostics.
- Summary, services, traces, spans, dependencies, service summary를 포함한
  `trace_import` analyzer result. 현재는 critical path와 deterministic
  finding도 함께 낸다.
- Summary card, service dependency/service latency chart, trace/span table,
  critical path row, parser diagnostics, Evidence Board 추가 action을 포함한
  Wails Trace Import page.
- CLI command:
  `archscope-engine trace import --in <file> --format auto|otlp-json|zipkin-v2-json|elastic-apm-search-json|elastic-apm-source-ndjson|jaeger-query-json|skywalking-graphql-json`.
- `examples/traces` 아래 sample trace fixture.

### Evidence Board Skeleton

- Analyzer finding, chart selection, table row, parser diagnostic, source
  metadata, comment, hypothesis, impact, recommendation을 담는 reusable local
  evidence-card model.
- Browser local storage 기반의 첫 desktop Evidence Board page.
- Trace Import 화면에서 finding, service edge, trace, source metadata를
  Evidence Board에 추가할 수 있다.
- Access Log, GC Log, JFR, Native Memory 화면에서 선택한 finding과 table row를
  Evidence Board에 추가할 수 있다.
- Evidence Board는 저장된 card를 local static HTML evidence report와 JSON
  evidence pack으로 export할 수 있다.

### Incident Timeline MVP

- Timestamp, source analyzer, severity, category, label, evidence reference,
  source metadata를 담는 공통 session timeline event model.
- Analysis Workspace 결과를 입력으로 사용하고 Evidence Board card를 출력으로
  사용하는 Wails Workspace 하위 Incident Timeline page.
- Deterministic finding, access-log error/latency series, GC alert, JFR
  pause/notable event, exception row, thread-dump contention/deadlock table,
  trace-import error/critical path를 timeline event로 매핑한다.

### SLO 및 Golden Signals MVP

- Access log, trace import, Jennifer MSA, exception, GC, JFR, thread dump,
  JVM metadata/runtime signal을 포함하는 Golden Signals inventory.
- Latency, traffic, errors, saturation에 대한 정규화된 SLI metric model.
- 기본 latency, error-rate, GC pause/throughput, OOM, deadlock,
  trace-integrity, MSA network-gap target을 적용하는 session-window SLO
  평가.
- Workspace 하위 Wails SLO / Golden Signals page에서 signal inventory, SLI
  metric, SLO violation, affected-scope breakdown, error-budget burn row,
  SLO violation의 Evidence Board capture를 제공한다.

### Service Flow MVP

- Trace Import `service_dependencies`, Jennifer MSA `tables.msa_edges`,
  Jennifer unprofiled external-call group을 위한 shared Service Flow input
  model.
- Caller, callee, call count, average/max/total latency, error count/rate,
  network gap, matched/unmatched call, evidence reference를 담는 common
  service-edge schema.
- Unmatched call, missing trace parent, 높은 MSA network gap에 대한
  deterministic service-flow finding.
- Workspace 하위 Wails Service Flow page에서 service-edge/finding table,
  Evidence Board capture, Mermaid sequence-like export, JSON export를 제공한다.

### 증거 기반 AI 보조 해석

- `apps/engine-native/internal/aiinterpretation` 아래 Go 구현.
- Evidence registry, evidence selector, prompt builder, Ollama client, AI
  finding validator.
- `InterpretationResult`를 `AnalysisResult`와 분리하는 contract.
- Local-only 기본 provider 정책, prompt-injection defense, validation,
  low-confidence filtering, evaluation requirement.

## 단기 로드맵: 0.3.x 안정화

이 항목들은 `work_status.md`의 현재 실행 큐와 계속 맞춰야 한다.

1. Release verification 상태를 유지한다.
   - Release cut 전 Windows GUI smoke를 반복하고, 가능한 경우 고객 환경에
     가까운 수동 smoke도 추가한다.
   - macOS signing/notarization credential이 complete한 상태로 검증되게
     유지한다.
   - Frontend startup chunk와 lazy chart-runtime chunk를 문서화된 budget
     안에서 추적한다.
   - 실제 로그가 현재 bounded-memory envelope를 넘으면 GC event streaming을
     더 깊게 검토한다.

2. JFR 및 Evidence Board 확장을 안정화한다.
   - 실제 CPU, wall, allocation, lock async-profiler JFR recording으로 stack
     aggregation을 검증한다.
   - 필요한 나머지 analyzer에도 "Add to Evidence" coverage를 넓힌다.
   - 현재 UI-level HTML/JSON export를 향후 engine-level report pack workflow로
     확장한다.
   - Evidence-reference integrity check가 통과한 뒤에만 AI interpretation을
     board와 연결한다.

3. Incident Timeline, SLO/Golden Signals, Service Flow MVP 이후 다음 Evidence
   Studio batch를 승격한다.
   - 다음 후보 track은 report pack과 evidence-gated AI interpretation
     productization이다.
   - 실제 customer export가 확보되면 Jaeger와 SkyWalking compatibility
     fixture가 대표성을 유지하는지 계속 검증한다.

## 중기 로드맵: Evidence Studio

### 장애 타임라인

- Report-pack generation에서 persisted timeline data가 필요해지면 Wails
  session timeline을 engine-level 또는 exportable `AnalysisResult`로 승격한다.
- Multi-file incident를 위해 event range, correlation ID, timeline grouping을
  더 풍부하게 만든다. Wails session projection은 이제 event range, duration,
  correlation ID, group metadata, `tables.groups`를 포함한다.
- 장애 중 무엇이 어떤 순서로 일어났는지를 설명하는 보고서 축으로 사용한다.
  Wails timeline은 이제 deterministic, evidence-linked narrative step을 만들고
  report pack에 포함한다.

### SLO 및 Golden Signals

- Report pack에서 persisted SLO data가 필요해지면 Wails session SLO /
  Golden Signals projection을 engine-level 또는 exportable `AnalysisResult`로
  승격한다.
- 사용자 편집 가능한 SLO target preset과 고객별 threshold profile을 추가한다.
- Analyzer가 충분한 timestamp series를 제공하는 경우 true time-window SLO
  evaluation을 추가하되, 현재 session aggregate view도 유지한다.
- Raw signal evidence를 숨기지 않는 방식으로 SLO violation과 budget row를
  report pack에 연결한다.

### Service Flow 및 MSA Topology

- Report pack에서 persisted service-flow data가 필요해지면 Wails session
  Service Flow projection을 engine-level 또는 exportable `AnalysisResult`로
  승격한다.
- Common service-edge contract가 더 많은 evidence source에서 안정화되면 현재
  Mermaid sequence-like export를 C4 dynamic view로 확장한다.
- Access log, trace, database slow log, broker log, runtime stack evidence가
  같은 service edge를 보강할 수 있도록 correlation-key stitching을 추가한다.

### 보고서 및 Evidence Pack

- Evidence Board 내용을 기반으로 session-derived Incident Timeline, SLO,
  Service Flow artifact를 포함한 report-ready HTML과 ZIP export를 만든다.
  Wails report pack은 현재 이 local HTML/ZIP bridge를 제공하며, 이후
  단계에서 PPTX/PDF를 추가할 수 있다.
- Source metadata, analyzer option, captured evidence, deterministic finding,
  derived artifact reference, optional AI interpretation provenance를 report
  pack JSON/HTML에 보존한다. 현재 report pack JSON/HTML은 이 provenance
  contract를 포함한다.
- 결론 뒤에 raw evidence를 숨기지 않는 고객-facing summary를 지원한다.
  현재 report pack은 evidence-linked key observation을 포함하고 exported
  pack 안에 raw evidence appendix를 유지한다.

### AI 보조 해석 제품화

- UI에 AI interpretation provenance를 표시한다. Analysis Workspace는 result가
  AI interpretation metadata를 포함할 때 provider/model/prompt metadata를
  표시한다.
- AI finding을 deterministic finding과 시각적으로 분리한다. Analysis
  Workspace는 confidence, reasoning, limitation, evidence reference를 포함한
  별도 AI-assisted panel에 AI finding을 표시한다.
- Golden diagnostics, evidence-reference integrity, quote-to-source matching,
  low-confidence filtering, hallucination review를 이용한 evaluation gate를
  추가한다. Go validator와 Wails workspace gate는 malformed,
  low-confidence, unknown-reference, quote-mismatched AI output을 차단한다.
- 생성된 모든 claim이 유효한 evidence reference를 가질 때만 Evidence Board와
  report generation에 연결한다. Evidence Board capture와 Report Pack AI
  section은 workspace evidence gate를 사용하며, 전체 interpretation이 통과한
  경우에만 AI finding을 포함한다.

## 중기 로드맵 플러스: 다중 언어 및 미들웨어 Evidence

현재 진행 중인 중기 Evidence Studio 사이클에 함께 묶을 추가 항목이다. 현재
parser/analyzer 커버리지를 다양한 프로그래밍 언어와 미들웨어 지원이라는 제품
약속과 대조해 보고 도출했으며, Incident Timeline, SLO/Golden Signals, Service
Flow, report pack, AI 제품화와 같은 호흡으로 함께 진행할 수 있는 규모로
잡았다.

### Access/Edge 로그 커버리지 확장

- 현재 access-log parser는 nginx(combined, combined+response time)와 apache
  combined 포맷만 지원한다. File-first 커버리지를 아래로 넓힌다.
  - Tomcat access valve와 Jetty NCSA request log.
  - HAProxy default와 HTTP log 포맷.
  - Envoy/Istio default와 JSON access log (service mesh trace header 포함).
  - AWS ELB/ALB classic, v2 access log, AWS CloudFront standard log.
  - GCP HTTP(S) Load Balancer JSON, Azure App Service/Front Door JSON.
  - IIS W3C extended log format과 Caddy/Traefik JSON access log.
  - Kong/Tyk/AWS API Gateway access log.
- trace-import importer dispatch와 비슷한 포맷 자동 탐지와 source별
  parser diagnostics를 추가한다.
- 신규 필드(upstream service, mesh trace-id, TLS 정보 등)를 기존 access-log
  `AnalysisResult` contract를 깨지 않고 정규화한다.

### Application/Web Server 진단

- Tomcat catalina.out, Jetty server log, JBoss/WildFly server log, WebLogic
  AdminServer/ManagedServer log, WebSphere SystemOut/SystemErr, GlassFish/Payara
  server log를 대상으로 하는 server-log analyzer family를 추가한다.
- Startup banner, deployment event, datasource pool warning, stuck thread,
  hung-thread detection, 알려진 severe error signature를 파싱한다.
- nginx와 Apache의 error log를 access log와 함께 파싱해 요청 evidence와
  worker 오류가 같은 Incident Timeline에 올라오게 한다.

### OpenTelemetry Logs 및 관측 신호 확장

- 기존 OTel trace JSONL 경로와 짝이 되는 OpenTelemetry Logs (OTLP JSON /
  NDJSON) parser/analyzer track을 추가한다. Severity, body, attribute,
  resource metadata, trace/span correlation, severity burst를 표현한다.
- 오프라인 metrics evidence를 위한 Prometheus snapshot/OpenMetrics importer를
  추가한다.
- LGTM 스택 export를 1급 evidence로 받기 위해 Loki query JSON export와 Tempo
  trace JSON export importer를 추가한다.
- Grafana dashboard JSON을 받아서 Evidence Board card에서 dashboard panel을
  참조할 수 있게 한다.

### Database Slow Query 및 Engine 로그 Evidence

- PostgreSQL(csvlog/text), MySQL/MariaDB slow query log, MongoDB
  profiler/diagnostic.data export, Redis slowlog get 출력, SQL Server
  extended events JSON을 위한 slow-query/engine-log parser/analyzer를
  추가한다.
- Query fingerprint, p95/p99 latency, top query, error count, lock wait,
  missing-index hint를 slow-query `AnalysisResult`로 집계한다.
- PostgreSQL/MySQL용 EXPLAIN plan importer를 추가해 plan evidence가
  Evidence Board에 함께 들어오게 한다.

### Message Broker 및 Streaming Middleware

- Kafka broker/controller/state-change log parsing을 추가한다. ISR change,
  rebalance, under-replicated, log compaction, KRaft quorum event를 다룬다.
- RabbitMQ server log parsing을 추가하고 connection churn, queue length,
  dead letter, partition event를 다룬다. `rabbitmq-diagnostics` JSON export
  도 함께 받는다.
- Pulsar broker log, NATS server log, ActiveMQ broker log parser를 후속으로
  추가한다.
- Broker finding을 Incident Timeline과 Service Flow에서 trace-import
  dependency와 함께 표시한다.

### Container / Kubernetes / Cloud Platform Evidence

- `kubectl get events`/`describe pod` JSON, audit log NDJSON, kubelet log,
  OOMKilled/restart/eviction 신호를 받는 Kubernetes evidence importer를
  추가한다.
- 애플리케이션 evidence와 함께 들어오는 container runtime log(containerd,
  CRI-O, Docker daemon)를 파싱한다.
- 보안 인시던트 Evidence Board card를 뒷받침하기 위해 AWS CloudTrail JSON,
  GCP Cloud Audit Logging JSON, Azure Activity Logs JSON cloud audit/log
  importer를 추가한다.

### 다중 언어 Stack 및 Profiler 커버리지

- 현재 runtime-stack parser는 Java, Go, Python, Node.js, .NET을 다룬다.
  Ruby(rbspy text/JSON, stackprof), PHP(Excimer, Tideways CLI export, Xdebug
  profile), Rust(perf script, tokio-console export), Kotlin/Scala JVM(JFR/
  thread dump 경유), Swift backtrace, async stack trace까지 확장한다.
- Go pprof, Datadog Ruby/PHP profiler, py-spy speedscope-via-pprof 등 pprof
  호환 runtime이 한 analyzer 경로를 공유하도록 generic pprof 바이너리
  (`.pb.gz`) importer를 추가한다.
- py-spy raw output, rbspy raw output, async-profiler `.html`/collapsed,
  dotnet-trace `.nettrace`/speedscope, perf script collapsed importer를
  추가한다.
- 기존 collapsed/JFR profiler analyzer를 language-tagged frame, native vs
  managed split, cross-language flamegraph rollup을 지원하는 통합
  multi-language profile analyzer로 승격한다.

### Correlation 및 Evidence Stitching

- trace-id, span-id, request-id, customer/tenant-id, container-id, pod-uid,
  host-id, PID를 다루는 cross-source correlation key model을 정의한다.
- Access log, trace, runtime stack, broker log, database slow log를
  correlation key로 묶어 Incident Timeline과 Evidence Board에 올리는 evidence
  stitching pass를 추가한다.
- Missing trace-id, dropped parent span, 매칭되지 않는 request log 같은
  correlation gap을 deterministic finding으로 노출한다.

### Local Continuous Profiling Import

- Grafana Pyroscope/Phlare snapshot export, Polar Signals Parca snapshot
  export를 file-first로 import해 같은 flamegraph analyzer를 통해 continuous
  profiling evidence가 흐르게 한다.
- 2026년 1분기 기준 public alpha인 OpenTelemetry Profiles signal은 장기 로드맵
  항목으로 추적하고, 안정화 시 정식 OTLP Profiles ingestion으로 승격한다.

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

### OpenTelemetry Profiles 및 eBPF Continuous Profiling

- OpenTelemetry Profiles signal lifecycle을 추적한다. 2026년 1분기 public alpha
  였으며 이후 RC/GA를 목표로 한다. 스펙이 안정되면 OTLP Profiles 파일
  importer를 추가한다.
- 오픈소스 OpenTelemetry eBPF profiler와 Parca/Pyroscope agent와 호환되는
  eBPF profile evidence importer를 계획한다. C/C++, Go, Rust, Python, Java,
  Node.js, .NET, PHP, Ruby, Perl frame을 다룬다.
- JFR, pprof, OTLP Profiles, eBPF sample을 묶는 통합 profile evidence schema를
  정의해 flamegraph 및 Evidence Board capture가 언어 독립적으로 동작하게 한다.

### Browser, Mobile, Client Evidence

- 오픈 RUM beacon export 기반의 Real User Monitoring(RUM) import 경로를
  추가한다. Core Web Vitals(LCP, INP, CLS)와 resource timing을 다룬다.
- Browser performance trace import(Chrome DevTools `.json`, Lighthouse report
  JSON)와 synthetic check export를 추가한다.
- 안정적인 파일 contract가 확보되면 모바일 성능 import(Firebase Performance
  export, Sentry performance, App Center diagnostic export)를 검토한다.

### Anomaly Detection 및 Causal Analysis

- Golden signal에 대한 통계적 baseline(rolling median/p95, seasonality-aware
  band)을 추가하고 편차를 deterministic finding으로 노출한다.
- Access-log latency, GC pause cluster, JFR CPU/lock signal, trace error rate
  에 대한 change-point detection을 추가한다.
- Incident timeline event를 correlation key로 묶어 ordered root-cause
  hypothesis를 제안하는 causal-chain explorer를 추가한다.

### Live / Streaming Evidence (Read-Only)

- 로컬 파일(access log, GC log, OTel exporter, broker log) tailing을 read-only
  로 검토한다. Local-first 보장을 유지하면서 Evidence Studio 세션에서 live
  하게 사용할 수 있게 한다.
- 발생하는 event를 evidence card로 승격하는 streaming 모드 Incident Timeline
  을 검토한다.

### Report Distribution 및 Workflow Integration

- 이슈 트래커나 ADR에 붙여 넣을 수 있는 Markdown/Mermaid/PlantUML 보고서 pack
  export를 추가한다.
- Jira/GitHub Issue, Slack/Teams summary post, email-friendly evidence pack
  zip을 위한 1-click template을 옵션으로 추가한다. 반드시 opt-in이며
  evidence-bound로 유지한다.

## 외부 APM Import 로드맵

### File-First 우선순위

1. OpenTelemetry OTLP JSON file - trace import MVP 완료.
2. Zipkin v2 JSON - trace import MVP 완료.
3. Elastic APM Elasticsearch `_search` response와 source-only NDJSON - trace
   import MVP 완료.
4. Jaeger compatibility import - stable QueryService/local trace JSON contract로
   완료.
5. Apache SkyWalking GraphQL response import - schema-guarded
   `queryTrace.spans` 지원 완료. Importer를 더 넓히기 전 추가 SkyWalking
   response version을 계속 검증한다.

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
