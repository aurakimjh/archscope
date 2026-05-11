# ArchScope 제품 확장 분석 메모

Last updated: 2026-05-11

이 문서는 대화 컨텍스트를 초기화해도 이어서 작업할 수 있도록 현재까지의
결론과 다음 조사 항목을 저장한 작업 메모다.

## 현재 상태

- 원격 `main`은 최신 상태로 pull 완료.
- 모든 표 헤더 기본 정렬 기능을 추가하고 커밋/푸시 완료.
  - Commit: `e04fc1d Add sortable table headers`
  - 변경 파일:
    - `apps/engine-native/cmd/archscope-app/frontend/src/hooks/useSortableTables.ts`
    - `apps/engine-native/cmd/archscope-app/frontend/src/App.tsx`
    - `apps/engine-native/cmd/archscope-app/frontend/public/style.css`
- 최신 실행파일 빌드 완료.
  - `C:\workspace\archscope\apps\engine-native\cmd\archscope-app\bin\archscope.exe`

## 현재 기능 축

ArchScope는 현재 로컬 우선 Go/Wails 데스크톱 앱으로 다음 진단 기능을 가진다.

- Profiler: collapsed stacks, FlameGraph SVG/HTML, Jennifer CSV, drill-down,
  execution breakdown, timeline, pprof export.
- Compare: profiler before/after diff.
- Access Log: 요청량, 응답시간, 오류율, URL/status 분석.
- GC Log: pause, heap, allocation/promotion, JVM info, findings.
- JFR / Native Memory: notable events, heatmap, event breakdown, native memory
  leak 후보.
- Exception: 이벤트, 타입/시그니처 분포, root cause, findings.
- Thread Dump: single dump, multi-dump correlation, lock contention, deadlock,
  JVM signals.
- MSA Timeline: Jennifer profile 기반 서비스 호출 타임라인, topology,
  treemap, network gap, slow SQL, parser report.

## 제품 방향 의견

현재 제품은 "파일별 분석기 모음"으로는 충분히 강하다. 다음 단계는
"아키텍처 회의와 고객 보고에 바로 붙일 수 있는 Evidence Studio"가 되는
방향이 적합하다.

웹 리서치 기준으로 확인한 기준 축:

- OpenTelemetry: traces, metrics, logs, profiles를 관찰성 핵심 신호로 본다.
- Google SRE: latency, traffic, errors, saturation을 4대 golden signals로 본다.
- SLO/SLI/error budget은 서비스 품질을 설명하는 표준 언어다.
- Azure Well-Architected는 reliability, security, cost optimization,
  operational excellence, performance efficiency를 아키텍처 검토 축으로 둔다.
- C4, arc42, ADR은 고객/개발자/아키텍처 회의용 설명 구조로 유용하다.

## 우선 추가 후보

1. Evidence Board / Evidence Pack
   - 각 분석 화면의 차트, 표, finding, 원본 파일 메타데이터를 증거 카드로 저장.
   - 카드별 코멘트, 원인 가설, 영향도, 권고안, 출처 파일, 분석 옵션 기록.
   - PPTX/HTML/PDF/ZIP export.
   - 가장 먼저 추가할 가치가 높다.

2. SLO / Golden Signals 리포트
   - Access Log, MSA, OTel, JFR 데이터를 묶어 latency, traffic, errors,
     saturation 자동 산출.
   - p50/p95/p99, 오류율, 처리량, 포화 징후, SLO 위반 시간대,
     error budget burn 표 제공.

3. Incident Timeline
   - Access Log 오류 피크, GC pause, JFR event, Thread Dump BLOCKED,
     Exception burst, Profiler hotspot을 한 시간축에 정렬.
   - 장애 보고서의 "무슨 일이 어떤 순서로 발생했는가"에 대응.

4. Service Flow / MSA Topology 강화
   - 서비스 간 호출 그래프에 호출 수, 평균/최대/합계 지연, 오류율,
     network gap, unmatched call 표시.
   - C4 dynamic view 또는 sequence-like view export.

5. API / Event Contract 분석
   - OpenAPI import 후 Access Log와 대조.
   - 문서에 없는 API, 사용되지 않는 API, 오류율 높은 API, 느린 API 탐지.
   - AsyncAPI import 후 Kafka/RabbitMQ/WebSocket producer/consumer 흐름 분석.

6. Architecture Documentation Pack
   - arc42 섹션 기반 Context, Runtime View, Deployment View,
     Quality Requirements, Risks 초안 생성.
   - ADR 초안 생성: 결정, 근거, 대안, 트레이드오프, 영향.

