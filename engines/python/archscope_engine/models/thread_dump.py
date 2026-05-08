"""Legacy thread-dump and exception records (Java jstack)."""
# [한글] thread_dump — 레거시 단일 Java jstack 분석기용 record.
# ThreadDumpRecord: 1개 스레드의 정보 (name/id/state/stack/lock_info
# /category/raw_block). 다언어 통합 모델은 thread_snapshot.* 를 사용.
# ExceptionRecord: 예외 분석기용 (Caused by 체인 포함).
# parity: Go engine-native internal/models 와 동일.
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
