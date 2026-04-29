# 고급 진단 설계

Phase 4의 고급 진단은 evidence-first 원칙을 유지한다. Timeline correlation, JFR parsing, OpenTelemetry log input은 UI나 report interpretation 전에 raw evidence reference를 보존하고 `AnalysisResult`로 정규화해야 한다.

## 통합 비전

ArchScope의 advanced-diagnostics thesis는 offline, multi-source, evidence-linked correlation이다. 기존 APM/profiler 제품은 agent가 데이터를 SaaS 또는 observability backend로 지속 전송할 때 강하다. ArchScope는 사고 이후 파일을 전달받고, production evidence를 외부로 업로드할 수 없으며, 같은 장애 시간대에 어떤 request, JVM runtime event, stack sample, log가 겹쳤는지 답해야 하는 사용자를 대상으로 한다.

Target scenario:

```text
access_log + JFR recording + OpenTelemetry logs
  -> normalized AnalysisResult files
  -> timeline_correlation meta-analyzer
  -> multi-lane evidence timeline
  -> report/AI interpretation with evidence_ref links
```

차별화 지점:

- mandatory SaaS ingestion 없는 local desktop workflow
- live-agent telemetry만이 아니라 post-mortem file analysis 지원
- access log, JVM runtime event, profiler stack, trace-aware log를 cross-source correlation
- fact와 heuristic interpretation을 분리하는 explicit evidence reference

Implementation trigger: production-quality time-bearing result type이 최소 2개 준비된 뒤 implementation-heavy correlation에 착수한다. 첫 practical pair는 `access_log`와 `otel_log`이고, JFR command bridge가 검증된 뒤 `jfr_recording`을 추가한다.

## 공통 시간 정책

모든 advanced result type은 timeline field를 ISO 8601 UTC string으로 정규화해야 한다.

규칙:

- `time`, `window_start`, `window_end`는 trailing `Z`를 포함한 ISO 8601 UTC를 사용한다.
- 최소 millisecond precision을 보존한다.
- OpenTelemetry처럼 source가 제공하는 경우 optional `time_unix_nano` field로 nanosecond precision을 보존한다.
- Source timezone 또는 timezone assumption은 `metadata.timezone` 또는 `metadata.time_policy`에 기록한다.
- 명시적 timezone이 없는 access log는 parser configured timezone을 적용한 뒤 UTC로 정규화한다.
- collapsed profiler stack처럼 relative-only source는 anchor timestamp가 제공되기 전까지 timeline-correlation input이 아니다.

## Timeline Correlation

Timeline correlation은 meta-analyzer이다. Raw source log가 아니라 normalized `AnalysisResult` JSON file 또는 in-memory typed result를 소비한다.

Input result type:

- `access_log`
- `otel_log`
- 향후 `jfr_recording`
- 향후 `gc_log`
- 향후 `thread_dump`
- 향후 anchored profiler result

권장 output type:

```text
type: timeline_correlation
summary:
  window_start
  window_end
  correlated_event_count
  highest_severity
series:
  correlated_events: [
    {
      time,
      time_unix_nano,
      source_type,
      event_type,
      severity,
      label,
      evidence_ref,
      confidence,
      join_strategy,
      thread_id,
      thread_name
    }
  ]
tables:
  evidence_links: [
    {
      evidence_ref,
      source_file,
      raw_line,
      raw_block,
      raw_preview,
      trace_id,
      span_id,
      thread_id,
      thread_name
    }
  ]
metadata:
  schema_version
  input_result_types
  correlation_window_ms
  clock_drift_ms
```

Join-key hierarchy:

1. OTel, trace context가 있는 access log, future trace/profile input에는 exact `trace_id`와 optional `span_id` join을 사용한다.
2. JFR과 OTel 또는 thread-dump evidence에는 timestamp plus thread id/name approximate join을 사용한다.
3. Shared identifier가 없는 source에는 timestamp-window proximity join을 사용한다.

Correlation confidence:

- `1.0`: exact trace/span join.
- `0.7` to `0.9`: `clock_drift_ms` 안의 timestamp plus thread join.
- below `0.7`: timestamp-only heuristic join.

기본 `clock_drift_ms`는 보수적으로 5000 ms에서 시작하고 metadata에 반드시 echo한다.

## JFR Recording Parser Spike

초기 feasibility path는 native parser를 바로 채택하지 않고 JDK `jfr` command를 external parser bridge로 사용하는 것이다. 최소 PoC는 `jfr print --json`이 낸 JSON을 입력으로 받아 draft `jfr_recording` result로 정규화한다. Raw `.jfr` 실행은 local JDK discovery와 packaging behavior가 안정화된 뒤 추가한다.

Parser alternatives:

| Option | Strength | Risk / cost | Initial decision |
|---|---|---|---|
| JDK `jfr` CLI | Official tool, JSON output 지원, parser risk 낮음 | user 또는 bundled JDK의 `jfr` 필요, 큰 JSON output은 streaming 필요 | Use first |
| Java `jdk.jfr.consumer` API | Complete programmatic access | Java sidecar 또는 JVM embedding 필요 | Defer |
| Grafana Go `jfr-parser` | Profile tooling에서 검증됨 | Go binary/toolchain dependency 및 license review 필요 | Defer |
| Pure Python parser | Single-language engine | JFR binary format complexity와 mature library 부족 | Reject for first spike |

