from __future__ import annotations

from collections import Counter
from pathlib import Path

from archscope_engine.common.file_utils import iter_text_lines


def parse_collapsed_file(path: Path) -> Counter[str]:
    stacks: Counter[str] = Counter()
    for line in iter_text_lines(path):
        stripped = line.strip()
        if not stripped:
            continue
        stack, samples = parse_collapsed_line(stripped)
        stacks[stack] += samples
    return stacks


def parse_collapsed_line(line: str) -> tuple[str, int]:
    stack, sample_text = line.rsplit(maxsplit=1)
    samples = int(sample_text)
    if samples < 0:
        raise ValueError("Sample count must be non-negative.")
    return stack, samples
