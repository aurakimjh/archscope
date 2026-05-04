"""Tests for Python py-spy / faulthandler parsers (T-198 / T-199)."""
from __future__ import annotations

from archscope_engine.models.thread_snapshot import StackFrame, ThreadState
from archscope_engine.parsers.thread_dump.python_dump import (
    PythonFaulthandlerParserPlugin,
    PythonPySpyParserPlugin,
    _infer_python_state,
    _strip_python_boilerplate,
)


_PYSPY_DUMP = """\
Process 12345: python /app/server.py
Python v3.11.7 (/usr/bin/python3)

Thread 12345 (idle): "MainThread"
    sleep (time.py:42)
    serve (server.py:88)

Thread 12346 (active): "worker-1"
    recv (socket.py:660)
    handle (server.py:120)
"""

_FAULTHANDLER_DUMP = """\
Fatal Python error: SIGTERM

Thread 0x00007f1234567890 (most recent call first):
  File "/app/server.py", line 42 in handler
  File "/app/main.py", line 10 in main

Thread 0x00007f1234560000 (most recent call first):
  File "/usr/lib/python3.11/threading.py", line 320 in wait
  File "/app/worker.py", line 55 in run
"""


def test_pyspy_can_parse_recognizes_banner() -> None:
    plugin = PythonPySpyParserPlugin()
    assert plugin.can_parse(_PYSPY_DUMP[:200])


def test_pyspy_rejects_other_formats() -> None:
    plugin = PythonPySpyParserPlugin()
    assert not plugin.can_parse('"worker" #1 nid=0x123\n')


def test_pyspy_extracts_threads_and_frames(tmp_path) -> None:
    dump = tmp_path / "pyspy.txt"
    dump.write_text(_PYSPY_DUMP, encoding="utf-8")
    bundle = PythonPySpyParserPlugin().parse(dump)

    assert bundle.language == "python"
    assert bundle.source_format == "python_pyspy"
    assert [snap.thread_name for snap in bundle.snapshots] == [
        "MainThread",
        "worker-1",
    ]
    main_frame = bundle.snapshots[0].stack_frames[0]
    assert main_frame.function == "sleep"
    assert main_frame.file == "time.py"
    assert main_frame.line == 42


def test_pyspy_promotes_recv_to_network_wait(tmp_path) -> None:
    dump = tmp_path / "pyspy.txt"
    dump.write_text(_PYSPY_DUMP, encoding="utf-8")
    bundle = PythonPySpyParserPlugin().parse(dump)

    worker = next(s for s in bundle.snapshots if s.thread_name == "worker-1")
    assert worker.state is ThreadState.NETWORK_WAIT


def test_faulthandler_can_parse_recognizes_thread_header() -> None:
    plugin = PythonFaulthandlerParserPlugin()
    assert plugin.can_parse(_FAULTHANDLER_DUMP)


def test_faulthandler_rejects_pyspy_text() -> None:
    plugin = PythonFaulthandlerParserPlugin()
    assert not plugin.can_parse(_PYSPY_DUMP)


def test_faulthandler_extracts_threads_and_frames(tmp_path) -> None:
    dump = tmp_path / "fh.txt"
    dump.write_text(_FAULTHANDLER_DUMP, encoding="utf-8")
    bundle = PythonFaulthandlerParserPlugin().parse(dump)

    assert bundle.source_format == "python_faulthandler"
    assert len(bundle.snapshots) == 2
    first_frame = bundle.snapshots[0].stack_frames[0]
    assert first_frame.function == "handler"
    assert first_frame.file == "/app/server.py"
    assert first_frame.line == 42


def test_faulthandler_threading_wait_promotes_to_lock_wait(tmp_path) -> None:
    dump = tmp_path / "fh.txt"
    dump.write_text(_FAULTHANDLER_DUMP, encoding="utf-8")
    bundle = PythonFaulthandlerParserPlugin().parse(dump)

    # Second thread top frame is `threading.py:320 in wait`.
    second = bundle.snapshots[1]
    assert second.state is ThreadState.LOCK_WAIT


def test_strip_boilerplate_removes_starlette_wrapper() -> None:
    frames = [
        StackFrame(
            function="__call__",
            file="/usr/lib/python3.11/site-packages/starlette/applications.py",
            line=120,
            language="python",
        ),
        StackFrame(
            function="route_handler",
            file="/app/server.py",
            line=42,
            language="python",
        ),
    ]
    cleaned = _strip_python_boilerplate(frames)
    # The starlette wrapper is dropped, leaving only the user view.
    assert [frame.function for frame in cleaned] == ["route_handler"]


def test_infer_python_state_promotes_select_to_io_wait() -> None:
    frames = [
        StackFrame(function="select", file="select.py", line=88, language="python"),
    ]
    assert _infer_python_state(ThreadState.RUNNABLE, frames) is ThreadState.IO_WAIT
