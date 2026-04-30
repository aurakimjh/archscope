# Parser Design

Parsers convert raw files into typed records. They should not own chart rendering or report formatting.

## Responsibilities

- Read files with encoding fallback.
- Parse line-oriented or block-oriented evidence.
- Preserve raw input fragments for traceability.
- Return typed records or parser diagnostics.
- Support streaming patterns for future large-file handling.

## Access Log Parser

Initial support targets an NGINX combined-like format with a response time field:

```text
127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] "GET /api/orders/1001 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123
```

The parser extracts:

- timestamp
- method
- uri
- status
- bytes sent
- referer
- user agent
- response time in milliseconds
- raw line

Future formats include Apache, OHS, WebLogic, Tomcat, and custom regex patterns.

## Collapsed Profiler Parser

Initial support targets async-profiler collapsed output:

```text
frame1;frame2;frame3 123
```

Rules:

- Last field is sample count.
- Stack string is everything before the final sample count.
- Duplicate stacks are merged by summing samples.
- Total samples are calculated across merged stacks.
- Estimated seconds are derived from `samples * interval_ms / 1000`.
- Top N stacks are sorted by sample count.

Collapsed stacks are also converted into the common `FlameNode` tree contract so drill-down and execution breakdown can work on a tree rather than only on flat top-stack rows.

## Jennifer APM Flamegraph CSV Parser

Jennifer APM flamegraph CSV import expects these canonical columns:

```text
key,parent_key,method_name,ratio,sample_count,color_category
```

Common aliases such as `id`, `parent_id`, `method`, `name`, `samples`, `count`, and `category` are accepted where practical. The parser reconstructs a tree from `key` and `parent_key` and maps rows to the shared `FlameNode` model:

```text
id
parentId
name
samples
ratio
category
color
children
path
```

If the CSV contains multiple root nodes, ArchScope creates a virtual `All` root. Malformed CSV rows are skipped and reported through `metadata.diagnostics` with `INVALID_JENNIFER_ROW`.

## JVM Parsers

GC logs, thread dumps, and exception stack traces follow the same parser and analyzer separation as access logs and profiler inputs.

- GC parser supports HotSpot unified GC pause lines and extracts timestamp, GC type, cause, pause, and heap before/after/committed values.
- Thread dump parser supports Java quoted thread blocks with `java.lang.Thread.State`, stack frames, thread ids, and basic lock evidence.
- Exception parser supports Java exception stack blocks with optional ISO timestamps, nested `Caused by` root causes, stack frames, and stable stack signatures.

Malformed record-level input is skipped and reported under `metadata.diagnostics`.

## Multi-runtime Parsers

The first multi-runtime analyzer MVP keeps the same tolerant parser contract:

- Node.js parser supports `Error`/`TypeError`/custom `*Error` blocks with `at ...` stack frames.
- Python parser supports standard `Traceback (most recent call last):` blocks and terminal exception lines.
- Go parser supports `panic:` headers and `goroutine N [state]:` blocks with function frames.
- .NET parser supports `*Exception` stack blocks and IIS W3C access lines with `#Fields:` metadata.

These parsers are intended for small diagnostic artifacts and demo scenarios. They do not replace full structured log ingestion or OpenTelemetry correlation.

## Error Handling

Parser error handling is fixed around this principle:

```text
File-level or configuration-level failures are fatal.
Record-level malformed input is non-fatal by default and must be reported in diagnostics.
```

### Default Mode

The default parser mode is tolerant. This is the right default for operational evidence because one bad log line should not prevent the user from analyzing the rest of a large file.

- Blank lines are ignored and are not diagnostics.
- Malformed records are skipped.
- Skipped records are counted.
- A bounded sample of skipped records is reported in `metadata.diagnostics`.
- Analyzer output remains valid if at least zero records can be parsed.
- Strict fail-fast behavior may be added later as an explicit option, but it is not the Phase 1 default.

### Fatal Errors

The parser or analyzer should fail the run for conditions where analysis cannot proceed reliably:

| Condition | Policy | Example error code |
|---|---|---|
| Input file does not exist or is unreadable | Fatal | `FILE_NOT_READABLE` |
| Unsupported parser format | Fatal | `UNSUPPORTED_FORMAT` |
| Encoding fallback chain cannot decode the file | Fatal | `ENCODING_ERROR` |
| Required analyzer option is invalid | Fatal | `INVALID_OPTION` |
| Output path cannot be written | Fatal in exporter/CLI layer | `OUTPUT_WRITE_ERROR` |
| Unexpected internal exception | Fatal | `INTERNAL_ERROR` |

### Non-Fatal Record Errors

The parser should skip and report record-level errors:

| Parser | Malformed condition | Policy | Reason code |
|---|---|---|---|
| Access Log | Line does not match selected log format | Skip | `NO_FORMAT_MATCH` |
| Access Log | Timestamp parse fails | Skip | `INVALID_TIMESTAMP` |
| Access Log | Numeric field conversion fails | Skip | `INVALID_NUMBER` |
| Collapsed Profiler | Line has no trailing sample count | Skip | `MISSING_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count is not an integer | Skip | `INVALID_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count is negative | Skip | `NEGATIVE_SAMPLE_COUNT` |

