#!/usr/bin/env bash
#
# build-archscope-wheel.sh — produce the unified `archscope` wheel.
#
# Steps:
#   1. Build the React frontend (apps/frontend → apps/frontend/dist).
#   2. Copy that dist tree into the engine package data location
#      (engines/python/archscope_engine/web/static/) so the wheel
#      ships everything at runtime via importlib.resources.
#   3. Run `python -m build` against engines/python/ to emit the
#      sdist + bdist_wheel under engines/python/dist/.
#
# Output:
#   engines/python/dist/archscope-<version>-py3-none-any.whl
#   engines/python/dist/archscope-<version>.tar.gz
#
# Usage:
#   ./scripts/build-archscope-wheel.sh              # full build
#   SKIP_FRONTEND=1 ./scripts/build-archscope-wheel.sh   # python-only rerun
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRONTEND_DIR="$REPO_ROOT/apps/frontend"
ENGINE_DIR="$REPO_ROOT/engines/python"
STATIC_DIR="$ENGINE_DIR/archscope_engine/web/static"

cd "$REPO_ROOT"

if [[ -z "${SKIP_FRONTEND:-}" ]]; then
  echo "==> Building React frontend"
  if [[ ! -d "$FRONTEND_DIR/node_modules" ]]; then
    (cd "$FRONTEND_DIR" && npm ci)
  fi
  (cd "$FRONTEND_DIR" && npm run build)
fi

if [[ ! -d "$FRONTEND_DIR/dist" ]]; then
  echo "ERROR: $FRONTEND_DIR/dist does not exist. Run with SKIP_FRONTEND unset to build it." >&2
  exit 1
fi

echo "==> Copying React bundle into archscope_engine.web.static"
rm -rf "$STATIC_DIR"
mkdir -p "$STATIC_DIR"
cp -R "$FRONTEND_DIR/dist/." "$STATIC_DIR/"
# .gitkeep is added back so the working tree placeholder is preserved.
cat > "$STATIC_DIR/.gitkeep" <<'NOTE'
This directory is populated at build time with the React bundle from
apps/frontend/dist (see scripts/build-archscope-wheel.sh). It is empty
in the repository on purpose; only the wheel ships the actual assets.
NOTE

echo "==> Building Python wheel + sdist"
cd "$ENGINE_DIR"
rm -rf build dist *.egg-info
python3 -m pip install --quiet --upgrade build
python3 -m build

echo
echo "Artifacts:"
ls -la "$ENGINE_DIR/dist"
echo
echo "Install with:"
echo "  pip install $ENGINE_DIR/dist/archscope-*.whl"
echo "  archscope serve"
