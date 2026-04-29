# ArchScope Work Status

Last updated: 2026-04-30

## Review Processing Status

- [x] Read `docs/review/done/2026-04-29_Phase1_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-29_claude-code_phase1-review.md`
- [x] Read `docs/review/done/2026-04-29_UI_Chart_Architecture_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-30_claude-code_ui-chart-foundation-review.md`
- [x] Read `docs/review/done/2026-04-30_Phase3_Packaging_Expansion_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-30_claude-code_phase3-packaging-runtime-review.md`
- [x] Consolidated review findings into this TO-DO
- [x] Created `review_decisions.md` with accepted/deferred/rejected/needs-decision classifications
- [x] Moved processed review documents to `docs/review/done/`

## Research Processing Status

- [x] Read `docs/research/2026-04-28_Competitor_Analysis_and_Positioning_Gemini.md`
- [x] Read `docs/research/2026-04-28_claude-code_product-enhancement.md`
- [x] Created `docs/research/research_decisions.md`
- [x] Reflected accepted research items into roadmap, architecture, and execution backlog

## Current Priority

The next work cycle should move into Phase 5 evidence guardrails and optional AI interpretation design after Phase 3 follow-ups and Phase 4 diagnostic design:

```text
AI evidence references -> local LLM interpretation design -> report automation boundaries
```

Engine-UI Bridge decision: Electron IPC + `child_process.execFile` invoking the Python CLI. Local HTTP/FastAPI is deferred unless web delivery becomes a near-term product goal.

AnalysisResult contract hardening scope: keep the common dataclass transport model and type-specific Python/TypeScript contracts. Strengthen shallow IPC runtime validation before adding many more result types; defer full Pydantic/Zod/schema generation until contract drift becomes a practical risk.

Parser error handling policy: file/configuration failures are fatal; malformed record-level input is skipped by default and reported under `metadata.diagnostics`. Strict fail-fast parsing is deferred until there is an explicit option.

## Execution Backlog

### Phase 1A - Foundation Stabilization

Goal: make the current skeleton run through one real diagnostic path with explicit contracts and reliable parser failure behavior.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-001 | P0 | [x] | Decide and document the Engine-UI bridge approach. Decision: Electron IPC + `child_process.execFile` invoking the Python CLI. | None | RD-001, RD-026 | Bridge design note in `docs/en/ARCHITECTURE.md` and `docs/ko/ARCHITECTURE.md` |
| T-002 | P0 | [x] | Define analyzer client interfaces before replacing mock data. Keep a mock client and add a real IPC client boundary. | T-001 | RD-003 | TypeScript analyzer client contract |
| T-003 | P0 | [x] | Build minimal Bridge PoC: select sample file, invoke Python CLI from Electron main process, return `AnalysisResult` JSON to renderer. | T-001, T-002, T-030, T-037 | RD-002, RS-005 | Working UI-to-engine diagnostic path |
| T-004 | P1 | [x] | Improve Access Log parser malformed-line handling with skipped-line count and diagnostics metadata. Policy is fixed in parser design docs. | T-031 | RD-004, RS-009 | Parser behavior change plus metadata |
| T-005 | P1 | [x] | Improve collapsed parser malformed-line handling for invalid trailing sample counts. Policy is fixed in parser design docs. | T-031 | RD-005, RS-009 | Parser behavior change plus metadata |
| T-006 | P1 | [x] | Add malformed access log parser tests. | T-004 | RD-017 | Parser regression tests |
| T-007 | P1 | [x] | Add malformed collapsed parser tests. | T-005 | RD-017 | Parser regression tests |
| T-008 | P1 | [x] | Add `statistics.py` edge-case tests for empty, single, repeated, negative, and percentile interpolation cases. | None | RD-016 | Utility regression tests |
| T-009 | P1 | [x] | Add JSON exporter write/read round-trip test. | None | RD-018 | Contract regression test |
| T-010 | P1 | [x] | Add type-specific Python `TypedDict` contracts for Access Log and Profiler result sections. Scope is fixed in data model docs. | T-004, T-005 preferred | RD-006 | Python contract types |
| T-011 | P1 | [x] | Align TypeScript result types with Python result contracts. | T-010 | RD-006 | UI contract types |
| T-012 | P1 | [x] | Document required keys for each result type in `docs/en/DATA_MODEL.md` and `docs/ko/DATA_MODEL.md`. | T-010, T-011 | RD-006 | Updated data model docs |
| T-013 | P1 | [x] | Add encoding, corrupt-input, and malformed-record behavior to parser design docs. | None | RD-004, RD-005 | Updated `docs/en/PARSER_DESIGN.md` and `docs/ko/PARSER_DESIGN.md` |
| T-030 | P0 | [x] | Declare Python runtime dependencies and console script metadata for the engine CLI. Include `typer`, `rich`, and `archscope-engine`. | None | RS-006 | Reliable Python CLI installation path |
| T-037 | P0 | [x] | Define the minimal Bridge PoC UX flow while implementing: file selection/drop, analyze action, loading state, success result rendering, parser diagnostics panel, and bridge/error messages. | T-001, T-002 | User follow-up | Minimal UI flow notes captured in implementation or UI design docs |
| T-031 | P1 | [x] | Fix `iter_text_lines` encoding fallback so a mid-file decode failure cannot emit duplicated lines across fallback retries. | None | RS-008 | Encoding-safe line iterator plus tests |

