from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

MAX_DIAGNOSTIC_SAMPLES = 20
RAW_PREVIEW_LIMIT = 200
ParseError = tuple[str, str]


@dataclass
class ParserDiagnostics:
    total_lines: int = 0
    parsed_records: int = 0
    skipped_lines: int = 0
    skipped_by_reason: dict[str, int] = field(default_factory=dict)
    samples: list[dict[str, int | str]] = field(default_factory=list)

    def add_skipped(
        self,
        *,
        line_number: int,
        reason: str,
        message: str,
        raw_line: str,
    ) -> None:
        self.skipped_lines += 1
        self.skipped_by_reason[reason] = self.skipped_by_reason.get(reason, 0) + 1

        if len(self.samples) >= MAX_DIAGNOSTIC_SAMPLES:
            return

        self.samples.append(
            {
                "line_number": line_number,
                "reason": reason,
                "message": message,
                "raw_preview": raw_line[:RAW_PREVIEW_LIMIT],
            }
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "total_lines": self.total_lines,
            "parsed_records": self.parsed_records,
            "skipped_lines": self.skipped_lines,
            "skipped_by_reason": dict(self.skipped_by_reason),
            "samples": list(self.samples),
        }
