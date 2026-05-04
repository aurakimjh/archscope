"""Python thread-dump parsers (T-198) + Python-only enrichment (T-199).

Two flavors of Python dump are common in production:

* ``py-spy dump --pid <pid>`` produces a banner like ``Process 12345:`` /
  ``Python v3.11.7`` followed by per-thread blocks::

      Thread 12345 (idle): "MainThread"
          sleep (time.py:42)
          worker (script.py:88)

* ``faulthandler.dump_traceback`` (and SIGSEGV crash dumps) produces::

      Thread 0x00007f1234567890 (most recent call first):
        File "/app/server.py", line 42 in handler
        File "/app/main.py", line 10 in main

Both are surfaced as separate plugins so detection is unambiguous, but
they share a single enrichment pass that normalizes Django/FastAPI/Flask
middleware wrappers and promotes ``select.*`` / ``socket.recv`` / async
sleep frames to the appropriate :class:`ThreadState`.
"""
from __future__ import annotations

import re
from dataclasses import replace
from pathlib import Path

from archscope_engine.models.thread_snapshot import (
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump.registry import DEFAULT_REGISTRY


# ---------------------------------------------------------------------------
# py-spy
# ---------------------------------------------------------------------------

_PYSPY_BANNER_RE = re.compile(
    r"^Process\s+\d+:.*\n.*Python\s+v?\d+\.\d+", re.MULTILINE
)
_PYSPY_THREAD_RE = re.compile(
    r"^Thread\s+(?P<tid>\d+)\s+\((?P<state>[^)]+)\):\s*(?P<name>.*)$"
)
_PYSPY_FRAME_RE = re.compile(
    r"^\s+(?P<func>[^\s(]+)\s+\((?P<file>[^:]+):(?P<line>\d+)\)\s*$"
)


class PythonPySpyParserPlugin:
    """Parser for ``py-spy dump`` text output."""

    format_id: str = "python_pyspy"
    language: str = "python"

    def can_parse(self, head: str) -> bool:
        return bool(_PYSPY_BANNER_RE.search(head))

    def parse(self, path: Path) -> ThreadDumpBundle:
        text = Path(path).read_text(encoding="utf-8", errors="replace")
        snapshots: list[ThreadSnapshot] = []
        current: ThreadSnapshot | None = None
        thread_index = 0
        for line in text.splitlines():
            line = line.rstrip("\r")
            thread_match = _PYSPY_THREAD_RE.match(line)
            if thread_match:
                if current is not None:
                    snapshots.append(_finalize_python_snapshot(current))
                state = ThreadState.coerce(thread_match.group("state"))
                name = thread_match.group("name").strip().strip('"') or (
                    f"thread-{thread_match.group('tid')}"
                )
                current = ThreadSnapshot(
                    snapshot_id=f"python::{thread_index}::{name}",
                    thread_name=name,
                    thread_id=thread_match.group("tid"),
                    state=state,
                    category=state.value,
                    stack_frames=[],
                    metadata={"raw_state": thread_match.group("state")},
                    language="python",
                    source_format=self.format_id,
                )
                thread_index += 1
                continue
            if current is None:
                continue
            frame_match = _PYSPY_FRAME_RE.match(line)
            if frame_match:
                try:
                    line_no = int(frame_match.group("line"))
                except ValueError:
                    line_no = None
                current.stack_frames.append(
                    StackFrame(
                        function=frame_match.group("func"),
                        file=frame_match.group("file"),
                        line=line_no,
                        language="python",
                    )
                )

        if current is not None:
            snapshots.append(_finalize_python_snapshot(current))

        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
        )


# ---------------------------------------------------------------------------
# faulthandler
# ---------------------------------------------------------------------------

_FAULTHANDLER_THREAD_RE = re.compile(
    r"^Thread\s+0x[0-9a-fA-F]+\s+\(most recent call first\):\s*$",
    re.MULTILINE,
)
_FAULTHANDLER_FRAME_RE = re.compile(
    r'^\s+File\s+"(?P<file>[^"]+)",\s+line\s+(?P<line>\d+)\s+in\s+(?P<func>\S+)\s*$'
)


class PythonFaulthandlerParserPlugin:
    """Parser for ``faulthandler.dump_traceback`` output."""

    format_id: str = "python_faulthandler"
    language: str = "python"

    def can_parse(self, head: str) -> bool:
        return bool(_FAULTHANDLER_THREAD_RE.search(head))

    def parse(self, path: Path) -> ThreadDumpBundle:
        text = Path(path).read_text(encoding="utf-8", errors="replace")
        snapshots: list[ThreadSnapshot] = []
        current: ThreadSnapshot | None = None
        thread_index = 0
        current_tid: str | None = None
        for raw_line in text.splitlines():
            line = raw_line.rstrip("\r")
            thread_match = _FAULTHANDLER_THREAD_RE.match(line)
            if thread_match:
                if current is not None:
                    snapshots.append(_finalize_python_snapshot(current))
                # Pull the hex tid out of the matched line for the snapshot id.
                tid_match = re.search(r"0x[0-9a-fA-F]+", line)
                current_tid = tid_match.group(0) if tid_match else f"thread-{thread_index}"
                current = ThreadSnapshot(
                    snapshot_id=f"python::{thread_index}::{current_tid}",
                    thread_name=current_tid,
                    thread_id=current_tid,
                    state=ThreadState.UNKNOWN,
                    category=None,
                    stack_frames=[],
                    metadata={"trace_order": "most_recent_first"},
                    language="python",
                    source_format=self.format_id,
                )
                thread_index += 1
                continue
            if current is None:
                continue
            frame_match = _FAULTHANDLER_FRAME_RE.match(line)
            if frame_match:
                try:
                    line_no = int(frame_match.group("line"))
                except ValueError:
                    line_no = None
                current.stack_frames.append(
                    StackFrame(
                        function=frame_match.group("func"),
                        file=frame_match.group("file"),
                        line=line_no,
                        language="python",
                    )
                )

        if current is not None:
            snapshots.append(_finalize_python_snapshot(current))

        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
        )


