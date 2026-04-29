from __future__ import annotations

from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

from archscope_engine.common.statistics import average, percentile
from archscope_engine.common.time_utils import minute_bucket
from archscope_engine.models.access_log import AccessLogRecord
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.parsers.access_log_parser import parse_access_log_with_diagnostics


def analyze_access_log(path: Path, log_format: str = "nginx") -> AnalysisResult:
    parse_result = parse_access_log_with_diagnostics(path, log_format)
    return build_access_log_result(
        parse_result.records,
        source_file=path,
        log_format=log_format,
        diagnostics=parse_result.diagnostics,
    )


def build_access_log_result(
    records: list[AccessLogRecord],
    source_file: Path,
    log_format: str,
    diagnostics: dict[str, Any] | None = None,
) -> AnalysisResult:
    response_times = [record.response_time_ms for record in records]
    error_count = sum(1 for record in records if record.status >= 400)
    total = len(records)

    by_minute: dict[str, list[AccessLogRecord]] = defaultdict(list)
    status_distribution: Counter[str] = Counter()
    url_counts: Counter[str] = Counter()
    url_response_times: dict[str, list[float]] = defaultdict(list)

    for record in records:
        by_minute[minute_bucket(record.timestamp)].append(record)
        status_distribution[_status_family(record.status)] += 1
        url_counts[record.uri] += 1
        url_response_times[record.uri].append(record.response_time_ms)

    requests_per_minute = [
        {"time": minute, "value": len(items)}
        for minute, items in sorted(by_minute.items())
    ]
    avg_response_time_per_minute = [
        {
            "time": minute,
            "value": round(average([item.response_time_ms for item in items]), 2),
        }
        for minute, items in sorted(by_minute.items())
    ]
    p95_response_time_per_minute = [
        {
            "time": minute,
            "value": round(percentile([item.response_time_ms for item in items], 95), 2),
        }
        for minute, items in sorted(by_minute.items())
    ]

    top_urls_by_avg_response_time = sorted(
        (
            {
                "uri": uri,
                "avg_response_ms": round(average(values), 2),
                "count": len(values),
            }
            for uri, values in url_response_times.items()
        ),
        key=lambda item: item["avg_response_ms"],
        reverse=True,
    )[:10]

    return AnalysisResult(
        type="access_log",
        source_files=[str(source_file)],
        summary={
            "total_requests": total,
            "avg_response_ms": round(average(response_times), 2),
            "p95_response_ms": round(percentile(response_times, 95), 2),
            "p99_response_ms": round(percentile(response_times, 99), 2),
            "error_rate": round((error_count / total * 100) if total else 0.0, 2),
        },
        series={
            "requests_per_minute": requests_per_minute,
            "avg_response_time_per_minute": avg_response_time_per_minute,
            "p95_response_time_per_minute": p95_response_time_per_minute,
            "status_code_distribution": [
                {"status": key, "count": value}
                for key, value in sorted(status_distribution.items())
            ],
            "top_urls_by_count": [
                {"uri": uri, "count": count}
                for uri, count in url_counts.most_common(10)
            ],
            "top_urls_by_avg_response_time": top_urls_by_avg_response_time,
        },
        tables={
            "sample_records": [
                {
                    "timestamp": record.timestamp.isoformat(),
                    "method": record.method,
                    "uri": record.uri,
                    "status": record.status,
                    "response_time_ms": round(record.response_time_ms, 2),
                }
                for record in records[:20]
            ]
        },
        metadata={
            "format": log_format,
            "parser": "nginx_combined_with_response_time",
            "schema_version": "0.1.0",
            **({"diagnostics": diagnostics} if diagnostics is not None else {}),
        },
    )


def _status_family(status: int) -> str:
    if 200 <= status <= 299:
        return "2xx"
    if 300 <= status <= 399:
        return "3xx"
    if 400 <= status <= 499:
        return "4xx"
    if status >= 500:
        return "5xx"
    return "other"
