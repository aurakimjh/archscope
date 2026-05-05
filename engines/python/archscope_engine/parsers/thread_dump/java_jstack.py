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
import os
from dataclasses import dataclass, field, replace
from pathlib import Path

from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.thread_dump import ThreadDumpRecord
from archscope_engine.models.thread_snapshot import (
    LockHandle,
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump.registry import DEFAULT_REGISTRY
from archscope_engine.parsers.thread_dump_parser import (
    parse_thread_block,
    parse_thread_dump,
)

# Heuristics for detecting jstack output:
#   - The classic header `Full thread dump OpenJDK ...`
#   - A quoted-name line followed by `nid=0x...` (matches every jstack ever)
_FULL_THREAD_HEADER = re.compile(r"Full thread dump\b", re.IGNORECASE)
_JSTACK_NID_LINE = re.compile(r'^"[^"]+".*\bnid=0x[0-9a-fA-F]+', re.MULTILINE)
_THREAD_BLOCK_HEADER = re.compile(r'^"[^"]+"')
_CLASS_HISTOGRAM_LIMIT_ENV = "ARCHSCOPE_CLASS_HISTOGRAM_ROW_LIMIT"
_DEFAULT_CLASS_HISTOGRAM_ROW_LIMIT = 500


@dataclass
class _JstackSection:
    records: list[ThreadDumpRecord] = field(default_factory=list)
    raw_lines: list[str] = field(default_factory=list)
    start_line: int = 0
    end_line: int = 0
    raw_timestamp: str | None = None


class JavaJstackParserPlugin:
    """Java jstack / JVM thread-dump parser plugin."""

    format_id: str = "java_jstack"
    language: str = "java"

    def can_parse(self, head: str) -> bool:
        if _FULL_THREAD_HEADER.search(head):
            return True
        return bool(_JSTACK_NID_LINE.search(head))

    def parse(
        self,
        path: Path,
        *,
        class_histogram_limit: int | None = None,
    ) -> ThreadDumpBundle:
        records = parse_thread_dump(Path(path))
        row_limit = _class_histogram_row_limit(class_histogram_limit)
        snapshots = [
            self._record_to_snapshot(record, index)
            for index, record in enumerate(records)
        ]
        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
            metadata={"class_histogram_row_limit": row_limit},
        )

    def parse_all(
        self,
        path: Path,
        *,
        class_histogram_limit: int | None = None,
    ) -> list[ThreadDumpBundle]:
        row_limit = _class_histogram_row_limit(class_histogram_limit)
        sections = _split_jstack_sections(Path(path))
        if not sections:
            return [self.parse(path, class_histogram_limit=row_limit)]

        bundles: list[ThreadDumpBundle] = []
        for section_index, section in enumerate(sections):
            snapshots = [
                self._record_to_snapshot(
                    record,
                    index,
                    section_index=section_index,
                )
                for index, record in enumerate(section.records)
            ]
            bundles.append(
                ThreadDumpBundle(
                    snapshots=snapshots,
                    source_file=str(path),
                    source_format=self.format_id,
                    language=self.language,
                    metadata=_section_metadata(
                        section,
                        class_histogram_limit=row_limit,
                    ),
                )
            )
        return bundles

    def _record_to_snapshot(
        self,
        record: ThreadDumpRecord,
        index: int,
        *,
        section_index: int = 0,
    ) -> ThreadSnapshot:
        state = ThreadState.coerce(record.state)
        frames = [_frame_from_jstack(line) for line in record.stack]
        # T-194: collapse CGLIB/proxy hash variants into a stable identifier.
        frames = [_normalize_proxy_frame(frame) for frame in frames]
        # T-195: promote RUNNABLE epoll/socket/file-IO threads to a more
        # specific waiting state so the multi-dump correlator does not
        # mistake a parked I/O thread for a hot CPU thread.
        state = _infer_java_state(state, frames)
        # T-219: pull lock IDs out of the raw block. `lock_info` (legacy
        # string field) keeps its byte-for-byte content; the new
        # `lock_holds`/`lock_waiting` fields carry the structured view.
        lock_holds, lock_waiting = _extract_lock_handles(record.raw_block)
        if (
            lock_waiting is not None
            and lock_waiting.wait_mode == "lock_entry_wait"
            and state in (ThreadState.RUNNABLE, ThreadState.UNKNOWN)
        ):
            state = ThreadState.LOCK_WAIT
        elif lock_waiting is not None and state is ThreadState.UNKNOWN:
            state = ThreadState.WAITING
        metadata: dict[str, object] = {"raw_block": record.raw_block}
        if _is_virtual_thread(record):
            metadata["is_virtual_thread"] = True
        native_method = _native_method(record)
        if native_method:
            metadata["native_method"] = native_method
        carrier_pinning = _carrier_pinning(record.raw_block, frames)
        if carrier_pinning:
            metadata["carrier_pinning"] = carrier_pinning
        if lock_waiting is not None and lock_waiting.wait_mode:
            metadata["monitor_wait_mode"] = lock_waiting.wait_mode
        return ThreadSnapshot(
            snapshot_id=f"java::{section_index}::{index}::{record.thread_name}",
            thread_name=record.thread_name,
            thread_id=record.thread_id,
            state=state,
            category=record.category,
            stack_frames=frames,
            lock_info=record.lock_info,
            metadata=metadata,
            language=self.language,
            source_format=self.format_id,
            lock_holds=lock_holds,
            lock_waiting=lock_waiting,
        )


