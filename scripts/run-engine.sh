#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -eq 0 ]; then
  set -- --help
fi

if command -v archscope-engine >/dev/null 2>&1; then
  exec archscope-engine "$@"
fi

cd "$(dirname "$0")/../engines/python"
if command -v python >/dev/null 2>&1; then
  python_cmd=python
elif command -v python3 >/dev/null 2>&1; then
  python_cmd=python3
else
  python_cmd=
fi

if [ -n "$python_cmd" ] && "$python_cmd" -c "import typer, rich" >/dev/null 2>&1; then
  exec "$python_cmd" -m archscope_engine.cli "$@"
fi

printf '%s\n' "archscope-engine is not installed. Run 'cd engines/python && pip install -e .' first." >&2
exit 127
