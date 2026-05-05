"""Access log analyzer (overhauled).

Inspired by GoAccess / AWStats / Datadog APM dashboards. Each minute of
traffic is bucketed for time-series analysis (latency percentiles,
status-class breakdown, throughput) and per-URL aggregates carry enough
detail (count, avg, p50/p90/p95, total bytes, error count, status mix)
that the frontend can sort by any axis without re-querying the engine.

Static-asset vs API request classification is a pragmatic regex on path
extension + common asset directories — enough to give the user separate
headline numbers for "page weight" vs "service traffic" without requiring
infrastructure-specific configuration.
"""
from __future__ import annotations

import re
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from datetime import datetime
from heapq import nlargest
from pathlib import Path
from typing import Any, Callable, Iterable, cast

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import detect_text_encoding
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
TOP_URL_LIMIT = 20

# ── Static-asset detection ──────────────────────────────────────────────────

_STATIC_EXT_RE = re.compile(
    r"\.(?:js|mjs|css|map|png|jpe?g|gif|ico|svg|webp|avif|bmp|tiff?|"
    r"woff2?|ttf|otf|eot|"
    r"mp4|webm|mp3|ogg|wav|flac|m4a|"
    r"pdf|zip|gz|tar|7z|rar|"
    r"json|xml|txt|md|wasm)(?:\?.*)?$",
    re.IGNORECASE,
)
_STATIC_PATH_HINT_RE = re.compile(
    r"/(?:static|assets|public|dist|build|images?|img|css|js|fonts?|media|files|downloads|cdn)/",
    re.IGNORECASE,
)


def _classify_request(uri: str) -> str:
    """Return ``"static"`` or ``"api"`` for *uri*. Best-effort heuristic."""
    if _STATIC_EXT_RE.search(uri) or _STATIC_PATH_HINT_RE.search(uri):
        return "static"
    return "api"


# ── Stats helpers ───────────────────────────────────────────────────────────


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


@dataclass
class _UrlStats:
    """Per-URL aggregate. Tracked once per unique URI."""

    count: int = 0
    total_response_ms: float = 0.0
    total_bytes: int = 0
    error_count: int = 0
    classification: str = "api"
    status_counts: Counter[str] = field(default_factory=Counter)
    rt: ResponseTimeStats = field(default_factory=ResponseTimeStats)
    sample_method: str = ""

    def add(self, record: AccessLogRecord) -> None:
        self.count += 1
        self.total_response_ms += record.response_time_ms
        self.total_bytes += record.bytes_sent
        if record.status >= 400:
            self.error_count += 1
        self.status_counts[_status_family(record.status)] += 1
        self.rt.add(record.response_time_ms)
        if not self.sample_method:
            self.sample_method = record.method


@dataclass(frozen=True)
class AccessLogAnalyzerOptions:
    max_lines: int | None = None
    start_time: datetime | None = None
    end_time: datetime | None = None


# ── Public entry points ─────────────────────────────────────────────────────


