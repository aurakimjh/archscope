# ArchScope Review Decisions

Last updated: 2026-04-28

## Scope

This document records acceptance decisions for review findings extracted from:

- `docs/review/done/2026-04-28_Architecture_and_Implementation_Review.md`
- `docs/review/done/2026-04-28_claude_opus_architecture_review.md`

Decision states:

- `Accepted`: Valid for the current direction and should be implemented.
- `Deferred`: Valid, but not required for the current stabilization cycle.
- `Rejected`: Not aligned with the current product or engineering direction.
- `Needs Decision`: Requires product or user direction before implementation.

## Decision Summary

| ID | Review item | Decision | Priority | Target phase | Rationale | Next action |
|---|---|---|---|---|---|---|
| RD-001 | Define Engine-UI bridge approach | Accepted | P0 | Foundation Stabilization | The Desktop UI currently uses mocked analyzer data, so a real integration path is required before chart and workflow expansion. Electron IPC plus Python CLI child process is the most practical initial bridge for a desktop-first tool. | Document IPC + `child_process.execFile` bridge and implement minimal PoC. |
| RD-002 | Build minimal Bridge PoC | Accepted | P0 | Foundation Stabilization | The core product path is `Desktop UI -> Python Engine -> AnalysisResult JSON -> ECharts`. A small end-to-end PoC will validate process execution, JSON exchange, and UI rendering before broader UI work. | Add sample-file selection, invoke Python CLI from Electron main process, return JSON to renderer. |
| RD-003 | Define analyzer client boundary before replacing mocks | Accepted | P0 | Foundation Stabilization | Keeping UI mocks is useful, but the API contract must be explicit so the mock and real IPC client can be swapped without page rewrites. | Define analyzer client interface and provide mock plus real implementation boundary. |
| RD-004 | Handle malformed access log lines with diagnostics | Accepted | P1 | Foundation Stabilization | Silent skips hide data quality issues and weaken diagnostic trust. The parser should skip invalid lines gracefully and report skipped counts in metadata. | Add skipped-line count, diagnostics metadata, and malformed-line parser tests. |
| RD-005 | Handle malformed collapsed stack lines | Accepted | P1 | Foundation Stabilization | `rsplit(maxsplit=1)` can fail on malformed non-empty lines. Profiler analysis should degrade gracefully rather than aborting the whole run. | Add invalid-line handling and metadata/reporting for collapsed parser. |
| RD-006 | Strengthen AnalysisResult contracts with type-specific keys | Accepted | P1 | Foundation Stabilization | `dict[str, Any]` is flexible but too loose as analyzers grow. Type-specific contracts reduce drift between Python output and TypeScript UI expectations. | Start with Python `TypedDict` for Access Log and Profiler result sections; align TypeScript types. |
| RD-007 | Adopt full Pydantic model validation | Deferred | P2 | Contract Hardening | Runtime validation is useful, but a full migration is larger than needed while the project has only initial analyzer types. TypedDict is a lower-cost first step. | Revisit after TypedDict contracts and bridge JSON flow stabilize. |
| RD-008 | Add large-file sampling options | Accepted | P2 | Large File Strategy | Large production logs can be too expensive for full in-memory analysis. Sampling is a practical short-term mitigation before streaming aggregation is complete. | Add max-lines and time-range options to analyzer inputs. |
| RD-009 | Refactor access log analysis toward streaming aggregation | Deferred | P2 | Large File Strategy | The risk is real, but streaming changes analyzer internals and should follow the simpler malformed-line and metadata work. | Design incremental counters/time buckets, then refactor after bridge PoC. |
| RD-010 | Document large-file streaming strategy in parser docs | Accepted | P2 | Large File Strategy | The implementation path should be explicit before deeper parser changes, especially around memory bounds and metadata reporting. | Update `PARSER_DESIGN.md` in English and Korean. |
| RD-011 | Add global UI state management such as Zustand | Deferred | P3 | UI Expansion | Current UI state is still simple. Introducing global state before the real engine data flow is known would add premature structure. | Reassess after Bridge PoC defines actual result, progress, and error flows. |
| RD-012 | Convert chart settings to JSON templates | Deferred | P2 | Chart Studio | ECharts option builders already provide a workable separation. Template extraction is better aligned with later Chart Studio or dynamic chart loading work. | Keep current builders, then extract templates during Phase 2 chart work. |
| RD-013 | Convert `App.tsx` page rendering to mapping table | Accepted | P2 | UI Structure | This is a low-risk maintainability improvement that makes page additions cleaner without requiring React Router. | Replace repeated conditionals with a page component mapping. |
| RD-014 | Add Analyze button placeholder states | Accepted | P2 | UI Structure | Even before the real bridge is complete, disabled/loading/error states clarify the future integration path and reduce UI churn. | Add placeholder handlers and state model. |
| RD-015 | Keep chart title, legend, and axis labels i18n-ready | Accepted | P2 | UI Structure | The project already supports Korean/English. Chart text should stay inside the same localization discipline. | Ensure dynamic chart labels use message keys or locale-aware mapping. |
| RD-016 | Add statistics edge-case tests | Accepted | P1 | Test Coverage | Percentile and average functions are shared primitives; small edge-case tests reduce risk with little cost. | Test empty list, single value, repeated values, negative values, and percentile interpolation. |
| RD-017 | Add malformed parser tests | Accepted | P1 | Test Coverage | Parser behavior changes should be locked down with tests so diagnostics do not regress. | Add access log and collapsed parser malformed-input tests. |
| RD-018 | Add JSON exporter round-trip test | Accepted | P1 | Test Coverage | AnalysisResult JSON is the cross-runtime contract. A write/read test helps protect the bridge path. | Add exporter test that writes JSON and reads it back for structural comparison. |
| RD-019 | Plan early Electron + PyInstaller packaging spike | Accepted | P3 | Packaging | Packaging can expose path, binary, and environment issues late. A lightweight spike reduces release risk without blocking current foundation work. | Add packaging spike to roadmap after bridge PoC. |
| RD-020 | Remove or raise low setuptools version ceiling | Deferred | P3 | Packaging | This is worth cleaning up, but it is not currently blocking functionality or the bridge decision. | Review packaging metadata during packaging spike. |
| RD-021 | Consolidate `setup.py` and `pyproject.toml` | Deferred | P3 | Packaging | Single-source packaging metadata is preferable, but changing it now is lower value than engine/UI integration. | Revisit during packaging cleanup. |
| RD-022 | Separate profiler stack classification rules from Python code | Deferred | P3 | Runtime Expansion | Configuration-driven rules matter when non-JVM runtimes are prioritized. Current hardcoded rules are acceptable for early profiler support. | Move to configurable classification before Node.js/Python/Go/.NET stack expansion. |
| RD-023 | Prioritize Timeline Correlation as a differentiator | Deferred | P4 | Advanced Diagnostics | Timeline correlation is strategically important but depends on stable analyzers, result contracts, and UI rendering first. | Keep in roadmap as a later differentiating feature. |
| RD-024 | Require raw evidence for AI-assisted interpretation | Deferred | P5 | AI Interpretation | This is the correct trust model for AI interpretation, but AI-assisted analysis is not part of the immediate foundation cycle. | When AI interpretation starts, require source evidence references such as `raw_line` or `raw_block`. |
| RD-025 | Update React and Electron versions now | Needs Decision | P3 | Dependency Management | Dependency freshness has security value, but upgrades can introduce UI/build churn. This should be decided with the desired release window and support policy. | Decide whether to upgrade during foundation work or defer to packaging hardening. |
| RD-026 | Choose desktop-only bridge vs local HTTP server for future web portability | Accepted | P0 | Foundation Stabilization | Desktop-first integration is the current product direction. Electron IPC plus Python CLI keeps the process model simple and avoids local server lifecycle and port-management overhead. | Use Electron IPC + `child_process.execFile` for the Bridge PoC; revisit local HTTP only if web delivery becomes a near-term requirement. |

