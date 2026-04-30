from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.thread_dump import ExceptionRecord
from archscope_engine.parsers.exception_parser import parse_exception_stack


def analyze_exception_stack(path: Path, *, top_n: int = 20) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    records = parse_exception_stack(path, diagnostics=diagnostics)
    return build_exception_stack_result(
        records,
        source_file=path,
        diagnostics=diagnostics.to_dict(),
        top_n=top_n,
    )


def build_exception_stack_result(
    records: list[ExceptionRecord],
    *,
    source_file: Path,
    diagnostics: dict[str, Any],
    top_n: int = 20,
) -> AnalysisResult:
    type_counts = Counter(record.exception_type for record in records)
    root_counts = Counter(record.root_cause or record.exception_type for record in records)
    signature_counts = Counter(record.signature for record in records)
    top_exception_type = type_counts.most_common(1)[0][0] if type_counts else None

    summary = {
        "total_exceptions": len(records),
        "unique_exception_types": len(type_counts),
        "unique_signatures": len(signature_counts),
        "top_exception_type": top_exception_type,
    }
    series = {
        "exception_type_distribution": [
            {"exception_type": key, "count": value}
            for key, value in type_counts.most_common(top_n)
        ],
        "root_cause_distribution": [
            {"root_cause": key, "count": value}
            for key, value in root_counts.most_common(top_n)
        ],
        "top_stack_signatures": [
            {"signature": key, "count": value}
            for key, value in signature_counts.most_common(top_n)
        ],
    }
    tables = {
        "exceptions": [
            {
                "timestamp": record.timestamp.isoformat() if record.timestamp else None,
                "language": record.language,
                "exception_type": record.exception_type,
                "message": record.message,
                "root_cause": record.root_cause,
                "signature": record.signature,
                "top_frame": record.stack[0] if record.stack else None,
                "stack": record.stack,
            }
            for record in records[:top_n]
        ]
    }
    metadata = {
        "parser": "java_exception_stack",
        "schema_version": "0.1.0",
        "diagnostics": diagnostics,
        "findings": _build_findings(records, signature_counts),
    }
    return AnalysisResult(
        type="exception_stack",
        source_files=[str(source_file)],
        summary=summary,
        series=series,
        tables=tables,
        metadata=metadata,
    )


def _build_findings(
    records: list[ExceptionRecord],
    signature_counts: Counter[str],
) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    repeated = signature_counts.most_common(1)
    if repeated and repeated[0][1] > 1:
        findings.append(
            {
                "severity": "warning",
                "code": "REPEATED_EXCEPTION_SIGNATURE",
                "message": "The same exception stack signature appears multiple times.",
                "evidence": {"signature": repeated[0][0], "count": repeated[0][1]},
            }
        )
    if any(record.root_cause for record in records):
        findings.append(
            {
                "severity": "info",
                "code": "ROOT_CAUSE_PRESENT",
                "message": "Nested root-cause exceptions were detected.",
                "evidence": {
                    "root_cause_count": sum(1 for record in records if record.root_cause)
                },
            }
        )
    return findings
