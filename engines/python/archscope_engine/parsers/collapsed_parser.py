from __future__ import annotations

from collections import Counter
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from archscope_engine.common.diagnostics import ParserDiagnostics, ParseError
from archscope_engine.common.file_utils import iter_text_lines

@dataclass(frozen=True)
class CollapsedParseResult:
    stacks: Counter[str]
    diagnostics: dict[str, Any]


def parse_collapsed_file(path: Path) -> Counter[str]:
    return parse_collapsed_file_with_diagnostics(path).stacks


def parse_collapsed_file_with_diagnostics(path: Path) -> CollapsedParseResult:
    stacks: Counter[str] = Counter()
    diagnostics = ParserDiagnostics()

    for line_number, line in enumerate(iter_text_lines(path), start=1):
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
