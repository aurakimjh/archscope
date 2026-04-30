from __future__ import annotations

import csv
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.models.flamegraph import FlameNode

COLUMN_ALIASES = {
    "key": {"key", "id", "node_key", "node_id"},
    "parent_key": {"parent_key", "parent", "parent_id", "parentKey"},
    "method_name": {"method_name", "method", "name", "frame", "methodName"},
    "ratio": {"ratio", "percent", "percentage"},
    "sample_count": {"sample_count", "samples", "sample", "count", "sampleCount"},
    "color_category": {"color_category", "category", "colorCategory"},
}


@dataclass(frozen=True)
class JenniferCsvParseResult:
    root: FlameNode
    diagnostics: dict[str, Any]


def parse_jennifer_flamegraph_csv(path: Path) -> JenniferCsvParseResult:
    diagnostics = ParserDiagnostics()
    with path.open("r", encoding="utf-8-sig", newline="") as handle:
        reader = csv.DictReader(handle)
        columns = _resolve_columns(reader.fieldnames or [])
        rows: dict[str, dict[str, Any]] = {}

        for line_number, row in enumerate(reader, start=2):
            diagnostics.total_lines += 1
            try:
                key = _required(row, columns, "key")
                name = _required(row, columns, "method_name")
                sample_count = int(float(_required(row, columns, "sample_count")))
                ratio = float(_optional(row, columns, "ratio") or 0.0)
            except (KeyError, ValueError) as exc:
                diagnostics.add_skipped(
                    line_number=line_number,
                    reason="INVALID_JENNIFER_ROW",
                    message=str(exc),
                    raw_line=str(row),
                )
                continue

            rows[key] = {
                "key": key,
                "parent_key": _optional(row, columns, "parent_key"),
                "method_name": name,
                "ratio": ratio,
                "sample_count": sample_count,
                "color_category": _optional(row, columns, "color_category"),
            }
            diagnostics.parsed_records += 1

    root = _build_tree(rows)
    return JenniferCsvParseResult(root=root, diagnostics=diagnostics.to_dict())


def _resolve_columns(fieldnames: list[str]) -> dict[str, str]:
    normalized = {field.strip(): field for field in fieldnames}
    lower = {field.strip().lower(): field for field in fieldnames}
    resolved: dict[str, str] = {}
    for canonical, aliases in COLUMN_ALIASES.items():
        for alias in aliases:
            if alias in normalized:
                resolved[canonical] = normalized[alias]
                break
            if alias.lower() in lower:
                resolved[canonical] = lower[alias.lower()]
                break
    return resolved


def _required(row: dict[str, str], columns: dict[str, str], canonical: str) -> str:
    column = columns.get(canonical)
    if column is None:
        raise KeyError(f"Missing required column: {canonical}")
    value = row.get(column, "").strip()
    if not value:
        raise ValueError(f"Missing required value: {canonical}")
    return value


def _optional(row: dict[str, str], columns: dict[str, str], canonical: str) -> str | None:
    column = columns.get(canonical)
    if column is None:
        return None
    value = row.get(column, "").strip()
    return value or None


def _build_tree(rows: dict[str, dict[str, Any]]) -> FlameNode:
    nodes: dict[str, FlameNode] = {}
    for key, row in rows.items():
        nodes[key] = FlameNode(
            id=key,
            parent_id=row["parent_key"] or None,
            name=row["method_name"],
            samples=row["sample_count"],
            ratio=row["ratio"],
            category=row["color_category"],
            color=row["color_category"],
            path=[],
        )

    roots: list[FlameNode] = []
    for key, node in nodes.items():
        parent_key = rows[key]["parent_key"]
        parent = nodes.get(parent_key) if parent_key else None
        if parent is None:
            roots.append(node)
        else:
            parent.children.append(node)

    if len(roots) == 1:
        root = roots[0]
        root.parent_id = None
    else:
        total_samples = sum(root.samples for root in roots)
        root = FlameNode(
            id="root",
            parent_id=None,
            name="All",
            samples=total_samples,
            ratio=100.0,
            children=sorted(roots, key=lambda item: item.samples, reverse=True),
            path=[],
        )
        for child in root.children:
            child.parent_id = root.id

    _assign_paths(root, [])
    return root


def _assign_paths(node: FlameNode, parent_path: list[str]) -> None:
    node.path = [*parent_path, node.name] if node.parent_id is not None else []
    for child in node.children:
        _assign_paths(child, node.path)
