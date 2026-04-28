#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../engines/python"
python -m archscope_engine.cli --help
