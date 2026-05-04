# Packaging Plan

ArchScope is delivered as a **local web application**, not as a desktop
binary. There is no Electron shell and no PyInstaller sidecar in the
current shipping model. The historical Electron-era plan is preserved
at the bottom of this document for context.

## Current shipping model

End users install the engine and serve the bundled React UI from a
single Python process:

```bash
cd engines/python
python -m venv .venv && source .venv/bin/activate
pip install -e .                       # registers `archscope-engine`

cd ../..
./scripts/serve-web.sh                 # builds apps/desktop/dist + serves
# Open http://127.0.0.1:8765
```

Components:

- **Python engine** — the `archscope-engine` distribution (defined by
  `engines/python/setup.cfg`). Installs Typer + FastAPI + uvicorn +
  defusedxml + python-multipart and exposes the `archscope-engine`
  console script. Wheel publishing and `pip install archscope-engine`
  from PyPI is the next step (see open items below).
- **React UI** — Vite produces a static bundle into
  `apps/desktop/dist`. `archscope-engine serve --static-dir
  apps/desktop/dist` (or the helper script) serves the bundle at `/`.
- **No platform binary** — there is no `.dmg`, `.msi`, or `.deb`. The
  user runs the engine from any Python ≥ 3.9. This collapses three
  past concerns (Electron version upgrades, electron-builder signing,
  PyInstaller sidecar paths) into a single supply chain.

User data lives under `~/.archscope/` (uploads, settings) and stays on
the local machine. The engine binds `127.0.0.1` by default.

## CSP policy

There is no Electron renderer to lock down anymore. The browser loads
the React bundle from FastAPI and talks back to the same origin via
`fetch('/api/...')`. No `unsafe-eval` is needed in production builds;
the only inline style still emitted comes from ECharts tooltip themes.

## Open items

These are the next packaging steps but are not yet implemented:

1. **Publish a versioned wheel to PyPI** so end users can run
   `pip install archscope-engine` without cloning the repository.
2. **Bundle the React `dist/` with the wheel** so the install does not
   require a Node.js toolchain. The wheel ships static files; the
   engine auto-resolves them from the package directory if
   `--static-dir` is omitted.
3. **Optional standalone runtime** — an `uv tool install
   archscope-engine` recipe so a user gets the CLI + web server from a
   single command without managing a virtualenv.
4. **Docker image** — `archscope-engine serve --host 0.0.0.0` for team
   use on a trusted internal host.

## Historical: Electron + PyInstaller spike (2026-Q1)

The original packaging plan stacked Electron over a PyInstaller
sidecar. That direction was abandoned in **Phase 1 (Web pivot, T-206
… T-209)** for three reasons: the Electron-bundled installer was too
large for the operations users we ship to, the PyInstaller sidecar
duplicated debugging surface, and the Electron IPC contract added
overhead the FastAPI HTTP boundary handles cleanly. The current shape
above replaces that plan in full. The retired implementation lived in
`apps/desktop/electron/` (deleted in Phase 1) and the spike artifacts
are no longer reachable from the build pipeline.
