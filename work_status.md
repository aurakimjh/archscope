# ArchScope Work Status

Last updated: 2026-04-29

## Review Processing Status

- [x] Read `docs/review/2026-04-28_Architecture_and_Implementation_Review.md`
- [x] Read `docs/review/2026-04-28_claude_opus_architecture_review.md`
- [x] Consolidated review findings into this TO-DO
- [x] Created `review_decisions.md` with accepted/deferred/rejected/needs-decision classifications
- [x] Moved processed review documents to `docs/review/done/`

## Research Processing Status

- [x] Read `docs/research/2026-04-28_Competitor_Analysis_and_Positioning_Gemini.md`
- [x] Read `docs/research/2026-04-28_claude-code_product-enhancement.md`
- [x] Created `docs/research/research_decisions.md`
- [x] Reflected accepted research items into roadmap, architecture, and execution backlog

## Current Priority

The next work cycle should focus on turning the current skeleton into a minimally integrated diagnostic flow:

```text
Desktop UI -> Electron IPC -> Python CLI -> AnalysisResult JSON -> ECharts
```

Engine-UI Bridge decision: Electron IPC + `child_process.execFile` invoking the Python CLI. Local HTTP/FastAPI is deferred unless web delivery becomes a near-term product goal.

AnalysisResult contract hardening scope: keep the common dataclass transport model, add type-specific Python `TypedDict` and TypeScript interfaces for `access_log` and `profiler_collapsed`, and defer full Pydantic migration until after the bridge JSON flow stabilizes.

Parser error handling policy: file/configuration failures are fatal; malformed record-level input is skipped by default and reported under `metadata.diagnostics`. Strict fail-fast parsing is deferred until there is an explicit option.

## Execution Backlog

### Phase 1A - Foundation Stabilization

Goal: make the current skeleton run through one real diagnostic path with explicit contracts and reliable parser failure behavior.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-001 | P0 | [x] | Decide and document the Engine-UI bridge approach. Decision: Electron IPC + `child_process.execFile` invoking the Python CLI. | None | RD-001, RD-026 | Bridge design note in `docs/en/ARCHITECTURE.md` and `docs/ko/ARCHITECTURE.md` |
| T-002 | P0 | [x] | Define analyzer client interfaces before replacing mock data. Keep a mock client and add a real IPC client boundary. | T-001 | RD-003 | TypeScript analyzer client contract |
| T-003 | P0 | [ ] | Build minimal Bridge PoC: select sample file, invoke Python CLI from Electron main process, return `AnalysisResult` JSON to renderer. | T-001, T-002, T-030, T-037 | RD-002, RS-005 | Working UI-to-engine diagnostic path |
| T-004 | P1 | [ ] | Improve Access Log parser malformed-line handling with skipped-line count and diagnostics metadata. Policy is fixed in parser design docs. | T-031 | RD-004, RS-009 | Parser behavior change plus metadata |
| T-005 | P1 | [ ] | Improve collapsed parser malformed-line handling for invalid trailing sample counts. Policy is fixed in parser design docs. | T-031 | RD-005, RS-009 | Parser behavior change plus metadata |
| T-006 | P1 | [ ] | Add malformed access log parser tests. | T-004 | RD-017 | Parser regression tests |
| T-007 | P1 | [ ] | Add malformed collapsed parser tests. | T-005 | RD-017 | Parser regression tests |
| T-008 | P1 | [ ] | Add `statistics.py` edge-case tests for empty, single, repeated, negative, and percentile interpolation cases. | None | RD-016 | Utility regression tests |
| T-009 | P1 | [ ] | Add JSON exporter write/read round-trip test. | None | RD-018 | Contract regression test |
| T-010 | P1 | [ ] | Add type-specific Python `TypedDict` contracts for Access Log and Profiler result sections. Scope is fixed in data model docs. | T-004, T-005 preferred | RD-006 | Python contract types |
| T-011 | P1 | [ ] | Align TypeScript result types with Python result contracts. | T-010 | RD-006 | UI contract types |
| T-012 | P1 | [ ] | Document required keys for each result type in `docs/en/DATA_MODEL.md` and `docs/ko/DATA_MODEL.md`. | T-010, T-011 | RD-006 | Updated data model docs |
| T-013 | P1 | [x] | Add encoding, corrupt-input, and malformed-record behavior to parser design docs. | None | RD-004, RD-005 | Updated `docs/en/PARSER_DESIGN.md` and `docs/ko/PARSER_DESIGN.md` |
| T-030 | P0 | [x] | Declare Python runtime dependencies and console script metadata for the engine CLI. Include `typer`, `rich`, and `archscope-engine`. | None | RS-006 | Reliable Python CLI installation path |
| T-037 | P0 | [ ] | Define the minimal Bridge PoC UX flow while implementing: file selection/drop, analyze action, loading state, success result rendering, parser diagnostics panel, and bridge/error messages. | T-001, T-002 | User follow-up | Minimal UI flow notes captured in implementation or UI design docs |
| T-031 | P1 | [ ] | Fix `iter_text_lines` encoding fallback so a mid-file decode failure cannot emit duplicated lines across fallback retries. | None | RS-008 | Encoding-safe line iterator plus tests |

