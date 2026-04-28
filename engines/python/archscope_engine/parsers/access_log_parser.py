from __future__ import annotations

import re
from pathlib import Path

from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.common.time_utils import parse_nginx_timestamp
from archscope_engine.models.access_log import AccessLogRecord

NGINX_WITH_RESPONSE_TIME = re.compile(
    r'^(?P<client_ip>\S+) \S+ \S+ \[(?P<timestamp>[^\]]+)\] '
    r'"(?P<method>\S+) (?P<uri>\S+) (?P<protocol>[^"]+)" '
    r"(?P<status>\d{3}) (?P<bytes_sent>\d+|-) "
    r'"(?P<referer>[^"]*)" "(?P<user_agent>[^"]*)" '
    r"(?P<response_time_sec>[0-9.]+)$"
)


def parse_access_log(path: Path, log_format: str = "nginx") -> list[AccessLogRecord]:
    if log_format.lower() != "nginx":
        raise ValueError("Only nginx format is implemented in the skeleton parser.")

    records: list[AccessLogRecord] = []
    for line in iter_text_lines(path):
        if not line.strip():
            continue
        record = parse_nginx_access_line(line)
        if record is not None:
            records.append(record)
    return records


def parse_nginx_access_line(line: str) -> AccessLogRecord | None:
    match = NGINX_WITH_RESPONSE_TIME.match(line)
    if match is None:
        return None

    groups = match.groupdict()
    bytes_sent = 0 if groups["bytes_sent"] == "-" else int(groups["bytes_sent"])
    return AccessLogRecord(
        timestamp=parse_nginx_timestamp(groups["timestamp"]),
        method=groups["method"],
        uri=groups["uri"],
        status=int(groups["status"]),
        response_time_ms=float(groups["response_time_sec"]) * 1000,
        bytes_sent=bytes_sent,
        client_ip=groups["client_ip"],
        user_agent=groups["user_agent"],
        referer=groups["referer"],
        raw_line=line,
    )
