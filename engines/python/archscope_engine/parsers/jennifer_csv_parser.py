from __future__ import annotations

import csv
from dataclasses import dataclass
import locale
from pathlib import Path
from typing import Any

from archscope_engine.common.debug_log import DebugLogCollector
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
REQUIRED_COLUMNS = ("key", "method_name", "sample_count")
ENCODING_FALLBACKS = ("utf-8-sig", "utf-8", "cp949")


@dataclass(frozen=True)
class JenniferCsvParseResult:
    root: FlameNode
    diagnostics: dict[str, Any]


def parse_jennifer_flamegraph_csv(
    path: Path,
    *,
    debug_log: DebugLogCollector | None = None,
    strict: bool = False,
) -> JenniferCsvParseResult:
    diagnostics = ParserDiagnostics(source_file=str(path), format="jennifer_flamegraph_csv")
    rows = _read_rows(path, diagnostics=diagnostics, debug_log=debug_log, strict=strict)
    root = _build_tree(rows, diagnostics=diagnostics, strict=strict)
    if _is_empty_file(path):
        diagnostics.add_warning(
            line_number=0,
            reason="EMPTY_FILE",
            message="Jennifer CSV file is empty.",
        )
    elif diagnostics.parsed_records == 0:
        diagnostics.add_warning(
            line_number=0,
            reason="NO_VALID_RECORDS",
            message="No valid Jennifer CSV rows were parsed.",
        )
    return JenniferCsvParseResult(root=root, diagnostics=diagnostics.to_dict())


def _is_empty_file(path: Path) -> bool:
    try:
        return path.stat().st_size == 0
    except OSError:
        return False


def _read_rows(
    path: Path,
    *,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
    strict: bool,
) -> dict[str, dict[str, Any]]:
    encodings = _encoding_candidates()
    last_decode_error: UnicodeDecodeError | None = None
    for encoding in encodings:
        checkpoint = _diagnostics_checkpoint(diagnostics)
        try:
            return _read_rows_with_encoding(
                path,
                encoding=encoding,
                diagnostics=diagnostics,
                debug_log=debug_log,
                strict=strict,
            )
        except UnicodeDecodeError as exc:
            _restore_diagnostics(diagnostics, checkpoint)
            last_decode_error = exc
            diagnostics.add_warning(
                line_number=0,
                reason="ENCODING_FALLBACK",
                message=f"Could not decode Jennifer CSV as {encoding}; trying next encoding.",
            )

    message = (
        "Could not decode Jennifer CSV with supported encodings: "
        f"{', '.join(encodings)}."
    )
    diagnostics.add_error(
        line_number=0,
        reason="ENCODING_ERROR",
        message=message,
    )
    if strict and last_decode_error is not None:
        raise ValueError(message) from last_decode_error
    return {}


def _diagnostics_checkpoint(diagnostics: ParserDiagnostics) -> dict[str, Any]:
    return diagnostics.to_dict()


def _restore_diagnostics(
    diagnostics: ParserDiagnostics,
    checkpoint: dict[str, Any],
) -> None:
    diagnostics.total_lines = int(checkpoint["total_lines"])
    diagnostics.parsed_records = int(checkpoint["parsed_records"])
    diagnostics.skipped_lines = int(checkpoint["skipped_lines"])
    diagnostics.skipped_by_reason = dict(checkpoint["skipped_by_reason"])
    diagnostics.samples = [dict(item) for item in checkpoint["samples"]]
    diagnostics.warning_count = int(checkpoint["warning_count"])
    diagnostics.error_count = int(checkpoint["error_count"])
    diagnostics.warnings = [dict(item) for item in checkpoint["warnings"]]
    diagnostics.errors = [dict(item) for item in checkpoint["errors"]]


def _read_rows_with_encoding(
    path: Path,
    *,
    encoding: str,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
    strict: bool,
) -> dict[str, dict[str, Any]]:
    rows: dict[str, dict[str, Any]] = {}
    with path.open("r", encoding=encoding, newline="") as handle:
        reader = csv.DictReader(handle)
        fieldnames = reader.fieldnames or []
        columns = _resolve_columns(fieldnames)
        missing_columns = [name for name in REQUIRED_COLUMNS if name not in columns]
        if missing_columns:
            message = (
                "Missing required Jennifer CSV columns: "
                f"{', '.join(missing_columns)}. Found columns: {', '.join(fieldnames)}."
            )
            diagnostics.add_error(
                line_number=1,
                reason="MISSING_REQUIRED_COLUMNS",
                message=message,
                raw_line=",".join(fieldnames),
            )
            if strict:
                raise ValueError(message)
            return rows

        for line_number, row in enumerate(reader, start=2):
            diagnostics.total_lines += 1
            try:
                parsed = _parse_row(row, columns)
            except ValueError as exc:
                _report_invalid_row(
                    row,
                    line_number=line_number,
                    message=str(exc),
                    diagnostics=diagnostics,
                    debug_log=debug_log,
                )
                if strict:
                    raise ValueError(f"{path}:{line_number}: {exc}") from exc
                continue

            key = parsed["key"]
            if key in rows:
                message = f"Duplicate Jennifer CSV key: {key}"
                _report_invalid_row(
                    row,
                    line_number=line_number,
                    message=message,
                    diagnostics=diagnostics,
                    debug_log=debug_log,
                    reason="DUPLICATE_KEY",
                )
                if strict:
                    raise ValueError(f"{path}:{line_number}: {message}")
                continue

            rows[key] = parsed
            diagnostics.parsed_records += 1

    return rows


