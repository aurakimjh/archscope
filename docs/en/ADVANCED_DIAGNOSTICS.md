# Advanced Diagnostics Design

Phase 4 keeps advanced diagnostics evidence-first. Timeline correlation, JFR parsing, and OpenTelemetry log input should preserve raw evidence references and normalize into `AnalysisResult` before UI or report interpretation.

## Timeline Correlation

Timeline correlation remains a planned differentiator for JVM and multi-runtime diagnostics. The first implementation should be a correlation analyzer, not a parser.

Input result types:

- `access_log`
- `profiler_collapsed`
- future `gc_log`
- future `thread_dump`
- future `jfr_recording`
- future `otel_log`

Recommended output type:

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

Rules:

- Use event timestamps as the primary join key.
- Use `trace_id` and `span_id` as stronger joins when available.
- Keep heuristic correlations marked with confidence and evidence references.
- Do not generate prose conclusions without linked evidence.

## JFR Recording Parser Spike

Initial feasibility path: use the JDK `jfr` command as an external parser bridge before adopting a native Python JFR parser. The JDK tool supports `jfr print --json`, `jfr metadata`, and `jfr summary`, which gives ArchScope a stable command boundary for the spike.

Recommended parser flow:

1. Verify the selected `.jfr` file is readable.
2. Run `jfr summary` to collect event counts and basic recording metadata.
3. Run `jfr print --json --stack-depth <n>` for a bounded event set.
4. Normalize selected events into a `jfr_recording` `AnalysisResult`.
5. Preserve event type, timestamp, duration, thread, stack frames, and raw JSON preview.

Initial event focus:

- GC pause and heap events
- CPU/profile execution samples
- thread park/block/sleep events
- exceptions and errors
- socket/file I/O latency events

Recommended output type:

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

Deferral: a native parser library decision should wait until the command-based spike proves which JFR events and payload shapes are most useful.

## OpenTelemetry Log Input

OpenTelemetry log input should accept OTLP-style JSON first, then legacy JSON/plain text formats that carry trace context fields. The OpenTelemetry logs data model defines top-level timestamp, trace context, severity, body, resource, scope, attributes, and event name fields.

Recommended input mapping:

| OpenTelemetry field | ArchScope field |
|---|---|
| `Timestamp` or `ObservedTimestamp` | event time |
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

Recommended output type:

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

Correlation rules:

- Prefer exact `trace_id` joins across access logs, OTel logs, and future trace/span inputs.
- If `span_id` is present without `trace_id`, keep the record but mark it as not trace-joinable.
- Map legacy JSON fields named `trace_id`, `span_id`, and `trace_flags` to OpenTelemetry trace context.
- Preserve original log body or raw JSON preview as evidence.

## References

- Oracle JDK `jfr` command: https://docs.oracle.com/en/java/javase/21/docs/specs/man/jfr.html
- OpenTelemetry logs data model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
- OpenTelemetry trace context in non-OTLP logs: https://opentelemetry.io/docs/specs/otel/compatibility/logging_trace_context/
