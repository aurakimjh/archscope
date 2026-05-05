"""Native memory allocation / leak analyzer.

Pairs the ``NativeMemoryAllocation`` and ``NativeMemoryFree`` events
emitted by async-profiler's ``nativemem`` mode and reports the call sites
that hold *unfreed* memory at the end of the recording — the canonical
fingerprint of a native memory leak.

Two output flavors are produced from the same event stream:

``leak``   — only allocations without a matching free, optionally
              ignoring allocations made in the last ``tail_ratio`` of
              the recording (defaults to 10% per async-profiler).

``alloc``  — all allocations regardless of free, useful for general
              hot-spot analysis of native allocations.

The flame tree counts ``size`` (bytes) when available, falling back to
``1`` per event when not. Engine returns flame ``samples`` in bytes so
the existing flamegraph renderer just works — display labels can show
units via ``metadata.unit``.
"""
from __future__ import annotations

from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.analyzers.flamegraph_builder import build_flame_tree_from_collapsed
from archscope_engine.parsers.jfr_parser import JfrEvent
from archscope_engine.parsers.jfr_recording import (
    JfrCliMissingError,
    parse_jfr_recording,
)

ALLOC_EVENT_TYPES = frozenset(
    {
        "jdk.NativeMemoryAllocation",
        "jdk.NativeAllocation",
    }
)
FREE_EVENT_TYPES = frozenset(
    {
        "jdk.NativeMemoryFree",
        "jdk.NativeFree",
    }
)


def analyze_native_memory(
    path: Path,
    *,
    leak_only: bool = True,
    tail_ratio: float = 0.10,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    """Analyze native memory events from a JFR recording.

    Parameters
    ----------
    leak_only:
        When True (default), report only allocations without a matching
        free. When False, report all allocations (hot-spot mode).
    tail_ratio:
        Allocations whose timestamp falls in the last ``tail_ratio`` of
        the recording are excluded from the leak set — they may simply
        not have had a chance to be freed yet. Set to 0 to disable.
    """
    diagnostics = ParserDiagnostics()
    try:
        events, source_info = parse_jfr_recording(
            path, diagnostics=diagnostics, debug_log=debug_log
        )
    except JfrCliMissingError as exc:
        raise ValueError(str(exc)) from exc

    allocs = [e for e in events if e["event_type"] in ALLOC_EVENT_TYPES]
    frees = [e for e in events if e["event_type"] in FREE_EVENT_TYPES]
    total_alloc_bytes = sum((e["size"] or 0) for e in allocs)
    total_free_bytes = sum((e["size"] or 0) for e in frees)

    freed_addresses = {e["address"] for e in frees if e["address"] is not None}

    cutoff = _tail_cutoff(allocs, tail_ratio) if tail_ratio > 0 else None

    leak_events: list[JfrEvent] = []
    for event in allocs:
        addr = event["address"]
        if leak_only:
            if addr is not None and addr in freed_addresses:
                continue
            if cutoff is not None:
                ts = _parse_event_time(event["time"])
                if ts is not None and ts > cutoff:
                    continue
        leak_events.append(event)

    leak_bytes = sum((e["size"] or 0) for e in leak_events)

    stacks: Counter[str] = Counter()
    for event in leak_events:
        if not event["frames"]:
            continue
        path_str = ";".join(event["frames"])
        # Bytes per event when available, otherwise count one event.
        weight = event["size"] if event["size"] and event["size"] > 0 else 1
        stacks[path_str] += weight

    flame = build_flame_tree_from_collapsed(stacks)

    # Top call sites (by bytes attributed to leaf path).
    top_sites = [
        {"stack": stack, "bytes": bytes_}
        for stack, bytes_ in stacks.most_common(50)
    ]

    return AnalysisResult(
        type="native_memory",
        source_files=[str(path)],
        summary={
            "alloc_event_count": len(allocs),
            "free_event_count": len(frees),
            "alloc_bytes_total": total_alloc_bytes,
            "free_bytes_total": total_free_bytes,
            "unfreed_event_count": len(leak_events),
            "unfreed_bytes_total": leak_bytes,
            "tail_ratio": tail_ratio,
            "tail_cutoff": cutoff.isoformat() if cutoff else None,
            "leak_only": leak_only,
        },
        series={},
        tables={"top_call_sites": top_sites},
        charts={"flamegraph": flame.to_dict()},
        metadata={
            "parser": "jdk_jfr_native_memory",
            "schema_version": "0.1.0",
            "unit": "bytes",
            "source_format": source_info.get("source_format"),
            "jfr_cli": source_info.get("jfr_cli"),
            "diagnostics": diagnostics.to_dict(),
        },
    )


def _tail_cutoff(allocs: list[JfrEvent], tail_ratio: float) -> datetime | None:
    times = [t for t in (_parse_event_time(e["time"]) for e in allocs) if t is not None]
    if not times:
        return None
    start = min(times)
    end = max(times)
    duration = (end - start).total_seconds()
    if duration <= 0 or tail_ratio <= 0:
        return None
    cutoff_seconds = duration * (1 - tail_ratio)
    from datetime import timedelta

    return start + timedelta(seconds=cutoff_seconds)


def _parse_event_time(value: str) -> datetime | None:
    if not value:
        return None
    try:
        dt = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt
