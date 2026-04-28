from __future__ import annotations

from pathlib import Path
from typing import Iterable


def iter_text_lines(path: Path) -> Iterable[str]:
    """Yield text lines using a small encoding fallback chain."""
    encodings = ("utf-8", "utf-8-sig", "cp949", "latin-1")
    last_error: UnicodeDecodeError | None = None

    for encoding in encodings:
        try:
            with path.open("r", encoding=encoding) as handle:
                for line in handle:
                    yield line.rstrip("\n")
            return
        except UnicodeDecodeError as exc:
            last_error = exc

    if last_error is not None:
        raise last_error
