# Parser Design

## Extension checklist (2026-11-11, T-628 foundation implemented)

Use the [evolution design §16](./LOCAL_DIAGNOSTICS_EVOLUTION_DESIGN.md#16-extension-and-compatibility-contracts)
when introducing the descriptor/catalog path. Existing family-specific CLI and
`Analyze*` descriptions below describe current behavior, not completion of this plan.

1. Specify producer/version/measurement/limits; consider declarative mapping, external conversion, then compiled code.
2. Register sanitized shared fixtures and expected semantics in projects-assets and update ASSET_INDEX.
3. Declare detection budgets, schemas, record family, capabilities, converters and adapter/fixture versions.
4. Keep decoding/source diagnostics, semantic normalization, and AnalysisResult analysis separate.
5. Register in the composition root and validate generic/timeline/SLI/compare/detail providers; retain old methods as wrappers.
6. Test ambiguity, loss, unknown versions, cancellation, partial results, metadata roundtrips and goldens; update the support matrix.
7. Existing semantic families/renderers should need only package, fixtures and registration. Explain additional service/CLI/UI switches and removal plans. New semantics or visual grammars require explicit contract work.

### T-628 detailed design scope and baseline

The detailed design was checked against `83b8ff9` and fixed at `eb3e782`.
T-628 now implements C1–C4 with one pilot importer. The logical contract below
is qualified by the implementation record at the end of this document; complete
ProfileSet, renderer and measurement-correction migrations remain separate.

| Current evidence | Behavior | Migration boundary |
|---|---|---|
| internal/ingestion/registry.go | 64 KiB probe, highest confidence wins, first sorted entry wins a tie | Retain legacy Detect while adding a candidate-set API |
| internal/ingestion/ingestion_test.go | Confidence, unknown format, HAR/NetLog tests | Add ambiguity, budget and explicit-selection cases |
| internal/threaddump/registry.go | Plugins, default mixed-format rejection and explicit override | Preserve existing options through wrappers without claiming broader support |
| internal/analyzers/profile/analyzer.go | profile_evidence 0.1.0 and legacy profiler calculations | T-628 wires contracts; T-629 migrates the core; T-634 corrects measurement semantics |

### Registration and selection contract

internal/catalog explicitly assembles and validates built-ins at
startup, then freezes the registry. Do not rely on package init side effects.
Separate executable handles from bounded descriptor views returned to consumers.

| Contract | Required rule |
|---|---|
| IDs/aliases | Nonempty lowercase ASCII letters/digits/dot/underscore/hyphen; normalize only documented legacy aliases. Reject duplicate canonical/alias names and reserved auto |
| Versions | Adapter version differs from verified producer versions; unknown producers are not assumed compatible |
| Record family | Validate parser/normalizer/analyzer family at composition and for every batch |
| Schemas | Resolve bundled ID/version/hash; never fetch external schema URLs |
| Capabilities | Descriptor is an upper bound; actual time/symbol/comparison capability is reduced by file evidence |
| Support evidence | Release registration requires fixture manifest/hash and conformance version |

DetectCandidates returns all candidates and reasons. Verified structural/signature
evidence differs from filename/extension hints. Confidence is a ranking aid, not
a probability or sufficient auto-selection rule. Nonfinite/out-of-range scores
are detector errors. Select automatically only when exactly one canonical importer
is verified. Multiple verified candidates or hints alone require selection; no
candidate is unsupported. Coalesce aliases before ranking by evidence, confidence,
then stable ID.

Explicit selection still validates structure, options and supported producer versions.
A known contradictory signature returns IMPORT_FORMAT_MISMATCH. Do not silently
retry another parser. Existing CLI aliases and thread overrides remain explicit
compatibility options covered by fixtures.

### Execution interfaces and ownership

These are logical internal Go/Wails contracts, not HTTP endpoints or an external
plugin ABI. DTO names may change without changing ownership or semantics.

```text
Catalog.List() -> bounded DescriptorView[]
Catalog.DetectCandidates(ctx, Probe) -> DetectionSet
Catalog.Resolve(importerID, options, policyRevision) -> ResolvedImporter
ImportEvidence(ctx, sourceRef, importerSelection, options) -> ImportJobRef
SelectImporter(jobID, generation, importerID) -> JobSnapshot
Parser.Parse(ctx, SourceLease, RecordSink) -> ParseReport
Normalizer.Normalize(ctx, RecordBatch, NormalizedSink) -> error
Analyzer.Analyze(ctx, NormalizedRef, AnalysisOptions) -> AnalysisResult
GetImportJob(jobID, generation) -> JobSnapshot
CancelImport(jobID, generation) -> CancelOutcome
```

- The service owns SourceLease open/close, source ID, safe display name, fingerprint and bounded read/reopen. Parsers cannot open unrelated files, networks or converters. Seek requirements use declared bounded spooling or an unsupported response.
- RecordBatch holds family-typed records, source byte/line/event positions and sequence. Ownership transfers to a synchronous, backpressured sink; parsers cannot mutate transferred batches or accumulate unbounded UI queues.
- Probe and parse must use the same source; one file handle alone does not guarantee immutability. If lifetime/fingerprint, consumed bytes and final hash cannot reliably establish consistency, first create an owned immutable snapshot. Snapshot budget failure or mutation fails the job, including SOURCE_CHANGED where applicable. Byte equality does not merge collection provenance.
- Freeze validated options and record a versioned canonical option hash. Do not expose secrets or raw paths through progress/hash explanations.
- Record job budgets for source/expanded bytes, record size/count, depth, unique keys, time and temporary disk. Retain the current 64 KiB probe default and independently bound decompression. Reject importer/user limits exceeding policy.
- Converters run in a separate explicitly selected local step with bounded arguments, output, time and temporary files, outside the parser.

### Job state and failure contract

```text
queued -> probing -> needs_selection -> validating -> parsing -> normalizing
       -> analyzing -> validating_result -> committing -> completed | partial
```

Stages are logical; streaming parse/normalize may overlap. Auto-selection skips
needs_selection; no full parse starts before a selection. SelectImporter applies
only to that waiting job/generation, and repeating the same choice is idempotent.
Changing importer after parsing starts requires cancellation and a new job. Active stages may end
failed/cancelled. All four final states are terminal. Atomic manifest publication
is the commit point; later cancellation returns already_terminal. Cancellation
for the same job/generation is idempotent.

Validate staged schema, measurements and references before publication. Failure
or cancellation cleans owned staging/leases and preserves prior revisions. Partial
output requires complete usable records, importer-defined partial semantics and
caller permission. Source mutation, family/schema errors, unknown required units
and broken core structure cannot become partial success. Budget exhaustion may
produce partial output only at a defined safe record boundary. Never invent missing
event endpoints or weights.

| Proposed code | Meaning and consumer response |
|---|---|
| IMPORT_REGISTRY_INVALID | Reject catalog activation on duplicate IDs, missing schemas or family mismatch |
| IMPORT_SELECTION_REQUIRED | Display ambiguity/hints and await selection |
| IMPORT_FORMAT_UNSUPPORTED / IMPORT_VERSION_UNSUPPORTED | Explain limits/conversion; do not create successful results |
| IMPORT_FORMAT_MISMATCH / IMPORT_OPTIONS_INVALID | Correct selection/options without silent parser fallback |
| IMPORT_BUDGET_EXCEEDED / SOURCE_CHANGED | Disclose actual constraint and partial availability |
| IMPORT_RESULT_INVALID | Reject commit and preserve existing snapshots |

Cancellation is a terminal job state, not a diagnostic finding. JobProblem contains
code, stage, retryability, source ID, safe message and bounded diagnostic references.
Failures cannot become empty successful AnalysisResults. Partial coverage also lives
in result metadata so save/reload/export does not depend on job history.

### T-628 implementation handoff

| Stage | Change | Required evidence |
|---|---|---|
| C1 | Descriptor/catalog and compatibility wrappers | Existing detection/alias/thread behavior plus new registration/ambiguity checks |
| C2 | Orchestration and bounded sinks | Selection, cancellation, partial safety, source mutation and cleanup regressions |
| C3 | Models/profiler metadata and shared decoder | [Data-model contract](./DATA_MODEL.md#t-628-detailed-measurement-and-compatibility-contract) precision, unknown-field and version cases |
| C4 | One existing importer end to end | CLI/Wails load-save-export fidelity and recorded switch count |

T-628 proves C1–C4 with at least one existing importer. All-format core migration,
generic rendering and semantic corrections remain T-629, T-631 and T-634.
Preserve profile goldens, metadata roundtrips and thread mixed-format tests. Add
duplicate/alias, hint-only, multiple verified, invalid score, cancellation around
commit, unsafe partial, schema mismatch, stale generation and exact-number cases.
Skipped required fixtures are not release evidence. Reusable fixtures and actual
JSON Schemas go to indexed shared assets during implementation; this section is
an ArchScope-specific execution contract, not a new shared schema file.

Parsers convert raw files into typed records. They should not own chart rendering or report formatting.

## Responsibilities

- Read files with encoding fallback.
- Parse line-oriented or block-oriented evidence.
- Preserve raw input fragments for traceability.
- Return typed records or parser diagnostics.
- Support streaming patterns for future large-file handling.

## Shared Ingestion Foundation

Mid-Term Plus importer work uses a shared Go package under
`apps/engine-native/internal/ingestion` before parser-specific records are
created. The boundary is:

- parser packages live under `internal/parsers/<family>` and only decode files,
  classify formats, and emit typed records plus diagnostics.
- analyzer packages live under `internal/analyzers/<family>` and build the
  `AnalysisResult` envelope.
- CLI groups use kebab-case family names, for example `server-log analyze`,
  `database-log analyze`, and `profile import`.
- Wails bindings keep the `Analyze<Family>` shape, for example
  `AnalyzeServerLog` and `AnalyzeDatabaseLog`.
- result types stay snake_case, for example `server_log`,
  `database_slow_query`, `broker_log`, `kubernetes_evidence`, and
  `profile_evidence`.

The same package also provides:

- a reusable source-format registry and bounded probe contract for auto-detect
  dispatch.
- a golden fixture diagnostic harness for valid, partial, malformed,
  unknown-format, and large-file importer samples.
- normalized source metadata for source kind, format, product/version, host,
  service, environment, and sanitized file identity.
- a cross-source correlation-key model for trace/span/request IDs,
  tenant/customer IDs, container/pod/host/PID identity, and timestamp windows.

## Evidence Stitching

`stitch analyze` is a second-pass analyzer over existing `AnalysisResult` JSON
files. It does not parse raw logs again. Instead, it extracts bounded evidence
nodes from tables, records trace/span/request/tenant/container/host/PID keys,
builds match groups, and emits correlation-gap findings for missing trace IDs,
dropped parent spans, unmatched request logs, unmatched database calls, and
unmatched broker events. Advanced stitching adds timestamp-window matching,
service alias normalization for workload/service/database/broker naming
variants, trace-profile linkage through profile labels, and `match_drilldowns`
for raw-source inspection.

## API And Event Contract Parser

`api-contract analyze` accepts OpenAPI and AsyncAPI JSON, plus a conservative
YAML subset that covers `paths`, HTTP method objects, `operationId`, `summary`,
`tags`, `channels`, `publish`, `subscribe`, and message names. The analyzer
compares OpenAPI operations to access-log result tables and AsyncAPI channels
to broker result tables, then emits deterministic findings for undocumented,
unused, slow, and high-error API/event surfaces.

## Architecture Documentation Package

`architecture-docs draft` is a second-pass analyzer over existing
`AnalysisResult` JSON files. It extracts architecture-review rows for arc42
Context, Runtime View, Deployment View, Quality Requirements, Risks, and ADR
drafts. Parser responsibility remains with the original evidence importers; this
package only translates already-normalized evidence and findings into
documentation inputs.

## Access Log Parser

Access-log parsing keeps the legacy NGINX/common/combined path compatible while
adding source-specific access and edge formats behind explicit selectors and
`auto` detection. Supported selectors include NGINX, Apache/common/combined,
OHS, WebLogic, Tomcat, Jetty, HAProxy HTTP, Envoy/Istio text and JSON, AWS
ELB/ALB/CloudFront, GCP HTTP(S) Load Balancer JSON, Azure App Service/Front
Door JSON, IIS W3C, Caddy JSON, Traefik JSON, Kong JSON, Tyk JSON, and AWS API
Gateway JSON.

The legacy combined-like path targets NGINX rows with a response time field and
tolerates common/combined access-log rows without response-time data:

```text
127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] "GET /api/orders/1001 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123
```

The parser extracts the common access-log fields:

- timestamp
- method
- uri
- status
- bytes sent
- referer
- user agent
- response time in milliseconds
- raw line

When a common/combined row has no response-time field, ArchScope keeps the
request/status evidence and records response time as `0.0` because the source
format does not provide latency. Apache, OHS, WebLogic, Tomcat, and Jetty
format selectors share this combined/common parsing path when their rows match
that shape.

Source-specific parsers may also populate additive edge metadata such as source
format, host, protocol/scheme, upstream service or cluster, route, backend,
gateway/upstream latency, TLS fields, response flags, termination state, retry
count, trace ID, request ID, consumer, cloud provider, and edge location. These
fields feed analyzer summaries and downstream projections but do not replace the
required access-log contract.

## Server Log Parser

The server-log parser accepts `auto`, Tomcat, Jetty, JBoss/WildFly, WebLogic,
WebSphere, GlassFish/Payara, nginx error, and Apache error selectors. Parser
responsibility stops at decoding rows into normalized event records:

- timestamp when present
- severity
- product
- component
- thread / host / service when present
- event type
- message
- trace ID and request ID when present
- raw line

The analyzer owns taxonomy and findings for startup, deployment, datasource
pool, stuck/hung thread, thread-pool pressure, managed-server health, severe
error, and worker/upstream error events.

## Collapsed Profiler Parser

Initial support targets async-profiler collapsed output:

```text
frame1;frame2;frame3 123
```

Rules:

- Last field is sample count.
- Stack string is everything before the final sample count.
- Integral numeric sample fields such as `1000` and `1e3` are accepted; negative,
  non-finite, or fractional sample counts are rejected.
- Duplicate stacks are merged by summing samples.
- Total samples are calculated across merged stacks.
- Estimated seconds are derived from `samples * interval_ms / 1000`.
- Top N stacks are sorted by sample count.

Collapsed stacks are also converted into the common `FlameNode` tree contract so drill-down and execution breakdown can work on a tree rather than only on flat top-stack rows.

## Unified Profile Evidence Parser

The `profile import` path normalizes profiler families into a common sample
schema before analysis. Each sample carries a stack of frames tagged with
language, runtime, `native`, `managed`, and `async` markers. The analyzer then
rolls those samples up to collapsed stacks and reuses the existing flamegraph,
top-stack, execution-breakdown, and drill-down logic.

Supported import contracts include:

- pprof binary/gzip profiles (`.pb.gz`) through `github.com/google/pprof/profile`
- async-profiler collapsed and HTML flamegraph payloads through the existing
  profiler parser
- py-spy and rbspy raw text stack dumps
- speedscope JSON, including dotnet-trace speedscope exports
- perf collapsed stacks for Rust/native evidence
- JFR JSON stack sample events
- Ruby StackProf JSON
- PHP Excimer/Tideways-style JSON and Xdebug cachegrind text
- Swift backtrace and generic async stack text
- Pyroscope/Phlare and Parca-style continuous-profiling snapshots

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

The reader tries `utf-8-sig`, `utf-8`, `cp949`, and the platform preferred
encoding. If the CSV contains multiple root nodes, ArchScope creates a virtual
`All` root. Missing parents are reported and treated as roots in tolerant mode;
duplicate keys and parent cycles are reported and safely skipped or detached.

Jennifer `sample_count` values are treated as inclusive node samples. Tree
rendering preserves those inclusive values, while top-stack and execution
breakdown calculations use non-overlapping exclusive paths so category totals do
not double-count parent and child samples.

## JVM Parsers

GC logs, thread dumps, and exception stack traces follow the same parser and analyzer separation as access logs and profiler inputs.

- **GC parser** (`gc_log_parser.py`) supports HotSpot unified GC pause lines and extracts timestamp, GC type, cause, pause, and heap before/after/committed values for Eden/Survivor/Old/Metaspace regions.
- **GC log header parser** (`gc_log_header.py`) reads the `OpenJDK ... VM` banner and `CommandLine flags:` line, plus the `[gc,init]` block (CPU count, memory, heap min/initial/max/region size, parallel/concurrent worker counts, NUMA, pre-touch). The result becomes the **JVM Info** card at the top of the GC log analyzer.
- **JFR recording parser** (`jfr_recording.py`) detects the binary `FLR\0` magic and shells out to the JDK `jfr` CLI (`ARCHSCOPE_JFR_CLI` → `JAVA_HOME/bin/jfr` → PATH) with `print --json`. The existing JSON path (`jfr_parser.py`) is unchanged. The parser also accepts mode/time-range/state filters before aggregation.
  The JDK `jfr` CLI is a binary-to-JSON conversion boundary only. ArchScope does
  not try to clone the full JDK `jfr view` or `jfr summary` UI; it builds
  report-oriented event summaries, async-profiler-style stack aggregation,
  recording hints, and Evidence Board rows from `stackTrace.frames` in the JSON
  output.
- **Thread dump parser registry** (`thread_dump/registry.py`) exposes a plugin contract `(format_id, can_parse(head: str) -> bool, parse(path) -> ThreadDumpBundle)`. The registry probes the first 4 KB of every input, dispatches to the matching plugin, and decodes UTF-16 / BOM-encoded inputs transparently. Mixing formats inside one multi-dump request raises `MixedFormatError` unless `format_override` is set.
- **Exception parser** supports Java exception stack blocks with optional ISO timestamps, nested `Caused by` root causes, stack frames, and stable stack signatures.

Malformed record-level input is skipped and reported under `metadata.diagnostics`.

## Multi-runtime Thread-dump Plugins

The thread-dump registry currently ships with **9 plugin variants**:

| Format ID                       | Runtime          | Detection signature |
| ------------------------------- | ---------------- | ------------------- |
| `java_jstack`                   | JVM              | `Full thread dump …` header **or** quoted-name + `nid=0x…` **or** quoted-name + state line (JDK 21+ no-`nid` form) |
| `java_jcmd_json`                | JVM              | JSON object containing `"threadDump"` |
| `go_goroutine`                  | Go               | `^goroutine \d+ \[\w` (covers `runtime.Stack`, panic, debug.Stack) |
| `python_pyspy`                  | Python (py-spy)  | `Process N:` + `Python vX.Y` |
| `python_faulthandler`           | Python (stdlib)  | `Thread 0xHEX (most recent call first):` |
| `python_traceback`              | Python (custom)  | `Thread ID: <n>` + `File "...", line N, in func` |
| `nodejs_diagnostic_report`      | Node.js (12+)    | JSON with `"header"` + `"javascriptStack"` |
| `nodejs_sample_trace`           | Node.js          | `Sample #N` + `Error` + `at fn(file:line:col)` |
| `dotnet_clrstack`               | .NET             | `OS Thread Id: 0xHEX` + `Child SP / IP / Call Site` |
| `dotnet_environment_stacktrace` | .NET             | `at Type.Method() in path:line N` |

Each plugin runs a language-only post-pass that promotes `RUNNABLE` /
`UNKNOWN` states into the more specific `NETWORK_WAIT` / `IO_WAIT` /
`LOCK_WAIT` / `CHANNEL_WAIT` categories so the multi-dump correlator
can build language-agnostic findings. See
[`MULTI_LANGUAGE_THREADS.md`](MULTI_LANGUAGE_THREADS.md) for the full
matrix.

## Multi-runtime Stack Parsers (single-shot)

Used for one-off exception/panic artifacts on the **single-stack**
analyzer pages — distinct from the thread-dump pipeline above:

- Node.js parser supports `Error`/`TypeError`/custom `*Error` blocks with `at ...` stack frames.
- Python parser supports standard `Traceback (most recent call last):` blocks and terminal exception lines.
- Go parser supports `panic:` headers and `goroutine N [state]:` blocks with function frames.
- .NET parser supports `*Exception` stack blocks and IIS W3C access lines with `#Fields:` metadata.

These parsers are intended for small diagnostic artifacts and demo scenarios. They do not replace full structured log ingestion or OpenTelemetry correlation.

## OpenTelemetry JSONL Parser

The OpenTelemetry MVP accepts line-delimited JSON log records. It supports common
field aliases such as `trace_id`/`traceId`, `span_id`/`spanId`,
`parent_span_id`/`parentSpanId`, `severity_text`/`severityText`,
`service_name`/`service.name`, and `body`.

The analyzer groups records by trace, service, and severity, then reports traces
that appear across more than one service as a lightweight correlation signal. If
parent span IDs are present, service paths are reconstructed from the span tree;
otherwise the analyzer falls back to timestamp order. Failed traces report the
first failing service, downstream services that appear after the failure, and a
failure-propagation finding when applicable. Full OTel resource/logs envelope
ingestion and span timing correlation remain future work.

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
- Parsers that expose `strict=True` fail fast on malformed records; the default
  remains tolerant/non-strict.

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
| Access Log | JSON access-log payload cannot be decoded | Skip | `INVALID_JSON` |
| Access Log | IIS W3C row appears before a `#Fields:` header or is missing required columns | Skip | `MISSING_W3C_FIELDS` |
| Collapsed Profiler | Line has no trailing sample count | Skip | `MISSING_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count is not an integer | Skip | `INVALID_SAMPLE_COUNT` |
| Collapsed Profiler | Sample count is negative | Skip | `NEGATIVE_SAMPLE_COUNT` |

### Diagnostics Shape

Parser diagnostics should be written under `AnalysisResult.metadata.diagnostics`.

Initial shape:

```text
metadata.diagnostics = {
  source_file: string | null,
  format: string | null,
  total_lines: number,
  parsed_records: number,
  skipped_lines: number,
  skipped_by_reason: Record<string, number>,
  warning_count: number,
  error_count: number,
  samples: [
    {
      line_number: number,
      reason: string,
      message: string,
      raw_preview: string
    }
  ],
  warnings: DiagnosticSample[],
  errors: DiagnosticSample[]
}
```

Rules:

- `samples`, `warnings`, and `errors` are bounded, currently 100 records each.
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
- Common/combined rows without a response-time field are parsed with
  `response_time_ms=0.0`.
- `metadata.diagnostics.parsed_records` must equal the number of records included in analyzer aggregation.
- `metadata.diagnostics.skipped_lines` must not include blank ignored lines.

### Collapsed Profiler Policy

- Empty or whitespace-only lines are ignored.
- Lines without a stack plus trailing whole-number sample count are skipped.
- Negative sample counts are skipped, not fatal.
- Zero sample counts are counted as parsed records but do not create flamegraph
  samples.
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

### Go Engine Streaming Baseline

As of the Go/Wails baseline, line-oriented parsers should expose callback-based
record iteration on top of `internal/textio.ForEachTextLine`.

- `ForEachTextLine` is the preferred path for access logs, GC logs, OTel JSONL,
  exception stacks, and simple runtime stack files.
- Compatibility helpers may still return slices for tests and small callers, but
  analyzer entrypoints should use callback iteration where practical.
- `ForEachTextLineWithContext` keeps only previous/current/next lines for debug
  context instead of duplicating the full file.
- Parser diagnostics must still count physical lines read, parsed records, and
  skipped records after `max_lines` and time filters are applied.

Large result payloads must be bounded separately from parser memory. GC log
series use `MaxSeriesPoints` plus deterministic event-bucket downsampling so
chart payloads cannot grow with every parsed event. JFR JSON and HTML profiler
inputs use file-size preflight because those formats still require structured
payload materialization.

### Percentile Sampling

Access-log summary and per-minute percentile values use a bounded deterministic sample rather than an unbounded response-time array. This keeps percentile memory use fixed for large files while preserving reproducible results for the same input. Because the sampler is approximate and input-order sensitive, percentile values should be interpreted as operational estimates rather than exact statistical truth for highly ordered or adversarial inputs.

The sampler seed must be a positive integer. `seed=0` is rejected because it degenerates the deterministic replacement stream and can repeatedly target the same reservoir slot.

### Access Log Findings

Access-log findings are bounded structured observations under `metadata.findings`. Initial rules:

- `HIGH_ERROR_RATE` when error rate is at or above 10%.
- `ELEVATED_ERROR_RATE` when error rate is at or above 5%.
- `SERVER_ERRORS_PRESENT` when one or more `5xx` responses are present.
- `SLOW_URL_AVERAGE` when the slowest average URL response time is at or above 1000 ms.
- `EDGE_RETRIES_PRESENT` when gateway or edge logs report retry attempts.
- `HAPROXY_TERMINATION_ERRORS` when HAProxy termination state indicates an abnormal close or server/client-side failure.
- `GATEWAY_LATENCY_ELEVATED` when gateway latency percentiles indicate the edge tier is adding material delay.
- `HIGH_ERROR_SERVICE_EDGE` when an inferred caller-to-upstream service edge has elevated HTTP errors.

Findings must include a stable `code`, `severity`, short `message`, and small structured `evidence`.

### Ordered profiler regex validation

Ordered regex filters validate the complete pattern budget and compile every
term once before traversing stacks, including terms no input can reach.
Invalid or unsafe include/exclude filters return an empty stage with
`INVALID_REGEX` or `UNSAFE_REGEX`, never a valid-looking zero-match result or
an unfiltered exclude result. Existing order, case, and reroot rules are retained.


## T-628 implementation record (2026-11-11)

The baseline is `eb3e782`. C1–C4 implement the common foundation and an existing
Tomcat server-log pilot. At the initial checkpoint no independent review had been performed; see the
review remediation below for the later CONDITIONAL verdict.

| Stage | Implementation | Evidence |
|---|---|---|
| C1 | Immutable ingestion descriptors/catalog, ID/alias/schema ID-version-hash/family validation, verified/hint candidates and explicit selection, built-in composition and legacy confidence wrapper | Registration/ambiguity/options/deadline regressions; existing confidence and mixed-thread/override tests retained |
| C2 | analysis.Service, owned snapshot/second hash-stat pass, synchronous normalization and sealed bounded sink, cancellation, safe partial, validation and atomic publish | Selection/idempotency/stale generation, forced mismatch, source edits, cancellation before/after publish, budgets, unsafe partial, prior revision and staging cleanup |
| C3 | wirejson/resultcontract, common/legacy metadata, operation gates and opaque bridge | DATA_MODEL W01–W10 regressions plus separate bridge/frontend checks |
| C4 | Existing server-log parser factory and streaming analyzer, evidence CLI and EngineService job/load/export | Actual CLI execution, direct Wails-method integration, and a same-family synthetic adapter added only by registration |

Commands are `evidence formats`, `evidence import --in FILE --importer tomcat --out result.json`,
and `evidence export --in result.json --export-format json|csv|html|report-pack --out FILE`.
Options accept `--options '{"top_n":25,"max_lines":0,"strict":false}'` and `--allow-partial`.
Hints/collisions print candidates and require selection. Existing family CLI aliases,
thread overrides and AnalyzeServerLog remain unchanged. Only `server.tomcat`/`tomcat`
is connected to the new built-in path; this is not an all-parser migration.

Wails exposes ListEvidenceImporters, ImportEvidence, GetImportJob, SelectImporter,
CancelImport, LoadEvidenceResult, GetEvidenceResult, GetEvidenceResultJSON,
GetEvidenceScalar, CompareEvidenceScalar and ExportEvidenceResult. Options are
JSON text; results use immutable backend handles and bounded views/opaque JSON.
The backend catalog decides operations; imported capability strings grant none.

Operational defaults: input/normalized data 64 MiB each, serialized record 1 MiB,
100,000 records, stack depth 1,024, 100,000 unique keys, temporary disk 160 MiB,
queue and execution 5 minutes each, probe 64 KiB/one second per detector, options 64 KiB. Requests
may lower these ceilings. Four jobs run concurrently, with 128 jobs per-process
history. Timeouts are cooperative, not a sandbox or forced preemption of blocked
OS I/O/trusted compiled functions. The lease closes outstanding readers; failed
temporary cleanup is disclosed as cleanupPending and retried on shutdown.

The atomic publication unit is the result revision JSON containing provenance.
A same-filesystem rename is its commit marker; separate case/index manifests
remain T-630. Failure/cancellation never replace a prior revision. Read-only
retention of an unsupported original is distinct from normal analysis-job commit.

The pilot supports uncompressed UTF-8 with a structurally verified record in the
probe. Other encodings, empty/unverified prefixes, seek and approved converters
use legacy paths or later adapters. No producer version is inferred from syntax.
The synthetic `diagnostics.ingestion.foundation.v1` manifest and measurement schema
are hashed and bundled. Review remediation adds pinned conformance validation for bundled
JSON Schema only; external schema loading remains disabled.

Evidence distinguishes actual CLI, direct Go bridge integration, TS state/opaque
transport, and package checks. No new generic job screen or actual WebView IPC
execution was produced (T-631). Windows native execution was not performed on Mac;
cross-builds are separate. ProfileSet (T-629), generic renderer (T-631), measurement
corrections (T-634), case/cache lifecycle (T-630), and Q01–Q04 remain separate.

### T-628 independent review remediation (2026-11-11)

The Claude review of `a698d26` is **CONDITIONAL**. Its unchanged original is
[archived](../review/done/2026-11-11_claude-code_T-628_implementation-review.ko.md).
The following implementation supersedes the initial checkpoint above; local
verification does not grant an independent PASS or close T-628's REVIEW state.

- **R-01:** Complete result trees use `2 × stackDepth + 64` JSON levels (2,112
  at the 1,024-frame policy), while metadata/options/records retain depth 64.
  The import service's result byte ceiling remains its 64 MiB policy; legacy
  report readers explicitly allow up to 512 MiB and refuse larger files with a
  byte-budget error. This is a new operational ceiling compared with historical
  unbounded report reads. A 65 MiB valid JSON test covers the old 64 MiB break;
  256-frame actual CLI reports and direct Wails-method load/export tests cover
  the tree regression. Over-budget trees/bytes and deep opaque metadata refuse.
- **R-02/R-13:** Up to 128 job snapshots are retained. Oldest finished history
  is reclaimed on admission; active jobs and result revisions are preserved.
  Removed IDs return IMPORT_JOB_STALE. Queue and execution each have separate
  five-minute ceilings (`queueTimeoutMs`, `timeoutMs`), independently lowerable.
  Execution time starts only after a concurrency slot is acquired.
- **R-03/R-11:** Trusted schema registrations declare input/option/record/result
  roles, semantic family and result type. Catalog composition checks every slot,
  normalizer/analyzer linkage, and both schema-definition and compiled-validator
  conformance cases. Service activation also requires a registered result reader
  for every analyzer output and a compatible reader for each declared payload
  version (independent of the schema document revision). Composition-time JSON Schema draft 2020-12 conformance uses
  the pinned compiler; runtime validation stays in trusted compiled adapters; external file/URL schema loading is disabled. Schema literals and conformance operands have bounded arithmetic independently
  of opaque storage. Definition hashes identify data, not compiled function code. Fixture manifest expectations
  are executed against the pilot, in addition to digest checks.
- **R-07/R-09:** Detector failures become per-importer sanitized diagnostics (at
  most the 256-entry catalog). An unrelated failure cannot block explicit valid
  selection. Automatic selection is withheld if a detector failed; the user must
  choose a usable candidate, and a failed selected detector is refused. Parent
  cancellation, detector timeout, buffer overflow and source I/O are distinct.
  Job retryability follows stable codes; raw exception/path text is not returned
  in JobProblem. Timeouts remain cooperative for trusted compiled functions.
- **R-05/R-10:** `max_lines` is an intentional completed selection, with
  `import_coverage.selection=max_lines`, rather than unsafe truncation requiring
  allow-partial. It stops before reading another logical line. Resource truncation
  still needs importer safety plus caller consent. Legacy strict/top_n behavior,
  summary/tables/correlation keys, and trailing-empty-line records are checked.
- **R-08:** Imports and Load share four slots and shutdown cancellation/waiting.
  A process-owned OS lock protects each store; only its exclusive owner collects
  crash-left staging at startup. Exact duplicate snapshots reuse a handle.
  Retention is bounded to 1,024 revisions / 512 MiB without silent eviction;
  ReleaseEvidenceResult explicitly frees an unused handle. Missing revisions
  return IMPORT_RESULT_STALE with saved-file recovery guidance. Saved exports
  are independent of this backend cache.

T-630 must still provide durable case manifests, migration/recovery of persisted
workspace references, reference ownership before releasing deduplicated handles,
user-controlled retention/deletion/restore and index lifecycle. The desktop store
remains under the OS user cache directory and can be cleared externally; a handle
alone is not a durable saved case. T-631 must wire per-load UI cancellation and
job/fallback views. These requirements are recorded in work_status.md rather than
being claimed as completed by this foundation fix. No new product/producer decision
is made for Q01–Q04.
