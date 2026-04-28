from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class ProfileStack:
    stack: str
    frames: list[str]
    samples: int
    estimated_seconds: float
    sample_ratio: float
    elapsed_ratio: float | None
    category: str | None = None
