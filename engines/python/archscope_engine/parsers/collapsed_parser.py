from __future__ import annotations

from collections import Counter
from dataclasses import dataclass
from math import isfinite
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


def parse_collapsed_files(paths: list[Path] | tuple[Path, ...]) -> Counter[str]:
    return parse_collapsed_files_with_diagnostics(paths).stacks


def parse_collapsed_file_with_diagnostics(
    path: Path,
    *,
    debug_log: DebugLogCollector | None = None,
    strict: bool = False,
) -> CollapsedParseResult:
    stacks: Counter[str] = Counter()
    diagnostics = ParserDiagnostics(source_file=str(path), format="async_profiler_collapsed")

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
            if strict:
                raise ValueError(f"{path}:{line_number}: {reason}: {message}")
            continue

        stack, samples = parsed
        if samples > 0:
            stacks[stack] += samples
        diagnostics.parsed_records += 1

    if diagnostics.total_lines == 0:
        diagnostics.add_warning(
            line_number=0,
            reason="EMPTY_FILE",
            message="Collapsed profiler file is empty.",
        )
    elif diagnostics.parsed_records == 0:
        diagnostics.add_warning(
            line_number=0,
            reason="NO_VALID_RECORDS",
            message="No valid collapsed profiler records were parsed.",
        )

    return CollapsedParseResult(stacks=stacks, diagnostics=diagnostics.to_dict())


def parse_collapsed_files_with_diagnostics(
    paths: list[Path] | tuple[Path, ...],
    *,
    strict: bool = False,
) -> CollapsedParseResult:
    stacks: Counter[str] = Counter()
    merged = ParserDiagnostics(
        source_file=";".join(str(path) for path in paths),
        format="async_profiler_collapsed",
    )
    for path in paths:
        result = parse_collapsed_file_with_diagnostics(path, strict=strict)
        stacks.update(result.stacks)
        _merge_diagnostics(merged, result.diagnostics)
    return CollapsedParseResult(stacks=stacks, diagnostics=merged.to_dict())


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

    samples, sample_error = _parse_sample_count(sample_text)
    if sample_error is not None:
        return None, sample_error

    if samples < 0:
        return None, ("NEGATIVE_SAMPLE_COUNT", "Sample count must be non-negative.")

    return (stack, samples), None


def _parse_sample_count(sample_text: str) -> tuple[int, ParseError | None]:
    try:
        return int(sample_text), None
    except ValueError:
        pass

    try:
        numeric = float(sample_text)
    except ValueError:
        return 0, ("INVALID_SAMPLE_COUNT", "Sample count must be an integer.")

    if not isfinite(numeric):
        return 0, ("INVALID_SAMPLE_COUNT", "Sample count must be finite.")
    if not numeric.is_integer():
        return 0, (
            "INVALID_SAMPLE_COUNT",
            "Sample count must be a whole number.",
        )
    return int(numeric), None


def _merge_diagnostics(target: ParserDiagnostics, source: dict[str, Any]) -> None:
    target.total_lines += int(source.get("total_lines") or 0)
    target.parsed_records += int(source.get("parsed_records") or 0)
    target.skipped_lines += int(source.get("skipped_lines") or 0)
    target.warning_count += int(source.get("warning_count") or 0)
    target.error_count += int(source.get("error_count") or 0)
    for reason, count in dict(source.get("skipped_by_reason") or {}).items():
        target.skipped_by_reason[str(reason)] = (
            target.skipped_by_reason.get(str(reason), 0) + int(count)
        )
    for key in ("samples", "warnings", "errors"):
        target_list = getattr(target, key)
        for item in list(source.get(key) or []):
            if len(target_list) >= 100:
                break
            if isinstance(item, dict):
                target_list.append(dict(item))


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