### Phase 1B - Large File Baseline

Goal: define and implement low-risk controls for large input files before deeper streaming refactors.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-014 | P2 | [x] | Add analyzer sampling options such as max lines and time range filter. | T-003 preferred | RD-008 | Analyzer input options |
| T-015 | P2 | [x] | Expand `docs/en/PARSER_DESIGN.md` and `docs/ko/PARSER_DESIGN.md` with memory-bounded streaming strategy. | T-014 | RD-010 | Streaming design documentation |
| T-016 | P2 | [x] | Design access log streaming aggregation with incremental counters, top URL aggregation, status distribution, and time buckets. | T-015 | RD-009 | Implementation design, not full refactor |
| T-017 | P2 | [x] | Refactor access log analyzer toward streaming aggregation. | T-016 | RD-009 | Memory-bounded analyzer path |
| T-032 | P2 | [x] | Add Access Log diagnostic rules beyond raw chart metrics, including status-code findings and slow URL interpretation. | T-004, T-010 | RS-017 | Report-grade access log findings |

### Phase 2 Readiness - Stop-line and Hygiene

Goal: close Phase 1 review findings that should be handled before broad Phase 2 feature expansion.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-023 | P0 | [x] | Upgrade Electron to 33+ to address EOL security concerns before Phase 2 UI expansion. | T-003 | RD-025, RD-032 | Desktop dependency security upgrade |
| T-042 | P0 | [x] | Implement Content Security Policy (CSP) to harden the renderer process. | T-023 preferred | RD-033 | Electron renderer security hardening |
| T-043 | P1 | [x] | Consolidate duplicated `ParserDiagnostics` into `archscope_engine.common.diagnostics`. | None | RD-034 | Shared parser diagnostics utility |
| T-044 | P1 | [x] | Extract duplicated `MetricCard` implementations into a shared component in `src/components`. | None | RD-035 | Shared metric card component |
| T-018 | P1 | [x] | Convert page rendering in `App.tsx` to a mapping table. | None | RD-013 | Cleaner page registration |
| T-046 | P1 | [x] | Separate analyzer logic tests from parser tests for better modularity. | T-043 preferred | RD-037 | Analyzer-focused regression tests |
| T-045 | P1 | [x] | Set up GitHub Actions CI for automated Python tests and desktop build/type checks. | T-046 preferred | RD-036 | CI workflow |
| T-047 | P1 | [x] | Add CLI end-to-end integration tests for analyzer commands and JSON output. | T-030, T-045 preferred | RD-039 | CLI contract regression tests |
| T-048 | P2 | [x] | Strengthen runtime `AnalysisResult` validation at the Electron IPC boundary. | T-010, T-011 | RD-038 | Safer engine-output validation |
| T-038 | P2 | [x] | Implement Generic IPC Handler (`analyzer:execute`) to unify analyzer execution paths. | T-048 preferred | RD-027 | Generic IPC bridge in Electron main and renderer |
| T-041 | P2 | [x] | Capture and pipe engine stderr/progress detail to UI feedback. | T-038 preferred | RD-031 | Detailed error/progress feedback in UI |
| T-049 | P2 | [x] | Replace remaining unbounded exact percentile arrays with bounded or approximate percentile aggregation. | T-014, T-017 | RD-028 | Reduced large-file memory pressure |

