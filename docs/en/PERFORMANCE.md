# Performance Measurement

ArchScope keeps performance changes measurement-first. Core analyzer timing can be
checked locally without extra dependencies:

```bash
cd engines/python
python3 benchmarks/core_benchmark.py --rows 10000 --repeat 5
python3 benchmarks/core_benchmark.py --rows 10000 --repeat 5 --json
```

The baseline benchmark generates temporary synthetic access-log and collapsed
profiler inputs, then measures:

- `access_log_analyzer`
- `profiler_collapsed_analyzer`

Use the JSON output in automation or before/after comparisons. The first
benchmark pass is a warm-up and is not included in reported timings.

## Profiling

Use `cProfile` for a first Python call graph:

```bash
cd engines/python
python3 -m cProfile -o /tmp/archscope-core.prof benchmarks/core_benchmark.py --rows 100000 --repeat 1
```

For sampling profiles during larger runs, use `py-spy` when available:

```bash
py-spy record -o /tmp/archscope.svg -- python3 benchmarks/core_benchmark.py --rows 100000 --repeat 3
```

Future CI work should publish benchmark JSON and alert on large regressions
rather than failing on small timing noise.
