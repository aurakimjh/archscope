from __future__ import annotations

import re
from collections import Counter
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Iterable

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.parsers.jfr_parser import JfrEvent
from archscope_engine.parsers.jfr_recording import (
    JfrCliMissingError,
    parse_jfr_recording,
)

SCHEMA_VERSION = "0.2.0"

# ── Event taxonomy ──────────────────────────────────────────────────────────
# Each "mode" maps to the JFR event types that meaningfully belong to it.
# `wall` is intentionally identical to `cpu` — JFR records the same events,
# only the sampling rate / scope differs at capture time. `all` keeps every
# event so the legacy view is preserved.

MODE_EVENT_TYPES: dict[str, frozenset[str]] = {
    "cpu": frozenset(
        {
            "jdk.CPUTimeSample",
            "jdk.ExecutionSample",
            "jdk.NativeMethodSample",
            "jdk.MethodTrace",
        }
    ),
    "wall": frozenset(
        {
            "jdk.CPUTimeSample",
            "jdk.ExecutionSample",
            "jdk.NativeMethodSample",
            "jdk.MethodTrace",
        }
    ),
    "alloc": frozenset(
        {
            "jdk.ObjectAllocationInNewTLAB",
            "jdk.ObjectAllocationOutsideTLAB",
            "jdk.ObjectAllocationSample",
            "jdk.OldObjectSample",
            "jdk.PromoteObjectInNewPLAB",
            "jdk.PromoteObjectOutsidePLAB",
        }
    ),
    "lock": frozenset(
        {
            "jdk.JavaMonitorEnter",
            "jdk.JavaMonitorWait",
            "jdk.ThreadPark",
            "jdk.ThreadSleep",
        }
    ),
    "gc": frozenset(
        {
            "jdk.GarbageCollection",
            "jdk.GCPhasePause",
            "jdk.GCPhaseParallel",
            "jdk.GCPhaseConcurrent",
            "jdk.GCHeapSummary",
            "jdk.G1HeapSummary",
            "jdk.MetaspaceSummary",
            "jdk.SafepointBegin",
            "jdk.SafepointEnd",
        }
    ),
    "exception": frozenset(
        {
            "jdk.JavaExceptionThrow",
            "jdk.JavaErrorThrow",
        }
    ),
    "io": frozenset(
        {
            "jdk.SocketRead",
            "jdk.SocketWrite",
            "jdk.FileRead",
            "jdk.FileWrite",
        }
    ),
    "nativemem": frozenset(
        {
            "jdk.NativeMemoryAllocation",
            "jdk.NativeMemoryFree",
            "jdk.NativeAllocation",
            "jdk.NativeFree",
        }
    ),
}
"""Event-type sets per mode. ``all`` is implicit (no filter)."""


def supported_modes() -> list[str]:
    return ["all", *sorted(MODE_EVENT_TYPES.keys())]


# ── Public entry point ──────────────────────────────────────────────────────