def _split_jstack_sections(path: Path) -> list[_JstackSection]:
    """Split a text file that may contain repeated JVM thread dumps."""
    sections: list[_JstackSection] = []
    current_section: _JstackSection | None = None
    current_block: list[str] = []
    prefix_lines: list[str] = []
    recent_lines: list[str] = []

    def flush_block() -> None:
        nonlocal current_block
        if current_section is None or not current_block:
            current_block = []
            return
        record = parse_thread_block(current_block)
        if record is not None:
            current_section.records.append(record)
        current_block = []

    def flush_section() -> None:
        nonlocal current_section
        if current_section is None:
            return
        flush_block()
        if current_section.records:
            sections.append(current_section)
        current_section = None

    def start_section(
        line_number: int,
        timestamp_lines: list[str] | None = None,
    ) -> None:
        nonlocal current_section
        current_section = _JstackSection(
            start_line=line_number,
            end_line=line_number,
            raw_timestamp=_extract_raw_timestamp(timestamp_lines or prefix_lines),
        )

    def remember(line: str) -> None:
        nonlocal recent_lines
        if line.strip():
            recent_lines = (recent_lines + [line])[-8:]

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        is_full_header = bool(_FULL_THREAD_HEADER.search(line))
        is_thread_header = bool(_THREAD_BLOCK_HEADER.match(line))

        if is_full_header:
            timestamp_lines = list(recent_lines)
            flush_section()
            start_section(line_number, timestamp_lines)
            current_section.raw_lines.append(line)
            current_section.end_line = line_number
            prefix_lines = []
            remember(line)
            continue

        if current_section is None:
            if is_thread_header:
                start_section(line_number)
                current_block = [line]
                current_section.raw_lines.append(line)
                current_section.end_line = line_number
                remember(line)
            elif line.strip():
                prefix_lines = (prefix_lines + [line])[-8:]
                remember(line)
            continue

        current_section.raw_lines.append(line)
        current_section.end_line = line_number
        if is_thread_header:
            flush_block()
            current_block = [line]
        elif current_block:
            current_block.append(line)
        remember(line)

    flush_section()
    return sections


def _extract_raw_timestamp(lines: list[str]) -> str | None:
    for line in reversed(lines):
        text = line.strip()
        if not text:
            continue
        if re.search(r"\d{4}[-/]\d{2}[-/]\d{2}|\d{2}:\d{2}:\d{2}", text):
            return text
    return None


