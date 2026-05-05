from __future__ import annotations

from collections import Counter, defaultdict

from archscope_engine.analyzers.flamegraph_builder import iter_leaf_paths
from archscope_engine.analyzers.profiler_breakdown import classify_execution_stack
from archscope_engine.models.flamegraph import FlameNode

SEGMENT_ORDER = (
    "STARTUP_FRAMEWORK",
    "INTERNAL_METHOD",
    "SQL_EXECUTION",
    "DB_NETWORK_WAIT",
    "EXTERNAL_CALL",
    "EXTERNAL_NETWORK_WAIT",
    "CONNECTION_POOL_WAIT",
    "LOCK_SYNCHRONIZATION_WAIT",
    "NETWORK_IO_WAIT",
    "FILE_IO",
    "JVM_GC_RUNTIME",
    "UNKNOWN",
)

SEGMENT_LABELS = {
    "STARTUP_FRAMEWORK": "Startup / framework",
    "INTERNAL_METHOD": "Internal method",
    "SQL_EXECUTION": "SQL execution",
    "DB_NETWORK_WAIT": "DB network wait",
    "EXTERNAL_CALL": "External call",
    "EXTERNAL_NETWORK_WAIT": "External network wait",
    "CONNECTION_POOL_WAIT": "Connection pool wait",
    "LOCK_SYNCHRONIZATION_WAIT": "Lock / synchronization wait",
    "NETWORK_IO_WAIT": "Network / I/O wait",
    "FILE_IO": "File I/O",
    "JVM_GC_RUNTIME": "JVM / GC runtime",
    "UNKNOWN": "Unclassified",
}

STARTUP_TOKENS = (
    "springapplication.run",
    "joblauncher",
    "commandlinejobrunner",
    "simplejoblauncher",
    "batchapplication",
    "main(",
    ".main",
    "application.run",
)


def build_timeline_analysis(
    root: FlameNode,
    *,
    interval_ms: float,
    elapsed_sec: float | None,
    total_samples: int | None = None,
    top_n: int = 5,
    timeline_base_method: str | None = None,
) -> list[dict[str, object]]:
    rows, _scope = build_timeline_analysis_with_scope(
        root,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        total_samples=total_samples,
        top_n=top_n,
        timeline_base_method=timeline_base_method,
    )
    return rows


def build_timeline_analysis_with_scope(
    root: FlameNode,
    *,
    interval_ms: float,
    elapsed_sec: float | None,
    total_samples: int | None = None,
    top_n: int = 5,
    timeline_base_method: str | None = None,
) -> tuple[list[dict[str, object]], dict[str, object]]:
    base_method = _normalize_base_method(timeline_base_method)
    original_total = total_samples if total_samples is not None else root.samples
    interval_seconds = interval_ms / 1000
    segment_samples: Counter[str] = Counter()
    segment_methods: dict[str, Counter[str]] = defaultdict(Counter)
    segment_chains: dict[str, Counter[str]] = defaultdict(Counter)
    segment_stacks: dict[str, Counter[str]] = defaultdict(Counter)
    stage_total = 0 if base_method else root.samples

    for path, samples in iter_leaf_paths(root):
        scoped_path = _timeline_path_for_scope(path, base_method)
        if scoped_path is None:
            continue
        if base_method:
            stage_total += samples
        segment = _timeline_segment(scoped_path)
        segment_samples[segment] += samples
        if scoped_path:
            segment_methods[segment][_method_name(scoped_path)] += samples
            segment_chains[segment][_method_chain(scoped_path, segment)] += samples
            segment_stacks[segment][";".join(scoped_path)] += samples

    scope = _timeline_scope(
        base_method=base_method,
        stage_total=stage_total,
        original_total=original_total,
    )
    if base_method and stage_total <= 0:
        return [], scope

    rows: list[dict[str, object]] = []
    for index, segment in enumerate(SEGMENT_ORDER):
        samples = segment_samples.get(segment, 0)
        if samples <= 0:
            continue
        estimated_seconds = samples * interval_seconds
        rows.append(
            {
                "index": index,
                "segment": segment,
                "label": SEGMENT_LABELS[segment],
                "samples": samples,
                "estimated_seconds": round(estimated_seconds, 3),
                "stage_ratio": round(samples / stage_total * 100, 4)
                if stage_total
                else 0.0,
                "total_ratio": round(samples / original_total * 100, 4)
                if original_total
                else 0.0,
                "elapsed_ratio": round(estimated_seconds / elapsed_sec * 100, 4)
                if elapsed_sec and elapsed_sec > 0
                else None,
                "top_methods": _top_counter(segment_methods[segment], top_n),
                "method_chains": _top_chain_rows(segment_chains[segment], top_n),
                "top_stacks": _top_counter(segment_stacks[segment], top_n),
            }
        )
    return rows, scope


def _normalize_base_method(value: str | None) -> str | None:
    if value is None:
        return None
    stripped = value.strip()
    return stripped or None


def _timeline_path_for_scope(
    path: list[str],
    base_method: str | None,
) -> list[str] | None:
    if not base_method:
        return path
    matched_index = _base_method_index(path, base_method)
    if matched_index is None:
        return None
    return path[matched_index:]


