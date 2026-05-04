"""Java/JVM jstack adapter (T-190) + Java-only enrichment (T-194/T-195).

* T-190 wraps the legacy single-dump jstack parser into a registry plugin
  so the multi-dump pipeline can ingest JVM dumps unchanged.
* T-194 normalizes CGLIB/JDK-proxy/Accessor synthetic class suffixes so
  the same logical call site collapses to a single stack signature
  regardless of which proxy hash showed up in this snapshot.
* T-195 promotes RUNNABLE threads stuck in epoll/socket to ``NETWORK_WAIT``
  and threads stuck in classic file I/O to ``IO_WAIT``. Never touches
  BLOCKED/WAITING/TIMED_WAITING so the runtime's own state takes priority.

Both enrichment steps run only on Java frames (``StackFrame.language ==
"java"``) so the registry can mix this plugin with Go/Python/etc. parsers
without cross-contamination.
"""
from __future__ import annotations

import re
from dataclasses import replace
from pathlib import Path

from archscope_engine.models.thread_dump import ThreadDumpRecord
from archscope_engine.models.thread_snapshot import (
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump.registry import DEFAULT_REGISTRY
from archscope_engine.parsers.thread_dump_parser import parse_thread_dump

# Heuristics for detecting jstack output:
#   - The classic header `Full thread dump OpenJDK ...`
#   - A quoted-name line followed by `nid=0x...` (matches every jstack ever)
_FULL_THREAD_HEADER = re.compile(r"Full thread dump\b", re.IGNORECASE)
_JSTACK_NID_LINE = re.compile(r'^"[^"]+".*\bnid=0x[0-9a-fA-F]+', re.MULTILINE)


class JavaJstackParserPlugin:
    """Java jstack / JVM thread-dump parser plugin."""

    format_id: str = "java_jstack"
    language: str = "java"

    def can_parse(self, head: str) -> bool:
        if _FULL_THREAD_HEADER.search(head):
            return True
        return bool(_JSTACK_NID_LINE.search(head))

    def parse(self, path: Path) -> ThreadDumpBundle:
        records = parse_thread_dump(Path(path))
        snapshots = [
            self._record_to_snapshot(record, index)
            for index, record in enumerate(records)
        ]
        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
        )

    def _record_to_snapshot(self, record: ThreadDumpRecord, index: int) -> ThreadSnapshot:
        state = ThreadState.coerce(record.state)
        if state is ThreadState.UNKNOWN and record.lock_info:
            # jstack reports threads parked on a lock as WAITING but the
            # state line is sometimes missing — fall back to the lock hint.
            state = ThreadState.LOCK_WAIT
        frames = [_frame_from_jstack(line) for line in record.stack]
        # T-194: collapse CGLIB/proxy hash variants into a stable identifier.
        frames = [_normalize_proxy_frame(frame) for frame in frames]
        # T-195: promote RUNNABLE epoll/socket/file-IO threads to a more
        # specific waiting state so the multi-dump correlator does not
        # mistake a parked I/O thread for a hot CPU thread.
        state = _infer_java_state(state, frames)
        return ThreadSnapshot(
            snapshot_id=f"java::{index}::{record.thread_name}",
            thread_name=record.thread_name,
            thread_id=record.thread_id,
            state=state,
            category=record.category,
            stack_frames=frames,
            lock_info=record.lock_info,
            metadata={"raw_block": record.raw_block},
            language=self.language,
            source_format=self.format_id,
        )


_JAVA_FRAME_RE = re.compile(
    r"^(?P<qualified>[\w$.<>]+)\((?P<location>[^)]*)\)\s*$"
)


def _frame_from_jstack(line: str) -> StackFrame:
    """Convert a single ``foo.Bar.method(File.java:42)`` frame to ``StackFrame``."""
    text = line.strip()
    match = _JAVA_FRAME_RE.match(text)
    if not match:
        return StackFrame(function=text, language="java")
    qualified = match.group("qualified")
    location = match.group("location") or ""
    if "." in qualified:
        module, _, function = qualified.rpartition(".")
    else:
        module, function = None, qualified
    file_part: str | None = None
    line_part: int | None = None
    if location and ":" in location:
        file_part, _, line_text = location.rpartition(":")
        try:
            line_part = int(line_text)
        except ValueError:
            line_part = None
            file_part = location
    elif location:
        file_part = location
    return StackFrame(
        function=function,
        module=module,
        file=file_part or None,
        line=line_part,
        language="java",
    )


