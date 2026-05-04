"""Tests for Phase 7 lock-contention analysis (T-218 / T-219 / T-220 / T-221)."""
from __future__ import annotations

import textwrap
from pathlib import Path

import pytest

from archscope_engine.analyzers.lock_contention_analyzer import analyze_lock_contention
from archscope_engine.analyzers.multi_thread_analyzer import (
    analyze_multi_thread_dumps,
)
from archscope_engine.models.thread_snapshot import (
    LockHandle,
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump.java_jstack import JavaJstackParserPlugin


# ---------------------------------------------------------------------------
# T-218: LockHandle + new ThreadSnapshot fields
# ---------------------------------------------------------------------------


def test_thread_snapshot_lock_fields_default_to_empty() -> None:
    snap = ThreadSnapshot(
        snapshot_id="x",
        thread_name="t",
        thread_id=None,
        state=ThreadState.RUNNABLE,
        category="RUNNABLE",
    )
    assert snap.lock_holds == []
    assert snap.lock_waiting is None


def test_thread_snapshot_lock_fields_serialize_to_dict() -> None:
    handle = LockHandle("0x1", "com.example.Foo")
    snap = ThreadSnapshot(
        snapshot_id="x",
        thread_name="t",
        thread_id=None,
        state=ThreadState.LOCK_WAIT,
        category="LOCK_WAIT",
        lock_holds=[handle],
        lock_waiting=handle,
    )
    payload = snap.to_dict()
    assert payload["lock_holds"] == [{"lock_id": "0x1", "lock_class": "com.example.Foo"}]
    assert payload["lock_waiting"] == {"lock_id": "0x1", "lock_class": "com.example.Foo"}


# ---------------------------------------------------------------------------
# T-219: Java jstack lock-ID extraction
# ---------------------------------------------------------------------------


_DUMP_WITH_LOCKS = textwrap.dedent(
    """\
    Full thread dump OpenJDK 64-Bit Server VM (17.0.1+12 mixed mode):

    "owner" #1 prio=5 os_prio=0 tid=0x0001 nid=0x0001 runnable [0x00007f]
       java.lang.Thread.State: RUNNABLE
    \tat com.example.Owner.run(Owner.java:42)
    \t- locked <0x000000076ab62208> (a com.example.Pool)
    \t- locked <0x00000007aaaaaaa1> (a com.example.Outer)

    "waiter-1" #2 prio=5 os_prio=0 tid=0x0002 nid=0x0002 waiting for monitor entry
       java.lang.Thread.State: BLOCKED (on object monitor)
    \tat com.example.Waiter.run(Waiter.java:42)
    \t- waiting to lock <0x000000076ab62208> (a com.example.Pool)

    "waiter-2" #3 prio=5 os_prio=0 tid=0x0003 nid=0x0003 waiting on condition
       java.lang.Thread.State: WAITING (parking)
    \tat sun.misc.Unsafe.park(Native Method)
    \t- parking to wait for <0x00000007ccccccc1> (a java.util.concurrent.locks.AbstractQueuedSynchronizer$ConditionObject)

    "obj-waiter" #4 prio=5 os_prio=0 tid=0x0004 nid=0x0004 in Object.wait
       java.lang.Thread.State: TIMED_WAITING (on object monitor)
    \tat java.lang.Object.wait(Native Method)
    \t- waiting on <0x00000007dddddddd> (a java.lang.Object)
    """
)


def _bundle_from_dump(tmp_path: Path, body: str) -> ThreadDumpBundle:
    path = tmp_path / "dump.txt"
    path.write_text(body, encoding="utf-8")
    return JavaJstackParserPlugin().parse(path)


def test_java_parser_extracts_multiple_lock_holds(tmp_path: Path) -> None:
    bundle = _bundle_from_dump(tmp_path, _DUMP_WITH_LOCKS)
    by_name = {snap.thread_name: snap for snap in bundle.snapshots}
    owner = by_name["owner"]
    assert [hold.lock_id for hold in owner.lock_holds] == [
        "0x000000076ab62208",
        "0x00000007aaaaaaa1",
    ]
    assert owner.lock_holds[0].lock_class == "com.example.Pool"
    assert owner.lock_waiting is None


def test_java_parser_recognizes_waiting_to_lock(tmp_path: Path) -> None:
    bundle = _bundle_from_dump(tmp_path, _DUMP_WITH_LOCKS)
    waiter = next(s for s in bundle.snapshots if s.thread_name == "waiter-1")
    assert waiter.lock_waiting is not None
    assert waiter.lock_waiting.lock_id == "0x000000076ab62208"
    assert waiter.lock_waiting.lock_class == "com.example.Pool"
    # State already BLOCKED — analyzer keeps it.
    assert waiter.state is ThreadState.BLOCKED


def test_java_parser_recognizes_parking_to_wait_for(tmp_path: Path) -> None:
    bundle = _bundle_from_dump(tmp_path, _DUMP_WITH_LOCKS)
    parked = next(s for s in bundle.snapshots if s.thread_name == "waiter-2")
    assert parked.lock_waiting is not None
    assert parked.lock_waiting.lock_id == "0x00000007ccccccc1"


def test_java_parser_recognizes_waiting_on_object_wait(tmp_path: Path) -> None:
    bundle = _bundle_from_dump(tmp_path, _DUMP_WITH_LOCKS)
    waiter = next(s for s in bundle.snapshots if s.thread_name == "obj-waiter")
    assert waiter.lock_waiting is not None
    assert waiter.lock_waiting.lock_id == "0x00000007dddddddd"


# ---------------------------------------------------------------------------
# T-220: lock contention analyzer
# ---------------------------------------------------------------------------


def _java_snapshot(
    name: str,
    *,
    state: ThreadState = ThreadState.RUNNABLE,
    holds: list[LockHandle] | None = None,
    waiting: LockHandle | None = None,
    stack: list[str] | None = None,
) -> ThreadSnapshot:
    return ThreadSnapshot(
        snapshot_id=f"java::{name}",
        thread_name=name,
        thread_id=None,
        state=state,
        category=state.value,
        stack_frames=[
            StackFrame(function=frame, language="java") for frame in (stack or [])
        ],
        language="java",
        source_format="java_jstack",
        lock_holds=list(holds or []),
        lock_waiting=waiting,
    )


def _bundle(snapshots: list[ThreadSnapshot], dump_index: int = 0) -> ThreadDumpBundle:
    return ThreadDumpBundle(
        snapshots=snapshots,
        source_file=f"d-{dump_index}.txt",
        source_format="java_jstack",
        language="java",
        dump_index=dump_index,
        dump_label=f"d-{dump_index}",
    )


def test_lock_contention_hotspot_ranks_by_waiter_count() -> None:
    pool = LockHandle("0xPOOL", "com.example.Pool")
    other = LockHandle("0xOTHER", "com.example.Other")
    bundle = _bundle(
        [
            _java_snapshot("owner", holds=[pool, other]),
            _java_snapshot("w1", state=ThreadState.BLOCKED, waiting=pool),
            _java_snapshot("w2", state=ThreadState.BLOCKED, waiting=pool),
            _java_snapshot("w3", state=ThreadState.BLOCKED, waiting=pool),
        ]
    )
    result = analyze_lock_contention(bundle)
    assert result.summary["contended_locks"] == 1
    findings = [
        f for f in result.metadata["findings"] if f["code"] == "LOCK_CONTENTION_HOTSPOT"
    ]
    assert len(findings) == 1
    evidence = findings[0]["evidence"]
    assert evidence["lock_id"] == "0xPOOL"
    assert evidence["waiter_count"] == 3
    assert evidence["owner_thread"] == "owner"


def test_lock_contention_detects_two_thread_deadlock() -> None:
    a = LockHandle("0xA", "com.example.A")
    b = LockHandle("0xB", "com.example.B")
    # T1 holds A, waits for B; T2 holds B, waits for A.
    bundle = _bundle(
        [
            _java_snapshot("T1", holds=[a], waiting=b, state=ThreadState.BLOCKED),
            _java_snapshot("T2", holds=[b], waiting=a, state=ThreadState.BLOCKED),
        ]
    )
    result = analyze_lock_contention(bundle)
    deadlocks = [
        f for f in result.metadata["findings"] if f["code"] == "DEADLOCK_DETECTED"
    ]
    assert len(deadlocks) == 1
    chain = deadlocks[0]["evidence"]
    assert sorted(chain["threads"]) == ["T1", "T2"]
    assert result.summary["deadlocks_detected"] == 1


def test_lock_contention_handles_three_thread_cycle() -> None:
    a = LockHandle("0xA")
    b = LockHandle("0xB")
    c = LockHandle("0xC")
    bundle = _bundle(
        [
            _java_snapshot("T1", holds=[a], waiting=b, state=ThreadState.BLOCKED),
            _java_snapshot("T2", holds=[b], waiting=c, state=ThreadState.BLOCKED),
            _java_snapshot("T3", holds=[c], waiting=a, state=ThreadState.BLOCKED),
        ]
    )
    result = analyze_lock_contention(bundle)
    deadlocks = [
        f for f in result.metadata["findings"] if f["code"] == "DEADLOCK_DETECTED"
    ]
    assert len(deadlocks) == 1
    assert sorted(deadlocks[0]["evidence"]["threads"]) == ["T1", "T2", "T3"]


def test_lock_contention_no_findings_when_locks_uncontended() -> None:
    a = LockHandle("0xA")
    bundle = _bundle([_java_snapshot("solo", holds=[a])])
    result = analyze_lock_contention(bundle)
    assert result.summary["contended_locks"] == 0
    assert result.summary["deadlocks_detected"] == 0
    findings = [
        f
        for f in result.metadata["findings"]
        if f["code"] in {"LOCK_CONTENTION_HOTSPOT", "DEADLOCK_DETECTED"}
    ]
    assert findings == []


def test_lock_contention_requires_at_least_one_bundle() -> None:
    with pytest.raises(ValueError):
        analyze_lock_contention([])


def test_lock_contention_unions_multiple_bundles() -> None:
    a = LockHandle("0xA")
    bundles = [
        _bundle(
            [
                _java_snapshot("owner", holds=[a]),
                _java_snapshot("w1", state=ThreadState.BLOCKED, waiting=a),
            ],
            dump_index=0,
        ),
        _bundle(
            [_java_snapshot("w2", state=ThreadState.BLOCKED, waiting=a)],
            dump_index=1,
        ),
    ]
    result = analyze_lock_contention(bundles)
    assert result.summary["total_dumps"] == 2
    rows = result.tables["locks"]
    assert rows[0]["lock_id"] == "0xA"
    assert rows[0]["waiter_count"] == 2


# ---------------------------------------------------------------------------
# T-221: GROWING_LOCK_CONTENTION in multi_thread_analyzer
# ---------------------------------------------------------------------------


def test_multi_dump_growing_lock_contention_fires_when_waiters_increase() -> None:
    pool = LockHandle("0xPOOL")
    bundles = []
    for index, waiter_names in enumerate([["w1"], ["w1", "w2"], ["w1", "w2", "w3"]]):
        snapshots = [_java_snapshot("owner", holds=[pool])]
        for name in waiter_names:
            snapshots.append(
                _java_snapshot(name, state=ThreadState.BLOCKED, waiting=pool)
            )
        bundles.append(_bundle(snapshots, dump_index=index))
    result = analyze_multi_thread_dumps(bundles, threshold=3)
    findings = [
        f for f in result.metadata["findings"] if f["code"] == "GROWING_LOCK_CONTENTION"
    ]
    assert len(findings) == 1
    assert findings[0]["evidence"]["lock_id"] == "0xPOOL"
    assert findings[0]["evidence"]["max_waiters"] == 3
    assert result.summary["growing_lock_contention"] == 1


def test_multi_dump_growing_lock_does_not_fire_on_flat_waiter_count() -> None:
    pool = LockHandle("0xPOOL")
    bundles = [
        _bundle(
            [
                _java_snapshot("owner", holds=[pool]),
                _java_snapshot("w1", state=ThreadState.BLOCKED, waiting=pool),
            ],
            dump_index=index,
        )
        for index in range(3)
    ]
    result = analyze_multi_thread_dumps(bundles, threshold=3)
    findings = [
        f for f in result.metadata["findings"] if f["code"] == "GROWING_LOCK_CONTENTION"
    ]
    assert findings == []
    assert result.summary["growing_lock_contention"] == 0
