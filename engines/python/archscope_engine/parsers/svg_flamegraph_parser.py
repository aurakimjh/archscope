"""Parser for FlameGraph.pl- and async-profiler-style SVG flamegraphs.

The SVG layout produced by Brendan Gregg's FlameGraph.pl, async-profiler
``-o svg`` and other compatible tools shares a small set of conventions:

- Each frame is a ``<g>`` element (often class ``func_g``) containing
  exactly one ``<rect>`` (the bar) and one ``<title>`` whose text matches
  ``"<frame> (<n> samples, <pct>%, ...)"``.
- The bar geometry encodes the position in the call tree: ``y`` selects
  the depth row, ``x`` and ``width`` mark the slice of the parent's range
  the frame covers.
- Brendan's default orientation puts the root at the bottom (highest
  ``y``); async-profiler's icicle output inverts it (root at the top).
  We auto-detect by picking the row that contains the widest rectangle.

We rebuild the call tree from the rectangles by, for each leaf rect (no
rectangle in the toward-leaves direction overlaps its x-range), walking
back toward the root and collecting frame names. The resulting stacks
are emitted as a ``Counter[str]`` keyed by ``"frame1;frame2;...;leaf"``
so the existing collapsed-profile pipeline (`build_collapsed_result`)
can ingest them unchanged.

XXE attacks on the input are blocked by ``defusedxml``.
"""
from __future__ import annotations

import re
from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from defusedxml import ElementTree as ET

from archscope_engine.common.diagnostics import ParserDiagnostics

SVG_NS = "{http://www.w3.org/2000/svg}"

# "name (12,345 samples, 0.78%)" or "name (12345 samples, 0.78%, 1.23 seconds)"
_TITLE_RE = re.compile(
    r"^(?P<name>.+?)\s*\(\s*(?P<samples>[\d,]+)\s*samples?\b",
    re.DOTALL,
)


@dataclass(frozen=True)
class _Rect:
    x: float
    y: float
    width: float
    height: float
    name: str
    samples: int


@dataclass(frozen=True)
class SvgFlamegraphParseResult:
    stacks: Counter[str]
    diagnostics: dict[str, Any]


def parse_svg_flamegraph(path: Path) -> SvgFlamegraphParseResult:
    """Parse an SVG flamegraph file into collapsed-format stacks."""
    text = path.read_text(encoding="utf-8", errors="replace")
    return parse_svg_flamegraph_text(text)


def parse_svg_flamegraph_text(
    text: str,
    *,
    diagnostics: ParserDiagnostics | None = None,
) -> SvgFlamegraphParseResult:
    """Parse an SVG flamegraph payload (already loaded into memory)."""
    diagnostics = diagnostics or ParserDiagnostics()

    try:
        root = ET.fromstring(text)
    except ET.ParseError as exc:
        diagnostics.add_skipped(
            line_number=0,
            reason="INVALID_SVG",
            message=str(exc),
            raw_line=text[:200],
        )
        return SvgFlamegraphParseResult(Counter(), diagnostics.to_dict())

    rects = _extract_rects(root, diagnostics)
    if not rects:
        return SvgFlamegraphParseResult(Counter(), diagnostics.to_dict())

    stacks = _stacks_from_rects(rects)
    diagnostics.total_lines = len(rects)
    diagnostics.parsed_records = sum(stacks.values())
    return SvgFlamegraphParseResult(stacks, diagnostics.to_dict())


def _extract_rects(root: Any, diagnostics: ParserDiagnostics) -> list[_Rect]:
    rects: list[_Rect] = []
    # Iterate over every <g> regardless of class — we filter by the presence
    # of a <title>+<rect> pair which is the universal flamegraph marker.
    for group in root.iter(f"{SVG_NS}g"):
        title_elem = group.find(f"{SVG_NS}title")
        rect_elem = group.find(f"{SVG_NS}rect")
        if title_elem is None or rect_elem is None:
            continue

        title_text = (title_elem.text or "").strip()
        match = _TITLE_RE.match(title_text)
        if not match:
            diagnostics.add_skipped(
                line_number=0,
                reason="UNPARSEABLE_TITLE",
                message=f"Title does not match name(N samples) pattern: {title_text[:80]}",
                raw_line=title_text,
            )
            continue

        try:
            x = float(rect_elem.attrib.get("x", "0"))
            y = float(rect_elem.attrib.get("y", "0"))
            width = float(rect_elem.attrib.get("width", "0"))
            height = float(rect_elem.attrib.get("height", "0"))
        except ValueError:
            diagnostics.add_skipped(
                line_number=0,
                reason="INVALID_RECT_GEOMETRY",
                message="Could not parse rect attributes.",
                raw_line=str(rect_elem.attrib)[:200],
            )
            continue

        if width <= 0 or height <= 0:
            continue

        name = match.group("name").strip()
        samples_text = match.group("samples").replace(",", "").replace(" ", "")
        try:
            samples = int(samples_text)
        except ValueError:
            diagnostics.add_skipped(
                line_number=0,
                reason="INVALID_SAMPLE_COUNT",
                message=f"Sample count is not an integer: {samples_text!r}",
                raw_line=title_text,
            )
            continue

        if samples <= 0:
            continue

        rects.append(_Rect(x=x, y=y, width=width, height=height, name=name, samples=samples))

    return rects


def _stacks_from_rects(rects: list[_Rect]) -> Counter[str]:
    if not rects:
        return Counter()

    heights = [r.height for r in rects if r.height > 0]
    row_height = min(heights) if heights else 16.0
    if row_height <= 0:
        row_height = 16.0

    def row_of(rect: _Rect) -> int:
        return round(rect.y / row_height)

    by_row: dict[int, list[_Rect]] = {}
    for rect in rects:
        by_row.setdefault(row_of(rect), []).append(rect)
    for bucket in by_row.values():
        bucket.sort(key=lambda r: r.x)

    widest = max(rects, key=lambda r: r.width)
    root_row = row_of(widest)

    rows_sorted = sorted(by_row.keys())
    if rows_sorted.index(root_row) == 0:
        # Icicle layout: root at top, leaves below (increasing y).
        toward_leaves_step = 1
    else:
        # Brendan default: root at bottom, leaves above (decreasing y).
        toward_leaves_step = -1
    toward_root_step = -toward_leaves_step

    leaves: list[_Rect] = []
    for rect in rects:
        next_rects = by_row.get(row_of(rect) + toward_leaves_step, [])
        has_descendant = any(
            not (nr.x + nr.width <= rect.x or nr.x >= rect.x + rect.width)
            for nr in next_rects
        )
        if not has_descendant:
            leaves.append(rect)

    stacks: Counter[str] = Counter()
    for leaf in leaves:
        path: list[str] = []
        current = leaf
        current_row = row_of(current)
        # Bound the walk by the maximum number of rows so a malformed SVG
        # never spirals into an infinite loop.
        for _ in range(len(rows_sorted) + 1):
            path.append(current.name)
            if current_row == root_row:
                break
            parent_row = current_row + toward_root_step
            parent_rects = by_row.get(parent_row, [])
            center = current.x + current.width / 2
            parent: _Rect | None = None
            for candidate in parent_rects:
                if candidate.x <= center <= candidate.x + candidate.width:
                    parent = candidate
                    break
            if parent is None:
                break
            current = parent
            current_row = parent_row

        if not path:
            continue
        path.reverse()
        stacks[";".join(path)] += leaf.samples

    return stacks