def analyze_jfr_print_json(
    path: Path,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
    *,
    mode: str = "all",
    from_time: str | None = None,
    to_time: str | None = None,
    state: str | None = None,
    min_duration_ms: float | None = None,
) -> AnalysisResult:
    """Analyze a JFR recording (binary `.jfr` or `jfr print --json` output).

    The function name is preserved for backwards compatibility — it now
    transparently handles binary recordings via the JDK ``jfr`` CLI.

    Parameters
    ----------
    mode:
        One of ``all``, ``cpu``, ``wall``, ``alloc``, ``lock``, ``gc``,
        ``exception``, ``io``. ``all`` keeps every event (legacy behavior).
    from_time / to_time:
        Optional time-range filters. Accepts ISO 8601, ``HH:MM:SS``, or
        relative offsets (``+30s``, ``-2m``, ``500ms``) interpreted from the
        recording's start (``+``) or end (``-``) timestamp.
    """
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = "utf-8"

    try:
        events, source_info = parse_jfr_recording(
            path, diagnostics=diagnostics, debug_log=debug_log
        )
    except JfrCliMissingError as exc:
        # Surface as a regular ValueError so the FastAPI wrapper turns it
        # into a structured INVALID_OPTION rather than ENGINE_FAILED.
        raise ValueError(str(exc)) from exc

    # ── Apply filters ──────────────────────────────────────────────────────
    selected_mode = mode if mode in {"all", *MODE_EVENT_TYPES.keys()} else "all"
    available_modes = _detect_available_modes(events)
    available_states = _detect_available_states(events)
    mode_events = _filter_by_mode(events, selected_mode)
    window = _resolve_window(events, from_time=from_time, to_time=to_time)
    filtered_events = _filter_by_window(mode_events, window)
    if state:
        filtered_events = [
            event
            for event in filtered_events
            if event.get("state") and event["state"].upper() == state.upper()
        ]
    if min_duration_ms is not None and min_duration_ms > 0:
        filtered_events = [
            event
            for event in filtered_events
            if (event["duration_ms"] or 0) >= min_duration_ms
        ]

    pause_events = [
        event
        for event in filtered_events
        if event["duration_ms"] is not None and event["duration_ms"] > 0
    ]
    notable_events = sorted(
        filtered_events,
        key=lambda event: event["duration_ms"] if event["duration_ms"] is not None else 0,
        reverse=True,
    )[:top_n]

    return AnalysisResult(
        type="jfr_recording",
        source_files=[str(path)],
        summary={
            "event_count": len(filtered_events),
            "event_count_total": len(events),
            "duration_ms": _recording_duration_ms(filtered_events),
            "gc_pause_total_ms": round(_duration_sum_ms(_gc_events(filtered_events)), 3),
            "blocked_thread_events": sum(
                1
                for event in filtered_events
                if event["event_type"] in {"jdk.ThreadPark", "jdk.JavaMonitorEnter"}
            ),
            "selected_mode": selected_mode,
        },
        series={
            "events_over_time": _events_over_time(filtered_events),
            "pause_events": [_to_pause_event(event) for event in pause_events],
            "events_by_type": _events_by_type(filtered_events),
            "heatmap_strip": _build_heatmap_strip(filtered_events),
        },
        tables={
            "notable_events": [
                _to_notable_event(event, index)
                for index, event in enumerate(notable_events, start=1)
            ]
        },
        metadata={
            "parser": "jdk_jfr_recording",
            "schema_version": SCHEMA_VERSION,
            "jfr_command_version": "external",
            "selected_mode": selected_mode,
            "available_modes": available_modes,
            "supported_modes": supported_modes(),
            "available_states": available_states,
            "selected_state": state,
            "min_duration_ms": min_duration_ms,
            "filter_window": _window_to_dict(window),
            "source_format": source_info.get("source_format"),
            "jfr_cli": source_info.get("jfr_cli"),
            "diagnostics": diagnostics.to_dict(),
        },
    )


def _detect_available_states(events: Iterable[JfrEvent]) -> list[str]:
    seen: set[str] = set()
    for event in events:
        s = event.get("state")
        if isinstance(s, str) and s:
            seen.add(s)
    return sorted(seen)


# ── Mode filter ─────────────────────────────────────────────────────────────


def _filter_by_mode(events: list[JfrEvent], mode: str) -> list[JfrEvent]:
    if mode == "all":
        return events
    allowed = MODE_EVENT_TYPES.get(mode)
    if not allowed:
        return events
    return [event for event in events if event["event_type"] in allowed]


def _detect_available_modes(events: Iterable[JfrEvent]) -> list[str]:
    """Return the modes that have at least one event in this recording."""
    seen_types = {event["event_type"] for event in events}
    available = ["all"] if seen_types else []
    for mode, types in MODE_EVENT_TYPES.items():
        if seen_types & types:
            available.append(mode)
    return available


# ── Time-range filter ───────────────────────────────────────────────────────


_RELATIVE_RE = re.compile(r"^([+-]?)(\d+(?:\.\d+)?)(ms|s|m|h)$")
_HMS_RE = re.compile(r"^(\d{1,2}):(\d{2})(?::(\d{2}(?:\.\d+)?))?$")


class _Window:
    __slots__ = ("start", "end", "from_input", "to_input")

    def __init__(
        self,
        start: datetime | None,
        end: datetime | None,
        from_input: str | None,
        to_input: str | None,
    ) -> None:
        self.start = start
        self.end = end
        self.from_input = from_input
        self.to_input = to_input


def _window_to_dict(window: _Window) -> dict[str, Any]:
    return {
        "from": window.from_input,
        "to": window.to_input,
        "effective_start": window.start.isoformat() if window.start else None,
        "effective_end": window.end.isoformat() if window.end else None,
    }


def _resolve_window(
    events: list[JfrEvent], *, from_time: str | None, to_time: str | None
) -> _Window:
    if from_time is None and to_time is None:
        return _Window(None, None, None, None)

    recording_start, recording_end = _recording_bounds(events)
    start = _parse_time_input(from_time, recording_start, recording_end)
    end = _parse_time_input(to_time, recording_start, recording_end)
    return _Window(start, end, from_time, to_time)


def _recording_bounds(
    events: list[JfrEvent],
) -> tuple[datetime | None, datetime | None]:
    times = [t for t in (_parse_event_time(e["time"]) for e in events) if t is not None]
    if not times:
        return None, None
    return min(times), max(times)


