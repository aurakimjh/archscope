# 고급 진단 설계

Phase 4의 고급 진단은 evidence-first 원칙을 유지한다. Timeline correlation, JFR parsing, OpenTelemetry log input은 UI나 report interpretation 전에 raw evidence reference를 보존하고 `AnalysisResult`로 정규화해야 한다.

## Timeline Correlation

Timeline correlation은 JVM 및 multi-runtime 진단의 차별화 기능으로 유지한다. 첫 구현은 parser가 아니라 correlation analyzer로 둔다.

Input result type:

- `access_log`
- `profiler_collapsed`
- 향후 `gc_log`
- 향후 `thread_dump`
- 향후 `jfr_recording`
- 향후 `otel_log`

권장 output type:

```text
type: timeline_correlation
summary:
  window_start
  window_end
  correlated_event_count
  highest_severity
series:
  correlated_events: [{ time, source_type, event_type, severity, label, evidence_ref }]
tables:
  evidence_links: [{ evidence_ref, source_file, raw_line, raw_block, trace_id, span_id }]
metadata:
  schema_version
  input_result_types
  correlation_window_ms
```

규칙:

- Event timestamp를 primary join key로 사용한다.
- `trace_id`와 `span_id`가 있으면 더 강한 join key로 사용한다.
- Heuristic correlation은 confidence와 evidence reference를 함께 표시한다.
- 연결된 evidence 없이 prose conclusion을 생성하지 않는다.

## JFR Recording Parser Spike

초기 feasibility path는 native Python JFR parser를 바로 채택하지 않고 JDK `jfr` command를 external parser bridge로 사용하는 것이다. JDK 도구는 `jfr print --json`, `jfr metadata`, `jfr summary`를 지원하므로 spike 단계에서 안정적인 command boundary를 제공한다.

권장 parser flow:

1. 선택된 `.jfr` 파일이 읽을 수 있는지 확인한다.
2. `jfr summary`로 event count와 recording metadata를 수집한다.
3. 제한된 event set에 대해 `jfr print --json --stack-depth <n>`을 실행한다.
4. 선택한 event를 `jfr_recording` `AnalysisResult`로 정규화한다.
5. Event type, timestamp, duration, thread, stack frame, raw JSON preview를 보존한다.

초기 event focus:

- GC pause 및 heap event
- CPU/profile execution sample
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
  events_over_time: [{ time, event_type, count }]
  pause_events: [{ time, duration_ms, event_type, thread }]
tables:
  notable_events: [{ time, event_type, duration_ms, thread, message, evidence_ref }]
metadata:
  parser: jdk_jfr_command
  schema_version
  jfr_command_version
  event_filters
```

Deferred decision: native parser library 선택은 command-based spike로 실제로 유용한 JFR event와 payload shape가 확인된 뒤 진행한다.

## OpenTelemetry Log Input

OpenTelemetry log input은 먼저 OTLP-style JSON을 받고, 이후 trace context field를 포함한 legacy JSON/plain text format으로 확장한다. OpenTelemetry logs data model은 timestamp, trace context, severity, body, resource, scope, attributes, event name field를 정의한다.

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
  logs_over_time: [{ time, severity, count }]
  trace_event_counts: [{ trace_id, count }]
tables:
  log_records: [{ time, severity, trace_id, span_id, service_name, body, evidence_ref }]
metadata:
  parser: otel_log_json
  schema_version
  accepted_formats
```

Correlation 규칙:

- Access log, OTel log, future trace/span input 사이의 join은 정확한 `trace_id`를 우선한다.
- `span_id`만 있고 `trace_id`가 없으면 record는 보존하되 trace-joinable하지 않은 것으로 표시한다.
- Legacy JSON field인 `trace_id`, `span_id`, `trace_flags`는 OpenTelemetry trace context로 매핑한다.
- 원본 log body 또는 raw JSON preview를 evidence로 보존한다.

## 참고 자료

- Oracle JDK `jfr` command: https://docs.oracle.com/en/java/javase/21/docs/specs/man/jfr.html
- OpenTelemetry logs data model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
- OpenTelemetry trace context in non-OTLP logs: https://opentelemetry.io/docs/specs/otel/compatibility/logging_trace_context/
