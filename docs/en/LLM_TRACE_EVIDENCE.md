# Offline LLM, gateway and lifecycle trace evidence

LLM trace support extends the existing `trace_import` result with payload 0.3.0 and an
`llm_core` 1.0.0 extension. The common AnalysisResult envelope, case persistence,
generic provider tables, timeline and visual/report export paths are reused.
Legacy trace payload 0.2.0 keeps its existing reader and dedicated screen.

## Qualified inputs

Normalize OTLP JSON/JSONL (optionally gzip) with an explicit trace namespace,
clock basis and semantic profile. The limited `trace.llm-json` importer (alias
`llm-trace`) accepts the resulting body-free LLMTrace 1.0.0 model with `{}`
options. No online API, SDK injection, automatic collection or vendor JSON reader.

| Profile | Qualified scope |
|---|---|
| `neutral.v1` | Explicit `diag.llm.*`, `diag.gateway.*`, `diag.lifecycle.*` attributes in the bundled profile mapping. |
| `otel-genai.v1.37.0` | Selected operation/provider/request model/input and output token fields from the pinned convention. Not every convention field or version. |
| `openinference.core.v1` | Explicit subset of span kind/model/token fields. This is a versioned local subset contract, not qualification of every OpenInference library version. |

A fixture was generated using actual OTel Python SDK 1.38.0 span objects and its
OTLP protobuf encoder, with deterministic synthetic data and no network exporter.
The protobuf JSON conversion replaces base64 IDs with OTLP JSON hex IDs. Separate
synthetic convention/edge fixtures test semantic mapping. These do not qualify
Langfuse/LangSmith product exports; **other Q01 vendor/version routes remain open**. Shared manifests pin
source, schema and expected model bytes; production code embeds the required
mapping and fixtures, so no sibling checkout or Internet is needed at runtime.

## Privacy and provenance

Normalization retains technical resource/scope identity, trace/span/parent IDs,
links, source hash/position, exact timestamps, selected role/model/token metadata,
explicit request/attempt identities and recognized structural lifecycle events.
Raw span names, arbitrary attributes, prompt/completion, tool arguments, status
messages and unknown event bodies are excluded before result creation and case
attachment. There is no body opt-in in this version. Do not put content or secrets
in technical identifiers or model/provider IDs: those allowlisted fields must
already be sanitized. The item count records excluded attributes, names/messages
and unknown events, not every raw JSON field.

Raw LLM carriers recognized by the supported semantic prefixes are refused by
legacy trace analysis and new case source intake. Case text indexing also refuses
such raw sources, including escaped keys and gzip carriers. Normalize before
copying, indexing or exporting; only attach the normalized model and result.
This guard recognizes the declared supported contracts, not arbitrary unknown
vendor content. Existing raw archives are not retroactively rewritten or scrubbed.
The source hash proves byte identity, not provenance authenticity or sanitization.

## Timing, graph and accounting

- Parent and link relations remain separate. Duplicate span observations are
  preserved without choosing a winner. Missing/ambiguous parents, cycles, reversed
  time and unknown clocks have explicit statuses. No invented parent or time join.
- Duration is observed end minus start. `observed_self_ns` subtracts the union of
  known, contained direct-child intervals once, avoiding nested overlap double
  counting. Incomplete/cyclic/ambiguous graphs do not justify an exclusive time.
  This elapsed observation is neither CPU self time nor a causal critical path.
- Logical requests require a declared request namespace and logical ID; attempts
  also require an attempt ID. Initial/retry/fallback/previous-attempt edges are
  declared; missing or cyclic predecessor chains are flagged. Time proximity,
  matching model names or span links never reconstruct identities.
- A unique direct CLIENT→SERVER pair with the same explicit attempt counts as one
  attempt. Repeated/ambiguous observations are flagged. Token authority for a
  pair must explicitly choose one side. A singleton can expose its one token
  observation. No cross-attempt/client-server token sum, cost or billing estimate.
- Model/revision/tokenizer/method, provider/route, boundary, outcome and population
  stay on each observation. Logical/attempt projections show all declared models
  and populations; they do not grant comparison compatibility.
- Cold/warm/warmup/recovery/unknown are explicit populations, not inferred from
  nearby events. Startup/model load/compile/cache/restart/replacement/recovery
  events remain visible even when untimed; requests are never silently excluded.
- TTFT uses explicit request start and first-token boundary. TPOT is
  `(last-first)/(output_tokens-1)` only with at least two tokens and distinct,
  ordered token boundaries. Exact nonterminating values use rational strings.
  Missing/conflicting/equal boundaries are null/status. Network/SSE chunks are not
  tokens. Token events are recognized only by the pinned profile event names.

## Desktop and CLI

In Workspace select the sanitized OTLP source and expand **LLM / gateway trace
evidence**. Choose the exact profile, enter a trace namespace and select `unix`
only when the supplied timestamps share that clock; otherwise leave `unknown`.
Inspect, save a new model, then Analyze using `trace.llm-json` and `{}`. Do not add
the raw carrier as a case source. The **LLM spans, attempts and lifecycle drilldown**
shows bounded pages of spans, full sanitized records, parent/link relations,
logical requests, attempts or events. Trace ID, record ID and explicit request
identity filters are exact. Context reveals service/instance and instrumentation
scope/version/schema. Parent-result digest and query selector bind every cursor;
changing selection cancels stale UI work. Provider tables and timeline can be
used in existing generic HTML/Report Pack and selected table visual exports.
No specialized trace raster, native clipboard/Office or new trace engine is claimed.

