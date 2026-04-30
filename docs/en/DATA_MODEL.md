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

## Contract Hardening Scope

The first contract-hardening pass is limited to the analyzer result types that already exist in code:

- `access_log`
- `profiler_collapsed`

The common `AnalysisResult` dataclass remains the outer transport model for now. The hardening layer adds type-specific contracts for the contents of `summary`, `series`, `tables`, and `metadata`.

### Included In This Scope

- Python `TypedDict` definitions for Access Log and Profiler result sections.
- Matching TypeScript interfaces for renderer and chart code.
- Required keys, value types, and units documented in this file.
- `schema_version` retained in `metadata` for future migration.

### Excluded From This Scope

- Full Pydantic model migration.
- Runtime validation for every nested field.
- GC log, thread dump, and exception result contracts before those analyzers are implemented.
- Chart Studio template schema.
- Dashboard sample data as a canonical contract. `dashboard_sample` is UI fixture data only.

### Versioning Rules

- Additive optional fields may be introduced under the same `schema_version`.
- Removing or renaming required fields requires a `schema_version` bump.
- Numeric fields must use explicit units in the key name when the unit is not obvious, for example `_ms`, `_sec`, or `_percent`.
- Parser diagnostics live under `metadata.diagnostics` for parsers that support malformed-input handling.

## Required Result Contracts

### Access Log Result

`type`: `access_log`

Required `summary` fields:

| Field | Type | Unit / meaning |
|---|---|---|
| `total_requests` | integer | Parsed request count |
| `avg_response_ms` | number | Average response time in milliseconds |
| `p95_response_ms` | number | 95th percentile response time in milliseconds |
| `p99_response_ms` | number | 99th percentile response time in milliseconds |
| `error_rate` | number | Percent of requests with status `>= 400` |

Required `series` fields:

| Field | Row shape |
|---|---|
| `requests_per_minute` | `{ time: string, value: number }` |
| `avg_response_time_per_minute` | `{ time: string, value: number }` |
| `p95_response_time_per_minute` | `{ time: string, value: number }` |
| `status_code_distribution` | `{ status: string, count: integer }` |
| `top_urls_by_count` | `{ uri: string, count: integer }` |
| `top_urls_by_avg_response_time` | `{ uri: string, avg_response_ms: number, count: integer }` |

Required `tables` fields:

| Field | Row shape |
|---|---|
| `sample_records` | `{ timestamp: string, method: string, uri: string, status: integer, response_time_ms: number }` |

Required `metadata` fields:

| Field | Type | Meaning |
|---|---|---|
| `format` | string | Access log format selector |
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line counts and skipped-record samples |
| `analysis_options` | `AccessLogAnalysisOptions` | Applied sampling and time-range options |
| `findings` | array of `AccessLogFinding` | Report-oriented access-log observations |

`AccessLogAnalysisOptions` required fields:

| Field | Type | Meaning |
|---|---|---|
| `max_lines` | integer or null | Maximum physical lines read from the source file |
| `start_time` | string or null | Inclusive ISO 8601 lower timestamp bound |
| `end_time` | string or null | Inclusive ISO 8601 upper timestamp bound |

`AccessLogFinding` required fields:

| Field | Type | Meaning |
|---|---|---|
| `severity` | string | Finding severity such as `warning` or `critical` |
| `code` | string | Stable finding code |
| `message` | string | Human-readable finding summary |
| `evidence` | object | Small structured values supporting the finding |

### Profiler Collapsed Result

`type`: `profiler_collapsed`

Required `summary` fields:

| Field | Type | Unit / meaning |
|---|---|---|
| `profile_kind` | string | Profile type, initially `wall` |
| `total_samples` | integer | Total collapsed stack sample count |
| `interval_ms` | number | Sampling interval in milliseconds |
| `estimated_seconds` | number | Estimated sampled time in seconds |
| `elapsed_seconds` | number or null | Optional observed elapsed time in seconds |

