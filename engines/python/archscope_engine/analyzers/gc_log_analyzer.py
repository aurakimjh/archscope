from __future__ import annotations

from collections import Counter
from dataclasses import asdict
from pathlib import Path
from typing import Any

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.gc_event import GcEvent
from archscope_engine.parsers.gc_log_parser import parse_gc_log


def analyze_gc_log(path: Path, *, top_n: int = 20) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    events = parse_gc_log(path, diagnostics=diagnostics)
    return build_gc_log_result(
        events,
        source_file=path,
        diagnostics=diagnostics.to_dict(),
        top_n=top_n,
    )


def build_gc_log_result(
    events: list[GcEvent],
    *,
    source_file: Path,
    diagnostics: dict[str, Any],
    top_n: int = 20,
) -> AnalysisResult:
    pauses = [event.pause_ms for event in events if event.pause_ms is not None]
    total_pause = sum(pauses)
    type_counts = Counter(event.gc_type or "UNKNOWN" for event in events)
    cause_counts = Counter(event.cause or "UNKNOWN" for event in events)

    summary = {
        "total_events": len(events),
        "total_pause_ms": round(total_pause, 3),
        "avg_pause_ms": round(total_pause / len(pauses), 3) if pauses else 0.0,
        "max_pause_ms": round(max(pauses), 3) if pauses else 0.0,
        "young_gc_count": sum(
            count for gc_type, count in type_counts.items() if "Young" in gc_type
        ),
        "full_gc_count": sum(
            count for gc_type, count in type_counts.items() if "Full" in gc_type
        ),
    }
    series = {
        "pause_timeline": [
            {
                "time": _event_time(event, index),
                "value": round(event.pause_ms or 0.0, 3),
                "gc_type": event.gc_type or "UNKNOWN",
            }
            for index, event in enumerate(events)
        ],
        "heap_after_mb": [
            {
                "time": _event_time(event, index),
                "value": event.heap_after_mb,
            }
            for index, event in enumerate(events)
            if event.heap_after_mb is not None
        ],
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
        "events": [
            {
                **asdict(event),
                "timestamp": event.timestamp.isoformat() if event.timestamp else None,
            }
            for event in events[:top_n]
        ]
    }
    metadata = {
        "parser": "hotspot_unified_gc_log",
        "schema_version": "0.1.0",
        "diagnostics": diagnostics,
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
