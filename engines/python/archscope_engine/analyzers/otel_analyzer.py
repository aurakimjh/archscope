from __future__ import annotations

from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import detect_text_encoding
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.otel import OTelLogRecord
from archscope_engine.parsers.otel_parser import parse_otel_jsonl


def analyze_otel_jsonl(
    path: Path,
    *,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    records = parse_otel_jsonl(path, diagnostics=diagnostics, debug_log=debug_log)
    return build_otel_result(
        records,
        source_file=path,
        diagnostics=diagnostics.to_dict(),
        top_n=top_n,
    )


def build_otel_result(
    records: list[OTelLogRecord],
    *,
    source_file: Path,
    diagnostics: dict[str, Any],
    top_n: int = 20,
) -> AnalysisResult:
    trace_counts = Counter(record.trace_id or "(no-trace)" for record in records)
    service_counts = Counter(record.service_name or "(unknown)" for record in records)
    severity_counts = Counter(record.severity for record in records)
    trace_services: dict[str, set[str]] = defaultdict(set)
    trace_records: dict[str, list[OTelLogRecord]] = defaultdict(list)
    for record in records:
        if record.trace_id and record.service_name:
            trace_services[record.trace_id].add(record.service_name)
        if record.trace_id:
            trace_records[record.trace_id].append(record)
    cross_service_traces = {
        trace_id: services
        for trace_id, services in trace_services.items()
        if len(services) > 1
    }
    trace_paths = _trace_paths(trace_records)
    trace_failures = _trace_failures(trace_records)
    failure_propagation = _failure_propagation(trace_records)
    summary = {
        "total_records": len(records),
        "unique_traces": len([key for key in trace_counts if key != "(no-trace)"]),
        "unique_services": len([key for key in service_counts if key != "(unknown)"]),
        "cross_service_traces": len(cross_service_traces),
        "failed_traces": len(trace_failures),
        "failure_propagation_traces": len(failure_propagation),
        "error_records": sum(
            count
            for severity, count in severity_counts.items()
            if severity.upper() in {"ERROR", "FATAL", "CRITICAL"}
        ),
    }
    series = {
        "severity_distribution": [
            {"severity": key, "count": value}
            for key, value in severity_counts.most_common(top_n)
        ],
        "service_distribution": [
            {"service": key, "count": value}
            for key, value in service_counts.most_common(top_n)
        ],
        "top_traces": [
            {"trace_id": key, "count": value}
            for key, value in trace_counts.most_common(top_n)
        ],
    }
    tables = {
        "records": [
            {
                "timestamp": record.timestamp,
                "trace_id": record.trace_id,
                "span_id": record.span_id,
                "parent_span_id": record.parent_span_id,
                "service_name": record.service_name,
                "severity": record.severity,
                "body": record.body,
            }
            for record in records[:top_n]
        ],
        "cross_service_traces": [
            {"trace_id": trace_id, "services": sorted(services)}
            for trace_id, services in sorted(cross_service_traces.items())[:top_n]
        ],
        "trace_service_paths": trace_paths[:top_n],
        "trace_failures": trace_failures[:top_n],
        "service_failure_propagation": failure_propagation[:top_n],
        "trace_span_topology": _trace_span_topology(trace_records, top_n=top_n),
        "service_trace_matrix": _service_trace_matrix(trace_records, top_n=top_n),
    }
    return AnalysisResult(
        type="otel_logs",
        source_files=[str(source_file)],
        summary=summary,
        series=series,
        tables=tables,
        metadata={
            "parser": "otel_jsonl",
            "schema_version": "0.1.0",
            "diagnostics": diagnostics,
            "findings": _findings(summary),
        },
    )


def _findings(summary: dict[str, int]) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    if summary["error_records"] > 0:
        findings.append(
            {
                "severity": "warning",
                "code": "OTEL_ERROR_RECORDS_PRESENT",
                "message": "OpenTelemetry logs include error-level records.",
                "evidence": {"error_records": summary["error_records"]},
            }
        )
    if summary["cross_service_traces"] > 0:
        findings.append(
            {
                "severity": "info",
                "code": "OTEL_CROSS_SERVICE_TRACE",
                "message": "At least one trace appears across multiple services.",
                "evidence": {"cross_service_traces": summary["cross_service_traces"]},
            }
        )
    if summary["failed_traces"] > 0:
        findings.append(
            {
                "severity": "warning",
                "code": "OTEL_FAILED_TRACE",
                "message": "At least one distributed trace contains an error signal.",
                "evidence": {"failed_traces": summary["failed_traces"]},
            }
        )
    if summary["failure_propagation_traces"] > 0:
        findings.append(
            {
                "severity": "warning",
                "code": "OTEL_FAILURE_PROPAGATION",
                "message": (
                    "Failure signals appear before downstream services in at least one trace."
                ),
                "evidence": {
                    "failure_propagation_traces": summary["failure_propagation_traces"]
                },
            }
        )
    return findings


def _trace_paths(
    trace_records: dict[str, list[OTelLogRecord]],
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for trace_id, records in sorted(trace_records.items()):
        services = _ordered_services(records)
        severities = [record.severity for record in records]
        rows.append(
            {
                "trace_id": trace_id,
                "service_path": " -> ".join(services),
                "service_count": len(services),
                "record_count": len(records),
                "has_error": any(_is_error_record(record) for record in records),
                "max_severity": _max_severity(severities),
            }
        )
    return rows


def _trace_failures(
    trace_records: dict[str, list[OTelLogRecord]],
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for trace_id, records in sorted(trace_records.items()):
        ordered_records = _ordered_records(records)
        error_records = [record for record in ordered_records if _is_error_record(record)]
        if not error_records:
            continue
        rows.append(
            {
                "trace_id": trace_id,
                "services": " -> ".join(_ordered_services(records)),
                "first_failing_service": error_records[0].service_name or "(unknown)",
                "error_services": ", ".join(
                    sorted(
                        {
                            record.service_name or "(unknown)"
                            for record in error_records
                        }
                    )
                ),
                "error_count": len(error_records),
                "first_error": error_records[0].body,
            }
        )
    return rows


def _failure_propagation(
    trace_records: dict[str, list[OTelLogRecord]],
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for trace_id, records in sorted(trace_records.items()):
        ordered_records = _ordered_records(records)
        first_error_index = next(
            (index for index, record in enumerate(ordered_records) if _is_error_record(record)),
            None,
        )
        if first_error_index is None:
            continue
        first_error = ordered_records[first_error_index]
        downstream_services = []
        for record in ordered_records[first_error_index + 1 :]:
            service_name = record.service_name or "(unknown)"
            if service_name not in downstream_services:
                downstream_services.append(service_name)
        if not downstream_services:
            continue
        rows.append(
            {
                "trace_id": trace_id,
                "service_path": " -> ".join(_ordered_services(records)),
                "first_failing_service": first_error.service_name or "(unknown)",
                "first_failing_span_id": first_error.span_id,
                "downstream_services": downstream_services,
                "downstream_service_count": len(downstream_services),
                "first_error": first_error.body,
            }
        )
    return rows


def _service_trace_matrix(
    trace_records: dict[str, list[OTelLogRecord]],
    *,
    top_n: int,
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for trace_id, records in sorted(trace_records.items())[:top_n]:
        counts = Counter(record.service_name or "(unknown)" for record in records)
        for service, count in sorted(counts.items()):
            rows.append(
                {
                    "trace_id": trace_id,
                    "service": service,
                    "record_count": count,
                }
            )
    return rows


def _trace_span_topology(
    trace_records: dict[str, list[OTelLogRecord]],
    *,
    top_n: int,
) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []
    for trace_id, records in sorted(trace_records.items())[:top_n]:
        span_records = _span_representatives(records)
        children_by_parent: dict[str | None, list[str]] = defaultdict(list)
        for span_id, record in span_records.items():
            parent_id = record.parent_span_id if record.parent_span_id in span_records else None
            children_by_parent[parent_id].append(span_id)
        for span_id, record in sorted(span_records.items()):
            rows.append(
                {
                    "trace_id": trace_id,
                    "span_id": span_id,
                    "parent_span_id": record.parent_span_id,
                    "service": record.service_name or "(unknown)",
                    "child_count": len(children_by_parent.get(span_id, [])),
                    "has_error": any(
                        _is_error_record(item)
                        for item in records
                        if item.span_id == span_id
                    ),
                }
            )
    return rows


def _ordered_services(records: list[OTelLogRecord]) -> list[str]:
    parent_ordered = _ordered_services_by_parent(records)
    if parent_ordered:
        return parent_ordered
    services: list[str] = []
    for record in _ordered_records(records):
        service_name = record.service_name or "(unknown)"
        if not services or services[-1] != service_name:
            services.append(service_name)
    return services


def _ordered_services_by_parent(records: list[OTelLogRecord]) -> list[str]:
    span_records = _span_representatives(records)
    if not any(record.parent_span_id for record in span_records.values()):
        return []
    children_by_parent: dict[str | None, list[str]] = defaultdict(list)
    for span_id, record in span_records.items():
        parent_id = record.parent_span_id if record.parent_span_id in span_records else None
        children_by_parent[parent_id].append(span_id)
    for children in children_by_parent.values():
        children.sort(key=lambda span_id: _record_sort_key(span_records[span_id]))
    ordered_services: list[str] = []
    visited: set[str] = set()

    def visit(span_id: str) -> None:
        if span_id in visited:
            return
        visited.add(span_id)
        service_name = span_records[span_id].service_name or "(unknown)"
        if not ordered_services or ordered_services[-1] != service_name:
            ordered_services.append(service_name)
        for child_id in children_by_parent.get(span_id, []):
            visit(child_id)

    for root_id in children_by_parent.get(None, []):
        visit(root_id)
    for span_id in sorted(span_records, key=lambda item: _record_sort_key(span_records[item])):
        visit(span_id)
    return ordered_services


def _ordered_records(records: list[OTelLogRecord]) -> list[OTelLogRecord]:
    span_order = {
        span_id: index
        for index, span_id in enumerate(
            [
                record.span_id
                for record in _records_by_parent(records)
                if record.span_id is not None
            ]
        )
    }
    return sorted(
        records,
        key=lambda record: (
            span_order.get(record.span_id, len(span_order)),
            _record_sort_key(record),
        ),
    )


def _records_by_parent(records: list[OTelLogRecord]) -> list[OTelLogRecord]:
    span_records = _span_representatives(records)
    ordered_span_ids = _ordered_services_by_parent_ids(span_records)
    if not ordered_span_ids:
        return sorted(records, key=_record_sort_key)
    records_by_span: dict[str, list[OTelLogRecord]] = defaultdict(list)
    no_span_records: list[OTelLogRecord] = []
    for record in records:
        if record.span_id:
            records_by_span[record.span_id].append(record)
        else:
            no_span_records.append(record)
    ordered_records: list[OTelLogRecord] = []
    for span_id in ordered_span_ids:
        ordered_records.extend(sorted(records_by_span.get(span_id, []), key=_record_sort_key))
    ordered_records.extend(sorted(no_span_records, key=_record_sort_key))
    return ordered_records


def _ordered_services_by_parent_ids(
    span_records: dict[str, OTelLogRecord],
) -> list[str]:
    if not any(record.parent_span_id for record in span_records.values()):
        return []
    children_by_parent: dict[str | None, list[str]] = defaultdict(list)
    for span_id, record in span_records.items():
        parent_id = record.parent_span_id if record.parent_span_id in span_records else None
        children_by_parent[parent_id].append(span_id)
    for children in children_by_parent.values():
        children.sort(key=lambda span_id: _record_sort_key(span_records[span_id]))
    ordered: list[str] = []
    visited: set[str] = set()

    def visit(span_id: str) -> None:
        if span_id in visited:
            return
        visited.add(span_id)
        ordered.append(span_id)
        for child_id in children_by_parent.get(span_id, []):
            visit(child_id)

    for root_id in children_by_parent.get(None, []):
        visit(root_id)
    for span_id in sorted(span_records, key=lambda item: _record_sort_key(span_records[item])):
        visit(span_id)
    return ordered


def _span_representatives(records: list[OTelLogRecord]) -> dict[str, OTelLogRecord]:
    representatives: dict[str, OTelLogRecord] = {}
    for record in sorted(records, key=_record_sort_key):
        if record.span_id and record.span_id not in representatives:
            representatives[record.span_id] = record
    return representatives


def _record_sort_key(record: OTelLogRecord) -> tuple[str, str]:
    return (record.timestamp or "", record.span_id or "")


def _is_error_record(record: OTelLogRecord) -> bool:
    severity = record.severity.upper()
    body = record.body.lower()
    return severity in {"ERROR", "FATAL", "CRITICAL"} or any(
        token in body
        for token in (
            "error",
            "exception",
            "failed",
            "failure",
            "timeout",
            "insufficient_stock",
            "gateway_timeout",
        )
    )


def _max_severity(severities: list[str]) -> str:
    rank = {
        "UNSPECIFIED": 0,
        "DEBUG": 1,
        "INFO": 2,
        "WARN": 3,
        "WARNING": 3,
        "ERROR": 4,
        "FATAL": 5,
        "CRITICAL": 5,
    }
    return max(severities or ["UNSPECIFIED"], key=lambda value: rank.get(value.upper(), 0))
