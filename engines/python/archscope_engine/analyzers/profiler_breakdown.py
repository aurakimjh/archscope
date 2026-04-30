from __future__ import annotations

from collections import Counter, defaultdict
from dataclasses import dataclass
from typing import Iterable

from archscope_engine.analyzers.flamegraph_builder import extract_leaf_paths
from archscope_engine.models.flamegraph import FlameNode

EXECUTIVE_LABELS = {
    "SQL_DATABASE": "SQL / DB",
    "EXTERNAL_API_HTTP": "External Call",
    "NETWORK_IO_WAIT": "Network / I/O Wait",
    "APPLICATION_LOGIC": "Application Method",
    "FRAMEWORK_MIDDLEWARE": "Application Method",
    "LOCK_SYNCHRONIZATION_WAIT": "Lock / Pool Wait",
    "CONNECTION_POOL_WAIT": "Lock / Pool Wait",
    "FILE_IO": "Network / I/O Wait",
    "GC_JVM_RUNTIME": "JVM / GC",
    "IDLE_BACKGROUND": "Others",
    "UNKNOWN": "Others",
}

RULES: list[tuple[str, tuple[str, ...]]] = [
    (
        "SQL_DATABASE",
        (
            "oracle.jdbc",
            "java.sql",
            "javax.sql",
            "T4CPreparedStatement",
            "T4CMAREngine",
            "executeQuery",
            "executeUpdate",
            "ResultSet",
            "com.mysql",
            "org.postgresql",
        ),
    ),
    (
        "EXTERNAL_API_HTTP",
        (
            "RestTemplate",
            "WebClient",
            "HttpClient",
            "OkHttp",
            "URLConnection",
            "InternalHttpClient",
            "MainClientExec",
            "BHttpConnection",
            "axios",
            "fetch",
            "requests.",
            "urllib",
            "net/http",
        ),
    ),
    (
        "NETWORK_IO_WAIT",
        (
            "SocketInputStream.socketRead",
            "NioSocketImpl",
            "SocketChannelImpl.read",
            "SocketDispatcher.read",
            "epollWait",
            "poll",
            "recv",
            "read0",
        ),
    ),
    (
        "CONNECTION_POOL_WAIT",
        (
            "HikariPool.getConnection",
            "ConcurrentBag",
            "SynchronousQueue",
            "BasicDataSource.getConnection",
            "DataSource.getConnection",
        ),
    ),
    (
        "LOCK_SYNCHRONIZATION_WAIT",
        (
            "LockSupport.park",
            "Unsafe.park",
            "Object.wait",
            "CountDownLatch.await",
            "Future.get",
            "ReentrantLock",
            "synchronized",
        ),
    ),
    (
        "FILE_IO",
        (
            "FileInputStream",
            "FileOutputStream",
            "Files.read",
            "Files.write",
            "RandomAccessFile",
            "FileChannel",
        ),
    ),
    (
        "GC_JVM_RUNTIME",
        (
            "GC",
            "G1",
            "ParallelGC",
            "VMThread",
            "CompilerThread",
            "Reference Handler",
            "Finalizer",
        ),
    ),
    (
        "IDLE_BACKGROUND",
        (
            "Thread.sleep",
            "TimerThread",
            "IdleConnectionEvictor",
            "ThreadPoolExecutor.getTask",
            "ScheduledThreadPoolExecutor",
        ),
    ),
]

PRIORITY = [
    "CONNECTION_POOL_WAIT",
    "LOCK_SYNCHRONIZATION_WAIT",
    "SQL_DATABASE",
    "EXTERNAL_API_HTTP",
    "NETWORK_IO_WAIT",
    "FILE_IO",
    "GC_JVM_RUNTIME",
    "IDLE_BACKGROUND",
    "APPLICATION_LOGIC",
    "UNKNOWN",
]


@dataclass(frozen=True)
class StackClassification:
    primary_category: str
    executive_label: str
    wait_reason: str | None
    matched_categories: list[str]


