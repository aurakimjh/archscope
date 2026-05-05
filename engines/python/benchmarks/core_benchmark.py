from __future__ import annotations

import argparse
import json
import statistics
import sys
import tempfile
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from time import perf_counter
from typing import Callable

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.analyzers.flamegraph_builder import build_flame_tree_from_collapsed
from archscope_engine.analyzers.profiler_analyzer import (
    analyze_collapsed_profile,
    analyze_jennifer_csv_profile,
)
from archscope_engine.analyzers.profiler_breakdown import build_execution_breakdown


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
        jennifer_csv = root / "jennifer.csv"
        _write_access_log(access_log, args.rows)
        _write_collapsed_profile(collapsed_profile, args.rows)
        _write_jennifer_csv(jennifer_csv, args.rows)
        breakdown_tree = _build_breakdown_tree(args.rows)

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
            _benchmark(
                "jennifer_csv_analyzer",
                args.rows,
                args.repeat,
                lambda: analyze_jennifer_csv_profile(jennifer_csv, interval_ms=100),
            ),
            _benchmark(
                "execution_breakdown_classifier",
                args.rows,
                args.repeat,
                lambda: build_execution_breakdown(
                    breakdown_tree,
                    interval_ms=100,
                    elapsed_sec=None,
                ),
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


def _write_jennifer_csv(path: Path, rows: int) -> None:
    with path.open("w", encoding="utf-8", newline="") as handle:
        handle.write("key,parent_key,method_name,ratio,sample_count,color_category\n")
        root_count = max(1, rows // 10)
        written = 0
        for root_index in range(root_count):
            if written >= rows:
                break
            root_key = f"root{root_index}"
            handle.write(
                f"{root_key},,com.example.Controller{root_index},100,10,application\n"
            )
            written += 1
            for child_index in range(9):
                if written >= rows:
                    break
                category = child_index % 3
                if category == 0:
                    method = "oracle.jdbc.driver.OraclePreparedStatement.executeQuery"
                    color = "sql"
                elif category == 1:
                    method = "org.springframework.web.client.RestTemplate.exchange"
                    color = "http"
                else:
                    method = "com.example.Service.handle"
                    color = "application"
                handle.write(
                    f"{root_key}_{child_index},{root_key},{method},10,1,{color}\n"
                )
                written += 1


def _build_breakdown_tree(rows: int):
    stacks: Counter[str] = Counter()
    for index in range(rows):
        kind = index % 4
        if kind == 0:
            stack = "com.example.Service;oracle.jdbc.driver.OracleStatement.executeQuery"
        elif kind == 1:
            stack = "com.example.Service;RestTemplate.exchange;SocketInputStream.socketRead"
        elif kind == 2:
            stack = "com.zaxxer.hikari.pool.HikariPool.getConnection;LockSupport.park"
        else:
            stack = f"com.example.Service;com.example.Worker{index % 100}"
        stacks[stack] += 1
    return build_flame_tree_from_collapsed(stacks)


def _format_runs(runs: list[float]) -> str:
    return ",".join(f"{run:.2f}" for run in runs)


if __name__ == "__main__":
    main()
