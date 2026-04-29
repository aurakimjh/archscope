# ArchScope Review Decisions

Last updated: 2026-04-29

## Scope

This document records acceptance decisions for review findings extracted from:

- `docs/review/done/2026-04-28_Architecture_and_Implementation_Review.md`
- `docs/review/done/2026-04-28_claude_opus_architecture_review.md`
- `docs/review/done/2026-04-29_Phase1_Review_by_Gemini.md`
- `docs/review/done/2026-04-29_claude-code_phase1-review.md`
- `docs/review/done/2026-04-29_Phase2_Hygiene_Review_by_Gemini.md`
- `docs/review/done/2026-04-29_claude-code_phase2-readiness-followup.md`

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
| RD-004 | Handle malformed access log lines with diagnostics | Accepted | P1 | Foundation Stabilization | Policy fixed: file/configuration failures are fatal, but malformed access-log records are skipped by default and reported under `metadata.diagnostics`. | Implement skipped-line count, reason codes, diagnostics samples, and malformed-line parser tests. |
| RD-005 | Handle malformed collapsed stack lines | Accepted | P1 | Foundation Stabilization | Policy fixed: malformed collapsed profiler records are non-fatal by default; invalid or missing sample counts are skipped and reported in diagnostics. | Implement invalid-line handling, reason codes, metadata reporting, and collapsed parser tests. |
| RD-006 | Strengthen AnalysisResult contracts with type-specific keys | Accepted | P1 | Foundation Stabilization | Scope fixed: keep the common dataclass as the outer transport model, add type-specific contracts for `access_log` and `profiler_collapsed`, and defer full Pydantic migration. | Implement Python `TypedDict` for Access Log and Profiler sections, align TypeScript interfaces, and keep required keys documented in `DATA_MODEL.md`. |
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
| RD-025 | Update React and Electron versions now | Accepted | P3 | Dependency Management | Research follow-up split the decision: Electron upgrade is accepted after Bridge PoC because Electron 31 is outside the supported major window; React 19 remains deferred unless compatibility requires it. | Track Electron upgrade under `T-023`; revisit React after Electron/tooling stabilization. |
| RD-026 | Choose desktop-only bridge vs local HTTP server for future web portability | Accepted | P0 | Foundation Stabilization | Desktop-first integration is the current product direction. Electron IPC plus Python CLI keeps the process model simple and avoids local server lifecycle and port-management overhead. | Use Electron IPC + `child_process.execFile` for the Bridge PoC; revisit local HTTP only if web delivery becomes a near-term requirement. |
| RD-027 | Implement Generic IPC Handler (`analyzer:execute`) | Accepted | P2 | Phase 2 Expansion | The current per-analyzer IPC channels are fine for Phase 1, but will create main-process churn as analyzer count grows. This should be done before adding several new analyzers, not retroactively listed under Phase 1A. | Refactor `main.ts`, preload, and analyzer client to use a generic execution path while preserving typed request/response contracts. |
| RD-028 | Introduce more memory-bounded aggregation for large access logs | Accepted | P2 | Large File Strategy | Phase 1B removed full `AccessLogRecord` materialization, but exact percentile arrays still grow with input size. The remaining memory risk should be tracked separately from the completed streaming refactor. | Replace exact unbounded percentile arrays with a bounded or approximate percentile strategy after current streaming baseline. |
| RD-029 | Centralize UI Analysis Session state | Deferred | P3 | UI Expansion | Current per-page state is still manageable. Introducing Zustand/Context now would add framework weight before cross-page analysis sharing exists. | Reassess when analysis sessions need to be shared across pages, persisted, or replayed. |
| RD-030 | Decouple Chart Builders into Factory Pattern | Accepted | P2 | UI Structure | Chart logic will become harder to maintain as result types grow. This overlaps with the existing chart-template preparation task and should be merged there instead of creating duplicate backlog. | Fold chart factory extraction into the chart-template/chart readiness work. |
| RD-031 | Add real-time stderr/progress feedback to UI | Accepted | P2 | Bridge UX | Phase 1 returns final JSON and coarse errors. Longer analyses need better progress/error detail, but this follows security and hygiene stop-line items. | Capture engine stderr and progress detail through the bridge and expose it in UI feedback. |
| RD-032 | Upgrade Electron to 33+ (Stop-the-line) | Accepted | P0 | Foundation Stabilization | Electron 31 is EOL. Security and Chromium patches are critical for a production-grade tool. | Upgrade Electron and verify compatibility. |
| RD-033 | Implement Content Security Policy (CSP) | Accepted | P1 | Foundation Stabilization | Preventing XSS and other injection attacks by restricting script and style loading. | Configure CSP in Electron main process. |
| RD-034 | Consolidate `ParserDiagnostics` into `common/diagnostics.py` | Accepted | P1 | Phase 2 Expansion | DRY principle: the same diagnostics logic is duplicated across multiple parsers. | Move shared diagnostics logic to a common module. |
| RD-035 | Extract `MetricCard` into a shared component | Accepted | P1 | Phase 2 Expansion | DRY principle: the component is duplicated across multiple pages. | Create `src/components/MetricCard.tsx`. |
| RD-036 | Add GitHub Actions CI for automated testing | Accepted | P1 | Infrastructure | Ensuring tests and linting run on every PR to prevent regressions. | Set up `.github/workflows/ci.yml`. |
| RD-037 | Separate Analyzer tests from Parser tests | Accepted | P1 | Test Coverage | Ensuring modular test coverage for aggregation logic independent of file parsing. | Create `tests/test_access_log_analyzer.py` etc. |
| RD-038 | Strengthen runtime `AnalysisResult` validation at the IPC boundary | Accepted | P2 | Contract Hardening | `isAnalysisResult` currently checks only a shallow subset of required fields. Phase 2 can start with a manual guard before evaluating Zod or generated schemas. | Validate `tables`, `metadata`, `created_at`, and type-specific required sections before returning engine output to the renderer. |
| RD-039 | Add CLI end-to-end integration tests | Accepted | P1 | Test Coverage | The Electron bridge depends on the CLI output path, but current tests mainly cover Python functions and packaging metadata. | Add tests for `archscope-engine access-log analyze ... --out ...` and profiler output round trip using installed/test entry points. |
| RD-040 | Add frontend regression tests for contract-driven UI paths | Deferred | P2 | UI Test Coverage | UI test coverage is valuable, but the project has no frontend test harness yet. It should follow CI and security hardening rather than block immediate Phase 2 readiness. | Reassess after CI is in place; start with component/contract rendering tests for diagnostics and analyzer pages. |
| RD-041 | Add structured local logging for engine and Electron main process | Deferred | P2 | Observability | Parser diagnostics are strong, but runtime logs would help debugging. This is useful after the bridge/progress path stabilizes. | Consider Python JSON logging and Electron main-process local logs during bridge UX work. |
| RD-042 | Restrict file path scope for analyzer inputs | Deferred | P3 | Security Hardening | `execFile` avoids shell injection and current desktop file selection is local, but symlink/path-scope controls may matter for packaged or enterprise deployments. | Reassess during packaging and enterprise policy work. |
| RD-043 | Evaluate JSON Schema or generated contracts as single source of truth | Deferred | P2 | Contract Hardening | Manual Python TypedDict and TypeScript contracts are acceptable with two analyzers, but drift risk grows with analyzer count. | Evaluate schema/codegen after type-specific contracts are exercised by more result types. |
| RD-044 | Document bounded percentile sampling behavior and limits | Accepted | P2 | Large File Strategy | Phase 2 Readiness introduced deterministic bounded sampling for percentiles. The memory benefit and approximate/input-order-sensitive behavior should be visible in parser design docs. | Add English and Korean `PARSER_DESIGN.md` notes for bounded percentile sampling. |
| RD-045 | Surface captured engine messages in analyzer UI feedback | Accepted | P2 | UI Structure | The bridge captures `engine_messages`, but Phase 2 analyzer UX should decide where to show them alongside disabled/loading/error states. | Fold engine-message display into T-019 analyzer feedback work. |
| RD-046 | Replace CSP `style-src 'unsafe-inline'` with nonce-based policy | Deferred | P3 | Security Hardening | Production CSP already blocks unsafe script execution. Removing inline styles is useful but should follow UI/runtime stabilization because React/ECharts styling needs compatibility testing. | Reassess nonce-based style CSP during packaging/security hardening. |
| RD-047 | Add CI lint and coverage reporting | Deferred | P2 | Infrastructure | CI now runs Python tests and desktop builds. Lint and coverage reporting are valuable but require choosing project lint/coverage tools and should not block Phase 2 entry. | Add a follow-up CI quality task after lint/coverage tools are selected. |
| RD-048 | Add seed-configurable percentile sampling | Deferred | P3 | Large File Strategy | Current deterministic sampler is reproducible for the same input. Seed injection may be useful for statistical validation but is not required for current analyzer output. | Reassess if percentile sampling needs multiple deterministic sample streams or user-visible reproducibility controls. |
| RD-049 | Verify CLI E2E dependencies run in CI | Accepted | P1 | Test Coverage | `typer` and `rich` are runtime dependencies in `setup.cfg`, so `pip install -e ".[dev]"` installs them together with pytest. Existing packaging tests also assert both runtime dependencies. | No new task; keep CLI E2E tests guarded for local environments without installed runtime dependencies. |
| RD-050 | Replace diagnostics method reference with lambda | Rejected | P2 | Code Clarity | `diagnostics.to_dict` is a bound zero-argument method and matches the callable type. The review explicitly recommends keeping the current behavior unless readability becomes a problem. | No action. |