# ---------------------------------------------------------------------------
# T-199 — Python framework cleanup + state inference
# ---------------------------------------------------------------------------

# Drop Django/FastAPI/Flask middleware wrapper frames so the actual view
# call sits at the top. We only strip frames whose function name matches
# one of these well-known wrappers; legitimate user code never appears here.
_PYTHON_BOILERPLATE_FUNCTIONS = frozenset(
    {
        "MiddlewareMixin.__call__",
        "__call__",  # generic ASGI/WSGI wrappers, scoped further by file path below
        "solve_dependencies",
        "run_endpoint_function",
        "view_func",
        "wraps",
        "wrapper",
        "_wrapper",
        "decorator",
        "dispatch_request",
        "dispatch",
    }
)
_PYTHON_BOILERPLATE_FILES = re.compile(
    r"(?:starlette|fastapi|django|flask|gunicorn|uvicorn|werkzeug|asgi)\b"
)


def _strip_python_boilerplate(frames: list[StackFrame]) -> list[StackFrame]:
    """Drop framework wrappers that obscure the user's view function."""
    cleaned: list[StackFrame] = []
    for frame in frames:
        if frame.language != "python":
            cleaned.append(frame)
            continue
        is_generic_wrapper = frame.function in _PYTHON_BOILERPLATE_FUNCTIONS
        in_framework_file = bool(
            frame.file and _PYTHON_BOILERPLATE_FILES.search(frame.file.lower())
        )
        if is_generic_wrapper and in_framework_file:
            continue
        cleaned.append(frame)
    return cleaned or list(frames)


_PY_NETWORK_FUNCTIONS = frozenset(
    {"recv", "recvfrom", "recvmsg", "send", "sendto", "sendmsg", "accept", "connect"}
)
_PY_NETWORK_FILE_HINTS = (
    "urllib3",
    "requests/",
    "httpx/",
    "http/client.py",
)
_PY_IO_FUNCTIONS = frozenset({"select", "poll", "epoll", "kqueue"})
_PY_LOCK_FUNCTIONS = frozenset({"acquire", "wait"})


def _infer_python_state(state: ThreadState, frames: list[StackFrame]) -> ThreadState:
    """Promote based on the (file, function) pair of the deepest frame.

    py-spy / faulthandler emit the function name and file separately, so
    matching a literal `socket.recv` against the joined text does not work.
    Instead we inspect the function name against a small allow-list and
    use the file path to disambiguate (e.g. `acquire` is a lock op only
    when the file is `threading.py`).
    """
    if not frames:
        return state
    top = frames[0]
    func = top.function or ""
    file_lower = (top.file or "").lower()

    # Network: socket recv/send/etc, urllib3 / requests / http.client
    if func in _PY_NETWORK_FUNCTIONS and "socket" in file_lower:
        return ThreadState.NETWORK_WAIT
    if any(hint in file_lower for hint in _PY_NETWORK_FILE_HINTS):
        return ThreadState.NETWORK_WAIT

    # Lock: threading.{acquire,wait}, queue.get
    if func in _PY_LOCK_FUNCTIONS and "threading.py" in file_lower:
        return ThreadState.LOCK_WAIT
    if func == "get" and "queue.py" in file_lower:
        return ThreadState.LOCK_WAIT

    # IO: select.* / asyncio.* / gevent.hub / os.read / file.read
    if func in _PY_IO_FUNCTIONS and "select" in file_lower:
        return ThreadState.IO_WAIT
    if "asyncio" in file_lower and func in {"sleep", "run_forever", "_run_once"}:
        return ThreadState.IO_WAIT
    if "gevent" in file_lower:
        return ThreadState.IO_WAIT
    if func == "read" and (
        "os.py" in file_lower or "file" in file_lower or "io.py" in file_lower
    ):
        return ThreadState.IO_WAIT

    return state


def _finalize_python_snapshot(snapshot: ThreadSnapshot) -> ThreadSnapshot:
    cleaned_frames = _strip_python_boilerplate(snapshot.stack_frames)
    new_state = _infer_python_state(snapshot.state, cleaned_frames)
    snapshot.stack_frames = cleaned_frames
    snapshot.state = new_state
    snapshot.category = new_state.value
    return snapshot


# Placeholder import to keep replace() within tree-shaking range (used by
# downstream consumers that build new frame instances).
__all__ = [
    "PythonFaulthandlerParserPlugin",
    "PythonPySpyParserPlugin",
    "_finalize_python_snapshot",
    "_infer_python_state",
    "_strip_python_boilerplate",
]
_replace_unused = replace  # noqa: F841 — re-exported via implicit dataclass usage


DEFAULT_REGISTRY.register(PythonPySpyParserPlugin())
DEFAULT_REGISTRY.register(PythonFaulthandlerParserPlugin())
