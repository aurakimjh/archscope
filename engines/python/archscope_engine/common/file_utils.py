from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


@dataclass(frozen=True)
class TextLineContext:
    line_number: int
    before: str | None
    target: str
    after: str | None


def detect_text_encoding(
    path: Path,
    encodings: tuple[str, ...] = ("utf-8", "utf-8-sig", "cp949", "latin-1"),
) -> str:
    last_error: UnicodeDecodeError | None = None

    for encoding in encodings:
        try:
            with path.open("r", encoding=encoding) as handle:
                while handle.read(8192):
                    pass
            return encoding
        except UnicodeDecodeError as exc:
            last_error = exc

    if last_error is not None:
        raise last_error

    raise ValueError("no encodings configured")


def iter_text_lines(path: Path) -> Iterable[str]:
    """Yield text lines using a small encoding fallback chain."""
    encoding = detect_text_encoding(path)

    with path.open("r", encoding=encoding) as handle:
        for line in handle:
            yield line.rstrip("\n")


def iter_text_lines_with_context(path: Path) -> Iterable[TextLineContext]:
    """Yield lines with one-line before/after context without materializing the file."""
    iterator = iter(iter_text_lines(path))
    previous: str | None = None
    try:
        current = next(iterator)
    except StopIteration:
        return

    line_number = 1
    for next_line in iterator:
        yield TextLineContext(
            line_number=line_number,
            before=previous,
            target=current,
            after=next_line,
        )
        previous = current
        current = next_line
        line_number += 1

    yield TextLineContext(
        line_number=line_number,
        before=previous,
        target=current,
        after=None,
    )
