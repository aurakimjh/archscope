from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True)
class GcEvent:
    timestamp: datetime | None
    uptime_sec: float | None
    gc_type: str | None
    cause: str | None
    pause_ms: float | None
    heap_before_mb: float | None
    heap_after_mb: float | None
    heap_committed_mb: float | None
    young_before_mb: float | None
    young_after_mb: float | None
    old_before_mb: float | None
    old_after_mb: float | None
    metaspace_before_mb: float | None
    metaspace_after_mb: float | None
    raw_line: str
