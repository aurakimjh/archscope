#!/usr/bin/env bash
set -euo pipefail

# Build the React app (if requested) and start the FastAPI server that
# serves both the API and the static UI bundle.
#
# Usage:
#   scripts/serve-web.sh              # build + serve on 127.0.0.1:8765
#   scripts/serve-web.sh --no-build   # serve only (uses existing apps/desktop/dist)
#   scripts/serve-web.sh --port 9000

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
desktop_dir="$repo_root/apps/desktop"
engine_dir="$repo_root/engines/python"

skip_build=0
extra_args=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-build)
      skip_build=1
      shift
      ;;
    *)
      extra_args+=("$1")
      shift
      ;;
  esac
done

if [ "$skip_build" -eq 0 ]; then
  echo "[serve-web] Building React app..."
  (cd "$desktop_dir" && npm install --no-audit --no-fund && npm run build)
fi

echo "[serve-web] Starting FastAPI server..."
exec "$repo_root/scripts/run-engine.sh" serve --static-dir "$desktop_dir/dist" "${extra_args[@]}"
