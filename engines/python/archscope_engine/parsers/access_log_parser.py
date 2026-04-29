from __future__ import annotations

import re
from dataclasses import dataclass, field
from datetime import datetime
from math import isfinite
from pathlib import Path
from typing import Any, Iterable

from archscope_engine.common.file_utils import iter_text_lines
from archscope_engine.common.time_utils import parse_nginx_timestamp
from archscope_engine.models.access_log import AccessLogRecord

NGINX_WITH_RESPONSE_TIME = re.compile(
    r'^(?P<client_ip>\S+) \S+ \S+ \[(?P<timestamp>[^\]]+)\] '
    r'"(?P<method>\S+) (?P<uri>\S+) (?P<protocol>[^"]+)" '
    r"(?P<status>\S+) (?P<bytes_sent>\S+) "
    r'"(?P<referer>[^"]*)" "(?P<user_agent>[^"]*)" '
    r"(?P<response_time_sec>\S+)$"
)

MAX_DIAGNOSTIC_SAMPLES = 20
RAW_PREVIEW_LIMIT = 200
ParseError = tuple[str, str]


@dataclass
class ParserDiagnostics:
    total_lines: int = 0
    parsed_records: int = 0
    skipped_lines: int = 0
    skipped_by_reason: dict[str, int] = field(default_factory=dict)
    samples: list[dict[str, int | str]] = field(default_factory=list)

    def add_skipped(
        self,
        *,
        line_number: int,
        reason: str,
        message: str,
        raw_line: str,
    ) -> None:
        self.skipped_lines += 1
        self.skipped_by_reason[reason] = self.skipped_by_reason.get(reason, 0) + 1

        if len(self.samples) >= MAX_DIAGNOSTIC_SAMPLES:
            return

        self.samples.append(
            {
                "line_number": line_number,
                "reason": reason,
                "message": message,
                "raw_preview": raw_line[:RAW_PREVIEW_LIMIT],
            }
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "total_lines": self.total_lines,
            "parsed_records": self.parsed_records,
            "skipped_lines": self.skipped_lines,
            "skipped_by_reason": dict(self.skipped_by_reason),
            "samples": list(self.samples),
        }


@dataclass(frozen=True)
class AccessLogParseResult:
    records: list[AccessLogRecord]
    diagnostics: dict[str, Any]


def parse_access_log(
    path: Path,
    log_format: str = "nginx",
    max_lines: int | None = None,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
) -> list[AccessLogRecord]:
    return parse_access_log_with_diagnostics(
        path,
        log_format,
        max_lines=max_lines,
        start_time=start_time,
        end_time=end_time,
    ).records


def parse_access_log_with_diagnostics(
    path: Path,
    log_format: str = "nginx",
    max_lines: int | None = None,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
) -> AccessLogParseResult:
    records: list[AccessLogRecord] = []
    diagnostics = ParserDiagnostics()
    for record in iter_access_log_records_with_diagnostics(
        path,
        log_format,
        diagnostics=diagnostics,
        max_lines=max_lines,
        start_time=start_time,
        end_time=end_time,
    ):
        records.append(record)

    return AccessLogParseResult(records=records, diagnostics=diagnostics.to_dict())


def iter_access_log_records_with_diagnostics(
    path: Path,
    log_format: str = "nginx",
    *,
    diagnostics: ParserDiagnostics,
    max_lines: int | None = None,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
) -> Iterable[AccessLogRecord]:
    if log_format.lower() != "nginx":
        raise ValueError("Only nginx format is implemented in the skeleton parser.")

    if max_lines is not None and max_lines <= 0:
        raise ValueError("max_lines must be a positive integer.")

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        if max_lines is not None and line_number > max_lines:
            break

        diagnostics.total_lines += 1
        if not line.strip():
            continue
        record, error = _parse_nginx_access_line(line)
        if record is not None:
            if not _is_in_time_range(record.timestamp, start_time, end_time):
                continue
            diagnostics.parsed_records += 1
            yield record
            continue

        if error is None:
            raise RuntimeError("access log parser returned neither record nor error")
        reason, message = error
        diagnostics.add_skipped(
            line_number=line_number,
            reason=reason,
            message=message,
            raw_line=line,
        )


def parse_nginx_access_line(line: str) -> AccessLogRecord | None:
    record, _ = _parse_nginx_access_line(line)
    return record


def _parse_nginx_access_line(
    line: str,
) -> tuple[AccessLogRecord | None, ParseError | None]:
    match = NGINX_WITH_RESPONSE_TIME.match(line)
    if match is None:
        return None, ("NO_FORMAT_MATCH", "Line does not match nginx access log format.")

    groups = match.groupdict()
    try:
        timestamp = parse_nginx_timestamp(groups["timestamp"])
    except ValueError:
        return None, ("INVALID_TIMESTAMP", "Timestamp does not match nginx format.")

    try:
        status = int(groups["status"])
        bytes_sent = 0 if groups["bytes_sent"] == "-" else int(groups["bytes_sent"])
        response_time_ms = float(groups["response_time_sec"]) * 1000
    except ValueError:
        return None, ("INVALID_NUMBER", "Numeric field could not be parsed.")

    if (
        status < 100
        or status > 999
        or bytes_sent < 0
        or not isfinite(response_time_ms)
        or response_time_ms < 0
    ):
        return None, ("INVALID_NUMBER", "Numeric field is outside the valid range.")

    return (
        AccessLogRecord(
            timestamp=timestamp,
            method=groups["method"],
            uri=groups["uri"],
            status=status,
            response_time_ms=response_time_ms,
            bytes_sent=bytes_sent,
            client_ip=groups["client_ip"],
            user_agent=groups["user_agent"],
            referer=groups["referer"],
            raw_line=line,
        ),
        None,
    )


def _is_in_time_range(
    value: datetime,
    start_time: datetime | None,
    end_time: datetime | None,
) -> bool:
    normalized_start = _align_boundary_timezone(value, start_time)
    normalized_end = _align_boundary_timezone(value, end_time)
    if normalized_start is not None and value < normalized_start:
        return False
    if normalized_end is not None and value > normalized_end:
        return False
    return True


def _align_boundary_timezone(
    value: datetime,
    boundary: datetime | None,
) -> datetime | None:
    if boundary is None:
        return None
    if boundary.tzinfo is None and value.tzinfo is not None:
        return boundary.replace(tzinfo=value.tzinfo)
    return boundary
