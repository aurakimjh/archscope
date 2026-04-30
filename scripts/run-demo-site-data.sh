#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
manifest_root="${1:-"$repo_root/../projects-assets/test-data/demo-site"}"
output_root="${2:-"$repo_root/demo-site-report-bundles"}"

exec "$repo_root/scripts/run-engine.sh" demo-site run \
  --manifest-root "$manifest_root" \
  --out "$output_root"
