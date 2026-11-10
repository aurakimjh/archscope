# Other-feature correctness audit — 2026-11-10

Baseline: `406038d11018c81068088ef2003b843c2ad61ed0`. Review only; no implementation fixes.

Focused coverage: Trace Import, access-log latency, Incident Timeline and Report Pack.
Multi-thread identity and CSV/report exporters were inspected as secondary candidates.
HTTP Analyzer, recent GC/flamegraph fixes and T-557 were outside this audit.
Evidence: `../evidence/2026-11-10-other-features/local-reproductions.json`.

## Confirmed findings, in execution order

| Task | Priority | Finding and evidence | Required correction and acceptance |
|---|---|---|---|
| T-614 | P1 | `reportPack.ts:176-200` reuses the latest derived artifact by type/time only. After building a timeline from REMOVED-SOURCE and replacing the source with CURRENT-SOURCE, the exported pack lists only CURRENT-SOURCE while the timeline still references REMOVED-SOURCE. | Match source IDs, content/version and relevant options before reuse, or explicitly carry a historical snapshot with complete provenance. Test source removal/replacement/addition and current custom SLO targets; valid customized artifacts must not be silently discarded. |
| T-615 | P1 | `traceimport/analyzer.go:395-417` adds inclusive parent/child durations. A wholly contained root 100ms / client 80ms / server 70ms chain produces `critical_path_ms=250`, while the trace is 100ms. Reproduced through Zipkin parsing and Build. | Define elapsed critical-path semantics using intervals/self-time; distinguish cumulative work if retained. Test nested, sequential and parallel children, missing timestamps, skew and malformed graphs. Update downstream SLO/timeline/report projections together. |
| T-616 | P1 | `traceimport/analyzer.go:216-243` counts the remote-service CLIENT span and its cross-service SERVER child separately. One RPC becomes two calls and 150ms total, averaging client 80ms and server 70ms to 75ms. | Count correlated CLIENT/SERVER representations once per logical request and define timing/error authority. Preserve server-only/client-only exports and distinct retries; do not deduplicate solely by service pair. Test downstream Service Flow/SLI rates. |
| T-617 | P1 | `accesslog/parser.go:265-272` leaves absent durations as zero; `accesslog/analyzer.go:394` and URL/minute/static/API accumulators include them. One unknown plus one 1000ms request yields average 500ms and p95 950ms, with no warning. | Carry availability and measured/unknown counts, exclude missing durations from latency statistics while keeping request/error/byte counts. Test real zero, all missing, mixed inputs and downstream SLI/SLO availability. |
| T-618 | P2 | `incidentTimeline.ts:117-133,269` drops per-source events after 120 and session events after 1000 without partial-result accounting. Of 130 time-ordered events, a critical last event disappears; output reports zero critical events and an earlier latest time with no truncation marker. | Keep deterministic bounded output but disclose total/emitted/omitted counts and selection policy. Preserve overall summaries or qualify them as retained-only. Test both caps and a severe late event through UI, JSON, HTML and Report Pack. |

All five are actionable on Mac. This review does not modify implementations or claim native-window/Windows execution.
Existing targeted Go suites and frontend state tests pass; they did not detect these cases.

## Secondary candidates not promoted as bugs

- CSV currently exports summary/series by its explicit documented contract. Adding canonical `tables` export is a feature extension; it was not classified as a new defect here.
- Multi-thread correlation deliberately keys on thread names and documents same-name merging. A stronger runtime/process/thread identity policy merits a separate design review; this audit did not prove a new acceptance-contract failure.

## Semantic sources

Nested spans denote sub-operations and carry timestamps; CLIENT and propagated child SERVER spans describe the two sides of a request. The numeric failures above were measured locally, not inferred solely from these documents.
[OpenTelemetry span concepts](https://opentelemetry.io/docs/concepts/signals/traces/#spans) · [SpanKind API](https://opentelemetry.io/docs/specs/otel/trace/api/#spankind).
