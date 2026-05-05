"""Lock contention analyzer (T-220).

Consumes one or more :class:`ThreadDumpBundle` objects and produces an
``AnalysisResult`` of type ``thread_dump_locks`` describing:

* per-lock owner / waiter rows (``lock_id``, ``lock_class``, owner thread,
  waiter count, top waiter names, owner stack signature),
* a contention ranking sorted by waiter count
  (``LOCK_CONTENTION_HOTSPOT`` findings),
* a waits-for graph cycle scan with one ``DEADLOCK_DETECTED`` finding
  per cycle (DFS-based, deterministic ordering).

Java jstack today is the only parser that fills the
``lock_holds``/``lock_waiting`` fields, so this analyzer is JVM-driven
in practice. Other runtimes simply yield empty fields and the
contention table comes back empty — no special-casing required.
"""
from __future__ import annotations

from collections import Counter, defaultdict
from typing import Iterable, Sequence

from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.thread_snapshot import (
    LockHandle,
    ThreadDumpBundle,
    ThreadSnapshot,
)


def analyze_lock_contention(
    bundles: ThreadDumpBundle | Sequence[ThreadDumpBundle],
    *,
    top_n: int = 20,
) -> AnalysisResult:
    """Run lock-owner/waiter analysis over a single bundle or a list.

    Multi-bundle input simply unions the snapshots from every dump
    (each thread is treated as one observation). For per-dump trend
    analysis use :func:`analyze_multi_thread_dumps` instead.
    """
    if isinstance(bundles, ThreadDumpBundle):
        bundle_list: list[ThreadDumpBundle] = [bundles]
    else:
        bundle_list = list(bundles)
    if not bundle_list:
        raise ValueError("analyze_lock_contention requires at least one bundle.")

    snapshots: list[ThreadSnapshot] = []
    for bundle in bundle_list:
        snapshots.extend(bundle.snapshots)

    lock_class_by_id: dict[str, str | None] = {}
    owners_by_lock: dict[str, list[str]] = defaultdict(list)
    waiters_by_lock: dict[str, list[str]] = defaultdict(list)
    wait_modes_by_lock: dict[str, list[str]] = defaultdict(list)
    threads_with_locks = 0
    threads_waiting = 0

    for snapshot in snapshots:
        if snapshot.lock_holds:
            threads_with_locks += 1
        if snapshot.lock_waiting is not None:
            threads_waiting += 1
        for hold in snapshot.lock_holds:
            owners_by_lock[hold.lock_id].append(snapshot.thread_name)
            _remember_class(lock_class_by_id, hold)
        if snapshot.lock_waiting is not None:
            waiters_by_lock[snapshot.lock_waiting.lock_id].append(snapshot.thread_name)
            wait_modes_by_lock[snapshot.lock_waiting.lock_id].append(
                snapshot.lock_waiting.wait_mode or "unknown_lock_wait"
            )
            _remember_class(lock_class_by_id, snapshot.lock_waiting)

    contention_rows = _contention_table(
        owners_by_lock=owners_by_lock,
        waiters_by_lock=waiters_by_lock,
        wait_modes_by_lock=wait_modes_by_lock,
        lock_class_by_id=lock_class_by_id,
        snapshots_by_name={s.thread_name: s for s in snapshots},
    )
    hotspots = [
        row
        for row in contention_rows
        if row["waiter_count"] > 0 and row["contention_candidate"]
    ]
    deadlocks = _detect_deadlocks(snapshots)
    findings = _build_findings(hotspots[:top_n], deadlocks)

    languages = sorted({bundle.language for bundle in bundle_list})
    formats = sorted({bundle.source_format for bundle in bundle_list})

    summary: dict[str, object] = {
        "total_dumps": len(bundle_list),
        "total_thread_snapshots": len(snapshots),
        "threads_with_locks": threads_with_locks,
        "threads_waiting_on_lock": threads_waiting,
        "unique_locks": len(lock_class_by_id),
        "contended_locks": len(hotspots),
        "deadlocks_detected": len(deadlocks),
        "languages_detected": languages,
        "source_formats": formats,
    }
    series: dict[str, object] = {
        "contention_ranking": [
            {
                "lock_id": row["lock_id"],
                "lock_class": row["lock_class"],
                "waiter_count": row["waiter_count"],
            }
            for row in hotspots[:top_n]
        ],
    }
    tables: dict[str, object] = {
        "locks": contention_rows[:top_n],
        "deadlock_chains": deadlocks,
        "dumps": [
            {
                "dump_index": bundle.dump_index,
                "dump_label": bundle.dump_label,
                "source_file": bundle.source_file,
                "source_format": bundle.source_format,
                "language": bundle.language,
                "thread_count": len(bundle.snapshots),
            }
            for bundle in bundle_list
        ],
    }
    metadata: dict[str, object] = {
        "parser": "thread_dump_locks",
        "schema_version": "0.1.0",
        "diagnostics": {
            "total_lines": 0,
            "parsed_records": len(snapshots),
            "skipped_lines": 0,
            "skipped_by_reason": {},
            "samples": [],
        },
        "findings": findings,
    }
    return AnalysisResult(
        type="thread_dump_locks",
        source_files=[bundle.source_file for bundle in bundle_list],
        summary=summary,
        series=series,
        tables=tables,
        metadata=metadata,
    )


