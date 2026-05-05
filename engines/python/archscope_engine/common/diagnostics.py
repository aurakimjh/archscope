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