def _parse_row(row: dict[str, str], columns: dict[str, str]) -> dict[str, Any]:
    key = _required(row, columns, "key")
    name = _required(row, columns, "method_name")
    sample_count = _parse_sample_count(_required(row, columns, "sample_count"))
    ratio = _parse_ratio(_optional(row, columns, "ratio"))
    return {
        "key": key,
        "parent_key": _optional(row, columns, "parent_key"),
        "method_name": name,
        "ratio": ratio,
        "sample_count": sample_count,
        "color_category": _optional(row, columns, "color_category"),
    }


def _report_invalid_row(
    row: dict[str, str],
    *,
    line_number: int,
    message: str,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
    reason: str = "INVALID_JENNIFER_ROW",
) -> None:
    raw_row = str(row)
    diagnostics.add_skipped(
        line_number=line_number,
        reason=reason,
        message=message,
        raw_line=raw_row,
    )
    if debug_log is not None:
        debug_log.add_parse_error(
            line_number=line_number,
            reason=reason,
            message=message,
            raw_context={"before": None, "target": raw_row, "after": None},
            failed_pattern="JENNIFER_FLAMEGRAPH_CSV_COLUMNS",
            field_shapes={
                "csv_columns": list(row.keys()),
                "non_empty_columns": [
                    key for key, value in row.items() if str(value).strip()
                ],
            },
        )


def _encoding_candidates() -> tuple[str, ...]:
    preferred = locale.getpreferredencoding(False)
    values: list[str] = []
    for encoding in (*ENCODING_FALLBACKS, preferred):
        if encoding and encoding not in values:
            values.append(encoding)
    return tuple(values)


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


def _parse_sample_count(value: str) -> int:
    try:
        parsed = float(value)
    except ValueError as exc:
        raise ValueError("sample_count must be numeric.") from exc
    if not parsed.is_integer():
        raise ValueError("sample_count must be a whole number.")
    sample_count = int(parsed)
    if sample_count < 0:
        raise ValueError("sample_count must be non-negative.")
    return sample_count


def _parse_ratio(value: str | None) -> float:
    if value is None:
        return 0.0
    normalized = value.strip().rstrip("%")
    if not normalized:
        return 0.0
    try:
        ratio = float(normalized)
    except ValueError as exc:
        raise ValueError("ratio must be numeric when present.") from exc
    if ratio < 0:
        raise ValueError("ratio must be non-negative.")
    return ratio


def _build_tree(
    rows: dict[str, dict[str, Any]],
    *,
    diagnostics: ParserDiagnostics,
    strict: bool,
) -> FlameNode:
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

    cycle_keys = _find_cycle_keys(
        {
            key: row["parent_key"]
            for key, row in rows.items()
            if row["parent_key"] in rows
        }
    )
    if cycle_keys:
        message = "Cycle detected in Jennifer CSV parent_key references: " + ", ".join(
            sorted(cycle_keys)
        )
        diagnostics.add_error(
            line_number=0,
            reason="PARENT_CYCLE",
            message=message,
        )
        if strict:
            raise ValueError(message)
        for key in cycle_keys:
            rows[key]["parent_key"] = None
            nodes[key].parent_id = None

    roots: list[FlameNode] = []
    for key, node in nodes.items():
        parent_key = rows[key]["parent_key"]
        parent = nodes.get(parent_key) if parent_key else None
        if parent is None:
            if parent_key:
                message = (
                    f"Missing Jennifer CSV parent_key '{parent_key}' for key '{key}'; "
                    "treating row as a root."
                )
                diagnostics.add_warning(
                    line_number=0,
                    reason="MISSING_PARENT",
                    message=message,
                )
                if strict:
                    raise ValueError(message)
            roots.append(node)
        else:
            parent.children.append(node)

    has_virtual_root = len(roots) != 1
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

    _validate_inclusive_samples(root, diagnostics=diagnostics)
    _assign_paths(root, [], include_root_path=not has_virtual_root)
    return root


def _assign_paths(
    root: FlameNode,
    parent_path: list[str],
    *,
    include_root_path: bool,
) -> None:
    stack: list[tuple[FlameNode, list[str], bool]] = [(root, parent_path, True)]
    while stack:
        node, path, is_root = stack.pop()
        if is_root and not include_root_path:
            node.path = []
        else:
            node.path = [*path, node.name]
        for child in reversed(node.children):
            stack.append((child, node.path, False))


def _find_cycle_keys(parent_by_key: dict[str, str]) -> set[str]:
    cycle_keys: set[str] = set()
    visited: set[str] = set()
    for start in parent_by_key:
        if start in visited:
            continue
        path_index: dict[str, int] = {}
        path: list[str] = []
        current: str | None = start
        while current is not None and current in parent_by_key:
            if current in path_index:
                cycle_keys.update(path[path_index[current] :])
                break
            if current in visited:
                break
            path_index[current] = len(path)
            path.append(current)
            current = parent_by_key.get(current)
        visited.update(path)
    return cycle_keys


def _validate_inclusive_samples(
    root: FlameNode,
    *,
    diagnostics: ParserDiagnostics,
) -> None:
    stack = [root]
    while stack:
        node = stack.pop()
        child_total = sum(child.samples for child in node.children)
        if node.children and child_total > node.samples:
            diagnostics.add_warning(
                line_number=0,
                reason="CHILD_SAMPLES_EXCEED_PARENT",
                message=(
                    f"Jennifer CSV node '{node.name}' has children totaling "
                    f"{child_total} samples, above parent sample_count {node.samples}."
                ),
            )
        stack.extend(node.children)
