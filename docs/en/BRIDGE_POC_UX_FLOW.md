# Engine ↔ UI Bridge

This document captured the original Electron-IPC bridge experiment.
Phase 1 (T-206 … T-209) replaced that bridge with an HTTP boundary
between the FastAPI engine and the React UI. The historical Electron
notes are at the bottom for context.

## Current bridge model

```text
Browser (React)
   │  window.archscope.* — same surface as the legacy IPC
   │      (selectFile / analyzer / exporter / demo / settings)
   │  src/api/httpBridge.ts mounts each call onto fetch('/api/...')
   ▼
FastAPI server (`archscope-engine serve`)
   • POST /api/upload                  multipart upload (uploads land
                                       under ~/.archscope/uploads/)
   • POST /api/analyzer/execute        single dispatcher (per type)
   • POST /api/analyzer/cancel         no-op (single-process)
   • POST /api/export/execute          html / pptx / diff
   • GET  /api/demo/list / POST /api/demo/run
   • GET  /api/files?path=…            stream local artifacts back
   • GET/PUT /api/settings             ~/.archscope/settings.json
   • GET  /                            static React build (--static-dir)
   ▼
archscope_engine package
   • analyzers called in-process — no subprocess, no Typer round-trip
   • Returns the AnalysisResult JSON envelope unchanged
```

### Why HTTP

- A browser can't speak Electron IPC, so the web pivot needed a
  language-neutral boundary anyway.
- One transport for both the local UI and any future LAN deployment
  (`--host 0.0.0.0`).
- The FastAPI process owns the analyzer modules directly, so each
  call stays in-process and avoids the previous subprocess fan-out.

### File selection contract

The UI exposes `window.archscope.selectFile(...)` exactly as the
Electron renderer used to. Under the hood:

1. The bridge spawns a hidden `<input type="file">` and resolves to the
   chosen `File` (or `{ canceled: true }` if the user dismisses).
2. The file is `multipart/form-data` POSTed to `/api/upload`.
3. The server stores it under `~/.archscope/uploads/<uuid>/<orig>` and
   returns `{ filePath, originalName, size }`.
4. The UI hands the server-side `filePath` to whichever analyzer
   request needs it (e.g. `params.filePath` on `/api/analyzer/execute`).

Per-analyzer pages can also instantiate the `FileDock` component
directly to bypass `selectFile` entirely; both paths land at the same
`/api/upload` endpoint.

### Cancellation

`/api/analyzer/cancel` exists but is a no-op in the current
single-process engine — every analyzer runs synchronously inside the
FastAPI request handler. The `archscope-engine serve --reload` dev
loop relies on uvicorn's auto-reload to pick up code changes; long
analyzer runs simply complete before the next request returns.

### Errors

The dispatcher returns structured errors with `code` + `message` plus
optional `detail`. The known codes used by the UI:

- `INVALID_OPTION` — request body is malformed.
- `FILE_NOT_FOUND` — the `filePath` no longer exists (uploads can be
  cleaned up under `~/.archscope/uploads/`).
- `UNKNOWN_THREAD_DUMP_FORMAT` — no plugin matched the head bytes of
  a thread-dump input.
- `MIXED_THREAD_DUMP_FORMATS` — multi-dump request resolved to more
  than one format. Pass `format` to override.
- `ENGINE_FAILED` — generic catch-all (the analyzer raised). The
  exception is preserved in `detail`.
- `ENGINE_OUTPUT_INVALID` (legacy CLI path) — kept for compatibility
  with the previous subprocess JSON contract.

## Historical: Electron IPC bridge (2026-Q1)

The original PoC let the React renderer call into the Electron main
process via IPC; the main process then `execFile`'d
`archscope-engine` as a subprocess and parsed its JSON output. Three
issues drove the move to HTTP in Phase 1:

1. The Electron sandbox blocked direct file-path access from the
   renderer, requiring an IPC handshake for every file selection.
2. The subprocess fan-out duplicated parser failure context across
   the engine, the IPC channel, and the renderer logger.
3. Bundle size — Electron + PyInstaller installer was several
   hundred MB before any user data; the FastAPI + Vite bundle is one
   pip install + one Vite build.

The retired Electron main / preload code lived in
`apps/desktop/electron/` and was deleted in Phase 1.