def _remember_class(
    lock_class_by_id: dict[str, str | None], lock: LockHandle
) -> None:
    existing = lock_class_by_id.get(lock.lock_id)
    if lock.lock_class and not existing:
        lock_class_by_id[lock.lock_id] = lock.lock_class
    elif lock.lock_id not in lock_class_by_id:
        lock_class_by_id[lock.lock_id] = lock.lock_class


def _contention_table(
    *,
    owners_by_lock: dict[str, list[str]],
    waiters_by_lock: dict[str, list[str]],
    wait_modes_by_lock: dict[str, list[str]],
    lock_class_by_id: dict[str, str | None],
    snapshots_by_name: dict[str, ThreadSnapshot],
) -> list[dict[str, object]]:
    rows: list[dict[str, object]] = []
    all_lock_ids: set[str] = (
        set(owners_by_lock) | set(waiters_by_lock) | set(lock_class_by_id)
    )
    for lock_id in all_lock_ids:
        owners = owners_by_lock.get(lock_id, [])
        waiters = waiters_by_lock.get(lock_id, [])
        wait_mode_counts = dict(Counter(wait_modes_by_lock.get(lock_id, [])))
        contention_candidate = any(
            mode not in {"object_wait", "parking_condition_wait"}
            for mode in wait_mode_counts
        )
        # Pick the most-frequent owner name in case the same lock is
        # reported by multiple snapshots (e.g. a multi-dump union).
        owner_name = (
            Counter(owners).most_common(1)[0][0] if owners else None
        )
        owner_snapshot = (
            snapshots_by_name.get(owner_name) if owner_name else None
        )
        owner_signature = (
            owner_snapshot.stack_signature() if owner_snapshot else None
        )
        rows.append(
            {
                "lock_id": lock_id,
                "lock_class": lock_class_by_id.get(lock_id),
                "owner_thread": owner_name,
                "owner_stack_signature": owner_signature,
                "owner_count": len(set(owners)),
                "waiter_count": len(set(waiters)),
                "wait_mode_counts": wait_mode_counts,
                "contention_candidate": contention_candidate,
                "top_waiters": _top_n_names(waiters, limit=5),
                "all_waiters": sorted(set(waiters)),
            }
        )
    rows.sort(
        key=lambda row: (
            -int(row["waiter_count"]),
            -int(row["owner_count"]),
            str(row["lock_id"]),
        )
    )
    return rows


def _top_n_names(names: Iterable[str], *, limit: int) -> list[str]:
    counter = Counter(names)
    return [name for name, _ in counter.most_common(limit)]


