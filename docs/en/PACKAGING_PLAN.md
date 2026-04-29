# Packaging Plan

ArchScope packaging should keep the desktop shell and Python engine independently verifiable while producing a single user-facing desktop application.

## Packaging Spike Scope

The Phase 3 spike uses this target shape:

- Electron packages the renderer and main process.
- The Python engine is built as a PyInstaller sidecar executable.
- Electron invokes the sidecar with the same CLI contract used by development: analyzer command arguments plus `--out`.
- Packaged builds must preserve `AnalysisResult` JSON as the boundary between UI and engine.

## Spike Checklist

1. Build the Python engine sidecar with PyInstaller on macOS first.
2. Add a packaged app resource path for the sidecar.
3. Verify `access-log analyze` and `profiler analyze-collapsed` through the packaged sidecar.
4. Verify malformed-line diagnostics and engine stderr detail still reach the UI.
5. Repeat on Windows after macOS path and signing assumptions are known.

## Metadata Decision

The low `setuptools<64` ceiling has been raised to a modern bounded range. Full metadata consolidation into `pyproject.toml` is deferred until the packaging spike proves which metadata source is best for PyInstaller, editable development installs, and future wheel publishing.

For now:

- `setup.cfg` remains the source of package metadata, dependencies, extras, and entry points.
- `setup.py` remains the minimal compatibility shim.
- `pyproject.toml` owns build-system requirements plus tool configuration.

## CSP Nonce Evaluation

Production CSP already blocks unsafe script execution. Removing `style-src 'unsafe-inline'` would require nonce propagation for style injection and compatibility checks with React, Vite output, and ECharts tooltips/themes.

Decision: keep the current style policy during Phase 3 packaging. Revisit nonce-based style CSP after packaged renderer behavior and chart export flows are stable.
