from __future__ import annotations

import bisect
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

_HISTOGRAM_BUCKETS_MS: tuple[tuple[str, float, float], ...] = (
    ("<1ms", 0.0, 1.0),
    ("1-5ms", 1.0, 5.0),
    ("5-10ms", 5.0, 10.0),
    ("10-50ms", 10.0, 50.0),
    ("50-100ms", 50.0, 100.0),
    ("100-500ms", 100.0, 500.0),
    ("500ms-1s", 500.0, 1_000.0),
    ("1-5s", 1_000.0, 5_000.0),
    (">=5s", 5_000.0, float("inf")),
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
    pauses_ms: list[float] = []
    type_counts: Counter[str] = Counter()
    cause_counts: Counter[str] = Counter()
    pause_timeline = []
    heap_after_mb = []
    heap_before_mb = []
    young_after_mb = []
    allocation_rate_timeline: list[dict[str, Any]] = []
    promotion_rate_timeline: list[dict[str, Any]] = []
    allocation_rates: list[float] = []
    promotion_rates: list[float] = []
    event_rows = []
    first_uptime_sec: float | None = None
    last_uptime_sec: float | None = None
    prev_event: GcEvent | None = None
    humongous_count = 0
    cmf_count = 0
    promotion_failure_count = 0

    for index, event in enumerate(events):
        total_events += 1
        type_counts[event.gc_type or "UNKNOWN"] += 1
        cause_counts[event.cause or "UNKNOWN"] += 1
        pause_ms = event.pause_ms
        if pause_ms is not None:
            pause_count += 1
            total_pause += pause_ms
            max_pause = max(max_pause, pause_ms)
            pauses_ms.append(pause_ms)
        if event.uptime_sec is not None:
            if first_uptime_sec is None:
                first_uptime_sec = event.uptime_sec
            last_uptime_sec = event.uptime_sec
        time_label = _event_time(event, index)
        pause_timeline.append(
            {
                "time": time_label,
                "value": round(pause_ms or 0.0, 3),
                "gc_type": event.gc_type or "UNKNOWN",
            }
        )
        if event.heap_after_mb is not None:
            heap_after_mb.append({"time": time_label, "value": event.heap_after_mb})
        if event.heap_before_mb is not None:
            heap_before_mb.append({"time": time_label, "value": event.heap_before_mb})
        if event.young_after_mb is not None:
            young_after_mb.append({"time": time_label, "value": event.young_after_mb})

        alloc_rate = _allocation_rate_mb_per_sec(prev_event, event)
        if alloc_rate is not None:
            allocation_rate_timeline.append({"time": time_label, "value": alloc_rate})
            allocation_rates.append(alloc_rate)
        promo_rate = _promotion_rate_mb_per_sec(prev_event, event)
        if promo_rate is not None:
            promotion_rate_timeline.append({"time": time_label, "value": promo_rate})
            promotion_rates.append(promo_rate)

        if _is_humongous(event):
            humongous_count += 1
        if _is_concurrent_mode_failure(event):
            cmf_count += 1
        if _is_promotion_failure(event):
            promotion_failure_count += 1

        if len(event_rows) < top_n:
            event_rows.append(
                {
                    **asdict(event),
                    "timestamp": event.timestamp.isoformat() if event.timestamp else None,
                }
            )
        prev_event = event

    wall_time_sec = _compute_wall_time_sec(first_uptime_sec, last_uptime_sec)
    throughput_percent = _compute_throughput_percent(total_pause, wall_time_sec)
    p50, p95, p99 = _compute_percentiles(pauses_ms)

    summary = {
        "total_events": total_events,
        "total_pause_ms": round(total_pause, 3),
        "avg_pause_ms": round(total_pause / pause_count, 3) if pause_count else 0.0,
        "max_pause_ms": round(max_pause, 3) if pause_count else 0.0,
        "p50_pause_ms": p50,
        "p95_pause_ms": p95,
        "p99_pause_ms": p99,
        "throughput_percent": throughput_percent,
        "wall_time_sec": round(wall_time_sec, 3) if wall_time_sec is not None else 0.0,
        "young_gc_count": sum(
            count for gc_type, count in type_counts.items() if "Young" in gc_type
        ),
        "full_gc_count": sum(
            count for gc_type, count in type_counts.items() if "Full" in gc_type
        ),
        "avg_allocation_rate_mb_per_sec": _avg(allocation_rates),
        "avg_promotion_rate_mb_per_sec": _avg(promotion_rates),
        "humongous_allocation_count": humongous_count,
        "concurrent_mode_failure_count": cmf_count,
        "promotion_failure_count": promotion_failure_count,
    }
    series = {
        "pause_timeline": pause_timeline,
        "heap_after_mb": heap_after_mb,
        "heap_before_mb": heap_before_mb,
        "young_after_mb": young_after_mb,
        "pause_histogram": _build_pause_histogram(pauses_ms),
        "allocation_rate_mb_per_sec": allocation_rate_timeline,
        "promotion_rate_mb_per_sec": promotion_rate_timeline,
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


def _compute_wall_time_sec(
    first_uptime_sec: float | None, last_uptime_sec: float | None
) -> float | None:
    if first_uptime_sec is None or last_uptime_sec is None:
        return None
    span = last_uptime_sec - first_uptime_sec
    return span if span > 0 else None


def _compute_throughput_percent(total_pause_ms: float, wall_time_sec: float | None) -> float:
    if wall_time_sec is None or wall_time_sec <= 0:
        return 0.0
    pause_sec = total_pause_ms / 1000.0
    if pause_sec >= wall_time_sec:
        return 0.0
    return round((1.0 - pause_sec / wall_time_sec) * 100.0, 3)


def _compute_percentiles(pauses_ms: list[float]) -> tuple[float, float, float]:
    if not pauses_ms:
        return 0.0, 0.0, 0.0
    sorted_pauses = sorted(pauses_ms)
    return (
        round(_percentile(sorted_pauses, 50), 3),
        round(_percentile(sorted_pauses, 95), 3),
        round(_percentile(sorted_pauses, 99), 3),
    )


def _percentile(sorted_values: list[float], percent: float) -> float:
    if not sorted_values:
        return 0.0
    if len(sorted_values) == 1:
        return sorted_values[0]
    rank = (percent / 100.0) * (len(sorted_values) - 1)
    lo = int(rank)
    hi = min(lo + 1, len(sorted_values) - 1)
    weight = rank - lo
    return sorted_values[lo] * (1.0 - weight) + sorted_values[hi] * weight


def _avg(values: list[float]) -> float:
    if not values:
        return 0.0
    return round(sum(values) / len(values), 3)


def _allocation_rate_mb_per_sec(prev: GcEvent | None, curr: GcEvent) -> float | None:
    if prev is None:
        return None
    if prev.uptime_sec is None or curr.uptime_sec is None:
        return None
    dt = curr.uptime_sec - prev.uptime_sec
    if dt <= 0:
        return None
    if prev.heap_after_mb is None or curr.heap_before_mb is None:
        return None
    allocated = curr.heap_before_mb - prev.heap_after_mb
    if allocated < 0:
        return None
    return round(allocated / dt, 3)


def _promotion_rate_mb_per_sec(prev: GcEvent | None, curr: GcEvent) -> float | None:
    if prev is None:
        return None
    if prev.uptime_sec is None or curr.uptime_sec is None:
        return None
    dt = curr.uptime_sec - prev.uptime_sec
    if dt <= 0:
        return None
    prev_old = _old_gen_after_mb(prev)
    curr_old = _old_gen_after_mb(curr)
    if prev_old is None or curr_old is None:
        return None
    promoted = curr_old - prev_old
    if promoted < 0:
        return None
    return round(promoted / dt, 3)


def _old_gen_after_mb(event: GcEvent) -> float | None:
    if event.old_after_mb is not None:
        return event.old_after_mb
    if event.heap_after_mb is not None and event.young_after_mb is not None:
        return max(0.0, event.heap_after_mb - event.young_after_mb)
    return None


def _is_humongous(event: GcEvent) -> bool:
    cause = (event.cause or "").lower()
    raw = (event.raw_line or "").lower()
    return "humongous" in cause or "humongous" in raw


def _is_concurrent_mode_failure(event: GcEvent) -> bool:
    gc_type = (event.gc_type or "").lower()
    cause = (event.cause or "").lower()
    raw = (event.raw_line or "").lower()
    if "to-space exhausted" in gc_type or "to-space overflow" in gc_type:
        return True
    if "concurrent mode failure" in cause or "concurrent mode failure" in raw:
        return True
    return False


def _is_promotion_failure(event: GcEvent) -> bool:
    cause = (event.cause or "").lower()
    raw = (event.raw_line or "").lower()
    return "promotion failed" in cause or "promotion failed" in raw


def _build_pause_histogram(pauses_ms: list[float]) -> list[dict[str, Any]]:
    if not pauses_ms:
        return [
            {"bucket": label, "min_ms": lo, "max_ms": hi if hi != float("inf") else None, "count": 0}
            for label, lo, hi in _HISTOGRAM_BUCKETS_MS
        ]
    sorted_pauses = sorted(pauses_ms)
    buckets: list[dict[str, Any]] = []
    for label, lo, hi in _HISTOGRAM_BUCKETS_MS:
        left = bisect.bisect_left(sorted_pauses, lo)
        right = (
            len(sorted_pauses)
            if hi == float("inf")
            else bisect.bisect_left(sorted_pauses, hi)
        )
        buckets.append(
            {
                "bucket": label,
                "min_ms": lo,
                "max_ms": hi if hi != float("inf") else None,
                "count": max(0, right - left),
            }
        )
    return buckets


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
    throughput = summary.get("throughput_percent", 0.0)
    if summary.get("wall_time_sec", 0.0) > 0 and throughput < 95.0:
        findings.append(
            {
                "severity": "warning",
                "code": "LOW_GC_THROUGHPUT",
                "message": "GC throughput is below 95% — application is spending significant time in GC.",
                "evidence": {"throughput_percent": throughput},
            }
        )
    p99 = summary.get("p99_pause_ms", 0.0)
    if p99 >= 200.0:
        findings.append(
            {
                "severity": "warning",
                "code": "HIGH_P99_PAUSE",
                "message": "p99 GC pause exceeds 200 ms — latency-sensitive workloads may be impacted.",
                "evidence": {"p99_pause_ms": p99},
            }
        )
    humongous = summary.get("humongous_allocation_count", 0)
    if humongous > 0:
        findings.append(
            {
                "severity": "warning",
                "code": "HUMONGOUS_ALLOCATION",
                "message": (
                    "G1 humongous allocations detected. Consider increasing -XX:G1HeapRegionSize "
                    "or refactoring the allocation site to use smaller objects."
                ),
                "evidence": {"humongous_allocation_count": humongous},
            }
        )
    cmf = summary.get("concurrent_mode_failure_count", 0)
    if cmf > 0:
        findings.append(
            {
                "severity": "critical",
                "code": "CONCURRENT_MODE_FAILURE",
                "message": (
                    "Concurrent mode failure / to-space exhausted detected — "
                    "concurrent collector could not keep up with allocations, causing a Full GC."
                ),
                "evidence": {"concurrent_mode_failure_count": cmf},
            }
        )
    promotion_failure = summary.get("promotion_failure_count", 0)
    if promotion_failure > 0:
        findings.append(
            {
                "severity": "critical",
                "code": "PROMOTION_FAILURE",
                "message": (
                    "Promotion failure detected — old generation cannot accommodate promoted objects. "
                    "Increase old gen size or reduce allocation pressure."
                ),
                "evidence": {"promotion_failure_count": promotion_failure},
            }
        )
    return findings