```sh
archscope-engine trace llm-inspect --in source.json --options-file options.json
archscope-engine trace llm-normalize --in source.json --options-file options.json --out llm-trace-set.json
archscope-engine evidence import --in llm-trace-set.json --importer trace.llm-json --options '{}' --out result.json
archscope-engine trace llm-query --in result.json --query '{"kind":"attempts","limit":25}'
archscope-engine evidence export --in result.json --export-format report-pack --out report.zip
```

Example options:

```json
{"semantic_profile":"neutral.v1","trace_namespace":"sanitized-capture-a","producer":"declared collector","producer_version":"1","clock":"unix"}
```

`trace llm-import` combines normalization and result construction. New destinations
refuse overwrite and managed stores. Original/expanded files ≤64 MiB each;
canonical models ≤32 MiB; results/snapshots ≤64 MiB; spans ≤50k, contexts ≤4096,
links and recognized events ≤100k each, JSON envelopes ≤128. Normalization admits
two calls with a five-minute limit. Queries return ≤100 rows (UI 25) and bounded
cursors. Byte limits may bind before record counts; split the original by trace,
never truncate silently. 10k/20k deep-chain synthetic models and saved readers
are locally exercised. Native Wails/Windows and actual vendor exports remain
separate acceptance evidence.

Primary definitions: [OTel GenAI 1.37.0](https://github.com/open-telemetry/semantic-conventions/blob/v1.37.0/docs/gen-ai/gen-ai-spans.md),
[OTLP trace protocol 1.7.0](https://github.com/open-telemetry/opentelemetry-proto/blob/v1.7.0/opentelemetry/proto/trace/v1/trace.proto),
[OpenInference conventions](https://github.com/Arize-ai/openinference/blob/main/spec/semantic_conventions.md).

Context inventory is capped at 20; use the contexts query for bounded pages of every matching instrumentation context.

The legacy raw trace analysis facade now refuses originals above 64 MiB with an explicit split instruction, so a partial privacy scan cannot admit an uninspected tail. Saved legacy readers are unchanged; decoded semantic keys are checked again before publishing a legacy result.


<a id="r17-intake-and-query-constraints"></a>

## intake and query constraints

Raw-carrier refusal recognizes known GenAI/OpenInference/diag/Traceloop/Langfuse
and input.value/output.value markers, including escaped/gzipped carriers. This is
best-effort detection with possible false positives for ordinary attribute names;
it is not complete arbitrary-body detection or a Langfuse/LangSmith adapter.
Sanitize unsupported exports before case copy. Queries serialize only the requested
page and the service reuses verified graph/model projections in a two-entry,
two-minute, 96 MiB conservative JSON charge cache; each request rechecks result
ownership and stored-file integrity. CLI commands remain cold processes.

The reviewed approximately 950-byte/span shape reaches 32 MiB near 35k spans,
before the nominal 50k record ceiling. Split the original export by trace before
normalization; there is no in-app trace-selection operation for an oversized model.

| Profile | First/last-token timing capability |
|---|---|
| neutral.v1 | Explicit diag.llm first/last boundaries or recognized token events. |
| otel-genai.v1.37.0 / openinference.core.v1 | Preserve profile fields; first/last timing requires the pinned custom boundaries/events. Ordinary total span duration is not TTFT/TPOT. |

`diag.llm.first_token_unix_nano`, `diag.llm.last_token_unix_nano`, `diag.llm.first_token`, `diag.llm.last_token`.

New normalization records token_timing_policy=token-boundaries.1.1.0. Contradicting supplied boundaries (outside span or first>last) clear both TTFT/TPOT with token_boundary_order_invalid, regardless of output token count. Missing policy preserves legacy saved projections; equal first/last does not provide distinct TPOT boundaries.

| Semantic profile | TTFT/TPOT capability |
|---|---|
| neutral.v1 | Requires explicit diag.llm first/last attributes or events and qualified span/clock/token count |
| otel-genai.v1.37.0 | Standard GenAI usage fields do not imply timing; explicit diag.llm extension required |
| openinference.core.v1 | Standard OpenInference spans do not imply timing; explicit diag.llm extension required |

## Langfuse 4.79 observations API selection (Q01)

Select semantic profile `langfuse-observations.v4.79`, producer `langfuse`, version `4.79.0`, an explicit namespace and clock `unix`. It accepts a selected body-free observations API v2 JSON page, not legacy trace ingestion events or LangSmith exports. Local self-hosted 4.79.0 was actually started with pinned Docker images; three synthetic OTLP observations were persisted and read through `GET /api/public/v2/observations?fields=core,basic,usage,model`. No external model/service or cloud account was used.

`export-langfuse-observations.py --trace-id HEX --out new-export.json` reads loopback hosting and credentials from LANGFUSE_HOST / LANGFUSE_PUBLIC_KEY / LANGFUSE_SECRET_KEY environment, selects body-free fields, and allowlists before disk. Never store keys in evidence. Nonempty pagination metadata is refused; export a selected page deliberately. The result does not claim full trace/collection coverage.

Only explicit usageDetails.input/output are measurements. API inputUsage/outputUsage defaults are ignored, so absent counts stay unavailable and explicit zero stays zero. GENERATION/RETRIEVER/EMBEDDING/SPAN map to roles; names, input/output, arbitrary metadata, project/user/session and costs are excluded. Boundary/outcome/token authority remain unknown unless actually represented; level ERROR maps to failed. Derived latency/TTFT fields do not invent token timestamps. Existing LLMTrace1.0.0/trace_import0.3.0 queries, original coordinate refs and common exports apply; old OTLP readers and saved bytes remain supported. This pinned slice does not qualify LangSmith, other Langfuse versions or client/server token deduplication.
