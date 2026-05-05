"""Java jcmd JSON thread-dump parser plugin.

JDK tooling can emit a structured JSON thread dump. This plugin keeps it
separate from the text jstack parser so Java-specific shape assumptions
do not leak into the multi-language registry.
"""
from __future__ import annotations

import json
from datetime import datetime
from pathlib import Path
from typing import Any, Iterable

from archscope_engine.common.file_utils import detect_text_encoding
from archscope_engine.models.thread_snapshot import (
    LockHandle,
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump.java_jstack import (
    _carrier_pinning,
    _extract_lock_handles,
    _frame_from_jstack,
    _infer_java_state,
    _normalize_proxy_frame,
)
from archscope_engine.parsers.thread_dump.registry import DEFAULT_REGISTRY


class JavaJcmdJsonParserPlugin:
    """Parser for jcmd JSON thread dumps."""

    format_id: str = "java_jcmd_json"
    language: str = "java"

    def can_parse(self, head: str) -> bool:
        stripped = head.lstrip()
        return stripped.startswith("{") and '"threadDump"' in head

    def parse(self, path: Path) -> ThreadDumpBundle:
        encoding = detect_text_encoding(Path(path))
        try:
            with Path(path).open("r", encoding=encoding) as handle:
                payload = json.load(handle)
        except json.JSONDecodeError as exc:
            raise ValueError(
                f"Malformed java_jcmd_json thread dump in {path}: "
                f"line {exc.lineno}, column {exc.colno}: {exc.msg}."
            ) from exc
        if not isinstance(payload, dict) or "threadDump" not in payload:
            found = sorted(payload.keys()) if isinstance(payload, dict) else type(payload).__name__
            raise ValueError(
                "Unsupported java_jcmd_json thread dump: missing required "
                f"'threadDump' object. Found: {found!r}."
            )

        snapshots: list[ThreadSnapshot] = []
        for index, thread in enumerate(_iter_thread_objects(payload)):
            snapshots.append(_snapshot_from_thread(thread, index, self.format_id))
        if not snapshots:
            raise ValueError(
                "Unsupported java_jcmd_json thread dump: no thread objects were "
                "found under 'threadDump'."
            )

        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
            captured_at=_parse_timestamp(payload.get("time") or payload.get("timestamp")),
            metadata={
                "source": "jcmd_json",
                "thread_count": len(snapshots),
                "raw_timestamp": payload.get("time") or payload.get("timestamp"),
            },
        )


def _iter_thread_objects(payload: Any) -> Iterable[dict[str, Any]]:
    stack: list[Any] = [payload]
    while stack:
        item = stack.pop()
        if isinstance(item, dict):
            if _looks_like_thread(item):
                yield item
                continue
            stack.extend(reversed(list(item.values())))
        elif isinstance(item, list):
            stack.extend(reversed(item))


def _looks_like_thread(item: dict[str, Any]) -> bool:
    keys = set(item)
    return bool(
        {"name", "threadName"} & keys
        and ({"stack", "stackTrace", "frames"} & keys or {"state", "threadState"} & keys)
    )


def _snapshot_from_thread(
    thread: dict[str, Any],
    index: int,
    source_format: str,
) -> ThreadSnapshot:
    name = str(thread.get("name") or thread.get("threadName") or f"thread-{index}")
    thread_id = _first_text(thread, "tid", "nid", "threadId", "id")
    raw_state = _first_text(thread, "threadState", "state")
    frames = [_normalize_proxy_frame(frame) for frame in _frames_from_thread(thread)]
    state = _infer_java_state(ThreadState.coerce(raw_state), frames)
    raw_text = json.dumps(thread, ensure_ascii=False, separators=(",", ":"))
    lock_holds, lock_waiting = _extract_json_locks(thread, raw_text)
    if (
        lock_waiting is not None
        and lock_waiting.wait_mode == "lock_entry_wait"
        and state in (ThreadState.RUNNABLE, ThreadState.UNKNOWN)
    ):
        state = ThreadState.LOCK_WAIT
    elif lock_waiting is not None and state is ThreadState.UNKNOWN:
        state = ThreadState.WAITING

    metadata: dict[str, object] = {"source_fields": sorted(thread.keys())[:80]}
    if _is_virtual_json_thread(thread, raw_text):
        metadata["is_virtual_thread"] = True
    native_method = _native_method_from_frames(frames)
    if native_method:
        metadata["native_method"] = native_method
    pinning = _carrier_pinning(raw_text, frames)
    if pinning:
        metadata["carrier_pinning"] = pinning
    if lock_waiting is not None and lock_waiting.wait_mode:
        metadata["monitor_wait_mode"] = lock_waiting.wait_mode

    return ThreadSnapshot(
        snapshot_id=f"java_json::0::{index}::{name}",
        thread_name=name,
        thread_id=thread_id,
        state=state,
        category=state.value,
        stack_frames=frames,
        lock_info=None,
        metadata=metadata,
        language="java",
        source_format=source_format,
        lock_holds=lock_holds,
        lock_waiting=lock_waiting,
    )


