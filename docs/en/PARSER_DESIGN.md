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
- **JFR recording parser** (`jfr_recording.py`) detects the binary `FLR\0` magic and shells out to the JDK `jfr` CLI (explicit absolute `ARCHSCOPE_JFR_CLI`; PATH/JAVA_HOME discovery does not authorize execution) with `print --json`. The existing JSON path (`jfr_parser.py`) is unchanged. The parser also accepts mode/time-range/state filters before aggregation.
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
  allow-partial. R3 suffix inspection records selection only when a non-whitespace rune remains;
  exact EOF and blank/Unicode-whitespace-only lines have no selection marker.
  Inspection streams with fixed memory under the SourceLease input-byte budget
  and job cancellation/deadline, and stops at the first nonblank rune. It may read
  the remaining whitespace bytes but does not parse/count more physical lines or
  alter diagnostics. Invalid UTF-8 is nonblank; excluded input stays unparsed.
  Read errors remain IMPORT_IO_FAILED. Old saved coverage stays unchanged;
  reimport obtains corrected blank-tail coverage for cross-source comparison. Resource truncation
  still needs importer safety plus caller consent. Legacy strict/top_n behavior,
  summary/tables/correlation keys, and trailing-empty-line records are checked.
- **R-08:** Imports and Load share four slots and shutdown cancellation/waiting.
  A process-owned OS lock protects each store; only its exclusive owner collects
  crash-left staging at startup. Exact duplicate snapshots reuse a handle.
  Retention is bounded to 1,024 revisions / 512 MiB without silent eviction;
  R2 N-02 removes ReleaseEvidenceResult from the public Wails API/bindings
  until T-630 defines caller/Workspace ownership. Backend Release is an
  administrative deletion only, not a caller reference release. Missing revisions
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

## T-629 implementation and migration boundary (2026-11-11)

The canonical path is `profile.set-json importer → ProfileSet → analyzers/profile.BuildSet
→ AnalysisResult(profile_evidence 0.2.0)`. The internal JSON transport is registered
with schema roles, fixture hashes and a compiled analyzer at the composition root.
It needs explicit `profile_ids` and `metric_id` options. Existing formats are not
added to ProfilerService format switches; its old Analyze/loadStacks functions now
use `profilelegacy`'s explicit compatibility adapter catalog. The common generic
trie is shared by exact model operations and old integer/float presentation adapters.
This retains the large legacy input fast path without materializing a second model.
Existing collapsed/Jennifer/SVG/HTML results keep their numeric semantics/versions.

`parsers/profile.ParseLegacySet` snapshots inputs through SourceLease and wraps
available normalized records, preserving recoverable frame identity, profiles,
weights and opaque normalization metadata. Its quality block explicitly discloses
legacy semantics and missing chronology; raw legacy timestamp values are labels,
not invented measured intervals. It refuses truncated normalization. This wrapper
cannot restore fractions, zero values, event duration or multivalue data already
lost by an old producer decoder. Those decoder fixes, actual producer/version
qualification and the support-matrix corrections remain T-634. No Q01–Q04 choice
or new product support is made here.

Examples:

```sh
archscope-engine evidence import --in model.json --importer profile-set \
  --options '{"profile_ids":["worker-a","worker-b"],"metric_id":"duration"}' --out result.json
archscope-engine profile-set tree --in result.json
archscope-engine profile-set diff --in result.json --after result.json
archscope-engine profile-set drilldown --in result.json --filters '[]'
archscope-engine profile-set pprof --in result.json --tick-scale 0.01 --out selected.pprof
archscope-engine profile-set normalize-legacy --in samples.collapsed --format collapsed --out normalized.json
```

Normalization writes a disclosed legacy-normalized model, not corrected producer
support. EngineService adds NormalizeLegacyProfileSet, GetProfileSetTree,
CompareProfileSets, DrilldownProfileSet and ExportProfileSetPprof. New Wails values
cross as JSON text/handles with cancellable context; model quantities do not pass
through JavaScript numbers or legacy typed integer results. Analysis, saved JSON,
reopen, CSV/HTML, Report Pack and selected pprof share the retained selection.
Queries share the service's bounded concurrency/lifetime and use the immutable model.