def _parse_time_input(
    raw: str | None,
    recording_start: datetime | None,
    recording_end: datetime | None,
) -> datetime | None:
    if raw is None or raw == "":
        return None
    text = raw.strip()

    # Relative offsets (e.g. +30s, -2m, 500ms). Prefix '-' = relative to end.
    rel_match = _RELATIVE_RE.match(text)
    if rel_match:
        sign, value, unit = rel_match.groups()
        delta_ms = float(value) * {"ms": 1, "s": 1_000, "m": 60_000, "h": 3_600_000}[unit]
        delta = timedelta(milliseconds=delta_ms)
        if sign == "-":
            return (recording_end or datetime.now(timezone.utc)) - delta
        return (recording_start or datetime.now(timezone.utc)) + delta

    # ISO 8601 first.
    try:
        return _ensure_aware(datetime.fromisoformat(text.replace("Z", "+00:00")))
    except ValueError:
        pass

    # HH:MM[:SS[.fff]] — anchored to the recording's date.
    hms = _HMS_RE.match(text)
    if hms and recording_start is not None:
        hour = int(hms.group(1))
        minute = int(hms.group(2))
        sec_text = hms.group(3) or "0"
        sec_int = int(float(sec_text))
        microseconds = int((float(sec_text) - sec_int) * 1_000_000)
        return recording_start.replace(
            hour=hour, minute=minute, second=sec_int, microsecond=microseconds
        )

    return None


def _ensure_aware(dt: datetime) -> datetime:
    if dt.tzinfo is None:
        return dt.replace(tzinfo=timezone.utc)
    return dt


def _parse_event_time(value: str) -> datetime | None:
    if not value:
        return None
    try:
        return _ensure_aware(datetime.fromisoformat(value.replace("Z", "+00:00")))
    except ValueError:
        return None


def _filter_by_window(events: list[JfrEvent], window: _Window) -> list[JfrEvent]:
    if window.start is None and window.end is None:
        return events
    out: list[JfrEvent] = []
    for event in events:
        ts = _parse_event_time(event["time"])
        if ts is None:
            # Keep events without parseable timestamps to avoid silently
            # dropping data when the JFR JSON omits absolute times.
            out.append(event)
            continue
        if window.start and ts < window.start:
            continue
        if window.end and ts > window.end:
            continue
        out.append(event)
    return out


# ── Series helpers ──────────────────────────────────────────────────────────


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


def _build_heatmap_strip(events: list[JfrEvent]) -> dict[str, Any]:
    """Bucket the (filtered) events by wall-clock time so the frontend can
    render a 1D density strip and let the user drag-select a sub-range.

    Bucket size adapts to the recording duration so the strip stays readable
    regardless of whether the recording was 10 seconds or 1 hour long.
    """
    times: list[datetime] = []
    for event in events:
        ts = _parse_event_time(event["time"])
        if ts is not None:
            times.append(ts)
    if not times:
        return {
            "bucket_seconds": 0.0,
            "start_time": None,
            "end_time": None,
            "max_count": 0,
            "buckets": [],
        }

    start = min(times)
    end = max(times)
    duration_s = max(0.001, (end - start).total_seconds())

    if duration_s <= 30:
        bucket_s = 0.5
    elif duration_s <= 120:
        bucket_s = 1.0
    elif duration_s <= 600:
        bucket_s = 5.0
    elif duration_s <= 3600:
        bucket_s = 30.0
    elif duration_s <= 86_400:
        bucket_s = 300.0
    else:
        bucket_s = 1_800.0

    bucket_count = max(1, int(duration_s / bucket_s) + 1)
    counts = [0] * bucket_count

    start_ts = start.timestamp()
    for ts in times:
        idx = int((ts.timestamp() - start_ts) / bucket_s)
        if idx < 0:
            idx = 0
        if idx >= bucket_count:
            idx = bucket_count - 1
        counts[idx] += 1

    buckets = []
    for i, count in enumerate(counts):
        bucket_start = start + timedelta(seconds=bucket_s * i)
        buckets.append(
            {
                "index": i,
                "time": bucket_start.isoformat(),
                "count": count,
            }
        )

    return {
        "bucket_seconds": bucket_s,
        "start_time": start.isoformat(),
        "end_time": end.isoformat(),
        "max_count": max(counts) if counts else 0,
        "buckets": buckets,
    }


def _events_over_time(events: list[JfrEvent]) -> list[dict[str, Any]]:
    counts: Counter[tuple[str, str]] = Counter(
        (event["time"], event["event_type"]) for event in events
    )
    return [
        {"time": time, "event_type": event_type, "count": count}
        for (time, event_type), count in sorted(counts.items())
    ]


def _events_by_type(events: list[JfrEvent]) -> list[dict[str, Any]]:
    counts: Counter[str] = Counter(event["event_type"] for event in events)
    return [
        {"event_type": event_type, "count": count}
        for event_type, count in counts.most_common()
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
