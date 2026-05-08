"""Go runtime panic + goroutine dump parser."""
# ─────────────────────────────────────────────────────────────────────
# [한글] go_panic_parser — Go 런타임 패닉/goroutine 덤프 파서.
#
# 입력 형식
#   `panic: <msg>` 라인이 panic block 시작.
#   `goroutine N [state]:` 라인이 goroutine block 시작.
#   각 block 안에는 `func.name(args)` + 다음 줄에 `\tfile:line +0xNN`
#   pair 가 반복.
#
# 알고리즘
#   1) 라인 단위로 panic / goroutine block 경계 인식.
#   2) block 안에서 GO_FUNC_RE 패턴 매칭한 라인은 stack frame.
#   3) 한 block = 한 RuntimeStackRecord (kind="panic"|"goroutine").
#   4) state ("running", "chan receive", "IO wait" 등) 도 보존.
#
# parity: PANIC_RE / GOROUTINE_RE / GO_FUNC_RE 정규식, kind 라벨이
# Go engine-native internal/parsers/runtimestack/gopanic.go 와 동일.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.models.runtime_stack import RuntimeStackRecord


PANIC_RE = re.compile(r"^panic:\s*(?P<message>.+)$")
GOROUTINE_RE = re.compile(r"^goroutine\s+(?P<id>\d+)\s+\[(?P<state>[^\]]+)\]:$")
GO_FUNC_RE = re.compile(r"^(?P<func>[\w./*()$-]+)\(.*\)$")


def parse_go_panic(
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
        if PANIC_RE.match(stripped) or GOROUTINE_RE.match(stripped):
            if current:
                blocks.append((current_start, current))
            current = [stripped]
            current_start = line_number
        elif current and stripped:
            current.append(stripped)
        elif stripped:
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="OUTSIDE_GO_PANIC",
                message="Line was outside a supported Go panic or goroutine block.",
                raw_line=line,
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=line_number,
                    reason="OUTSIDE_GO_PANIC",
                    message="Line was outside a supported Go panic or goroutine block.",
                    raw_context={"before": None, "target": line, "after": None},
                    failed_pattern="GO_PANIC_OR_GOROUTINE_HEADER",
                    field_shapes=infer_field_shapes(stripped),
                )

    if current:
        blocks.append((current_start, current))

    records: list[RuntimeStackRecord] = []
    for _, block in blocks:
        record = _parse_block(block)
        if record is not None:
            records.append(record)
            own_diagnostics.parsed_records += 1
    return records


def _parse_block(block: list[str]) -> RuntimeStackRecord | None:
    header = block[0]
    panic = PANIC_RE.match(header)
    goroutine = GOROUTINE_RE.match(header)
    if panic is None and goroutine is None:
        return None

    stack: list[str] = []
    for line in block[1:]:
        match = GO_FUNC_RE.match(line)
        if match and not line.startswith("\t"):
            stack.append(match.group("func"))

    record_type = "panic" if panic else "goroutine"
    message = panic.group("message") if panic else goroutine.group("state")
    top_frame = stack[0] if stack else "(no-frame)"
    return RuntimeStackRecord(
        runtime="go",
        record_type=record_type,
        headline=header,
        message=message,
        stack=stack,
        signature=f"{record_type}|{top_frame}",
        raw_block="\n".join(block),
    )
