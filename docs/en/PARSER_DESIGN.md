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

## Placeholder Parsers

GC logs, thread dumps, and exception stack traces have placeholder modules in the initial skeleton. They should follow the same parser and analyzer separation.

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
