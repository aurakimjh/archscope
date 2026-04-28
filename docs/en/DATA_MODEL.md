# Data Model

ArchScope normalizes all parser outputs into shared analysis structures. This lets the UI, chart templates, and exporters work against stable result contracts.

## Common Result Model

```text
AnalysisResult
  type
  source_files
  created_at
  summary
  series
  tables
  charts
  metadata
```

### Field Purpose

- `type`: Diagnostic result type, such as `access_log` or `profiler_collapsed`.
- `source_files`: Source file paths used to produce the result.
- `created_at`: ISO 8601 timestamp.
- `summary`: High-level metrics suitable for cards and executive summaries.
- `series`: Chart-ready time series or distributions.
- `tables`: Report-ready tabular data.
- `charts`: Optional chart template references and rendering metadata.
- `metadata`: Parser format, runtime, time zone, intervals, and other context.

## AccessLogRecord

```text
timestamp
method
uri
status
response_time_ms
bytes_sent
client_ip
user_agent
referer
raw_line
```

## GcEvent

```text
timestamp
uptime_sec
gc_type
cause
pause_ms
heap_before_mb
heap_after_mb
heap_committed_mb
young_before_mb
young_after_mb
old_before_mb
old_after_mb
metaspace_before_mb
metaspace_after_mb
raw_line
```

## ProfileStack

```text
stack
frames
samples
estimated_seconds
sample_ratio
elapsed_ratio
category
```

## ThreadDumpRecord

```text
thread_name
thread_id
state
stack
lock_info
category
raw_block
```

## ExceptionRecord

```text
timestamp
language
exception_type
message
root_cause
stack
signature
raw_block
```

## Design Rules

- Parsers preserve raw evidence where practical through `raw_line` or `raw_block`.
- Analyzers produce numeric fields with explicit units.
- Chart inputs come from `series` and `tables`, not parser-specific objects.
- Runtime-specific fields should live under `metadata` unless they are broadly reusable.
