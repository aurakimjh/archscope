# ArchScope Work Status

Last updated: 2026-04-28

## Review Processing Status

- [x] Read `docs/review/2026-04-28_Architecture_and_Implementation_Review.md`
- [x] Read `docs/review/2026-04-28_claude_opus_architecture_review.md`
- [x] Consolidated review findings into this TO-DO
- [x] Moved processed review documents to `docs/review/done/`

## Current Priority

The next work cycle should focus on turning the current skeleton into a minimally integrated diagnostic flow:

```text
Desktop UI -> Electron IPC -> Python CLI -> AnalysisResult JSON -> ECharts
```

## TO-DO

### P0 - Engine/UI Integration

- [ ] Decide and document the Engine-UI bridge approach.
  - Preferred initial approach: Electron IPC + `child_process.execFile`.
  - Python engine should be invoked as a CLI first, then later bundled as a PyInstaller sidecar.
- [ ] Add a minimal Bridge PoC.
  - Desktop selects a sample file.
  - Electron main process calls Python CLI.
  - UI receives generated AnalysisResult JSON.
  - Dashboard or Access Log Analyzer renders returned chart data.
- [ ] Define analyzer client interfaces before replacing mock data.
  - Keep mock implementation for UI development.
  - Add a real implementation boundary for Electron IPC.

### P1 - Parser Robustness

- [ ] Improve Access Log parser malformed-line handling.
  - Count skipped lines.
  - Include skipped line count in `metadata`.
  - Add parser diagnostics without crashing the whole analysis.
- [ ] Improve collapsed parser malformed-line handling.
  - Handle lines without a trailing sample count.
  - Report invalid lines in metadata or diagnostics.
- [ ] Add encoding and corrupt-input behavior to parser design docs.

### P1 - AnalysisResult Contract Hardening

- [ ] Add type-specific result contracts.
  - Start with Python `TypedDict` for Access Log summary/series.
  - Add Profiler collapsed summary/series contract.
  - Keep full Pydantic migration as a later decision.
- [ ] Align TypeScript result types with Python result contracts.
- [ ] Document required keys for each result type in `docs/en/DATA_MODEL.md` and `docs/ko/DATA_MODEL.md`.

### P1 - Test Coverage

- [ ] Add `statistics.py` edge case tests.
  - Empty list
  - Single value
  - Repeated values
  - Negative values
  - Percentile interpolation
- [ ] Add malformed access log parser tests.
- [ ] Add malformed collapsed parser tests.
- [ ] Add JSON exporter write/read test.

### P2 - Large File Strategy

- [ ] Add sampling options to analyzers.
  - Max lines
  - Time range filter
- [ ] Refactor access log analyzer toward streaming aggregation.
  - Avoid materializing all `AccessLogRecord` values for large files.
  - Keep top URL, status distribution, and time buckets as incremental aggregates.
- [ ] Expand `docs/en/PARSER_DESIGN.md` and `docs/ko/PARSER_DESIGN.md` with streaming design details.

### P2 - Chart and UI Structure

- [ ] Convert page rendering in `App.tsx` to a mapping table.
- [ ] Add placeholder `Analyze` handlers with disabled/loading/error states.
- [ ] Prepare chart templates for dynamic loading in Phase 2.
- [ ] Keep i18n labels for chart titles, legends, and axis labels.

### P3 - Packaging and Runtime Expansion

- [ ] Plan early packaging spike for Electron + PyInstaller sidecar.
- [ ] Separate profiler stack classification rules from hardcoded Python logic.
- [ ] Add configuration-driven classification for JVM, Node.js, Python, Go, and .NET stacks.

## Workflow For New Review Documents

When new review documents are placed in `docs/review/`:

1. Read every unprocessed review document in `docs/review/`.
2. Extract actionable findings, risks, and recommendations.
3. Update this `work_status.md` TO-DO list.
4. Move processed review documents into `docs/review/done/`.
5. Keep AI-local instruction files out of git by using `.gitignore`.

## Notes

- `docs/review/done/` is an archive of processed reviews.
- `CLAUDE.MD`, `GEMINI.MD`, and `AGENTS.MD` are local AI working instruction files and must not be committed.
