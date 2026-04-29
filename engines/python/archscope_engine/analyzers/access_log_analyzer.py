from __future__ import annotations

from collections import Counter, defaultdict
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any, Callable, Iterable, cast

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.statistics import BoundedPercentile
from archscope_engine.common.time_utils import minute_bucket
from archscope_engine.models.access_log import AccessLogRecord
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.result_contracts import (
    AccessLogAnalysisOptions,
    AccessLogFinding,
    AccessLogMetadata,
    AccessLogSampleRecordRow,
    AccessLogSeries,
    AccessLogSummary,
    AccessLogTables,
    ParserDiagnostics as ParserDiagnosticsContract,
)
from archscope_engine.parsers.access_log_parser import iter_access_log_records_with_diagnostics

PERCENTILE_SAMPLE_LIMIT = 10_000


@dataclass
class ResponseTimeStats:
    total_ms: float = 0.0
    count: int = 0
    percentiles: BoundedPercentile = field(
        default_factory=lambda: BoundedPercentile(PERCENTILE_SAMPLE_LIMIT)
    )

    def add(self, value_ms: float) -> None:
        self.total_ms += value_ms
        self.count += 1
        self.percentiles.add(value_ms)

    def average(self) -> float:
        if self.count == 0:
            return 0.0
        return self.total_ms / self.count

    def percentile(self, percent: float) -> float:
        return self.percentiles.percentile(percent)


@dataclass(frozen=True)
class AccessLogAnalyzerOptions:
    max_lines: int | None = None
    start_time: datetime | None = None
    end_time: datetime | None = None


def analyze_access_log(
    path: Path,
    log_format: str = "nginx",
    *,
    max_lines: int | None = None,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
) -> AnalysisResult:
    options = AccessLogAnalyzerOptions(
        max_lines=max_lines,
        start_time=start_time,
        end_time=end_time,
    )
    diagnostics = ParserDiagnostics()
    records = iter_access_log_records_with_diagnostics(
        path,
        log_format,
        diagnostics=diagnostics,
        max_lines=options.max_lines,
        start_time=options.start_time,
        end_time=options.end_time,
    )
    return build_access_log_result(
        records,
        source_file=path,
        log_format=log_format,
        diagnostics=diagnostics.to_dict,
        options=options,
    )


