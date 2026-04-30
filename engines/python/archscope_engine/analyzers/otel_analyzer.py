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
    for record in records:
        if record.trace_id and record.service_name:
            trace_services[record.trace_id].add(record.service_name)
    cross_service_traces = {
        trace_id: services
        for trace_id, services in trace_services.items()
        if len(services) > 1
    }
    summary = {
        "total_records": len(records),
        "unique_traces": len([key for key in trace_counts if key != "(no-trace)"]),
        "unique_services": len([key for key in service_counts if key != "(unknown)"]),
        "cross_service_traces": len(cross_service_traces),
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
    return findings
