from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import detect_text_encoding
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.runtime_stack import RuntimeStackRecord
from archscope_engine.parsers.dotnet_parser import parse_dotnet_exception_and_iis
from archscope_engine.parsers.go_panic_parser import parse_go_panic
from archscope_engine.parsers.nodejs_stack_parser import parse_nodejs_stack
from archscope_engine.parsers.python_traceback_parser import parse_python_traceback


def analyze_nodejs_stack(
    path: Path,
    *,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    records = parse_nodejs_stack(path, diagnostics=diagnostics, debug_log=debug_log)
    return _build_stack_result(
        result_type="nodejs_stack",
        parser="nodejs_stack_trace",
        records=records,
        source_file=path,
        diagnostics=diagnostics.to_dict(),
        top_n=top_n,
    )


def analyze_python_traceback(
    path: Path,
    *,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    records = parse_python_traceback(path, diagnostics=diagnostics, debug_log=debug_log)
    return _build_stack_result(
        result_type="python_traceback",
        parser="python_traceback",
        records=records,
        source_file=path,
        diagnostics=diagnostics.to_dict(),
        top_n=top_n,
    )


def analyze_go_panic(
    path: Path,
    *,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    records = parse_go_panic(path, diagnostics=diagnostics, debug_log=debug_log)
    return _build_stack_result(
        result_type="go_panic",
        parser="go_panic_goroutine",
        records=records,
        source_file=path,
        diagnostics=diagnostics.to_dict(),
        top_n=top_n,
    )


def analyze_dotnet_exception_iis(
    path: Path,
    *,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    exceptions, iis_records = parse_dotnet_exception_and_iis(
        path,
        diagnostics=diagnostics,
        debug_log=debug_log,
    )
    stack_result = _stack_payload(exceptions, top_n=top_n)
    status_counts = Counter(_status_bucket(record.status) for record in iis_records)
    slow_urls = Counter[str]()
    for record in sorted(
        iis_records,
        key=lambda item: item.time_taken_ms or 0,
        reverse=True,
    )[:top_n]:
        slow_urls[f"{record.method} {record.uri}"] = record.time_taken_ms or 0
    summary = {
        "total_exceptions": len(exceptions),
        "unique_signatures": len(stack_result["signature_counts"]),
        "iis_requests": len(iis_records),
        "iis_error_requests": sum(1 for record in iis_records if record.status >= 500),
        "max_iis_time_taken_ms": max(
            (record.time_taken_ms or 0 for record in iis_records),
            default=0,
        ),
    }
    series = {
        **stack_result["series"],
        "iis_status_distribution": [
            {"status": key, "count": value}
            for key, value in status_counts.most_common()
        ],
        "iis_slowest_urls": [
            {"uri": key, "time_taken_ms": value}
            for key, value in slow_urls.most_common(top_n)
        ],
    }
    tables = {
        **stack_result["tables"],
        "iis_requests": [
            {
                "method": record.method,
                "uri": record.uri,
                "status": record.status,
                "time_taken_ms": record.time_taken_ms,
            }
            for record in iis_records[:top_n]
        ],
    }
    metadata = _metadata(
        parser="dotnet_exception_iis_w3c",
        diagnostics=diagnostics.to_dict(),
        findings=_dotnet_findings(summary),
    )
    return AnalysisResult(
        type="dotnet_exception_iis",
        source_files=[str(path)],
        summary=summary,
        series=series,
        tables=tables,
        metadata=metadata,
    )


def _build_stack_result(
    *,
    result_type: str,
    parser: str,
    records: list[RuntimeStackRecord],
    source_file: Path,
    diagnostics: dict[str, Any],
    top_n: int,
) -> AnalysisResult:
    payload = _stack_payload(records, top_n=top_n)
    summary = {
        "total_records": len(records),
        "unique_record_types": len(payload["type_counts"]),
        "unique_signatures": len(payload["signature_counts"]),
        "top_record_type": payload["top_record_type"],
    }
    return AnalysisResult(
        type=result_type,
        source_files=[str(source_file)],
        summary=summary,
        series=payload["series"],
        tables=payload["tables"],
        metadata=_metadata(
            parser=parser,
            diagnostics=diagnostics,
            findings=_stack_findings(records, payload["signature_counts"]),
        ),
    )


def _stack_payload(
    records: list[RuntimeStackRecord],
    *,
    top_n: int,
) -> dict[str, Any]:
    type_counts = Counter(record.record_type for record in records)
    signature_counts = Counter(record.signature for record in records)
    top_record_type = type_counts.most_common(1)[0][0] if type_counts else None
    return {
        "type_counts": type_counts,
        "signature_counts": signature_counts,
        "top_record_type": top_record_type,
        "series": {
            "record_type_distribution": [
                {"record_type": key, "count": value}
                for key, value in type_counts.most_common(top_n)
            ],
            "top_stack_signatures": [
                {"signature": key, "count": value}
                for key, value in signature_counts.most_common(top_n)
            ],
        },
        "tables": {
            "records": [
                {
                    "runtime": record.runtime,
                    "record_type": record.record_type,
                    "headline": record.headline,
                    "message": record.message,
                    "signature": record.signature,
                    "top_frame": record.stack[0] if record.stack else None,
                    "stack": record.stack,
                }
                for record in records[:top_n]
            ]
        },
    }


def _metadata(
    *,
    parser: str,
    diagnostics: dict[str, Any],
    findings: list[dict[str, Any]],
) -> dict[str, Any]:
    return {
        "parser": parser,
        "schema_version": "0.1.0",
        "diagnostics": diagnostics,
        "findings": findings,
    }


def _stack_findings(
    records: list[RuntimeStackRecord],
    signature_counts: Counter[str],
) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    repeated = signature_counts.most_common(1)
    if repeated and repeated[0][1] > 1:
        findings.append(
            {
                "severity": "warning",
                "code": "REPEATED_RUNTIME_STACK_SIGNATURE",
                "message": "The same runtime stack signature appears multiple times.",
                "evidence": {"signature": repeated[0][0], "count": repeated[0][1]},
            }
        )
    if any(not record.stack for record in records):
        findings.append(
            {
                "severity": "info",
                "code": "STACKLESS_RUNTIME_RECORDS",
                "message": "Some records did not include stack frames.",
                "evidence": {
                    "count": sum(1 for record in records if not record.stack),
                },
            }
        )
    return findings


def _dotnet_findings(summary: dict[str, int]) -> list[dict[str, Any]]:
    findings = []
    if summary["iis_error_requests"] > 0:
        findings.append(
            {
                "severity": "warning",
                "code": "IIS_5XX_PRESENT",
                "message": "IIS access records include 5xx responses.",
                "evidence": {"iis_error_requests": summary["iis_error_requests"]},
            }
        )
    if summary["total_exceptions"] > 0:
        findings.append(
            {
                "severity": "info",
                "code": "DOTNET_EXCEPTIONS_PRESENT",
                "message": ".NET exception stack traces were detected.",
                "evidence": {"total_exceptions": summary["total_exceptions"]},
            }
        )
    return findings


def _status_bucket(status: int) -> str:
    return f"{status // 100}xx"
