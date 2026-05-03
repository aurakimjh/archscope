from __future__ import annotations

import argparse
import json
import statistics
import sys
import tempfile
from dataclasses import dataclass
from pathlib import Path
from time import perf_counter
from typing import Callable

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.analyzers.profiler_analyzer import analyze_collapsed_profile


@dataclass(frozen=True)
class BenchmarkCase:
    name: str
    rows: int
    best_ms: float
    median_ms: float
    runs: list[float]


def main() -> None:
    parser = argparse.ArgumentParser(description="Run ArchScope core analyzer benchmarks.")
    parser.add_argument("--rows", type=int, default=10_000)
    parser.add_argument("--repeat", type=int, default=5)
    parser.add_argument("--json", action="store_true", help="Print machine-readable JSON.")
    args = parser.parse_args()

    if args.rows <= 0:
        raise SystemExit("--rows must be positive")
    if args.repeat <= 0:
        raise SystemExit("--repeat must be positive")

    with tempfile.TemporaryDirectory(prefix="archscope-benchmark-") as temp_dir:
        root = Path(temp_dir)
        access_log = root / "access.log"
        collapsed_profile = root / "profile.collapsed"
        _write_access_log(access_log, args.rows)
        _write_collapsed_profile(collapsed_profile, args.rows)

        cases = [
            _benchmark(
                "access_log_analyzer",
                args.rows,
                args.repeat,
                lambda: analyze_access_log(access_log),
            ),
            _benchmark(
                "profiler_collapsed_analyzer",
                args.rows,
                args.repeat,
                lambda: analyze_collapsed_profile(collapsed_profile, interval_ms=100),
            ),
        ]

    if args.json:
        print(json.dumps([case.__dict__ for case in cases], indent=2))
        return

    print(f"rows={args.rows} repeat={args.repeat}")
    for case in cases:
        print(
            f"{case.name}: best={case.best_ms:.2f}ms "
            f"median={case.median_ms:.2f}ms runs={_format_runs(case.runs)}"
        )


def _benchmark(name: str, rows: int, repeat: int, func: Callable[[], object]) -> BenchmarkCase:
    runs = []
    func()
    for _ in range(repeat):
        started = perf_counter()
        func()
        runs.append((perf_counter() - started) * 1000)
    return BenchmarkCase(
        name=name,
        rows=rows,
        best_ms=min(runs),
        median_ms=statistics.median(runs),
        runs=runs,
    )


def _write_access_log(path: Path, rows: int) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for index in range(rows):
            second = index % 60
            minute = (index // 60) % 60
            status = 500 if index % 97 == 0 else 200
            response_time = 2.5 if status >= 500 else 0.05 + ((index % 200) / 1000)
            handle.write(
                "127.0.0.1 - - "
                f"[27/Apr/2026:10:{minute:02d}:{second:02d} +0900] "
                f'"GET /api/orders/{index % 1000} HTTP/1.1" '
                f'{status} 1234 "-" "benchmark" {response_time:.3f}\n'
            )


def _write_collapsed_profile(path: Path, rows: int) -> None:
    with path.open("w", encoding="utf-8") as handle:
        for index in range(rows):
            endpoint = index % 200
            handle.write(
                "com.example.Controller;"
                f"com.example.Service{endpoint};"
                f"com.example.Dao{endpoint % 20} 1\n"
            )


def _format_runs(runs: list[float]) -> str:
    return ",".join(f"{run:.2f}" for run in runs)


if __name__ == "__main__":
    main()
