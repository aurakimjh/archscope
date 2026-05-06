# ArchScope Work Status

Last updated: 2026-05-06 (Phase 1 web pivot — T-206 design note + T-207 Electron removal + T-208 unified wheel landed. `pip install archscope && archscope serve` is now the install path: distribution renamed `archscope-engine`→`archscope`, new `archscope` console script auto-opens the browser and inherits every analyzer subcommand from the engine CLI, React bundle ships as wheel package data resolved through `importlib.resources`, build helper at `scripts/build-archscope-wheel.sh`. `apps/desktop/` is gone; FastAPI gained `POST /api/files/select` + `WebSocket /ws/progress` + task-id keyed real `analyze/cancel`, all backed by a new `ProgressRegistry` symmetric with the Wails T-240f task registry. The Go/Native Profiler trail-end batch (T-240f / DebugLog UI / T-245 / T-246 / T-242 / T-247) shipped earlier in the day; that section has no open P1/P2 items left in the alpha scope. Final native binary 8.4 MB raw / 10 MB .app on darwin/arm64.)

## Review Processing Status

- [x] Read `docs/review/done/2026-04-29_Phase1_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-29_claude-code_phase1-review.md`
- [x] Read `docs/review/done/2026-04-29_UI_Chart_Architecture_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-30_claude-code_ui-chart-foundation-review.md`
- [x] Read `docs/review/done/2026-04-30_Phase3_Packaging_Expansion_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-30_claude-code_phase3-packaging-runtime-review.md`
- [x] Read `docs/review/done/2026-04-30_Phase4_Advanced_Diagnostics_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-30_claude-code_phase4-advanced-diagnostics-review.md`
- [x] Read `docs/review/done/2026-04-30_claude-code_phase5-ai-interpretation-review.md`
- [x] Read `docs/review/done/2026-04-30_Phase5_AI_Interpretation_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-04-30_Phase5_AI_Interpretation_Independent_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-05-01_Phase5_AI_Interpretation_Review_by_Gemini.md`
- [x] Read `docs/review/done/2026-05-01_claude-code_archscope-final-review.md`
- [x] Consolidated review findings into this TO-DO
- [x] Created `review_decisions.md` with accepted/deferred/rejected/needs-decision classifications
- [x] Moved processed review documents to `docs/review/done/`

## Research Processing Status

- [x] Read `docs/research/2026-04-28_Competitor_Analysis_and_Positioning_Gemini.md`
- [x] Read `docs/research/2026-04-28_claude-code_product-enhancement.md`
- [x] Created `docs/research/research_decisions.md`
- [x] Reflected accepted research items into roadmap, architecture, and execution backlog

## Current Priority

Active cycle (2026-05-05, revised): pivot from Electron desktop to a browser-served web app first, redesign the UI on the new web shell, expand the profiler to FlameGraph SVG/HTML inputs, build a language-agnostic multi-dump thread-analysis framework with Java/Go/Python/Node.js/.NET parsers, add interactive GC timeline plus thread-dump→flamegraph conversion, and harden JVM thread-dump behavior using selected TDA-main algorithms. See "Web Platform Migration, UI Redesign, Multi-Language Diagnostics Expansion, and Diagnostics Polish" plus "TDA-main JVM Thread Dump Hardening Follow-up" below (T-180 onward; T-224–T-233 added in this revision). Prior `T-001`–`T-179` backlog is complete. **Execution order is enforced to avoid double-work**: Phase 1 (T-206–T-209) lands before any UI rebuild; the original Electron-era cleanups T-180–T-183 are intentionally subsumed by the Phase 2 redesign (T-210–T-213) and must not be implemented separately.

The latest feature cycle started the next roadmap layer across report automation, Chart Studio, and multi-runtime diagnostics:

```text
AnalysisResult/debug JSON -> portable HTML/PPTX/diff report -> Export Center IPC/UI
Chart template registry -> editable Chart Studio preview
Node/Python/Go/.NET-IIS/OTel sample artifacts -> parser/analyzer/CLI MVPs
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

### Phase 4 Follow-up - Advanced Diagnostics Review Hardening

Goal: close Phase 4 review findings that must be resolved before AI-assisted interpretation or implementation-heavy advanced diagnostics work.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-073 | P1 | [x] | Strengthen advanced-diagnostics integrated vision, target scenario, market differentiation, and implementation triggers. | T-028, T-034, T-035 | RD-076 | Updated `ADVANCED_DIAGNOSTICS.md` vision section |
| T-074 | P1 | [x] | Define common timestamp normalization, timeline input contract, join-key hierarchy, clock-drift tolerance, confidence, and thread evidence fields. | T-073 preferred | RD-077, RD-078, RD-079, RD-088 | Correlation contract hardening |
| T-075 | P1 | [x] | Build a minimal JFR parser PoC from `jfr print --json` output to a draft `jfr_recording` `AnalysisResult`. | T-074 preferred | RD-080 | JFR command bridge validation |
| T-076 | P1 | [x] | Add JFR parser alternatives, licensing, JDK command prerequisite, and recording/parser JDK compatibility matrix. | T-034 | RD-081 | JFR spike decision matrix |
| T-077 | P2 | [x] | Expand JFR event model for `jdk.CPUTimeSample`, sampled/exact event semantics, and stack-frame representation. | T-076 preferred | RD-082 | JFR event model update |
| T-078 | P2 | [x] | Define large-file streaming/filtering strategy for JFR and OTel inputs, including event filters, stack-depth, time range, and top-N limits. | T-075 preferred | RD-083 | Bounded advanced-input processing plan |
| T-079 | P2 | [x] | Add OTel privacy/evidence retention policy, spec baseline, and OTel Profiles reevaluation trigger. | T-035 | RD-084, RD-085 | OTel evidence policy update |
| T-080 | P2 | [x] | Strengthen UI/runtime schema-version compatibility warnings for new `AnalysisResult` types. | T-048, T-074 preferred | RD-087 | Schema-version warning path |
| T-081 | P3 | [x] | Design a multi-lane ECharts timeline visualization for `timeline_correlation` results. | T-074 preferred | RD-086 | Correlation chart design |

### Phase 5 - AI-Assisted Interpretation

Goal: only introduce AI interpretation with strict evidence requirements.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-029 | P5 | [x] | Require raw evidence references such as `raw_line` or `raw_block` for AI-assisted interpretation. | Stable report data model, Phase 5 start | RD-024 | AI interpretation guardrail |
| T-036 | P5 | [x] | Design optional local LLM/Ollama interpretation layer with explicit evidence references and AI-generated labeling. | T-029, T-028 | RS-016 | Local AI interpretation design |

### Phase 5 Follow-up - AI Interpretation Hardening

Goal: convert Phase 5 policy guardrails into enforceable contracts, validators, security controls, and UI behavior before any production LLM path is added.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-082 | P1 | [x] | Define canonical `evidence_ref` grammar and registry across Access Log, Profiler, JFR, OTel, and timeline correlation outputs. | T-029, T-074 preferred | RD-090 | Reference grammar and analyzer namespace rules |
| T-083 | P1 | [x] | Define AI Finding and `InterpretationResult` contracts in Python and TypeScript, including the transport model and IPC validation path. | T-082, T-048 | RD-091 | Typed contracts plus IPC/schema compatibility updates |
| T-084 | P1 | [x] | Implement `AiFindingValidator` before LLM calls, with non-empty evidence refs, registry/range checks, quote/content matching, invalid-output rejection, and tests. | T-082, T-083 | RD-092 | Enforced AI evidence guardrail |
| T-085 | P1 | [x] | Add prompt-injection defense to AI interpretation: structural separation of system instructions and evidence data, data-as-data rules, suspicious instruction handling, and tests. | T-084 preferred | RD-093 | Prompt-injection mitigation design and implementation |
| T-086 | P1 | [x] | Add AI provenance UI guardrails: distinct AI labeling, deterministic-vs-AI separation, evidence expand/collapse, confidence display, and a disable switch that prevents LLM calls. | T-083, T-084 | RD-094 | User-visible AI provenance and disable path |
| T-087 | P2 | [x] | Design versioned `PromptBuilder` and `EvidenceSelector` pipeline with bounded excerpts, token budgets, template versioning, and English/Korean response strategy. | T-082, T-085 preferred | RD-095 | Prompt/evidence selection contract |
| T-088 | P2 | [x] | Define local LLM runtime policy: Ollama availability checks, localhost-only URL validation, timeouts, concurrency limits, worker/process isolation, and graceful fallback. | T-087 | RD-096 | Optional local LLM runtime controls |
| T-089 | P2 | [x] | Document local model, hardware, and packaging decisions: user-installed Ollama/models, no bundled model payloads, recommended model shortlist, minimum hardware, and model metadata/hash capture. | T-088 | RD-097 | Local AI runtime support guide |
| T-090 | P2 | [x] | Build AI interpretation evaluation pipeline with golden diagnostic scenarios, evidence-integrity checks, hallucination/relevance metrics, and model/prompt regression gates. | T-084, T-087 | RD-098 | AI quality regression framework |
| T-091 | P2 | [x] | Define AI output confidence, partial failure, retry, and invalid-schema handling policy for reports and UI display. | T-083, T-084 | RD-099 | Predictable AI output handling rules |
| T-092 | P2 | [x] | Apply privacy and logging policy to LLM inputs and outputs, including OTel redaction before prompt construction and disabled prompt/response logging by default. | T-079, T-087, T-088 | RD-100 | LLM privacy and retention safeguards |
| T-163 | P1 | [x] | Implement a concrete `LocalLlmClient`/`OllamaClient` execution boundary with localhost policy validation, timeout-bound `/api/generate` calls, JSON envelope normalization, and `AiFindingValidator` enforcement. | T-083, T-084, T-087, T-088 | RD-101 | Executable local LLM client path |
| T-164 | P1 | [x] | Add a non-blocking `execute_async()` wrapper for local LLM execution so UI/worker callers can avoid blocking their main loop. | T-163 | RD-102 | Async local AI execution contract |
| T-165 | P2 | [x] | Externalize prompt templates by model/version once more than one local model profile is actively supported. | T-087, T-163 | RD-103 | Versioned prompt-template configuration |
| T-166 | P3 | [x] | Add language-optimized system prompt variants for English/Korean if model evaluation shows response-language drift. | T-087, T-090, T-165 preferred | RD-104 | Multilingual prompt-template strategy |

### Profiler Feature Expansion - Flamegraph Drill-down and Execution Breakdown

Goal: add profiler flamegraph drill-down, Jennifer APM CSV import, and execution breakdown analysis as modular additions on top of the existing parser/analyzer/exporter/UI boundaries.

Scope exclusions for this feature cycle: PowerPoint export, full HTML flamegraph export, AI interpretation, and large-file optimization.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-093 | P0 | [x] | Define a common `FlameNode` data model and profiler flamegraph result contract with `id`, `parentId`, `name`, `samples`, `ratio`, `category`, `color`, `children`, and `path`. | T-010, T-011 | User feature request | Shared Python/TypeScript flame tree contract |
| T-094 | P0 | [x] | Convert existing collapsed stack profiler output into the common flame tree model without breaking existing profiler summaries. | T-093 | User feature request | Collapsed stack to `FlameNode` conversion |
| T-095 | P0 | [x] | Implement Jennifer APM flamegraph CSV parser with expected columns, practical aliases, key/parent tree reconstruction, multiple-root virtual root handling, and diagnostics for malformed rows. | T-093 | User feature request | Jennifer CSV parser and tree builder |
| T-096 | P1 | [x] | Add Jennifer sample input at `examples/profiler/sample-jennifer-flame.csv`. | T-095 | User feature request | Representative Jennifer CSV fixture |
| T-097 | P1 | [x] | Add CLI command `python -m archscope_engine.cli profiler analyze-jennifer-csv` returning an `AnalysisResult` compatible with profiler drill-down and breakdown. | T-095, T-096 | User feature request | Jennifer CSV CLI analyzer path |
| T-098 | P0 | [x] | Implement multi-stage flamegraph drill-down engine with immutable stages, per-stage data, metrics, flamegraph, top stacks, and top child frames. | T-093, T-094, T-095 preferred | User feature request | Reusable drill-down stage engine |
| T-099 | P0 | [x] | Support drill-down filter types: include text, exclude text, regex include, and regex exclude with validation and user-facing error metadata. | T-098 | User feature request | Filter predicate implementation |
| T-100 | P0 | [x] | Support drill-down match modes: anywhere, ordered, and subtree. | T-098, T-099 | User feature request | Match-mode implementation |
| T-101 | P1 | [x] | Support drill-down view modes: preserve full path and re-root at matched frame. | T-098, T-100 | User feature request | View-mode tree projection |
| T-102 | P1 | [x] | Calculate drill-down stage metrics: matched samples, estimated seconds, total ratio, parent-stage ratio, and elapsed ratio. | T-098 | User feature request | Stage metric calculator |
| T-103 | P1 | [x] | Add CLI command `python -m archscope_engine.cli profiler drilldown` for applying drill-down filters to collapsed-stack or Jennifer CSV profiler inputs. | T-097, T-098, T-102 | User feature request | Drill-down CLI command |
| T-104 | P0 | [x] | Implement priority-based execution breakdown classifier with required detailed categories and default stack/frame matching rules. | T-093, T-094 | User feature request | Category classifier and rule table |
| T-105 | P1 | [x] | Add simplified Executive View label mapping: SQL / DB, External Call, Network / I/O Wait, Application Method, Lock / Pool Wait, JVM / GC, and Others. | T-104 | User feature request | Executive category rollup |
| T-106 | P1 | [x] | Support `primary_category` and `wait_reason` when stacks match both external-call and network/wait categories. | T-104 | User feature request | Multi-signal classification output |
| T-107 | P1 | [x] | Calculate per-category breakdown metrics: samples, estimated seconds, total ratio, parent-stage ratio, elapsed ratio, top methods, and top stacks. | T-102, T-104, T-106 | User feature request | Breakdown aggregation engine |
| T-108 | P1 | [x] | Recalculate execution breakdown whenever the current drill-down stage changes. | T-098, T-107 | User feature request | Stage-aware breakdown integration |
| T-109 | P1 | [x] | Add CLI command `python -m archscope_engine.cli profiler breakdown` for collapsed-stack and Jennifer CSV profiler inputs, including optional current-stage/filter arguments. | T-103, T-107 | User feature request | Breakdown CLI command |
| T-110 | P1 | [x] | Update Profiler Analyzer UI for iterative drill-down: filter builder, stage creation, breadcrumb navigation `All > filter1 > filter2`, per-stage flamegraph rendering, metrics, top stacks, and top child frames. | T-098, T-102, T-103 preferred | User feature request | Multi-stage drill-down UI |
| T-111 | P1 | [x] | Add execution breakdown UI tied to the selected stage with donut chart, horizontal bar chart, and category top stack table. | T-108, T-110 | User feature request | Breakdown charts and table |
| T-112 | P1 | [x] | Add regression tests for Jennifer CSV parser tree reconstruction, collapsed stack to flame tree conversion, multi-stage drill-down, ordered match, and re-root mode. | T-094, T-095, T-098, T-100, T-101 | User feature request | Profiler drill-down parser/engine tests |
| T-113 | P1 | [x] | Add execution breakdown tests for SQL, HTTP external call, network wait, Hikari pool wait, and unknown classification. | T-104, T-106, T-107 | User feature request | Breakdown classifier tests |
| T-114 | P2 | [x] | Update documentation for the new profiler features in root README plus English/Korean architecture, parser design, chart design, data model, and roadmap docs while preserving language-specific documentation. | T-093 through T-113 | User feature request | Updated README and docs |

### JVM Analyzer MVP Expansion

Goal: replace the remaining GC log, thread dump, and exception analyzer placeholders with small but real parser/analyzer/CLI/UI paths while preserving parser/analyzer separation and the common `AnalysisResult` contract.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-115 | P1 | [x] | Implement HotSpot unified GC log parser and `gc_log` analyzer result with pause, heap, cause/type breakdowns, diagnostics, and findings. | T-012 | User follow-up | GC parser/analyzer MVP |
| T-116 | P1 | [x] | Implement Java thread dump parser and `thread_dump` analyzer result with state/category distributions, lock evidence, stack signatures, diagnostics, and findings. | T-012 | User follow-up | Thread dump parser/analyzer MVP |
| T-117 | P1 | [x] | Implement Java exception stack parser and `exception_stack` analyzer result with type/root-cause/signature aggregation, diagnostics, and findings. | T-012 | User follow-up | Exception parser/analyzer MVP |
| T-118 | P1 | [x] | Add CLI commands for JVM analyzers: `gc-log analyze`, `thread-dump analyze`, and `exception analyze`. | T-115, T-116, T-117 | User follow-up | CLI analyzer paths |
| T-119 | P2 | [x] | Wire JVM analyzer result types through TypeScript contracts, Electron IPC validation, and existing analyzer UI pages. | T-118, T-048 | User follow-up | Desktop analyzer connectivity |
| T-120 | P2 | [x] | Add sample inputs and regression tests for JVM parser/analyzer behavior. | T-115, T-116, T-117 | User follow-up | JVM samples and tests |
| T-121 | P2 | [x] | Update README, parser design, data model, roadmap, and work status docs for JVM analyzer MVP scope. | T-115 through T-120 | User follow-up | Updated English/Korean docs |

### Parser Debug Log - Portable Field Support

Goal: create a redacted, portable debug log JSON that a field user can send as a single artifact so developers can identify parser failures without receiving the full original log file.

Scope decisions:

- Default debug output location is `archscope-debug/` under the ArchScope execution directory, not the input log directory.
- Redaction is enabled by default and must preserve parsing structure: delimiters, quotes, brackets, timestamp shape, numeric shape, field count, failed pattern, partial match, and field shapes.
- Unredacted debug logs are out of scope unless a user explicitly requests an unsafe diagnostic mode in a later task.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-122 | P0 | [x] | Implement `common/debug_log.py` with `DebugLogCollector`, portable filename generation, verdict calculation, per-error-type sample limits, 1 MB cap, environment/context metadata, and JSON writing. | T-043, T-121 | `docs/work-instructions/parser-debug-log.md` | Debug log collector core |
| T-123 | P0 | [x] | Implement `common/redaction.py` for default-on masking of tokens, cookies, query values, emails, long identifiers, IP/host/user identifiers, and absolute paths while preserving parseable structure. | T-122 | User follow-up | Redaction engine and tests |
| T-124 | P0 | [x] | Add field-shape extraction helpers for line, URL, CSV row, and JFR JSON contexts so masked logs still preserve parser-fix evidence. | T-123 | User follow-up | `field_shapes` support |
| T-125 | P1 | [x] | Expose text encoding detection metadata from `file_utils` without breaking existing `iter_text_lines` callers. | T-122 | Work-instruction review | Encoding metadata API |
| T-126 | P1 | [x] | Integrate debug log collection into access log and collapsed parsers, including before/target/after context, failed pattern names, partial match where available, and redaction. | T-122, T-123, T-124, T-125 | Work-instruction review | Access/collapsed debug logs |
| T-127 | P1 | [x] | Integrate debug log collection into GC, thread dump, exception, and Jennifer CSV parsers with parser-specific line/row context and field shapes. | T-126 | Work-instruction review | JVM/Jennifer debug logs |
| T-128 | P1 | [x] | Add `ParserDiagnostics` and debug log support to JFR JSON parsing using JSON path/event index context instead of only raising `ValueError`. | T-122, T-124 | Work-instruction review | JFR diagnostics/debug logs |
| T-129 | P1 | [x] | Add analyzer-level exception capture so fatal and partial parser failures record traceback, phase, context, and `FATAL_ERROR` verdict consistently. | T-122, T-126, T-128 | Work-instruction review | Exception-aware debug logs |
| T-130 | P1 | [x] | Add CLI options `--debug-log` and `--debug-log-dir` across analyzer commands, with default output under execution-directory `archscope-debug/`. | T-122, T-129 | Work-instruction review | CLI debug log controls |
| T-131 | P2 | [x] | Wire Electron portable debug-log directory handling through `main.ts`, using executable directory in packaged mode and repo/cwd in development mode. | T-130, T-038 | User follow-up | Portable desktop debug path |
| T-132 | P1 | [x] | Add regression tests for clean/no-file behavior, forced clean debug log, per-type sample limit, context accuracy, redaction, field-shape preservation, file-size cap, JFR diagnostics, and filename pattern. | T-122 through T-131 | Work-instruction review | Debug log test suite |
| T-133 | P2 | [x] | Update README and English/Korean parser/data/architecture docs for portable redacted parser debug logs. | T-132 | Work-instruction review | Documentation updates |

### Report Automation, Chart Studio, and Multi-runtime MVPs

Goal: add the first useful slices of the remaining roadmap without rebuilding the architecture: static HTML report export, a usable Chart Studio template preview screen, and small parser/analyzer/CLI paths for non-JVM runtimes.

Scope exclusions for this cycle: PowerPoint export, before/after report diff, full interactive HTML flamegraph export, OpenTelemetry correlation, large-file optimization, and production-grade structured log ingestion.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-134 | P1 | [x] | Implement a static HTML report exporter that renders `AnalysisResult` JSON summary, findings, diagnostics, series, tables, and chart data previews. | T-009, T-121, T-133 | User follow-up | `html_exporter.py` |
| T-135 | P1 | [x] | Allow parser debug JSON to render through the same HTML report path with verdict, redaction state, parse error groups, samples, exceptions, and hints. | T-122, T-134 | User follow-up | Parser-debug HTML report support |
| T-136 | P1 | [x] | Add CLI command `python -m archscope_engine.cli report html --input ... --out ...`. | T-134, T-135 | User follow-up | Report HTML CLI |
| T-137 | P2 | [x] | Replace the Chart Studio placeholder with a template catalog preview that supports title override, renderer selection, theme selection, export preset metadata, and ECharts option JSON inspection. | T-021, T-054, T-055 | User follow-up | Usable Chart Studio MVP |
| T-138 | P2 | [x] | Add Node.js stack parser/analyzer/CLI MVP for `Error`/`TypeError` style stack blocks. | T-043, T-121 | User follow-up | `nodejs_stack` result |
| T-139 | P2 | [x] | Add Python traceback parser/analyzer/CLI MVP for standard traceback blocks. | T-043, T-121 | User follow-up | `python_traceback` result |
| T-140 | P2 | [x] | Add Go panic/goroutine parser/analyzer/CLI MVP. | T-043, T-121 | User follow-up | `go_panic` result |
| T-141 | P2 | [x] | Add .NET exception and IIS W3C parser/analyzer/CLI MVP. | T-043, T-121 | User follow-up | `dotnet_exception_iis` result |
| T-142 | P2 | [x] | Add sample runtime artifacts for Node.js, Python, Go, and .NET/IIS. | T-138 through T-141 | User follow-up | `examples/runtime/*` fixtures |
| T-143 | P1 | [x] | Add regression tests for HTML export, runtime analyzers, and CLI output. | T-134 through T-142 | User follow-up | Python test coverage |
| T-144 | P2 | [x] | Update README and English/Korean report, chart, parser, data model, and roadmap docs. | T-143 | User follow-up | Documentation updates |
| T-145 | P1 | [x] | Add before/after comparison report generation that compares numeric summary metrics and finding counts without re-parsing raw evidence. | T-134 | User follow-up | `comparison_report` result and `report diff` CLI |
| T-146 | P1 | [x] | Add minimal PowerPoint `.pptx` export from `AnalysisResult` JSON with title, source metadata, summary metrics, and findings slides. | T-134 | User follow-up | `report pptx` CLI |
| T-147 | P1 | [x] | Render profiler `charts.flamegraph` as a static HTML flamegraph section in the report exporter. | T-134, T-093 | User follow-up | Static flamegraph HTML export |
| T-148 | P2 | [x] | Add OpenTelemetry JSONL log parser/analyzer/CLI MVP with trace, service, severity, and cross-service trace grouping. | T-043, T-079 | User follow-up | `otel_logs` result and `otel analyze` CLI |
| T-149 | P2 | [x] | Add OTel sample JSONL fixture and regression tests for analyzer and CLI behavior. | T-148 | User follow-up | `examples/otel/sample-otel-logs.jsonl` and tests |
| T-150 | P2 | [x] | Update README and English/Korean report/parser/data/roadmap docs for report diff, PPTX, static flamegraph HTML, and OTel JSONL MVP. | T-145 through T-149 | User follow-up | Documentation updates |
| T-151 | P1 | [x] | Connect Export Center to the Python report CLI through Electron IPC for HTML, before/after diff, and PPTX exports. | T-136, T-145, T-146 | User follow-up | `export:execute` bridge |
| T-152 | P1 | [x] | Replace Export Center placeholder with JSON selection, export execution, generated path display, engine messages, and error states. | T-151 | User follow-up | Functional Export Center UI |

### Demo-Site Data Center and Report Bundles

Goal: make the shared `projects-assets/test-data/demo-site` corpus executable from
the CLI and desktop UI, while keeping reusable demo manifests and analyzer
mapping in the shared asset repository.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-153 | P1 | [x] | Add demo-site manifest runner that reads scenario manifests and writes per-scenario JSON/HTML/PPTX bundles. | T-136, T-146, T-148 | User follow-up | `demo-site run` CLI and `scripts/run-demo-site-data.sh` |
| T-154 | P1 | [x] | Generate scenario and root `index.html` files with analyzer output summary, skipped line report, failed analyzer report, reference-only files, and expected signals. | T-153 | User follow-up | Portable demo bundle indexes |
| T-155 | P1 | [x] | Generate normal-baseline comparison reports for every matching analyzer type, not only access logs. | T-145, T-153 | User follow-up | `normal-baseline-vs-<analyzer_type>` outputs |
| T-156 | P1 | [x] | Add Demo Data Center UI for manifest root selection, real/synthetic filters, all-scenario execution, artifact opening, Export Center handoff, failed/skipped summaries, and reference-only context. | T-153, T-151 | User follow-up | Desktop demo execution flow |
| T-157 | P1 | [x] | Single-source demo `analyzer_type` command mapping by reading `projects-assets/test-data/demo-site/analyzer_type_mapping.json` at runtime. | T-153 | User follow-up | `demo_site_mapping.py` loader and `demo-site mapping` CLI |
| T-158 | P1 | [x] | Strengthen OTel cross-service analysis with parent-span service paths, first failing service, downstream propagation, and propagation findings. | T-148 | User follow-up | OTel parent-span path and failure propagation tables |
| T-159 | P2 | [x] | Compare OTel demo analysis against manifest expected services and trace counts, and emit mismatch findings. | T-153, T-158 | User follow-up | Demo manifest validation metadata |
| T-160 | P2 | [x] | Increase desktop demo-site run timeout for larger scenario batches and keep running-state feedback in the UI. | T-156 | User follow-up | 5-minute demo runner IPC timeout |
| T-161 | P2 | [x] | Document demo-site CLI/UI workflow, output structure, analyzer mapping source, OTel behavior, and remaining manual UI verification. | T-153 through T-160 | User follow-up | README and English/Korean docs |
| T-162 | P2 | [x] | Add Playwright/Electron smoke automation for Demo Data Center after a UI test harness is introduced. | T-156 | User follow-up | Deferred automated UI smoke test |

### Final Performance, Defect, and Algorithm Review Hardening

Goal: close the high-risk items from the 2026-05-01 final review before deeper optimization work. The review was based on commit `2e1a0ae`; AI client findings already closed by later commits are not duplicated here.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-167 | P0 | [x] | Fix `BoundedPercentile` reservoir sampling so large inputs continue to be represented after the reservoir fills, add sorted percentile cache, and add large-distribution regression tests. | T-049 | RD-105, RD-108, RD-115 | Correct Algorithm R sampling plus tests |
| T-168 | P0 | [x] | Stop profiler drill-down from mutating the source `AnalysisResult`; return a new result with copied result sections and preserve `created_at`. | T-098 | RD-106 | Immutable drill-down result handling plus regression test |
| T-169 | P0 | [x] | Add baseline performance measurement infrastructure for core Python analyzers, including a dependency-free benchmark script and English/Korean profiling guide. | None | RD-107 | `benchmarks/core_benchmark.py` and performance docs |
| T-170 | P1 | [x] | Remove the double file-read path in text encoding detection while preserving debug-log encoding metadata. | T-125, T-169 | RD-111 | Single-pass or bounded-probe text line iteration |
| T-171 | P1 | [x] | Add ReDoS protection for user-provided profiler drill-down regular expressions. | T-098, T-169 | RD-112 | Regex length/timeout or safe-regex engine guard |
| T-172 | P1 | [x] | Detect and report OTel span parent cycles and self-parent links without dropping trace topology evidence silently. | T-158 | RD-113 | OTel topology diagnostics and tests |
| T-173 | P1 | [x] | Refactor GC log analysis toward streaming aggregation for large logs while preserving existing summary, series, table, and findings output. | T-115, T-167 | RD-109 | Memory-bounded GC analyzer path |
| T-174 | P2 | [x] | Replace access-log URL full sorting with heap/top-k `Counter.most_common()` semantics. | T-169 | RD-116 | Lower-cost URL top-N aggregation |
| T-175 | P2 | [x] | Add ECharts large-data options and split `ChartPanel` initialization from option updates to avoid unnecessary chart reinitialization. | T-137, T-169 | RD-119, RD-120 | Large-chart rendering optimization |
| T-176 | P2 | [x] | Tighten OTel `_is_error_record` classification to reduce body-keyword false positives. | T-158 | RD-122 | More accurate OTel error statistics |
| T-177 | P2 | [x] | Cache repeated profiler stack classification work where profiles contain repeated stack keys. | T-104, T-169 | RD-125 | Lower-cost component breakdown |
| T-178 | P2 | [x] | Add CI benchmark publishing/regression alerts after local benchmark output stabilizes. | T-169 | RD-107 | Performance regression visibility in CI |
| T-179 | P2 | [x] | Add analyzer cancellation support from UI through Electron process control. | T-038, T-070 | RD-114 | User-visible cancel path |

### Web Platform Migration, UI Redesign, Multi-Language Diagnostics Expansion, and Diagnostics Polish

Goal: pivot ArchScope from an Electron desktop app to a browser-served web app with a Python-only install (`pip install archscope && archscope serve`), redesign the UI on top of the new web runtime so analyzer pages are migrated **once**, expand the profiler to consume FlameGraph SVG/HTML, build a language-agnostic multi-dump thread-analysis framework with Java/Go/Python/Node.js/.NET parser plugins, and finish with interactive GC timeline plus thread-dump→flamegraph conversion.

Reference source tree (read-only research input — do not import Java sources directly, port behavior into `archscope_engine`):

- FlameGraph SVG/HTML rendering reference — `aitop/10. projects/aitop-tools/src/main/java/com/lgcns/aitop/tools/thread/flamegraph/{FlameGraphGenerator,FlameGraphSVGGenerator,StackCollapseParser}.java`.
- JVM-only proxy/state inference reference — `aitop/10. projects/aitop-tools/src/main/java/com/lgcns/aitop/tools/thread/ThreadDumpAnalyzerTools.java` (`cleanProxyClassNames` is Java/Spring AOP only; `epollWait`/`socketRead0` are JVM frame names — both must stay behind language-tagged plugins, never applied against non-Java frames).
- Stack collapse for thread-dump→flamegraph batch conversion — same `ThreadDumpAnalyzerTools.java:401-513` (`convertToCollapsedFormat`).

Scope exclusions for this cycle:

- Heap dump analysis is **out of scope this cycle** — no HPROF parsing, no Eclipse MAT bundling, no MAT-result viewer. The aitop heap path is intentionally not ported.
- Playwright iTop+ automation (`Itop*Analyzer.java`) — automation, not analysis.
- PowerPoint expansion of new outputs.

Design notes:

- "Multi-language" thread-dump framework keeps language-specific cleanup and state-inference rules behind language-tagged enrichment plugins. The common engine never special-cases JVM frame names.
- A correlation bundle is a list of dumps from **one process at successive points in time**. Within a bundle, all files must share the same `source_format`/language; mixed-language bundles are rejected with a clear error.
- Auto-detection sniffs the first ~4 KB of each file; users can override with an explicit `--format` flag.

#### Execution order (enforced to avoid double-work)

The platform pivot lands **before** any UI rebuild because retouching Electron-era components that will be rebuilt on top of the web stack would force the same migration twice.

1. **Phase 1 — Web platform migration (P0, T-206 → T-207 → T-208 → T-209)**: design the FastAPI + uvicorn runtime, remove the Electron shell, ship `pip install archscope && archscope serve`, and run a cross-browser smoke test. All later UI work assumes the web shell.
2. **Phase 2 — UI redesign on the web shell (P1, T-210 → T-211 → T-212 → T-213)**: layout redesign with file-selection top strip and tabbed content, collapsible sidebar, Tailwind + shadcn/ui design system, slim global topbar. Analyzer pages are migrated **once** here.
3. **Phase 3 — Electron-era UI cleanups subsumed by Phase 2**: `T-180`/`T-181`/`T-182`/`T-183` remain in the backlog as superseded references and **must not be implemented as separate Electron-era patches** — see the per-task supersession notes below.
4. **Phase 4 — Profiler SVG/HTML inputs (parallel with Phases 1-2 for backend; UI waits on Phase 2)**: T-184/T-185/T-186 are pure Python parser/analyzer work and can land any time. T-187 (UI exposure) waits until Phase 2 finishes so it lands once on the new design system.
5. **Phase 5 — Multi-language thread-dump framework (parallel backend, sequential UI)**: T-188–T-192, T-194–T-205 backend/CLI work runs in parallel with Phases 1-2. T-193 (multi-dump UI) waits on Phase 2.
6. **Phase 6 — Diagnostics polish (P2, T-214 → T-215, T-216 → T-217)**: GC interactive timeline and algorithm comparison build on Phase 2 + the existing GC analyzer. Thread-dump→collapsed batch conversion (T-216) and Canvas/WebGL flamegraph rendering (T-217) build on the Java parser plugin (T-194) and Phase 2.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-180 | P1 | [~] | **[Superseded by T-213 — do not implement as a separate Electron-era patch.]** Original intent: remove the `main-panel > header.topbar` block in `apps/desktop/src/components/Layout.tsx` (`appTagline` eyebrow + `ArchScope` `h1` + `.locale-switcher`) and reclaim the wasted top space. The Phase 2 slim-topbar redesign replaces this layout entirely; the locale and brand are handled by T-211/T-213. | None | User feature request | Tracked-as-superseded; behavior delivered by T-213 |
| T-181 | P1 | [~] | **[Superseded by T-213 — do not implement as a separate Electron-era patch.]** Original intent: relocate the locale `<select>` from the right-pane header into `Sidebar.tsx`. The Phase 2 slim global topbar consolidates locale, search, and settings together; the sidebar receives only the brand mark plus the collapsible nav from T-211. | T-180 | User feature request | Tracked-as-superseded; behavior delivered by T-213 |
| T-182 | P1 | [~] | **[Superseded by T-209 — once the app runs in the browser, the standard `<input type="file">` and `selectFile` HTTP/uploads handle file selection without Electron sandbox path restrictions.]** Original intent: remove the `<input type="file">` from `FileDropZone.tsx` because Electron's sandbox does not expose `File.path` reliably. After Phase 1, FileDropZone is rebuilt against the web file-selection contract decided in T-206. | None | User feature request | Tracked-as-superseded; behavior delivered by T-207/T-209 |
| T-183 | P2 | [~] | **[Superseded by T-210 — the Phase 2 layout redesign sweeps every analyzer page onto the new top-fixed file strip + tabbed content area.]** Original intent: audit every analyzer page to confirm none of them depended on the now-removed Electron `<input type="file">`. The audit is folded into the per-page migration in T-210. | T-182 | User feature request | Tracked-as-superseded; behavior delivered by T-210 |
| T-184 | P1 | [x] | Added `parsers/svg_flamegraph_parser.py` that ingests FlameGraph.pl/async-profiler-style SVG (rect-per-frame with `<title>function (samples samples, p%)</title>`) and reconstructs collapsed stacks (`Counter[str]`) compatible with `T-093`. Uses `defusedxml.ElementTree` for XXE-safe parsing + `ParserDiagnostics`; auto-detects Brendan default vs icicle layout; fails gracefully on non-FlameGraph SVGs. | T-093, T-094 | User feature request | SVG flamegraph → collapsed stacks |
| T-185 | P1 | [x] | Added `parsers/html_profiler_parser.py` that detects the two common HTML flamegraph variants — async-profiler self-contained HTML (embedded `var root = {...}` JS tree, with parent self-time tracking) and inline-SVG-wrapped HTML — and routes to the SVG parser or to a JS-tree extractor. Skips arbitrary HTML and emits `UNSUPPORTED_HTML_FORMAT` diagnostics. | T-184 | User feature request | HTML flamegraph → collapsed stacks |
| T-186 | P1 | [x] | Extended `analyzers/profiler_analyzer.py` (new `analyze_flamegraph_svg_profile` / `analyze_flamegraph_html_profile`), the `profiler analyze-flamegraph-svg|html` CLI commands, and the FastAPI `/api/analyzer/execute` dispatch with new `profile_format` values `flamegraph_svg` and `flamegraph_html` that reuse `T-098/T-104` drill-down and breakdown after parser conversion. Preserves the existing collapsed/Jennifer code paths. | T-184, T-185, T-098, T-104 | User feature request | Profiler analyzer accepts SVG/HTML inputs |
| T-187 | P1 | [x] | Wired the new profile formats into the Phase-2 ProfilerAnalyzer page: `flamegraph_svg`/`flamegraph_html` added to the `profileFormat` selector, FileDock `accept` adapts (`.svg` / `.html,.htm`), and engine diagnostics (including `UNSUPPORTED_HTML_FORMAT`) flow through the existing feedback panel. Ships on the redesigned UI. | T-186, T-212 | User feature request | UI exposes SVG/HTML profiler inputs |
| T-188 | P0 | [x] | Defined language-agnostic `ThreadSnapshot` and `ThreadDumpBundle` dataclasses in `models/thread_snapshot.py` plus a `ThreadState` str-enum (RUNNABLE/BLOCKED/WAITING/TIMED_WAITING/NETWORK_WAIT/IO_WAIT/LOCK_WAIT/CHANNEL_WAIT/DEAD/NEW/UNKNOWN) with a `coerce()` alias map. `StackFrame` carries `function`, `module`, `file`, `line`, `language`. The existing JVM `ThreadDumpRecord` stays intact for the single-dump path; the multi-dump path uses the new model. | T-116 | User feature request | Multi-language record contract |
| T-189 | P0 | [x] | Implemented `parsers/thread_dump/registry.py` with a `ThreadDumpParserPlugin` Protocol (`format_id`, `language`, `can_parse(head: str) -> bool`, `parse(path) -> ThreadDumpBundle`) and a `ParserRegistry` that probes the first 4 KB of each input to choose a parser. `parse_many` rejects bundles whose files resolve to mixed `source_format` values via `MixedFormatError`. Accepts a `format_override` that bypasses detection. `DEFAULT_REGISTRY` is the package-level singleton. | T-188 | User feature request | Parser plugin interface + auto-detection |
| T-190 | P0 | [x] | Ported the existing JVM jstack parser into a `JavaJstackParserPlugin` (in `parsers/thread_dump/java_jstack.py`) auto-registered with `DEFAULT_REGISTRY`. Detection: `Full thread dump` header or quoted-name + `nid=0x` line. Reuses `parsers/thread_dump_parser.py` for the heavy lifting; the plugin only adapts each `ThreadDumpRecord` into `ThreadSnapshot`/`StackFrame`. | T-188, T-189 | User feature request | Java reference plugin |
| T-191 | P0 | [x] | Implemented multi-dump correlation in `analyzers/multi_thread_analyzer.py` over `ThreadSnapshot`. For each thread name, builds a sorted observation timeline and emits findings `LONG_RUNNING_THREAD` (RUNNABLE for ≥3 consecutive dumps with the same stack signature) and `PERSISTENT_BLOCKED_THREAD` (BLOCKED/LOCK_WAIT for ≥3 consecutive dumps). Output `AnalysisResult(type="thread_dump_multi")` carries `summary.languages_detected`, per-dump state distribution, top-N persistence table, and per-thread state-transition timeline. Language-agnostic: never imports JVM helpers directly. | T-188, T-190 | User feature request | Common correlation engine |
| T-192 | P1 | [x] | Added CLI command `archscope-engine thread-dump analyze-multi --input <f1> --input <f2> ... [--format <id>] [--consecutive-threshold N] [--top-n N]`. Auto-detects per file unless `--format` overrides; writes a `thread_dump_multi` JSON via `write_json_result`. | T-189, T-191 | User feature request | Multi-dump CLI |
| T-193 | P1 | [x] | Replaced the `PlaceholderPage`-driven ThreadDumpAnalyzer page with a custom multi-file ThreadDumpAnalyzerPage on the Phase-2 shell: cumulative file list (FileDock adds; per-row `X` removes), `--consecutive-threshold` and `--format` inputs, FastAPI dispatch (`/api/analyzer/execute` `type:"thread_dump_multi"`), Findings tab with severity-colored cards + LONG_RUNNING / PERSISTENT_BLOCKED tables, D3BarChart for per-dump thread counts and top-thread persistence, and the standard "Save all charts" 2x batch export. | T-192, T-212 | User feature request | Multi-language multi-dump UI |
| T-194 | P2 | [x] | Implemented Java/Spring AOP proxy normalization in `parsers/thread_dump/java_jstack.py::_normalize_proxy_frame`. Applies only when `StackFrame.language == "java"`. Patterns collapsed: `$$EnhancerByCGLIB$$<hex>` (strip), `$$FastClassByCGLIB$$<hex>` (strip), `(\$\$Proxy)\d+` (drop digits), `(GeneratedMethodAccessor)\d+`, `(GeneratedConstructorAccessor)\d+`, `(Accessor)\d+` (drop digits). Two requests with different proxy hashes now collapse to the same stack signature. | T-190 | aitop-tools (Java only) | Java AOP-aware signatures |
| T-195 | P2 | [x] | Implemented JVM network/IO state inference in `parsers/thread_dump/java_jstack.py::_infer_java_state`. RUNNABLE threads stuck in `EPoll.epollWait` / `EPollSelectorImpl.doSelect` / `socketAccept` / `socketRead0` / `NioSocketImpl.read` / Netty NIO event loop become `NETWORK_WAIT`; threads stuck in `FileInputStream.read*` / `FileChannelImpl.read` / `RandomAccessFile.read*` / `BufferedReader.readLine` become `IO_WAIT`. Never overrides BLOCKED/WAITING/TIMED_WAITING. | T-190 | aitop-tools (Java only) | Java network/IO state inference |
| T-196 | P1 | [x] | Implemented `GoGoroutineParserPlugin` (`parsers/thread_dump/go_goroutine.py`) for `runtime/debug.Stack()`, `runtime.Stack`, and panic-style goroutine dumps. Detection: `^goroutine \d+ \[\w` via MULTILINE search. Frame pairs `package.func(args)` + `\tfile.go:LINE +0xOFFSET`; "created by" parent attached as the bottom frame. State derived from `[STATE, DURATION?]` via `ThreadState.coerce` (e.g. `chan receive` → CHANNEL_WAIT). Auto-registers with `DEFAULT_REGISTRY`. | T-188, T-189 | User feature request | Go goroutine parser |
| T-197 | P2 | [x] | Implemented Go framework cleanup + state inference inside `go_goroutine.py`. Cleanup: `gin.HandlerFunc.func1` → `gin.HandlerFunc`, `echo.(*Echo).ServeHTTP`/`chi.(*Mux).routeHTTP`/`fiber.(*App).Handler` preserved as receiver method; trailing anonymous `.func1.func2` chains stripped. State inference: `gopark`/`runtime.selectgo`/`chanrecv`/`chansend` → CHANNEL_WAIT; `runtime.netpoll`/`netpollblock`/`net.(*conn).Read` → NETWORK_WAIT; `semacquire`/`sync.(*Mutex).Lock`/`sync.(*RWMutex).Lock` → LOCK_WAIT; file IO → IO_WAIT. Applied only when `frame.language == "go"`. | T-196 | User feature request | Go framework + state enrichment |
| T-198 | P1 | [x] | Implemented two Python plugins (`parsers/thread_dump/python_dump.py`): `PythonPySpyParserPlugin` (detection: `Process \d+:` + `Python v?\d+\.\d+`; frames: `module.func (file:line)`) and `PythonFaulthandlerParserPlugin` (detection: `Thread 0x[0-9a-f]+ \(most recent call first\):`; frames: `File "/abs/foo.py", line 42 in func`). Both emit `StackFrame(language="python")` and auto-register. | T-188, T-189 | User feature request | Python py-spy/faulthandler parsers |
| T-199 | P2 | [x] | Implemented Python framework cleanup + state inference in `python_dump.py::_finalize_python_snapshot`. Cleanup drops Django/Starlette/FastAPI/Flask/gunicorn/uvicorn/werkzeug wrapper frames whose function matches `__call__`/`MiddlewareMixin.__call__`/`solve_dependencies`/`run_endpoint_function`/`view_func`/`wrapper`/`dispatch_request` *and* whose file path is in a framework directory. State inference uses (function, file) pairs: socket recv/send/accept/connect → NETWORK_WAIT; urllib3/requests/httpx/http.client → NETWORK_WAIT; threading.{acquire,wait}/queue.get → LOCK_WAIT; select.{select,poll,epoll,kqueue}/asyncio sleep/run_forever/gevent → IO_WAIT. | T-198 | User feature request | Python framework + state enrichment |
| T-200 | P1 | [x] | Implemented `NodejsDiagnosticReportParserPlugin` (`parsers/thread_dump/nodejs_report.py`) for `process.report.writeReport()` JSON. Detection: regex `"header"\s*:\s*\{...?"javascriptStack"\s*:` so we recognize the file with only the first 4 KB. JS frames go into `StackFrame(language="javascript")`, native frames (from `nativeStack[]`) into `StackFrame(language="native")` so the JS enrichment plugin can target only JS frames. `libuv` resource list (first 50) and selected `header` keys (event/trigger/filename/nodejsVersion) preserved under `bundle.metadata`. | T-188, T-189 | User feature request | Node.js diagnostic-report parser |
| T-201 | P2 | [x] | Implemented Node.js framework cleanup + state inference inside `nodejs_report.py`. Cleanup strips Express `Layer.handle [as handle_request]` aliases. State inference reads `payload["libuv"]`: any active TCP/UDP/pipe handle → main thread NETWORK_WAIT; any active timer/fs_event/fs_poll → IO_WAIT. Applied only to `language == "javascript"` frames. | T-200 | User feature request | Node.js framework + state enrichment |
| T-202 | P2 | [x] | Implemented `DotnetClrstackParserPlugin` (`parsers/thread_dump/dotnet_clrstack.py`) for `dotnet-dump analyze` clrstack output. Detection: `^OS Thread Id:\s+0x` MULTILINE. Each block parses `Child SP / IP / Call Site` lines into `StackFrame(language="dotnet")`. Light cleanup: `<Outer>g__Inner|3_0` synthesized local-function name → `Outer.Inner`; async state machine `MyApp.Async.<DoWorkAsync>d__0.MoveNext()` → `MyApp.Async.DoWorkAsync.MoveNext`. State inference: Monitor.Enter / SpinLock / SemaphoreSlim → LOCK_WAIT; Socket.Receive/Send / NetworkStream.Read / HttpClient.Send → NETWORK_WAIT; FileStream.Read / StreamReader → IO_WAIT. `Sync Block Owner Info:` table captured under `bundle.metadata.sync_block_owner_info`. | T-188, T-189 | User feature request | .NET dotnet-dump parser |
| T-203 | P2 | [x] | Implemented `LATENCY_SECTION_DETECTED` (warning) in `analyzers/multi_thread_analyzer.py::_build_latency_sections`. Walks each thread's observation timeline and, for every wait category in `_LATENCY_WAIT_CATEGORIES = (NETWORK_WAIT, IO_WAIT, CHANNEL_WAIT)`, records the longest consecutive-dump run; emits one finding per (thread, category) pair that meets `consecutive_dump_threshold`. `LOCK_WAIT` is intentionally excluded (already owned by `PERSISTENT_BLOCKED_THREAD`). Also added `summary.latency_sections` count and `tables.latency_sections` rows. Language-agnostic: only consumes `ThreadState`. | T-195, T-197, T-199, T-201 | aitop-tools (generalized) | Cross-language latency finding |
| T-204 | P2 | [x] | Added end-to-end regression suite `tests/test_phase5_regression.py` (21 cases): registry routing across all six format signatures, mixed-language guard rail (`MixedFormatError`) and `--format` override, parametric 3-dump correlation per language emits at least one finding, Java fixture asserts both `PERSISTENT_BLOCKED_THREAD` and a `NETWORK_WAIT` `LATENCY_SECTION_DETECTED`, Go fixture asserts a `CHANNEL_WAIT` latency, plus pure-analyzer scenarios for the latency finding (fires, doesn't fire on broken runs, never duplicates `LOCK_WAIT`, runs per-category independently, table is exposed). Profiler SVG/HTML / Java AOP / Go closure / Python boilerplate / Node.js layer / .NET sync-block coverage already lives in the per-language test files added in earlier commits. | T-187, T-193, T-194, T-197, T-199, T-201, T-202, T-203 | User feature request | Profiler + multi-language thread-dump regression coverage |
| T-205 | P2 | [x] | Added bilingual user-facing docs `docs/en/MULTI_LANGUAGE_THREADS.md` and `docs/ko/MULTI_LANGUAGE_THREADS.md` covering: supported formats (with detection signatures), normalized data model (`StackFrame` / `ThreadSnapshot` / `ThreadDumpBundle`), `ThreadState` enum and `coerce()` aliases, per-language enrichment matrix (frame normalization + state promotion table), the three multi-dump findings (`LONG_RUNNING_THREAD` / `PERSISTENT_BLOCKED_THREAD` / `LATENCY_SECTION_DETECTED`), CLI (`thread-dump analyze` / `analyze-multi`), HTTP / UI surface, profiler SVG/HTML cross-reference (T-184~T-187), and the explicit deferral of heap dump analysis. | T-204 | User feature request | Documentation updates |
| T-206 | P0 | [x] | Added a "Web platform pivot — design decisions (T-206)" section to `docs/en/ARCHITECTURE.md` and `docs/ko/ARCHITECTURE.md`, **explicitly superseding T-001** (Electron + `child_process.execFile` IPC). Locks: FastAPI + uvicorn in-process runtime; HTTP for analyzer/file/settings/exports + WebSocket `/ws/progress` for engine progress streaming + cancellation (task-registry pattern symmetric with the Wails track T-240f); file selection — server-side absolute path via `POST /api/files/select` by default with `POST /api/upload` multipart as a non-localhost fallback; packaging — single top-level `archscope` Python distribution with the React bundle shipped as wheel data resolved through `importlib.resources`, console script wrapping uvicorn so `pip install archscope && archscope serve` is the install path; CSP/CORS — `allow_origins=["http://127.0.0.1:5173"]` for the Vite dev server only, `--no-dev-cors` for hardened deploy, CSP with `connect-src 'self' ws://127.0.0.1:8765` for the progress stream and `style-src 'unsafe-inline'` retained for shadcn variables; final apps/ layout post-T-207 = `frontend/` + `profiler-native/` only (the Electron `apps/desktop/` directory is deleted). | T-001, T-038 | User feature request | Web platform design note |
| T-207 | P0 | [x] | Electron shell deleted (`apps/desktop/` removed entirely — Electron never carried React source, the frontend already lived at `apps/frontend/`, so no rename was needed). `scripts/build-desktop.sh` removed. CI's `desktop` job in `.github/workflows/ci.yml` re-pointed to `apps/frontend` (lint / vitest / playwright e2e / vite build). The previously-existing FastAPI server at `engines/python/archscope_engine/web/server.py` (which T-001 had already migrated under) gained two T-206 endpoints: `POST /api/files/select` (server-side absolute-path normaliser with existence + file-type checks) and `WebSocket /ws/progress` (subscribe / cancel protocol). `POST /api/analyzer/cancel` was promoted from a no-op to a real task-id keyed cancel. New `archscope_engine.web.progress.ProgressRegistry` (threading.Lock around connection map + cancel events) is the single source of truth for both endpoints, symmetric with the Wails track's task registry from T-240f. `apps/frontend/src/api/httpBridge.ts` already mounts `window.archscope` over HTTP (no further changes needed at this slice). README and `apps/frontend/package.json` description updated to remove Electron references. Smoke import + route check confirms 19 routes including `/api/files/select` and `/ws/progress`. | T-206 | User feature request | Web-only runtime, no Electron |
| T-208 | P0 | [x] | Renamed the Python distribution `archscope-engine` → **`archscope`** in `engines/python/setup.cfg` (existing `archscope-engine` console script kept for backward compatibility). New `archscope_engine.app` Typer wrapper inherits every command/group from the engine CLI (excluding the engine's own `serve` group) and adds a UX-friendly `archscope serve` (auto-opens the default browser unless `--no-browser` or `ARCHSCOPE_NO_BROWSER` is set, prints "ArchScope serving on …" banner). New `archscope_engine.web.static/` directory is the wheel data drop-zone for the React bundle; `setup.cfg` `[options.package_data]` ships `*` + `**/*` from there, and `_resolve_static_dir` (server.py) now searches `importlib.resources` of `archscope_engine.web.static` (with `index.html` presence check) before the dev-tree fallback. New `scripts/build-archscope-wheel.sh` runs `npm run build` → copies `apps/frontend/dist` into the package data location → `python -m build` to produce `engines/python/dist/archscope-*.whl`. README quick-start replaced with `pip install archscope && archscope serve`. `.gitignore` excludes the static drop-zone contents while keeping the `.gitkeep` placeholder tracked. | T-207 | User feature request | One-command web install |
| T-209 | P1 | [ ] | Cross-browser smoke. Manually exercise the served web UI in latest Chrome, Edge, Firefox, and Safari: file selection (T-206 model), drag-drop, ECharts rendering, WebSocket progress streaming, the i18n switcher, and at least one analyzer end-to-end. Document the support matrix and any per-browser caveats in README plus `docs/en/USER_GUIDE.md` / `docs/ko/USER_GUIDE.md`. | T-208 | User feature request | Browser support matrix |
| T-210 | P1 | [ ] | Layout redesign — file-selection top-fixed strip, tabbed analyzer content. Replace the current side-by-side `workspace-grid` with a top-fixed file-selection strip and a tabbed content area below it; minimize vertical scroll. Sweep all analyzer pages (`AccessLogAnalyzerPage`, `GcLogAnalyzerPage`, `ProfilerAnalyzerPage`, `ExceptionAnalyzerPage`, `JfrAnalyzerPage`, `ThreadDumpAnalyzerPage`, `ChartStudioPage`, `DemoDataCenterPage`, `ExportCenterPage`, `SettingsPage`) onto the new layout in this single sweep — this absorbs the work that T-183 was originally tracking. | T-209 | User feature request | New layout shell |
| T-211 | P1 | [ ] | Collapsible left sidebar. Add an expand/collapse toggle to the sidebar with an icon-only collapsed state, smooth transition, keyboard shortcut, and `localStorage` persistence. Preserve nav-item `data-testid` selectors so existing E2E coverage keeps working. | T-210 | User feature request | Collapsible sidebar |
| T-212 | P1 | [ ] | Adopt a modern design system: Tailwind CSS + shadcn/ui (or an equivalent headless component library if the spike surfaces a blocker). Migrate the existing custom CSS classes (`primary-button`, `secondary-button`, `tool-panel`, `panel-header`, `metric-card`, `chart-grid`, `placeholder-list`, etc.) onto the new system in one sweep. Update `eslint.config.js`, Vite/PostCSS config, and ECharts theme to align. | T-211 | User feature request | Design-system migration |
| T-213 | P1 | [ ] | Slim global topbar consolidating search (analyzer pages, recent files), settings entry, and the locale switcher. Replaces the legacy `header.topbar` (T-180 superseded) and keeps the locale switcher out of the sidebar (T-181/T-211 superseded). Sidebar retains brand mark + nav only. | T-212 | User feature request | Slim topbar |
| T-214 | P2 | [x] | Made `D3TimelineChart` zoom/pan/brush-aware via opt-in `interactive` and `onSelectionChange` props (uses `d3-zoom` for wheel/drag and `d3-brushX` for the bottom selection band, with a Reset-zoom action). Hover popup now also surfaces a per-event `payload` (cause, before/after/committed heap, pause duration) for nearby `TimelineEvent` markers. `GcLogAnalyzerPage` enables the interactive flag on the GC pause timeline, sends Full-GC events with their full row payload, and renders a 4-stat selection-summary card (count / avg / p95 / max pause) the moment the user brushes a window. Dark-mode handled via the existing `useTheme` hook. | T-115, T-173, T-212 | User feature request | Interactive GC timeline |
| T-215 | P2 | [x] | New "Algorithm comparison" tab on `GcLogAnalyzerPage` aggregates pause statistics per `gc_type` from the events table (count / avg / p95 / max / total ms) and renders the rank both as a sortable shadcn table and as two horizontal `D3BarChart`s (avg + max ms) for side-by-side collector comparison. Fully frontend — no engine schema change. | T-214 | User feature request | GC algorithm comparison |
| T-216 | P1 | [x] | New `analyzers/thread_dump_to_collapsed.py` (`convert_thread_dumps_to_collapsed` + `write_collapsed_file`) drives the parser registry, applies all per-language enrichment (CGLIB/proxy cleanup from T-194 included), reverses the stack so the root sits left, prepends the (sanitized) thread name as a synthetic root frame, and aggregates identical stacks across files. CLI command `archscope-engine thread-dump to-collapsed --input <file> --input <file> ... --output <out> [--format <id>] [--no-thread-name]`. FastAPI dispatch adds `type:"thread_dump_to_collapsed"` to `/api/analyzer/execute` (writes the collapsed output under `~/.archscope/uploads/collapsed/` and returns `{ outputPath, uniqueStacks, inputCount }`). | T-194 | aitop-tools | Batch dump→collapsed CLI + web |
| T-217 | P2 | [x] | New `CanvasFlameGraph.tsx` paints the same `FlameGraphNode` tree onto a Canvas 2D context (HiDPI-aware via `devicePixelRatio`, click-to-zoom, hover tooltip, "Save PNG" via `canvas.toDataURL()`). `ProfilerAnalyzerPage` counts the rendered nodes and auto-switches: `< 4000` keeps the crisp SVG `D3FlameGraph`, `≥ 4000` swaps in `CanvasFlameGraph` so the converted output from T-216 (typically tens of thousands of frames) stays interactive. Both components share the same `D3ChartFrame` wrapper (image-export dropdown still works for SVG/PNG/JPEG presets). | T-216, T-187, T-212 | User feature request | High-volume flamegraph rendering |
| T-218 | P1 | [x] | Phase 7 — extended `models/thread_snapshot.py` with frozen `LockHandle(lock_id, lock_class)` and added `ThreadSnapshot.lock_holds: list[LockHandle]` + `lock_waiting: LockHandle \| None`. Existing `lock_info: str \| None` kept for backward compatibility. `ThreadSnapshot.to_dict()` serializes the new fields. Non-Java parsers leave them empty (their dump formats do not expose lock IDs). | T-188 | User feature request | Lock-aware thread record contract |
| T-219 | P1 | [x] | `parsers/thread_dump/java_jstack.py` now extracts lock IDs in `_extract_lock_handles`. Patterns: `- locked <0x…> (a Foo.Bar)` → append to `lock_holds`; `- waiting to lock <0x…>`, `- waiting on <0x…>`, `- parking to wait for <0x…> (a …)` → set `lock_waiting`. Multiple holds per thread supported; re-entrant locks (same ID held + waiting on the same thread) are recorded as held only. RUNNABLE/UNKNOWN threads with a non-empty `lock_waiting` are promoted to `LOCK_WAIT`. The legacy `lock_info` string keeps its raw first-match line. | T-218 | User feature request | Java lock ID extraction |
| T-220 | P1 | [x] | New `analyzers/lock_contention_analyzer.analyze_lock_contention()` accepts a single `ThreadDumpBundle` or a sequence. Builds an owner ↔ waiter map keyed by `lock_id` (deduped, owner picked by frequency for multi-bundle unions), ranks `LOCK_CONTENTION_HOTSPOT` findings by waiter count (owner thread + stack signature surfaced), and runs a DFS cycle detector that emits one `DEADLOCK_DETECTED` per simple cycle (canonical-rotation dedup). Output `AnalysisResult(type="thread_dump_locks")` with per-lock rows, contention ranking, deadlock chains, dump table, and `summary` (contended_locks / deadlocks_detected / threads_with_locks / threads_waiting_on_lock / unique_locks). | T-218, T-219, T-191 | User feature request | Lock owner/waiter graph + deadlock detector |
| T-221 | P2 | [x] | `multi_thread_analyzer._build_growing_lock_findings` walks each `lock_id`'s waiter count across the dump-index timeline and fires `GROWING_LOCK_CONTENTION` (warning) when the count strictly increases in ≥ `consecutive_dump_threshold` consecutive dumps (max waiters reported as evidence). New `summary.growing_lock_contention` and `tables.growing_lock_contention` fields. | T-218, T-219, T-191 | User feature request | Multi-dump lock contention growth |
| T-222 | P1 | [x] | CLI: `archscope-engine thread-dump analyze-locks --input <f> [--input <f> ...] --out <result.json> [--format <id>] [--top-n N]` — uses the parser registry like the other multi-input commands and prints a one-line `(N contended locks, M deadlocks)` summary on success. FastAPI: `/api/analyzer/execute` now dispatches `type:"thread_dump_locks"` (`filePaths`, `format`, `topN`); errors map to `UNKNOWN_THREAD_DUMP_FORMAT` / `MIXED_THREAD_DUMP_FORMATS` / `ENGINE_FAILED` exactly like the existing thread-dump endpoints. | T-220, T-221 | User feature request | Lock-contention CLI + web |
| T-223 | P2 | [x] | New "Lock Contention" tab on `ThreadDumpAnalyzerPage` (auto-fetched in parallel with `thread_dump_multi` after **Correlate dumps**). Per-lock shadcn table (`lock_id`, `lock_class`, owner, waiter count, top waiter names). Horizontal `D3BarChart` ranking top contended locks. Dedicated red severity card for each `DEADLOCK_DETECTED` cycle rendered as `T1 → T2 → T3 → T1` plus per-edge lock evidence. `analyzerContract.ts` extended with `AnalyzeLockContentionRequest`; en/ko strings added. | T-222 | User feature request | Lock contention UI |

### TDA-main JVM Thread Dump Hardening Follow-up

Goal: port the useful JVM thread-dump algorithms discovered in `C:\workspace\tda-main` into ArchScope without importing Java sources directly. Keep the existing parser/analyzer/UI separation and the language-agnostic thread-dump framework; only add JVM-specific behavior behind the Java parser plugin or Java-only analyzers.

Scope exclusions:

- Do not port TDA Swing UI, FlatLaf, VisualVM plugin, JConsole plugin, macOS packaging, or TDA's MCP server.
- Do not add HPROF heap-dump analysis in this cycle. Text class histograms from `jcmd`/`PrintClassHistogram` are allowed because they are lightweight log records, not heap-dump parsing.
- Do not replace ArchScope's multi-dump correlation algorithm with TDA's first-dump exact-content comparison; ArchScope's timeline/run-length approach remains canonical.

Recommended implementation ownership:

- Backend/parser owner: T-224 through T-226. These touch file sniffing, encoding, parser registry, and Java parser plugin boundaries.
- JVM diagnostics owner: T-227 through T-231. These require JVM-specific semantics for virtual threads, SMR, native methods, and monitor states.
- Test/docs owner: T-232 and T-233 after the parser/diagnostic contracts settle.
- Frontend owner only after backend schemas are stable; most items should first land as CLI/FastAPI result data, then UI tables/cards can be added in a small follow-up.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-224 | P0 | [x] | Add single-file JVM multi-dump splitting. A single large log containing multiple `Full thread dump` sections should produce ordered `ThreadDumpBundle` entries with `dump_index`, `dump_label`, source file, start line, and best-effort timestamp. The existing multi-dump analyzer should accept these extracted bundles the same way it accepts multiple files. Include tests with representative Java 8/11/21 multi-dump logs. | T-188, T-189, T-190, T-191 | `tda-main` `SunJDKParser` / `AbstractDumpParser` behavior | One log file -> ordered dump timeline |
| T-225 | P0 | [x] | Add UTF-16 BOM/null-byte text detection for JVM thread dumps and registry sniffing. Extend common text encoding detection and `_read_head()` so UTF-16LE/BE thread dumps can be detected before parser selection, while preserving existing `utf-8`, `utf-8-sig`, `cp949`, and `latin-1` fallback behavior. | T-125, T-170, T-189, T-224 preferred | `tda-main` `DumpParserFactory.detectCharset()` | UTF-16 Java dump support plus tests |
| T-226 | P1 | [x] | Add a Java `jcmd Thread.dump_to_file -format=json` parser plugin. Detect JSON objects containing `threadDump`, parse `time`, `threadContainers[].threads[]`, `name`, `tid`, stack frames, and any available state/native metadata. Emit clear diagnostics for missing `threadDump`, malformed JSON, and unsupported JSON variants. | T-188, T-189 | `tda-main` `JCmdJSONParser` | `java_jcmd_json` parser plugin |
| T-227 | P1 | [x] | Add JVM virtual-thread and carrier-thread pinning enrichment. Detect virtual thread headers, `VirtualThread[#...]`, `Carrying virtual thread`, carrier thread relationships, and carrier stacks stuck in application code. Emit a deterministic `VIRTUAL_THREAD_CARRIER_PINNING` finding/table with thread name, carrier evidence, top application frame, and dump index. | T-190, T-224, T-226 preferred | `tda-main` `SunJDKParser` / `Analyzer` | Java virtual-thread diagnostics |
| T-228 | P1 | [x] | Add SMR info parsing and unresolved/zombie thread diagnostics. Parse `Threads class SMR info`, map SMR addresses back to parsed Java thread `tid` values where possible, and emit `SMR_UNRESOLVED_THREAD` evidence for addresses that cannot be resolved. Preserve raw SMR evidence in bounded metadata. | T-190, T-224, T-225 | `tda-main` `ThreadDumpInfo` SMR resolution | SMR table and unresolved-thread findings |
| T-229 | P1 | [x] | Add Native Method thread extraction. Build a `native_method_threads` table for Java threads whose stack contains `Native Method`, including thread name, native method text, top Java frame, state, dump index, and source file. Use this table as supporting evidence for network/file wait findings without overriding existing state semantics. | T-190, T-224 | `tda-main` MCP `get_native_threads()` | Native-method evidence table |
| T-230 | P2 | [x] | Add JVM text class-histogram parser/analyzer for `num   #instances    #bytes  class name` blocks. Extract class name, instance count, bytes, totals, configurable row limit, truncation metadata, and top classes by bytes/instances. Keep HPROF heap dumps out of scope. | T-224, T-225 preferred | `tda-main` `parseNextClassHistogram()` | Lightweight class-histogram result |
| T-231 | P2 | [x] | Refine Java monitor semantics beyond generic lock IDs. Preserve existing `lock_holds` / `lock_waiting`, but classify wait mode as `lock_entry_wait`, `object_wait`, `parking_condition_wait`, or `locked_owner` based on `waiting to lock`, `waiting on`, `parking to wait`, and `locked` lines. Avoid treating pure `Object.wait()` sleep as lock contention when there is no owner evidence. | T-218, T-219, T-220, T-224 | `tda-main` `MonitorMap` | More accurate Java monitor/wait classification |
| T-232 | P2 | [x] | Add fixture-backed regression coverage from minimized TDA samples. Cover Java 8/11/21 text dumps, UTF-16 dumps, jcmd JSON dumps, carrier pinning, SMR unresolved addresses, native method rows, multi-dump log splitting, monitor semantics, and class histogram parsing. Do not vendor the full TDA project; copy or synthesize minimal fixtures only. | T-224 through T-231 | `tda-main` test resources | JVM hardening regression suite |
| T-233 | P2 | [x] | Update English/Korean docs for the JVM hardening scope: single-file multi-dump behavior, UTF-16 support, jcmd JSON support, virtual-thread/carrier findings, SMR caveats, Native Method evidence, monitor semantic labels, and the text-histogram vs HPROF boundary. | T-224 through T-232 | User follow-up | Updated parser/user/data-model docs |
| T-234 | P2 | [ ] | Improve SMR address-to-thread resolution with real-world JDK 8/11/17/21 fixtures. Current implementation preserves bounded unresolved/zombie raw evidence and emits findings; exact JavaThread address to parsed `nid`/`tid` matching remains heuristic and should be validated against more dump variants. | T-228 | Implementation follow-up | Higher-confidence SMR correlation |
| T-235 | P3 | [ ] | Add incomplete text class-histogram diagnostics for truncated histogram blocks. Current parser extracts bounded class rows and totals when present, but does not yet emit a dedicated incomplete-block warning. | T-230 | Implementation follow-up | Better histogram parser diagnostics |
| T-236 | P2 | [ ] | Add UI/HTTP payload controls for very large virtual-thread dumps. Backend synthetic benchmark handled 10k virtual threads in ~0.7s and 50k in ~4.1s parse+analyze, but frontend tables should still use pagination/virtualization and avoid requesting full snapshot/raw evidence payloads by default. | T-224, T-227 | User scalability follow-up | Safer large virtual-thread UX |

### Profiler Batch Timeline Follow-up

Goal: make profiler timeline analysis usable for batch-job scoped investigations where whole-profile sample ratios include idle threads, scheduler loops, framework launchers, or other business workloads.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-237 | P1 | [x] | Implemented a profiler timeline base-method option. `timelineBaseMethod` / `--timeline-base-method` matches stacks containing the configured method, re-roots each matched stack at the base frame, uses matched samples as the 100% denominator, preserves whole-profile context through `metadata.timeline_scope.base_ratio_of_total` and per-row `total_ratio`, and emits `TIMELINE_BASE_METHOD_NOT_FOUND` when no stack matches. Wired through CLI, FastAPI, TypeScript request/result contracts, and the Profiler timeline UI. | T-104, T-186, T-187 | User feature request | Batch-job scoped profiler timeline |

### Go/Native Profiler Follow-up

Goal: validate a lightweight native desktop direction for the profiler-first product slice. Keep the current Python/React implementation intact while a Go profiler core and a native GUI (framework choice tracked under the Active Decision Queue: Wails v3 vs Fyne) prove size, speed, and feature parity.

Binary size context (single distributable, post-Python-removal): Electron ~120 MB → Wails v3 ~12–20 MB / Fyne ~25–40 MB. Wails reuses the existing `apps/frontend` React UI through a system webview (WKWebView/WebView2/WebKitGTK); Fyne rebuilds the UI on its own OpenGL toolkit.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-238 | P0 | [x] | Created `apps/profiler-native` as the Go-native profiler app workspace and implemented the dependency-light Go CLI/core POC for collapsed profiler analysis, including flamegraph expansion, execution breakdown, parser diagnostics, and timeline base-method scope output for comparison against Python before adding native UI widgets. | T-237 | User feature request | Go profiler POC workspace |
| T-239 | P1 | [~] | **Decomposed into T-239a–T-239g below.** Original intent: port profiler feature parity to Go. Tracked-as-superseded so PR history stays clean. | T-238 | User feature request | Decomposed |
| T-239a | P1 | [x] | Jennifer APM flamegraph CSV parser in Go (`internal/profiler/jennifer.go`) — column aliases, encoding fallbacks (utf-8-sig / utf-8 / cp949 via `golang.org/x/text/encoding/korean`), required-column validation, virtual-root for multiple roots, parent-cycle detection, missing-parent warning, child-samples-exceed-parent warning, ratio percent suffix. `AnalyzeJenniferFile` reuses the collapsed analyzer's flame tree / breakdown / timeline pipeline and emits `type:"profiler_jennifer"`. CLI flag `--jennifer-csv` (mutually exclusive with `--collapsed`). 12 regression tests covering virtual root, duplicate key, missing parent, parent cycle, missing required columns, cp949 fallback, BOM, ratio %, empty file, column alias, child-samples-exceed-parent, end-to-end `AnalyzeJenniferFile`. | T-238 | User feature request | Go Jennifer parser + analyzer + tests |
| T-239b | P1 | [x] | Ported `engines/python/archscope_engine/parsers/svg_flamegraph_parser.py` to `apps/profiler-native/internal/profiler/svg.go`. Streams `encoding/xml` (XXE-safe by default — no external entity expansion), reconstructs collapsed stacks from `<g><title>name (N samples)</title><rect/></g>` pairs, auto-detects Brendan default vs icicle by widest-rect row, emits `INVALID_SVG`/`UNPARSEABLE_TITLE`/`INVALID_RECT_GEOMETRY`/`INVALID_SAMPLE_COUNT` diagnostics. `AnalyzeFlamegraphSVGFile` reuses the collapsed analyzer pipeline; CLI flag `--flamegraph-svg`; ProfilerService accepts `format:"flamegraph_svg"`; OS file-picker filter added. 7 regression tests (Brendan, icicle, comma-grouped samples, missing sample marker, malformed SVG, disk read, AnalyzeFlamegraphSVGFile end-to-end) all pass. | T-238 | User feature request | Go SVG flamegraph → collapsed stacks |
| T-239c | P1 | [x] | Ported `engines/python/archscope_engine/parsers/html_profiler_parser.py` to `apps/profiler-native/internal/profiler/html.go`. Two flavours: inline-SVG HTML (delegates to the SVG parser via a regex-located `<svg>...</svg>` block) and async-profiler self-contained HTML (regex-located `var/let/const root\|levels\|profileTree\|flame = {...}`, JS-shape keys auto-quoted before `json.Unmarshal`, walks the tree honoring `name/title/frame` and `samples/value/v` and `children/c`, preserves intermediate self-time when parent samples > sum of children). `UNSUPPORTED_HTML_FORMAT`/`INVALID_ASYNC_PROFILER_JSON` diagnostics. `AnalyzeFlamegraphHTMLFile` reuses the collapsed analyzer pipeline; CLI `--flamegraph-html`; ProfilerService format `flamegraph_html`; OS dialog filter. 6 regression tests (inline SVG, async-profiler JS, JS self-time accounting, unsupported HTML, disk read, AnalyzeFlamegraphHTMLFile end-to-end). | T-239b | User feature request | Go HTML flamegraph parser |
| T-239d | P1 | [x] | Ported `profiler_drilldown.py` to `apps/profiler-native/internal/profiler/drilldown.go`. `DrilldownFilter`, `CreateRootStage`, `ApplyDrilldownFilter`, `BuildDrilldownStages`. Filter types: include_text / exclude_text / regex_include / regex_exclude. Match modes: anywhere / ordered (`a > b > c`) / subtree. View modes: preserve_full_path / reroot_at_match. ReDoS guard via 500-char length cap (Go's RE2 already prevents catastrophic backtracking; nested-quantifier sniffer kept as defense-in-depth). `INVALID_REGEX` and `UNSAFE_REGEX` diagnostics. `ProfilerService.Drilldown(req)` exposes it to the renderer; `DrilldownPanel.tsx` adds filter form + breadcrumb + filter chips + stage metrics + per-stage Canvas flamegraph. 11 regression tests pass. | T-239a | User feature request | Go drill-down engine + native UI |
| T-239e | P1 | [x] | Ported `engines/python/archscope_engine/analyzers/profiler_diff.py` to `apps/profiler-native/internal/profiler/diff.go`. `BuildDiffFlameTree` (baseline + target stack maps → FlameNode tree with `metadata = {a, b, delta}` per node + summary {baseline_total, target_total, common_paths, added_paths, removed_paths, max_increase, max_decrease, total_unit, tree_total}); optional totals normalisation so a 10× longer target doesn't drown the baseline; sample width = max(a, b) so the visual reads truthfully and the divergent palette comes from the metadata. `AnalyzeProfilerDiff` returns a `profiler_diff` AnalysisResult with `metadata.diff_summary` / `metadata.diff_tables` so the renderer can colour-code regressions/improvements. `ProfilerService.Diff(req)` with collapsed/Jennifer/SVG/HTML inputs on either side. `pages/DiffPage.tsx` adds two file pickers, normalize toggle, summary card, divergent flamegraph (red regression / green improvement / gray unchanged), top-10 increase + decrease tables. 4 regression tests pass. | T-239d | User feature request | Go profiler diff + native UI |
| T-239f | P2 | [x] | Added `apps/profiler-native/internal/profiler/pprof.go` using the official `github.com/google/pprof/profile` package. `ExportToPprof(stacks, outPath, sampleType, unit, intervalMs)` builds a `profile.Profile` with one Function + Location per frame (ID-keyed dedupe), reverses collapsed paths to leaf-first as pprof expects, and emits two SampleTypes (`samples/count` plus `duration/milliseconds` when an interval is supplied) with `Period = intervalMs`. Output is gzip-compressed `.pb.gz` ready for `go tool pprof`. `ProfilerService.ExportPprof(req)` + `PickPprofOutput()` (Wails Save dialog filtered to `*.pb.gz`). Tabs strip exposes "Export as pprof" with spinner + saved-path notice. 2 regression tests (round-trip parse via `profile.Parse`, empty-stack tolerance). | T-239a | User feature request | Native Go pprof exporter |
| T-239g | P2 | [x] | Ported Python's `DebugLogCollector` JSON shape to Go: restructured `DebugLog.payload()` to emit `environment`/`context`/`redaction`/`summary`/`errors_by_type`/`exceptions`/`hints` matching the Python `to_dict()` contract; added `InferFieldShapes` (token count, quote/bracket count, request shape, timestamp shape, suffix key=value); wired `*DebugLog` into `ParseSvgFlamegraphFile`, `ParseHtmlProfilerText/File`, `ParseJenniferFlamegraphCSV` and their internal error sites (UNPARSEABLE_TITLE, INVALID_SAMPLE_COUNT, INVALID_RECT_GEOMETRY, UNSUPPORTED_HTML_FORMAT, INVALID_ASYNC_PROFILER_JSON, ENCODING_ERROR, MISSING_REQUIRED_COLUMNS, INVALID_JENNIFER_ROW, DUPLICATE_KEY); added `DebugLog`/`DebugLogDir` to `Options` so `Analyze*File` auto-writes on parse errors; added `--debug-log`/`--debug-log-dir` CLI flags; verdict policy aligned with Python (CLEAN/PARTIAL_SUCCESS/MAJORITY_FAILED/FATAL_ERROR); 12 new regression tests covering JSON shape parity, field shapes, verdict logic, and per-parser debug log population. Also fixed CI parity job CLI mismatch (`profiler analyze --collapsed --interval-ms` → `profiler analyze-collapsed --wall --wall-interval-ms`). | T-239b, T-239c | User feature request | Go portable parser debug logs |
| T-240 | P1 | [~] | **Decomposed into T-240a–T-240f below.** Original intent: build the native profiler desktop UI over the Go core. Tracked-as-superseded. The actual GUI framework (Wails v3 vs Fyne) is decided in the Active Decision Queue before T-240a starts. | T-239 preferred | User feature request | Decomposed |
| T-240a | P1 | [~] | Native UI shell first slice landed via Wails v3 alpha.84 in `apps/profiler-native/cmd/archscope-profiler-app/`: top-fixed file strip + option row + content area, OS file dialog through `ProfilerService.PickProfileFile`, `ProfilerService.Analyze` returning the existing `internal/profiler.AnalysisResult` contract, en/ko locale toggle, dark slate/indigo CSS baseline, Summary + diagnostics + top-5 stacks cards. Remaining: collapsible sidebar, slim global topbar, full Phase-2 layout parity, shadcn/Tailwind migration. **Binary size (darwin/arm64): 8.0 MB raw / 9.9 MB `.app` — well under the 12–20 MB Wails v3 target.** | T-239a, GUI decision (Wails v3) | User feature request | Native main shell first slice |
| T-240b | P1 | [x] | i18n strings extracted into `apps/profiler-native/cmd/archscope-profiler-app/frontend/src/i18n/messages.ts` with profiler-scoped keys mirroring the web app's `apps/frontend/src/i18n/messages.ts` vocabulary; lightweight `I18nProvider` context with `localStorage` persistence (`archscope.profiler.locale`); `App.tsx` consumes `useI18n()`. Sharing the catalog with the web app is deferred until the native UI grows beyond profiler-only. | T-240a | User feature request | Bilingual native UI |
| T-240c | P1 | [x] | Added `frontend/src/theme/ThemeProvider.tsx` mirroring the web app's `useTheme` semantics: `light` / `dark` / `system`, persisted under `archscope.profiler.theme` localStorage key, listens to `prefers-color-scheme` for `system`. CSS variables split into a default (light) palette and a `.dark` override for every token (`--bg`, `--bg-elev`, `--bg-soft`, `--border`, `--text`, accents, danger, tooltip, canvas stroke). Topbar three-way pill toggle. CanvasFlameGraph re-renders on theme change. | T-240a | User feature request | Native light/dark/system theme |
| T-240d | P1 | [x] | Ported T-217 `CanvasFlameGraph` from `apps/frontend/src/components/charts/CanvasFlameGraph.tsx` into `apps/profiler-native/cmd/archscope-profiler-app/frontend/src/components/CanvasFlameGraph.tsx`. Uses `d3-hierarchy` only (full `d3` not bundled), HiDPI-aware via `devicePixelRatio`, click-to-zoom with Reset, hover tooltip, "Save PNG" via `canvas.toDataURL`. Wired to `result.charts.flamegraph` via a `FlameNode → FlameGraphNode` adapter so the painter stays decoupled from the Wails-generated binding shape. **+9 KB to the Vite bundle, +16 KB to the Go binary** — total still 8.0 MB raw / 9.9 MB `.app`. SVG/auto-switch-threshold variant deferred to a later slice; current scope renders Canvas unconditionally. | T-239a, T-240a | User feature request | Native flamegraph rendering |
| T-240e | P1 | [~] | Added `frontend/src/components/HorizontalBarChart.tsx` — dependency-free SVG-style bar chart driven by row data, used to render `result.series.execution_breakdown` and `result.series.timeline_analysis` cards under the flamegraph. Auto-derives ratios from sample counts when `total_ratio` is missing, color cycle from a 10-color palette, theme-token aware (`--bg-soft`/`--border-soft`). PNG export not yet wired (Canvas flamegraph already covers the largest export need); SVG-to-PNG export and a separate timeline-strip view are deferred to a follow-up. | T-240a | User feature request | Native breakdown + timeline charts |
| T-240f | P2 | [x] | First slice (5-tab layout + spinner + lockable options + recovered hidden info) lands plus the real cancel path: `ProfilerService.AnalyzeAsync` returns a `taskId` immediately and emits typed Wails events (`analyze:done` / `analyze:error` / `analyze:cancelled`) keyed by that taskId; `ProfilerService.Cancel(taskId)` flips a registered cancel channel so the goroutine emits `analyze:cancelled` instead of `analyze:done`. Renderer subscribes via `Events.On` and exposes a working **Cancel** button on the spinner overlay. The goroutine itself runs to completion (the analyzer is CPU-bound and finishes in ms) — the cancel signal causes the renderer to discard the result, which matches user intent. | T-240a | User feature request | Native UX feedback complete |
| T-241 | P2 | [x] | Added `internal/profiler/golden_test.go` (4 invariants pinned against the shared fixtures: collapsed total 32629, Jennifer total 115, SVG/HTML cross-input parity) and `internal/profiler/bench_test.go` (Apple M5 Max baseline: AnalyzeCollapsed 35 µs, AnalyzeJennifer 42 µs, ParseSvg 14 µs, ParseHtml 11 µs, BuildDiffFlameTree 6 µs, ExportToPprof 90 µs). The actual cross-engine Python ↔ Go comparison runs in CI (T-244) — the GHA matrix executes both pipelines on the same fixture and asserts type/total_samples/parser/top_stack agreement. | T-239a–T-239g, T-240a–T-240f | User feature request | Go-vs-Python profiler validation |
| T-242 | P2 | [x] | **Decision (2026-05-06): keep `apps/profiler-native/` inside the monorepo for the alpha cycle.** Rationale — the Go core under `internal/profiler/` shares fixtures with the Python engine for the parity job (T-244), the `apps/frontend` React UI is the source of truth for design tokens / i18n / Canvas flamegraph (T-217 → T-240d), and the GHA matrix gates both implementations on the same PR. Revisit if (a) Wails v3 leaves alpha and the Python ↔ Go parity pipeline can be retired, or (b) the profiler ships separately from the wider ArchScope product. | T-241 | User feature request | Product/repo boundary decision |
| T-243 | P1 | [~] | Added `package:darwin` / `package:windows` / `package:linux` / `package:all` shortcuts to `apps/profiler-native/cmd/archscope-profiler-app/Taskfile.yml` and shipped `PACKAGING.md` with per-target build commands, webview prerequisites (WebView2 Evergreen on Win10, libwebkit2gtk-4.1 on Linux), cross-compile model (docker for non-host targets, arch-specific macOS hosts in CI for darwin/{arm64,amd64}), build-size targets vs measured (8.4 MB darwin/arm64 raw, 10 MB .app), and signing posture (ad-hoc on macOS for now; Developer ID + notarytool deferred). Real Win/Linux size measurements populate from the GHA matrix in T-244. | T-240a | User feature request | Distributable installers |
| T-244 | P1 | [x] | Added `.github/workflows/profiler-native.yml` with three jobs: (1) `go-test` matrix `[ubuntu-latest, macos-14, windows-latest]` running `go vet`, `go build`, `go test ./internal/... -race -count=1`, and a 1×iteration bench smoke; (2) `python-go-parity` matrix `[ubuntu-latest, macos-14]` building the Go CLI (`go build -o bin/archscope-profiler ./cmd/archscope-profiler`), installing the Python engine via `pip install -e ".[dev]"`, running both pipelines on `examples/profiler/sample-wall.collapsed` (Go: `--collapsed ... --interval-ms 100 --elapsed-sec 1336.559 --top-n 20 --out`; Python: `archscope_engine.cli profiler analyze-collapsed --wall ... --wall-interval-ms 100 --elapsed-sec 1336.559 --top-n 20 --out`) and asserting `type` / `summary.total_samples` / `summary.profile_kind` / `metadata.parser` / `series.top_stacks[0].{stack,samples}` agree (drift raises `SystemExit` so the job fails); (3) `package` matrix `[macos-14, macos-13, windows-latest, ubuntu-latest]` installing `wails3@v3.0.0-alpha.84`, the Task runner (`taskfile.dev` install script on macOS/Linux, `choco install go-task` on Windows), `libgtk-3-dev` + `libwebkit2gtk-4.1-dev` on Linux, then running `task package` per target and uploading `bin/*` as a versioned GHA artifact (`archscope-profiler-darwin-arm64` / `darwin-amd64` / `windows-amd64` / `linux-amd64`). Workflow is path-filtered to `apps/profiler-native/**`, `examples/profiler/**`, and the workflow file itself. | T-241, T-243 | User feature request | Native profiler CI gate |
| T-245 | P2 | [x] | Added `docs/en/PROFILER_NATIVE.md` and `docs/ko/PROFILER_NATIVE.md` covering install (developer mode + CI artifacts), system prerequisites (WKWebView / WebView2 Evergreen / WebKitGTK 4.1), the 5 input formats, drill-down / diff / pprof / debug log surfaces, layout (sidebar + tabs), Settings persistence keys, the `cmd/archscope-profiler` CLI, troubleshooting (executable-missing, WebView2, libwebkit2gtk, drag-drop fallback, macOS quarantine), and cross-references to PACKAGING.md / CI workflow / Python parity reference / roadmap. | T-243 | User feature request | Bilingual native docs |
| T-246 | P2 | [x] | Added `frontend/src/hooks/useShortcuts.ts` with global keydown router: Cmd/Ctrl+O (open file), Cmd/Ctrl+R (re-analyze), Esc (cancel in-flight), Cmd/Ctrl+, (Settings), Cmd/Ctrl+1..5 (jump to Profiler tab). The hook ignores typing inside text inputs except for OS-conventional Cmd shortcuts. ARIA already in place from T-240a (`role="tablist"` + `role="tab"` + `aria-selected` on Tabs.tsx, `aria-pressed` on locale/theme pills, `aria-label` on Sidebar nav). `shortcutLabels` export gives platform-aware UI strings if a help overlay is added later. | T-240a | User feature request | Native a11y baseline |
| T-247 | P3 | [x] | **Decision (2026-05-06): manual download notice for the alpha cycle, no auto-updater.** Rationale — Wails v3 is alpha so the wire/UI surface still moves, signing certificates (Apple Developer ID, Windows code-signing) are not yet provisioned, and the user base is internal/evaluation-only. CI uploads per-arch artifacts on every push (`archscope-profiler-darwin-arm64` / `darwin-amd64` / `windows-amd64` / `linux-amd64`) which serves as the manual update channel. Revisit when (a) Wails v3 hits stable, (b) Developer ID + Windows code-signing certs land, and (c) external users start tracking versions. Candidate runtimes when revisited: Sparkle (mac), Squirrel.Windows or WinSparkle (win), AppImageUpdate (linux). | T-243 | User feature request | Update strategy decision |

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
16. `T-073 -> T-074 -> T-075`: clarify the advanced-diagnostics thesis, harden correlation contracts, then validate the JFR command bridge.
17. `T-076 -> T-077 -> T-078`: document JFR parser tradeoffs, update event modeling, then define bounded advanced-input processing.
18. `T-079 -> T-080 -> T-081`: close OTel evidence policy, schema compatibility warnings, and timeline visualization design before Phase 5 AI work.
19. `T-082 -> T-083 -> T-084`: define evidence references, AI contracts, and the validator before any LLM call path.
20. `T-084 -> T-085 -> T-087 -> T-088`: enforce evidence validity, defend prompt construction, then add local LLM runtime controls.
21. `T-083 -> T-086` and `T-084 -> T-091`: make AI provenance visible and define low-confidence/invalid-output behavior.
22. `T-084 -> T-090`: build AI evaluation only after validator semantics are testable.
23. `T-079 -> T-092`: extend OTel privacy policy into LLM prompt and logging safeguards.
24. `T-083/T-084/T-087/T-088 -> T-163 -> T-164`: add the executable local LLM client only after contracts, validation, prompt construction, and runtime policy are in place.
25. `T-087/T-090/T-163 -> T-165 -> T-166`: externalize model/language prompt templates after the first executable path and evaluation signals exist.
26. `T-093 -> T-094` and `T-093 -> T-095 -> T-097`: define the common flame tree contract, then normalize collapsed-stack and Jennifer CSV inputs into it.
27. `T-093 -> T-098 -> T-099 -> T-100 -> T-101 -> T-102 -> T-103`: build drill-down engine semantics before exposing the CLI.
28. `T-093 -> T-104 -> T-105 -> T-106 -> T-107 -> T-108 -> T-109`: build execution classification and stage-aware aggregation before the breakdown CLI.
29. `T-098 -> T-110 -> T-111`: wire UI drill-down first, then add stage-aware breakdown charts.
30. `T-094/T-095/T-098/T-104 -> T-112/T-113 -> T-114`: lock behavior with tests before final documentation updates.
31. `T-115/T-116/T-117 -> T-118 -> T-119 -> T-120 -> T-121`: implement JVM parsers/analyzers, expose them through CLI/UI, then lock behavior with samples, tests, and docs.
32. `T-122 -> T-123 -> T-124 -> T-125 -> T-126 -> T-127 -> T-128 -> T-129 -> T-130 -> T-131 -> T-132 -> T-133`: build portable redacted parser debug logs, integrate parsers/CLI/Electron, then lock behavior with tests and docs.
33. `T-134 -> T-135 -> T-136` and `T-137`: add the first report automation and Chart Studio slices after result/debug contracts are stable.
34. `T-138/T-139/T-140/T-141 -> T-142 -> T-143 -> T-144`: add multi-runtime parser/analyzer MVPs, sample artifacts, tests, and docs.
35. `T-145/T-146/T-147 -> T-150`: close the next report automation follow-ups with diff, PPTX, and static flamegraph HTML coverage.
36. `T-148 -> T-149 -> T-150`: add the first OTel analyzer path, fixture coverage, and docs before deeper trace/span correlation work.
37. `T-136/T-145/T-146 -> T-151 -> T-152`: expose the report automation CLI paths through the desktop Export Center.
38. `T-153 -> T-154 -> T-155 -> T-156 -> T-157`: run shared demo-site manifests through CLI/UI and keep command mapping single-sourced in `projects-assets`.
39. `T-148 -> T-158 -> T-159`: move OTel from lightweight grouping to parent-span service paths, failure propagation, and manifest expectation checks.
40. `T-156 -> T-160 -> T-162`: support larger demo runs now, then add automated Electron smoke coverage after the harness exists.
41. `T-167 -> T-170/T-173/T-174`: fix percentile correctness before further parser/analyzer performance work.
42. `T-168 -> T-177`: preserve result immutability before adding cache-oriented profiler optimizations.
43. `T-169 -> T-178`: establish local benchmark output before adding CI performance gates.
44. `T-098 -> T-171` and `T-158 -> T-172/T-176`: harden user regex and OTel topology/error classification edge cases.
45. `T-038/T-070 -> T-179`: add analyzer cancellation after the generic IPC bridge and sidecar cleanup path are stable.
**Phase 1 — Web platform migration (must finish before any UI rebuild):**

46. `T-206 -> T-207 -> T-208 -> T-209`: design the web pivot → remove Electron and serve via FastAPI → ship `pip install archscope && archscope serve` → cross-browser smoke. Phase 1 lands first.

**Phase 2 — UI redesign on the new web shell:**

47. `T-209 -> T-210 -> T-211 -> T-212 -> T-213`: layout redesign → collapsible sidebar → Tailwind/shadcn design system → slim global topbar. Analyzer pages migrate **once** here.

**Phase 3 — Electron-era cleanups absorbed by Phase 2 (do NOT implement separately):**

48. `{T-210, T-211, T-212, T-213}` supersede `{T-180, T-181, T-182, T-183}`: the original Electron-era UI cleanups are intentionally folded into the Phase 2 redesign. T-180–T-183 stay in the backlog as superseded references only.

**Phase 4 — Profiler SVG/HTML (backend parallel with Phases 1-2; UI gated on Phase 2):**

49. `T-184 -> T-185 -> T-186`: SVG flame parser → HTML detector reusing it → analyzer/CLI integration. Backend-only, parallel-safe.
50. `T-186 -> T-212 -> T-187`: profiler SVG/HTML UI exposure waits for the Phase 2 design system so it ships once.

**Phase 5 — Multi-language thread-dump framework (backend parallel; UI gated on Phase 2):**

51. `T-188 -> T-189 -> T-190 -> T-191 -> T-192`: language-agnostic record contract → parser plugin interface and auto-detection → Java reference plugin → common correlation engine → CLI. Backend-only, parallel-safe.
52. `T-192 -> T-212 -> T-193`: multi-dump UI lands on the Phase 2 shell.
53. `T-190 -> {T-194, T-195}`: Java reference plugin must exist before the JVM-only AOP cleanup and JVM-only network categorization plugins attach to it.
54. `T-189 -> {T-196, T-198, T-200, T-202}`: other-language parser plugins (Go, Python, Node.js, .NET) can be built in parallel once the registry/auto-detection contract exists.
55. `T-196 -> T-197`, `T-198 -> T-199`, `T-200 -> T-201`: each language's parser lands before its language-tagged framework-cleanup and state-inference enrichment.
56. `{T-195, T-197, T-199, T-201} -> T-203`: cross-language latency finding only after each language has populated its `category` enrichment.
57. `{T-187, T-193, T-194, T-197, T-199, T-201, T-202, T-203} -> T-204 -> T-205`: lock new profiler and multi-language thread-dump behavior with tests before bilingual docs.

**Phase 6 — Diagnostics polish (after Phase 2 + relevant analyzer/parser work):**

58. `{T-115, T-173, T-212} -> T-214 -> T-215`: GC interactive timeline and algorithm comparison build on the existing GC analyzer plus the Phase 2 shell.
59. `T-194 -> T-216 -> T-217`: thread-dump→collapsed batch converter piggybacks on the Java parser plugin; Canvas/WebGL flamegraph rendering arrives after the converter and the Phase 2 shell exist (`T-187`/`T-212` co-deps).

**Phase 7 — TDA-main JVM hardening follow-up:**

60. `T-188/T-189/T-190/T-191 -> T-224 -> T-225`: split single-file JVM multi-dump logs first, then extend encoding/sniffing so UTF-16 dumps enter the same path.
61. `T-188/T-189 -> T-226` and `T-224 -> {T-227, T-228, T-229, T-230, T-231}`: add jcmd JSON as another Java parser plugin, then layer JVM-only diagnostics on the normalized dump timeline.
62. `{T-224, T-225, T-226, T-227, T-228, T-229, T-230, T-231} -> T-232 -> T-233`: lock the TDA-derived behavior with minimized fixtures before updating user-facing docs.

## Active Decision Queue

| Decision | Required before | Default |
|---|---|---|
| React 19 upgrade timing | After T-023 | Defer unless Electron/tooling compatibility requires it. (Revisit during T-212 since the design-system migration touches React anyway.) |
| HTML flamegraph format coverage | T-185 | Support async-profiler self-contained HTML and inline-SVG-wrapped HTML; emit `UNSUPPORTED_HTML_FORMAT` for the rest rather than guessing. |
| Thread-dump parser auto-detection vs. explicit `--format` | T-189 | Auto-detect by sniffing the first ~4 KB; allow `--format` to override. Reject mixed-language files within one bundle with a clear error. |
| Per-language framework cleanup scope | T-194, T-197, T-199, T-201 | Cover the top one or two web frameworks per language (Java: Spring AOP/CGLIB; Go: gin/echo/chi; Python: Django/FastAPI/Flask; Node.js: Express/Koa/NestJS/fastify). Other frameworks are deferred until requested. |
| Heap dump analyzer | This cycle | Out of scope. No HPROF parsing, no Eclipse MAT bundling, no MAT-result viewer. Revisit only via a future review document. |
| Web file-selection model | T-206 | Default to localhost-only server reading server-side absolute paths via `/api/files/select`; fall back to browser upload only if a non-localhost deployment becomes a goal. |
| Web transport for engine progress and cancellation | T-206, T-207 | WebSocket (`/ws/progress`) for streaming engine messages and cancellation signals; HTTP for synchronous analyzer execute/cancel endpoints. |
| Phase ordering — UI cleanups vs. platform pivot | T-180–T-183 | Do not implement T-180–T-183 separately. They are subsumed by Phase 2 (T-210–T-213) on the new web shell to avoid the same migration twice. |
| Design system choice | T-212 | Tailwind CSS + shadcn/ui (or equivalent headless component library if a spike surfaces a blocker). |
| Web app directory rename | T-207 | Rename `apps/desktop/` to `apps/web-ui/` (or equivalent) so the directory name reflects the post-Electron role. |
| TDA-main code reuse boundary | T-224 | Port behavior and algorithms into ArchScope Python code; do not copy Java source or UI/plugin packaging code directly. |
| Single-file vs multi-file thread-dump bundle semantics | T-224 | Treat a single log with multiple dumps as the same ordered timeline model used by multi-file correlation. Preserve source file and start-line metadata per extracted dump. |
| JVM text histogram vs heap dump scope | T-230 | Support lightweight text class histograms only. HPROF heap dump parsing and MAT integration remain out of scope. |
| Native profiler GUI framework | T-240a | **Decided 2026-05-05: Wails v3.** ~12–20 MB single binary post-Python-removal, reuses the existing `apps/frontend` React/Tailwind/shadcn/D3 stack so Phase-2 layout, en/ko i18n, dark mode, T-217 Canvas flamegraph, and T-214/T-215 GC charts ship without re-implementation. Webview dependencies — macOS WKWebView (built-in), Win10 WebView2 (bootstrapper), Linux WebKitGTK — are accepted trade-offs. Wails v3 is alpha; if blockers surface during T-240a fall back to Wails v2 (~15–25 MB). Fyne was rejected because every UI element would have to be re-built and maintained in parallel with the web UI. |

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
- Demo-site test log data (6 scenarios) is maintained in `projects-assets/test-data/demo-site/`, not in this repository. See `projects-assets/ASSET_INDEX.md` for the full catalog.
- Demo-site analyzer type mapping is canonical in `projects-assets/test-data/demo-site/analyzer_type_mapping.json` and is read by ArchScope at runtime.
- Automated UI smoke coverage exists for Demo Data Center navigation, stubbed run-result rendering, and Export Center handoff. Manual verification before the next review should still cover at least one real shared demo-site scenario and generated artifact opening.
