from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.runtime_stack import RuntimeStackRecord


NODE_ERROR_RE = re.compile(
    r"^(?:(?P<timestamp>\d{4}-\d{2}-\d{2}[T ][^ ]+)\s+)?"
    r"(?P<type>(?:Error|Exception)|[A-Za-z_$][\w.$]*(?:Error|Exception))"
    r"(?::\s*(?P<message>.*))?$"
)


def parse_nodejs_stack(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
    debug_log: DebugLogCollector | None = None,
) -> list[RuntimeStackRecord]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    blocks: list[tuple[int, list[str]]] = []
    current: list[str] = []
    current_start = 0

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        own_diagnostics.total_lines += 1
        stripped = line.strip()
        if NODE_ERROR_RE.match(stripped):
            if current:
                blocks.append((current_start, current))
            current = [stripped]
            current_start = line_number
        elif current and (stripped.startswith("at ") or stripped.startswith("Caused by:")):
            current.append(stripped)
        elif current and stripped:
            current.append(stripped)
        elif stripped:
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="NO_NODEJS_ERROR_HEADER",
                message="Line did not start a supported Node.js stack block.",
                raw_line=line,
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=line_number,
                    reason="NO_NODEJS_ERROR_HEADER",
                    message="Line did not start a supported Node.js stack block.",
                    raw_context={"before": None, "target": line, "after": None},
                    failed_pattern="NODEJS_ERROR_HEADER",
                    field_shapes=infer_field_shapes(stripped),
                )

    if current:
        blocks.append((current_start, current))

    records: list[RuntimeStackRecord] = []
    for line_number, block in blocks:
        record = _parse_block(block)
        if record is None:
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="INVALID_NODEJS_STACK",
                message="Node.js stack block was missing a supported header.",
                raw_line="\n".join(block),
            )
            continue
        records.append(record)
        own_diagnostics.parsed_records += 1
    return records


def _parse_block(block: list[str]) -> RuntimeStackRecord | None:
    header = NODE_ERROR_RE.match(block[0])
    if header is None:
        return None
    stack = [line[3:] for line in block[1:] if line.startswith("at ")]
    error_type = header.group("type")
    top_frame = stack[0] if stack else "(no-frame)"
    return RuntimeStackRecord(
        runtime="nodejs",
        record_type=error_type,
        headline=error_type,
        message=header.group("message"),
        stack=stack,
        signature=f"{error_type}|{top_frame}",
        raw_block="\n".join(block),
    )
