# ArchScope Research Decisions

Last updated: 2026-04-28

## Scope

This document records acceptance decisions for research findings extracted from:

- `docs/research/2026-04-28_Competitor_Analysis_and_Positioning_Gemini.md`
- `docs/research/2026-04-28_claude-code_product-enhancement.md`

Decision states:

- `Accepted`: Valid and should be reflected in design, roadmap, or execution backlog.
- `Deferred`: Valid, but later than the current planning horizon or blocked by earlier work.
- `Rejected`: Not aligned with ArchScope direction.
- `Needs Decision`: Requires product or user direction before implementation.

Disposition targets:

- `Design`: Capture in architecture, data, parser, chart, or export design docs.
- `Roadmap`: Add or clarify roadmap direction.
- `TO-DO`: Add to `work_status.md` execution backlog.
- `Existing`: Already covered by current decisions or backlog.

## Decision Summary

| ID | Research item | Decision | Target | Priority | Rationale | Follow-up |
|---|---|---|---|---|---|---|
| RS-001 | Privacy-first local professional positioning | Accepted | Design, Roadmap | P0 | This is the clearest market gap versus SaaS JVM tools and server-based observability stacks. It also matches the Electron + Python sidecar architecture. | Add positioning to architecture docs and keep packaging work aligned with local-only operation. |
| RS-002 | Cross-evidence timeline correlation as differentiator | Accepted | Roadmap, Existing | P4 | Correlating GC, access logs, profiler data, JFR, and thread evidence is a stronger differentiator than isolated viewers. | Keep `T-028`; clarify roadmap language around cross-evidence correlation. |
| RS-003 | Extensible diagnostic SDK + UI positioning | Accepted | Design | P2 | The `AnalysisResult` contract and parser/analyzer separation support this direction. | Add architecture language that future parser additions should preserve SDK-like extension boundaries. |
| RS-004 | Evidence-backed AI interpretation | Accepted | Roadmap, Existing | P5 | AI interpretation is useful only if every claim traces back to evidence. This is already aligned with RD-024. | Keep `T-029`; add optional local LLM/Ollama design item for Phase 5. |
| RS-005 | Engine-UI Bridge PoC remains top bottleneck | Accepted | Existing | P0 | This repeats the existing P0 path and should not create duplicate work. | Continue `T-002` and `T-003`. |
| RS-006 | Python runtime dependencies and console script must be explicit | Accepted | TO-DO | P0 | The bridge depends on a reliable CLI install path. Missing `typer`/`rich` declarations can break setup and packaging. | Add Phase 1 task to declare runtime dependencies and `archscope-engine` script metadata. |
| RS-007 | Full Python packaging modernization | Accepted | Existing, TO-DO | P3 | Worth doing, but less urgent than declaring dependencies and Bridge PoC. | Keep packaging cleanup under `T-024` and `T-025`; revisit backend choice during packaging spike. |
| RS-008 | `iter_text_lines` encoding fallback can duplicate lines after mid-file decode failure | Accepted | TO-DO | P1 | This is a data integrity bug and should be fixed before parser diagnostics are trusted. | Add Phase 1 task before malformed parser implementation. |
| RS-009 | Parser diagnostics implementation | Accepted | Existing | P1 | Already approved and now backed by parser policy docs. | Continue `T-004` to `T-007`. |
| RS-010 | Focused test coverage expansion | Accepted | Existing | P1 | Existing backlog already covers statistics, malformed parser, and JSON exporter tests. | Continue `T-006` to `T-009`. |
| RS-011 | Electron supported-version upgrade | Accepted | TO-DO | P2 | Official Electron release notes show Electron 41 is current in 2026 and older majors have reached end-of-support. Upgrade after Bridge PoC to avoid mixing process-model changes with dependency churn. | Add Electron upgrade task after `T-003`. |
| RS-012 | ECharts 6 upgrade and dark mode/SVG export capabilities | Accepted | Roadmap, TO-DO | P2 | Official ECharts 6 features align with report-ready charts: dynamic themes, dark mode, broken axis, custom charts, and SSR/SVG export direction. | Add Phase 2 chart upgrade task and roadmap item. |
| RS-013 | JFR recording parser | Accepted | Roadmap, TO-DO | P4 | OpenJDK JEP 509 strengthens JFR's relevance for CPU-time profiling. Binary parsing complexity means this should start as a design spike after core JVM parsers. | Add JVM roadmap item and advanced diagnostics design task. |
| RS-014 | OpenTelemetry log input and trace context support | Accepted | Roadmap, TO-DO | P4 | OTel Logs Data Model is stable and includes trace/span context fields useful for correlation. | Add roadmap item and observability input design task. |
| RS-015 | electron-builder + PyInstaller distribution pipeline | Accepted | Existing | P3 | Already aligned with Architecture and `T-022`; no duplicate task needed. | Keep `T-022`; clarify roadmap distribution direction. |
| RS-016 | Optional local LLM/Ollama integration | Deferred | Roadmap, TO-DO | P5 | Useful for privacy-first AI, but only after stable evidence models and correlation. | Add Phase 5 roadmap detail and optional design task dependent on `T-029`. |
| RS-017 | Access Log advanced diagnostic rules | Accepted | TO-DO | P2 | Status distribution exists, but report-grade diagnostics need explicit rules and findings beyond raw charts. | Add task for Access Log rule enrichment. |
| RS-018 | React 19 upgrade | Deferred | Roadmap | P3 | React 18 is stable and not the immediate risk; Electron upgrade has higher security value. | Revisit after Electron upgrade and packaging hardening. |
| RS-019 | Broad cloud observability comparison | Accepted | Design | P0 | Useful for positioning, but not a feature task. | Reflect as local/offline/report-ready positioning only. |

## External Verification

Claims that depend on current external versions or standards were checked against primary sources:

| Topic | Verification | Source |
|---|---|---|
| Electron current major and support context | Electron 41 release notes list Chromium 146 / Node 24 and note older major end-of-support policy behavior. | `https://www.electronjs.org/blog/electron-41-0`, `https://releases.electronjs.org/release/v41.2.1` |
| ECharts 6 feature relevance | ECharts 6 officially includes dynamic theme switching, dark mode, broken axis, beeswarm, custom charts, and related charting upgrades. | `https://echarts.apache.org/handbook/en/basics/release-note/v6-feature/` |
| OpenTelemetry log correlation | OTel Logs Data Model is stable and defines trace context fields such as TraceId and SpanId. | `https://opentelemetry.io/docs/specs/otel/logs/data-model/`, `https://opentelemetry.io/docs/specs/otel/compatibility/logging_trace_context/` |
| JFR CPU-time profiling relevance | OpenJDK JEP 509 is delivered in JDK 25 and adds experimental JFR CPU-time profiling on Linux. | `https://openjdk.org/jeps/509` |

## Backlog Mapping

| Research decisions | Work status task IDs | Notes |
|---|---|---|
| RS-005 | T-002, T-003 | Existing Engine-UI bridge work. |
| RS-006 | T-030 | New Phase 1 dependency declaration task. |
| RS-008 | T-031 | New Phase 1 encoding fallback bug task. |
| RS-009, RS-010 | T-004 to T-009 | Existing parser diagnostics and test work. |
| RS-011 | T-023 | Existing dependency task updated to supported Electron upgrade. |
| RS-012 | T-033 | New ECharts 6 chart upgrade task. |
| RS-013 | T-034 | New JFR parser design spike. |
| RS-014 | T-035 | New OpenTelemetry input design task. |
| RS-016 | T-036 | New optional local LLM design task. |
| RS-017 | T-032 | New Access Log diagnostics rules task. |