def analyze_access_log(
    path: Path,
    log_format: str = "nginx",
    *,
    max_lines: int | None = None,
    start_time: datetime | None = None,
    end_time: datetime | None = None,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    options = AccessLogAnalyzerOptions(
        max_lines=max_lines,
        start_time=start_time,
        end_time=end_time,
    )
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    records = iter_access_log_records_with_diagnostics(
        path,
        log_format,
        diagnostics=diagnostics,
        max_lines=options.max_lines,
        start_time=options.start_time,
        end_time=options.end_time,
        debug_log=debug_log,
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
    static_response_times = ResponseTimeStats()
    api_response_times = ResponseTimeStats()
    error_count = 0
    static_count = 0
    api_count = 0
    static_bytes = 0
    api_bytes = 0
    total_bytes = 0
    total = 0
    earliest: datetime | None = None
    latest: datetime | None = None

    requests_by_minute: Counter[str] = Counter()
    bytes_by_minute: Counter[str] = Counter()
    response_times_by_minute: dict[str, ResponseTimeStats] = defaultdict(ResponseTimeStats)
    status_class_by_minute: dict[str, Counter[str]] = defaultdict(Counter)
    status_distribution: Counter[str] = Counter()
    status_code_counts: Counter[int] = Counter()
    non_standard_statuses: Counter[int] = Counter()
    method_distribution: Counter[str] = Counter()
    classification_distribution: Counter[str] = Counter()
    url_stats: dict[str, _UrlStats] = defaultdict(_UrlStats)
    sample_records: list[AccessLogSampleRecordRow] = []

    for record in records:
        total += 1
        response_times.add(record.response_time_ms)
        if record.status >= 400:
            error_count += 1
        total_bytes += record.bytes_sent
        method_distribution[record.method or "?"] += 1

        classification = _classify_request(record.uri)
        classification_distribution[classification] += 1
        if classification == "static":
            static_count += 1
            static_bytes += record.bytes_sent
            static_response_times.add(record.response_time_ms)
        else:
            api_count += 1
            api_bytes += record.bytes_sent
            api_response_times.add(record.response_time_ms)

        if earliest is None or record.timestamp < earliest:
            earliest = record.timestamp
        if latest is None or record.timestamp > latest:
            latest = record.timestamp

        minute = minute_bucket(record.timestamp)
        requests_by_minute[minute] += 1
        bytes_by_minute[minute] += record.bytes_sent
        response_times_by_minute[minute].add(record.response_time_ms)
        family = _status_family(record.status)
        status_class_by_minute[minute][family] += 1
        status_distribution[family] += 1
        status_code_counts[record.status] += 1
        if record.status < 100 or record.status > 599:
            non_standard_statuses[record.status] += 1
        bucket = url_stats[record.uri]
        if bucket.count == 0:
            bucket.classification = classification
        bucket.add(record)

        if len(sample_records) < 25:
            sample_records.append(
                {
                    "timestamp": record.timestamp.isoformat(),
                    "method": record.method,
                    "uri": record.uri,
                    "status": record.status,
                    "response_time_ms": round(record.response_time_ms, 2),
                }
            )

    # ── Time-series ──────────────────────────────────────────────────────────
    requests_per_minute = [
        {"time": minute, "value": count}
        for minute, count in sorted(requests_by_minute.items())
    ]
    avg_response_time_per_minute = [
        {"time": minute, "value": round(stats.average(), 2)}
        for minute, stats in sorted(response_times_by_minute.items())
    ]
    p50_response_time_per_minute = [
        {"time": minute, "value": round(stats.percentile(50), 2)}
        for minute, stats in sorted(response_times_by_minute.items())
    ]
    p90_response_time_per_minute = [
        {"time": minute, "value": round(stats.percentile(90), 2)}
        for minute, stats in sorted(response_times_by_minute.items())
    ]
    p95_response_time_per_minute = [
        {"time": minute, "value": round(stats.percentile(95), 2)}
        for minute, stats in sorted(response_times_by_minute.items())
    ]
    p99_response_time_per_minute = [
        {"time": minute, "value": round(stats.percentile(99), 2)}
        for minute, stats in sorted(response_times_by_minute.items())
    ]
    status_class_per_minute = [
        {
            "time": minute,
            "2xx": status_class_by_minute[minute].get("2xx", 0),
            "3xx": status_class_by_minute[minute].get("3xx", 0),
            "4xx": status_class_by_minute[minute].get("4xx", 0),
            "5xx": status_class_by_minute[minute].get("5xx", 0),
            "other": status_class_by_minute[minute].get("other", 0),
        }
        for minute in sorted(status_class_by_minute.keys())
    ]
    error_rate_per_minute = []
    for minute, total_count in sorted(requests_by_minute.items()):
        errors = (
            status_class_by_minute[minute].get("4xx", 0)
            + status_class_by_minute[minute].get("5xx", 0)
        )
        error_rate_per_minute.append(
            {
                "time": minute,
                "value": round((errors / total_count * 100) if total_count else 0.0, 2),
                "errors": errors,
                "total": total_count,
            }
        )
    bytes_per_minute_series = [
        {"time": minute, "value": bytes_by_minute.get(minute, 0)}
        for minute, _ in sorted(requests_by_minute.items())
    ]
    throughput_per_minute = [
        {
            "time": minute,
            "requests_per_sec": round(requests_by_minute.get(minute, 0) / 60.0, 3),
            "bytes_per_sec": round(bytes_by_minute.get(minute, 0) / 60.0, 2),
        }
        for minute, _ in sorted(requests_by_minute.items())
    ]

    # ── Per-URL aggregates ───────────────────────────────────────────────────
    url_rows: list[dict[str, Any]] = []
    for uri, bucket in url_stats.items():
        avg_ms = bucket.total_response_ms / bucket.count if bucket.count else 0.0
        url_rows.append(
            {
                "uri": uri,
                "method": bucket.sample_method,
                "classification": bucket.classification,
                "count": bucket.count,
                "avg_response_ms": round(avg_ms, 2),
                "p50_response_ms": round(bucket.rt.percentile(50), 2),
                "p90_response_ms": round(bucket.rt.percentile(90), 2),
                "p95_response_ms": round(bucket.rt.percentile(95), 2),
                "p99_response_ms": round(bucket.rt.percentile(99), 2),
                "total_bytes": bucket.total_bytes,
                "error_count": bucket.error_count,
                "status_2xx": bucket.status_counts.get("2xx", 0),
                "status_3xx": bucket.status_counts.get("3xx", 0),
                "status_4xx": bucket.status_counts.get("4xx", 0),
                "status_5xx": bucket.status_counts.get("5xx", 0),
            }
        )

    top_by_count = sorted(url_rows, key=lambda r: r["count"], reverse=True)[:TOP_URL_LIMIT]
    top_by_avg = sorted(url_rows, key=lambda r: r["avg_response_ms"], reverse=True)[:TOP_URL_LIMIT]
    top_by_p95 = sorted(url_rows, key=lambda r: r["p95_response_ms"], reverse=True)[:TOP_URL_LIMIT]
    top_by_bytes = sorted(url_rows, key=lambda r: r["total_bytes"], reverse=True)[:TOP_URL_LIMIT]
    top_by_errors = sorted(
        (r for r in url_rows if r["error_count"] > 0),
        key=lambda r: r["error_count"],
        reverse=True,
    )[:TOP_URL_LIMIT]

    # ── Top status codes (full code, not just family) ────────────────────────
    top_status_codes = [
        {"status": status, "count": count}
        for status, count in status_code_counts.most_common(20)
    ]

    # ── Wall time + throughput ───────────────────────────────────────────────
    wall_time_sec = (
        max(0.0, (latest - earliest).total_seconds()) if earliest and latest else 0.0
    )
    avg_requests_per_sec = round(total / wall_time_sec, 3) if wall_time_sec > 0 else 0.0
    avg_bytes_per_sec = round(total_bytes / wall_time_sec, 2) if wall_time_sec > 0 else 0.0

    # ── Summary ──────────────────────────────────────────────────────────────
    summary: dict[str, Any] = {
        "total_requests": total,
        "avg_response_ms": round(response_times.average(), 2),
        "p50_response_ms": round(response_times.percentile(50), 2),
        "p90_response_ms": round(response_times.percentile(90), 2),
        "p95_response_ms": round(response_times.percentile(95), 2),
        "p99_response_ms": round(response_times.percentile(99), 2),
        "error_rate": round((error_count / total * 100) if total else 0.0, 2),
        "error_count": error_count,
        "total_bytes": total_bytes,
        "wall_time_sec": round(wall_time_sec, 3),
        "avg_requests_per_sec": avg_requests_per_sec,
        "avg_bytes_per_sec": avg_bytes_per_sec,
        "static_count": static_count,
        "api_count": api_count,
        "static_bytes": static_bytes,
        "api_bytes": api_bytes,
        "static_avg_response_ms": round(static_response_times.average(), 2),
        "api_avg_response_ms": round(api_response_times.average(), 2),
        "api_p95_response_ms": round(api_response_times.percentile(95), 2),
        "earliest_timestamp": earliest.isoformat() if earliest else None,
        "latest_timestamp": latest.isoformat() if latest else None,
        "unique_uris": len(url_stats),
    }

    series: dict[str, Any] = {
        "requests_per_minute": requests_per_minute,
        "avg_response_time_per_minute": avg_response_time_per_minute,
        "p50_response_time_per_minute": p50_response_time_per_minute,
        "p90_response_time_per_minute": p90_response_time_per_minute,
        "p95_response_time_per_minute": p95_response_time_per_minute,
        "p99_response_time_per_minute": p99_response_time_per_minute,
        "status_class_per_minute": status_class_per_minute,
        "error_rate_per_minute": error_rate_per_minute,
        "bytes_per_minute": bytes_per_minute_series,
        "throughput_per_minute": throughput_per_minute,
        "status_code_distribution": [
            {"status": key, "count": value}
            for key, value in sorted(status_distribution.items())
        ],
        "method_distribution": [
            {"method": key, "count": value}
            for key, value in method_distribution.most_common()
        ],
        "request_classification": [
            {"classification": key, "count": value}
            for key, value in classification_distribution.most_common()
        ],
        # Backwards-compat fields the frontend already consumed.
        "top_urls_by_count": [
            {"uri": row["uri"], "count": row["count"]} for row in top_by_count
        ],
        "top_urls_by_avg_response_time": [
            {
                "uri": row["uri"],
                "avg_response_ms": row["avg_response_ms"],
                "count": row["count"],
            }
            for row in top_by_avg
        ],
    }
    tables: dict[str, Any] = {
        "sample_records": sample_records,
        "url_stats": url_rows,
        "top_urls_by_count": top_by_count,
        "top_urls_by_avg_response_time": top_by_avg,
        "top_urls_by_p95_response_time": top_by_p95,
        "top_urls_by_bytes": top_by_bytes,
        "top_urls_by_errors": top_by_errors,
        "top_status_codes": top_status_codes,
    }
    diagnostics_payload = diagnostics() if callable(diagnostics) else diagnostics
    analysis_options = _analysis_options_to_dict(options)
    findings = _build_access_log_findings(
        summary,
        status_distribution,
        url_rows,
        error_rate_per_minute,
        non_standard_statuses,
    )
    metadata: dict[str, Any] = {
        "format": log_format,
        "parser": "nginx_combined_with_response_time",
        "schema_version": "0.2.0",
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
        summary=cast(AccessLogSummary, summary),
        series=cast(AccessLogSeries, series),
        tables=cast(AccessLogTables, tables),
        metadata=cast(AccessLogMetadata, metadata),
    )


# ── Helpers ─────────────────────────────────────────────────────────────────


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
        "source_file": None,
        "format": "nginx",
        "total_lines": total,
        "parsed_records": total,
        "skipped_lines": 0,
        "skipped_by_reason": {},
        "samples": [],
        "warning_count": 0,
        "error_count": 0,
        "warnings": [],
        "errors": [],
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
    summary: dict[str, Any],
    status_distribution: Counter[str],
    url_rows: list[dict[str, Any]],
    error_rate_per_minute: list[dict[str, Any]],
    non_standard_statuses: Counter[int],
) -> list[AccessLogFinding]:
    findings: list[AccessLogFinding] = []
    error_rate = float(summary.get("error_rate", 0))
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

    server_errors = status_distribution.get("5xx", 0)
    if server_errors:
        findings.append(
            {
                "severity": "warning",
                "code": "SERVER_ERRORS_PRESENT",
                "message": "One or more server-side errors were observed.",
                "evidence": {"status": "5xx", "count": server_errors},
            }
        )

    if non_standard_statuses:
        statuses = ", ".join(str(status) for status in sorted(non_standard_statuses))
        findings.append(
            {
                "severity": "warning",
                "code": "NON_STANDARD_HTTP_STATUS",
                "message": "One or more non-standard HTTP status codes were observed.",
                "evidence": {
                    "statuses": statuses,
                    "count": sum(non_standard_statuses.values()),
                },
            }
        )

    # Slow URL — pick the slowest by p95 instead of avg so a single tail
    # event doesn't crown an otherwise-fast endpoint.
    slowest = max(
        (row for row in url_rows if row["count"] >= 5),
        key=lambda row: row["p95_response_ms"],
        default=None,
    )
    if slowest and slowest["p95_response_ms"] >= 1000:
        findings.append(
            {
                "severity": "warning",
                "code": "SLOW_URL_P95",
                "message": "An endpoint's p95 response time is at or above 1 second.",
                "evidence": {
                    "uri": slowest["uri"],
                    "p95_response_ms": slowest["p95_response_ms"],
                    "count": slowest["count"],
                    "classification": slowest["classification"],
                },
            }
        )

    # Error spike — any single minute with > 50% error rate AND ≥ 5 requests.
    spike = max(
        (row for row in error_rate_per_minute if row["total"] >= 5),
        key=lambda row: row["value"],
        default=None,
    )
    if spike and spike["value"] >= 50:
        findings.append(
            {
                "severity": "critical",
                "code": "ERROR_BURST_DETECTED",
                "message": "A single minute crossed 50% error rate.",
                "evidence": {
                    "minute": spike["time"],
                    "error_rate": spike["value"],
                    "errors": spike["errors"],
                    "requests": spike["total"],
                },
            }
        )

    return findings
