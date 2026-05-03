from __future__ import annotations

import re
from pathlib import Path
from typing import Iterable

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import (
    TextLineContext,
    iter_text_lines,
    iter_text_lines_with_context,
)
from archscope_engine.models.gc_event import GcEvent


UNIFIED_GC_RE = re.compile(
    r"^\[(?P<timestamp>[^\]]+)\].*?GC\(\d+\)\s+"
    r"(?P<label>.*?)\s+"
    r"(?P<before>\d+(?:\.\d+)?)(?P<before_unit>[KMG])->"
    r"(?P<after>\d+(?:\.\d+)?)(?P<after_unit>[KMG])"
    r"(?:\((?P<committed>\d+(?:\.\d+)?)(?P<committed_unit>[KMG])\))?\s+"
    r"(?P<pause>\d+(?:\.\d+)?)ms"
)


def parse_gc_log(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
    debug_log: DebugLogCollector | None = None,
) -> list[GcEvent]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    return list(
        iter_gc_log_events_with_diagnostics(
            path,
            diagnostics=own_diagnostics,
            debug_log=debug_log,
        )
    )


def iter_gc_log_events_with_diagnostics(
    path: Path,
    *,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None = None,
) -> Iterable[GcEvent]:
    line_iterable = (
        iter_text_lines_with_context(path)
        if debug_log is not None
        else _line_contexts_without_neighbors(path)
    )
    for context in line_iterable:
        line_number = context.line_number
        line = context.target
        diagnostics.total_lines += 1
        if not line.strip():
            continue

        event = _parse_unified_gc_line(line)
        if event is None:
            diagnostics.add_skipped(
                line_number=line_number,
                reason="NO_GC_FORMAT_MATCH",
                message="Line did not match the supported HotSpot unified GC format.",
                raw_line=line,
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=line_number,
                    reason="NO_GC_FORMAT_MATCH",
                    message="Line did not match the supported HotSpot unified GC format.",
                    raw_context={
                        "before": context.before,
                        "target": line,
                        "after": context.after,
                    },
                    failed_pattern="HOTSPOT_UNIFIED_GC_PAUSE",
                    field_shapes=infer_field_shapes(line),
                )
            continue

        yield event
        diagnostics.parsed_records += 1


def _parse_unified_gc_line(line: str) -> GcEvent | None:
    match = UNIFIED_GC_RE.search(line)
    if match is None:
        return None

    label = match.group("label").strip()
    gc_type, cause = _split_label(label)

    return GcEvent(
        timestamp=_parse_timestamp(match.group("timestamp")),
        uptime_sec=None,
        gc_type=gc_type,
        cause=cause,
        pause_ms=float(match.group("pause")),
        heap_before_mb=_to_mb(match.group("before"), match.group("before_unit")),
        heap_after_mb=_to_mb(match.group("after"), match.group("after_unit")),
        heap_committed_mb=(
            _to_mb(match.group("committed"), match.group("committed_unit"))
            if match.group("committed")
            else None
        ),
        young_before_mb=None,
        young_after_mb=None,
        old_before_mb=None,
        old_after_mb=None,
        metaspace_before_mb=None,
        metaspace_after_mb=None,
        raw_line=line,
    )


def _split_label(label: str) -> tuple[str, str | None]:
    gc_type = label.split(" (", 1)[0].strip()
    start = label.rfind(" (")
    if start == -1 or not label.endswith(")"):
        return gc_type or label, None
    return gc_type or label, label[start + 2 : -1]


def _parse_timestamp(value: str):
    from datetime import datetime

    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None


def _to_mb(value: str | None, unit: str | None) -> float | None:
    if value is None or unit is None:
        return None

    numeric = float(value)
    if unit == "K":
        return round(numeric / 1024, 3)
    if unit == "G":
        return round(numeric * 1024, 3)
    return numeric


def _line_contexts_without_neighbors(path: Path):
    for line_number, line in enumerate(iter_text_lines(path), start=1):
        yield TextLineContext(
            line_number=line_number,
            before=None,
            target=line,
            after=None,
        )
