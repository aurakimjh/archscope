from __future__ import annotations

from datetime import datetime


NGINX_TIME_FORMAT = "%d/%b/%Y:%H:%M:%S %z"


def parse_nginx_timestamp(value: str) -> datetime:
    return datetime.strptime(value, NGINX_TIME_FORMAT)


def minute_bucket(value: datetime) -> str:
    return value.strftime("%Y-%m-%dT%H:%M:00%z")
