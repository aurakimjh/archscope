# ArchScope Architecture

ArchScope is an application architecture diagnostic and reporting toolkit. Its core responsibility is to transform raw operational evidence into normalized analysis results and report-ready visualizations.

## Product Positioning

ArchScope is positioned as a **privacy-first local professional diagnostic workbench**.

It should combine:

- the convenience of SaaS JVM diagnostic tools,
- the local/offline safety of traditional desktop analyzers,
- modern report-ready visualization,
- and a normalized evidence contract that can support multiple runtimes.

The product direction is not to become a general log viewer or a full observability backend. ArchScope should stay focused on turning offline operational evidence into architecture diagnosis and report artifacts.

## System Flow

```text
Raw Data
  -> Parsing
  -> Analysis / Aggregation
  -> Visualization
  -> Report-ready Export
```

## Components

### Desktop UI

The desktop application is built with Electron, React, TypeScript, and Apache ECharts. It provides:

- Analyzer navigation
- File selection workflows
- Chart dashboards
- Chart Studio placeholders
- Export Center placeholders
- English/Korean UI locale switching

The UI reads normalized analysis result JSON rather than raw log files. This keeps UI rendering independent from parser implementation details.

### Python Analysis Engine

The Python engine owns parsing, normalization, aggregation, and export. It is organized around clear module boundaries:

- `parsers`: Convert source files into typed records.
- `analyzers`: Aggregate records into summary, series, and table structures.
- `ai_interpretation`: Evidence-bound AI guardrails, prompt construction, local runtime policy, and validation.
- `models`: Shared dataclasses for normalized data.
- `exporters`: Write normalized results to JSON, CSV, HTML, and future formats.
- `common`: Shared file, time, and statistics utilities.

### Result Contract

All analyzers should emit an AnalysisResult-like JSON structure:

```text
type
source_files
created_at
summary
series
tables
charts
metadata
```

Charts are rendered from normalized result fields, not raw log lines.
User-facing labels should come from locale resources so the same normalized result can be shown in English or Korean.

Profiler analysis adds a common `FlameNode` tree contract under `charts.flamegraph`. Both async-profiler collapsed stacks and Jennifer APM flamegraph CSV are normalized into this tree, then reused by drill-down stages and execution breakdown.

### AI Interpretation Boundary

AI-assisted interpretation is an optional downstream layer. It consumes existing `AnalysisResult` evidence and produces a separate `InterpretationResult`. It must not mutate deterministic analyzer findings or become required for normal analysis.

The boundary is:

```text
AnalysisResult
  -> EvidenceRegistry / EvidenceSelector
  -> PromptBuilder
  -> optional LocalLlmClient
  -> AiFindingValidator
  -> Report/UI provenance panel
```

The local LLM policy is localhost-only, disabled by default, and prompt/response logging is off by default. Electron may show `metadata.ai_interpretation` only after it validates the AI interpretation contract.

## Engine-UI Bridge

The initial Engine-UI bridge is fixed as:

```text
React Renderer
  -> preload-exposed API
  -> Electron IPC
  -> Electron Main Process
  -> child_process.execFile
  -> Python Engine CLI
  -> AnalysisResult JSON
  -> ECharts renderer
```

### Decision

ArchScope will use **Electron IPC + Python CLI child process** for the desktop integration path.

The renderer process must not spawn Python directly. It calls a narrow API exposed by `preload.ts`. The preload layer forwards requests through `ipcRenderer.invoke`. The Electron main process owns process execution and invokes the Python engine with `child_process.execFile`, never shell execution.

### Development Runtime

During development, the main process may invoke the Python engine through one of these equivalent forms:

```text
python -m archscope_engine.cli ...
```

or the installed console script:

```text
archscope-engine ...
```

The command writes an `AnalysisResult` JSON file to a temporary output path. The main process reads and validates the JSON shape before returning it to the renderer.

### Packaged Runtime

For packaged desktop builds, the Python engine should be bundled later as a PyInstaller sidecar binary and resolved from the application resources directory. This packaging step is intentionally deferred until after the Bridge PoC.

### IPC Contract

The renderer-facing API should be typed around analyzer requests, not raw command lines. Initial request shapes:

```text
analyzeAccessLog({ filePath, format })
analyzeCollapsedProfile({ wallPath, wallIntervalMs, elapsedSec, topN })
```

Initial response shape:

```text
{
  ok: true,
  result: AnalysisResult
}
```

Error response shape:

```text
{
  ok: false,
  error: {
    code,
    message,
    detail?
  }
}
```

### Bridge Rules

- Use `execFile`, not `exec`, to avoid shell interpolation.
- Keep allowed analyzer commands explicit in the Electron main process.
- Pass file paths and analyzer options as argument arrays.
- Store temporary JSON output outside the project tree.
- Return normalized JSON to the renderer; do not expose stdout parsing as the data contract.
- Preserve `contextIsolation: true` and keep `nodeIntegration: false`.
- Treat local HTTP/FastAPI as a future option only if web delivery becomes a near-term product goal.

## Extension Model

New diagnostic data types should follow this path:

1. Add record model in `models`.
2. Add streaming parser in `parsers`.
3. Add aggregation logic in `analyzers`.
4. Add JSON export support through the shared exporter.
5. Add chart templates in the desktop chart catalog.
6. Add UI page or extend an existing analyzer page.

This extension model should remain SDK-like: parser and analyzer additions must be possible without rewriting UI foundations or changing the outer `AnalysisResult` transport shape.

## Runtime Scope

Although JVM diagnostics are an early focus, ArchScope should remain runtime-neutral. The model supports Java, Node.js, Python, Go, .NET, and middleware-specific evidence.

Future JVM and observability inputs may include JFR recordings and OpenTelemetry log records. These inputs should still normalize into `AnalysisResult` and preserve traceable raw evidence or event references.

## Packaging Direction

Future packaging will use:

- `electron-builder` for the desktop application.
- `PyInstaller` for a distributable Python engine.

The initial skeleton intentionally avoids packaging implementation.
