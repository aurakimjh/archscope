# Archive

This directory keeps retired source trees for reference. The active product
line is now the Go/Wails implementation under `apps/engine-native`.

## Contents

- `python-engine/` — the former Python package (`archscope_engine`), Typer CLI,
  FastAPI server, and Python test suite. It is retained for parity reference and
  migration history, but it is no longer the active implementation target.
- `web-frontend-python/` — the former browser-served React frontend used by the
  Python `archscope serve` wheel path. The active desktop UI now lives under
  `apps/engine-native/cmd/archscope-profiler-app/frontend`.
- `work_status_legacy_2026-05-09.md` — the previous long-form project status and
  historical backlog before `work_status.md` was rewritten as the current active
  execution status.

## Policy

- Do not add new product features to archived code.
- Use archived code only for behavior comparison, migration audits, or release
  archaeology.
- Keep secrets, private logs, and generated heavyweight artifacts out of this
  directory.
- If a useful diagnostic rule or prompt is extracted from archived code, move
  the reusable asset to `../projects-assets` and update its asset index.