Existing profile goldens remain unchanged. A baseline CLI comparison against
63d3b6a covers collapsed, Jennifer, SVG, HTML and drilldown, excluding only creation
and progress-log paths. Additional fixtures test decimals, zero, large integers,
signed/recursive/colliding frames, multiple profiles/metrics, exact scale, refused
projections, version relabeling and source/selection compatibility. Direct Wails
method and generated-binding checks do not constitute actual WebView UI execution.
Generic result screens remain T-631 and case/baseline lifecycle remains T-630/T-635.

## T-629 review remediation: bounded scale and comparison (R4)

Internal profile adapter 1.0.1 keeps ProfileSet input 1.0.0 and AnalysisResult
profile_evidence 0.2.0 numerical semantics. The parser validates one bounded model,
then emits profile-set.chunk.v1 records (<=256 KiB raw data + exact byte offset).
The analyzer reassembles under 32 MiB and validates the completed model. Common
1 MiB record, 64 MiB normalized/result and 160 MiB temp policies are unchanged;
base64 overhead counts toward spool/temp. This is bounded transport, not a fully
streaming producer decoder. Source lease consistency, backpressure, cancellation,
atomic commit and failure cleanup remain on the common path. Limits/size
amplification and budget details are listed in DATA_MODEL.

The 64 KiB detector verifies only a complete model within its probe. Larger JSON
remains a hint requiring explicit importer plus profile/metric selection and full
input validation; truncated prefix JSON never grants verified support. Shared
scale cases cover 3,000 stacks and >=50,000 tree nodes. R4 quality comparison,
legacy saved-policy compatibility and registry validation cache are specified in
DATA_MODEL; the metadata.quality copy is now authoritative-model checked. No new
producer support or entity mapping is claimed (T-634/T-635).

R5-01 preserves the wire decoder's resource error through the ProfileSet input
boundary: a single string/key above 4 MiB is a detailed budget refusal, not a
format mismatch. Exactly-at-limit and above-limit imports are tested; malformed
JSON retains format classification. R5-02 capture-key policy is explicitly handed
to T-634/T-635 in DATA_MODEL; current opaque equality is unchanged.

## T-630 safe intake and adapter boundary (2026-11-11)

`internal/cases` adds context/storage/intake around the existing analysis service;
it does not parse profiles, reinterpret weights or implement the T-631 renderer.
ZIP/TAR/tar.gz preview validates all entries and binds intake to the archive SHA256.
At most 128 entries, 16 path components, 64 MiB per file and 256 MiB compressed/
expanded archive are permitted. ZIP central-directory allocation is bounded before
using archive/zip. Split/ZIP64, encryption, links/special files, TAR extended/sparse
formats, absolute/traversing/Windows-reserved paths, Unicode/case-normalized
collisions, parent-file collisions and nested archives are refused. Content streams
must match sizes/CRC. No archive permissions, ownership or timestamps are applied.
Selected bytes are staged in bounded memory; only after the entire structural scan
passes can immutable objects and one manifest HEAD be published. Capture/explicit
hash failures are retained as per-source failed/omitted records, not successful
objects. Structural failures/cancellation publish no revision. Group only by explicit
capture file/entity references, never basename or mtime.

The separate decoder chooses explicit encoding → BOM → validated UTF-8 → explicitly
permitted legacy sniff candidates. CP949 extensions are rejected by strict EUC-KR;
ambiguous legacy inputs require selection. Lossy replacement is opt-in and counted.
UTF-16 surrogate pairs, BOM offsets and CRLF are covered. Each line records original
half-open byte coordinates and decoded UTF-8 offsets as decimal strings. The index
revision pins source digest, decoder revision, encoding and replacement count;
encoding edits regenerate it and invalidate dependent artifacts. Source pages have
at most 200 lines, 4 KiB per line and 64 KiB text. A sanitized copy is a distinct UTF-8
source with parent digest/redaction profile; its coordinates describe that copy,
never a false original offset. External originals are read-only regular files (resolve filesystem links explicitly);
a second content/stat pass refuses changes during a read. Cancellation is checked
before the HEAD commit point; an already committed revision is not rolled back. Existing legacy
textio behavior and existing parser golden tests are unchanged.

