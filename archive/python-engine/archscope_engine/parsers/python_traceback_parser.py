"""Python traceback parser (CPython 3.x format)."""
# ─────────────────────────────────────────────────────────────────────
# [한글] python_traceback_parser — CPython traceback 텍스트 파서.
#
# 입력 형식 (CPython 3.x 표준)
#   `Traceback (most recent call last):` 라인이 block 시작.
#   각 frame 은 `  File "path/to/file.py", line N, in func_name`
#   + 선택적으로 다음 줄에 source line.
#   block 마지막 라인은 `ExceptionType: message`.
#
# 알고리즘
#   1) 라인 단위 dispatch.
#   2) Traceback 헤더로 block 시작, exception 라인으로 block 종료.
#   3) 각 frame 은 PY_FILE_RE 매칭으로 (file, line, func) 추출.
#   4) "Caused by:" 대신 chained traceback ("During handling..." /
#      "The above exception...") 도 multi-block 으로 처리.
#
# parity: PY_EXCEPTION_RE / PY_FILE_RE 정규식, frame 키 이름이
# Go engine-native internal/parsers/runtimestack/pythontraceback.go 와 동일.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.runtime_stack import RuntimeStackRecord


PY_EXCEPTION_RE = re.compile(
    r"^(?P<type>[A-Za-z_][\w.]*Error|[A-Za-z_][\w.]*Exception)"
    r":?\s*(?P<message>.*)$"
)
PY_FILE_RE = re.compile(r'^\s*File "(?P<file>[^"]+)", line (?P<line>\d+), in (?P<func>.+)$')


def parse_python_traceback(
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
        stripped = line.rstrip()
        if stripped.startswith("Traceback (most recent call last):"):
            if current:
                blocks.append((current_start, current))
            current = [stripped]
            current_start = line_number
        elif current:
            current.append(stripped)
        elif stripped:
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="OUTSIDE_PYTHON_TRACEBACK",
                message="Line was outside a Python traceback block.",
                raw_line=line,
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=line_number,
                    reason="OUTSIDE_PYTHON_TRACEBACK",
                    message="Line was outside a Python traceback block.",
                    raw_context={"before": None, "target": line, "after": None},
                    failed_pattern="PYTHON_TRACEBACK_HEADER",
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
                reason="INVALID_PYTHON_TRACEBACK",
                message="Python traceback block was missing a terminal exception line.",
                raw_line="\n".join(block),
            )
            continue
        records.append(record)
        own_diagnostics.parsed_records += 1
    return records


def _parse_block(block: list[str]) -> RuntimeStackRecord | None:
    exception_match = None
    for line in reversed(block):
        exception_match = PY_EXCEPTION_RE.match(line.strip())
        if exception_match:
            break
    if exception_match is None:
        return None

    stack: list[str] = []
    for line in block:
        frame = PY_FILE_RE.match(line)
        if frame:
            stack.append(
                f"{frame.group('file')}:{frame.group('line')} in {frame.group('func')}"
            )
    error_type = exception_match.group("type")
    top_frame = stack[-1] if stack else "(no-frame)"
    return RuntimeStackRecord(
        runtime="python",
        record_type=error_type,
        headline=error_type,
        message=exception_match.group("message") or None,
        stack=stack,
        signature=f"{error_type}|{top_frame}",
        raw_block="\n".join(block),
    )