def _first_text(thread: dict[str, Any], *keys: str) -> str | None:
    for key in keys:
        value = thread.get(key)
        if value is not None:
            return str(value)
    return None


def _frames_from_thread(thread: dict[str, Any]) -> list[StackFrame]:
    frames: list[StackFrame] = []
    for raw in thread.get("stack") or thread.get("stackTrace") or thread.get("frames") or []:
        frame = _frame_from_json_item(raw)
        if frame is not None:
            frames.append(frame)
    return frames


def _frame_from_json_item(item: Any) -> StackFrame | None:
    if isinstance(item, str):
        text = item.strip()
        if text.startswith("at "):
            text = text[3:]
        return _frame_from_jstack(text)
    if not isinstance(item, dict):
        return None

    class_name = _first_text(item, "className", "class", "declaringClass")
    method_name = _first_text(item, "methodName", "method", "function")
    file_name = _first_text(item, "fileName", "file")
    line_number = item.get("lineNumber") or item.get("line")
    line: int | None = None
    if isinstance(line_number, int):
        line = line_number
    elif line_number is not None:
        try:
            line = int(str(line_number))
        except ValueError:
            line = None
    if method_name is None and class_name is None:
        rendered = _first_text(item, "frame", "text")
        return _frame_from_jstack(rendered) if rendered else None
    return StackFrame(
        function=method_name or class_name or "(unknown)",
        module=class_name if method_name else None,
        file=file_name,
        line=line,
        language="java",
    )


def _extract_json_locks(
    thread: dict[str, Any],
    raw_text: str,
) -> tuple[list[LockHandle], LockHandle | None]:
    holds: list[LockHandle] = []
    seen: set[str] = set()
    for key in ("lockedMonitors", "lockedSynchronizers", "lockedOwnableSynchronizers"):
        for item in _as_list(thread.get(key)):
            lock = _lock_from_json(item, wait_mode="locked_owner")
            if lock is not None and lock.lock_id not in seen:
                seen.add(lock.lock_id)
                holds.append(lock)

    waiting = _lock_from_json(
        thread.get("lockInfo")
        or thread.get("blockedOn")
        or thread.get("waitingOn"),
        wait_mode="lock_entry_wait",
    )
    if waiting is None:
        text_holds, text_waiting = _extract_lock_handles(raw_text)
        for hold in text_holds:
            if hold.lock_id not in seen:
                seen.add(hold.lock_id)
                holds.append(hold)
        waiting = text_waiting
    if waiting is not None and waiting.lock_id in seen:
        waiting = None
    return holds, waiting


def _lock_from_json(
    item: Any,
    *,
    wait_mode: str | None = None,
) -> LockHandle | None:
    if not isinstance(item, dict):
        return None
    lock_class = _first_text(item, "className", "class", "type")
    raw_id = _first_text(item, "identityHashCode", "id", "address", "lockId")
    if raw_id is None and lock_class is None:
        return None
    lock_id = raw_id or lock_class or "unknown-lock"
    if raw_id is not None and raw_id.isdigit():
        lock_id = hex(int(raw_id))
    return LockHandle(lock_id=lock_id, lock_class=lock_class, wait_mode=wait_mode)


def _as_list(value: Any) -> list[Any]:
    if value is None:
        return []
    if isinstance(value, list):
        return value
    return [value]


def _is_virtual_json_thread(thread: dict[str, Any], raw_text: str) -> bool:
    if thread.get("virtual") is True:
        return True
    lowered = raw_text.lower()
    return "virtualthread" in lowered or "virtual thread" in lowered


def _native_method_from_frames(frames: list[StackFrame]) -> str | None:
    for frame in frames:
        rendered = frame.render()
        if "Native Method" in rendered:
            return rendered
    return None


def _parse_timestamp(value: Any) -> datetime | None:
    if value is None:
        return None
    text = str(value).strip()
    if not text:
        return None
    try:
        return datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        return None


DEFAULT_REGISTRY.register(JavaJcmdJsonParserPlugin())