`analysis.OpenDurable` migrates the old OS cache to app data under both kernel locks.
Every result is decoded and published before the final versioned migration marker;
partial copies retry through content deduplication. Originals remain untouched and
are explicitly outside case purge scope. A completed marker prevents resurrection
of cache files intentionally removed later. Saved cases own their own immutable
result bytes and reopen through `LoadSnapshot`, independently of old handles/cache
loss. Callers acquire distinct opaque leases for shared handles and release only
those leases; administrative Release refuses any remaining owner. Public Wails
ReleaseEvidenceResult remains absent. Workspace acquires/releases its own owners,
rejects late reopen after clear, and has a compact saved-case menu. It saves backend
snapshots rather than JS projections; legacy entries without backend handles remain
in-memory. Case source binding/lifecycle/portable actions are available through CLI
and Wails; generic case presentation/discovery/search remain T-631/T-633/T-635.

CLI: `case create --title ...`, `case list`, `case show --id ...`,
`case edit --request edit.json`, `case save-result --in result.json --request refs.json`,
`case export-result --id ... --result ... --out ...`, `case preview --in bundle.zip`,
`case pack --id ... --request reviewed-selection.json --out case.zip`,
`case restore --in case.zip --sha256 <preview digest>`, `case delete-plan --id ...`,
`case purge --id ... --revision ... --token ...`, `case rebuild-index`,
`case cleanup-cache`. `--store` selects an isolated local store. Edit JSON uses
case_id/expected_revision and one explicit operation: details, retention, workspace,
capture, source, replace-source, encoding, omit-source, archive-intake,
review-source, review-result, artifact, baseline, archive, restore or delete.
SaveResult refs use case_id/expected_revision/source_ids/capture_id/selection/
options_hash/versions; source_ids may be empty for a snapshot-only save. Portable
selection uses manifest_reviewed/source_ids/result_ids/include_workspace. No export
or restore auto-approves content for sharing. Wails exposes these requests/results
as JSON text; typed lease IDs are strings. Go bridge tests are not WebView IPC/UI
execution or Windows runtime evidence. No independent review PASS is inferred.

Case export destinations cannot overwrite the managed case store or current external original references/aliases.

## T-631 generic consumer integration (2026-11-11)

`AnalyzeEvidence` shares the existing `ImportEvidence` catalog/job pipeline;
legacy `Analyze*` endpoints keep their contracts. A same-family adapter adds its
package/schema/fixtures/registration and reuses its result descriptor and generic
consumer without a new service/CLI/UI format switch. A new semantic provider
requires a compiled declaration and payload conformance tests, not metadata claims.
Timeline and SLI schemas are available as bounded row projections; no new overlay,
SLO policy or cross-source stitching algorithm is implied by displaying them.

CLI adds `evidence analyze` (import alias), `evidence describe --in result.json`,
`evidence query --in result.json --provider ID --offset 0 --limit 25`, and
`evidence compare --in baseline.json --target target.json --provider ID`. Structured
comparison refusal is output data, not a fabricated zero. Wails adds cancellable
`OpenEvidenceResult`, `DescribeEvidenceResult`, `QueryEvidenceProvider` and
`CompareEvidenceProvider`; the prior load/export APIs remain compatible.

Analysis Workspace offers catalog import/options/explicit partial opt-in,
ambiguous-format selection, job state/cancel, saved-result open, provider selection,
text-only summary/table/diagnostic/detail views, pagination, ordered comparison,
exact JSON export, and explicit Workspace insertion. Saved-case snapshots use the
same renderer. It owns one lease, releases late acquisitions, cancels each load/query
through its Wails context, rejects superseded completions and validates page handle/
digest/provider. A late Start response still receives CancelImport. Stale/corrupt
handles offer saved-file or saved-case reopen. Backend-less legacy Workspace entries
retain their existing display; they are not serialized as exact backend snapshots.
Native Windows testing is owned by the separate Windows-PC run. Unit/Go bridge,
state/build and CLI checks alone do not certify actual WebView interaction.

## T-630 R6 source paging and recovery (2026-11-11)

`Store.SourcePage` uses one revision/source-digest/options-bound compact index:
up to 64 MiB decoded UTF-8 and 1,000,000 offsets (16 bytes each). Repeated pages
reuse decoding and verify the current source with a bounded streaming hash, rather
than trusting mtime or allocating the full input again. Changing even same-size/
mtime-spoofed source bytes fails closed. Case edits, deletion, cleanup and store
close discard derived text. Decoder coordinates and `text-index-v1` identity for
previous valid inputs are preserved across UTF-8/UTF-16/CP949/EUC-KR.