JDK assumptions:

- 초기에는 JDK를 재배포하지 않고 사용자의 installed JDK command에 의존한다.
- 첫 documented support path의 parser JDK target은 JDK 21이다.
- Release claim 전 JDK 17, 21, 25 recording과의 compatibility를 테스트한다.
- JDK 25 `jdk.CPUTimeSample`은 experimental이지만 CPU-time profiling의 진단 가치가 크므로 event selection에 포함한다.

권장 parser flow:

1. 선택된 `.jfr` 파일이 읽을 수 있는지 확인한다.
2. `jfr summary`로 event count와 recording metadata를 수집한다.
3. 제한된 event set에 대해 `jfr print --json --events <filters> --stack-depth <n>`을 실행한다.
4. 매우 큰 output은 한 번에 load하지 않고 stream 또는 incremental 처리한다.
5. 선택한 event를 `jfr_recording` `AnalysisResult`로 정규화한다.
6. Event type, timestamp, duration, thread id/name, stack frame, sampled/exact semantics, raw JSON preview를 보존한다.

초기 event focus:

- `jdk.CPUTimeSample` (JDK 25+, experimental, sampled)
- `jdk.ExecutionSample` (sampled)
- `jdk.GarbageCollection`, `jdk.GCPhasePause` 같은 GC pause 및 heap event
- thread park/block/sleep event
- exception 및 error
- socket/file I/O latency event

권장 output type:

```text
type: jfr_recording
summary:
  event_count
  duration_ms
  gc_pause_total_ms
  blocked_thread_events
series:
  events_over_time: [{ time, time_unix_nano, event_type, count }]
  pause_events: [{ time, time_unix_nano, duration_ms, event_type, thread, sampling_type }]
tables:
  notable_events: [
    { time, event_type, duration_ms, thread, message, frames, sampling_type, evidence_ref }
  ]
metadata:
  parser: jdk_jfr_print_json
  schema_version
  jfr_command_version
  parser_jdk_version
  event_filters
  stack_depth
```

## OpenTelemetry Log Input

OpenTelemetry log input은 먼저 OTLP-style JSON을 받고, 이후 trace context field를 포함한 legacy JSON/plain text format으로 확장한다. Design baseline은 2026-04-30에 확인한 OpenTelemetry Logs Data Model이다.

권장 input mapping:

| OpenTelemetry field | ArchScope field |
|---|---|
| `Timestamp` 또는 `ObservedTimestamp` | event time |
| `TraceId` / `trace_id` | `trace_id` |
| `SpanId` / `span_id` | `span_id` |
| `TraceFlags` / `trace_flags` | `trace_flags` |
| `SeverityText` | severity label |
| `SeverityNumber` | normalized severity |
| `Body` | message/body |
| `Resource` | service/runtime metadata |
| `InstrumentationScope` | instrumentation scope metadata |
| `Attributes` | structured event attributes |
| `EventName` | event type |

권장 output type:

```text
type: otel_log
summary:
  log_count
  error_count
  trace_linked_count
  service_count
series:
  logs_over_time: [{ time, time_unix_nano, severity, count }]
  trace_event_counts: [{ trace_id, count }]
tables:
  log_records: [{ time, severity, trace_id, span_id, service_name, body_preview, evidence_ref }]
metadata:
  parser: otel_log_json
  schema_version
  accepted_formats
  privacy_policy
```

Privacy and evidence retention:

- 기본적으로 bounded `body_preview`와 `raw_preview`만 저장한다.
- Full `Body`와 `Attributes`는 explicit option 뒤에만 유지한다.
- Token, authorization header, password, API key 같은 likely secret은 preview에서 redact한다.
- Redaction 수행 여부는 `metadata.privacy_policy`에 기록한다.

OTel Profiles는 signal이 early 상태인 동안 Phase 5 blocker가 아니다. Profiles가 stable signal level에 도달하거나 profile-to-trace file이 고객 evidence에서 일반화되면 재평가한다.

## Large-File Strategy

- JFR: `--events`, `--categories`, `--stack-depth`로 `jfr print --json` 범위를 제한하고, 실제 output이 memory budget을 넘으면 streaming JSON parsing으로 이동한다.
- OTel logs: access-log control과 같은 `max_lines`, `start_time`, `end_time`을 재사용한다.
- `trace_event_counts` 같은 high-cardinality series는 top-N plus "other" aggregation으로 제한한다.
- Table은 bounded evidence row를 유지하고 count는 `summary` 또는 `series`에 저장한다.

## Multi-Lane Timeline Visualization

`timeline_correlation` UI는 source family별 lane을 가진 ECharts custom 또는 timeline-like chart로 렌더링한다.

- access log requests/errors
- OTel logs/traces
- JFR runtime events
- profiler/thread/GC evidence as they become available

Chart requirements:

- `source_type`별 stable vertical lane
- severity 기반 event marker 색상
- deterministic join을 heuristic join보다 강한 visual link로 표시
- hover/tooltip에 `evidence_ref`, join strategy, confidence, timestamp 표시
- evidence table row로 click-through

## 참고 자료

- Oracle JDK `jfr` command: https://docs.oracle.com/en/java/javase/21/docs/specs/man/jfr.html
- OpenTelemetry logs data model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
- OpenTelemetry trace context in non-OTLP logs: https://opentelemetry.io/docs/specs/otel/compatibility/logging_trace_context/
