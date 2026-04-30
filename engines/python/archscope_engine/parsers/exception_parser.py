from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.thread_dump import ExceptionRecord


EXCEPTION_LINE_RE = re.compile(
    r"^(?:(?P<timestamp>\d{4}-\d{2}-\d{2}[T ][^ ]+)\s+)?"
    r"(?:(?P<caused>Caused by:\s+))?"
    r"(?P<type>[A-Za-z_$][\w.$]*(?:Exception|Error|Throwable))"
    r"(?::\s*(?P<message>.*))?$"
)


def parse_exception_stack(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
) -> list[ExceptionRecord]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    blocks: list[tuple[int, list[str]]] = []
    current: list[str] = []
    current_start = 0

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        own_diagnostics.total_lines += 1
        stripped = line.strip()
        if EXCEPTION_LINE_RE.match(stripped) and not stripped.startswith("Caused by:"):
            if current:
                blocks.append((current_start, current))
            current = [stripped]
            current_start = line_number
        elif current and (
            stripped.startswith("at ") or stripped.startswith("Caused by:")
        ):
            current.append(stripped)
        elif current and stripped:
            current.append(stripped)
        elif not current and stripped:
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="NO_EXCEPTION_HEADER",
                message="Line did not start a supported exception stack block.",
                raw_line=line,
            )

    if current:
        blocks.append((current_start, current))

    records: list[ExceptionRecord] = []
    for line_number, block in blocks:
        record = _parse_exception_block(block)
        if record is None:
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="INVALID_EXCEPTION_BLOCK",
                message="Exception block was missing a supported exception header.",
                raw_line="\n".join(block),
            )
            continue
        records.append(record)
        own_diagnostics.parsed_records += 1

    return records


def _parse_exception_block(block: list[str]) -> ExceptionRecord | None:
    header = EXCEPTION_LINE_RE.match(block[0])
    if header is None:
        return None

    root_type = header.group("type")
    root_message = header.group("message")
    caused_by: list[tuple[str, str | None]] = []
    stack: list[str] = []

    for line in block[1:]:
        if line.startswith("at "):
            stack.append(line[3:])
            continue
        caused_match = EXCEPTION_LINE_RE.match(line)
        if caused_match and caused_match.group("caused"):
            caused_by.append((caused_match.group("type"), caused_match.group("message")))

    root_cause = caused_by[-1][0] if caused_by else None
    signature_frame = stack[0] if stack else "(no-frame)"
    return ExceptionRecord(
        timestamp=_parse_timestamp(header.group("timestamp")),
        language="java",
        exception_type=root_type,
        message=root_message,
        root_cause=root_cause,
        stack=stack,
        signature=f"{root_type}|{signature_frame}",
        raw_block="\n".join(block),
    )


def _parse_timestamp(value: str | None):
    from datetime import datetime

    if not value:
        return None
    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None