### Phase 1B - Large File Baseline

Goal: define and implement low-risk controls for large input files before deeper streaming refactors.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-014 | P2 | [ ] | Add analyzer sampling options such as max lines and time range filter. | T-003 preferred | RD-008 | Analyzer input options |
| T-015 | P2 | [ ] | Expand `docs/en/PARSER_DESIGN.md` and `docs/ko/PARSER_DESIGN.md` with memory-bounded streaming strategy. | T-014 | RD-010 | Streaming design documentation |
| T-016 | P2 | [ ] | Design access log streaming aggregation with incremental counters, top URL aggregation, status distribution, and time buckets. | T-015 | RD-009 | Implementation design, not full refactor |
| T-017 | P2 | [ ] | Refactor access log analyzer toward streaming aggregation. | T-016 | RD-009 | Memory-bounded analyzer path |
| T-032 | P2 | [ ] | Add Access Log diagnostic rules beyond raw chart metrics, including status-code findings and slow URL interpretation. | T-004, T-010 | RS-017 | Report-grade access log findings |

### Phase 2 - Report-Ready UI and Charts

Goal: make the UI easier to extend and prepare chart rendering for dynamic result data.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-018 | P2 | [ ] | Convert page rendering in `App.tsx` to a mapping table. | None | RD-013 | Cleaner page registration |
| T-019 | P2 | [ ] | Add placeholder Analyze handlers with disabled, loading, and error states. | T-037 | RD-014 | UI state skeleton |
| T-020 | P2 | [ ] | Keep chart titles, legends, and axis labels i18n-ready. | T-003 preferred | RD-015 | Locale-aware chart labels |
| T-021 | P2 | [ ] | Prepare chart templates for dynamic loading during Chart Studio work. | T-003, T-020 | RD-012 | Template extraction plan or initial structure |
| T-033 | P2 | [ ] | Upgrade to ECharts 6 and evaluate dark mode, broken axis, custom chart, and SVG export impact. | T-003, T-020 | RS-012 | Chart upgrade plan and implementation spike |

### Phase 3 - Packaging and Runtime Expansion

Goal: reduce release risk and prepare analyzer expansion after the foundation is stable.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-022 | P3 | [ ] | Plan early packaging spike for Electron + PyInstaller sidecar. | T-003 | RD-019 | Packaging spike plan |
| T-023 | P3 | [ ] | Upgrade Electron to a supported major version after Bridge PoC validation. React upgrade remains deferred unless required by compatibility. | T-003, T-022 preferred | RD-025, RS-011 | Desktop dependency security upgrade |
| T-024 | P3 | [ ] | Review `setuptools` ceiling and packaging metadata cleanup. | T-022 | RD-020 | Packaging metadata decision |
| T-025 | P3 | [ ] | Decide whether to consolidate `setup.py` and `pyproject.toml`. | T-024 | RD-021 | Packaging metadata cleanup task |
| T-026 | P3 | [ ] | Separate profiler stack classification rules from hardcoded Python logic. | T-003, T-010 | RD-022 | Configurable classification design |
| T-027 | P3 | [ ] | Add configuration-driven classification for JVM, Node.js, Python, Go, and .NET stacks. | T-026 | RD-022 | Runtime classification configuration |

### Phase 4 - Advanced Diagnostics

Goal: add higher-value diagnostic correlation after analyzer contracts and UI integration are stable.

| ID | Priority | Status | Task | Depends on | Source | Output |
|---|---|---|---|---|---|---|
| T-028 | P4 | [ ] | Keep Timeline Correlation as a planned differentiator for JVM diagnostics. | T-003, T-010, Phase 3 JVM parsers | RD-023 | Roadmap item retained |
| T-034 | P4 | [ ] | Add JFR recording parser design spike, including parser library feasibility and AnalysisResult shape. | GC/thread parser direction, T-010 | RS-013 | JFR parser design decision |
| T-035 | P4 | [ ] | Add OpenTelemetry log input design, including trace/span context mapping for future correlation. | T-010, T-028 preferred | RS-014 | OTel input design decision |

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
5. `T-003 -> T-022`: only plan packaging spike after the actual bridge path exists.

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
