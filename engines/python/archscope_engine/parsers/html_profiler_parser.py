"""Detector + parser for HTML-wrapped flamegraph payloads.

Two HTML flavors land here today:

1. **Inline-SVG HTML** — the HTML wrapper that ``perf script | flamegraph.pl
   --html`` and many CI dashboards emit. We locate the embedded ``<svg>``
   element and reuse :mod:`svg_flamegraph_parser`.

2. **async-profiler self-contained HTML** — the post-2.x output that ships
   the call tree as a JavaScript array of ``{name, samples, children}``
   nodes. We extract the JSON-ish array via a tolerant regex, parse it
   with the standard library, and flatten it into collapsed stacks.

Anything else (random HTML reports, SPA shells, etc.) is reported as
``UNSUPPORTED_HTML_FORMAT`` so the caller can surface the issue without
guessing.
"""
from __future__ import annotations

import json
import re
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.parsers.svg_flamegraph_parser import (
    SvgFlamegraphParseResult,
    parse_svg_flamegraph_text,
)

_SVG_BLOCK_RE = re.compile(r"<svg\b[^>]*>.*?</svg>", re.DOTALL | re.IGNORECASE)
# async-profiler 2.x/3.x emits a JS variable that holds the root flame node.
# We accept any of the common identifiers:
#   var root = {...}
#   const root = {...}
#   let levels = {...}
_AP_ROOT_RE = re.compile(
    r"(?:var|let|const)\s+(?:root|levels|profileTree|flame)\s*=\s*(\{.*?\})\s*;",
    re.DOTALL,
)


@dataclass(frozen=True)
class HtmlProfilerParseResult:
    stacks: Counter[str]
    diagnostics: dict[str, Any]
    detected_format: str


def parse_html_profiler(path: Path) -> HtmlProfilerParseResult:
    """Detect the flavor of an HTML flamegraph file and dispatch."""
    text = path.read_text(encoding="utf-8", errors="replace")
    return parse_html_profiler_text(text)


def parse_html_profiler_text(text: str) -> HtmlProfilerParseResult:
    diagnostics = ParserDiagnostics()
    diagnostics.total_lines = text.count("\n") + 1

    # 1) Inline SVG (the most common case: perf flamegraph + html shell).
    svg_match = _SVG_BLOCK_RE.search(text)
    if svg_match is not None:
        svg_text = svg_match.group(0)
        svg_result = parse_svg_flamegraph_text(svg_text)
        if svg_result.stacks:
            return HtmlProfilerParseResult(
                stacks=svg_result.stacks,
                diagnostics=_merge_svg_diagnostics(diagnostics, svg_result),
                detected_format="inline_svg",
            )

    # 2) async-profiler embedded JS tree.
    js_match = _AP_ROOT_RE.search(text)
    if js_match is not None:
        json_text = js_match.group(1)
        stacks = _stacks_from_async_profiler_js(json_text, diagnostics)
        if stacks:
            diagnostics.parsed_records = sum(stacks.values())
            return HtmlProfilerParseResult(
                stacks=stacks,
                diagnostics=diagnostics.to_dict(),
                detected_format="async_profiler_js",
            )

    diagnostics.add_skipped(
        line_number=0,
        reason="UNSUPPORTED_HTML_FORMAT",
        message=(
            "HTML payload does not contain an inline <svg> flamegraph nor a "
            "recognized async-profiler JS data block."
        ),
        raw_line=text[:200],
    )
    return HtmlProfilerParseResult(
        stacks=Counter(),
        diagnostics=diagnostics.to_dict(),
        detected_format="unsupported",
    )


def _merge_svg_diagnostics(
    diagnostics: ParserDiagnostics,
    svg_result: SvgFlamegraphParseResult,
) -> dict[str, Any]:
    merged = diagnostics.to_dict()
    svg_diag = svg_result.diagnostics
    merged["parsed_records"] = svg_diag.get("parsed_records", 0)
    merged["skipped_lines"] = merged.get("skipped_lines", 0) + svg_diag.get(
        "skipped_lines", 0
    )
    svg_skips = svg_diag.get("skipped_by_reason", {}) or {}
    merged_skips = dict(merged.get("skipped_by_reason", {}))
    for reason, count in svg_skips.items():
        merged_skips[reason] = merged_skips.get(reason, 0) + count
    merged["skipped_by_reason"] = merged_skips
    samples = list(merged.get("samples", []))
    samples.extend(svg_diag.get("samples", []))
    merged["samples"] = samples[:20]
    return merged


def _stacks_from_async_profiler_js(
    json_text: str,
    diagnostics: ParserDiagnostics,
) -> Counter[str]:
    """Parse a JS-style flame tree object into collapsed stacks.

    The async-profiler JS uses unquoted keys, so we normalize the most
    common keys (``name``, ``samples``, ``children``, ``value``) into
    JSON-compatible quoted form before handing the buffer to ``json.loads``.
    """
    normalized = _normalize_js_object(json_text)
    try:
        root = json.loads(normalized)
    except json.JSONDecodeError as exc:
        diagnostics.add_skipped(
            line_number=0,
            reason="INVALID_ASYNC_PROFILER_JSON",
            message=str(exc),
            raw_line=json_text[:200],
        )
        return Counter()

    stacks: Counter[str] = Counter()
    _walk_async_profiler_node(root, [], stacks)
    return stacks


def _walk_async_profiler_node(
    node: Any,
    path: list[str],
    stacks: Counter[str],
) -> None:
    if not isinstance(node, dict):
        return
    name = node.get("name") or node.get("title") or node.get("frame") or ""
    if not isinstance(name, str):
        name = str(name)
    next_path = path + ([name] if name else [])

    children = node.get("children") or node.get("c") or []
    samples_field = node.get("samples")
    if samples_field is None:
        samples_field = node.get("value")
    if samples_field is None:
        samples_field = node.get("v")

    has_children = isinstance(children, list) and len(children) > 0
    if not has_children:
        try:
            samples_int = int(samples_field) if samples_field is not None else 0
        except (TypeError, ValueError):
            samples_int = 0
        if samples_int > 0 and next_path:
            stacks[";".join(next_path)] += samples_int
        return

    # Track self-time on intermediate nodes (parent samples beyond the sum
    # of child samples) so flamegraphs that record both parent and child
    # totals don't lose the parent's exclusive cost.
    try:
        parent_samples = int(samples_field) if samples_field is not None else 0
    except (TypeError, ValueError):
        parent_samples = 0
    children_total = 0
    for child in children:
        child_samples = child.get("samples") if isinstance(child, dict) else None
        if child_samples is None and isinstance(child, dict):
            child_samples = child.get("value") or child.get("v")
        try:
            children_total += int(child_samples) if child_samples is not None else 0
        except (TypeError, ValueError):
            pass

    self_samples = parent_samples - children_total
    if self_samples > 0 and next_path:
        stacks[";".join(next_path)] += self_samples

    for child in children:
        _walk_async_profiler_node(child, next_path, stacks)


_JS_KEY_RE = re.compile(r"(?<=[\{,])\s*([A-Za-z_][A-Za-z0-9_]*)\s*:")


def _normalize_js_object(text: str) -> str:
    """Quote unquoted keys so :func:`json.loads` can parse the payload."""
    return _JS_KEY_RE.sub(r' "\1":', text)
