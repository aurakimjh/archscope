from __future__ import annotations

import json
from html import escape
from pathlib import Path
from typing import Any


def write_html_report(
    input_path: Path,
    output_path: Path,
    *,
    title: str | None = None,
) -> None:
    """Render an AnalysisResult or parser debug JSON file as a portable HTML report."""
    payload = json.loads(input_path.read_text(encoding="utf-8"))
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(
        render_html_report(payload, source_path=input_path, title=title),
        encoding="utf-8",
    )


def render_html_report(
    payload: dict[str, Any],
    *,
    source_path: Path | None = None,
    title: str | None = None,
) -> str:
    report_title = title or _default_title(payload)
    body = (
        _render_debug_log(payload, source_path=source_path)
        if _is_debug_log(payload)
        else _render_analysis_result(payload, source_path=source_path)
    )
    return "\n".join(
        [
            "<!doctype html>",
            '<html lang="en">',
            "<head>",
            '  <meta charset="utf-8">',
            '  <meta name="viewport" content="width=device-width, initial-scale=1">',
            f"  <title>{escape(report_title)}</title>",
            "  <style>",
            _stylesheet(),
            "  </style>",
            "</head>",
            "<body>",
            f"  <h1>{escape(report_title)}</h1>",
            body,
            "</body>",
            "</html>",
        ]
    )


def _render_analysis_result(
    payload: dict[str, Any],
    *,
    source_path: Path | None,
) -> str:
    result_type = str(payload.get("type", "unknown"))
    metadata = _dict(payload.get("metadata"))
    diagnostics = _dict(metadata.get("diagnostics"))
    parts = [
        _section(
            "Report Metadata",
            _definition_list(
                {
                    "Input JSON": str(source_path) if source_path else None,
                    "Result type": result_type,
                    "Created at": payload.get("created_at"),
                    "Schema version": metadata.get("schema_version"),
                    "Source files": ", ".join(_strings(payload.get("source_files"))),
                }
            ),
        ),
        _section("Summary", _render_key_values(_dict(payload.get("summary")))),
    ]

    findings = metadata.get("findings")
    if isinstance(findings, list) and findings:
        parts.append(_section("Findings", _render_records(findings, limit=20)))

    if diagnostics:
        parts.append(_section("Parser Diagnostics", _render_key_values(diagnostics)))

    parts.extend(_render_series_and_tables(payload))

    charts = _dict(payload.get("charts"))
    flamegraph = charts.get("flamegraph")
    if isinstance(flamegraph, dict):
        parts.append(_section("Flamegraph", _render_flamegraph(flamegraph)))
    if charts:
        parts.append(_section("Chart Data", _render_json_preview(charts)))

    return "\n".join(parts)


def _render_debug_log(
    payload: dict[str, Any],
    *,
    source_path: Path | None,
) -> str:
    summary = _dict(payload.get("summary"))
    context = _dict(payload.get("context"))
    redaction = _dict(payload.get("redaction"))
    parts = [
        _section(
            "Debug Log Metadata",
            _definition_list(
                {
                    "Input JSON": str(source_path) if source_path else None,
                    "Analyzer": context.get("analyzer_type"),
                    "Parser": context.get("parser"),
                    "Source file": context.get("source_file_name"),
                    "Encoding": context.get("encoding_detected"),
                    "Redaction": "enabled" if redaction.get("enabled") else "disabled",
                }
            ),
        ),
        _section("Summary", _render_key_values(summary)),
    ]
    errors_by_type = _dict(payload.get("errors_by_type"))
    if errors_by_type:
        parts.append(_section("Parse Errors", _render_debug_errors(errors_by_type)))
    exceptions = payload.get("exceptions")
    if isinstance(exceptions, list) and exceptions:
        parts.append(_section("Exceptions", _render_records(exceptions, limit=10)))
    hints = payload.get("hints")
    if isinstance(hints, list) and hints:
        hint_items = "".join(f"<li>{escape(str(h))}</li>" for h in hints)
        parts.append(_section("Hints", f"<ul>{hint_items}</ul>"))
    return "\n".join(parts)