def _section_metadata(
    section: _JstackSection,
    *,
    class_histogram_limit: int,
) -> dict[str, object]:
    metadata: dict[str, object] = {
        "start_line": section.start_line,
        "end_line": section.end_line,
        "class_histogram_row_limit": class_histogram_limit,
    }
    if section.raw_timestamp:
        metadata["raw_timestamp"] = section.raw_timestamp
    smr = _parse_smr_diagnostics(section.raw_lines)
    if smr:
        metadata["smr"] = smr
    class_histogram = _parse_text_class_histogram(
        section.raw_lines,
        row_limit=class_histogram_limit,
    )
    if class_histogram:
        metadata["class_histogram"] = class_histogram
    heap_block = _parse_g1_heap_block(section.raw_lines)
    if heap_block:
        metadata["jvm_heap_block"] = heap_block
    return metadata


_JAVA_FRAME_RE = re.compile(
    r"^(?P<qualified>[\w$./<>]+)\((?P<location>[^)]*)\)\s*$"
)


def _frame_from_jstack(line: str) -> StackFrame:
    """Convert a single ``foo.Bar.method(File.java:42)`` frame to ``StackFrame``."""
    text = line.strip()
    match = _JAVA_FRAME_RE.match(text)
    if not match:
        return StackFrame(function=text, language="java")
    qualified = match.group("qualified")
    if "/" in qualified:
        _, _, qualified = qualified.partition("/")
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


# ---------------------------------------------------------------------------
# JVM-specific metadata enrichment
# ---------------------------------------------------------------------------

_CLASS_HISTOGRAM_ROW_RE = re.compile(
    r"^\s*(?P<rank>\d+):\s+(?P<instances>\d+)\s+(?P<bytes>\d+)\s+(?P<class_name>.+?)\s*$"
)
_CLASS_HISTOGRAM_TOTAL_RE = re.compile(
    r"^\s*Total\s+(?P<instances>\d+)\s+(?P<bytes>\d+)\s*$",
    re.IGNORECASE,
)
def _is_virtual_thread(record: ThreadDumpRecord) -> bool:
    text = f"{record.thread_name}\n{record.raw_block}".lower()
    return "virtualthread" in text or "virtual thread" in text


def _native_method(record: ThreadDumpRecord) -> str | None:
    for frame in record.stack:
        if "(Native Method)" in frame:
            return frame.strip()
    return None


def _carrier_pinning(
    raw_block: str,
    frames: list[StackFrame],
) -> dict[str, object] | None:
    text = raw_block.lower()
    if "virtual" not in text or ("carrier" not in text and "pinn" not in text):
        return None
    top_frame = frames[0].render() if frames else None
    candidate = _first_non_jdk_frame(frames)
    return {
        "top_frame": top_frame,
        "candidate_method": candidate or top_frame,
        "reason": "virtual_thread_carrier_or_pinning_marker",
    }


def _first_non_jdk_frame(frames: list[StackFrame]) -> str | None:
    for frame in frames:
        rendered = frame.render()
        if not rendered.startswith(
            (
                "java.",
                "javax.",
                "jdk.",
                "sun.",
                "com.sun.",
                "java.base.",
            )
        ):
            return rendered
    return None


_G1_HEAP_TOTAL_RE = re.compile(
    r"garbage-first heap\s+total\s+(?P<total_kb>\d+)K,\s*used\s+(?P<used_kb>\d+)K"
)
_G1_REGION_RE = re.compile(
    r"region\s+size\s+(?P<region_kb>\d+)K,\s*(?P<young>\d+)\s+young\s*\((?P<young_kb>\d+)K\),\s*(?P<survivors>\d+)\s+survivors\s*\((?P<survivors_kb>\d+)K\)"
)
_METASPACE_RE = re.compile(
    r"^\s*Metaspace\s+used\s+(?P<used_kb>\d+)K,\s*capacity\s+(?P<capacity_kb>\d+)K,\s*committed\s+(?P<committed_kb>\d+)K,\s*reserved\s+(?P<reserved_kb>\d+)K"
)


