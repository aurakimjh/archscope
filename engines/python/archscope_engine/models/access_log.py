from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(frozen=True)
class AccessLogRecord:
    timestamp: datetime
    method: str
    uri: str
    status: int
    response_time_ms: float
    bytes_sent: int
    client_ip: str
    user_agent: str
    referer: str
    raw_line: str
