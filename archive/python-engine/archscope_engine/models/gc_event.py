"""HotSpot GC log event dataclass."""
# [한글] gc_event.GcEvent — HotSpot JVM GC 로그 한 이벤트.
# parser 가 unified/G1-legacy/legacy 세 형식 어느 것이든 같은 record
# 로 정규화. 필드는 모두 Optional — 형식별로 일부 정보가 없을 수 있음.
# 시간 필드: timestamp (절대시각) / uptime_sec (JVM 시작 후 경과).
# parity: Go engine-native internal/models 의 GcEvent 와 동일.
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