# ---------------------------------------------------------------------------
# T-194: aitop's `cleanProxyClassNames` (Java/Spring AOP-aware)
# ---------------------------------------------------------------------------

# Patterns that strip the dynamic suffix while keeping the rest of the
# qualified identifier intact. Order matters — the longest-prefix variants
# must be tried before the shorter ones.
_PROXY_PATTERNS: tuple[tuple[re.Pattern[str], str], ...] = (
    (re.compile(r"\$\$EnhancerByCGLIB\$\$[\w$]+"), ""),
    (re.compile(r"\$\$FastClassByCGLIB\$\$[\w$]+"), ""),
    (re.compile(r"(\$\$Proxy)\d+"), r"\1"),
    (re.compile(r"(GeneratedMethodAccessor)\d+"), r"\1"),
    (re.compile(r"(GeneratedConstructorAccessor)\d+"), r"\1"),
    # Generic "Accessor<digits>" catch-all for SerializedLambda, JDK proxy
    # accessor classes, etc. (must come after the specific Generated*
    # patterns so we don't strip the prefix twice).
    (re.compile(r"(Accessor)\d+"), r"\1"),
)


def _normalize_proxy_text(text: str) -> str:
    for pattern, replacement in _PROXY_PATTERNS:
        text = pattern.sub(replacement, text)
    # Collapse the leftover ".." / leading-dot artifacts when EnhancerByCGLIB
    # was strip-replaced with an empty string in the middle of an identifier.
    text = re.sub(r"\.{2,}", ".", text)
    return text.strip(".")


def _normalize_proxy_frame(frame: StackFrame) -> StackFrame:
    if frame.language != "java":
        return frame
    new_function = _normalize_proxy_text(frame.function)
    new_module = _normalize_proxy_text(frame.module) if frame.module else None
    if new_function == frame.function and new_module == frame.module:
        return frame
    return replace(frame, function=new_function, module=new_module)


# ---------------------------------------------------------------------------
# T-195: Java network/IO state inference
# ---------------------------------------------------------------------------

_NETWORK_WAIT_PATTERNS = re.compile(
    r"(?:"
    r"epollWait|EPoll(?:Selector)?\.wait|EPollArrayWrapper\.poll|"
    r"socketAccept|Socket\.\w*accept0|NioSocketImpl\.accept|"
    r"socketRead0|SocketInputStream\.socketRead|"
    r"NioSocketImpl\.read|SocketChannelImpl\.read|"
    r"SocketDispatcher\.read|sun\.nio\.ch\.\w+Selector\.\w*poll|"
    r"netty\.\w+EventLoop\.\w*select|netty\.channel\.nio\.NioEventLoop\.run"
    r")"
)
_IO_WAIT_PATTERNS = re.compile(
    r"(?:"
    r"FileInputStream\.read|FileInputStream\.readBytes|"
    r"FileChannelImpl\.read|FileChannelImpl\.transferFrom|"
    r"RandomAccessFile\.read|RandomAccessFile\.readBytes|"
    r"BufferedReader\.readLine|FileDispatcherImpl\.\w+|"
    r"java\.io\.FileInputStream\.read"
    r")"
)


def _infer_java_state(state: ThreadState, frames: list[StackFrame]) -> ThreadState:
    """Promote RUNNABLE threads to the more specific NETWORK_WAIT/IO_WAIT
    states when the top frame indicates the thread is parked in I/O.

    The runtime-reported state always wins for non-RUNNABLE threads — a
    BLOCKED thread stays BLOCKED even if its top frame happens to look
    like a socket read.
    """
    if state is not ThreadState.RUNNABLE or not frames:
        return state
    top = frames[0]
    text_parts: list[str] = []
    if top.module:
        text_parts.append(f"{top.module}.{top.function}")
    text_parts.append(top.function)
    text = " ".join(text_parts)
    if _NETWORK_WAIT_PATTERNS.search(text):
        return ThreadState.NETWORK_WAIT
    if _IO_WAIT_PATTERNS.search(text):
        return ThreadState.IO_WAIT
    return state


# Auto-register so callers that just `import archscope_engine.parsers.thread_dump`
# pick up the Java plugin without touching the registry directly.
DEFAULT_REGISTRY.register(JavaJstackParserPlugin())
