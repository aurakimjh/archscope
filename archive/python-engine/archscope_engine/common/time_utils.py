"""Time parsing / bucketing helpers."""
# [한글] time_utils — 시간 변환 헬퍼.
# parse_nginx_timestamp: "27/Apr/2026:10:00:01 +0900" → tz-aware datetime.
# minute_bucket: datetime → "YYYY-MM-DDTHH:MM:00+TZ" 분 단위 버킷 키.
# parity: NGINX_TIME_FORMAT, minute_bucket 형식이 Go internal/timeutil 동일.
from __future__ import annotations

from datetime import datetime


NGINX_TIME_FORMAT = "%d/%b/%Y:%H:%M:%S %z"


def parse_nginx_timestamp(value: str) -> datetime:
    return datetime.strptime(value, NGINX_TIME_FORMAT)


def minute_bucket(value: datetime) -> str:
    return value.strftime("%Y-%m-%dT%H:%M:00%z")