def classify_execution_stack(frames: Iterable[str]) -> StackClassification:
    frame_text = ";".join(frames)
    lower_stack = frame_text.casefold()
    matched: list[str] = []
    for category, tokens in RULES:
        if any(token.casefold() in lower_stack for token in tokens):
            matched.append(category)

    if not matched and _looks_like_application(frame_text):
        matched.append("APPLICATION_LOGIC")
    if not matched:
        matched.append("UNKNOWN")

    primary = min(matched, key=lambda category: PRIORITY.index(category))
    wait_reason = _wait_reason(matched, primary)
    return StackClassification(
        primary_category=primary,
        executive_label=EXECUTIVE_LABELS[primary],
        wait_reason=wait_reason,
        matched_categories=matched,
    )


def build_execution_breakdown(
    root: FlameNode,
    *,
    interval_ms: float,
    elapsed_sec: float | None,
    total_samples: int | None = None,
    parent_samples: int | None = None,
    top_n: int = 5,
) -> list[dict[str, object]]:
    total = total_samples or root.samples
    parent_total = parent_samples or root.samples
    interval_seconds = interval_ms / 1000
    category_samples: Counter[str] = Counter()
    category_methods: dict[str, Counter[str]] = defaultdict(Counter)
    category_stacks: dict[str, Counter[str]] = defaultdict(Counter)
    category_wait_reason: dict[str, Counter[str]] = defaultdict(Counter)

    for path, samples in extract_leaf_paths(root):
        classification = classify_execution_stack(path)
        category = classification.primary_category
        category_samples[category] += samples
        if path:
            category_methods[category][path[-1]] += samples
            category_stacks[category][";".join(path)] += samples
        if classification.wait_reason:
            category_wait_reason[category][classification.wait_reason] += samples

    rows = []
    for category, samples in category_samples.most_common():
        estimated_seconds = samples * interval_seconds
        rows.append(
            {
                "category": category,
                "executive_label": EXECUTIVE_LABELS[category],
                "primary_category": category,
                "wait_reason": _top_key(category_wait_reason[category]),
                "samples": samples,
                "estimated_seconds": round(estimated_seconds, 3),
                "total_ratio": round(samples / total * 100, 4) if total else 0.0,
                "parent_stage_ratio": round(samples / parent_total * 100, 4)
                if parent_total
                else 0.0,
                "elapsed_ratio": round(estimated_seconds / elapsed_sec * 100, 4)
                if elapsed_sec and elapsed_sec > 0
                else None,
                "top_methods": _top_counter(category_methods[category], top_n),
                "top_stacks": _top_counter(category_stacks[category], top_n),
            }
        )
    return rows


def _wait_reason(matched: list[str], primary: str) -> str | None:
    if primary == "EXTERNAL_API_HTTP" and "NETWORK_IO_WAIT" in matched:
        return "NETWORK_IO_WAIT"
    if primary == "SQL_DATABASE" and "NETWORK_IO_WAIT" in matched:
        return "NETWORK_IO_WAIT"
    if primary != "CONNECTION_POOL_WAIT" and "CONNECTION_POOL_WAIT" in matched:
        return "CONNECTION_POOL_WAIT"
    if primary != "LOCK_SYNCHRONIZATION_WAIT" and "LOCK_SYNCHRONIZATION_WAIT" in matched:
        return "LOCK_SYNCHRONIZATION_WAIT"
    return None


def _looks_like_application(stack: str) -> bool:
    lower = stack.casefold()
    return any(token in lower for token in ("com.", "org.", "service", "controller"))


def _top_key(counter: Counter[str]) -> str | None:
    if not counter:
        return None
    return counter.most_common(1)[0][0]


def _top_counter(counter: Counter[str], top_n: int) -> list[dict[str, int | str]]:
    return [
        {"name": name, "samples": samples}
        for name, samples in counter.most_common(top_n)
    ]
