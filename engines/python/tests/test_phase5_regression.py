"""End-to-end Phase 5 regression coverage (T-203 / T-204).

This module exercises the multi-language thread-dump framework as a
*system* — registry auto-detection across all six format signatures,
multi-dump correlation findings (LONG_RUNNING / PERSISTENT_BLOCKED /
LATENCY_SECTION_DETECTED), and the mixed-language guardrail. Each
language is represented by a small but realistic 3-dump fixture so the
correlator's threshold (≥3 consecutive dumps) actually fires.
"""
from __future__ import annotations

import json
import textwrap
from pathlib import Path

import pytest

from archscope_engine.analyzers.multi_thread_analyzer import (
    analyze_multi_thread_dumps,
)
from archscope_engine.models.thread_snapshot import (
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump import (
    DEFAULT_REGISTRY,
    MixedFormatError,
)


# ---------------------------------------------------------------------------
# Fixtures — one 3-dump bundle per language
# ---------------------------------------------------------------------------


def _write_three(tmp_path: Path, name: str, body: str) -> list[Path]:
    out: list[Path] = []
    for index in range(3):
        path = tmp_path / f"{name}-{index}.txt"
        path.write_text(body, encoding="utf-8")
        out.append(path)
    return out


JAVA_DUMP = textwrap.dedent(
    """\
    Full thread dump OpenJDK 64-Bit Server VM (17.0.1+12 mixed mode):

    "http-nio-8080-exec-1" #25 prio=5 os_prio=0 tid=0x00007f001 nid=0x00aa runnable [0x00007f]
       java.lang.Thread.State: RUNNABLE
    \tat sun.nio.ch.EPoll.epollWait(EPoll.java:42)
    \tat com.example.PaymentController.handle(PaymentController.java:12)

    "blocked-worker" #26 prio=5 os_prio=0 tid=0x00007f002 nid=0x00ab waiting for monitor entry
       java.lang.Thread.State: BLOCKED (on object monitor)
    \tat com.example.LockyService.run(LockyService.java:88)
    """
)

GO_DUMP = textwrap.dedent(
    """\
    goroutine 1 [running]:
    main.main()
    \t/app/main.go:42 +0x1a

    goroutine 5 [chan receive, 5 minutes]:
    main.worker(0xc0000a8000)
    \t/app/main.go:88 +0x55
    """
)

PYSPY_DUMP = textwrap.dedent(
    """\
    Process 12345: python /app/server.py
    Python v3.11.7

    Thread 12345 (active): "MainThread"
        recv (socket.py:660)
        handle (server.py:120)
    """
)

FAULTHANDLER_DUMP = textwrap.dedent(
    """\
    Fatal Python error: SIGTERM

    Thread 0x00007f1234567890 (most recent call first):
      File "/usr/lib/python3.11/threading.py", line 320 in wait
      File "/app/worker.py", line 55 in run
    """
)

NODEJS_REPORT_BASE: dict[str, object] = {
    "header": {"event": "JavaScript API", "nodejsVersion": "v20.10.0"},
    "javascriptStack": {
        "message": "Diagnostic report on demand",
        "stack": [
            "    at handler (/app/server.js:42:5)",
            "    at /app/server.js:88:3",
        ],
    },
    "nativeStack": [{"pc": "0x1", "symbol": "uv_run"}],
    "libuv": [{"type": "tcp", "is_active": True}],
}

DOTNET_DUMP = textwrap.dedent(
    """\
    OS Thread Id: 0x1a4 (1)
            Child SP               IP Call Site
    000000A1B2C3D4E0 00007FF812345678 System.Threading.Monitor.Enter(System.Object)
    000000A1B2C3D4F8 00007FF812345abc MyApp.Service.Process(MyApp.Request)
    """
)


# ---------------------------------------------------------------------------
# Format auto-detection across all six plugins (T-204)
# ---------------------------------------------------------------------------


def _parse_via_registry(path: Path, dump_index: int) -> ThreadDumpBundle:
    return DEFAULT_REGISTRY.parse_one(path, dump_index=dump_index)


@pytest.mark.parametrize(
    ("body", "expected_format"),
    [
        (JAVA_DUMP, "java_jstack"),
        (GO_DUMP, "go_goroutine"),
        (PYSPY_DUMP, "python_pyspy"),
        (FAULTHANDLER_DUMP, "python_faulthandler"),
        (json.dumps(NODEJS_REPORT_BASE), "nodejs_diagnostic_report"),
        (DOTNET_DUMP, "dotnet_clrstack"),
    ],
)
def test_each_language_routes_to_correct_plugin(
    tmp_path: Path, body: str, expected_format: str
) -> None:
    fixture = tmp_path / f"{expected_format}.txt"
    fixture.write_text(body, encoding="utf-8")
    bundle = _parse_via_registry(fixture, 0)
    assert bundle.source_format == expected_format
    assert bundle.snapshots, "parser should emit at least one snapshot"


# ---------------------------------------------------------------------------
# Mixed-language guard rail (T-189 / T-204)
# ---------------------------------------------------------------------------


def test_registry_rejects_mixing_java_and_go(tmp_path: Path) -> None:
    java = tmp_path / "java.txt"
    java.write_text(JAVA_DUMP, encoding="utf-8")
    go = tmp_path / "go.txt"
    go.write_text(GO_DUMP, encoding="utf-8")
    with pytest.raises(MixedFormatError):
        DEFAULT_REGISTRY.parse_many([java, go])


def test_registry_format_override_lets_operator_force_one_plugin(
    tmp_path: Path,
) -> None:
    java = tmp_path / "java.txt"
    java.write_text(JAVA_DUMP, encoding="utf-8")
    second = tmp_path / "java2.txt"
    # Even a headerless JVM dump should now parse via the override.
    second.write_text(JAVA_DUMP.split("Full thread dump")[1], encoding="utf-8")
    bundles = DEFAULT_REGISTRY.parse_many(
        [java, second], format_override="java_jstack"
    )
    assert {b.source_format for b in bundles} == {"java_jstack"}


# ---------------------------------------------------------------------------
# T-191: LONG_RUNNING / PERSISTENT_BLOCKED across 3 dumps per language
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("name", "body", "format_id"),
    [
        ("java", JAVA_DUMP, "java_jstack"),
        ("go", GO_DUMP, "go_goroutine"),
        ("pyspy", PYSPY_DUMP, "python_pyspy"),
        ("faulthandler", FAULTHANDLER_DUMP, "python_faulthandler"),
        ("nodejs", json.dumps(NODEJS_REPORT_BASE), "nodejs_diagnostic_report"),
        ("dotnet", DOTNET_DUMP, "dotnet_clrstack"),
    ],
)
def test_each_language_3_dump_correlation_emits_findings(
    tmp_path: Path, name: str, body: str, format_id: str
) -> None:
    paths = _write_three(tmp_path, name, body)
    bundles = DEFAULT_REGISTRY.parse_many(paths)
    assert {b.source_format for b in bundles} == {format_id}

    result = analyze_multi_thread_dumps(bundles)
    assert result.summary["total_dumps"] == 3
    assert result.summary["unique_threads"] >= 1
    # Every fixture has at least one thread in a wait category that
    # repeats across all three dumps, so we expect *some* finding.
    assert (
        result.summary["long_running_threads"]
        + result.summary["persistent_blocked_threads"]
        + result.summary["latency_sections"]
    ) > 0


