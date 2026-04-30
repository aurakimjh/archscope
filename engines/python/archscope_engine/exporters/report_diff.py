from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from archscope_engine.models.analysis_result import AnalysisResult


def build_comparison_report(
    before_path: Path,
    after_path: Path,
    *,
    label: str | None = None,
) -> AnalysisResult:
    before = json.loads(before_path.read_text(encoding="utf-8"))
    after = json.loads(after_path.read_text(encoding="utf-8"))
    before_summary = _dict(before.get("summary"))
    after_summary = _dict(after.get("summary"))
    metric_rows = _metric_rows(before_summary, after_summary)
    findings_rows = [
        {
            "side": "before",
            "finding_count": _finding_count(before),
        },
        {
            "side": "after",
            "finding_count": _finding_count(after),
        },
    ]
    return AnalysisResult(
        type="comparison_report",
        source_files=[str(before_path), str(after_path)],
        summary={
            "label": label,
            "before_type": before.get("type"),
            "after_type": after.get("type"),
            "changed_metrics": sum(1 for row in metric_rows if row["delta"] != 0),
            "before_findings": findings_rows[0]["finding_count"],
            "after_findings": findings_rows[1]["finding_count"],
            "finding_delta": findings_rows[1]["finding_count"]
            - findings_rows[0]["finding_count"],
        },
        series={
            "summary_metric_deltas": metric_rows,
            "finding_count_comparison": findings_rows,
        },
        tables={"summary_metric_deltas": metric_rows},
        metadata={
            "schema_version": "0.1.0",
            "report_kind": "before_after_diff",
            "before_created_at": before.get("created_at"),
            "after_created_at": after.get("created_at"),
        },
    )


def _metric_rows(
    before_summary: dict[str, Any],
    after_summary: dict[str, Any],
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for metric in sorted(set(before_summary) | set(after_summary)):
        before_value = before_summary.get(metric)
        after_value = after_summary.get(metric)
        if not isinstance(before_value, (int, float)) or not isinstance(
            after_value,
            (int, float),
        ):
            continue
        delta = after_value - before_value
        rows.append(
            {
                "metric": metric,
                "before": before_value,
                "after": after_value,
                "delta": round(delta, 6),
                "change_percent": _change_percent(before_value, after_value),
            }
        )
    return rows


def _change_percent(before: float, after: float) -> float | None:
    if before == 0:
        return None
    return round(((after - before) / before) * 100, 4)


def _finding_count(payload: dict[str, Any]) -> int:
    metadata = _dict(payload.get("metadata"))
    findings = metadata.get("findings")
    return len(findings) if isinstance(findings, list) else 0


def _dict(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}
