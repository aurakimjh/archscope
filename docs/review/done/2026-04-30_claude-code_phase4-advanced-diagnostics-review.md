# Phase 4 — Advanced Diagnostics 설계 결정 품질 감사

**일자**: 2026-04-30
**검토자**: Claude Code (Opus)
**페르소나**: 시니어 진단/관측성 아키텍트 · 시니어 시스템 설계자 · 도메인 리서처
**범위**: T-028, T-034, T-035
**커밋**: `aa64764` ("Complete phase 3 follow-up and phase 4 diagnostics")
**변경**: 22 파일, +577/−27
**검토 성격**: 설계 결정 품질 감사 (Design Decision Audit)

---

## 0단계 — 컨텍스트 로드

### 사전 참조 문서

| 문서 | 핵심 확인 사항 |
|---|---|
| RD-023 | Timeline Correlation을 P4 차별화 요소로 보존. Phase 3 JVM 파서 이후 착수 |
| RS-013 | JFR 파서를 설계 스파이크로 시작. JEP 509가 JFR 관련성 강화 |
| RS-014 | OTel Logs Data Model 안정화(Stable). trace/span context 필드 활용 가능 |
| T-010 | `AnalysisResult` 공통 계약: `type`, `source_files`, `summary`, `series`, `tables`, `charts`, `metadata`, `created_at` |
| T-026/T-027 | 다중 스택 분류: `StackClassificationRule` frozen dataclass, 외부 JSON 설정 로드 지원 |

### AnalysisResult 현재 구조

```python
@dataclass(frozen=True)
class AnalysisResult:
    type: str                           # "access_log" | "profiler_collapsed"
    source_files: list[str]
    summary: dict[str, Any]
    series: dict[str, Any]
    tables: dict[str, Any]
    charts: dict[str, Any]
    metadata: dict[str, Any]
    created_at: str
```

현재 구현된 type-specific 계약: `AccessLogSummary`, `AccessLogSeries`, `AccessLogTables`, `AccessLogMetadata`, `ProfilerCollapsedSummary`, `ProfilerCollapsedSeries`, `ProfilerCollapsedTables`, `ProfilerCollapsedMetadata`. 모든 계약은 `TypedDict`로 정의됨.

**핵심 관찰**: `AnalysisResult`의 `summary`/`series`/`tables`/`metadata`는 `dict[str, Any]`이므로, 새로운 type(`jfr_recording`, `otel_log`, `timeline_correlation`)을 추가할 때 Python 런타임 타입 안전성은 `TypedDict`에 의존한다. 기존 패턴과의 통합 가능성은 열려 있다.

---

## 1단계 — 산출물 형태 식별

### 결정 문서 위치

세 작업의 산출물이 **단일 문서** `docs/en/ADVANCED_DIAGNOSTICS.md` (138행)에 통합되어 있다.

| 작업 | 문서 위치 | 형태 |
|---|---|---|
| T-028 | `ADVANCED_DIAGNOSTICS.md` §Timeline Correlation | AnalysisResult shape + 규칙 4개 |
| T-034 | `ADVANCED_DIAGNOSTICS.md` §JFR Recording Parser Spike | 파서 흐름 5단계 + shape + deferral |
| T-035 | `ADVANCED_DIAGNOSTICS.md` §OpenTelemetry Log Input | 필드 매핑 테이블 + shape + 규칙 4개 |

별도 ADR(Architecture Decision Record)이나 스파이크 보고서 디렉토리(`docs/decisions/`, `docs/spike/`)는 존재하지 않는다. 모든 결정이 하나의 설계 문서에 병합되어 있다.

### 로드맵 문서

`docs/en/ROADMAP.md`에 Phase 4가 "Multi-runtime and Observability Inputs"로 기록됨. T-028(Timeline Correlation), T-034(JFR), T-035(OTel)이 모두 명시적 항목으로 반영됨.

### AnalysisResult 변경 여부

**코드 변경 없음.** `result_contracts.py`에 `jfr_recording`, `otel_log`, `timeline_correlation`에 대한 TypedDict가 추가되지 않았다. Shape은 문서에만 텍스트로 정의됨.

### PoC 코드

**없음.** Python 엔진에 JFR/OTel 관련 코드가 전혀 없다 (`grep` 확인). 별도 브랜치도 없다.

### 산출물 형태 평가

| 항목 | 평가 |
|---|---|
| 문서 일관성 | 세 결정이 하나의 문서에 통합되어 있어 횡단 참조가 용이함. 긍정적 |
| ADR 형식 부재 | 결정의 배경(Context), 대안(Alternatives), 결과(Consequences) 구조가 없음. 결정 추적이 어려움 |
| EN/KO 문서 동시 제공 | `docs/ko/ADVANCED_DIAGNOSTICS.md` 미러 확인 완료 |
| 로드맵 반영 | Phase 4 항목 명시적 기록 완료 |

---

## 2단계 — 작업별 심층 검증

---

### T-028: Timeline Correlation 로드맵 보존

#### 보존 근거 분석

문서에서 Timeline Correlation은 "a planned differentiator for JVM and multi-runtime diagnostics"로 기술된다. 그러나:

| 점검 항목 | 결과 | 평가 |
|---|---|---|
| 차별화 가설의 명시적 서술 | "planned differentiator"라는 한 단어 선언만 존재. **왜** 차별화인지, **어떤 가치**를 만드는지 서술 없음 | **부족** |
| 경쟁 도구 비교 | 전혀 없음. Datadog, JProfiler, Grafana Pyroscope 등과의 비교 부재 | **부족** |
| 타깃 사용자 시나리오 | 없음. "누가, 언제, 왜" 사용하는지 묘사 없음 | **부족** |
| 선행 조건 명시 | 입력 type 6개 나열됨 (`access_log`, `profiler_collapsed`, `gc_log`, `thread_dump`, `jfr_recording`, `otel_log`). 어떤 것이 필수이고 어떤 것이 선택인지 구분 없음 | **부분적** |
| 실현 시점 트리거 | 정의되지 않음. "언제 시작할 것인가"에 대한 기준 부재 | **부족** |
| 로드맵 위치 | `ROADMAP.md` Phase 4에 명시적 항목. 단순 TODO가 아닌 Phase 제목 수준 배치 | **양호** |