def _base_method_index(path: list[str], base_method: str) -> int | None:
    needle = base_method.casefold()
    for index, frame in enumerate(path):
        if needle in frame.casefold():
            return index
    return None


def _timeline_scope(
    *,
    base_method: str | None,
    stage_total: int,
    original_total: int,
) -> dict[str, object]:
    warnings: list[dict[str, str]] = []
    if base_method and stage_total <= 0:
        warnings.append(
            {
                "code": "TIMELINE_BASE_METHOD_NOT_FOUND",
                "message": (
                    "No profiler stack matched the configured timeline base method."
                ),
            }
        )
    return {
        "mode": "base_method" if base_method else "full_profile",
        "base_method": base_method,
        "match_mode": "frame_contains_case_insensitive",
        "view_mode": "reroot_at_base_frame" if base_method else "preserve_full_path",
        "base_samples": stage_total,
        "total_samples": original_total,
        "base_ratio_of_total": round(stage_total / original_total * 100, 4)
        if original_total
        else None,
        "warnings": warnings,
    }


def _timeline_segment(path: list[str]) -> str:
    classification = classify_execution_stack(path)
    primary = classification.primary_category
    wait_reason = classification.wait_reason

    if primary == "SQL_DATABASE" and wait_reason == "NETWORK_IO_WAIT":
        return "DB_NETWORK_WAIT"
    if primary == "EXTERNAL_API_HTTP" and wait_reason == "NETWORK_IO_WAIT":
        return "EXTERNAL_NETWORK_WAIT"
    if primary == "SQL_DATABASE":
        return "SQL_EXECUTION"
    if primary == "EXTERNAL_API_HTTP":
        return "EXTERNAL_CALL"
    if primary == "CONNECTION_POOL_WAIT":
        return "CONNECTION_POOL_WAIT"
    if primary == "LOCK_SYNCHRONIZATION_WAIT":
        return "LOCK_SYNCHRONIZATION_WAIT"
    if primary == "NETWORK_IO_WAIT":
        return "NETWORK_IO_WAIT"
    if primary == "FILE_IO":
        return "FILE_IO"
    if primary == "GC_JVM_RUNTIME":
        return "JVM_GC_RUNTIME"
    if _looks_like_startup(path):
        return "STARTUP_FRAMEWORK"
    if primary in {"APPLICATION_LOGIC", "FRAMEWORK_MIDDLEWARE"}:
        return "INTERNAL_METHOD"
    return "UNKNOWN"


def _looks_like_startup(path: list[str]) -> bool:
    stack = ";".join(path).casefold()
    return any(token in stack for token in STARTUP_TOKENS)


def _method_name(path: list[str]) -> str:
    return path[-1] if path else "(no-frame)"


def _method_chain(path: list[str], segment: str) -> str:
    frames = _select_chain_frames(path, segment)
    return " -> ".join(frames) if frames else "(no-frame)"


def _select_chain_frames(path: list[str], segment: str) -> list[str]:
    if len(path) <= 6:
        return path
    if segment in {
        "SQL_EXECUTION",
        "DB_NETWORK_WAIT",
        "EXTERNAL_CALL",
        "EXTERNAL_NETWORK_WAIT",
        "CONNECTION_POOL_WAIT",
        "LOCK_SYNCHRONIZATION_WAIT",
    }:
        lower_tokens = _segment_tokens(segment)
        selected = [frame for frame in path if _matches_any(frame, lower_tokens)]
        if selected:
            return selected[:6]
    return path[-6:]


def _segment_tokens(segment: str) -> tuple[str, ...]:
    if segment in {"SQL_EXECUTION", "DB_NETWORK_WAIT"}:
        return (
            "oracle.jdbc",
            "java.sql",
            "t4cpreparedstatement",
            "t4cmarengine",
            "executequery",
            "executeupdate",
            "resultset",
            "socketinputstream.socketread",
            "niosocketimpl",
        )
    if segment in {"EXTERNAL_CALL", "EXTERNAL_NETWORK_WAIT"}:
        return (
            "resttemplate",
            "webclient",
            "httpclient",
            "okhttp",
            "urlconnection",
            "mainclientexec",
            "bhttpconnection",
            "socketinputstream.socketread",
            "niosocketimpl",
        )
    if segment == "CONNECTION_POOL_WAIT":
        return ("hikaripool.getconnection", "concurrentbag", "synchronousqueue")
    if segment == "LOCK_SYNCHRONIZATION_WAIT":
        return ("locksupport.park", "unsafe.park", "object.wait", "future.get")
    return ()


def _matches_any(frame: str, tokens: tuple[str, ...]) -> bool:
    lowered = frame.casefold()
    return any(token in lowered for token in tokens)


def _top_counter(counter: Counter[str], top_n: int) -> list[dict[str, int | str]]:
    return [
        {"name": name, "samples": samples}
        for name, samples in counter.most_common(top_n)
    ]


def _top_chain_rows(counter: Counter[str], top_n: int) -> list[dict[str, object]]:
    return [
        {
            "chain": chain,
            "samples": samples,
            "frames": chain.split(" -> ") if chain != "(no-frame)" else [],
        }
        for chain, samples in counter.most_common(top_n)
    ]
