from __future__ import annotations

from collections import Counter
from dataclasses import asdict
from pathlib import Path
from typing import Any, Iterable

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import detect_text_encoding
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.gc_event import GcEvent
from archscope_engine.parsers.gc_log_parser import (
    detect_gc_log_format,
    iter_gc_log_events_with_diagnostics,
)

_PARSER_NAMES = {
    "unified": "hotspot_unified_gc_log",
    "g1_legacy": "hotspot_g1_legacy_gc_log",
    "legacy": "hotspot_legacy_gc_log",
    "unknown": "hotspot_unified_gc_log",
}


def analyze_gc_log(
    path: Path,
    *,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    gc_format = detect_gc_log_format(path)
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    return build_gc_log_result(
        iter_gc_log_events_with_diagnostics(
            path,
            diagnostics=diagnostics,
            debug_log=debug_log,
        ),
        source_file=path,
        diagnostics=diagnostics,
        top_n=top_n,
        gc_format=gc_format,
    )


def build_gc_log_result(
    events: Iterable[GcEvent],
    *,
    source_file: Path,
    diagnostics: dict[str, Any] | ParserDiagnostics,
    top_n: int = 20,
    gc_format: str = "unknown",
) -> AnalysisResult:
    total_events = 0
    pause_count = 0
    total_pause = 0.0
    max_pause = 0.0
    type_counts: Counter[str] = Counter()
    cause_counts: Counter[str] = Counter()
    pause_timeline = []
    heap_after_mb = []
    event_rows = []

    for index, event in enumerate(events):
        total_events += 1
        type_counts[event.gc_type or "UNKNOWN"] += 1
        cause_counts[event.cause or "UNKNOWN"] += 1
        pause_ms = event.pause_ms
        if pause_ms is not None:
            pause_count += 1
            total_pause += pause_ms
            max_pause = max(max_pause, pause_ms)
        pause_timeline.append(
            {
                "time": _event_time(event, index),
                "value": round(pause_ms or 0.0, 3),
                "gc_type": event.gc_type or "UNKNOWN",
            }
        )
        if event.heap_after_mb is not None:
            heap_after_mb.append(
                {
                    "time": _event_time(event, index),
                    "value": event.heap_after_mb,
                }
            )
        if len(event_rows) < top_n:
            event_rows.append(
                {
                    **asdict(event),
                    "timestamp": event.timestamp.isoformat() if event.timestamp else None,
                }
            )

    summary = {
        "total_events": total_events,
        "total_pause_ms": round(total_pause, 3),
        "avg_pause_ms": round(total_pause / pause_count, 3) if pause_count else 0.0,
        "max_pause_ms": round(max_pause, 3) if pause_count else 0.0,
        "young_gc_count": sum(
            count for gc_type, count in type_counts.items() if "Young" in gc_type
        ),
        "full_gc_count": sum(
            count for gc_type, count in type_counts.items() if "Full" in gc_type
        ),
    }
    series = {
        "pause_timeline": pause_timeline,
        "heap_after_mb": heap_after_mb,
        "gc_type_breakdown": [
            {"gc_type": key, "count": value}
            for key, value in type_counts.most_common()
        ],
        "cause_breakdown": [
            {"cause": key, "count": value}
            for key, value in cause_counts.most_common()
        ],
    }
    tables = {
        "events": event_rows,
    }
    diagnostics_payload = (
        diagnostics.to_dict() if isinstance(diagnostics, ParserDiagnostics) else diagnostics
    )
    metadata = {
        "parser": _PARSER_NAMES.get(gc_format, "hotspot_unified_gc_log"),
        "gc_format": gc_format,
        "schema_version": "0.1.0",
        "diagnostics": diagnostics_payload,
        "findings": _build_findings(summary),
    }
    return AnalysisResult(
        type="gc_log",
        source_files=[str(source_file)],
        summary=summary,
        series=series,
        tables=tables,
        metadata=metadata,
    )


def _event_time(event: GcEvent, index: int) -> str:
    if event.timestamp is not None:
        return event.timestamp.isoformat()
    if event.uptime_sec is not None:
        return str(event.uptime_sec)
    return str(index)


def _build_findings(summary: dict[str, Any]) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    if summary["max_pause_ms"] >= 100:
        findings.append(
            {
                "severity": "warning",
                "code": "LONG_GC_PAUSE",
                "message": "One or more GC pauses exceeded 100 ms.",
                "evidence": {"max_pause_ms": summary["max_pause_ms"]},
            }
        )
    if summary["full_gc_count"] > 0:
        findings.append(
            {
                "severity": "warning",
                "code": "FULL_GC_PRESENT",
                "message": "Full GC activity was detected.",
                "evidence": {"full_gc_count": summary["full_gc_count"]},
            }
        )
    return findings
