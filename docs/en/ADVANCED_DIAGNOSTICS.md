# Advanced Diagnostics Design

Phase 4 keeps advanced diagnostics evidence-first. Timeline correlation, JFR parsing, and OpenTelemetry log input should preserve raw evidence references and normalize into `AnalysisResult` before UI or report interpretation.

## Integrated Vision

ArchScope's advanced-diagnostics thesis is offline, multi-source, evidence-linked correlation. Existing APM/profiler products are strong when agents continuously ship data to a SaaS or observability backend. ArchScope should serve the operator who receives files after an incident, cannot upload production evidence, and still needs to answer: which requests, JVM runtime events, stack samples, and logs lined up in the same failure window?

Target scenario:

```text
access_log + JFR recording + OpenTelemetry logs
  -> normalized AnalysisResult files
  -> timeline_correlation meta-analyzer
  -> multi-lane evidence timeline
  -> report/AI interpretation with evidence_ref links
```

Differentiation points:

- local desktop workflow without mandatory SaaS ingestion
- post-mortem file analysis rather than only live-agent telemetry
- cross-source correlation across access logs, JVM runtime events, profiler stacks, and trace-aware logs
- explicit evidence references that separate facts from heuristic interpretation

Implementation trigger: start implementation-heavy correlation only after at least two production-quality time-bearing result types exist. The first practical pair should be `access_log` plus `otel_log`, followed by `jfr_recording` once the JFR command bridge is validated.

## Common Time Policy

All advanced result types must normalize timeline fields to ISO 8601 UTC strings.

Rules:

- `time`, `window_start`, and `window_end` use ISO 8601 UTC with a trailing `Z`.
- Preserve millisecond precision at minimum.
- Preserve nanosecond precision in an optional `time_unix_nano` field when the source provides it, such as OpenTelemetry.
- Record the source timezone or timezone assumption under `metadata.timezone` or `metadata.time_policy`.
- Access logs without explicit timezone must use the parser's configured timezone before UTC normalization.
- Relative-only sources, such as collapsed profiler stacks, are not timeline-correlation inputs until an anchor timestamp is supplied.

## Timeline Correlation

Timeline correlation is a meta-analyzer. It consumes normalized `AnalysisResult` JSON files or in-memory typed results, not raw source logs.

Input result types:

- `access_log`
- `otel_log`
- future `jfr_recording`
- future `gc_log`
- future `thread_dump`
- future anchored profiler results

Recommended output type:

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

1. Exact `trace_id` plus optional `span_id` join for OTel, access logs with trace context, and future trace/profile inputs.
2. Timestamp plus thread id/name approximate join for JFR to OTel or thread-dump evidence.
3. Timestamp-window proximity join for sources without shared identifiers.

Correlation confidence:

- `1.0`: exact trace/span join.
- `0.7` to `0.9`: timestamp plus thread join inside `clock_drift_ms`.
- below `0.7`: timestamp-only heuristic join.

The default `clock_drift_ms` should be conservative, initially 5000 ms, and must be echoed in metadata.

## JFR Recording Parser Spike

Initial feasibility path: use the JDK `jfr` command as an external parser bridge before adopting a native parser. The minimal PoC accepts JSON emitted by `jfr print --json` and normalizes it to a draft `jfr_recording` result. Raw `.jfr` execution should be added after local JDK discovery and packaging behavior are settled.

Parser alternatives:

| Option | Strength | Risk / cost | Initial decision |
|---|---|---|---|
| JDK `jfr` CLI | Official tool, supports JSON output, low parser risk | Requires a user or bundled JDK with `jfr`; large JSON output needs streaming | Use first |
| Java `jdk.jfr.consumer` API | Complete programmatic access | Requires Java sidecar or embedding JVM logic | Defer |
| Grafana Go `jfr-parser` | Proven in profile tooling | Adds Go binary/toolchain dependency and license review | Defer |
| Pure Python parser | Single-language engine | JFR binary format complexity and weak mature-library availability | Reject for first spike |

JDK assumptions:

- ArchScope should initially depend on the user's installed JDK command rather than redistributing a JDK.
- Minimum parser JDK target: JDK 21 for the first documented support path.
- Recording compatibility must be tested with JDK 17, 21, and 25 recordings before release claims.
- JDK 25 `jdk.CPUTimeSample` is experimental but should be included in event selection because CPU-time profiling improves diagnostic value.

Recommended parser flow:

1. Verify the selected `.jfr` file is readable.
2. Run `jfr summary` to collect event counts and basic recording metadata.
3. Run `jfr print --json --events <filters> --stack-depth <n>` for a bounded event set.
4. Stream or incrementally process the JSON output instead of loading very large outputs at once.
5. Normalize selected events into a `jfr_recording` `AnalysisResult`.
6. Preserve event type, timestamp, duration, thread id/name, stack frames, sampled/exact semantics, and raw JSON preview.

Initial event focus:

- `jdk.CPUTimeSample` (JDK 25+, experimental, sampled)
- `jdk.ExecutionSample` (sampled)
- GC pause and heap events, such as `jdk.GarbageCollection` and `jdk.GCPhasePause`
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

OpenTelemetry log input should accept OTLP-style JSON first, then legacy JSON/plain text formats that carry trace context fields. The design baseline is the OpenTelemetry Logs Data Model checked on 2026-04-30.

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

- Store bounded `body_preview` and `raw_preview` by default.
- Keep full `Body` and `Attributes` only behind an explicit option.
- Redact likely secrets from previews, including tokens, authorization headers, passwords, and API keys.
- Record whether redaction ran under `metadata.privacy_policy`.

OTel Profiles is not a Phase 5 blocker while the signal remains early. Reevaluate when Profiles reaches a stable signal level or when profile-to-trace files become common in customer evidence.

## Large-File Strategy

- JFR: use `--events`, `--categories`, and `--stack-depth` to constrain `jfr print --json`; move to streaming JSON parsing if real outputs exceed memory budgets.
- OTel logs: reuse access-log controls such as `max_lines`, `start_time`, and `end_time`.
- High-cardinality series such as `trace_event_counts` must be capped with top-N plus "other" aggregation.
- Tables should keep bounded evidence rows and store counts in `summary` or `series`.

## Multi-Lane Timeline Visualization

The `timeline_correlation` UI should render an ECharts custom or timeline-like chart with one lane per source family:

- access log requests/errors
- OTel logs/traces
- JFR runtime events
- profiler/thread/GC evidence as they become available

Chart requirements:

- stable vertical lanes by `source_type`
- event markers colored by severity
- deterministic joins displayed with stronger visual linking than heuristic joins
- hover/tooltips showing `evidence_ref`, join strategy, confidence, and timestamp
- click-through to evidence table rows

## References

- Oracle JDK `jfr` command: https://docs.oracle.com/en/java/javase/21/docs/specs/man/jfr.html
- OpenTelemetry logs data model: https://opentelemetry.io/docs/specs/otel/logs/data-model/
- OpenTelemetry trace context in non-OTLP logs: https://opentelemetry.io/docs/specs/otel/compatibility/logging_trace_context/
