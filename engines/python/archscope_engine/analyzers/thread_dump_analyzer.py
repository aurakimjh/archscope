"""Single thread-dump analyzer (legacy Java jstack)."""
# ─────────────────────────────────────────────────────────────────────
# [한글] thread_dump_analyzer — 단일 Java jstack 덤프 분석기.
#
# 책임/목적
#   하나의 Java jstack 텍스트 파일을 받아 thread_dump 타입의
#   AnalysisResult 를 만든다. 다중 덤프 분석은 multi_thread_analyzer
#   가 담당하며, 이 모듈은 보편적인 단일-덤프 요약 (state 분포,
#   카테고리 분포, 상위 stack signature) 만 제공.
#
# 알고리즘 흐름
#   parse_thread_dump → ThreadDumpRecord 리스트 →
#   Counter 로 state/category/signature 분포 산출 →
#   simple finding(blocked_threads_present, waiting_threads_dominate) →
#   AnalysisResult 조립.
#
# 주요 함수
#   - analyze_thread_dump: 파일 경로 입력 진입점.
#   - build_thread_dump_result: 파싱 결과로부터 AnalysisResult 조립.
#   - _stack_signature: 상위 3 프레임 합쳐 signature 생성.
#   - _build_findings: blocked/waiting 비율 기반 finding.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import detect_text_encoding
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.thread_dump import ThreadDumpRecord
from archscope_engine.parsers.thread_dump_parser import parse_thread_dump


def analyze_thread_dump(
    path: Path,
    *,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    diagnostics = ParserDiagnostics()
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    records = parse_thread_dump(path, diagnostics=diagnostics, debug_log=debug_log)
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
