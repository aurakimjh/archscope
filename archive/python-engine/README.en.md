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

JFR — binary `.jfr` (JDK 11+ on PATH, or `JAVA_HOME/bin/jfr`, or
`ARCHSCOPE_JFR_CLI` env var) **or** the JSON output of
`jfr print --json`:

```bash
archscope-engine jfr analyze \
  --file recording.jfr \
  --out ../../examples/outputs/jfr-result.json \
  [--event-modes cpu,wall,alloc,lock,gc,exception,io,nativemem] \
  [--time-from "+30s"] [--time-to "-2m"] \
  [--thread-state RUNNABLE,BLOCKED] \
  [--min-duration-ms 5]

archscope-engine jfr analyze-json \
  --file ../../examples/jfr/sample-jfr-print.json \
  --out ../../examples/outputs/jfr-result.json
```

Profiler diff and pprof export:

```bash
# differential flame across two profile runs
archscope-engine profiler diff \
  --baseline before.json --target after.json \
  --out diff.json [--normalize] [--top-n 50]

# Google pprof binary (gzipped) — opens in Pyroscope, Speedscope,
# `go tool pprof`, or pprof.dev
archscope-engine profiler export-pprof \
  --input result.json --output profile.pb.gz
```

Multi-runtime thread dumps:

```bash
archscope-engine thread-dump analyze-multi \
  --input dump-1.txt --input dump-2.txt --input dump-3.txt \
  --out multi.json [--consecutive-threshold 3] [--format <plugin-id>]
```

The parser registry auto-detects across **9 plugin variants** —
`java_jstack`, `java_jcmd_json`, `go_goroutine`, `python_pyspy`,
`python_faulthandler`, `python_traceback`,
`nodejs_diagnostic_report`, `nodejs_sample_trace`,
`dotnet_clrstack`, `dotnet_environment_stacktrace`. UTF-16 / BOM
encoded inputs are auto-decoded. See
[`docs/en/MULTI_LANGUAGE_THREADS.md`](../../docs/en/MULTI_LANGUAGE_THREADS.md)
for the per-language enrichment matrix.

## Web server

```bash
# build the React UI once
npm --prefix ../../apps/frontend install
npm --prefix ../../apps/frontend run build

# start FastAPI engine + serve the static bundle
archscope-engine serve --static-dir ../../apps/frontend/dist
# open http://127.0.0.1:8765
```

The engine binds `127.0.0.1` by default. CORS is open
(`allow_origins=["*"]`) since requests are local-only and the bundled
Electron build loads the renderer from `file://`.
