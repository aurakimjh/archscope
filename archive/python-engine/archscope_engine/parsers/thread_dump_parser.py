"""Legacy single-file Java jstack thread-dump parser."""
# ─────────────────────────────────────────────────────────────────────
# [한글] thread_dump_parser — legacy 단일 Java jstack 파서.
#
# 책임/목적
#   하나의 Java jstack 파일을 ThreadDumpRecord 리스트로 변환. 다중
#   런타임/플러그인 등록 시스템은 parsers/thread_dump/ 패키지가 담당
#   하며, 이 모듈은 thread_dump_analyzer 가 사용하는 단순 경로.
#
# 입력 형식 (jstack)
#   `"thread_name" #N daemon prio=... tid=0xABC nid=0xDEF state` 헤더 +
#   다음 줄에 `   java.lang.Thread.State: STATE` + 들여쓰기된 stack frame.
#   빈 라인이 record 구분자.
#
# 알고리즘
#   1) THREAD_HEADER_RE 매칭으로 record 시작.
#   2) record 안에서 TID_RE / STATE_RE 매칭으로 메타 추출.
#   3) "  at ..." 같은 들여쓰기 라인은 stack frame 으로 누적.
#   4) lock/monitor 정보 라인 ("- locked", "- waiting on") 도 캡처.
#
# parity: 정규식 / record 키가 Go 측 internal/parsers/threaddump
# (또는 javajstack 플러그인) 과 동일. legacy 경로지만 frontend 호환
# 위해 유지.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
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
    debug_log: DebugLogCollector | None = None,
) -> list[ThreadDumpRecord]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    own_diagnostics.set_context(source_file=str(path), format="java_thread_dump")
    records: list[ThreadDumpRecord] = []
    current: list[str] = []
    current_start = 0

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        own_diagnostics.total_lines += 1
        if THREAD_HEADER_RE.match(line):
            if current:
                _append_thread_record(
                    records,
                    current,
                    line_number=current_start,
                    diagnostics=own_diagnostics,
                    debug_log=debug_log,
                )
            current = [line]
            current_start = line_number
        elif current:
            current.append(line)
        elif line.strip():
            previous_line = None
            own_diagnostics.add_skipped(
                line_number=line_number,
                reason="OUTSIDE_THREAD_BLOCK",
                message="Line was outside a supported Java thread block.",
                raw_line=line,
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=line_number,
                    reason="OUTSIDE_THREAD_BLOCK",
                    message="Line was outside a supported Java thread block.",
                    raw_context={"before": previous_line, "target": line, "after": None},
                    failed_pattern="JAVA_THREAD_QUOTED_BLOCK",
                    field_shapes=infer_field_shapes(line),
                )

    if current:
        _append_thread_record(
            records,
            current,
            line_number=current_start,
            diagnostics=own_diagnostics,
            debug_log=debug_log,
        )

    if own_diagnostics.total_lines == 0:
        own_diagnostics.add_warning(
            line_number=0,
            reason="EMPTY_FILE",
            message="Thread dump file is empty.",
        )
    elif own_diagnostics.parsed_records == 0:
        own_diagnostics.add_warning(
            line_number=0,
            reason="NO_THREAD_BLOCKS",
            message="No supported Java thread blocks were parsed.",
        )

    return records


def _append_thread_record(
    records: list[ThreadDumpRecord],
    block: list[str],
    *,
    line_number: int,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
) -> None:
    record = parse_thread_block(block)
    if record is None:
        diagnostics.add_skipped(
            line_number=line_number,
            reason="INVALID_THREAD_BLOCK",
            message="Thread block was missing a quoted header.",
            raw_line="\n".join(block),
        )
        if debug_log is not None:
            debug_log.add_parse_error(
                line_number=line_number,
                reason="INVALID_THREAD_BLOCK",
                message="Thread block was missing a quoted header.",
                raw_context={
                    "before": None,
                    "target": "\n".join(block),
                    "after": None,
                },
                failed_pattern="JAVA_THREAD_QUOTED_BLOCK",
                field_shapes={"block_line_count": len(block)},
            )
        return
    records.append(record)
    diagnostics.parsed_records += 1


def parse_thread_block(block: list[str]) -> ThreadDumpRecord | None:
    """Parse one quoted JVM thread block.

    Exposed for the plugin-based parser so it can split a single physical
    file into multiple logical dumps without duplicating block parsing.
    """
    if not block:
        return None
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
            for token in ("waiting to lock", "waiting on", "locked", "parking to wait")
        ):
            lock_info = stripped

    if state is None:
        state = _state_from_header(header.group("rest"))

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


def _parse_thread_block(block: list[str]) -> ThreadDumpRecord | None:
    return parse_thread_block(block)


def _state_from_header(rest: str) -> str | None:
    text = rest.lower()
    if "waiting for monitor entry" in text or " blocked" in text:
        return "BLOCKED"
    if "timed_waiting" in text or "timed waiting" in text:
        return "TIMED_WAITING"
    if "waiting on condition" in text or "parking" in text or "object.wait" in text:
        return "WAITING"
    if "runnable" in text or "running" in text:
        return "RUNNABLE"
    return None


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