### Phase 2 - Report-Ready UI and Charts

Goal: make the UI easier to extend and prepare chart rendering for dynamic result data.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-050 | P2 | [x] | Document bounded percentile sampling behavior and limitations in parser design docs. | T-049 | RD-044 | Updated `docs/en/PARSER_DESIGN.md` and `docs/ko/PARSER_DESIGN.md` |
| T-019 | P2 | [x] | Add placeholder Analyze handlers with disabled, loading, error, and engine-message feedback states. | T-037, T-041 | RD-014, RD-045 | UI state skeleton |
| T-020 | P2 | [x] | Keep chart titles, legends, and axis labels i18n-ready. | T-003 preferred | RD-015 | Locale-aware chart labels |
| T-021 | P2 | [x] | Prepare chart templates and chart-option factory extraction for dynamic loading during Chart Studio work. | T-003, T-020 | RD-012, RD-030 | Template extraction plan or initial chart factory |
| T-033 | P2 | [x] | Upgrade to ECharts 6 and evaluate dark mode, broken axis, custom chart, and SVG export impact. | T-003, T-020 | RS-012 | Chart upgrade plan and implementation spike |
| T-051 | P2 | [x] | Add CI lint and coverage reporting after Python/TypeScript tooling choices are settled. | T-045 | RD-047 | CI quality reporting |

### Phase 2 Follow-up - UI/Chart Foundation Hardening

Goal: close review findings that should be handled before deeper Chart Studio and analyzer UI expansion.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-054 | P1 | [x] | Route analyzer-page charts through the shared chart factory and remove local ECharts option builders. | T-021 | RD-051 | Analyzer chart factory integration |
| T-055 | P1 | [x] | Move ECharts theme registration from Dashboard-only setup to app initialization. | T-033 | RD-058 | App-level chart theme registration |
| T-056 | P1 | [x] | Replace `ChartPanel` window resize listener with scoped `ResizeObserver`. | T-033 | RD-052 | Container-aware chart resizing |
| T-057 | P1 | [x] | Add accessibility states for analyzer/chart feedback, including error alerts and chart busy state. | T-019, T-033 | RD-055 | Improved assistive-tech feedback |
| T-058 | P2 | [x] | Localize file dialog filter labels and remaining analyzer UI filter text. | T-020 | RD-056 | Locale-aware file dialog labels |
| T-059 | P2 | [x] | Extract duplicated UI value formatters into shared utilities. | T-044 | RD-057 | `src/utils/formatters.ts` |
| T-060 | P2 | [x] | Add chart option builder/factory regression tests for template output shapes. | T-021, T-054 | RD-062 | Frontend chart factory tests |
| T-061 | P2 | [x] | Add placeholder analyzer async unmount guard before real IPC replacement. | T-019 | RD-054 | Safer placeholder state transitions |

### Phase 3 - Packaging and Runtime Expansion

