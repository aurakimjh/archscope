from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from archscope_engine.ai_interpretation.validation import (
    AiFindingValidationError,
    AiFindingValidator,
)


@dataclass(frozen=True)
class InterpretationEvaluation:
    finding_count: int
    valid: bool
    evidence_integrity_ratio: float
    issue_codes: list[str]


def evaluate_interpretation(
    payload: dict[str, Any],
    validator: AiFindingValidator,
) -> InterpretationEvaluation:
    findings = payload.get("findings")
    finding_count = len(findings) if isinstance(findings, list) else 0
    try:
        validator.validate_interpretation(payload)
    except AiFindingValidationError as exc:
        valid_refs = max(
            sum(
                1
                for issue in exc.issues
                if not issue.code.startswith("EVIDENCE_REF")
                and issue.code != "EVIDENCE_QUOTE_MISMATCH"
            ),
            0,
        )
        total_issues = len(exc.issues)
        ratio = 0.0 if total_issues else 1.0
        if finding_count > 0:
            ratio = max(0.0, 1.0 - ((total_issues - valid_refs) / finding_count))
        return InterpretationEvaluation(
            finding_count=finding_count,
            valid=False,
            evidence_integrity_ratio=ratio,
            issue_codes=[issue.code for issue in exc.issues],
        )

    return InterpretationEvaluation(
        finding_count=finding_count,
        valid=True,
        evidence_integrity_ratio=1.0,
        issue_codes=[],
    )
