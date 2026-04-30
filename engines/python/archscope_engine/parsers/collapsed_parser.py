from __future__ import annotations

from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics, ParseError
from archscope_engine.common.file_utils import iter_text_lines, iter_text_lines_with_context

@dataclass(frozen=True)
class CollapsedParseResult:
    stacks: Counter[str]
    diagnostics: dict[str, Any]


def parse_collapsed_file(path: Path) -> Counter[str]:
    return parse_collapsed_file_with_diagnostics(path).stacks


def parse_collapsed_file_with_diagnostics(
    path: Path,
    *,
    debug_log: DebugLogCollector | None = None,
) -> CollapsedParseResult:
    stacks: Counter[str] = Counter()
    diagnostics = ParserDiagnostics()

    line_iterable = (
        iter_text_lines_with_context(path)
        if debug_log is not None
        else _line_contexts_without_neighbors(path)
    )
    for context in line_iterable:
        line_number = context.line_number
        line = context.target
        diagnostics.total_lines += 1
        stripped = line.strip()
        if not stripped:
            continue

        parsed, error = _parse_collapsed_line(stripped)
        if parsed is None:
            if error is None:
                raise RuntimeError("collapsed parser returned neither record nor error")
            reason, message = error
            diagnostics.add_skipped(
                line_number=line_number,
                reason=reason,
                message=message,
                raw_line=line,
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=line_number,
                    reason=reason,
                    message=message,
                    raw_context={
                        "before": context.before,
                        "target": line,
                        "after": context.after,
                    },
                    partial_match=_partial_match(stripped, reason),
                    failed_pattern="COLLAPSED_STACK_WITH_SAMPLE_COUNT",
                    field_shapes=infer_field_shapes(stripped),
                )
            continue

        stack, samples = parsed
        stacks[stack] += samples
        diagnostics.parsed_records += 1

    return CollapsedParseResult(stacks=stacks, diagnostics=diagnostics.to_dict())


def parse_collapsed_line(line: str) -> tuple[str, int]:
    parsed, error = _parse_collapsed_line(line.strip())
    if parsed is not None:
        return parsed

    if error is None:
        raise ValueError("Collapsed line could not be parsed.")
    reason, message = error
    raise ValueError(f"{reason}: {message}")


def _parse_collapsed_line(line: str) -> tuple[tuple[str, int] | None, ParseError | None]:
    try:
        stack, sample_text = line.rsplit(maxsplit=1)
    except ValueError:
        return None, (
            "MISSING_SAMPLE_COUNT",
            "Line must contain a stack and trailing sample count.",
        )

    if not stack.strip():
        return None, (
            "MISSING_SAMPLE_COUNT",
            "Line must contain a stack and trailing sample count.",
        )

    try:
        samples = int(sample_text)
    except ValueError:
        return None, ("INVALID_SAMPLE_COUNT", "Sample count must be an integer.")

    if samples < 0:
        return None, ("NEGATIVE_SAMPLE_COUNT", "Sample count must be non-negative.")

    return (stack, samples), None


def _partial_match(line: str, reason: str) -> dict[str, Any] | None:
    parts = line.rsplit(maxsplit=1)
    if len(parts) != 2:
        return None
    stack, sample_text = parts
    return {
        "matched_up_to": "stack" if reason != "MISSING_SAMPLE_COUNT" else None,
        "stack_frame_count": len(stack.split(";")) if stack else 0,
        "sample_text": sample_text,
    }


def _line_contexts_without_neighbors(path: Path):
    from archscope_engine.common.file_utils import TextLineContext

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        yield TextLineContext(
            line_number=line_number,
            before=None,
            target=line,
            after=None,
        )