def test_java_3_dump_run_emits_persistent_blocked_and_network_latency(
    tmp_path: Path,
) -> None:
    paths = _write_three(tmp_path, "java", JAVA_DUMP)
    bundles = DEFAULT_REGISTRY.parse_many(paths)
    result = analyze_multi_thread_dumps(bundles)

    codes = [f["code"] for f in result.metadata["findings"]]
    # The blocked-worker thread is BLOCKED in all 3 dumps → critical finding.
    assert "PERSISTENT_BLOCKED_THREAD" in codes
    # The Tomcat exec thread sits in EPoll.epollWait → enriched to
    # NETWORK_WAIT → latency finding.
    latency = [
        f for f in result.metadata["findings"] if f["code"] == "LATENCY_SECTION_DETECTED"
    ]
    assert any(
        finding["evidence"]["wait_category"] == "NETWORK_WAIT"
        for finding in latency
    )


def test_go_3_dump_run_emits_channel_wait_latency(tmp_path: Path) -> None:
    paths = _write_three(tmp_path, "go", GO_DUMP)
    bundles = DEFAULT_REGISTRY.parse_many(paths)
    result = analyze_multi_thread_dumps(bundles)

    latency = [
        f for f in result.metadata["findings"] if f["code"] == "LATENCY_SECTION_DETECTED"
    ]
    assert any(
        finding["evidence"]["wait_category"] == "CHANNEL_WAIT"
        for finding in latency
    )


