"""Java/JVM jstack adapter (T-190).

Wraps the existing :func:`archscope_engine.parsers.thread_dump_parser.parse_thread_dump`
into a :class:`ParserPlugin` so the new multi-dump pipeline can consume
JVM dumps without rewriting the proven parser. The single-dump analyzer
keeps using the original parser unchanged for byte-for-byte parity.
"""
from __future__ import annotations

import re
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
        snapshots = [self._record_to_snapshot(record, index) for index, record in enumerate(records)]
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


# Auto-register so callers that just `import archscope_engine.parsers.thread_dump`
# pick up the Java plugin without touching the registry directly.
DEFAULT_REGISTRY.register(JavaJstackParserPlugin())
