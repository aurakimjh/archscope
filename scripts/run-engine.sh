#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -eq 0 ]; then
  set -- --help
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"

if command -v archscope-engine >/dev/null 2>&1; then
  exec archscope-engine "$@"
fi

cd "$repo_root/apps/engine-native"
exec go run ./cmd/archscope-engine "$@"