def build_access_log_result(
    records: Iterable[AccessLogRecord],
    source_file: Path,
    log_format: str,
    diagnostics: dict[str, Any] | Callable[[], dict[str, Any]] | None = None,
    options: AccessLogAnalyzerOptions | None = None,
) -> AnalysisResult:
    response_times = ResponseTimeStats()
    error_count = 0
    total = 0

    requests_by_minute: Counter[str] = Counter()
    response_times_by_minute: dict[str, ResponseTimeStats] = defaultdict(ResponseTimeStats)
    status_distribution: Counter[str] = Counter()
    url_counts: Counter[str] = Counter()
    url_response_totals: dict[str, float] = defaultdict(float)
    sample_records: list[AccessLogSampleRecordRow] = []

    for record in records:
        total += 1
        response_times.add(record.response_time_ms)
        if record.status >= 400:
            error_count += 1

        minute = minute_bucket(record.timestamp)
        requests_by_minute[minute] += 1
        response_times_by_minute[minute].add(record.response_time_ms)
        status_distribution[_status_family(record.status)] += 1
        url_counts[record.uri] += 1
        url_response_totals[record.uri] += record.response_time_ms

        if len(sample_records) < 20:
            sample_records.append(
                {
                    "timestamp": record.timestamp.isoformat(),
                    "method": record.method,
                    "uri": record.uri,
                    "status": record.status,
                    "response_time_ms": round(record.response_time_ms, 2),
                }
            )

    requests_per_minute = [
        {"time": minute, "value": count}
        for minute, count in sorted(requests_by_minute.items())
    ]
    avg_response_time_per_minute = [
        {
            "time": minute,
            "value": round(stats.average(), 2),
        }
        for minute, stats in sorted(response_times_by_minute.items())
    ]
    p95_response_time_per_minute = [
        {
            "time": minute,
            "value": round(stats.percentile(95), 2),
        }
        for minute, stats in sorted(response_times_by_minute.items())
    ]

    top_urls_by_avg_response_time = sorted(
        (
            {
                "uri": uri,
                "avg_response_ms": round(url_response_totals[uri] / count, 2),
                "count": count,
            }
            for uri, count in url_counts.items()
        ),
        key=lambda item: item["avg_response_ms"],
        reverse=True,
    )[:10]

    summary: AccessLogSummary = {
        "total_requests": total,
        "avg_response_ms": round(response_times.average(), 2),
        "p95_response_ms": round(response_times.percentile(95), 2),
        "p99_response_ms": round(response_times.percentile(99), 2),
        "error_rate": round((error_count / total * 100) if total else 0.0, 2),
    }
    series: AccessLogSeries = {
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
    }
    tables: AccessLogTables = {"sample_records": sample_records}
    diagnostics_payload = diagnostics() if callable(diagnostics) else diagnostics
    analysis_options = _analysis_options_to_dict(options)
    findings = _build_access_log_findings(summary, series)
    metadata: AccessLogMetadata = {
        "format": log_format,
        "parser": "nginx_combined_with_response_time",
        "schema_version": "0.1.0",
        "diagnostics": cast(
            ParserDiagnosticsContract,
            diagnostics_payload
            if diagnostics_payload is not None
            else _default_diagnostics(total),
        ),
        "analysis_options": analysis_options,
        "findings": findings,
    }

    return AnalysisResult(
        type="access_log",
        source_files=[str(source_file)],
        summary=summary,
        series=series,
        tables=tables,
        metadata=metadata,
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


def _default_diagnostics(total: int) -> dict[str, Any]:
    return {
        "total_lines": total,
        "parsed_records": total,
        "skipped_lines": 0,
        "skipped_by_reason": {},
        "samples": [],
    }


def _analysis_options_to_dict(
    options: AccessLogAnalyzerOptions | None,
) -> AccessLogAnalysisOptions:
    return {
        "max_lines": options.max_lines if options else None,
        "start_time": (
            options.start_time.isoformat() if options and options.start_time else None
        ),
        "end_time": (
            options.end_time.isoformat() if options and options.end_time else None
        ),
    }


def _build_access_log_findings(
    summary: AccessLogSummary,
    series: AccessLogSeries,
) -> list[AccessLogFinding]:
    findings: list[AccessLogFinding] = []
    error_rate = summary["error_rate"]
    if error_rate >= 10:
        findings.append(
            {
                "severity": "critical",
                "code": "HIGH_ERROR_RATE",
                "message": "Error rate is at or above 10%.",
                "evidence": {"error_rate": error_rate},
            }
        )
    elif error_rate >= 5:
        findings.append(
            {
                "severity": "warning",
                "code": "ELEVATED_ERROR_RATE",
                "message": "Error rate is at or above 5%.",
                "evidence": {"error_rate": error_rate},
            }
        )

    server_errors = next(
        (
            row["count"]
            for row in series["status_code_distribution"]
            if row["status"] == "5xx"
        ),
        0,
    )
    if server_errors:
        findings.append(
            {
                "severity": "warning",
                "code": "SERVER_ERRORS_PRESENT",
                "message": "One or more server-side errors were observed.",
                "evidence": {"status": "5xx", "count": server_errors},
            }
        )

    slowest_url = next(iter(series["top_urls_by_avg_response_time"]), None)
    if slowest_url and slowest_url["avg_response_ms"] >= 1000:
        findings.append(
            {
                "severity": "warning",
                "code": "SLOW_URL_AVERAGE",
                "message": "The slowest URL average response time is at or above 1 second.",
                "evidence": {
                    "uri": slowest_url["uri"],
                    "avg_response_ms": slowest_url["avg_response_ms"],
                    "count": slowest_url["count"],
                },
            }
        )

    return findings