## Backlog Mapping

The executable TO-DO list is maintained in `work_status.md` under `Execution Backlog`.

| Decision range | Work status task IDs | Notes |
|---|---|---|
| RD-001, RD-002, RD-003, RD-026 | T-001 to T-003 | Engine-UI bridge decision, client boundary, and Bridge PoC. |
| RD-004, RD-005, RD-017 | T-004 to T-007, T-013 | Parser malformed-input handling, parser tests, and parser docs. |
| RD-006 | T-010 to T-012 | Python and TypeScript result contracts plus data model docs. |
| RD-016, RD-018 | T-008, T-009 | Statistics and JSON exporter regression tests. |
| RD-008, RD-009, RD-010 | T-014 to T-017 | Sampling, streaming strategy docs, streaming design, and streaming baseline refactor. |
| RD-012, RD-013, RD-014, RD-015, RD-030 | T-018 to T-021 | UI structure and chart readiness work. |
| RD-019, RD-020, RD-021 | T-022, T-024, T-025 | Packaging and packaging-metadata cleanup work. |
| RD-025, RD-032 | T-023 | Electron support/security upgrade. |
| RD-022 | T-026, T-027 | Runtime classification expansion. |
| RD-023 | T-028 | Timeline correlation retained as later advanced diagnostics. |
| RD-024 | T-029 | AI interpretation evidence guardrail. |
| RD-027, RD-031, RD-038, RD-039 | T-038, T-041, T-048, T-047 | Bridge scalability, progress feedback, runtime validation, and CLI integration tests. |
| RD-028 | T-049 | Remaining large-file percentile memory risk after Phase 1B streaming baseline. |
| RD-033 to RD-037 | T-042 to T-046 | Phase 2 readiness stop-line and hygiene work from Claude review. |
| RD-044 | T-050 | Bounded percentile sampling documentation. |
| RD-045 | T-019 | Analyzer UI state work should include captured engine-message feedback. |
| RD-046 | T-052 | Future nonce-based CSP hardening. |
| RD-047 | T-051 | Future CI lint/coverage reporting. |
| RD-048 | T-053 | Future percentile sampler seed/configuration evaluation. |

## Open Decisions

No review-sourced decisions are currently open. Research-sourced decisions are tracked in `docs/research/research_decisions.md`.