## Backlog Mapping

The executable TO-DO list is maintained in `work_status.md` under `Execution Backlog`.

| Decision range | Work status task IDs | Notes |
|---|---|---|
| RD-001, RD-002, RD-003, RD-026 | T-001 to T-003 | Engine-UI bridge decision, client boundary, and Bridge PoC. |
| RD-004, RD-005, RD-017 | T-004 to T-007, T-013 | Parser malformed-input handling, parser tests, and parser docs. |
| RD-006 | T-010 to T-012 | Python and TypeScript result contracts plus data model docs. |
| RD-016, RD-018 | T-008, T-009 | Statistics and JSON exporter regression tests. |
| RD-008, RD-009, RD-010 | T-014 to T-017 | Sampling, streaming strategy docs, streaming design, and later refactor. |
| RD-012, RD-013, RD-014, RD-015 | T-018 to T-021 | UI structure and chart readiness work. |
| RD-019, RD-020, RD-021, RD-025 | T-022 to T-025 | Packaging and dependency-management work. |
| RD-022 | T-026, T-027 | Runtime classification expansion. |
| RD-023 | T-028 | Timeline correlation retained as later advanced diagnostics. |
| RD-024 | T-029 | AI interpretation evidence guardrail. |

## Open Decisions

| ID | Question | Default recommendation |
|---|---|---|
| RD-025 | Should React/Electron dependency upgrades happen during foundation stabilization? | Defer until packaging hardening unless a known vulnerability forces an earlier update. |
