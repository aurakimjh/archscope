from __future__ import annotations

import argparse
import json
from dataclasses import dataclass
from pathlib import Path
from typing import Any


DEFAULT_THRESHOLDS_MS = {
    "access_log_analyzer": 5_000.0,
    "profiler_collapsed_analyzer": 1_000.0,
    "jennifer_csv_analyzer": 2_000.0,
    "execution_breakdown_classifier": 1_000.0,
}


@dataclass(frozen=True)
class BenchmarkRow:
    name: str
    rows: int
    best_ms: float
    median_ms: float
    threshold_ms: float | None

    @property
    def status(self) -> str:
        if self.threshold_ms is not None and self.median_ms > self.threshold_ms:
            return "FAIL"
        return "PASS"


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Create a CI benchmark summary and enforce regression thresholds.",
    )
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--summary-out", required=True, type=Path)
    parser.add_argument(
        "--threshold",
        action="append",
        default=[],
        metavar="NAME=MS",
        help="Override median threshold in milliseconds for one benchmark.",
    )
    args = parser.parse_args()

    thresholds = {**DEFAULT_THRESHOLDS_MS, **_parse_thresholds(args.threshold)}
    rows = _read_rows(args.input, thresholds)
    summary = _render_summary(rows)
    args.summary_out.write_text(summary, encoding="utf-8")

    failed = [row for row in rows if row.status == "FAIL"]
    for row in failed:
        print(
            "::warning title=Benchmark regression::"
            f"{row.name} median {row.median_ms:.2f}ms exceeded "
            f"threshold {row.threshold_ms:.2f}ms"
        )
    if failed:
        raise SystemExit(1)


def _parse_thresholds(values: list[str]) -> dict[str, float]:
    thresholds: dict[str, float] = {}
    for value in values:
        if "=" not in value:
            raise SystemExit(f"Invalid threshold '{value}'. Expected NAME=MS.")
        name, raw_ms = value.split("=", 1)
        try:
            threshold = float(raw_ms)
        except ValueError as exc:
            raise SystemExit(f"Invalid threshold for {name}: {raw_ms}") from exc
        if not name or threshold <= 0:
            raise SystemExit(f"Invalid threshold '{value}'.")
        thresholds[name] = threshold
    return thresholds


def _read_rows(path: Path, thresholds: dict[str, float]) -> list[BenchmarkRow]:
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, list):
        raise SystemExit("Benchmark JSON must be a list.")
    rows: list[BenchmarkRow] = []
    for item in payload:
        if not isinstance(item, dict):
            raise SystemExit("Benchmark entries must be objects.")
        rows.append(_row_from_item(item, thresholds))
    return rows


def _row_from_item(
    item: dict[str, Any],
    thresholds: dict[str, float],
) -> BenchmarkRow:
    name = item.get("name")
    rows = item.get("rows")
    best_ms = item.get("best_ms")
    median_ms = item.get("median_ms")
    if not isinstance(name, str) or not name:
        raise SystemExit("Benchmark entry has invalid name.")
    if not isinstance(rows, int) or rows <= 0:
        raise SystemExit(f"Benchmark {name} has invalid rows.")
    if not isinstance(best_ms, (int, float)) or not isinstance(median_ms, (int, float)):
        raise SystemExit(f"Benchmark {name} has invalid timing fields.")
    return BenchmarkRow(
        name=name,
        rows=rows,
        best_ms=float(best_ms),
        median_ms=float(median_ms),
        threshold_ms=thresholds.get(name),
    )


def _render_summary(rows: list[BenchmarkRow]) -> str:
    lines = [
        "## ArchScope Core Benchmarks",
        "",
        "| Benchmark | Rows | Best ms | Median ms | Threshold ms | Status |",
        "|---|---:|---:|---:|---:|---|",
    ]
    for row in rows:
        threshold = f"{row.threshold_ms:.2f}" if row.threshold_ms is not None else "-"
        lines.append(
            f"| `{row.name}` | {row.rows} | {row.best_ms:.2f} | "
            f"{row.median_ms:.2f} | {threshold} | {row.status} |"
        )
    lines.append("")
    return "\n".join(lines)


if __name__ == "__main__":
    main()
