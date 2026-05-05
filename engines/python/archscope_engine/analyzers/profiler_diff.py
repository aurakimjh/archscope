"""Differential flame graph analyzer.

Compare two profile recordings (baseline → target) and surface a flame tree
where each node carries its baseline samples, target samples, and delta.
The frontend renders this with a red/blue divergent palette so engineers
can see at a glance which stacks got more or less expensive between
recordings.

The two recordings are normalized to a common total before subtracting
when ``normalize=True`` (default), so a 10× longer target run does not
visually drown out the baseline. Set ``normalize=False`` to diff raw
sample counts.
"""
from __future__ import annotations

from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.flamegraph import FlameNode

SCHEMA_VERSION = "0.1.0"


@dataclass
class _DiffNode:
    name: str
    a: float = 0.0
    b: float = 0.0
    children: dict[str, "_DiffNode"] = field(default_factory=dict)
    path: list[str] = field(default_factory=list)


def build_diff_flame_tree(
    baseline_stacks: Counter[str],
    target_stacks: Counter[str],
    *,
    normalize: bool = True,
) -> tuple[FlameNode, dict[str, Any]]:
    """Return ``(diff_root, summary)``.

    ``diff_root`` is a :class:`FlameNode` where every node's ``metadata``
    dict carries ``{"a": float, "b": float, "delta": float, "delta_ratio":
    float}``. ``summary`` aggregates totals, top gains/losses, and path
    counts so the analyzer wrapper can populate the AnalysisResult shell.
    """
    base_total = sum(baseline_stacks.values())
    target_total = sum(target_stacks.values())

    if normalize and base_total > 0 and target_total > 0:
        a_scale = 1.0 / base_total
        b_scale = 1.0 / target_total
    else:
        a_scale = 1.0
        b_scale = 1.0

    root = _DiffNode(name="All")

    def _add(stacks: Counter[str], scale: float, *, side: str) -> None:
        for stack, samples in stacks.items():
            if samples <= 0:
                continue
            value = samples * scale
            current = root
            path: list[str] = []
            for frame in [part for part in stack.split(";") if part]:
                path.append(frame)
                child = current.children.get(frame)
                if child is None:
                    child = _DiffNode(name=frame, path=list(path))
                    current.children[frame] = child
                if side == "a":
                    child.a += value
                else:
                    child.b += value
                current = child
            # Inclusive accumulation on root happens by skipping the root
            # itself; the total is computed as max(sum_a, sum_b) on freeze.

    _add(baseline_stacks, a_scale, side="a")
    _add(target_stacks, b_scale, side="b")

    flat_paths: list[tuple[str, float, float]] = []  # (path_str, a, b)
    flame_root, root_total = _freeze_diff(root, flat_paths)

    # Build summary.
    deltas = [(path, a, b, b - a) for path, a, b in flat_paths]
    gains = sorted(deltas, key=lambda row: row[3], reverse=True)
    losses = sorted(deltas, key=lambda row: row[3])
    common = sum(1 for _, a, b, _ in deltas if a > 0 and b > 0)
    added = sum(1 for _, a, b, _ in deltas if a == 0 and b > 0)
    removed = sum(1 for _, a, b, _ in deltas if a > 0 and b == 0)

    summary = {
        "baseline_total": base_total,
        "target_total": target_total,
        "normalized": normalize,
        "common_paths": common,
        "added_paths": added,
        "removed_paths": removed,
        "max_increase": _row(gains[0]) if gains and gains[0][3] > 0 else None,
        "max_decrease": _row(losses[0]) if losses and losses[0][3] < 0 else None,
        "total_unit": "ratio" if normalize else "samples",
        "tree_total": root_total,
    }

    tables = {
        "biggest_increases": [_row(row) for row in gains[:30] if row[3] > 0],
        "biggest_decreases": [_row(row) for row in losses[:30] if row[3] < 0],
    }

    return flame_root, {"summary": summary, "tables": tables}


def _row(item: tuple[str, float, float, float]) -> dict[str, Any]:
    path, a, b, delta = item
    return {
        "stack": path,
        "baseline": round(a, 6),
        "target": round(b, 6),
        "delta": round(delta, 6),
        "delta_ratio": round((delta / a) if a > 0 else float("inf") if delta > 0 else 0.0, 4),
    }


def _freeze_diff(
    node: _DiffNode,
    flat: list[tuple[str, float, float]],
) -> tuple[FlameNode, float]:
    """Recursively materialize the mutable diff tree into FlameNode form.

    Each node's display ``samples`` is ``max(a, b)`` so the visualization
    width reflects the larger of the two sides; the diff color uses the
    real ``delta`` from ``metadata``.

    Returns the FlameNode plus the *root-level* total (max of the two
    cumulative sums) so callers can normalize ratios.
    """
    # Sum direct children first so root totals are accurate.
    children_nodes: list[FlameNode] = []
    sum_a = 0.0
    sum_b = 0.0
    for child in sorted(node.children.values(), key=lambda c: -max(c.a, c.b)):
        sum_a += child.a
        sum_b += child.b
        if child.path:
            flat.append((";".join(child.path), child.a, child.b))
        child_node, _ = _freeze_diff(child, flat)
        children_nodes.append(child_node)

    if not node.path:
        node.a = max(node.a, sum_a)
        node.b = max(node.b, sum_b)

    a_val = node.a
    b_val = node.b
    delta = b_val - a_val
    samples = max(a_val, b_val)

    flame_node = FlameNode(
        id=";".join(node.path) if node.path else "root",
        parent_id=";".join(node.path[:-1]) if len(node.path) > 1 else ("root" if node.path else None),
        name=node.name,
        samples=int(round(samples * 1_000_000)) if samples > 0 else 0,
        ratio=0.0,
        path=list(node.path),
        children=children_nodes,
        metadata={
            "a": round(a_val, 6),
            "b": round(b_val, 6),
            "delta": round(delta, 6),
        },
    )
    return flame_node, max(a_val, b_val)


def analyze_profiler_diff(
    baseline_stacks: Counter[str],
    target_stacks: Counter[str],
    *,
    baseline_path: Path | None = None,
    target_path: Path | None = None,
    normalize: bool = True,
) -> AnalysisResult:
    """Build a full ``profiler_diff`` AnalysisResult."""
    flame_root, parts = build_diff_flame_tree(
        baseline_stacks, target_stacks, normalize=normalize
    )
    summary = parts["summary"]
    tables = parts["tables"]

    source_files: list[str] = []
    if baseline_path is not None:
        source_files.append(str(baseline_path))
    if target_path is not None:
        source_files.append(str(target_path))

    return AnalysisResult(
        type="profiler_diff",
        source_files=source_files,
        summary=summary,
        series={},
        tables=tables,
        charts={"flamegraph": flame_root.to_dict()},
        metadata={
            "parser": "profiler_diff",
            "schema_version": SCHEMA_VERSION,
            "normalized": normalize,
            "baseline_file": str(baseline_path) if baseline_path else None,
            "target_file": str(target_path) if target_path else None,
        },
    )