# ---------------------------------------------------------------------------
# T-203: pure analyzer-level latency finding (synthetic timelines)
# ---------------------------------------------------------------------------


def _bundle(dump_index: int, snapshots: list[ThreadSnapshot]) -> ThreadDumpBundle:
    return ThreadDumpBundle(
        snapshots=snapshots,
        source_file=f"dump-{dump_index}.txt",
        source_format="java_jstack",
        language="java",
        dump_index=dump_index,
        dump_label=f"dump-{dump_index}",
    )


def _snapshot(name: str, state: ThreadState, frame: str = "frame") -> ThreadSnapshot:
    return ThreadSnapshot(
        snapshot_id=f"java::{name}",
        thread_name=name,
        thread_id=None,
        state=state,
        category=state.value,
        stack_frames=[StackFrame(function=frame, language="java")],
        language="java",
        source_format="java_jstack",
    )


def test_latency_finding_fires_for_consecutive_network_wait() -> None:
    bundles = [
        _bundle(i, [_snapshot("nio-thread", ThreadState.NETWORK_WAIT, "epollWait")])
        for i in range(3)
    ]
    result = analyze_multi_thread_dumps(bundles)
    findings = [
        f for f in result.metadata["findings"] if f["code"] == "LATENCY_SECTION_DETECTED"
    ]
    assert len(findings) == 1
    assert findings[0]["evidence"]["wait_category"] == "NETWORK_WAIT"
    assert findings[0]["evidence"]["dumps"] == 3
    assert findings[0]["evidence"]["thread_name"] == "nio-thread"


def test_latency_finding_does_not_fire_when_run_breaks() -> None:
    bundles = [
        _bundle(0, [_snapshot("worker", ThreadState.NETWORK_WAIT)]),
        _bundle(1, [_snapshot("worker", ThreadState.RUNNABLE)]),
        _bundle(2, [_snapshot("worker", ThreadState.NETWORK_WAIT)]),
    ]
    result = analyze_multi_thread_dumps(bundles)
    assert result.summary["latency_sections"] == 0


def test_latency_finding_excludes_lock_wait_to_avoid_double_signal() -> None:
    # LOCK_WAIT/BLOCKED is owned by PERSISTENT_BLOCKED_THREAD; the latency
    # finding should not duplicate it.
    bundles = [
        _bundle(i, [_snapshot("locker", ThreadState.LOCK_WAIT)])
        for i in range(3)
    ]
    result = analyze_multi_thread_dumps(bundles)
    codes = [f["code"] for f in result.metadata["findings"]]
    assert "PERSISTENT_BLOCKED_THREAD" in codes
    latency = [
        f for f in result.metadata["findings"] if f["code"] == "LATENCY_SECTION_DETECTED"
    ]
    assert latency == []


def test_latency_finding_runs_per_category_independently() -> None:
    bundles = [
        _bundle(
            i,
            [
                _snapshot("net-thread", ThreadState.NETWORK_WAIT),
                _snapshot("io-thread", ThreadState.IO_WAIT),
            ],
        )
        for i in range(3)
    ]
    result = analyze_multi_thread_dumps(bundles)
    by_category = {
        finding["evidence"]["wait_category"]
        for finding in result.metadata["findings"]
        if finding["code"] == "LATENCY_SECTION_DETECTED"
    }
    assert by_category == {"NETWORK_WAIT", "IO_WAIT"}


def test_latency_table_present_in_result() -> None:
    bundles = [
        _bundle(i, [_snapshot("worker", ThreadState.IO_WAIT)])
        for i in range(3)
    ]
    result = analyze_multi_thread_dumps(bundles)
    assert "latency_sections" in result.tables
    assert result.tables["latency_sections"][0]["thread_name"] == "worker"
    assert result.tables["latency_sections"][0]["wait_category"] == "IO_WAIT"
