#!/usr/bin/env bash
set -euo pipefail

# ==========================================================================
# build-desktop.sh — One-command build for the ArchScope desktop app.
#
# Produces a distributable Electron package with the Python engine bundled
# so end-users need ZERO runtime dependencies (no Python, no Node).
#
# Prerequisites (build machine only):
#   - Python 3.9+ with pip
#   - Node 18+ with npm
#   - PyInstaller  (pip install pyinstaller)
#
# Usage:
#   scripts/build-desktop.sh                 # full build
#   scripts/build-desktop.sh --skip-engine   # reuse existing engine binary
#   scripts/build-desktop.sh --dir           # produce unpacked dir (faster)
# ==========================================================================

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
engine_dir="$repo_root/engines/python"
frontend_dir="$repo_root/apps/frontend"
desktop_dir="$repo_root/apps/desktop"

skip_engine=0
builder_flag=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --skip-engine) skip_engine=1; shift ;;
    --dir)         builder_flag="--dir"; shift ;;
    *)             echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

# --------------------------------------------------------------------------
# Step 1: Bundle the Python engine with PyInstaller
# --------------------------------------------------------------------------
if [ "$skip_engine" -eq 0 ]; then
  echo ""
  echo "━━━ Step 1/3: Building Python engine (PyInstaller) ━━━"
  echo ""
  cd "$engine_dir"

  # Ensure dependencies + pyinstaller are available
  pip install -e ".[dev]" --quiet 2>/dev/null || pip install -e . --quiet
  pip install pyinstaller --quiet

  # Clean previous build
  rm -rf build/ dist/

  # Build — onedir mode (faster startup, easier to debug)
  pyinstaller archscope-engine.spec --noconfirm --clean

  echo ""
  echo "  ✓ Engine built: $engine_dir/dist/archscope-engine/"
else
  echo ""
  echo "━━━ Step 1/3: Skipping engine build (--skip-engine) ━━━"
  echo ""
  if [ ! -d "$engine_dir/dist/archscope-engine" ]; then
    echo "  ⚠ Warning: $engine_dir/dist/archscope-engine/ does not exist."
    echo "    The Electron app will fall back to looking for Python on PATH."
  fi
fi

# --------------------------------------------------------------------------
# Step 2: Build the React frontend
# --------------------------------------------------------------------------
echo ""
echo "━━━ Step 2/3: Building React frontend ━━━"
echo ""
cd "$frontend_dir"
npm install --no-audit --no-fund
npm run build
echo ""
echo "  ✓ Frontend built: $frontend_dir/dist/"

# --------------------------------------------------------------------------
# Step 3: Compile Electron + package
# --------------------------------------------------------------------------
echo ""
echo "━━━ Step 3/3: Packaging Electron app ━━━"
echo ""
cd "$desktop_dir"
npm install --no-audit --no-fund

# Compile TypeScript
npx tsc -p tsconfig.electron.json

# Package
if [ -n "$builder_flag" ]; then
  npx electron-builder $builder_flag
else
  npx electron-builder
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  ✓ Build complete!"
echo "  Output: $desktop_dir/release/"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
