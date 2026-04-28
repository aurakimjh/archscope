# Bridge PoC UX Flow

This document fixes the minimum user experience for the first Engine-UI Bridge PoC.

The goal is to prove this path:

```text
select local file -> invoke analyzer -> receive AnalysisResult JSON -> render summary/charts/tables
```

The PoC should feel like a working diagnostic path, not a final analyzer workspace.

## Scope

The first PoC covers:

- Access Log analysis through `analyzeAccessLog({ filePath, format })`
- Collapsed profiler analysis through `analyzeCollapsedProfile({ wallPath, wallIntervalMs, elapsedSec, topN })`
- loading, success, parser diagnostics, and bridge error states

It does not cover:

- multi-file correlation
- chart editing
- report export
- persisted analysis history
- custom parser configuration UI

## Access Log Flow

1. The user selects or drops one access log file.
2. The user selects a log format. The default is `nginx`.
3. The Analyze button becomes enabled only when a file path and format are present.
4. On Analyze, the renderer calls `AnalyzerClient.analyzeAccessLog`.
5. While the request is pending, the file control and format control remain visible, and the Analyze button shows a loading state.
6. On success, the page renders the returned `AnalysisResult` summary, chart-ready series, and parser diagnostics.
7. On failure, the page keeps the selected file and options visible and shows a bridge error message near the action area.

## Profiler Flow

1. The user selects or drops one wall-clock collapsed stack file.
2. The user sets `wallIntervalMs`. The default is `100`.
3. The user may set `elapsedSec` and `topN`. The default `topN` is `20`.
4. The Analyze button becomes enabled only when a wall file path and positive interval are present.
5. On Analyze, the renderer calls `AnalyzerClient.analyzeCollapsedProfile`.
6. While the request is pending, controls remain visible and the Analyze button shows a loading state.
7. On success, the page renders summary metrics and the top stack table from the returned `AnalysisResult`.
8. On failure, the page keeps the selected file and options visible and shows a bridge error message near the action area.

## UI State Model

Analyzer pages should use these states:

| State | Meaning | Primary UI |
|---|---|---|
| `idle` | No analysis has started. | Empty metrics, empty chart/table area, disabled Analyze until required inputs exist. |
| `ready` | Required inputs are present. | Analyze enabled. |
| `running` | IPC request is pending. | Analyze disabled with loading text, previous result retained until a new success replaces it. |
| `success` | Analyzer returned `ok: true`. | Summary, series, tables, and diagnostics rendered from `result`. |
| `error` | Analyzer returned `ok: false` or IPC threw. | Error panel with stable code and human-readable message. |

The renderer should not parse stdout or infer success from process exit text. The `AnalyzerResponse` contract is the UI boundary.

## Success Rendering

Success rendering should prefer normalized `AnalysisResult` fields:

- summary cards from `result.summary`
- trend charts from `result.series`
- top-N or detail tables from `result.tables`
- optional chart templates from `result.charts`
- diagnostics from `result.metadata.diagnostics`

If a field is missing in the first PoC result, the affected panel should show an empty state instead of failing the whole page.

## Diagnostics Panel

The parser diagnostics panel is shown when `result.metadata.diagnostics` exists.

Minimum fields to support:

| Field | Display |
|---|---|
| `parsed_records` | Number of records included in aggregation. |
| `skipped_lines` | Number of malformed non-blank records skipped. |
| `encoding` | Encoding used to read the source file, when available. |
| `samples` | Bounded malformed-record examples, when available. |

Diagnostics are informational on a successful run. A result with skipped records can still be successful.

## Error Messages

Bridge errors should follow the `BridgeError` contract:

```text
{
  code,
  message,
  detail?
}
```

Initial error code categories:

| Code | When |
|---|---|
| `ANALYZER_NOT_CONNECTED` | Mock client or IPC bridge is not connected. |
| `FILE_NOT_FOUND` | Selected file path no longer exists or is not readable. |
| `INVALID_OPTION` | Required analyzer option is missing or invalid. |
| `ENGINE_EXITED` | Python CLI returned a non-zero exit code. |
| `ENGINE_OUTPUT_INVALID` | CLI finished but did not produce valid `AnalysisResult` JSON. |
| `IPC_FAILED` | IPC invocation failed before a normalized bridge response was returned. |

The UI should show `message` as the primary text and keep `code` visible for support/debugging. `detail` may be shown in a compact expandable area once that UI exists.

## Implementation Notes For T-003

- Keep file selection in the renderer, but execute Python only in the Electron main process.
- Pass typed request objects from renderer to preload to IPC.
- The main process should call the Python CLI with `execFile`, write output JSON to a temporary path, read the JSON, and return `AnalyzerResponse`.
- Do not expose raw command strings to the renderer.
- Do not make stdout parsing part of the data contract.
