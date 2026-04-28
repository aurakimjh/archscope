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

Parsers should skip malformed lines only when configured to do so. Analyzer results should report skipped line counts in `metadata` once validation is implemented.
