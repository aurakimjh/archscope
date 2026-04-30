from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class OTelLogRecord:
    timestamp: str | None
    trace_id: str | None
    span_id: str | None
    service_name: str | None
    severity: str
    body: str
    raw_line: str
