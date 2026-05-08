"""Parser diagnostics counter (skip reasons + samples)."""
# ─────────────────────────────────────────────────────────────────────
# [한글] diagnostics — 파서가 채우는 진단 카운터.
#
# 책임/목적
#   parser 가 라인 단위로 처리하면서 skip / warning / error 가 발생할
#   때마다 자동 누적. analyzer 가 metadata.diagnostics 로 노출 →
#   사용자가 "내 파일이 얼마나 잘 파싱됐는가" 를 확인할 수 있게 함.
#
# 카운트 항목
#   - total_lines: 전체 입력 라인 수.
#   - parsed_records: 정상 record 로 변환된 수.
#   - skipped_lines + skipped_by_reason[reason]: 스킵 + 사유별 카운트.
#   - warning_count / error_count: 경고/에러 누적.
#   - samples / warnings / errors: 사례 샘플 (각 100개 cap).
#
# 상수
#   MAX_DIAGNOSTIC_SAMPLES (100): 샘플 수집 cap.
#   RAW_PREVIEW_LIMIT (200): 한 샘플의 raw_preview 길이 cap.
#
# parity: Go engine-native internal/diagnostics 와 동일 카운트 / cap.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

MAX_DIAGNOSTIC_SAMPLES = 100
RAW_PREVIEW_LIMIT = 200
ParseError = tuple[str, str]


@dataclass
class ParserDiagnostics:
    source_file: str | None = None
    format: str | None = None
    total_lines: int = 0
    parsed_records: int = 0
    skipped_lines: int = 0
    skipped_by_reason: dict[str, int] = field(default_factory=dict)
    samples: list[dict[str, int | str]] = field(default_factory=list)
    warning_count: int = 0
    error_count: int = 0
    warnings: list[dict[str, int | str]] = field(default_factory=list)
    errors: list[dict[str, int | str]] = field(default_factory=list)

    def set_context(self, *, source_file: str | None = None, format: str | None = None) -> None:
        if source_file is not None:
            self.source_file = source_file
        if format is not None:
            self.format = format

    def add_skipped(
        self,
        *,
        line_number: int,
        reason: str,
        message: str,
        raw_line: str,
        severity: str = "error",
    ) -> None:
        self.skipped_lines += 1
        self.skipped_by_reason[reason] = self.skipped_by_reason.get(reason, 0) + 1
        self._add_issue(
            line_number=line_number,
            reason=reason,
            message=message,
            raw_line=raw_line,
            severity=severity,
            include_in_samples=True,
        )

    def add_warning(
        self,
        *,
        line_number: int,
        reason: str,
        message: str,
        raw_line: str = "",
        skipped: bool = False,
    ) -> None:
        if skipped:
            self.add_skipped(
                line_number=line_number,
                reason=reason,
                message=message,
                raw_line=raw_line,
                severity="warning",
            )
            return
        self._add_issue(
            line_number=line_number,
            reason=reason,
            message=message,
            raw_line=raw_line,
            severity="warning",
            include_in_samples=True,
        )

    def add_error(
        self,
        *,
        line_number: int,
        reason: str,
        message: str,
        raw_line: str = "",
    ) -> None:
        self._add_issue(
            line_number=line_number,
            reason=reason,
            message=message,
            raw_line=raw_line,
            severity="error",
            include_in_samples=True,
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "source_file": self.source_file,
            "format": self.format,
            "total_lines": self.total_lines,
            "parsed_records": self.parsed_records,
            "skipped_lines": self.skipped_lines,
            "skipped_by_reason": dict(self.skipped_by_reason),
            "samples": list(self.samples),
            "warning_count": self.warning_count,
            "error_count": self.error_count,
            "warnings": list(self.warnings),
            "errors": list(self.errors),
        }

    def _add_issue(
        self,
        *,
        line_number: int,
        reason: str,
        message: str,
        raw_line: str,
        severity: str,
        include_in_samples: bool,
    ) -> None:
        issue = {
            "line_number": line_number,
            "reason": reason,
            "message": message,
            "raw_preview": raw_line[:RAW_PREVIEW_LIMIT],
        }
        if severity == "warning":
            self.warning_count += 1
            if len(self.warnings) < MAX_DIAGNOSTIC_SAMPLES:
                self.warnings.append(issue)
        else:
            self.error_count += 1
            if len(self.errors) < MAX_DIAGNOSTIC_SAMPLES:
                self.errors.append(issue)

        if include_in_samples and len(self.samples) < MAX_DIAGNOSTIC_SAMPLES:
            self.samples.append(issue)