The entire bounded input is validated and counted even when the index prefix is
full. `total_lines`, `indexed_lines` and `index_truncated` disclose this limit;
the first million lines remain readable instead of refusing the whole file. A
request beyond the indexed prefix returns no rows with the explicit truncation
flag, not an assertion of complete EOF. Original bytes and export remain complete.
T-635 must honor this coverage or add byte-window access before broader source-link
claims. Pages retain 200-line/4 KiB-per-line/64 KiB-text ceilings. The older full
`ReadSource` API is not the Wails paging path and may allocate a larger typed index.

New maintenance endpoints are `CleanupEvidenceCache`, `GetCorruptCaseDeletionPlan`
and `PurgeCorruptCase`. Corrupt deletion requires inspection of a fresh plan and
its exact token: `case corrupt-delete-plan --store PATH --id ID`, then, only after
reviewing scope, `case purge-corrupt --store PATH --id ID --token TOKEN`. Normal
case lifecycle remains separate. CLI cache maintenance asks the user to close the
owning desktop app. No cleanup or purge runs automatically against user cases.

### T-631 R8 consumer boundary

Reference-set preparation and private projection/column caching belong to the
result contract and analysis service, not parsers or analyzers. Display-only
control escaping never rewrites normalized records, metric names, source identity
or original exports. The cache is transient; durable source/search/baseline indexes
remain T-635. See [Data model](DATA_MODEL.md#t-631-r8-review-remediation-2026-11-12).

## T-633 input assistance boundary (2026-11-12)

`inputassist` owns bounded, snapshot-backed hints and explicit capture-file binding.
It does not parse results or grant support from an extension/recipe. `localconvert`
owns installed-tool execution, fixed arguments, streaming limits, cancellation and
provenance; JFR parsing and analysis retain their separate responsibilities.
The generic catalog and common AnalysisResult contract remain unchanged.
See [Input assistance](INPUT_ASSISTANCE.md) for limits, refusal codes and manual
kit/CLI/UI usage. Runtime assets are embedded; no sibling checkout is needed.

### T-634 original-source semantic revision

The exact source normalizer adds `org.archscope.profile_source.v1` revision
1.0.0 to ProfileSet schema 1.0.0 and emits AnalysisResult profile_evidence 0.3.0.
Old source facades/results and v2 hashes keep their semantics. The new generic
transport is `profile.source-set-json` (alias profile-source-set), disjoint from
profile.set-json / payload 0.2.0. Every profile and value/unit choice survives;
selection is explicit. Compiled fixture-qualified capture policies and v3 canonical
comparison retain both original options and incidental-difference disclosure.
Unknown bindings stay strict; generic scalar consumers stay on strict capture
identity pending T-635. This supersedes the implementation-pending R5 statement
only for these qualified canonical source slices. See
[Exact profile sources](PROFILE_SOURCE_IMPORT.md) for format semantics, bounds,
qualification, policy keys and CLI/desktop behavior.

### T-635 canonical queries, verified baseline and R11 revision

[Case evidence queries](CASE_EVIDENCE_QUERIES.md) defines bounded cursor-bound
profile views/evidence, source/summary case search with indexed coverage, original
coordinate links, immutable selected results, and baseline replay/mapping gates.
R11 correction emits source normalizer 1.1.0 / result 0.4.0: original type/unit is
part of metric identity without a producer policy; unitless Speedscope weights
are generic count, not proven samples. Source 0.3.0 snapshots keep old bytes and
hashes/read/save/query/export, but comparisons require new original reimport.
New compiled policy snapshots disclose declared/file-exporter basis and warnings.
Durable verified reuse replays both originals on a new query and rechecks both
original/result byte hashes and full case bindings on every page, preserving
identity_basis=declared. R12 permits only a bounded process-local validated diff
cache; imported receipts never grant verification. Existing generic scalar comparison stays strict; the new
canonical/baseline consumer uses qualified policies. Clock alignment and unknown
producer promotion are not inferred. Native Windows/UI and independent review
remain separate from local implementation verification.

### T-638 metric history

[Exact metric history](METRIC_HISTORY.md) defines MetricHistory model/normalizer
1.0.0, `metrics.history-json` transport and `metrics_timeseries` payload 0.2.0 (saved 0.1.0 reader retained).
Neutral snapshot sequences, explicit-provenance Prometheus range matrices and
OTLP JSON preserve exact values, resource/scope/schema/attribute identity and
temporality. Delta/cumulative/reset/gap/overlap and explicit/exponential buckets
remain distinct. Generic detail/timeline and saved-result query/export share the
common envelope. SDK Python 1.38.0 synthetic fixtures qualify only that slice;
Q04/GenAI aliases and native acceptance stay separate. Legacy metrics_snapshot
semantics are unchanged; there is no automatic cross-series sum or quantile math.

## T-637 system evidence adapters

`system.evidence-json` accepts SystemEvidence 1.1.0 canonical JSON through ≤256 KiB
chunks, with compiled shape/semantic checks and digest-pinned bundled fixtures.
Original sources use explicit perf-script/events-jsonl/bpftrace-jsonl normalization
with capture context; parsing never executes tools. `system_evidence` 0.2.0 owns
raw observations, separate scheduler interval analysis and original aggregate
buckets. [System evidence](SYSTEM_EVIDENCE.md) defines exact fields/version slices,
lifetime/loss/clock refusals, bounds and source drilldown. Existing profile and
metric readers are preserved.


## T-639 device and serving evidence

Limited `inference.evidence-json` carries InferenceEvidence 1.0.0 into common
`inference_serving` 0.1.0. Original MetricHistory is stored once, signals bind
explicit series/entity/context/unit/form/population, and missing values remain
unavailable. Native NVIDIA CSV uses an explicit fixed query contract; fixtures
are synthetic and Q03 version-specific vLLM aliases remain gated. Parser, semantic
analyzer and generic/export consumers remain separate. See
[offline device/serving contract](INFERENCE_EVIDENCE.md) for bounds, precise
request formulas, original metric drilldown and unexecuted native acceptance.

SystemEvidence 1.0.0 / system_evidence 0.1.0 saved readers remain supported; new 1.1.0 models deduplicate frames/stacks and 0.2.0 results retain old snapshots unchanged.

## T-640 implementation boundary (2026-11-12)

The limited `trace.llm-json` importer normalizes LLMTrace 1.0.0 into existing trace_import 0.3.0 / llm_core 1.0.0. Legacy trace 0.2.0 remains unchanged. Parent/link, observed child-union durations, explicit logical/attempt accounting, token boundaries and lifecycle populations stay separate. Body-free normalization precedes case copies/indexing. Exact bounded query/UI and existing providers/exports are reused. Q01 vendor JSON adapters stay gated. See [LLM trace evidence](LLM_TRACE_EVIDENCE.md). T-639 R16 adds explicitly declared untyped range metadata and user event-ID normalization, with 1.0/1.1 model and 0.1/0.2 result readers preserved; see [inference evidence](INFERENCE_EVIDENCE.md).


## T-641 runtime evidence

Runtime evidence uses the separate bounded canonical model within AnalysisResult. See [RUNTIME_EVIDENCE.md](RUNTIME_EVIDENCE.md) for pinned producer formats, identity/window qualification, live allocation profile reuse, GC union, plan per-loop semantics and saved-reader boundaries.


## T-642 execution evidence

Bounded benchmark/crash context uses execution_core 1.0.0 inside common AnalysisResult, execution_evidence 0.1.0. Pinned k6/LLMPerf and structural JTL/actual sanitized HotSpot inputs normalize before generic providers/cases/exports. Comparison is explicitly declared-condition snapshot verification, separate from profile original-reimport baselines. See [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md).

T-643 explicitly recognizes apple-ollama-capture.v1, validates closed bounded exports, and projects exact metrics before existing generic import. Import never runs manual collection. [APPLE_SILICON_EVIDENCE.md](APPLE_SILICON_EVIDENCE.md).

R19-01 adds execution_core 1.1.0 / execution_evidence 0.2.0 for exact large-JTL distributions; saved 1.0.0/0.1.0 stays unchanged. [EXECUTION_EVIDENCE.md](EXECUTION_EVIDENCE.md).

T-644: trace.gpu-execution-json / trace_import0.4.0 / gpu_execution_core1.0.0 preserves exact scoped CPU/GPU/rank intervals, explicit correlations, separate kernel sum/union and summary-only evidence. CLI/Wails/Workspace and bounded selected SVG reuse common consumers; existing saved trace/LLM readers remain. [GPU_EXECUTION_EVIDENCE.md](GPU_EXECUTION_EVIDENCE.md).


T-645 bounded actual DB/OS snapshot batch: [Manual database and socket snapshots](SYSTEM_DB_SNAPSHOTS.md). Existing MetricHistory transport, exact epochs/gauges and common consumers; other adapters remain fixture-gated.