7. Security / Compliance Evidence
   - OWASP Top 10 관점의 로그 패턴, 비정상 상태 코드, 민감정보 노출 가능성,
     보안 로깅 누락 징후.
   - SBOM/CycloneDX import, 취약점/라이선스/영향 서비스 맵.

8. Before / After 멀티 시그널 비교
   - Profiler뿐 아니라 Access Log, GC, JFR, MSA까지 비교.
   - 튜닝/배포 전후 p95, 오류율, GC pause, hotspot, thread blocking,
     external call latency 비교.

## 메뉴체계 검토안

현재 메뉴는 분석기 단위로 평면에 가깝다. 기능이 늘면 업무 흐름 기준으로
재편하는 것이 좋다.

추천 구조:

- Workspace
  - Evidence Board
  - Reports / Exports
  - Findings
- Diagnostics
  - Profiler
  - Access Logs
  - GC / JVM
  - JFR / Native Memory
  - Exceptions
  - Thread Dumps
- Service Flow
  - MSA / Timeline
  - Topology
  - Incident Timeline
  - Trace / OTel
- Architecture
  - API Contracts
  - C4 Diagrams
  - ADR / Decisions
  - Risks / Tech Debt
- Operations
  - SLO / Golden Signals
  - Capacity / Saturation
  - Before / After
  - Delivery Metrics
- Security
  - Threat Model
  - SBOM / Vulnerabilities
  - Sensitive Data / Redaction
- Settings

메뉴명 제안:

- `MSA Timeline` -> `MSA / Service Flow`
- `Compare` -> `Before / After`
- `Parser Report`는 각 분석기 하위에 유지하되, 전역 `Evidence Board`에서
  필요한 parser evidence를 끌어올 수 있게 한다.

## APM export 가능성 조사 메모

사용자가 추가로 요청한 내용:

- 다른 APM 툴들의 기능을 분석한다.
- Jennifer처럼 로컬로 내려받아 ArchScope가 재분석 가능한 export 형식이
  있는지 확인한다.
- 대상 후보:
  - Dynatrace
  - Datadog APM / Continuous Profiler
  - New Relic APM / NerdGraph
  - Cisco AppDynamics
  - Elastic APM
  - Jaeger
  - Zipkin
  - Apache SkyWalking
  - Pinpoint
  - OpenTelemetry Collector file exporter

다음 조사 기준:

- UI에서 CSV/JSON/trace/profile export가 가능한가.
- 공식 API로 trace/span/transaction snapshot/profile을 받을 수 있는가.
- local-first 분석에 적합한 파일 포맷인가.
- 보존 기간과 샘플링 때문에 evidence 완전성이 떨어지는지.
- ArchScope import 대상:
  - trace JSON
  - span table CSV
  - service dependency map
  - pprof/profile
  - metrics time series CSV/JSON
  - log/event export

예상 결론 가설:

- Elastic APM, Jaeger, Zipkin, OpenTelemetry Collector는 로컬 파일 기반
  재분석 친화도가 높을 가능성이 크다.
- Datadog, Dynatrace, New Relic, AppDynamics는 API export는 가능하지만
  SaaS 권한/보존/샘플링/스키마 제약이 있어 "Jennifer처럼 파일 하나를
  내려받아 분석" 경험은 제품별로 차이가 클 가능성이 있다.
- ArchScope에는 우선 OpenTelemetry/Jaeger/Zipkin/Elastic 계열 trace JSON
  import를 추가하는 것이 범용성이 높다.

## APM export 가능성 조사 결과

2026-05-11에 `docs/ko/APM_IMPORT_MATRIX.md`를 추가했다.

1차 import 우선순위:

1. OpenTelemetry OTLP JSON file
2. Zipkin v2 JSON
3. Elastic APM Elasticsearch documents / `_search` export
4. Jaeger Query JSON 또는 QueryService는 2차 안정화 대상

SaaS API connector(Datadog, Dynatrace, New Relic, AppDynamics)는 권한,
보존 기간, 샘플링, rate limit 영향을 크게 받으므로 파일 import 이후 단계로 둔다.

## 실행 TO-DO

제품 확장 작업은 `docs/ko/PRODUCT_EXPANSION_TODO.md`에 우선순위별로 정리했다.

현재 P1 순서:

1. Elastic APM `_search` / source-only NDJSON importer.
2. Trace import result Wails UI 연결.
3. Trace critical path / service dependency 시각화.
4. Evidence Board 최소 기능 설계.
5. Incident Timeline / SLO 리포트 설계.