def _parse_g1_heap_block(lines: list[str]) -> dict[str, object] | None:
    """Extract the ``{Heap before GC ...}`` block JDK 8 G1 frequently
    embeds at the top of jstack-style multi-dump files.

    Returns ``None`` when no recognizable heap block is found. Sizes are
    surfaced in MB so the UI doesn't have to convert.
    """
    found: dict[str, object] = {}
    in_block = False
    block_start_line: int | None = None
    for offset, line in enumerate(lines, start=1):
        stripped = line.strip()
        # JDK 8 G1 marker: "{Heap before GC invocations=..."
        if stripped.startswith("{Heap"):
            in_block = True
            block_start_line = offset
            found.setdefault("phase", "before_gc" if "before" in stripped.lower() else "after_gc")
            continue
        if not in_block:
            # Some dumps include the heap header without the {Heap marker.
            # Match "garbage-first heap" anywhere in the leading lines.
            if "garbage-first heap" in line.lower() and offset < 50:
                in_block = True
                block_start_line = offset
        if not in_block:
            continue

        m = _G1_HEAP_TOTAL_RE.search(line)
        if m:
            found["heap_total_mb"] = round(int(m.group("total_kb")) / 1024.0, 2)
            found["heap_used_mb"] = round(int(m.group("used_kb")) / 1024.0, 2)
        m = _G1_REGION_RE.search(line)
        if m:
            found["region_size_mb"] = round(int(m.group("region_kb")) / 1024.0, 2)
            found["young_regions"] = int(m.group("young"))
            found["young_used_mb"] = round(int(m.group("young_kb")) / 1024.0, 2)
            found["survivor_regions"] = int(m.group("survivors"))
            found["survivor_used_mb"] = round(int(m.group("survivors_kb")) / 1024.0, 2)
        m = _METASPACE_RE.match(line)
        if m:
            found["metaspace_used_mb"] = round(int(m.group("used_kb")) / 1024.0, 2)
            found["metaspace_committed_mb"] = round(int(m.group("committed_kb")) / 1024.0, 2)
            found["metaspace_reserved_mb"] = round(int(m.group("reserved_kb")) / 1024.0, 2)

        # Block ends with "}" or after a thread "Foo" quote-line takes over.
        if stripped == "}" or stripped.startswith('"'):
            break
    if not found.get("heap_total_mb") and not found.get("metaspace_used_mb"):
        return None
    if block_start_line is not None:
        found["section_start_line"] = block_start_line
    return found


def _parse_smr_diagnostics(lines: list[str]) -> dict[str, object] | None:
    unresolved: list[dict[str, object]] = []
    in_smr_block = False
    remaining = 0
    for offset, line in enumerate(lines, start=1):
        lower = line.lower()
        if "smr" in lower and "thread" in lower:
            in_smr_block = True
            remaining = 80
        if not in_smr_block:
            continue
        if "unresolved" in lower or "zombie" in lower:
            unresolved.append({"section_line": offset, "line": line.strip()})
        remaining -= 1
        if remaining <= 0:
            in_smr_block = False
    if not unresolved:
        return None
    return {
        "unresolved_count": len(unresolved),
        "unresolved": unresolved,
    }


def _class_histogram_row_limit(value: int | None = None) -> int:
    raw: object = value
    if raw is None:
        raw = os.environ.get(_CLASS_HISTOGRAM_LIMIT_ENV)
    if raw in (None, ""):
        return _DEFAULT_CLASS_HISTOGRAM_ROW_LIMIT
    try:
        limit = int(raw)
    except (TypeError, ValueError) as exc:
        raise ValueError(
            f"{_CLASS_HISTOGRAM_LIMIT_ENV} / class_histogram_limit must be a "
            "positive integer."
        ) from exc
    if limit <= 0:
        raise ValueError(
            f"{_CLASS_HISTOGRAM_LIMIT_ENV} / class_histogram_limit must be a "
            "positive integer."
        )
    return limit