Goal: reduce release risk and prepare analyzer expansion after the foundation is stable.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-022 | P3 | [x] | Plan early packaging spike for Electron + PyInstaller sidecar. | T-003 | RD-019 | Packaging spike plan |
| T-024 | P3 | [x] | Review `setuptools` ceiling and packaging metadata cleanup. | T-022 | RD-020 | Packaging metadata decision |
| T-025 | P3 | [x] | Decide whether to consolidate `setup.py` and `pyproject.toml`. | T-024 | RD-021 | Packaging metadata cleanup task |
| T-026 | P3 | [x] | Separate profiler stack classification rules from hardcoded Python logic. | T-003, T-010 | RD-022 | Configurable classification design |
| T-027 | P3 | [x] | Add configuration-driven classification for JVM, Node.js, Python, Go, and .NET stacks. | T-026 | RD-022 | Runtime classification configuration |
| T-052 | P3 | [x] | Evaluate nonce-based CSP style policy to remove `style-src 'unsafe-inline'` after UI/runtime stabilization. | T-042 | RD-046 | CSP hardening decision |
| T-053 | P3 | [x] | Evaluate seed-configurable bounded percentile sampling for reproducibility-sensitive analysis. | T-049 | RD-048 | Percentile sampler decision |
| T-062 | P3 | [x] | Generalize chart factory data typing beyond `DashboardSampleResult` for real analyzer results. | T-054 | RD-059 | Analyzer-aware chart factory contracts |
| T-063 | P3 | [x] | Design chart option serialization, deep-merge, and persistence for Chart Studio. | T-062 | RD-061 | Chart Studio persistence design |
| T-064 | P3 | [x] | Evaluate ECharts `echarts/core` tree-shaking after chart catalog expansion. | T-033, T-063 preferred | RD-053 | Bundle optimization decision |

### Phase 3 Follow-up - Packaging/Runtime Review Hardening

Goal: close Phase 3 review findings that reduce packaging, runtime-classification, and sampler edge-case risk before Phase 4 diagnostics expand.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-065 | P1 | [x] | Add profiler classification regression tests for Application fallback, first-match rule ordering, and case-insensitive matching. | T-026, T-027 | RD-069, RD-070, RD-071 | Runtime classification edge-case tests |
| T-066 | P1 | [x] | Fix Python package `long_description` metadata by adding an engine README or removing the missing file reference. | T-024, T-025 | RD-074 | Packaging metadata cleanup |
| T-067 | P2 | [x] | Validate or document `BoundedPercentile.seed` constraints, including the degenerate `seed=0` case. | T-053 | RD-072 | Sampler validation/docs plus tests |
| T-068 | P2 | [x] | Tighten broad profiler classification tokens and document rule-author specificity/ordering constraints. | T-065 | RD-067, RD-068 | Safer classification rules and guidance |
| T-070 | P2 | [x] | Add Electron analyzer sidecar lifecycle cleanup so active Python child processes are terminated on app/process exit. | T-038, T-041 | RD-065 | Orphan-process prevention |
| T-069 | P3 | [x] | Add external runtime classification config loader and packaged resource plan for PyInstaller sidecar builds. | T-068, T-070 preferred | RD-064 | Config-driven runtime classification path |
| T-071 | P3 | [x] | Run an ECharts nonce CSP compatibility spike before replacing `style-src 'unsafe-inline'`. | T-052, T-063 preferred | RD-066 | CSP nonce compatibility decision |
| T-072 | P3 | [x] | Add an explicit Linux deferred/out-of-scope note to the packaging spike plan. | T-022 | RD-075 | Packaging scope clarification |

### Phase 4 - Advanced Diagnostics

