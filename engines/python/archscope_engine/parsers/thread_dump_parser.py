from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.thread_dump import ThreadDumpRecord


THREAD_HEADER_RE = re.compile(r'^"(?P<name>[^"]+)"(?P<rest>.*)$')
TID_RE = re.compile(r"\b(?:tid|nid)=(?P<tid>0x[0-9a-fA-F]+|\S+)")
STATE_RE = re.compile(r"java\.lang\.Thread\.State:\s+(?P<state>[A-Z_]+)")


def parse_thread_dump(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
) -> list[ThreadDumpRecord]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    blocks: list[tuple[int, list[str]]] = []
    current: list[str] = []
    current_start = 0

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        own_diagnostics.total_lines += 1
        if THREAD_HEADER_RE.match(line):
            if current:
                blocks.append((current_start, current))
            current = [line]
            current_start = line_number
        elif current:
            current.append(line)
        elif line.strip():
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="OUTSIDE_THREAD_BLOCK",
                message="Line was outside a supported Java thread block.",
                raw_line=line,
            )

    if current:
        blocks.append((current_start, current))

    records: list[ThreadDumpRecord] = []
    for line_number, block in blocks:
        record = _parse_thread_block(block)
        if record is None:
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="INVALID_THREAD_BLOCK",
                message="Thread block was missing a quoted header.",
                raw_line="\n".join(block),
            )
            continue
        records.append(record)
        own_diagnostics.parsed_records += 1

    return records


def _parse_thread_block(block: list[str]) -> ThreadDumpRecord | None:
    header = THREAD_HEADER_RE.match(block[0])
    if header is None:
        return None

    state = None
    stack: list[str] = []
    lock_info = None
    for line in block[1:]:
        stripped = line.strip()
        state_match = STATE_RE.search(stripped)
        if state_match:
            state = state_match.group("state")
            continue
        if stripped.startswith("at "):
            stack.append(stripped[3:])
        elif any(
            token in stripped
            for token in ("waiting to lock", "locked", "parking to wait")
        ):
            lock_info = stripped

    tid_match = TID_RE.search(block[0])
    return ThreadDumpRecord(
        thread_name=header.group("name"),
        thread_id=tid_match.group("tid") if tid_match else None,
        state=state,
        stack=stack,
        lock_info=lock_info,
        category=_category_for_state(state),
        raw_block="\n".join(block),
    )


def _category_for_state(state: str | None) -> str:
    if state == "RUNNABLE":
        return "RUNNABLE"
    if state == "BLOCKED":
        return "BLOCKED"
    if state in {"WAITING", "TIMED_WAITING"}:
        return "WAITING"
    if state == "NEW":
        return "NEW"
    if state == "TERMINATED":
        return "TERMINATED"
    return "UNKNOWN"