def _parse_text_class_histogram(
    lines: list[str],
    *,
    row_limit: int,
) -> dict[str, object] | None:
    classes: list[dict[str, object]] = []
    total_instances: int | None = None
    total_bytes: int | None = None
    total_rows = 0
    for line in lines:
        row = _CLASS_HISTOGRAM_ROW_RE.match(line)
        if row:
            total_rows += 1
            if len(classes) < row_limit:
                classes.append(
                    {
                        "rank": int(row.group("rank")),
                        "instances": int(row.group("instances")),
                        "bytes": int(row.group("bytes")),
                        "class_name": row.group("class_name").strip(),
                    }
                )
            continue
        total = _CLASS_HISTOGRAM_TOTAL_RE.match(line)
        if total:
            total_instances = int(total.group("instances"))
            total_bytes = int(total.group("bytes"))
    if not classes:
        return None
    payload: dict[str, object] = {
        "classes": classes,
        "row_limit": row_limit,
        "total_rows": total_rows,
        "truncated": total_rows > row_limit,
    }
    if total_instances is not None:
        payload["total_instances"] = total_instances
    if total_bytes is not None:
        payload["total_bytes"] = total_bytes
    return payload


# ---------------------------------------------------------------------------
# T-219: Lock-handle extraction
# ---------------------------------------------------------------------------

# `- locked <0x000000076ab62208> (a java.util.concurrent.locks.ReentrantLock$NonfairSync)`
_LOCKED_RE = re.compile(
    r"-\s+locked\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?"
)
# `- waiting to lock <0x...> (a Foo)` and `- waiting on <0x...> (a Foo)`
# (the latter form is what `Object.wait()` produces).
_WAITING_TO_LOCK_RE = re.compile(
    r"-\s+waiting to lock\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?"
)
_WAITING_ON_RE = re.compile(
    r"-\s+waiting on\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?"
)
# `- parking to wait for <0x...> (a java.util.concurrent.locks.AbstractQueuedSynchronizer$ConditionObject)`
_PARKING_RE = re.compile(
    r"-\s+parking to wait for\s+<(?P<id>0x[0-9a-fA-F]+)>(?:\s+\(a\s+(?P<cls>[^)]+)\))?"
)


def _extract_lock_handles(raw_block: str) -> tuple[list[LockHandle], LockHandle | None]:
    """Pull lock IDs out of a raw jstack block.

    Returns ``(lock_holds, lock_waiting)`` — multiple `locked` lines may
    appear (re-entrant or nested locks); the first matching wait line
    wins. Lock IDs that show up as both `locked` and `waiting to lock`
    on the same thread are treated as held (the thread already owns the
    monitor and is re-entering it).
    """
    holds: list[LockHandle] = []
    seen_ids: set[str] = set()
    waiting: LockHandle | None = None
    for raw_line in raw_block.splitlines():
        line = raw_line.strip()
        if not line:
            continue

        for pattern, wait_mode in (
            (_WAITING_TO_LOCK_RE, "lock_entry_wait"),
            (_WAITING_ON_RE, "object_wait"),
            (_PARKING_RE, "parking_condition_wait"),
        ):
            match = pattern.match(line)
            if match and waiting is None:
                waiting = LockHandle(
                    lock_id=match.group("id"),
                    lock_class=match.group("cls"),
                    wait_mode=wait_mode,
                )
                break

        locked_match = _LOCKED_RE.match(line)
        if locked_match:
            lock_id = locked_match.group("id")
            if lock_id in seen_ids:
                continue
            seen_ids.add(lock_id)
            holds.append(
                LockHandle(
                    lock_id=lock_id,
                    lock_class=locked_match.group("cls"),
                    wait_mode="locked_owner",
                )
            )

    if waiting is not None and waiting.lock_id in seen_ids:
        # The thread already holds this monitor — treat as a re-entrant
        # acquire instead of a contention point.
        waiting = None

    return holds, waiting


# Auto-register so callers that just `import archscope_engine.parsers.thread_dump`
# pick up the Java plugin without touching the registry directly.
DEFAULT_REGISTRY.register(JavaJstackParserPlugin())