Goal: add higher-value diagnostic correlation after analyzer contracts and UI integration are stable.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-028 | P4 | [x] | Keep Timeline Correlation as a planned differentiator for JVM diagnostics. | T-003, T-010, Phase 3 JVM parsers | RD-023 | Roadmap item retained |
| T-034 | P4 | [x] | Add JFR recording parser design spike, including parser library feasibility and AnalysisResult shape. | GC/thread parser direction, T-010 | RS-013 | JFR parser design decision |
| T-035 | P4 | [x] | Add OpenTelemetry log input design, including trace/span context mapping for future correlation. | T-010, T-028 preferred | RS-014 | OTel input design decision |

### Phase 5 - AI-Assisted Interpretation

Goal: only introduce AI interpretation with strict evidence requirements.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-029 | P5 | [ ] | Require raw evidence references such as `raw_line` or `raw_block` for AI-assisted interpretation. | Stable report data model, Phase 5 start | RD-024 | AI interpretation guardrail |
| T-036 | P5 | [ ] | Design optional local LLM/Ollama interpretation layer with explicit evidence references and AI-generated labeling. | T-029, T-028 | RS-016 | Local AI interpretation design |

## Dependency Order

1. `T-001 -> T-002 -> T-030 -> T-037 -> T-003`: bridge decision, client boundary, CLI install metadata, minimal UX flow, then end-to-end PoC.
2. `T-031 -> T-004 -> T-006` and `T-031 -> T-005 -> T-007`: encoding-safe reads, parser behavior, then tests.
3. `T-010 -> T-011 -> T-012`: Python contracts, TypeScript contracts, then data model docs.
4. `T-014 -> T-015 -> T-016 -> T-017`: sampling and design before streaming refactor.
5. `T-023 -> T-042`: resolve Electron support and CSP before broad Phase 2 UI expansion.
6. `T-043 -> T-046 -> T-045 -> T-047`: consolidate diagnostics, separate analyzer tests, then automate them in CI and add CLI integration coverage.
7. `T-048 -> T-038 -> T-041`: strengthen IPC result validation, then generalize analyzer execution and improve progress/error feedback.
8. `T-049 -> T-050`: document bounded percentile sampling tradeoffs after the memory-bound implementation lands.
9. `T-019 -> T-020 -> T-021 -> T-033`: start Phase 2 UI state work, keep chart text localized, then extract chart factories and evaluate ECharts 6.
10. `T-054 -> T-055 -> T-056 -> T-057 -> T-058 -> T-059 -> T-060 -> T-061`: close UI/chart foundation review follow-ups before deeper Chart Studio work.
11. `T-054 -> T-062 -> T-063 -> T-064`: generalize chart factory contracts, then design Chart Studio persistence and revisit bundle optimization.
12. `T-003 -> T-022`: only plan packaging spike after the actual bridge path exists.
13. `T-065 -> T-068 -> T-069`: lock down classification edge cases, refine rule specificity, then add external config loading.
14. `T-066 -> T-067 -> T-070`: close independent packaging metadata, sampler, and sidecar lifecycle hardening before broader Phase 4 work.
15. `T-071`: revisit nonce CSP only after packaged renderer and chart export behavior are stable.

## Active Decision Queue

| Decision | Required before | Default |
|---|---|---|
| React 19 upgrade timing | After T-023 | Defer unless Electron/tooling compatibility requires it. |

## Workflow For New Review Documents

When new review documents are placed in `docs/review/`:

1. Read every unprocessed review document in `docs/review/`.
2. Extract actionable findings, risks, and recommendations.
3. Classify each finding in `review_decisions.md` as Accepted, Deferred, Rejected, or Needs Decision.
4. Update this `work_status.md` TO-DO list from accepted and decided findings.
5. Move processed review documents into `docs/review/done/`.
6. Keep AI-local instruction files out of git by using `.gitignore`.

## Notes

- `docs/review/done/` is an archive of processed reviews.
- `review_decisions.md` is the source of truth for review acceptance decisions.
- `docs/research/research_decisions.md` is the source of truth for research acceptance decisions.
- `CLAUDE.MD`, `GEMINI.MD`, and `AGENTS.MD` are local AI working instruction files and must not be committed.
