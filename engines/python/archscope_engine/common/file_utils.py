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
    probe_bytes: int = 1_048_576,
) -> str:
    if probe_bytes <= 0:
        raise ValueError("probe_bytes must be a positive integer.")

    with path.open("rb") as handle:
        data = handle.read(probe_bytes)
    return detect_text_encoding_from_bytes(data, encodings=encodings)


def detect_text_encoding_from_bytes(
    data: bytes,
    encodings: tuple[str, ...] = ("utf-8", "utf-8-sig", "cp949", "latin-1"),
) -> str:
    """Best-effort text encoding detection for parser head/probe bytes.

    Some JVM tools on Windows produce UTF-16 text dumps. Relying only on
    UTF-8/cp949/latin-1 makes the parser fail format detection because
    the header becomes ``F\0u\0l\0l...``. BOM and null-byte heuristics are
    cheap and keep this utility dependency-free.
    """
    if data.startswith((b"\xff\xfe", b"\xfe\xff")):
        return "utf-16"
    if data.startswith(b"\xef\xbb\xbf"):
        return "utf-8-sig"

    sample = data[:4096]
    if sample:
        even_nulls = sample[0::2].count(0)
        odd_nulls = sample[1::2].count(0)
        even_len = max(1, len(sample[0::2]))
        odd_len = max(1, len(sample[1::2]))
        if odd_nulls / odd_len > 0.30 and even_nulls / even_len < 0.05:
            return "utf-16-le"
        if even_nulls / even_len > 0.30 and odd_nulls / odd_len < 0.05:
            return "utf-16-be"

    last_error: UnicodeDecodeError | None = None

    for encoding in encodings:
        try:
            data.decode(encoding)
            return encoding
        except UnicodeDecodeError as exc:
            last_error = exc

    if last_error is not None:
        raise last_error

    raise ValueError("no encodings configured")


def iter_text_lines(path: Path, encoding: str | None = None) -> Iterable[str]:
    """Yield text lines using a small encoding fallback chain."""
    detected_encoding = encoding or detect_text_encoding(path)

    with path.open("r", encoding=detected_encoding) as handle:
        for line in handle:
            yield line.rstrip("\n")


def iter_text_lines_with_context(
    path: Path,
    encoding: str | None = None,
) -> Iterable[TextLineContext]:
    """Yield lines with one-line before/after context without materializing the file."""
    iterator = iter(iter_text_lines(path, encoding=encoding))
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
