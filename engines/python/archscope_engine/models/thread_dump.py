from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True)
class ThreadDumpRecord:
    thread_name: str
    thread_id: str | None
    state: str | None
    stack: list[str]
    lock_info: str | None
    category: str | None
    raw_block: str


@dataclass(frozen=True)
class ExceptionRecord:
    timestamp: datetime | None
    language: str
    exception_type: str
    message: str | None
    root_cause: str | None
    stack: list[str]
    signature: str
    raw_block: str
