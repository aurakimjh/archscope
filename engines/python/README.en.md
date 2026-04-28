# ArchScope Python Engine

The Python engine parses raw diagnostic files, normalizes records, aggregates statistics, and writes AnalysisResult-style JSON files.

## Install

```bash
cd engines/python
python -m venv .venv
source .venv/bin/activate
pip install -e .
```

## CLI

After installation, use the console script:

```bash
archscope-engine --help
```

Access log sample:

```bash
archscope-engine access-log analyze \
  --file ../../examples/access-logs/sample-nginx-access.log \
  --format nginx \
  --out ../../examples/outputs/access-log-result.json
```

Collapsed profiler sample:

```bash
archscope-engine profiler analyze-collapsed \
  --wall ../../examples/profiler/sample-wall.collapsed \
  --wall-interval-ms 100 \
  --elapsed-sec 1336.559 \
  --out ../../examples/outputs/profiler-result.json
```

For source-tree development before installation, `python -m archscope_engine.cli ...` remains supported.