def _render_series_and_tables(payload: dict[str, Any]) -> list[str]:
    sections: list[str] = []
    for group_name in ("series", "tables"):
        group = _dict(payload.get(group_name))
        if not group:
            continue
        for name, value in group.items():
            title = f"{group_name.title()}: {name}"
            if isinstance(value, list):
                sections.append(_section(title, _render_records(value, limit=50)))
            else:
                sections.append(_section(title, _render_json_preview(value)))
    return sections


def _render_debug_errors(errors_by_type: dict[str, Any]) -> str:
    blocks: list[str] = []
    for reason, entry in errors_by_type.items():
        error = _dict(entry)
        samples = error.get("samples")
        sample_html = _render_records(samples, limit=5) if isinstance(samples, list) else ""
        blocks.append(
            "<article class=\"subsection\">"
            f"<h3>{escape(reason)}</h3>"
            + _definition_list(
                {
                    "Count": error.get("count"),
                    "Description": error.get("description"),
                    "Failed pattern": error.get("failed_pattern"),
                }
            )
            + sample_html
            + "</article>"
        )
    return "\n".join(blocks)


def _render_flamegraph(root: dict[str, Any]) -> str:
    total_samples = _number(root.get("samples")) or 1
    return (
        '<div class="flamegraph" role="tree">'
        + _render_flame_node(root, total_samples=total_samples, depth=0)
        + "</div>"
    )


def _render_flame_node(
    node: dict[str, Any],
    *,
    total_samples: float,
    depth: int,
) -> str:
    children = node.get("children")
    child_nodes = (
        [child for child in children if isinstance(child, dict)]
        if isinstance(children, list)
        else []
    )
    samples = _number(node.get("samples")) or 0
    ratio = max(0.5, min(100.0, (samples / total_samples) * 100)) if total_samples else 0
    name = escape(str(node.get("name", "(unknown)")))
    sample_text = escape(str(node.get("samples", 0)))
    child_html = "".join(
        _render_flame_node(child, total_samples=total_samples, depth=depth + 1)
        for child in child_nodes
    )
    row = (
        f'<div class="flame-row" style="margin-left:{depth * 14}px">'
        f'<div class="flame-bar" style="width:{ratio:.2f}%">'
        f"<span>{name}</span><small>{sample_text}</small>"
        "</div></div>"
    )
    if not child_html:
        return row
    return f"<details open><summary>{row}</summary>{child_html}</details>"


def _render_key_values(values: dict[str, Any]) -> str:
    if not values:
        return "<p>No data.</p>"
    simple = {
        key: value
        for key, value in values.items()
        if not isinstance(value, (dict, list))
    }
    complex_values = {
        key: value
        for key, value in values.items()
        if isinstance(value, (dict, list))
    }
    html = _definition_list(simple)
    for key, value in complex_values.items():
        html += f"<h3>{escape(str(key))}</h3>{_render_json_preview(value)}"
    return html


def _render_records(records: list[Any], *, limit: int) -> str:
    rows = [record for record in records[:limit] if isinstance(record, dict)]
    if not rows:
        return _render_json_preview(records[:limit])
    columns: list[str] = []
    for row in rows:
        for key in row:
            if key not in columns:
                columns.append(key)
    header = "".join(f"<th>{escape(str(column))}</th>" for column in columns)
    body_rows = []
    for row in rows:
        cells = "".join(
            f"<td>{_format_cell(row.get(column))}</td>" for column in columns
        )
        body_rows.append(f"<tr>{cells}</tr>")
    note = (
        f"<p class=\"muted\">Showing {limit} of {len(records)} rows.</p>"
        if len(records) > limit
        else ""
    )
    return (
        f'{note}<div class="table-wrap"><table><thead><tr>{header}</tr></thead>'
        f"<tbody>{''.join(body_rows)}</tbody></table></div>"
    )


def _definition_list(values: dict[str, Any]) -> str:
    rows = []
    for key, value in values.items():
        if value is None:
            continue
        rows.append(
            f"<div><dt>{escape(str(key))}</dt><dd>{_format_cell(value)}</dd></div>"
        )
    return "<dl>" + "".join(rows) + "</dl>" if rows else "<p>No data.</p>"


def _section(title: str, content: str) -> str:
    return f"<section><h2>{escape(title)}</h2>{content}</section>"