### Diagnostics Shape

Parser diagnostics should be written under `AnalysisResult.metadata.diagnostics`.

Initial shape:

```text
metadata.diagnostics = {
  total_lines: number,
  parsed_records: number,
  skipped_lines: number,
  skipped_by_reason: Record<string, number>,
  samples: [
    {
      line_number: number,
      reason: string,
      message: string,
      raw_preview: string
    }
  ]
}
```

Rules:

- `samples` should be bounded, initially 20 records.
- `raw_preview` should be truncated, initially 200 characters.
- Diagnostics must not include full large log records when a short preview is enough.
- Parser implementations may keep richer internal diagnostics, but `metadata.diagnostics` is the stable external contract.

### Portable Parser Debug Logs

Parser diagnostics are intentionally compact because they travel inside `AnalysisResult`. For field parser fixes, the engine can also write a separate debug JSON file under `archscope-debug/` in the ArchScope execution directory.

Rules:

- Debug logs are written automatically when skipped records or parser exceptions are captured, and can be forced with `--debug-log`.
- `--debug-log-dir` overrides the default portable output directory.
- Raw context is redacted by default. Tokens, cookies, query values, emails, long identifiers, IP/user/host identifiers, and absolute paths are masked.
- Redaction must preserve parser evidence: delimiters, quotes, brackets, timestamp shape, numeric shape, field count, failed pattern names, partial match data, and `field_shapes`.
- The debug log is a developer artifact; `metadata.diagnostics` remains the stable UI/result contract.

### Access Log Policy

- Empty or whitespace-only lines are ignored.
- Lines that do not match the configured format are skipped with `NO_FORMAT_MATCH`.
- Bad timestamp, status, bytes, or response-time values are skipped with a specific reason code.
- `metadata.diagnostics.parsed_records` must equal the number of records included in analyzer aggregation.
- `metadata.diagnostics.skipped_lines` must not include blank ignored lines.

### Collapsed Profiler Policy

- Empty or whitespace-only lines are ignored.
- Lines without a stack plus trailing integer sample count are skipped.
- Negative sample counts are skipped, not fatal.
- Duplicate valid stacks continue to be merged by summing samples.
- `metadata.diagnostics.parsed_records` should represent valid collapsed lines before stack merging.

### Encoding And Corrupt Input

`iter_text_lines` uses the fallback chain `utf-8`, `utf-8-sig`, `cp949`, then `latin-1`. Because `latin-1` can decode any byte sequence, many corrupt byte sequences will still reach parser logic as malformed records. The policy is:

- Decoding failure after all configured encodings is fatal.
- Decoded but semantically invalid records are non-fatal and reported as skipped records.
- A future binary/corruption detector may add warnings, but it should not replace parser diagnostics.

## Large File Baseline

The Phase 1B baseline keeps parser and analyzer responsibilities separated while avoiding full record materialization for access-log analysis.

### Sampling Options

Access-log analysis accepts these optional controls:

| Option | Policy |
|---|---|
| `max_lines` | Stop reading after this many physical source lines. Must be a positive integer. |
| `start_time` | Include only parsed records whose timestamp is greater than or equal to this ISO 8601 datetime. |
| `end_time` | Include only parsed records whose timestamp is less than or equal to this ISO 8601 datetime. |

Rules:

- `max_lines` is applied before parsing a line beyond the configured limit.
- Time filters are applied after a line is parsed successfully because malformed lines have no trustworthy timestamp.
- `metadata.diagnostics.total_lines` counts physical lines read, bounded by `max_lines` when provided.
- `metadata.diagnostics.parsed_records` counts valid records included in analyzer aggregation after time filtering.
- Applied options must be echoed under `metadata.analysis_options`.

### Streaming Aggregation

The access-log analyzer should consume parser records as an iterator and update aggregation state incrementally:

- total request count
- error count
- response-time samples for summary percentiles
- per-minute request counters and response-time samples
- status-family counters
- URL request counters
- URL response-time totals and counts for average latency ranking
- bounded sample records for table output

The analyzer must not build a full `list[AccessLogRecord]` for the main analysis path. Exact percentile calculation may still keep response-time sample arrays in Phase 1B; replacing those with approximate sketches is a later large-file optimization.

### Percentile Sampling

Access-log summary and per-minute percentile values use a bounded deterministic sample rather than an unbounded response-time array. This keeps percentile memory use fixed for large files while preserving reproducible results for the same input. Because the sampler is approximate and input-order sensitive, percentile values should be interpreted as operational estimates rather than exact statistical truth for highly ordered or adversarial inputs.

The sampler seed must be a positive integer. `seed=0` is rejected because it degenerates the deterministic replacement stream and can repeatedly target the same reservoir slot.

### Access Log Findings

Access-log findings are bounded structured observations under `metadata.findings`. Initial rules:

- `HIGH_ERROR_RATE` when error rate is at or above 10%.
- `ELEVATED_ERROR_RATE` when error rate is at or above 5%.
- `SERVER_ERRORS_PRESENT` when one or more `5xx` responses are present.
- `SLOW_URL_AVERAGE` when the slowest average URL response time is at or above 1000 ms.

Findings must include a stable `code`, `severity`, short `message`, and small structured `evidence`.
