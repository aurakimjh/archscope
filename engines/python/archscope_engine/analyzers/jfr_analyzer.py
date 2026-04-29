from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any

from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.parsers.jfr_parser import JfrEvent, parse_jfr_print_json

SCHEMA_VERSION = "0.1.0"
DEFAULT_EVENT_FILTERS = [
    "jdk.CPUTimeSample",
    "jdk.ExecutionSample",
    "jdk.GarbageCollection",
    "jdk.GCPhasePause",
    "jdk.JavaExceptionThrow",
    "jdk.ThreadPark",
    "jdk.SocketRead",
    "jdk.SocketWrite",
]


def analyze_jfr_print_json(path: Path, top_n: int = 20) -> AnalysisResult:
    events = parse_jfr_print_json(path)
    pause_events = [
        event
        for event in events
        if event["duration_ms"] is not None and event["duration_ms"] > 0
    ]
    notable_events = sorted(
        events,
        key=lambda event: event["duration_ms"] if event["duration_ms"] is not None else 0,
        reverse=True,
    )[:top_n]

    return AnalysisResult(
        type="jfr_recording",
        source_files=[str(path)],
        summary={
            "event_count": len(events),
            "duration_ms": _recording_duration_ms(events),
            "gc_pause_total_ms": round(_duration_sum_ms(_gc_events(events)), 3),
            "blocked_thread_events": sum(
                1
                for event in events
                if event["event_type"] in {"jdk.ThreadPark", "jdk.JavaMonitorEnter"}
            ),
        },
        series={
            "events_over_time": _events_over_time(events),
            "pause_events": [_to_pause_event(event) for event in pause_events],
        },
        tables={
            "notable_events": [
                _to_notable_event(event, index)
                for index, event in enumerate(notable_events, start=1)
            ]
        },
        metadata={
            "parser": "jdk_jfr_print_json",
            "schema_version": SCHEMA_VERSION,
            "jfr_command_version": "external",
            "event_filters": DEFAULT_EVENT_FILTERS,
            "poc": True,
        },
    )


def _recording_duration_ms(events: list[JfrEvent]) -> float:
    return round(_duration_sum_ms(events), 3)


def _duration_sum_ms(events: list[JfrEvent]) -> float:
    return sum(event["duration_ms"] or 0 for event in events)


def _gc_events(events: list[JfrEvent]) -> list[JfrEvent]:
    return [
        event
        for event in events
        if "GC" in event["event_type"] or "Garbage" in event["event_type"]
    ]


def _events_over_time(events: list[JfrEvent]) -> list[dict[str, Any]]:
    counts: Counter[tuple[str, str]] = Counter(
        (event["time"], event["event_type"]) for event in events
    )
    return [
        {"time": time, "event_type": event_type, "count": count}
        for (time, event_type), count in sorted(counts.items())
    ]


def _to_pause_event(event: JfrEvent) -> dict[str, Any]:
    return {
        "time": event["time"],
        "duration_ms": event["duration_ms"],
        "event_type": event["event_type"],
        "thread": event["thread"],
        "sampling_type": _sampling_type(event["event_type"]),
    }


def _to_notable_event(event: JfrEvent, index: int) -> dict[str, Any]:
    return {
        "time": event["time"],
        "event_type": event["event_type"],
        "duration_ms": event["duration_ms"],
        "thread": event["thread"],
        "message": event["message"],
        "frames": event["frames"],
        "sampling_type": _sampling_type(event["event_type"]),
        "evidence_ref": f"jfr:event:{index}",
        "raw_preview": event["raw_preview"],
    }


def _sampling_type(event_type: str) -> str:
    if event_type in {"jdk.CPUTimeSample", "jdk.ExecutionSample"}:
        return "sampled"
    return "exact"
