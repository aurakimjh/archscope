"""Access log record dataclass."""
# [한글] access_log.AccessLogRecord — nginx/apache/IIS 액세스 로그
# 한 라인을 정규화한 record. parser 가 채우고, analyzer 가 통계 산출.
# 필드: timestamp / method / uri / status / response_time_ms /
# bytes_sent / client_ip / user_agent / referer / raw_line.
# parity: Go engine-native internal/models/access_log.go 와 동일.
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