def _detect_deadlocks(snapshots: Iterable[ThreadSnapshot]) -> list[dict[str, object]]:
    """DFS the waits-for graph and report each simple cycle once.

    Builds an edge ``thread → owner_of_the_lock_it_is_waiting_on``
    directly so a 2-thread classic deadlock (T1 holds L1 wants L2,
    T2 holds L2 wants L1) shows up as ``T1 → T2 → T1``.
    """
    snapshots_list = list(snapshots)
    holders_by_lock: dict[str, str] = {}
    for snapshot in snapshots_list:
        for hold in snapshot.lock_holds:
            # First-seen hold wins; multi-dump unions can list the same
            # lock multiple times. Either way the owner thread name is
            # canonical because lock_id is unique per JVM.
            holders_by_lock.setdefault(hold.lock_id, snapshot.thread_name)

    waits_for: dict[str, str] = {}
    for snapshot in snapshots_list:
        if snapshot.lock_waiting is None:
            continue
        if snapshot.lock_waiting.wait_mode in {"object_wait", "parking_condition_wait"}:
            continue
        owner = holders_by_lock.get(snapshot.lock_waiting.lock_id)
        if owner and owner != snapshot.thread_name:
            waits_for[snapshot.thread_name] = owner

    # Cycle detection — colors: 0 unseen, 1 on-stack, 2 done.
    color: dict[str, int] = {}
    cycles: list[list[str]] = []
    seen_canonical: set[tuple[str, ...]] = set()

    def dfs(node: str, stack: list[str]) -> None:
        color[node] = 1
        stack.append(node)
        nxt = waits_for.get(node)
        if nxt is not None:
            if color.get(nxt) == 1:
                start = stack.index(nxt)
                cycle = stack[start:]
                canonical = _canonical_cycle(cycle)
                if canonical not in seen_canonical:
                    seen_canonical.add(canonical)
                    cycles.append(cycle)
            elif color.get(nxt, 0) == 0:
                dfs(nxt, stack)
        stack.pop()
        color[node] = 2

    for thread_name in sorted(waits_for.keys()):
        if color.get(thread_name, 0) == 0:
            dfs(thread_name, [])

    out: list[dict[str, object]] = []
    for cycle in cycles:
        edges: list[dict[str, str | None]] = []
        for index, thread in enumerate(cycle):
            target = cycle[(index + 1) % len(cycle)]
            # The edge label is the lock that `thread` is waiting on
            # — easiest to pull from the original snapshot.
            waiting = next(
                (
                    snap.lock_waiting
                    for snap in snapshots_list
                    if snap.thread_name == thread and snap.lock_waiting is not None
                ),
                None,
            )
            edges.append(
                {
                    "from_thread": thread,
                    "to_thread": target,
                    "lock_id": waiting.lock_id if waiting else None,
                    "lock_class": waiting.lock_class if waiting else None,
                }
            )
        out.append({"threads": cycle, "edges": edges})
    return out


def _canonical_cycle(cycle: list[str]) -> tuple[str, ...]:
    if not cycle:
        return ()
    rotations = [tuple(cycle[i:] + cycle[:i]) for i in range(len(cycle))]
    return min(rotations)


def _build_findings(
    hotspots: list[dict[str, object]],
    deadlocks: list[dict[str, object]],
) -> list[dict[str, object]]:
    findings: list[dict[str, object]] = []
    for row in hotspots:
        findings.append(
            {
                "severity": "warning",
                "code": "LOCK_CONTENTION_HOTSPOT",
                "message": (
                    f"Lock {row['lock_id']} "
                    f"({row.get('lock_class') or 'unknown class'}) "
                    f"has {row['waiter_count']} waiters; owner: "
                    f"{row['owner_thread'] or 'unknown'}."
                ),
                "evidence": row,
            }
        )
    for chain in deadlocks:
        thread_names = chain["threads"]
        if not isinstance(thread_names, list):  # pragma: no cover - defensive
            continue
        chain_text = " → ".join(list(thread_names) + [thread_names[0]])
        findings.append(
            {
                "severity": "critical",
                "code": "DEADLOCK_DETECTED",
                "message": f"Deadlock cycle: {chain_text}.",
                "evidence": chain,
            }
        )
    return findings
