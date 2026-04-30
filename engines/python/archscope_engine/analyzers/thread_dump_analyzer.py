from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.thread_dump import ThreadDumpRecord
from archscope_engine.parsers.thread_dump_parser import parse_thread_dump


def analyze_thread_dump(path: Path, *, top_n: int = 20) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    records = parse_thread_dump(path, diagnostics=diagnostics)
    return build_thread_dump_result(
        records,
        source_file=path,
        diagnostics=diagnostics.to_dict(),
        top_n=top_n,
    )


def build_thread_dump_result(
    records: list[ThreadDumpRecord],
    *,
    source_file: Path,
    diagnostics: dict[str, Any],
    top_n: int = 20,
) -> AnalysisResult:
    state_counts = Counter(record.state or "UNKNOWN" for record in records)
    category_counts = Counter(record.category or "UNKNOWN" for record in records)
    signatures = Counter(_stack_signature(record) for record in records)
    blocked = category_counts.get("BLOCKED", 0)
    waiting = category_counts.get("WAITING", 0)

    summary = {
        "total_threads": len(records),
        "runnable_threads": category_counts.get("RUNNABLE", 0),
        "blocked_threads": blocked,
        "waiting_threads": waiting,
        "threads_with_locks": sum(1 for record in records if record.lock_info),
    }
    series = {
        "state_distribution": [
            {"state": key, "count": value}
            for key, value in state_counts.most_common()
        ],
        "category_distribution": [
            {"category": key, "count": value}
            for key, value in category_counts.most_common()
        ],
        "top_stack_signatures": [
            {"signature": key, "count": value}
            for key, value in signatures.most_common(top_n)
        ],
    }
    tables = {
        "threads": [
            {
                "thread_name": record.thread_name,
                "thread_id": record.thread_id,
                "state": record.state,
                "category": record.category,
                "top_frame": record.stack[0] if record.stack else None,
                "lock_info": record.lock_info,
                "stack": record.stack,
            }
            for record in records[:top_n]
        ]
    }
    metadata = {
        "parser": "java_thread_dump",
        "schema_version": "0.1.0",
        "diagnostics": diagnostics,
        "findings": _build_findings(summary),
    }
    return AnalysisResult(
        type="thread_dump",
        source_files=[str(source_file)],
        summary=summary,
        series=series,
        tables=tables,
        metadata=metadata,
    )


def _stack_signature(record: ThreadDumpRecord) -> str:
    if not record.stack:
        return "(no-stack)"
    return " | ".join(record.stack[:3])


def _build_findings(summary: dict[str, int]) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    if summary["blocked_threads"] > 0:
        findings.append(
            {
                "severity": "warning",
                "code": "BLOCKED_THREADS_PRESENT",
                "message": "Blocked Java threads were detected.",
                "evidence": {"blocked_threads": summary["blocked_threads"]},
            }
        )
    if summary["waiting_threads"] > summary["runnable_threads"]:
        findings.append(
            {
                "severity": "info",
                "code": "WAITING_THREADS_DOMINATE",
                "message": "Waiting threads exceed runnable threads.",
                "evidence": {
                    "waiting_threads": summary["waiting_threads"],
                    "runnable_threads": summary["runnable_threads"],
                },
            }
        )
    return findings