def _render_json_preview(value: Any) -> str:
    return (
        "<pre>"
        + escape(json.dumps(value, ensure_ascii=False, indent=2, default=str))
        + "</pre>"
    )


def _format_cell(value: Any) -> str:
    if isinstance(value, (dict, list)):
        return _render_json_preview(value)
    if value is None:
        return '<span class="muted">null</span>'
    return escape(str(value))


def _number(value: Any) -> float | None:
    if isinstance(value, (int, float)):
        return float(value)
    return None


def _is_debug_log(payload: dict[str, Any]) -> bool:
    return "errors_by_type" in payload and "context" in payload and "environment" in payload


def _default_title(payload: dict[str, Any]) -> str:
    if _is_debug_log(payload):
        analyzer = _dict(payload.get("context")).get("analyzer_type") or "parser"
        return f"ArchScope Parser Debug Report - {analyzer}"
    return f"ArchScope Analysis Report - {payload.get('type', 'unknown')}"


def _dict(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def _strings(value: Any) -> list[str]:
    return [str(item) for item in value] if isinstance(value, list) else []


def _stylesheet() -> str:
    return "\n".join(
        [
            ":root {",
            "  color: #111827;",
            "  background: #f5f7fb;",
            "  font-family: Inter, ui-sans-serif, system-ui, -apple-system,",
            '    BlinkMacSystemFont, "Segoe UI", sans-serif;',
            "}",
            "body { max-width: 1180px; margin: 0 auto; padding: 32px; }",
            "h1 { margin: 0 0 20px; font-size: 30px; }",
            "h2 { margin: 0 0 14px; font-size: 19px; }",
            "h3 { margin: 18px 0 10px; font-size: 15px; }",
            "section {",
            "  margin: 0 0 18px;",
            "  padding: 18px;",
            "  border: 1px solid #dbe3ef;",
            "  border-radius: 8px;",
            "  background: #fff;",
            "  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);",
            "}",
            ".subsection {",
            "  margin: 12px 0;",
            "  padding: 12px;",
            "  border: 1px solid #e2e8f0;",
            "  border-radius: 8px;",
            "  background: #f8fafc;",
            "}",
            "dl {",
            "  display: grid;",
            "  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));",
            "  gap: 10px;",
            "  margin: 0;",
            "}",
            "dt { color: #64748b; font-size: 12px; font-weight: 800; }",
            "dd { margin: 4px 0 0; font-weight: 700; overflow-wrap: anywhere; }",
            ".table-wrap { overflow-x: auto; }",
            "table { width: 100%; border-collapse: collapse; font-size: 13px; }",
            "th, td {",
            "  padding: 10px;",
            "  border-bottom: 1px solid #e5e7eb;",
            "  text-align: left;",
            "  vertical-align: top;",
            "}",
            "th { color: #475569; background: #f8fafc; }",
            "pre {",
            "  max-height: 320px;",
            "  overflow: auto;",
            "  padding: 10px;",
            "  border-radius: 8px;",
            "  background: #0f172a;",
            "  color: #e2e8f0;",
            "  font-size: 12px;",
            "  white-space: pre-wrap;",
            "  overflow-wrap: anywhere;",
            "}",
            ".flamegraph {",
            "  overflow-x: auto;",
            "  padding: 10px;",
            "  border: 1px solid #e2e8f0;",
            "  border-radius: 8px;",
            "  background: #f8fafc;",
            "}",
            ".flamegraph details { margin: 0; }",
            ".flamegraph summary { display: block; cursor: pointer; }",
            ".flame-row { min-width: 760px; padding: 2px 0; }",
            ".flame-bar {",
            "  display: flex;",
            "  min-width: 28px;",
            "  justify-content: space-between;",
            "  gap: 8px;",
            "  padding: 4px 8px;",
            "  border-radius: 4px;",
            "  color: #fff;",
            "  background: #2563eb;",
            "  font-size: 12px;",
            "}",
            ".flame-bar span { overflow: hidden; text-overflow: ellipsis; }",
            ".flame-bar small { flex: 0 0 auto; opacity: 0.85; }",
            ".muted { color: #64748b; font-weight: 500; }",
        ]
    )
