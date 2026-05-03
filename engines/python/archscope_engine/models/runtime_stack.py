from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class RuntimeStackRecord:
    runtime: str
    record_type: str
    headline: str
    message: str | None
    stack: list[str]
    signature: str
    raw_block: str


@dataclass(frozen=True)
class IisAccessRecord:
    method: str
    uri: str
    status: int
    time_taken_ms: int | None
    raw_line: str