#### AnalysisResult Shape 평가

제안된 `timeline_correlation` shape:

```
summary: window_start, window_end, correlated_event_count, highest_severity
series: correlated_events [{time, source_type, event_type, severity, label, evidence_ref}]
tables: evidence_links [{evidence_ref, source_file, raw_line, raw_block, trace_id, span_id}]
metadata: schema_version, input_result_types, correlation_window_ms
```

**긍정적 관찰**:
- `evidence_ref`를 통해 상관 이벤트와 원본 증거를 분리한 설계가 우수. RD-024("AI 해석에 원본 증거 요구") 원칙과 일관됨
- `trace_id`/`span_id`를 evidence_links에 포함하여 OTel 데이터와의 결합점 확보
- "heuristic correlations marked with confidence" 규칙이 추론과 사실을 분리

**부족한 점**:
- `confidence` 필드가 규칙에 언급되지만 shape의 `correlated_events`에는 포함되지 않음
- `time` 필드의 정밀도/표현 형식 미정의 (UTC? ns? ms? ISO 8601?)
- 시간 윈도우의 정렬 정밀도에 대한 논의 부재 (서로 다른 소스의 clock skew 처리)

#### 발견 사항

| # | 심각도 | 발견 | 권고 |
|---|---|---|---|
| F-001 | **High** | 차별화 가설이 선언적 한 줄에 불과. 경쟁 도구 비교, 사용자 시나리오, 가치 제안(value proposition) 부재. "왜 차별화인가"에 답하지 못하면 1년 후 이 항목은 방치된 TODO가 될 위험 | 별도 섹션으로 차별화 가설 서술: 경쟁 도구 한계, 목표 사용자 시나리오, ArchScope만의 접근 차이점을 1-2 문단으로 명시 |
| F-002 | Medium | 실현 시점 트리거 미정의. 어떤 파서/데이터가 준비되면 착수할 것인지 명시 필요 | "최소 2개의 시계열 결과 type이 프로덕션 품질에 도달할 때" 등 구체적 조건 기술 |
| F-003 | Medium | `correlated_events` shape에 `confidence` 필드 누락. 규칙("mark with confidence")과 모델이 불일치 | shape에 `confidence: float | null` 추가, 또는 규칙에서 confidence 요구 사항 제거 |
| F-004 | Minor | `time` 필드의 정밀도/형식 미정의. 향후 ms vs ns 불일치 위험 | timestamp 표현 규칙을 공통으로 정의 (ISO 8601 UTC 권장, 나노초 정밀도 보존 가능) |

#### 판정

T-028은 로드맵 보존이라는 명목 수준에서는 완료되었으나, **능동적인 차별화 전략 선언으로서는 미흡**하다. Shape 설계 자체는 우수하나 차별화 근거가 빈약하다.

**결과**: ⚠️ **조건부 PASS** — 차별화 가설 보강 필요

---

### T-034: JFR 파서 설계 스파이크

#### 라이브러리 타당성 평가

| 점검 항목 | 문서 기록 | 평가 |
|---|---|---|
| 고려된 옵션 | JDK `jfr` 커맨드만 명시적으로 선택. "native parser library decision should wait" 한 줄 언급 | **부족** — 대안 비교 테이블 없음 |
| 순수 Python 파서 | 검토 흔적 없음 | **부족** |
| Java 라이브러리 (JMC 파서, Jafar) | 검토 흔적 없음 | **부족** |
| Go 파서 (Grafana jfr-parser) | 검토 흔적 없음 | **부족** |
| 라이선스 | 전혀 언급 없음 | **부족** |
| JDK 버전 의존성 | 전혀 언급 없음 | **부족** |
| Python에서의 사용 방안 | JDK `jfr` CLI를 외부 프로세스로 호출하는 모델은 기존 Python→CLI 패턴과 일관됨 (Phase 3 `execFile` 패턴). 이 점은 아키텍처적으로 타당 | **양호** |
| 성능 추정 | 대용량 JFR (GB 단위) 처리에 대한 언급 없음 | **부족** |
| JEP 509 인지 | 전혀 없음. JDK 25에서 `jdk.CPUTimeSample` 이벤트가 실험적으로 추가된 사실 미반영 | **부족** |

**외부 리서치에 기반한 보충 평가**:

현재(2026년 4월) JFR 파서 생태계:
- **JDK `jfr` CLI**: JDK 21+ 기준 `print --json`, `summary`, `view`, `metadata` 지원. JDK 26 EA에서 `--exact` 옵션 추가(정밀 타임스탬프). 가장 안정적이나 별도 JDK 런타임 필요
- **Java API** (`jdk.jfr.consumer.RecordingFile`): JVM 내부에서 프로그래밍 방식 접근. Python 엔진과의 연동에는 별도 Java 사이드카 또는 JNI 필요
- **Grafana jfr-parser** (Go): Pyroscope에서 사용. Go 바이너리 의존
- **순수 Python JFR 파서**: 2026년 4월 기준 성숙한 라이브러리 부재. [Gunnar Morling의 JFR 파일 형식 분석](https://www.morling.dev/blog/jdk-flight-recorder-file-format/) 참조 시, 바이너리 포맷 파싱은 상당한 구현 비용
- **JEP 509** (JDK 25): `jdk.CPUTimeSample` 실험적 이벤트. CPU 시간 기반 프로파일링으로 JFR의 진단 가치 크게 향상. ArchScope가 이를 인지하고 우선 이벤트 목록에 포함해야 함

**JDK `jfr` CLI 접근의 타당성**: 순수 Python 파서 부재와 Java 사이드카의 복잡도를 고려할 때, CLI 호출은 **현실적으로 올바른 초기 선택**이다. 다만 이 결론에 도달하기까지의 **대안 비교 과정이 문서에 기록되지 않았다**.

#### AnalysisResult Shape 평가

제안된 `jfr_recording` shape:

```
summary: event_count, duration_ms, gc_pause_total_ms, blocked_thread_events
series: events_over_time [{time, event_type, count}], pause_events [{time, duration_ms, event_type, thread}]
tables: notable_events [{time, event_type, duration_ms, thread, message, evidence_ref}]
metadata: parser, schema_version, jfr_command_version, event_filters
```

**긍정적 관찰**:
- 기존 `AnalysisResult` 패턴(`summary`/`series`/`tables`/`metadata`)에 자연스럽게 얹힘. T-010 이후의 구조적 일관성 유지
- `evidence_ref` 필드로 원본 증거 참조 유지 (RD-024 원칙 준수)
- `jfr_command_version`으로 파서 종류와 버전 추적 가능
- `event_filters`로 분석 범위 재현 가능성 확보

**부족한 점**:

| # | 심각도 | 발견 | 권고 |
|---|---|---|---|
| F-005 | **High** | **PoC 부재**. Python 엔진에 JFR 관련 코드가 전혀 없고 (`grep` 확인), 별도 브랜치에도 없음. 스파이크(spike)는 본질적으로 **코드로 검증**하는 활동. 문서만으로 내린 결정은 "실제 JFR 파일을 파싱해보니 이런 문제가 있었다"는 경험적 근거가 없다 | JDK `jfr print --json`으로 실제 JFR 파일을 파싱하고, JSON 출력을 Python에서 로드하여 `AnalysisResult`로 변환하는 최소 PoC 필요 |
| F-006 | **High** | 라이브러리 대안 비교 부재. JDK CLI / Java API / Grafana Go 파서 / 순수 Python 파서 — 어느 것도 비교 테이블에 올라오지 않음. "스파이크"라는 이름으로 결정을 내렸지만, 스파이크의 핵심인 **대안 탐색과 실험**이 수행되지 않음 | 대안 비교 테이블 추가: 각 옵션의 장단점, 라이선스, JDK 호환성, Python 연동 방안 명시 |
| F-007 | **High** | 라이선스 미검토. JDK `jfr` CLI는 Oracle JDK 라이선스(NFTC 또는 GPL+CE)와 OpenJDK GPL v2+CE 중 어디에 해당하는지, 재배포/번들링 가능성 미확인 | `jfr` CLI는 JDK의 일부로 배포되므로 ArchScope가 JDK를 번들링하지 않는 한 라이선스 문제는 낮으나, "사용자 JDK에 의존한다"는 전제 조건을 명시해야 함 |
| F-008 | Medium | JDK 버전 호환성 미정의. `jfr print --json`은 JDK 14+ 지원. 분석 대상 JFR 파일은 JDK 8(commercial), 11, 17, 21, 25 등에서 생성 가능. 파서 JDK와 녹화 JDK의 버전 매칭 문제 미언급 | 최소 지원 JDK 버전(파서용), 지원 JFR 녹화 버전 범위 명시 |
| F-009 | Medium | 이벤트 커버리지의 우선순위 근거 부재. GC, CPU sample, thread block, exception, I/O 5가지가 나열되지만 **왜 이 5가지인지** 설명 없음 | 각 이벤트의 진단 가치와 Timeline Correlation 기여도를 기준으로 우선순위 근거 명시 |
| F-010 | Medium | JEP 509 (`jdk.CPUTimeSample`) 미반영. JDK 25에서 실험적으로 추가된 CPU 시간 기반 프로파일링 이벤트. JFR의 프로파일링 가치를 크게 높이는 기능이므로 우선 이벤트 후보에 포함되어야 함 | 우선 이벤트 목록에 `jdk.CPUTimeSample` (JDK 25+ 실험적) 추가 |
| F-011 | Medium | 메모리 사용 모델 미정의. `jfr print --json`은 전체 이벤트를 JSON으로 출력하므로 대용량 JFR(GB 단위)에서 메모리 폭발 가능. `--events` 필터로 부분 로드하는 전략이 필요하나 문서에 없음 | `--events` 필터와 `--stack-depth` 조합으로 점진적 로드 전략 기술 |
| F-012 | Medium | 샘플 vs 정확 이벤트 구분 부재. `jdk.ExecutionSample`(샘플링 기반)과 `jdk.GCPhasePause`(정확 이벤트)는 해석 방법이 다름. shape에 이 구분이 없음 | `event_type`에 `sampling_type: "sampled" | "exact"` 메타 필드 고려 |
| F-013 | Minor | 콜스택 계층 표현 방식 미정의. flat 문자열 / 프레임 배열 / 트리 — profiler_collapsed에서는 `frames: list[str]`을 사용하지만, JFR shape에는 콜스택 표현이 빠져 있음 | `notable_events`에 `frames: list[str]` 또는 `stack_trace: str` 필드 추가 |

#### 판정

**결과**: ❌ **조건부 FAIL** — 스파이크의 본질(코드 검증)이 결여됨

JDK CLI 접근이라는 **방향은 타당**하지만, "스파이크"를 표방하면서 PoC 코드가 없고 대안 비교 테이블이 없는 것은 스파이크의 정의에 부합하지 않는다. 문서 품질 자체(shape 설계, 파서 흐름)는 양호하나, 경험적 근거 없는 설계 결정의 신뢰도가 낮다.

---

### T-035: OpenTelemetry 로그 입력 설계

#### OTel 사양 준수 검증

OTel Logs Data Model은 **Stable** 상태이다 (opentelemetry.io 직접 확인). 문서의 필드 매핑을 사양과 대조한다:

| OTel 공식 필드 | 문서 매핑 | 사양 일치 | 비고 |
|---|---|---|---|
| `Timestamp` (uint64 ns) | event time | ✅ | 단, 정밀도 명시 필요 |
| `ObservedTimestamp` (uint64 ns) | event time (대체) | ✅ | `Timestamp` 부재 시 fallback |
| `TraceId` (16바이트) | `trace_id` | ✅ | |
| `SpanId` (8바이트) | `span_id` | ✅ | |
| `TraceFlags` (byte) | `trace_flags` | ✅ | |
| `SeverityText` (string) | severity label | ✅ | |
| `SeverityNumber` (1-24) | normalized severity | ✅ | 사양의 1-24 범위 매핑 정확 |
| `Body` (AnyValue) | message/body | ✅ | |
| `Resource` | service/runtime metadata | ✅ | |
| `InstrumentationScope` | instrumentation scope metadata | ✅ | |
| `Attributes` | structured event attributes | ✅ | |
| `EventName` (string) | event type | ✅ | |

**사양 준수 판정**: 필드 매핑이 OTel Logs Data Model과 **정확히 일치**한다. 이 부분은 높이 평가할 만하다.

#### Non-OTLP 로그 trace context 처리

문서의 규칙: "Map legacy JSON fields named `trace_id`, `span_id`, and `trace_flags` to OpenTelemetry trace context."

OTel 공식 호환 사양(Trace Context in Non-OTLP Log Formats) 확인 결과:
- 필드명 `trace_id`, `span_id`, `trace_flags`를 lowercase hex로 기록하는 것이 권장됨
- 문서의 매핑이 이 권고와 일치

#### 입력 포맷 지원 범위

| 점검 항목 | 문서 기록 | 평가 |
|---|---|---|
| OTLP/JSON | "OTLP-style JSON first" — 명시적 우선순위 | ✅ |
| OTLP/Protobuf | 미언급 | 허용 범위 — JSON 우선은 올바른 초기 결정 |
| OTLP/gRPC | 미언급 | 허용 범위 — 파일 기반 도구에서 gRPC 수신은 범위 외 |
| 레거시 JSON/plain text | 언급됨 | ✅ |
| 로그 수집 경로 | 파일 기반 암시. 별도 receiver 구현 미언급 | 기존 아키텍처(파일 선택 → CLI 분석)와 일관됨 |

#### AnalysisResult Shape 평가

제안된 `otel_log` shape:

```
summary: log_count, error_count, trace_linked_count, service_count
series: logs_over_time [{time, severity, count}], trace_event_counts [{trace_id, count}]
tables: log_records [{time, severity, trace_id, span_id, service_name, body, evidence_ref}]
metadata: parser, schema_version, accepted_formats
```

**긍정적 관찰**:
- `trace_linked_count`로 trace 연결 가능 로그 비율을 즉시 파악 가능 — Timeline Correlation 진입점으로 우수
- `evidence_ref`로 원본 증거 보존 (T-028과 일관)
- `service_name` 포함으로 다중 서비스 분석 기반 확보

#### 발견 사항

| # | 심각도 | 발견 | 권고 |
|---|---|---|---|
| F-014 | Medium | OTel 사양 버전 미명시. Logs Data Model은 Stable이나, 사양은 지속 진화 중. 어느 시점의 사양을 기준으로 하는지 기록 필요 | 참조 사양 버전 또는 확인 일자 명시 (예: "OTel Logs Data Model v1.x, 2026-04 확인") |
| F-015 | Medium | 민감 정보(PII/secret) 처리 방침 부재. OTel 로그 `Body`와 `Attributes`에는 사용자 데이터, API 키, 개인정보가 포함될 수 있음. 파일 기반 분석 도구라도 결과 JSON에 원본 로그가 남으면 데이터 유출 경로가 됨 | `evidence_ref`의 원본 보존 범위 정책 추가: 전체 body 포함 vs 요약/마스킹 옵션 |
| F-016 | Medium | 대용량 로그 입력 시 샘플링/필터링 정책 부재. access log에서는 `max_lines`와 `time_range` 필터가 있으나 OTel 로그에 대한 동등한 제어가 정의되지 않음 | Phase 1B의 sampling 패턴(`max_lines`, `time_range`, `BoundedPercentile`)을 OTel 파서에도 적용할 계획 명시 |
| F-017 | Minor | `trace_event_counts` series가 `trace_id`별 카운트를 제공하지만, 고카디널리티 환경(수천~수만 trace)에서 이 시리즈의 크기가 폭발할 수 있음 | top-N 제한 또는 aggregate 전략 필요 |
| F-018 | Minor | `Timestamp` 정밀도 (ns) vs 기존 access_log `timestamp` (초/밀리초 단위 문자열)의 불일치. 향후 Timeline Correlation에서 정렬 정밀도 차이 발생 가능 | 공통 timestamp 정규화 정책 정의 |

#### 판정

**결과**: ✅ **PASS** — OTel 사양 준수가 정확하고 shape 설계가 기존 패턴과 잘 통합됨

T-035는 세 작업 중 가장 완성도가 높다. 사양 준수, 필드 매핑, 레거시 호환, 상관관계 키 보존이 모두 적절하다. 민감 정보 처리와 샘플링 정책은 구현 시점에 정의 가능한 범위.

---

## 3단계 — 횡단 검증

### 3.1 데이터 모델 정합성

세 작업 모두 기존 `AnalysisResult` 패턴(summary/series/tables/metadata)을 따른다. 새로운 type은:
- `timeline_correlation`: 다른 결과를 입력으로 받는 **2차 분석기(meta-analyzer)** 패턴
- `jfr_recording`: 단일 파일 → 결과 변환 (기존 access_log/profiler_collapsed와 동일)
- `otel_log`: 단일 파일/파일셋 → 결과 변환

**긍정적**: 세 shape 모두 `evidence_ref` 필드를 공유하여 원본 증거 추적 가능. 이 일관성은 Timeline Correlation이 자연스럽게 각 결과를 소비할 수 있게 한다.

**문제**: `timeline_correlation`이 다른 `AnalysisResult`를 입력으로 받는 모델이지만, 이 입력 인터페이스가 정의되지 않았다. 단일 파일 경로인지, 복수 결과 JSON인지, 메모리 내 결과 객체인지 불명확.

### 3.2 시간 축 처리

| 데이터 소스 | timestamp 표현 | 정밀도 | 시간대 |
|---|---|---|---|
| access_log | ISO 8601 문자열 | 초 단위 | 로컬 시간대 (로그 원본 의존) |
| profiler_collapsed | 없음 (샘플 카운트 기반) | 해당 없음 | 해당 없음 |
| jfr_recording | `time` 필드 (형식 미정의) | 미정의 | 미정의 |
| otel_log | OTel spec: uint64 나노초 | 나노초 | UTC |
| timeline_correlation | `time` 필드 (형식 미정의) | 미정의 | 미정의 |

**심각한 정합성 갭**: 시간 축 표현이 **통일되지 않았다**. OTel은 나노초 UTC, access_log는 로컬 시간대 초 단위, profiler_collapsed는 시간 축 자체가 없고, JFR과 timeline_correlation은 형식조차 미정의.

Timeline Correlation의 핵심 전제는 "서로 다른 소스의 이벤트를 시간 축에 배치"하는 것이다. **공통 timestamp 정규화 규칙 없이는 correlation이 불가능**하다.

| # | 심각도 | 발견 | 권고 |
|---|---|---|---|
| F-019 | **High** | 공통 timestamp 정규화 정책 부재. 세 결정 문서 어디에도 시간 표현 통일 규칙이 없음. 이것은 향후 구현 시 마찰이 아니라 **아키텍처 결함** | `ADVANCED_DIAGNOSTICS.md`에 공통 시간 표현 규칙 추가: ISO 8601 UTC, 밀리초 이상 정밀도, 나노초 보존 옵션. 모든 result type의 time 필드가 이 규칙을 따르도록 정의 |

### 3.3 식별자 정합성

| 식별자 | OTel | JFR | Timeline Correlation |
|---|---|---|---|
| trace_id | ✅ (16바이트 hex) | ❌ (JFR에는 trace context 없음) | ✅ (evidence_links에 포함) |
| span_id | ✅ (8바이트 hex) | ❌ | ✅ (evidence_links에 포함) |
| thread_id/name | `Resource`에 가능 | ✅ (JFR 이벤트의 핵심 필드) | ❌ (shape에 미포함) |
| event_type | `EventName` | JFR event type | `event_type` 필드 |

**핵심 관찰**: JFR과 OTel 사이에 **직접적인 공통 식별자가 없다**. JFR에는 trace_id/span_id가 없고, OTel 로그에는 JFR의 thread ID 개념이 Resource 내부에 매핑될 뿐이다. 두 데이터를 연결할 수 있는 유일한 키는 **timestamp + thread name/id** 조합의 근사 매칭뿐인데, 이에 대한 전략이 문서에 없다.

| # | 심각도 | 발견 | 권고 |
|---|---|---|---|
| F-020 | **High** | JFR↔OTel 결합 키 전략 부재. trace_id는 JFR에 없고, thread_id는 OTel에서 선택적. 시간+스레드 근사 매칭이 유일한 방법이나 이에 대한 설계가 없음 | Timeline Correlation 규칙에 결합 키 계층 명시: (1) trace_id 정확 매칭 (OTel↔access_log), (2) timestamp+thread 근사 매칭 (JFR↔OTel), (3) timestamp 범위 기반 추론 (confidence 부여) |

### 3.4 차별화 비전의 일관성

세 문서를 통합 독해한 결과:

**통합 비전이 **암묵적으로**는 존재한다**: JFR(JVM 런타임 진단) + OTel 로그(분산 추적 컨텍스트) + Timeline Correlation(시간 축 상관관계) → "단일 데스크톱 도구에서 JVM 런타임 이벤트와 분산 추적 로그를 시간 축에 겹쳐 보여주는" 비전.

그러나 이 비전이 **명시적으로 서술되어 있지 않다**. 세 섹션을 각각 읽으면 독립된 결정처럼 읽히며, "이 세 가지가 결합되면 어떤 진단 시나리오가 가능해지는가"에 대한 서술이 없다.

| # | 심각도 | 발견 | 권고 |
|---|---|---|---|
| F-021 | **High** | 통합 비전 서술 부재. 세 섹션이 독립 결정으로 읽힘. "JFR + OTel + Timeline Correlation = 어떤 진단 시나리오"라는 통합 서사가 없음 | `ADVANCED_DIAGNOSTICS.md` 서두에 통합 비전 섹션 추가: (1) 목표 시나리오 (예: "GC 멈춤 중 어떤 HTTP 요청이 영향받았는지 OTel trace로 추적"), (2) 데이터 흐름 다이어그램, (3) 경쟁 도구 대비 차별화 포인트 |

### 3.5 이전 페이즈 산출물과의 연결

| 연결점 | 평가 |
|---|---|
| Phase 3 T-027 스택 분류 → JFR/OTel 적용 가능성 | JFR 이벤트는 스택 트레이스를 포함하므로 기존 `classify_stack` 적용 가능. 그러나 JFR shape에 `category` 필드가 없음 — 향후 연결점이 누락 |
| Phase 3 T-022 PyInstaller 사이드카 → JFR 파서 수용 | JDK `jfr` CLI 접근은 사이드카와 무관. 사용자 JDK에 의존. 이 전제가 명시되어 있지 않음 |
| Phase 1 `AnalysisResult` 공통 계약 | 세 shape 모두 기존 패턴 준수. 통합 가능성 양호 |

---

## 4단계 — Phase 5 진입 적합성 평가

### 판정: ⚠️ 방향은 잡혔으나 일부 세부 결정이 더 필요

| 작업 | Phase 5 진입 적합성 | 근거 |
|---|---|---|
| T-028 | ⚠️ 보강 필요 | Shape은 우수하나 차별화 가설 미비, timestamp 통일 규칙 부재, confidence 필드 불일치 |
| T-034 | ❌ 추가 작업 필요 | PoC 부재, 대안 비교 부재, JDK 버전/라이선스 미정의. 구현 착수 시 첫 번째 작업이 "스파이크 재수행"이 될 것 |
| T-035 | ✅ 즉시 구현 착수 가능 | OTel 사양 정확 준수, shape 설계 양호, 기존 파서 패턴과 일관 |

### 선행 보강 항목

Phase 5(AI-Assisted Interpretation) 진입 전, 다음이 해결되어야 한다:

1. **공통 timestamp 정규화 정책** 정의 (F-019)
2. **JFR 파서 PoC** 수행: 실제 JFR 파일 → `jfr print --json` → Python 로드 → AnalysisResult 변환 (F-005)
3. **JFR↔OTel 결합 키 전략** 정의 (F-020)
4. **통합 비전 서술** 추가 (F-021)

항목 1, 3, 4는 문서 작업이므로 비용이 낮다. 항목 2는 코드 작업이지만 범위가 명확하다(1-2일 스파이크).

---

## 5단계 — 외부 리서치 및 시장 위치 검증

### 5.1 JFR 생태계 현황 (2025-2026)

**JEP 509: JFR CPU-Time Profiling (Experimental)**
- JDK 25에 포함 (2025년 9월 GA)
- `jdk.CPUTimeSample` 이벤트: SIGPROF 기반 CPU 시간 샘플링
- 기본 throttle 500/s, profiling 설정에서 10ms
- Linux 전용 (실험적)
- **ArchScope에 대한 시사점**: 기존 `jdk.ExecutionSample`(wall-clock 기반)과 함께 CPU-time 기반 프로파일링이 JFR에 추가됨. T-034의 우선 이벤트 목록에 반영 필요

**JDK 26 EA `jfr print --exact`**
- 타임스탬프와 숫자를 전체 정밀도로 출력
- 다른 소스와의 상관관계(correlation) 검증에 유용
- **ArchScope에 대한 시사점**: Timeline Correlation에 필요한 정밀 타임스탬프 획득 경로

**JFR 파서 생태계**:
- 순수 Python JFR 파서: 성숙한 라이브러리 **없음** (2026년 4월 기준)
- Grafana `jfr-parser` (Go): Pyroscope에서 사용, 활발히 유지보수
- Java `jdk.jfr.consumer` API: 가장 완전하나 JVM 필요
- JDK `jfr` CLI: 외부 도구로서 가장 안정적이고 접근성 높음

**결론**: JDK `jfr` CLI 선택은 현재 생태계에서 **타당한 초기 결정**이다. 다만 이 결론에 도달한 비교 과정이 문서에 없다.

### 5.2 OpenTelemetry Logs 사양 현황 (2025-2026)

**OTel Logs Data Model: Stable**
- 핵심 필드(Timestamp, TraceId, SpanId, SeverityNumber/Text, Body, Resource, Attributes, EventName)가 안정화
- T-035의 필드 매핑은 현재 사양과 정확히 일치

**OTel Profiles Signal: Alpha (2026년 3월)**
- 프로파일링이 OTel의 4번째 신호(signal)로 추가됨
- `trace_id`/`span_id`를 통한 프로파일-트레이스 상관관계 지원
- pprof/JFR 포맷과의 왕복 변환 가능
- **ArchScope에 대한 시사점**: OTel Profiles가 안정화되면 JFR과 OTel을 trace_id로 직접 연결하는 경로가 열림. 이는 F-020(JFR↔OTel 결합 키 부재)의 장기 해법이 될 수 있음

**로그-트레이스 상관관계 베스트 프랙티스**:
- `trace_id` 정확 매칭이 최우선
- Span Profiles(Grafana Pyroscope)가 trace와 프로파일을 스팬 수준에서 연결
- 업계 표준은 trace_id → span_id → profile sample의 계층 매핑

### 5.3 Timeline Correlation 기능 비교

| 도구 | 접근 방식 | 강점 | 한계 |
|---|---|---|---|
| **Datadog APM** | Continuous Profiler Timeline View + Trace 연계 | 스팬에서 프로파일 데이터로 직접 이동. 스레드/goroutine/이벤트 루프별 시간 분해. 7개 언어 지원 | SaaS 전용. 에이전트 설치 필요. 오프라인/데스크톱 분석 불가 |
| **Grafana Pyroscope** | Span Profiles + Trace 연계 | OTel 표준 기반 trace↔profile 상관관계. Pyroscope 2.0(2025) 이후 대규모 처리 입증 | Grafana 생태계 의존. 실시간 수집 기반(파일 분석 아님) |
| **JProfiler** | 스레드 타임라인 + 모니터 상관관계 | 스레드별 색상 타임라인, 모니터 ID로 잠금 상관관계. JProfiler 16(2026.02)에서 에이전틱 Java 프로파일링 추가 | JVM 전용. 라이브 연결 또는 스냅샷 기반. 분산 추적 미지원 |
| **IntelliJ Profiler** | async-profiler 기반 flamegraph + 타임라인 | IDE 통합, 설정 없음. CPU 시간 flame graph, 메서드 호출 트리 | JVM 전용. IDE 종속. 오프라인 파일 분석 제한적 |
| **New Relic** | JFR 메트릭 기반 실시간 프로파일링 + APM 연계 | JFR 이벤트를 실시간 수집하여 APM과 연계. 코드 수준 레이턴시 추적 | SaaS 전용. 에이전트 의존 |

#### ArchScope의 차별화 포지셔닝 분석

**기존 도구의 공통 한계**:
1. **SaaS 의존**: Datadog, New Relic, Grafana Cloud는 데이터 송출 필요. 보안 민감 환경에서 제약
2. **실시간 수집 전제**: 대부분 에이전트 기반 실시간 수집. 사후(post-mortem) 파일 분석 약함
3. **단일 런타임**: JProfiler, IntelliJ는 JVM 전용. 크로스 런타임 상관관계 미지원
4. **도구 분산**: 로그(ELK/Loki), 프로파일(Pyroscope), 추적(Tempo/Jaeger)이 별도 도구

**ArchScope의 잠재적 차별화**:
- **오프라인 데스크톱 분석**: 데이터 외부 송출 없이 로컬에서 파일 기반 분석
- **다중 진단 소스 통합**: access log + JFR + OTel 로그를 단일 Timeline에 시각화
- **증거 기반 상관관계**: `evidence_ref` 모델로 추론과 원본 증거를 분리

**그러나 이 차별화가 ADVANCED_DIAGNOSTICS.md에 서술되어 있지 않다.**

| # | 심각도 | 발견 | 권고 |
|---|---|---|---|
| F-022 | **High** | 차별화 포지셔닝이 문서화되지 않음. 위에서 식별한 "오프라인 + 다중 소스 + 증거 기반"이라는 차별화 가설이 문서에 없으면, 향후 구현 방향 결정 시 근거가 사라짐 | 문서 서두에 시장 위치 섹션 추가: 기존 도구의 한계, ArchScope의 접근 차이, 목표 사용자 (예: 보안 정책상 SaaS 사용 불가한 조직의 성능 엔지니어) |

### 5.4 결정 사항의 외부 표준 부합성

| 결정 | 외부 표준 | 부합 여부 |
|---|---|---|
| T-035 OTel 필드 매핑 | OTel Logs Data Model (Stable) | ✅ 정확 일치 |
| T-035 레거시 trace context | OTel Trace Context in Non-OTLP Logs | ✅ 필드명 규약 일치 |
| T-034 JFR CLI 접근 | JDK `jfr` 공식 사양 (Oracle) | ✅ 공식 도구 활용 |
| T-034 이벤트 선택 | JEP 509 (CPU-Time, JDK 25) | ❌ 미반영 |
| T-028 프로파일-트레이스 상관관계 | OTel Profiles Signal (Alpha, 2026.03) | 인지 부재 — 장기적으로 JFR↔trace 연결의 표준 경로가 될 가능성 |

---

## 6단계 — 신규 이슈 식별

| ID | 심각도 | 카테고리 | 설명 | 권고 조치 | 우선순위 |
|---|---|---|---|---|---|
| NI-001 | **High** | 스파이크 검증 | T-034 JFR 파서 스파이크에 PoC 코드 부재. `jfr print --json`으로 실제 JFR 파일 파싱 → Python 로드 → AnalysisResult 변환 최소 PoC 필요 | JFR PoC 스파이크 태스크 추가 | P1 |
| NI-002 | **High** | 데이터 모델 | 공통 timestamp 정규화 정책 부재. 서로 다른 소스의 시간 축 표현이 통일되지 않아 Timeline Correlation 구현 불가 | 공통 시간 표현 규칙 정의 (ISO 8601 UTC, ms/ns 정밀도) | P1 |
| NI-003 | **High** | 정합성 | JFR↔OTel 결합 키 전략 부재. trace_id는 JFR에 없고 thread_id는 OTel에서 선택적 | 결합 키 계층 정의: trace_id 정확 매칭 → timestamp+thread 근사 매칭 → 범위 기반 추론 | P1 |
| NI-004 | **High** | 비전 | 통합 차별화 비전 서술 부재. 세 결정이 독립적으로 읽히며, 경쟁 도구 대비 차별화 근거가 문서에 없음 | ADVANCED_DIAGNOSTICS.md 서두에 통합 비전 + 시장 위치 섹션 추가 | P1 |
| NI-005 | **High** | 스파이크 | T-034 라이브러리 대안 비교 부재. JDK CLI / Java API / Go 파서 / Python 파서 비교 테이블 필요 | 대안 비교 테이블 추가 (라이선스, JDK 호환성, 성능, Python 연동) | P1 |
| NI-006 | Medium | 사양 추적 | JEP 509 `jdk.CPUTimeSample` (JDK 25, 실험적) 미반영. JFR 진단 가치를 높이는 핵심 이벤트 | 우선 이벤트 목록에 추가 | P2 |
| NI-007 | Medium | 데이터 모델 | `timeline_correlation` shape의 `correlated_events`에 `confidence` 필드 누락. 규칙("mark with confidence")과 모델 불일치 | shape에 `confidence` 필드 추가 | P2 |
| NI-008 | Medium | 보안 | OTel 로그의 PII/민감 정보 처리 방침 부재 | evidence_ref 보존 범위 정책 추가 | P2 |
| NI-009 | Medium | 성능 | 대용량 JFR/OTel 파일의 메모리 사용 모델 미정의 | 점진적 로드/필터링 전략 기술 | P2 |
| NI-010 | Medium | 라이선스 | JDK `jfr` CLI 사용 전제 조건(사용자 JDK 설치 의존) 미명시 | ADVANCED_DIAGNOSTICS.md에 전제 조건 섹션 추가 | P2 |
| NI-011 | Medium | JDK 호환성 | 파서 JDK와 녹화 JDK의 버전 매칭 미정의. 최소 지원 JDK 버전 불명확 | 지원 JDK 범위 명시 | P2 |
| NI-012 | Medium | 사양 추적 | OTel Profiles Signal (Alpha, 2026.03) 미인지. 장기적으로 JFR↔trace 연결의 표준 경로 가능성 | 향후 재평가 트리거로 기록 | P3 |
| NI-013 | Minor | 데이터 모델 | JFR `notable_events`에 콜스택 표현 필드 누락. 기존 profiler_collapsed의 `frames: list[str]` 패턴 미적용 | `frames` 또는 `stack_trace` 필드 추가 | P3 |
| NI-014 | Minor | 데이터 모델 | `otel_log` series `trace_event_counts`의 고카디널리티 문제 | top-N 제한 또는 aggregate 전략 추가 | P3 |
| NI-015 | Minor | 문서 형식 | 결정 문서에 ADR(Architecture Decision Record) 형식 부재. Context/Decision/Alternatives/Consequences 구조가 없어 결정 추적이 어려움 | ADR 형식 또는 최소한 "고려된 대안" 섹션 추가 | P3 |

---

## 종합 평가

### 잘 된 부분

1. **AnalysisResult 구조적 일관성**: 세 shape 모두 기존 `summary`/`series`/`tables`/`metadata` 패턴을 정확히 따른다. T-010 이후 축적된 구조적 규율이 잘 적용됨
2. **증거 기반 설계 원칙**: `evidence_ref` 모델이 T-028, T-034, T-035 전체에 일관되게 적용됨. RD-024의 "원본 증거 요구" 원칙이 Phase 4 전체에 관통
3. **OTel 사양 준수 정확성**: T-035의 필드 매핑이 Stable 사양과 정확히 일치. 레거시 trace context 처리도 OTel 호환 가이드라인과 부합
4. **JDK CLI 접근의 아키텍처 일관성**: 기존 Python CLI → `execFile` 패턴과 일관된 외부 프로세스 호출 모델
5. **Timeline Correlation의 2차 분석기 모델**: 다른 결과를 입력으로 받는 meta-analyzer 패턴은 기존 파서→분석기→결과 파이프라인을 자연스럽게 확장

### 부족한 부분

1. **T-034 스파이크의 본질 결여**: 코드 검증 없는 "스파이크"는 스파이크가 아니다
2. **차별화 비전의 명시적 부재**: 가장 치명적. 이 Phase의 존재 이유인 "차별화"가 선언적 한 줄로만 존재
3. **시간 축 정합성 미해결**: Timeline Correlation의 핵심 전제인 시간 표현 통일이 정의되지 않음
4. **JFR↔OTel 결합 키의 공백**: 두 핵심 데이터 소스를 연결할 키 전략이 없음

### 작업별 최종 판정

| 작업 | 판정 | 요약 |
|---|---|---|
| T-028 | ⚠️ 조건부 PASS | Shape 우수, 차별화 근거 미비 |
| T-034 | ❌ 조건부 FAIL | 방향 타당, PoC/대안 비교 부재 |
| T-035 | ✅ PASS | OTel 사양 정확 준수, shape 양호 |

### Phase 5 진입 적합성

**⚠️ 방향은 잡혔으나 선행 보강 필요**

Phase 5(AI-Assisted Interpretation)는 안정적 결과 데이터 위에서 작동해야 한다. 현재 Phase 4의 결정 품질은:
- T-035(OTel): 즉시 구현 가능
- T-028(Timeline Correlation): timestamp 정규화와 결합 키 정의 후 구현 가능
- T-034(JFR): PoC 스파이크 재수행 필요

**권고**: T-034 PoC와 timestamp 정규화를 "Phase 4 보강" 단계로 수행한 뒤 Phase 5 진입

---

## 참조 출처

- [OTel Logs Data Model](https://opentelemetry.io/docs/specs/otel/logs/data-model/) — Stable 사양
- [OTel Trace Context in Non-OTLP Logs](https://opentelemetry.io/docs/specs/otel/compatibility/logging_trace_context/)
- [OTel Profiles Alpha 발표](https://opentelemetry.io/blog/2026/profiles-alpha/) — 2026년 3월
- [JDK 21 `jfr` 커맨드 사양](https://docs.oracle.com/en/java/javase/21/docs/specs/man/jfr.html)
- [JDK 26 EA `jfr` 커맨드 (--exact 추가)](https://download.java.net/java/early_access/jdk26/docs/specs/man/jfr.html)
- [JEP 509: JFR CPU-Time Profiling](https://openjdk.org/jeps/509) — JDK 25, 실험적
- [JFR 파일 포맷 분석 (Gunnar Morling)](https://www.morling.dev/blog/jdk-flight-recorder-file-format/)
- [Grafana jfr-parser (Go)](https://github.com/grafana/jfr-parser)
- [Datadog Continuous Profiler Timeline View](https://www.datadoghq.com/blog/continuous-profiler-timeline-view/)
- [Datadog Traces to Profiles](https://docs.datadoghq.com/profiler/connect_traces_and_profiles/)
- [Grafana Pyroscope Trace-Profile 통합](https://grafana.com/docs/pyroscope/latest/introduction/profile-tracing/)
- [JProfiler 스레드 프로파일링](https://www.ej-technologies.com/resources/jprofiler/help/doc/main/threads.html)
- [JProfiler 16: 에이전틱 Java 프로파일링 (2026.02)](https://www.ej-technologies.com/blog/2026/02/jprofiler-16-profiling-agentic-java-applications/)
- [New Relic JFR 메트릭 기반 실시간 프로파일링](https://docs.newrelic.com/docs/apm/agents/java-agent/features/real-time-profiling-java-using-jfr-metrics/)
