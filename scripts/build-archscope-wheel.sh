#!/usr/bin/env bash
# build-archscope-wheel.sh — deprecated.
#
# The active product line is now Go/Wails under apps/engine-native.
# The retired Python wheel source is preserved under archive/python-engine
# for audit and parity reference only.
set -euo pipefail

echo "The Python wheel build path has been retired."
echo "Use: cd apps/engine-native && go build ./cmd/archscope-engine"
echo "Use: cd apps/engine-native/cmd/archscope-profiler-app && task package"
exit 2