Required `series` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null }` |
| `component_breakdown` | `{ component: string, samples: integer }` |
| `execution_breakdown` | optional `{ category, executive_label, primary_category, wait_reason, samples, estimated_seconds, total_ratio, parent_stage_ratio, elapsed_ratio, top_methods, top_stacks }` |

Required `tables` fields:

| Field | Row shape |
|---|---|
| `top_stacks` | `{ stack: string, samples: integer, estimated_seconds: number, sample_ratio: number, elapsed_ratio: number | null, frames: string[] }` |
| `top_child_frames` | optional `{ frame: string, samples: integer, ratio: number }` |

Optional `charts` fields:

| Field | Shape |
|---|---|
| `flamegraph` | `FlameNode` |
| `drilldown_stages` | array of drill-down stage objects |

`FlameNode` shape:

```text
{
  id: string,
  parentId: string | null,
  name: string,
  samples: integer,
  ratio: number,
  category: string | null,
  color: string | null,
  children: FlameNode[],
  path: string[]
}
```

Execution breakdown categories:

```text
SQL_DATABASE
EXTERNAL_API_HTTP
NETWORK_IO_WAIT
APPLICATION_LOGIC
FRAMEWORK_MIDDLEWARE
LOCK_SYNCHRONIZATION_WAIT
CONNECTION_POOL_WAIT
FILE_IO
GC_JVM_RUNTIME
IDLE_BACKGROUND
UNKNOWN
```

Required `metadata` fields:

| Field | Type | Meaning |
|---|---|---|
| `parser` | string | Parser implementation identifier |
| `schema_version` | string | Result schema version |
| `diagnostics` | `ParserDiagnostics` | Parser line counts and skipped-record samples |

### ParserDiagnostics

Required fields:

| Field | Type | Meaning |
|---|---|---|
| `total_lines` | integer | Physical lines read from the source file |
| `parsed_records` | integer | Valid non-blank records accepted by the parser |
| `skipped_lines` | integer | Malformed non-blank records skipped by the parser |
| `skipped_by_reason` | object mapping string to integer | Skipped record counts by reason code |
| `samples` | array of `DiagnosticSample` | Bounded examples of skipped records |

`DiagnosticSample` required fields:

| Field | Type | Meaning |
|---|---|---|
| `line_number` | integer | 1-based source line number |
| `reason` | string | Stable reason code, such as `NO_FORMAT_MATCH` |
| `message` | string | Human-readable parser message |
| `raw_preview` | string | Truncated raw input preview, currently capped at 200 characters |

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

## AI Interpretation Contract

AI interpretation does not replace `AnalysisResult`. It is a separate `InterpretationResult` linked back to a source result.

```text
InterpretationResult
  schema_version
  provider
  model
  prompt_version
  source_result_type
  source_schema_version
  generated_at
  findings
  disabled
```

`AiFinding` required fields:

| Field | Type | Meaning |
|---|---|---|
| `id` | string | Stable finding id |
| `label` | string | Short title |
| `severity` | string | `info`, `warning`, or `critical` |
| `generated_by` | string | Must be `ai` |
| `model` | string | Local model name |
| `summary` | string | User-facing interpretation |
| `reasoning` | string | Short reasoning bound to evidence |
| `evidence_refs` | array of string | Non-empty canonical evidence references |
| `evidence_quotes` | object | Optional exact evidence substrings keyed by `evidence_ref` |
| `confidence` | number | Model confidence from `0` to `1`; initial display threshold is `0.3` |
| `limitations` | array of string | Missing evidence or uncertainty |

Canonical `evidence_ref` grammar:

```text
{source_type}:{entity_type}:{identifier}
```

Registered namespaces:

| Source | Entities |
|---|---|
| `access_log` | `record`, `finding` |
| `profiler` | `stack`, `frame`, `finding` |
| `jfr` | `event` |
| `otel` | `log`, `span`, `event` |
| `timeline` | `event`, `correlation` |

AI output must pass runtime validation before it is shown. Validation includes non-empty references, grammar and namespace checks, reference presence in the source evidence registry, confidence threshold, and quote-to-source matching when quotes are provided.

## Design Rules

- Parsers preserve raw evidence where practical through `raw_line` or `raw_block`.
- Analyzers produce numeric fields with explicit units.
- Chart inputs come from `series` and `tables`, not parser-specific objects.
- Runtime-specific fields should live under `metadata` unless they are broadly reusable.
- Analyzer sampling and filter settings should be echoed under `metadata.analysis_options`.
- Report-grade interpretations should be expressed as bounded structured findings, not prose-only blobs.
- AI-assisted interpretations must include non-empty `evidence_refs` that point to existing raw evidence such as `raw_line`, `raw_block`, `raw_preview`, or `evidence_ref` rows.
